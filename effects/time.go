package effects

import (
	"time"

	"github.com/rickb777/date/v2/timespan"
)

type TimeSpan = timespan.TimeSpan

func NewTimeSpan(from, to time.Time) TimeSpan {
	return timespan.BetweenTimes(from, to)
}

const epsilon = time.Millisecond

func Now() TimeSpan {
	now := time.Now()
	return timespan.BetweenTimes(now.Add(-1*epsilon), now.Add(epsilon))
}

type TimeBounded interface {
	TimeSpan() TimeSpan
}
