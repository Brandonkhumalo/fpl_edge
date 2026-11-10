# FPL Edge product updates roadmap

Updated: 10 November 2026

## Product direction

The next releases should create a useful weekly retention loop rather than add isolated pages:

`Import squad -> detect changes -> recommend actions -> remind before deadline -> measure results -> improve next week`

All official FPL integration must remain read-only unless a separately reviewed and authorized integration becomes available. FPL Edge must never request an official FPL password or session cookie, and internal transfers or chip selections must not be represented as completed official FPL actions.

## Priorities

| Priority | Feature | Retention value | Estimated effort |
| --- | --- | --- | --- |
| 6 | Mini-league and rival intelligence | High | High |

## 6. Mini-league and rival intelligence

Subject to provider terms, allow users to import selected mini-leagues and follow chosen rivals. Potential capabilities include:

- Ownership differences between the user and rivals
- Captain-risk comparison
- Protect-rank versus chase-rank strategies
- Productive differentials competitors do not own
- Projected standings after proposed moves
- Shareable weekly matchup cards

Projected ranks and outcomes must be labelled as estimates.

Official reference: [FPL leagues and cups guide](https://www.premierleague.com/en/news/2174934/fpl-basics-leagues-and-cups)

## Completed retention release

Implemented on 5 November 2026:

- Deadline readiness centre with timezone-aware deadline, availability, blank/double, chip, transfer and captaincy checks
- In-app deadline alerts, persisted preferences, durable delivery records and downloadable calendar reminders
- Player watchlists with persisted price, availability and forecast-change events
- Immutable recommendation snapshots, recorded user decisions and expected-versus-manually-recorded-actual review
- Named three-to-eight-Gameweek transfer scenarios with bank/free-transfer carryover, explicit assumptions and stale-revision detection

Implemented on 6 November 2026:

- Cached Live Gameweek snapshots with a database-wide provider sync lease, provisional/final labels, squad/captain/bench totals, estimated bonus, defensive progress, remaining fixtures and estimated automatic substitutions
- Deadline-locked internal squad selections so next-Gameweek planning does not rewrite the prior Gameweek's live total
- Grounded advisor answers assembled only from immutable recommendation, fixture, transfer, chip, player and bounded-plan evidence
- Optional opt-in OpenAI Structured Outputs integration that ranks evidence IDs only; deterministic evidence ranking remains available without an API key
- Audited advisor questions, evidence bundles, responses and per-user request limits

Implemented on 10 November 2026:

- Read-only public FPL manager/team ID connection without passwords or session cookies
- Rate-limited and audited official squad previews containing all 15 players, bank, team value, captaincy, chip usage, and current/past Gameweek history
- Explicit internal-versus-official reconciliation differences that require confirmation before atomically replacing the FPL Edge squad
- Persisted official-team linkage for repeat post-Gameweek reconciliation, with stale preview and squad-revision protection
- Transparent free-transfer estimation and current-price fallback because the public API does not expose exact free-transfer balances or player purchase prices

Email and web-push jobs remain disabled until delivery adapters and provider credentials are configured. Actual results remain explicitly manual until finalized provider settlement is integrated. Mini-league movement remains unavailable until mini-league import is implemented under item 6; the Live page states this instead of estimating a rank.

## Required platform work

The roadmap also depends on:

- Email and web-push delivery adapters, worker retries and unsubscribe handling
- Licensed live-data replacement, provider monitoring and correction reconciliation
- Automated finalized-result settlement and richer outcome analytics
- Analytics for activation, weekly retention, alert engagement, and recommendation adoption
- Observability, backup, and recovery procedures
- End-to-end tests for import, reconciliation, external delivery, planning, and provider result settlement
