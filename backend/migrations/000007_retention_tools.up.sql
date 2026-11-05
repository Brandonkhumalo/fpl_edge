ALTER TABLE user_preferences
    ADD COLUMN timezone text NOT NULL DEFAULT 'UTC',
    ADD COLUMN deadline_reminder_hours smallint NOT NULL DEFAULT 24 CHECK (deadline_reminder_hours BETWEEN 1 AND 168),
    ADD COLUMN in_app_alerts boolean NOT NULL DEFAULT true,
    ADD COLUMN email_alerts boolean NOT NULL DEFAULT false,
    ADD COLUMN web_push_alerts boolean NOT NULL DEFAULT false;

CREATE TABLE player_watchlists (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    player_id uuid NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, player_id)
);
CREATE INDEX player_watchlists_player_idx ON player_watchlists(player_id);

CREATE TABLE watchlist_observations (
    user_id uuid NOT NULL,
    player_id uuid NOT NULL,
    price integer NOT NULL,
    status text NOT NULL,
    ownership numeric(6,2),
    forecast_points numeric(8,2),
    observed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, player_id),
    FOREIGN KEY (user_id, player_id) REFERENCES player_watchlists(user_id, player_id) ON DELETE CASCADE
);

CREATE TABLE user_notifications (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category text NOT NULL,
    title text NOT NULL,
    message text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    dedupe_key text NOT NULL,
    read_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, dedupe_key)
);
CREATE INDEX user_notifications_feed_idx ON user_notifications(user_id, created_at DESC);

CREATE TABLE notification_deliveries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    notification_id uuid NOT NULL REFERENCES user_notifications(id) ON DELETE CASCADE,
    channel text NOT NULL CHECK (channel IN ('email', 'web_push')),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'sent', 'failed', 'disabled')),
    attempts integer NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    last_error text,
    sent_at timestamptz,
    UNIQUE (notification_id, channel)
);
CREATE INDEX notification_deliveries_pending_idx ON notification_deliveries(next_attempt_at) WHERE status IN ('pending', 'failed');

CREATE TABLE recommendation_snapshots (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    squad_id uuid NOT NULL REFERENCES user_squads(id) ON DELETE CASCADE,
    gameweek smallint NOT NULL,
    squad_revision bigint NOT NULL,
    model_version text NOT NULL,
    data_updated_at timestamptz NOT NULL,
    expected_points numeric(8,2) NOT NULL,
    confidence text NOT NULL,
    recommendation jsonb NOT NULL,
    selected_action jsonb,
    selected_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (squad_id, gameweek, squad_revision, model_version, data_updated_at)
);
CREATE INDEX recommendation_snapshots_user_idx ON recommendation_snapshots(user_id, created_at DESC);

CREATE TABLE recommendation_outcomes (
    snapshot_id uuid PRIMARY KEY REFERENCES recommendation_snapshots(id) ON DELETE CASCADE,
    actual_points integer NOT NULL CHECK (actual_points >= 0),
    source text NOT NULL DEFAULT 'manual' CHECK (source IN ('manual', 'provider')),
    notes text NOT NULL DEFAULT '',
    settled_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE transfer_plans (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    squad_id uuid NOT NULL REFERENCES user_squads(id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    strategy text NOT NULL CHECK (strategy IN ('conservative', 'balanced', 'aggressive', 'chip_preparation', 'injury_response')),
    requested_horizon smallint NOT NULL CHECK (requested_horizon BETWEEN 3 AND 8),
    effective_horizon smallint NOT NULL CHECK (effective_horizon BETWEEN 1 AND 8),
    squad_revision bigint NOT NULL,
    assumptions jsonb NOT NULL,
    plan jsonb NOT NULL,
    expected_gain numeric(8,2) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX transfer_plans_user_idx ON transfer_plans(user_id, updated_at DESC);
