package vrclog

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strconv"
)

const maxLineSize = 1 << 20

const lineReaderBufSize = 64 * 1024

type lineReader struct {
	br     *bufio.Reader
	offset int64
	line   uint64
}

func newLineReader(r io.Reader, startOffset int64, startLine uint64) *lineReader {
	return &lineReader{
		br:     bufio.NewReaderSize(r, lineReaderBufSize),
		offset: startOffset,
		line:   startLine,
	}
}

// next reads the next physical line. terminated reports whether the
// returned data ended with a newline. When terminated is false (EOF
// reached with unterminated trailing data), lr.offset and lr.line are
// NOT advanced — the next call to next() (against a fresh reader seeked
// to the same position) will re-read the same fragment from its start.
func (lr *lineReader) next() (raw []byte, rawHash [32]byte, offset int64, nextOffset int64, line uint64, terminated bool, issue *RecordIssue, err error) {
	lineStart := lr.offset
	lineNum := lr.line

	h := sha256.New()
	var accumulated []byte
	var totalBytes int64
	var oversized bool
	var hasData bool

	for {
		fragment, readErr := lr.br.ReadSlice('\n')

		if len(fragment) > 0 {
			hasData = true
			h.Write(fragment)
			totalBytes += int64(len(fragment))

			if !oversized {
				if totalBytes <= maxLineSize {
					cp := make([]byte, len(fragment))
					copy(cp, fragment)
					accumulated = append(accumulated, cp...)
				} else {
					// Crossed the threshold mid-fragment: keep only up to maxLineSize
					keep := int64(len(fragment)) - (totalBytes - maxLineSize)
					if keep > 0 {
						cp := make([]byte, keep)
						copy(cp, fragment[:keep])
						accumulated = append(accumulated, cp...)
					}
					oversized = true
				}
			}
		}

		if readErr == nil {
			// Found '\n' — line complete
			lr.offset = lineStart + totalBytes
			lr.line = lineNum + 1

			// Strip terminator from accumulated for Raw field
			raw = stripTerminator(accumulated)

			copy(rawHash[:], h.Sum(nil))
			offset = lineStart
			nextOffset = lr.offset
			line = lineNum
			terminated = true

			if oversized {
				issue = &RecordIssue{
					Code:    "line_too_long",
					Message: "line exceeds maximum size of " + strconv.FormatInt(maxLineSize, 10) + " bytes",
				}
			}
			return
		}

		if readErr == bufio.ErrBufferFull {
			// Partial read — line continues beyond buffer
			continue
		}

		if readErr == io.EOF {
			if !hasData {
				err = io.EOF
				return
			}

			// Final line without terminator. lr.offset/lr.line are
			// intentionally NOT advanced: callers that must not commit
			// unterminated data (active Follow) rely on this to safely
			// re-read the same fragment from a fresh reader later.
			raw = accumulated

			copy(rawHash[:], h.Sum(nil))
			offset = lineStart
			nextOffset = lineStart + totalBytes
			line = lineNum
			terminated = false

			if oversized {
				issue = &RecordIssue{
					Code:    "line_too_long",
					Message: "line exceeds maximum size of " + strconv.FormatInt(maxLineSize, 10) + " bytes",
				}
			}
			return
		}

		// Fatal I/O error
		err = readErr
		return
	}
}

func stripTerminator(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	// The last byte should be '\n' if we got here from a complete line
	if b[len(b)-1] == '\n' {
		b = b[:len(b)-1]
		if len(b) > 0 && b[len(b)-1] == '\r' {
			b = b[:len(b)-1]
		}
	}
	return b
}

// record_id = SHA-256(source_id + NUL + decimal(offset) + NUL + raw_hash)
func computeRecordID(sourceID SourceID, offset int64, rawHash [32]byte) RecordID {
	h := sha256.New()
	h.Write([]byte(string(sourceID)))
	h.Write([]byte{0})
	h.Write([]byte(strconv.FormatInt(offset, 10)))
	h.Write([]byte{0})
	h.Write(rawHash[:])
	sum := h.Sum(nil)
	return RecordID(hex.EncodeToString(sum))
}
