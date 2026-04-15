package meet

import (
	"bufio"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/lukaculjak/mak-cli/internal/meetings"
)

func promptString(r *bufio.Reader, label, current string) (string, error) {
	for {
		if current != "" {
			fmt.Printf("%s [%s]: ", label, current)
		} else {
			fmt.Printf("%s: ", label)
		}
		input, err := r.ReadString('\n')
		if err != nil {
			return "", err
		}
		input = strings.TrimSpace(input)
		if input == "" && current != "" {
			return current, nil
		}
		if input != "" {
			return input, nil
		}
		fmt.Println("  This field cannot be empty.")
	}
}

func promptConfirm(r *bufio.Reader, label string) (bool, error) {
	for {
		fmt.Printf("%s (y/n): ", label)
		input, err := r.ReadString('\n')
		if err != nil {
			return false, err
		}
		switch strings.ToLower(strings.TrimSpace(input)) {
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			fmt.Println("  Please enter y or n.")
		}
	}
}

func promptDays(r *bufio.Reader, current []int) ([]int, error) {
	currentStr := ""
	if len(current) > 0 {
		parts := make([]string, len(current))
		for i, d := range current {
			parts[i] = strconv.Itoa(d)
		}
		currentStr = strings.Join(parts, ", ")
	}

	for {
		fmt.Println("  1=Mon  2=Tue  3=Wed  4=Thu  5=Fri  6=Sat  7=Sun")
		if currentStr != "" {
			fmt.Printf("Days (comma-separated) [%s]: ", currentStr)
		} else {
			fmt.Print("Days (comma-separated): ")
		}
		input, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		input = strings.TrimSpace(input)
		if input == "" && len(current) > 0 {
			return current, nil
		}
		days, ok := parseDays(input)
		if !ok {
			fmt.Println("  Invalid input. Enter numbers 1-7 separated by commas (e.g. 1, 3, 5).")
			continue
		}
		return days, nil
	}
}

func parseDays(input string) ([]int, bool) {
	parts := strings.Split(input, ",")
	seen := map[int]bool{}
	var days []int
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 7 {
			return nil, false
		}
		if seen[n] {
			continue
		}
		seen[n] = true
		days = append(days, n)
	}
	if len(days) == 0 {
		return nil, false
	}
	sort.Ints(days)
	return days, true
}

func promptTime(r *bufio.Reader, label, current string) (string, error) {
	for {
		if current != "" {
			fmt.Printf("%s [%s]: ", label, current)
		} else {
			fmt.Printf("%s: ", label)
		}
		input, err := r.ReadString('\n')
		if err != nil {
			return "", err
		}
		input = strings.TrimSpace(input)
		if input == "" && current != "" {
			return current, nil
		}
		if isValidTime(input) {
			return input, nil
		}
		fmt.Println("  Invalid time. Use 24h format like 09:00 or 14:30.")
	}
}

func isValidTime(t string) bool {
	parts := strings.Split(t, ":")
	if len(parts) != 2 {
		return false
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return false
	}
	return h >= 0 && h <= 23 && m >= 0 && m <= 59
}

// collectMeetingDetails interactively collects meeting details and loops until
// the user confirms the summary. Pass existing to pre-fill values for edit mode.
// aliasConflicts reports whether a given alias is already taken.
func collectMeetingDetails(r *bufio.Reader, existing *meetings.Meeting, aliasConflicts func(string) bool) (*meetings.Meeting, error) {
	var defaultAlias, defaultLink string
	var defaultDays []int
	var defaultSchedule []meetings.DaySchedule

	if existing != nil {
		defaultAlias = existing.Alias
		defaultLink = existing.Link
		for _, s := range existing.Schedule {
			defaultDays = append(defaultDays, s.Day)
		}
		defaultSchedule = existing.Schedule
	}

	for {
		fmt.Println()

		// Alias
		alias, err := promptString(r, "Meeting name (alias)", defaultAlias)
		if err != nil {
			return nil, err
		}
		if aliasConflicts(alias) {
			fmt.Printf("  A meeting named %q already exists. Please choose a different name.\n", alias)
			defaultAlias = alias
			continue
		}

		// Link
		link, err := promptString(r, "Meeting link", defaultLink)
		if err != nil {
			return nil, err
		}

		// Days
		fmt.Println("On which days of the week does this meeting repeat?")
		days, err := promptDays(r, defaultDays)
		if err != nil {
			return nil, err
		}

		// Times
		var schedule []meetings.DaySchedule

		if len(days) == 1 {
			var currentTime string
			if len(defaultSchedule) > 0 {
				currentTime = defaultSchedule[0].Time
			}
			t, err := promptTime(r, fmt.Sprintf("Start time on %s (HH:MM)", meetings.DayFull[days[0]]), currentTime)
			if err != nil {
				return nil, err
			}
			schedule = []meetings.DaySchedule{{Day: days[0], Time: t}}
		} else {
			sameTime, err := promptConfirm(r, "Does the meeting start at the same time each day?")
			if err != nil {
				return nil, err
			}
			if sameTime {
				var currentTime string
				if len(defaultSchedule) > 0 {
					currentTime = defaultSchedule[0].Time
				}
				t, err := promptTime(r, "Start time each day (HH:MM)", currentTime)
				if err != nil {
					return nil, err
				}
				for _, d := range days {
					schedule = append(schedule, meetings.DaySchedule{Day: d, Time: t})
				}
			} else {
				for _, d := range days {
					var currentTime string
					for _, s := range defaultSchedule {
						if s.Day == d {
							currentTime = s.Time
							break
						}
					}
					t, err := promptTime(r, fmt.Sprintf("Start time on %s (HH:MM)", meetings.DayFull[d]), currentTime)
					if err != nil {
						return nil, err
					}
					schedule = append(schedule, meetings.DaySchedule{Day: d, Time: t})
				}
			}
		}

		m := &meetings.Meeting{Alias: alias, Link: link, Schedule: schedule}

		// Summary
		fmt.Println()
		fmt.Println("Does this look okay to you?")
		fmt.Printf("  Meeting name: %s\n", m.Alias)
		fmt.Printf("  Meeting link: %s\n", m.Link)
		fmt.Printf("  %s\n", m.FormatScheduleDetailed())
		fmt.Println()

		ok, err := promptConfirm(r, "Confirm")
		if err != nil {
			return nil, err
		}
		if ok {
			return m, nil
		}

		// Let user start over with current values as defaults
		defaultAlias = alias
		defaultLink = link
		defaultDays = days
		defaultSchedule = schedule
	}
}
