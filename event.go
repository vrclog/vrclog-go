package vrclog

type Event interface {
	Kind() EventKind
	validate() error
	isEvent()
}

const (
	EventKindPlayerJoined          EventKind = "player.joined"
	EventKindPlayerLeft            EventKind = "player.left"
	EventKindWorldEnteringObserved EventKind = "world.entering_observed"
	EventKindWorldJoiningObserved  EventKind = "world.joining_observed"
	EventKindResourceURLObserved   EventKind = "resource.url_observed"
	EventKindResourceResolved      EventKind = "resource.resolved"
	EventKindMediaErrorObserved    EventKind = "media.error_observed"
)
