CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE seasons (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL UNIQUE,
    starts_on date NOT NULL,
    ends_on date NOT NULL,
    is_current boolean NOT NULL DEFAULT false,
    rules jsonb NOT NULL,
    CHECK (ends_on > starts_on)
);

CREATE UNIQUE INDEX one_current_season ON seasons (is_current) WHERE is_current;

CREATE TABLE gameweeks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    season_id uuid NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
    number smallint NOT NULL CHECK (number BETWEEN 1 AND 60),
    deadline_at timestamptz NOT NULL,
    is_current boolean NOT NULL DEFAULT false,
    UNIQUE (season_id, number)
);

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email text NOT NULL,
    password_hash text NOT NULL,
    display_name text NOT NULL,
    email_verified_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE UNIQUE INDEX users_email_unique_active ON users (lower(email)) WHERE deleted_at IS NULL;

CREATE TABLE user_sessions (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token_hash bytea NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    replaced_by uuid REFERENCES user_sessions(id),
    user_agent text,
    ip_address inet,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX user_sessions_user_active_idx ON user_sessions(user_id, expires_at) WHERE revoked_at IS NULL;

CREATE TABLE user_preferences (
    user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    strategy text NOT NULL DEFAULT 'balanced' CHECK (strategy IN ('conservative', 'balanced', 'aggressive')),
    planning_horizon smallint NOT NULL DEFAULT 6 CHECK (planning_horizon BETWEEN 1 AND 8),
    notifications jsonb NOT NULL DEFAULT '{}'::jsonb,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE clubs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    season_id uuid NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
    provider_id integer,
    name text NOT NULL,
    short_name text NOT NULL,
    UNIQUE (season_id, name),
    UNIQUE NULLS NOT DISTINCT (season_id, provider_id)
);

CREATE TABLE players (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    season_id uuid NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
    club_id uuid NOT NULL REFERENCES clubs(id),
    provider_id integer,
    first_name text NOT NULL,
    last_name text NOT NULL,
    position text NOT NULL CHECK (position IN ('GKP', 'DEF', 'MID', 'FWD')),
    price integer NOT NULL CHECK (price > 0),
    status text NOT NULL DEFAULT 'available',
    total_points integer NOT NULL DEFAULT 0,
    form numeric(6,2),
    expected_goals numeric(8,4),
    expected_assists numeric(8,4),
    expected_minutes numeric(6,2),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE NULLS NOT DISTINCT (season_id, provider_id)
);
CREATE INDEX players_search_idx ON players(season_id, position, price);
CREATE INDEX players_club_idx ON players(club_id);

CREATE TABLE fixtures (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    season_id uuid NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
    gameweek_id uuid REFERENCES gameweeks(id),
    provider_id integer,
    home_club_id uuid NOT NULL REFERENCES clubs(id),
    away_club_id uuid NOT NULL REFERENCES clubs(id),
    kickoff_at timestamptz,
    status text NOT NULL DEFAULT 'scheduled',
    home_difficulty smallint CHECK (home_difficulty BETWEEN 1 AND 5),
    away_difficulty smallint CHECK (away_difficulty BETWEEN 1 AND 5),
    UNIQUE NULLS NOT DISTINCT (season_id, provider_id),
    CHECK (home_club_id <> away_club_id)
);

CREATE TABLE user_squads (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    season_id uuid NOT NULL REFERENCES seasons(id),
    current_gameweek smallint NOT NULL,
    bank integer NOT NULL CHECK (bank >= 0),
    team_value integer NOT NULL CHECK (team_value >= 0),
    free_transfers smallint NOT NULL CHECK (free_transfers >= 0),
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    captain_id uuid REFERENCES players(id),
    vice_captain_id uuid REFERENCES players(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(user_id, season_id),
    CHECK (captain_id IS NULL OR captain_id <> vice_captain_id)
);

CREATE TABLE squad_players (
    squad_id uuid NOT NULL REFERENCES user_squads(id) ON DELETE CASCADE,
    player_id uuid NOT NULL REFERENCES players(id),
    purchase_price integer NOT NULL CHECK (purchase_price > 0),
    selling_price integer NOT NULL CHECK (selling_price > 0),
    is_starter boolean NOT NULL DEFAULT false,
    bench_order smallint CHECK (bench_order BETWEEN 1 AND 4),
    PRIMARY KEY(squad_id, player_id),
    CHECK ((is_starter AND bench_order IS NULL) OR (NOT is_starter AND bench_order IS NOT NULL))
);
CREATE UNIQUE INDEX squad_bench_order_unique ON squad_players(squad_id, bench_order) WHERE bench_order IS NOT NULL;

CREATE TABLE squad_snapshots (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    squad_id uuid NOT NULL REFERENCES user_squads(id) ON DELETE CASCADE,
    revision bigint NOT NULL,
    reason text NOT NULL,
    state jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(squad_id, revision)
);

CREATE TABLE squad_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    squad_id uuid NOT NULL REFERENCES user_squads(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    revision bigint NOT NULL,
    event_type text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX squad_events_squad_idx ON squad_events(squad_id, revision);

CREATE TABLE user_transfer_ledger (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    squad_id uuid NOT NULL REFERENCES user_squads(id) ON DELETE CASCADE,
    player_out_id uuid NOT NULL REFERENCES players(id),
    player_in_id uuid NOT NULL REFERENCES players(id),
    selling_price integer NOT NULL,
    purchase_price integer NOT NULL,
    bank_before integer NOT NULL,
    bank_after integer NOT NULL,
    points_hit integer NOT NULL DEFAULT 0,
    status text NOT NULL DEFAULT 'internal_confirmed' CHECK (status IN ('planned', 'internal_confirmed', 'marked_official')),
    idempotency_key text NOT NULL,
    squad_revision bigint NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(squad_id, idempotency_key)
);

CREATE TABLE outbox_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_type text NOT NULL,
    aggregate_id uuid NOT NULL,
    event_type text NOT NULL,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    payload jsonb NOT NULL,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz,
    attempts integer NOT NULL DEFAULT 0
);
CREATE INDEX outbox_unpublished_idx ON outbox_events(occurred_at) WHERE published_at IS NULL;

