# Runbook: Operator Sovereignty

Operator interrupts, corrections, and terminations are unconditional and take
precedence at every gate (Lambda.6). They are authenticated and ledgered.

## Endpoints (require `Authorization: Bearer $VOSS_OPERATOR_TOKEN`)

| Action      | Endpoint      | Body                                          | Effect                        |
|-------------|---------------|-----------------------------------------------|-------------------------------|
| Interrupt   | `POST /interrupt` | `{intent_id, operator_id}`                | Halt intent; deny current/in-flight; `ExecHalted` |
| Correct     | `POST /correct`   | `{intent_id, operator_id, directive}`     | Record directive; halt intent; `ExecHalted`      |
| Terminate   | `POST /terminate` | `{intent_id, operator_id}`                | Terminal; conflict forever; `ExecTerminated`     |

## Behavior

- Interrupt wins *during* an in-flight request (final re-check immediately
  before the provider call).
- A corrected intent is re-denied with `rule_ref=Lambda.6` and reason surfacing
  the directive.
- A terminated intent is terminal: every later attempt returns HTTP 409 DENY.
- Every operator event writes a `DENY Lambda.6` ledger entry bound to the
  intent and the sovereign operator.

## If the token is missing

`VOSS_OPERATOR_TOKEN` unset logs a warning at startup and the three operator
endpoints fail closed (HTTP 401). The public `/route` surface is unaffected.

## Operator field / identity

Sovereign operators are declared per tenant (`sovereign_operator`). A request
whose operator is not the tenant's sovereign fails identity separation
(`Lambda.4`). Requests to `/correct` and `/terminate` supply `operator_id` and
are logged; the bearer token proves the operator API caller.

## Incident: need to halt a runaway intent

```bash
curl -X POST localhost:8080/interrupt \
  -H "Authorization: Bearer $VOSS_OPERATOR_TOKEN" \
  -d '{"intent_id":"intent-1","operator_id":"op-admin"}'
```

Verify cessation: `voss_router_execution_total` stops increasing and the intent
reports `ExecHalted` (`GET /ledger` or the `execution` state endpoint).