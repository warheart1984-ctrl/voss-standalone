package main

import (
	"crypto/sha256"
	"encoding/json"
	"sync"
	"time"
)

type LedgerEntry struct {
	Timestamp time.Time `json:"ts"`
	IntentID  string    `json:"intent_id"`
	TenantID  string    `json:"tenant_id"`
	Provider  string    `json:"provider"`
	Admitted  bool      `json:"admitted"`
	PrevHash  string    `json:"prev_hash"`
	Hash      string    `json:"hash"`
}

type Ledger struct {
	mu sync.Mutex
	entries []LedgerEntry
	lastHash string
}

func NewLedger() *Ledger { return &Ledger{} }

func (l *Ledger) Append(e LedgerEntry) LedgerEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	e.PrevHash = l.lastHash
	b, _ := json.Marshal(e)
	h := sha256.Sum256(b)
	e.Hash = string(h[:])
	l.entries = append(l.entries, e)
	l.lastHash = e.Hash
	return e
}

func (l *Ledger) WriteDecision(req CapabilityRequest, admitted bool) LedgerEntry {
	e := LedgerEntry{
		Timestamp: time.Now().UTC(),
		IntentID:  req.IntentID,
		TenantID:  req.TenantID,
		Provider:  req.ModelRef.ProviderID,
		Admitted:  admitted,
	}
	return l.Append(e)
}
