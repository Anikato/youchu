package photo

import (
	"fmt"
	"os"
	"path/filepath"
)

func originalPath(dataDir string, id int64) string {
	return filepath.Join(dataDir, "originals", fmt.Sprintf("%d.jpg", id))
}

func thumbPath(dataDir string, id int64) string {
	return filepath.Join(dataDir, "thumbnails", fmt.Sprintf("%d.jpg", id))
}

func Place(dataDir string, id int64, original, thumb []byte) (err error) {
	tmpDir := filepath.Join(dataDir, "tmp")
	var tmps []string
	var finals []string
	defer func() {
		if err == nil {
			return
		}
		for _, p := range tmps {
			os.Remove(p)
		}
		for _, p := range finals {
			os.Remove(p)
		}
	}()

	origTmp, err := writeTemp(tmpDir, original)
	if err != nil {
		return err
	}
	tmps = append(tmps, origTmp)

	thumbTmp, err := writeTemp(tmpDir, thumb)
	if err != nil {
		return err
	}
	tmps = append(tmps, thumbTmp)

	origFinal := originalPath(dataDir, id)
	if err = os.Rename(origTmp, origFinal); err != nil {
		return err
	}
	finals = append(finals, origFinal)

	thumbFinal := thumbPath(dataDir, id)
	if err = os.Rename(thumbTmp, thumbFinal); err != nil {
		return err
	}
	finals = append(finals, thumbFinal)
	return nil
}

func writeTemp(dir string, data []byte) (string, error) {
	f, err := os.CreateTemp(dir, "photo-*.jpg")
	if err != nil {
		return "", err
	}
	name := f.Name()
	_, werr := f.Write(data)
	cerr := f.Close()
	if werr != nil {
		os.Remove(name)
		return "", werr
	}
	if cerr != nil {
		os.Remove(name)
		return "", cerr
	}
	return name, nil
}

func Remove(dataDir string, id int64) error {
	err1 := os.Remove(originalPath(dataDir, id))
	err2 := os.Remove(thumbPath(dataDir, id))
	if err1 != nil && !os.IsNotExist(err1) {
		return err1
	}
	if err2 != nil && !os.IsNotExist(err2) {
		return err2
	}
	return nil
}
