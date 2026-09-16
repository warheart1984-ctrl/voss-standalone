package main

type NoOpAdapter struct{}

func (a NoOpAdapter) CapabilityRequest(req CapabilityRequest) (bool, string) {
	// Provider must implement capability check
	return false, "provider not configured"
}

func (a NoOpAdapter) Generate(req CapabilityRequest) (map[string]interface{}, error) {
	return nil, nil
}

func (a NoOpAdapter) Trace(req CapabilityRequest) (map[string]interface{}, error) {
	return nil, nil
}
