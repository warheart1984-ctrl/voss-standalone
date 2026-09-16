package main

type ProjectInfinityAdapter struct {
	baseURL string
}

func NewProjectInfinityAdapter(baseURL string) *ProjectInfinityAdapter {
	return &ProjectInfinityAdapter{baseURL: baseURL}
}

func (a *ProjectInfinityAdapter) CapabilityRequest(req CapabilityRequest) (bool, string) {
	if req.ModelRef.KeyRef == "" {
		return false, "key_ref required for Project Infinity"
	}
	return true, ""
}

func (a *ProjectInfinityAdapter) Generate(req CapabilityRequest) (map[string]interface{}, error) {
	return map[string]interface{}{
		"provider": "project-infinity",
		"model": req.ModelRef.ModelID,
		"intent": req.IntentID,
	}, nil
}

func (a *ProjectInfinityAdapter) Trace(req CapabilityRequest) (map[string]interface{}, error) {
	return map[string]interface{}{"trace_id": req.IntentID}, nil
}
