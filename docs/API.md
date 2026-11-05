# HTTP API (implemented slice)

All application endpoints use `/api/v1`. Errors have the shape:

```json
{"error":{"code":"Bad Request","message":"...","requestId":"..."}}
```

## Public

- `POST /auth/register` — email, display name and password; creates a session
- `POST /auth/login` — email and password; creates a session
- `POST /auth/refresh` — rotates the `HttpOnly` refresh session cookie
- `GET /health/live`
- `GET /health/ready`

## Authenticated

Supply `Authorization: Bearer <access-token>`. Browser requests must also use credentials for refresh-cookie rotation.

- `POST /auth/logout`
- `GET /auth/me`
- `GET /seasons/current`
- `GET /players?seasonId=<uuid>&position=<position>&maxPrice=<integer>`
- `GET /squad`
- `POST /squad`
- `POST /transfers/confirm` — requires `Idempotency-Key` and `expectedRevision`
- `POST /optimizer/lineup`
- `GET /squad/tactics` — chip windows, usage provenance, history review status and planning multipliers
- `PUT /squad/chip-history` — replace self-reported prior chip usage for the authenticated user's squad

Example chip-history request:

```json
{
  "expectedRevision": 3,
  "usages": [
    {"chipId": 4, "gameweek": 6},
    {"chipId": 7, "gameweek": 25},
    {"chipId": 3, "gameweek": null}
  ]
}
```

Use chip IDs returned by `/squad/tactics`; do not hard-code IDs across seasons.
Each ID represents one availability window. A null gameweek means "used in this
window, exact week unknown". An empty `usages` list explicitly records no prior
usage and removes only previous self-reported entries. In-app activations cannot
be overwritten. Future windows, invalid weeks, duplicate chips, conflicting
known gameweeks and consecutive Free Hits are rejected. Revision checks serialize
history edits with chip activation; before/after entries are recorded in the audit.
Unknown Free Hit dates may prevent activation near a window boundary until clarified.

The lineup optimizer is a foundation endpoint, not verified personalized advice:
stored player queries currently return zero expected points. See
[ADVISORY_STATUS.md](ADVISORY_STATUS.md) for the current capability audit.

Transfer confirmation is serializable and rejects stale revisions, unowned outgoing players, already-owned incoming players, position changes, insufficient funds and fourth players from one club.
