ALTER TABLE locations ADD COLUMN icon TEXT CHECK (icon IS NULL OR icon <> '');
