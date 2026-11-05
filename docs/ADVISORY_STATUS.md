# Weekly advisory engine — 5 November 2026

FPL Edge now provides authenticated, squad-specific baseline advice at `/advice`.
It is an explainable statistical decision-support system, **not a calibrated ML
model, a guarantee of points, or an official FPL execution integration**.

| Capability | Implemented | Limits |
| --- | --- | --- |
| Player forecasts | Every eligible catalogue player; fixture-level components and gameweek totals | Experimental estimates, no accuracy/backtest claim |
| Captain / VC / XI | Legal formation, distinct owned starters, highest upcoming-GW expected points | Next-five totals break ties; no autosub or captain-fallback simulation |
| Transfers | Top five independent one-player alternatives, hold threshold, same-calendar XI gain minus hits | No multi-period free-transfer strategy or combined ordinary-transfer plan |
| Bench Boost / Triple Captain | Extra expected points, best week within available forecast/window, use versus hold | Fixed conservative thresholds; no full-season chip scheduling |
| Wildcard / Free Hit | Legal example rebuilds with budget, position and club constraints; estimated gains | Greedy search up to eight changed slots, not a global optimum; cannot discover all downgrade-funded combinations |
| Prior chip use | Previously used sets, current windows, consecutive FH restrictions | Self-reported official history must be reviewed first |
| Provenance | Atomic stored provider snapshot, fetched timestamp, model version, squad revision | Source retained as latest snapshot; immutable prediction-result history not yet implemented |

## Data and calendar

Production sync fetches official FPL bootstrap statistics and fixtures at startup
and every six hours. Both must validate before committing a new snapshot and
catalogue. Failed syncs preserve the prior snapshot. Advice is withheld after
24 hours without a successful sync, for absent data or a closed planning deadline.
Development-only fake catalogues do not generate advice.

For each club, select the next five assigned, dated, unstarted fixtures. Extend
the shared comparison window through the latest fifth fixture (at least five
gameweeks, capped by season end). Include additional fixtures within that window.
This avoids comparing five matches ending in different weeks. Blank gameweeks
are zero; doubles sum their fixtures. Undated/unassigned fixtures are omitted with
a warning. Fewer than five fixtures is shown explicitly; ordinary transfers require
full coverage for both players. Near season end, rankings may thus be withheld.

Predictions recalculate on request using the saved squad revision and latest source.
The UI query key includes user and revision; squad/chip edits cause fresh advice.
Concurrent edits produce a conflict instead of mixed-revision output. Endpoints
require authentication and derive the squad from the authenticated user. Advice
never applies changes automatically or sends official FPL actions.

## Model: `fixture-baseline-2026.1`

- Seasonal starts and minutes estimate an 80-minute-start/20-minute-cameo mixture;
  start duration adjusts to observed minutes. Availability scales outcomes once.
  Injury probability is held constant across the horizon: no invented recovery
  dates. No-minute players get zero expected minutes and low evidence strength.
- xG/xA per 90 shrink towards conservative position priors with 450 equivalent
  minutes of weight. Fixture difficulty uses an exponential multiplier and small
  home/away adjustment. Coefficients are heuristic, not fitted or calibrated.
- Clean-sheet probability uses shrunk team concession rate, opponent difficulty,
  estimated minutes and probability of playing at least 60 minutes.
- Goals: GKP/DEF/MID/FWD multipliers 10/6/5/4; assists 3; clean sheets 4/4/1/0.
- Defensive actions use Poisson threshold probabilities: 10 for defenders and 12
  for midfielders/forwards, awarding 2; goalkeepers receive no defensive bonus.
- Saves and conceded-goal deductions use expected integer groups (3 and 2), not
  the incorrect floor of expected saves/goals.
- Bonus uses historical rates capped at 3 per fixture, not future BPS rankings.
  Penalty saves/misses, yellow/red cards and own goals use shrunk observed rates
  and their official scoring multipliers.
- Fixture breakdowns add up to the predicted score. Low/moderate evidence reflects
  sample minutes, not a calibrated statistical confidence interval.

Ordinary transfers must improve optimized XI+captain totals by at least 2 expected
points after hits. Moves must be affordable, same-position, unowned, available and
respect three players per club. Shortlist entries are independent alternatives.
An active Free Hit restricts transfer gains to this gameweek because the original
squad returns; next-five player forecasts remain visible for context. Active BB/TC
modify the current-week transfer objective; future weeks use normal multipliers.

BB/TC use thresholds of 12/7 extra points unless the chip expires this week. FH
requires a temporary rebuild gain of at least 12; WC requires 20 over the window.
Both must need more than free transfers+1 moves. If several chips qualify, the
higher immediate extra-point option is preferred. These are heuristic thresholds,
not proven optimal full-season chip timing.

## Official sources

Verified 5 November 2026:

- [Scoring and defensive contributions](https://www.premierleague.com/en/news/2174909/)
- [2026/27 chip sets and expiry](https://www.premierleague.com/en/news/4679879/whats-happening-with-fpl-chips-in-202627)
- [Bootstrap data](https://fantasy.premierleague.com/api/bootstrap-static/)
- [Fixtures](https://fantasy.premierleague.com/api/fixtures/)

All four chips have two sets, split at GW19/GW20; no carryover and no consecutive
Free Hits. The app uses provider windows rather than hardcoded January dates.

## Remaining work

Chronological backtesting/calibration, recent match-level minutes, measured
uncertainty, full-season chip opportunity cost, multi-period transfer search,
immutable prediction-result history, official squad reconciliation, notifications,
live result scoring and optional LLM explanations. Saved strategy/horizon settings
do not override this model's fixed five-fixture minimum.

## Local validation

- Go race suite with isolated PostgreSQL: migrations, persisted source, per-user
  squad advice, chip history, transfer-driven recalculation and stale-data rejection.
- Domain tests: scoring components, blank/double weeks, unavailable players,
  legal zero-projection lineups, transfer hits/budget/club limits and rebuild horizon.
- Provider test: statistics normalization and failure of either upstream source.
- `go vet ./...`, frontend TypeScript/Vite build and 11 Vitest tests passed.
- Production-mode local smoke with 670 official catalogue players: authenticated
  advice, five transfer alternatives and legal chip rebuild output. Approximately
  0.2 seconds for the test squad on this workstation; not a load-test guarantee.
- Headless browser: advisor page, player breakdown and 390px mobile overflow checks.

These checks do not establish Railway deployment, forecast accuracy or calibration.
