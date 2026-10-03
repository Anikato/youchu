package mcp

import "youchu/internal/catalog"

type pathJSON struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	Code         *string `json:"code"`
	Icon         *string `json:"icon"`
	CustomIconID *int64  `json:"custom_icon_id"`
}

type locationJSON struct {
	ID              int64      `json:"id"`
	Name            string     `json:"name"`
	Type            string     `json:"type"`
	Code            *string    `json:"code"`
	ParentID        *int64     `json:"parent_id"`
	Icon            *string    `json:"icon"`
	CustomIconID    *int64     `json:"custom_icon_id"`
	Version         int64      `json:"version"`
	CreatedAt       string     `json:"created_at"`
	UpdatedAt       string     `json:"updated_at"`
	Path            []pathJSON `json:"path"`
	DirectItemCount int        `json:"direct_item_count"`
}

type locationIconJSON struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	SVG       string `json:"svg"`
	Version   int64  `json:"version"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type locationIconPageJSON struct {
	Data []locationIconJSON `json:"data"`
}

type locationPageJSON struct {
	Data   []locationJSON `json:"data"`
	Total  int            `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

type categoryPathJSON struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type categoryJSON struct {
	ID              int64              `json:"id"`
	Name            string             `json:"name"`
	ParentID        *int64             `json:"parent_id"`
	Version         int64              `json:"version"`
	CreatedAt       string             `json:"created_at"`
	UpdatedAt       string             `json:"updated_at"`
	Path            []categoryPathJSON `json:"path"`
	DirectItemCount int                `json:"direct_item_count"`
}

type categoryPageJSON struct {
	Data   []categoryJSON `json:"data"`
	Total  int            `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

type itemLocationJSON struct {
	LocationID int64      `json:"location_id"`
	Note       *string    `json:"note"`
	Path       []pathJSON `json:"path"`
}

type itemCategoryJSON struct {
	CategoryID int64              `json:"category_id"`
	Source     string             `json:"source"`
	Path       []categoryPathJSON `json:"path"`
}

type photoJSON struct {
	ID        int64  `json:"id"`
	ItemID    int64  `json:"item_id"`
	Position  int    `json:"position"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	ByteSize  int64  `json:"byte_size"`
	Version   int64  `json:"version"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type coverPhotoJSON struct {
	ID int64 `json:"id"`
}

type returnTaskJSON struct {
	ID              int64           `json:"id"`
	ItemID          int64           `json:"item_id"`
	ItemName        string          `json:"item_name"`
	PartNote        *string         `json:"part_note"`
	Reason          *string         `json:"reason"`
	DestinationNote *string         `json:"destination_note"`
	CompletedAt     *string         `json:"completed_at"`
	Version         int64           `json:"version"`
	CreatedAt       string          `json:"created_at"`
	UpdatedAt       string          `json:"updated_at"`
	CoverPhoto      *coverPhotoJSON `json:"cover_photo"`
}

type itemJSON struct {
	ID           int64              `json:"id"`
	Name         string             `json:"name"`
	Alias        *string            `json:"alias"`
	Model        *string            `json:"model"`
	Spec         *string            `json:"spec"`
	QuantityNote *string            `json:"quantity_note"`
	Note         *string            `json:"note"`
	Version      int64              `json:"version"`
	CreatedAt    string             `json:"created_at"`
	UpdatedAt    string             `json:"updated_at"`
	DeletedAt    *string            `json:"deleted_at"`
	Locations    []itemLocationJSON `json:"locations"`
	Categories   []itemCategoryJSON `json:"categories"`
	ReturnTasks  []returnTaskJSON   `json:"return_tasks"`
	Photos       []photoJSON        `json:"photos"`
}

type itemPageJSON struct {
	Data   []itemJSON `json:"data"`
	Total  int        `json:"total"`
	Limit  int        `json:"limit"`
	Offset int        `json:"offset"`
}

type returnTaskPageJSON struct {
	Data   []returnTaskJSON `json:"data"`
	Total  int              `json:"total"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}

func toLocationJSON(loc catalog.Location) locationJSON {
	path := make([]pathJSON, len(loc.Path))
	for i, node := range loc.Path {
		path[i] = pathJSON{
			ID: node.ID, Name: node.Name, Type: node.Type, Code: node.Code,
			Icon: node.Icon, CustomIconID: node.CustomIconID,
		}
	}
	return locationJSON{
		ID:              loc.ID,
		Name:            loc.Name,
		Type:            loc.Type,
		Code:            loc.Code,
		ParentID:        loc.ParentID,
		Icon:            loc.Icon,
		CustomIconID:    loc.CustomIconID,
		Version:         loc.Version,
		CreatedAt:       loc.CreatedAt,
		UpdatedAt:       loc.UpdatedAt,
		Path:            path,
		DirectItemCount: loc.DirectItemCount,
	}
}

func toLocationIconJSON(icon catalog.LocationIcon) locationIconJSON {
	return locationIconJSON{
		ID:        icon.ID,
		Name:      icon.Name,
		SVG:       icon.SVG,
		Version:   icon.Version,
		CreatedAt: icon.CreatedAt,
		UpdatedAt: icon.UpdatedAt,
	}
}

func toLocationIconPage(icons []catalog.LocationIcon) locationIconPageJSON {
	data := make([]locationIconJSON, 0, len(icons))
	for _, icon := range icons {
		data = append(data, toLocationIconJSON(icon))
	}
	return locationIconPageJSON{Data: data}
}

func toLocationPage(page catalog.ListResult) locationPageJSON {
	data := make([]locationJSON, 0, len(page.Locations))
	for _, loc := range page.Locations {
		data = append(data, toLocationJSON(loc))
	}
	return locationPageJSON{Data: data, Total: page.Total, Limit: page.Limit, Offset: page.Offset}
}

func toCategoryJSON(cat catalog.Category) categoryJSON {
	path := make([]categoryPathJSON, len(cat.Path))
	for i, node := range cat.Path {
		path[i] = categoryPathJSON{ID: node.ID, Name: node.Name}
	}
	return categoryJSON{
		ID:              cat.ID,
		Name:            cat.Name,
		ParentID:        cat.ParentID,
		Version:         cat.Version,
		CreatedAt:       cat.CreatedAt,
		UpdatedAt:       cat.UpdatedAt,
		Path:            path,
		DirectItemCount: cat.DirectItemCount,
	}
}

func toCategoryPage(page catalog.CategoryList) categoryPageJSON {
	data := make([]categoryJSON, 0, len(page.Categories))
	for _, cat := range page.Categories {
		data = append(data, toCategoryJSON(cat))
	}
	return categoryPageJSON{Data: data, Total: page.Total, Limit: page.Limit, Offset: page.Offset}
}

func toItemJSON(item catalog.Item) itemJSON {
	locs := make([]itemLocationJSON, 0, len(item.Locations))
	for _, link := range item.Locations {
		path := make([]pathJSON, 0, len(link.Path))
		for _, node := range link.Path {
			path = append(path, pathJSON{
				ID: node.ID, Name: node.Name, Type: node.Type, Code: node.Code,
				Icon: node.Icon, CustomIconID: node.CustomIconID,
			})
		}
		locs = append(locs, itemLocationJSON{LocationID: link.LocationID, Note: link.Note, Path: path})
	}
	cats := make([]itemCategoryJSON, 0, len(item.Categories))
	for _, link := range item.Categories {
		path := make([]categoryPathJSON, 0, len(link.Path))
		for _, node := range link.Path {
			path = append(path, categoryPathJSON{ID: node.ID, Name: node.Name})
		}
		cats = append(cats, itemCategoryJSON{CategoryID: link.CategoryID, Source: link.Source, Path: path})
	}
	tasks := make([]returnTaskJSON, 0, len(item.ReturnTasks))
	for _, task := range item.ReturnTasks {
		tasks = append(tasks, toReturnTaskJSON(task))
	}
	photos := make([]photoJSON, 0, len(item.Photos))
	for _, p := range item.Photos {
		photos = append(photos, toPhotoJSON(p))
	}
	return itemJSON{
		ID:           item.ID,
		Name:         item.Name,
		Alias:        item.Alias,
		Model:        item.Model,
		Spec:         item.Spec,
		QuantityNote: item.QuantityNote,
		Note:         item.Note,
		Version:      item.Version,
		CreatedAt:    item.CreatedAt,
		UpdatedAt:    item.UpdatedAt,
		DeletedAt:    item.DeletedAt,
		Locations:    locs,
		Categories:   cats,
		ReturnTasks:  tasks,
		Photos:       photos,
	}
}

func toPhotoJSON(p catalog.Photo) photoJSON {
	return photoJSON{
		ID:        p.ID,
		ItemID:    p.ItemID,
		Position:  p.Position,
		Width:     p.Width,
		Height:    p.Height,
		ByteSize:  p.ByteSize,
		Version:   p.Version,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}

func toItemPage(page catalog.ItemList) itemPageJSON {
	data := make([]itemJSON, 0, len(page.Items))
	for _, item := range page.Items {
		data = append(data, toItemJSON(item))
	}
	return itemPageJSON{Data: data, Total: page.Total, Limit: page.Limit, Offset: page.Offset}
}

func toReturnTaskJSON(task catalog.ReturnTask) returnTaskJSON {
	var cover *coverPhotoJSON
	if task.CoverPhoto != nil {
		cover = &coverPhotoJSON{ID: task.CoverPhoto.ID}
	}
	return returnTaskJSON{
		ID:              task.ID,
		ItemID:          task.ItemID,
		ItemName:        task.ItemName,
		PartNote:        task.PartNote,
		Reason:          task.Reason,
		DestinationNote: task.DestinationNote,
		CompletedAt:     task.CompletedAt,
		Version:         task.Version,
		CreatedAt:       task.CreatedAt,
		UpdatedAt:       task.UpdatedAt,
		CoverPhoto:      cover,
	}
}

func toReturnTaskPage(page catalog.ReturnTaskList) returnTaskPageJSON {
	data := make([]returnTaskJSON, 0, len(page.Tasks))
	for _, task := range page.Tasks {
		data = append(data, toReturnTaskJSON(task))
	}
	return returnTaskPageJSON{Data: data, Total: page.Total, Limit: page.Limit, Offset: page.Offset}
}
