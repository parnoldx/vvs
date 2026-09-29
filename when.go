// Natural-language date/time parsing — "8:00" (today, tomorrow if past),
// "tomorrow 17:30", "friday", "morgen 7:45", "2026-10-01 08:00", "1.10." —
// English and German words alike. Returns nil for empty input.
package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type When struct {
	Date string // "YYYYMMDD", "" = today
	Time string // "HHMM", "" = now
}

// times need a colon ("8:00"); dots stay reserved for dates like "1.10."
var timeRe = regexp.MustCompile(`(^|\s)(\d{1,2}):(\d{2})(:\d{2})?\b`)
var isoRe = regexp.MustCompile(`(\d{4})-(\d{1,2})-(\d{1,2})`)
var gerRe = regexp.MustCompile(`(^|\s)(\d{1,2})\.(\d{1,2})\.?\s*(\d{4})?`)
var notWordRe = regexp.MustCompile(`[^a-zà-ü]+`)

// weekday: 1 = Monday … 7 = Sunday, JS getDay() style (Sunday = 0) input
var weekdayNames = map[string]int{
	"montag": 1, "monday": 1, "mon": 1,
	"die": 2, "dienstag": 2, "tuesday": 2, "tue": 2,
	"mit": 3, "mittwoch": 3, "wednesday": 3, "wed": 3,
	"don": 4, "donnerstag": 4, "thursday": 4, "thu": 4,
	"fre": 5, "freitag": 5, "friday": 5, "fri": 5,
	"sam": 6, "samstag": 6, "saturday": 6, "sat": 6,
	"son": 7, "sonntag": 7, "sunday": 7, "sun": 7,
}

func pad2(n int) string { return fmt.Sprintf("%02d", n) }

func parseWhen(input string) *When {
	s := strings.ToLower(strings.TrimSpace(input))
	if s == "" {
		return nil
	}
	var hh, mm int
	var hasTime bool
	if tm := timeRe.FindStringSubmatch(s); tm != nil {
		hh, _ = strconv.Atoi(tm[2])
		mm, _ = strconv.Atoi(tm[3])
		hasTime = true
	}
	var y, mo, d int
	var haveDate bool
	dayOffset := -1 // -1 = unset
	if m := isoRe.FindStringSubmatch(s); m != nil {
		y, _ = strconv.Atoi(m[1])
		mo, _ = strconv.Atoi(m[2])
		d, _ = strconv.Atoi(m[3])
		haveDate = true
	} else if m := gerRe.FindStringSubmatch(s); m != nil {
		if m[4] != "" {
			y, _ = strconv.Atoi(m[4])
		} else {
			y = time.Now().Year()
		}
		mo, _ = strconv.Atoi(m[3])
		d, _ = strconv.Atoi(m[2])
		haveDate = true
	} else if strings.Contains(s, "übermorgen") || strings.Contains(s, "day after tomorrow") {
		dayOffset = 2
	} else if wordBoundary(s, "morgen") || wordBoundary(s, "tomorrow") {
		dayOffset = 1
	} else if wordBoundary(s, "heute") || wordBoundary(s, "today") {
		dayOffset = 0
	} else {
		for _, word := range notWordRe.Split(s, -1) {
			if wd, ok := weekdayNames[word]; ok {
				dayOffset = (wd - int(time.Now().Weekday()) + 7) % 7
				if dayOffset == 0 {
					dayOffset = 7
				}
				break
			}
		}
	}
	var date string
	if haveDate {
		date = fmt.Sprintf("%04d%02d%02d", y, mo, d)
	} else {
		now := time.Now()
		past := hasTime && now.Hour()*60+now.Minute() >= hh*60+mm
		shift := 0
		if dayOffset >= 0 {
			shift = dayOffset
		} else if past {
			shift = 1
		}
		now = now.AddDate(0, 0, shift)
		date = now.Format("20060102")
	}
	timeStr := ""
	if hasTime {
		timeStr = pad2(hh) + pad2(mm)
	}
	return &When{Date: date, Time: timeStr}
}

// \b(...) match on plain ASCII words, Go regexp \b agrees here
func wordBoundary(s, word string) bool {
	re := regexp.MustCompile(`\b` + word + `\b`)
	return re.MatchString(s)
}
