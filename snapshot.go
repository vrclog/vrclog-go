package vrclog

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vrclog/vrclog-go/internal/logfile"
)

// LogSnapshot captures the byte head of every currently existing VRChat
// output_log file in a directory at a single point in time. It lets a
// downstream consumer distinguish bytes that existed before the snapshot
// was captured from bytes appended afterward, without relying on
// timestamp heuristics.
//
// LogSnapshot uses path-and-size semantics: SourceID identifies a log
// file by its normalized path, not by a stable file incarnation. If a
// file at the same path is replaced by a different file after capture
// (same path, different content, but a size at or above the captured
// size), Contains cannot distinguish that from an untouched file — it
// will report true for any record within the captured size. VRChat log
// files are created with unique timestamped names
// (output_log_YYYY-MM-DD_HH-MM-SS.txt), so same-path replacement does
// not occur in practice.
type LogSnapshot struct {
	sizes map[SourceID]int64
}

// CaptureLogSnapshot captures the byte head of every currently existing
// VRChat output_log file in the selected directory. An empty directory
// argument uses DefaultLogDirectory. No log files is a valid empty
// snapshot, not an error.
func CaptureLogSnapshot(directory string) (LogSnapshot, error) {
	dir := directory
	if dir == "" {
		d, err := DefaultLogDirectory()
		if err != nil {
			return LogSnapshot{}, err
		}
		dir = d
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return LogSnapshot{}, err
	}
	dir = filepath.Clean(absDir)

	files, err := logfile.ListLogFilesStrict(dir)
	if err != nil {
		if errors.Is(err, logfile.ErrNoLogFiles) {
			return LogSnapshot{sizes: map[SourceID]int64{}}, nil
		}
		return LogSnapshot{}, err
	}

	sizes := make(map[SourceID]int64, len(files))
	for _, f := range files {
		info, err := os.Stat(f.Path)
		if err != nil {
			return LogSnapshot{}, fmt.Errorf("stat %s: %w", f.Path, err)
		}
		srcIDStr, err := logfile.SourceID(f.Path)
		if err != nil {
			return LogSnapshot{}, fmt.Errorf("source ID for %s: %w", f.Path, err)
		}
		sizes[SourceID(srcIDStr)] = info.Size()
	}

	return LogSnapshot{sizes: sizes}, nil
}

// Contains reports whether all bytes of record already existed when the
// snapshot was captured: record.SourceID must have been present in the
// snapshot, and record.NextOffset must not exceed the captured size for
// that source.
func (s LogSnapshot) Contains(record Record) bool {
	size, ok := s.sizes[record.SourceID]
	if !ok {
		return false
	}
	return record.NextOffset <= size
}
