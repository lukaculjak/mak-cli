package devenv

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionedPythonFallbackUsesNewestAndPreservesUserRuntime(t *testing.T) {
	prefix := t.TempDir()
	for _, version := range []string{"3.9", "3.13", "3.14"} {
		bin := filepath.Join(prefix, "opt", "python@"+version, "bin")
		if err := os.MkdirAll(bin, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(bin, "python3"), []byte("#!/bin/sh\n"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	want := filepath.Join(prefix, "opt/python@3.14/bin/python3")
	if got := executable(brewFallbackPath(prefix, "/usr/bin:/bin"), "python3"); got != want {
		t.Fatalf("selected %q instead of newest installed Python %q", got, want)
	}
	project := filepath.Join(t.TempDir(), "venv", "bin")
	if err := os.MkdirAll(project, 0700); err != nil {
		t.Fatal(err)
	}
	python := filepath.Join(project, "python3")
	if err := os.WriteFile(python, []byte("#!/bin/sh\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if got := executable(brewFallbackPath(prefix, project+":/usr/bin:/bin"), "python3"); got != python {
		t.Fatalf("replaced project Python with %q", got)
	}
}

func TestPythonPreflightFailureKeepsConfiguration(t *testing.T) {
	i, f := testInstaller(t)
	paths := seedEnvironment(t, i)
	var out bytes.Buffer
	i.out = &out
	f.fail = "python3 -B"
	err := i.setup(context.Background())
	if err == nil || !strings.Contains(err.Error(), "Python runtime cannot install Mason tools") {
		t.Fatalf("missing Python preflight error: %v", err)
	}
	assertOriginals(t, paths)
	for _, call := range f.calls {
		if strings.HasPrefix(call, "nvim ") || strings.HasPrefix(call, "git clone ") {
			t.Fatal("started rebuilding editor before checking Python:", call)
		}
	}
	if strings.Contains(out.String(), "Replacing ") {
		t.Fatal("replaced configuration before Python preflight:", out.String())
	}
}
