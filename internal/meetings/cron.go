package meetings

import (
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

const cronMarker = "# mak:meet:"

func cronStartTag(alias string) string { return cronMarker + alias }
func cronEndTag(alias string) string   { return cronMarker + alias + ":end" }

func readCrontab() (string, error) {
	out, err := exec.Command("crontab", "-l").Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			return "", nil
		}
		return "", err
	}
	return string(out), nil
}

func writeCrontab(content string) error {
	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(content)
	return cmd.Run()
}

// stripCronBlock removes the tagged block for alias from crontab content.
func stripCronBlock(crontab, alias string) string {
	start := cronStartTag(alias)
	end := cronEndTag(alias)
	var out []string
	skipping := false
	for _, line := range strings.Split(crontab, "\n") {
		if line == start {
			skipping = true
			continue
		}
		if skipping {
			if line == end {
				skipping = false
			}
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// buildCronBlock generates the tagged cron block for a meeting.
// Days with the same start time are merged into one cron line.
func buildCronBlock(m Meeting, makPath string) string {
	// group days by time, preserving schedule order
	timeTodays := map[string][]int{}
	var timeOrder []string
	for _, s := range m.Schedule {
		if _, ok := timeTodays[s.Time]; !ok {
			timeOrder = append(timeOrder, s.Time)
		}
		timeTodays[s.Time] = append(timeTodays[s.Time], s.Day)
	}

	var cronLines []string
	for _, t := range timeOrder {
		days := timeTodays[t]
		sort.Ints(days)
		parts := strings.Split(t, ":")
		h, _ := strconv.Atoi(parts[0])
		mn, _ := strconv.Atoi(parts[1])

		// our model: 1=Mon…6=Sat, 7=Sun; cron: 0=Sun, 1=Mon…6=Sat
		cronDays := make([]string, len(days))
		for i, d := range days {
			if d == 7 {
				cronDays[i] = "0"
			} else {
				cronDays[i] = strconv.Itoa(d)
			}
		}
		line := fmt.Sprintf("%d %d * * %s %s meet open %q",
			mn, h, strings.Join(cronDays, ","), makPath, m.Alias)
		cronLines = append(cronLines, line)
	}

	var sb strings.Builder
	sb.WriteString(cronStartTag(m.Alias) + "\n")
	for _, l := range cronLines {
		sb.WriteString(l + "\n")
	}
	sb.WriteString(cronEndTag(m.Alias) + "\n")
	return sb.String()
}

func makExecutable() string {
	if path, err := exec.LookPath("mak"); err == nil {
		return path
	}
	return "mak"
}

// SyncCronJob creates or replaces the cron entries for a meeting.
// Pass removeAlias = old alias when the meeting was renamed so the old block
// is cleaned up; for a plain add/update pass the same as m.Alias.
func SyncCronJob(removeAlias string, m Meeting) error {
	crontab, err := readCrontab()
	if err != nil {
		return fmt.Errorf("reading crontab: %w", err)
	}
	crontab = stripCronBlock(crontab, removeAlias)
	crontab = strings.TrimRight(crontab, "\n") + "\n"
	crontab += buildCronBlock(m, makExecutable())
	if err := writeCrontab(crontab); err != nil {
		return fmt.Errorf("writing crontab: %w", err)
	}
	return nil
}

// RemoveCronJob removes the cron entries for the given alias.
func RemoveCronJob(alias string) error {
	crontab, err := readCrontab()
	if err != nil {
		return fmt.Errorf("reading crontab: %w", err)
	}
	crontab = stripCronBlock(crontab, alias)
	if err := writeCrontab(crontab); err != nil {
		return fmt.Errorf("writing crontab: %w", err)
	}
	return nil
}
