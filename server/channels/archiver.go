package channels

import (
	"bytes"
	"context"
	"fmt"
	"time"

	pluginapi "github.com/mattermost/mattermost/server/public/pluginapi"

	"github.com/mattermost/mattermost-plugin-retention-tooling/server/bot"
	"github.com/mattermost/mattermost-plugin-retention-tooling/server/store"
)

type Reason string

const (
	ReasonDone      Reason = "completed normally"
	ReasonCancelled Reason = "canceled"
	ReasonError     Reason = "error"
)

type ArchiverOpts struct {
	StaleChannelOpts store.StaleChannelOpts

	BatchSize   int
	ListOnly    bool // don't archive channels, just list results
	MaxWarnings int

	ProgressFn func(results *ArchiverResults) // optional callback to receive results per batch
	Bot        *bot.Bot                       // optional bot for posting channel archived notification posts
}

type ArchiverResults struct {
	ChannelsArchived []string
	ExitReason       Reason
	Duration         time.Duration
	start            time.Time
}

func ArchiveStaleChannels(ctx context.Context, sqlstore *store.SQLStore, client *pluginapi.Client, opts ArchiverOpts) (results *ArchiverResults, retErr error) {
	results = &ArchiverResults{
		ChannelsArchived: make([]string, 0),
		ExitReason:       ReasonDone,
		start:            time.Now(),
	}

	defer func() {
		if p := recover(); p != nil {
			retErr = fmt.Errorf("panic recovered: %v", p)
		}
		if retErr != nil {
			results.ExitReason = ReasonError
		}
		results.Duration = time.Since(results.start)
	}()

	if opts.ListOnly {
		return results, listStaleChannels(ctx, sqlstore, opts, results)
	}

	client.Log.Debug(
		"Archiving stale channels.",
		"AgeInDays", opts.StaleChannelOpts.AgeInDays,
		"exclude", opts.StaleChannelOpts.ExcludeChannels,
		"open", opts.StaleChannelOpts.IncludeChannelTypeOpen,
		"private", opts.StaleChannelOpts.IncludeChannelTypePrivate,
		"dm", opts.StaleChannelOpts.IncludeChannelTypeDirect,
		"gm", opts.StaleChannelOpts.IncludeChannelTypeGroup,
	)

	return results, archiveStaleChannels(ctx, sqlstore, client, opts, results)
}

func archiveStaleChannels(ctx context.Context, sqlstore *store.SQLStore, client *pluginapi.Client, opts ArchiverOpts, results *ArchiverResults) error {
	var buffer bytes.Buffer

	buffer.WriteString("Archived Channels:\n")
	for {
		staleChannels, more, err := sqlstore.GetStaleChannels(opts.StaleChannelOpts, 0, opts.BatchSize)
		if err != nil {
			results.ExitReason = ReasonError
			return fmt.Errorf("cannot fetch stale channels: %w", err)
		}

		for _, ch := range staleChannels {
			// archive the channel after posting notice.
			if opts.Bot != nil {
				msg := fmt.Sprintf("This channel has been archived due to inactivity for more than %d days.", opts.StaleChannelOpts.AgeInDays)
				_ = opts.Bot.SendPost(ch.Id, msg)
			}
			appErr := client.Channel.Delete(ch.Id)
			if appErr != nil {
				return fmt.Errorf("cannot archive channel %s (%s): %w", ch.Name, ch.Id, err)
			}
			archivedChannelStr := fmt.Sprintf("%s (%s)\n", ch.Name, ch.Id)
			results.ChannelsArchived = append(results.ChannelsArchived, archivedChannelStr)
			if opts.StaleChannelOpts.AdminChannel != "" {
				buffer.WriteString(archivedChannelStr)
			}

			// sleep a short time so we don't peg the cpu
			select {
			case <-time.After(time.Millisecond * 10):
			case <-ctx.Done():
				results.ExitReason = ReasonCancelled
				return nil
			}
		}

		if opts.ProgressFn != nil {
			opts.ProgressFn(results)
		}

		if !more {
			return handleAdminChannelPost(opts.Bot, &buffer, "archived", opts.StaleChannelOpts.AdminChannel, "The following channels have been archived:")
		}

		// sleep so we don't peg the cpu; longer here to allow websocket events to flush
		select {
		case <-time.After(time.Millisecond * 2000):
		case <-ctx.Done():
			results.ExitReason = ReasonCancelled
			return nil
		}
	}
}

func listStaleChannels(ctx context.Context, sqlstore *store.SQLStore, opts ArchiverOpts, results *ArchiverResults) error {
	page := 0
	var buffer bytes.Buffer

	buffer.WriteString("Stale Channels:\n")
	for {
		staleChannels, more, err := sqlstore.GetStaleChannels(opts.StaleChannelOpts, page, opts.BatchSize)
		if err != nil {
			results.ExitReason = ReasonError
			return fmt.Errorf("cannot fetch stale channels: %w", err)
		}
		page++

		for _, ch := range staleChannels {
			buffer.WriteString(fmt.Sprintf("%s (%s)\n", ch.Name, ch.Id))
			results.ChannelsArchived = append(results.ChannelsArchived, fmt.Sprintf("**%s** (%s)", ch.Name, ch.Id))
		}

		if !more {
			break
		}

		// sleep a short time so we don't peg the cpu
		select {
		case <-time.After(time.Millisecond * 10):
		case <-ctx.Done():
			results.ExitReason = ReasonCancelled
			return nil
		}
	}

	return handleAdminChannelPost(opts.Bot, &buffer, "stale", opts.StaleChannelOpts.AdminChannel, "The following channels have been identified as stale:")
}

func handleAdminChannelPost(bot *bot.Bot, buffer *bytes.Buffer, fileType string, adminChannel, msg string) error {
	if adminChannel != "" {
		timeMs := time.Now().UnixMilli()
		fileName := fmt.Sprintf("%d_%s-channels.txt", timeMs, fileType)
		fileInfo, err := bot.UploadFile(buffer, fileName, adminChannel)
		if err != nil {
			return fmt.Errorf("failed to upload file: %w", err)
		}

		err = bot.SendPostWithAttachment(adminChannel, msg, fileInfo)
		if err != nil {
			return fmt.Errorf("failed to create post: %w", err)
		}
	}

	return nil
}
