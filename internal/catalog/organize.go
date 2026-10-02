package catalog

import (
	"context"
	"database/sql"
)

// Item existence is checked before category_ids. Links do not bump item version.
func AddAIItemCategories(ctx context.Context, db *sql.DB, itemID int64, ids []int64) (Item, error) {
	var item Item
	err := withImmediate(ctx, db, func(conn *sql.Conn) error {
		cur, err := loadLiveItem(ctx, conn, itemID)
		if err != nil {
			return err
		}
		if err := validateOrganizeCategoryIDs(ctx, conn, ids); err != nil {
			return err
		}
		have := map[int64]struct{}{}
		for _, c := range cur.Categories {
			have[c.CategoryID] = struct{}{}
		}
		for _, id := range ids {
			if _, ok := have[id]; ok {
				continue
			}
			if _, err := conn.ExecContext(ctx, `INSERT INTO item_categories (item_id, category_id, source) VALUES (?, ?, 'ai')`, itemID, id); err != nil {
				return err
			}
		}
		item, err = loadItem(ctx, conn, itemID)
		return err
	})
	if err != nil {
		return Item{}, err
	}
	return item, nil
}

func RemoveAIItemCategories(ctx context.Context, db *sql.DB, itemID int64, ids []int64) (Item, error) {
	var item Item
	err := withImmediate(ctx, db, func(conn *sql.Conn) error {
		cur, err := loadLiveItem(ctx, conn, itemID)
		if err != nil {
			return err
		}
		if err := validateOrganizeCategoryIDs(ctx, conn, ids); err != nil {
			return err
		}
		byID := map[int64]string{}
		for _, c := range cur.Categories {
			byID[c.CategoryID] = c.Source
		}
		for _, id := range ids {
			if byID[id] == "human" {
				return fieldError(map[string]string{"category_ids": "不能去掉人工分类"})
			}
		}
		for _, id := range ids {
			if _, ok := byID[id]; !ok {
				return fieldError(map[string]string{"category_ids": "该物品没有这个分类"})
			}
		}
		for _, id := range ids {
			if _, err := conn.ExecContext(ctx, `DELETE FROM item_categories WHERE item_id = ? AND category_id = ? AND source = 'ai'`, itemID, id); err != nil {
				return err
			}
		}
		item, err = loadItem(ctx, conn, itemID)
		return err
	})
	if err != nil {
		return Item{}, err
	}
	return item, nil
}

func loadLiveItem(ctx context.Context, conn *sql.Conn, id int64) (Item, error) {
	item, err := loadItem(ctx, conn, id)
	if err != nil {
		return Item{}, err
	}
	if itemTrashed(item) {
		return Item{}, ErrNotFound
	}
	return item, nil
}

func validateOrganizeCategoryIDs(ctx context.Context, conn *sql.Conn, ids []int64) error {
	if len(ids) == 0 {
		return fieldError(map[string]string{"category_ids": "分类格式不正确"})
	}
	seen := map[int64]struct{}{}
	for _, id := range ids {
		if id < 1 {
			return fieldError(map[string]string{"category_ids": "分类格式不正确"})
		}
		if _, ok := seen[id]; ok {
			return fieldError(map[string]string{"category_ids": "同一分类只能关联一次"})
		}
		seen[id] = struct{}{}
	}
	for _, id := range ids {
		ok, err := categoryExists(ctx, conn, id)
		if err != nil {
			return err
		}
		if !ok {
			return fieldError(map[string]string{"category_ids": "所选分类不存在"})
		}
	}
	return nil
}
