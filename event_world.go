package vrclog

import "errors"

type World struct {
	ID         string `json:"id,omitempty"`
	Name       string `json:"name,omitempty"`
	InstanceID string `json:"instance_id,omitempty"`
}

type WorldEnteringObserved struct {
	World World `json:"world"`
}

func (e WorldEnteringObserved) Kind() EventKind { return EventKindWorldEnteringObserved }

func (e WorldEnteringObserved) validate() error {
	if e.World.Name == "" {
		return errors.New("world name is required for entering observation")
	}
	return nil
}

func (e WorldEnteringObserved) isEvent() {}

type WorldJoiningObserved struct {
	World World `json:"world"`
}

func (e WorldJoiningObserved) Kind() EventKind { return EventKindWorldJoiningObserved }

func (e WorldJoiningObserved) validate() error {
	if e.World.ID == "" {
		return errors.New("world ID is required for joining observation")
	}
	if e.World.InstanceID == "" {
		return errors.New("world instance_id is required for joining observation")
	}
	return nil
}

func (e WorldJoiningObserved) isEvent() {}
