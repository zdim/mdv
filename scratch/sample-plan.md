# Sample Implementation Plan

A throwaway doc for trying `mdv`. Open this, scroll around, press `enter` on a heading to leave a note. Quit with `q`. Your notes land in `sample-plan.notes.md` next to this file.

## Overview

We're going to ship a thing. The thing has three phases. Each phase has subparts.

## Phase 1: Data layer

Stand up the new schema, write the migration, backfill in batches.

### Schema changes

Add a `status` column and an index on `(user_id, created_at)`.

### Migration

Online migration via `pt-online-schema-change`. Backfill in 10k-row batches with a 200ms pause between batches.

### Verification

Spot-check 100 random rows for status consistency. Run the new query plan and confirm index usage.

## Phase 2: API layer

Add the new endpoint, update the existing one to dual-write during the migration window.

### New endpoint

`POST /v2/widgets` accepts `{name, status, tags[]}`. Returns `201` with the created entity.

### Dual write

For 48h after Phase 1 completes, write to both `widgets` and `widgets_v2`. Read from `widgets_v2` only.

## Phase 3: Client cutover

Flip the feature flag, monitor error rates, ramp 1% → 10% → 100% over a week.

### Rollout plan

- 1%: 24h, watch p95 latency and error rate
- 10%: 48h, watch the same plus DB CPU
- 100%: rest of the week

### Rollback

If error rate climbs >2% above baseline at any ramp, flip the flag off. Investigate, fix, retry.
