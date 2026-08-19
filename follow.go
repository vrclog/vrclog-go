package vrclog

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"os"
	"path/filepath"
	"strings"
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
		yield(Record{}, ErrCursorSourceMissing)
		return
	}
	path = filepath.Clean(path)

	srcID, err := logfile.SourceID(path)
	if err != nil {
		yield(Record{}, ErrCursorSourceMissing)
		return
	}

	if SourceID(srcID) != cursor.SourceID {
		yield(Record{}, ErrCursorSourceMissing)
		return
	}

	f, info, err := logfile.OpenRegular(path)
	if err != nil {
		yield(Record{}, ErrCursorSourceMissing)
		return
	}
	defer f.Close()

	if cursor.Offset > info.Size() {
		yield(Record{}, fmt.Errorf("%w: cursor offset %d exceeds file size %d", ErrInvalidOffset, cursor.Offset, info.Size()))
		return
	}

	if cursor.Offset > 0 {
		if _, err := f.Seek(cursor.Offset, io.SeekStart); err != nil {
			yield(Record{}, ErrCursorSourceMissing)
			return
		}
	}

	lr := newLineReader(f, cursor.Offset, cursor.Line)
	sid := SourceID(srcID)

	if !readFileRecords(ctx, lr, sid, path, yield) {
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

		files, err := logfile.ListLogFiles(fs.dir)
		if err == nil && len(files) > 0 {
			latestPath = files[len(files)-1].Path
			break
		}

		if err != nil && !errors.Is(err, logfile.ErrNoLogFiles) {
			yield(Record{}, err)
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

	if !readFileRecords(ctx, lr, sid, latestPath, yield) {
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

		grown, _ := hasFileGrown(fs.currentFile, fs.currentOff)
		if grown {
			if !fs.readGrowth(ctx, yield) {
				return
			}
			continue
		}

		newerFiles := fs.findNewerFiles()
		if len(newerFiles) > 0 {
			if !fs.settleAndSwitch(ctx, newerFiles, yield) {
				return
			}
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

		grown, _ := hasFileGrown(fs.currentFile, fs.currentOff)
		if grown {
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

	grown, _ := hasFileGrown(fs.currentFile, fs.currentOff)
	if grown {
		if !fs.readGrowth(ctx, yield) {
			return false
		}
	}

	for _, nf := range newerFiles {
		if ctx.Err() != nil {
			return false
		}

		off, line, ok := readEntireFile(ctx, nf.Path, yield)
		if !ok {
			return false
		}
		fs.currentFile = nf.Path
		fs.currentOff = off
		fs.currentLine = line
	}

	return true
}

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

		rawBytes, rawHash, offset, nextOffset, lineNum, issue, readErr := lr.next()
		if readErr == io.EOF {
			fs.currentOff = lr.offset
			fs.currentLine = lr.line
			return true
		}
		if readErr != nil {
			yield(Record{}, readErr)
			return false
		}

		rawStr := strings.ToValidUTF8(string(rawBytes), "�")
		t, level, message, ok := decodeHeader(rawStr, nil)
		if !ok {
			message = rawStr
			level = LevelUnknown
		}

		rec := Record{
			ID:         computeRecordID(sid, offset, rawHash),
			Time:       t,
			Level:      level,
			Message:    message,
			Raw:        rawStr,
			SourceID:   sid,
			Path:       fs.currentFile,
			Offset:     offset,
			NextOffset: nextOffset,
			Line:       lineNum,
			Issue:      issue,
		}

		if !yield(rec, nil) {
			return false
		}
	}
}

func (fs *followState) findNewerFiles() []logfile.LogFileInfo {
	files, err := logfile.ListLogFiles(fs.dir)
	if err != nil {
		return nil
	}

	idx := -1
	for i, f := range files {
		if f.Path == fs.currentFile {
			idx = i
			break
		}
	}

	if idx < 0 || idx >= len(files)-1 {
		return nil
	}

	return files[idx+1:]
}

func readFileRecords(ctx context.Context, lr *lineReader, srcID SourceID, path string, yield func(Record, error) bool) bool {
	for {
		if ctx.Err() != nil {
			return false
		}

		rawBytes, rawHash, offset, nextOffset, lineNum, issue, readErr := lr.next()
		if readErr == io.EOF {
			return true
		}
		if readErr != nil {
			yield(Record{}, readErr)
			return false
		}

		rawStr := strings.ToValidUTF8(string(rawBytes), "�")
		t, level, message, ok := decodeHeader(rawStr, nil)
		if !ok {
			message = rawStr
			level = LevelUnknown
		}

		rec := Record{
			ID:         computeRecordID(srcID, offset, rawHash),
			Time:       t,
			Level:      level,
			Message:    message,
			Raw:        rawStr,
			SourceID:   srcID,
			Path:       path,
			Offset:     offset,
			NextOffset: nextOffset,
			Line:       lineNum,
			Issue:      issue,
		}

		if !yield(rec, nil) {
			return false
		}
	}
}

func readEntireFile(ctx context.Context, path string, yield func(Record, error) bool) (finalOff int64, finalLine uint64, ok bool) {
	f, _, err := logfile.OpenRegular(path)
	if err != nil {
		return 0, 0, true
	}
	defer f.Close()

	srcIDStr, err := logfile.SourceID(path)
	if err != nil {
		return 0, 0, true
	}
	sid := SourceID(srcIDStr)
	lr := newLineReader(f, 0, 1)

	if !readFileRecords(ctx, lr, sid, path, yield) {
		return 0, 0, false
	}
	return lr.offset, lr.line, true
}

func hasFileGrown(path string, lastOffset int64) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return info.Size() > lastOffset, nil
}
