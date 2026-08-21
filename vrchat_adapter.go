package vrclog

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	rePlayerJoined = regexp.MustCompile(`^\[Behaviour\] OnPlayerJoined (.+?)(?:\s+\((usr_[a-f0-9-]+)\))?$`)
	rePlayerLeft   = regexp.MustCompile(`^\[Behaviour\] OnPlayerLeft (.+?)(?:\s+\((usr_[a-f0-9-]+)\))?$`)
	reEnteringRoom = regexp.MustCompile(`^\[Behaviour\] Entering Room: (.+)$`)
	reJoiningWorld = regexp.MustCompile(`^\[Behaviour\] Joining (wrld_[a-f0-9-]+):(.+)$`)

	reVideoResolveAttempt = regexp.MustCompile(`^\[Video Playback\] Attempting to resolve URL '([^']+)'$`)
	reVideoResolved       = regexp.MustCompile(`^\[Video Playback\] URL '([^']+)' resolved to '([^']+)'$`)

	reVideoPlaybackError = regexp.MustCompile(`^\[Video Playback\] ERROR: (.+)$`)
	reAVProError         = regexp.MustCompile(`^\[AVProVideo\] Error: (.+)$`)

	// reHTTPURLInText matches an http(s) URL embedded in free-form error
	// text so it can be redacted before the text is placed into a
	// canonical Event. It stops at whitespace and common URL-adjacent
	// delimiters; it deliberately does not require a trailing boundary
	// beyond that, since VRChat error strings can end mid-token.
	reHTTPURLInText = regexp.MustCompile(`(?i)https?://[^[:space:]"'<>]+`)
)

var exclusionSubstrings = []string{
	"OnPlayerJoined:",
	"OnPlayerLeftRoom",
	"Joining or Creating",
	"Joining friend",
}

const avproOpeningPrefix = "[AVProVideo] Opening "
const avproOffsetMarker = " (offset "

type vrchatAdapter struct{}

func NewVRChatAdapter() Adapter {
	return vrchatAdapter{}
}

func (a vrchatAdapter) ID() AdapterID {
	return AdapterID("vrchat.core")
}

func (a vrchatAdapter) Decode(record Record) ([]Emission, error) {
	if record.Time.IsZero() {
		return nil, nil
	}

	msg := record.Message

	for _, ex := range exclusionSubstrings {
		if strings.Contains(msg, ex) {
			return nil, nil
		}
	}

	// Prefix checks anchor recognition to the start of the message so
	// an embedded log fragment (e.g. a third-party mod echoing another
	// tool's line inside its own bracketed tag) is never mistaken for a
	// genuine VRChat client line.
	hasBehaviour := strings.HasPrefix(msg, "[Behaviour]")
	hasVideoPlayback := strings.HasPrefix(msg, "[Video Playback]")
	hasAVPro := strings.HasPrefix(msg, "[AVProVideo]")

	if !hasBehaviour && !hasVideoPlayback && !hasAVPro {
		return nil, nil
	}

	var emissions []Emission

	if hasBehaviour {
		if m := rePlayerJoined.FindStringSubmatch(msg); m != nil {
			p := Player{DisplayName: m[1]}
			if m[2] != "" {
				p.ID = m[2]
			}
			emissions = append(emissions, Emission{
				Rule:  RuleID("player_joined"),
				Event: PlayerJoined{Player: p},
			})
			return emissions, nil
		}

		if m := rePlayerLeft.FindStringSubmatch(msg); m != nil {
			p := Player{DisplayName: m[1]}
			if m[2] != "" {
				p.ID = m[2]
			}
			emissions = append(emissions, Emission{
				Rule:  RuleID("player_left"),
				Event: PlayerLeft{Player: p},
			})
			return emissions, nil
		}

		if m := reEnteringRoom.FindStringSubmatch(msg); m != nil {
			emissions = append(emissions, Emission{
				Rule: RuleID("world_entering"),
				Event: WorldEnteringObserved{
					World: World{Name: strings.TrimSpace(m[1])},
				},
			})
			return emissions, nil
		}

		if m := reJoiningWorld.FindStringSubmatch(msg); m != nil {
			emissions = append(emissions, Emission{
				Rule: RuleID("world_joining"),
				Event: WorldJoiningObserved{
					World: World{
						ID:         m[1],
						InstanceID: m[2],
					},
				},
			})
			return emissions, nil
		}
	}

	if hasVideoPlayback {
		if m := reVideoResolveAttempt.FindStringSubmatch(msg); m != nil {
			url := m[1]
			if isHTTPURL(url) {
				emissions = append(emissions, Emission{
					Rule: RuleID("video_resolve_attempt"),
					Event: ResourceURLObserved{
						Resource: RemoteResource{
							URL:  url,
							Kind: ResourceKindVideo,
							Role: ResourceRoleResolverInput,
						},
						Target: &MediaTarget{Component: "vrchat", Backend: MediaBackendUnknown},
					},
				})
				return emissions, nil
			}
			return nil, nil
		}

		if m := reVideoResolved.FindStringSubmatch(msg); m != nil {
			inputURL := m[1]
			outputURL := m[2]
			if isHTTPURL(inputURL) && isHTTPURL(outputURL) {
				emissions = append(emissions, Emission{
					Rule: RuleID("video_resolved"),
					Event: ResourceResolved{
						Input: RemoteResource{
							URL:  inputURL,
							Kind: ResourceKindVideo,
							Role: ResourceRoleResolverInput,
						},
						Output: RemoteResource{
							URL:  outputURL,
							Kind: ResourceKindVideo,
							Role: ResourceRoleResolved,
						},
						Target: &MediaTarget{Component: "vrchat", Backend: MediaBackendUnknown},
					},
				})
				return emissions, nil
			}
			return nil, nil
		}

		if m := reVideoPlaybackError.FindStringSubmatch(msg); m != nil {
			emissions = append(emissions, Emission{
				Rule: RuleID("video_playback_error"),
				Event: MediaErrorObserved{
					Stage:   MediaStageResolve,
					Message: sanitizeErrorText(m[1]),
					Target:  &MediaTarget{Component: "vrchat", Backend: MediaBackendUnknown},
				},
			})
			return emissions, nil
		}
	}

	if hasAVPro {
		if strings.HasPrefix(msg, avproOpeningPrefix) {
			rest := msg[len(avproOpeningPrefix):]

			var url string
			if oi := strings.LastIndex(rest, avproOffsetMarker); oi >= 0 {
				url = rest[:oi]
			} else {
				url = strings.TrimRight(rest, " \t")
			}

			if isHTTPURL(url) {
				emissions = append(emissions, Emission{
					Rule: RuleID("avpro_open"),
					Event: ResourceURLObserved{
						Resource: RemoteResource{
							URL:  url,
							Kind: ResourceKindVideo,
							Role: ResourceRolePlaybackInput,
						},
						Target: &MediaTarget{
							Component: "vrchat",
							Backend:   MediaBackendAVPro,
						},
					},
				})
				return emissions, nil
			}
			return nil, nil
		}

		if m := reAVProError.FindStringSubmatch(msg); m != nil {
			emissions = append(emissions, Emission{
				Rule: RuleID("avpro_error"),
				Event: MediaErrorObserved{
					Stage:   MediaStageLoad,
					Message: sanitizeErrorText(m[1]),
					Target: &MediaTarget{
						Component: "vrchat",
						Backend:   MediaBackendAVPro,
					},
				},
			})
			return emissions, nil
		}
	}

	return nil, nil
}

// isHTTPURL reports whether rawURL is a safe, absolute http(s) URL. It
// delegates to validateHTTPURL, the same check canonical RemoteResource
// values are validated against, so a URL accepted here is guaranteed to
// also pass Event validation downstream.
func isHTTPURL(rawURL string) bool {
	return validateHTTPURL(rawURL) == nil
}

// sanitizeErrorText prepares free-form VRChat error text for inclusion
// in a canonical MediaErrorObserved.Message. It:
//  1. trims surrounding whitespace,
//  2. redacts any embedded http(s) URL (which may carry a signed access
//     token) before any character normalization touches it,
//  3. normalizes control characters and Unicode bidi formatting
//     characters to spaces,
//  4. redacts URLs a second time in case normalization exposed or
//     re-joined a URL that spanned a control character, and
//  5. truncates to a bounded byte length on a valid UTF-8 boundary.
//
// The double redaction pass matters: control-character normalization
// replaces unsafe runes with spaces, which could otherwise split a URL
// containing an embedded control character in a way the first redaction
// pass would only partially match.
func sanitizeErrorText(s string) string {
	s = strings.TrimSpace(s)
	s = redactHTTPURLs(s)
	s = normalizeUnsafeText(s)
	s = strings.TrimSpace(s)
	s = redactHTTPURLs(s)
	return truncateUTF8(s, maxMediaErrorMessageBytes)
}

func redactHTTPURLs(s string) string {
	return reHTTPURLInText.ReplaceAllString(s, "<url>")
}

func normalizeUnsafeText(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || isBidiFormat(r) {
			return ' '
		}
		return r
	}, s)
}

func truncateUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	b := maxBytes
	for b > 0 && !utf8.RuneStart(s[b]) {
		b--
	}
	return s[:b]
}
