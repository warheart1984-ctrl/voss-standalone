package main

import "strings"

// CoherenceProjection is a read-only, bounded slice of external evidence
// attached to a request. Workflows may consume it as context for reasoning;
// it is never authority — classification stays "evidence_only".
type CoherenceProjection struct {
	// MaxEntries bounds the projection. Immutable after construction.
	MaxEntries int
	Entries    []ProjectionEntry
}

type ProjectionEntry struct {
	Key          string `json:"key"`
	Source       string `json:"source"`
	Classification string `json:"classification"` // evidence_only | authority (authority rejected at admission)
	Value        string `json:"value,omitempty"`
}

func NewCoherenceProjection(max int) *CoherenceProjection {
	return &CoherenceProjection{MaxEntries: max, Entries: []ProjectionEntry{}}
}

// Attach appends an evidence-only entry, enforcing the bound. Returns false
// when the projection is full or the entry claims authority.
func (p *CoherenceProjection) Attach(key, source, classification, value string) bool {
	if strings.ToLower(classification) == "authority" {
		return false
	}
	if len(p.Entries) >= p.MaxEntries {
		return false
	}
	p.Entries = append(p.Entries, ProjectionEntry{Key: key, Source: source, Classification: classification, Value: value})
	return true
}

// Bounds describes the projection limits the workflow exposes to adapters.
type CoherenceBounds struct {
	MaxEntries int    `json:"max_entries"`
	EnforcedBy string `json:"enforced_by"`
}

func (p *CoherenceProjection) Bounds() CoherenceBounds {
	return CoherenceBounds{MaxEntries: p.MaxEntries, EnforcedBy: "governed-intake"}
}