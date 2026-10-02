package catalog

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type ReturnTaskInput struct {
	PartNote        OptionalText
	Reason          OptionalText
	DestinationNote OptionalText
}

type ReturnTaskList struct {
	Tasks  []ReturnTask
	Total  int
	Limit  int
	Offset int
}

const returnTaskCols = `t.id, t.item_id, i.name, t.part_note, t.reason, t.destination_note, t.completed_at, t.version, t.created_at, t.updated_at`

func CreateReturnTask(ctx context.Context, db *sql.DB, now time.Time, itemID int64, in ReturnTaskInput) (ReturnTask, error) {
	var created ReturnTask
	err := withImmediate(ctx, db, func(conn *sql.Conn) error {
		if _, err := liveItemName(ctx, conn, itemID); err != nil {
			return err
		}
		fields := map[string]string{}
		part := assignText("part_note", in.PartNote, NormalizePartNote, fields)
		reason := assignText("reason", in.Reason, NormalizeReason, fields)
		dest := assignText("destination_note", in.DestinationNote, NormalizeDestinationNote, fields)
		if ferr := fieldError(fields); ferr != nil {
			return ferr
		}
		ts := now.UTC().Format(time.RFC3339Nano)
		res, err := conn.ExecContext(ctx, `
INSERT INTO return_tasks (item_id, part_note, reason, destination_note, completed_at, version, created_at, updated_at)
VALUES (?, ?, ?, ?, NULL, 1, ?, ?)`,
			itemID, sqlText(part), sqlText(reason), sqlText(dest), ts, ts)
		if err != nil {
			return err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		created, err = loadReturnTask(ctx, conn, id)
		return err
	})
	if err != nil {
		return ReturnTask{}, err
	}
	return created, nil
}

func ListOpenReturnTasks(ctx context.Context, db *sql.DB, limit, offset int) (ReturnTaskList, error) {
	result := ReturnTaskList{Limit: limit, Offset: offset, Tasks: []ReturnTask{}}
	err := withRead(ctx, db, func(conn *sql.Conn) error {
		const where = `
FROM return_tasks t
JOIN items i ON i.id = t.item_id
WHERE t.completed_at IS NULL AND i.deleted_at IS NULL`
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) `+where).Scan(&result.Total); err != nil {
			return err
		}
		rows, err := conn.QueryContext(ctx, `SELECT `+returnTaskCols+where+` ORDER BY t.created_at, t.id LIMIT ? OFFSET ?`, limit, offset)
		if err != nil {
			return err
		}
		defer rows.Close()
		tasks := []ReturnTask{}
		for rows.Next() {
			task, err := scanReturnTask(rows)
			if err != nil {
				return err
			}
			tasks = append(tasks, task)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		for i := range tasks {
			cover, err := loadCoverPhoto(ctx, conn, tasks[i].ItemID)
			if err != nil {
				return err
			}
			tasks[i].CoverPhoto = cover
		}
		result.Tasks = tasks
		return nil
	})
	if err != nil {
		return ReturnTaskList{}, err
	}
	return result, nil
}

func GetReturnTask(ctx context.Context, db *sql.DB, id int64) (ReturnTask, error) {
	var task ReturnTask
	err := withRead(ctx, db, func(conn *sql.Conn) error {
		var err error
		task, err = loadReturnTask(ctx, conn, id)
		return err
	})
	if err != nil {
		return ReturnTask{}, err
	}
	return task, nil
}

func CompleteReturnTask(ctx context.Context, db *sql.DB, now time.Time, id int64, in VersionInput) (ReturnTask, error) {
	var completed ReturnTask
	err := withImmediate(ctx, db, func(conn *sql.Conn) error {
		cur, err := loadReturnTask(ctx, conn, id)
		if err != nil {
			return err
		}
		if !in.VersionPresent || in.Version < 1 {
			return fieldError(map[string]string{"version": "版本不正确"})
		}
		if in.Version != cur.Version {
			return ErrVersion
		}
		if cur.CompletedAt != nil {
			return ErrAlreadyCompleted
		}
		ts := now.UTC().Format(time.RFC3339Nano)
		res, err := conn.ExecContext(ctx, `
UPDATE return_tasks
SET completed_at = ?, version = version + 1, updated_at = ?
WHERE id = ? AND version = ? AND completed_at IS NULL`, ts, ts, id, cur.Version)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return returnTaskConflictOrMissing(ctx, conn, id)
		}
		completed, err = loadReturnTask(ctx, conn, id)
		return err
	})
	if err != nil {
		return ReturnTask{}, err
	}
	return completed, nil
}

func DeleteReturnTask(ctx context.Context, db *sql.DB, id, version int64) error {
	return withImmediate(ctx, db, func(conn *sql.Conn) error {
		cur, err := loadReturnTask(ctx, conn, id)
		if err != nil {
			return err
		}
		if version != cur.Version {
			return ErrVersion
		}
		if cur.CompletedAt != nil {
			return ErrAlreadyCompleted
		}
		res, err := conn.ExecContext(ctx, `DELETE FROM return_tasks WHERE id = ? AND version = ? AND completed_at IS NULL`, id, version)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return returnTaskConflictOrMissing(ctx, conn, id)
		}
		return nil
	})
}

func liveItemName(ctx context.Context, conn *sql.Conn, id int64) (string, error) {
	var name string
	var deleted sql.NullString
	err := conn.QueryRowContext(ctx, `SELECT name, deleted_at FROM items WHERE id = ?`, id).Scan(&name, &deleted)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if deleted.Valid && deleted.String != "" {
		return "", ErrNotFound
	}
	return name, nil
}

func loadReturnTask(ctx context.Context, conn *sql.Conn, id int64) (ReturnTask, error) {
	var deleted sql.NullString
	task, err := scanReturnTask(conn.QueryRowContext(ctx, `
SELECT `+returnTaskCols+`, i.deleted_at
FROM return_tasks t
JOIN items i ON i.id = t.item_id
WHERE t.id = ?`, id), &deleted)
	if errors.Is(err, sql.ErrNoRows) {
		return ReturnTask{}, ErrNotFound
	}
	if err != nil {
		return ReturnTask{}, err
	}
	if deleted.Valid && deleted.String != "" {
		return ReturnTask{}, ErrNotFound
	}
	cover, err := loadCoverPhoto(ctx, conn, task.ItemID)
	if err != nil {
		return ReturnTask{}, err
	}
	task.CoverPhoto = cover
	return task, nil
}

func loadItemReturnTasks(ctx context.Context, conn *sql.Conn, itemID int64, itemName string) ([]ReturnTask, error) {
	cover, err := loadCoverPhoto(ctx, conn, itemID)
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
SELECT id, item_id, part_note, reason, destination_note, completed_at, version, created_at, updated_at
FROM return_tasks
WHERE item_id = ?
ORDER BY CASE WHEN completed_at IS NULL THEN 0 ELSE 1 END, id`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := []ReturnTask{}
	for rows.Next() {
		var task ReturnTask
		var part, reason, dest, completed sql.NullString
		if err := rows.Scan(&task.ID, &task.ItemID, &part, &reason, &dest, &completed, &task.Version, &task.CreatedAt, &task.UpdatedAt); err != nil {
			return nil, err
		}
		task.ItemName = itemName
		task.PartNote = nullStringPtr(part)
		task.Reason = nullStringPtr(reason)
		task.DestinationNote = nullStringPtr(dest)
		task.CompletedAt = nullStringPtr(completed)
		task.CoverPhoto = cover
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tasks, nil
}

func scanReturnTask(sc rowScanner, extra ...any) (ReturnTask, error) {
	var task ReturnTask
	var part, reason, dest, completed sql.NullString
	dests := []any{&task.ID, &task.ItemID, &task.ItemName, &part, &reason, &dest, &completed, &task.Version, &task.CreatedAt, &task.UpdatedAt}
	dests = append(dests, extra...)
	if err := sc.Scan(dests...); err != nil {
		return ReturnTask{}, err
	}
	task.PartNote = nullStringPtr(part)
	task.Reason = nullStringPtr(reason)
	task.DestinationNote = nullStringPtr(dest)
	task.CompletedAt = nullStringPtr(completed)
	return task, nil
}

func returnTaskConflictOrMissing(ctx context.Context, conn *sql.Conn, id int64) error {
	var version int64
	var completed, deleted sql.NullString
	err := conn.QueryRowContext(ctx, `
SELECT t.version, t.completed_at, i.deleted_at
FROM return_tasks t
JOIN items i ON i.id = t.item_id
WHERE t.id = ?`, id).Scan(&version, &completed, &deleted)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if deleted.Valid && deleted.String != "" {
		return ErrNotFound
	}
	return ErrVersion
}
