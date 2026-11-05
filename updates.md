# FPL Edge product updates roadmap

Updated: 5 November 2026

## Product direction

The next releases should create a useful weekly retention loop rather than add isolated pages:

`Import squad -> detect changes -> recommend actions -> remind before deadline -> measure results -> improve next week`

All official FPL integration must remain read-only unless a separately reviewed and authorized integration becomes available. FPL Edge must never request an official FPL password or session cookie, and internal transfers or chip selections must not be represented as completed official FPL actions.

## Priorities

| Priority | Feature | Retention value | Estimated effort |
| --- | --- | --- | --- |
| 1 | Official squad import and reconciliation | Very high | Medium |
| 6 | Mini-league and rival intelligence | High | High |
| 7 | Live Gameweek experience | High | High |
| 8 | Grounded AI explanations | Medium | Medium |

## 1. Official squad import and reconciliation

Allow users to enter their public FPL manager or team ID and import:

- Current 15-player squad
- Bank and team value
- Free transfers
- Captain and vice-captain
- Active and previously used chips
- Gameweek history

Reconcile official state against FPL Edge after each Gameweek and clearly show differences before applying them internally. The integration must be read-only, rate-limited, auditable, and designed around provider terms and failure handling.

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

## 7. Live Gameweek mode

During matches, show:

- Live squad points
- Captain contribution
- Bench points
- Estimated bonus points
- Defensive-contribution progress
- Mini-league movement
- Remaining players and fixtures
- Final points after the official lock

This requires an approved live-data source, caching, rate limiting, correction handling, and a clear distinction between provisional and final points.

Official reference: [2026/27 FPL game updates](https://fantasy.premierleague.com/help/new)

## 8. Grounded AI advisor

Add AI explanations only after immutable recommendation snapshots and outcome measurement exist. The AI must explain engine outputs rather than invent recommendations or evidence.

Supported questions could include:

- Why is this captain recommended?
- Why is this transfer worth a hit?
- What changed since yesterday?
- Compare these two players.
- Build a conservative three-week plan.
- What evidence would change this recommendation?

Every numerical or factual claim must come from persisted engine output and carry fixture, model, and source provenance.

## Completed retention release

Implemented on 5 November 2026:

- Deadline readiness centre with timezone-aware deadline, availability, blank/double, chip, transfer and captaincy checks
- In-app deadline alerts, persisted preferences, durable delivery records and downloadable calendar reminders
- Player watchlists with persisted price, availability and forecast-change events
- Immutable recommendation snapshots, recorded user decisions and expected-versus-manually-recorded-actual review
- Named three-to-eight-Gameweek transfer scenarios with bank/free-transfer carryover, explicit assumptions and stale-revision detection

Email and web-push jobs remain disabled until delivery adapters and provider credentials are configured. Actual results remain explicitly manual until finalized provider settlement is integrated.

## Required platform work

The roadmap also depends on:

- Email and web-push delivery adapters, worker retries and unsubscribe handling
- Provider rate limiting and caching
- Automated finalized-result settlement and richer outcome analytics
- Analytics for activation, weekly retention, alert engagement, and recommendation adoption
- Observability, backup, and recovery procedures
- End-to-end tests for import, reconciliation, external delivery, planning, and provider result settlement
