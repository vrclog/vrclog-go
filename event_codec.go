package vrclog

import "encoding/json"

func EncodeEvent(event Event) (EventKind, json.RawMessage, error) {
	if event == nil {
		return "", nil, ErrUnknownEventKind
	}
	if err := event.validate(); err != nil {
		return "", nil, err
	}
	var (
		kind     EventKind
		data     []byte
		err      error
		mismatch bool
	)
	switch e := event.(type) {
	case PlayerJoined:
		kind = EventKindPlayerJoined
		mismatch = e.Kind() != kind
		data, err = json.Marshal(e)
	case PlayerLeft:
		kind = EventKindPlayerLeft
		mismatch = e.Kind() != kind
		data, err = json.Marshal(e)
	case WorldEnteringObserved:
		kind = EventKindWorldEnteringObserved
		mismatch = e.Kind() != kind
		data, err = json.Marshal(e)
	case WorldJoiningObserved:
		kind = EventKindWorldJoiningObserved
		mismatch = e.Kind() != kind
		data, err = json.Marshal(e)
	case ResourceURLObserved:
		kind = EventKindResourceURLObserved
		mismatch = e.Kind() != kind
		data, err = json.Marshal(e)
	case ResourceResolved:
		kind = EventKindResourceResolved
		mismatch = e.Kind() != kind
		data, err = json.Marshal(e)
	case MediaErrorObserved:
		kind = EventKindMediaErrorObserved
		mismatch = e.Kind() != kind
		data, err = json.Marshal(e)
	default:
		return "", nil, ErrUnknownEventKind
	}
	if mismatch {
		return "", nil, ErrEventKindMismatch
	}
	if err != nil {
		return "", nil, err
	}
	return kind, json.RawMessage(data), nil
}

func DecodeEvent(kind EventKind, payload []byte) (Event, error) {
	var (
		event Event
		err   error
	)
	switch kind {
	case EventKindPlayerJoined:
		var e PlayerJoined
		err = json.Unmarshal(payload, &e)
		event = e
	case EventKindPlayerLeft:
		var e PlayerLeft
		err = json.Unmarshal(payload, &e)
		event = e
	case EventKindWorldEnteringObserved:
		var e WorldEnteringObserved
		err = json.Unmarshal(payload, &e)
		event = e
	case EventKindWorldJoiningObserved:
		var e WorldJoiningObserved
		err = json.Unmarshal(payload, &e)
		event = e
	case EventKindResourceURLObserved:
		var e ResourceURLObserved
		err = json.Unmarshal(payload, &e)
		event = e
	case EventKindResourceResolved:
		var e ResourceResolved
		err = json.Unmarshal(payload, &e)
		event = e
	case EventKindMediaErrorObserved:
		var e MediaErrorObserved
		err = json.Unmarshal(payload, &e)
		event = e
	default:
		return nil, ErrUnknownEventKind
	}
	if err != nil {
		return nil, err
	}
	if vErr := event.validate(); vErr != nil {
		return nil, vErr
	}
	return event, nil
}
