package productivity

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestDownloadWaitsForLocalContents(t *testing.T) {
	checks, requests := 0, 0
	err := downloadAndWait(context.Background(), "note.norg", func() (bool, error) {
		checks++
		return checks < 3, nil
	}, func(context.Context) error {
		requests++
		return nil
	})
	if err != nil || requests != 1 || checks != 3 {
		t.Fatalf("err=%v, requests=%d, checks=%d", err, requests, checks)
	}
}

func TestDownloadSkipsLocalFilesAndPropagatesErrors(t *testing.T) {
	for _, problem := range []string{"already local", "stat failed", "download failed", "poll failed"} {
		t.Run(problem, func(t *testing.T) {
			checks, requests := 0, 0
			failure := errors.New(problem)
			err := downloadAndWait(context.Background(), "note.norg", func() (bool, error) {
				checks++
				if problem == "stat failed" || problem == "poll failed" && checks > 1 {
					return false, failure
				}
				return problem != "already local", nil
			}, func(context.Context) error {
				requests++
				if problem == "download failed" {
					return failure
				}
				return nil
			})
			if problem == "already local" {
				if err != nil || requests != 0 {
					t.Fatalf("local file triggered a download: %v", err)
				}
			} else if !errors.Is(err, failure) {
				t.Fatalf("lost failure: %v", err)
			}
			if problem == "stat failed" && requests != 0 {
				t.Fatal("requested download after stat failure")
			}
		})
	}
}

func TestDownloadHonorsCancellationAndTimeout(t *testing.T) {
	for _, stage := range []string{"before request", "during request", "while waiting"} {
		t.Run(stage, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			if stage == "before request" {
				cancel()
			}
			requests := 0
			err := downloadAndWait(ctx, "note.norg", func() (bool, error) {
				return true, nil
			}, func(ctx context.Context) error {
				requests++
				deadline, ok := ctx.Deadline()
				if !ok || time.Until(deadline) > 2*time.Minute {
					t.Fatal("download request has no bounded deadline")
				}
				if stage == "during request" {
					cancel()
					return os.ErrInvalid
				}
				return nil
			})
			want := context.Canceled
			if stage == "while waiting" {
				want = context.DeadlineExceeded
				if !strings.Contains(err.Error(), "Keep Downloaded") {
					t.Fatal("timeout lacks recovery guidance:", err)
				}
			}
			if !errors.Is(err, want) {
				t.Fatalf("want %v, got %v", want, err)
			}
			if stage == "before request" && requests != 0 {
				t.Fatal("started a download after cancellation")
			}
		})
	}
}
