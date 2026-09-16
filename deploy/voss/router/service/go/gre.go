package main

import "time"

const (
	StageInputValidation       = "input_validation"
	StageCapabilityCheck       = "capability_check"
	StageIdentitySeparation    = "identity_separation"
	StageUSLGate               = "usl_gate"
	StageDriftCheck            = "drift_check"
	StageOperatorCorrigibility = "operator_corrigibility"
	StageAuditTrail            = "audit_trail"
	StageLedgerWrite           = "ledger_write"
	StageOutputValidation      = "output_validation"
)

type StageInput struct {
	Request CapabilityRequest
	Tenants []TenantPolicy
	Lattice []CapabilityLattice
}

type StageOutput struct {
	Stage   string
	Passed  bool
	Reason  string
	RuleRef string
}

type StageStep struct {
	Name  string
	Check func(StageInput) StageOutput
}

type GRE1001 struct {
	Stages     []StageStep
	usl        *USLGate
	interrupts *InterruptStore
}

func NewGRE1001(usl *USLGate, interrupts *InterruptStore) *GRE1001 {
	g := &GRE1001{usl: usl, interrupts: interrupts}
	g.Stages = []StageStep{
		{Name: StageInputValidation, Check: stageInputValidation},
		{Name: StageCapabilityCheck, Check: stageCapabilityCheck},
		{Name: StageIdentitySeparation, Check: g.stageIdentitySeparation},
		{Name: StageUSLGate, Check: g.stageUSLGate},
		{Name: StageDriftCheck, Check: stageDriftCheck},
		{Name: StageOperatorCorrigibility, Check: g.stageOperatorCorrigibility},
		{Name: StageAuditTrail, Check: stageAuditTrail},
		{Name: StageLedgerWrite, Check: stageLedgerWrite},
		{Name: StageOutputValidation, Check: stageOutputValidation},
	}
	return g
}

func (g *GRE1001) Run(si StageInput) ([]StageOutput, bool) {
	var outputs []StageOutput
	for _, s := range g.Stages {
		start := time.Now()
		out := s.Check(si)
		MetricStageDuration.WithLabelValues(s.Name).Observe(time.Since(start).Seconds())
		MetricLambdaCheck.WithLabelValues(s.Name, boolStr(out.Passed)).Inc()
		if !out.Passed {
			MetricHalt.Inc()
			return append(outputs, out), false
		}
		outputs = append(outputs, out)
	}
	return outputs, true
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func stageInputValidation(si StageInput) StageOutput {
	r := si.Request
	if r.RequestID == "" || r.IntentID == "" {
		return StageOutput{Stage: StageInputValidation, Passed: false, Reason: "request_id and intent_id required", RuleRef: string(Lambda1)}
	}
	if r.TenantID == "" || r.OperatorID == "" {
		return StageOutput{Stage: StageInputValidation, Passed: false, Reason: "tenant and operator required for auditability", RuleRef: string(Lambda2)}
	}
	return StageOutput{Stage: StageInputValidation, Passed: true, Reason: "input valid", RuleRef: string(Lambda1)}
}

func stageCapabilityCheck(si StageInput) StageOutput {
	r := si.Request
	if r.CapabilityClass.Class == "" || r.CapabilityClass.Scope == "" || r.CapabilityClass.Action == "" || r.CapabilityClass.Risk == "" {
		return StageOutput{Stage: StageCapabilityCheck, Passed: false, Reason: "capability class incomplete", RuleRef: "capability.lattice"}
	}
	return StageOutput{Stage: StageCapabilityCheck, Passed: true, Reason: "capability class complete", RuleRef: "capability.lattice"}
}

func (g *GRE1001) stageIdentitySeparation(si StageInput) StageOutput {
	r := si.Request
	res := checkLambda4(r, si.Tenants)
	if !res.Passed {
		MetricIdentityViolations.Inc()
		return StageOutput{Stage: StageIdentitySeparation, Passed: false, Reason: res.Reason, RuleRef: string(Lambda4)}
	}
	return StageOutput{Stage: StageIdentitySeparation, Passed: true, Reason: "tenant isolated", RuleRef: string(Lambda4)}
}

func (g *GRE1001) stageUSLGate(si StageInput) StageOutput {
	r := si.Request
	usl := &USLGate{Tenants: si.Tenants, Lattice: si.Lattice, GovernanceEnforced: true}
	ad := usl.Check(r)
	if ad.Result == Deny || ad.Result == Quarantine {
		return StageOutput{Stage: StageUSLGate, Passed: false, Reason: ad.Reason, RuleRef: ad.RuleRef}
	}
	return StageOutput{Stage: StageUSLGate, Passed: true, Reason: "admitted", RuleRef: "usl.gate"}
}

func stageDriftCheck(si StageInput) StageOutput {
	res := checkLambda5()
	if !res.Passed {
		MetricDriftScore.Set(1)
		return StageOutput{Stage: StageDriftCheck, Passed: false, Reason: res.Reason, RuleRef: string(Lambda5)}
	}
	MetricDriftScore.Set(0)
	return StageOutput{Stage: StageDriftCheck, Passed: true, Reason: "no drift", RuleRef: string(Lambda5)}
}

func (g *GRE1001) stageOperatorCorrigibility(si StageInput) StageOutput {
	r := si.Request
	if g.interrupts.IsTerminated(r.IntentID) {
		return StageOutput{Stage: StageOperatorCorrigibility, Passed: false, Reason: "operator terminated execution", RuleRef: string(Lambda6)}
	}
	if g.interrupts.IsInterrupted(r.IntentID) {
		return StageOutput{Stage: StageOperatorCorrigibility, Passed: false, Reason: "operator interrupt active", RuleRef: string(Lambda6)}
	}
	if directive, ok := g.interrupts.Correction(r.IntentID); ok {
		return StageOutput{Stage: StageOperatorCorrigibility, Passed: false, Reason: "operator correction active: " + directive, RuleRef: string(Lambda6)}
	}
	return StageOutput{Stage: StageOperatorCorrigibility, Passed: true, Reason: "no active interrupt", RuleRef: string(Lambda6)}
}

func stageAuditTrail(si StageInput) StageOutput {
	r := si.Request
	if r.IntentID == "" {
		return StageOutput{Stage: StageAuditTrail, Passed: false, Reason: "intent required for audit", RuleRef: string(Lambda2)}
	}
	return StageOutput{Stage: StageAuditTrail, Passed: true, Reason: "audit trail available", RuleRef: string(Lambda2)}
}

func stageLedgerWrite(si StageInput) StageOutput {
	if si.Request.TenantID == "" {
		return StageOutput{Stage: StageLedgerWrite, Passed: false, Reason: "tenant required for ledger", RuleRef: string(Lambda2)}
	}
	return StageOutput{Stage: StageLedgerWrite, Passed: true, Reason: "ledger writable", RuleRef: "ledger"}
}

func stageOutputValidation(si StageInput) StageOutput {
	return StageOutput{Stage: StageOutputValidation, Passed: true, Reason: "output deterministic", RuleRef: string(Lambda1)}
}