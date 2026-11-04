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

Transfer confirmation is serializable and rejects stale revisions, unowned outgoing players, already-owned incoming players, position changes, insufficient funds and fourth players from one club.
