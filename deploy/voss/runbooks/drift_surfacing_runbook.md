# Drift Surfacing Runbook

## Trigger
Alert `VossDriftDeviation` fires when `voss_drift_deviation_score > 0.8` for >2m.

## Response
1. Surface deviation to operator immediately. No propagation.
2. Capture baseline vs current metrics, model provider, capability class.
3. Log to ledger with drift score and context.
4. Pause related capability class if drift continues.

## Recovery
1. Operator reviews drift report and audit trail.
2. If false positive, update baseline with operator approval.
3. If true drift, revert to last known good state via ledger replay.
4. Update drift detection thresholds in Prometheus alerts.
5. Record remediation in continuity ledger.
