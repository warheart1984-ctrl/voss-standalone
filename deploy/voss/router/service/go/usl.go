package main

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

type TenantPolicy struct {
	TenantID         string   `json:"tenant_id"`
	Name             string   `json:"name"`
	MLCALane         string   `json:"mlca_lane"`
	AllowedClasses   []string `json:"allowed_capability_classes"`
	AllowedScopes    []string `json:"allowed_scopes"`
	AllowedActions   []string `json:"allowed_actions"`
	AllowedRisks     []string `json:"allowed_risks"`
	RequiresApproval bool     `json:"requires_operator_approval"`
	IdentityNamespace string  `json:"identity_namespace"`
	SovereignOperator string  `json:"sovereign_operator"`
}

type CapabilityLattice struct {
	Grant struct {
		Class  string `json:"class"`
		Scope  string `json:"scope"`
		Action string `json:"action"`
		Risk   string `json:"risk"`
	} `json:"grant"`
	Lanes []string `json:"lanes"`
}

type USLGate struct {
	Tenants            []TenantPolicy
	Lattice            []CapabilityLattice
	GovernanceEnforced bool
}

func (u *USLGate) Check(req CapabilityRequest) AdmissionDecision {
	now := time.Now().UTC()
	// Lambda.7 first: governance supremacy
	if !u.GovernanceEnforced {
		MetricUSLViolations.Inc()
		return decision(Deny, "governance enforcement disabled", string(Lambda7), req.RequestID, now)
	}
	// Unknown tenant fails closed
	tenant, ok := u.findTenant(req.TenantID)
	if !ok {
		MetricUSLViolations.Inc()
		MetricIdentityViolations.Inc()
		return decision(Deny, "unknown tenant", string(Lambda4), req.RequestID, now)
	}
	// Lane mismatch fails closed
	if tenant.MLCALane != string(req.MLCALane) {
		MetricUSLViolations.Inc()
		MetricIdentityViolations.Inc()
		return decision(Deny, "mlca lane mismatch", string(Lambda4), req.RequestID, now)
	}
	// Capability lattice grants
	if !u.capabilityGranted(req.CapabilityClass, req.MLCALane) {
		MetricUSLViolations.Inc()
		return decision(Deny, "capability not granted in lattice", "capability.lattice", req.RequestID, now)
	}
	// Tenant policy allows class/scope/action/risk
	if !tenant.Allows(req.CapabilityClass) {
		MetricUSLViolations.Inc()
		return decision(Deny, "capability not allowed for tenant", "tenant.capability_policy", req.RequestID, now)
	}
	// Operator required for corrigibility
	if req.OperatorID == "" {
		MetricUSLViolations.Inc()
		return decision(Deny, "operator required", string(Lambda6), req.RequestID, now)
	}
	return decision(Admit, "admitted", "usl.gate", req.RequestID, now)
}

func decision(result AdmitResult, reason, ruleRef, requestID string, ts time.Time) AdmissionDecision {
	return AdmissionDecision{
		Result:     result,
		Reason:     reason,
		RuleRef:    ruleRef,
		DecisionID: decisionID(),
		RequestID:  requestID,
		Timestamp:  ts,
	}
}

func decisionID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (u *USLGate) findTenant(id string) (*TenantPolicy, bool) {
	for i := range u.Tenants {
		if u.Tenants[i].TenantID == id {
			return &u.Tenants[i], true
		}
	}
	return nil, false
}

func (u *USLGate) capabilityGranted(c CapabilityClass, lane MLCALane) bool {
	for _, g := range u.Lattice {
		if g.Grant.Class == c.Class &&
			g.Grant.Scope == c.Scope &&
			g.Grant.Action == c.Action &&
			g.Grant.Risk == c.Risk {
			for _, l := range g.Lanes {
				if l == string(lane) {
					return true
				}
			}
		}
	}
	return false
}

func (t *TenantPolicy) Allows(c CapabilityClass) bool {
	hasClass := contains(t.AllowedClasses, c.Class)
	hasScope := len(t.AllowedScopes) == 0 || contains(t.AllowedScopes, c.Scope)
	hasAction := len(t.AllowedActions) == 0 || contains(t.AllowedActions, c.Action)
	hasRisk := len(t.AllowedRisks) == 0 || contains(t.AllowedRisks, c.Risk)
	return hasClass && hasScope && hasAction && hasRisk
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}