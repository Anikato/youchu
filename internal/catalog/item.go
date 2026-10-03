package catalog

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

type ItemInput struct {
	Name           OptionalText
	Alias          OptionalText
	Model          OptionalText
	Spec           OptionalText
	QuantityNote   OptionalText
	Note           OptionalText
	VersionPresent bool
	Version        int64
	LocationsSet   bool
	LocationsNull  bool
	Locations      []ItemLinkInput
	CategoriesSet  bool
	CategoriesNull bool
	Categories     []ItemLinkInput
}

type ItemLinkInput struct {
	IDPresent bool
	IDNull    bool
	ID        int64
	Note      OptionalText
}

type Item struct {
	ID           int64
	Name         string
	Alias        *string
	Model        *string
	Spec         *string
	QuantityNote *string
	Note         *string
	Version      int64
	CreatedAt    string
	UpdatedAt    string
	DeletedAt    *string
	Locations    []ItemLocation
	Categories   []ItemCategory
	ReturnTasks  []ReturnTask
	Photos       []Photo
}

type ReturnTask struct {
	ID              int64
	ItemID          int64
	ItemName        string
	PartNote        *string
	Reason          *string
	DestinationNote *string
	CompletedAt     *string
	Version         int64
	CreatedAt       string
	UpdatedAt       string
	CoverPhoto      *CoverPhoto
}

type VersionInput struct {
	VersionPresent bool
	Version        int64
}

type ItemCategory struct {
	CategoryID int64
	Source     string
	Path       []CategoryPathNode
}

type ItemLocation struct {
	LocationID int64
	Note       *string
	Path       []PathNode
}

type ItemFilter struct {
	Unlocated             bool
	HasLocation           bool
	LocationID            int64
	HasInLocation         bool
	InLocationID          int64
	InLocationDescendants bool
	Keyword               string
	Uncategorized         bool
	CategoryIDs           []int64
	CategoryMatchAll      bool
	CategoryDescendants   bool
	SortName              bool
	Limit                 int
	Offset                int
}

type ItemList struct {
	Items  []Item
	Total  int
	Limit  int
	Offset int
}

type storedLink struct {
	LocationID int64
	Note       *string
}

const itemCols = `id, name, alias, model, spec, quantity_note, note, version, created_at, updated_at, deleted_at`

func CreateItem(ctx context.Context, db *sql.DB, now time.Time, in ItemInput) (Item, error) {
	var created Item
	err := withImmediate(ctx, db, func(conn *sql.Conn) error {
		name, alias, model, spec, qty, note, links, cats, err := prepareCreate(ctx, conn, in)
		if err != nil {
			return err
		}
		ts := now.UTC().Format(time.RFC3339Nano)
		res, err := conn.ExecContext(ctx, `
INSERT INTO items (name, alias, model, spec, quantity_note, note, version, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?)`,
			name, sqlText(alias), sqlText(model), sqlText(spec), sqlText(qty), sqlText(note), ts, ts)
		if err != nil {
			return err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		if err := insertLinks(ctx, conn, id, links); err != nil {
			return err
		}
		if err := insertCategoryLinks(ctx, conn, id, cats); err != nil {
			return err
		}
		created, err = loadItem(ctx, conn, id)
		return err
	})
	if err != nil {
		return Item{}, err
	}
	return created, nil
}

// A missing row is not a field error. locations null and an empty name still return ErrNotFound.
func UpdateItem(ctx context.Context, db *sql.DB, now time.Time, id int64, in ItemInput) (Item, error) {
	var updated Item
	err := withImmediate(ctx, db, func(conn *sql.Conn) error {
		cur, err := loadItem(ctx, conn, id)
		if err != nil {
			return err
		}
		if itemTrashed(cur) {
			return ErrNotFound
		}
		next, links, cats, replace, replaceCats, err := prepareUpdate(ctx, conn, cur, in)
		if err != nil {
			return err
		}
		if in.Version != cur.Version {
			return ErrVersion
		}
		ts := now.UTC().Format(time.RFC3339Nano)
		res, err := conn.ExecContext(ctx, `
UPDATE items
SET name = ?, alias = ?, model = ?, spec = ?, quantity_note = ?, note = ?, version = version + 1, updated_at = ?
WHERE id = ? AND version = ?`,
			next.Name, sqlText(next.Alias), sqlText(next.Model), sqlText(next.Spec), sqlText(next.QuantityNote), sqlText(next.Note), ts, id, cur.Version)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return itemConflictOrMissing(ctx, conn, id)
		}
		if replace {
			if _, err := conn.ExecContext(ctx, `DELETE FROM item_locations WHERE item_id = ?`, id); err != nil {
				return err
			}
			if err := insertLinks(ctx, conn, id, links); err != nil {
				return err
			}
		}
		if replaceCats {
			if _, err := conn.ExecContext(ctx, `DELETE FROM item_categories WHERE item_id = ?`, id); err != nil {
				return err
			}
			if err := insertCategoryLinks(ctx, conn, id, cats); err != nil {
				return err
			}
		}
		updated, err = loadItem(ctx, conn, id)
		return err
	})
	if err != nil {
		return Item{}, err
	}
	return updated, nil
}

func DeleteItem(ctx context.Context, db *sql.DB, now time.Time, id, version int64) error {
	return withImmediate(ctx, db, func(conn *sql.Conn) error {
		var stored int64
		var deleted sql.NullString
		err := conn.QueryRowContext(ctx, `SELECT version, deleted_at FROM items WHERE id = ?`, id).Scan(&stored, &deleted)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if deleted.Valid && deleted.String != "" {
			return ErrNotFound
		}
		if stored != version {
			return ErrVersion
		}
		ts := now.UTC().Format(time.RFC3339Nano)
		res, err := conn.ExecContext(ctx, `
UPDATE items SET deleted_at = ?, version = version + 1, updated_at = ?
WHERE id = ? AND version = ? AND deleted_at IS NULL`, ts, ts, id, version)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return itemConflictOrMissing(ctx, conn, id)
		}
		return nil
	})
}

func GetItem(ctx context.Context, db *sql.DB, id int64) (Item, error) {
	var item Item
	err := withRead(ctx, db, func(conn *sql.Conn) error {
		var err error
		item, err = loadItem(ctx, conn, id)
		if err != nil {
			return err
		}
		if itemTrashed(item) {
			return ErrNotFound
		}
		return nil
	})
	if err != nil {
		return Item{}, err
	}
	return item, nil
}

func GetTrashItem(ctx context.Context, db *sql.DB, id int64) (Item, error) {
	var item Item
	err := withRead(ctx, db, func(conn *sql.Conn) error {
		var err error
		item, err = loadItem(ctx, conn, id)
		if err != nil {
			return err
		}
		if !itemTrashed(item) {
			return ErrNotFound
		}
		return nil
	})
	if err != nil {
		return Item{}, err
	}
	return item, nil
}

func RestoreItem(ctx context.Context, db *sql.DB, now time.Time, id int64, in VersionInput) (Item, error) {
	var restored Item
	err := withImmediate(ctx, db, func(conn *sql.Conn) error {
		cur, err := loadItem(ctx, conn, id)
		if err != nil {
			return err
		}
		if !itemTrashed(cur) {
			return ErrNotFound
		}
		if !in.VersionPresent || in.Version < 1 {
			return fieldError(map[string]string{"version": "版本不正确"})
		}
		if in.Version != cur.Version {
			return ErrVersion
		}
		ts := now.UTC().Format(time.RFC3339Nano)
		res, err := conn.ExecContext(ctx, `
UPDATE items SET deleted_at = NULL, version = version + 1, updated_at = ?
WHERE id = ? AND version = ? AND deleted_at IS NOT NULL`, ts, id, cur.Version)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return itemConflictOrMissing(ctx, conn, id)
		}
		restored, err = loadItem(ctx, conn, id)
		return err
	})
	if err != nil {
		return Item{}, err
	}
	return restored, nil
}

func PurgeItem(ctx context.Context, db *sql.DB, id, version int64) (photoIDs []int64, err error) {
	err = withImmediate(ctx, db, func(conn *sql.Conn) error {
		var stored int64
		var deleted sql.NullString
		err := conn.QueryRowContext(ctx, `SELECT version, deleted_at FROM items WHERE id = ?`, id).Scan(&stored, &deleted)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if !deleted.Valid || deleted.String == "" {
			return ErrNotFound
		}
		if stored != version {
			return ErrVersion
		}
		ids, err := loadItemPhotoIDs(ctx, conn, id)
		if err != nil {
			return err
		}
		res, err := conn.ExecContext(ctx, `DELETE FROM items WHERE id = ? AND version = ? AND deleted_at IS NOT NULL`, id, version)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return itemConflictOrMissing(ctx, conn, id)
		}
		photoIDs = ids
		return nil
	})
	return photoIDs, err
}

func ListItems(ctx context.Context, db *sql.DB, f ItemFilter) (ItemList, error) {
	result := ItemList{Limit: f.Limit, Offset: f.Offset, Items: []Item{}}
	err := withRead(ctx, db, func(conn *sql.Conn) error {
		if f.HasLocation {
			ok, err := locationExists(ctx, conn, f.LocationID)
			if err != nil {
				return err
			}
			if !ok {
				return ErrNotFound
			}
		}
		if f.HasInLocation {
			ok, err := locationExists(ctx, conn, f.InLocationID)
			if err != nil {
				return err
			}
			if !ok {
				return ErrNotFound
			}
		}
		for _, id := range f.CategoryIDs {
			ok, err := categoryExists(ctx, conn, id)
			if err != nil {
				return err
			}
			if !ok {
				return ErrNotFound
			}
		}
		countSQL, pageSQL, args := itemListSQL(f)
		if err := conn.QueryRowContext(ctx, countSQL, args...).Scan(&result.Total); err != nil {
			return err
		}
		pageArgs := append(append([]any{}, args...), f.Limit, f.Offset)
		rows, err := conn.QueryContext(ctx, pageSQL, pageArgs...)
		if err != nil {
			return err
		}
		defer rows.Close()
		items := []Item{}
		for rows.Next() {
			item, err := scanItem(rows)
			if err != nil {
				return err
			}
			items = append(items, item)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		if err := attachItemAssocs(ctx, conn, items); err != nil {
			return err
		}
		result.Items = items
		return nil
	})
	if err != nil {
		return ItemList{}, err
	}
	return result, nil
}

func ListTrash(ctx context.Context, db *sql.DB, limit, offset int) (ItemList, error) {
	result := ItemList{Limit: limit, Offset: offset, Items: []Item{}}
	err := withRead(ctx, db, func(conn *sql.Conn) error {
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM items WHERE deleted_at IS NOT NULL`).Scan(&result.Total); err != nil {
			return err
		}
		rows, err := conn.QueryContext(ctx, `SELECT `+itemCols+` FROM items WHERE deleted_at IS NOT NULL ORDER BY name, id LIMIT ? OFFSET ?`, limit, offset)
		if err != nil {
			return err
		}
		defer rows.Close()
		items := []Item{}
		for rows.Next() {
			item, err := scanItem(rows)
			if err != nil {
				return err
			}
			items = append(items, item)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		if err := attachItemAssocs(ctx, conn, items); err != nil {
			return err
		}
		result.Items = items
		return nil
	})
	if err != nil {
		return ItemList{}, err
	}
	return result, nil
}

func attachItemAssocs(ctx context.Context, conn *sql.Conn, items []Item) error {
	for i := range items {
		links, err := loadItemLinks(ctx, conn, items[i].ID)
		if err != nil {
			return err
		}
		items[i].Locations = links
		cats, err := loadItemCategories(ctx, conn, items[i].ID)
		if err != nil {
			return err
		}
		items[i].Categories = cats
		tasks, err := loadItemReturnTasks(ctx, conn, items[i].ID, items[i].Name)
		if err != nil {
			return err
		}
		items[i].ReturnTasks = tasks
		photos, err := loadItemPhotos(ctx, conn, items[i].ID)
		if err != nil {
			return err
		}
		items[i].Photos = photos
	}
	return nil
}

func prepareCreate(ctx context.Context, conn *sql.Conn, in ItemInput) (name string, alias, model, spec, qty, note *string, links []storedLink, cats []int64, err error) {
	fields := map[string]string{}
	if !in.Name.Present || in.Name.Null {
		fields["name"] = "请填写名称"
	} else if n, nerr := NormalizeName(in.Name.Value); nerr != nil {
		fields["name"] = nerr.Error()
	} else {
		name = n
	}
	alias = assignText("alias", in.Alias, NormalizeAlias, fields)
	model = assignText("model", in.Model, NormalizeModel, fields)
	spec = assignText("spec", in.Spec, NormalizeSpec, fields)
	qty = assignText("quantity_note", in.QuantityNote, NormalizeQuantityNote, fields)
	note = assignText("note", in.Note, NormalizeNote, fields)
	links, err = linkFields(ctx, conn, in, fields)
	if err != nil {
		return "", nil, nil, nil, nil, nil, nil, nil, err
	}
	cats, err = categoryFields(ctx, conn, in, fields)
	if err != nil {
		return "", nil, nil, nil, nil, nil, nil, nil, err
	}
	if ferr := fieldError(fields); ferr != nil {
		return "", nil, nil, nil, nil, nil, nil, nil, ferr
	}
	return name, alias, model, spec, qty, note, links, cats, nil
}

func prepareUpdate(ctx context.Context, conn *sql.Conn, cur Item, in ItemInput) (Item, []storedLink, []int64, bool, bool, error) {
	fields := map[string]string{}
	if !in.VersionPresent || in.Version < 1 {
		fields["version"] = "版本不正确"
	}
	if !itemChange(in) {
		fields["request"] = "没有要修改的内容"
	}
	next := cur
	if in.Name.Present {
		if in.Name.Null {
			fields["name"] = "请填写名称"
		} else if n, nerr := NormalizeName(in.Name.Value); nerr != nil {
			fields["name"] = nerr.Error()
		} else {
			next.Name = n
		}
	}
	if in.Alias.Present {
		next.Alias = assignText("alias", in.Alias, NormalizeAlias, fields)
	}
	if in.Model.Present {
		next.Model = assignText("model", in.Model, NormalizeModel, fields)
	}
	if in.Spec.Present {
		next.Spec = assignText("spec", in.Spec, NormalizeSpec, fields)
	}
	if in.QuantityNote.Present {
		next.QuantityNote = assignText("quantity_note", in.QuantityNote, NormalizeQuantityNote, fields)
	}
	if in.Note.Present {
		next.Note = assignText("note", in.Note, NormalizeNote, fields)
	}
	links, err := linkFields(ctx, conn, in, fields)
	if err != nil {
		return Item{}, nil, nil, false, false, err
	}
	cats, err := categoryFields(ctx, conn, in, fields)
	if err != nil {
		return Item{}, nil, nil, false, false, err
	}
	if ferr := fieldError(fields); ferr != nil {
		return Item{}, nil, nil, false, false, ferr
	}
	return next, links, cats, in.LocationsSet && !in.LocationsNull, in.CategoriesSet && !in.CategoriesNull, nil
}

func itemChange(in ItemInput) bool {
	return in.Name.Present || in.Alias.Present || in.Model.Present || in.Spec.Present || in.QuantityNote.Present || in.Note.Present || in.LocationsSet || in.CategoriesSet
}

func assignText(key string, in OptionalText, norm func(string) (string, error), fields map[string]string) *string {
	if !in.Present || in.Null {
		return nil
	}
	v, err := norm(in.Value)
	if err != nil {
		fields[key] = err.Error()
		return nil
	}
	if v == "" {
		return nil
	}
	return &v
}

// Only the first locations error is kept. Other fields are collected by the caller.
func linkFields(ctx context.Context, conn *sql.Conn, in ItemInput, fields map[string]string) ([]storedLink, error) {
	if !in.LocationsSet {
		return nil, nil
	}
	if in.LocationsNull {
		fields["locations"] = "位置格式不正确"
		return nil, nil
	}
	seen := map[int64]struct{}{}
	out := []storedLink{}
	for _, link := range in.Locations {
		if !link.IDPresent || link.IDNull {
			fields["locations"] = "位置格式不正确"
			return nil, nil
		}
		if _, ok := seen[link.ID]; ok {
			fields["locations"] = "同一位置只能关联一次"
			return nil, nil
		}
		if link.ID < 1 {
			fields["locations"] = "所选位置不存在"
			return nil, nil
		}
		ok, err := locationExists(ctx, conn, link.ID)
		if err != nil {
			return nil, err
		}
		if !ok {
			fields["locations"] = "所选位置不存在"
			return nil, nil
		}
		var note *string
		if link.Note.Present && !link.Note.Null {
			v, nerr := NormalizePlacementNote(link.Note.Value)
			if nerr != nil {
				fields["locations"] = nerr.Error()
				return nil, nil
			}
			if v != "" {
				note = &v
			}
		}
		seen[link.ID] = struct{}{}
		out = append(out, storedLink{LocationID: link.ID, Note: note})
	}
	return out, nil
}

// Only the first categories error is kept. Other fields are collected by the caller.
func categoryFields(ctx context.Context, conn *sql.Conn, in ItemInput, fields map[string]string) ([]int64, error) {
	if !in.CategoriesSet {
		return nil, nil
	}
	if in.CategoriesNull {
		fields["categories"] = "分类格式不正确"
		return nil, nil
	}
	seen := map[int64]struct{}{}
	out := []int64{}
	for _, link := range in.Categories {
		if !link.IDPresent || link.IDNull {
			fields["categories"] = "分类格式不正确"
			return nil, nil
		}
		if _, ok := seen[link.ID]; ok {
			fields["categories"] = "同一分类只能关联一次"
			return nil, nil
		}
		if link.ID < 1 {
			fields["categories"] = "所选分类不存在"
			return nil, nil
		}
		ok, err := categoryExists(ctx, conn, link.ID)
		if err != nil {
			return nil, err
		}
		if !ok {
			fields["categories"] = "所选分类不存在"
			return nil, nil
		}
		seen[link.ID] = struct{}{}
		out = append(out, link.ID)
	}
	return out, nil
}

func insertLinks(ctx context.Context, conn *sql.Conn, itemID int64, links []storedLink) error {
	for _, link := range links {
		if _, err := conn.ExecContext(ctx, `INSERT INTO item_locations (item_id, location_id, note) VALUES (?, ?, ?)`, itemID, link.LocationID, sqlText(link.Note)); err != nil {
			return err
		}
	}
	return nil
}

func insertCategoryLinks(ctx context.Context, conn *sql.Conn, itemID int64, cats []int64) error {
	for _, categoryID := range cats {
		if _, err := conn.ExecContext(ctx, `INSERT INTO item_categories (item_id, category_id, source) VALUES (?, ?, 'human')`, itemID, categoryID); err != nil {
			return err
		}
	}
	return nil
}

func itemConflictOrMissing(ctx context.Context, conn *sql.Conn, id int64) error {
	var version int64
	err := conn.QueryRowContext(ctx, `SELECT version FROM items WHERE id = ?`, id).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	return ErrVersion
}

func loadItem(ctx context.Context, conn *sql.Conn, id int64) (Item, error) {
	item, err := scanItem(conn.QueryRowContext(ctx, `SELECT `+itemCols+` FROM items WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Item{}, ErrNotFound
	}
	if err != nil {
		return Item{}, err
	}
	item.Locations, err = loadItemLinks(ctx, conn, item.ID)
	if err != nil {
		return Item{}, err
	}
	item.Categories, err = loadItemCategories(ctx, conn, item.ID)
	if err != nil {
		return Item{}, err
	}
	item.ReturnTasks, err = loadItemReturnTasks(ctx, conn, item.ID, item.Name)
	if err != nil {
		return Item{}, err
	}
	item.Photos, err = loadItemPhotos(ctx, conn, item.ID)
	if err != nil {
		return Item{}, err
	}
	return item, nil
}

func loadItemLinks(ctx context.Context, conn *sql.Conn, itemID int64) ([]ItemLocation, error) {
	rows, err := conn.QueryContext(ctx, `SELECT location_id, note FROM item_locations WHERE item_id = ? ORDER BY location_id`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	links := []ItemLocation{}
	for rows.Next() {
		var id int64
		var note sql.NullString
		if err := rows.Scan(&id, &note); err != nil {
			return nil, err
		}
		links = append(links, ItemLocation{LocationID: id, Note: nullStringPtr(note)})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for i := range links {
		links[i].Path, err = loadPath(ctx, conn, links[i].LocationID)
		if err != nil {
			return nil, err
		}
	}
	return links, nil
}

func loadItemCategories(ctx context.Context, conn *sql.Conn, itemID int64) ([]ItemCategory, error) {
	rows, err := conn.QueryContext(ctx, `SELECT category_id, source FROM item_categories WHERE item_id = ? ORDER BY category_id`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	links := []ItemCategory{}
	for rows.Next() {
		var id int64
		var source string
		if err := rows.Scan(&id, &source); err != nil {
			return nil, err
		}
		links = append(links, ItemCategory{CategoryID: id, Source: source})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for i := range links {
		links[i].Path, err = loadCategoryPath(ctx, conn, links[i].CategoryID)
		if err != nil {
			return nil, err
		}
	}
	return links, nil
}

func scanItem(sc rowScanner) (Item, error) {
	var item Item
	var alias, model, spec, qty, note, deleted sql.NullString
	if err := sc.Scan(&item.ID, &item.Name, &alias, &model, &spec, &qty, &note, &item.Version, &item.CreatedAt, &item.UpdatedAt, &deleted); err != nil {
		return Item{}, err
	}
	item.Alias = nullStringPtr(alias)
	item.Model = nullStringPtr(model)
	item.Spec = nullStringPtr(spec)
	item.QuantityNote = nullStringPtr(qty)
	item.Note = nullStringPtr(note)
	item.DeletedAt = nullStringPtr(deleted)
	item.Locations = []ItemLocation{}
	item.Categories = []ItemCategory{}
	item.ReturnTasks = []ReturnTask{}
	item.Photos = []Photo{}
	return item, nil
}

func itemTrashed(item Item) bool {
	return item.DeletedAt != nil && *item.DeletedAt != ""
}

func nullStringPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

func sqlText(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

func likePattern(q string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return `%` + r.Replace(q) + `%`
}

func itemListSQL(f ItemFilter) (countSQL, pageSQL string, args []any) {
	order := ` ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`
	if f.SortName {
		order = ` ORDER BY name, id LIMIT ? OFFSET ?`
	}
	conds := []string{`deleted_at IS NULL`}
	if f.Unlocated {
		conds = append(conds, `NOT EXISTS (SELECT 1 FROM item_locations WHERE item_locations.item_id = items.id)`)
	}
	if f.HasLocation {
		conds = append(conds, `id IN (SELECT item_id FROM item_locations WHERE location_id = ?)`)
		args = append(args, f.LocationID)
	}
	if f.HasInLocation {
		if f.InLocationDescendants {
			conds = append(conds, `EXISTS (
  SELECT 1 FROM item_locations il
  WHERE il.item_id = items.id AND il.location_id IN (
    WITH RECURSIVE tree(id) AS (
      SELECT id FROM locations WHERE id = ?
      UNION ALL
      SELECT locations.id FROM locations JOIN tree ON locations.parent_id = tree.id
    )
    SELECT id FROM tree
  )
)`)
		} else {
			conds = append(conds, `EXISTS (SELECT 1 FROM item_locations il WHERE il.item_id = items.id AND il.location_id = ?)`)
		}
		args = append(args, f.InLocationID)
	}
	if f.Keyword != "" {
		conds = append(conds, `(name LIKE ? ESCAPE '\' OR alias LIKE ? ESCAPE '\' OR model LIKE ? ESCAPE '\' OR note LIKE ? ESCAPE '\')`)
		pat := likePattern(f.Keyword)
		args = append(args, pat, pat, pat, pat)
	}
	if f.Uncategorized {
		conds = append(conds, `NOT EXISTS (SELECT 1 FROM item_categories WHERE item_categories.item_id = items.id)`)
	}
	if len(f.CategoryIDs) > 0 {
		var parts []string
		for _, id := range f.CategoryIDs {
			if f.CategoryDescendants {
				parts = append(parts, `EXISTS (
  SELECT 1 FROM item_categories ic
  WHERE ic.item_id = items.id AND ic.category_id IN (
    WITH RECURSIVE tree(id) AS (
      SELECT id FROM categories WHERE id = ?
      UNION ALL
      SELECT categories.id FROM categories JOIN tree ON categories.parent_id = tree.id
    )
    SELECT id FROM tree
  )
)`)
			} else {
				parts = append(parts, `EXISTS (SELECT 1 FROM item_categories ic WHERE ic.item_id = items.id AND ic.category_id = ?)`)
			}
			args = append(args, id)
		}
		joiner := " OR "
		if f.CategoryMatchAll {
			joiner = " AND "
		}
		conds = append(conds, `(`+strings.Join(parts, joiner)+`)`)
	}
	where := ""
	if len(conds) > 0 {
		where = ` WHERE ` + strings.Join(conds, " AND ")
	}
	return `SELECT COUNT(*) FROM items` + where,
		`SELECT ` + itemCols + ` FROM items` + where + order,
		args
}
