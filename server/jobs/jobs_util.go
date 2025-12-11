package jobs

import (
	"strings"
	"time"

	"github.com/pkg/errors"
)

const (
	Monthly Frequency = "monthly" // Run job monthly
	Daily   Frequency = "daily"   // Run job daily
	Weekly  Frequency = "weekly"  // Run job weekly
)

var (
	ErrInvalidFrequency = errors.New("invalid frequency")
)

type loggerIface interface {
	// Error logs an error message, optionally structured with alternating key, value parameters.
	Error(message string, keyValuePairs ...interface{})

	// Warn logs an error message, optionally structured with alternating key, value parameters.
	Warn(message string, keyValuePairs ...interface{})

	// Info logs an error message, optionally structured with alternating key, value parameters.
	Info(message string, keyValuePairs ...interface{})

	// Debug logs an error message, optionally structured with alternating key, value parameters.
	Debug(message string, keyValuePairs ...interface{})
}

type Frequency string

func FreqFromString(s string) (Frequency, error) {
	switch strings.ToLower(s) {
	case string(Monthly):
		return Monthly, nil
	case string(Weekly):
		return Weekly, nil
	case string(Daily):
		return Daily, nil
	default:
		return "", errors.Wrapf(ErrInvalidFrequency, "'%s' is not a valid frequency", s)
	}
}

// CalcNext determines the next time based on a starting time, this frequency, and the time of day option.
func (f Frequency) CalcNext(last time.Time, dayOfWeek int, timeOfDay time.Time) time.Time {
	// everything in UTC
	originalLocation := timeOfDay.Location()
	last = last.In(originalLocation)

	var next time.Time
	var dowAdjust bool

	switch f {
	case Monthly:
		next = time.Date(last.Year(), last.Month()+1, last.Day(), timeOfDay.Hour(), timeOfDay.Minute(), timeOfDay.Second(), 0, timeOfDay.Location())
		dowAdjust = true
	case Weekly:
		next = time.Date(last.Year(), last.Month(), last.Day()+7, timeOfDay.Hour(), timeOfDay.Minute(), timeOfDay.Second(), 0, timeOfDay.Location())
		dowAdjust = true
	case Daily:
		next = time.Date(last.Year(), last.Month(), last.Day()+1, timeOfDay.Hour(), timeOfDay.Minute(), timeOfDay.Second(), 0, timeOfDay.Location())
	}

	// adjust for day of week.
	if dowAdjust {
		nextWeekday := int(next.Weekday())
		if nextWeekday <= dayOfWeek {
			next = next.AddDate(0, 0, dayOfWeek-nextWeekday)
		} else {
			next = next.AddDate(0, 0, (dayOfWeek+7)-nextWeekday)
		}
	}

	return next.In(originalLocation)
}
