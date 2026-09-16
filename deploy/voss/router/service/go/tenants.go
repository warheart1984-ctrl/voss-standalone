package main

import (
	"encoding/json"
	"os"
	"sync"
)

type TenantsConfig struct {
	Version  string            `json:"version"`
	Tenants  []Tenant          `json:"tenants"`
	MLCALanes map[string]Lane  `json:"mlca_lanes"`
	Governance Gov `json:"governance"`
}

type Tenant struct {
	TenantID string `json:"tenant_id"`
	Name string `json:"name"`
	MLCALane string `json:"mlca_lane"`
	CapabilityPolicy Policy `json:"capability_policy"`
	ModelRouting Routing `json:"model_routing"`
}

type Policy struct {
	AllowedCapabilityClasses []string `json:"allowed_capability_classes"`
	RequiresOperatorApproval bool `json:"requires_operator_approval"`
	IdentitySeparation bool `json:"identity_separation"`
}

type Routing struct {
	Policy string `json:"policy"`
	AllowedProviders []string `json:"allowed_providers"`
	KeyRefs map[string]string `json:"key_refs"`
}

type Lane struct {
	Description string `json:"description"`
}

type Gov struct {
	VossBinding []string `json:"voss_binding"`
	USLGateEnforced bool `json:"usl_gate_enforced"`
	LedgerRequired bool `json:"ledger_required"`
}

var tenantsOnce sync.Once
var tenantsCfg TenantsConfig

func LoadTenants(path string) TenantsConfig {
	tenantsOnce.Do(func() {
		data, err := os.ReadFile(path)
		if err != nil { return }
		json.Unmarshal(data, &tenantsCfg)
	})
	return tenantsCfg
}
