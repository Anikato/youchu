package catalog

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

type CategoryInput struct {
	Name           OptionalText
	Parent         OptionalID
	VersionPresent bool
	Version        int64
}

type CategoryPathNode struct {
	ID   int64
	Name string
}

type Category struct {
	ID              int64
	Name            string
	ParentID        *int64
	Version         int64
	CreatedAt       string
	UpdatedAt       string
	Path            []CategoryPathNode
	DirectItemCount int
}

type CategoryFilter struct {
	Kind       ListKind
	ParentID   int64
	ExcludeID  int64
	HasExclude bool
	Limit      int
	Offset     int
}

type CategoryList struct {
	Categories []Category
	Total      int
	Limit      int
	Offset     int
}

const categoryCols = `id, name, parent_id, version, created_at, updated_at`

func CreateCategory(ctx context.Context, db *sql.DB, now time.Time, in CategoryInput) (Category, error) {
	name, err := validateCategoryCreate(in)
	if err != nil {
		return Category{}, err
	}
	var created Category
	err = withImmediate(ctx, db, func(conn *sql.Conn) error {
		parent, err := resolveCategoryParent(ctx, conn, 0, in.Parent)
		if err != nil {
			return err
		}
		if err := ensureCategoryNameFree(ctx, conn, 0, parent, name); err != nil {
			return err
		}
		ts := now.UTC().Format(time.RFC3339Nano)
		res, err := conn.ExecContext(ctx, `
INSERT INTO categories (name, parent_id, version, created_at, updated_at)
VALUES (?, ?, 1, ?, ?)`, name, categoryParentArg(parent), ts, ts)
		if err != nil {
			return mapCategoryConstraint(err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		created, err = loadCategory(ctx, conn, id)
		return err
	})
	if err != nil {
		return Category{}, err
	}
	return created, nil
}

func UpdateCategory(ctx context.Context, db *sql.DB, now time.Time, id int64, in CategoryInput) (Category, error) {
	var updated Category
	err := withImmediate(ctx, db, func(conn *sql.Conn) error {
		cat, err := loadCategory(ctx, conn, id)
		if err != nil {
			return err
		}
		name, err := resolveCategoryUpdate(cat, in)
		if err != nil {
			return err
		}
		// Version is compared before parent, cycle, and sibling-name checks.
		if in.Version != cat.Version {
			return ErrVersion
		}
		parent := cat.ParentID
		if in.Parent.Present {
			parent, err = resolveCategoryParent(ctx, conn, cat.ID, in.Parent)
			if err != nil {
				return err
			}
		}
		if err := ensureCategoryNameFree(ctx, conn, cat.ID, parent, name); err != nil {
			return err
		}
		ts := now.UTC().Format(time.RFC3339Nano)
		res, err := conn.ExecContext(ctx, `
UPDATE categories
SET name = ?, parent_id = ?, version = version + 1, updated_at = ?
WHERE id = ? AND version = ?`, name, categoryParentArg(parent), ts, id, cat.Version)
		if err != nil {
			return mapCategoryConstraint(err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return categoryConflictOrMissing(ctx, conn, id)
		}
		updated, err = loadCategory(ctx, conn, id)
		return err
	})
	if err != nil {
		return Category{}, err
	}
	return updated, nil
}

func DeleteCategory(ctx context.Context, db *sql.DB, id, version int64) error {
	return withImmediate(ctx, db, func(conn *sql.Conn) error {
		var stored int64
		err := conn.QueryRowContext(ctx, `SELECT version FROM categories WHERE id = ?`, id).Scan(&stored)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if stored != version {
			return ErrVersion
		}
		var children, links int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM categories WHERE parent_id = ?`, id).Scan(&children); err != nil {
			return err
		}
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM item_categories WHERE category_id = ?`, id).Scan(&links); err != nil {
			return err
		}
		if children > 0 || links > 0 {
			return ErrInUse
		}
		res, err := conn.ExecContext(ctx, `DELETE FROM categories WHERE id = ? AND version = ?`, id, version)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return categoryConflictOrMissing(ctx, conn, id)
		}
		return nil
	})
}

func GetCategory(ctx context.Context, db *sql.DB, id int64) (Category, error) {
	var cat Category
	err := withRead(ctx, db, func(conn *sql.Conn) error {
		var err error
		cat, err = loadCategory(ctx, conn, id)
		return err
	})
	if err != nil {
		return Category{}, err
	}
	return cat, nil
}

func ListCategories(ctx context.Context, db *sql.DB, f CategoryFilter) (CategoryList, error) {
	result := CategoryList{Limit: f.Limit, Offset: f.Offset, Categories: []Category{}}
	err := withRead(ctx, db, func(conn *sql.Conn) error {
		if f.Kind == ListChildren {
			ok, err := categoryExists(ctx, conn, f.ParentID)
			if err != nil {
				return err
			}
			if !ok {
				return ErrNotFound
			}
		}
		if f.Kind == ListEligible && f.HasExclude {
			ok, err := categoryExists(ctx, conn, f.ExcludeID)
			if err != nil {
				return err
			}
			if !ok {
				return ErrNotFound
			}
		}
		countSQL, pageSQL, args := categoryListSQL(f)
		if err := conn.QueryRowContext(ctx, countSQL, args...).Scan(&result.Total); err != nil {
			return err
		}
		pageArgs := append(append([]any{}, args...), f.Limit, f.Offset)
		rows, err := conn.QueryContext(ctx, pageSQL, pageArgs...)
		if err != nil {
			return err
		}
		defer rows.Close()
		cats := []Category{}
		for rows.Next() {
			cat, err := scanCategory(rows)
			if err != nil {
				return err
			}
			cats = append(cats, cat)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		for i := range cats {
			cats[i].Path, err = loadCategoryPath(ctx, conn, cats[i].ID)
			if err != nil {
				return err
			}
			if err := loadCategoryDirectCount(ctx, conn, &cats[i]); err != nil {
				return err
			}
		}
		result.Categories = cats
		return nil
	})
	if err != nil {
		return CategoryList{}, err
	}
	return result, nil
}

func validateCategoryCreate(in CategoryInput) (string, error) {
	fields := map[string]string{}
	name := ""
	if !in.Name.Present || in.Name.Null {
		fields["name"] = "请填写名称"
	} else if n, nerr := NormalizeName(in.Name.Value); nerr != nil {
		fields["name"] = nerr.Error()
	} else {
		name = n
	}
	return name, fieldError(fields)
}

func resolveCategoryUpdate(cat Category, in CategoryInput) (string, error) {
	fields := map[string]string{}
	if !in.VersionPresent || in.Version < 1 {
		fields["version"] = "版本不正确"
	}
	if !in.Name.Present && !in.Parent.Present {
		fields["request"] = "没有要修改的内容"
	}
	name := cat.Name
	if in.Name.Present {
		if in.Name.Null {
			fields["name"] = "请填写名称"
		} else if n, nerr := NormalizeName(in.Name.Value); nerr != nil {
			fields["name"] = nerr.Error()
		} else {
			name = n
		}
	}
	return name, fieldError(fields)
}

func resolveCategoryParent(ctx context.Context, conn *sql.Conn, id int64, parent OptionalID) (*int64, error) {
	if !parent.Present || parent.Null {
		return nil, nil
	}
	if err := ensureCategoryParent(ctx, conn, id, parent.Value); err != nil {
		return nil, err
	}
	v := parent.Value
	return &v, nil
}

func ensureCategoryParent(ctx context.Context, conn *sql.Conn, id, parentID int64) error {
	if parentID < 1 {
		return ErrParent
	}
	seen := map[int64]struct{}{}
	cur := parentID
	for {
		if cur == id {
			return ErrCycle
		}
		if _, ok := seen[cur]; ok {
			break
		}
		seen[cur] = struct{}{}
		var next sql.NullInt64
		err := conn.QueryRowContext(ctx, `SELECT parent_id FROM categories WHERE id = ?`, cur).Scan(&next)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrParent
		}
		if err != nil {
			return err
		}
		if !next.Valid {
			break
		}
		cur = next.Int64
	}
	return nil
}

func ensureCategoryNameFree(ctx context.Context, conn *sql.Conn, id int64, parent *int64, name string) error {
	var n int
	var err error
	if parent == nil {
		err = conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM categories WHERE parent_id IS NULL AND name = ? AND id != ?`, name, id).Scan(&n)
	} else {
		err = conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM categories WHERE parent_id = ? AND name = ? AND id != ?`, *parent, name, id).Scan(&n)
	}
	if err != nil {
		return err
	}
	if n > 0 {
		return ErrNameTaken
	}
	return nil
}

func mapCategoryConstraint(err error) error {
	if strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return ErrNameTaken
	}
	return err
}

func categoryParentArg(parent *int64) any {
	if parent == nil {
		return nil
	}
	return *parent
}

func categoryConflictOrMissing(ctx context.Context, conn *sql.Conn, id int64) error {
	var version int64
	err := conn.QueryRowContext(ctx, `SELECT version FROM categories WHERE id = ?`, id).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	return ErrVersion
}

func categoryExists(ctx context.Context, conn *sql.Conn, id int64) (bool, error) {
	var one int
	err := conn.QueryRowContext(ctx, `SELECT 1 FROM categories WHERE id = ?`, id).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func loadCategory(ctx context.Context, conn *sql.Conn, id int64) (Category, error) {
	cat, err := scanCategory(conn.QueryRowContext(ctx, `SELECT `+categoryCols+` FROM categories WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Category{}, ErrNotFound
	}
	if err != nil {
		return Category{}, err
	}
	cat.Path, err = loadCategoryPath(ctx, conn, cat.ID)
	if err != nil {
		return Category{}, err
	}
	if err := loadCategoryDirectCount(ctx, conn, &cat); err != nil {
		return Category{}, err
	}
	return cat, nil
}

func loadCategoryDirectCount(ctx context.Context, conn *sql.Conn, cat *Category) error {
	return conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM item_categories WHERE category_id = ?`, cat.ID).Scan(&cat.DirectItemCount)
}

func scanCategory(sc rowScanner) (Category, error) {
	var cat Category
	var parent sql.NullInt64
	if err := sc.Scan(&cat.ID, &cat.Name, &parent, &cat.Version, &cat.CreatedAt, &cat.UpdatedAt); err != nil {
		return Category{}, err
	}
	if parent.Valid {
		id := parent.Int64
		cat.ParentID = &id
	}
	return cat, nil
}

func loadCategoryPath(ctx context.Context, conn *sql.Conn, id int64) ([]CategoryPathNode, error) {
	var chain []CategoryPathNode
	seen := map[int64]struct{}{}
	for {
		if _, ok := seen[id]; ok {
			return nil, errors.New("category path repeats an id")
		}
		seen[id] = struct{}{}
		var node CategoryPathNode
		var parent sql.NullInt64
		err := conn.QueryRowContext(ctx, `SELECT id, name, parent_id FROM categories WHERE id = ?`, id).
			Scan(&node.ID, &node.Name, &parent)
		if err != nil {
			return nil, err
		}
		chain = append(chain, node)
		if !parent.Valid {
			break
		}
		id = parent.Int64
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain, nil
}

func categoryListSQL(f CategoryFilter) (countSQL, pageSQL string, args []any) {
	switch f.Kind {
	case ListChildren:
		where := `parent_id = ?`
		return `SELECT COUNT(*) FROM categories WHERE ` + where,
			`SELECT ` + categoryCols + ` FROM categories WHERE ` + where + ` ORDER BY name, id LIMIT ? OFFSET ?`,
			[]any{f.ParentID}
	case ListFlat:
		return `SELECT COUNT(*) FROM categories`,
			`SELECT ` + categoryCols + ` FROM categories ORDER BY id LIMIT ? OFFSET ?`,
			nil
	case ListEligible:
		if f.HasExclude {
			cte := `WITH RECURSIVE excluded(id) AS (
    SELECT id FROM categories WHERE id = ?
    UNION ALL
    SELECT child.id FROM categories AS child JOIN excluded ON child.parent_id = excluded.id
) `
			where := `id NOT IN (SELECT id FROM excluded)`
			return cte + `SELECT COUNT(*) FROM categories WHERE ` + where,
				cte + `SELECT ` + categoryCols + ` FROM categories WHERE ` + where + ` ORDER BY id LIMIT ? OFFSET ?`,
				[]any{f.ExcludeID}
		}
		return `SELECT COUNT(*) FROM categories`,
			`SELECT ` + categoryCols + ` FROM categories ORDER BY id LIMIT ? OFFSET ?`,
			nil
	default:
		where := `parent_id IS NULL`
		return `SELECT COUNT(*) FROM categories WHERE ` + where,
			`SELECT ` + categoryCols + ` FROM categories WHERE ` + where + ` ORDER BY name, id LIMIT ? OFFSET ?`,
			nil
	}
}
