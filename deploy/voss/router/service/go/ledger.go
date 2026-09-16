package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type Ledger struct {
	mu       sync.Mutex
	entries  []LedgerEntry
	lastHash string
	nextIdx  uint64
}

func NewLedger() *Ledger {
	return &Ledger{lastHash: "genesis"}
}

func (l *Ledger) Append(entry LedgerEntry) LedgerEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry.Index = l.nextIdx
	entry.Timestamp = time.Now().UTC()
	entry.PrevHash = l.lastHash
	entry.Hash = l.hashEntry(entry)
	l.entries = append(l.entries, entry)
	l.lastHash = entry.Hash
	l.nextIdx++
	return entry
}

func (l *Ledger) WriteDecision(req CapabilityRequest, admitted bool, result AdmitResult, reason, ruleRef, decisionID string, stageLog []StageRecord) LedgerEntry {
	e := LedgerEntry{
		RequestID:  req.RequestID,
		IntentID:   req.IntentID,
		TenantID:   req.TenantID,
		MLCALane:   req.MLCALane,
		Capability: req.CapabilityClass,
		Provider:   req.ModelRef.ProviderID,
		Admitted:   admitted,
		Result:     result,
		Reason:     reason,
		RuleRef:    ruleRef,
		DecisionID: decisionID,
		StageLog:   stageLog,
		ReplayID:   fmt.Sprintf("replay-%s-%d", req.IntentID, time.Now().UnixNano()),
	}
	return l.Append(e)
}

// WriteExecution records the result of a governed provider invocation, bound
// to the admission decision that preceded it.
func (l *Ledger) WriteExecution(req CapabilityRequest, decisionID, executionHash string) LedgerEntry {
	e := LedgerEntry{
		RequestID:     req.RequestID,
		IntentID:      req.IntentID,
		TenantID:      req.TenantID,
		MLCALane:      req.MLCALane,
		Capability:    req.CapabilityClass,
		Provider:      req.ModelRef.ProviderID,
		Admitted:      true,
		Result:        Admit,
		Reason:        "executed under governance",
		RuleRef:       "provider.execution",
		DecisionID:    decisionID,
		ExecutionHash: executionHash,
		ReplayID:      fmt.Sprintf("replay-%s-%d", req.IntentID, time.Now().UnixNano()),
	}
	return l.Append(e)
}

func (l *Ledger) Verify() (bool, int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.entries) == 0 {
		return true, 0
	}
	if l.entries[0].PrevHash != "genesis" {
		return false, 0
	}
	breaks := 0
	for i := 1; i < len(l.entries); i++ {
		if l.entries[i].PrevHash != l.entries[i-1].Hash {
			breaks++
		}
	}
	return breaks == 0, breaks
}

func (l *Ledger) Export() []LedgerEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]LedgerEntry, len(l.entries))
	copy(out, l.entries)
	return out
}

func (l *Ledger) ExportToFile(path string) error {
	data, err := json.MarshalIndent(l.Export(), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// VerifyExport independently verifies a ledger export after process exit.
func VerifyExport(entries []LedgerEntry) (bool, int) {
	if len(entries) == 0 {
		return true, 0
	}
	if entries[0].PrevHash != "genesis" {
		return false, 0
	}
	breaks := 0
	for i := 1; i < len(entries); i++ {
		if entries[i].PrevHash != entries[i-1].Hash {
			breaks++
		}
	}
	return breaks == 0, breaks
}

func (l *Ledger) GenerateCER(decisionID string) (*CER, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, e := range l.entries {
		if e.DecisionID == decisionID {
			cer := &CER{
				Version:           "1.0",
				DecisionID:        e.DecisionID,
				RequestID:         e.RequestID,
				IntentID:          e.IntentID,
				TenantID:          e.TenantID,
				RuntimeIdentity:   "voss-standalone",
				EvidenceArtifacts: []string{e.Hash},
				VerificationChecks: e.StageLog,
				ReplayMarker:      e.ReplayID,
				Lineage:           l.entries,
				Timestamp:         time.Now().UTC(),
			}
			input, _ := json.Marshal(e)
			h := sha256.Sum256(input)
			cer.InputHash = hex.EncodeToString(h[:])
			return cer, true
		}
	}
	return nil, false
}

func (l *Ledger) Count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.entries)
}

func (l *Ledger) hashEntry(entry LedgerEntry) string {
	entry.Hash = ""
	entry.PrevHash = ""
	data, _ := json.Marshal(entry)
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
