package ui

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestProgressRedirectedOutputIsPlain(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("COLORTERM", "truecolor")
	var out bytes.Buffer
	p := NewProgress(&out)
	p.Stage(1, 7, "Checking prerequisites...")
	fmt.Fprint(p, "Checking Homebrew\n")
	p.Stage(2, 7, "Installing tools...")
	p.Close()
	p.Close()
	fmt.Fprintln(p, "Ready")
	want := "[step] Step 1/7: Checking prerequisites...\nChecking Homebrew\n[step] Step 2/7: Installing tools...\nReady\n"
	if out.String() != want {
		t.Fatalf("unexpected redirected output: %q", out.String())
	}
}

func TestProgressPreservesLogChunksAndPrompts(t *testing.T) {
	var out bytes.Buffer
	p := &Progress{out: &out, palette: palette{enabled: true, orange: "1;38;2;255;149;0"}, lineStart: true}
	p.Stage(4, 7, "Installing plugins...")
	if !strings.Contains(out.String(), "\x1b[1;38;2;255;149;0m⠋ Step 4/7") {
		t.Fatal("missing orange spinner and stage counter")
	}
	fmt.Fprint(p, "Downloading")
	before := out.Len()
	p.redraw()
	if out.Len() != before {
		t.Fatal("spinner overwrote an incomplete log line")
	}
	fmt.Fprint(p, " plugin\n")
	if !strings.Contains(out.String(), "Downloading plugin\n") || !p.visible {
		t.Fatal("log chunks were interrupted or spinner did not resume")
	}
	p.Pause()
	fmt.Fprint(p, "Password: ")
	before = out.Len()
	p.redraw()
	if out.Len() != before {
		t.Fatal("spinner interrupted the interactive prompt")
	}
	fmt.Fprint(p, "\n")
	if p.visible {
		t.Fatal("spinner resumed during an interactive subprocess")
	}
	p.Resume()
	if !p.visible {
		t.Fatal("spinner did not resume after the interactive subprocess")
	}
	fmt.Fprint(p, "Installation failed")
	p.Close()
	if !strings.HasSuffix(out.String(), "Installation failed\n") {
		t.Fatal("recovery output would be joined to the last log line")
	}
	before = out.Len()
	p.redraw()
	if out.Len() != before || p.visible {
		t.Fatal("spinner continued after completion")
	}
}

func TestProgressConcurrentLogsRemainIntact(t *testing.T) {
	var out bytes.Buffer
	p := &Progress{out: &out, palette: palette{enabled: true, orange: "1;33"}, lineStart: true}
	p.Stage(5, 7, "Installing language servers...")
	var workers sync.WaitGroup
	for worker := 0; worker < 3; worker++ {
		workers.Add(1)
		go func(worker int) {
			defer workers.Done()
			for line := 0; line < 30; line++ {
				if worker == 2 {
					p.redraw()
				} else {
					fmt.Fprintf(p, "worker %d line %d\n", worker, line)
				}
			}
		}(worker)
	}
	workers.Wait()
	p.Close()
	for worker := 0; worker < 2; worker++ {
		for line := 0; line < 30; line++ {
			if !strings.Contains(out.String(), fmt.Sprintf("worker %d line %d\n", worker, line)) {
				t.Fatalf("log line was corrupted: worker %d line %d", worker, line)
			}
		}
	}
}
