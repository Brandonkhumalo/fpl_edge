# FPL Edge implementation plan

This plan deliberately delivers working vertical slices in dependency order. A phase is complete only when its acceptance tests pass; unfinished capabilities are not represented by mocked recommendations.

## Phase 1 - Foundation

- [x] Go/Echo API, PostgreSQL/pgx, migrations, structured logs, health checks
- [x] React/TypeScript/Vite application shell and authenticated routing
- [x] Docker Compose and environment contract
- [x] Access/refresh authentication with Argon2id and refresh-token rotation
- [ ] Email delivery adapter for verification and password reset
- [ ] Redis cache and durable background worker
- [ ] Authorized football-data provider ingestion

## Phase 2 - Persistent squad management

- [x] Season-aware schema with integer price units
- [x] Backend squad composition, club-limit, budget, lineup and captain validation
- [x] Optimistic concurrency through squad revisions
- [x] Atomic internal transfers with idempotency and immutable ledger entries
- [x] Squad snapshot/audit records and transactional outbox
- [x] Selling-price and point-hit calculations with tests
- [ ] WebSocket delivery backed by the outbox and Redis Pub/Sub
- [ ] Full onboarding player picker and squad reconciliation UI

## Phase 3 - Scouting and baseline predictions

- [x] Explainable Go baseline projection formula
- [x] Deterministic starting-XI optimizer
- [ ] Provider-backed fixtures, current-season players and player stats
- [ ] Scouting/filtering UI, FDR dashboard, stored prediction snapshots
- [ ] Personalized captaincy and transfer recommendation APIs

## Phase 4 - Advanced optimization

- [ ] Multi-period transfer beam search
- [ ] Wildcard, Free Hit, Triple Captain and Bench Boost optimizers
- [ ] Immutable scenario simulations and comparison

## Phase 5 - Optional analytics service

- [ ] FastAPI boundary, chronological training pipeline, LightGBM evaluation
- [ ] OR-Tools optimizer and Go fallback contract
- [ ] Backtesting and calibration reports

## Phase 6 - Product hardening

- [ ] AI advisor grounded only in engine outputs
- [ ] Notifications, analytics, watchlists
- [ ] Full Vitest/RTL and Playwright workflow
- [ ] Rate-limit storage, observability, backups and production deployment runbook

