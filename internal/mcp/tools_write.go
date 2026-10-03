package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"youchu/internal/catalog"
)

var (
	hintTrue         = true
	writeAnnotations = &sdk.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: &hintFalse}
	trashAnnotations = &sdk.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: &hintTrue}
)

type itemLocationArg struct {
	LocationID *int64  `json:"location_id"`
	Note       *string `json:"note,omitempty"`
}

type createItemArgs struct {
	Name         string             `json:"name" jsonschema:"物品名称"`
	Alias        *string            `json:"alias,omitempty" jsonschema:"别名"`
	Model        *string            `json:"model,omitempty" jsonschema:"型号"`
	Spec         *string            `json:"spec,omitempty" jsonschema:"规格"`
	QuantityNote *string            `json:"quantity_note,omitempty" jsonschema:"数量说明"`
	Note         *string            `json:"note,omitempty" jsonschema:"备注"`
	Locations    *[]itemLocationArg `json:"locations,omitempty" jsonschema:"存放位置"`
}

type updateItemArgs struct {
	ID           int64              `json:"id" jsonschema:"物品 id"`
	Version      int64              `json:"version" jsonschema:"版本"`
	Name         *string            `json:"name,omitempty" jsonschema:"物品名称"`
	Alias        *string            `json:"alias,omitempty" jsonschema:"别名"`
	Model        *string            `json:"model,omitempty" jsonschema:"型号"`
	Spec         *string            `json:"spec,omitempty" jsonschema:"规格"`
	QuantityNote *string            `json:"quantity_note,omitempty" jsonschema:"数量说明"`
	Note         *string            `json:"note,omitempty" jsonschema:"备注"`
	Locations    *[]itemLocationArg `json:"locations,omitempty" jsonschema:"存放位置"`
}

type cloneLocationArgs struct {
	ID int64 `json:"id" jsonschema:"位置 id"`
}

type createLocationArgs struct {
	Name         string  `json:"name" jsonschema:"位置名称"`
	Type         string  `json:"type" jsonschema:"area、fixed 或 movable"`
	Code         *string `json:"code,omitempty" jsonschema:"编号"`
	ParentID     *int64  `json:"parent_id,omitempty" jsonschema:"父级位置 id"`
	Icon         *string `json:"icon,omitempty" jsonschema:"内置图标短名"`
	CustomIconID *int64  `json:"custom_icon_id,omitempty" jsonschema:"自传图标 id"`
}

type updateLocationArgs struct {
	ID           int64           `json:"id" jsonschema:"位置 id"`
	Version      int64           `json:"version" jsonschema:"版本"`
	Name         *string         `json:"name,omitempty" jsonschema:"位置名称"`
	Code         *string         `json:"code,omitempty" jsonschema:"编号"`
	ParentID     json.RawMessage `json:"parent_id,omitempty" jsonschema:"父级位置 id，省略则不变，null 表示移到根级"`
	Icon         json.RawMessage `json:"icon,omitempty" jsonschema:"内置图标短名，null 表示改回类型默认"`
	CustomIconID json.RawMessage `json:"custom_icon_id,omitempty" jsonschema:"自传图标 id"`
}

type createLocationIconArgs struct {
	Name string `json:"name" jsonschema:"图标名称"`
	SVG  string `json:"svg" jsonschema:"SVG 文本"`
}

type updateLocationIconArgs struct {
	ID      int64   `json:"id" jsonschema:"图标 id"`
	Version int64   `json:"version" jsonschema:"版本"`
	Name    *string `json:"name,omitempty" jsonschema:"图标名称"`
	SVG     *string `json:"svg,omitempty" jsonschema:"SVG 文本"`
}

type updateCategoryArgs struct {
	ID       int64           `json:"id" jsonschema:"分类 id"`
	Version  int64           `json:"version" jsonschema:"版本"`
	Name     *string         `json:"name,omitempty" jsonschema:"分类名称"`
	ParentID json.RawMessage `json:"parent_id,omitempty" jsonschema:"父级分类 id，省略则不变，null 表示移到根级"`
}

type idVersionArgs struct {
	ID      int64 `json:"id" jsonschema:"记录 id"`
	Version int64 `json:"version" jsonschema:"版本"`
}

type createReturnTaskArgs struct {
	ItemID          int64   `json:"item_id" jsonschema:"物品 id"`
	PartNote        *string `json:"part_note,omitempty" jsonschema:"部件说明"`
	Reason          *string `json:"reason,omitempty" jsonschema:"原因"`
	DestinationNote *string `json:"destination_note,omitempty" jsonschema:"临时去向"`
}

func (s *Server) registerWriteTools(mcpServer *sdk.Server) {
	sdk.AddTool(mcpServer, &sdk.Tool{
		Name:        "youchu_create_item",
		Description: "新建物品。与 POST /api/v1/items 相同，不接受分类。",
		Annotations: writeAnnotations,
	}, s.createItem)
	sdk.AddTool(mcpServer, &sdk.Tool{
		Name:        "youchu_update_item",
		Description: "编辑物品。与 PATCH /api/v1/items/{id} 相同，不替换分类。",
		Annotations: writeAnnotations,
	}, s.updateItem)
	sdk.AddTool(mcpServer, &sdk.Tool{
		Name:        "youchu_create_location",
		Description: "新建位置。与 POST /api/v1/locations 相同。",
		Annotations: writeAnnotations,
	}, s.createLocation)
	sdk.AddTool(mcpServer, &sdk.Tool{
		Name:        "youchu_update_location",
		Description: "编辑位置。与 PATCH /api/v1/locations/{id} 相同。",
		Annotations: writeAnnotations,
		InputSchema: locationUpdateInputSchema(),
	}, s.updateLocation)
	sdk.AddTool(mcpServer, &sdk.Tool{
		Name:        "youchu_clone_location",
		Description: "克隆移动容器。与 POST /api/v1/locations/{id}/clone 相同。",
		Annotations: writeAnnotations,
	}, s.cloneLocation)
	sdk.AddTool(mcpServer, &sdk.Tool{
		Name:        "youchu_create_location_icon",
		Description: "上传位置自传 SVG 图标。与 POST /api/v1/location-icons 相同。",
		Annotations: writeAnnotations,
	}, s.createLocationIcon)
	sdk.AddTool(mcpServer, &sdk.Tool{
		Name:        "youchu_update_location_icon",
		Description: "改名或替换位置自传 SVG 图标。与 PATCH /api/v1/location-icons/{id} 相同。",
		Annotations: writeAnnotations,
	}, s.updateLocationIcon)
	sdk.AddTool(mcpServer, &sdk.Tool{
		Name:        "youchu_delete_location_icon",
		Description: "删除未被位置使用的自传图标。与 DELETE /api/v1/location-icons/{id} 相同。",
		Annotations: writeAnnotations,
	}, s.deleteLocationIcon)
	sdk.AddTool(mcpServer, &sdk.Tool{
		Name:        "youchu_update_category",
		Description: "编辑分类。与 PATCH /api/v1/categories/{id} 相同。",
		Annotations: writeAnnotations,
		InputSchema: parentIDInputSchema[updateCategoryArgs](),
	}, s.updateCategory)
	sdk.AddTool(mcpServer, &sdk.Tool{
		Name:        "youchu_trash_item",
		Description: "把物品放进回收站。与 DELETE /api/v1/items/{id} 相同。",
		Annotations: trashAnnotations,
	}, s.trashItem)
	sdk.AddTool(mcpServer, &sdk.Tool{
		Name:        "youchu_restore_item",
		Description: "从回收站恢复物品。与 POST /api/v1/trash/{id}/restore 相同。",
		Annotations: writeAnnotations,
	}, s.restoreItem)
	sdk.AddTool(mcpServer, &sdk.Tool{
		Name:        "youchu_create_return_task",
		Description: "登记待归位。与 POST /api/v1/items/{id}/return-tasks 相同。",
		Annotations: writeAnnotations,
	}, s.createReturnTask)
	sdk.AddTool(mcpServer, &sdk.Tool{
		Name:        "youchu_complete_return_task",
		Description: "完成一条归位事项。",
		Annotations: writeAnnotations,
	}, s.completeReturnTask)
	sdk.AddTool(mcpServer, &sdk.Tool{
		Name:        "youchu_delete_return_task",
		Description: "去掉未完成的归位事项。",
		Annotations: writeAnnotations,
	}, s.deleteReturnTask)
}

func (s *Server) createItem(ctx context.Context, _ *sdk.CallToolRequest, in createItemArgs) (*sdk.CallToolResult, any, error) {
	if res := s.requireScope(ctx, "write"); res != nil {
		return res, nil, nil
	}
	set, links := itemLinksFromArgs(in.Locations)
	item, err := catalog.CreateItem(ctx, s.db, s.now(), catalog.ItemInput{
		Name:         catalog.OptionalText{Present: true, Value: in.Name},
		Alias:        optionalText(in.Alias),
		Model:        optionalText(in.Model),
		Spec:         optionalText(in.Spec),
		QuantityNote: optionalText(in.QuantityNote),
		Note:         optionalText(in.Note),
		LocationsSet: set,
		Locations:    links,
	})
	if err != nil {
		return writeToolError(err), nil, nil
	}
	return textResult(toItemJSON(item))
}

func (s *Server) updateItem(ctx context.Context, _ *sdk.CallToolRequest, in updateItemArgs) (*sdk.CallToolResult, any, error) {
	if res := s.requireScope(ctx, "write"); res != nil {
		return res, nil, nil
	}
	set, links := itemLinksFromArgs(in.Locations)
	item, err := catalog.UpdateItem(ctx, s.db, s.now(), in.ID, catalog.ItemInput{
		Name:           optionalText(in.Name),
		Alias:          optionalText(in.Alias),
		Model:          optionalText(in.Model),
		Spec:           optionalText(in.Spec),
		QuantityNote:   optionalText(in.QuantityNote),
		Note:           optionalText(in.Note),
		VersionPresent: true,
		Version:        in.Version,
		LocationsSet:   set,
		Locations:      links,
	})
	if err != nil {
		return writeToolError(err), nil, nil
	}
	return textResult(toItemJSON(item))
}

func (s *Server) createLocation(ctx context.Context, _ *sdk.CallToolRequest, in createLocationArgs) (*sdk.CallToolResult, any, error) {
	if res := s.requireScope(ctx, "write"); res != nil {
		return res, nil, nil
	}
	parent := catalog.OptionalID{}
	if in.ParentID != nil {
		parent = catalog.OptionalID{Present: true, Value: *in.ParentID}
	}
	custom := catalog.OptionalID{}
	if in.CustomIconID != nil {
		custom = catalog.OptionalID{Present: true, Value: *in.CustomIconID}
	}
	loc, err := catalog.CreateLocation(ctx, s.db, s.now(), catalog.CreateInput{
		Name:   catalog.OptionalText{Present: true, Value: in.Name},
		Type:   catalog.OptionalText{Present: true, Value: in.Type},
		Code:   optionalText(in.Code),
		Parent: parent,
		Icon:   optionalText(in.Icon),
		Custom: custom,
	})
	if err != nil {
		return writeToolError(err), nil, nil
	}
	return textResult(toLocationJSON(loc))
}

func (s *Server) cloneLocation(ctx context.Context, _ *sdk.CallToolRequest, in cloneLocationArgs) (*sdk.CallToolResult, any, error) {
	if res := s.requireScope(ctx, "write"); res != nil {
		return res, nil, nil
	}
	loc, err := catalog.CloneLocation(ctx, s.db, s.now(), in.ID)
	if err != nil {
		return writeToolError(err), nil, nil
	}
	return textResult(toLocationJSON(loc))
}

func (s *Server) updateLocation(ctx context.Context, _ *sdk.CallToolRequest, in updateLocationArgs) (*sdk.CallToolResult, any, error) {
	if res := s.requireScope(ctx, "write"); res != nil {
		return res, nil, nil
	}
	parent, ferr := optionalIDFromRaw(in.ParentID, "parent_id")
	if ferr != nil {
		return ferr, nil, nil
	}
	icon, ferr := optionalTextFromRaw(in.Icon, "icon")
	if ferr != nil {
		return ferr, nil, nil
	}
	custom, ferr := optionalIDFromRaw(in.CustomIconID, "custom_icon_id")
	if ferr != nil {
		return ferr, nil, nil
	}
	loc, err := catalog.UpdateLocation(ctx, s.db, s.now(), in.ID, catalog.UpdateInput{
		Name:           optionalText(in.Name),
		Code:           optionalText(in.Code),
		Parent:         parent,
		Icon:           icon,
		Custom:         custom,
		VersionPresent: true,
		Version:        in.Version,
	})
	if err != nil {
		return writeToolError(err), nil, nil
	}
	return textResult(toLocationJSON(loc))
}

func (s *Server) updateCategory(ctx context.Context, _ *sdk.CallToolRequest, in updateCategoryArgs) (*sdk.CallToolResult, any, error) {
	if res := s.requireScope(ctx, "write"); res != nil {
		return res, nil, nil
	}
	parent, ferr := optionalIDFromRaw(in.ParentID, "parent_id")
	if ferr != nil {
		return ferr, nil, nil
	}
	cat, err := catalog.UpdateCategory(ctx, s.db, s.now(), in.ID, catalog.CategoryInput{
		Name:           optionalText(in.Name),
		Parent:         parent,
		VersionPresent: true,
		Version:        in.Version,
	})
	if err != nil {
		return categoryToolError(err), nil, nil
	}
	return textResult(toCategoryJSON(cat))
}

func (s *Server) trashItem(ctx context.Context, _ *sdk.CallToolRequest, in idVersionArgs) (*sdk.CallToolResult, any, error) {
	if res := s.requireScope(ctx, "write"); res != nil {
		return res, nil, nil
	}
	if err := catalog.DeleteItem(ctx, s.db, s.now(), in.ID, in.Version); err != nil {
		return writeToolError(err), nil, nil
	}
	item, err := catalog.GetTrashItem(ctx, s.db, in.ID)
	if err != nil {
		return writeToolError(err), nil, nil
	}
	return textResult(toItemJSON(item))
}

func (s *Server) restoreItem(ctx context.Context, _ *sdk.CallToolRequest, in idVersionArgs) (*sdk.CallToolResult, any, error) {
	if res := s.requireScope(ctx, "write"); res != nil {
		return res, nil, nil
	}
	item, err := catalog.RestoreItem(ctx, s.db, s.now(), in.ID, catalog.VersionInput{
		VersionPresent: true,
		Version:        in.Version,
	})
	if err != nil {
		return writeToolError(err), nil, nil
	}
	return textResult(toItemJSON(item))
}

func (s *Server) createReturnTask(ctx context.Context, _ *sdk.CallToolRequest, in createReturnTaskArgs) (*sdk.CallToolResult, any, error) {
	if res := s.requireScope(ctx, "write"); res != nil {
		return res, nil, nil
	}
	task, err := catalog.CreateReturnTask(ctx, s.db, s.now(), in.ItemID, catalog.ReturnTaskInput{
		PartNote:        optionalText(in.PartNote),
		Reason:          optionalText(in.Reason),
		DestinationNote: optionalText(in.DestinationNote),
	})
	if err != nil {
		return writeToolError(err), nil, nil
	}
	return textResult(toReturnTaskJSON(task))
}

func (s *Server) completeReturnTask(ctx context.Context, _ *sdk.CallToolRequest, in idVersionArgs) (*sdk.CallToolResult, any, error) {
	if res := s.requireScope(ctx, "write"); res != nil {
		return res, nil, nil
	}
	task, err := catalog.CompleteReturnTask(ctx, s.db, s.now(), in.ID, catalog.VersionInput{
		VersionPresent: true,
		Version:        in.Version,
	})
	if err != nil {
		return writeToolError(err), nil, nil
	}
	return textResult(toReturnTaskJSON(task))
}

func (s *Server) deleteReturnTask(ctx context.Context, _ *sdk.CallToolRequest, in idVersionArgs) (*sdk.CallToolResult, any, error) {
	if res := s.requireScope(ctx, "write"); res != nil {
		return res, nil, nil
	}
	if err := catalog.DeleteReturnTask(ctx, s.db, in.ID, in.Version); err != nil {
		return writeToolError(err), nil, nil
	}
	return textResult(map[string]any{})
}

func (s *Server) createLocationIcon(ctx context.Context, _ *sdk.CallToolRequest, in createLocationIconArgs) (*sdk.CallToolResult, any, error) {
	if res := s.requireScope(ctx, "write"); res != nil {
		return res, nil, nil
	}
	icon, err := catalog.CreateLocationIcon(ctx, s.db, s.now(), in.Name, in.SVG)
	if err != nil {
		return writeToolError(err), nil, nil
	}
	return textResult(toLocationIconJSON(icon))
}

func (s *Server) updateLocationIcon(ctx context.Context, _ *sdk.CallToolRequest, in updateLocationIconArgs) (*sdk.CallToolResult, any, error) {
	if res := s.requireScope(ctx, "write"); res != nil {
		return res, nil, nil
	}
	icon, err := catalog.UpdateLocationIcon(ctx, s.db, s.now(), in.ID, in.Version, optionalText(in.Name), optionalText(in.SVG))
	if err != nil {
		return writeToolError(err), nil, nil
	}
	return textResult(toLocationIconJSON(icon))
}

func (s *Server) deleteLocationIcon(ctx context.Context, _ *sdk.CallToolRequest, in idVersionArgs) (*sdk.CallToolResult, any, error) {
	if res := s.requireScope(ctx, "write"); res != nil {
		return res, nil, nil
	}
	if err := catalog.DeleteLocationIcon(ctx, s.db, in.ID, in.Version); err != nil {
		return writeToolError(err), nil, nil
	}
	return textResult(map[string]any{})
}

func locationUpdateInputSchema() *jsonschema.Schema {
	schema, err := jsonschema.For[updateLocationArgs](&jsonschema.ForOptions{
		TypeSchemas: map[reflect.Type]*jsonschema.Schema{
			reflect.TypeFor[json.RawMessage](): {Types: []string{"null", "integer", "string"}},
		},
	})
	if err != nil {
		panic(err)
	}
	return schema
}

func parentIDInputSchema[T any]() *jsonschema.Schema {
	schema, err := jsonschema.For[T](&jsonschema.ForOptions{
		TypeSchemas: map[reflect.Type]*jsonschema.Schema{
			reflect.TypeFor[json.RawMessage](): {Types: []string{"null", "integer"}},
		},
	})
	if err != nil {
		panic(err)
	}
	return schema
}

func optionalText(p *string) catalog.OptionalText {
	if p == nil {
		return catalog.OptionalText{}
	}
	return catalog.OptionalText{Present: true, Value: *p}
}

func itemLinksFromArgs(locs *[]itemLocationArg) (bool, []catalog.ItemLinkInput) {
	if locs == nil {
		return false, nil
	}
	links := make([]catalog.ItemLinkInput, len(*locs))
	for i, loc := range *locs {
		link := catalog.ItemLinkInput{Note: optionalText(loc.Note)}
		if loc.LocationID != nil {
			link.IDPresent = true
			link.ID = *loc.LocationID
		}
		links[i] = link
	}
	return true, links
}

func optionalIDFromRaw(raw json.RawMessage, field string) (catalog.OptionalID, *sdk.CallToolResult) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return catalog.OptionalID{}, nil
	}
	if strings.TrimSpace(string(raw)) == "null" {
		return catalog.OptionalID{Present: true, Null: true}, nil
	}
	var id int64
	if err := json.Unmarshal(raw, &id); err != nil {
		return catalog.OptionalID{}, toolFields(map[string]string{field: "参数不正确"})
	}
	return catalog.OptionalID{Present: true, Value: id}, nil
}

func optionalTextFromRaw(raw json.RawMessage, field string) (catalog.OptionalText, *sdk.CallToolResult) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return catalog.OptionalText{}, nil
	}
	if strings.TrimSpace(string(raw)) == "null" {
		return catalog.OptionalText{Present: true, Null: true}, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return catalog.OptionalText{}, toolFields(map[string]string{field: "参数不正确"})
	}
	return catalog.OptionalText{Present: true, Value: s}, nil
}

func writeToolError(err error) *sdk.CallToolResult {
	var fe *catalog.FieldError
	switch {
	case errors.As(err, &fe):
		return toolFields(fe.Fields)
	case errors.Is(err, catalog.ErrParent):
		return toolError("invalid_parent", "不能放在这个父级下")
	case errors.Is(err, catalog.ErrVersion):
		return toolError("version_conflict", "记录已被修改")
	case errors.Is(err, catalog.ErrCycle):
		return toolError("location_cycle", "不能移到自己的下级")
	case errors.Is(err, catalog.ErrInUse):
		return toolError("location_in_use", "这个位置下面还有内容")
	case errors.Is(err, catalog.ErrIconInUse):
		return toolError("icon_in_use", "有位置正在使用")
	case errors.Is(err, catalog.ErrCodeTaken):
		return toolError("code_taken", "编号已被使用")
	case errors.Is(err, catalog.ErrNotFound):
		return toolError("not_found", "未找到")
	case errors.Is(err, catalog.ErrAlreadyCompleted):
		return toolError("already_completed", "这条事项已经完成")
	default:
		return toolError("internal", "服务器错误")
	}
}
