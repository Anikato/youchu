package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"youchu/internal/auth"
	"youchu/internal/catalog"
)

var readOnly = &sdk.ToolAnnotations{ReadOnlyHint: true}

type searchItemsArgs struct {
	Q                     string  `json:"q,omitempty" jsonschema:"名称、别名、型号或备注关键词"`
	Placement             string  `json:"placement,omitempty" jsonschema:"unlocated 表示尚未放入位置"`
	InLocation            *int64  `json:"in_location,omitempty" jsonschema:"位置范围 id"`
	InLocationDescendants *int64  `json:"in_location_descendants,omitempty" jsonschema:"0 仅当前位置，1 含下级"`
	Category              []int64 `json:"category,omitempty" jsonschema:"分类 id 列表"`
	CategoryMatch         string  `json:"category_match,omitempty" jsonschema:"any 或 all"`
	CategoryDescendants   *int64  `json:"category_descendants,omitempty" jsonschema:"0 仅当前分类，1 含下级"`
	Uncategorized         *int64  `json:"uncategorized,omitempty" jsonschema:"1 表示未分类"`
	Sort                  string  `json:"sort,omitempty" jsonschema:"created_at 或 name，省略为按添加时间新到旧"`
	Limit                 *int64  `json:"limit,omitempty" jsonschema:"每页数量，默认 30，最大 100"`
	Offset                *int64  `json:"offset,omitempty" jsonschema:"起点，从 0 开始"`
}

type idArgs struct {
	ID int64 `json:"id" jsonschema:"记录 id"`
}

type listTreeArgs struct {
	Parent *int64 `json:"parent,omitempty" jsonschema:"父级 id"`
	Flat   *int64 `json:"flat,omitempty" jsonschema:"1 表示扁平列表"`
	Limit  *int64 `json:"limit,omitempty" jsonschema:"每页数量，默认 30，最大 100"`
	Offset *int64 `json:"offset,omitempty" jsonschema:"起点，从 0 开始"`
}

type listPageArgs struct {
	Limit  *int64 `json:"limit,omitempty" jsonschema:"每页数量，默认 30，最大 100"`
	Offset *int64 `json:"offset,omitempty" jsonschema:"起点，从 0 开始"`
}

type getPhotoArgs struct {
	ID      int64  `json:"id" jsonschema:"照片 id"`
	Variant string `json:"variant,omitempty" jsonschema:"thumbnail 或 original，默认 thumbnail"`
}

func (s *Server) registerReadTools(mcpServer *sdk.Server) {
	sdk.AddTool(mcpServer, &sdk.Tool{
		Name:        "youchu_search_items",
		Description: "查找活物品。筛选与 GET /api/v1/items 相同，不含 location=。默认每页 30，最多 100。",
		Annotations: readOnly,
	}, s.searchItems)
	sdk.AddTool(mcpServer, &sdk.Tool{
		Name:        "youchu_get_item",
		Description: "读取一件活物品。回收站中的 id 为未找到。",
		Annotations: readOnly,
	}, s.getItem)
	sdk.AddTool(mcpServer, &sdk.Tool{
		Name:        "youchu_list_locations",
		Description: "列出位置。parent 与 flat 互斥。",
		Annotations: readOnly,
	}, s.listLocations)
	sdk.AddTool(mcpServer, &sdk.Tool{
		Name:        "youchu_get_location",
		Description: "读取一个位置。",
		Annotations: readOnly,
	}, s.getLocation)
	sdk.AddTool(mcpServer, &sdk.Tool{
		Name:        "youchu_list_categories",
		Description: "列出分类。parent 与 flat 互斥。",
		Annotations: readOnly,
	}, s.listCategories)
	sdk.AddTool(mcpServer, &sdk.Tool{
		Name:        "youchu_get_category",
		Description: "读取一个分类。",
		Annotations: readOnly,
	}, s.getCategory)
	sdk.AddTool(mcpServer, &sdk.Tool{
		Name:        "youchu_list_return_tasks",
		Description: "列出未完成、活物品上的归位事项。",
		Annotations: readOnly,
	}, s.listReturnTasks)
	sdk.AddTool(mcpServer, &sdk.Tool{
		Name:        "youchu_get_return_task",
		Description: "读取一条归位事项。",
		Annotations: readOnly,
	}, s.getReturnTask)
	sdk.AddTool(mcpServer, &sdk.Tool{
		Name:        "youchu_list_trash",
		Description: "列出回收站中的物品。",
		Annotations: readOnly,
	}, s.listTrash)
	sdk.AddTool(mcpServer, &sdk.Tool{
		Name:        "youchu_get_trash_item",
		Description: "读取回收站中的一件物品。",
		Annotations: readOnly,
	}, s.getTrashItem)
	sdk.AddTool(mcpServer, &sdk.Tool{
		Name:        "youchu_get_photo",
		Description: "读取物品照片 JPEG。variant 为 thumbnail（默认）或 original。回收站中的物品仍可读。",
		Annotations: readOnly,
	}, s.getPhoto)
	sdk.AddTool(mcpServer, &sdk.Tool{
		Name:        "youchu_list_location_icons",
		Description: "列出位置自传 SVG 图标。与 GET /api/v1/location-icons 相同。",
		Annotations: readOnly,
	}, s.listLocationIcons)
}

func (s *Server) searchItems(ctx context.Context, _ *sdk.CallToolRequest, in searchItemsArgs) (*sdk.CallToolResult, any, error) {
	if res := s.requireScope(ctx, "read"); res != nil {
		return res, nil, nil
	}
	filter, fields := itemFilterFromArgs(in)
	if len(fields) > 0 {
		return toolFields(fields), nil, nil
	}
	page, err := catalog.ListItems(ctx, s.db, filter)
	if err != nil {
		return catalogToolError(err), nil, nil
	}
	return textResult(toItemPage(page))
}

func (s *Server) getItem(ctx context.Context, _ *sdk.CallToolRequest, in idArgs) (*sdk.CallToolResult, any, error) {
	if res := s.requireScope(ctx, "read"); res != nil {
		return res, nil, nil
	}
	if in.ID < 1 {
		return toolError("not_found", "未找到"), nil, nil
	}
	item, err := catalog.GetItem(ctx, s.db, in.ID)
	if err != nil {
		return catalogToolError(err), nil, nil
	}
	return textResult(toItemJSON(item))
}

func (s *Server) listLocations(ctx context.Context, _ *sdk.CallToolRequest, in listTreeArgs) (*sdk.CallToolResult, any, error) {
	if res := s.requireScope(ctx, "read"); res != nil {
		return res, nil, nil
	}
	kind, parentID, limit, offset, fields := treeMode(in)
	if len(fields) > 0 {
		return toolFields(fields), nil, nil
	}
	page, err := catalog.ListLocations(ctx, s.db, catalog.ListFilter{
		Kind: kind, ParentID: parentID, Limit: limit, Offset: offset,
	})
	if err != nil {
		return catalogToolError(err), nil, nil
	}
	return textResult(toLocationPage(page))
}

func (s *Server) getLocation(ctx context.Context, _ *sdk.CallToolRequest, in idArgs) (*sdk.CallToolResult, any, error) {
	if res := s.requireScope(ctx, "read"); res != nil {
		return res, nil, nil
	}
	if in.ID < 1 {
		return toolError("not_found", "未找到"), nil, nil
	}
	loc, err := catalog.GetLocation(ctx, s.db, in.ID)
	if err != nil {
		return catalogToolError(err), nil, nil
	}
	return textResult(toLocationJSON(loc))
}

func (s *Server) listCategories(ctx context.Context, _ *sdk.CallToolRequest, in listTreeArgs) (*sdk.CallToolResult, any, error) {
	if res := s.requireScope(ctx, "read"); res != nil {
		return res, nil, nil
	}
	kind, parentID, limit, offset, fields := treeMode(in)
	if len(fields) > 0 {
		return toolFields(fields), nil, nil
	}
	page, err := catalog.ListCategories(ctx, s.db, catalog.CategoryFilter{
		Kind: kind, ParentID: parentID, Limit: limit, Offset: offset,
	})
	if err != nil {
		return catalogToolError(err), nil, nil
	}
	return textResult(toCategoryPage(page))
}

func (s *Server) getCategory(ctx context.Context, _ *sdk.CallToolRequest, in idArgs) (*sdk.CallToolResult, any, error) {
	if res := s.requireScope(ctx, "read"); res != nil {
		return res, nil, nil
	}
	if in.ID < 1 {
		return toolError("not_found", "未找到"), nil, nil
	}
	cat, err := catalog.GetCategory(ctx, s.db, in.ID)
	if err != nil {
		return catalogToolError(err), nil, nil
	}
	return textResult(toCategoryJSON(cat))
}

func (s *Server) listReturnTasks(ctx context.Context, _ *sdk.CallToolRequest, in listPageArgs) (*sdk.CallToolResult, any, error) {
	if res := s.requireScope(ctx, "read"); res != nil {
		return res, nil, nil
	}
	limit, offset, fields := pageFromArgs(in)
	if len(fields) > 0 {
		return toolFields(fields), nil, nil
	}
	page, err := catalog.ListOpenReturnTasks(ctx, s.db, limit, offset)
	if err != nil {
		return catalogToolError(err), nil, nil
	}
	return textResult(toReturnTaskPage(page))
}

func (s *Server) getReturnTask(ctx context.Context, _ *sdk.CallToolRequest, in idArgs) (*sdk.CallToolResult, any, error) {
	if res := s.requireScope(ctx, "read"); res != nil {
		return res, nil, nil
	}
	if in.ID < 1 {
		return toolError("not_found", "未找到"), nil, nil
	}
	task, err := catalog.GetReturnTask(ctx, s.db, in.ID)
	if err != nil {
		return catalogToolError(err), nil, nil
	}
	return textResult(toReturnTaskJSON(task))
}

func (s *Server) listTrash(ctx context.Context, _ *sdk.CallToolRequest, in listPageArgs) (*sdk.CallToolResult, any, error) {
	if res := s.requireScope(ctx, "read"); res != nil {
		return res, nil, nil
	}
	limit, offset, fields := pageFromArgs(in)
	if len(fields) > 0 {
		return toolFields(fields), nil, nil
	}
	page, err := catalog.ListTrash(ctx, s.db, limit, offset)
	if err != nil {
		return catalogToolError(err), nil, nil
	}
	return textResult(toItemPage(page))
}

func (s *Server) getTrashItem(ctx context.Context, _ *sdk.CallToolRequest, in idArgs) (*sdk.CallToolResult, any, error) {
	if res := s.requireScope(ctx, "read"); res != nil {
		return res, nil, nil
	}
	if in.ID < 1 {
		return toolError("not_found", "未找到"), nil, nil
	}
	item, err := catalog.GetTrashItem(ctx, s.db, in.ID)
	if err != nil {
		return catalogToolError(err), nil, nil
	}
	return textResult(toItemJSON(item))
}

func (s *Server) getPhoto(ctx context.Context, _ *sdk.CallToolRequest, in getPhotoArgs) (*sdk.CallToolResult, any, error) {
	if res := s.requireScope(ctx, "read"); res != nil {
		return res, nil, nil
	}
	if in.ID < 1 {
		return toolError("not_found", "未找到"), nil, nil
	}
	dir := "thumbnails"
	switch in.Variant {
	case "", "thumbnail":
		dir = "thumbnails"
	case "original":
		dir = "originals"
	default:
		return toolFields(map[string]string{"variant": "不支持的参数"}), nil, nil
	}
	if _, err := catalog.GetPhoto(ctx, s.db, in.ID); err != nil {
		return catalogToolError(err), nil, nil
	}
	data, err := os.ReadFile(filepath.Join(s.dataDir, dir, fmt.Sprintf("%d.jpg", in.ID)))
	if err != nil {
		return toolError("internal", "无法读取照片"), nil, nil
	}
	return &sdk.CallToolResult{Content: []sdk.Content{&sdk.ImageContent{Data: data, MIMEType: "image/jpeg"}}}, nil, nil
}

func (s *Server) listLocationIcons(ctx context.Context, _ *sdk.CallToolRequest, _ struct{}) (*sdk.CallToolResult, any, error) {
	if res := s.requireScope(ctx, "read"); res != nil {
		return res, nil, nil
	}
	icons, err := catalog.ListLocationIcons(ctx, s.db)
	if err != nil {
		return catalogToolError(err), nil, nil
	}
	return textResult(toLocationIconPage(icons))
}

func (s *Server) requireScope(ctx context.Context, scope string) *sdk.CallToolResult {
	tok, ok := AccessTokenFrom(ctx)
	if !ok || !hasScope(tok, scope) {
		return toolError("forbidden", "当前令牌没有这项权限")
	}
	return nil
}

func hasScope(tok auth.AccessToken, scope string) bool {
	for _, s := range tok.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

func toolError(code, message string) *sdk.CallToolResult {
	b, _ := json.Marshal(map[string]string{"code": code, "message": message})
	return &sdk.CallToolResult{IsError: true, Content: []sdk.Content{&sdk.TextContent{Text: string(b)}}}
}

func toolFields(fields map[string]string) *sdk.CallToolResult {
	b, _ := json.Marshal(struct {
		Code    string            `json:"code"`
		Message string            `json:"message"`
		Fields  map[string]string `json:"fields"`
	}{Code: "invalid_fields", Message: "有字段不符合要求", Fields: fields})
	return &sdk.CallToolResult{IsError: true, Content: []sdk.Content{&sdk.TextContent{Text: string(b)}}}
}

func catalogToolError(err error) *sdk.CallToolResult {
	var fe *catalog.FieldError
	switch {
	case errors.As(err, &fe):
		return toolFields(fe.Fields)
	case errors.Is(err, catalog.ErrNotFound):
		return toolError("not_found", "未找到")
	default:
		return toolError("internal", "服务器错误")
	}
}

func textResult(v any) (*sdk.CallToolResult, any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return toolError("internal", "服务器错误"), nil, nil
	}
	return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: string(b)}}}, nil, nil
}

func itemFilterFromArgs(in searchItemsArgs) (catalog.ItemFilter, map[string]string) {
	fields := map[string]string{}
	hasPlacement := in.Placement != ""
	hasInLocation := in.InLocation != nil
	hasUncategorized := in.Uncategorized != nil
	hasCategory := in.Category != nil
	if hasPlacement && hasInLocation {
		fields["placement"] = "不支持的参数"
		fields["in_location"] = "不支持的参数"
	}
	if hasUncategorized && hasCategory {
		fields["uncategorized"] = "不支持的参数"
		fields["category"] = "不支持的参数"
	}

	unlocated := false
	if hasPlacement && fields["placement"] == "" {
		if in.Placement != "unlocated" {
			fields["placement"] = "不支持的参数"
		} else {
			unlocated = true
		}
	}

	var inLocationID int64
	hasInLocationID := false
	if hasInLocation && fields["in_location"] == "" {
		if *in.InLocation < 1 {
			fields["in_location"] = "参数不正确"
		} else {
			inLocationID = *in.InLocation
			hasInLocationID = true
		}
	}

	var categoryIDs []int64
	categoryOK := false
	if hasCategory && fields["category"] == "" {
		ids, ok := validCategoryIDs(in.Category)
		if !ok {
			fields["category"] = "参数不正确"
		} else {
			categoryIDs = ids
			categoryOK = true
		}
	}

	uncategorized := false
	if hasUncategorized && fields["uncategorized"] == "" {
		if *in.Uncategorized != 1 {
			fields["uncategorized"] = "不支持的参数"
		} else {
			uncategorized = true
		}
	}

	if in.InLocationDescendants != nil && !hasInLocationID {
		fields["in_location_descendants"] = "不支持的参数"
	}
	if in.CategoryDescendants != nil && !categoryOK {
		fields["category_descendants"] = "不支持的参数"
	}
	if in.CategoryMatch != "" && (!categoryOK || len(categoryIDs) < 2) {
		fields["category_match"] = "不支持的参数"
	}
	if hasUncategorized {
		if in.CategoryMatch != "" {
			fields["category_match"] = "不支持的参数"
		}
		if in.CategoryDescendants != nil {
			fields["category_descendants"] = "不支持的参数"
		}
	}

	inLocationDescendants := hasInLocationID
	if in.InLocationDescendants != nil && fields["in_location_descendants"] == "" {
		switch *in.InLocationDescendants {
		case 1:
			inLocationDescendants = true
		case 0:
			inLocationDescendants = false
		default:
			fields["in_location_descendants"] = "不支持的参数"
		}
	}
	categoryDescendants := categoryOK
	if in.CategoryDescendants != nil && fields["category_descendants"] == "" {
		switch *in.CategoryDescendants {
		case 1:
			categoryDescendants = true
		case 0:
			categoryDescendants = false
		default:
			fields["category_descendants"] = "不支持的参数"
		}
	}
	categoryMatchAll := false
	if in.CategoryMatch != "" && fields["category_match"] == "" {
		switch in.CategoryMatch {
		case "any":
			categoryMatchAll = false
		case "all":
			categoryMatchAll = true
		default:
			fields["category_match"] = "不支持的参数"
		}
	}

	keyword := ""
	if in.Q != "" {
		k, err := catalog.NormalizeKeyword(in.Q)
		if err != nil {
			fields["q"] = err.Error()
		} else {
			keyword = k
		}
	}
	limit, offset := parsePage(in.Limit, in.Offset, fields)
	sortName := false
	if in.Sort != "" {
		switch in.Sort {
		case "created_at":
			sortName = false
		case "name":
			sortName = true
		default:
			fields["sort"] = "不支持的参数"
		}
	}
	if len(fields) > 0 {
		return catalog.ItemFilter{}, fields
	}
	return catalog.ItemFilter{
		Unlocated:             unlocated,
		HasInLocation:         hasInLocationID,
		InLocationID:          inLocationID,
		InLocationDescendants: inLocationDescendants,
		Keyword:               keyword,
		Uncategorized:         uncategorized,
		CategoryIDs:           categoryIDs,
		CategoryMatchAll:      categoryMatchAll,
		CategoryDescendants:   categoryDescendants,
		SortName:              sortName,
		Limit:                 limit,
		Offset:                offset,
	}, nil
}

func validCategoryIDs(ids []int64) ([]int64, bool) {
	if len(ids) == 0 || len(ids) > 20 {
		return nil, false
	}
	seen := map[int64]struct{}{}
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id < 1 {
			return nil, false
		}
		if _, dup := seen[id]; dup {
			return nil, false
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, true
}

func treeMode(in listTreeArgs) (kind catalog.ListKind, parentID int64, limit, offset int, fields map[string]string) {
	fields = map[string]string{}
	hasParent := in.Parent != nil
	hasFlat := in.Flat != nil
	if hasParent && hasFlat {
		fields["parent"] = "不支持的参数"
		fields["flat"] = "不支持的参数"
	}
	if hasFlat && fields["flat"] == "" && *in.Flat != 1 {
		fields["flat"] = "不支持的参数"
	}
	if hasParent && fields["parent"] == "" {
		if *in.Parent < 1 {
			fields["parent"] = "参数不正确"
		} else {
			parentID = *in.Parent
		}
	}
	limit, offset = parsePage(in.Limit, in.Offset, fields)
	if len(fields) > 0 {
		return 0, 0, 0, 0, fields
	}
	switch {
	case hasParent:
		kind = catalog.ListChildren
	case hasFlat:
		kind = catalog.ListFlat
	default:
		kind = catalog.ListRoots
	}
	return kind, parentID, limit, offset, nil
}

func pageFromArgs(in listPageArgs) (int, int, map[string]string) {
	fields := map[string]string{}
	limit, offset := parsePage(in.Limit, in.Offset, fields)
	if len(fields) > 0 {
		return 0, 0, fields
	}
	return limit, offset, nil
}

func parsePage(limit, offset *int64, fields map[string]string) (int, int) {
	lim := 30
	if limit != nil {
		if *limit < 1 || *limit > 100 {
			fields["limit"] = "数量超出范围"
		} else {
			lim = int(*limit)
		}
	}
	off := 0
	if offset != nil {
		if *offset < 0 {
			fields["offset"] = "起点不正确"
		} else {
			off = int(*offset)
		}
	}
	return lim, off
}
