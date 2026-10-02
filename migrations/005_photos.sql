CREATE TABLE photos (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    item_id INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    position INTEGER NOT NULL CHECK (position >= 0),
    width INTEGER NOT NULL CHECK (width >= 1),
    height INTEGER NOT NULL CHECK (height >= 1),
    byte_size INTEGER NOT NULL CHECK (byte_size >= 1),
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (item_id, position)
);

CREATE INDEX photos_item_id ON photos(item_id);
