package pkg

import "time"

func ParseTimestamp(b []byte) time.Time {
	if len(b) < 20 {
		return time.Time{}
	}
	year := int(b[0]-'0')*1000 + int(b[1]-'0')*100 + int(b[2]-'0')*10 + int(b[3]-'0')
	month := int(b[5]-'0')*10 + int(b[6]-'0')
	day := int(b[8]-'0')*10 + int(b[9]-'0')
	hour := int(b[11]-'0')*10 + int(b[12]-'0')
	min := int(b[14]-'0')*10 + int(b[15]-'0')
	sec := int(b[17]-'0')*10 + int(b[18]-'0')

	if b[19] != 'Z' && len(b) >= 25 {
		offHour := int(b[20]-'0')*10 + int(b[21]-'0')
		offMin := int(b[23]-'0')*10 + int(b[24]-'0')
		if b[19] == '+' {
			min -= offMin
			hour -= offHour
		} else {
			min += offMin
			hour += offHour
		}
	}

	return time.Date(year, time.Month(month), day, hour, min, sec, 0, time.UTC)
}

func GetHourOfDay(t time.Time) int {
	return t.UTC().Hour()
}

func GetDayOfWeek(t time.Time) int {
	return (int(t.UTC().Weekday()) + 6) % 7
}

func CalcMinutesBetween(t1, t2 time.Time) float64 {
	return t2.Sub(t1).Minutes()
}
