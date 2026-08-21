package vrclog

import (
	"strings"
	"time"
)

type RecordIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Record struct {
	ID         RecordID     `json:"id"`
	Time       time.Time    `json:"time"`
	Level      Level        `json:"level"`
	Message    string       `json:"message"`
	Raw        string       `json:"raw"`
	SourceID   SourceID     `json:"source_id"`
	Path       string       `json:"path"`
	Offset     int64        `json:"offset"`
	NextOffset int64        `json:"next_offset"`
	Line       uint64       `json:"line"`
	Issue      *RecordIssue `json:"issue,omitempty"`
}

type Cursor struct {
	SourceID SourceID `json:"source_id"`
	Path     string   `json:"path"`
	Offset   int64    `json:"offset"`
	Line     uint64   `json:"line"`
}

func (r Record) Cursor() Cursor {
	return Cursor{
		SourceID: r.SourceID,
		Path:     r.Path,
		Offset:   r.NextOffset,
		Line:     r.Line + 1,
	}
}

// buildRecord constructs a Record from a lineReader.next() result.
func buildRecord(rawBytes []byte, rawHash [32]byte, offset, nextOffset int64, lineNum uint64, issue *RecordIssue, srcID SourceID, path string) Record {
	rawStr := strings.ToValidUTF8(string(rawBytes), "�")

	t, level, message, ok := decodeHeader(rawStr, nil)
	if !ok {
		message = rawStr
		level = LevelUnknown
	}

	return Record{
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
}
