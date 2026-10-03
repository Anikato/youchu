CREATE TABLE location_icons (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    svg TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

ALTER TABLE locations ADD COLUMN custom_icon_id INTEGER REFERENCES location_icons(id);

CREATE INDEX locations_custom_icon_id ON locations(custom_icon_id);
