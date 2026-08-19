package vrclog

import (
	"testing"
	"time"
)

var testLoc = time.FixedZone("TEST", 0)

func TestDecodeHeader_Log(t *testing.T) {
	raw := "2026.08.18 12:00:00 Log        -  [Behaviour] Entering Room: Test World"
	ts, level, message, ok := decodeHeader(raw, testLoc)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if level != LevelLog {
		t.Errorf("level = %q, want %q", level, LevelLog)
	}
	expected := time.Date(2026, 8, 18, 12, 0, 0, 0, testLoc)
	if !ts.Equal(expected) {
		t.Errorf("time = %v, want %v", ts, expected)
	}
	if message != "[Behaviour] Entering Room: Test World" {
		t.Errorf("message = %q, want %q", message, "[Behaviour] Entering Room: Test World")
	}
}

func TestDecodeHeader_Warning(t *testing.T) {
	raw := "2026.08.18 12:00:00 Warning    -  something went wrong"
	_, level, message, ok := decodeHeader(raw, testLoc)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if level != LevelWarning {
		t.Errorf("level = %q, want %q", level, LevelWarning)
	}
	if message != "something went wrong" {
		t.Errorf("message = %q", message)
	}
}

func TestDecodeHeader_Error(t *testing.T) {
	raw := "2026.08.18 12:00:00 Error      -  fatal error occurred"
	_, level, _, ok := decodeHeader(raw, testLoc)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if level != LevelError {
		t.Errorf("level = %q, want %q", level, LevelError)
	}
}

func TestDecodeHeader_Debug(t *testing.T) {
	raw := "2026.08.18 12:00:00 Debug      -  debug info"
	_, level, _, ok := decodeHeader(raw, testLoc)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if level != LevelDebug {
		t.Errorf("level = %q, want %q", level, LevelDebug)
	}
}

func TestDecodeHeader_Exception(t *testing.T) {
	raw := "2026.08.18 12:00:00 Exception  -  stack trace here"
	_, level, _, ok := decodeHeader(raw, testLoc)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if level != LevelException {
		t.Errorf("level = %q, want %q", level, LevelException)
	}
}

func TestDecodeHeader_UnrecognizedLevel(t *testing.T) {
	raw := "2026.08.18 12:00:00 Trace      -  some trace"
	_, level, message, ok := decodeHeader(raw, testLoc)
	if !ok {
		t.Fatal("expected ok=true for unrecognized level")
	}
	if level != LevelUnknown {
		t.Errorf("level = %q, want %q", level, LevelUnknown)
	}
	if message != "some trace" {
		t.Errorf("message = %q, want %q", message, "some trace")
	}
}

func TestDecodeHeader_InvalidTimestamp(t *testing.T) {
	raw := "not-a-timestamp Log        -  message"
	_, _, _, ok := decodeHeader(raw, testLoc)
	if ok {
		t.Error("expected ok=false for invalid timestamp")
	}
}

func TestDecodeHeader_TooShort(t *testing.T) {
	raw := "short"
	_, _, _, ok := decodeHeader(raw, testLoc)
	if ok {
		t.Error("expected ok=false for too-short input")
	}
}

func TestDecodeHeader_NoSeparator(t *testing.T) {
	raw := "2026.08.18 12:00:00 Log no separator here"
	_, _, _, ok := decodeHeader(raw, testLoc)
	if ok {
		t.Error("expected ok=false when separator is missing")
	}
}

func TestDecodeHeader_MessageExtraction(t *testing.T) {
	raw := "2026.08.18 12:00:00 Log        -  [Video Playback] Attempting to resolve URL 'https://example.com/video.mp4'"
	_, _, message, ok := decodeHeader(raw, testLoc)
	if !ok {
		t.Fatal("expected ok=true")
	}
	want := "[Video Playback] Attempting to resolve URL 'https://example.com/video.mp4'"
	if message != want {
		t.Errorf("message = %q, want %q", message, want)
	}
}

func TestDecodeHeader_UnicodeMessage(t *testing.T) {
	raw := "2026.08.18 12:00:00 Log        -  [Behaviour] OnPlayerJoined テストユーザー"
	_, _, message, ok := decodeHeader(raw, testLoc)
	if !ok {
		t.Fatal("expected ok=true")
	}
	want := "[Behaviour] OnPlayerJoined テストユーザー"
	if message != want {
		t.Errorf("message = %q, want %q", message, want)
	}
}

func TestDecodeHeader_EmptyMessage(t *testing.T) {
	raw := "2026.08.18 12:00:00 Log        -  "
	_, level, message, ok := decodeHeader(raw, testLoc)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if level != LevelLog {
		t.Errorf("level = %q, want %q", level, LevelLog)
	}
	if message != "" {
		t.Errorf("message = %q, want empty", message)
	}
}

func TestDecodeHeader_NilLocation(t *testing.T) {
	raw := "2026.08.18 12:00:00 Log        -  msg"
	_, level, _, ok := decodeHeader(raw, nil)
	if !ok {
		t.Fatal("expected ok=true with nil location")
	}
	if level != LevelLog {
		t.Errorf("level = %q, want %q", level, LevelLog)
	}
}
