package meetings

import (
	"fmt"
	"strings"
)

type DaySchedule struct {
	Day  int    `json:"day"`
	Time string `json:"time"`
}

type Meeting struct {
	Alias    string        `json:"alias"`
	Link     string        `json:"link"`
	Schedule []DaySchedule `json:"schedule"`
}

var DayShort = map[int]string{
	1: "Mon", 2: "Tue", 3: "Wed", 4: "Thu", 5: "Fri", 6: "Sat", 7: "Sun",
}

var DayFull = map[int]string{
	1: "Monday", 2: "Tuesday", 3: "Wednesday", 4: "Thursday",
	5: "Friday", 6: "Saturday", 7: "Sunday",
}

// FormatSchedule returns a compact one-line schedule description.
func (m *Meeting) FormatSchedule() string {
	if len(m.Schedule) == 0 {
		return "no schedule"
	}

	allSame := true
	first := m.Schedule[0].Time
	for _, s := range m.Schedule[1:] {
		if s.Time != first {
			allSame = false
			break
		}
	}

	if allSame {
		days := make([]string, len(m.Schedule))
		for i, s := range m.Schedule {
			days[i] = DayShort[s.Day]
		}
		return fmt.Sprintf("repeats on %s at %s", strings.Join(days, ", "), first)
	}

	parts := make([]string, len(m.Schedule))
	for i, s := range m.Schedule {
		parts[i] = fmt.Sprintf("%s at %s", DayShort[s.Day], s.Time)
	}
	return "repeats on " + strings.Join(parts, ", ")
}

// FormatScheduleDetailed returns a human-readable schedule for detail views.
func (m *Meeting) FormatScheduleDetailed() string {
	if len(m.Schedule) == 0 {
		return "  no schedule"
	}

	allSame := true
	first := m.Schedule[0].Time
	for _, s := range m.Schedule[1:] {
		if s.Time != first {
			allSame = false
			break
		}
	}

	days := make([]string, len(m.Schedule))
	for i, s := range m.Schedule {
		days[i] = DayShort[s.Day]
	}

	if allSame {
		return fmt.Sprintf("Occurs on %s at %s", joinDays(days), first)
	}

	var sb strings.Builder
	sb.WriteString("Occurs on:\n")
	for _, s := range m.Schedule {
		sb.WriteString(fmt.Sprintf("  %s at %s\n", DayShort[s.Day], s.Time))
	}
	return strings.TrimRight(sb.String(), "\n")
}

func joinDays(days []string) string {
	switch len(days) {
	case 1:
		return days[0]
	case 2:
		return days[0] + " and " + days[1]
	default:
		return strings.Join(days[:len(days)-1], ", ") + " and " + days[len(days)-1]
	}
}
