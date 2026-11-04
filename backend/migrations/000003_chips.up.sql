CREATE TABLE season_chips (
 season_id uuid NOT NULL REFERENCES seasons(id),
 provider_id integer NOT NULL,
 name text NOT NULL CHECK (name IN ('wildcard','freehit','bboost','3xc')),
 start_gameweek integer NOT NULL,
 end_gameweek integer NOT NULL CHECK (end_gameweek >= start_gameweek),
 PRIMARY KEY (season_id,provider_id)
);
CREATE TABLE squad_chips (
 squad_id uuid NOT NULL REFERENCES user_squads(id),
 provider_id integer NOT NULL,
 name text NOT NULL,
 gameweek integer NOT NULL,
 backup jsonb,
 activated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY (squad_id,provider_id),
 UNIQUE(squad_id,gameweek)
);
ALTER TABLE user_transfer_ledger ADD COLUMN gameweek integer;
