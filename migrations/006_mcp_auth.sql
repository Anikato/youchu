CREATE TABLE access_tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL CHECK (name <> ''),
    token_hash TEXT NOT NULL UNIQUE,
    token_prefix TEXT NOT NULL,
    scopes TEXT NOT NULL,
    created_at TEXT NOT NULL
);

ALTER TABLE item_categories
    ADD COLUMN source TEXT NOT NULL DEFAULT 'human'
    CHECK (source IN ('human', 'ai'));
