package photo

import (
	"os"
	"path/filepath"
)

func EnsureDirs(dataDir string) error {
	for _, name := range []string{"originals", "thumbnails", "tmp"} {
		if err := os.MkdirAll(filepath.Join(dataDir, name), 0o755); err != nil {
			return err
		}
	}
	return nil
}
