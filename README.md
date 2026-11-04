# FPL Edge

FPL Edge is a production-oriented Fantasy Premier League decision-support platform. The current implementation provides a secure foundation and a working registration → squad setup → persisted dashboard flow. It deliberately withholds live advice when authorized football data is unavailable.

## What works now

- Echo/Go API with PostgreSQL migrations and pgx connection pooling
- Argon2id passwords, short-lived JWT access tokens and rotating opaque refresh sessions
- Refresh tokens stored in `HttpOnly`, `SameSite=Strict` cookies
- Season-aware persistent squads with backend composition, budget and formation validation
- Revision-controlled, idempotent internal transfers with a transactional outbox
- Selling-value, transfer-hit, baseline projection and starting-XI domain logic
- Responsive React application with a polished landing page, email/password authentication, formation-aware squad onboarding and personalized squad dashboard
- Synthetic player fixtures only when `APP_ENV=development`, visibly labelled in the UI
- Current clubs, players, prices and gameweeks synchronized from the official FPL bootstrap feed in production
- One multi-stage, non-root container serving both the Go API and React SPA

See [docs/IMPLEMENTATION_PLAN.md](docs/IMPLEMENTATION_PLAN.md) for completed and remaining phases. Unfinished optimizer, chip, provider and AI surfaces are explicitly labelled and do not return fabricated results.

## Local setup

Prerequisites: Docker with Compose v2 (or Go 1.22+, Node 22+ and PostgreSQL 16 for native development).

For local Docker Compose, create `.env` with the Railway variables plus local PostgreSQL values:

```env
APP_ENV=development
PORT=8080
POSTGRES_DB=fpl_edge
POSTGRES_USER=fpl_edge
POSTGRES_PASSWORD=<random-local-password>
JWT_SECRET=<at-least-32-random-bytes>
CORS_ORIGINS=http://localhost:8080
```

Then start the stack:

```bash
docker compose up --build
```

- Web app: `http://localhost:8080`
- API readiness: `http://localhost:8080/health/ready`
- PostgreSQL and Redis are internal to the Compose network.

Migrations run transactionally at API startup. In development, the API seeds clearly named synthetic clubs and players so the onboarding workflow can be exercised. In production, the API refreshes the current official FPL catalogue at startup and every six hours. A temporary provider failure does not crash the service or delete the last successful catalogue.

## Railway deployment

Railway builds the root [Dockerfile](Dockerfile). It contains three stages: the React build, the Go build, and one minimal runtime image. The Go process is the only runtime process and serves both `/api/v1/*` and the compiled SPA, including client-side route fallback.

1. Create a Railway service from this repository. Do not set a root directory; the Dockerfile is at the repository root.
2. Add a Railway PostgreSQL service and expose its `DATABASE_URL` to the application service.
3. Configure these application variables:

   - `APP_ENV=production`
   - `JWT_SECRET=<at least 32 random bytes>`
   - `DATABASE_URL=${{Postgres.DATABASE_URL}}` using Railway's variable reference for your PostgreSQL service

4. Do not define `HTTP_ADDR` on Railway. The API automatically binds to Railway's injected `PORT`.
5. Set the health-check path to `/health/ready`.

`VITE_API_URL` defaults to `/api/v1` during the image build, so no public API hostname is required and browser credentials remain same-origin. `STATIC_DIR=/app/public` is already set in the runtime image.

`CORS_ORIGINS`, `ACCESS_TOKEN_TTL` and `REFRESH_TOKEN_TTL` are optional overrides. They are not needed for the standard same-origin Railway deployment.

## Native validation

```bash
cd backend
GOCACHE=/tmp/fpl-edge-go-cache go test ./...

cd ../frontend
npm install
npm run build
npm audit --omit=dev
```

## Security and data boundaries

- Never provide an official FPL password or session cookie to this application.
- Confirmed transfers update the FPL Edge internal ledger only. They do not execute an official FPL transfer.
- Production requires HTTPS so refresh cookies receive the `Secure` flag.
- Configure an explicit `CORS_ORIGINS` allowlist; credentials are never accepted from wildcard origins.
- The production catalogue currently uses FPL's official but undocumented bootstrap endpoint. Keep this integration isolated and replace it with a licensed provider if the product's scale or commercial terms require one.

## Team management and chips

My Squad loads the saved team and supports transfers, legal substitutions, and
captain/vice-captain changes. Chip Centre uses availability windows from the FPL
bootstrap feed, permits one chip per planning gameweek, and records usage.
Bench Boost includes the bench in scoring selections; Triple Captain changes the
captain multiplier to three. These are planning weights, not a live points feed.

Wildcard and Free Hit transfers do not consume free transfers or incur hits.
Free Hit saves the original squad, bank, purchase prices, and captaincy, then
restores them on the first authenticated request after that gameweek's deadline.
Wildcard changes remain permanent. Activate these transfer chips before making
transfers in that FPL Edge gameweek; retroactive activation is not supported.
Once activated, transfer chips cannot be cancelled. Bench Boost and Triple Captain
can be cancelled before the deadline.

The upcoming deadline determines the planning gameweek. Chip actions and team
changes affect FPL Edge only and do not submit actions to the official FPL game.
Existing official FPL chip usage is not imported automatically.

Player Scout, Team History, and Settings are available in the sidebar. Saved
strategy preferences do not yet drive an automated advisor. Live scoring,
fixture analysis, predictions, and AI advice remain unfinished.

## Price representation

All prices use integer tenths of a million: `55` represents £5.5m. Binary floating-point values are not used for squad finances.

