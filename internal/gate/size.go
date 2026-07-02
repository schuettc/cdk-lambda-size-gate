package gate

import (
	"io/fs"
	"path/filepath"
)

// DirUnzippedSize returns the total unzipped size in bytes of every file
// under root — mirroring how AWS accounts for the unzipped deployment
// package. WalkDir never follows symlinks: a symlink (to file or dir) is a
// single non-dir entry counted at its own lstat size, so a self-referential
// link can't loop or double-count its target. Unreadable or vanished entries
// contribute nothing rather than failing the gate.
func DirUnzippedSize(root string) int64 {
	var total int64
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil // skip unreadable entries and directories themselves
		}
		info, err := d.Info() // lstat semantics for symlink entries
		if err != nil {
			return nil // dangling symlink or file vanished mid-walk
		}
		total += info.Size()
		return nil
	})
	return total
}
