package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
)

type Config struct {
	Version  string            `json:"version"`
	Tenants  []LegacyTenant    `json:"tenants"`
	MLCALanes map[string]struct {
		Description string `json:"description"`
	} `json:"mlca_lanes"`
	Governance struct {
		VossBinding     []string `json:"voss_binding"`
		USLGateEnforced bool     `json:"usl_gate_enforced"`
		LedgerRequired  bool     `json:"ledger_required"`
	} `json:"governance"`
}

type LegacyTenant struct {
	TenantID string `json:"tenant_id"`
	Name     string `json:"name"`
	MLCALane string `json:"mlca_lane"`
	CapabilityPolicy struct {
		AllowedCapabilityClasses []string `json:"allowed_capability_classes"`
		RequiresOperatorApproval bool     `json:"requires_operator_approval"`
		IdentitySeparation       bool     `json:"identity_separation"`
	} `json:"capability_policy"`
	ModelRouting struct {
		Policy          string            `json:"policy"`
		AllowedProviders []string          `json:"allowed_providers"`
		KeyRefs          map[string]string `json:"key_refs"`
	} `json:"model_routing"`
	OperatorSovereignty struct {
		OperatorIDs              []string `json:"operator_ids"`
		CorrigibilityUnconditional bool    `json:"corrigibility_unconditional"`
	} `json:"operator_sovereignty"`
}

func LoadConfigFromEnv() Config {
	path := os.Getenv("VOSS_CONFIG")
	if path == "" {
		path = "deploy/voss/config/tenants.production.json"
	}
	return LoadConfig(path)
}

func LoadConfig(path string) Config {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	// Strip UTF-8 BOM if present; JSON requires BOM-free UTF-8.
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg
	}
	return cfg
}

func (c Config) Build() ([]TenantPolicy, []CapabilityLattice) {
	policies := make([]TenantPolicy, 0, len(c.Tenants))
	lattice := make([]CapabilityLattice, 0)
	for _, t := range c.Tenants {
		allowed := make([]string, 0, len(t.CapabilityPolicy.AllowedCapabilityClasses))
		allowedScopes := []string{}
		allowedActions := []string{}
		allowedRisks := []string{}
		for _, cl := range t.CapabilityPolicy.AllowedCapabilityClasses {
			parts := strings.Split(cl, ".")
			if len(parts) >= 2 {
				allowed = append(allowed, parts[0])
				if !contains(allowedScopes, parts[1]) {
					allowedScopes = append(allowedScopes, parts[1])
				}
				if len(parts) >= 3 && !contains(allowedActions, parts[2]) {
					allowedActions = append(allowedActions, parts[2])
				}
			} else {
				allowed = append(allowed, cl)
			}
		}
		if len(allowedActions) == 0 {
			allowedActions = []string{"propose", "execute", "preview", "verify", "apply"}
		}
		op := ""
		if len(t.OperatorSovereignty.OperatorIDs) > 0 {
			op = t.OperatorSovereignty.OperatorIDs[0]
		}
		policies = append(policies, TenantPolicy{
			TenantID:          t.TenantID,
			Name:              t.Name,
			MLCALane:          t.MLCALane,
			AllowedClasses:    allowed,
			AllowedScopes:     allowedScopes,
			AllowedActions:    allowedActions,
			AllowedRisks:      allowedRisks,
			RequiresApproval:  t.CapabilityPolicy.RequiresOperatorApproval,
			IdentityNamespace: t.TenantID + "-ns",
			SovereignOperator: op,
		})
		for _, cl := range t.CapabilityPolicy.AllowedCapabilityClasses {
			parts := strings.Split(cl, ".")
			g := CapabilityLattice{}
			g.Grant.Class = parts[0]
			g.Grant.Scope = "default"
			g.Grant.Action = "execute"
			g.Grant.Risk = "low"
			if len(parts) >= 2 {
				g.Grant.Scope = parts[1]
			}
			g.Lanes = []string{t.MLCALane}
			lattice = append(lattice, g)
		}
	}
	return policies, lattice
}