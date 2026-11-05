ALTER TABLE user_squads ADD COLUMN chip_history_reviewed boolean NOT NULL DEFAULT false;

-- Historical usage does not activate a chip or carry a Free Hit squad backup.
CREATE TABLE prior_chip_usage (
    squad_id uuid NOT NULL REFERENCES user_squads(id) ON DELETE CASCADE,
    provider_id integer NOT NULL,
    gameweek integer CHECK (gameweek BETWEEN 1 AND 60),
    recorded_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (squad_id, provider_id),
    UNIQUE (squad_id, gameweek)
);
