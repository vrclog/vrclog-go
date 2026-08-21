package vrclog

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"path/filepath"

	"github.com/vrclog/vrclog-go/internal/logfile"
)

type ReadFileConfig struct {
	Path   string
	Offset int64
	Line   uint64
}

func ReadFile(ctx context.Context, cfg ReadFileConfig) iter.Seq2[Record, error] {
	return func(yield func(Record, error) bool) {
		if cfg.Path == "" {
			yield(Record{}, errors.New("path is required"))
			return
		}
		if cfg.Offset < 0 {
			yield(Record{}, errors.New("offset must not be negative"))
			return
		}
		if cfg.Offset > 0 && cfg.Line == 0 {
			yield(Record{}, errors.New("line is required when offset > 0"))
			return
		}

		path, err := filepath.Abs(cfg.Path)
		if err != nil {
			yield(Record{}, err)
			return
		}
		path = filepath.Clean(path)

		startLine := cfg.Line
		if startLine == 0 && cfg.Offset == 0 {
			startLine = 1
		}

		f, info, err := logfile.OpenRegular(path)
		if err != nil {
			yield(Record{}, err)
			return
		}
		defer f.Close()

		if cfg.Offset > info.Size() {
			yield(Record{}, fmt.Errorf("%w: offset %d exceeds file size %d", ErrInvalidOffset, cfg.Offset, info.Size()))
			return
		}

		srcIDStr, err := logfile.SourceID(path)
		if err != nil {
			yield(Record{}, err)
			return
		}
		srcID := SourceID(srcIDStr)

		if cfg.Offset > 0 {
			if _, err := f.Seek(cfg.Offset, io.SeekStart); err != nil {
				yield(Record{}, err)
				return
			}
		}

		lr := newLineReader(f, cfg.Offset, startLine)

		for {
			if ctx.Err() != nil {
				return
			}

			// ReadFile has finite semantics: an unterminated final line
			// (terminated == false) is still emitted, matching a settled
			// read of the whole file.
			rawBytes, rawHash, offset, nextOffset, lineNum, _, issue, readErr := lr.next()
			if readErr == io.EOF {
				return
			}
			if readErr != nil {
				yield(Record{}, readErr)
				return
			}

			rec := buildRecord(rawBytes, rawHash, offset, nextOffset, lineNum, issue, srcID, path)

			if !yield(rec, nil) {
				return
			}
		}
	}
}
