package prefills

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestStoreRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const password = "correct horse battery staple"

	if err := InitStore(password); err != nil {
		t.Fatal(err)
	}
	projects := []Project{{
		Name: "Example",
		Domains: []Domain{{
			Label:    "Local",
			URL:      "http://localhost:3000/login",
			Email:    "person@example.com",
			Password: "secret",
		}},
	}}
	if err := Save(password, projects); err != nil {
		t.Fatal(err)
	}

	got, err := Load(password)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Domains[0].Password != "secret" {
		t.Fatalf("unexpected round trip result: %#v", got)
	}
	if _, err := Load("wrong password"); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("Load() error = %v, want ErrWrongPassword", err)
	}

	path, err := storePath()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if gotMode := info.Mode().Perm(); gotMode != 0o600 {
		t.Fatalf("store mode = %o, want 600", gotMode)
	}
}

func TestSummaryDoesNotRevealPassword(t *testing.T) {
	project := Project{Name: "Example", Domains: []Domain{{Password: "super-secret"}}}
	if summary := project.Summary(); summary == "" || strings.Contains(summary, "super-secret") {
		t.Fatalf("Summary() exposed password: %q", summary)
	}
}
