CREATE TABLE categories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL CHECK (name <> ''),
    parent_id INTEGER REFERENCES categories(id),
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    CHECK (parent_id IS NULL OR parent_id <> id)
);

CREATE UNIQUE INDEX categories_root_name
    ON categories(name) WHERE parent_id IS NULL;
CREATE UNIQUE INDEX categories_sibling_name
    ON categories(parent_id, name) WHERE parent_id IS NOT NULL;
CREATE INDEX categories_parent_id ON categories(parent_id);

CREATE TABLE item_categories (
    item_id INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    category_id INTEGER NOT NULL REFERENCES categories(id),
    PRIMARY KEY (item_id, category_id)
);

CREATE INDEX item_categories_category_id ON item_categories(category_id);
