package repo

import (
	"os"
	"path/filepath"
)

func clearRuntimeIndex(dataDir string) error {
	path := filepath.Join(dataDir, "index.db")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
