package vrclog

import (
	"bytes"
	"errors"
)

// detachEventForObservation normalizes an Emission's Event before it is
// validated and stored in an Observation. All eight canonical Event
// implementations use value receivers, so their pointer forms also satisfy
// the sealed Event interface: an Adapter returning e.g. &PlayerJoined{...}
// or &AdapterEvent{...} would otherwise bypass the codec's value-only type
// switch (becoming unencodable) and, for AdapterEvent, bypass the
// byte-slice detachment below. A typed-nil pointer (e.g.
// (*AdapterEvent)(nil)) also satisfies the interface's `== nil` check as
// false, so it must be rejected explicitly here instead of panicking later
// when a value-receiver method implicitly dereferences it.
func detachEventForObservation(event Event) (Event, error) {
	switch e := event.(type) {
	case PlayerJoined:
		return e, nil
	case *PlayerJoined:
		if e == nil {
			return nil, errors.New("nil player.joined event")
		}
		return *e, nil
	case PlayerLeft:
		return e, nil
	case *PlayerLeft:
		if e == nil {
			return nil, errors.New("nil player.left event")
		}
		return *e, nil
	case WorldEnteringObserved:
		return e, nil
	case *WorldEnteringObserved:
		if e == nil {
			return nil, errors.New("nil world.entering_observed event")
		}
		return *e, nil
	case WorldJoiningObserved:
		return e, nil
	case *WorldJoiningObserved:
		if e == nil {
			return nil, errors.New("nil world.joining_observed event")
		}
		return *e, nil
	case ResourceURLObserved:
		e.Target = cloneMediaTarget(e.Target)
		return e, nil
	case *ResourceURLObserved:
		if e == nil {
			return nil, errors.New("nil resource.url_observed event")
		}
		v := *e
		v.Target = cloneMediaTarget(v.Target)
		return v, nil
	case ResourceResolved:
		e.Target = cloneMediaTarget(e.Target)
		return e, nil
	case *ResourceResolved:
		if e == nil {
			return nil, errors.New("nil resource.resolved event")
		}
		v := *e
		v.Target = cloneMediaTarget(v.Target)
		return v, nil
	case MediaErrorObserved:
		e.Resource = cloneRemoteResource(e.Resource)
		e.Target = cloneMediaTarget(e.Target)
		return e, nil
	case *MediaErrorObserved:
		if e == nil {
			return nil, errors.New("nil media.error_observed event")
		}
		v := *e
		v.Resource = cloneRemoteResource(v.Resource)
		v.Target = cloneMediaTarget(v.Target)
		return v, nil
	case AdapterEvent:
		e.Data = bytes.Clone(e.Data)
		return e, nil
	case *AdapterEvent:
		if e == nil {
			return nil, errors.New("nil adapter.event event")
		}
		v := *e
		v.Data = bytes.Clone(v.Data)
		return v, nil
	default:
		return nil, ErrUnknownEventKind
	}
}

func cloneMediaTarget(t *MediaTarget) *MediaTarget {
	if t == nil {
		return nil
	}
	v := *t
	return &v
}

func cloneRemoteResource(r *RemoteResource) *RemoteResource {
	if r == nil {
		return nil
	}
	v := *r
	return &v
}
