package vrclog

type Emission struct {
	Rule  RuleID
	Event Event
}

type Adapter interface {
	ID() AdapterID
	Decode(record Record) ([]Emission, error)
}
