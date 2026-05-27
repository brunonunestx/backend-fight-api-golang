package pkg

import "time"

func ParseTimestamp(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func GetHourOfDay(t time.Time) int {
	return t.UTC().Hour()
}

func GetDayOfWeek(t time.Time) int {
	return int(t.UTC().Weekday())
}

func CalcMinutesBetween(t1, t2 time.Time) float64 {
	return t2.Sub(t1).Minutes()
}
