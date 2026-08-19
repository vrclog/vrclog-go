package vrclog

import (
	"testing"
	"time"
)

var fixedTime = time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)

func makeRecord(msg string) Record {
	return Record{Time: fixedTime, Message: msg}
}

func TestVRChatAdapterID(t *testing.T) {
	a := NewVRChatAdapter()
	if got := a.ID(); got != AdapterID("vrchat.core") {
		t.Fatalf("ID() = %q, want %q", got, "vrchat.core")
	}
}

func TestVRChatAdapterZeroTime(t *testing.T) {
	a := NewVRChatAdapter()
	r := Record{Message: "[Behaviour] OnPlayerJoined TestUser (usr_00000000-0000-0000-0000-000000000001)"}
	emissions, err := a.Decode(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if emissions != nil {
		t.Fatalf("expected nil emissions for zero-time record, got %d", len(emissions))
	}
}

func TestVRChatAdapterPlayerJoined(t *testing.T) {
	tests := []struct {
		name      string
		msg       string
		wantName  string
		wantID    string
		wantHasID bool
	}{
		{
			name:      "with usr_id",
			msg:       "[Behaviour] OnPlayerJoined TestUser (usr_00000000-0000-0000-0000-000000000001)",
			wantName:  "TestUser",
			wantID:    "usr_00000000-0000-0000-0000-000000000001",
			wantHasID: true,
		},
		{
			name:     "without usr_id",
			msg:      "[Behaviour] OnPlayerJoined SomePlayer",
			wantName: "SomePlayer",
		},
		{
			name:      "unicode japanese name",
			msg:       "[Behaviour] OnPlayerJoined 星野 アクア (usr_00000000-0000-0000-0000-000000000002)",
			wantName:  "星野 アクア",
			wantID:    "usr_00000000-0000-0000-0000-000000000002",
			wantHasID: true,
		},
		{
			name:      "name with spaces",
			msg:       "[Behaviour] OnPlayerJoined UNICO CHAAAAN (usr_00000000-0000-0000-0000-000000000003)",
			wantName:  "UNICO CHAAAAN",
			wantID:    "usr_00000000-0000-0000-0000-000000000003",
			wantHasID: true,
		},
	}

	a := NewVRChatAdapter()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			emissions, err := a.Decode(makeRecord(tc.msg))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(emissions) != 1 {
				t.Fatalf("expected 1 emission, got %d", len(emissions))
			}
			em := emissions[0]
			if em.Rule != RuleID("player_joined") {
				t.Errorf("rule = %q, want %q", em.Rule, "player_joined")
			}
			if em.Event.Kind() != EventKindPlayerJoined {
				t.Errorf("kind = %q, want %q", em.Event.Kind(), EventKindPlayerJoined)
			}
			pj := em.Event.(PlayerJoined)
			if pj.Player.DisplayName != tc.wantName {
				t.Errorf("display name = %q, want %q", pj.Player.DisplayName, tc.wantName)
			}
			if tc.wantHasID {
				if pj.Player.ID != tc.wantID {
					t.Errorf("player ID = %q, want %q", pj.Player.ID, tc.wantID)
				}
			} else {
				if pj.Player.ID != "" {
					t.Errorf("expected empty player ID, got %q", pj.Player.ID)
				}
			}
		})
	}
}

func TestVRChatAdapterPlayerLeft(t *testing.T) {
	tests := []struct {
		name      string
		msg       string
		wantName  string
		wantID    string
		wantHasID bool
	}{
		{
			name:     "without usr_id",
			msg:      "[Behaviour] OnPlayerLeft TestUser",
			wantName: "TestUser",
		},
		{
			name:      "with usr_id",
			msg:       "[Behaviour] OnPlayerLeft 星野 アクア (usr_00000000-0000-0000-0000-000000000002)",
			wantName:  "星野 アクア",
			wantID:    "usr_00000000-0000-0000-0000-000000000002",
			wantHasID: true,
		},
	}

	a := NewVRChatAdapter()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			emissions, err := a.Decode(makeRecord(tc.msg))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(emissions) != 1 {
				t.Fatalf("expected 1 emission, got %d", len(emissions))
			}
			em := emissions[0]
			if em.Rule != RuleID("player_left") {
				t.Errorf("rule = %q, want %q", em.Rule, "player_left")
			}
			if em.Event.Kind() != EventKindPlayerLeft {
				t.Errorf("kind = %q, want %q", em.Event.Kind(), EventKindPlayerLeft)
			}
			pl := em.Event.(PlayerLeft)
			if pl.Player.DisplayName != tc.wantName {
				t.Errorf("display name = %q, want %q", pl.Player.DisplayName, tc.wantName)
			}
			if tc.wantHasID {
				if pl.Player.ID != tc.wantID {
					t.Errorf("player ID = %q, want %q", pl.Player.ID, tc.wantID)
				}
			} else {
				if pl.Player.ID != "" {
					t.Errorf("expected empty player ID, got %q", pl.Player.ID)
				}
			}
		})
	}
}

func TestVRChatAdapterWorldEntering(t *testing.T) {
	a := NewVRChatAdapter()
	emissions, err := a.Decode(makeRecord("[Behaviour] Entering Room: Lake Side House"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emissions) != 1 {
		t.Fatalf("expected 1 emission, got %d", len(emissions))
	}
	em := emissions[0]
	if em.Rule != RuleID("world_entering") {
		t.Errorf("rule = %q, want %q", em.Rule, "world_entering")
	}
	we := em.Event.(WorldEnteringObserved)
	if we.World.Name != "Lake Side House" {
		t.Errorf("world name = %q, want %q", we.World.Name, "Lake Side House")
	}
}

func TestVRChatAdapterWorldJoining(t *testing.T) {
	tests := []struct {
		name           string
		msg            string
		wantWorldID    string
		wantInstanceID string
	}{
		{
			name:           "private instance",
			msg:            "[Behaviour] Joining wrld_00000000-0000-0000-0000-000000000001:28010~private(usr_00000000-0000-0000-0000-000000000001)~region(jp)",
			wantWorldID:    "wrld_00000000-0000-0000-0000-000000000001",
			wantInstanceID: "28010~private(usr_00000000-0000-0000-0000-000000000001)~region(jp)",
		},
		{
			name:           "hidden instance",
			msg:            "[Behaviour] Joining wrld_00000000-0000-0000-0000-000000000002:58591~hidden(usr_00000000-0000-0000-0000-000000000003)~region(jp)",
			wantWorldID:    "wrld_00000000-0000-0000-0000-000000000002",
			wantInstanceID: "58591~hidden(usr_00000000-0000-0000-0000-000000000003)~region(jp)",
		},
		{
			name:           "hex token instance",
			msg:            "[Behaviour] Joining wrld_00000000-0000-0000-0000-000000000003:2433ee0749~region(jp)",
			wantWorldID:    "wrld_00000000-0000-0000-0000-000000000003",
			wantInstanceID: "2433ee0749~region(jp)",
		},
		{
			name:           "group with groupAccessType multi-tag",
			msg:            "[Behaviour] Joining wrld_00000000-0000-0000-0000-000000000004:61081~group(grp_00000000-0000-0000-0000-000000000001)~groupAccessType(public)~region(jp)",
			wantWorldID:    "wrld_00000000-0000-0000-0000-000000000004",
			wantInstanceID: "61081~group(grp_00000000-0000-0000-0000-000000000001)~groupAccessType(public)~region(jp)",
		},
	}

	a := NewVRChatAdapter()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			emissions, err := a.Decode(makeRecord(tc.msg))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(emissions) != 1 {
				t.Fatalf("expected 1 emission, got %d", len(emissions))
			}
			em := emissions[0]
			if em.Rule != RuleID("world_joining") {
				t.Errorf("rule = %q, want %q", em.Rule, "world_joining")
			}
			wj := em.Event.(WorldJoiningObserved)
			if wj.World.ID != tc.wantWorldID {
				t.Errorf("world ID = %q, want %q", wj.World.ID, tc.wantWorldID)
			}
			if wj.World.InstanceID != tc.wantInstanceID {
				t.Errorf("instance ID = %q, want %q", wj.World.InstanceID, tc.wantInstanceID)
			}
		})
	}
}

func TestVRChatAdapterVideoResolveAttempt(t *testing.T) {
	a := NewVRChatAdapter()
	emissions, err := a.Decode(makeRecord("[Video Playback] Attempting to resolve URL 'https://youtu.be/FAKEVIDEOID1'"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emissions) != 1 {
		t.Fatalf("expected 1 emission, got %d", len(emissions))
	}
	em := emissions[0]
	if em.Rule != RuleID("video_resolve_attempt") {
		t.Errorf("rule = %q, want %q", em.Rule, "video_resolve_attempt")
	}
	ro := em.Event.(ResourceURLObserved)
	if ro.Resource.URL != "https://youtu.be/FAKEVIDEOID1" {
		t.Errorf("url = %q", ro.Resource.URL)
	}
	if ro.Resource.Kind != ResourceKindVideo {
		t.Errorf("kind = %q", ro.Resource.Kind)
	}
	if ro.Resource.Role != ResourceRoleResolverInput {
		t.Errorf("role = %q", ro.Resource.Role)
	}
	if ro.Target == nil || ro.Target.Component != "vrchat" {
		t.Errorf("target = %+v", ro.Target)
	}
}

func TestVRChatAdapterVideoResolveAttemptNonHTTP(t *testing.T) {
	a := NewVRChatAdapter()
	emissions, err := a.Decode(makeRecord("[Video Playback] Attempting to resolve URL 'ftp://example.invalid/video.mp4'"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if emissions != nil {
		t.Fatalf("expected nil emissions for non-http URL, got %d", len(emissions))
	}
}

func TestVRChatAdapterVideoResolved(t *testing.T) {
	a := NewVRChatAdapter()
	msg := "[Video Playback] URL 'https://youtu.be/FAKEVIDEOID1' resolved to 'https://rr3---sn-example.example.invalid/videoplayback?expire=1234567890&ei=FAKETOKEN&itag=18&source=youtube'"
	emissions, err := a.Decode(makeRecord(msg))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emissions) != 1 {
		t.Fatalf("expected 1 emission, got %d", len(emissions))
	}
	em := emissions[0]
	if em.Rule != RuleID("video_resolved") {
		t.Errorf("rule = %q, want %q", em.Rule, "video_resolved")
	}
	rr := em.Event.(ResourceResolved)
	if rr.Input.URL != "https://youtu.be/FAKEVIDEOID1" {
		t.Errorf("input url = %q", rr.Input.URL)
	}
	if rr.Input.Role != ResourceRoleResolverInput {
		t.Errorf("input role = %q", rr.Input.Role)
	}
	if rr.Output.URL != "https://rr3---sn-example.example.invalid/videoplayback?expire=1234567890&ei=FAKETOKEN&itag=18&source=youtube" {
		t.Errorf("output url = %q", rr.Output.URL)
	}
	if rr.Output.Role != ResourceRoleResolved {
		t.Errorf("output role = %q", rr.Output.Role)
	}
	if rr.Target == nil || rr.Target.Component != "vrchat" {
		t.Errorf("target = %+v", rr.Target)
	}
}

func TestVRChatAdapterAVProOpenWithOffset(t *testing.T) {
	a := NewVRChatAdapter()
	msg := "[AVProVideo] Opening https://rr3---sn-example.example.invalid/videoplayback?expire=1234567890&ei=FAKETOKEN&itag=18 (offset 0) with API MediaFoundation"
	emissions, err := a.Decode(makeRecord(msg))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emissions) != 1 {
		t.Fatalf("expected 1 emission, got %d", len(emissions))
	}
	em := emissions[0]
	if em.Rule != RuleID("avpro_open") {
		t.Errorf("rule = %q, want %q", em.Rule, "avpro_open")
	}
	ro := em.Event.(ResourceURLObserved)
	if ro.Resource.URL != "https://rr3---sn-example.example.invalid/videoplayback?expire=1234567890&ei=FAKETOKEN&itag=18" {
		t.Errorf("url = %q", ro.Resource.URL)
	}
	if ro.Resource.Role != ResourceRolePlaybackInput {
		t.Errorf("role = %q", ro.Resource.Role)
	}
	if ro.Target == nil || ro.Target.Backend != MediaBackendAVPro {
		t.Errorf("target = %+v", ro.Target)
	}
}

func TestVRChatAdapterAVProOpenBareForm(t *testing.T) {
	a := NewVRChatAdapter()
	msg := "[AVProVideo] Opening https://www.youtube.com/watch?v=FAKEVIDEOID2"
	emissions, err := a.Decode(makeRecord(msg))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emissions) != 1 {
		t.Fatalf("expected 1 emission, got %d", len(emissions))
	}
	em := emissions[0]
	if em.Rule != RuleID("avpro_open") {
		t.Errorf("rule = %q, want %q", em.Rule, "avpro_open")
	}
	ro := em.Event.(ResourceURLObserved)
	if ro.Resource.URL != "https://www.youtube.com/watch?v=FAKEVIDEOID2" {
		t.Errorf("url = %q", ro.Resource.URL)
	}
}

func TestVRChatAdapterVideoPlaybackError(t *testing.T) {
	a := NewVRChatAdapter()
	msg := "[Video Playback] ERROR: [generic] uc?export=view&id=FAKE_ID: Unable to download webpage: HTTP Error 404: Not Found (caused by <HTTPError 404: Not Found>)"
	emissions, err := a.Decode(makeRecord(msg))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emissions) != 1 {
		t.Fatalf("expected 1 emission, got %d", len(emissions))
	}
	em := emissions[0]
	if em.Rule != RuleID("video_playback_error") {
		t.Errorf("rule = %q, want %q", em.Rule, "video_playback_error")
	}
	me := em.Event.(MediaErrorObserved)
	if me.Stage != MediaStageResolve {
		t.Errorf("stage = %q, want %q", me.Stage, MediaStageResolve)
	}
	wantMsg := "[generic] uc?export=view&id=FAKE_ID: Unable to download webpage: HTTP Error 404: Not Found (caused by <HTTPError 404: Not Found>)"
	if me.Message != wantMsg {
		t.Errorf("message = %q, want %q", me.Message, wantMsg)
	}
	if me.Target == nil || me.Target.Component != "vrchat" {
		t.Errorf("target = %+v", me.Target)
	}
}

func TestVRChatAdapterAVProErrorDoubleSpace(t *testing.T) {
	a := NewVRChatAdapter()
	msg := "[AVProVideo] Error: Loading failed.  File not found, codec not supported, video resolution too high or insufficient system resources."
	emissions, err := a.Decode(makeRecord(msg))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emissions) != 1 {
		t.Fatalf("expected 1 emission, got %d", len(emissions))
	}
	em := emissions[0]
	if em.Rule != RuleID("avpro_error") {
		t.Errorf("rule = %q, want %q", em.Rule, "avpro_error")
	}
	me := em.Event.(MediaErrorObserved)
	if me.Stage != MediaStageLoad {
		t.Errorf("stage = %q, want %q", me.Stage, MediaStageLoad)
	}
	wantMsg := "Loading failed.  File not found, codec not supported, video resolution too high or insufficient system resources."
	if me.Message != wantMsg {
		t.Errorf("message = %q, want %q", me.Message, wantMsg)
	}
	if me.Target == nil || me.Target.Backend != MediaBackendAVPro {
		t.Errorf("target backend = %+v", me.Target)
	}
}

func TestVRChatAdapterNegativeCorpus(t *testing.T) {
	negatives := []struct {
		name string
		msg  string
	}{
		{"protv_color_tag", `[<color=#1F84A9>A</color><color=#A3A3A3>T</color> <color=#c0c0c0>INFO</color>    	<color=#00ff00>TVManager (Simple (ProTV))</color>] [AVProHQ] loading URL by user 'TestUser': https://youtu.be/FAKEVIDEOID1`},
		{"avpro_no_mediareference", "[AVProVideo] No MediaReference specified"},
		{"avpro_no_filepath", "[AVProVideo] No file path specified"},
		{"avpro_using_playback_path", "[AVProVideo] Using playback path: MF-MediaEngine-Hardware (640x360@24.00)"},
		{"avpro_shutdown", "[AVProVideo] Shutdown"},
		{"avpro_init", "[AVProVideo] Initialising AVPro Video v3.3.6 (native plugin v3.2.6f1-ultra) on NVIDIA GeForce RTX 5080/Direct3D 11.0 [level 11.1] (MT False) on WindowsPlayer"},
		{"avpro_movie_capture", "[AVProMovieCapture] Init version: 5.0.5 (plugin v5.0.0f1-ultra) with GPU NVIDIA GeForce RTX 5080 Direct3D 11.0 [level 11.1] OS: Windows 11  (10.0.26200) 64bit"},
		{"exclusion_joined_colon", "[Behaviour] OnPlayerJoined: TestUser logged in"},
		{"exclusion_left_room", "[Behaviour] OnPlayerLeftRoom"},
		{"exclusion_joining_or_creating", "[Behaviour] Joining or Creating Room: Some World"},
		{"exclusion_joining_friend", "[Behaviour] Joining friend SomeFriend in instance"},
		{"non_http_resolve", "[Video Playback] Attempting to resolve URL 'ftp://example.invalid/video.mp4'"},
		{"yamaplayer", "[YamaPlayer] Loading URL: https://youtu.be/FAKEVIDEOID1"},
		{"iwasync3", "[iwaSync3] Playback started: https://youtu.be/FAKEVIDEOID1"},
		{"command_line_url", `C:\Program Files\VRChat\VRChat.exe --url=https://example.invalid/launch`},
		{"empty_message", ""},
		{"random_debug", "Some random debug output with no brackets"},
		{"udon_behaviour", "[UdonBehaviour] Something happened"},
	}

	a := NewVRChatAdapter()
	for _, tc := range negatives {
		t.Run(tc.name, func(t *testing.T) {
			emissions, err := a.Decode(makeRecord(tc.msg))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(emissions) != 0 {
				t.Errorf("expected no emissions for %q, got %d: rule=%q kind=%q",
					tc.name, len(emissions), emissions[0].Rule, emissions[0].Event.Kind())
			}
		})
	}
}

func TestVRChatAdapterInstanceIDFullString(t *testing.T) {
	a := NewVRChatAdapter()
	msg := "[Behaviour] Joining wrld_00000000-0000-0000-0000-000000000004:61081~group(grp_00000000-0000-0000-0000-000000000001)~groupAccessType(public)~region(jp)"
	emissions, err := a.Decode(makeRecord(msg))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emissions) != 1 {
		t.Fatalf("expected 1 emission, got %d", len(emissions))
	}
	wj := emissions[0].Event.(WorldJoiningObserved)
	want := "61081~group(grp_00000000-0000-0000-0000-000000000001)~groupAccessType(public)~region(jp)"
	if wj.World.InstanceID != want {
		t.Errorf("instance ID = %q, want %q", wj.World.InstanceID, want)
	}
}
