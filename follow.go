package vrclog

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"os"
	"path/filepath"
	"time"

	"github.com/vrclog/vrclog-go/internal/logfile"
)

const DefaultPollInterval = 1 * time.Second

const MinPollInterval = 100 * time.Millisecond

const rotationSettlePolls = 2

const rotationSettleTimeout = 2 * time.Second

type FollowConfig struct {
	Directory    string
	Cursor       *Cursor
	PollInterval time.Duration
}

func DefaultLogDirectory() (string, error) {
	dir, err := logfile.DefaultLogDirectory()
	if err != nil {
		if errors.Is(err, logfile.ErrNoLogDirectory) {
			return "", ErrNoLogDirectory
		}
		return "", err
	}
	return dir, nil
}

func Follow(ctx context.Context, cfg FollowConfig) iter.Seq2[Record, error] {
	return func(yield func(Record, error) bool) {
		if cfg.PollInterval < 0 {
			yield(Record{}, errors.New("poll interval must not be negative"))
			return
		}
		if cfg.PollInterval > 0 && cfg.PollInterval < MinPollInterval {
			yield(Record{}, fmt.Errorf("poll interval must be at least %s", MinPollInterval))
			return
		}

		pollInterval := cfg.PollInterval
		if pollInterval == 0 {
			pollInterval = DefaultPollInterval
		}

		dir := cfg.Directory
		if dir == "" {
			d, err := DefaultLogDirectory()
			if err != nil {
				yield(Record{}, ErrNoLogDirectory)
				return
			}
			dir = d
		}
		absDir, err := filepath.Abs(dir)
		if err != nil {
			yield(Record{}, err)
			return
		}
		dir = filepath.Clean(absDir)

		fs := &followState{
			dir:          dir,
			pollInterval: pollInterval,
		}

		if cfg.Cursor != nil {
			fs.startWithCursor(ctx, cfg.Cursor, yield)
		} else {
			fs.startWithoutCursor(ctx, yield)
		}
	}
}

type followState struct {
	dir          string
	pollInterval time.Duration
	currentFile  string
	currentOff   int64
	currentLine  uint64
}

func (fs *followState) startWithCursor(ctx context.Context, cursor *Cursor, yield func(Record, error) bool) {
	path, err := filepath.Abs(cursor.Path)
	if err != nil {
		yield(Record{}, fmt.Errorf("cursor path: %w", err))
		return
	}
	path = filepath.Clean(path)

	srcID, err := logfile.SourceID(path)
	if err != nil {
		yield(Record{}, fmt.Errorf("cursor source ID: %w", err))
		return
	}

	if SourceID(srcID) != cursor.SourceID {
		yield(Record{}, ErrCursorSourceMissing)
		return
	}

	f, info, err := logfile.OpenRegular(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			yield(Record{}, ErrCursorSourceMissing)
			return
		}
		yield(Record{}, fmt.Errorf("open cursor file: %w", err))
		return
	}
	defer f.Close()

	if cursor.Offset > info.Size() {
		yield(Record{}, fmt.Errorf("%w: cursor offset %d exceeds file size %d", ErrInvalidOffset, cursor.Offset, info.Size()))
		return
	}

	if cursor.Offset > 0 {
		if _, err := f.Seek(cursor.Offset, io.SeekStart); err != nil {
			yield(Record{}, fmt.Errorf("seek cursor file: %w", err))
			return
		}
	}

	sid := SourceID(srcID)

	// The cursor file is "settled" (flush its final unterminated
	// fragment, if any) when a newer log file already exists; otherwise
	// it is the active/newest file and unterminated fragments must not
	// be committed. Use the strict variant so a directory listing or
	// candidate open failure is reported rather than silently defaulting
	// to "cursor file is latest" (which would delay flushing a settled
	// file's final fragment).
	files, err := logfile.ListLogFilesStrict(fs.dir)
	if err != nil {
		yield(Record{}, fmt.Errorf("list log files: %w", err))
		return
	}
	isLatest := files[len(files)-1].Path == path

	lr := newLineReader(f, cursor.Offset, cursor.Line)

	var ok bool
	if isLatest {
		ok = readActiveRecords(ctx, lr, sid, path, yield)
	} else {
		ok = readFiniteRecords(ctx, lr, sid, path, yield)
	}
	if !ok {
		return
	}

	fs.currentFile = path
	fs.currentOff = lr.offset
	fs.currentLine = lr.line

	fs.pollLoop(ctx, yield)
}

func (fs *followState) startWithoutCursor(ctx context.Context, yield func(Record, error) bool) {
	var latestPath string

	for {
		if ctx.Err() != nil {
			return
		}

		files, err := logfile.ListLogFilesStrict(fs.dir)
		if err == nil {
			latestPath = files[len(files)-1].Path
			break
		}

		if !errors.Is(err, logfile.ErrNoLogFiles) {
			yield(Record{}, fmt.Errorf("list log files: %w", err))
			return
		}

		select {
		case <-time.After(fs.pollInterval):
		case <-ctx.Done():
			return
		}
	}

	f, _, err := logfile.OpenRegular(latestPath)
	if err != nil {
		yield(Record{}, err)
		return
	}

	srcIDStr, err := logfile.SourceID(latestPath)
	if err != nil {
		f.Close()
		yield(Record{}, err)
		return
	}
	sid := SourceID(srcIDStr)
	lr := newLineReader(f, 0, 1)

	// The latest file at startup is always the active file.
	ok := readActiveRecords(ctx, lr, sid, latestPath, yield)
	if !ok {
		f.Close()
		return
	}

	fs.currentFile = latestPath
	fs.currentOff = lr.offset
	fs.currentLine = lr.line
	f.Close()

	fs.pollLoop(ctx, yield)
}

func (fs *followState) pollLoop(ctx context.Context, yield func(Record, error) bool) {
	for {
		if ctx.Err() != nil {
			return
		}

		status, err := checkFileStatus(fs.currentFile, fs.currentOff)
		if err != nil {
			yield(Record{}, err)
			return
		}

		progressed := false
		if status == fileGrown {
			beforeOff := fs.currentOff
			if !fs.readGrowth(ctx, yield) {
				return
			}
			// readGrowth only advances fs.currentOff when it committed at
			// least one newline-terminated record. If the file grew but
			// only added to a still-unterminated fragment, currentOff is
			// unchanged here — that must NOT be treated as "progress",
			// or the loop below would busy-spin (re-opening the file and
			// re-scanning the directory on every iteration) for as long
			// as the fragment stays incomplete.
			progressed = fs.currentOff != beforeOff
		}

		// Always check for rotation, even when growth was just read: an
		// unterminated fragment in the active file must never prevent
		// rotation detection (it would otherwise busy-loop forever
		// re-reading the same fragment without ever seeing newer files).
		newerFiles, err := fs.findNewerFiles()
		if err != nil {
			yield(Record{}, err)
			return
		}
		if len(newerFiles) > 0 {
			if !fs.settleAndSwitch(ctx, newerFiles, yield) {
				return
			}
			continue
		}

		if progressed {
			// At least one record was just committed; check again
			// immediately in case more complete lines are already
			// available, instead of waiting out a full poll interval.
			continue
		}

		select {
		case <-time.After(fs.pollInterval):
		case <-ctx.Done():
			return
		}
	}
}

func (fs *followState) settleAndSwitch(ctx context.Context, newerFiles []logfile.LogFileInfo, yield func(Record, error) bool) bool {
	deadline := time.Now().Add(rotationSettleTimeout)

	for i := 0; i < rotationSettlePolls; i++ {
		if time.Now().After(deadline) {
			break
		}
		if ctx.Err() != nil {
			return false
		}

		status, err := checkFileStatus(fs.currentFile, fs.currentOff)
		if err != nil {
			yield(Record{}, err)
			return false
		}
		if status == fileGrown {
			if !fs.readGrowth(ctx, yield) {
				return false
			}
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		pause := fs.pollInterval
		if pause > remaining {
			pause = remaining
		}
		select {
		case <-time.After(pause):
		case <-ctx.Done():
			return false
		}
	}

	status, err := checkFileStatus(fs.currentFile, fs.currentOff)
	if err != nil {
		yield(Record{}, err)
		return false
	}
	if status == fileGrown {
		if !fs.readGrowth(ctx, yield) {
			return false
		}
	}

	// The old (now-settled) file may still hold an unterminated final
	// fragment. Flush it exactly once before moving on to newer files.
	if !fs.readFinalFlush(ctx, yield) {
		return false
	}

	for i, nf := range newerFiles {
		if ctx.Err() != nil {
			return false
		}

		isLast := i == len(newerFiles)-1
		off, line, ok := readEntireFile(ctx, nf.Path, !isLast, yield)
		if !ok {
			return false
		}
		fs.currentFile = nf.Path
		fs.currentOff = off
		fs.currentLine = line
	}

	return true
}

// readFinalFlush reads and flushes the current file's unterminated final
// fragment (if any) exactly once. It is used when the current file has
// settled (a newer file now exists) so its trailing fragment will never
// receive a completing newline.
func (fs *followState) readFinalFlush(ctx context.Context, yield func(Record, error) bool) bool {
	f, _, err := logfile.OpenRegular(fs.currentFile)
	if err != nil {
		yield(Record{}, err)
		return false
	}
	defer f.Close()

	srcIDStr, err := logfile.SourceID(fs.currentFile)
	if err != nil {
		yield(Record{}, err)
		return false
	}
	sid := SourceID(srcIDStr)

	if fs.currentOff > 0 {
		if _, err := f.Seek(fs.currentOff, io.SeekStart); err != nil {
			yield(Record{}, err)
			return false
		}
	}

	lr := newLineReader(f, fs.currentOff, fs.currentLine)
	if !readFiniteRecords(ctx, lr, sid, fs.currentFile, yield) {
		return false
	}
	fs.currentOff = lr.offset
	fs.currentLine = lr.line
	return true
}

// readGrowth reads newly-appended data from the active current file.
// Only newline-terminated lines are emitted; an unterminated trailing
// fragment is left unread and un-committed so a later poll can re-read
// and (once it completes) emit it as a single Record.
func (fs *followState) readGrowth(ctx context.Context, yield func(Record, error) bool) bool {
	f, _, err := logfile.OpenRegular(fs.currentFile)
	if err != nil {
		yield(Record{}, err)
		return false
	}
	defer f.Close()

	srcIDStr, err := logfile.SourceID(fs.currentFile)
	if err != nil {
		yield(Record{}, err)
		return false
	}
	sid := SourceID(srcIDStr)

	if fs.currentOff > 0 {
		if _, err := f.Seek(fs.currentOff, io.SeekStart); err != nil {
			yield(Record{}, err)
			return false
		}
	}

	lr := newLineReader(f, fs.currentOff, fs.currentLine)

	for {
		if ctx.Err() != nil {
			return false
		}

		rawBytes, rawHash, offset, nextOffset, lineNum, terminated, issue, readErr := lr.next()
		if readErr == io.EOF {
			return true
		}
		if readErr != nil {
			yield(Record{}, readErr)
			return false
		}
		if !terminated {
			// Unterminated fragment at EOF: do not emit, do not commit.
			// lr.offset/lr.line were not advanced by lineReader, so the
			// next lr.next() call returns io.EOF and this loop exits via
			// the branch above with fs.currentOff left untouched.
			continue
		}

		rec := buildRecord(rawBytes, rawHash, offset, nextOffset, lineNum, issue, sid, fs.currentFile)

		if !yield(rec, nil) {
			return false
		}

		fs.currentOff = nextOffset
		fs.currentLine = lineNum + 1
	}
}

func (fs *followState) findNewerFiles() ([]logfile.LogFileInfo, error) {
	files, err := logfile.ListLogFilesStrict(fs.dir)
	if err != nil {
		if errors.Is(err, logfile.ErrNoLogFiles) {
			return nil, nil
		}
		return nil, err
	}

	idx := -1
	for i, f := range files {
		if f.Path == fs.currentFile {
			idx = i
			break
		}
	}

	if idx < 0 || idx >= len(files)-1 {
		return nil, nil
	}

	return files[idx+1:], nil
}

// readActiveRecords reads lr to EOF, emitting only newline-terminated
// records. An unterminated trailing fragment is silently dropped from
// this read (the caller is expected to track its own committed
// offset/line separately, e.g. via followState.currentOff/currentLine,
// and re-read the fragment later once more data arrives).
func readActiveRecords(ctx context.Context, lr *lineReader, srcID SourceID, path string, yield func(Record, error) bool) bool {
	for {
		if ctx.Err() != nil {
			return false
		}

		rawBytes, rawHash, offset, nextOffset, lineNum, terminated, issue, readErr := lr.next()
		if readErr == io.EOF {
			return true
		}
		if readErr != nil {
			yield(Record{}, readErr)
			return false
		}
		if !terminated {
			continue
		}

		rec := buildRecord(rawBytes, rawHash, offset, nextOffset, lineNum, issue, srcID, path)

		if !yield(rec, nil) {
			return false
		}
	}
}

// readFiniteRecords reads lr to EOF, emitting every record including an
// unterminated final fragment. It is used for settled files: the current
// (now-superseded) file after rotation, and intermediate files skipped
// over during rotation.
func readFiniteRecords(ctx context.Context, lr *lineReader, srcID SourceID, path string, yield func(Record, error) bool) bool {
	for {
		if ctx.Err() != nil {
			return false
		}

		rawBytes, rawHash, offset, nextOffset, lineNum, terminated, issue, readErr := lr.next()
		if readErr == io.EOF {
			return true
		}
		if readErr != nil {
			yield(Record{}, readErr)
			return false
		}

		rec := buildRecord(rawBytes, rawHash, offset, nextOffset, lineNum, issue, srcID, path)

		if !yield(rec, nil) {
			return false
		}

		if !terminated {
			// lineReader does not advance its own offset/line for
			// unterminated data; since we are flushing it here, commit
			// the position manually so the caller's bookkeeping (and any
			// subsequent read from this lr) is consistent.
			lr.offset = nextOffset
			lr.line = lineNum + 1
		}
	}
}

// readEntireFile opens path fresh and reads it from the start using
// either finite (flush=true) or active (flush=false) semantics.
//
// ok is false whenever the caller must stop without yielding again: this
// covers both a genuine read error (already reported via yield exactly
// once, either here or inside readFiniteRecords/readActiveRecords) and
// the consumer breaking out of the range loop (yield already returned
// false once — calling it again would violate the iterator contract).
func readEntireFile(ctx context.Context, path string, flush bool, yield func(Record, error) bool) (finalOff int64, finalLine uint64, ok bool) {
	f, _, err := logfile.OpenRegular(path)
	if err != nil {
		yield(Record{}, fmt.Errorf("open %s: %w", path, err))
		return 0, 0, false
	}
	defer f.Close()

	srcIDStr, err := logfile.SourceID(path)
	if err != nil {
		yield(Record{}, fmt.Errorf("source ID for %s: %w", path, err))
		return 0, 0, false
	}
	sid := SourceID(srcIDStr)
	lr := newLineReader(f, 0, 1)

	var readOK bool
	if flush {
		readOK = readFiniteRecords(ctx, lr, sid, path, yield)
	} else {
		readOK = readActiveRecords(ctx, lr, sid, path, yield)
	}
	return lr.offset, lr.line, readOK
}

type fileStatus int

const (
	fileUnchanged fileStatus = iota
	fileGrown
)

// checkFileStatus stats path and compares its size against lastOffset.
// It returns ErrSourceTruncated (wrapped) if the file has shrunk below
// the already-committed offset, and otherwise reports whether the file
// has grown. Any stat error (missing file, permission denied, etc.) is
// returned as-is so callers can distinguish it from "no new data".
func checkFileStatus(path string, lastOffset int64) (fileStatus, error) {
	info, err := os.Stat(path)
	if err != nil {
		return fileUnchanged, err
	}
	if info.Size() < lastOffset {
		return fileUnchanged, fmt.Errorf("%w: file size %d < committed offset %d", ErrSourceTruncated, info.Size(), lastOffset)
	}
	if info.Size() > lastOffset {
		return fileGrown, nil
	}
	return fileUnchanged, nil
}
