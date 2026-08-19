package logfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSourceID_Determinism(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	id1, err := SourceID(path)
	if err != nil {
		t.Fatalf("SourceID() error = %v", err)
	}

	id2, err := SourceID(path)
	if err != nil {
		t.Fatalf("SourceID() error = %v", err)
	}

	if id1 != id2 {
		t.Errorf("SourceID not deterministic: %q != %q", id1, id2)
	}

	if id1 == "" {
		t.Error("SourceID returned empty string")
	}
}

func TestSourceID_DifferentPaths(t *testing.T) {
	dir := t.TempDir()
	path1 := filepath.Join(dir, "a.txt")
	path2 := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(path1, []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path2, []byte("b"), 0644); err != nil {
		t.Fatal(err)
	}

	id1, err := SourceID(path1)
	if err != nil {
		t.Fatalf("SourceID(path1) error = %v", err)
	}

	id2, err := SourceID(path2)
	if err != nil {
		t.Fatalf("SourceID(path2) error = %v", err)
	}

	if id1 == id2 {
		t.Errorf("different paths produced same SourceID: %q", id1)
	}
}

func TestSourceID_RelativeVsAbsolute(t *testing.T) {
	dir := t.TempDir()

	// Resolve symlinks so both abs and rel paths match (macOS /var -> /private/var)
	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(realDir, "test.txt")
	if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	absID, err := SourceID(path)
	if err != nil {
		t.Fatalf("SourceID(abs) error = %v", err)
	}

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	if err := os.Chdir(realDir); err != nil {
		t.Fatal(err)
	}

	relID, err := SourceID("test.txt")
	if err != nil {
		t.Fatalf("SourceID(rel) error = %v", err)
	}

	if absID != relID {
		t.Errorf("relative and absolute paths produced different IDs: %q != %q", absID, relID)
	}
}

func TestNormalizeWindows_LowercasesDriveLetterAndPath(t *testing.T) {
	got := normalizeWindows(`C:\Users\Gra\AppData\LocalLow\VRChat\VRChat\output_log_2026-08-18_16-06-48.txt`)
	want := `c:\users\gra\appdata\locallow\vrchat\vrchat\output_log_2026-08-18_16-06-48.txt`

	if got != want {
		t.Fatalf("normalizeWindows() = %q, want %q", got, want)
	}
}

func TestNormalizeWindows_MixedCasePathEquivalent(t *testing.T) {
	a := normalizeWindows(`C:\Users\Gra\AppData\LocalLow\VRChat\VRChat\output_log_2026-08-18_16-06-48.txt`)
	b := normalizeWindows(`c:\users\gra\appdata\locallow\vrchat\vrchat\OUTPUT_LOG_2026-08-18_16-06-48.TXT`)

	if a != b {
		t.Fatalf("mixed-case Windows paths should normalize equally: %q != %q", a, b)
	}
}
