package devenv

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestBrewPermissionDirectories(t *testing.T) {
	const heading = "Warning: The following directories are not writable by your user:\n"
	for _, tc := range []struct {
		name, prefix, report string
		want                 []string
	}{
		{"Intel report", "/usr/local", "brew: exit status 1\n" + heading + "/usr/local/Homebrew\n/usr/local/share/man/man3\n\nYou should change ownership.\n  sudo chown -R Luka /usr/local/Homebrew", []string{"/usr/local/Homebrew", "/usr/local/share/man/man3"}},
		{"Apple Silicon", "/opt/homebrew", heading + "  /opt/homebrew\n  /opt/homebrew/var/homebrew/locks\n\n", []string{"/opt/homebrew", "/opt/homebrew/var/homebrew/locks"}},
		{"unknown warning", "/usr/local", "brew: permission denied", nil},
		{"shared prefix", "/usr/local", heading + "/usr/local\n\n", nil},
		{"outside prefix", "/usr/local", heading + "/usr/local/Homebrew\n/etc\n\n", nil},
		{"path traversal", "/usr/local", heading + "/usr/local/../../etc\n\n", nil},
		{"similar prefix", "/usr/local", heading + "/usr/local-other\n\n", nil},
		{"escape codes", "/usr/local", heading + "/usr/local/\x1b[31mHomebrew\n\n", nil},
		{"truncated", "/usr/local", heading + "/usr/local/Homebrew", nil},
		{"suggested command in list", "/usr/local", heading + "/usr/local/Homebrew\nsudo chown -R Luka /\n\n", nil},
		{"missing directories", "/usr/local", heading + "/usr/local/Homebrew\n\nWarning: The following directories do not exist:\n/usr/local/Cellar\n\n", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := brewPermissionDirectories(tc.report, tc.prefix); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("directories = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCellarPermissionsCheckedBeforeInstall(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	for _, tc := range []struct {
		name, input                string
		combined, escape, repaired bool
	}{
		{name: "declined", input: "no\n"},
		{name: "approved", input: "yes\n", repaired: true},
		{name: "combined with brew doctor", input: "yes\n", combined: true, repaired: true},
		{name: "symlink outside prefix", input: "yes\n", escape: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			i, f := testInstaller(t)
			originals := seedEnvironment(t, i)
			prefix := filepath.Join(i.home, "brew")
			rack := filepath.Join(prefix, "Cellar", "libunistring")
			if err := os.MkdirAll(rack, 0o755); err != nil {
				t.Fatal(err)
			}
			target := rack
			if tc.escape {
				target = t.TempDir()
				if err := os.Remove(rack); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, rack); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Chmod(target, 0o500); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(target, 0o755) })
			fish := filepath.Join(prefix, "share", "fish")
			if err := os.MkdirAll(fish, 0o755); err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			i.out, i.in = &out, strings.NewReader(tc.input)
			repaired := false
			i.run = func(ctx context.Context, env []string, program string, args ...string) (string, error) {
				switch filepath.Base(program) {
				case "brew":
					if args[0] == "--prefix" {
						return prefix, nil
					}
					if args[0] == "doctor" && tc.combined && !repaired {
						return "", fmt.Errorf("The following directories are not writable by your user:\n%s\n\n", fish)
					}
					if args[0] == "install" && !repaired {
						t.Fatal("installation started before Cellar repair")
					}
				case "sudo":
					want := []string{rack}
					if tc.combined {
						want = append([]string{fish}, want...)
					}
					if !reflect.DeepEqual(args, append([]string{"/usr/sbin/chown", "-R", "-P", strconv.Itoa(os.Getuid())}, want...)) {
						t.Fatalf("unexpected ownership repair: %v", args)
					}
					return "", nil
				case "chmod":
					repaired = true
					return "", os.Chmod(rack, 0o755)
				}
				return f.run(ctx, env, program, args...)
			}
			err := i.setup(context.Background())
			if repaired != tc.repaired || (err == nil) != tc.repaired {
				t.Fatalf("repaired=%v, err=%v\n%s", repaired, err, &out)
			}
			if !tc.repaired {
				assertOriginals(t, originals)
				if i.journal != nil {
					t.Fatal("Cellar permission failure reached installation journal")
				}
			}
			if tc.repaired && !strings.Contains(out.String(), rack) {
				t.Fatalf("repair prompt did not identify the affected rack: %s", &out)
			}
		})
	}
}

func TestCellarPermissionCheckIsReadOnly(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	i, _ := testInstaller(t)
	prefix := filepath.Join(i.home, "brew")
	rack := filepath.Join(prefix, "Cellar", "libunistring")
	if err := os.MkdirAll(rack, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(rack, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(rack, 0o755) })
	i.run = func(ctx context.Context, env []string, program string, args ...string) (string, error) {
		if args[0] == "--prefix" {
			return prefix, nil
		}
		if args[0] != "doctor" {
			t.Fatalf("permission check mutated the system: %s %v", program, args)
		}
		return "", nil
	}
	err := i.checkBrewPermissions(context.Background())
	if err == nil || !strings.Contains(err.Error(), rack) {
		t.Fatalf("missed unwritable existing formula rack: %v", err)
	}
	st, err := os.Stat(rack)
	if err != nil || st.Mode().Perm() != 0o500 {
		t.Fatalf("permission check changed the rack: %v %v", st, err)
	}
}

func TestInstallHomebrewPermissionRepair(t *testing.T) {
	for _, action := range []string{"setup", "node", "ghostty"} {
		t.Run(action, func(t *testing.T) {
			for _, tc := range []struct {
				name, input, failure string
				wantRepair, wantOK   bool
			}{
				{"approve", "yes\n", "", true, true},
				{"decline", "no\n", "", false, false},
				{"Enter declines", "\n", "", false, false},
				{"EOF declines", "", "", false, false},
				{"sudo fails", "yes\n", "sudo", true, false},
				{"chmod fails", "yes\n", "chmod", true, false},
				{"still unwritable", "yes\n", "recheck", true, false},
				{"later setup failure", "yes\n", "nvim", true, false},
				{"symlink escapes prefix", "yes\n", "symlink", false, false},
				{"cancelled", "yes\n", "cancel", false, false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					if tc.failure == "nvim" && action != "setup" {
						t.Skip("Neovim runs only during full setup")
					}
					i, f := testInstaller(t)
					originals := seedEnvironment(t, i)
					prefix := filepath.Join(i.home, "brew")
					dirs := []string{filepath.Join(prefix, "Homebrew"), filepath.Join(prefix, "share/man")}
					for _, dir := range dirs {
						if err := os.MkdirAll(dir, 0o755); err != nil {
							t.Fatal(err)
						}
					}
					if tc.failure == "symlink" {
						if err := os.RemoveAll(dirs[0]); err != nil {
							t.Fatal(err)
						}
						if err := os.Symlink(t.TempDir(), dirs[0]); err != nil {
							t.Fatal(err)
						}
					}
					if tc.failure == "nvim" {
						f.fail = "nvim --headless"
					}
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					i.in = strings.NewReader(tc.input)
					var out bytes.Buffer
					i.out = &out
					var repairs []string
					checks := 0
					repaired := false
					i.run = func(ctx context.Context, env []string, program string, args ...string) (string, error) {
						switch filepath.Base(program) {
						case "brew":
							if args[0] == "--prefix" {
								return prefix + "\n", nil
							}
							if args[0] == "doctor" {
								checks++
								if repaired && tc.failure != "recheck" {
									return "", nil
								}
								if tc.failure == "cancel" {
									cancel()
								}
								return "", fmt.Errorf("brew: exit status 1\nWarning: The following directories are not writable by your user:\n%s\n\nYou should change ownership.", strings.Join(dirs, "\n"))
							}
						case "sudo", "chmod":
							name := filepath.Base(program)
							repairs = append(repairs, name)
							want := append([]string{"u+w"}, dirs...)
							if name == "sudo" {
								want = append([]string{"/usr/sbin/chown", "-R", "-P", strconv.Itoa(os.Getuid())}, dirs...)
							}
							if !reflect.DeepEqual(args, want) {
								t.Fatalf("repair arguments: %v, want %v", args, want)
							}
							if tc.failure == name {
								return "", errors.New("simulated " + name + " failure")
							}
							if name == "chmod" {
								repaired = true
							}
							return "", nil
						}
						return f.run(ctx, env, program, args...)
					}
					var err error
					if action == "setup" {
						err = i.setup(ctx)
					} else {
						err = i.installPackage(ctx, packageForTest(t, action))
					}
					if (err == nil) != tc.wantOK || (len(repairs) != 0) != tc.wantRepair {
						t.Fatalf("err=%v, repairs=%v\n%s", err, repairs, &out)
					}
					if tc.wantRepair && tc.failure != "sudo" && tc.failure != "chmod" && checks != 2 {
						t.Fatalf("repair was not rechecked: %d checks", checks)
					}
					if !tc.wantOK {
						assertOriginals(t, originals)
						for _, call := range f.calls {
							if tc.failure != "nvim" && strings.HasPrefix(call, "brew install ") {
								t.Fatalf("installed before permissions passed: %s", call)
							}
						}
					}
					if tc.wantRepair {
						for _, text := range []string{"[y/N]", "may affect other accounts", "remain after a setup failure", dirs[0], dirs[1]} {
							if !strings.Contains(out.String(), text) {
								t.Fatalf("missing repair information %q: %s", text, &out)
							}
						}
					}
				})
			}
		})
	}
}
