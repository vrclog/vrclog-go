package vrclog

import "testing"

func TestRecordCursor(t *testing.T) {
	r := Record{
		SourceID:   "src-abc",
		Path:       "/var/log/output_log.txt",
		Offset:     100,
		NextOffset: 200,
		Line:       5,
	}
	c := r.Cursor()

	if c.SourceID != r.SourceID {
		t.Errorf("Cursor.SourceID = %q, want %q", c.SourceID, r.SourceID)
	}
	if c.Path != r.Path {
		t.Errorf("Cursor.Path = %q, want %q", c.Path, r.Path)
	}
	if c.Offset != r.NextOffset {
		t.Errorf("Cursor.Offset = %d, want %d (NextOffset)", c.Offset, r.NextOffset)
	}
	if c.Line != r.Line+1 {
		t.Errorf("Cursor.Line = %d, want %d (Line+1)", c.Line, r.Line+1)
	}
}

func TestRecordCursorZeroLine(t *testing.T) {
	r := Record{
		SourceID:   "src-xyz",
		Path:       "/tmp/test.txt",
		Offset:     0,
		NextOffset: 50,
		Line:       1,
	}
	c := r.Cursor()
	if c.Line != 2 {
		t.Errorf("Cursor.Line = %d, want 2", c.Line)
	}
	if c.Offset != 50 {
		t.Errorf("Cursor.Offset = %d, want 50", c.Offset)
	}
}
