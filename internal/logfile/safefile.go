package logfile

import (
	"errors"
	"os"
)

var ErrNotRegularFile = errors.New("not a regular file")

var ErrFileChangedDuringOpen = errors.New("file changed during open")

func OpenRegular(path string) (*os.File, os.FileInfo, error) {
	lstatInfo, err := os.Lstat(path)
	if err != nil {
		return nil, nil, err
	}

	if !lstatInfo.Mode().IsRegular() {
		return nil, nil, ErrNotRegularFile
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}

	fdInfo, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, err
	}

	if !fdInfo.Mode().IsRegular() {
		f.Close()
		return nil, nil, ErrNotRegularFile
	}

	if !os.SameFile(lstatInfo, fdInfo) {
		f.Close()
		return nil, nil, ErrFileChangedDuringOpen
	}

	return f, fdInfo, nil
}
