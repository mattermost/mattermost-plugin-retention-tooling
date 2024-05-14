package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/mattermost/mattermost/server/public/pluginapi/experimental/command"

	"github.com/mattermost/mattermost-plugin-retention-tooling/server/bot"
	"github.com/mattermost/mattermost-plugin-retention-tooling/server/channels"
	"github.com/mattermost/mattermost-plugin-retention-tooling/server/config"
	"github.com/mattermost/mattermost-plugin-retention-tooling/server/store"
)

const (
	ArchiverTrigger    = "channel-archiver"
	paramNameDays      = "days"
	paramNameBatchSize = "batch-size"
	paramNameExclude   = "exclude"
)

type ErrInvalidSubCommand struct {
	subCommand string
}

func (e ErrInvalidSubCommand) Error() string {
	return "invalid subcommand '" + e.subCommand + "'"
}

type ChannelArchiverCmd struct {
	client   *pluginapi.Client
	sqlStore *store.SQLStore
	commands []*model.AutocompleteData
	bot      *bot.Bot
}

func getDefaultBatchSize(list bool) int {
	if list {
		return config.DefaultListBatchSize
	}
	return config.DefaultArchiveBatchSize
}

// RegisterChannelArchiver is called by the plugin to register all necessary commands
func RegisterChannelArchiver(client *pluginapi.Client, store *store.SQLStore) (*ChannelArchiverCmd, error) {
	cmdArchive := model.NewAutocompleteData("archive", "", "Archive stale channels")
	cmdList := model.NewAutocompleteData("list", "", "List stale channels that would be archived")
	cmdHelp := model.NewAutocompleteData("help", "", "Display help text")
	commands := []*model.AutocompleteData{cmdArchive, cmdList, cmdHelp}

	cmdArchive.AddNamedTextArgument(paramNameDays, "Number of days of inactivity for a channel to be considered stale", fmt.Sprintf("[int - min %d days]", config.MinAgeInDays), "[0-9]*", true)
	cmdArchive.AddNamedTextArgument(paramNameBatchSize, fmt.Sprintf("Channels will be archived in batches of this size. (default=%d)", config.DefaultArchiveBatchSize), "[int]", "[0-9]*", false)
	cmdArchive.AddNamedTextArgument(paramNameExclude, "Comma separated list of channel names/IDs to exclude. No Spaces.", "", "", false)

	cmdList.AddNamedTextArgument(paramNameDays, "Number of days of inactivity for a channel to be considered stale", fmt.Sprintf("[int - min %d days]", config.MinAgeInDays), "[0-9]*", true)
	cmdList.AddNamedTextArgument(paramNameExclude, "Comma separated list of channel names/IDs to exclude. No Spaces.", "", "", false)

	names := []string{}
	for _, c := range commands {
		names = append(names, c.Trigger)
	}
	hint := "[" + strings.Join(names[:4], "|") + "...]"

	cmd := model.NewAutocompleteData(ArchiverTrigger, hint, "Manage stale channels.")
	cmd.SubCommands = commands

	iconData, err := command.GetIconData(&client.System, "assets/archiver.svg")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get icon data")
	}

	bot, err := bot.New(client)
	if err != nil {
		return nil, err
	}

	err = client.SlashCommand.Register(&model.Command{
		Trigger:              ArchiverTrigger,
		DisplayName:          "Channel Archiver",
		Description:          "Manage and archive stale channels.",
		AutoComplete:         true,
		AutoCompleteDesc:     strings.Join(names, ", "),
		AutoCompleteHint:     "(subcommand)",
		AutocompleteData:     cmd,
		AutocompleteIconData: iconData,
	})
	if err != nil {
		return nil, err
	}

	return &ChannelArchiverCmd{
		client:   client,
		sqlStore: store,
		commands: commands,
		bot:      bot,
	}, nil
}

func (ca *ChannelArchiverCmd) Execute(args *model.CommandArgs) (*model.CommandResponse, error) {
	params := parseNamedArgs(args.Command)
	subCommand := params[SubCommandKey]

	var err error
	var msg string

	switch subCommand {
	case "archive":
		msg, err = ca.handleArchive(args, params, false)
	case "list":
		msg, err = ca.handleArchive(args, params, true)
	case "help":
		msg, err = ca.handleHelp()
	default:
		err = ErrInvalidSubCommand{subCommand: subCommand}
	}

	if msg != "" {
		_ = ca.bot.SendEphemeralPost(args.ChannelId, args.UserId, msg)
	}

	return &model.CommandResponse{}, err
}

func (ca *ChannelArchiverCmd) handleArchive(args *model.CommandArgs, params map[string]string, list bool) (string, error) {
	if !ca.client.User.HasPermissionTo(args.UserId, model.PermissionManageSystem) {
		return fmt.Sprintf("You require %s permissions to execute this command.", model.PermissionManageSystem.Id), nil
	}

	days, err := config.ParseInt(params[paramNameDays], config.MinAgeInDays, config.MaxAgeInDays)
	if err != nil {
		return fmt.Sprintf("Missing or invalid '%s' parameter: %s", paramNameDays, err.Error()), nil
	}

	batchSize := getDefaultBatchSize(list)
	if bs, ok := params[paramNameBatchSize]; ok {
		batchSize, err = config.ParseInt(bs, config.MinBatchSize, config.MaxBatchSize)
		if err != nil {
			return fmt.Sprintf("Invalid '%s' parameter: %s", paramNameBatchSize, err.Error()), nil
		}
	}

	var exclude []string
	if ex, ok := params[paramNameExclude]; ok {
		exclude = strings.Split(ex, ",")
	}

	opts := channels.ArchiverOpts{
		StaleChannelOpts: store.StaleChannelOpts{
			AgeInDays:                 days,
			ExcludeChannels:           exclude,
			IncludeChannelTypeOpen:    true,
			IncludeChannelTypePrivate: true,
		},
		BatchSize: batchSize,
		ListOnly:  list,
		ProgressFn: func(results *channels.ArchiverResults) {
			if list {
				return
			}
			ca.client.Log.Debug("Channel Archiver", "archived_count", len(results.ChannelsArchived))
			msg := fmt.Sprintf("Channel-archiver progress -- %d channels archived.", len(results.ChannelsArchived))
			_ = ca.bot.SendEphemeralPost(args.ChannelId, args.UserId, msg)
		},
	}

	results, err := channels.ArchiveStaleChannels(context.TODO(), ca.sqlStore, ca.client, opts)
	if err != nil {
		return fmt.Sprintf("Error archiving channels: %s", err.Error()), nil
	}

	if list {
		ca.reportChannelList(args, results.ChannelsArchived)
		msg := fmt.Sprintf("count: %d\n%s", len(results.ChannelsArchived), results.ExitReason)
		return msg, nil
	}

	return fmt.Sprintf("%d channels archived in %v.\n%s",
		len(results.ChannelsArchived), results.Duration, results.ExitReason), nil
}

func (ca *ChannelArchiverCmd) handleHelp() (string, error) {
	resp := ""
	for _, cmd := range ca.commands {
		desc := cmd.Trigger
		if cmd.HelpText != "" {
			desc += " - " + cmd.HelpText
		}
		resp += fmt.Sprintf("/%s %s\n", ArchiverTrigger, desc)
	}

	return resp, nil
}

func (ca *ChannelArchiverCmd) reportChannelList(args *model.CommandArgs, channelIDs []string) {
	total := len(channelIDs)
	const itemsPerPost = 500
	var sb strings.Builder
	var idx, start, itemsInPage int

	for _, ch := range channelIDs {
		sb.WriteString(ch)
		sb.WriteString("\n")
		itemsInPage++

		if itemsInPage >= itemsPerPost {
			msg := fmt.Sprintf("Stale channels %d to %d of %d\n%s", start+1, idx+1, total, sb.String())
			_ = ca.bot.SendEphemeralPost(args.ChannelId, args.UserId, msg)
			start = idx + 1
			itemsInPage = 0
			sb.Reset()
		}
		idx++
	}

	if itemsInPage > 0 {
		msg := fmt.Sprintf("Stale channels %d to %d of %d\n%s", start+1, idx, total, sb.String())
		_ = ca.bot.SendEphemeralPost(args.ChannelId, args.UserId, msg)
	}
}
