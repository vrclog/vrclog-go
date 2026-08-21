package vrclog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

type RecordRef struct {
	ID       RecordID `json:"id"`
	SourceID SourceID `json:"source_id"`
	Offset   int64    `json:"offset"`
	Line     uint64   `json:"line"`
}

type Observation struct {
	ID        ObservationID `json:"id"`
	Time      time.Time     `json:"time"`
	AdapterID AdapterID     `json:"adapter_id"`
	RuleID    RuleID        `json:"rule_id"`
	Record    RecordRef     `json:"record"`
	Event     Event         `json:"-"`
}

type observationDTO struct {
	ID        ObservationID   `json:"id"`
	Time      time.Time       `json:"time"`
	AdapterID AdapterID       `json:"adapter_id"`
	RuleID    RuleID          `json:"rule_id"`
	Record    RecordRef       `json:"record"`
	Type      EventKind       `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

func EncodeObservationJSON(obs Observation) ([]byte, error) {
	kind, payload, err := EncodeEvent(obs.Event)
	if err != nil {
		return nil, err
	}
	dto := observationDTO{
		ID:        obs.ID,
		Time:      obs.Time,
		AdapterID: obs.AdapterID,
		RuleID:    obs.RuleID,
		Record:    obs.Record,
		Type:      kind,
		Payload:   payload,
	}
	return json.Marshal(dto)
}

func DecodeObservationJSON(data []byte) (Observation, error) {
	var dto observationDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		return Observation{}, err
	}
	event, err := DecodeEvent(dto.Type, dto.Payload)
	if err != nil {
		return Observation{}, err
	}
	return Observation{
		ID:        dto.ID,
		Time:      dto.Time,
		AdapterID: dto.AdapterID,
		RuleID:    dto.RuleID,
		Record:    dto.Record,
		Event:     event,
	}, nil
}

func generateObservationID(recordID RecordID, adapterID AdapterID, ruleID RuleID) ObservationID {
	h := sha256.New()
	h.Write([]byte(recordID))
	h.Write([]byte{0})
	h.Write([]byte(adapterID))
	h.Write([]byte{0})
	h.Write([]byte(ruleID))
	return ObservationID(hex.EncodeToString(h.Sum(nil)))
}
