package gate

import (
	"os"
	"path/filepath"
	"testing"
)

// writeSparse creates a file reporting size via stat without consuming disk.
func writeSparse(t *testing.T, path string, size int64) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if size > 0 {
		if err := f.Truncate(size); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDirUnzippedSizeSumsFiles(t *testing.T) {
	dir := t.TempDir()
	writeSparse(t, filepath.Join(dir, "a", "one.bin"), 1000)
	writeSparse(t, filepath.Join(dir, "a", "sub", "two.bin"), 2500)
	if got := DirUnzippedSize(filepath.Join(dir, "a")); got != 3500 {
		t.Errorf("got %d, want 3500", got)
	}
}

func TestDirUnzippedSizeHandlesSymlinks(t *testing.T) {
	// Symlinks are counted by their own entry size, never followed. A link to
	// a file must NOT re-count the target (no double count), a dangling link
	// must not crash, and a self-referential dir link must not loop the walk.
	root := filepath.Join(t.TempDir(), "a")
	writeSparse(t, filepath.Join(root, "real.bin"), 4096)
	fileLink := filepath.Join(root, "link_to_real")
	if err := os.Symlink(filepath.Join(root, "real.bin"), fileLink); err != nil {
		t.Skip("symlinks unsupported on this platform")
	}
	dangling := filepath.Join(root, "dangling")
	if err := os.Symlink(filepath.Join(root, "nope.bin"), dangling); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(root, filepath.Join(root, "loop")); err != nil {
		t.Fatal(err)
	}

	linkSize := func(p string) int64 {
		fi, err := os.Lstat(p)
		if err != nil {
			t.Fatal(err)
		}
		return fi.Size()
	}
	// loop is a symlink-to-dir: WalkDir sees it as a non-dir entry → counted
	// by its own lstat size, not traversed.
	want := 4096 + linkSize(fileLink) + linkSize(dangling) + linkSize(filepath.Join(root, "loop"))
	if got := DirUnzippedSize(root); got != want {
		t.Errorf("got %d, want %d", got, want)
	}
	if got := DirUnzippedSize(root); got >= 2*4096 {
		t.Errorf("real file bytes double-counted: got %d", got)
	}
}
