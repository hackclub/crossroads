package db

import "time"

// CountsFrom is when the project's Hackatime time starts counting: its own
// hours_start if it kept an older start when the program's moved back, or
// else start, the program's.
func (p Project) CountsFrom(start time.Time) time.Time {
	if p.HoursStart != nil {
		return *p.HoursStart
	}
	return start
}
