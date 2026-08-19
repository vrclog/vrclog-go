package vrclog

import "errors"

type Player struct {
	ID          string `json:"id,omitempty"`
	DisplayName string `json:"display_name"`
}

type PlayerJoined struct {
	Player Player `json:"player"`
}

func (e PlayerJoined) Kind() EventKind { return EventKindPlayerJoined }

func (e PlayerJoined) validate() error {
	if e.Player.DisplayName == "" {
		return errors.New("player display_name is required")
	}
	return nil
}

func (e PlayerJoined) isEvent() {}

type PlayerLeft struct {
	Player Player `json:"player"`
}

func (e PlayerLeft) Kind() EventKind { return EventKindPlayerLeft }

func (e PlayerLeft) validate() error {
	if e.Player.DisplayName == "" {
		return errors.New("player display_name is required")
	}
	return nil
}

func (e PlayerLeft) isEvent() {}
