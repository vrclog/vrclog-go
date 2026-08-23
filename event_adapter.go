package vrclog

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
)

const (
	maxAdapterTagBytes  = 128
	maxAdapterDataBytes = 64 * 1024
	maxAdapterDataDepth = 64

	// maxAdapterPayloadBytes bounds the full "tag"+"data" JSON envelope
	// accepted by DecodeEvent for EventKindAdapter, checked before
	// json.Unmarshal allocates anything. It must exceed maxAdapterDataBytes
	// (the Data field's own cap) to leave room for the tag, JSON
	// structural overhead, and field names.
	maxAdapterPayloadBytes = maxAdapterDataBytes + maxAdapterTagBytes + 1024
)

var adapterTagPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*(?:\.[a-z][a-z0-9_-]*)+\.v[1-9][0-9]*$`)

type AdapterEvent struct {
	Tag  string          `json:"tag"`
	Data json.RawMessage `json:"data"`
}

func (e AdapterEvent) Kind() EventKind { return EventKindAdapter }

func (e AdapterEvent) validate() error {
	if e.Tag == "" {
		return errors.New("adapter event tag is required")
	}
	if len(e.Tag) > maxAdapterTagBytes {
		return fmt.Errorf("adapter event tag exceeds %d bytes", maxAdapterTagBytes)
	}
	if containsUnsafeControlOrBidi(e.Tag) {
		return errors.New("adapter event tag contains control or bidi formatting characters")
	}
	if !adapterTagPattern.MatchString(e.Tag) {
		return fmt.Errorf("invalid adapter event tag: %q", e.Tag)
	}
	if len(e.Data) == 0 {
		return errors.New("adapter event data is required")
	}
	if len(e.Data) > maxAdapterDataBytes {
		return fmt.Errorf("adapter event data exceeds %d bytes", maxAdapterDataBytes)
	}

	// Validate the *decoded* JSON value in a single pass, not the raw
	// bytes: a raw-byte scan for unsafe characters would miss a JSON
	// escape sequence like ‮, which is inert ASCII in the raw form
	// but becomes an actual bidi override character once a downstream
	// consumer decodes it.
	decoder := json.NewDecoder(bytes.NewReader(e.Data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("adapter event data must be a JSON object: %w", err)
	}
	if decoder.More() {
		return errors.New("adapter event data must be a single JSON value")
	}
	object, ok := value.(map[string]any)
	if !ok || object == nil {
		return errors.New("adapter event data must be a JSON object")
	}
	if err := validateAdapterJSONValue(object, 1); err != nil {
		return fmt.Errorf("adapter event data: %w", err)
	}
	return nil
}

func (e AdapterEvent) isEvent() {}

// validateAdapterJSONValue walks a decoded JSON value (produced by
// json.Decoder with UseNumber) and rejects control/bidi formatting
// characters in decoded strings and object keys, while enforcing the
// nesting depth limit. depth is the depth of v itself (the initial
// top-level object is depth 1).
func validateAdapterJSONValue(v any, depth int) error {
	if depth > maxAdapterDataDepth {
		return fmt.Errorf("nesting depth exceeds %d", maxAdapterDataDepth)
	}
	switch x := v.(type) {
	case string:
		if containsUnsafeControlOrBidi(x) {
			return errors.New("contains control or bidi formatting characters")
		}
	case map[string]any:
		for k, child := range x {
			if containsUnsafeControlOrBidi(k) {
				return errors.New("object key contains control or bidi formatting characters")
			}
			if err := validateAdapterJSONValue(child, depth+1); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range x {
			if err := validateAdapterJSONValue(child, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}
