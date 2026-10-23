package devenv

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const incompatibleBrewError = "brew: exit status 1\nError: unknown or unsupported macOS version: :build\n"

func TestInstallRefreshesIncompatibleHomebrewBeforeSnapshots(t *testing.T) {
	for _, action := range []string{"setup", "node", "ghostty"} {
		t.Run(action, func(t *testing.T) {
			for _, failure := range []string{"", "update", "persistent", "unrelated", "cancel"} {
				t.Run(failure, func(t *testing.T) {
					i, f := testInstaller(t)
					originals := seedEnvironment(t, i)
					f.aliases = map[string]string{"python": "python@3.13"}
					f.formulae["python@3.13"] = true
					var out bytes.Buffer
					i.out = &out
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					updates, metadataReads := 0, 0
					metadataReady := false
					i.run = func(ctx context.Context, env []string, program string, args ...string) (string, error) {
						if filepath.Base(program) == "brew" {
							if envValue(env, "HOMEBREW_NO_AUTO_UPDATE") != "1" {
								t.Fatal("automatic updates must stay disabled during tracked installation")
							}
							switch args[0] {
							case "info":
								metadataReads++
								if failure == "cancel" {
									cancel()
									return "", ctx.Err()
								}
								if failure == "unrelated" {
									return "", errors.New("network unavailable")
								}
								if updates == 0 || failure == "persistent" {
									return "", errors.New(incompatibleBrewError)
								}
								metadataReady = true
							case "update":
								updates++
								if !reflect.DeepEqual(args, []string{"update"}) {
									t.Fatalf("expected Homebrew update only: %v", args)
								}
								if envValue(env, "HOMEBREW_NO_INSTALL_UPGRADE") != "1" || envValue(env, "HOMEBREW_NO_INSTALL_CLEANUP") != "1" {
									t.Fatal("package preservation settings are missing")
								}
								if failure == "update" {
									return "", errors.New("update failed")
								}
								// Simulate a migration during Homebrew's update. It must
								// be included in the pre-install inventory, not claimed by mak.
								delete(f.formulae, "python@3.13")
								f.formulae["python@3.14"] = true
								f.aliases["python"] = "python@3.14"
							case "list":
								if !metadataReady {
									t.Fatal("package snapshot taken before metadata was ready")
								}
							}
						}
						return f.run(ctx, env, program, args...)
					}
					var err error
					if action == "setup" {
						err = i.setup(ctx)
					} else {
						err = i.installPackage(ctx, packageForTest(t, action))
					}
					wantUpdates, wantReads := 1, 2
					if failure == "" {
						// Saving the verified installation resolves package names again.
						wantReads = 3
					} else if failure == "unrelated" || failure == "cancel" {
						wantUpdates, wantReads = 0, 1
					} else if failure == "update" {
						wantReads = 1
					}
					if updates != wantUpdates || metadataReads != wantReads {
						t.Fatalf("updates=%d reads=%d; want %d/%d", updates, metadataReads, wantUpdates, wantReads)
					}
					if failure == "" {
						if err != nil {
							t.Fatal(err)
						}
						record := mustRecord(t, i)
						if strings.Contains(strings.Join(record.Formulae, " "), "python@") {
							t.Fatalf("claimed a package migrated by Homebrew: %v", record.Formulae)
						}
					} else {
						if err == nil {
							t.Fatal("expected preflight failure")
						}
						if failure == "cancel" && !errors.Is(err, context.Canceled) {
							t.Fatalf("cancellation lost: %v", err)
						}
						if failure == "persistent" && !strings.Contains(err.Error(), "brew config and sw_vers") {
							t.Fatalf("missing recovery guidance: %v", err)
						}
						assertOriginals(t, originals)
						if i.journal != nil {
							t.Fatal("metadata failure began a setup journal")
						}
						for _, call := range f.calls {
							if strings.HasPrefix(call, "brew install ") || strings.HasPrefix(call, "brew uninstall ") {
								t.Fatalf("metadata failure changed packages: %s", call)
							}
						}
					}
				})
			}
		})
	}
}

func TestReadOnlyCommandsReportIncompatibleHomebrewWithoutUpdating(t *testing.T) {
	for _, action := range []string{"doctor", "list"} {
		t.Run(action, func(t *testing.T) {
			i, out, _ := doctorFixture(t)
			before := snapshotFiles(t, i.home)
			run := i.run
			i.run = func(ctx context.Context, env []string, program string, args ...string) (string, error) {
				if filepath.Base(program) == "brew" && args[0] == "info" {
					return "", errors.New(incompatibleBrewError)
				}
				return run(ctx, env, program, args...)
			}
			var err error
			if action == "doctor" {
				err = i.doctor(context.Background())
				if !errors.Is(err, ErrUnhealthy) || !strings.Contains(out.String(), "Homebrew package metadata") || !strings.Contains(out.String(), "brew update") {
					t.Fatalf("missing metadata diagnostic: %v\n%s", err, out)
				}
			} else {
				err = i.listPackages(context.Background())
				if err == nil || !strings.Contains(err.Error(), "reading Homebrew package metadata") {
					t.Fatalf("missing metadata error: %v", err)
				}
			}
			if !reflect.DeepEqual(before, snapshotFiles(t, i.home)) {
				t.Fatal("read-only command changed installation or backups")
			}
			if _, err := os.Stat(i.journalPath()); !os.IsNotExist(err) {
				t.Fatalf("read-only command left a journal: %v", err)
			}
		})
	}
}
