CREATE TABLE locations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL CHECK (name <> ''),
    type TEXT NOT NULL CHECK (type IN ('area', 'fixed', 'movable')),
    code TEXT CHECK (code IS NULL OR code <> ''),
    parent_id INTEGER REFERENCES locations(id),
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    CHECK (parent_id IS NULL OR parent_id <> id),
    CHECK (
        (type = 'area' AND code IS NULL)
        OR (type IN ('fixed', 'movable') AND code IS NOT NULL)
    ),
    CHECK (type <> 'fixed' OR parent_id IS NOT NULL)
);

CREATE UNIQUE INDEX locations_code_unique ON locations(code) WHERE code IS NOT NULL;
CREATE INDEX locations_parent_id ON locations(parent_id);

CREATE TABLE items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL CHECK (name <> ''),
    alias TEXT CHECK (alias IS NULL OR alias <> ''),
    model TEXT CHECK (model IS NULL OR model <> ''),
    spec TEXT CHECK (spec IS NULL OR spec <> ''),
    quantity_note TEXT CHECK (quantity_note IS NULL OR quantity_note <> ''),
    note TEXT CHECK (note IS NULL OR note <> ''),
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX items_name_id ON items(name, id);

CREATE TABLE item_locations (
    item_id INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    location_id INTEGER NOT NULL REFERENCES locations(id),
    note TEXT CHECK (note IS NULL OR note <> ''),
    PRIMARY KEY (item_id, location_id)
);

CREATE INDEX item_locations_location_id ON item_locations(location_id);
