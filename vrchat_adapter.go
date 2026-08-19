package vrclog

import (
	"net/url"
	"regexp"
	"strings"
	"unicode"
)

var (
	rePlayerJoined = regexp.MustCompile(`\[Behaviour\] OnPlayerJoined (.+?)(?:\s+\((usr_[a-f0-9-]+)\))?$`)
	rePlayerLeft   = regexp.MustCompile(`\[Behaviour\] OnPlayerLeft (.+?)(?:\s+\((usr_[a-f0-9-]+)\))?$`)
	reEnteringRoom = regexp.MustCompile(`\[Behaviour\] Entering Room: (.+)$`)
	reJoiningWorld = regexp.MustCompile(`\[Behaviour\] Joining (wrld_[a-f0-9-]+):(.+)$`)

	reVideoResolveAttempt = regexp.MustCompile(`\[Video Playback\] Attempting to resolve URL '([^']+)'`)
	reVideoResolved       = regexp.MustCompile(`\[Video Playback\] URL '([^']+)' resolved to '([^']+)'`)

	reVideoPlaybackError = regexp.MustCompile(`\[Video Playback\] ERROR: (.+)$`)
	reAVProError         = regexp.MustCompile(`\[AVProVideo\] Error: (.+)$`)
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

	hasBehaviour := strings.Contains(msg, "[Behaviour]")
	hasVideoPlayback := strings.Contains(msg, "[Video Playback]")
	hasAVPro := strings.Contains(msg, "[AVProVideo]")

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
						Target: &MediaTarget{Component: "vrchat"},
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
						Target: &MediaTarget{Component: "vrchat"},
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
					Message: m[1],
					Target:  &MediaTarget{Component: "vrchat"},
				},
			})
			return emissions, nil
		}
	}

	if hasAVPro {
		if strings.Contains(msg, avproOpeningPrefix) {
			idx := strings.Index(msg, avproOpeningPrefix)
			rest := msg[idx+len(avproOpeningPrefix):]

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
					Message: m[1],
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

func isHTTPURL(rawURL string) bool {
	if strings.ContainsFunc(rawURL, func(r rune) bool {
		return unicode.IsControl(r) || unicode.IsSpace(r)
	}) {
		return false
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	if u.User != nil {
		return false
	}
	return u.Host != ""
}
