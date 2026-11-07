package maker

import (
	"os"
	"path/filepath"
)

// WriteAtomic writes data to a temp file beside path and renames it over path,
// so a failure never leaves a half-written exe. os.CreateTemp makes the temp
// file 0600 (the file is as secret as the password; Windows ignores the bits),
// and the rename gives that mode to the result even when path already existed.
func WriteAtomic(path string, data []byte) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".fleet-rdp-*.tmp")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()
	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
