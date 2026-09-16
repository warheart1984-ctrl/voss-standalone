package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func isolationRouter() *Router {
	r := NewRouter(testTenants(), testLattice())
	r.RegisterProvider("fake", NewFakeAdapter(true))
	return r
}

func TestIsolationCrossTenantDenied(t *testing.T) {
	r := isolationRouter()

	// A request claiming tenant-b's sovereign namespace but operated by
	// tenant-a's operator: identity separation must fail closed (Lambda.4).
	req := testRequest()
	req.TenantID = "tenant-b" // tenant-b sovereign is op-b
	req.MLCALane = LaneNormal
	req.OperatorID = "op-a" // not tenant-b's sovereign operator

	out, err := r.Admit(req)
	if err != nil {
		t.Fatalf("cross-tenant must be a governed DENY, not an error: %v", err)
	}
	if out.Decision.Result != Deny {
		t.Fatalf("expected DENY for cross-tenant access, got %s", out.Decision.Result)
	}
	if out.Decision.RuleRef != string(Lambda4) {
		t.Fatalf("expected Lambda.4, got %s", out.Decision.RuleRef)
	}
	sawIdentity := false
	for _, s := range out.Stages {
		if s.Stage == StageIdentitySeparation && !s.Passed {
			sawIdentity = true
		}
	}
	if !sawIdentity {
		t.Fatal("identity separation must halt with a failed Lambda.4 stage record")
	}
}

func TestIsolationNoCrossTenantCapabilityInheritance(t *testing.T) {
	// tenant-a allows inference only; a request claiming tenant-a but asking
	// for a capability it was never granted must be denied at tenant policy.
	r := isolationRouter()
	req := testRequest()
	req.CapabilityClass = CapabilityClass{Class: "inference", Scope: "default", Action: "execute", Risk: "high"}
	out, _ := r.Admit(req)
	if out.Decision.Result != Deny {
		t.Fatalf("expected DENY for unallowed risk, got %s", out.Decision.Result)
	}
}

func TestIsolationLaneBoundaryEnforced(t *testing.T) {
	r := isolationRouter()
	// tenant-a is SAFE only; an EXPRESS lane claim from it must fail.
	req := testRequest()
	req.MLCALane = LaneExpress
	out, _ := r.Admit(req)
	if out.Decision.Result != Deny || out.Decision.RuleRef != string(Lambda4) {
		t.Fatalf("expected DENY Lambda.4 on lane boundary, got %s %s", out.Decision.Result, out.Decision.RuleRef)
	}
}

func TestIsolationLedgerTenantSeparation(t *testing.T) {
	r := isolationRouter()

	// tenant-a admitted; tenant-b request denied. The ledger must distinguish
	// entries by tenant while remaining a single valid chain.
	ta := testRequest()
	r.Admit(ta)

	tb := testRequest()
	tb.TenantID = "tenant-b"
	tb.MLCALane = LaneNormal
	tb.OperatorID = "op-b"
	r.Admit(tb)

	entries := r.Ledger().Export()
	if len(entries) != 2 {
		t.Fatalf("expected 2 ledger entries, got %d", len(entries))
	}
	if entries[0].TenantID != "tenant-a" || entries[1].TenantID != "tenant-b" {
		t.Fatalf("ledger tenant attribution wrong: %s then %s", entries[0].TenantID, entries[1].TenantID)
	}
	verified, breaks := r.Ledger().Verify()
	if !verified || breaks != 0 {
		t.Fatalf("shared ledger must remain chained: %d breaks", breaks)
	}
}

// Trust-bundle parity: workflow contract digests must match the committed
// manifest. Any drift fails CI before the runtime propagates a stale intent.
func TestTrustBundleManifestParity(t *testing.T) {
	root := "../../../workflow"
	manifestPath := filepath.Join(root, "manifest-digests.sha256")
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("missing manifest: %v", err)
	}

	want := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			t.Fatalf("malformed manifest line: %q", line)
		}
		want[filepath.Base(parts[1])] = parts[0]
	}

	files, err := filepath.Glob(filepath.Join(root, "*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no workflow manifests found")
	}

	digest := func(path string) string {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return sha256hex(data)
	}

	for _, f := range files {
		name := filepath.Base(f)
		got := digest(f)
		if want[name] == "" {
			t.Fatalf("workflow %s has no pinned digest; update manifest-digests.sha256", name)
		}
		if !strings.EqualFold(got, want[name]) {
			t.Fatalf("trust bundle drift: %s digest %s != pinned %s. Unreviewed contract change.",
				name, got, want[name])
		}
	}
	if len(files) != len(want) {
		t.Fatalf("manifest pins %d workflows but disk has %d", len(want), len(files))
	}
}

func sha256hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}