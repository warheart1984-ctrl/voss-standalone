# Voss governance contract

This file is the checked-in evidence baseline for the standalone runtime. Claims not backed by a repository artifact are explicitly marked pending.

## Λ requirements trace matrix

| Law | Metric | Alert | Test | Ledger fields |
|---|---|---|---|---|
| Λ.1 bounded execution | `voss_cycle_latency_seconds` | `VossCycleLatency` | input/output bounds | intent, capability, result |
| Λ.2 auditability | `voss_ledger_entries_total` | `VossLedgerChainBreak` | chain replay | prev_hash, hash, replay_marker |
| Λ.3 fail closed | `voss_halt_events_total` | `VossFailClosed` | violation halts | halt_reason, stage |
| Λ.4 identity separation | `voss_identity_cross_access_total` | `VossIdentitySeparationViolation` | cross-tenant denial | tenant_id, identity_boundary |
| Λ.5 drift | `voss_drift_deviation_score` | `VossDriftDeviation` | baseline deviation | baseline_id, drift_score |
| Λ.6 corrigibility | `voss_operator_interrupt_latency_seconds` | `VossCorrigibilityLatency` | interrupt precedence | operator_id, interrupt |
| Λ.7 governance supremacy | `voss_usl_admission_total` | `VossUSLAdmissionRejection` | lattice admission | rule_ref, decision |

## GRE-1001 pipeline

`input_validation` → `capability_check` → `identity_separation` → `usl_gate` → `drift_check` → `operator_corrigibility` → `audit_trail` → `ledger_write` → `output_validation`.
Each stage receives the immutable request plus prior stage evidence and emits a pass/fail decision and evidence envelope. Any failure halts and surfaces the reason.

## USL and constitutional model

Capabilities are `{class, scope, action, risk}`. A tenant slice is a set of capability grants represented as a lattice: `tenant > lane > capability class > action`; admission is `ADMIT`, `DENY`, or `QUARANTINE`, with a reason and rule reference logged for every decision.

Roles are `operator` (sovereign), `governance` (policy author), `agent` (executor), and `auditor` (read-only). Operators may interrupt, correct, or terminate; agents may request and execute only admitted capabilities; auditors cannot mutate. Allowed transitions are `requested→admitted→executing→completed`, with `→halted` from every state and no transition out of `halted` without operator action.

MLCA lanes: SAFE permits low-risk inference with mandatory isolation and full audit; NORMAL permits approved production capabilities with full audit; EXPRESS permits only pre-approved low-risk actions and bounded latency. Lane changes require operator admission.

## Evidence baseline

- GCRE standards: Zenodo DOI(s) — **pending verification; do not treat as present**.
- ProjectInfinity runtime spine: GitHub reference — **pending verification; do not treat as present**.
- Proof bundles referenced by the brief: **none were present in this archive**; future bundles must be registered here with digest, source, and verification date.
