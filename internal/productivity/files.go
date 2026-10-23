package productivity

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type fileVersion struct {
	path     string
	modified time.Time
	size     int64
	hash     [sha256.Size]byte
}

func readVersion(ctx context.Context, path string) (fileVersion, error) {
	if err := ctx.Err(); err != nil {
		return fileVersion{}, err
	}
	st, err := os.Lstat(path)
	if err != nil {
		return fileVersion{}, err
	}
	if !st.Mode().IsRegular() {
		return fileVersion{}, fmt.Errorf("expected a regular file at %s; symlinks and special files cannot be synced", path)
	}
	if err := checkDownloaded(ctx, path); err != nil {
		return fileVersion{}, err
	}
	f, err := os.Open(path)
	if err != nil {
		return fileVersion{}, err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return fileVersion{}, err
	}
	after, err := f.Stat()
	if err != nil {
		return fileVersion{}, err
	}
	if !os.SameFile(st, after) || st.Size() != after.Size() || !st.ModTime().Equal(after.ModTime()) {
		return fileVersion{}, fmt.Errorf("%s changed while being read; save your files and retry", path)
	}
	v := fileVersion{path: path, modified: st.ModTime(), size: st.Size()}
	copy(v.hash[:], hash.Sum(nil))
	return v, ctx.Err()
}

func scanTree(ctx context.Context, root string) (map[string]fileVersion, error) {
	files := map[string]fileVersion{}
	st, err := os.Lstat(root)
	if os.IsNotExist(err) {
		return files, nil
	}
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("expected a directory at %s", root)
	}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := checkDownloaded(ctx, path); err != nil {
			return err
		}
		if strings.HasPrefix(entry.Name(), ".") && strings.HasSuffix(entry.Name(), ".icloud") {
			return fmt.Errorf("%s is an iCloud placeholder; choose Keep Downloaded on .MAK in Finder, wait for downloads, then retry", path)
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Name() == ".DS_Store" {
			return nil
		}
		v, err := readVersion(ctx, path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[rel] = v
		return nil
	})
	return files, err
}

// Refuse path collisions before any note is replaced. Inputs are canonical paths,
// so a symlink below a synced root is never followed during directory creation.
func checkParents(path string) error {
	for dir := filepath.Dir(path); ; dir = filepath.Dir(dir) {
		st, err := os.Lstat(dir)
		if err == nil && !st.IsDir() {
			return fmt.Errorf("expected a directory at %s; refusing a symlink or file in the sync path", dir)
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if filepath.Dir(dir) == dir {
			return nil
		}
	}
}

func checkDestination(ctx context.Context, path string, expected *fileVersion) error {
	if err := checkParents(path); err != nil {
		return err
	}
	current, err := readVersion(ctx, path)
	if expected == nil && os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if expected == nil || current.hash != expected.hash || !current.modified.Equal(expected.modified) {
		return fmt.Errorf("%s changed during the transfer; save your files and retry", path)
	}
	return nil
}

// Stage a complete verified copy beside its destination, then replace atomically.
// The old destination must still match the version used to plan this transfer.
func copyVersion(ctx context.Context, source fileVersion, target string, expected *fileVersion) error {
	if err := checkDestination(ctx, target, expected); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return err
	}
	in, err := os.Open(source.path)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp, err := os.CreateTemp(filepath.Dir(target), ".mak-copy-")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(tmp, hash), in)
	if err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	var digest [sha256.Size]byte
	copy(digest[:], hash.Sum(nil))
	if size != source.size || digest != source.hash {
		return fmt.Errorf("%s changed during the transfer; save your files and retry", source.path)
	}
	if err := os.Chtimes(tmp.Name(), source.modified, source.modified); err != nil {
		return err
	}
	if err := checkDestination(ctx, target, expected); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), target)
}

// Resolve aliases, including existing dotfile symlinks, before overlap checks.
func canonicalPath(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	ancestor := path
	for {
		resolved, err := filepath.EvalSymlinks(ancestor)
		if err == nil {
			rel, err := filepath.Rel(ancestor, path)
			if err != nil {
				return "", err
			}
			return filepath.Join(resolved, rel), nil
		}
		if !os.IsNotExist(err) || filepath.Dir(ancestor) == ancestor {
			return "", err
		}
		ancestor = filepath.Dir(ancestor)
	}
}

func overlaps(a, b string) bool {
	return a == b || strings.HasPrefix(a, b+string(os.PathSeparator)) || strings.HasPrefix(b, a+string(os.PathSeparator))
}
