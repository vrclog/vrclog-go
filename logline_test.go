package vrclog

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strings"
	"testing"
)

func TestLineReader_LF(t *testing.T) {
	input := "line1\nline2\nline3\n"
	lr := newLineReader(strings.NewReader(input), 0, 1)

	expected := []struct {
		raw        string
		offset     int64
		nextOffset int64
		line       uint64
	}{
		{"line1", 0, 6, 1},
		{"line2", 6, 12, 2},
		{"line3", 12, 18, 3},
	}

	for i, exp := range expected {
		raw, _, offset, nextOffset, line, _, issue, err := lr.next()
		if err != nil {
			t.Fatalf("line %d: unexpected error: %v", i+1, err)
		}
		if issue != nil {
			t.Fatalf("line %d: unexpected issue: %v", i+1, issue)
		}
		if string(raw) != exp.raw {
			t.Errorf("line %d: raw = %q, want %q", i+1, string(raw), exp.raw)
		}
		if offset != exp.offset {
			t.Errorf("line %d: offset = %d, want %d", i+1, offset, exp.offset)
		}
		if nextOffset != exp.nextOffset {
			t.Errorf("line %d: nextOffset = %d, want %d", i+1, nextOffset, exp.nextOffset)
		}
		if line != exp.line {
			t.Errorf("line %d: line = %d, want %d", i+1, line, exp.line)
		}
	}

	_, _, _, _, _, _, _, err := lr.next()
	if err != io.EOF {
		t.Errorf("expected io.EOF, got %v", err)
	}
}

func TestLineReader_CRLF(t *testing.T) {
	input := "line1\r\nline2\r\n"
	lr := newLineReader(strings.NewReader(input), 0, 1)

	raw, _, offset, nextOffset, line, _, _, err := lr.next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(raw) != "line1" {
		t.Errorf("raw = %q, want %q", string(raw), "line1")
	}
	if offset != 0 || nextOffset != 7 || line != 1 {
		t.Errorf("offset=%d nextOffset=%d line=%d, want 0, 7, 1", offset, nextOffset, line)
	}

	raw, _, offset, nextOffset, line, _, _, err = lr.next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(raw) != "line2" {
		t.Errorf("raw = %q, want %q", string(raw), "line2")
	}
	if offset != 7 || nextOffset != 14 || line != 2 {
		t.Errorf("offset=%d nextOffset=%d line=%d, want 7, 14, 2", offset, nextOffset, line)
	}
}

func TestLineReader_FinalLineNoTerminator(t *testing.T) {
	input := "line1\nline2"
	lr := newLineReader(strings.NewReader(input), 0, 1)

	raw, _, _, _, _, _, _, err := lr.next()
	if err != nil {
		t.Fatalf("line 1: unexpected error: %v", err)
	}
	if string(raw) != "line1" {
		t.Errorf("line 1: raw = %q, want %q", string(raw), "line1")
	}

	raw, _, offset, nextOffset, line, _, _, err := lr.next()
	if err != nil {
		t.Fatalf("line 2: unexpected error: %v", err)
	}
	if string(raw) != "line2" {
		t.Errorf("line 2: raw = %q, want %q", string(raw), "line2")
	}
	if offset != 6 || nextOffset != 11 || line != 2 {
		t.Errorf("offset=%d nextOffset=%d line=%d, want 6, 11, 2", offset, nextOffset, line)
	}

	_, _, _, _, _, _, _, err = lr.next()
	if err != io.EOF {
		t.Errorf("expected io.EOF, got %v", err)
	}
}

func TestLineReader_BlankLine(t *testing.T) {
	input := "line1\n\nline3\n"
	lr := newLineReader(strings.NewReader(input), 0, 1)

	raw, _, _, _, _, _, _, err := lr.next()
	if err != nil {
		t.Fatalf("line 1 error: %v", err)
	}
	if string(raw) != "line1" {
		t.Errorf("line 1 raw = %q, want %q", string(raw), "line1")
	}

	raw, _, offset, nextOffset, line, _, _, err := lr.next()
	if err != nil {
		t.Fatalf("line 2 error: %v", err)
	}
	if string(raw) != "" {
		t.Errorf("blank line raw = %q, want empty", string(raw))
	}
	if offset != 6 || nextOffset != 7 || line != 2 {
		t.Errorf("blank: offset=%d nextOffset=%d line=%d, want 6, 7, 2", offset, nextOffset, line)
	}

	raw, _, _, _, _, _, _, err = lr.next()
	if err != nil {
		t.Fatalf("line 3 error: %v", err)
	}
	if string(raw) != "line3" {
		t.Errorf("line 3 raw = %q, want %q", string(raw), "line3")
	}
}

func TestLineReader_OffsetAccuracy(t *testing.T) {
	input := "abc\ndefgh\ni\n"
	lr := newLineReader(strings.NewReader(input), 0, 1)

	type check struct {
		offset, nextOffset int64
	}
	expected := []check{
		{0, 4},   // "abc\n" = 4 bytes
		{4, 10},  // "defgh\n" = 6 bytes
		{10, 12}, // "i\n" = 2 bytes
	}

	for i, exp := range expected {
		_, _, offset, nextOffset, _, _, _, err := lr.next()
		if err != nil {
			t.Fatalf("line %d: %v", i+1, err)
		}
		if offset != exp.offset || nextOffset != exp.nextOffset {
			t.Errorf("line %d: offset=%d nextOffset=%d, want %d %d",
				i+1, offset, nextOffset, exp.offset, exp.nextOffset)
		}
	}
}

func TestLineReader_LineNumbering(t *testing.T) {
	input := "a\nb\nc\n"
	lr := newLineReader(strings.NewReader(input), 0, 1)

	for i := uint64(1); i <= 3; i++ {
		_, _, _, _, line, _, _, err := lr.next()
		if err != nil {
			t.Fatalf("line %d: %v", i, err)
		}
		if line != i {
			t.Errorf("line number = %d, want %d", line, i)
		}
	}
}

func TestLineReader_StartingOffset(t *testing.T) {
	input := "hello\n"
	lr := newLineReader(strings.NewReader(input), 100, 5)

	_, _, offset, nextOffset, line, _, _, err := lr.next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if offset != 100 || nextOffset != 106 || line != 5 {
		t.Errorf("offset=%d nextOffset=%d line=%d, want 100, 106, 5",
			offset, nextOffset, line)
	}
}

func TestRecordID_Determinism(t *testing.T) {
	srcID := SourceID("abc123")
	rawHash := sha256.Sum256([]byte("test line\n"))

	id1 := computeRecordID(srcID, 0, rawHash)
	id2 := computeRecordID(srcID, 0, rawHash)
	const want = RecordID("2fadb25b35e3b9220419a474bc23e23e201a37c05f4da2a1154725bdf10ed7d4")

	if id1 != id2 {
		t.Errorf("RecordID not deterministic: %q != %q", id1, id2)
	}
	if id1 != want {
		t.Errorf("RecordID = %q, want %q", id1, want)
	}
}

func TestRecordID_DifferentBytes(t *testing.T) {
	srcID := SourceID("abc123")
	hash1 := sha256.Sum256([]byte("line A\n"))
	hash2 := sha256.Sum256([]byte("line B\n"))

	id1 := computeRecordID(srcID, 0, hash1)
	id2 := computeRecordID(srcID, 0, hash2)

	if id1 == id2 {
		t.Errorf("different content produced same RecordID: %q", id1)
	}
}

func TestRecordID_DifferentOffset(t *testing.T) {
	srcID := SourceID("abc123")
	rawHash := sha256.Sum256([]byte("same\n"))

	id1 := computeRecordID(srcID, 0, rawHash)
	id2 := computeRecordID(srcID, 100, rawHash)

	if id1 == id2 {
		t.Errorf("different offsets produced same RecordID: %q", id1)
	}
}

func TestLineReader_RawHash(t *testing.T) {
	input := "hello\n"
	lr := newLineReader(strings.NewReader(input), 0, 1)

	_, rawHash, _, _, _, _, _, err := lr.next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedHash := sha256.Sum256([]byte("hello\n"))
	if rawHash != expectedHash {
		t.Errorf("rawHash = %s, want %s",
			hex.EncodeToString(rawHash[:]),
			hex.EncodeToString(expectedHash[:]))
	}
}

func TestLineReader_RawHashCRLF(t *testing.T) {
	input := "hello\r\n"
	lr := newLineReader(strings.NewReader(input), 0, 1)

	_, rawHash, _, _, _, _, _, err := lr.next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedHash := sha256.Sum256([]byte("hello\r\n"))
	if rawHash != expectedHash {
		t.Errorf("rawHash includes CRLF: got %s, want %s",
			hex.EncodeToString(rawHash[:]),
			hex.EncodeToString(expectedHash[:]))
	}
}

func TestLineReader_RawHashNoTerminator(t *testing.T) {
	input := "hello"
	lr := newLineReader(strings.NewReader(input), 0, 1)

	_, rawHash, _, _, _, _, _, err := lr.next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedHash := sha256.Sum256([]byte("hello"))
	if rawHash != expectedHash {
		t.Errorf("rawHash without terminator: got %s, want %s",
			hex.EncodeToString(rawHash[:]),
			hex.EncodeToString(expectedHash[:]))
	}
}

func TestLineReader_OversizedLine(t *testing.T) {
	bigLine := bytes.Repeat([]byte("X"), maxLineSize+100)
	input := append(bigLine, '\n')
	input = append(input, []byte("next line\n")...)

	lr := newLineReader(bytes.NewReader(input), 0, 1)

	raw, rawHash, offset, nextOffset, line, _, issue, err := lr.next()
	if err != nil {
		t.Fatalf("oversized line error: %v", err)
	}
	if issue == nil {
		t.Fatal("expected issue for oversized line")
	}
	if issue.Code != "line_too_long" {
		t.Errorf("issue code = %q, want %q", issue.Code, "line_too_long")
	}
	if offset != 0 {
		t.Errorf("offset = %d, want 0", offset)
	}
	expectedNextOffset := int64(maxLineSize + 100 + 1)
	if nextOffset != expectedNextOffset {
		t.Errorf("nextOffset = %d, want %d", nextOffset, expectedNextOffset)
	}
	if line != 1 {
		t.Errorf("line = %d, want 1", line)
	}
	if int64(len(raw)) > maxLineSize {
		t.Errorf("raw length = %d, should be <= %d", len(raw), maxLineSize)
	}

	// Verify hash covers ALL bytes including the discarded portion
	fullHash := sha256.Sum256(append(bigLine, '\n'))
	if rawHash != fullHash {
		t.Error("rawHash should cover all bytes including discarded portion")
	}

	// Next line should read correctly
	raw, _, offset, nextOffset, line, _, issue, err = lr.next()
	if err != nil {
		t.Fatalf("next line after oversized: error = %v", err)
	}
	if issue != nil {
		t.Errorf("unexpected issue on next line: %v", issue)
	}
	if string(raw) != "next line" {
		t.Errorf("next line raw = %q, want %q", string(raw), "next line")
	}
	if offset != expectedNextOffset {
		t.Errorf("next line offset = %d, want %d", offset, expectedNextOffset)
	}
	if line != 2 {
		t.Errorf("next line number = %d, want 2", line)
	}
	expectedFinalOffset := expectedNextOffset + int64(len("next line\n"))
	if nextOffset != expectedFinalOffset {
		t.Errorf("next line nextOffset = %d, want %d", nextOffset, expectedFinalOffset)
	}
}

func TestLineReader_InvalidUTF8(t *testing.T) {
	invalidBytes := []byte{0x80, 0x81, 0x82, '\n'}
	lr := newLineReader(bytes.NewReader(invalidBytes), 0, 1)

	raw, rawHash, _, _, _, _, _, err := lr.next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Hash must be computed on original bytes (including terminator)
	expectedHash := sha256.Sum256(invalidBytes)
	if rawHash != expectedHash {
		t.Error("rawHash should be computed on original bytes, not sanitized")
	}

	// Raw is the original bytes minus terminator (NOT yet sanitized - sanitization is done in read.go)
	if !bytes.Equal(raw, []byte{0x80, 0x81, 0x82}) {
		t.Errorf("raw = %v, want original bytes minus terminator", raw)
	}
}

func TestLineReader_EmptyInput(t *testing.T) {
	lr := newLineReader(strings.NewReader(""), 0, 1)

	_, _, _, _, _, _, _, err := lr.next()
	if err != io.EOF {
		t.Errorf("expected io.EOF for empty input, got %v", err)
	}
}
