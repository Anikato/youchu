package mcp

import (
	"context"
	"errors"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"youchu/internal/catalog"
)

var (
	hintFalse           = false
	organizeAnnotations = &sdk.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: &hintFalse}
)

type itemCategoriesArgs struct {
	ItemID      int64   `json:"item_id" jsonschema:"物品 id"`
	CategoryIDs []int64 `json:"category_ids" jsonschema:"分类 id 列表"`
}

type createCategoryArgs struct {
	Name     string `json:"name" jsonschema:"分类名称"`
	ParentID *int64 `json:"parent_id,omitempty" jsonschema:"父级分类 id"`
}

func (s *Server) registerOrganizeTools(mcpServer *sdk.Server) {
	sdk.AddTool(mcpServer, &sdk.Tool{
		Name:        "youchu_create_category",
		Description: "新建分类。与 POST /api/v1/categories 相同。",
		Annotations: organizeAnnotations,
	}, s.createCategory)
	sdk.AddTool(mcpServer, &sdk.Tool{
		Name:        "youchu_add_item_categories",
		Description: "给活物品追加 AI 分类。已有关联跳过。不改物品版本。",
		Annotations: organizeAnnotations,
	}, s.addItemCategories)
	sdk.AddTool(mcpServer, &sdk.Tool{
		Name:        "youchu_remove_ai_item_categories",
		Description: "去掉物品上的 AI 分类。不能去掉人工分类。不改物品版本。",
		Annotations: organizeAnnotations,
	}, s.removeAIItemCategories)
}

func (s *Server) createCategory(ctx context.Context, _ *sdk.CallToolRequest, in createCategoryArgs) (*sdk.CallToolResult, any, error) {
	if res := s.requireScope(ctx, "organize"); res != nil {
		return res, nil, nil
	}
	input := catalog.CategoryInput{Name: catalog.OptionalText{Present: true, Value: in.Name}}
	if in.ParentID != nil {
		input.Parent = catalog.OptionalID{Present: true, Value: *in.ParentID}
	}
	cat, err := catalog.CreateCategory(ctx, s.db, s.now(), input)
	if err != nil {
		return categoryToolError(err), nil, nil
	}
	return textResult(toCategoryJSON(cat))
}

func (s *Server) addItemCategories(ctx context.Context, _ *sdk.CallToolRequest, in itemCategoriesArgs) (*sdk.CallToolResult, any, error) {
	if res := s.requireScope(ctx, "organize"); res != nil {
		return res, nil, nil
	}
	item, err := catalog.AddAIItemCategories(ctx, s.db, in.ItemID, in.CategoryIDs)
	if err != nil {
		return catalogToolError(err), nil, nil
	}
	return textResult(toItemJSON(item))
}

func (s *Server) removeAIItemCategories(ctx context.Context, _ *sdk.CallToolRequest, in itemCategoriesArgs) (*sdk.CallToolResult, any, error) {
	if res := s.requireScope(ctx, "organize"); res != nil {
		return res, nil, nil
	}
	item, err := catalog.RemoveAIItemCategories(ctx, s.db, in.ItemID, in.CategoryIDs)
	if err != nil {
		return catalogToolError(err), nil, nil
	}
	return textResult(toItemJSON(item))
}

func categoryToolError(err error) *sdk.CallToolResult {
	var fe *catalog.FieldError
	switch {
	case errors.As(err, &fe):
		return toolFields(fe.Fields)
	case errors.Is(err, catalog.ErrParent):
		return toolError("invalid_parent", "不能放在这个父级下")
	case errors.Is(err, catalog.ErrVersion):
		return toolError("version_conflict", "记录已被修改")
	case errors.Is(err, catalog.ErrCycle):
		return toolError("category_cycle", "不能移到自己的下级")
	case errors.Is(err, catalog.ErrInUse):
		return toolError("category_in_use", "这个分类下面还有内容")
	case errors.Is(err, catalog.ErrNameTaken):
		return toolError("name_taken", "同级已有相同名称")
	case errors.Is(err, catalog.ErrNotFound):
		return toolError("not_found", "未找到")
	default:
		return toolError("internal", "服务器错误")
	}
}
