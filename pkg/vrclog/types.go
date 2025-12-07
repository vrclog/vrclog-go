package vrclog

import "github.com/vrclog/vrclog-go/pkg/vrclog/event"

// Re-export event types for convenience.
// Users can import just "github.com/vrclog/vrclog-go/pkg/vrclog"
// and use vrclog.Event, vrclog.EventPlayerJoin, etc.

// Event represents a parsed VRChat log event.
type Event = event.Event

// EventType represents the type of VRChat log event.
type EventType = event.Type

// Event type constants.
const (
	EventWorldJoin  = event.WorldJoin
	EventPlayerJoin = event.PlayerJoin
	EventPlayerLeft = event.PlayerLeft
)
