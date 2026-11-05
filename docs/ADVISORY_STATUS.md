# Advisory capability audit — 5 November 2026

FPL Edge is currently a persistent squad-management and chip-planning application.
It is **not yet a complete personalized advisory system**.

| Capability | Current implementation | Missing for advice |
| --- | --- | --- |
| Transfers | User-selected replacements; budget, club limit, position, revision and ownership checks | Candidate rankings, projected gain after hits, hold/transfer comparisons and multi-week planning |
| Captaincy | Save distinct owned starters as captain and vice-captain | Squad-specific rankings using trustworthy minutes and fixture projections |
| Chips | Availability windows, activation, cancellation of team chips, Free Hit restore, Wildcard persistence, scoring-selection multipliers, prior-history inventory | Recommendations for which chip to use and when, compared against saving it |
| Data | FPL bootstrap catalogue: clubs, players, prices, ownership, gameweeks and chip windows | Fixture ingestion, a freshness-aware projection pipeline and stored prediction provenance |
| Optimization | Tested projection formula and legal lineup optimizer | Runtime integration with real predictions and meaningful ranking |
| AI advisor | Not implemented | Grounded explanations over validated recommendation results |

Evidence:

- `backend/internal/domain/prediction.go`: `ProjectPoints` exists but has no production caller; it is exercised by its unit test.
- `backend/internal/store/store.go`: catalogue, squad and incoming-player queries populate expected points with literal zero.
- `backend/internal/httpapi/api.go`: `/optimizer/lineup` calls the domain optimizer with those values. Legal output does not establish useful recommendation quality.
- `backend/internal/football/fpl.go`: the bootstrap adapter does not ingest fixture schedules or create prediction snapshots.
- `frontend/src/pages/DashboardPage.tsx`: captain is the user's saved choice; automated advice is explicitly unavailable.

Completeness requires real projections and fixture inputs, personalized APIs for
transfers/captaincy/chip timing, explanations tied to source timestamps and squad
revision, and integration tests showing recalculation after squad and history edits.
Manual controls or chip availability must not be presented as those recommendations.

## Verified chip windows

Official 2026/27 guidance grants two sets of **all four** chips: Wildcard, Free Hit,
Triple Captain and Bench Boost. The first set expires at the Gameweek 19 deadline;
the second applies from Gameweek 20. Unused first-set chips do not roll over.
This is a gameweek boundary, not a generic January reset date. Free Hit cannot be
played in consecutive gameweeks, including GW19 then GW20.

Sources verified on 5 November 2026:

- [Premier League: 2026/27 chip rules](https://www.premierleague.com/en/news/4679879/whats-happening-with-fpl-chips-in-202627)
- [Premier League: how and when to use chips](https://www.premierleague.com/en/news/4362085)

The implementation uses provider-supplied availability windows rather than assuming
all future seasons have the same split. Prior history is self-reported, per squad
and per window; it is not an automatic import from official FPL.
