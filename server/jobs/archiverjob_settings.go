package jobs

import (
	"fmt"
	"strings"
	"time"

	"github.com/mattermost/mattermost-plugin-retention-tooling/server/config"
)

const (
	FullLayout      = "Jan 2, 2006 3:04pm -0700"
	TimeOfDayLayout = "3:04pm -0700"
)

type ChannelArchiverJobSettings struct {
	EnableChannelArchiver           bool
	EnableChannelArchiverDryRunMode bool
	AgeInDays                       int
	Frequency                       Frequency
	DayOfWeek                       int
	TimeOfDay                       time.Time
	ExcludeChannels                 []string
	BatchSize                       int
	AdminChannel                    string
}

func (c *ChannelArchiverJobSettings) Clone() *ChannelArchiverJobSettings {
	exclude := make([]string, len(c.ExcludeChannels))
	copy(exclude, c.ExcludeChannels)

	return &ChannelArchiverJobSettings{
		EnableChannelArchiver:           c.EnableChannelArchiver,
		EnableChannelArchiverDryRunMode: c.EnableChannelArchiverDryRunMode,
		AgeInDays:                       c.AgeInDays,
		Frequency:                       c.Frequency,
		TimeOfDay:                       c.TimeOfDay,
		ExcludeChannels:                 exclude,
		BatchSize:                       c.BatchSize,
		AdminChannel:                    c.AdminChannel,
	}
}

func (c *ChannelArchiverJobSettings) String() string {
	return fmt.Sprintf("enabled=%T; ageDays=%d; freq=%s; tod=%s; batchSize=%d; excludeLen=%d",
		c.EnableChannelArchiver, c.AgeInDays, c.Frequency, c.TimeOfDay.Format(TimeOfDayLayout), c.BatchSize, len(c.ExcludeChannels))
}

func parseChannelArchiverJobSettings(cfg *config.Configuration) (*ChannelArchiverJobSettings, error) {
	if !cfg.EnableChannelArchiver {
		return &ChannelArchiverJobSettings{
			EnableChannelArchiver: false,
		}, nil
	}

	if cfg.AgeInDays < config.MinAgeInDays {
		return nil, fmt.Errorf("`Days of inactivity` cannot be less than %d", config.MinAgeInDays)
	}

	freq, err := FreqFromString(cfg.Frequency)
	if err != nil {
		return nil, err
	}

	dow, err := config.ParseInt(cfg.DayOfWeek, 0, 6)
	if err != nil {
		return nil, fmt.Errorf("cannot parse `Day of week`: %w", err)
	}

	tod, err := time.Parse(TimeOfDayLayout, cfg.TimeOfDay)
	if err != nil {
		return nil, fmt.Errorf("cannot parse `Time of day`: %w", err)
	}

	nospaces := strings.ReplaceAll(cfg.ExcludeChannels, " ", ",")
	split := strings.Split(nospaces, ",")
	excludes := make([]string, 0)
	for _, s := range split {
		ch := strings.TrimSpace(s)
		if ch != "" {
			excludes = append(excludes, ch)
		}
	}

	if cfg.BatchSize < config.MinBatchSize || cfg.BatchSize > config.MaxBatchSize {
		return nil, fmt.Errorf("`Batch size` cannot be less than %d or more than %d", config.MinBatchSize, config.MaxBatchSize)
	}

	return &ChannelArchiverJobSettings{
		EnableChannelArchiver:           cfg.EnableChannelArchiver,
		EnableChannelArchiverDryRunMode: cfg.EnableChannelArchiverDryRunMode,
		AgeInDays:                       cfg.AgeInDays,
		Frequency:                       freq,
		DayOfWeek:                       dow,
		TimeOfDay:                       tod,
		ExcludeChannels:                 excludes,
		BatchSize:                       cfg.BatchSize,
		AdminChannel:                    cfg.AdminChannel,
	}, nil
}
