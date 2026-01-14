package parser

import (
	"testing"
)

// BenchmarkParse_PlayerJoin benchmarks parsing a player join event.
func BenchmarkParse_PlayerJoin(b *testing.B) {
	line := "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerJoined TestUser"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Parse(line)
	}
}

// BenchmarkParse_PlayerLeft benchmarks parsing a player left event.
func BenchmarkParse_PlayerLeft(b *testing.B) {
	line := "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerLeft TestUser"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Parse(line)
	}
}

// BenchmarkParse_WorldJoin_EnteringRoom benchmarks parsing a world join event (Entering Room format).
func BenchmarkParse_WorldJoin_EnteringRoom(b *testing.B) {
	line := "2024.01.15 23:59:59 Log        -  [Behaviour] Entering Room: Test World Name"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Parse(line)
	}
}

// BenchmarkParse_WorldJoin_Joining benchmarks parsing a world join event (Joining wrld_ format).
func BenchmarkParse_WorldJoin_Joining(b *testing.B) {
	line := "2024.01.15 23:59:59 Log        -  Joining wrld_abc123-xyz"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Parse(line)
	}
}

// BenchmarkParse_NoMatch benchmarks parsing a line that doesn't match any pattern.
func BenchmarkParse_NoMatch(b *testing.B) {
	line := "2024.01.15 23:59:59 Log        -  Some other log message that doesn't match"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Parse(line)
	}
}

// BenchmarkParse_NoTimestamp benchmarks parsing a line without a timestamp.
func BenchmarkParse_NoTimestamp(b *testing.B) {
	line := "This is not a VRChat log line"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Parse(line)
	}
}

// BenchmarkParse_LongPlayerName benchmarks parsing with a very long player name.
func BenchmarkParse_LongPlayerName(b *testing.B) {
	longName := "ThisIsAVeryLongPlayerNameThatSomeoneDecidedToUseInVRChatForSomeReasonAndItKeepsGoingAndGoingAndGoing"
	line := "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerJoined " + longName

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Parse(line)
	}
}

// BenchmarkParse_LongWorldName benchmarks parsing with a very long world name.
func BenchmarkParse_LongWorldName(b *testing.B) {
	longWorld := "This Is A Very Long World Name With Lots Of Words And Spaces And Special Characters!@#$%"
	line := "2024.01.15 23:59:59 Log        -  [Behaviour] Entering Room: " + longWorld

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Parse(line)
	}
}

// BenchmarkParse_ExclusionPattern benchmarks parsing a line that matches an exclusion pattern.
func BenchmarkParse_ExclusionPattern(b *testing.B) {
	line := "2024.01.15 23:59:59 Log        -  Received Message of type: notification"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Parse(line)
	}
}

// BenchmarkParse_InvalidUTF8 benchmarks parsing with invalid UTF-8 sequences.
func BenchmarkParse_InvalidUTF8(b *testing.B) {
	line := "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerJoined \xff\xfe\xfd"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Parse(line)
	}
}
