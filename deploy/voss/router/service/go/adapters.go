package main

type NvidiaNIMAdapter struct {
	baseURL string
	keyRef string
}

func NewNvidiaNIMAdapter(baseURL, keyRef string) *NvidiaNIMAdapter {
	return &NvidiaNIMAdapter{baseURL: baseURL, keyRef: keyRef}
}

func (a *NvidiaNIMAdapter) CapabilityRequest(req CapabilityRequest) (bool, string) {
	// Check key_ref present
	if req.ModelRef.KeyRef == "" {
		return false, "key_ref required for Nvidia NIM"
	}
	return true, ""
}

func (a *NvidiaNIMAdapter) Generate(req CapabilityRequest) (map[string]interface{}, error) {
	return map[string]interface{}{"provider":"nvidia-nim","model":req.ModelRef.ModelID}, nil
}

func (a *NvidiaNIMAdapter) Trace(req CapabilityRequest) (map[string]interface{}, error) {
	return map[string]interface{}{"trace_id":req.IntentID}, nil
}

type GenericAdapter struct {
	providerID string
}

func NewGenericAdapter(id string) *GenericAdapter { return &GenericAdapter{providerID:id} }

func (a *GenericAdapter) CapabilityRequest(req CapabilityRequest) (bool, string) {
	return true, ""
}

func (a *GenericAdapter) Generate(req CapabilityRequest) (map[string]interface{}, error) {
	return map[string]interface{}{"provider":a.providerID,"model":req.ModelRef.ModelID}, nil
}

func (a *GenericAdapter) Trace(req CapabilityRequest) (map[string]interface{}, error) {
	return map[string]interface{}{"trace_id":req.IntentID}, nil
}
