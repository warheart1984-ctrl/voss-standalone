package main

import "errors"

type Adapter interface {
	CapabilityRequest(req CapabilityRequest) (bool, string)
	Generate(req CapabilityRequest) (map[string]interface{}, error)
	Trace(req CapabilityRequest) (map[string]interface{}, error)
}

type FakeAdapter struct {
	allow bool
}

func NewFakeAdapter(allow bool) *FakeAdapter { return &FakeAdapter{allow: allow} }

func (a *FakeAdapter) CapabilityRequest(req CapabilityRequest) (bool, string) {
	if !a.allow {
		return false, "fake adapter denies"
	}
	return true, ""
}

func (a *FakeAdapter) Generate(req CapabilityRequest) (map[string]interface{}, error) {
	if req.ModelRef.KeyRef == "" {
		return nil, errors.New("key_ref required")
	}
	return map[string]interface{}{
		"provider": req.ModelRef.ProviderID,
		"model":    req.ModelRef.ModelID,
		"intent":   req.IntentID,
	}, nil
}

func (a *FakeAdapter) Trace(req CapabilityRequest) (map[string]interface{}, error) {
	return map[string]interface{}{"trace_id": req.IntentID}, nil
}

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
		"model":    req.ModelRef.ModelID,
		"intent":   req.IntentID,
	}, nil
}

func (a *ProjectInfinityAdapter) Trace(req CapabilityRequest) (map[string]interface{}, error) {
	return map[string]interface{}{"trace_id": req.IntentID}, nil
}