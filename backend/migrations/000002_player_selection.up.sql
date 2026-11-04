ALTER TABLE players ADD COLUMN selected_by_percent numeric(5,2)
    CHECK (selected_by_percent BETWEEN 0 AND 100);
