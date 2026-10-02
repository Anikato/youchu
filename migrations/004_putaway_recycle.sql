ALTER TABLE items ADD COLUMN deleted_at TEXT;

CREATE INDEX items_deleted_at ON items(deleted_at) WHERE deleted_at IS NOT NULL;

CREATE TABLE return_tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    item_id INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    part_note TEXT CHECK (part_note IS NULL OR part_note <> ''),
    reason TEXT CHECK (reason IS NULL OR reason <> ''),
    destination_note TEXT CHECK (destination_note IS NULL OR destination_note <> ''),
    completed_at TEXT,
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX return_tasks_item_id ON return_tasks(item_id);
CREATE INDEX return_tasks_open ON return_tasks(item_id) WHERE completed_at IS NULL;
