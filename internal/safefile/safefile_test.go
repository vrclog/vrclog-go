package safefile

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestOpenRegular_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	f, info, err := OpenRegular(path)
	if err != nil {
		t.Fatalf("OpenRegular() error = %v, want nil", err)
	}
	defer f.Close()

	if !info.Mode().IsRegular() {
		t.Error("expected regular file")
	}

	// Verify we can read from the file
	buf := make([]byte, 12)
	n, err := f.Read(buf)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if string(buf[:n]) != "test content" {
		t.Errorf("Read() = %q, want %q", string(buf[:n]), "test content")
	}
}

func TestOpenRegular_FileNotExist(t *testing.T) {
	_, _, err := OpenRegular("/nonexistent/path/file.txt")
	if err == nil {
		t.Error("OpenRegular() expected error for nonexistent file")
	}
	if !os.IsNotExist(err) {
		t.Errorf("OpenRegular() error = %v, want os.IsNotExist", err)
	}
}

func TestOpenRegular_RejectsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink test requires Unix")
	}

	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	link := filepath.Join(dir, "link.txt")

	if err := os.WriteFile(target, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	_, _, err := OpenRegular(link)
	if err == nil {
		t.Error("OpenRegular() expected error for symlink")
	}
	if !errors.Is(err, ErrNotRegularFile) {
		t.Errorf("OpenRegular() error = %v, want ErrNotRegularFile", err)
	}
}

func TestOpenRegular_RejectsDirectory(t *testing.T) {
	dir := t.TempDir()

	_, _, err := OpenRegular(dir)
	if err == nil {
		t.Error("OpenRegular() expected error for directory")
	}
	if !errors.Is(err, ErrNotRegularFile) {
		t.Errorf("OpenRegular() error = %v, want ErrNotRegularFile", err)
	}
}

func TestOpenRegular_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	f, info, err := OpenRegular(path)
	if err != nil {
		t.Fatalf("OpenRegular() error = %v, want nil", err)
	}
	defer f.Close()

	if info.Size() != 0 {
		t.Errorf("Size() = %d, want 0", info.Size())
	}
}
