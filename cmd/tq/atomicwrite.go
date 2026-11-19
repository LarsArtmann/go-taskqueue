package main

import (
	"fmt"
	"os"
)

// writeFileAtomic writes data to path through a temp file in the same
// directory followed by a rename, so a crash mid-write never leaves a torn
// final file. Used for config/service files and the verdict result file.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"

	if err := os.WriteFile(tmp, data, perm); err != nil {
		return fmt.Errorf("write temp %s: %w", tmp, err)
	}

	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)

		return fmt.Errorf("rename %s -> %s: %w", tmp, path, err)
	}

	return nil
}
