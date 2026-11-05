CREATE TABLE football_forecast_sources (
    season_id uuid PRIMARY KEY REFERENCES seasons(id),
    fetched_at timestamptz NOT NULL DEFAULT now(),
    payload jsonb NOT NULL
);
