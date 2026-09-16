package main

type USLGate struct {
	tenants TenantsConfig
}

func NewUSLGateWithTenants(cfg TenantsConfig) USLGate {
	return USLGate{tenants: cfg}
}

func (u USLGate) Check(req CapabilityRequest) (bool, string) {
	// Λ.7 Governance Supremacy
	if !u.tenants.Governance.USLGateEnforced {
		return false, "USL gate disabled"
	}
	// Find tenant
	var tenant *Tenant
	for i := range u.tenants.Tenants {
		if u.tenants.Tenants[i].TenantID == req.TenantID {
			tenant = &u.tenants.Tenants[i]
			break
		}
	}
	if tenant == nil {
		return false, "tenant not found"
	}
	// Λ.4 Identity Separation - lane must match
	if tenant.MLCALane != req.MLCALane {
		return false, "MLCA lane mismatch"
	}
	// Capability lattice check
	allowed := false
	for _, c := range tenant.CapabilityPolicy.AllowedCapabilityClasses {
		if c == req.CapabilityClass {
			allowed = true
			break
		}
	}
	if !allowed {
		return false, "capability class not allowed"
	}
	// Λ.6 Corrigibility - operator must be present
	if req.OperatorID == "" {
		return false, "operator required for corrigibility"
	}
	// Provider allowed?
	if len(tenant.ModelRouting.AllowedProviders) > 0 {
		ok := false
		for _, p := range tenant.ModelRouting.AllowedProviders {
			if p == req.ModelRef.ProviderID {
				ok = true
				break
			}
		}
		if !ok {
			return false, "provider not allowed for tenant"
		}
	}
	return true, ""
}
