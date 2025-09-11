package utils

import (
	"time"
)

// IsWithinLastMinute checks if the given timestamp is within the last minute
func IsWithinLastMinute(timestamp time.Time) bool {
	return time.Since(timestamp) <= time.Minute
}

// TruncateToMinute truncates a time to the minute (removes seconds and nanoseconds)
func TruncateToMinute(t time.Time) time.Time {
	return t.Truncate(time.Minute)
}

// GetCurrentMinuteKey returns a string key representing the current minute
// This can be used for caching purposes
func GetCurrentMinuteKey() string {
	return TruncateToMinute(time.Now()).Format("2006-01-02T15:04")
}

// GetMinuteKey returns a string key representing the minute for the given time
func GetMinuteKey(t time.Time) string {
	return TruncateToMinute(t).Format("2006-01-02T15:04")
}

// IsTimestampStale checks if a timestamp is older than the specified duration
func IsTimestampStale(timestamp time.Time, staleDuration time.Duration) bool {
	return time.Since(timestamp) > staleDuration
}

// MinutesAgo returns a time that is the specified number of minutes ago
func MinutesAgo(minutes int) time.Time {
	return time.Now().Add(-time.Duration(minutes) * time.Minute)
}
