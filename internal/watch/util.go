package watch

import "os"

func fileInfo(p string) (os.FileInfo, error) {
	return os.Stat(p)
}
