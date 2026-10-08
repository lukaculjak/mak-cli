package updater

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNotificationCachesSuccessAndFailure(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(map[bool]string{false: "online", true: "offline"}[failure], func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "cache", "check.json")
			now := time.Now()
			calls := 0
			fetch := func() (string, error) {
				calls++
				if failure {
					return "", errors.New("offline")
				}
				return "1.2.3", nil
			}
			want := "1.2.3"
			if failure {
				want = ""
			}
			if got := cachedLatestVersion(path, now, fetch); got != want {
				t.Fatal(got)
			}
			if got := cachedLatestVersion(path, now.Add(time.Hour), fetch); got != want || calls != 1 {
				t.Fatalf("got=%s requests=%d", got, calls)
			}
			_ = cachedLatestVersion(path, now.Add(25*time.Hour), fetch)
			if calls != 2 {
				t.Fatal("expired cache wasn't refreshed")
			}
		})
	}
}

func TestNotificationPreservesLastKnownReleaseOffline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	now := time.Now()
	cachedLatestVersion(path, now, func() (string, error) { return "1.2.3", nil })
	got := cachedLatestVersion(path, now.Add(25*time.Hour), func() (string, error) { return "", errors.New("offline") })
	if got != "1.2.3" {
		t.Fatal(got)
	}
}

func TestNotificationRepairsInvalidCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	if err := os.WriteFile(path, []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := cachedLatestVersion(path, time.Now(), func() (string, error) { return "1.2.3", nil }); got != "1.2.3" {
		t.Fatal(got)
	}
}

type notificationTransport func(*http.Request) (*http.Response, error)

func (f notificationTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestNotificationNetworkWaitIsBounded(t *testing.T) {
	client := *notificationClient
	client.Transport = notificationTransport(func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() })
	start := time.Now()
	_, err := latestVersion(&client)
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > time.Second {
		t.Fatalf("notification did not return promptly: %v %s", err, time.Since(start))
	}
}
