package config

import (
	"errors"
	"fmt"
	"strconv"
)

const (
	DefaultArchiveBatchSize = 100
	DefaultListBatchSize    = 1000
	MinBatchSize            = 10
	MaxBatchSize            = 10000

	DefaultAgeInDays = 365
	MinAgeInDays     = 30
	MaxAgeInDays     = 10000
)

var (
	ErrInvalidConfig = errors.New("invalid config")
)

// Configuration captures the plugin's external Configuration as exposed in the Mattermost server
// Configuration, as well as values computed from the Configuration. Any public fields will be
// deserialized from the Mattermost server Configuration in OnConfigurationChange.
//
// As plugins are inherently concurrent (hooks being called asynchronously), and the plugin
// Configuration can change at any time, access to the Configuration must be synchronized. The
// strategy used in this plugin is to guard a pointer to the Configuration, and clone the entire
// struct whenever it changes. You may replace this with whatever strategy you choose.
//
// If you add non-reference types to your Configuration struct, be sure to rewrite Clone as a deep
// copy appropriate for your types.
type Configuration struct {
	EnableChannelArchiver bool
	EnableUserCleanup     bool
	AgeInDays             int
	Frequency             string
	DayOfWeek             string
	TimeOfDay             string
	ExcludeChannels       string
	BatchSize             int
}

func NewConfiguration() *Configuration {
	return &Configuration{
		AgeInDays: DefaultAgeInDays,
		BatchSize: DefaultArchiveBatchSize,
	}
}

// Clone shallow copies the configuration. Your implementation may require a deep copy if
// your configuration has reference types.
func (c *Configuration) Clone() *Configuration {
	var clone = *c
	return &clone
}

func ParseInt(s string, min int, max int) (int, error) {
	i64, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, err
	}
	i := int(i64)

	if i < min {
		return 0, fmt.Errorf("number must be greater than or equal to %d", min)
	}

	if i > max {
		return 0, fmt.Errorf("number must be less than or equal to %d", max)
	}
	return i, nil
}
