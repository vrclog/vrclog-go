package vrclog

type DiagnosticCode string

const (
	DiagnosticRecordIssue          DiagnosticCode = "record_issue"
	DiagnosticAdapterError         DiagnosticCode = "adapter_error"
	DiagnosticInvalidAdapterResult DiagnosticCode = "invalid_adapter_result"
	DiagnosticInvalidRuleID        DiagnosticCode = "invalid_rule_id"
	DiagnosticInvalidEvent         DiagnosticCode = "invalid_event"
	DiagnosticDuplicateRuleID      DiagnosticCode = "duplicate_rule_id"
)

type Diagnostic struct {
	Code      DiagnosticCode `json:"code"`
	Message   string         `json:"message"`
	AdapterID AdapterID      `json:"adapter_id,omitempty"`
	RuleID    RuleID         `json:"rule_id,omitempty"`
	Record    RecordRef      `json:"record"`
	Err       error          `json:"-"`
}
