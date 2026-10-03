package catalog

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

type OptionalText struct {
	Present bool
	Null    bool
	Value   string
}

type OptionalID struct {
	Present bool
	Null    bool
	Value   int64
}

type CreateInput struct {
	Name   OptionalText
	Type   OptionalText
	Code   OptionalText
	Parent OptionalID
	Icon   OptionalText
	Custom OptionalID
}

type UpdateInput struct {
	Name           OptionalText
	Type           OptionalText
	Code           OptionalText
	Parent         OptionalID
	Icon           OptionalText
	Custom         OptionalID
	VersionPresent bool
	Version        int64
}

type Location struct {
	ID              int64
	Name            string
	Type            string
	Code            *string
	ParentID        *int64
	Icon            *string
	CustomIconID    *int64
	Version         int64
	CreatedAt       string
	UpdatedAt       string
	Path            []PathNode
	DirectItemCount int
}

type PathNode struct {
	ID           int64
	Name         string
	Type         string
	Code         *string
	Icon         *string
	CustomIconID *int64
}

type ListKind int

const (
	ListRoots ListKind = iota
	ListChildren
	ListFlat
	ListEligible
)

type ListFilter struct {
	Kind       ListKind
	ParentID   int64
	Eligible   string
	ExcludeID  int64
	HasExclude bool
	Limit      int
	Offset     int
}

type ListResult struct {
	Locations []Location
	Total     int
	Limit     int
	Offset    int
}

const locationCols = `id, name, type, code, parent_id, icon, custom_icon_id, version, created_at, updated_at`

const siblingOrder = `CASE type WHEN 'area' THEN 0 WHEN 'fixed' THEN 1 ELSE 2 END, code IS NULL, code, name, id`

func CreateLocation(ctx context.Context, db *sql.DB, now time.Time, in CreateInput) (Location, error) {
	name, typ, code, parentID, hasParent, icon, custom, err := validateCreate(in)
	if err != nil {
		return Location{}, err
	}
	var created Location
	err = withImmediate(ctx, db, func(conn *sql.Conn) error {
		if hasParent {
			var parentType string
			err := conn.QueryRowContext(ctx, `SELECT type FROM locations WHERE id = ?`, parentID).Scan(&parentType)
			if errors.Is(err, sql.ErrNoRows) {
				return ErrParent
			}
			if err != nil {
				return err
			}
			if !parentPairOK(typ, parentType) {
				return ErrParent
			}
		}
		if custom != nil {
			if err := ensureCustomIcon(ctx, conn, *custom); err != nil {
				return err
			}
		}
		ts := now.UTC().Format(time.RFC3339Nano)
		var codeArg any
		if code != "" {
			codeArg = code
		}
		var parentArg any
		if hasParent {
			parentArg = parentID
		}
		var iconArg any
		if icon != nil {
			iconArg = *icon
		}
		var customArg any
		if custom != nil {
			customArg = *custom
		}
		res, err := conn.ExecContext(ctx, `
INSERT INTO locations (name, type, code, parent_id, icon, custom_icon_id, version, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?)`, name, typ, codeArg, parentArg, iconArg, customArg, ts, ts)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				return ErrCodeTaken
			}
			return err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		created, err = loadLocation(ctx, conn, id)
		return err
	})
	if err != nil {
		return Location{}, err
	}
	return created, nil
}

func CloneLocation(ctx context.Context, db *sql.DB, now time.Time, id int64) (Location, error) {
	var created Location
	err := withImmediate(ctx, db, func(conn *sql.Conn) error {
		loc, err := loadLocation(ctx, conn, id)
		if err != nil {
			return err
		}
		if loc.Type != "movable" {
			return fieldError(map[string]string{"type": "只有移动容器可以克隆"})
		}
		if loc.Code == nil {
			return fieldError(map[string]string{"code": "编号无法递增"})
		}
		code := *loc.Code
		chosen := ""
		for range 10000 {
			next, err := NextBoxCode(code)
			if err != nil {
				return err
			}
			taken, err := codeTaken(ctx, conn, next)
			if err != nil {
				return err
			}
			if !taken {
				chosen = next
				break
			}
			code = next
		}
		if chosen == "" {
			return fieldError(map[string]string{"code": "编号无法递增"})
		}
		ts := now.UTC().Format(time.RFC3339Nano)
		var parentArg any
		if loc.ParentID != nil {
			parentArg = *loc.ParentID
		}
		var iconArg any
		if loc.Icon != nil {
			iconArg = *loc.Icon
		}
		var customArg any
		if loc.CustomIconID != nil {
			customArg = *loc.CustomIconID
		}
		res, err := conn.ExecContext(ctx, `
INSERT INTO locations (name, type, code, parent_id, icon, custom_icon_id, version, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?)`, loc.Name, loc.Type, chosen, parentArg, iconArg, customArg, ts, ts)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				return ErrCodeTaken
			}
			return err
		}
		newID, err := res.LastInsertId()
		if err != nil {
			return err
		}
		created, err = loadLocation(ctx, conn, newID)
		return err
	})
	if err != nil {
		return Location{}, err
	}
	return created, nil
}

func codeTaken(ctx context.Context, conn *sql.Conn, code string) (bool, error) {
	var n int
	err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM locations WHERE code = ?`, code).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func UpdateLocation(ctx context.Context, db *sql.DB, now time.Time, id int64, in UpdateInput) (Location, error) {
	var updated Location
	err := withImmediate(ctx, db, func(conn *sql.Conn) error {
		loc, err := loadLocation(ctx, conn, id)
		if err != nil {
			return err
		}
		name, code, parent, icon, custom, err := resolveUpdate(ctx, conn, loc, in)
		if err != nil {
			return err
		}
		if custom != nil {
			if err := ensureCustomIcon(ctx, conn, *custom); err != nil {
				return err
			}
		}
		var codeArg, parentArg, iconArg, customArg any
		if code != nil {
			codeArg = *code
		}
		if parent != nil {
			parentArg = *parent
		}
		if icon != nil {
			iconArg = *icon
		}
		if custom != nil {
			customArg = *custom
		}
		ts := now.UTC().Format(time.RFC3339Nano)
		res, err := conn.ExecContext(ctx, `
UPDATE locations
SET name = ?, code = ?, parent_id = ?, icon = ?, custom_icon_id = ?, version = version + 1, updated_at = ?
WHERE id = ? AND version = ?`, name, codeArg, parentArg, iconArg, customArg, ts, id, loc.Version)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				return ErrCodeTaken
			}
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return conflictOrMissing(ctx, conn, id)
		}
		updated, err = loadLocation(ctx, conn, id)
		return err
	})
	if err != nil {
		return Location{}, err
	}
	return updated, nil
}

func DeleteLocation(ctx context.Context, db *sql.DB, id, version int64) error {
	return withImmediate(ctx, db, func(conn *sql.Conn) error {
		var stored int64
		err := conn.QueryRowContext(ctx, `SELECT version FROM locations WHERE id = ?`, id).Scan(&stored)
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
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM locations WHERE parent_id = ?`, id).Scan(&children); err != nil {
			return err
		}
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM item_locations WHERE location_id = ?`, id).Scan(&links); err != nil {
			return err
		}
		if children > 0 || links > 0 {
			return ErrInUse
		}
		res, err := conn.ExecContext(ctx, `DELETE FROM locations WHERE id = ? AND version = ?`, id, version)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return conflictOrMissing(ctx, conn, id)
		}
		return nil
	})
}

func resolveUpdate(ctx context.Context, conn *sql.Conn, loc Location, in UpdateInput) (string, *string, *int64, *string, *int64, error) {
	fields := map[string]string{}
	if !in.VersionPresent || in.Version < 1 {
		fields["version"] = "版本不正确"
	}
	if in.Type.Present {
		fields["type"] = "类型不能修改"
	}
	if !in.Name.Present && !in.Code.Present && !in.Parent.Present && !in.Icon.Present && !in.Custom.Present {
		fields["request"] = "没有要修改的内容"
	}
	name := loc.Name
	if in.Name.Present {
		if in.Name.Null {
			fields["name"] = "请填写名称"
		} else if n, nerr := NormalizeName(in.Name.Value); nerr != nil {
			fields["name"] = nerr.Error()
		} else {
			name = n
		}
	}
	var code *string
	if loc.Code != nil {
		c := *loc.Code
		code = &c
	}
	if in.Code.Present {
		next, msg := codeForUpdate(loc.Type, in.Code)
		if msg != "" {
			fields["code"] = msg
		} else {
			code = next
		}
	}
	if in.Parent.Present && in.Parent.Null && loc.Type == "fixed" {
		fields["parent_id"] = "请选择父级"
	}
	icon := loc.Icon
	custom := loc.CustomIconID
	if in.Icon.Present && in.Custom.Present {
		nextIcon, iconMsg := parseLocationIcon(in.Icon)
		nextCustom, customMsg := parseCustomIcon(in.Custom)
		if iconMsg != "" {
			fields["icon"] = iconMsg
		}
		if customMsg != "" {
			fields["custom_icon_id"] = customMsg
		}
		if nextIcon != nil && nextCustom != nil {
			fields["icon"] = "不能同时选用内置和自传图标"
			fields["custom_icon_id"] = "不能同时选用内置和自传图标"
		} else {
			icon = nextIcon
			custom = nextCustom
		}
	} else if in.Icon.Present {
		next, msg := parseLocationIcon(in.Icon)
		if msg != "" {
			fields["icon"] = msg
		} else {
			icon = next
			custom = nil
		}
	} else if in.Custom.Present {
		next, msg := parseCustomIcon(in.Custom)
		if msg != "" {
			fields["custom_icon_id"] = msg
		} else {
			custom = next
			if next != nil {
				icon = nil
			}
		}
	}
	if err := fieldError(fields); err != nil {
		return "", nil, nil, nil, nil, err
	}
	if in.Version != loc.Version {
		return "", nil, nil, nil, nil, ErrVersion
	}
	parent := loc.ParentID
	if in.Parent.Present {
		if in.Parent.Null {
			parent = nil
		} else {
			if err := ensureParent(ctx, conn, loc, in.Parent.Value); err != nil {
				return "", nil, nil, nil, nil, err
			}
			id := in.Parent.Value
			parent = &id
		}
	}
	return name, code, parent, icon, custom, nil
}

func codeForUpdate(typ string, code OptionalText) (*string, string) {
	if typ == "area" {
		if code.Null {
			return nil, ""
		}
		_, err := NormalizeCode(code.Value)
		if err != nil {
			if err.Error() == "请填写编号" {
				return nil, ""
			}
			return nil, err.Error()
		}
		return nil, "区域不使用编号"
	}
	if code.Null {
		return nil, "编号不能清空"
	}
	norm, err := NormalizeCode(code.Value)
	if err != nil {
		if err.Error() == "请填写编号" {
			return nil, "编号不能清空"
		}
		return nil, err.Error()
	}
	out := norm
	return &out, ""
}

// A descendant is a cycle even when that type could not be a parent.
func ensureParent(ctx context.Context, conn *sql.Conn, loc Location, parentID int64) error {
	if parentID < 1 {
		return ErrParent
	}
	seen := map[int64]struct{}{}
	cur := parentID
	var parentType string
	for {
		if cur == loc.ID {
			return ErrCycle
		}
		if _, ok := seen[cur]; ok {
			break
		}
		seen[cur] = struct{}{}
		var typ string
		var next sql.NullInt64
		err := conn.QueryRowContext(ctx, `SELECT type, parent_id FROM locations WHERE id = ?`, cur).Scan(&typ, &next)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrParent
		}
		if err != nil {
			return err
		}
		if parentType == "" {
			parentType = typ
		}
		if !next.Valid {
			break
		}
		cur = next.Int64
	}
	if !parentPairOK(loc.Type, parentType) {
		return ErrParent
	}
	return nil
}

func conflictOrMissing(ctx context.Context, conn *sql.Conn, id int64) error {
	var version int64
	err := conn.QueryRowContext(ctx, `SELECT version FROM locations WHERE id = ?`, id).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	return ErrVersion
}

func GetLocation(ctx context.Context, db *sql.DB, id int64) (Location, error) {
	var loc Location
	err := withRead(ctx, db, func(conn *sql.Conn) error {
		var err error
		loc, err = loadLocation(ctx, conn, id)
		return err
	})
	if err != nil {
		return Location{}, err
	}
	return loc, nil
}

func ListLocations(ctx context.Context, db *sql.DB, f ListFilter) (ListResult, error) {
	result := ListResult{Limit: f.Limit, Offset: f.Offset, Locations: []Location{}}
	err := withRead(ctx, db, func(conn *sql.Conn) error {
		if f.Kind == ListChildren {
			ok, err := locationExists(ctx, conn, f.ParentID)
			if err != nil {
				return err
			}
			if !ok {
				return ErrNotFound
			}
		}
		if f.Kind == ListEligible && f.HasExclude {
			ok, err := locationExists(ctx, conn, f.ExcludeID)
			if err != nil {
				return err
			}
			if !ok {
				return ErrNotFound
			}
		}
		countSQL, pageSQL, args := listSQL(f)
		if err := conn.QueryRowContext(ctx, countSQL, args...).Scan(&result.Total); err != nil {
			return err
		}
		pageArgs := append(append([]any{}, args...), f.Limit, f.Offset)
		rows, err := conn.QueryContext(ctx, pageSQL, pageArgs...)
		if err != nil {
			return err
		}
		defer rows.Close()
		locs := []Location{}
		for rows.Next() {
			loc, err := scanLocation(rows)
			if err != nil {
				return err
			}
			locs = append(locs, loc)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		for i := range locs {
			locs[i].Path, err = loadPath(ctx, conn, locs[i].ID)
			if err != nil {
				return err
			}
			if err := loadLocationDirectCount(ctx, conn, &locs[i]); err != nil {
				return err
			}
		}
		result.Locations = locs
		return nil
	})
	if err != nil {
		return ListResult{}, err
	}
	return result, nil
}

func validateCreate(in CreateInput) (name, typ, code string, parentID int64, hasParent bool, icon *string, custom *int64, err error) {
	fields := map[string]string{}
	if !in.Name.Present || in.Name.Null {
		fields["name"] = "请填写名称"
	} else if n, nerr := NormalizeName(in.Name.Value); nerr != nil {
		fields["name"] = nerr.Error()
	} else {
		name = n
	}
	switch {
	case !in.Type.Present || in.Type.Null:
		fields["type"] = "类型不正确"
	case in.Type.Value != "area" && in.Type.Value != "fixed" && in.Type.Value != "movable":
		fields["type"] = "类型不正确"
	default:
		typ = in.Type.Value
	}
	if typ != "" {
		var msg string
		code, msg = codeForCreate(typ, in.Code)
		if msg != "" {
			fields["code"] = msg
		}
		parentID, hasParent, msg = parentForCreate(typ, in.Parent)
		if msg != "" {
			fields["parent_id"] = msg
			hasParent = false
		}
	} else if msg := codeShapeError(in.Code); msg != "" {
		fields["code"] = msg
	}
	if in.Icon.Present {
		next, msg := parseLocationIcon(in.Icon)
		if msg != "" {
			fields["icon"] = msg
		} else {
			icon = next
		}
	}
	if in.Custom.Present {
		next, msg := parseCustomIcon(in.Custom)
		if msg != "" {
			fields["custom_icon_id"] = msg
		} else {
			custom = next
		}
	}
	if icon != nil && custom != nil {
		fields["icon"] = "不能同时选用内置和自传图标"
		fields["custom_icon_id"] = "不能同时选用内置和自传图标"
	}
	return name, typ, code, parentID, hasParent, icon, custom, fieldError(fields)
}

// Length, internal whitespace, and control characters do not need a valid type.
// 请填写编号 stays type-dependent.
func codeShapeError(code OptionalText) string {
	if !code.Present || code.Null {
		return ""
	}
	_, err := NormalizeCode(code.Value)
	if err == nil {
		return ""
	}
	switch err.Error() {
	case "编号过长", "编号不能包含空白", "编号不能包含控制字符":
		return err.Error()
	default:
		return ""
	}
}

func codeForCreate(typ string, code OptionalText) (string, string) {
	if typ == "area" {
		if !code.Present || code.Null {
			return "", ""
		}
		// Blank area codes are NULL. NormalizeCode reports that blank as 请填写编号, which areas do not use.
		_, err := NormalizeCode(code.Value)
		if err == nil {
			return "", "区域不使用编号"
		}
		if err.Error() == "请填写编号" {
			return "", ""
		}
		return "", err.Error()
	}
	if !code.Present || code.Null {
		return "", "请填写编号"
	}
	norm, err := NormalizeCode(code.Value)
	if err != nil {
		return "", err.Error()
	}
	return norm, ""
}

func parentForCreate(typ string, parent OptionalID) (int64, bool, string) {
	missing := !parent.Present || parent.Null
	if typ == "fixed" {
		if missing {
			return 0, false, "请选择父级"
		}
		return parent.Value, true, ""
	}
	if missing {
		return 0, false, ""
	}
	return parent.Value, true, ""
}

func parentPairOK(child, parent string) bool {
	switch child {
	case "area":
		return parent == "area"
	case "fixed":
		return parent == "area" || parent == "fixed"
	case "movable":
		return parent == "fixed" || parent == "movable"
	default:
		return false
	}
}

func locationExists(ctx context.Context, conn *sql.Conn, id int64) (bool, error) {
	var one int
	err := conn.QueryRowContext(ctx, `SELECT 1 FROM locations WHERE id = ?`, id).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func loadLocation(ctx context.Context, conn *sql.Conn, id int64) (Location, error) {
	loc, err := scanLocation(conn.QueryRowContext(ctx, `SELECT `+locationCols+` FROM locations WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Location{}, ErrNotFound
	}
	if err != nil {
		return Location{}, err
	}
	loc.Path, err = loadPath(ctx, conn, loc.ID)
	if err != nil {
		return Location{}, err
	}
	if err := loadLocationDirectCount(ctx, conn, &loc); err != nil {
		return Location{}, err
	}
	return loc, nil
}

func loadLocationDirectCount(ctx context.Context, conn *sql.Conn, loc *Location) error {
	return conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM item_locations WHERE location_id = ?`, loc.ID).Scan(&loc.DirectItemCount)
}

func scanLocation(sc rowScanner) (Location, error) {
	var loc Location
	var code sql.NullString
	var parent sql.NullInt64
	var icon sql.NullString
	var custom sql.NullInt64
	if err := sc.Scan(&loc.ID, &loc.Name, &loc.Type, &code, &parent, &icon, &custom, &loc.Version, &loc.CreatedAt, &loc.UpdatedAt); err != nil {
		return Location{}, err
	}
	if code.Valid {
		s := code.String
		loc.Code = &s
	}
	if parent.Valid {
		id := parent.Int64
		loc.ParentID = &id
	}
	if icon.Valid {
		s := icon.String
		loc.Icon = &s
	}
	if custom.Valid {
		id := custom.Int64
		loc.CustomIconID = &id
	}
	return loc, nil
}

func loadPath(ctx context.Context, conn *sql.Conn, id int64) ([]PathNode, error) {
	var chain []PathNode
	seen := map[int64]struct{}{}
	for {
		if _, ok := seen[id]; ok {
			return nil, errors.New("location path repeats an id")
		}
		seen[id] = struct{}{}
		var node PathNode
		var code sql.NullString
		var parent sql.NullInt64
		var icon sql.NullString
		var custom sql.NullInt64
		err := conn.QueryRowContext(ctx, `SELECT id, name, type, code, parent_id, icon, custom_icon_id FROM locations WHERE id = ?`, id).
			Scan(&node.ID, &node.Name, &node.Type, &code, &parent, &icon, &custom)
		if err != nil {
			return nil, err
		}
		if code.Valid {
			s := code.String
			node.Code = &s
		}
		if icon.Valid {
			s := icon.String
			node.Icon = &s
		}
		if custom.Valid {
			cid := custom.Int64
			node.CustomIconID = &cid
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

func listSQL(f ListFilter) (countSQL, pageSQL string, args []any) {
	switch f.Kind {
	case ListChildren:
		where := `parent_id = ?`
		return `SELECT COUNT(*) FROM locations WHERE ` + where,
			`SELECT ` + locationCols + ` FROM locations WHERE ` + where + ` ORDER BY ` + siblingOrder + ` LIMIT ? OFFSET ?`,
			[]any{f.ParentID}
	case ListFlat:
		return `SELECT COUNT(*) FROM locations`,
			`SELECT ` + locationCols + ` FROM locations ORDER BY id LIMIT ? OFFSET ?`,
			nil
	case ListEligible:
		cond, condArgs := eligibleCond(f.Eligible)
		if f.HasExclude {
			cte := `WITH RECURSIVE excluded(id) AS (
    SELECT id FROM locations WHERE id = ?
    UNION ALL
    SELECT child.id FROM locations AS child JOIN excluded ON child.parent_id = excluded.id
) `
			where := cond + ` AND id NOT IN (SELECT id FROM excluded)`
			args = append([]any{f.ExcludeID}, condArgs...)
			return cte + `SELECT COUNT(*) FROM locations WHERE ` + where,
				cte + `SELECT ` + locationCols + ` FROM locations WHERE ` + where + ` ORDER BY id LIMIT ? OFFSET ?`,
				args
		}
		return `SELECT COUNT(*) FROM locations WHERE ` + cond,
			`SELECT ` + locationCols + ` FROM locations WHERE ` + cond + ` ORDER BY id LIMIT ? OFFSET ?`,
			condArgs
	default:
		where := `parent_id IS NULL`
		return `SELECT COUNT(*) FROM locations WHERE ` + where,
			`SELECT ` + locationCols + ` FROM locations WHERE ` + where + ` ORDER BY ` + siblingOrder + ` LIMIT ? OFFSET ?`,
			nil
	}
}

func eligibleCond(kind string) (string, []any) {
	switch kind {
	case "area":
		return `type = ?`, []any{"area"}
	case "fixed":
		return `type IN (?, ?)`, []any{"area", "fixed"}
	case "movable":
		return `type IN (?, ?)`, []any{"fixed", "movable"}
	default:
		return `0`, nil
	}
}
