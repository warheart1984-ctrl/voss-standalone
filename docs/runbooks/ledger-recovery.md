# Runbook: Ledger Recovery

The ledger is a hash chain (`prev_hash -> hash`). Every entry is sealed with
`genesis` at the head. `GET /verify` returns `valid` and `chain_breaks`.

## Symptom: `chain_breaks > 0`

The runtime fails closed on any new request with `rule_ref=ledger.integrity`
(HTTP DENY) until the chain is restored. **Do not write new entries over a
broken chain** — append with `VOSS_MODE=ledger POST /append` refuses when the
existing chain is invalid.

## Recovery steps

1. Stop the router: `docker compose stop voss-model-router`.
2. Inspect the persisted chain at `deploy/voss/data/ledger/ledger.json`.
3. Audit the break: an entry whose `prev_hash` does not equal the prior entry's
   `hash`. Identify whether it is corruption of a prior entry (rehash the
   entry content; `hash` is `sha256` of the payload fields with `hash` and
   `prev_hash` zeroed) or a forged append.
4. If the tampered entry is a genuine past decision, replay it from the last
   intact entry, recomputing hashes. If it is forged, drop it — but if later
   entries point at it you must re-seal them as well; this is an evidence
   trigger, not an edit-and-forget operation.
5. Verify: `GET /verify` on the persisted file yields zero breaks.
6. Restart the chain via `VOSS_LEDGER_DIR` reload, then restart the router.

## Offline verification

- `deploy/voss/router/service/go/router_test.go` / `VerifyExport` implement the
  same integrity check; a standalone check runs against a static export.

## Backups

- The ledger volume `./data/ledger` is the source of truth. Snapshot it before
  any recovery mutation.

## Invariant

> A broken chain stops the runtime; recovery is explicit and audited, never an
> unnoticed overwrite.