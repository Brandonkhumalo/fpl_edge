ALTER TABLE clubs
    ADD COLUMN kit_code integer CHECK (kit_code IS NULL OR kit_code > 0);

ALTER TABLE players
    ADD COLUMN squad_number smallint CHECK (squad_number IS NULL OR squad_number BETWEEN 1 AND 999);
