package store

import "time"

// UnixMillis returns Unix time in milliseconds for persisted/API timestamps.
func UnixMillis(t time.Time) int64 { return t.UnixMilli() }

func NowMillis() int64 { return time.Now().UnixMilli() }

func TimeFromMillis(value int64) time.Time { return time.UnixMilli(value) }
