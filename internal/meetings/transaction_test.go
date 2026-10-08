package meetings

import (
	"errors"
	"strings"
	"testing"
)

func TestSaveAndSyncFailuresKeepPreviousState(t *testing.T) {
	for _, failure := range []string{"read", "cron", "save", "restore"} {
		t.Run(failure, func(t *testing.T) {
			original := "MAILTO=user@example.com\n" + buildCronBlock(Meeting{Alias: "Old", Schedule: []DaySchedule{{Day: 1, Time: "09:00"}}}, "/mak") + "0 0 * * * backup\n"
			cron := original
			saves, writes := 0, 0
			sentinel := errors.New(failure)
			read := func() (string, error) {
				if failure == "read" {
					return "", sentinel
				}
				return cron, nil
			}
			write := func(next string) error {
				writes++
				if failure == "cron" || (failure == "restore" && writes == 2) {
					return sentinel
				}
				cron = next
				return nil
			}
			save := func([]Meeting) error {
				saves++
				if failure == "save" || failure == "restore" {
					return sentinel
				}
				return nil
			}
			err := saveAndSync([]Meeting{{Alias: "New", Schedule: []DaySchedule{{Day: 2, Time: "10:00"}}}}, read, write, save)
			if !errors.Is(err, sentinel) {
				t.Fatal(err)
			}
			if failure != "restore" && cron != original {
				t.Fatal("previous schedule changed")
			}
			if (failure == "read" || failure == "cron") && saves != 0 {
				t.Fatal("saved meetings despite cron failure")
			}
			if failure == "restore" && !strings.Contains(err.Error(), "retry the meeting change") {
				t.Fatal("missing recovery instructions")
			}
		})
	}
}

func TestSaveAndSyncReconcilesSchedulesAndPreservesOtherJobs(t *testing.T) {
	old := buildCronBlock(Meeting{Alias: "Old"}, "/mak")
	cron := "MAILTO=user@example.com\n" + old + "0 0 * * * backup\n"
	saved := false
	list := []Meeting{{Alias: "New", Schedule: []DaySchedule{{Day: 2, Time: "10:00"}}}}
	err := saveAndSync(list, func() (string, error) { return cron, nil }, func(s string) error { cron = s; return nil }, func([]Meeting) error { saved = true; return nil })
	if err != nil || !saved || strings.Contains(cron, cronStartTag("Old")) || !strings.Contains(cron, cronStartTag("New")) || !strings.Contains(cron, "backup") || !strings.Contains(cron, "MAILTO") {
		t.Fatalf("failed reconciliation: %v %s", err, cron)
	}
	if err := saveAndSync(nil, func() (string, error) { return cron, nil }, func(s string) error { cron = s; return nil }, func([]Meeting) error { return nil }); err != nil || strings.Contains(cron, cronMarker) {
		t.Fatalf("delete left schedules: %v %s", err, cron)
	}
}
