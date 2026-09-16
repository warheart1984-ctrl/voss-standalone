package main

import (
	"encoding/json"
	"time"
)

type AdmitResult string

const (
	Admit   AdmitResult = "ADMIT"
	Deny    AdmitResult = "DENY"
	Quarantine AdmitResult = "QUARANTINE"
)

type ImmuneClass string

const (
	ImmuneAllow      ImmuneClass = "ALLOW"
	ImmuneClamp      ImmuneClass = "CLAMP"
	ImmuneReroute    ImmuneClass = "REROUTE"
	ImmuneReject     ImmuneClass = "REJECT"
	ImmuneQuarantine ImmuneClass = "QUARANTINE"
)

type MLCALane string

const (
	LaneSafe    MLCALane = "SAFE"
	LaneNormal  MLCALane = "NORMAL"
	LaneExpress MLCALane = "EXPRESS"
)

type CapabilityClass struct {
	Class  string `json:"class"`
	Scope  string `json:"scope"`
	Action string `json:"action"`
	Risk   string `json:"risk"`
}

type CapabilityRequest struct {
	RequestID       string            `json:"request_id"`
	TenantID        string            `json:"tenant_id"`
	MLCALane        MLCALane          `json:"mlca_lane"`
	CapabilityClass CapabilityClass   `json:"capability_class"`
	IntentID        string            `json:"intent_id"`
	OperatorID      string            `json:"operator_id"`
	ModelRef        ModelRef          `json:"model_ref"`
	Payload         json.RawMessage   `json:"payload,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

type ModelRef struct {
	ProviderID string `json:"provider_id"`
	ModelID    string `json:"model_id"`
	KeyRef     string `json:"key_ref,omitempty"`
}

type AdmissionDecision struct {
	Result     AdmitResult `json:"result"`
	Reason     string      `json:"reason"`
	RuleRef    string      `json:"rule_ref"`
	DecisionID string      `json:"decision_id"`
	RequestID  string      `json:"request_id"`
	Timestamp  time.Time   `json:"timestamp"`
}

type LedgerEntry struct {
	Index      uint64            `json:"index"`
	Timestamp  time.Time         `json:"timestamp"`
	RequestID  string            `json:"request_id"`
	IntentID   string            `json:"intent_id"`
	TenantID   string            `json:"tenant_id"`
	MLCALane   MLCALane          `json:"mlca_lane"`
	Capability CapabilityClass   `json:"capability"`
	Provider   string            `json:"provider"`
	Admitted   bool              `json:"admitted"`
	Result     AdmitResult       `json:"result"`
	Reason     string            `json:"reason"`
	RuleRef    string            `json:"rule_ref"`
	DecisionID string            `json:"decision_id"`
	StageLog   []StageRecord     `json:"stage_log"`
	PrevHash   string            `json:"prev_hash"`
	Hash       string            `json:"hash"`
	ReplayID   string            `json:"replay_id,omitempty"`
}

type StageRecord struct {
	Stage   string `json:"stage"`
	Passed  bool   `json:"passed"`
	Reason  string `json:"reason"`
	RuleRef string `json:"rule_ref"`
}

type CER struct {
	Version          string        `json:"version"`
	DecisionID       string        `json:"decision_id"`
	RequestID        string        `json:"request_id"`
	IntentID         string        `json:"intent_id"`
	TenantID         string        `json:"tenant_id"`
	RuntimeIdentity  string        `json:"runtime_identity"`
	InputHash        string        `json:"input_hash"`
	EvidenceArtifacts []string     `json:"evidence_artifacts"`
	VerificationChecks []StageRecord `json:"verification_checks"`
	ReplayMarker     string        `json:"replay_marker"`
	Lineage          []LedgerEntry `json:"lineage"`
	Timestamp        time.Time     `json:"timestamp"`
}

type LambdaLaw string

const (
	Lambda1 LambdaLaw = "Lambda.1"
	Lambda2 LambdaLaw = "Lambda.2"
	Lambda3 LambdaLaw = "Lambda.3"
	Lambda4 LambdaLaw = "Lambda.4"
	Lambda5 LambdaLaw = "Lambda.5"
	Lambda6 LambdaLaw = "Lambda.6"
	Lambda7 LambdaLaw = "Lambda.7"
)

type LambdaResult struct {
	Law    LambdaLaw `json:"law"`
	Passed bool      `json:"passed"`
	Reason string    `json:"reason"`
}
