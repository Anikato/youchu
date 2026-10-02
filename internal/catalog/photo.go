package catalog

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

const (
	maxPhotosPerItem   = 20
	positionTempOffset = 100000
)

type Photo struct {
	ID        int64
	ItemID    int64
	Position  int
	Width     int
	Height    int
	ByteSize  int64
	Version   int64
	CreatedAt string
	UpdatedAt string
}

type CoverPhoto struct {
	ID int64
}

func AddPhoto(ctx context.Context, db *sql.DB, now time.Time, itemID int64, width, height int, byteSize int64, place func(id int64) error) (Photo, error) {
	var created Photo
	err := withImmediate(ctx, db, func(conn *sql.Conn) error {
		if _, err := liveItemName(ctx, conn, itemID); err != nil {
			return err
		}
		var count int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM photos WHERE item_id = ?`, itemID).Scan(&count); err != nil {
			return err
		}
		if count >= maxPhotosPerItem {
			return ErrPhotoLimit
		}
		ts := now.UTC().Format(time.RFC3339Nano)
		res, err := conn.ExecContext(ctx, `
INSERT INTO photos (item_id, position, width, height, byte_size, version, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, 1, ?, ?)`,
			itemID, count, width, height, byteSize, ts, ts)
		if err != nil {
			return err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		if err := place(id); err != nil {
			return err
		}
		created, err = loadPhoto(ctx, conn, id)
		return err
	})
	if err != nil {
		return Photo{}, err
	}
	return created, nil
}

func SetPhotoFirst(ctx context.Context, db *sql.DB, now time.Time, id int64, in VersionInput) (Photo, error) {
	var updated Photo
	err := withImmediate(ctx, db, func(conn *sql.Conn) error {
		cur, err := loadLivePhoto(ctx, conn, id)
		if err != nil {
			return err
		}
		if !in.VersionPresent || in.Version < 1 {
			return fieldError(map[string]string{"version": "版本不正确"})
		}
		if in.Version != cur.Version {
			return ErrVersion
		}
		photos, err := loadItemPhotos(ctx, conn, cur.ItemID)
		if err != nil {
			return err
		}
		ids := make([]int64, 0, len(photos))
		ids = append(ids, cur.ID)
		for _, p := range photos {
			if p.ID == cur.ID {
				continue
			}
			ids = append(ids, p.ID)
		}
		if err := rewritePositions(ctx, conn, cur.ItemID, ids); err != nil {
			return err
		}
		ts := now.UTC().Format(time.RFC3339Nano)
		res, err := conn.ExecContext(ctx, `
UPDATE photos SET version = version + 1, updated_at = ?
WHERE id = ? AND version = ?`, ts, id, cur.Version)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrVersion
		}
		updated, err = loadPhoto(ctx, conn, id)
		return err
	})
	if err != nil {
		return Photo{}, err
	}
	return updated, nil
}

func DeletePhoto(ctx context.Context, db *sql.DB, now time.Time, id, version int64) error {
	return withImmediate(ctx, db, func(conn *sql.Conn) error {
		cur, err := loadLivePhoto(ctx, conn, id)
		if err != nil {
			return err
		}
		if version != cur.Version {
			return ErrVersion
		}
		res, err := conn.ExecContext(ctx, `DELETE FROM photos WHERE id = ? AND version = ?`, id, version)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
		photos, err := loadItemPhotos(ctx, conn, cur.ItemID)
		if err != nil {
			return err
		}
		ids := make([]int64, 0, len(photos))
		for _, p := range photos {
			ids = append(ids, p.ID)
		}
		return rewritePositions(ctx, conn, cur.ItemID, ids)
	})
}

func loadItemPhotos(ctx context.Context, conn *sql.Conn, itemID int64) ([]Photo, error) {
	rows, err := conn.QueryContext(ctx, `
SELECT id, item_id, position, width, height, byte_size, version, created_at, updated_at
FROM photos WHERE item_id = ? ORDER BY position, id`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	photos := []Photo{}
	for rows.Next() {
		p, err := scanPhoto(rows)
		if err != nil {
			return nil, err
		}
		photos = append(photos, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return photos, nil
}

func loadCoverPhoto(ctx context.Context, conn *sql.Conn, itemID int64) (*CoverPhoto, error) {
	var id int64
	err := conn.QueryRowContext(ctx, `
SELECT p.id FROM photos p
WHERE p.item_id = ?
ORDER BY p.position, p.id
LIMIT 1`, itemID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &CoverPhoto{ID: id}, nil
}

func loadItemPhotoIDs(ctx context.Context, conn *sql.Conn, itemID int64) ([]int64, error) {
	rows, err := conn.QueryContext(ctx, `SELECT id FROM photos WHERE item_id = ?`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}

func loadLivePhoto(ctx context.Context, conn *sql.Conn, id int64) (Photo, error) {
	var p Photo
	var deleted sql.NullString
	err := conn.QueryRowContext(ctx, `
SELECT p.id, p.item_id, p.position, p.width, p.height, p.byte_size, p.version, p.created_at, p.updated_at, i.deleted_at
FROM photos p
JOIN items i ON i.id = p.item_id
WHERE p.id = ?`, id).Scan(
		&p.ID, &p.ItemID, &p.Position, &p.Width, &p.Height, &p.ByteSize, &p.Version, &p.CreatedAt, &p.UpdatedAt, &deleted)
	if errors.Is(err, sql.ErrNoRows) {
		return Photo{}, ErrNotFound
	}
	if err != nil {
		return Photo{}, err
	}
	if deleted.Valid && deleted.String != "" {
		return Photo{}, ErrNotFound
	}
	return p, nil
}

func GetPhoto(ctx context.Context, db *sql.DB, id int64) (Photo, error) {
	var p Photo
	err := withRead(ctx, db, func(conn *sql.Conn) error {
		var err error
		p, err = loadPhoto(ctx, conn, id)
		return err
	})
	if err != nil {
		return Photo{}, err
	}
	return p, nil
}

func loadPhoto(ctx context.Context, conn *sql.Conn, id int64) (Photo, error) {
	p, err := scanPhoto(conn.QueryRowContext(ctx, `
SELECT id, item_id, position, width, height, byte_size, version, created_at, updated_at
FROM photos WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Photo{}, ErrNotFound
	}
	if err != nil {
		return Photo{}, err
	}
	return p, nil
}

func scanPhoto(sc rowScanner) (Photo, error) {
	var p Photo
	if err := sc.Scan(&p.ID, &p.ItemID, &p.Position, &p.Width, &p.Height, &p.ByteSize, &p.Version, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return Photo{}, err
	}
	return p, nil
}

func rewritePositions(ctx context.Context, conn *sql.Conn, itemID int64, ids []int64) error {
	if _, err := conn.ExecContext(ctx, `UPDATE photos SET position = position + ? WHERE item_id = ?`, positionTempOffset, itemID); err != nil {
		return err
	}
	for i, id := range ids {
		if _, err := conn.ExecContext(ctx, `UPDATE photos SET position = ? WHERE id = ? AND item_id = ?`, i, id, itemID); err != nil {
			return err
		}
	}
	return nil
}
