package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func mcpPost(h http.Handler, token, origin, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

const mcpInitialize = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`

func TestMCPRequiresBearer(t *testing.T) {
	_, h, _ := testHandler(t, false)
	rec := mcpPost(h, "", "", mcpInitialize)
	assertCode(t, rec, http.StatusUnauthorized, "unauthenticated")
	if rec.Header().Get("WWW-Authenticate") != `Bearer realm="youchu"` {
		t.Fatalf("www=%q", rec.Header().Get("WWW-Authenticate"))
	}
}

func TestMCPRejectsCookieWithoutBearer(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(mcpInitialize))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.AddCookie(&http.Cookie{Name: "youchu_session", Value: cookie})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assertCode(t, rec, http.StatusUnauthorized, "unauthenticated")
}

func createPAT(t *testing.T, h http.Handler, cookie, body string) (int64, string) {
	t.Helper()
	created := request(h, http.MethodPost, "/api/v1/tokens", body, "http://127.0.0.1:5173", "203.0.113.5:1", cookie)
	if created.Code != http.StatusCreated {
		t.Fatalf("create=%d %s", created.Code, created.Body.String())
	}
	var tok struct {
		ID    int64  `json:"id"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &tok); err != nil {
		t.Fatal(err)
	}
	if tok.ID == 0 || tok.Token == "" {
		t.Fatalf("id=%d token=%q", tok.ID, tok.Token)
	}
	return tok.ID, tok.Token
}

func TestMCPOriginAndInitialize(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)
	_, token := createPAT(t, h, cookie, `{"name":"mcp","scopes":["read"]}`)
	evil := mcpPost(h, token, "https://evil.example", mcpInitialize)
	assertCode(t, evil, http.StatusForbidden, "origin_rejected")
	ok := mcpPost(h, token, "", mcpInitialize)
	if ok.Code != http.StatusOK || !strings.Contains(ok.Body.String(), `"name":"youchu"`) {
		t.Fatalf("init=%d %s", ok.Code, ok.Body.String())
	}
	local := mcpPost(h, token, "http://127.0.0.1:6274", mcpInitialize)
	if local.Code != http.StatusOK {
		t.Fatalf("local origin=%d %s", local.Code, local.Body.String())
	}
}

func TestRevokedTokenRejectedOnMCP(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)
	id, token := createPAT(t, h, cookie, `{"name":"mcp","scopes":["read"]}`)
	del := request(h, http.MethodDelete, "/api/v1/tokens/"+strconv.FormatInt(id, 10), "", "http://127.0.0.1:5173", "203.0.113.5:1", cookie)
	if del.Code != http.StatusNoContent {
		t.Fatalf("del=%d %s", del.Code, del.Body.String())
	}
	rec := mcpPost(h, token, "", mcpInitialize)
	assertCode(t, rec, http.StatusUnauthorized, "unauthenticated")
}

func mcpRPC(t *testing.T, h http.Handler, token, body string) map[string]any {
	t.Helper()
	rec := mcpPost(h, token, "", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("mcp status=%d body=%s", rec.Code, rec.Body.String())
	}
	return parseMCPMessage(t, rec.Body.String())
}

func parseMCPMessage(t *testing.T, body string) map[string]any {
	t.Helper()
	if obj, ok := tryJSONObject([]byte(strings.TrimSpace(body))); ok {
		return obj
	}
	var data strings.Builder
	flush := func() map[string]any {
		s := strings.TrimSpace(data.String())
		data.Reset()
		obj, ok := tryJSONObject([]byte(s))
		if ok {
			return obj
		}
		return nil
	}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case strings.HasPrefix(line, "data:"):
			payload := strings.TrimPrefix(line, "data:")
			payload = strings.TrimPrefix(payload, " ")
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(payload)
		case line == "":
			if obj := flush(); obj != nil {
				return obj
			}
		}
	}
	if obj := flush(); obj != nil {
		return obj
	}
	t.Fatalf("mcp body is not json-rpc: %s", body)
	return nil
}

func tryJSONObject(raw []byte) (map[string]any, bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '{' {
		return nil, false
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, false
	}
	return obj, true
}

func mcpCall(t *testing.T, h http.Handler, token, name string, args any) map[string]any {
	t.Helper()
	if args == nil {
		args = map[string]any{}
	}
	payload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params":  map[string]any{"name": name, "arguments": args},
	})
	if err != nil {
		t.Fatal(err)
	}
	msg := mcpRPC(t, h, token, string(payload))
	if errObj, ok := msg["error"]; ok {
		t.Fatalf("tools/call %s error=%v", name, errObj)
	}
	result, ok := msg["result"].(map[string]any)
	if !ok {
		t.Fatalf("result=%T %v", msg["result"], msg["result"])
	}
	return result
}

func mcpToolTextJSON(t *testing.T, result map[string]any) []byte {
	t.Helper()
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("content=%v", result["content"])
	}
	first, ok := content[0].(map[string]any)
	if !ok {
		t.Fatalf("content[0]=%T %v", content[0], content[0])
	}
	text, ok := first["text"].(string)
	if !ok {
		t.Fatalf("text=%v", first)
	}
	return []byte(text)
}

func TestMCPSearchItems(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)
	cat := mustCreateCat(t, h, cookie, `{"name":"玩具"}`)
	item := mustItem(t, h, cookie, fmt.Sprintf(`{"name":"遥控车","categories":[{"category_id":%d}]}`, cat.ID))
	_, token := createPAT(t, h, cookie, `{"name":"mcp","scopes":["read"]}`)

	result := mcpCall(t, h, token, "youchu_search_items", map[string]any{})
	var page map[string]any
	if err := json.Unmarshal(mcpToolTextJSON(t, result), &page); err != nil {
		t.Fatal(err)
	}
	if _, ok := mcpNumber(page["total"]); !ok {
		t.Fatalf("total=%T %v", page["total"], page["total"])
	}
	found := false
	rows, _ := page["data"].([]any)
	for _, row := range rows {
		m, _ := row.(map[string]any)
		if m["name"] == "遥控车" {
			found = true
			id, ok := mcpInt64(m["id"])
			if !ok || id != item.ID {
				t.Fatalf("search id=%v want %d", m["id"], item.ID)
			}
		}
	}
	if !found {
		t.Fatalf("search missing 遥控车: %s", mcpToolTextJSON(t, result))
	}

	got := mcpCall(t, h, token, "youchu_get_item", map[string]any{"id": item.ID})
	var detail map[string]any
	if err := json.Unmarshal(mcpToolTextJSON(t, got), &detail); err != nil {
		t.Fatal(err)
	}
	id, ok := mcpInt64(detail["id"])
	if !ok || id != item.ID {
		t.Fatalf("get id=%v want %d", detail["id"], item.ID)
	}
	cats, _ := detail["categories"].([]any)
	if len(cats) != 1 {
		t.Fatalf("categories=%v", detail["categories"])
	}
	link, _ := cats[0].(map[string]any)
	if link["source"] != "human" {
		t.Fatalf("source=%v", link["source"])
	}
}

func TestMCPReadToolList(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)
	_, token := createPAT(t, h, cookie, `{"name":"mcp","scopes":["read"]}`)

	msg := mcpRPC(t, h, token, `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	if errObj, ok := msg["error"]; ok {
		t.Fatalf("tools/list error=%v", errObj)
	}
	result, ok := msg["result"].(map[string]any)
	if !ok {
		t.Fatalf("result=%T %v", msg["result"], msg["result"])
	}
	tools, _ := result["tools"].([]any)
	want := map[string]struct{}{
		"youchu_search_items":              {},
		"youchu_get_item":                  {},
		"youchu_list_locations":            {},
		"youchu_get_location":              {},
		"youchu_list_categories":           {},
		"youchu_get_category":              {},
		"youchu_list_return_tasks":         {},
		"youchu_get_return_task":           {},
		"youchu_list_trash":                {},
		"youchu_get_trash_item":            {},
		"youchu_get_photo":                 {},
		"youchu_create_category":           {},
		"youchu_add_item_categories":       {},
		"youchu_remove_ai_item_categories": {},
		"youchu_create_item":               {},
		"youchu_update_item":               {},
		"youchu_create_location":           {},
		"youchu_update_location":           {},
		"youchu_update_category":           {},
		"youchu_trash_item":                {},
		"youchu_restore_item":              {},
		"youchu_create_return_task":        {},
		"youchu_complete_return_task":      {},
		"youchu_delete_return_task":        {},
	}
	got := map[string]struct{}{}
	for _, raw := range tools {
		tool, _ := raw.(map[string]any)
		name, _ := tool["name"].(string)
		if name == "" {
			t.Fatalf("tool=%v", raw)
		}
		got[name] = struct{}{}
	}
	if len(got) != len(want) {
		t.Fatalf("tools=%v", got)
	}
	for name := range want {
		if _, ok := got[name]; !ok {
			t.Fatalf("missing %s in %v", name, got)
		}
	}
	for _, name := range []string{
		"youchu_purge_item",
		"youchu_delete_location",
		"youchu_delete_category",
		"youchu_upload_photo",
		"youchu_delete_photo",
		"youchu_set_photo_first",
	} {
		if _, ok := got[name]; ok {
			t.Fatalf("%s should not be listed", name)
		}
	}

	payload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "tools/call",
		"params":  map[string]any{"name": "youchu_create_item", "arguments": map[string]any{"name": "不应创建"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	create := mcpRPC(t, h, token, string(payload))
	if errObj, ok := create["error"].(map[string]any); ok {
		msgText, _ := errObj["message"].(string)
		if !strings.Contains(msgText, "unknown tool") {
			t.Fatalf("create error=%v", errObj)
		}
		return
	}
	res, ok := create["result"].(map[string]any)
	if !ok {
		t.Fatalf("create=%v", create)
	}
	if res["isError"] != true {
		t.Fatalf("expected forbidden tool error: %v", res)
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(mcpToolTextJSON(t, res), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != "forbidden" {
		t.Fatalf("code=%q", body.Code)
	}
}

func TestMCPGetPhoto(t *testing.T) {
	_, h, _, _ := testHandlerDir(t, false)
	cookie := login(t, h)
	item := mustItem(t, h, cookie, `{"name":"遥控车"}`)
	photo := assertPhotoCreated(t, postPhoto(t, h, cookie, itemPhotosURL(item.ID), encodeTestJPEG(t, 8, 8)))
	_, token := createPAT(t, h, cookie, `{"name":"mcp","scopes":["read"]}`)

	result := mcpCall(t, h, token, "youchu_get_photo", map[string]any{"id": photo.ID})
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("content=%v", result["content"])
	}
	first, _ := content[0].(map[string]any)
	if first["type"] != "image" {
		t.Fatalf("type=%v", first["type"])
	}
	if first["mimeType"] != "image/jpeg" {
		t.Fatalf("mimeType=%v", first["mimeType"])
	}
	dataStr, _ := first["data"].(string)
	raw, err := base64.StdEncoding.DecodeString(dataStr)
	if err != nil {
		t.Fatal(err)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if format != "jpeg" {
		t.Fatalf("format=%q", format)
	}
	if cfg.Width < 1 || cfg.Height < 1 {
		t.Fatalf("size=%dx%d", cfg.Width, cfg.Height)
	}
	if _, err := jpeg.Decode(bytes.NewReader(raw)); err != nil {
		t.Fatalf("jpeg: %v", err)
	}
	thumb := request(h, http.MethodGet, photoThumbURL(photo.ID), "", "", testRemote, cookie)
	if thumb.Code != http.StatusOK {
		t.Fatalf("web thumb=%d %s", thumb.Code, thumb.Body.String())
	}
}

func TestMCPOrganize(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)
	catA := mustCreateCat(t, h, cookie, `{"name":"分类A"}`)
	catB := mustCreateCat(t, h, cookie, `{"name":"分类B"}`)
	item := mustItem(t, h, cookie, fmt.Sprintf(`{"name":"遥控车","categories":[{"category_id":%d}]}`, catA.ID))
	if len(item.Categories) != 1 || item.Categories[0].CategoryID != catA.ID || item.Categories[0].Source != "human" {
		t.Fatalf("item cats=%+v", item.Categories)
	}
	beforeVersion := item.Version
	_, token := createPAT(t, h, cookie, `{"name":"mcp","scopes":["read","organize"]}`)

	added := mcpCall(t, h, token, "youchu_add_item_categories", map[string]any{
		"item_id": item.ID, "category_ids": []int64{catB.ID},
	})
	if added["isError"] == true {
		t.Fatalf("add B: %s", mcpToolTextJSON(t, added))
	}
	var afterAdd itemBody
	if err := json.Unmarshal(mcpToolTextJSON(t, added), &afterAdd); err != nil {
		t.Fatal(err)
	}
	if afterAdd.Version != beforeVersion {
		t.Fatalf("version after add=%d want %d", afterAdd.Version, beforeVersion)
	}
	byID := map[int64]itemCatBody{}
	for _, c := range afterAdd.Categories {
		byID[c.CategoryID] = c
	}
	if byID[catA.ID].Source != "human" {
		t.Fatalf("A source=%q", byID[catA.ID].Source)
	}
	if byID[catB.ID].Source != "ai" {
		t.Fatalf("B source=%q", byID[catB.ID].Source)
	}

	removeA := mcpCall(t, h, token, "youchu_remove_ai_item_categories", map[string]any{
		"item_id": item.ID, "category_ids": []int64{catA.ID},
	})
	if removeA["isError"] != true {
		t.Fatalf("remove A expected error: %v", removeA)
	}
	var removeAErr struct {
		Code   string            `json:"code"`
		Fields map[string]string `json:"fields"`
	}
	if err := json.Unmarshal(mcpToolTextJSON(t, removeA), &removeAErr); err != nil {
		t.Fatal(err)
	}
	if removeAErr.Code != "invalid_fields" || removeAErr.Fields["category_ids"] != "不能去掉人工分类" {
		t.Fatalf("remove A err=%+v", removeAErr)
	}
	still := requireItem(t, h, cookie, item.ID)
	if still.Version != beforeVersion {
		t.Fatalf("version after failed remove=%d", still.Version)
	}
	stillByID := map[int64]itemCatBody{}
	for _, c := range still.Categories {
		stillByID[c.CategoryID] = c
	}
	if stillByID[catA.ID].Source != "human" || stillByID[catB.ID].Source != "ai" || len(still.Categories) != 2 {
		t.Fatalf("want both rows, got %+v", still.Categories)
	}

	removeB := mcpCall(t, h, token, "youchu_remove_ai_item_categories", map[string]any{
		"item_id": item.ID, "category_ids": []int64{catB.ID},
	})
	if removeB["isError"] == true {
		t.Fatalf("remove B: %s", mcpToolTextJSON(t, removeB))
	}
	var afterRemove itemBody
	if err := json.Unmarshal(mcpToolTextJSON(t, removeB), &afterRemove); err != nil {
		t.Fatal(err)
	}
	if afterRemove.Version != beforeVersion {
		t.Fatalf("version after remove B=%d", afterRemove.Version)
	}
	if len(afterRemove.Categories) != 1 || afterRemove.Categories[0].CategoryID != catA.ID || afterRemove.Categories[0].Source != "human" {
		t.Fatalf("after remove B=%+v", afterRemove.Categories)
	}

	_, readToken := createPAT(t, h, cookie, `{"name":"read","scopes":["read"]}`)
	forbidden := mcpCall(t, h, readToken, "youchu_add_item_categories", map[string]any{
		"item_id": item.ID, "category_ids": []int64{catB.ID},
	})
	if forbidden["isError"] != true {
		t.Fatalf("read add expected error: %v", forbidden)
	}
	var forb struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(mcpToolTextJSON(t, forbidden), &forb); err != nil {
		t.Fatal(err)
	}
	if forb.Code != "forbidden" {
		t.Fatalf("code=%q", forb.Code)
	}
	onlyA := requireItem(t, h, cookie, item.ID)
	if len(onlyA.Categories) != 1 || onlyA.Categories[0].CategoryID != catA.ID {
		t.Fatalf("read add mutated: %+v", onlyA.Categories)
	}

	created := mcpCall(t, h, token, "youchu_create_category", map[string]any{"name": "AI类"})
	if created["isError"] == true {
		t.Fatalf("create category: %s", mcpToolTextJSON(t, created))
	}
	var newCat catBody
	if err := json.Unmarshal(mcpToolTextJSON(t, created), &newCat); err != nil {
		t.Fatal(err)
	}
	if newCat.ID < 1 {
		t.Fatalf("create category id=%d", newCat.ID)
	}
	got := requireCat(t, h, cookie, newCat.ID)
	if got.Name != "AI类" {
		t.Fatalf("web get name=%q", got.Name)
	}

	missing := mcpCall(t, h, token, "youchu_add_item_categories", map[string]any{
		"item_id": int64(999999999), "category_ids": []int64{-1},
	})
	if missing["isError"] != true {
		t.Fatalf("missing item expected error: %v", missing)
	}
	var miss struct {
		Code   string            `json:"code"`
		Fields map[string]string `json:"fields"`
	}
	if err := json.Unmarshal(mcpToolTextJSON(t, missing), &miss); err != nil {
		t.Fatal(err)
	}
	if miss.Code != "not_found" {
		t.Fatalf("missing item code=%q fields=%v", miss.Code, miss.Fields)
	}
}

func TestMCPWrite(t *testing.T) {
	db, h, _ := testHandler(t, false)
	cookie := login(t, h)
	_, readToken := createPAT(t, h, cookie, `{"name":"read","scopes":["read"]}`)

	var before int
	if err := db.QueryRow(`SELECT COUNT(*) FROM items`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "tools/call",
		"params":  map[string]any{"name": "youchu_create_item", "arguments": map[string]any{"name": "遥控车"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	denied := mcpRPC(t, h, readToken, string(payload))
	if errObj, ok := denied["error"]; ok {
		t.Fatalf("expected tool forbidden, got protocol error=%v", errObj)
	}
	deniedRes, ok := denied["result"].(map[string]any)
	if !ok {
		t.Fatalf("denied=%v", denied)
	}
	if deniedRes["isError"] != true {
		t.Fatalf("expected forbidden: %v", deniedRes)
	}
	var forb struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(mcpToolTextJSON(t, deniedRes), &forb); err != nil {
		t.Fatal(err)
	}
	if forb.Code != "forbidden" {
		t.Fatalf("code=%q", forb.Code)
	}
	var after int
	if err := db.QueryRow(`SELECT COUNT(*) FROM items`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("items count %d -> %d", before, after)
	}

	_, writeToken := createPAT(t, h, cookie, `{"name":"write","scopes":["read","write"]}`)
	created := mcpCall(t, h, writeToken, "youchu_create_item", map[string]any{"name": "遥控车"})
	if created["isError"] == true {
		t.Fatalf("create: %s", mcpToolTextJSON(t, created))
	}
	var item struct {
		ID      int64  `json:"id"`
		Name    string `json:"name"`
		Version int64  `json:"version"`
	}
	if err := json.Unmarshal(mcpToolTextJSON(t, created), &item); err != nil {
		t.Fatal(err)
	}
	if item.ID < 1 || item.Name != "遥控车" {
		t.Fatalf("created=%+v", item)
	}
	web := getItem(h, cookie, itemURL(item.ID))
	if web.Code != http.StatusOK {
		t.Fatalf("web get=%d %s", web.Code, web.Body.String())
	}
	got := decodeItem(t, web)
	if got.Name != "遥控车" {
		t.Fatalf("web name=%q", got.Name)
	}

	msg := mcpRPC(t, h, writeToken, `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	if errObj, ok := msg["error"]; ok {
		t.Fatalf("tools/list error=%v", errObj)
	}
	list, ok := msg["result"].(map[string]any)
	if !ok {
		t.Fatalf("list=%v", msg)
	}
	tools, _ := list["tools"].([]any)
	names := map[string]struct{}{}
	for _, raw := range tools {
		tool, _ := raw.(map[string]any)
		name, _ := tool["name"].(string)
		names[name] = struct{}{}
	}
	for _, name := range []string{
		"youchu_purge_item",
		"youchu_delete_location",
		"youchu_delete_category",
		"youchu_upload_photo",
	} {
		if _, ok := names[name]; ok {
			t.Fatalf("listed %s", name)
		}
	}

	version := item.Version
	if version < 1 {
		version = got.Version
	}
	trashed := mcpCall(t, h, writeToken, "youchu_trash_item", map[string]any{"id": item.ID, "version": version})
	if trashed["isError"] == true {
		t.Fatalf("trash: %s", mcpToolTextJSON(t, trashed))
	}
	live := getItem(h, cookie, itemURL(item.ID))
	if live.Code != http.StatusNotFound {
		t.Fatalf("live after trash=%d %s", live.Code, live.Body.String())
	}
	fromTrash := getItem(h, cookie, trashURL(item.ID))
	if fromTrash.Code != http.StatusOK {
		t.Fatalf("trash get=%d %s", fromTrash.Code, fromTrash.Body.String())
	}

	parentCat := mustCreateCat(t, h, cookie, `{"name":"玩具"}`)
	childCat := mustCreateCat(t, h, cookie, `{"name":"遥控"}`)
	movedCat := mcpCall(t, h, writeToken, "youchu_update_category", map[string]any{
		"id": childCat.ID, "version": childCat.Version, "parent_id": parentCat.ID,
	})
	if movedCat["isError"] == true {
		t.Fatalf("update category parent int: %s", mcpToolTextJSON(t, movedCat))
	}
	var catJSON struct {
		ParentID *int64 `json:"parent_id"`
		Version  int64  `json:"version"`
	}
	if err := json.Unmarshal(mcpToolTextJSON(t, movedCat), &catJSON); err != nil {
		t.Fatal(err)
	}
	if catJSON.ParentID == nil || *catJSON.ParentID != parentCat.ID {
		t.Fatalf("category parent_id=%v want %d", catJSON.ParentID, parentCat.ID)
	}
	rootedCat := mcpCall(t, h, writeToken, "youchu_update_category", map[string]any{
		"id": childCat.ID, "version": catJSON.Version, "parent_id": nil,
	})
	if rootedCat["isError"] == true {
		t.Fatalf("update category parent null: %s", mcpToolTextJSON(t, rootedCat))
	}
	if err := json.Unmarshal(mcpToolTextJSON(t, rootedCat), &catJSON); err != nil {
		t.Fatal(err)
	}
	if catJSON.ParentID != nil {
		t.Fatalf("category parent_id after null=%v", catJSON.ParentID)
	}

	parentLoc := mustCreate(t, h, cookie, `{"name":"客厅","type":"area"}`)
	childLoc := mustCreate(t, h, cookie, `{"name":"阳台","type":"area"}`)
	movedLoc := mcpCall(t, h, writeToken, "youchu_update_location", map[string]any{
		"id": childLoc.ID, "version": childLoc.Version, "parent_id": parentLoc.ID,
	})
	if movedLoc["isError"] == true {
		t.Fatalf("update location parent int: %s", mcpToolTextJSON(t, movedLoc))
	}
	var locJSON struct {
		ParentID *int64 `json:"parent_id"`
		Version  int64  `json:"version"`
	}
	if err := json.Unmarshal(mcpToolTextJSON(t, movedLoc), &locJSON); err != nil {
		t.Fatal(err)
	}
	if locJSON.ParentID == nil || *locJSON.ParentID != parentLoc.ID {
		t.Fatalf("location parent_id=%v want %d", locJSON.ParentID, parentLoc.ID)
	}
	rootedLoc := mcpCall(t, h, writeToken, "youchu_update_location", map[string]any{
		"id": childLoc.ID, "version": locJSON.Version, "parent_id": nil,
	})
	if rootedLoc["isError"] == true {
		t.Fatalf("update location parent null: %s", mcpToolTextJSON(t, rootedLoc))
	}
	if err := json.Unmarshal(mcpToolTextJSON(t, rootedLoc), &locJSON); err != nil {
		t.Fatal(err)
	}
	if locJSON.ParentID != nil {
		t.Fatalf("location parent_id after null=%v", locJSON.ParentID)
	}
}

type bearerRT struct {
	token string
	base  http.RoundTripper
}

func (b bearerRT) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	rt := b.base
	if rt == nil {
		rt = http.DefaultTransport
	}
	return rt.RoundTrip(r)
}

func mcpSearchHasName(t *testing.T, raw []byte, name string) bool {
	t.Helper()
	var page map[string]any
	if err := json.Unmarshal(raw, &page); err != nil {
		t.Fatal(err)
	}
	rows, _ := page["data"].([]any)
	for _, row := range rows {
		m, _ := row.(map[string]any)
		if m["name"] == name {
			return true
		}
	}
	return false
}

func TestMCPTwoClients(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)
	_, token := createPAT(t, h, cookie, `{"name":"mcp","scopes":["read","write"]}`)
	created := mcpCall(t, h, token, "youchu_create_item", map[string]any{"name": "遥控车"})
	if created["isError"] == true {
		t.Fatalf("create: %s", mcpToolTextJSON(t, created))
	}

	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	ctx := context.Background()
	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil)
	session, err := client.Connect(ctx, &sdk.StreamableClientTransport{
		Endpoint:             srv.URL + "/mcp",
		HTTPClient:           &http.Client{Transport: bearerRT{token: token}},
		DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })

	res, err := session.CallTool(ctx, &sdk.CallToolParams{
		Name:      "youchu_search_items",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("sdk search error: %+v", res)
	}
	if len(res.Content) == 0 {
		t.Fatal("sdk search empty content")
	}
	text, ok := res.Content[0].(*sdk.TextContent)
	if !ok {
		t.Fatalf("content=%T", res.Content[0])
	}
	if !mcpSearchHasName(t, []byte(text.Text), "遥控车") {
		t.Fatalf("sdk missing 遥控车: %s", text.Text)
	}

	raw := mcpCall(t, h, token, "youchu_search_items", map[string]any{})
	if raw["isError"] == true {
		t.Fatalf("rpc search: %s", mcpToolTextJSON(t, raw))
	}
	if !mcpSearchHasName(t, mcpToolTextJSON(t, raw), "遥控车") {
		t.Fatalf("rpc missing 遥控车: %s", mcpToolTextJSON(t, raw))
	}
}

func mcpNumber(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}

func mcpInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		i := int64(n)
		return i, float64(i) == n
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	case int:
		return int64(n), true
	case int64:
		return n, true
	default:
		return 0, false
	}
}
