package vrclog

import "time"

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
