package meetings

import (
	"strings"
	"testing"
)

func TestBuildCronBlockQuotesArguments(t *testing.T) {
	meeting := Meeting{
		Alias:    "weekly $(touch /tmp/nope) 'sync'",
		Schedule: []DaySchedule{{Day: 1, Time: "09:30"}},
	}
	makPath := "/Applications/Mak Tools/mak"

	block := buildCronBlock(meeting, makPath)
	if !strings.Contains(block, shellQuote(makPath)+" meet open "+shellQuote(meeting.Alias)) {
		t.Fatalf("cron command arguments are not safely quoted: %q", block)
	}
	if !strings.Contains(block, "30 9 * * 1") {
		t.Fatalf("unexpected cron schedule: %q", block)
	}
}

func TestStripCronBlockSupportsCurrentAndLegacyMarkers(t *testing.T) {
	meeting := Meeting{Alias: "Weekly sync"}
	current := cronStartTag(meeting.Alias) + "\ncommand\n" + cronEndTag(meeting.Alias) + "\n"
	legacy := legacyCronStartTag(meeting.Alias) + "\ncommand\n" + legacyCronEndTag(meeting.Alias) + "\n"

	for _, input := range []string{current, legacy} {
		got := stripCronBlock("before\n"+input+"after\n", meeting.Alias)
		if strings.Contains(got, "command") || !strings.Contains(got, "before") || !strings.Contains(got, "after") {
			t.Fatalf("stripCronBlock() returned %q", got)
		}
	}
}

func TestStripAllCronBlocks(t *testing.T) {
	first := buildCronBlock(Meeting{Alias: "One"}, "/usr/local/bin/mak")
	second := buildCronBlock(Meeting{Alias: "Two"}, "/usr/local/bin/mak")
	got := stripAllCronBlocks("MAILTO=user@example.com\n" + first + second + "0 0 * * * backup\n")
	if strings.Contains(got, cronMarker) || !strings.Contains(got, "MAILTO") || !strings.Contains(got, "backup") {
		t.Fatalf("stripAllCronBlocks() returned %q", got)
	}
}
