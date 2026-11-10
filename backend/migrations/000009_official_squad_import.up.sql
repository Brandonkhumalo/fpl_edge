CREATE TABLE official_squad_imports (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    squad_id uuid REFERENCES user_squads(id) ON DELETE SET NULL,
    season_id uuid REFERENCES seasons(id) ON DELETE SET NULL,
    manager_id bigint NOT NULL CHECK (manager_id > 0),
    provider_gameweek smallint CHECK (provider_gameweek BETWEEN 1 AND 60),
    status text NOT NULL CHECK (status IN ('fetching', 'preview', 'applied', 'failed')),
    expected_squad_revision bigint,
    payload jsonb,
    differences jsonb,
    failure_code text,
    fetched_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    applied_at timestamptz
);
CREATE INDEX official_squad_imports_user_created_idx
    ON official_squad_imports(user_id, created_at DESC);

CREATE TABLE official_squad_links (
    user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    manager_id bigint NOT NULL CHECK (manager_id > 0),
    latest_import_id uuid NOT NULL REFERENCES official_squad_imports(id),
    last_reconciled_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);
