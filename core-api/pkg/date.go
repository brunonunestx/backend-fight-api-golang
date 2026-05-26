package pkg

import "time"

func GetHourOfDay(timestamp string) int {
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return 0
	}
	return t.Hour()
}

func GetDayOfWeek(timestamp string) int {
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return 0
	}
	return int(t.Weekday())
}

func CalcMinutesBetween(t1, t2 string) float64 {
	time1, err1 := time.Parse(time.RFC3339, t1)
	time2, err2 := time.Parse(time.RFC3339, t2)
	if err1 != nil || err2 != nil {
		return 0
	}
	return time2.Sub(time1).Minutes()
}

