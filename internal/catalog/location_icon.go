package catalog

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

const maxLocationIcons = 40

type LocationIcon struct {
	ID        int64
	Name      string
	SVG       string
	Version   int64
	CreatedAt string
	UpdatedAt string
}

func ListLocationIcons(ctx context.Context, db *sql.DB) ([]LocationIcon, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, name, svg, version, created_at, updated_at FROM location_icons ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]LocationIcon, 0)
	for rows.Next() {
		icon, err := scanLocationIcon(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, icon)
	}
	return out, rows.Err()
}

func GetLocationIcon(ctx context.Context, db *sql.DB, id int64) (LocationIcon, error) {
	icon, err := scanLocationIcon(db.QueryRowContext(ctx, `SELECT id, name, svg, version, created_at, updated_at FROM location_icons WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return LocationIcon{}, ErrNotFound
	}
	return icon, err
}

func CreateLocationIcon(ctx context.Context, db *sql.DB, now time.Time, name, svg string) (LocationIcon, error) {
	n, nerr := NormalizeName(name)
	clean, svgMsg := SanitizeSVG(svg)
	fields := map[string]string{}
	if nerr != nil {
		fields["name"] = nerr.Error()
	}
	if svgMsg != "" {
		fields["svg"] = svgMsg
	}
	if err := fieldError(fields); err != nil {
		return LocationIcon{}, err
	}
	var created LocationIcon
	err := withImmediate(ctx, db, func(conn *sql.Conn) error {
		var count int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM location_icons`).Scan(&count); err != nil {
			return err
		}
		if count >= maxLocationIcons {
			return fieldError(map[string]string{"svg": "图标已满"})
		}
		ts := now.UTC().Format(time.RFC3339Nano)
		res, err := conn.ExecContext(ctx, `
INSERT INTO location_icons (name, svg, version, created_at, updated_at)
VALUES (?, ?, 1, ?, ?)`, n, clean, ts, ts)
		if err != nil {
			return err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		created, err = scanLocationIcon(conn.QueryRowContext(ctx, `SELECT id, name, svg, version, created_at, updated_at FROM location_icons WHERE id = ?`, id))
		return err
	})
	return created, err
}

func UpdateLocationIcon(ctx context.Context, db *sql.DB, now time.Time, id int64, version int64, name, svg OptionalText) (LocationIcon, error) {
	fields := map[string]string{}
	if version < 1 {
		fields["version"] = "版本不正确"
	}
	if !name.Present && !svg.Present {
		fields["request"] = "没有要修改的内容"
	}
	var nextName, nextSVG string
	if name.Present {
		if name.Null {
			fields["name"] = "请填写名称"
		} else if n, err := NormalizeName(name.Value); err != nil {
			fields["name"] = err.Error()
		} else {
			nextName = n
		}
	}
	if svg.Present {
		if svg.Null {
			fields["svg"] = "图标不正确"
		} else if clean, msg := SanitizeSVG(svg.Value); msg != "" {
			fields["svg"] = msg
		} else {
			nextSVG = clean
		}
	}
	if err := fieldError(fields); err != nil {
		return LocationIcon{}, err
	}
	var updated LocationIcon
	err := withImmediate(ctx, db, func(conn *sql.Conn) error {
		cur, err := scanLocationIcon(conn.QueryRowContext(ctx, `SELECT id, name, svg, version, created_at, updated_at FROM location_icons WHERE id = ?`, id))
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if version != cur.Version {
			return ErrVersion
		}
		if !name.Present {
			nextName = cur.Name
		}
		if !svg.Present {
			nextSVG = cur.SVG
		}
		ts := now.UTC().Format(time.RFC3339Nano)
		res, err := conn.ExecContext(ctx, `
UPDATE location_icons SET name = ?, svg = ?, version = version + 1, updated_at = ?
WHERE id = ? AND version = ?`, nextName, nextSVG, ts, id, cur.Version)
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
		updated, err = scanLocationIcon(conn.QueryRowContext(ctx, `SELECT id, name, svg, version, created_at, updated_at FROM location_icons WHERE id = ?`, id))
		return err
	})
	return updated, err
}

func DeleteLocationIcon(ctx context.Context, db *sql.DB, id, version int64) error {
	if version < 1 {
		return fieldError(map[string]string{"version": "版本不正确"})
	}
	return withImmediate(ctx, db, func(conn *sql.Conn) error {
		var curVersion int64
		err := conn.QueryRowContext(ctx, `SELECT version FROM location_icons WHERE id = ?`, id).Scan(&curVersion)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if version != curVersion {
			return ErrVersion
		}
		var n int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM locations WHERE custom_icon_id = ?`, id).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return ErrIconInUse
		}
		res, err := conn.ExecContext(ctx, `DELETE FROM location_icons WHERE id = ? AND version = ?`, id, version)
		if err != nil {
			return err
		}
		gone, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if gone == 0 {
			return ErrNotFound
		}
		return nil
	})
}

func parseCustomIcon(in OptionalID) (*int64, string) {
	if !in.Present {
		return nil, ""
	}
	if in.Null {
		return nil, ""
	}
	if in.Value < 1 {
		return nil, "图标不存在"
	}
	v := in.Value
	return &v, ""
}

func ensureCustomIcon(ctx context.Context, conn *sql.Conn, id int64) error {
	var n int
	err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM location_icons WHERE id = ?`, id).Scan(&n)
	if err != nil {
		return err
	}
	if n == 0 {
		return fieldError(map[string]string{"custom_icon_id": "图标不存在"})
	}
	return nil
}

type iconScanner interface {
	Scan(dest ...any) error
}

func scanLocationIcon(sc iconScanner) (LocationIcon, error) {
	var icon LocationIcon
	err := sc.Scan(&icon.ID, &icon.Name, &icon.SVG, &icon.Version, &icon.CreatedAt, &icon.UpdatedAt)
	return icon, err
}
