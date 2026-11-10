CREATE TABLE live_gameweek_snapshots (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    season_id uuid NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
    gameweek smallint NOT NULL CHECK (gameweek BETWEEN 1 AND 60),
    payload_hash bytea NOT NULL,
    payload jsonb NOT NULL,
    status text NOT NULL CHECK (status IN ('scheduled', 'provisional', 'final')),
    fetched_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (season_id, gameweek, payload_hash)
);
CREATE INDEX live_gameweek_latest_idx ON live_gameweek_snapshots(season_id, gameweek, fetched_at DESC);

CREATE TABLE gameweek_squad_selections (
    squad_id uuid NOT NULL REFERENCES user_squads(id) ON DELETE CASCADE,
    gameweek smallint NOT NULL CHECK (gameweek BETWEEN 1 AND 60),
    squad_revision bigint NOT NULL,
    state jsonb NOT NULL,
    locked_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (squad_id, gameweek)
);

CREATE TABLE grounded_advisor_messages (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    squad_id uuid NOT NULL REFERENCES user_squads(id) ON DELETE CASCADE,
    recommendation_snapshot_id uuid REFERENCES recommendation_snapshots(id) ON DELETE SET NULL,
    question text NOT NULL CHECK (char_length(question) BETWEEN 1 AND 500),
    evidence jsonb NOT NULL,
    response jsonb NOT NULL,
    provider text NOT NULL CHECK (provider IN ('deterministic', 'openai')),
    model text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX grounded_advisor_messages_user_idx ON grounded_advisor_messages(user_id, created_at DESC);
