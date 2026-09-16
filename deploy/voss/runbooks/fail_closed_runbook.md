# Fail-Closed Runbook

## Trigger
Alert `VossFailClosed` fires on `voss_usl_violation_total > 0` or GRE-1001 stage failure.

## Response
1. Halt execution immediately. Do not retry.
2. Capture intent_id, tenant_id, operator_id, stage failed.
3. Write incident to ledger with `admitted: false` and reason.
4. Surface to operator via corrigibility channel.
5. Notify on-call via Prometheus alert.

## Recovery
1. Operator reviews USL Gate logs and ledger entry.
2. If invariant violation is resolvable, operator issues corrigibility interrupt and provides corrected intent.
3. If violation is structural, change must be made to capability lattice or tenant policy before re-admission.
4. No bypass allowed. Λ.3 Fail-Closed is absolute.
