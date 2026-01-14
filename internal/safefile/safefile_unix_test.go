//go:build !windows

package safefile

import (
	"errors"
	"path/filepath"
	"syscall"
	"testing"
)

func TestOpenRegular_RejectsFIFO(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "fifo")

	if err := syscall.Mkfifo(fifo, 0644); err != nil {
		t.Fatal(err)
	}

	_, _, err := OpenRegular(fifo)
	if err == nil {
		t.Error("OpenRegular() expected error for FIFO")
	}
	if !errors.Is(err, ErrNotRegularFile) {
		t.Errorf("OpenRegular() error = %v, want ErrNotRegularFile", err)
	}
}
