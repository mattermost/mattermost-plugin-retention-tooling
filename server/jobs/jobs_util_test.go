package jobs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFrequency_CalcNext(t *testing.T) {
	tests := []struct {
		name      string
		f         Frequency
		last      string
		dayOfWeek int
		timeOfDay string
		want      string
	}{
		{name: "monthly sundays", f: Monthly, last: "Aug 26, 2023 12:48am -0700", dayOfWeek: 0, timeOfDay: "1:00am -0700", want: "Oct 1, 2023 1:00am -0700"},
		{name: "monthly mondays", f: Monthly, last: "Aug 26, 2023 12:48am -0700", dayOfWeek: 1, timeOfDay: "1:00am -0700", want: "Oct 2, 2023 1:00am -0700"},
		{name: "monthly tuesdays", f: Monthly, last: "Aug 26, 2023 12:48am -0700", dayOfWeek: 2, timeOfDay: "1:00am -0700", want: "Sep 26, 2023 1:00am -0700"},
		{name: "monthly wednesdays", f: Monthly, last: "Aug 26, 2023 12:48am -0700", dayOfWeek: 3, timeOfDay: "1:00am -0700", want: "Sep 27, 2023 1:00am -0700"},
		{name: "monthly thursdays", f: Monthly, last: "Aug 26, 2023 12:48am -0700", dayOfWeek: 4, timeOfDay: "1:00am -0700", want: "Sep 28, 2023 1:00am -0700"},
		{name: "monthly fridays", f: Monthly, last: "Aug 26, 2023 12:48am -0700", dayOfWeek: 5, timeOfDay: "1:00am -0700", want: "Sep 29, 2023 1:00am -0700"},
		{name: "monthly saturdays", f: Monthly, last: "Aug 26, 2023 12:48am -0700", dayOfWeek: 6, timeOfDay: "1:00am -0700", want: "Sep 30, 2023 1:00am -0700"},

		{name: "weekly sundays (monday start)", f: Weekly, last: "Aug 28, 2023 12:48am -0700", dayOfWeek: 0, timeOfDay: "1:00am -0700", want: "Sep 10, 2023 1:00am -0700"},
		{name: "weekly mondays (monday start)", f: Weekly, last: "Aug 28, 2023 12:48am -0700", dayOfWeek: 1, timeOfDay: "1:00am -0700", want: "Sep 4, 2023 1:00am -0700"},
		{name: "weekly tuesdays (monday start)", f: Weekly, last: "Aug 28, 2023 12:48am -0700", dayOfWeek: 2, timeOfDay: "1:00am -0700", want: "Sep 5, 2023 1:00am -0700"},
		{name: "weekly wednesdays (monday start)", f: Weekly, last: "Aug 28, 2023 12:48am -0700", dayOfWeek: 3, timeOfDay: "1:00am -0700", want: "Sep 6, 2023 1:00am -0700"},
		{name: "weekly thursdays (monday start)", f: Weekly, last: "Aug 28, 2023 12:48am -0700", dayOfWeek: 4, timeOfDay: "1:00am -0700", want: "Sep 7, 2023 1:00am -0700"},
		{name: "weekly fridays (monday start)", f: Weekly, last: "Aug 28, 2023 12:48am -0700", dayOfWeek: 5, timeOfDay: "1:00am -0700", want: "Sep 8, 2023 1:00am -0700"},
		{name: "weekly saturdays (monday start)", f: Weekly, last: "Aug 28, 2023 12:48am -0700", dayOfWeek: 6, timeOfDay: "1:00am -0700", want: "Sep 9, 2023 1:00am -0700"},

		{name: "weekly sundays (saturday start)", f: Weekly, last: "Aug 26, 2023 12:48am -0700", dayOfWeek: 0, timeOfDay: "1:00am -0700", want: "Sep 3, 2023 1:00am -0700"},
		{name: "weekly mondays (saturday start)", f: Weekly, last: "Aug 26, 2023 12:48am -0700", dayOfWeek: 1, timeOfDay: "1:00am -0700", want: "Sep 4, 2023 1:00am -0700"},
		{name: "weekly tuesdays (saturday start)", f: Weekly, last: "Aug 26, 2023 12:48am -0700", dayOfWeek: 2, timeOfDay: "1:00am -0700", want: "Sep 5, 2023 1:00am -0700"},
		{name: "weekly wednesdays (saturday start)", f: Weekly, last: "Aug 26, 2023 12:48am -0700", dayOfWeek: 3, timeOfDay: "1:00am -0700", want: "Sep 6, 2023 1:00am -0700"},
		{name: "weekly thursdays (saturday start)", f: Weekly, last: "Aug 26, 2023 12:48am -0700", dayOfWeek: 4, timeOfDay: "1:00am -0700", want: "Sep 7, 2023 1:00am -0700"},
		{name: "weekly fridays (saturday start)", f: Weekly, last: "Aug 26, 2023 12:48am -0700", dayOfWeek: 5, timeOfDay: "1:00am -0700", want: "Sep 8, 2023 1:00am -0700"},
		{name: "weekly saturdays (saturday start)", f: Weekly, last: "Aug 26, 2023 12:48am -0700", dayOfWeek: 6, timeOfDay: "1:00am -0700", want: "Sep 2, 2023 1:00am -0700"},

		{name: "daily sundays", f: Daily, last: "Aug 6, 2023 12:48am -0700", dayOfWeek: 0, timeOfDay: "11:30pm -0700", want: "Aug 7, 2023 11:30pm -0700"},
		{name: "daily mondays", f: Daily, last: "Aug 7, 2023 12:48am -0700", dayOfWeek: 1, timeOfDay: "11:30pm -0700", want: "Aug 8, 2023 11:30pm -0700"},
		{name: "daily tuesdays", f: Daily, last: "Aug 8, 2023 12:48am -0700", dayOfWeek: 2, timeOfDay: "11:30pm -0700", want: "Aug 9, 2023 11:30pm -0700"},
		{name: "daily wednesdays", f: Daily, last: "Aug 9, 2023 12:48am -0700", dayOfWeek: 3, timeOfDay: "11:30pm -0700", want: "Aug 10, 2023 11:30pm -0700"},
		{name: "daily thursdays", f: Daily, last: "Aug 10, 2023 12:48am -0700", dayOfWeek: 4, timeOfDay: "11:30pm -0700", want: "Aug 11, 2023 11:30pm -0700"},
		{name: "daily fridays", f: Daily, last: "Aug 11, 2023 12:48am -0700", dayOfWeek: 5, timeOfDay: "11:30pm -0700", want: "Aug 12, 2023 11:30pm -0700"},
		{name: "daily saturdays", f: Daily, last: "Sep 30, 2023 12:48am -0700", dayOfWeek: 6, timeOfDay: "11:30pm -0700", want: "Oct 1, 2023 11:30pm -0700"},

		{name: "newyear monthly fridays", f: Monthly, last: "Dec 29, 2023 12:48am -0700", dayOfWeek: 5, timeOfDay: "1:00am -0700", want: "Feb 2, 2024 1:00am -0700"},
		{name: "newyear weekly sundays (monday start)", f: Weekly, last: "Dec 25, 2023 12:48am -0700", dayOfWeek: 0, timeOfDay: "1:00am -0700", want: "Jan 7, 2024 1:00am -0700"},

		{name: "monthly tuesdays -0400", f: Monthly, last: "Aug 26, 2023 11:24pm -0400", dayOfWeek: 2, timeOfDay: "1:30am -0400", want: "Sep 26, 2023 1:30am -0400"},
		{name: "monthly sundays -0700", f: Monthly, last: "Aug 26, 2023 11:24pm -0400", dayOfWeek: 2, timeOfDay: "1:30am -0700", want: "Sep 26, 2023 1:30am -0700"},

		{name: "monthly tuesdays UTC", f: Monthly, last: "Aug 26, 2023 11:24pm +0000", dayOfWeek: 2, timeOfDay: "1:30am +0000", want: "Sep 26, 2023 1:30am +0000"},
		{name: "monthly sundays UTC", f: Monthly, last: "Aug 26, 2023 11:24pm +0000", dayOfWeek: 2, timeOfDay: "1:30am +0000", want: "Sep 26, 2023 1:30am +0000"},

		{name: "monthly tuesdays mixed", f: Monthly, last: "Aug 26, 2023 7:00am -0700", dayOfWeek: 2, timeOfDay: "1:30am -0400", want: "Sep 26, 2023 1:30am -0400"},
		{name: "monthly sundays mixed", f: Monthly, last: "Aug 26, 2023 7:00am -0700", dayOfWeek: 2, timeOfDay: "1:30am -0400", want: "Sep 26, 2023 1:30am -0400"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			last, err := time.Parse(FullLayout, tt.last)
			require.NoError(t, err)
			timeOfDay, err := time.Parse(TimeOfDayLayout, tt.timeOfDay)
			require.NoError(t, err)

			zone, offset := timeOfDay.Zone()
			t.Logf("hour:=%d; minute=%d;  second=%d;  tz=%s; zone=%s;  offset=%d\n", timeOfDay.Hour(), timeOfDay.Minute(), timeOfDay.Second(), timeOfDay.Location(), zone, offset)

			got := tt.f.CalcNext(last, tt.dayOfWeek, timeOfDay)
			gotFormatted := got.Format(FullLayout)

			assert.Equal(t, tt.want, gotFormatted)
		})
	}
}
