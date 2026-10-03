package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"youchu/internal/auth"
	"youchu/internal/catalog"
	"youchu/internal/photo"
)

var positiveID = regexp.MustCompile(`^[1-9][0-9]*$`)

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

func (h *handler) locationsCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listLocations(w, r)
	case http.MethodPost:
		h.createLocation(w, r)
	default:
		h.notFound(w, r)
	}
}

func (h *handler) locationByID(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getLocation(w, r)
	case http.MethodPatch:
		h.patchLocation(w, r)
	case http.MethodDelete:
		h.deleteLocation(w, r)
	default:
		h.notFound(w, r)
	}
}

func (h *handler) locationClone(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.cloneLocation(w, r)
	default:
		h.notFound(w, r)
	}
}

func (h *handler) cloneLocation(w http.ResponseWriter, r *http.Request) {
	if !h.originOK(w, r) {
		return
	}
	if !h.currentUser(w, r) {
		return
	}
	body, ok := readLimited(w, r, 32768)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" {
		writeFields(w, queryRejected(r))
		return
	}
	if trimmed := strings.TrimSpace(string(body)); trimmed != "" {
		if !strings.HasPrefix(trimmed, "{") {
			writeError(w, http.StatusBadRequest, "invalid_body", "请求格式不正确")
			return
		}
		var obj map[string]json.RawMessage
		if err := json.Unmarshal([]byte(trimmed), &obj); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_body", "请求格式不正确")
			return
		}
	}
	id, ok := parseDecimalID(r.PathValue("id"))
	if !ok {
		h.notFound(w, r)
		return
	}
	loc, err := catalog.CloneLocation(r.Context(), h.db, h.now(), id)
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toLocationJSON(loc))
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

func (h *handler) locationIconsCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listLocationIcons(w, r)
	case http.MethodPost:
		h.createLocationIcon(w, r)
	default:
		h.notFound(w, r)
	}
}

func (h *handler) locationIconByID(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getLocationIcon(w, r)
	case http.MethodPatch:
		h.patchLocationIcon(w, r)
	case http.MethodDelete:
		h.deleteLocationIcon(w, r)
	default:
		h.notFound(w, r)
	}
}

func (h *handler) listLocationIcons(w http.ResponseWriter, r *http.Request) {
	if !h.currentUser(w, r) {
		return
	}
	if r.URL.RawQuery != "" {
		writeFields(w, queryRejected(r))
		return
	}
	icons, err := catalog.ListLocationIcons(r.Context(), h.db)
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toIconPageJSON(icons))
}

func (h *handler) createLocationIcon(w http.ResponseWriter, r *http.Request) {
	if !h.originOK(w, r) {
		return
	}
	if !h.currentUser(w, r) {
		return
	}
	body, ok := readLimited(w, r, 65536)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" {
		writeFields(w, queryRejected(r))
		return
	}
	in, err := decodeLocationIconWrite(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求格式不正确")
		return
	}
	icon, err := catalog.CreateLocationIcon(r.Context(), h.db, h.now(), iconWriteText(in.Name), iconWriteText(in.SVG))
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toIconJSON(icon))
}

func (h *handler) getLocationIcon(w http.ResponseWriter, r *http.Request) {
	if !h.currentUser(w, r) {
		return
	}
	if r.URL.RawQuery != "" {
		writeFields(w, queryRejected(r))
		return
	}
	id, ok := parseDecimalID(r.PathValue("id"))
	if !ok {
		h.notFound(w, r)
		return
	}
	icon, err := catalog.GetLocationIcon(r.Context(), h.db, id)
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toIconJSON(icon))
}

func (h *handler) patchLocationIcon(w http.ResponseWriter, r *http.Request) {
	if !h.originOK(w, r) {
		return
	}
	if !h.currentUser(w, r) {
		return
	}
	body, ok := readLimited(w, r, 65536)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" {
		writeFields(w, queryRejected(r))
		return
	}
	in, err := decodeLocationIconWrite(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求格式不正确")
		return
	}
	id, ok := parseDecimalID(r.PathValue("id"))
	if !ok {
		h.notFound(w, r)
		return
	}
	version := int64(0)
	if in.VersionPresent {
		version = in.Version
	}
	icon, err := catalog.UpdateLocationIcon(r.Context(), h.db, h.now(), id, version, iconOptional(in.Name), iconOptional(in.SVG))
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toIconJSON(icon))
}

func (h *handler) deleteLocationIcon(w http.ResponseWriter, r *http.Request) {
	if !h.originOK(w, r) {
		return
	}
	if !h.currentUser(w, r) {
		return
	}
	version, fields := deleteVersion(r)
	if len(fields) > 0 {
		writeFields(w, fields)
		return
	}
	id, ok := parseDecimalID(r.PathValue("id"))
	if !ok {
		h.notFound(w, r)
		return
	}
	if err := catalog.DeleteLocationIcon(r.Context(), h.db, id, version); err != nil {
		writeCatalogError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func iconWriteText(v textValue) string {
	if !v.Present || v.Null {
		return ""
	}
	return v.Value
}

func iconOptional(v textValue) catalog.OptionalText {
	return catalog.OptionalText{Present: v.Present, Null: v.Null, Value: v.Value}
}

func toIconJSON(icon catalog.LocationIcon) locationIconJSON {
	return locationIconJSON{
		ID:        icon.ID,
		Name:      icon.Name,
		SVG:       icon.SVG,
		Version:   icon.Version,
		CreatedAt: icon.CreatedAt,
		UpdatedAt: icon.UpdatedAt,
	}
}

func toIconPageJSON(icons []catalog.LocationIcon) locationIconPageJSON {
	data := make([]locationIconJSON, 0, len(icons))
	for _, icon := range icons {
		data = append(data, toIconJSON(icon))
	}
	return locationIconPageJSON{Data: data}
}

func (h *handler) categoriesCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listCategories(w, r)
	case http.MethodPost:
		h.createCategory(w, r)
	default:
		h.notFound(w, r)
	}
}

func (h *handler) categoryByID(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getCategory(w, r)
	case http.MethodPatch:
		h.patchCategory(w, r)
	case http.MethodDelete:
		h.deleteCategory(w, r)
	default:
		h.notFound(w, r)
	}
}

func (h *handler) getLocation(w http.ResponseWriter, r *http.Request) {
	if !h.currentUser(w, r) {
		return
	}
	if r.URL.RawQuery != "" {
		writeFields(w, queryRejected(r))
		return
	}
	id, ok := parseDecimalID(r.PathValue("id"))
	if !ok {
		h.notFound(w, r)
		return
	}
	loc, err := catalog.GetLocation(r.Context(), h.db, id)
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toLocationJSON(loc))
}

func (h *handler) patchLocation(w http.ResponseWriter, r *http.Request) {
	if !h.originOK(w, r) {
		return
	}
	if !h.currentUser(w, r) {
		return
	}
	body, ok := readLimited(w, r, 32768)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" {
		writeFields(w, queryRejected(r))
		return
	}
	in, err := decodeLocationWrite(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求格式不正确")
		return
	}
	id, ok := parseDecimalID(r.PathValue("id"))
	if !ok {
		h.notFound(w, r)
		return
	}
	loc, err := catalog.UpdateLocation(r.Context(), h.db, h.now(), id, updateInput(in))
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toLocationJSON(loc))
}

func (h *handler) deleteLocation(w http.ResponseWriter, r *http.Request) {
	if !h.originOK(w, r) {
		return
	}
	if !h.currentUser(w, r) {
		return
	}
	version, fields := deleteVersion(r)
	if len(fields) > 0 {
		writeFields(w, fields)
		return
	}
	id, ok := parseDecimalID(r.PathValue("id"))
	if !ok {
		h.notFound(w, r)
		return
	}
	if err := catalog.DeleteLocation(r.Context(), h.db, id, version); err != nil {
		writeCatalogError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func deleteVersion(r *http.Request) (int64, map[string]string) {
	values := r.URL.Query()
	fields := map[string]string{}
	for key, vals := range values {
		if key != "version" || len(vals) != 1 {
			fields[key] = "不支持的参数"
		}
	}
	if vals, ok := values["version"]; ok && len(vals) == 1 {
		n, good := parseDecimalID(vals[0])
		if !good {
			fields["version"] = "版本不正确"
			return 0, fields
		}
		if len(fields) > 0 {
			return 0, fields
		}
		return n, nil
	}
	if _, exists := fields["version"]; !exists {
		fields["version"] = "版本不正确"
	}
	return 0, fields
}

func (h *handler) createLocation(w http.ResponseWriter, r *http.Request) {
	if !h.originOK(w, r) {
		return
	}
	if !h.currentUser(w, r) {
		return
	}
	body, ok := readLimited(w, r, 32768)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" {
		fields := map[string]string{}
		for key := range r.URL.Query() {
			fields[key] = "不支持的参数"
		}
		if len(fields) == 0 {
			fields["query"] = "不支持的参数"
		}
		writeFields(w, fields)
		return
	}
	in, err := decodeLocationWrite(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求格式不正确")
		return
	}
	loc, err := catalog.CreateLocation(r.Context(), h.db, h.now(), createInput(in))
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toLocationJSON(loc))
}

func (h *handler) listLocations(w http.ResponseWriter, r *http.Request) {
	if !h.currentUser(w, r) {
		return
	}
	filter, fields := parseLocationQuery(r)
	if len(fields) > 0 {
		writeFields(w, fields)
		return
	}
	page, err := catalog.ListLocations(r.Context(), h.db, filter)
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toPageJSON(page))
}

func (h *handler) currentUser(w http.ResponseWriter, r *http.Request) bool {
	c, err := r.Cookie("youchu_session")
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "未登录")
		return false
	}
	if _, err = auth.Lookup(r.Context(), h.db, c.Value, h.now()); err != nil {
		if errors.Is(err, auth.ErrUnauthenticated) {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "未登录")
			return false
		}
		http.Error(w, "", http.StatusInternalServerError)
		return false
	}
	return true
}

func createInput(in locationWrite) catalog.CreateInput {
	return catalog.CreateInput{
		Name:   catalog.OptionalText{Present: in.Name.Present, Null: in.Name.Null, Value: in.Name.Value},
		Type:   catalog.OptionalText{Present: in.Type.Present, Null: in.Type.Null, Value: in.Type.Value},
		Code:   catalog.OptionalText{Present: in.Code.Present, Null: in.Code.Null, Value: in.Code.Value},
		Parent: catalog.OptionalID{Present: in.ParentPresent, Null: in.ParentNull, Value: in.ParentID},
		Icon:   catalog.OptionalText{Present: in.Icon.Present, Null: in.Icon.Null, Value: in.Icon.Value},
		Custom: catalog.OptionalID{Present: in.CustomPresent, Null: in.CustomNull, Value: in.CustomID},
	}
}

func updateInput(in locationWrite) catalog.UpdateInput {
	return catalog.UpdateInput{
		Name:           catalog.OptionalText{Present: in.Name.Present, Null: in.Name.Null, Value: in.Name.Value},
		Type:           catalog.OptionalText{Present: in.Type.Present, Null: in.Type.Null, Value: in.Type.Value},
		Code:           catalog.OptionalText{Present: in.Code.Present, Null: in.Code.Null, Value: in.Code.Value},
		Parent:         catalog.OptionalID{Present: in.ParentPresent, Null: in.ParentNull, Value: in.ParentID},
		Icon:           catalog.OptionalText{Present: in.Icon.Present, Null: in.Icon.Null, Value: in.Icon.Value},
		Custom:         catalog.OptionalID{Present: in.CustomPresent, Null: in.CustomNull, Value: in.CustomID},
		VersionPresent: in.VersionPresent,
		Version:        in.Version,
	}
}

func parseLocationQuery(r *http.Request) (catalog.ListFilter, map[string]string) {
	values := r.URL.Query()
	fields := map[string]string{}
	allowed := map[string]struct{}{
		"limit": {}, "offset": {}, "parent": {},
		"flat": {}, "eligible_parent_for": {}, "exclude": {},
	}
	for key, vals := range values {
		if _, ok := allowed[key]; !ok || len(vals) != 1 {
			fields[key] = "不支持的参数"
		}
	}
	modeKeys := []string{"parent", "flat", "eligible_parent_for"}
	present := 0
	for _, key := range modeKeys {
		if _, ok := values[key]; ok {
			present++
		}
	}
	if present > 1 {
		for _, key := range modeKeys {
			if _, ok := values[key]; ok {
				fields[key] = "不支持的参数"
			}
		}
	}
	if _, ok := values["exclude"]; ok {
		if _, hasEligible := values["eligible_parent_for"]; !hasEligible {
			fields["exclude"] = "不支持的参数"
		}
	}

	limit := 30
	if vals, ok := values["limit"]; ok && fields["limit"] == "" {
		n, good := parseLimit(vals[0])
		if !good {
			fields["limit"] = "数量超出范围"
		} else {
			limit = n
		}
	}
	offset := 0
	if vals, ok := values["offset"]; ok && fields["offset"] == "" {
		n, good := parseOffset(vals[0])
		if !good {
			fields["offset"] = "起点不正确"
		} else {
			offset = n
		}
	}
	if vals, ok := values["flat"]; ok && fields["flat"] == "" && vals[0] != "1" {
		fields["flat"] = "不支持的参数"
	}
	eligible := ""
	if vals, ok := values["eligible_parent_for"]; ok && fields["eligible_parent_for"] == "" {
		switch vals[0] {
		case "area", "fixed", "movable":
			eligible = vals[0]
		default:
			fields["eligible_parent_for"] = "不支持的参数"
		}
	}
	var parentID int64
	if vals, ok := values["parent"]; ok && fields["parent"] == "" {
		n, good := parseDecimalID(vals[0])
		if !good {
			fields["parent"] = "参数不正确"
		} else {
			parentID = n
		}
	}
	var excludeID int64
	hasExclude := false
	if vals, ok := values["exclude"]; ok && fields["exclude"] == "" {
		n, good := parseDecimalID(vals[0])
		if !good {
			fields["exclude"] = "参数不正确"
		} else {
			excludeID = n
			hasExclude = true
		}
	}
	if len(fields) > 0 {
		return catalog.ListFilter{}, fields
	}
	filter := catalog.ListFilter{Limit: limit, Offset: offset}
	switch {
	case values.Has("parent"):
		filter.Kind = catalog.ListChildren
		filter.ParentID = parentID
	case values.Has("flat"):
		filter.Kind = catalog.ListFlat
	case values.Has("eligible_parent_for"):
		filter.Kind = catalog.ListEligible
		filter.Eligible = eligible
		filter.HasExclude = hasExclude
		filter.ExcludeID = excludeID
	default:
		filter.Kind = catalog.ListRoots
	}
	return filter, nil
}

func parseLimit(s string) (int, bool) {
	n, ok := parseDecimalID(s)
	if !ok || n > 100 {
		return 0, false
	}
	return int(n), true
}

func parseOffset(s string) (int, bool) {
	if s == "0" {
		return 0, true
	}
	n, ok := parseDecimalID(s)
	if !ok || n > int64(^uint(0)>>1) {
		return 0, false
	}
	return int(n), true
}

func parseDecimalID(s string) (int64, bool) {
	if !positiveID.MatchString(s) {
		return 0, false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 1 {
		return 0, false
	}
	return n, true
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

func toPageJSON(page catalog.ListResult) locationPageJSON {
	data := make([]locationJSON, 0, len(page.Locations))
	for _, loc := range page.Locations {
		data = append(data, toLocationJSON(loc))
	}
	return locationPageJSON{Data: data, Total: page.Total, Limit: page.Limit, Offset: page.Offset}
}

func writeFields(w http.ResponseWriter, fields map[string]string) {
	writeJSON(w, http.StatusBadRequest, struct {
		Code    string            `json:"code"`
		Message string            `json:"message"`
		Fields  map[string]string `json:"fields"`
	}{Code: "invalid_fields", Message: "有字段不符合要求", Fields: fields})
}

func writeCatalogError(w http.ResponseWriter, err error) {
	var fe *catalog.FieldError
	switch {
	case errors.As(err, &fe):
		writeFields(w, fe.Fields)
	case errors.Is(err, catalog.ErrParent):
		writeError(w, http.StatusBadRequest, "invalid_parent", "不能放在这个父级下")
	case errors.Is(err, catalog.ErrVersion):
		writeError(w, http.StatusConflict, "version_conflict", "记录已被修改")
	case errors.Is(err, catalog.ErrCycle):
		writeError(w, http.StatusConflict, "location_cycle", "不能移到自己的下级")
	case errors.Is(err, catalog.ErrInUse):
		writeError(w, http.StatusConflict, "location_in_use", "这个位置下面还有内容")
	case errors.Is(err, catalog.ErrIconInUse):
		writeError(w, http.StatusConflict, "icon_in_use", "有位置正在使用")
	case errors.Is(err, catalog.ErrCodeTaken):
		writeError(w, http.StatusConflict, "code_taken", "编号已被使用")
	case errors.Is(err, catalog.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "未找到")
	case errors.Is(err, catalog.ErrAlreadyCompleted):
		writeError(w, http.StatusConflict, "already_completed", "这条事项已经完成")
	default:
		http.Error(w, "", http.StatusInternalServerError)
	}
}

func writeCategoryError(w http.ResponseWriter, err error) {
	var fe *catalog.FieldError
	switch {
	case errors.As(err, &fe):
		writeFields(w, fe.Fields)
	case errors.Is(err, catalog.ErrParent):
		writeError(w, http.StatusBadRequest, "invalid_parent", "不能放在这个父级下")
	case errors.Is(err, catalog.ErrVersion):
		writeError(w, http.StatusConflict, "version_conflict", "记录已被修改")
	case errors.Is(err, catalog.ErrCycle):
		writeError(w, http.StatusConflict, "category_cycle", "不能移到自己的下级")
	case errors.Is(err, catalog.ErrInUse):
		writeError(w, http.StatusConflict, "category_in_use", "这个分类下面还有内容")
	case errors.Is(err, catalog.ErrNameTaken):
		writeError(w, http.StatusConflict, "name_taken", "同级已有相同名称")
	case errors.Is(err, catalog.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "未找到")
	default:
		http.Error(w, "", http.StatusInternalServerError)
	}
}

func (h *handler) getCategory(w http.ResponseWriter, r *http.Request) {
	if !h.currentUser(w, r) {
		return
	}
	if r.URL.RawQuery != "" {
		writeFields(w, queryRejected(r))
		return
	}
	id, ok := parseDecimalID(r.PathValue("id"))
	if !ok {
		h.notFound(w, r)
		return
	}
	cat, err := catalog.GetCategory(r.Context(), h.db, id)
	if err != nil {
		writeCategoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toCategoryJSON(cat))
}

func (h *handler) patchCategory(w http.ResponseWriter, r *http.Request) {
	if !h.originOK(w, r) {
		return
	}
	if !h.currentUser(w, r) {
		return
	}
	body, ok := readLimited(w, r, 32768)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" {
		writeFields(w, queryRejected(r))
		return
	}
	in, err := decodeCategoryWrite(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求格式不正确")
		return
	}
	id, ok := parseDecimalID(r.PathValue("id"))
	if !ok {
		h.notFound(w, r)
		return
	}
	cat, err := catalog.UpdateCategory(r.Context(), h.db, h.now(), id, categoryInput(in))
	if err != nil {
		writeCategoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toCategoryJSON(cat))
}

func (h *handler) deleteCategory(w http.ResponseWriter, r *http.Request) {
	if !h.originOK(w, r) {
		return
	}
	if !h.currentUser(w, r) {
		return
	}
	version, fields := deleteVersion(r)
	if len(fields) > 0 {
		writeFields(w, fields)
		return
	}
	id, ok := parseDecimalID(r.PathValue("id"))
	if !ok {
		h.notFound(w, r)
		return
	}
	if err := catalog.DeleteCategory(r.Context(), h.db, id, version); err != nil {
		writeCategoryError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) createCategory(w http.ResponseWriter, r *http.Request) {
	if !h.originOK(w, r) {
		return
	}
	if !h.currentUser(w, r) {
		return
	}
	body, ok := readLimited(w, r, 32768)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" {
		writeFields(w, queryRejected(r))
		return
	}
	in, err := decodeCategoryWrite(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求格式不正确")
		return
	}
	cat, err := catalog.CreateCategory(r.Context(), h.db, h.now(), categoryInput(in))
	if err != nil {
		writeCategoryError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toCategoryJSON(cat))
}

func (h *handler) listCategories(w http.ResponseWriter, r *http.Request) {
	if !h.currentUser(w, r) {
		return
	}
	filter, fields := parseCategoryQuery(r)
	if len(fields) > 0 {
		writeFields(w, fields)
		return
	}
	page, err := catalog.ListCategories(r.Context(), h.db, filter)
	if err != nil {
		writeCategoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toCategoryPage(page))
}

func categoryInput(in categoryWrite) catalog.CategoryInput {
	return catalog.CategoryInput{
		Name:           catalog.OptionalText{Present: in.Name.Present, Null: in.Name.Null, Value: in.Name.Value},
		Parent:         catalog.OptionalID{Present: in.ParentPresent, Null: in.ParentNull, Value: in.ParentID},
		VersionPresent: in.VersionPresent,
		Version:        in.Version,
	}
}

func parseCategoryQuery(r *http.Request) (catalog.CategoryFilter, map[string]string) {
	values := r.URL.Query()
	fields := map[string]string{}
	allowed := map[string]struct{}{
		"limit": {}, "offset": {}, "parent": {},
		"flat": {}, "eligible_parent": {}, "exclude": {},
	}
	for key, vals := range values {
		if _, ok := allowed[key]; !ok || len(vals) != 1 {
			fields[key] = "不支持的参数"
		}
	}
	modeKeys := []string{"parent", "flat", "eligible_parent"}
	present := 0
	for _, key := range modeKeys {
		if _, ok := values[key]; ok {
			present++
		}
	}
	if present > 1 {
		for _, key := range modeKeys {
			if _, ok := values[key]; ok {
				fields[key] = "不支持的参数"
			}
		}
	}
	if _, ok := values["exclude"]; ok {
		if _, hasEligible := values["eligible_parent"]; !hasEligible {
			fields["exclude"] = "不支持的参数"
		}
	}

	limit := 30
	if vals, ok := values["limit"]; ok && fields["limit"] == "" {
		n, good := parseLimit(vals[0])
		if !good {
			fields["limit"] = "数量超出范围"
		} else {
			limit = n
		}
	}
	offset := 0
	if vals, ok := values["offset"]; ok && fields["offset"] == "" {
		n, good := parseOffset(vals[0])
		if !good {
			fields["offset"] = "起点不正确"
		} else {
			offset = n
		}
	}
	if vals, ok := values["flat"]; ok && fields["flat"] == "" && vals[0] != "1" {
		fields["flat"] = "不支持的参数"
	}
	if vals, ok := values["eligible_parent"]; ok && fields["eligible_parent"] == "" && vals[0] != "1" {
		fields["eligible_parent"] = "不支持的参数"
	}
	var parentID int64
	if vals, ok := values["parent"]; ok && fields["parent"] == "" {
		n, good := parseDecimalID(vals[0])
		if !good {
			fields["parent"] = "参数不正确"
		} else {
			parentID = n
		}
	}
	var excludeID int64
	hasExclude := false
	if vals, ok := values["exclude"]; ok && fields["exclude"] == "" {
		n, good := parseDecimalID(vals[0])
		if !good {
			fields["exclude"] = "参数不正确"
		} else {
			excludeID = n
			hasExclude = true
		}
	}
	if len(fields) > 0 {
		return catalog.CategoryFilter{}, fields
	}
	filter := catalog.CategoryFilter{Limit: limit, Offset: offset}
	switch {
	case values.Has("parent"):
		filter.Kind = catalog.ListChildren
		filter.ParentID = parentID
	case values.Has("flat"):
		filter.Kind = catalog.ListFlat
	case values.Has("eligible_parent"):
		filter.Kind = catalog.ListEligible
		filter.HasExclude = hasExclude
		filter.ExcludeID = excludeID
	default:
		filter.Kind = catalog.ListRoots
	}
	return filter, nil
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

func (h *handler) itemsCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listItems(w, r)
	case http.MethodPost:
		h.createItem(w, r)
	default:
		h.notFound(w, r)
	}
}

func (h *handler) itemByID(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getItem(w, r)
	case http.MethodPatch:
		h.patchItem(w, r)
	case http.MethodDelete:
		h.deleteItem(w, r)
	default:
		h.notFound(w, r)
	}
}

func (h *handler) listItems(w http.ResponseWriter, r *http.Request) {
	if !h.currentUser(w, r) {
		return
	}
	filter, fields := parseItemQuery(r)
	if len(fields) > 0 {
		writeFields(w, fields)
		return
	}
	page, err := catalog.ListItems(r.Context(), h.db, filter)
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toItemPage(page))
}

func (h *handler) getItem(w http.ResponseWriter, r *http.Request) {
	if !h.currentUser(w, r) {
		return
	}
	if r.URL.RawQuery != "" {
		writeFields(w, queryRejected(r))
		return
	}
	id, ok := parseDecimalID(r.PathValue("id"))
	if !ok {
		h.notFound(w, r)
		return
	}
	item, err := catalog.GetItem(r.Context(), h.db, id)
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toItemJSON(item))
}

func (h *handler) createItem(w http.ResponseWriter, r *http.Request) {
	if !h.originOK(w, r) {
		return
	}
	if !h.currentUser(w, r) {
		return
	}
	body, ok := readLimited(w, r, 32768)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" {
		writeFields(w, queryRejected(r))
		return
	}
	in, err := decodeItemWrite(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求格式不正确")
		return
	}
	item, err := catalog.CreateItem(r.Context(), h.db, h.now(), itemInput(in))
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toItemJSON(item))
}

func (h *handler) patchItem(w http.ResponseWriter, r *http.Request) {
	if !h.originOK(w, r) {
		return
	}
	if !h.currentUser(w, r) {
		return
	}
	body, ok := readLimited(w, r, 32768)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" {
		writeFields(w, queryRejected(r))
		return
	}
	in, err := decodeItemWrite(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求格式不正确")
		return
	}
	id, ok := parseDecimalID(r.PathValue("id"))
	if !ok {
		h.notFound(w, r)
		return
	}
	item, err := catalog.UpdateItem(r.Context(), h.db, h.now(), id, itemInput(in))
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toItemJSON(item))
}

func (h *handler) deleteItem(w http.ResponseWriter, r *http.Request) {
	if !h.originOK(w, r) {
		return
	}
	if !h.currentUser(w, r) {
		return
	}
	version, fields := deleteVersion(r)
	if len(fields) > 0 {
		writeFields(w, fields)
		return
	}
	id, ok := parseDecimalID(r.PathValue("id"))
	if !ok {
		h.notFound(w, r)
		return
	}
	if err := catalog.DeleteItem(r.Context(), h.db, h.now(), id, version); err != nil {
		writeCatalogError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) trashCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listTrash(w, r)
	default:
		h.notFound(w, r)
	}
}

func (h *handler) trashByID(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getTrash(w, r)
	case http.MethodDelete:
		h.purgeTrash(w, r)
	default:
		h.notFound(w, r)
	}
}

func (h *handler) restoreTrash(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.restoreTrashItem(w, r)
	default:
		h.notFound(w, r)
	}
}

func (h *handler) listTrash(w http.ResponseWriter, r *http.Request) {
	if !h.currentUser(w, r) {
		return
	}
	limit, offset, fields := parseTrashQuery(r)
	if len(fields) > 0 {
		writeFields(w, fields)
		return
	}
	page, err := catalog.ListTrash(r.Context(), h.db, limit, offset)
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toItemPage(page))
}

func (h *handler) getTrash(w http.ResponseWriter, r *http.Request) {
	if !h.currentUser(w, r) {
		return
	}
	if r.URL.RawQuery != "" {
		writeFields(w, queryRejected(r))
		return
	}
	id, ok := parseDecimalID(r.PathValue("id"))
	if !ok {
		h.notFound(w, r)
		return
	}
	item, err := catalog.GetTrashItem(r.Context(), h.db, id)
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toItemJSON(item))
}

func (h *handler) restoreTrashItem(w http.ResponseWriter, r *http.Request) {
	if !h.originOK(w, r) {
		return
	}
	if !h.currentUser(w, r) {
		return
	}
	body, ok := readLimited(w, r, 32768)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" {
		writeFields(w, queryRejected(r))
		return
	}
	in, err := decodeVersionWrite(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求格式不正确")
		return
	}
	id, ok := parseDecimalID(r.PathValue("id"))
	if !ok {
		h.notFound(w, r)
		return
	}
	item, err := catalog.RestoreItem(r.Context(), h.db, h.now(), id, catalog.VersionInput{
		VersionPresent: in.VersionPresent,
		Version:        in.Version,
	})
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toItemJSON(item))
}

func (h *handler) purgeTrash(w http.ResponseWriter, r *http.Request) {
	if !h.originOK(w, r) {
		return
	}
	if !h.currentUser(w, r) {
		return
	}
	version, fields := deleteVersion(r)
	if len(fields) > 0 {
		writeFields(w, fields)
		return
	}
	id, ok := parseDecimalID(r.PathValue("id"))
	if !ok {
		h.notFound(w, r)
		return
	}
	ids, err := catalog.PurgeItem(r.Context(), h.db, id, version)
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	for _, photoID := range ids {
		if err := photo.Remove(h.cfg.DataDir, photoID); err != nil {
			slog.Error("清理物品照片失败", "photo_id", photoID, "error", err)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseTrashQuery(r *http.Request) (int, int, map[string]string) {
	return parseLimitOffset(r)
}

func parseLimitOffset(r *http.Request) (int, int, map[string]string) {
	values := r.URL.Query()
	fields := map[string]string{}
	allowed := map[string]struct{}{"limit": {}, "offset": {}}
	for key, vals := range values {
		if _, ok := allowed[key]; !ok || len(vals) != 1 {
			fields[key] = "不支持的参数"
		}
	}
	limit := 30
	if vals, ok := values["limit"]; ok && fields["limit"] == "" {
		n, good := parseLimit(vals[0])
		if !good {
			fields["limit"] = "数量超出范围"
		} else {
			limit = n
		}
	}
	offset := 0
	if vals, ok := values["offset"]; ok && fields["offset"] == "" {
		n, good := parseOffset(vals[0])
		if !good {
			fields["offset"] = "起点不正确"
		} else {
			offset = n
		}
	}
	if len(fields) > 0 {
		return 0, 0, fields
	}
	return limit, offset, nil
}

func (h *handler) itemReturnTasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.createReturnTask(w, r)
	default:
		h.notFound(w, r)
	}
}

func (h *handler) returnTasksCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listReturnTasks(w, r)
	default:
		h.notFound(w, r)
	}
}

func (h *handler) returnTaskByID(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getReturnTask(w, r)
	case http.MethodDelete:
		h.deleteReturnTask(w, r)
	default:
		h.notFound(w, r)
	}
}

func (h *handler) completeReturnTask(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.postCompleteReturnTask(w, r)
	default:
		h.notFound(w, r)
	}
}

func (h *handler) listReturnTasks(w http.ResponseWriter, r *http.Request) {
	if !h.currentUser(w, r) {
		return
	}
	limit, offset, fields := parseLimitOffset(r)
	if len(fields) > 0 {
		writeFields(w, fields)
		return
	}
	page, err := catalog.ListOpenReturnTasks(r.Context(), h.db, limit, offset)
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toReturnTaskPage(page))
}

func (h *handler) createReturnTask(w http.ResponseWriter, r *http.Request) {
	if !h.originOK(w, r) {
		return
	}
	if !h.currentUser(w, r) {
		return
	}
	body, ok := readLimited(w, r, 32768)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" {
		writeFields(w, queryRejected(r))
		return
	}
	in, err := decodeReturnTaskWrite(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求格式不正确")
		return
	}
	id, ok := parseDecimalID(r.PathValue("id"))
	if !ok {
		h.notFound(w, r)
		return
	}
	task, err := catalog.CreateReturnTask(r.Context(), h.db, h.now(), id, returnTaskInput(in))
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toReturnTaskJSON(task))
}

func (h *handler) getReturnTask(w http.ResponseWriter, r *http.Request) {
	if !h.currentUser(w, r) {
		return
	}
	if r.URL.RawQuery != "" {
		writeFields(w, queryRejected(r))
		return
	}
	id, ok := parseDecimalID(r.PathValue("id"))
	if !ok {
		h.notFound(w, r)
		return
	}
	task, err := catalog.GetReturnTask(r.Context(), h.db, id)
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toReturnTaskJSON(task))
}

func (h *handler) postCompleteReturnTask(w http.ResponseWriter, r *http.Request) {
	if !h.originOK(w, r) {
		return
	}
	if !h.currentUser(w, r) {
		return
	}
	body, ok := readLimited(w, r, 32768)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" {
		writeFields(w, queryRejected(r))
		return
	}
	in, err := decodeVersionWrite(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求格式不正确")
		return
	}
	id, ok := parseDecimalID(r.PathValue("id"))
	if !ok {
		h.notFound(w, r)
		return
	}
	task, err := catalog.CompleteReturnTask(r.Context(), h.db, h.now(), id, catalog.VersionInput{
		VersionPresent: in.VersionPresent,
		Version:        in.Version,
	})
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toReturnTaskJSON(task))
}

func (h *handler) deleteReturnTask(w http.ResponseWriter, r *http.Request) {
	if !h.originOK(w, r) {
		return
	}
	if !h.currentUser(w, r) {
		return
	}
	version, fields := deleteVersion(r)
	if len(fields) > 0 {
		writeFields(w, fields)
		return
	}
	id, ok := parseDecimalID(r.PathValue("id"))
	if !ok {
		h.notFound(w, r)
		return
	}
	if err := catalog.DeleteReturnTask(r.Context(), h.db, id, version); err != nil {
		writeCatalogError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func returnTaskInput(in returnTaskWrite) catalog.ReturnTaskInput {
	return catalog.ReturnTaskInput{
		PartNote:        catalog.OptionalText{Present: in.PartNote.Present, Null: in.PartNote.Null, Value: in.PartNote.Value},
		Reason:          catalog.OptionalText{Present: in.Reason.Present, Null: in.Reason.Null, Value: in.Reason.Value},
		DestinationNote: catalog.OptionalText{Present: in.DestinationNote.Present, Null: in.DestinationNote.Null, Value: in.DestinationNote.Value},
	}
}

type returnTaskPageJSON struct {
	Data   []returnTaskJSON `json:"data"`
	Total  int              `json:"total"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
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

func queryRejected(r *http.Request) map[string]string {
	fields := map[string]string{}
	for key := range r.URL.Query() {
		fields[key] = "不支持的参数"
	}
	if len(fields) == 0 {
		fields["query"] = "不支持的参数"
	}
	return fields
}

func itemInput(in itemWrite) catalog.ItemInput {
	links := make([]catalog.ItemLinkInput, len(in.Locations))
	for i, link := range in.Locations {
		links[i] = catalog.ItemLinkInput{
			IDPresent: link.IDPresent,
			IDNull:    link.IDNull,
			ID:        link.ID,
			Note:      catalog.OptionalText{Present: link.Note.Present, Null: link.Note.Null, Value: link.Note.Value},
		}
	}
	cats := make([]catalog.ItemLinkInput, len(in.Categories))
	for i, link := range in.Categories {
		cats[i] = catalog.ItemLinkInput{
			IDPresent: link.IDPresent,
			IDNull:    link.IDNull,
			ID:        link.ID,
		}
	}
	return catalog.ItemInput{
		Name:           catalog.OptionalText{Present: in.Name.Present, Null: in.Name.Null, Value: in.Name.Value},
		Alias:          catalog.OptionalText{Present: in.Alias.Present, Null: in.Alias.Null, Value: in.Alias.Value},
		Model:          catalog.OptionalText{Present: in.Model.Present, Null: in.Model.Null, Value: in.Model.Value},
		Spec:           catalog.OptionalText{Present: in.Spec.Present, Null: in.Spec.Null, Value: in.Spec.Value},
		QuantityNote:   catalog.OptionalText{Present: in.QuantityNote.Present, Null: in.QuantityNote.Null, Value: in.QuantityNote.Value},
		Note:           catalog.OptionalText{Present: in.Note.Present, Null: in.Note.Null, Value: in.Note.Value},
		VersionPresent: in.VersionPresent,
		Version:        in.Version,
		LocationsSet:   in.LocationsSet,
		LocationsNull:  in.LocationsNull,
		Locations:      links,
		CategoriesSet:  in.CategoriesSet,
		CategoriesNull: in.CategoriesNull,
		Categories:     cats,
	}
}

func parseItemQuery(r *http.Request) (catalog.ItemFilter, map[string]string) {
	values := r.URL.Query()
	fields := map[string]string{}
	allowed := map[string]struct{}{
		"limit": {}, "offset": {}, "placement": {}, "location": {},
		"q": {}, "in_location": {}, "in_location_descendants": {},
		"category": {}, "category_match": {}, "category_descendants": {},
		"uncategorized": {}, "sort": {},
	}
	for key, vals := range values {
		if _, ok := allowed[key]; !ok || len(vals) != 1 {
			fields[key] = "不支持的参数"
		}
	}
	_, hasPlacement := values["placement"]
	_, hasLocation := values["location"]
	_, hasInLocation := values["in_location"]
	_, hasUncategorized := values["uncategorized"]
	_, hasCategory := values["category"]
	if hasPlacement && hasLocation {
		fields["placement"] = "不支持的参数"
		fields["location"] = "不支持的参数"
	}
	if hasPlacement && hasInLocation {
		fields["placement"] = "不支持的参数"
		fields["in_location"] = "不支持的参数"
	}
	if hasLocation && hasInLocation {
		fields["location"] = "不支持的参数"
		fields["in_location"] = "不支持的参数"
	}
	if hasUncategorized && hasCategory {
		fields["uncategorized"] = "不支持的参数"
		fields["category"] = "不支持的参数"
	}

	unlocated := false
	if vals, ok := values["placement"]; ok && fields["placement"] == "" {
		if vals[0] != "unlocated" {
			fields["placement"] = "不支持的参数"
		} else {
			unlocated = true
		}
	}
	var locationID int64
	hasLocationID := false
	if vals, ok := values["location"]; ok && fields["location"] == "" {
		n, good := parseDecimalID(vals[0])
		if !good {
			fields["location"] = "参数不正确"
		} else {
			locationID = n
			hasLocationID = true
		}
	}
	var inLocationID int64
	hasInLocationID := false
	if vals, ok := values["in_location"]; ok && fields["in_location"] == "" {
		n, good := parseDecimalID(vals[0])
		if !good {
			fields["in_location"] = "参数不正确"
		} else {
			inLocationID = n
			hasInLocationID = true
		}
	}
	var categoryIDs []int64
	categoryOK := false
	if vals, ok := values["category"]; ok && fields["category"] == "" {
		ids, good := parseCategoryIDs(vals[0])
		if !good {
			fields["category"] = "参数不正确"
		} else {
			categoryIDs = ids
			categoryOK = true
		}
	}
	uncategorized := false
	if vals, ok := values["uncategorized"]; ok && fields["uncategorized"] == "" {
		if vals[0] != "1" {
			fields["uncategorized"] = "不支持的参数"
		} else {
			uncategorized = true
		}
	}
	if _, ok := values["in_location_descendants"]; ok && !hasInLocationID {
		fields["in_location_descendants"] = "不支持的参数"
	}
	if _, ok := values["category_descendants"]; ok && !categoryOK {
		fields["category_descendants"] = "不支持的参数"
	}
	if _, ok := values["category_match"]; ok && (!categoryOK || len(categoryIDs) < 2) {
		fields["category_match"] = "不支持的参数"
	}
	if hasUncategorized {
		if _, ok := values["category_match"]; ok {
			fields["category_match"] = "不支持的参数"
		}
		if _, ok := values["category_descendants"]; ok {
			fields["category_descendants"] = "不支持的参数"
		}
	}

	inLocationDescendants := hasInLocationID
	if vals, ok := values["in_location_descendants"]; ok && fields["in_location_descendants"] == "" {
		switch vals[0] {
		case "1":
			inLocationDescendants = true
		case "0":
			inLocationDescendants = false
		default:
			fields["in_location_descendants"] = "不支持的参数"
		}
	}
	categoryDescendants := categoryOK
	if vals, ok := values["category_descendants"]; ok && fields["category_descendants"] == "" {
		switch vals[0] {
		case "1":
			categoryDescendants = true
		case "0":
			categoryDescendants = false
		default:
			fields["category_descendants"] = "不支持的参数"
		}
	}
	categoryMatchAll := false
	if vals, ok := values["category_match"]; ok && fields["category_match"] == "" {
		switch vals[0] {
		case "any":
			categoryMatchAll = false
		case "all":
			categoryMatchAll = true
		default:
			fields["category_match"] = "不支持的参数"
		}
	}
	keyword := ""
	if vals, ok := values["q"]; ok && fields["q"] == "" {
		k, err := catalog.NormalizeKeyword(vals[0])
		if err != nil {
			fields["q"] = err.Error()
		} else {
			keyword = k
		}
	}
	sortName := false
	if vals, ok := values["sort"]; ok && fields["sort"] == "" {
		switch vals[0] {
		case "created_at":
			sortName = false
		case "name":
			sortName = true
		default:
			fields["sort"] = "不支持的参数"
		}
	}
	limit := 30
	if vals, ok := values["limit"]; ok && fields["limit"] == "" {
		n, good := parseLimit(vals[0])
		if !good {
			fields["limit"] = "数量超出范围"
		} else {
			limit = n
		}
	}
	offset := 0
	if vals, ok := values["offset"]; ok && fields["offset"] == "" {
		n, good := parseOffset(vals[0])
		if !good {
			fields["offset"] = "起点不正确"
		} else {
			offset = n
		}
	}
	if len(fields) > 0 {
		return catalog.ItemFilter{}, fields
	}
	return catalog.ItemFilter{
		Unlocated:             unlocated,
		HasLocation:           hasLocationID,
		LocationID:            locationID,
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

func parseCategoryIDs(s string) ([]int64, bool) {
	parts := strings.Split(s, ",")
	if len(parts) == 0 || len(parts) > 20 {
		return nil, false
	}
	seen := map[int64]struct{}{}
	ids := make([]int64, 0, len(parts))
	for _, part := range parts {
		n, ok := parseDecimalID(part)
		if !ok {
			return nil, false
		}
		if _, dup := seen[n]; dup {
			return nil, false
		}
		seen[n] = struct{}{}
		ids = append(ids, n)
	}
	return ids, true
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

func (h *handler) itemPhotos(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.uploadItemPhoto(w, r)
	default:
		h.notFound(w, r)
	}
}

func (h *handler) photoByID(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodDelete:
		h.deletePhoto(w, r)
	default:
		h.notFound(w, r)
	}
}

func (h *handler) photoFirst(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.setPhotoFirst(w, r)
	default:
		h.notFound(w, r)
	}
}

func (h *handler) setPhotoFirst(w http.ResponseWriter, r *http.Request) {
	if !h.originOK(w, r) {
		return
	}
	if !h.currentUser(w, r) {
		return
	}
	body, ok := readLimited(w, r, 32768)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" {
		writeFields(w, queryRejected(r))
		return
	}
	in, err := decodeVersionWrite(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求格式不正确")
		return
	}
	id, ok := parseDecimalID(r.PathValue("id"))
	if !ok {
		h.notFound(w, r)
		return
	}
	updated, err := catalog.SetPhotoFirst(r.Context(), h.db, h.now(), id, catalog.VersionInput{
		VersionPresent: in.VersionPresent,
		Version:        in.Version,
	})
	if err != nil {
		writePhotoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toPhotoJSON(updated))
}

func (h *handler) deletePhoto(w http.ResponseWriter, r *http.Request) {
	if !h.originOK(w, r) {
		return
	}
	if !h.currentUser(w, r) {
		return
	}
	version, fields := deleteVersion(r)
	if len(fields) > 0 {
		writeFields(w, fields)
		return
	}
	id, ok := parseDecimalID(r.PathValue("id"))
	if !ok {
		h.notFound(w, r)
		return
	}
	if err := catalog.DeletePhoto(r.Context(), h.db, h.now(), id, version); err != nil {
		writePhotoError(w, err)
		return
	}
	if err := photo.Remove(h.cfg.DataDir, id); err != nil {
		slog.Error("清理照片失败", "photo_id", id, "error", err)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) photoThumbnail(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getPhotoFile(w, r, "thumbnails")
	default:
		h.notFound(w, r)
	}
}

func (h *handler) photoOriginal(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getPhotoFile(w, r, "originals")
	default:
		h.notFound(w, r)
	}
}

func (h *handler) uploadItemPhoto(w http.ResponseWriter, r *http.Request) {
	if !h.originOK(w, r) {
		return
	}
	if !h.currentUser(w, r) {
		return
	}
	if r.URL.RawQuery != "" {
		writeFields(w, queryRejected(r))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, photo.MaxUploadBytes)
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "multipart/form-data" {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求格式不正确")
		return
	}
	if err := r.ParseMultipartForm(photo.MaxUploadBytes); err != nil {
		writeMaxBytesOrInvalidBody(w, err)
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	id, ok := parseDecimalID(r.PathValue("id"))
	if !ok {
		h.notFound(w, r)
		return
	}
	file, hdr, err := r.FormFile("file")
	if file != nil {
		defer file.Close()
	}
	missing := err != nil || file == nil || hdr == nil || hdr.Size == 0
	var data []byte
	if !missing {
		data, err = io.ReadAll(file)
		if err != nil {
			writeMaxBytesOrInvalidBody(w, err)
			return
		}
		if len(data) == 0 {
			missing = true
		}
	}
	if _, err := catalog.GetItem(r.Context(), h.db, id); err != nil {
		writePhotoError(w, err)
		return
	}
	if missing {
		writeFields(w, map[string]string{"file": "没有照片文件"})
		return
	}
	out, err := photo.Transcode(data)
	if err != nil {
		writePhotoError(w, err)
		return
	}
	var placed int64
	created, err := catalog.AddPhoto(r.Context(), h.db, h.now(), id, out.Width, out.Height, int64(len(out.Original)), func(photoID int64) error {
		placed = photoID
		return photo.Place(h.cfg.DataDir, photoID, out.Original, out.Thumb)
	})
	if err != nil {
		if placed != 0 {
			_ = photo.Remove(h.cfg.DataDir, placed)
		}
		writePhotoError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toPhotoJSON(created))
}

func (h *handler) getPhotoFile(w http.ResponseWriter, r *http.Request, dir string) {
	if !h.currentUser(w, r) {
		return
	}
	if r.URL.RawQuery != "" {
		writeFields(w, queryRejected(r))
		return
	}
	id, ok := parseDecimalID(r.PathValue("id"))
	if !ok {
		h.notFound(w, r)
		return
	}
	if _, err := catalog.GetPhoto(r.Context(), h.db, id); err != nil {
		writePhotoError(w, err)
		return
	}
	f, err := os.Open(filepath.Join(h.cfg.DataDir, dir, fmt.Sprintf("%d.jpg", id)))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, f)
}

func writeMaxBytesOrInvalidBody(w http.ResponseWriter, err error) {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		writeError(w, http.StatusRequestEntityTooLarge, "body_too_large", "请求正文过大")
		return
	}
	writeError(w, http.StatusBadRequest, "invalid_body", "请求格式不正确")
}

func writePhotoError(w http.ResponseWriter, err error) {
	var fe *catalog.FieldError
	switch {
	case errors.As(err, &fe):
		writeFields(w, fe.Fields)
	case errors.Is(err, catalog.ErrVersion):
		writeError(w, http.StatusConflict, "version_conflict", "记录已被修改")
	case errors.Is(err, catalog.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "未找到")
	case errors.Is(err, catalog.ErrPhotoLimit):
		writeError(w, http.StatusConflict, "photo_limit", "一件物品最多 20 张照片")
	case errors.Is(err, photo.ErrHEIC):
		writeFields(w, map[string]string{"file": "暂不支持这种照片格式"})
	case errors.Is(err, photo.ErrTooManyPixels):
		writeFields(w, map[string]string{"file": "照片像素过多"})
	case errors.Is(err, photo.ErrNotImage):
		writeFields(w, map[string]string{"file": "不是可用的照片"})
	default:
		http.Error(w, "", http.StatusInternalServerError)
	}
}
