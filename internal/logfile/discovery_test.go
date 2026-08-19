package logfile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestListLogFiles_SortsByFilenameTimestamp(t *testing.T) {
	dir := t.TempDir()

	names := []string{
		"output_log_2024-01-03_10-00-00.txt",
		"output_log_2024-01-01_08-00-00.txt",
		"output_log_2024-01-02_09-00-00.txt",
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("test\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	files, err := ListLogFiles(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(files))
	}

	expected := []string{
		"output_log_2024-01-01_08-00-00.txt",
		"output_log_2024-01-02_09-00-00.txt",
		"output_log_2024-01-03_10-00-00.txt",
	}
	for i, want := range expected {
		if files[i].Name != want {
			t.Errorf("files[%d].Name = %q, want %q", i, files[i].Name, want)
		}
	}
}

func TestListLogFiles_RelativeDirReturnsAbsolutePaths(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "logs")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "output_log_2024-01-01_00-00-00.txt"), []byte("test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	if err := os.Chdir(parent); err != nil {
		t.Fatal(err)
	}

	files, err := ListLogFiles("logs")
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if !filepath.IsAbs(files[0].Path) {
		t.Fatalf("Path = %q, want absolute path", files[0].Path)
	}
	want, err := filepath.Abs(filepath.Join("logs", "output_log_2024-01-01_00-00-00.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want = filepath.Clean(want)
	if files[0].Path != want {
		t.Fatalf("Path = %q, want %q", files[0].Path, want)
	}
}

func TestListLogFiles_DirGlobMetacharactersAreLiteral(t *testing.T) {
	parent := t.TempDir()
	literalDir := filepath.Join(parent, "logs[abc]")
	if err := os.Mkdir(literalDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(literalDir, "output_log_2024-01-01_00-00-00.txt"), []byte("test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := ListLogFiles(literalDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Path != filepath.Join(literalDir, "output_log_2024-01-01_00-00-00.txt") {
		t.Fatalf("matched unexpected path %q", files[0].Path)
	}
}

func TestListLogFiles_FallbackToModTimeAndName(t *testing.T) {
	dir := t.TempDir()

	names := []string{
		"output_log_zzz.txt",
		"output_log_aaa.txt",
		"output_log_mmm.txt",
	}

	base := time.Now()
	for i, name := range names {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("test\n"), 0644); err != nil {
			t.Fatal(err)
		}
		mt := base.Add(time.Duration(i) * time.Second)
		if err := os.Chtimes(p, mt, mt); err != nil {
			t.Fatal(err)
		}
	}

	files, err := ListLogFiles(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(files))
	}

	expected := []string{
		"output_log_zzz.txt",
		"output_log_aaa.txt",
		"output_log_mmm.txt",
	}
	for i, want := range expected {
		if files[i].Name != want {
			t.Errorf("files[%d].Name = %q, want %q", i, files[i].Name, want)
		}
	}
}

func TestListLogFiles_ModTimeTiebreakByName(t *testing.T) {
	dir := t.TempDir()

	names := []string{
		"output_log_beta.txt",
		"output_log_alpha.txt",
	}

	mt := time.Now()
	for _, name := range names {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("test\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, mt, mt); err != nil {
			t.Fatal(err)
		}
	}

	files, err := ListLogFiles(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}

	if files[0].Name != "output_log_alpha.txt" {
		t.Errorf("files[0].Name = %q, want output_log_alpha.txt", files[0].Name)
	}
	if files[1].Name != "output_log_beta.txt" {
		t.Errorf("files[1].Name = %q, want output_log_beta.txt", files[1].Name)
	}
}

func TestListLogFiles_ParseableBeforeUnparseable(t *testing.T) {
	dir := t.TempDir()

	names := []string{
		"output_log_custom.txt",
		"output_log_2024-01-01_00-00-00.txt",
	}

	mt := time.Now().Add(time.Hour)
	for _, name := range names {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("test\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, mt, mt); err != nil {
			t.Fatal(err)
		}
	}

	files, err := ListLogFiles(dir)
	if err != nil {
		t.Fatal(err)
	}

	if files[0].Name != "output_log_2024-01-01_00-00-00.txt" {
		t.Errorf("parseable file should come first, got %q", files[0].Name)
	}
}

func TestListLogFiles_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	_, err := ListLogFiles(dir)
	if !errors.Is(err, ErrNoLogFiles) {
		t.Errorf("expected ErrNoLogFiles, got %v", err)
	}
}

func TestListLogFiles_SkipsNonRegular(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "output_log_real.txt"), []byte("test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	subdir := filepath.Join(dir, "output_log_dir.txt")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatal(err)
	}

	files, err := ListLogFiles(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Name != "output_log_real.txt" {
		t.Errorf("expected output_log_real.txt, got %q", files[0].Name)
	}
}

func TestFindLatestLogFile_ReturnsNewest(t *testing.T) {
	dir := t.TempDir()

	names := []string{
		"output_log_2024-01-01_00-00-00.txt",
		"output_log_2024-01-03_00-00-00.txt",
		"output_log_2024-01-02_00-00-00.txt",
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("test\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	path, err := FindLatestLogFile(dir)
	if err != nil {
		t.Fatal(err)
	}

	if filepath.Base(path) != "output_log_2024-01-03_00-00-00.txt" {
		t.Errorf("expected newest file, got %q", filepath.Base(path))
	}
}

func TestFindLatestLogFile_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	_, err := FindLatestLogFile(dir)
	if !errors.Is(err, ErrNoLogFiles) {
		t.Errorf("expected ErrNoLogFiles, got %v", err)
	}
}

func TestMaxFollowCandidateFiles_Constant(t *testing.T) {
	if MaxFollowCandidateFiles != 4096 {
		t.Errorf("MaxFollowCandidateFiles = %d, want 4096", MaxFollowCandidateFiles)
	}
}

func TestParseFilenameTimestamp(t *testing.T) {
	ts, ok := parseFilenameTimestamp("output_log_2026-08-18_16-06-48.txt")
	if !ok {
		t.Fatal("expected parse to succeed")
	}
	if ts.Year() != 2026 || ts.Month() != 8 || ts.Day() != 18 {
		t.Errorf("unexpected date: %v", ts)
	}
	if ts.Hour() != 16 || ts.Minute() != 6 || ts.Second() != 48 {
		t.Errorf("unexpected time: %v", ts)
	}

	_, ok = parseFilenameTimestamp("output_log_invalid.txt")
	if ok {
		t.Error("expected parse to fail for invalid timestamp")
	}

	_, ok = parseFilenameTimestamp("other_file.txt")
	if ok {
		t.Error("expected parse to fail for non-matching prefix")
	}
}
