package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"youchu/internal/auth"
	"youchu/internal/config"
	"youchu/internal/migrate"
	"youchu/internal/photo"
	"youchu/migrations"
)

const (
	webOrigin  = "http://127.0.0.1:5173"
	frozenAt   = "2026-09-29T12:00:00Z"
	testRemote = "203.0.113.40:9"
)

func TestMigration002(t *testing.T) {
	db := migratedDB(t)
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = 2`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("version 2 rows = %d", n)
	}
	for _, name := range []string{"locations", "items"} {
		var sqlText string
		if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE name = ?`, name).Scan(&sqlText); err != nil {
			t.Fatal(name, err)
		}
		if !strings.Contains(sqlText, "AUTOINCREMENT") {
			t.Fatalf("%s missing AUTOINCREMENT: %s", name, sqlText)
		}
	}
	var indexSQL string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE name = 'locations_code_unique'`).Scan(&indexSQL); err != nil {
		t.Fatal(err)
	}
	files, err := loadEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	if err := migrate.Apply(context.Background(), db, files, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = 2`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("second apply rows = %d", n)
	}
}

func TestMigration003(t *testing.T) {
	db := migratedDB(t)
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = 3`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("version 3 rows = %d", n)
	}
	var sqlText string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE name = 'categories'`).Scan(&sqlText); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sqlText, "AUTOINCREMENT") {
		t.Fatalf("categories missing AUTOINCREMENT: %s", sqlText)
	}
	for _, name := range []string{"categories_root_name", "categories_sibling_name"} {
		var indexSQL string
		if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE name = ?`, name).Scan(&indexSQL); err != nil {
			t.Fatal(name, err)
		}
		if !strings.Contains(indexSQL, "UNIQUE") {
			t.Fatalf("%s not unique: %s", name, indexSQL)
		}
	}
	var pk string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE name = 'item_categories'`).Scan(&pk); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(pk, "PRIMARY KEY") {
		t.Fatalf("item_categories pk: %s", pk)
	}
	files, err := loadEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	if err := migrate.Apply(context.Background(), db, files, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = 3`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("second apply rows = %d", n)
	}
}

func TestMigration004(t *testing.T) {
	db := migratedDB(t)
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = 4`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("version 4 rows = %d", n)
	}
	var sqlText string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE name = 'return_tasks'`).Scan(&sqlText); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sqlText, "AUTOINCREMENT") {
		t.Fatalf("return_tasks missing AUTOINCREMENT: %s", sqlText)
	}
	if strings.Contains(sqlText, "location_id") {
		t.Fatalf("return_tasks must not store location_id: %s", sqlText)
	}
	if _, err := db.Exec(`SELECT deleted_at FROM items`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"items_deleted_at", "return_tasks_item_id", "return_tasks_open"} {
		var indexSQL string
		if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE name = ?`, name).Scan(&indexSQL); err != nil {
			t.Fatal(name, err)
		}
		if name == "items_deleted_at" && !strings.Contains(indexSQL, "deleted_at IS NOT NULL") {
			t.Fatalf("%s not partial: %s", name, indexSQL)
		}
	}
	files, err := loadEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	if err := migrate.Apply(context.Background(), db, files, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = 4`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("second apply rows = %d", n)
	}
}

func TestMigration005(t *testing.T) {
	db := migratedDB(t)
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = 5`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("version 5 rows = %d", n)
	}
	var sqlText string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE name = 'photos'`).Scan(&sqlText); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sqlText, "AUTOINCREMENT") {
		t.Fatalf("photos missing AUTOINCREMENT: %s", sqlText)
	}
	if !strings.Contains(sqlText, "UNIQUE") {
		t.Fatalf("photos missing UNIQUE: %s", sqlText)
	}
	var indexSQL string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE name = 'photos_item_id'`).Scan(&indexSQL); err != nil {
		t.Fatal(err)
	}
	files, err := loadEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	if err := migrate.Apply(context.Background(), db, files, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = 5`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("second apply rows = %d", n)
	}
}

func TestMigration006(t *testing.T) {
	db := migratedDB(t)
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = 6`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("version 6 rows = %d", n)
	}
	var sqlText string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE name = 'access_tokens'`).Scan(&sqlText); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sqlText, "AUTOINCREMENT") {
		t.Fatalf("access_tokens: %s", sqlText)
	}
	var col string
	if err := db.QueryRow(`SELECT name FROM pragma_table_info('item_categories') WHERE name = 'source'`).Scan(&col); err != nil {
		t.Fatal(err)
	}
	if col != "source" {
		t.Fatalf("source column=%q", col)
	}
	files, err := loadEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	if err := migrate.Apply(context.Background(), db, files, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = 6`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("second apply rows = %d", n)
	}
}

func TestMigration007(t *testing.T) {
	db := migratedDB(t)
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = 7`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("version 7 rows = %d", n)
	}
	var col string
	if err := db.QueryRow(`SELECT name FROM pragma_table_info('locations') WHERE name = 'icon'`).Scan(&col); err != nil {
		t.Fatal(err)
	}
	if col != "icon" {
		t.Fatalf("icon column=%q", col)
	}
}

func TestLocationIcon(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)
	created := mustCreate(t, h, cookie, `{"name":"厨房","type":"area"}`)
	if created.Icon != nil {
		t.Fatalf("new icon=%v", created.Icon)
	}
	if created.Path[0].Icon != nil {
		t.Fatalf("path icon=%v", created.Path[0].Icon)
	}

	rec := postLoc(h, cookie, "/api/v1/locations", `{"name":"灶台","type":"area","icon":"kitchen"}`)
	kitchen := assertCreated(t, rec)
	if kitchen.Icon == nil || *kitchen.Icon != "kitchen" {
		t.Fatalf("kitchen icon=%v body=%s", kitchen.Icon, rec.Body.String())
	}
	if kitchen.Path[0].Icon == nil || *kitchen.Path[0].Icon != "kitchen" {
		t.Fatalf("kitchen path icon=%v", kitchen.Path[0].Icon)
	}

	rec = postLoc(h, cookie, "/api/v1/locations", `{"name":"坏图标","type":"area","icon":"nope"}`)
	assertInvalidFields(t, rec, map[string]string{"icon": "图标不正确"})
	rec = postLoc(h, cookie, "/api/v1/locations", `{"name":"空图标","type":"area","icon":""}`)
	assertInvalidFields(t, rec, map[string]string{"icon": "图标不正确"})

	rec = patchLoc(h, cookie, fmt.Sprintf("/api/v1/locations/%d", created.ID), fmt.Sprintf(`{"version":%d,"icon":"living"}`, created.Version))
	if rec.Code != http.StatusOK {
		t.Fatalf("patch living status=%d body=%s", rec.Code, rec.Body.String())
	}
	updated := decodeLoc(t, rec)
	if updated.Icon == nil || *updated.Icon != "living" {
		t.Fatalf("living icon=%v", updated.Icon)
	}

	rec = patchLoc(h, cookie, fmt.Sprintf("/api/v1/locations/%d", updated.ID), fmt.Sprintf(`{"version":%d,"icon":null}`, updated.Version))
	cleared := decodeLoc(t, rec)
	if rec.Code != http.StatusOK || cleared.Icon != nil {
		t.Fatalf("clear icon status=%d icon=%v body=%s", rec.Code, cleared.Icon, rec.Body.String())
	}

	item := mustItem(t, h, cookie, fmt.Sprintf(`{"name":"锅","locations":[{"location_id":%d}]}`, kitchen.ID))
	if len(item.Locations) != 1 || item.Locations[0].Path[0].Icon == nil || *item.Locations[0].Path[0].Icon != "kitchen" {
		t.Fatalf("item path icon=%+v", item.Locations)
	}

	rec = postLoc(h, "", "/api/v1/locations", `{"name":"无会话","type":"area","icon":"kitchen"}`)
	assertCode(t, rec, http.StatusUnauthorized, "unauthenticated")
}

func TestMigration008(t *testing.T) {
	db := migratedDB(t)
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = 8`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("version 8 rows = %d", n)
	}
	var name string
	if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'location_icons'`).Scan(&name); err != nil {
		t.Fatal(err)
	}
	var col string
	if err := db.QueryRow(`SELECT name FROM pragma_table_info('locations') WHERE name = 'custom_icon_id'`).Scan(&col); err != nil {
		t.Fatal(err)
	}
	if col != "custom_icon_id" {
		t.Fatalf("custom_icon_id column=%q", col)
	}
}

const testSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M4 8h16v12H4z" fill="#111111"/></svg>`

type iconBody struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	SVG       string `json:"svg"`
	Version   int64  `json:"version"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func TestLocationIconLibrary(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)

	rec := request(h, http.MethodPost, "/api/v1/location-icons", `{"name":"药箱","svg":`+jsonString(testSVG)+`}`, webOrigin, testRemote, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create icon status=%d body=%s", rec.Code, rec.Body.String())
	}
	created := decodeIcon(t, rec)
	if created.Name != "药箱" || created.ID < 1 || !strings.Contains(created.SVG, "currentColor") || strings.Contains(created.SVG, "#111111") {
		t.Fatalf("icon=%+v", created)
	}

	rec = request(h, http.MethodPatch, fmt.Sprintf("/api/v1/location-icons/%d", created.ID), fmt.Sprintf(`{"version":%d,"name":"急救箱"}`, created.Version), webOrigin, testRemote, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("rename status=%d body=%s", rec.Code, rec.Body.String())
	}
	created = decodeIcon(t, rec)
	if created.Name != "急救箱" {
		t.Fatalf("renamed=%+v", created)
	}

	rec = request(h, http.MethodPost, "/api/v1/location-icons", `{"name":"坏","svg":"<svg viewBox=\"0 0 24 24\"><script>x</script></svg>"}`, webOrigin, testRemote, cookie)
	assertInvalidFields(t, rec, map[string]string{"svg": "图标不正确"})

	rec = request(h, http.MethodPost, "/api/v1/location-icons", `{"name":"空","svg":""}`, webOrigin, testRemote, cookie)
	assertInvalidFields(t, rec, map[string]string{"svg": "图标不正确"})

	rec = request(h, http.MethodPost, "/api/v1/location-icons", `{"name":"大","svg":`+jsonString(strings.Repeat("x", 16385))+`}`, webOrigin, testRemote, cookie)
	assertInvalidFields(t, rec, map[string]string{"svg": "图标过大"})

	rec = getLoc(h, cookie, "/api/v1/location-icons")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"data":[`) {
		t.Fatalf("list status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = getLoc(h, cookie, "/api/v1/location-icons?foo=1")
	assertInvalidFields(t, rec, map[string]string{"foo": "不支持的参数"})
	rec = getLoc(h, cookie, fmt.Sprintf("/api/v1/location-icons/%d", created.ID))
	if rec.Code != http.StatusOK || decodeIcon(t, rec).ID != created.ID {
		t.Fatalf("get icon status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = postLoc(h, cookie, "/api/v1/locations", fmt.Sprintf(`{"name":"药柜","type":"area","custom_icon_id":%d}`, created.ID))
	loc := assertCreated(t, rec)
	if loc.Icon != nil || loc.CustomIconID == nil || *loc.CustomIconID != created.ID {
		t.Fatalf("loc icon=%v custom=%v body=%s", loc.Icon, loc.CustomIconID, rec.Body.String())
	}
	if loc.Path[0].CustomIconID == nil || *loc.Path[0].CustomIconID != created.ID {
		t.Fatalf("path custom=%v", loc.Path[0].CustomIconID)
	}

	rec = postLoc(h, cookie, "/api/v1/locations", fmt.Sprintf(`{"name":"冲突","type":"area","icon":"kitchen","custom_icon_id":%d}`, created.ID))
	assertInvalidFields(t, rec, map[string]string{"icon": "不能同时选用内置和自传图标", "custom_icon_id": "不能同时选用内置和自传图标"})

	rec = postLoc(h, cookie, "/api/v1/locations", `{"name":"没有","type":"area","custom_icon_id":999}`)
	assertInvalidFields(t, rec, map[string]string{"custom_icon_id": "图标不存在"})

	item := mustItem(t, h, cookie, fmt.Sprintf(`{"name":"膏药","locations":[{"location_id":%d}]}`, loc.ID))
	if len(item.Locations) != 1 || item.Locations[0].Path[0].CustomIconID == nil || *item.Locations[0].Path[0].CustomIconID != created.ID {
		t.Fatalf("item path=%+v", item.Locations)
	}

	rec = request(h, http.MethodDelete, fmt.Sprintf("/api/v1/location-icons/%d?version=%d", created.ID, created.Version), "", webOrigin, testRemote, cookie)
	assertError(t, rec, http.StatusConflict, "icon_in_use", "有位置正在使用")

	rec = patchLoc(h, cookie, locURL(loc.ID), fmt.Sprintf(`{"version":%d,"icon":null}`, loc.Version))
	if rec.Code != http.StatusOK {
		t.Fatalf("clear custom status=%d body=%s", rec.Code, rec.Body.String())
	}
	cleared := decodeLoc(t, rec)
	if cleared.CustomIconID != nil || cleared.Icon != nil {
		t.Fatalf("cleared=%+v", cleared)
	}

	rec = request(h, http.MethodDelete, fmt.Sprintf("/api/v1/location-icons/%d?version=%d", created.ID, created.Version), "", webOrigin, testRemote, cookie)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = request(h, http.MethodPost, "/api/v1/location-icons", `{"name":"无会话","svg":`+jsonString(testSVG)+`}`, webOrigin, testRemote, "")
	assertCode(t, rec, http.StatusUnauthorized, "unauthenticated")
	rec = request(h, http.MethodPost, "/api/v1/location-icons", `{"name":"坏源","svg":`+jsonString(testSVG)+`}`, "http://evil.example", testRemote, cookie)
	assertCode(t, rec, http.StatusForbidden, "origin_rejected")
}

func TestLocationIconLimit(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)
	svg := jsonString(testSVG)
	for i := 0; i < 40; i++ {
		rec := request(h, http.MethodPost, "/api/v1/location-icons", fmt.Sprintf(`{"name":"图%02d","svg":%s}`, i+1, svg), webOrigin, testRemote, cookie)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create %d status=%d body=%s", i+1, rec.Code, rec.Body.String())
		}
	}
	rec := request(h, http.MethodPost, "/api/v1/location-icons", `{"name":"满","svg":`+svg+`}`, webOrigin, testRemote, cookie)
	assertInvalidFields(t, rec, map[string]string{"svg": "图标已满"})
}

func jsonString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func decodeIcon(t *testing.T, rec *httptest.ResponseRecorder) iconBody {
	t.Helper()
	var icon iconBody
	if err := json.Unmarshal(rec.Body.Bytes(), &icon); err != nil {
		t.Fatalf("body=%s err=%v", rec.Body.String(), err)
	}
	return icon
}

func loadEmbedded() ([]migrate.File, error) {
	var files []migrate.File
	err := fs.WalkDir(migrations.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		version, err := migrate.ParseVersion(path)
		if err != nil {
			return err
		}
		body, err := fs.ReadFile(migrations.FS, path)
		if err != nil {
			return err
		}
		files = append(files, migrate.File{Version: version, Name: path, SQL: string(body)})
		return nil
	})
	return files, err
}

type locBody struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	Code         *string `json:"code"`
	ParentID     *int64  `json:"parent_id"`
	Icon         *string `json:"icon"`
	CustomIconID *int64  `json:"custom_icon_id"`
	Version      int64   `json:"version"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
	Path         []struct {
		ID           int64   `json:"id"`
		Name         string  `json:"name"`
		Type         string  `json:"type"`
		Code         *string `json:"code"`
		Icon         *string `json:"icon"`
		CustomIconID *int64  `json:"custom_icon_id"`
	} `json:"path"`
	DirectItemCount int `json:"direct_item_count"`
}

type pageBody struct {
	Data   []locBody `json:"data"`
	Total  int       `json:"total"`
	Limit  int       `json:"limit"`
	Offset int       `json:"offset"`
}

func login(t *testing.T, h http.Handler) string {
	t.Helper()
	rec := request(h, http.MethodPost, "/api/v1/session", `{"username":"ada","password":"correct-horse"}`, webOrigin, "203.0.113.5:1", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "youchu_session" || cookies[0].Value == "" {
		t.Fatalf("cookies=%v", cookies)
	}
	return cookies[0].Value
}

func locationCount(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM locations`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func assertCount(t *testing.T, db *sql.DB, want int) {
	t.Helper()
	if n := locationCount(t, db); n != want {
		t.Fatalf("locations=%d want %d", n, want)
	}
}

func assertError(t *testing.T, rec *httptest.ResponseRecorder, status int, code, message string) {
	t.Helper()
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("status=%d body=%s err=%v", rec.Code, rec.Body.String(), err)
	}
	if rec.Code != status || body["code"] != code || body["message"] != message {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func assertInvalidFields(t *testing.T, rec *httptest.ResponseRecorder, fields map[string]string) {
	t.Helper()
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Code    string            `json:"code"`
		Message string            `json:"message"`
		Fields  map[string]string `json:"fields"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body=%s err=%v", rec.Body.String(), err)
	}
	if body.Code != "invalid_fields" || body.Message != "有字段不符合要求" {
		t.Fatalf("body=%s", rec.Body.String())
	}
	if len(body.Fields) != len(fields) {
		t.Fatalf("fields=%v want %v", body.Fields, fields)
	}
	for k, want := range fields {
		if body.Fields[k] != want {
			t.Fatalf("fields=%v want %v", body.Fields, fields)
		}
	}
}

func assertNullJSON(t *testing.T, body, key string) {
	t.Helper()
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &obj); err != nil {
		t.Fatalf("body=%s err=%v", body, err)
	}
	raw, ok := obj[key]
	if !ok || string(raw) != "null" {
		t.Fatalf("%s=%s body=%s", key, raw, body)
	}
}

func postLoc(h http.Handler, cookie, path, body string) *httptest.ResponseRecorder {
	return request(h, http.MethodPost, path, body, webOrigin, testRemote, cookie)
}

func getLoc(h http.Handler, cookie, path string) *httptest.ResponseRecorder {
	return request(h, http.MethodGet, path, "", "", testRemote, cookie)
}

func decodeLoc(t *testing.T, rec *httptest.ResponseRecorder) locBody {
	t.Helper()
	var loc locBody
	if err := json.Unmarshal(rec.Body.Bytes(), &loc); err != nil {
		t.Fatalf("body=%s err=%v", rec.Body.String(), err)
	}
	if loc.Path == nil {
		t.Fatalf("path is null: %s", rec.Body.String())
	}
	return loc
}

func assertCreated(t *testing.T, rec *httptest.ResponseRecorder) locBody {
	t.Helper()
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	loc := decodeLoc(t, rec)
	if loc.ID < 1 || loc.Version != 1 || loc.CreatedAt != frozenAt || loc.UpdatedAt != frozenAt {
		t.Fatalf("location=%+v body=%s", loc, rec.Body.String())
	}
	if len(loc.Path) == 0 || loc.Path[len(loc.Path)-1].ID != loc.ID {
		t.Fatalf("path=%+v body=%s", loc.Path, rec.Body.String())
	}
	return loc
}

func mustCreate(t *testing.T, h http.Handler, cookie, body string) locBody {
	t.Helper()
	return assertCreated(t, postLoc(h, cookie, "/api/v1/locations", body))
}

func decodePage(t *testing.T, rec *httptest.ResponseRecorder) pageBody {
	t.Helper()
	if strings.Contains(rec.Body.String(), `"data":null`) {
		t.Fatalf("data is null: %s", rec.Body.String())
	}
	var page pageBody
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("body=%s err=%v", rec.Body.String(), err)
	}
	if page.Data == nil {
		t.Fatalf("data is null: %s", rec.Body.String())
	}
	return page
}

func mustPage(t *testing.T, rec *httptest.ResponseRecorder) pageBody {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	return decodePage(t, rec)
}

func assertNames(t *testing.T, page pageBody, want ...string) {
	t.Helper()
	got := map[string]bool{}
	for _, loc := range page.Data {
		got[loc.Name] = true
	}
	if page.Total != len(want) || len(page.Data) != len(want) {
		t.Fatalf("total=%d data=%v want %v", page.Total, got, want)
	}
	for _, name := range want {
		if !got[name] {
			t.Fatalf("missing %s in %v", name, got)
		}
	}
}

func pageHasName(page pageBody, name string) bool {
	for _, loc := range page.Data {
		if loc.Name == name {
			return true
		}
	}
	return false
}

func TestCreateLocation(t *testing.T) {
	db, h, _ := testHandler(t, false)
	cookie := login(t, h)
	assertCount(t, db, 0)

	rec := postLoc(h, cookie, "/api/v1/locations", `{"name":"厨房","type":"area"}`)
	kitchen := assertCreated(t, rec)
	assertNullJSON(t, rec.Body.String(), "code")
	assertNullJSON(t, rec.Body.String(), "parent_id")
	if kitchen.Name != "厨房" || kitchen.Type != "area" || kitchen.Code != nil || kitchen.ParentID != nil {
		t.Fatalf("kitchen=%+v", kitchen)
	}
	if len(kitchen.Path) != 1 || kitchen.Path[0].ID != kitchen.ID || kitchen.Path[0].Name != "厨房" || kitchen.Path[0].Type != "area" || kitchen.Path[0].Code != nil {
		t.Fatalf("path=%+v", kitchen.Path)
	}
	assertCount(t, db, 1)

	rec = postLoc(h, cookie, "/api/v1/locations", `{"name":"带编号","type":"area","code":"001"}`)
	assertInvalidFields(t, rec, map[string]string{"code": "区域不使用编号"})
	assertCount(t, db, 1)

	rec = postLoc(h, cookie, "/api/v1/locations", `{"name":"无父级","type":"fixed","code":"008"}`)
	assertInvalidFields(t, rec, map[string]string{"parent_id": "请选择父级"})
	assertCount(t, db, 1)

	rec = postLoc(h, cookie, "/api/v1/locations", fmt.Sprintf(`{"name":"错放盒","type":"movable","code":"014","parent_id":%d}`, kitchen.ID))
	assertError(t, rec, http.StatusBadRequest, "invalid_parent", "不能放在这个父级下")
	assertCount(t, db, 1)

	box := mustCreate(t, h, cookie, `{"name":"工具箱","type":"movable","code":"099"}`)
	if box.Type != "movable" || box.ParentID != nil || box.Code == nil || *box.Code != "099" {
		t.Fatalf("box=%+v", box)
	}
	assertCount(t, db, 2)

	rec = postLoc(h, cookie, "/api/v1/locations", fmt.Sprintf(`{"name":"错放区","type":"area","parent_id":%d}`, box.ID))
	assertError(t, rec, http.StatusBadRequest, "invalid_parent", "不能放在这个父级下")
	assertCount(t, db, 2)

	counter := mustCreate(t, h, cookie, fmt.Sprintf(`{"name":"操作台","type":"area","parent_id":%d}`, kitchen.ID))
	if counter.Type != "area" || counter.Code != nil || counter.ParentID == nil || *counter.ParentID != kitchen.ID {
		t.Fatalf("counter=%+v", counter)
	}
	drawer := mustCreate(t, h, cookie, fmt.Sprintf(`{"name":"抽屉","type":"fixed","code":"001","parent_id":%d}`, kitchen.ID))
	if drawer.Type != "fixed" || drawer.Code == nil || *drawer.Code != "001" || drawer.ParentID == nil || *drawer.ParentID != kitchen.ID {
		t.Fatalf("drawer=%+v", drawer)
	}
	mov := mustCreate(t, h, cookie, fmt.Sprintf(`{"name":"传感器盒","type":"movable","code":"014","parent_id":%d}`, drawer.ID))
	if mov.Type != "movable" || mov.Code == nil || *mov.Code != "014" || mov.ParentID == nil || *mov.ParentID != drawer.ID {
		t.Fatalf("movable=%+v", mov)
	}
	assertCount(t, db, 5)

	rec = postLoc(h, cookie, "/api/v1/locations", fmt.Sprintf(`{"name":"重复","type":"fixed","code":"001","parent_id":%d}`, kitchen.ID))
	assertError(t, rec, http.StatusConflict, "code_taken", "编号已被使用")
	assertCount(t, db, 5)

	narrow := mustCreate(t, h, cookie, fmt.Sprintf(`{"name":"窄格","type":"fixed","code":"01","parent_id":%d}`, kitchen.ID))
	if narrow.Code == nil || *narrow.Code != "01" {
		t.Fatalf("narrow=%+v", narrow)
	}
	assertCount(t, db, 6)

	rec = getLoc(h, cookie, fmt.Sprintf("/api/v1/locations/%d", drawer.ID))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	gotDrawer := decodeLoc(t, rec)
	if gotDrawer.Code == nil || *gotDrawer.Code != "001" || !strings.Contains(rec.Body.String(), `"code":"001"`) {
		t.Fatalf("drawer get=%s", rec.Body.String())
	}

	blank := mustCreate(t, h, cookie, `{"name":"空白码","type":"area","code":"   "}`)
	if blank.Code != nil || blank.ParentID != nil {
		t.Fatalf("blank=%+v", blank)
	}
	nullCode := mustCreate(t, h, cookie, `{"name":"空码","type":"area","code":null}`)
	if nullCode.Code != nil {
		t.Fatalf("null code=%+v", nullCode)
	}
	assertCount(t, db, 8)

	rec = getLoc(h, cookie, fmt.Sprintf("/api/v1/locations/%d", kitchen.ID))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	gotKitchen := decodeLoc(t, rec)
	assertNullJSON(t, rec.Body.String(), "code")
	assertNullJSON(t, rec.Body.String(), "parent_id")
	if gotKitchen.ID != kitchen.ID || gotKitchen.Name != "厨房" || gotKitchen.Type != "area" || gotKitchen.Version != 1 || gotKitchen.CreatedAt != frozenAt || gotKitchen.UpdatedAt != frozenAt {
		t.Fatalf("get kitchen=%+v", gotKitchen)
	}
	if len(gotKitchen.Path) != 1 || gotKitchen.Path[0].ID != kitchen.ID || gotKitchen.Path[0].Name != "厨房" || gotKitchen.Path[0].Type != "area" || gotKitchen.Path[0].Code != nil {
		t.Fatalf("get path=%+v", gotKitchen.Path)
	}

	roots := mustPage(t, getLoc(h, cookie, "/api/v1/locations"))
	assertNames(t, roots, "厨房", "工具箱", "空白码", "空码")
	if roots.Limit != 30 || roots.Offset != 0 {
		t.Fatalf("roots page=%+v", roots)
	}
	for _, loc := range roots.Data {
		if loc.ParentID != nil || len(loc.Path) != 1 || loc.Path[0].ID != loc.ID {
			t.Fatalf("root=%+v", loc)
		}
	}
	children := mustPage(t, getLoc(h, cookie, fmt.Sprintf("/api/v1/locations?parent=%d", kitchen.ID)))
	assertNames(t, children, "操作台", "抽屉", "窄格")
	for _, loc := range children.Data {
		if loc.ParentID == nil || *loc.ParentID != kitchen.ID || len(loc.Path) != 2 || loc.Path[0].ID != kitchen.ID || loc.Path[1].ID != loc.ID {
			t.Fatalf("child=%+v", loc)
		}
	}

	for _, id := range []string{"0", "01", "abc"} {
		rec = getLoc(h, cookie, "/api/v1/locations/"+id)
		assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	}
	rec = getLoc(h, cookie, "/api/v1/locations/999999")
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")

	rec = getLoc(h, cookie, fmt.Sprintf("/api/v1/locations/%d", mov.ID))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	gotMov := decodeLoc(t, rec)
	if len(gotMov.Path) != 3 || gotMov.Path[0].ID != kitchen.ID || gotMov.Path[1].ID != drawer.ID || gotMov.Path[2].ID != mov.ID || gotMov.Path[2].ID != gotMov.ID {
		t.Fatalf("path=%+v", gotMov.Path)
	}
	if gotMov.Path[0].Name != "厨房" || gotMov.Path[0].Type != "area" || gotMov.Path[0].Code != nil {
		t.Fatalf("root=%+v", gotMov.Path[0])
	}
	if gotMov.Path[1].Type != "fixed" || gotMov.Path[1].Code == nil || *gotMov.Path[1].Code != "001" {
		t.Fatalf("middle=%+v", gotMov.Path[1])
	}
	if gotMov.Path[2].Type != "movable" || gotMov.Path[2].Code == nil || *gotMov.Path[2].Code != "014" {
		t.Fatalf("last=%+v", gotMov.Path[2])
	}

	rec = postLoc(h, cookie, "/api/v1/locations?parent=1", `{"name":"查询参数","type":"area"}`)
	assertInvalidFields(t, rec, map[string]string{"parent": "不支持的参数"})
	assertCount(t, db, 8)

	rec = postLoc(h, cookie, "/api/v1/locations", `{"name":"零父级","type":"fixed","code":"050","parent_id":0}`)
	if strings.Contains(rec.Body.String(), "请选择父级") {
		t.Fatalf("parent 0 treated as missing parent: %s", rec.Body.String())
	}
	assertError(t, rec, http.StatusBadRequest, "invalid_parent", "不能放在这个父级下")
	assertCount(t, db, 8)

	rec = postLoc(h, cookie, "/api/v1/locations", `{"type":"fixed"}`)
	assertInvalidFields(t, rec, map[string]string{
		"name":      "请填写名称",
		"code":      "请填写编号",
		"parent_id": "请选择父级",
	})
	assertCount(t, db, 8)

	rec = postLoc(h, cookie, "/api/v1/locations", `{"name":"厨房","type":"shelf","code":"`+strings.Repeat("a", 33)+`"}`)
	assertInvalidFields(t, rec, map[string]string{
		"type": "类型不正确",
		"code": "编号过长",
	})
	assertCount(t, db, 8)

	rec = postLoc(h, cookie, "/api/v1/locations", `{"code":"0 1"}`)
	assertInvalidFields(t, rec, map[string]string{
		"name": "请填写名称",
		"type": "类型不正确",
		"code": "编号不能包含空白",
	})
	assertCount(t, db, 8)
}

func TestLocationLists(t *testing.T) {
	db, h, _ := testHandler(t, false)
	cookie := login(t, h)

	rec := getLoc(h, cookie, "/api/v1/locations")
	empty := mustPage(t, rec)
	if empty.Total != 0 || len(empty.Data) != 0 || empty.Limit != 30 || empty.Offset != 0 {
		t.Fatalf("empty=%+v body=%s", empty, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"data":[]`) {
		t.Fatalf("empty data=%s", rec.Body.String())
	}

	rec = getLoc(h, cookie, "/api/v1/locations?offset=0")
	zero := mustPage(t, rec)
	if zero.Total != 0 || len(zero.Data) != 0 || zero.Offset != 0 || zero.Limit != 30 {
		t.Fatalf("offset 0=%+v", zero)
	}

	badQuery := []struct {
		raw    string
		fields map[string]string
	}{
		{"limit=0", map[string]string{"limit": "数量超出范围"}},
		{"limit=101", map[string]string{"limit": "数量超出范围"}},
		{"limit=01", map[string]string{"limit": "数量超出范围"}},
		{"limit=+1", map[string]string{"limit": "数量超出范围"}},
		{"limit=-1", map[string]string{"limit": "数量超出范围"}},
		{"offset=00", map[string]string{"offset": "起点不正确"}},
		{"offset=-1", map[string]string{"offset": "起点不正确"}},
		{"flat=2", map[string]string{"flat": "不支持的参数"}},
		{"eligible_parent_for=bag", map[string]string{"eligible_parent_for": "不支持的参数"}},
		{"nope=1", map[string]string{"nope": "不支持的参数"}},
		{"limit=1&limit=2", map[string]string{"limit": "不支持的参数"}},
		{"exclude=1", map[string]string{"exclude": "不支持的参数"}},
		{"parent=0", map[string]string{"parent": "参数不正确"}},
		{"flat=1&parent=1", map[string]string{"flat": "不支持的参数", "parent": "不支持的参数"}},
		{"limit=0&offset=-1", map[string]string{"limit": "数量超出范围", "offset": "起点不正确"}},
	}
	for _, tc := range badQuery {
		rec = getLoc(h, cookie, "/api/v1/locations?"+tc.raw)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status=%d body=%s", tc.raw, rec.Code, rec.Body.String())
		}
		assertInvalidFields(t, rec, tc.fields)
	}

	rec = getLoc(h, cookie, "/api/v1/locations?parent=999999")
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	rec = getLoc(h, cookie, "/api/v1/locations?eligible_parent_for=area&exclude=999999")
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")

	rec = getLoc(h, "", "/api/v1/locations")
	assertError(t, rec, http.StatusUnauthorized, "unauthenticated", "未登录")
	rec = postLoc(h, "", "/api/v1/locations", `{"name":"未登录","type":"area"}`)
	assertError(t, rec, http.StatusUnauthorized, "unauthenticated", "未登录")
	assertCount(t, db, 0)
	rec = request(h, http.MethodPost, "/api/v1/locations", `{"name":"坏来源","type":"area"}`, "http://evil.example", testRemote, "")
	assertError(t, rec, http.StatusForbidden, "origin_rejected", "来源不被接受")
	assertCount(t, db, 0)
	rec = request(h, http.MethodPost, "/api/v1/locations", `{"name":"坏来源","type":"area"}`, "http://evil.example", testRemote, cookie)
	assertError(t, rec, http.StatusForbidden, "origin_rejected", "来源不被接受")
	assertCount(t, db, 0)

	rec = postLoc(h, cookie, "/api/v1/locations/1", `{"name":"不该创建","type":"area"}`)
	if rec.Code == http.StatusMethodNotAllowed {
		t.Fatalf("POST id returned 405: %s", rec.Body.String())
	}
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	rec = request(h, http.MethodPatch, "/api/v1/locations/1", `{"name":"不该改"}`, webOrigin, testRemote, cookie)
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	rec = request(h, http.MethodDelete, "/api/v1/locations/1", "", webOrigin, testRemote, cookie)
	if rec.Code == http.StatusMethodNotAllowed {
		t.Fatalf("DELETE id returned 405: %s", rec.Body.String())
	}
	assertInvalidFields(t, rec, map[string]string{"version": "版本不正确"})
	assertCount(t, db, 0)

	parent := mustCreate(t, h, cookie, `{"name":"父区域","type":"area"}`)
	child := mustCreate(t, h, cookie, fmt.Sprintf(`{"name":"子区域","type":"area","parent_id":%d}`, parent.ID))
	grand := mustCreate(t, h, cookie, fmt.Sprintf(`{"name":"孙区域","type":"area","parent_id":%d}`, child.ID))
	sibling := mustCreate(t, h, cookie, `{"name":"旁区域","type":"area"}`)
	eligible := mustPage(t, getLoc(h, cookie, fmt.Sprintf("/api/v1/locations?eligible_parent_for=area&exclude=%d", parent.ID)))
	assertNames(t, eligible, "旁区域")
	if eligible.Data[0].ID != sibling.ID {
		t.Fatalf("sibling=%+v", eligible.Data[0])
	}
	for _, loc := range eligible.Data {
		if loc.ID == parent.ID || loc.ID == child.ID || loc.ID == grand.ID {
			t.Fatalf("excluded id returned: %+v", loc)
		}
	}

	db, h, _ = testHandler(t, false)
	cookie = login(t, h)
	for i := 1; i <= 101; i++ {
		name := fmt.Sprintf("区域-%03d", i)
		body := fmt.Sprintf(`{"name":%q,"type":"area"}`, name)
		rec = postLoc(h, cookie, "/api/v1/locations", body)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create %s status=%d body=%s", name, rec.Code, rec.Body.String())
		}
	}
	roots := mustPage(t, getLoc(h, cookie, "/api/v1/locations?offset=100"))
	if roots.Total != 101 || roots.Limit != 30 || roots.Offset != 100 || len(roots.Data) != 1 || !pageHasName(roots, "区域-101") {
		t.Fatalf("offset 100=%+v names=%v", roots, roots.Data)
	}
	flat := mustPage(t, getLoc(h, cookie, "/api/v1/locations?flat=1&limit=100&offset=100"))
	if flat.Total != 101 || flat.Limit != 100 || flat.Offset != 100 || len(flat.Data) != 1 || !pageHasName(flat, "区域-101") {
		t.Fatalf("flat=%+v", flat.Data)
	}
	areas := mustPage(t, getLoc(h, cookie, "/api/v1/locations?eligible_parent_for=area&limit=100&offset=100"))
	if areas.Total != 101 || areas.Limit != 100 || areas.Offset != 100 || len(areas.Data) != 1 || !pageHasName(areas, "区域-101") {
		t.Fatalf("eligible=%+v", areas.Data)
	}
	past := mustPage(t, getLoc(h, cookie, "/api/v1/locations?offset=101"))
	if past.Total != 101 || len(past.Data) != 0 || past.Offset != 101 || past.Limit != 30 {
		t.Fatalf("past=%+v", past)
	}
}

func patchLoc(h http.Handler, cookie, path, body string) *httptest.ResponseRecorder {
	return request(h, http.MethodPatch, path, body, webOrigin, testRemote, cookie)
}

func deleteLoc(h http.Handler, cookie, path string) *httptest.ResponseRecorder {
	return request(h, http.MethodDelete, path, "", webOrigin, testRemote, cookie)
}

func locURL(id int64) string {
	return fmt.Sprintf("/api/v1/locations/%d", id)
}

func requireLoc(t *testing.T, h http.Handler, cookie string, id int64) locBody {
	t.Helper()
	rec := getLoc(h, cookie, locURL(id))
	if rec.Code != http.StatusOK {
		t.Fatalf("get %d status=%d body=%s", id, rec.Code, rec.Body.String())
	}
	return decodeLoc(t, rec)
}

func assertNoFieldsKey(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if strings.Contains(rec.Body.String(), `"fields"`) {
		t.Fatalf("unexpected fields: %s", rec.Body.String())
	}
}

func assertParent(t *testing.T, loc locBody, parent int64) {
	t.Helper()
	if loc.ParentID == nil || *loc.ParentID != parent {
		t.Fatalf("parent=%v want %d loc=%+v", loc.ParentID, parent, loc)
	}
}

func assertPatched(t *testing.T, rec *httptest.ResponseRecorder) locBody {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	loc := decodeLoc(t, rec)
	if len(loc.Path) == 0 || loc.Path[len(loc.Path)-1].ID != loc.ID {
		t.Fatalf("path=%+v body=%s", loc.Path, rec.Body.String())
	}
	return loc
}

func TestMoveLocation(t *testing.T) {
	db, h, _ := testHandler(t, false)
	cookie := login(t, h)
	living := mustCreate(t, h, cookie, `{"name":"客厅","type":"area"}`)
	cabinet := mustCreate(t, h, cookie, fmt.Sprintf(`{"name":"储物柜","type":"fixed","code":"002","parent_id":%d}`, living.ID))
	toy := mustCreate(t, h, cookie, fmt.Sprintf(`{"name":"玩具盒","type":"movable","code":"014","parent_id":%d}`, cabinet.ID))

	rec := patchLoc(h, cookie, locURL(living.ID), fmt.Sprintf(`{"version":1,"parent_id":%d}`, toy.ID))
	assertError(t, rec, http.StatusConflict, "location_cycle", "不能移到自己的下级")
	got := requireLoc(t, h, cookie, living.ID)
	if got.ParentID != nil || got.Version != 1 || got.Name != "客厅" || got.Type != "area" {
		t.Fatalf("area after cycle=%+v", got)
	}

	rec = patchLoc(h, cookie, locURL(toy.ID), fmt.Sprintf(`{"version":1,"parent_id":%d}`, toy.ID))
	assertError(t, rec, http.StatusConflict, "location_cycle", "不能移到自己的下级")
	assertNoFieldsKey(t, rec)
	got = requireLoc(t, h, cookie, toy.ID)
	assertParent(t, got, cabinet.ID)
	if got.Version != 1 || got.Code == nil || *got.Code != "014" {
		t.Fatalf("self cycle=%+v", got)
	}

	rec = patchLoc(h, cookie, locURL(living.ID), `{"version":1}`)
	assertInvalidFields(t, rec, map[string]string{"request": "没有要修改的内容"})
	if requireLoc(t, h, cookie, living.ID).Version != 1 {
		t.Fatal("version changed")
	}

	rec = patchLoc(h, cookie, locURL(living.ID), `{"version":1,"type":"area"}`)
	assertInvalidFields(t, rec, map[string]string{"type": "类型不能修改", "request": "没有要修改的内容"})
	got = requireLoc(t, h, cookie, living.ID)
	if got.Type != "area" || got.Version != 1 || got.Name != "客厅" {
		t.Fatalf("type patch=%+v", got)
	}

	rec = patchLoc(h, cookie, locURL(living.ID), `{"version":1,"type":"area","name":"新客厅"}`)
	assertInvalidFields(t, rec, map[string]string{"type": "类型不能修改"})
	if requireLoc(t, h, cookie, living.ID).Name != "客厅" {
		t.Fatal("type patch wrote name")
	}

	rec = patchLoc(h, cookie, locURL(living.ID), `{"version":1,"type":"area","name":""}`)
	assertInvalidFields(t, rec, map[string]string{"type": "类型不能修改", "name": "请填写名称"})

	rec = patchLoc(h, cookie, locURL(cabinet.ID), `{"version":1,"code":null}`)
	assertInvalidFields(t, rec, map[string]string{"code": "编号不能清空"})
	got = requireLoc(t, h, cookie, cabinet.ID)
	if got.Version != 1 || got.Code == nil || *got.Code != "002" {
		t.Fatalf("fixed code=%+v", got)
	}

	rec = patchLoc(h, cookie, locURL(toy.ID), `{"version":1,"code":"   "}`)
	assertInvalidFields(t, rec, map[string]string{"code": "编号不能清空"})
	got = requireLoc(t, h, cookie, toy.ID)
	if got.Version != 1 || got.Code == nil || *got.Code != "014" {
		t.Fatalf("movable code=%+v", got)
	}

	rec = patchLoc(h, cookie, locURL(living.ID), `{"version":1,"code":"009"}`)
	assertInvalidFields(t, rec, map[string]string{"code": "区域不使用编号"})
	got = requireLoc(t, h, cookie, living.ID)
	if got.Version != 1 || got.Code != nil {
		t.Fatalf("area code=%+v", got)
	}

	rec = patchLoc(h, cookie, locURL(cabinet.ID), `{"version":1,"parent_id":null}`)
	assertInvalidFields(t, rec, map[string]string{"parent_id": "请选择父级"})
	got = requireLoc(t, h, cookie, cabinet.ID)
	assertParent(t, got, living.ID)
	if got.Version != 1 {
		t.Fatal("fixed parent cleared")
	}

	rec = patchLoc(h, cookie, locURL(cabinet.ID), `{"version":1,"code":null,"parent_id":null}`)
	assertInvalidFields(t, rec, map[string]string{"code": "编号不能清空", "parent_id": "请选择父级"})
	got = requireLoc(t, h, cookie, cabinet.ID)
	assertParent(t, got, living.ID)
	if got.Version != 1 || got.Code == nil || *got.Code != "002" {
		t.Fatalf("combined=%+v", got)
	}

	inner := mustCreate(t, h, cookie, fmt.Sprintf(`{"name":"内间","type":"area","parent_id":%d}`, living.ID))
	rec = patchLoc(h, cookie, locURL(inner.ID), `{"version":1,"code":"009","parent_id":null}`)
	assertInvalidFields(t, rec, map[string]string{"code": "区域不使用编号"})
	got = requireLoc(t, h, cookie, inner.ID)
	assertParent(t, got, living.ID)
	if got.Version != 1 {
		t.Fatal("illegal area code wrote parent")
	}

	rec = patchLoc(h, cookie, locURL(inner.ID), `{"version":1,"parent_id":null}`)
	moved := assertPatched(t, rec)
	if moved.ID != inner.ID || moved.Version != 2 || moved.ParentID != nil || moved.CreatedAt != frozenAt || moved.Name != "内间" {
		t.Fatalf("clear parent=%+v", moved)
	}
	if len(moved.Path) != 1 || moved.Path[0].ID != inner.ID {
		t.Fatalf("path=%+v", moved.Path)
	}
	assertNullJSON(t, rec.Body.String(), "parent_id")

	blankArea := mustCreate(t, h, cookie, `{"name":"空白区","type":"area"}`)
	rec = patchLoc(h, cookie, locURL(blankArea.ID), `{"version":1,"code":"   "}`)
	cleared := assertPatched(t, rec)
	if cleared.Version != 2 || cleared.Code != nil || cleared.CreatedAt != frozenAt {
		t.Fatalf("blank area code=%+v", cleared)
	}
	assertNullJSON(t, rec.Body.String(), "code")

	rec = patchLoc(h, cookie, locURL(toy.ID), `{"version":1,"parent_id":0}`)
	if strings.Contains(rec.Body.String(), "请选择父级") || strings.Contains(rec.Body.String(), `"fields"`) {
		t.Fatalf("parent 0 was a field error: %s", rec.Body.String())
	}
	assertError(t, rec, http.StatusBadRequest, "invalid_parent", "不能放在这个父级下")
	got = requireLoc(t, h, cookie, toy.ID)
	assertParent(t, got, cabinet.ID)
	if got.Version != 1 {
		t.Fatal("parent 0 wrote")
	}

	rec = patchLoc(h, cookie, locURL(toy.ID), `{"version":1,"parent_id":999999}`)
	assertError(t, rec, http.StatusBadRequest, "invalid_parent", "不能放在这个父级下")
	assertParent(t, requireLoc(t, h, cookie, toy.ID), cabinet.ID)

	yard := mustCreate(t, h, cookie, `{"name":"阳台","type":"area"}`)
	rec = patchLoc(h, cookie, locURL(yard.ID), fmt.Sprintf(`{"version":1,"parent_id":%d}`, cabinet.ID))
	assertError(t, rec, http.StatusBadRequest, "invalid_parent", "不能放在这个父级下")
	if requireLoc(t, h, cookie, yard.ID).ParentID != nil {
		t.Fatal("area accepted fixed parent")
	}

	rec = patchLoc(h, cookie, locURL(living.ID)+"?unused=1", `{"name":"客厅改","version":1}`)
	assertInvalidFields(t, rec, map[string]string{"unused": "不支持的参数"})
	got = requireLoc(t, h, cookie, living.ID)
	if got.Name != "客厅" || got.Version != 1 {
		t.Fatalf("query applied=%+v", got)
	}

	rec = request(h, http.MethodPatch, locURL(living.ID), `{`, webOrigin, testRemote, "")
	assertError(t, rec, http.StatusUnauthorized, "unauthenticated", "未登录")
	if requireLoc(t, h, cookie, living.ID).Name != "客厅" {
		t.Fatal("unauthenticated patch wrote")
	}
	rec = request(h, http.MethodPatch, locURL(living.ID), `{"name":"客厅改","version":1}`, "http://evil.example", testRemote, cookie)
	assertError(t, rec, http.StatusForbidden, "origin_rejected", "来源不被接受")
	if requireLoc(t, h, cookie, living.ID).Name != "客厅" {
		t.Fatal("bad origin wrote")
	}
	rec = request(h, http.MethodPatch, locURL(living.ID), `{`, "http://evil.example", testRemote, "")
	assertError(t, rec, http.StatusForbidden, "origin_rejected", "来源不被接受")
	rec = request(h, http.MethodPatch, locURL(living.ID), `{"name":"客厅改","version":1}`, "", testRemote, cookie)
	assertError(t, rec, http.StatusForbidden, "origin_rejected", "来源不被接受")
	if requireLoc(t, h, cookie, living.ID).Name != "客厅" {
		t.Fatal("missing origin wrote")
	}

	rec = patchLoc(h, cookie, "/api/v1/locations/999999", `{"name":"不存在","version":1}`)
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	rec = patchLoc(h, cookie, "/api/v1/locations/999999", `{"name":"","version":1}`)
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	assertNoFieldsKey(t, rec)
	rec = patchLoc(h, cookie, "/api/v1/locations/999999", `{`)
	assertError(t, rec, http.StatusBadRequest, "invalid_body", "请求格式不正确")
	assertNoFieldsKey(t, rec)

	rec = request(h, http.MethodPatch, "/api/v1/locations/abc", `{"name":"客厅","version":1}`, "http://evil.example", testRemote, "")
	assertError(t, rec, http.StatusForbidden, "origin_rejected", "来源不被接受")
	rec = request(h, http.MethodPut, locURL(living.ID), `{"name":"客厅改","version":1}`, webOrigin, testRemote, cookie)
	if rec.Code == http.StatusMethodNotAllowed {
		t.Fatalf("PUT returned 405: %s", rec.Body.String())
	}
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	if requireLoc(t, h, cookie, living.ID).Name != "客厅" {
		t.Fatal("PUT wrote")
	}

	rec = patchLoc(h, cookie, locURL(toy.ID), `{"name":"","version":0}`)
	assertInvalidFields(t, rec, map[string]string{"name": "请填写名称", "version": "版本不正确"})
	got = requireLoc(t, h, cookie, toy.ID)
	if got.Name != "玩具盒" || got.Version != 1 {
		t.Fatalf("version 0 wrote=%+v", got)
	}

	rec = patchLoc(h, cookie, locURL(toy.ID), `{"version":1,"parent_id":null}`)
	rooted := assertPatched(t, rec)
	if rooted.Version != 2 || rooted.ParentID != nil || rooted.Name != "玩具盒" || rooted.CreatedAt != frozenAt || rooted.UpdatedAt != frozenAt {
		t.Fatalf("movable root=%+v", rooted)
	}
	if rooted.Code == nil || *rooted.Code != "014" || len(rooted.Path) != 1 {
		t.Fatalf("movable root path=%+v", rooted)
	}
	rec = patchLoc(h, cookie, locURL(toy.ID), fmt.Sprintf(`{"version":2,"parent_id":%d}`, cabinet.ID))
	back := assertPatched(t, rec)
	assertParent(t, back, cabinet.ID)
	if back.Version != 3 || len(back.Path) != 3 || back.Path[0].ID != living.ID || back.Path[1].ID != cabinet.ID || back.Path[2].ID != toy.ID {
		t.Fatalf("moved back=%+v", back)
	}

	rec = patchLoc(h, cookie, locURL(cabinet.ID), `{"version":1,"code":"008"}`)
	coded := assertPatched(t, rec)
	assertParent(t, coded, living.ID)
	if coded.Version != 2 || coded.Code == nil || *coded.Code != "008" || !strings.Contains(rec.Body.String(), `"code":"008"`) {
		t.Fatalf("code change=%+v body=%s", coded, rec.Body.String())
	}
	narrow := mustCreate(t, h, cookie, fmt.Sprintf(`{"name":"窄盒","type":"movable","code":"08","parent_id":%d}`, cabinet.ID))
	rec = patchLoc(h, cookie, locURL(narrow.ID), `{"version":1,"code":"008"}`)
	assertError(t, rec, http.StatusConflict, "code_taken", "编号已被使用")
	got = requireLoc(t, h, cookie, narrow.ID)
	if got.Version != 1 || got.Code == nil || *got.Code != "08" {
		t.Fatalf("taken code wrote=%+v", got)
	}
	if locationCount(t, db) < 6 {
		t.Fatalf("locations=%d", locationCount(t, db))
	}
}

func TestDeleteLocation(t *testing.T) {
	db, h, _ := testHandler(t, false)
	cookie := login(t, h)
	living := mustCreate(t, h, cookie, `{"name":"客厅","type":"area"}`)
	cabinet := mustCreate(t, h, cookie, fmt.Sprintf(`{"name":"储物柜","type":"fixed","code":"002","parent_id":%d}`, living.ID))
	toy := mustCreate(t, h, cookie, fmt.Sprintf(`{"name":"玩具盒","type":"movable","code":"014","parent_id":%d}`, cabinet.ID))

	rec := deleteLoc(h, cookie, locURL(living.ID)+"?version=1")
	assertError(t, rec, http.StatusConflict, "location_in_use", "这个位置下面还有内容")
	got := requireLoc(t, h, cookie, living.ID)
	if got.Version != 1 || got.Name != "客厅" {
		t.Fatalf("parent deleted=%+v", got)
	}

	rec = patchLoc(h, cookie, locURL(living.ID), `{"name":"客厅西","version":1}`)
	renamed := assertPatched(t, rec)
	if renamed.Version != 2 || renamed.Name != "客厅西" {
		t.Fatalf("rename=%+v", renamed)
	}
	rec = deleteLoc(h, cookie, locURL(living.ID)+"?version=1")
	assertError(t, rec, http.StatusConflict, "version_conflict", "记录已被修改")
	if strings.Contains(rec.Body.String(), "location_in_use") {
		t.Fatalf("stale delete reported in use: %s", rec.Body.String())
	}
	got = requireLoc(t, h, cookie, living.ID)
	if got.Version != 2 || got.Name != "客厅西" {
		t.Fatalf("stale delete wrote=%+v", got)
	}

	yard := mustCreate(t, h, cookie, `{"name":"阳台","type":"area"}`)
	res, err := db.Exec(`INSERT INTO items (name, created_at, updated_at) VALUES ('传感器', ?, ?)`, frozenAt, frozenAt)
	if err != nil {
		t.Fatal(err)
	}
	itemID, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO item_locations (item_id, location_id) VALUES (?, ?)`, itemID, yard.ID); err != nil {
		t.Fatal(err)
	}
	rec = deleteLoc(h, cookie, locURL(yard.ID)+"?version=1")
	assertError(t, rec, http.StatusConflict, "location_in_use", "这个位置下面还有内容")
	got = requireLoc(t, h, cookie, yard.ID)
	if got.Name != "阳台" || got.Version != 1 {
		t.Fatalf("linked location=%+v", got)
	}
	var items, links int
	if err := db.QueryRow(`SELECT COUNT(*) FROM items`).Scan(&items); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM item_locations WHERE location_id = ?`, yard.ID).Scan(&links); err != nil {
		t.Fatal(err)
	}
	if items != 1 || links != 1 {
		t.Fatalf("items=%d links=%d", items, links)
	}

	rec = deleteLoc(h, cookie, locURL(toy.ID)+"?version=1")
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	rec = getLoc(h, cookie, locURL(toy.ID))
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	if requireLoc(t, h, cookie, cabinet.ID).Version != 1 {
		t.Fatal("parent changed")
	}

	var maxID int64
	empty := mustCreate(t, h, cookie, `{"name":"空箱","type":"movable","code":"021"}`)
	if err := db.QueryRow(`SELECT MAX(id) FROM locations`).Scan(&maxID); err != nil {
		t.Fatal(err)
	}
	if empty.ID != maxID {
		t.Fatalf("empty id=%d max=%d", empty.ID, maxID)
	}
	rec = deleteLoc(h, cookie, locURL(empty.ID)+"?version=1")
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	next := mustCreate(t, h, cookie, `{"name":"新箱","type":"movable","code":"022"}`)
	if next.ID == empty.ID || next.Version != 1 || next.Name != "新箱" {
		t.Fatalf("reused id=%d next=%+v", empty.ID, next)
	}
	rec = patchLoc(h, cookie, locURL(empty.ID), `{"name":"被改掉","version":1}`)
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	rec = deleteLoc(h, cookie, locURL(empty.ID)+"?version=1")
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	got = requireLoc(t, h, cookie, next.ID)
	if got.Name != "新箱" || got.Version != 1 {
		t.Fatalf("new location changed=%+v", got)
	}

	rec = deleteLoc(h, cookie, "/api/v1/locations/999999?version=abc")
	assertInvalidFields(t, rec, map[string]string{"version": "版本不正确"})
	rec = deleteLoc(h, cookie, "/api/v1/locations/999999?version=1")
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")

	rec = deleteLoc(h, cookie, locURL(living.ID)+"?version=2&extra=1")
	assertInvalidFields(t, rec, map[string]string{"extra": "不支持的参数"})
	got = requireLoc(t, h, cookie, living.ID)
	if got.Version != 2 || got.Name != "客厅西" {
		t.Fatalf("extra query deleted=%+v", got)
	}
	rec = deleteLoc(h, cookie, locURL(living.ID))
	assertInvalidFields(t, rec, map[string]string{"version": "版本不正确"})
	got = requireLoc(t, h, cookie, living.ID)
	if got.Version != 2 || got.Name != "客厅西" {
		t.Fatalf("missing version deleted=%+v", got)
	}
	rec = deleteLoc(h, cookie, locURL(living.ID)+"?version=01")
	assertInvalidFields(t, rec, map[string]string{"version": "版本不正确"})
	if requireLoc(t, h, cookie, living.ID).Version != 2 {
		t.Fatal("leading zero version deleted")
	}

	rec = request(h, http.MethodDelete, "/api/v1/locations/abc", "", "http://evil.example", testRemote, "")
	assertError(t, rec, http.StatusForbidden, "origin_rejected", "来源不被接受")
	if locationCount(t, db) < 4 {
		t.Fatalf("locations=%d", locationCount(t, db))
	}
}

func TestLocationVersion(t *testing.T) {
	db := migratedDB(t)
	if err := auth.Bootstrap(context.Background(), db, "ada", "correct-horse"); err != nil {
		t.Fatal(err)
	}
	dummy, err := auth.NewDummyHash()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	h := New(db, config.Config{
		PublicOrigin: webOrigin,
		SessionTTL:   time.Hour,
		LoginLimit:   10,
		CookieSecure: false,
	}, dummy, func() time.Time { return now })
	cookie := login(t, h)
	box := mustCreate(t, h, cookie, `{"name":"玩具盒","type":"movable","code":"014"}`)
	now = now.Add(time.Second)
	rec := patchLoc(h, cookie, locURL(box.ID), `{"name":"玩具盒","version":1}`)
	same := assertPatched(t, rec)
	later := "2026-09-29T12:00:01Z"
	if same.Version != 2 || same.CreatedAt != frozenAt || same.UpdatedAt != later || same.Name != "玩具盒" {
		t.Fatalf("same name=%+v", same)
	}
	if same.Code == nil || *same.Code != "014" || same.ParentID != nil {
		t.Fatalf("same name fields=%+v", same)
	}
	stored := requireLoc(t, h, cookie, box.ID)
	if stored.Version != 2 || stored.CreatedAt != frozenAt || stored.UpdatedAt != later || stored.Name != "玩具盒" {
		t.Fatalf("stored=%+v", stored)
	}

	db, h, _ = testHandler(t, false)
	cookie = login(t, h)
	living := mustCreate(t, h, cookie, `{"name":"客厅","type":"area"}`)
	cabinet := mustCreate(t, h, cookie, fmt.Sprintf(`{"name":"储物柜","type":"fixed","code":"002","parent_id":%d}`, living.ID))
	toy := mustCreate(t, h, cookie, fmt.Sprintf(`{"name":"玩具盒","type":"movable","code":"014","parent_id":%d}`, cabinet.ID))
	other := mustCreate(t, h, cookie, fmt.Sprintf(`{"name":"另一格","type":"fixed","code":"007","parent_id":%d}`, living.ID))

	rec = patchLoc(h, cookie, locURL(living.ID), `{"name":"起居室","version":1}`)
	renamed := assertPatched(t, rec)
	if renamed.Version != 2 || renamed.Name != "起居室" || renamed.CreatedAt != frozenAt || renamed.UpdatedAt != frozenAt || renamed.ParentID != nil {
		t.Fatalf("rename=%+v", renamed)
	}

	rec = patchLoc(h, cookie, locURL(living.ID), fmt.Sprintf(`{"version":1,"parent_id":%d}`, toy.ID))
	assertError(t, rec, http.StatusConflict, "version_conflict", "记录已被修改")
	if strings.Contains(rec.Body.String(), "location_cycle") {
		t.Fatalf("stale cycle=%s", rec.Body.String())
	}
	got := requireLoc(t, h, cookie, living.ID)
	if got.ParentID != nil || got.Version != 2 || got.Name != "起居室" {
		t.Fatalf("stale cycle wrote=%+v", got)
	}

	rec = patchLoc(h, cookie, locURL(living.ID), `{"name":"","version":1}`)
	assertInvalidFields(t, rec, map[string]string{"name": "请填写名称"})
	got = requireLoc(t, h, cookie, living.ID)
	if got.Name != "起居室" || got.Version != 2 {
		t.Fatalf("empty stale name=%+v", got)
	}

	rec = patchLoc(h, cookie, locURL(living.ID), `{"name":"","version":0}`)
	assertInvalidFields(t, rec, map[string]string{"name": "请填写名称", "version": "版本不正确"})
	got = requireLoc(t, h, cookie, living.ID)
	if got.Name != "起居室" || got.Version != 2 {
		t.Fatalf("version 0 wrote=%+v", got)
	}

	rec = patchLoc(h, cookie, locURL(living.ID), `{"name":"别的","version":0}`)
	assertInvalidFields(t, rec, map[string]string{"version": "版本不正确"})
	if requireLoc(t, h, cookie, living.ID).Name != "起居室" {
		t.Fatal("version 0 applied name")
	}

	rec = patchLoc(h, cookie, locURL(living.ID), `{"name":"改名"}`)
	assertInvalidFields(t, rec, map[string]string{"version": "版本不正确"})
	if requireLoc(t, h, cookie, living.ID).Name != "起居室" {
		t.Fatal("missing version applied name")
	}

	rec = patchLoc(h, cookie, locURL(toy.ID), `{"version":9,"parent_id":0}`)
	assertError(t, rec, http.StatusConflict, "version_conflict", "记录已被修改")
	assertNoFieldsKey(t, rec)
	got = requireLoc(t, h, cookie, toy.ID)
	assertParent(t, got, cabinet.ID)
	if got.Version != 1 {
		t.Fatalf("stale parent 0=%+v", got)
	}

	rec = patchLoc(h, cookie, locURL(cabinet.ID), `{"version":4,"code":"007"}`)
	assertError(t, rec, http.StatusConflict, "version_conflict", "记录已被修改")
	got = requireLoc(t, h, cookie, cabinet.ID)
	if got.Version != 1 || got.Code == nil || *got.Code != "002" {
		t.Fatalf("stale code=%+v", got)
	}
	got = requireLoc(t, h, cookie, other.ID)
	if got.Code == nil || *got.Code != "007" || got.Version != 1 {
		t.Fatalf("other=%+v", got)
	}
}

type itemPathNode struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	Code         *string `json:"code"`
	Icon         *string `json:"icon"`
	CustomIconID *int64  `json:"custom_icon_id"`
}

type itemLinkBody struct {
	LocationID int64          `json:"location_id"`
	Note       *string        `json:"note"`
	Path       []itemPathNode `json:"path"`
}

type itemCatBody struct {
	CategoryID int64         `json:"category_id"`
	Source     string        `json:"source"`
	Path       []catPathNode `json:"path"`
}

type photoBody struct {
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

type coverPhotoBody struct {
	ID int64 `json:"id"`
}

type returnTaskBody struct {
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
	CoverPhoto      *coverPhotoBody `json:"cover_photo"`
}

type itemBody struct {
	ID           int64            `json:"id"`
	Name         string           `json:"name"`
	Alias        *string          `json:"alias"`
	Model        *string          `json:"model"`
	Spec         *string          `json:"spec"`
	QuantityNote *string          `json:"quantity_note"`
	Note         *string          `json:"note"`
	Version      int64            `json:"version"`
	CreatedAt    string           `json:"created_at"`
	UpdatedAt    string           `json:"updated_at"`
	DeletedAt    *string          `json:"deleted_at"`
	Locations    []itemLinkBody   `json:"locations"`
	Categories   []itemCatBody    `json:"categories"`
	ReturnTasks  []returnTaskBody `json:"return_tasks"`
	Photos       []photoBody      `json:"photos"`
}

type itemPageBody struct {
	Data   []itemBody `json:"data"`
	Total  int        `json:"total"`
	Limit  int        `json:"limit"`
	Offset int        `json:"offset"`
}

func postItem(h http.Handler, cookie, path, body string) *httptest.ResponseRecorder {
	return request(h, http.MethodPost, path, body, webOrigin, testRemote, cookie)
}

func patchItem(h http.Handler, cookie, path, body string) *httptest.ResponseRecorder {
	return request(h, http.MethodPatch, path, body, webOrigin, testRemote, cookie)
}

func getItem(h http.Handler, cookie, path string) *httptest.ResponseRecorder {
	return request(h, http.MethodGet, path, "", "", testRemote, cookie)
}

func deleteItem(h http.Handler, cookie, path string) *httptest.ResponseRecorder {
	return request(h, http.MethodDelete, path, "", webOrigin, testRemote, cookie)
}

func itemURL(id int64) string {
	return fmt.Sprintf("/api/v1/items/%d", id)
}

func itemPhotosURL(id int64) string {
	return fmt.Sprintf("/api/v1/items/%d/photos", id)
}

func photoThumbURL(id int64) string {
	return fmt.Sprintf("/api/v1/photos/%d/thumbnail", id)
}

func photoOriginalURL(id int64) string {
	return fmt.Sprintf("/api/v1/photos/%d/original", id)
}

func photoURL(id int64) string {
	return fmt.Sprintf("/api/v1/photos/%d", id)
}

func photoFirstURL(id int64) string {
	return fmt.Sprintf("/api/v1/photos/%d/first", id)
}

func trashURL(id int64) string {
	return fmt.Sprintf("/api/v1/trash/%d", id)
}

func restoreURL(id int64) string {
	return fmt.Sprintf("/api/v1/trash/%d/restore", id)
}

func countQuery(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func decodeItem(t *testing.T, rec *httptest.ResponseRecorder) itemBody {
	t.Helper()
	if strings.Contains(rec.Body.String(), `"locations":null`) {
		t.Fatalf("locations is null: %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"categories":null`) {
		t.Fatalf("categories is null: %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"return_tasks":null`) {
		t.Fatalf("return_tasks is null: %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"photos":null`) {
		t.Fatalf("photos is null: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"photos":`) {
		t.Fatalf("photos key missing: %s", rec.Body.String())
	}
	var item itemBody
	if err := json.Unmarshal(rec.Body.Bytes(), &item); err != nil {
		t.Fatalf("body=%s err=%v", rec.Body.String(), err)
	}
	if item.Locations == nil {
		t.Fatalf("locations is null: %s", rec.Body.String())
	}
	if item.Categories == nil {
		t.Fatalf("categories is null: %s", rec.Body.String())
	}
	if item.ReturnTasks == nil {
		t.Fatalf("return_tasks is null: %s", rec.Body.String())
	}
	if item.Photos == nil {
		t.Fatalf("photos is null: %s", rec.Body.String())
	}
	for _, link := range item.Locations {
		if link.Path == nil || len(link.Path) == 0 || link.Path[len(link.Path)-1].ID != link.LocationID {
			t.Fatalf("path=%+v body=%s", link.Path, rec.Body.String())
		}
	}
	for _, link := range item.Categories {
		if link.Path == nil || len(link.Path) == 0 || link.Path[len(link.Path)-1].ID != link.CategoryID {
			t.Fatalf("path=%+v body=%s", link.Path, rec.Body.String())
		}
	}
	return item
}

func assertItemCreated(t *testing.T, rec *httptest.ResponseRecorder) itemBody {
	t.Helper()
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	item := decodeItem(t, rec)
	if item.ID < 1 || item.Version != 1 || item.CreatedAt != frozenAt || item.UpdatedAt != frozenAt {
		t.Fatalf("item=%+v body=%s", item, rec.Body.String())
	}
	return item
}

func mustItem(t *testing.T, h http.Handler, cookie, body string) itemBody {
	t.Helper()
	return assertItemCreated(t, postItem(h, cookie, "/api/v1/items", body))
}

func assertItemOK(t *testing.T, rec *httptest.ResponseRecorder) itemBody {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	return decodeItem(t, rec)
}

func requireItem(t *testing.T, h http.Handler, cookie string, id int64) itemBody {
	t.Helper()
	rec := getItem(h, cookie, itemURL(id))
	if rec.Code != http.StatusOK {
		t.Fatalf("get %d status=%d body=%s", id, rec.Code, rec.Body.String())
	}
	return decodeItem(t, rec)
}

func decodeItemPage(t *testing.T, rec *httptest.ResponseRecorder) itemPageBody {
	t.Helper()
	if strings.Contains(rec.Body.String(), `"data":null`) {
		t.Fatalf("data is null: %s", rec.Body.String())
	}
	var page itemPageBody
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("body=%s err=%v", rec.Body.String(), err)
	}
	if page.Data == nil {
		t.Fatalf("data is null: %s", rec.Body.String())
	}
	for _, item := range page.Data {
		if item.Locations == nil {
			t.Fatalf("locations is null: %s", rec.Body.String())
		}
		if item.ReturnTasks == nil {
			t.Fatalf("return_tasks is null: %s", rec.Body.String())
		}
		if item.Photos == nil {
			t.Fatalf("photos is null: %s", rec.Body.String())
		}
		for _, link := range item.Locations {
			if link.Path == nil || len(link.Path) == 0 || link.Path[len(link.Path)-1].ID != link.LocationID {
				t.Fatalf("path=%+v body=%s", link.Path, rec.Body.String())
			}
		}
	}
	return page
}

func requireTrash(t *testing.T, h http.Handler, cookie string, id int64) itemBody {
	t.Helper()
	rec := getItem(h, cookie, trashURL(id))
	if rec.Code != http.StatusOK {
		t.Fatalf("get trash %d status=%d body=%s", id, rec.Code, rec.Body.String())
	}
	item := decodeItem(t, rec)
	if item.DeletedAt == nil || *item.DeletedAt == "" {
		t.Fatalf("deleted_at empty: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"return_tasks":[]`) {
		t.Fatalf("return_tasks not empty array: %s", rec.Body.String())
	}
	return item
}

func pageHasItemID(page itemPageBody, id int64) bool {
	for _, item := range page.Data {
		if item.ID == id {
			return true
		}
	}
	return false
}

func itemDeletedAt(t *testing.T, db *sql.DB, id int64) string {
	t.Helper()
	var at sql.NullString
	if err := db.QueryRow(`SELECT deleted_at FROM items WHERE id = ?`, id).Scan(&at); err != nil {
		t.Fatal(err)
	}
	if !at.Valid || at.String == "" {
		t.Fatalf("deleted_at empty for %d", id)
	}
	return at.String
}

func mustItemPage(t *testing.T, rec *httptest.ResponseRecorder) itemPageBody {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	return decodeItemPage(t, rec)
}

func assertItemNames(t *testing.T, page itemPageBody, want ...string) {
	t.Helper()
	if page.Total != len(want) || len(page.Data) != len(want) {
		got := make([]string, len(page.Data))
		for i, item := range page.Data {
			got[i] = item.Name
		}
		t.Fatalf("names=%v total=%d want %v", got, page.Total, want)
	}
	for i, name := range want {
		if page.Data[i].Name != name {
			t.Fatalf("data[%d]=%s want %s", i, page.Data[i].Name, name)
		}
	}
}

func linkHasName(link itemLinkBody, name string) bool {
	for _, node := range link.Path {
		if node.Name == name {
			return true
		}
	}
	return false
}

func oversizedItemBody() string {
	return `{"name":"` + strings.Repeat("a", 32769) + `"}`
}

func TestItemCreate(t *testing.T) {
	db, h, _ := testHandler(t, false)
	cookie := login(t, h)
	if countQuery(t, db, `SELECT COUNT(*) FROM items`) != 0 {
		t.Fatal("items already present")
	}

	rec := postItem(h, cookie, "/api/v1/items", `{"name":"温湿度传感器"}`)
	first := assertItemCreated(t, rec)
	if first.Name != "温湿度传感器" || len(first.Locations) != 0 || len(first.Categories) != 0 || !strings.Contains(rec.Body.String(), `"locations":[]`) || !strings.Contains(rec.Body.String(), `"categories":[]`) {
		t.Fatalf("first=%+v body=%s", first, rec.Body.String())
	}
	for _, key := range []string{"alias", "model", "spec", "quantity_note", "note"} {
		assertNullJSON(t, rec.Body.String(), key)
	}
	if countQuery(t, db, `SELECT COUNT(*) FROM items`) != 1 {
		t.Fatal("item row missing")
	}

	rec = postItem(h, cookie, "/api/v1/items", `{"name":"温湿度传感器"}`)
	second := assertItemCreated(t, rec)
	if second.ID == first.ID || second.Name != "温湿度传感器" || len(second.Locations) != 0 {
		t.Fatalf("second=%+v first=%d", second, first.ID)
	}

	trimmed := mustItem(t, h, cookie, `{"name":"  温湿度传感器  ","alias":" 别名 ","model":"  ","spec":null,"quantity_note":"","note":"  \n  "}`)
	if trimmed.Name != "温湿度传感器" || trimmed.Alias == nil || *trimmed.Alias != "别名" || trimmed.Model != nil || trimmed.Spec != nil || trimmed.QuantityNote != nil || trimmed.Note != nil {
		t.Fatalf("trimmed=%+v", trimmed)
	}
	got := requireItem(t, h, cookie, trimmed.ID)
	if got.Alias == nil || *got.Alias != "别名" || got.Note != nil {
		t.Fatalf("stored trim=%+v", got)
	}

	emptyLinks := mustItem(t, h, cookie, `{"name":"空关联数组","locations":[]}`)
	if len(emptyLinks.Locations) != 0 {
		t.Fatalf("empty locations=%+v", emptyLinks.Locations)
	}
	emptyCats := mustItem(t, h, cookie, `{"name":"空分类数组","categories":[]}`)
	if len(emptyCats.Categories) != 0 {
		t.Fatalf("empty categories=%+v", emptyCats.Categories)
	}
	if countQuery(t, db, `SELECT COUNT(*) FROM item_categories`) != 0 {
		t.Fatal("item_categories rows present")
	}

	before := countQuery(t, db, `SELECT COUNT(*) FROM items`)
	rec = postItem(h, cookie, "/api/v1/items", `{"name":""}`)
	assertInvalidFields(t, rec, map[string]string{"name": "请填写名称"})
	rec = postItem(h, cookie, "/api/v1/items", `{}`)
	assertInvalidFields(t, rec, map[string]string{"name": "请填写名称"})
	rec = postItem(h, cookie, "/api/v1/items", `{"name":"空关联","locations":null}`)
	assertInvalidFields(t, rec, map[string]string{"locations": "位置格式不正确"})
	rec = postItem(h, cookie, "/api/v1/items", `{"name":"","locations":null}`)
	assertInvalidFields(t, rec, map[string]string{"name": "请填写名称", "locations": "位置格式不正确"})
	rec = postItem(h, cookie, "/api/v1/items", `{"name":"好","alias":"a\u0001b"}`)
	assertInvalidFields(t, rec, map[string]string{"alias": "别名不能包含控制字符"})
	rec = postItem(h, cookie, "/api/v1/items?flat=1", `{`)
	assertInvalidFields(t, rec, map[string]string{"flat": "不支持的参数"})
	if countQuery(t, db, `SELECT COUNT(*) FROM items`) != before {
		t.Fatal("rejected create wrote a row")
	}

	rec = request(h, http.MethodPut, "/api/v1/items", `{"name":"x"}`, webOrigin, testRemote, cookie)
	if rec.Code == http.StatusMethodNotAllowed {
		t.Fatalf("PUT collection returned 405: %s", rec.Body.String())
	}
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	rec = request(h, http.MethodDelete, "/api/v1/items", "", webOrigin, testRemote, cookie)
	if rec.Code == http.StatusMethodNotAllowed {
		t.Fatalf("DELETE collection returned 405: %s", rec.Body.String())
	}
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
}

func TestItemContainerMove(t *testing.T) {
	db, h, _ := testHandler(t, false)
	cookie := login(t, h)
	living := mustCreate(t, h, cookie, `{"name":"客厅","type":"area"}`)
	cabinet := mustCreate(t, h, cookie, fmt.Sprintf(`{"name":"储物柜","type":"fixed","code":"002","parent_id":%d}`, living.ID))
	other := mustCreate(t, h, cookie, fmt.Sprintf(`{"name":"另一柜","type":"fixed","code":"003","parent_id":%d}`, living.ID))
	toy := mustCreate(t, h, cookie, fmt.Sprintf(`{"name":"玩具盒","type":"movable","code":"014","parent_id":%d}`, cabinet.ID))

	first := mustItem(t, h, cookie, fmt.Sprintf(`{"name":"遥控车","locations":[{"location_id":%d}]}`, toy.ID))
	second := mustItem(t, h, cookie, fmt.Sprintf(`{"name":"电池","locations":[{"location_id":%d}]}`, toy.ID))
	for _, item := range []itemBody{first, second} {
		if len(item.Locations) != 1 || item.Locations[0].LocationID != toy.ID || !linkHasName(item.Locations[0], "储物柜") || linkHasName(item.Locations[0], "另一柜") {
			t.Fatalf("before move=%+v", item.Locations)
		}
	}

	rec := patchLoc(h, cookie, locURL(toy.ID), fmt.Sprintf(`{"version":1,"parent_id":%d}`, other.ID))
	moved := assertPatched(t, rec)
	assertParent(t, moved, other.ID)
	if moved.Version != 2 {
		t.Fatalf("container version=%d", moved.Version)
	}

	for _, id := range []int64{first.ID, second.ID} {
		got := requireItem(t, h, cookie, id)
		if got.Version != 1 || len(got.Locations) != 1 {
			t.Fatalf("item after move=%+v", got)
		}
		link := got.Locations[0]
		if link.LocationID != toy.ID || link.Path[len(link.Path)-1].ID != link.LocationID {
			t.Fatalf("location_id=%d path=%+v", link.LocationID, link.Path)
		}
		if !linkHasName(link, "客厅") || !linkHasName(link, "另一柜") || linkHasName(link, "储物柜") {
			t.Fatalf("path=%+v", link.Path)
		}
		if len(link.Path) != 3 || link.Path[0].ID != living.ID || link.Path[1].ID != other.ID || link.Path[2].ID != toy.ID {
			t.Fatalf("path=%+v", link.Path)
		}
		if countQuery(t, db, `SELECT COUNT(*) FROM item_locations WHERE item_id = ? AND location_id = ?`, id, toy.ID) != 1 {
			t.Fatalf("item_locations rewritten for %d", id)
		}
	}

	oldParent := mustItemPage(t, getItem(h, cookie, fmt.Sprintf("/api/v1/items?location=%d", cabinet.ID)))
	if oldParent.Total != 0 || len(oldParent.Data) != 0 {
		t.Fatalf("old parent items=%+v", oldParent)
	}
	inBox := mustItemPage(t, getItem(h, cookie, fmt.Sprintf("/api/v1/items?location=%d", toy.ID)))
	if inBox.Total != 2 || len(inBox.Data) != 2 {
		t.Fatalf("box items=%+v", inBox)
	}
	newParent := mustItemPage(t, getItem(h, cookie, fmt.Sprintf("/api/v1/items?location=%d", other.ID)))
	if newParent.Total != 0 || len(newParent.Data) != 0 {
		t.Fatalf("new parent items=%+v", newParent)
	}
}

func TestItemLocationOrder(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)
	lower := mustCreate(t, h, cookie, `{"name":"车位","type":"area"}`)
	higher := mustCreate(t, h, cookie, `{"name":"备胎位","type":"area"}`)
	if higher.ID <= lower.ID {
		t.Fatalf("ids lower=%d higher=%d", lower.ID, higher.ID)
	}
	body := fmt.Sprintf(`{"name":"遥控车","locations":[{"location_id":%d,"note":"车和遥控器"},{"location_id":%d,"note":"备用轮胎"}]}`, higher.ID, lower.ID)
	created := mustItem(t, h, cookie, body)
	got := requireItem(t, h, cookie, created.ID)
	for _, item := range []itemBody{created, got} {
		if len(item.Locations) != 2 {
			t.Fatalf("locations=%+v", item.Locations)
		}
		if item.Locations[0].LocationID != lower.ID || item.Locations[0].Note == nil || *item.Locations[0].Note != "备用轮胎" {
			t.Fatalf("first=%+v", item.Locations[0])
		}
		if item.Locations[1].LocationID != higher.ID || item.Locations[1].Note == nil || *item.Locations[1].Note != "车和遥控器" {
			t.Fatalf("second=%+v", item.Locations[1])
		}
		if item.Locations[0].LocationID >= item.Locations[1].LocationID {
			t.Fatalf("order=%+v", item.Locations)
		}
	}
}

func TestItemDirectOnly(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)
	parent := mustCreate(t, h, cookie, `{"name":"客厅","type":"area"}`)
	child := mustCreate(t, h, cookie, fmt.Sprintf(`{"name":"抽屉","type":"fixed","code":"010","parent_id":%d}`, parent.ID))
	direct := mustItem(t, h, cookie, fmt.Sprintf(`{"name":"父级物品","locations":[{"location_id":%d}]}`, parent.ID))
	nested := mustItem(t, h, cookie, fmt.Sprintf(`{"name":"子级物品","locations":[{"location_id":%d}]}`, child.ID))
	if requireItem(t, h, cookie, nested.ID).Locations[0].LocationID != child.ID {
		t.Fatal("nested item is not linked to the child")
	}

	page := mustItemPage(t, getItem(h, cookie, fmt.Sprintf("/api/v1/items?location=%d", parent.ID)))
	assertItemNames(t, page, "父级物品")
	for _, item := range page.Data {
		if item.ID == nested.ID {
			t.Fatalf("child item listed under parent: %+v", item)
		}
	}
	if page.Data[0].ID != direct.ID {
		t.Fatalf("direct=%+v", page.Data[0])
	}

	children := mustPage(t, getLoc(h, cookie, fmt.Sprintf("/api/v1/locations?parent=%d", parent.ID)))
	assertNames(t, children, "抽屉")
	if children.Data[0].ID != child.ID {
		t.Fatalf("child=%+v", children.Data[0])
	}
}

func TestItemPlacement(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)
	box := mustCreate(t, h, cookie, `{"name":"工具箱","type":"movable","code":"014"}`)
	emptyPlace := mustCreate(t, h, cookie, `{"name":"空位","type":"area"}`)
	loose := mustItem(t, h, cookie, `{"name":"待定位钳"}`)
	placed := mustItem(t, h, cookie, fmt.Sprintf(`{"name":"已放扳手","locations":[{"location_id":%d}]}`, box.ID))

	page := mustItemPage(t, getItem(h, cookie, "/api/v1/items?placement=unlocated"))
	assertItemNames(t, page, "待定位钳")
	if page.Data[0].ID != loose.ID || len(page.Data[0].Locations) != 0 || page.Limit != 30 || page.Offset != 0 {
		t.Fatalf("unlocated=%+v", page)
	}
	for _, item := range page.Data {
		if item.ID == placed.ID {
			t.Fatalf("located item in unlocated list: %+v", item)
		}
	}

	rec := patchItem(h, cookie, itemURL(loose.ID), fmt.Sprintf(`{"version":1,"locations":[{"location_id":%d}]}`, box.ID))
	linked := assertItemOK(t, rec)
	if linked.Version != 2 || len(linked.Locations) != 1 || linked.Locations[0].LocationID != box.ID {
		t.Fatalf("linked=%+v", linked)
	}
	page = mustItemPage(t, getItem(h, cookie, "/api/v1/items?placement=unlocated"))
	if page.Total != 0 || len(page.Data) != 0 || !strings.Contains(rec.Body.String(), `"locations":[`) {
		t.Fatalf("still unlocated=%+v body=%s", page, rec.Body.String())
	}
	left := mustItemPage(t, getItem(h, cookie, "/api/v1/items?placement=unlocated"))
	if left.Total != 0 || len(left.Data) != 0 || !strings.Contains(getItem(h, cookie, "/api/v1/items?placement=unlocated").Body.String(), `"data":[]`) {
		t.Fatalf("left=%+v", left)
	}

	rec = getItem(h, cookie, "/api/v1/items?placement=any")
	assertInvalidFields(t, rec, map[string]string{"placement": "不支持的参数"})
	rec = getItem(h, cookie, fmt.Sprintf("/api/v1/items?placement=unlocated&location=%d", box.ID))
	assertInvalidFields(t, rec, map[string]string{"placement": "不支持的参数", "location": "不支持的参数"})
	rec = getItem(h, cookie, "/api/v1/items?placement=any&location=1")
	assertInvalidFields(t, rec, map[string]string{"placement": "不支持的参数", "location": "不支持的参数"})
	rec = getItem(h, cookie, "/api/v1/items?placement=unlocated&location=abc")
	assertInvalidFields(t, rec, map[string]string{"placement": "不支持的参数", "location": "不支持的参数"})

	rec = getItem(h, cookie, "/api/v1/items?location=999999")
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	empty := mustItemPage(t, getItem(h, cookie, fmt.Sprintf("/api/v1/items?location=%d", emptyPlace.ID)))
	if empty.Total != 0 || len(empty.Data) != 0 || empty.Limit != 30 || empty.Offset != 0 || !strings.Contains(getItem(h, cookie, fmt.Sprintf("/api/v1/items?location=%d", emptyPlace.ID)).Body.String(), `"data":[]`) {
		t.Fatalf("empty place=%+v", empty)
	}

	badQuery := []struct {
		raw    string
		fields map[string]string
	}{
		{"parent=1", map[string]string{"parent": "不支持的参数"}},
		{"flat=1", map[string]string{"flat": "不支持的参数"}},
		{"limit=1&limit=2", map[string]string{"limit": "不支持的参数"}},
		{"location=0", map[string]string{"location": "参数不正确"}},
		{"location=01", map[string]string{"location": "参数不正确"}},
		{"location=abc", map[string]string{"location": "参数不正确"}},
		{"limit=0&offset=-1", map[string]string{"limit": "数量超出范围", "offset": "起点不正确"}},
		{"limit=0&offset=-1&placement=any", map[string]string{"limit": "数量超出范围", "offset": "起点不正确", "placement": "不支持的参数"}},
		{"placement=unlocated&placement=any", map[string]string{"placement": "不支持的参数"}},
	}
	for _, tc := range badQuery {
		rec = getItem(h, cookie, "/api/v1/items?"+tc.raw)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status=%d body=%s", tc.raw, rec.Code, rec.Body.String())
		}
		assertInvalidFields(t, rec, tc.fields)
	}

	rec = request(h, http.MethodGet, "/api/v1/items?placement=unlocated", "", "http://evil.example", testRemote, cookie)
	page = mustItemPage(t, rec)
	if page.Total != 0 {
		t.Fatalf("origin affected GET: %+v", page)
	}
}

func TestItemPatch(t *testing.T) {
	db, h, _ := testHandler(t, false)
	cookie := login(t, h)
	place := mustCreate(t, h, cookie, `{"name":"抽屉","type":"movable","code":"014"}`)
	other := mustCreate(t, h, cookie, `{"name":"另一格","type":"area"}`)
	item := mustItem(t, h, cookie, fmt.Sprintf(`{"name":"钳","alias":"尖嘴","locations":[{"location_id":%d,"note":"原位"}]}`, place.ID))

	rec := patchItem(h, cookie, itemURL(item.ID), `{"name":"钳","version":1}`)
	same := assertItemOK(t, rec)
	if same.Version != 2 || same.Name != "钳" || same.Alias == nil || *same.Alias != "尖嘴" || same.CreatedAt != frozenAt {
		t.Fatalf("same name=%+v", same)
	}
	if len(same.Locations) != 1 || same.Locations[0].LocationID != place.ID || same.Locations[0].Note == nil || *same.Locations[0].Note != "原位" {
		t.Fatalf("omitted locations=%+v", same.Locations)
	}

	rec = patchItem(h, cookie, itemURL(item.ID), `{"alias":null,"version":2}`)
	cleared := assertItemOK(t, rec)
	assertNullJSON(t, rec.Body.String(), "alias")
	if cleared.Version != 3 || cleared.Alias != nil || len(cleared.Locations) != 1 || cleared.Locations[0].Note == nil || *cleared.Locations[0].Note != "原位" {
		t.Fatalf("null alias=%+v", cleared)
	}
	again := requireItem(t, h, cookie, item.ID)
	if again.Alias != nil || again.Version != 3 {
		t.Fatalf("read back=%+v", again)
	}

	rec = patchItem(h, cookie, itemURL(item.ID), `{"locations":[],"version":3}`)
	unlocated := assertItemOK(t, rec)
	if unlocated.Version != 4 || len(unlocated.Locations) != 0 || !strings.Contains(rec.Body.String(), `"locations":[]`) || unlocated.Alias != nil {
		t.Fatalf("cleared locations=%+v body=%s", unlocated, rec.Body.String())
	}
	if countQuery(t, db, `SELECT COUNT(*) FROM item_locations WHERE item_id = ?`, item.ID) != 0 {
		t.Fatal("links remained")
	}

	rec = patchItem(h, cookie, itemURL(item.ID), fmt.Sprintf(`{"version":4,"locations":[{"location_id":%d,"note":"  "}]}`, place.ID))
	blankNote := assertItemOK(t, rec)
	if blankNote.Version != 5 || len(blankNote.Locations) != 1 || blankNote.Locations[0].Note != nil {
		t.Fatalf("blank note=%+v", blankNote.Locations)
	}
	assertNullJSON(t, rec.Body.String(), "note")

	rec = patchItem(h, cookie, itemURL(item.ID), fmt.Sprintf(`{"version":5,"locations":[{"location_id":%d,"note":"原位"}]}`, place.ID))
	if assertItemOK(t, rec).Version != 6 {
		t.Fatal("same link did not bump version")
	}
	rec = patchItem(h, cookie, itemURL(item.ID)+"?flat=1", `{`)
	assertInvalidFields(t, rec, map[string]string{"flat": "不支持的参数"})
	if requireItem(t, h, cookie, item.ID).Version != 6 {
		t.Fatal("query patch wrote")
	}

	rec = patchItem(h, cookie, itemURL(item.ID), `{"version":6}`)
	assertInvalidFields(t, rec, map[string]string{"request": "没有要修改的内容"})
	rec = patchItem(h, cookie, itemURL(item.ID), `{"name":"别的","version":0}`)
	assertInvalidFields(t, rec, map[string]string{"version": "版本不正确"})
	rec = patchItem(h, cookie, itemURL(item.ID), `{"name":"别的"}`)
	assertInvalidFields(t, rec, map[string]string{"version": "版本不正确"})
	rec = patchItem(h, cookie, itemURL(item.ID), `{"name":"","version":0}`)
	assertInvalidFields(t, rec, map[string]string{"name": "请填写名称", "version": "版本不正确"})
	rec = patchItem(h, cookie, itemURL(item.ID), `{"name":"别的","version":1.5}`)
	assertError(t, rec, http.StatusBadRequest, "invalid_body", "请求格式不正确")
	assertNoFieldsKey(t, rec)
	kept := requireItem(t, h, cookie, item.ID)
	if kept.Version != 6 || kept.Name != "钳" || len(kept.Locations) != 1 || kept.Locations[0].LocationID != place.ID {
		t.Fatalf("invalid patch wrote=%+v", kept)
	}

	rec = patchItem(h, cookie, itemURL(item.ID), fmt.Sprintf(`{"version":6,"locations":[{"location_id":%d}]}`, other.ID))
	swapped := assertItemOK(t, rec)
	if swapped.Version != 7 || len(swapped.Locations) != 1 || swapped.Locations[0].LocationID != other.ID {
		t.Fatalf("replaced=%+v", swapped.Locations)
	}
	if countQuery(t, db, `SELECT COUNT(*) FROM item_locations WHERE item_id = ?`, item.ID) != 1 {
		t.Fatal("old link remained after replace")
	}

	fresh := mustItem(t, h, cookie, `{"name":"温湿度传感器"}`)
	db2 := migratedDB(t)
	if err := auth.Bootstrap(context.Background(), db2, "ada", "correct-horse"); err != nil {
		t.Fatal(err)
	}
	dummy, err := auth.NewDummyHash()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	h2 := New(db2, config.Config{
		PublicOrigin: webOrigin,
		SessionTTL:   time.Hour,
		LoginLimit:   10,
		CookieSecure: false,
	}, dummy, func() time.Time { return now })
	cookie2 := login(t, h2)
	stamped := mustItem(t, h2, cookie2, `{"name":"温湿度传感器"}`)
	now = now.Add(time.Second)
	rec = patchItem(h2, cookie2, itemURL(stamped.ID), `{"name":"温湿度传感器","version":1}`)
	bumped := assertItemOK(t, rec)
	later := "2026-09-29T12:00:01Z"
	if bumped.Version != 2 || bumped.CreatedAt != frozenAt || bumped.UpdatedAt != later || bumped.Name != "温湿度传感器" {
		t.Fatalf("timestamp=%+v", bumped)
	}
	if fresh.Version != 1 {
		t.Fatalf("unrelated item=%+v", fresh)
	}
}

func TestItemCategoriesReplace(t *testing.T) {
	db, h, _ := testHandler(t, false)
	cookie := login(t, h)
	elec := mustCreateCat(t, h, cookie, `{"name":"电子配件"}`)
	sensor := mustCreateCat(t, h, cookie, fmt.Sprintf(`{"name":"传感器","parent_id":%d}`, elec.ID))
	place := mustCreate(t, h, cookie, `{"name":"抽屉","type":"movable","code":"040"}`)

	createRec := postItem(h, cookie, "/api/v1/items", fmt.Sprintf(`{"name":"温湿度传感器","categories":[{"category_id":%d}]}`, sensor.ID))
	created := assertItemCreated(t, createRec)
	if !strings.Contains(createRec.Body.String(), `"source":"human"`) {
		t.Fatalf("missing source: %s", createRec.Body.String())
	}
	if len(created.Categories) != 1 || created.Categories[0].CategoryID != sensor.ID {
		t.Fatalf("created=%+v", created.Categories)
	}
	if len(created.Categories[0].Path) != 2 || created.Categories[0].Path[0].ID != elec.ID || created.Categories[0].Path[1].ID != sensor.ID {
		t.Fatalf("path=%+v", created.Categories[0].Path)
	}
	if created.Categories[0].Path[0].Name != "电子配件" || created.Categories[0].Path[1].Name != "传感器" {
		t.Fatalf("path names=%+v", created.Categories[0].Path)
	}
	got := requireItem(t, h, cookie, created.ID)
	if len(got.Categories) != 1 || got.Categories[0].CategoryID != sensor.ID {
		t.Fatalf("read=%+v", got.Categories)
	}

	ordered := mustItem(t, h, cookie, fmt.Sprintf(`{"name":"配件盒","categories":[{"category_id":%d},{"category_id":%d}]}`, sensor.ID, elec.ID))
	if len(ordered.Categories) != 2 || ordered.Categories[0].CategoryID != elec.ID || ordered.Categories[1].CategoryID != sensor.ID {
		t.Fatalf("order=%+v", ordered.Categories)
	}
	if ordered.Categories[0].CategoryID >= ordered.Categories[1].CategoryID {
		t.Fatalf("order=%+v", ordered.Categories)
	}

	rec := patchItem(h, cookie, itemURL(created.ID), `{"name":"温湿度传感器","version":1}`)
	same := assertItemOK(t, rec)
	if same.Version != 2 || len(same.Categories) != 1 || same.Categories[0].CategoryID != sensor.ID {
		t.Fatalf("omitted=%+v", same)
	}

	rec = patchItem(h, cookie, itemURL(created.ID), `{"categories":[],"version":2}`)
	cleared := assertItemOK(t, rec)
	if cleared.Version != 3 || len(cleared.Categories) != 0 || !strings.Contains(rec.Body.String(), `"categories":[]`) {
		t.Fatalf("cleared=%+v body=%s", cleared, rec.Body.String())
	}
	if countQuery(t, db, `SELECT COUNT(*) FROM item_categories WHERE item_id = ?`, created.ID) != 0 {
		t.Fatal("category links remained")
	}

	rec = patchItem(h, cookie, itemURL(created.ID), fmt.Sprintf(`{"version":3,"categories":[{"category_id":%d}]}`, sensor.ID))
	restored := assertItemOK(t, rec)
	if restored.Version != 4 || len(restored.Categories) != 1 || restored.Categories[0].CategoryID != sensor.ID {
		t.Fatalf("restored=%+v", restored)
	}
	rec = patchItem(h, cookie, itemURL(created.ID), fmt.Sprintf(`{"version":4,"categories":[{"category_id":%d}]}`, sensor.ID))
	if assertItemOK(t, rec).Version != 5 {
		t.Fatal("same categories did not bump version")
	}

	for _, body := range []string{
		`{"version":5,"categories":"电子配件"}`,
		`{"version":5,"categories":[null]}`,
	} {
		rec = patchItem(h, cookie, itemURL(created.ID), body)
		assertError(t, rec, http.StatusBadRequest, "invalid_body", "请求格式不正确")
		assertNoFieldsKey(t, rec)
	}
	kept := requireItem(t, h, cookie, created.ID)
	if kept.Version != 5 || len(kept.Categories) != 1 || kept.Categories[0].CategoryID != sensor.ID {
		t.Fatalf("invalid body wrote=%+v", kept)
	}

	rec = patchItem(h, cookie, itemURL(created.ID), `{"version":5,"categories":null}`)
	assertInvalidFields(t, rec, map[string]string{"categories": "分类格式不正确"})

	before := countQuery(t, db, `SELECT COUNT(*) FROM items`)
	rec = postItem(h, cookie, "/api/v1/items", fmt.Sprintf(`{"name":"双份","categories":[{"category_id":%d},{"category_id":%d}]}`, sensor.ID, sensor.ID))
	assertInvalidFields(t, rec, map[string]string{"categories": "同一分类只能关联一次"})
	rec = postItem(h, cookie, "/api/v1/items", `{"name":"零","categories":[{"category_id":0}]}`)
	assertInvalidFields(t, rec, map[string]string{"categories": "所选分类不存在"})
	rec = postItem(h, cookie, "/api/v1/items", `{"name":"负","categories":[{"category_id":-4}]}`)
	assertInvalidFields(t, rec, map[string]string{"categories": "所选分类不存在"})
	rec = postItem(h, cookie, "/api/v1/items", `{"name":"不存在","categories":[{"category_id":999999}]}`)
	assertInvalidFields(t, rec, map[string]string{"categories": "所选分类不存在"})
	rec = postItem(h, cookie, "/api/v1/items", `{"name":"缺","categories":[{}]}`)
	assertInvalidFields(t, rec, map[string]string{"categories": "分类格式不正确"})
	rec = postItem(h, cookie, "/api/v1/items", `{"name":"空id","categories":[{"category_id":null}]}`)
	assertInvalidFields(t, rec, map[string]string{"categories": "分类格式不正确"})
	rec = postItem(h, cookie, "/api/v1/items", `{"name":"","categories":[{"category_id":0}]}`)
	assertInvalidFields(t, rec, map[string]string{"name": "请填写名称", "categories": "所选分类不存在"})
	if countQuery(t, db, `SELECT COUNT(*) FROM items`) != before {
		t.Fatal("rejected category create wrote a row")
	}

	rec = patchItem(h, cookie, itemURL(created.ID), fmt.Sprintf(`{"version":5,"locations":[{"location_id":%d}],"categories":[{"category_id":%d}]}`, place.ID, elec.ID))
	both := assertItemOK(t, rec)
	if both.Version != 6 || len(both.Locations) != 1 || both.Locations[0].LocationID != place.ID {
		t.Fatalf("locations=%+v", both.Locations)
	}
	if len(both.Categories) != 1 || both.Categories[0].CategoryID != elec.ID {
		t.Fatalf("categories=%+v", both.Categories)
	}

	rec = patchItem(h, cookie, itemURL(created.ID), fmt.Sprintf(`{"version":6,"categories":[{"category_id":%d,"note":"忽略"}]}`, sensor.ID))
	ignored := assertItemOK(t, rec)
	if ignored.Version != 7 || len(ignored.Categories) != 1 || ignored.Categories[0].CategoryID != sensor.ID {
		t.Fatalf("ignored extra=%+v", ignored.Categories)
	}
	if requireItem(t, h, cookie, created.ID).Version != 7 {
		t.Fatal("extra field write failed")
	}
}

func TestItemCategorySourceDefaultHuman(t *testing.T) {
	db, h, _ := testHandler(t, false)
	cookie := login(t, h)
	cat := mustCreateCat(t, h, cookie, `{"name":"传感器"}`)
	item := mustItem(t, h, cookie, fmt.Sprintf(`{"name":"温湿度传感器","categories":[{"category_id":%d}]}`, cat.ID))
	if len(item.Categories) != 1 || item.Categories[0].Source != "human" {
		t.Fatalf("categories=%+v", item.Categories)
	}
	var source string
	if err := db.QueryRow(`SELECT source FROM item_categories WHERE item_id = ?`, item.ID).Scan(&source); err != nil {
		t.Fatal(err)
	}
	if source != "human" {
		t.Fatalf("source=%q", source)
	}
}

func TestItemDelete(t *testing.T) {
	db, h, _ := testHandler(t, false)
	cookie := login(t, h)
	place := mustCreate(t, h, cookie, `{"name":"抽屉","type":"movable","code":"014"}`)
	cat := mustCreateCat(t, h, cookie, `{"name":"传感器"}`)
	item := mustItem(t, h, cookie, fmt.Sprintf(`{"name":"门磁传感器","locations":[{"location_id":%d,"note":"上层"}],"categories":[{"category_id":%d}]}`, place.ID, cat.ID))
	if countQuery(t, db, `SELECT COUNT(*) FROM item_locations WHERE item_id = ?`, item.ID) != 1 {
		t.Fatal("link missing before delete")
	}
	if countQuery(t, db, `SELECT COUNT(*) FROM item_categories WHERE item_id = ?`, item.ID) != 1 {
		t.Fatal("category link missing before delete")
	}

	rec := deleteItem(h, cookie, itemURL(item.ID)+"?version=1")
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	if countQuery(t, db, `SELECT COUNT(*) FROM items WHERE id = ?`, item.ID) != 1 {
		t.Fatal("item row missing")
	}
	if itemDeletedAt(t, db, item.ID) == "" {
		t.Fatal("deleted_at empty")
	}
	if countQuery(t, db, `SELECT COUNT(*) FROM item_locations WHERE item_id = ?`, item.ID) != 1 {
		t.Fatal("item_locations removed")
	}
	if countQuery(t, db, `SELECT COUNT(*) FROM item_categories WHERE item_id = ?`, item.ID) != 1 {
		t.Fatal("item_categories removed")
	}
	kept := requireLoc(t, h, cookie, place.ID)
	if kept.Name != "抽屉" || kept.Version != 1 {
		t.Fatalf("location changed=%+v", kept)
	}
	keptCat := requireCat(t, h, cookie, cat.ID)
	if keptCat.Name != "传感器" || keptCat.Version != 1 {
		t.Fatalf("category changed=%+v", keptCat)
	}
	rec = getItem(h, cookie, itemURL(item.ID))
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")

	trashed := requireTrash(t, h, cookie, item.ID)
	if trashed.Name != "门磁传感器" || len(trashed.Locations) != 1 || trashed.Locations[0].LocationID != place.ID || trashed.Locations[0].Note == nil || *trashed.Locations[0].Note != "上层" || len(trashed.Categories) != 1 || trashed.Categories[0].CategoryID != cat.ID {
		t.Fatalf("trash item=%+v", trashed)
	}

	rec = deleteLoc(h, cookie, locURL(place.ID)+"?version=1")
	assertError(t, rec, http.StatusConflict, "location_in_use", "这个位置下面还有内容")
	kept = requireLoc(t, h, cookie, place.ID)
	if kept.Name != "抽屉" || kept.Version != 1 {
		t.Fatalf("in-use location=%+v", kept)
	}

	linked := mustItem(t, h, cookie, fmt.Sprintf(`{"name":"仍在","locations":[{"location_id":%d}]}`, place.ID))
	rec = deleteLoc(h, cookie, locURL(place.ID)+"?version=1")
	assertError(t, rec, http.StatusConflict, "location_in_use", "这个位置下面还有内容")
	kept = requireLoc(t, h, cookie, place.ID)
	if kept.Name != "抽屉" || kept.Version != 1 {
		t.Fatalf("in-use location=%+v", kept)
	}
	still := requireItem(t, h, cookie, linked.ID)
	if still.Name != "仍在" || len(still.Locations) != 1 || still.Locations[0].LocationID != place.ID {
		t.Fatalf("item changed=%+v", still)
	}

	rec = deleteItem(h, cookie, "/api/v1/items/999999?version=abc")
	if rec.Code == http.StatusNotFound {
		t.Fatalf("bad version returned 404: %s", rec.Body.String())
	}
	assertInvalidFields(t, rec, map[string]string{"version": "版本不正确"})
	rec = deleteItem(h, cookie, "/api/v1/items/999999?version=1")
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	rec = request(h, http.MethodDelete, "/api/v1/items/abc", "", "http://evil.example", testRemote, "")
	assertError(t, rec, http.StatusForbidden, "origin_rejected", "来源不被接受")
	rec = request(h, http.MethodPut, itemURL(linked.ID), `{"name":"x"}`, webOrigin, testRemote, cookie)
	if rec.Code == http.StatusMethodNotAllowed {
		t.Fatalf("PUT id returned 405: %s", rec.Body.String())
	}
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	if requireItem(t, h, cookie, linked.ID).Name != "仍在" {
		t.Fatal("wrong method wrote")
	}
}

func TestItemIDReuse(t *testing.T) {
	db, h, _ := testHandler(t, false)
	cookie := login(t, h)
	first := mustItem(t, h, cookie, `{"name":"旧记录"}`)
	maxID := countQuery(t, db, `SELECT MAX(id) FROM items`)
	if int64(maxID) != first.ID {
		t.Fatalf("max=%d item=%d", maxID, first.ID)
	}
	rec := deleteItem(h, cookie, itemURL(first.ID)+"?version=1")
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	trashed := requireTrash(t, h, cookie, first.ID)
	if trashed.Name != "旧记录" || trashed.DeletedAt == nil || *trashed.DeletedAt == "" {
		t.Fatalf("trash=%+v", trashed)
	}
	next := mustItem(t, h, cookie, `{"name":"新记录"}`)
	if next.ID == first.ID || next.Version != 1 || next.Name != "新记录" {
		t.Fatalf("reused id=%d next=%+v", first.ID, next)
	}
	rec = patchItem(h, cookie, itemURL(first.ID), `{"name":"被改掉","version":1}`)
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	rec = deleteItem(h, cookie, itemURL(first.ID)+"?version=1")
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	got := requireItem(t, h, cookie, next.ID)
	if got.Name != "新记录" || got.Version != 1 {
		t.Fatalf("new item changed=%+v", got)
	}
	if requireTrash(t, h, cookie, first.ID).Name != "旧记录" {
		t.Fatal("old trash row changed")
	}
}

func TestItemPurgeIDReuse(t *testing.T) {
	db, h, _ := testHandler(t, false)
	cookie := login(t, h)
	first := mustItem(t, h, cookie, `{"name":"旧记录"}`)
	maxID := countQuery(t, db, `SELECT MAX(id) FROM items`)
	if int64(maxID) != first.ID {
		t.Fatalf("max=%d item=%d", maxID, first.ID)
	}
	rec := deleteItem(h, cookie, itemURL(first.ID)+"?version=1")
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	trashed := requireTrash(t, h, cookie, first.ID)
	if trashed.Version != 2 {
		t.Fatalf("trash version=%d", trashed.Version)
	}
	rec = deleteItem(h, cookie, trashURL(first.ID)+"?version=2")
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("purge status=%d body=%q", rec.Code, rec.Body.String())
	}
	next := mustItem(t, h, cookie, `{"name":"新记录"}`)
	if next.ID == first.ID || next.Version != 1 || next.Name != "新记录" {
		t.Fatalf("reused id=%d next=%+v", first.ID, next)
	}
	rec = patchItem(h, cookie, itemURL(first.ID), `{"name":"被改掉","version":1}`)
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	rec = deleteItem(h, cookie, itemURL(first.ID)+"?version=1")
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	rec = deleteItem(h, cookie, trashURL(first.ID)+"?version=1")
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	got := requireItem(t, h, cookie, next.ID)
	if got.Name != "新记录" || got.Version != 1 {
		t.Fatalf("new item changed=%+v", got)
	}
}

func TestItemVersion(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)
	place := mustCreate(t, h, cookie, `{"name":"抽屉","type":"movable","code":"021"}`)
	item := mustItem(t, h, cookie, fmt.Sprintf(`{"name":"旧名","locations":[{"location_id":%d,"note":"原位"}]}`, place.ID))
	rec := patchItem(h, cookie, itemURL(item.ID), `{"name":"新名","version":1}`)
	renamed := assertItemOK(t, rec)
	if renamed.Version != 2 || renamed.Name != "新名" || len(renamed.Locations) != 1 {
		t.Fatalf("rename=%+v", renamed)
	}

	rec = patchItem(h, cookie, itemURL(item.ID), `{"name":"更新名","version":1,"locations":[]}`)
	assertError(t, rec, http.StatusConflict, "version_conflict", "记录已被修改")
	assertNoFieldsKey(t, rec)
	got := requireItem(t, h, cookie, item.ID)
	if got.Name != "新名" || got.Version != 2 || len(got.Locations) != 1 || got.Locations[0].LocationID != place.ID || got.Locations[0].Note == nil || *got.Locations[0].Note != "原位" {
		t.Fatalf("stale patch wrote=%+v", got)
	}

	rec = deleteItem(h, cookie, itemURL(item.ID)+"?version=1")
	assertError(t, rec, http.StatusConflict, "version_conflict", "记录已被修改")
	got = requireItem(t, h, cookie, item.ID)
	if got.Name != "新名" || got.Version != 2 || len(got.Locations) != 1 {
		t.Fatalf("stale delete wrote=%+v", got)
	}

	rec = patchItem(h, cookie, itemURL(item.ID), `{"name":"","version":1}`)
	assertInvalidFields(t, rec, map[string]string{"name": "请填写名称"})
	got = requireItem(t, h, cookie, item.ID)
	if got.Name != "新名" || got.Version != 2 || len(got.Locations) != 1 || got.Locations[0].Note == nil || *got.Locations[0].Note != "原位" {
		t.Fatalf("empty stale name wrote=%+v", got)
	}
}

func TestItemBody(t *testing.T) {
	db, h, _ := testHandler(t, false)
	cookie := login(t, h)
	place := mustCreate(t, h, cookie, `{"name":"抽屉","type":"movable","code":"030"}`)
	item := mustItem(t, h, cookie, fmt.Sprintf(`{"name":"锚点","alias":"原别名","locations":[{"location_id":%d,"note":"原说明"}]}`, place.ID))

	missing := int64(999999)
	rec := patchItem(h, cookie, itemURL(missing), `{`)
	assertError(t, rec, http.StatusBadRequest, "invalid_body", "请求格式不正确")
	assertNoFieldsKey(t, rec)
	rec = patchItem(h, cookie, itemURL(missing), `{"name":"","version":1}`)
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	assertNoFieldsKey(t, rec)

	bodies := []string{
		`{"version":1,"locations":"客厅"}`,
		`{"version":1,"locations":[1]}`,
		`{"version":1,"locations":[null]}`,
		`[]`,
	}
	for _, body := range bodies {
		rec = patchItem(h, cookie, itemURL(item.ID), body)
		assertError(t, rec, http.StatusBadRequest, "invalid_body", "请求格式不正确")
		assertNoFieldsKey(t, rec)
	}
	nullBody := `{"version":1,"locations":null}`
	rec = patchItem(h, cookie, itemURL(item.ID), nullBody)
	assertInvalidFields(t, rec, map[string]string{"locations": "位置格式不正确"})
	rec = patchItem(h, cookie, itemURL(missing), nullBody)
	if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "invalid_fields") || strings.Contains(rec.Body.String(), `"fields"`) {
		t.Fatalf("missing locations null=%d %s", rec.Code, rec.Body.String())
	}
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	kept := requireItem(t, h, cookie, item.ID)
	if kept.Version != 1 || kept.Name != "锚点" || kept.Alias == nil || *kept.Alias != "原别名" || len(kept.Locations) != 1 || kept.Locations[0].Note == nil || *kept.Locations[0].Note != "原说明" {
		t.Fatalf("body errors wrote=%+v", kept)
	}

	before := countQuery(t, db, `SELECT COUNT(*) FROM items`)
	dup := fmt.Sprintf(`{"name":"双份","locations":[{"location_id":%d,"note":"一次"},{"location_id":%d,"note":"再次"}]}`, place.ID, place.ID)
	rec = postItem(h, cookie, "/api/v1/items", dup)
	assertInvalidFields(t, rec, map[string]string{"locations": "同一位置只能关联一次"})
	rec = postItem(h, cookie, "/api/v1/items", `{"name":"零","locations":[{"location_id":0}]}`)
	assertInvalidFields(t, rec, map[string]string{"locations": "所选位置不存在"})
	rec = postItem(h, cookie, "/api/v1/items", `{"name":"负","locations":[{"location_id":-4}]}`)
	assertInvalidFields(t, rec, map[string]string{"locations": "所选位置不存在"})
	rec = postItem(h, cookie, "/api/v1/items", `{"name":"缺","locations":[{"note":"放这儿"}]}`)
	assertInvalidFields(t, rec, map[string]string{"locations": "位置格式不正确"})
	rec = postItem(h, cookie, "/api/v1/items", `{"name":"空位","locations":[{"location_id":null}]}`)
	assertInvalidFields(t, rec, map[string]string{"locations": "位置格式不正确"})
	rec = postItem(h, cookie, "/api/v1/items", `{"name":"不存在","locations":[{"location_id":999999}]}`)
	assertInvalidFields(t, rec, map[string]string{"locations": "所选位置不存在"})
	rec = postItem(h, cookie, "/api/v1/items", `{"name":"","locations":[{"location_id":0},{"location_id":0}]}`)
	assertInvalidFields(t, rec, map[string]string{"name": "请填写名称", "locations": "所选位置不存在"})
	longNote := strings.Repeat("测", 2001)
	rec = postItem(h, cookie, "/api/v1/items", fmt.Sprintf(`{"name":"长说明","locations":[{"location_id":%d,"note":%q}]}`, place.ID, longNote))
	assertInvalidFields(t, rec, map[string]string{"locations": "放置说明过长"})
	rec = postItem(h, cookie, "/api/v1/items", fmt.Sprintf(`{"name":"控制","locations":[{"location_id":%d,"note":"a\u0000b"}]}`, place.ID))
	assertInvalidFields(t, rec, map[string]string{"locations": "放置说明不能包含控制字符"})
	if countQuery(t, db, `SELECT COUNT(*) FROM items`) != before {
		t.Fatal("rejected link create wrote a row")
	}

	rec = patchItem(h, cookie, itemURL(item.ID), fmt.Sprintf(`{"version":1,"name":"被改掉","locations":[{"location_id":%d},{"location_id":%d}]}`, place.ID, place.ID))
	assertInvalidFields(t, rec, map[string]string{"locations": "同一位置只能关联一次"})
	rec = patchItem(h, cookie, itemURL(item.ID), `{"version":1,"locations":[{"location_id":0}]}`)
	assertInvalidFields(t, rec, map[string]string{"locations": "所选位置不存在"})
	rec = patchItem(h, cookie, itemURL(item.ID), `{"version":1,"locations":[{"location_id":-2}]}`)
	assertInvalidFields(t, rec, map[string]string{"locations": "所选位置不存在"})
	rec = patchItem(h, cookie, itemURL(item.ID), `{"version":1,"locations":[{}]}`)
	assertInvalidFields(t, rec, map[string]string{"locations": "位置格式不正确"})
	rec = patchItem(h, cookie, itemURL(item.ID), `{"version":1,"locations":[{"location_id":null}]}`)
	assertInvalidFields(t, rec, map[string]string{"locations": "位置格式不正确"})
	rec = patchItem(h, cookie, itemURL(item.ID), `{"version":1,"name":"被改掉","locations":[{"location_id":999999}]}`)
	assertInvalidFields(t, rec, map[string]string{"locations": "所选位置不存在"})
	kept = requireItem(t, h, cookie, item.ID)
	if kept.Version != 1 || kept.Name != "锚点" || kept.Alias == nil || *kept.Alias != "原别名" || len(kept.Locations) != 1 || kept.Locations[0].LocationID != place.ID || kept.Locations[0].Note == nil || *kept.Locations[0].Note != "原说明" {
		t.Fatalf("link errors wrote=%+v", kept)
	}
	if countQuery(t, db, `SELECT COUNT(*) FROM item_locations WHERE item_id = ?`, item.ID) != 1 {
		t.Fatal("links changed")
	}
}

func TestItemAuth(t *testing.T) {
	db, h, _ := testHandler(t, false)
	cookie := login(t, h)
	if countQuery(t, db, `SELECT COUNT(*) FROM items`) != 0 {
		t.Fatal("items already present")
	}
	rec := postItem(h, "", "/api/v1/items", `{"name":"未登录"}`)
	assertError(t, rec, http.StatusUnauthorized, "unauthenticated", "未登录")
	rec = request(h, http.MethodPost, "/api/v1/items", `{"name":"坏来源"}`, "http://evil.example", testRemote, cookie)
	assertError(t, rec, http.StatusForbidden, "origin_rejected", "来源不被接受")
	rec = request(h, http.MethodPost, "/api/v1/items", `{"name":"坏来源"}`, "http://evil.example", testRemote, "")
	assertError(t, rec, http.StatusForbidden, "origin_rejected", "来源不被接受")
	if countQuery(t, db, `SELECT COUNT(*) FROM items`) != 0 {
		t.Fatal("rejected auth create wrote a row")
	}

	big := oversizedItemBody()
	rec = postItem(h, cookie, "/api/v1/items", big)
	assertError(t, rec, http.StatusRequestEntityTooLarge, "body_too_large", "请求正文过大")
	rec = request(h, http.MethodPost, "/api/v1/items", big, webOrigin, testRemote, "")
	assertError(t, rec, http.StatusUnauthorized, "unauthenticated", "未登录")
	rec = request(h, http.MethodPost, "/api/v1/items", big, "http://evil.example", testRemote, "")
	assertError(t, rec, http.StatusForbidden, "origin_rejected", "来源不被接受")
	rec = request(h, http.MethodPost, "/api/v1/items", big, "http://evil.example", testRemote, cookie)
	assertError(t, rec, http.StatusForbidden, "origin_rejected", "来源不被接受")
	rec = request(h, http.MethodPatch, "/api/v1/items/999999", big, webOrigin, testRemote, cookie)
	assertError(t, rec, http.StatusRequestEntityTooLarge, "body_too_large", "请求正文过大")
	rec = request(h, http.MethodPatch, "/api/v1/items/999999", big, webOrigin, testRemote, "")
	assertError(t, rec, http.StatusUnauthorized, "unauthenticated", "未登录")
	rec = request(h, http.MethodPatch, "/api/v1/items/999999", big, "http://evil.example", testRemote, cookie)
	assertError(t, rec, http.StatusForbidden, "origin_rejected", "来源不被接受")
	if countQuery(t, db, `SELECT COUNT(*) FROM items`) != 0 {
		t.Fatal("oversized request wrote a row")
	}

	rec = getItem(h, "", "/api/v1/items")
	assertError(t, rec, http.StatusUnauthorized, "unauthenticated", "未登录")

	loginBody := `{"username":"ada","password":"` + strings.Repeat("a", 4096) + `"}`
	rec = request(h, http.MethodPost, "/api/v1/session", loginBody, webOrigin, "203.0.113.77:1", "")
	assertError(t, rec, http.StatusRequestEntityTooLarge, "body_too_large", "请求正文过大")
}

func TestItemListOrder(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)
	rec := getItem(h, cookie, "/api/v1/items")
	empty := mustItemPage(t, rec)
	if empty.Total != 0 || len(empty.Data) != 0 || empty.Limit != 30 || empty.Offset != 0 || !strings.Contains(rec.Body.String(), `"data":[]`) {
		t.Fatalf("empty=%+v body=%s", empty, rec.Body.String())
	}

	// 甲 is created first. Unicode order still returns 乙 before 甲; pinyin and id order would not.
	jia := mustItem(t, h, cookie, `{"name":"甲"}`)
	yi := mustItem(t, h, cookie, `{"name":"乙"}`)
	if yi.ID <= jia.ID {
		t.Fatalf("ids jia=%d yi=%d", jia.ID, yi.ID)
	}
	page := mustItemPage(t, getItem(h, cookie, "/api/v1/items"))
	assertItemNames(t, page, "乙", "甲")
	if page.Limit != 30 || page.Offset != 0 || page.Data[0].ID != yi.ID || page.Data[1].ID != jia.ID {
		t.Fatalf("order page=%+v", page)
	}

	_, h, _ = testHandler(t, false)
	cookie = login(t, h)
	ids := make([]int64, 31)
	for i := 0; i < 31; i++ {
		ids[i] = mustItem(t, h, cookie, `{"name":"同名"}`).ID
	}
	page = mustItemPage(t, getItem(h, cookie, "/api/v1/items"))
	if page.Total != 31 || page.Limit != 30 || page.Offset != 0 || len(page.Data) != 30 {
		t.Fatalf("first page=%+v len=%d", page, len(page.Data))
	}
	if page.Data[0].ID != ids[0] || page.Data[29].ID != ids[29] {
		t.Fatalf("page ids=%d..%d want %d..%d", page.Data[0].ID, page.Data[29].ID, ids[0], ids[29])
	}
	for _, item := range page.Data {
		if item.Locations == nil || len(item.Locations) != 0 {
			t.Fatalf("locations=%+v", item.Locations)
		}
	}
	last := mustItemPage(t, getItem(h, cookie, "/api/v1/items?offset=30"))
	if last.Total != 31 || last.Limit != 30 || last.Offset != 30 || len(last.Data) != 1 || last.Data[0].ID != ids[30] {
		t.Fatalf("offset 30=%+v", last)
	}
	past := mustItemPage(t, getItem(h, cookie, "/api/v1/items?offset=31"))
	if past.Total != 31 || past.Limit != 30 || past.Offset != 31 || len(past.Data) != 0 {
		t.Fatalf("past=%+v", past)
	}
	pastRec := getItem(h, cookie, "/api/v1/items?offset=31")
	if !strings.Contains(pastRec.Body.String(), `"data":[]`) {
		t.Fatalf("past body=%s", pastRec.Body.String())
	}
}

func assertLinkNoteNull(t *testing.T, body string) {
	t.Helper()
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &obj); err != nil {
		t.Fatalf("body=%s err=%v", body, err)
	}
	var links []map[string]json.RawMessage
	if err := json.Unmarshal(obj["locations"], &links); err != nil {
		t.Fatalf("locations=%s err=%v", obj["locations"], err)
	}
	if len(links) != 1 || string(links[0]["note"]) != "null" {
		t.Fatalf("link note=%s body=%s", links[0]["note"], body)
	}
}

func TestItemStaleLocation(t *testing.T) {
	db, h, _ := testHandler(t, false)
	cookie := login(t, h)
	place := mustCreate(t, h, cookie, `{"name":"抽屉","type":"movable","code":"041"}`)
	item := mustItem(t, h, cookie, fmt.Sprintf(`{"name":"锚点","locations":[{"location_id":%d,"note":"原位"}]}`, place.ID))

	for _, locID := range []int64{0, 999999} {
		body := fmt.Sprintf(`{"version":2,"name":"被改掉","locations":[{"location_id":%d}]}`, locID)
		rec := patchItem(h, cookie, itemURL(item.ID), body)
		if rec.Code == http.StatusConflict || strings.Contains(rec.Body.String(), "version_conflict") {
			t.Fatalf("stale bad location returned conflict: %s", rec.Body.String())
		}
		assertInvalidFields(t, rec, map[string]string{"locations": "所选位置不存在"})
		got := requireItem(t, h, cookie, item.ID)
		if got.Name != "锚点" || got.Version != 1 || len(got.Locations) != 1 || got.Locations[0].LocationID != place.ID || got.Locations[0].Note == nil || *got.Locations[0].Note != "原位" {
			t.Fatalf("stale bad location wrote=%+v", got)
		}
		if countQuery(t, db, `SELECT COUNT(*) FROM item_locations WHERE item_id = ? AND location_id = ?`, item.ID, place.ID) != 1 {
			t.Fatal("links changed")
		}
	}
}

func TestItemPatchBadID(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)

	rec := request(h, http.MethodPatch, "/api/v1/items/abc", `{`, "http://evil.example", testRemote, "")
	assertError(t, rec, http.StatusForbidden, "origin_rejected", "来源不被接受")
	assertNoFieldsKey(t, rec)

	rec = request(h, http.MethodPatch, "/api/v1/items/abc", `{`, webOrigin, testRemote, cookie)
	assertError(t, rec, http.StatusBadRequest, "invalid_body", "请求格式不正确")
	assertNoFieldsKey(t, rec)

	rec = request(h, http.MethodPatch, "/api/v1/items/abc", `{"name":"x","version":1}`, webOrigin, testRemote, cookie)
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
}

func TestItemDeleteBody(t *testing.T) {
	db, h, _ := testHandler(t, false)
	cookie := login(t, h)
	item := mustItem(t, h, cookie, `{"name":"待删"}`)
	rec := request(h, http.MethodDelete, itemURL(item.ID)+"?version=1", `{`, webOrigin, testRemote, cookie)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 || strings.Contains(rec.Body.String(), "invalid_body") {
		t.Fatalf("delete read body: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if countQuery(t, db, `SELECT COUNT(*) FROM items WHERE id = ?`, item.ID) != 1 {
		t.Fatal("item missing after body delete")
	}
	if itemDeletedAt(t, db, item.ID) == "" {
		t.Fatal("deleted_at empty after body delete")
	}

	other := mustItem(t, h, cookie, `{"name":"正文版本"}`)
	rec = request(h, http.MethodDelete, itemURL(other.ID)+"?version=1", `{"version":999}`, webOrigin, testRemote, cookie)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 || strings.Contains(rec.Body.String(), "version_conflict") {
		t.Fatalf("body version affected delete: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if countQuery(t, db, `SELECT COUNT(*) FROM items WHERE id = ?`, other.ID) != 1 {
		t.Fatal("item missing after body version delete")
	}
	if itemDeletedAt(t, db, other.ID) == "" {
		t.Fatal("deleted_at empty after body version delete")
	}

	rec = request(h, http.MethodDelete, "/api/v1/items/999999?version=abc", `{`, webOrigin, testRemote, cookie)
	if rec.Code == http.StatusNotFound || strings.Contains(rec.Body.String(), "invalid_body") {
		t.Fatalf("bad version with body: status=%d body=%s", rec.Code, rec.Body.String())
	}
	assertInvalidFields(t, rec, map[string]string{"version": "版本不正确"})
}

func TestItemDeleteAuth(t *testing.T) {
	db, h, _ := testHandler(t, false)
	cookie := login(t, h)
	item := mustItem(t, h, cookie, `{"name":"留着"}`)
	path := itemURL(item.ID) + "?version=1"

	rec := request(h, http.MethodDelete, path, "", webOrigin, testRemote, "")
	assertError(t, rec, http.StatusUnauthorized, "unauthenticated", "未登录")
	rec = request(h, http.MethodDelete, path, "", "http://evil.example", testRemote, cookie)
	assertError(t, rec, http.StatusForbidden, "origin_rejected", "来源不被接受")
	rec = request(h, http.MethodDelete, path, "", "http://evil.example", testRemote, "")
	assertError(t, rec, http.StatusForbidden, "origin_rejected", "来源不被接受")

	got := requireItem(t, h, cookie, item.ID)
	if got.Name != "留着" || got.Version != 1 {
		t.Fatalf("rejected delete wrote=%+v", got)
	}
	if countQuery(t, db, `SELECT COUNT(*) FROM items WHERE id = ?`, item.ID) != 1 {
		t.Fatal("rejected delete removed the row")
	}
}

func TestItemTrash(t *testing.T) {
	db, h, _ := testHandler(t, false)
	cookie := login(t, h)
	place := mustCreate(t, h, cookie, `{"name":"抽屉","type":"movable","code":"014"}`)
	cat := mustCreateCat(t, h, cookie, `{"name":"传感器"}`)
	item := mustItem(t, h, cookie, fmt.Sprintf(`{"name":"门磁传感器","locations":[{"location_id":%d,"note":"上层"}],"categories":[{"category_id":%d}]}`, place.ID, cat.ID))

	rec := deleteItem(h, cookie, itemURL(item.ID)+"?version=1")
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	rec = getItem(h, cookie, itemURL(item.ID))
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	trashedRec := getItem(h, cookie, trashURL(item.ID))
	trashed := assertItemOK(t, trashedRec)
	if trashed.DeletedAt == nil || *trashed.DeletedAt == "" || trashed.Version != 2 || trashed.Name != "门磁传感器" {
		t.Fatalf("trash get=%+v", trashed)
	}
	if len(trashed.Locations) != 1 || trashed.Locations[0].LocationID != place.ID || trashed.Locations[0].Note == nil || *trashed.Locations[0].Note != "上层" {
		t.Fatalf("trash locations=%+v", trashed.Locations)
	}
	if len(trashed.Categories) != 1 || trashed.Categories[0].CategoryID != cat.ID {
		t.Fatalf("trash categories=%+v", trashed.Categories)
	}
	if !strings.Contains(trashedRec.Body.String(), `"return_tasks":[]`) {
		t.Fatalf("return_tasks=%s", trashedRec.Body.String())
	}
	if pageHasItemID(mustItemPage(t, getItem(h, cookie, "/api/v1/items")), item.ID) {
		t.Fatal("live list still has trashed item")
	}
	if pageHasItemID(mustItemPage(t, getItem(h, cookie, "/api/v1/items?q="+url.QueryEscape("门磁"))), item.ID) {
		t.Fatal("search still has trashed item")
	}
	trashPage := mustItemPage(t, getItem(h, cookie, "/api/v1/trash"))
	if trashPage.Total != 1 || trashPage.Limit != 30 || trashPage.Offset != 0 || !pageHasItemID(trashPage, item.ID) {
		t.Fatalf("trash list=%+v", trashPage)
	}

	rec = postItem(h, cookie, restoreURL(item.ID), `{"version":2,"ignored":true}`)
	restored := assertItemOK(t, rec)
	if restored.Version != 3 || restored.DeletedAt != nil || restored.Name != "门磁传感器" || len(restored.ReturnTasks) != 0 {
		t.Fatalf("restore=%+v", restored)
	}
	assertNullJSON(t, rec.Body.String(), "deleted_at")
	if !strings.Contains(rec.Body.String(), `"return_tasks":[]`) {
		t.Fatalf("restore return_tasks=%s", rec.Body.String())
	}
	live := requireItem(t, h, cookie, item.ID)
	if live.Version != 3 || live.DeletedAt != nil {
		t.Fatalf("live after restore=%+v", live)
	}
	rec = getItem(h, cookie, trashURL(item.ID))
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	if !pageHasItemID(mustItemPage(t, getItem(h, cookie, "/api/v1/items")), item.ID) {
		t.Fatal("live list missing restored item")
	}
	if !pageHasItemID(mustItemPage(t, getItem(h, cookie, "/api/v1/items?q="+url.QueryEscape("门磁"))), item.ID) {
		t.Fatal("search missing restored item")
	}

	rec = deleteItem(h, cookie, itemURL(item.ID)+"?version=3")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("second trash status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = patchItem(h, cookie, itemURL(item.ID), `{"name":"被改掉","version":4}`)
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	gotTrash := requireTrash(t, h, cookie, item.ID)
	if gotTrash.Name != "门磁传感器" || gotTrash.Version != 4 {
		t.Fatalf("patch wrote trash=%+v", gotTrash)
	}
	rec = deleteItem(h, cookie, itemURL(item.ID)+"?version=4")
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	if requireTrash(t, h, cookie, item.ID).Version != 4 {
		t.Fatal("second live delete wrote")
	}

	stillLive := mustItem(t, h, cookie, `{"name":"仍在列表"}`)
	rec = postItem(h, cookie, restoreURL(stillLive.ID), `{"version":1}`)
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	if requireItem(t, h, cookie, stillLive.ID).Name != "仍在列表" {
		t.Fatal("restore live wrote")
	}
	rec = getItem(h, cookie, "/api/v1/items?deleted=1")
	assertInvalidFields(t, rec, map[string]string{"deleted": "不支持的参数"})
	rec = getItem(h, cookie, "/api/v1/items?trashed=1")
	assertInvalidFields(t, rec, map[string]string{"trashed": "不支持的参数"})
	rec = getItem(h, cookie, "/api/v1/items?include_deleted=1")
	assertInvalidFields(t, rec, map[string]string{"include_deleted": "不支持的参数"})

	rec = deleteItem(h, cookie, trashURL(item.ID)+"?version=4")
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("purge status=%d body=%q", rec.Code, rec.Body.String())
	}
	if countQuery(t, db, `SELECT COUNT(*) FROM items WHERE id = ?`, item.ID) != 0 {
		t.Fatal("purged item row remained")
	}
	if countQuery(t, db, `SELECT COUNT(*) FROM item_locations WHERE item_id = ?`, item.ID) != 0 {
		t.Fatal("purged item_locations remained")
	}
	if countQuery(t, db, `SELECT COUNT(*) FROM item_categories WHERE item_id = ?`, item.ID) != 0 {
		t.Fatal("purged item_categories remained")
	}
	if requireLoc(t, h, cookie, place.ID).Name != "抽屉" {
		t.Fatal("purge removed location")
	}
	if requireCat(t, h, cookie, cat.ID).Name != "传感器" {
		t.Fatal("purge removed category")
	}
	rec = postItem(h, cookie, restoreURL(item.ID), `{"version":4}`)
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	rec = deleteItem(h, cookie, trashURL(item.ID)+"?version=4")
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")

	named := mustItem(t, h, cookie, `{"name":"合法名"}`)
	rec = patchItem(h, cookie, itemURL(named.ID), `{"name":"新合法名","version":1}`)
	if assertItemOK(t, rec).Version != 2 {
		t.Fatal("rename")
	}
	rec = deleteItem(h, cookie, itemURL(named.ID)+"?version=1")
	assertError(t, rec, http.StatusConflict, "version_conflict", "记录已被修改")
	assertNoFieldsKey(t, rec)
	if !pageHasItemID(mustItemPage(t, getItem(h, cookie, "/api/v1/items")), named.ID) {
		t.Fatal("stale trash hid the item")
	}
	if requireItem(t, h, cookie, named.ID).Name != "新合法名" {
		t.Fatal("stale trash wrote")
	}
	rec = deleteItem(h, cookie, itemURL(named.ID)+"?version=2")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("first trash status=%d body=%s", rec.Code, rec.Body.String())
	}
	firstTrash := requireTrash(t, h, cookie, named.ID)
	if firstTrash.Version != 3 {
		t.Fatalf("first trash version=%d", firstTrash.Version)
	}
	rec = postItem(h, cookie, restoreURL(named.ID), `{"version":3}`)
	if assertItemOK(t, rec).Version != 4 {
		t.Fatal("restore after first trash")
	}
	rec = deleteItem(h, cookie, itemURL(named.ID)+"?version=4")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("second trash status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = postItem(h, cookie, restoreURL(named.ID), `{"version":3}`)
	assertError(t, rec, http.StatusConflict, "version_conflict", "记录已被修改")
	assertNoFieldsKey(t, rec)
	if requireTrash(t, h, cookie, named.ID).Version != 5 {
		t.Fatal("stale restore wrote")
	}
	rec = deleteItem(h, cookie, trashURL(named.ID)+"?version=3")
	assertError(t, rec, http.StatusConflict, "version_conflict", "记录已被修改")
	if requireTrash(t, h, cookie, named.ID).Version != 5 {
		t.Fatal("stale purge wrote")
	}

	rec = getItem(h, cookie, trashURL(named.ID)+"?foo=1")
	if rec.Code == http.StatusOK || rec.Code == http.StatusNotFound {
		t.Fatalf("trash query status=%d body=%s", rec.Code, rec.Body.String())
	}
	assertInvalidFields(t, rec, map[string]string{"foo": "不支持的参数"})
	rec = getItem(h, "", trashURL(named.ID)+"?foo=1")
	assertError(t, rec, http.StatusUnauthorized, "unauthenticated", "未登录")
	rec = request(h, http.MethodGet, trashURL(named.ID), "", "http://evil.example", testRemote, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("trash get checked origin: status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = request(h, http.MethodPost, "/api/v1/trash/abc/restore", `{`, "http://evil.example", testRemote, "")
	assertError(t, rec, http.StatusForbidden, "origin_rejected", "来源不被接受")
	assertNoFieldsKey(t, rec)
	for _, body := range []string{`[`, `[]`, `null`, `{`, `{"version":1}{"x":1}`} {
		rec = postItem(h, cookie, "/api/v1/trash/999999/restore", body)
		if rec.Code == http.StatusNotFound {
			t.Fatalf("invalid restore body returned 404: %s", rec.Body.String())
		}
		assertError(t, rec, http.StatusBadRequest, "invalid_body", "请求格式不正确")
		assertNoFieldsKey(t, rec)
	}
	rec = deleteItem(h, cookie, "/api/v1/trash/999999?version=abc")
	if rec.Code == http.StatusNotFound {
		t.Fatalf("bad trash version returned 404: %s", rec.Body.String())
	}
	assertInvalidFields(t, rec, map[string]string{"version": "版本不正确"})
	rec = deleteItem(h, cookie, "/api/v1/trash/abc?version=abc")
	if rec.Code == http.StatusNotFound {
		t.Fatalf("bad trash id version returned 404: %s", rec.Body.String())
	}
	assertInvalidFields(t, rec, map[string]string{"version": "版本不正确"})

	rec = postItem(h, cookie, "/api/v1/trash/999999/restore?foo=1", `{"version":1}`)
	if rec.Code == http.StatusNotFound {
		t.Fatalf("restore query returned 404: %s", rec.Body.String())
	}
	assertInvalidFields(t, rec, map[string]string{"foo": "不支持的参数"})
	rec = postItem(h, cookie, restoreURL(named.ID), `{}`)
	assertInvalidFields(t, rec, map[string]string{"version": "版本不正确"})
	rec = postItem(h, cookie, restoreURL(named.ID), `{"version":0}`)
	assertInvalidFields(t, rec, map[string]string{"version": "版本不正确"})
	if requireTrash(t, h, cookie, named.ID).Version != 5 {
		t.Fatal("bad restore version wrote")
	}

	rec = request(h, http.MethodPost, restoreURL(named.ID), oversizedItemBody(), webOrigin, testRemote, cookie)
	assertError(t, rec, http.StatusRequestEntityTooLarge, "body_too_large", "请求正文过大")
	rec = request(h, http.MethodPut, trashURL(named.ID), `{"name":"x"}`, webOrigin, testRemote, cookie)
	if rec.Code == http.StatusMethodNotAllowed {
		t.Fatalf("PUT trash returned 405: %s", rec.Body.String())
	}
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	rec = request(h, http.MethodGet, restoreURL(named.ID), "", "", testRemote, cookie)
	if rec.Code == http.StatusMethodNotAllowed {
		t.Fatalf("GET restore returned 405: %s", rec.Body.String())
	}
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")

	rec = getItem(h, "", "/api/v1/trash")
	assertError(t, rec, http.StatusUnauthorized, "unauthenticated", "未登录")
	rec = getItem(h, cookie, "/api/v1/trash?deleted=1")
	assertInvalidFields(t, rec, map[string]string{"deleted": "不支持的参数"})
	rec = getItem(h, cookie, "/api/v1/trash?limit=0")
	assertInvalidFields(t, rec, map[string]string{"limit": "数量超出范围"})
	rec = getItem(h, cookie, "/api/v1/trash?offset=-1")
	assertInvalidFields(t, rec, map[string]string{"offset": "起点不正确"})

	jia := mustItem(t, h, cookie, `{"name":"甲"}`)
	yi := mustItem(t, h, cookie, `{"name":"乙"}`)
	if deleteItem(h, cookie, itemURL(jia.ID)+"?version=1").Code != http.StatusNoContent {
		t.Fatal("trash 甲")
	}
	if deleteItem(h, cookie, itemURL(yi.ID)+"?version=1").Code != http.StatusNoContent {
		t.Fatal("trash 乙")
	}
	sorted := mustItemPage(t, getItem(h, cookie, "/api/v1/trash"))
	if sorted.Total != 3 || len(sorted.Data) != 3 || sorted.Data[0].Name != "乙" || sorted.Data[1].Name != "新合法名" || sorted.Data[2].Name != "甲" {
		t.Fatalf("trash sort=%+v", sorted)
	}
}

func assertDirectItemCountJSON(t *testing.T, rec *httptest.ResponseRecorder, got, want int) {
	t.Helper()
	if rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got != want {
		t.Fatalf("direct_item_count=%d want %d body=%s", got, want, rec.Body.String())
	}
	needle := fmt.Sprintf(`"direct_item_count":%d`, want)
	if !strings.Contains(rec.Body.String(), needle) {
		t.Fatalf("missing %s in %s", needle, rec.Body.String())
	}
}

func TestTrashOccupiesLocationAndCategory(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)

	emptyLocRec := postLoc(h, cookie, "/api/v1/locations", `{"name":"空位置","type":"area"}`)
	emptyLoc := assertCreated(t, emptyLocRec)
	assertDirectItemCountJSON(t, emptyLocRec, emptyLoc.DirectItemCount, 0)
	emptyCatRec := postCat(h, cookie, "/api/v1/categories", `{"name":"空分类"}`)
	if emptyCatRec.Code != http.StatusCreated {
		t.Fatalf("empty category status=%d body=%s", emptyCatRec.Code, emptyCatRec.Body.String())
	}
	emptyCat := decodeCat(t, emptyCatRec)
	assertDirectItemCountJSON(t, emptyCatRec, emptyCat.DirectItemCount, 0)

	place := mustCreate(t, h, cookie, `{"name":"抽屉","type":"movable","code":"014"}`)
	cat := mustCreateCat(t, h, cookie, `{"name":"传感器"}`)
	item := mustItem(t, h, cookie, fmt.Sprintf(`{"name":"门磁传感器","locations":[{"location_id":%d}],"categories":[{"category_id":%d}]}`, place.ID, cat.ID))

	rec := deleteItem(h, cookie, itemURL(item.ID)+"?version=1")
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("trash status=%d body=%q", rec.Code, rec.Body.String())
	}

	locRec := getLoc(h, cookie, locURL(place.ID))
	loc := decodeLoc(t, locRec)
	assertDirectItemCountJSON(t, locRec, loc.DirectItemCount, 1)
	catRec := getCat(h, cookie, catURL(cat.ID))
	gotCat := decodeCat(t, catRec)
	assertDirectItemCountJSON(t, catRec, gotCat.DirectItemCount, 1)

	rec = deleteLoc(h, cookie, locURL(place.ID)+"?version=1")
	assertError(t, rec, http.StatusConflict, "location_in_use", "这个位置下面还有内容")
	rec = deleteCat(h, cookie, catURL(cat.ID)+"?version=1")
	assertError(t, rec, http.StatusConflict, "category_in_use", "这个分类下面还有内容")

	rec = deleteItem(h, cookie, trashURL(item.ID)+"?version=2")
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("purge status=%d body=%q", rec.Code, rec.Body.String())
	}

	locRec = getLoc(h, cookie, locURL(place.ID))
	loc = decodeLoc(t, locRec)
	assertDirectItemCountJSON(t, locRec, loc.DirectItemCount, 0)
	catRec = getCat(h, cookie, catURL(cat.ID))
	gotCat = decodeCat(t, catRec)
	assertDirectItemCountJSON(t, catRec, gotCat.DirectItemCount, 0)

	rec = deleteLoc(h, cookie, locURL(place.ID)+"?version=1")
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("delete location status=%d body=%q", rec.Code, rec.Body.String())
	}
	rec = deleteCat(h, cookie, catURL(cat.ID)+"?version=1")
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("delete category status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestItemOptionalText(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)
	full := `{"name":"钳","alias":"尖嘴","model":"M1","spec":"S1","quantity_note":"两把","note":"备注"}`

	item := mustItem(t, h, cookie, full)
	rec := patchItem(h, cookie, itemURL(item.ID), `{"version":1,"model":null,"spec":null,"quantity_note":null,"note":null}`)
	cleared := assertItemOK(t, rec)
	if cleared.Version != 2 || cleared.Name != "钳" || cleared.Alias == nil || *cleared.Alias != "尖嘴" {
		t.Fatalf("null text=%+v", cleared)
	}
	for _, key := range []string{"model", "spec", "quantity_note", "note"} {
		assertNullJSON(t, rec.Body.String(), key)
	}
	gotRec := getItem(h, cookie, itemURL(item.ID))
	got := assertItemOK(t, gotRec)
	if got.Version != 2 || got.Model != nil || got.Spec != nil || got.QuantityNote != nil || got.Note != nil || got.Alias == nil || *got.Alias != "尖嘴" {
		t.Fatalf("read back null text=%+v", got)
	}
	for _, key := range []string{"model", "spec", "quantity_note", "note"} {
		assertNullJSON(t, gotRec.Body.String(), key)
	}

	spaced := mustItem(t, h, cookie, full)
	rec = patchItem(h, cookie, itemURL(spaced.ID), `{"version":1,"model":"  ","spec":"  ","quantity_note":"  ","note":"  "}`)
	blanked := assertItemOK(t, rec)
	if blanked.Version != 2 || blanked.Alias == nil || *blanked.Alias != "尖嘴" {
		t.Fatalf("blank text=%+v", blanked)
	}
	for _, key := range []string{"model", "spec", "quantity_note", "note"} {
		assertNullJSON(t, rec.Body.String(), key)
	}
	gotRec = getItem(h, cookie, itemURL(spaced.ID))
	got = assertItemOK(t, gotRec)
	if got.Model != nil || got.Spec != nil || got.QuantityNote != nil || got.Note != nil {
		t.Fatalf("read back blank text=%+v", got)
	}
	for _, key := range []string{"model", "spec", "quantity_note", "note"} {
		assertNullJSON(t, gotRec.Body.String(), key)
	}

	kept := mustItem(t, h, cookie, full)
	rec = patchItem(h, cookie, itemURL(kept.ID), `{"version":1,"name":"新名"}`)
	renamed := assertItemOK(t, rec)
	if renamed.Version != 2 || renamed.Name != "新名" || renamed.Alias == nil || *renamed.Alias != "尖嘴" || renamed.Model == nil || *renamed.Model != "M1" || renamed.Spec == nil || *renamed.Spec != "S1" || renamed.QuantityNote == nil || *renamed.QuantityNote != "两把" || renamed.Note == nil || *renamed.Note != "备注" {
		t.Fatalf("omitted text=%+v", renamed)
	}
	got = requireItem(t, h, cookie, kept.ID)
	if got.Name != "新名" || got.Version != 2 || got.Alias == nil || *got.Alias != "尖嘴" || got.Model == nil || *got.Model != "M1" || got.Spec == nil || *got.Spec != "S1" || got.QuantityNote == nil || *got.QuantityNote != "两把" || got.Note == nil || *got.Note != "备注" {
		t.Fatalf("read back omitted text=%+v", got)
	}
}

func TestItemLinkNoteNull(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)
	place := mustCreate(t, h, cookie, `{"name":"抽屉","type":"movable","code":"042"}`)

	rec := postItem(h, cookie, "/api/v1/items", fmt.Sprintf(`{"name":"省略说明","locations":[{"location_id":%d}]}`, place.ID))
	omitted := assertItemCreated(t, rec)
	if omitted.Locations[0].Note != nil || omitted.Locations[0].Path[len(omitted.Locations[0].Path)-1].ID != place.ID {
		t.Fatalf("omitted note=%+v", omitted.Locations[0])
	}
	assertLinkNoteNull(t, rec.Body.String())
	gotRec := getItem(h, cookie, itemURL(omitted.ID))
	got := assertItemOK(t, gotRec)
	if got.Locations[0].Note != nil {
		t.Fatalf("read back omitted note=%+v", got.Locations[0])
	}
	assertLinkNoteNull(t, gotRec.Body.String())

	rec = postItem(h, cookie, "/api/v1/items", fmt.Sprintf(`{"name":"空说明","locations":[{"location_id":%d,"note":null}]}`, place.ID))
	nullNote := assertItemCreated(t, rec)
	if nullNote.Locations[0].Note != nil {
		t.Fatalf("null note=%+v", nullNote.Locations[0])
	}
	assertLinkNoteNull(t, rec.Body.String())
	gotRec = getItem(h, cookie, itemURL(nullNote.ID))
	got = assertItemOK(t, gotRec)
	if got.Locations[0].Note != nil {
		t.Fatalf("read back null note=%+v", got.Locations[0])
	}
	assertLinkNoteNull(t, gotRec.Body.String())
}

func TestItemPathFields(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)
	living := mustCreate(t, h, cookie, `{"name":"客厅","type":"area"}`)
	cabinet := mustCreate(t, h, cookie, fmt.Sprintf(`{"name":"储物柜","type":"fixed","code":"002","parent_id":%d}`, living.ID))
	rec := postItem(h, cookie, "/api/v1/items", fmt.Sprintf(`{"name":"遥控车","locations":[{"location_id":%d}]}`, cabinet.ID))
	created := assertItemCreated(t, rec)
	assertItemPathFields(t, rec.Body.String(), living.ID, cabinet.ID)
	if created.Locations[0].LocationID != cabinet.ID || created.Locations[0].Path[len(created.Locations[0].Path)-1].ID != cabinet.ID {
		t.Fatalf("created path=%+v", created.Locations)
	}
	gotRec := getItem(h, cookie, itemURL(created.ID))
	got := assertItemOK(t, gotRec)
	assertItemPathFields(t, gotRec.Body.String(), living.ID, cabinet.ID)
	last := got.Locations[0].Path[len(got.Locations[0].Path)-1]
	if got.Locations[0].LocationID != cabinet.ID || last.ID != got.Locations[0].LocationID || last.Type != "fixed" || last.Code == nil || *last.Code != "002" {
		t.Fatalf("path=%+v", got.Locations[0])
	}
}

func TestItemSearchMatch(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)
	elec := mustCreateCat(t, h, cookie, `{"name":"电子配件"}`)
	sensor := mustCreateCat(t, h, cookie, fmt.Sprintf(`{"name":"传感器","parent_id":%d}`, elec.ID))
	onlyChild := mustItem(t, h, cookie, fmt.Sprintf(
		`{"name":"温湿度传感器","categories":[{"category_id":%d}]}`, sensor.ID))
	onlyParent := mustItem(t, h, cookie, fmt.Sprintf(
		`{"name":"配件盒","categories":[{"category_id":%d}]}`, elec.ID))

	all := mustItemPage(t, getItem(h, cookie, fmt.Sprintf(
		"/api/v1/items?category=%d,%d&category_match=all", elec.ID, sensor.ID)))
	if all.Total != 1 || len(all.Data) != 1 || all.Data[0].ID != onlyChild.ID {
		t.Fatalf("all descendants=%+v", all)
	}
	direct := mustItemPage(t, getItem(h, cookie, fmt.Sprintf(
		"/api/v1/items?category=%d,%d&category_match=all&category_descendants=0", elec.ID, sensor.ID)))
	if direct.Total != 0 || len(direct.Data) != 0 {
		t.Fatalf("all direct=%+v", direct)
	}
	anyPage := mustItemPage(t, getItem(h, cookie, fmt.Sprintf(
		"/api/v1/items?category=%d,%d", elec.ID, sensor.ID)))
	if anyPage.Total != 2 {
		t.Fatalf("any=%+v", anyPage)
	}
	parentOnly := mustItemPage(t, getItem(h, cookie, fmt.Sprintf("/api/v1/items?category=%d", elec.ID)))
	if parentOnly.Total != 2 {
		t.Fatalf("parent branch=%+v", parentOnly)
	}
	parentDirect := mustItemPage(t, getItem(h, cookie, fmt.Sprintf("/api/v1/items?category=%d&category_descendants=0", elec.ID)))
	if parentDirect.Total != 1 || len(parentDirect.Data) != 1 || parentDirect.Data[0].ID != onlyParent.ID {
		t.Fatalf("parent direct=%+v", parentDirect)
	}
	childDirect := mustItemPage(t, getItem(h, cookie, fmt.Sprintf("/api/v1/items?category=%d&category_descendants=0", sensor.ID)))
	if childDirect.Total != 1 || len(childDirect.Data) != 1 || childDirect.Data[0].ID != onlyChild.ID {
		t.Fatalf("child direct=%+v", childDirect)
	}
}

func TestItemSearchKeyword(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)
	pct := mustItem(t, h, cookie, `{"name":"100%"}`)
	under := mustItem(t, h, cookie, `{"name":"a_b"}`)
	usb := mustItem(t, h, cookie, `{"name":"USB-Cable"}`)
	noteItem := mustItem(t, h, cookie, `{"name":"备注件","note":"中文线索"}`)
	specItem := mustItem(t, h, cookie, `{"name":"规格件","spec":"中文线索"}`)

	page := mustItemPage(t, getItem(h, cookie, "/api/v1/items?q="+url.QueryEscape("100%")))
	if page.Total != 1 || len(page.Data) != 1 || page.Data[0].ID != pct.ID {
		t.Fatalf("percent=%+v", page)
	}
	page = mustItemPage(t, getItem(h, cookie, "/api/v1/items?q="+url.QueryEscape("a_b")))
	if page.Total != 1 || len(page.Data) != 1 || page.Data[0].ID != under.ID {
		t.Fatalf("underscore=%+v", page)
	}
	page = mustItemPage(t, getItem(h, cookie, "/api/v1/items?q="+url.QueryEscape("100")))
	if page.Total != 1 || len(page.Data) != 1 || page.Data[0].ID != pct.ID {
		t.Fatalf("hundred=%+v", page)
	}
	page = mustItemPage(t, getItem(h, cookie, "/api/v1/items?q="+url.QueryEscape("usb")))
	if page.Total != 1 || len(page.Data) != 1 || page.Data[0].ID != usb.ID {
		t.Fatalf("usb=%+v", page)
	}
	page = mustItemPage(t, getItem(h, cookie, "/api/v1/items?q="+url.QueryEscape("中文线索")))
	if page.Total != 1 || len(page.Data) != 1 || page.Data[0].ID != noteItem.ID {
		t.Fatalf("note=%+v", page)
	}
	_ = specItem
}

func TestItemSearchLocationRange(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)
	parent := mustCreate(t, h, cookie, `{"name":"客厅","type":"area"}`)
	child := mustCreate(t, h, cookie, fmt.Sprintf(`{"name":"抽屉","type":"fixed","code":"010","parent_id":%d}`, parent.ID))
	other := mustCreate(t, h, cookie, `{"name":"书房","type":"area"}`)
	nested := mustItem(t, h, cookie, fmt.Sprintf(`{"name":"温湿度传感器","locations":[{"location_id":%d}]}`, child.ID))
	elsewhere := mustItem(t, h, cookie, fmt.Sprintf(`{"name":"温湿度计","locations":[{"location_id":%d}]}`, other.ID))

	direct := mustItemPage(t, getItem(h, cookie, fmt.Sprintf("/api/v1/items?location=%d", parent.ID)))
	if direct.Total != 0 || len(direct.Data) != 0 {
		t.Fatalf("location parent=%+v", direct)
	}
	inRange := mustItemPage(t, getItem(h, cookie, fmt.Sprintf("/api/v1/items?in_location=%d", parent.ID)))
	if inRange.Total != 1 || len(inRange.Data) != 1 || inRange.Data[0].ID != nested.ID {
		t.Fatalf("in_location=%+v", inRange)
	}
	self := mustItemPage(t, getItem(h, cookie, fmt.Sprintf("/api/v1/items?in_location=%d&in_location_descendants=0", parent.ID)))
	if self.Total != 0 || len(self.Data) != 0 {
		t.Fatalf("in_location self=%+v", self)
	}
	keywordOnly := mustItemPage(t, getItem(h, cookie, "/api/v1/items?q="+url.QueryEscape("温湿度")))
	if keywordOnly.Total != 2 {
		t.Fatalf("keyword only=%+v", keywordOnly)
	}
	both := mustItemPage(t, getItem(h, cookie, fmt.Sprintf("/api/v1/items?in_location=%d&q=%s", parent.ID, url.QueryEscape("温湿度"))))
	if both.Total != 1 || len(both.Data) != 1 || both.Data[0].ID != nested.ID {
		t.Fatalf("keyword and range=%+v", both)
	}
	rec := getItem(h, cookie, "/api/v1/items?in_location=999999")
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	_ = elsewhere
}

func TestItemSearchQueryErrors(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)
	cases := []struct {
		raw    string
		fields map[string]string
	}{
		{"placement=unlocated&in_location=1", map[string]string{"placement": "不支持的参数", "in_location": "不支持的参数"}},
		{"location=1&in_location=1", map[string]string{"location": "不支持的参数", "in_location": "不支持的参数"}},
		{"uncategorized=1&category=1", map[string]string{"uncategorized": "不支持的参数", "category": "不支持的参数"}},
		{"category_match=all&category=1", map[string]string{"category_match": "不支持的参数"}},
		{"in_location_descendants=0", map[string]string{"in_location_descendants": "不支持的参数"}},
		{"category=1&category=2", map[string]string{"category": "不支持的参数"}},
		{"category=3,3", map[string]string{"category": "参数不正确"}},
	}
	for _, tc := range cases {
		rec := getItem(h, cookie, "/api/v1/items?"+tc.raw)
		assertInvalidFields(t, rec, tc.fields)
	}
}

func TestItemUncategorized(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)
	loose := mustItem(t, h, cookie, `{"name":"未分类物"}`)
	page := mustItemPage(t, getItem(h, cookie, "/api/v1/items?uncategorized=1"))
	if page.Total != 1 || len(page.Data) != 1 || page.Data[0].ID != loose.ID {
		t.Fatalf("uncategorized=%+v", page)
	}
	cat := mustCreateCat(t, h, cookie, `{"name":"电子配件"}`)
	rec := patchItem(h, cookie, itemURL(loose.ID), fmt.Sprintf(`{"version":1,"categories":[{"category_id":%d}]}`, cat.ID))
	if assertItemOK(t, rec).Version != 2 {
		t.Fatal("attach category")
	}
	page = mustItemPage(t, getItem(h, cookie, "/api/v1/items?uncategorized=1"))
	if page.Total != 0 || len(page.Data) != 0 {
		t.Fatalf("after attach=%+v", page)
	}

	place := mustCreate(t, h, cookie, `{"name":"抽屉","type":"movable","code":"014"}`)
	named := mustItem(t, h, cookie, `{"name":"温湿度传感器"}`)
	placed := mustItem(t, h, cookie, fmt.Sprintf(`{"name":"温湿度计","locations":[{"location_id":%d}]}`, place.ID))
	page = mustItemPage(t, getItem(h, cookie, "/api/v1/items?placement=unlocated&q="+url.QueryEscape("温湿度")))
	if page.Total != 1 || len(page.Data) != 1 || page.Data[0].ID != named.ID {
		t.Fatalf("unlocated keyword=%+v", page)
	}
	_ = placed
}

func assertItemPathFields(t *testing.T, body string, livingID, cabinetID int64) {
	t.Helper()
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &obj); err != nil {
		t.Fatalf("body=%s err=%v", body, err)
	}
	var links []struct {
		LocationID int64                        `json:"location_id"`
		Path       []map[string]json.RawMessage `json:"path"`
	}
	if err := json.Unmarshal(obj["locations"], &links); err != nil {
		t.Fatalf("locations=%s err=%v", obj["locations"], err)
	}
	if len(links) != 1 || links[0].LocationID != cabinetID || len(links[0].Path) != 2 {
		t.Fatalf("links=%s", obj["locations"])
	}
	for _, node := range links[0].Path {
		for _, key := range []string{"id", "name", "type", "code"} {
			if _, ok := node[key]; !ok {
				t.Fatalf("path missing %s: %s", key, node)
			}
		}
	}
	root := links[0].Path[0]
	if string(root["id"]) != fmt.Sprint(livingID) || string(root["name"]) != `"客厅"` || string(root["type"]) != `"area"` || string(root["code"]) != "null" {
		t.Fatalf("root=%s", root)
	}
	last := links[0].Path[1]
	if string(last["id"]) != fmt.Sprint(cabinetID) || string(last["name"]) != `"储物柜"` || string(last["type"]) != `"fixed"` || string(last["code"]) != `"002"` || string(last["id"]) != fmt.Sprint(links[0].LocationID) {
		t.Fatalf("last=%s", last)
	}
}

func TestDetailQuery(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)
	loc := mustCreate(t, h, cookie, `{"name":"厨房","type":"area"}`)
	item := mustItem(t, h, cookie, `{"name":"温湿度传感器"}`)

	cases := []struct {
		path string
		ok   string
	}{
		{locURL(loc.ID), "/api/v1/locations/abc"},
		{itemURL(item.ID), "/api/v1/items/abc"},
	}
	for _, tc := range cases {
		rec := getLoc(h, cookie, tc.path+"?foo=1")
		if rec.Code == http.StatusOK || rec.Code == http.StatusNotFound {
			t.Fatalf("%s status=%d body=%s", tc.path, rec.Code, rec.Body.String())
		}
		assertInvalidFields(t, rec, map[string]string{"foo": "不支持的参数"})

		rec = getLoc(h, cookie, tc.path+"?foo=1&bar=2")
		assertInvalidFields(t, rec, map[string]string{"foo": "不支持的参数", "bar": "不支持的参数"})

		rec = getLoc(h, cookie, tc.ok+"?foo=1")
		if rec.Code == http.StatusNotFound {
			t.Fatalf("bad id returned 404 before query: %s", rec.Body.String())
		}
		assertInvalidFields(t, rec, map[string]string{"foo": "不支持的参数"})

		rec = getLoc(h, "", tc.path+"?foo=1")
		assertError(t, rec, http.StatusUnauthorized, "unauthenticated", "未登录")
	}

	if requireLoc(t, h, cookie, loc.ID).Name != "厨房" {
		t.Fatal("location query changed the row")
	}
	if requireItem(t, h, cookie, item.ID).Name != "温湿度传感器" {
		t.Fatal("item query changed the row")
	}
}

func TestWritePathIDOrder(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)

	type step struct {
		method, path, body, origin, cookie string
		status                             int
		code, message                      string
		fields                             map[string]string
	}
	checks := []step{
		{http.MethodPatch, "/api/v1/locations/abc", `{`, "http://evil.example", "", http.StatusForbidden, "origin_rejected", "来源不被接受", nil},
		{http.MethodPatch, "/api/v1/locations/abc", `{`, webOrigin, cookie, http.StatusBadRequest, "invalid_body", "请求格式不正确", nil},
		{http.MethodPatch, "/api/v1/locations/abc", `{"name":"客厅","version":1}`, webOrigin, cookie, http.StatusNotFound, "not_found", "未找到", nil},
		{http.MethodDelete, "/api/v1/locations/abc", "", "http://evil.example", "", http.StatusForbidden, "origin_rejected", "来源不被接受", nil},
		{http.MethodDelete, "/api/v1/locations/abc?version=abc", "", webOrigin, cookie, http.StatusBadRequest, "invalid_fields", "有字段不符合要求", map[string]string{"version": "版本不正确"}},
		{http.MethodDelete, "/api/v1/locations/abc?version=1", "", webOrigin, cookie, http.StatusNotFound, "not_found", "未找到", nil},
		{http.MethodPatch, "/api/v1/items/abc", `{`, "http://evil.example", "", http.StatusForbidden, "origin_rejected", "来源不被接受", nil},
		{http.MethodPatch, "/api/v1/items/abc", `{`, webOrigin, cookie, http.StatusBadRequest, "invalid_body", "请求格式不正确", nil},
		{http.MethodPatch, "/api/v1/items/abc", `{"name":"传感器","version":1}`, webOrigin, cookie, http.StatusNotFound, "not_found", "未找到", nil},
		{http.MethodDelete, "/api/v1/items/abc", "", "http://evil.example", "", http.StatusForbidden, "origin_rejected", "来源不被接受", nil},
		{http.MethodDelete, "/api/v1/items/abc?version=abc", "", webOrigin, cookie, http.StatusBadRequest, "invalid_fields", "有字段不符合要求", map[string]string{"version": "版本不正确"}},
		{http.MethodDelete, "/api/v1/items/abc?version=1", "", webOrigin, cookie, http.StatusNotFound, "not_found", "未找到", nil},
	}
	for _, tc := range checks {
		rec := request(h, tc.method, tc.path, tc.body, tc.origin, testRemote, tc.cookie)
		if tc.fields != nil {
			if rec.Code == http.StatusNotFound {
				t.Fatalf("%s %s returned 404: %s", tc.method, tc.path, rec.Body.String())
			}
			assertInvalidFields(t, rec, tc.fields)
			continue
		}
		assertError(t, rec, tc.status, tc.code, tc.message)
		if tc.code == "invalid_body" || tc.code == "origin_rejected" {
			assertNoFieldsKey(t, rec)
		}
	}
}

type catPathNode struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type catBody struct {
	ID              int64         `json:"id"`
	Name            string        `json:"name"`
	ParentID        *int64        `json:"parent_id"`
	Version         int64         `json:"version"`
	CreatedAt       string        `json:"created_at"`
	UpdatedAt       string        `json:"updated_at"`
	Path            []catPathNode `json:"path"`
	DirectItemCount int           `json:"direct_item_count"`
}

type catPageBody struct {
	Data   []catBody `json:"data"`
	Total  int       `json:"total"`
	Limit  int       `json:"limit"`
	Offset int       `json:"offset"`
}

func postCat(h http.Handler, cookie, path, body string) *httptest.ResponseRecorder {
	return request(h, http.MethodPost, path, body, webOrigin, testRemote, cookie)
}

func getCat(h http.Handler, cookie, path string) *httptest.ResponseRecorder {
	return request(h, http.MethodGet, path, "", "", testRemote, cookie)
}

func patchCat(h http.Handler, cookie, path, body string) *httptest.ResponseRecorder {
	return request(h, http.MethodPatch, path, body, webOrigin, testRemote, cookie)
}

func deleteCat(h http.Handler, cookie, path string) *httptest.ResponseRecorder {
	return request(h, http.MethodDelete, path, "", webOrigin, testRemote, cookie)
}

func catURL(id int64) string {
	return fmt.Sprintf("/api/v1/categories/%d", id)
}

func oversizedCatBody() string {
	return `{"name":"` + strings.Repeat("a", 32769) + `"}`
}

func decodeCat(t *testing.T, rec *httptest.ResponseRecorder) catBody {
	t.Helper()
	var cat catBody
	if err := json.Unmarshal(rec.Body.Bytes(), &cat); err != nil {
		t.Fatalf("body=%s err=%v", rec.Body.String(), err)
	}
	checkCatPath(t, rec.Body.Bytes(), cat)
	return cat
}

func checkCatPath(t *testing.T, raw []byte, cat catBody) {
	t.Helper()
	if cat.Path == nil {
		t.Fatalf("path is null: %s", raw)
	}
	if len(cat.Path) == 0 || cat.Path[len(cat.Path)-1].ID != cat.ID {
		t.Fatalf("path last id: %+v body=%s", cat.Path, raw)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("body=%s err=%v", raw, err)
	}
	pathRaw, ok := obj["path"]
	if !ok || string(pathRaw) == "null" {
		t.Fatalf("path is null: %s", raw)
	}
	var nodes []map[string]json.RawMessage
	if err := json.Unmarshal(pathRaw, &nodes); err != nil {
		t.Fatalf("path=%s err=%v", pathRaw, err)
	}
	for _, node := range nodes {
		if _, hasType := node["type"]; hasType {
			t.Fatalf("path has type: %s", raw)
		}
		if _, hasCode := node["code"]; hasCode {
			t.Fatalf("path has code: %s", raw)
		}
	}
}

func mustCreateCat(t *testing.T, h http.Handler, cookie, body string) catBody {
	t.Helper()
	rec := postCat(h, cookie, "/api/v1/categories", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	cat := decodeCat(t, rec)
	if cat.ID < 1 || cat.Version != 1 || cat.CreatedAt != frozenAt || cat.UpdatedAt != frozenAt {
		t.Fatalf("category=%+v body=%s", cat, rec.Body.String())
	}
	return cat
}

func requireCat(t *testing.T, h http.Handler, cookie string, id int64) catBody {
	t.Helper()
	rec := getCat(h, cookie, catURL(id))
	if rec.Code != http.StatusOK {
		t.Fatalf("get %d status=%d body=%s", id, rec.Code, rec.Body.String())
	}
	return decodeCat(t, rec)
}

func decodeCatPage(t *testing.T, rec *httptest.ResponseRecorder) catPageBody {
	t.Helper()
	if strings.Contains(rec.Body.String(), `"data":null`) {
		t.Fatalf("data is null: %s", rec.Body.String())
	}
	var page catPageBody
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("body=%s err=%v", rec.Body.String(), err)
	}
	if page.Data == nil {
		t.Fatalf("data is null: %s", rec.Body.String())
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &obj); err != nil {
		t.Fatalf("body=%s err=%v", rec.Body.String(), err)
	}
	var rawItems []json.RawMessage
	if err := json.Unmarshal(obj["data"], &rawItems); err != nil {
		t.Fatalf("data=%s err=%v", obj["data"], err)
	}
	if len(rawItems) != len(page.Data) {
		t.Fatalf("data len=%d decoded=%d", len(rawItems), len(page.Data))
	}
	for i, raw := range rawItems {
		checkCatPath(t, raw, page.Data[i])
	}
	return page
}

func mustCatPage(t *testing.T, rec *httptest.ResponseRecorder) catPageBody {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	return decodeCatPage(t, rec)
}

func TestCategoryNameTaken(t *testing.T) {
	db, h, _ := testHandler(t, false)
	cookie := login(t, h)

	rec := postCat(h, cookie, "/api/v1/categories", `{"name":"电子配件"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	elec := decodeCat(t, rec)
	assertNullJSON(t, rec.Body.String(), "parent_id")
	if elec.Name != "电子配件" || elec.ParentID != nil || elec.Version != 1 {
		t.Fatalf("root=%+v", elec)
	}
	if len(elec.Path) != 1 || elec.Path[0].ID != elec.ID || elec.Path[0].Name != "电子配件" {
		t.Fatalf("path=%+v", elec.Path)
	}

	rec = postCat(h, cookie, "/api/v1/categories", `{"name":"  电子配件  "}`)
	assertError(t, rec, http.StatusConflict, "name_taken", "同级已有相同名称")
	if countQuery(t, db, `SELECT COUNT(*) FROM categories`) != 1 {
		t.Fatal("duplicate root wrote a row")
	}

	home := mustCreateCat(t, h, cookie, `{"name":"智能家居"}`)
	sensor := mustCreateCat(t, h, cookie, fmt.Sprintf(`{"name":"传感器","parent_id":%d}`, elec.ID))
	if sensor.ParentID == nil || *sensor.ParentID != elec.ID || len(sensor.Path) != 2 {
		t.Fatalf("sensor=%+v", sensor)
	}
	if sensor.Path[0].ID != elec.ID || sensor.Path[1].ID != sensor.ID || sensor.Path[1].Name != "传感器" {
		t.Fatalf("sensor path=%+v", sensor.Path)
	}
	other := mustCreateCat(t, h, cookie, fmt.Sprintf(`{"name":"传感器","parent_id":%d}`, home.ID))
	if other.ParentID == nil || *other.ParentID != home.ID {
		t.Fatalf("other=%+v", other)
	}

	rec = patchCat(h, cookie, catURL(other.ID), fmt.Sprintf(`{"version":1,"parent_id":%d}`, elec.ID))
	assertError(t, rec, http.StatusConflict, "name_taken", "同级已有相同名称")
	got := requireCat(t, h, cookie, other.ID)
	if got.ParentID == nil || *got.ParentID != home.ID || got.Version != 1 || got.Name != "传感器" {
		t.Fatalf("moved sibling=%+v", got)
	}

	rec = patchCat(h, cookie, catURL(elec.ID), `{"version":1,"name":"智能家居"}`)
	assertError(t, rec, http.StatusConflict, "name_taken", "同级已有相同名称")
	got = requireCat(t, h, cookie, elec.ID)
	if got.Name != "电子配件" || got.Version != 1 || got.ParentID != nil {
		t.Fatalf("renamed root=%+v", got)
	}

	rec = patchCat(h, cookie, catURL(sensor.ID), `{"version":1,"name":"传感器"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	same := decodeCat(t, rec)
	if same.Version != 2 || same.Name != "传感器" || same.ParentID == nil || *same.ParentID != elec.ID {
		t.Fatalf("same name=%+v", same)
	}
}

func TestCategoryCycle(t *testing.T) {
	db, h, _ := testHandler(t, false)
	cookie := login(t, h)
	root := mustCreateCat(t, h, cookie, `{"name":"电子配件"}`)
	child := mustCreateCat(t, h, cookie, fmt.Sprintf(`{"name":"传感器","parent_id":%d}`, root.ID))

	rec := patchCat(h, cookie, catURL(root.ID), fmt.Sprintf(`{"version":1,"parent_id":%d}`, child.ID))
	assertError(t, rec, http.StatusConflict, "category_cycle", "不能移到自己的下级")
	got := requireCat(t, h, cookie, root.ID)
	if got.ParentID != nil || got.Version != 1 || got.Name != "电子配件" {
		t.Fatalf("after cycle=%+v", got)
	}

	before := countQuery(t, db, `SELECT COUNT(*) FROM categories`)
	rec = postCat(h, cookie, "/api/v1/categories", `{"name":"x","parent_id":999999}`)
	assertError(t, rec, http.StatusBadRequest, "invalid_parent", "不能放在这个父级下")
	if countQuery(t, db, `SELECT COUNT(*) FROM categories`) != before {
		t.Fatal("invalid parent wrote a row")
	}
}

func TestCategoryDelete(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)
	root := mustCreateCat(t, h, cookie, `{"name":"电子配件"}`)
	child := mustCreateCat(t, h, cookie, fmt.Sprintf(`{"name":"传感器","parent_id":%d}`, root.ID))

	rec := deleteCat(h, cookie, catURL(root.ID)+"?version=1")
	assertError(t, rec, http.StatusConflict, "category_in_use", "这个分类下面还有内容")
	got := requireCat(t, h, cookie, root.ID)
	if got.Version != 1 || got.Name != "电子配件" {
		t.Fatalf("parent deleted=%+v", got)
	}
	if requireCat(t, h, cookie, child.ID).Name != "传感器" {
		t.Fatal("child deleted with parent")
	}

	rec = deleteCat(h, cookie, catURL(child.ID)+"?version=1")
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	rec = getCat(h, cookie, catURL(child.ID))
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	if requireCat(t, h, cookie, root.ID).Version != 1 {
		t.Fatal("parent changed")
	}

	linked := mustCreateCat(t, h, cookie, `{"name":"智能家居"}`)
	item := mustItem(t, h, cookie, fmt.Sprintf(`{"name":"温湿度传感器","categories":[{"category_id":%d}]}`, linked.ID))
	rec = deleteCat(h, cookie, catURL(linked.ID)+"?version=1")
	assertError(t, rec, http.StatusConflict, "category_in_use", "这个分类下面还有内容")
	got = requireCat(t, h, cookie, linked.ID)
	if got.Name != "智能家居" || got.Version != 1 {
		t.Fatalf("in-use category=%+v", got)
	}
	still := requireItem(t, h, cookie, item.ID)
	if still.Name != "温湿度传感器" || len(still.Categories) != 1 || still.Categories[0].CategoryID != linked.ID {
		t.Fatalf("item changed=%+v", still)
	}
}

func TestCategoryAuth(t *testing.T) {
	db, h, _ := testHandler(t, false)
	cookie := login(t, h)
	if countQuery(t, db, `SELECT COUNT(*) FROM categories`) != 0 {
		t.Fatal("categories already present")
	}

	rec := postCat(h, "", "/api/v1/categories", `{"name":"未登录"}`)
	assertError(t, rec, http.StatusUnauthorized, "unauthenticated", "未登录")
	if countQuery(t, db, `SELECT COUNT(*) FROM categories`) != 0 {
		t.Fatal("unauthenticated create wrote a row")
	}

	rec = request(h, http.MethodPost, "/api/v1/categories", `{"name":"坏来源"}`, "http://evil.example", testRemote, cookie)
	assertError(t, rec, http.StatusForbidden, "origin_rejected", "来源不被接受")
	rec = request(h, http.MethodPost, "/api/v1/categories", `{"name":"坏来源"}`, "http://evil.example", testRemote, "")
	assertError(t, rec, http.StatusForbidden, "origin_rejected", "来源不被接受")
	if countQuery(t, db, `SELECT COUNT(*) FROM categories`) != 0 {
		t.Fatal("rejected origin wrote a row")
	}

	rec = request(h, http.MethodPatch, "/api/v1/categories/abc", `{"name":"x","version":1}`, "http://evil.example", testRemote, "")
	assertError(t, rec, http.StatusForbidden, "origin_rejected", "来源不被接受")

	big := oversizedCatBody()
	rec = postCat(h, cookie, "/api/v1/categories", big)
	assertError(t, rec, http.StatusRequestEntityTooLarge, "body_too_large", "请求正文过大")
	if countQuery(t, db, `SELECT COUNT(*) FROM categories`) != 0 {
		t.Fatal("oversized request wrote a row")
	}
}

func TestCategoryDetailQuery(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)
	cat := mustCreateCat(t, h, cookie, `{"name":"电子配件"}`)

	rec := getCat(h, cookie, catURL(cat.ID)+"?foo=1")
	if rec.Code == http.StatusOK || rec.Code == http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	assertInvalidFields(t, rec, map[string]string{"foo": "不支持的参数"})

	rec = getCat(h, "", catURL(cat.ID)+"?foo=1")
	assertError(t, rec, http.StatusUnauthorized, "unauthenticated", "未登录")
}

func TestCategoryIDReuse(t *testing.T) {
	db, h, _ := testHandler(t, false)
	cookie := login(t, h)
	first := mustCreateCat(t, h, cookie, `{"name":"旧分类"}`)
	maxID := countQuery(t, db, `SELECT MAX(id) FROM categories`)
	if int64(maxID) != first.ID {
		t.Fatalf("max=%d category=%d", maxID, first.ID)
	}
	rec := deleteCat(h, cookie, catURL(first.ID)+"?version=1")
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	next := mustCreateCat(t, h, cookie, `{"name":"新分类"}`)
	if next.ID == first.ID || next.Version != 1 || next.Name != "新分类" {
		t.Fatalf("reused id=%d next=%+v", first.ID, next)
	}
	rec = patchCat(h, cookie, catURL(first.ID), `{"name":"被改掉","version":1}`)
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	rec = deleteCat(h, cookie, catURL(first.ID)+"?version=1")
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	got := requireCat(t, h, cookie, next.ID)
	if got.Name != "新分类" || got.Version != 1 {
		t.Fatalf("new category changed=%+v", got)
	}
}

func TestCategoryVersion(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)
	root := mustCreateCat(t, h, cookie, `{"name":"电子配件"}`)
	if root.Version != 1 {
		t.Fatalf("create version=%d", root.Version)
	}
	home := mustCreateCat(t, h, cookie, `{"name":"智能家居"}`)
	child := mustCreateCat(t, h, cookie, fmt.Sprintf(`{"name":"传感器","parent_id":%d}`, root.ID))

	rec := patchCat(h, cookie, catURL(root.ID), `{"version":1,"name":"电子配件"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	same := decodeCat(t, rec)
	if same.Version != 2 || same.Name != "电子配件" || same.ParentID != nil {
		t.Fatalf("same name=%+v", same)
	}

	rec = patchCat(h, cookie, catURL(root.ID), `{"name":"起居电器","version":1}`)
	assertError(t, rec, http.StatusConflict, "version_conflict", "记录已被修改")
	assertNoFieldsKey(t, rec)
	got := requireCat(t, h, cookie, root.ID)
	if got.Name != "电子配件" || got.Version != 2 || got.ParentID != nil {
		t.Fatalf("stale name wrote=%+v", got)
	}

	rec = patchCat(h, cookie, catURL(root.ID), fmt.Sprintf(`{"version":1,"parent_id":%d}`, child.ID))
	assertError(t, rec, http.StatusConflict, "version_conflict", "记录已被修改")
	if strings.Contains(rec.Body.String(), "category_cycle") {
		t.Fatalf("stale cycle=%s", rec.Body.String())
	}
	got = requireCat(t, h, cookie, root.ID)
	if got.ParentID != nil || got.Version != 2 || got.Name != "电子配件" {
		t.Fatalf("stale cycle wrote=%+v", got)
	}

	rec = patchCat(h, cookie, catURL(root.ID), `{"version":1,"name":"智能家居"}`)
	assertError(t, rec, http.StatusConflict, "version_conflict", "记录已被修改")
	if strings.Contains(rec.Body.String(), "name_taken") {
		t.Fatalf("stale name taken=%s", rec.Body.String())
	}
	got = requireCat(t, h, cookie, root.ID)
	if got.Name != "电子配件" || got.Version != 2 || got.ParentID != nil {
		t.Fatalf("stale sibling wrote=%+v", got)
	}
	if requireCat(t, h, cookie, home.ID).Name != "智能家居" {
		t.Fatal("sibling changed")
	}
}

func TestCategoryLists(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)
	elec := mustCreateCat(t, h, cookie, `{"name":"电子配件"}`)
	home := mustCreateCat(t, h, cookie, `{"name":"智能家居"}`)
	sensor := mustCreateCat(t, h, cookie, fmt.Sprintf(`{"name":"传感器","parent_id":%d}`, elec.ID))
	leaf := mustCreateCat(t, h, cookie, fmt.Sprintf(`{"name":"温湿度","parent_id":%d}`, sensor.ID))

	roots := mustCatPage(t, getCat(h, cookie, "/api/v1/categories"))
	if roots.Total != 2 || roots.Limit != 30 || roots.Offset != 0 || len(roots.Data) != 2 {
		t.Fatalf("roots=%+v", roots)
	}
	if roots.Data[0].ID != home.ID || roots.Data[0].Name != "智能家居" || roots.Data[1].ID != elec.ID || roots.Data[1].Name != "电子配件" {
		t.Fatalf("root order=%+v", roots.Data)
	}

	children := mustCatPage(t, getCat(h, cookie, fmt.Sprintf("/api/v1/categories?parent=%d", elec.ID)))
	if children.Total != 1 || len(children.Data) != 1 || children.Data[0].ID != sensor.ID {
		t.Fatalf("children=%+v", children.Data)
	}
	if children.Data[0].ParentID == nil || *children.Data[0].ParentID != elec.ID {
		t.Fatalf("child parent=%+v", children.Data[0])
	}

	flat := mustCatPage(t, getCat(h, cookie, "/api/v1/categories?flat=1"))
	if flat.Total != 4 || len(flat.Data) != 4 {
		t.Fatalf("flat=%+v", flat.Data)
	}
	if flat.Data[0].ID != elec.ID || flat.Data[1].ID != home.ID || flat.Data[2].ID != sensor.ID || flat.Data[3].ID != leaf.ID {
		t.Fatalf("flat order=%+v", flat.Data)
	}

	eligible := mustCatPage(t, getCat(h, cookie, fmt.Sprintf("/api/v1/categories?eligible_parent=1&exclude=%d", elec.ID)))
	if eligible.Total != 1 || len(eligible.Data) != 1 || eligible.Data[0].ID != home.ID {
		t.Fatalf("eligible=%+v", eligible.Data)
	}
	for _, cat := range eligible.Data {
		if cat.ID == elec.ID || cat.ID == sensor.ID || cat.ID == leaf.ID {
			t.Fatalf("excluded id returned: %+v", cat)
		}
	}

	rec := getCat(h, cookie, fmt.Sprintf("/api/v1/categories?parent=%d&flat=1", elec.ID))
	assertInvalidFields(t, rec, map[string]string{"parent": "不支持的参数", "flat": "不支持的参数"})
	rec = getCat(h, cookie, fmt.Sprintf("/api/v1/categories?exclude=%d", elec.ID))
	assertInvalidFields(t, rec, map[string]string{"exclude": "不支持的参数"})
}

func itemReturnTasksURL(id int64) string {
	return fmt.Sprintf("/api/v1/items/%d/return-tasks", id)
}

func returnTaskURL(id int64) string {
	return fmt.Sprintf("/api/v1/return-tasks/%d", id)
}

func completeReturnTaskURL(id int64) string {
	return fmt.Sprintf("/api/v1/return-tasks/%d/complete", id)
}

func decodeReturnTask(t *testing.T, rec *httptest.ResponseRecorder) returnTaskBody {
	t.Helper()
	if !strings.Contains(rec.Body.String(), `"cover_photo":`) {
		t.Fatalf("cover_photo key missing: %s", rec.Body.String())
	}
	var task returnTaskBody
	if err := json.Unmarshal(rec.Body.Bytes(), &task); err != nil {
		t.Fatalf("body=%s err=%v", rec.Body.String(), err)
	}
	if task.ID < 1 {
		t.Fatalf("task=%+v body=%s", task, rec.Body.String())
	}
	if task.CoverPhoto == nil && !strings.Contains(rec.Body.String(), `"cover_photo":null`) {
		t.Fatalf("cover_photo is not null: %s", rec.Body.String())
	}
	return task
}

func assertTaskCreated(t *testing.T, rec *httptest.ResponseRecorder) returnTaskBody {
	t.Helper()
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	task := decodeReturnTask(t, rec)
	if task.Version != 1 || task.CreatedAt != frozenAt || task.UpdatedAt != frozenAt || task.CompletedAt != nil {
		t.Fatalf("task=%+v body=%s", task, rec.Body.String())
	}
	return task
}

func assertTaskOK(t *testing.T, rec *httptest.ResponseRecorder) returnTaskBody {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	return decodeReturnTask(t, rec)
}

type returnTaskPageBody struct {
	Data   []returnTaskBody `json:"data"`
	Total  int              `json:"total"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}

func mustReturnTaskPage(t *testing.T, rec *httptest.ResponseRecorder) returnTaskPageBody {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"data":null`) {
		t.Fatalf("data is null: %s", rec.Body.String())
	}
	var page returnTaskPageBody
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("body=%s err=%v", rec.Body.String(), err)
	}
	if page.Data == nil {
		t.Fatalf("data is null: %s", rec.Body.String())
	}
	return page
}

func pageHasTaskID(page returnTaskPageBody, id int64) bool {
	for _, task := range page.Data {
		if task.ID == id {
			return true
		}
	}
	return false
}

func TestReturnTasks(t *testing.T) {
	db, h, _ := testHandler(t, false)
	cookie := login(t, h)

	empty := mustReturnTaskPage(t, getItem(h, cookie, "/api/v1/return-tasks"))
	if empty.Total != 0 || len(empty.Data) != 0 || empty.Limit != 30 || empty.Offset != 0 {
		t.Fatalf("empty list=%+v", empty)
	}

	locA := mustCreate(t, h, cookie, `{"name":"位置甲","type":"movable","code":"101"}`)
	locB := mustCreate(t, h, cookie, `{"name":"位置乙","type":"movable","code":"102"}`)
	car := mustItem(t, h, cookie, fmt.Sprintf(`{"name":"遥控车","locations":[{"location_id":%d}]}`, locA.ID))
	clip := mustItem(t, h, cookie, `{"name":"钳"}`)
	if countQuery(t, db, `SELECT COUNT(*) FROM return_tasks`) != 0 {
		t.Fatal("return_tasks already present")
	}

	createPath := itemReturnTasksURL(car.ID)
	rec := request(h, http.MethodPost, createPath, `{}`, webOrigin, testRemote, "")
	assertError(t, rec, http.StatusUnauthorized, "unauthenticated", "未登录")
	rec = request(h, http.MethodPost, createPath, `{}`, "http://evil.example", testRemote, cookie)
	assertError(t, rec, http.StatusForbidden, "origin_rejected", "来源不被接受")
	rec = request(h, http.MethodPost, createPath, `{}`, "http://evil.example", testRemote, "")
	assertError(t, rec, http.StatusForbidden, "origin_rejected", "来源不被接受")
	if countQuery(t, db, `SELECT COUNT(*) FROM return_tasks`) != 0 {
		t.Fatal("rejected create wrote a row")
	}

	rec = request(h, http.MethodPost, "/api/v1/return-tasks/abc/complete", `{"version":1}`, "http://evil.example", testRemote, cookie)
	assertError(t, rec, http.StatusForbidden, "origin_rejected", "来源不被接受")
	assertNoFieldsKey(t, rec)

	longNote := strings.Repeat("一", 201)
	rec = postItem(h, cookie, "/api/v1/items/999999/return-tasks", fmt.Sprintf(`{"part_note":"%s"}`, longNote))
	if rec.Code == http.StatusBadRequest {
		t.Fatalf("missing item long part_note returned 400: %s", rec.Body.String())
	}
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	rec = postItem(h, cookie, "/api/v1/items/abc/return-tasks", fmt.Sprintf(`{"part_note":"%s"}`, longNote))
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	rec = postItem(h, cookie, "/api/v1/items/999999/return-tasks", `{`)
	if rec.Code == http.StatusNotFound {
		t.Fatalf("bad json on missing item returned 404: %s", rec.Body.String())
	}
	assertError(t, rec, http.StatusBadRequest, "invalid_body", "请求格式不正确")
	assertNoFieldsKey(t, rec)
	if countQuery(t, db, `SELECT COUNT(*) FROM return_tasks`) != 0 {
		t.Fatal("invalid create wrote a row")
	}

	rec = request(h, http.MethodPost, createPath, oversizedItemBody(), webOrigin, testRemote, cookie)
	assertError(t, rec, http.StatusRequestEntityTooLarge, "body_too_large", "请求正文过大")
	if countQuery(t, db, `SELECT COUNT(*) FROM return_tasks`) != 0 {
		t.Fatal("oversized create wrote a row")
	}

	rec = getItem(h, cookie, "/api/v1/return-tasks/1?foo=1")
	if rec.Code == http.StatusOK || rec.Code == http.StatusNotFound {
		t.Fatalf("task query status=%d body=%s", rec.Code, rec.Body.String())
	}
	assertInvalidFields(t, rec, map[string]string{"foo": "不支持的参数"})
	rec = getItem(h, "", "/api/v1/return-tasks/1?foo=1")
	assertError(t, rec, http.StatusUnauthorized, "unauthenticated", "未登录")
	rec = getItem(h, "", "/api/v1/return-tasks")
	assertError(t, rec, http.StatusUnauthorized, "unauthenticated", "未登录")

	rec = postItem(h, cookie, createPath+"?foo=1", `{}`)
	if rec.Code == http.StatusNotFound || rec.Code == http.StatusCreated {
		t.Fatalf("create query status=%d body=%s", rec.Code, rec.Body.String())
	}
	assertInvalidFields(t, rec, map[string]string{"foo": "不支持的参数"})
	if countQuery(t, db, `SELECT COUNT(*) FROM return_tasks`) != 0 {
		t.Fatal("create with query wrote a row")
	}

	for _, body := range []string{`{"part_note":1}`, `{"reason":true}`, `{"destination_note":[]}`, `{"part_note":{"x":1}}`, `[]`, `null`} {
		rec = postItem(h, cookie, itemReturnTasksURL(clip.ID), body)
		if rec.Code == http.StatusCreated || rec.Code == http.StatusNotFound {
			t.Fatalf("wrong type %s status=%d body=%s", body, rec.Code, rec.Body.String())
		}
		assertError(t, rec, http.StatusBadRequest, "invalid_body", "请求格式不正确")
		assertNoFieldsKey(t, rec)
	}
	if countQuery(t, db, `SELECT COUNT(*) FROM return_tasks`) != 0 {
		t.Fatal("type error wrote a row")
	}

	rec = postItem(h, cookie, itemReturnTasksURL(clip.ID), `{}`)
	omitted := assertTaskCreated(t, rec)
	if omitted.ItemID != clip.ID || omitted.ItemName != "钳" || omitted.PartNote != nil || omitted.Reason != nil || omitted.DestinationNote != nil {
		t.Fatalf("omitted=%+v", omitted)
	}
	for _, key := range []string{"part_note", "reason", "destination_note", "completed_at"} {
		assertNullJSON(t, rec.Body.String(), key)
	}
	rec = postItem(h, cookie, itemReturnTasksURL(clip.ID), `{"part_note":null,"reason":null,"destination_note":null}`)
	nulled := assertTaskCreated(t, rec)
	if nulled.PartNote != nil || nulled.Reason != nil || nulled.DestinationNote != nil {
		t.Fatalf("null fields=%+v", nulled)
	}
	rec = postItem(h, cookie, itemReturnTasksURL(clip.ID), `{"part_note":"  ","reason":"  ","destination_note":"  "}`)
	blank := assertTaskCreated(t, rec)
	if blank.PartNote != nil || blank.Reason != nil || blank.DestinationNote != nil {
		t.Fatalf("blank fields=%+v", blank)
	}
	rec = postItem(h, cookie, itemReturnTasksURL(clip.ID), "{\"destination_note\":\"楼上\\n楼下\"}")
	withBreak := assertTaskCreated(t, rec)
	if withBreak.DestinationNote == nil || *withBreak.DestinationNote != "楼上\n楼下" {
		t.Fatalf("newline dest=%+v", withBreak)
	}
	if countQuery(t, db, `SELECT COUNT(*) FROM return_tasks WHERE part_note = '' OR reason = '' OR destination_note = ''`) != 0 {
		t.Fatal("stored empty string instead of NULL")
	}

	rec = postItem(h, cookie, itemReturnTasksURL(clip.ID), fmt.Sprintf(`{"part_note":"%s"}`, longNote))
	assertInvalidFields(t, rec, map[string]string{"part_note": "配件说明过长"})
	rec = postItem(h, cookie, itemReturnTasksURL(clip.ID), `{"part_note":"a\u0000b"}`)
	assertInvalidFields(t, rec, map[string]string{"part_note": "配件说明不能包含控制字符"})
	rec = postItem(h, cookie, itemReturnTasksURL(clip.ID), `{"reason":"a\u0000b"}`)
	assertInvalidFields(t, rec, map[string]string{"reason": "原因不能包含控制字符"})
	rec = postItem(h, cookie, itemReturnTasksURL(clip.ID), `{"destination_note":"a\tb"}`)
	assertInvalidFields(t, rec, map[string]string{"destination_note": "临时去向不能包含控制字符"})

	beforeCar := requireItem(t, h, cookie, car.ID)
	rec = postItem(h, cookie, itemReturnTasksURL(car.ID), `{"reason":"借出","version":99}`)
	whole := assertTaskCreated(t, rec)
	if whole.ItemID != car.ID || whole.ItemName != "遥控车" || whole.PartNote != nil || whole.Reason == nil || *whole.Reason != "借出" || whole.DestinationNote != nil || whole.Version != 1 {
		t.Fatalf("whole=%+v", whole)
	}
	assertNullJSON(t, rec.Body.String(), "part_note")
	rec = postItem(h, cookie, itemReturnTasksURL(car.ID), `{"part_note":"充电器","destination_note":"放在老王家"}`)
	charger := assertTaskCreated(t, rec)
	if charger.ID <= whole.ID || charger.PartNote == nil || *charger.PartNote != "充电器" || charger.DestinationNote == nil || *charger.DestinationNote != "放在老王家" || charger.Reason != nil || charger.Version != 1 {
		t.Fatalf("charger=%+v", charger)
	}
	afterCreate := requireItem(t, h, cookie, car.ID)
	if afterCreate.Version != beforeCar.Version || afterCreate.UpdatedAt != beforeCar.UpdatedAt {
		t.Fatalf("create changed item version/updated_at before=%+v after=%+v", beforeCar, afterCreate)
	}
	if len(afterCreate.Locations) != 1 || afterCreate.Locations[0].LocationID != locA.ID {
		t.Fatalf("create changed locations=%+v", afterCreate.Locations)
	}
	if len(afterCreate.ReturnTasks) != 2 || afterCreate.ReturnTasks[0].ID != whole.ID || afterCreate.ReturnTasks[1].ID != charger.ID {
		t.Fatalf("item tasks after create=%+v", afterCreate.ReturnTasks)
	}
	if afterCreate.ReturnTasks[0].CompletedAt != nil || afterCreate.ReturnTasks[1].CompletedAt != nil {
		t.Fatalf("new tasks completed=%+v", afterCreate.ReturnTasks)
	}

	listPage := mustItemPage(t, getItem(h, cookie, "/api/v1/items"))
	var listedCar itemBody
	for _, it := range listPage.Data {
		if it.ID == car.ID {
			listedCar = it
		}
	}
	if listedCar.ID != car.ID || len(listedCar.ReturnTasks) != 2 || listedCar.ReturnTasks[0].ID != whole.ID || listedCar.ReturnTasks[1].ID != charger.ID {
		t.Fatalf("list item tasks=%+v", listedCar.ReturnTasks)
	}

	openBefore := mustReturnTaskPage(t, getItem(h, cookie, "/api/v1/return-tasks"))
	if openBefore.Limit != 30 || openBefore.Offset != 0 || openBefore.Total < 2 {
		t.Fatalf("open before complete=%+v", openBefore)
	}
	if !pageHasTaskID(openBefore, whole.ID) || !pageHasTaskID(openBefore, charger.ID) {
		t.Fatalf("open before missing tasks=%+v", openBefore.Data)
	}
	for i := 1; i < len(openBefore.Data); i++ {
		prev, cur := openBefore.Data[i-1], openBefore.Data[i]
		if prev.CreatedAt > cur.CreatedAt || (prev.CreatedAt == cur.CreatedAt && prev.ID > cur.ID) {
			t.Fatalf("open list not sorted created_at,id: %+v", openBefore.Data)
		}
		if cur.CompletedAt != nil {
			t.Fatalf("open list has completed=%+v", cur)
		}
	}

	rec = postItem(h, cookie, completeReturnTaskURL(whole.ID)+"?foo=1", `{"version":1}`)
	if rec.Code == http.StatusOK || rec.Code == http.StatusNotFound {
		t.Fatalf("complete query status=%d body=%s", rec.Code, rec.Body.String())
	}
	assertInvalidFields(t, rec, map[string]string{"foo": "不支持的参数"})
	rec = postItem(h, cookie, completeReturnTaskURL(whole.ID), `{}`)
	assertInvalidFields(t, rec, map[string]string{"version": "版本不正确"})
	rec = postItem(h, cookie, completeReturnTaskURL(whole.ID), `{"version":0}`)
	assertInvalidFields(t, rec, map[string]string{"version": "版本不正确"})
	rec = postItem(h, cookie, completeReturnTaskURL(whole.ID), `{"version":"1"}`)
	assertError(t, rec, http.StatusBadRequest, "invalid_body", "请求格式不正确")
	assertNoFieldsKey(t, rec)

	snapshot := requireItem(t, h, cookie, car.ID)
	rec = postItem(h, cookie, completeReturnTaskURL(whole.ID), `{"version":1,"ignored":true}`)
	done := assertTaskOK(t, rec)
	if done.ID != whole.ID || done.Version != 2 || done.CompletedAt == nil || *done.CompletedAt != frozenAt || done.CreatedAt != whole.CreatedAt || done.PartNote != nil || done.Reason == nil || *done.Reason != "借出" {
		t.Fatalf("completed whole=%+v", done)
	}
	afterComplete := requireItem(t, h, cookie, car.ID)
	if afterComplete.Version != snapshot.Version || afterComplete.UpdatedAt != snapshot.UpdatedAt {
		t.Fatalf("complete changed item version/updated_at before=%+v after=%+v", snapshot, afterComplete)
	}
	if len(afterComplete.Locations) != 1 || afterComplete.Locations[0].LocationID != locA.ID {
		t.Fatalf("complete changed locations=%+v", afterComplete.Locations)
	}
	if len(afterComplete.ReturnTasks) != 2 {
		t.Fatalf("item tasks after complete=%+v", afterComplete.ReturnTasks)
	}
	if afterComplete.ReturnTasks[0].ID != charger.ID || afterComplete.ReturnTasks[0].CompletedAt != nil || afterComplete.ReturnTasks[0].Version != 1 {
		t.Fatalf("incomplete should be first=%+v", afterComplete.ReturnTasks)
	}
	if afterComplete.ReturnTasks[1].ID != whole.ID || afterComplete.ReturnTasks[1].CompletedAt == nil || afterComplete.ReturnTasks[1].Version != 2 {
		t.Fatalf("completed should be last=%+v", afterComplete.ReturnTasks)
	}
	gotCharger := assertTaskOK(t, getItem(h, cookie, returnTaskURL(charger.ID)))
	if gotCharger.Version != 1 || gotCharger.CompletedAt != nil || gotCharger.PartNote == nil || *gotCharger.PartNote != "充电器" || gotCharger.CreatedAt != charger.CreatedAt || gotCharger.UpdatedAt != charger.UpdatedAt {
		t.Fatalf("other task changed=%+v", gotCharger)
	}
	openAfter := mustReturnTaskPage(t, getItem(h, cookie, "/api/v1/return-tasks"))
	if pageHasTaskID(openAfter, whole.ID) {
		t.Fatalf("completed task still in open list=%+v", openAfter.Data)
	}
	if !pageHasTaskID(openAfter, charger.ID) {
		t.Fatalf("charger missing from open list=%+v", openAfter.Data)
	}
	gotWhole := assertTaskOK(t, getItem(h, cookie, returnTaskURL(whole.ID)))
	if gotWhole.Version != 2 || gotWhole.CompletedAt == nil || *gotWhole.CompletedAt != frozenAt {
		t.Fatalf("get completed=%+v", gotWhole)
	}

	rec = patchItem(h, cookie, itemURL(car.ID), `{"name":"遥控车改名","version":1}`)
	renamed := assertItemOK(t, rec)
	if renamed.Name != "遥控车改名" || renamed.Version != 2 {
		t.Fatalf("rename=%+v", renamed)
	}
	named := assertTaskOK(t, getItem(h, cookie, returnTaskURL(charger.ID)))
	if named.ItemName != "遥控车改名" {
		t.Fatalf("item_name not current=%+v", named)
	}
	if len(renamed.ReturnTasks) != 2 || renamed.ReturnTasks[0].ItemName != "遥控车改名" || renamed.ReturnTasks[1].ItemName != "遥控车改名" {
		t.Fatalf("item tasks item_name=%+v", renamed.ReturnTasks)
	}

	rec = patchItem(h, cookie, itemURL(car.ID), fmt.Sprintf(`{"version":2,"locations":[{"location_id":%d}]}`, locB.ID))
	moved := assertItemOK(t, rec)
	if moved.Version != 3 || len(moved.Locations) != 1 || moved.Locations[0].LocationID != locB.ID {
		t.Fatalf("moved=%+v", moved)
	}
	stillCharger := assertTaskOK(t, getItem(h, cookie, returnTaskURL(charger.ID)))
	if stillCharger.DestinationNote == nil || *stillCharger.DestinationNote != "放在老王家" || stillCharger.ID != charger.ID || stillCharger.CompletedAt != nil {
		t.Fatalf("task after move=%+v", stillCharger)
	}
	rec = deleteLoc(h, cookie, locURL(locA.ID)+"?version=1")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete empty A status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = deleteLoc(h, cookie, locURL(locB.ID)+"?version=1")
	assertError(t, rec, http.StatusConflict, "location_in_use", "这个位置下面还有内容")
	if requireLoc(t, h, cookie, locB.ID).Name != "位置乙" {
		t.Fatal("in-use B removed")
	}

	firstCompletedAt := *gotWhole.CompletedAt
	rec = postItem(h, cookie, completeReturnTaskURL(whole.ID), `{"version":2}`)
	assertError(t, rec, http.StatusConflict, "already_completed", "这条事项已经完成")
	assertNoFieldsKey(t, rec)
	again := assertTaskOK(t, getItem(h, cookie, returnTaskURL(whole.ID)))
	if again.CompletedAt == nil || *again.CompletedAt != firstCompletedAt || again.Version != 2 {
		t.Fatalf("already completed wrote=%+v", again)
	}
	rec = deleteItem(h, cookie, returnTaskURL(whole.ID)+"?version=2")
	assertError(t, rec, http.StatusConflict, "already_completed", "这条事项已经完成")
	assertNoFieldsKey(t, rec)
	if countQuery(t, db, `SELECT COUNT(*) FROM return_tasks WHERE id = ?`, whole.ID) != 1 {
		t.Fatal("delete completed removed the row")
	}
	rec = postItem(h, cookie, completeReturnTaskURL(whole.ID), `{"version":1}`)
	assertError(t, rec, http.StatusConflict, "version_conflict", "记录已被修改")
	assertNoFieldsKey(t, rec)
	if strings.Contains(rec.Body.String(), "already_completed") {
		t.Fatalf("stale complete returned already_completed: %s", rec.Body.String())
	}
	staleDone := assertTaskOK(t, getItem(h, cookie, returnTaskURL(whole.ID)))
	if staleDone.CompletedAt == nil || *staleDone.CompletedAt != firstCompletedAt || staleDone.Version != 2 {
		t.Fatalf("stale complete wrote=%+v", staleDone)
	}
	rec = postItem(h, cookie, completeReturnTaskURL(charger.ID), `{"version":99}`)
	assertError(t, rec, http.StatusConflict, "version_conflict", "记录已被修改")
	assertNoFieldsKey(t, rec)
	if strings.Contains(rec.Body.String(), "already_completed") {
		t.Fatalf("stale incomplete complete returned already_completed: %s", rec.Body.String())
	}
	if assertTaskOK(t, getItem(h, cookie, returnTaskURL(charger.ID))).CompletedAt != nil {
		t.Fatal("stale complete completed the charger")
	}

	beforeDelete := requireItem(t, h, cookie, car.ID)
	rec = postItem(h, cookie, itemReturnTasksURL(car.ID), `{"part_note":"遥控手柄"}`)
	extra := assertTaskCreated(t, rec)
	if extra.Version != 1 {
		t.Fatalf("new task version=%d", extra.Version)
	}
	afterExtra := requireItem(t, h, cookie, car.ID)
	if afterExtra.Version != beforeDelete.Version || afterExtra.UpdatedAt != beforeDelete.UpdatedAt {
		t.Fatalf("extra create changed item=%+v", afterExtra)
	}
	rec = deleteItem(h, cookie, returnTaskURL(extra.ID)+"?version=1")
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("delete incomplete status=%d body=%q", rec.Code, rec.Body.String())
	}
	afterExtraDelete := requireItem(t, h, cookie, car.ID)
	if afterExtraDelete.Version != beforeDelete.Version || afterExtraDelete.UpdatedAt != beforeDelete.UpdatedAt {
		t.Fatalf("delete task changed item version/updated_at before=%+v after=%+v", beforeDelete, afterExtraDelete)
	}
	if countQuery(t, db, `SELECT COUNT(*) FROM return_tasks WHERE id = ?`, extra.ID) != 0 {
		t.Fatal("deleted incomplete task remained")
	}
	rec = postItem(h, cookie, completeReturnTaskURL(extra.ID), `{"version":1}`)
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	rec = deleteItem(h, cookie, returnTaskURL(extra.ID)+"?version=1")
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")

	rec = postItem(h, cookie, itemReturnTasksURL(car.ID), `{"part_note":"待删最大"}`)
	maxTask := assertTaskCreated(t, rec)
	var maxID int64
	if err := db.QueryRow(`SELECT MAX(id) FROM return_tasks`).Scan(&maxID); err != nil {
		t.Fatal(err)
	}
	if maxTask.ID != maxID {
		t.Fatalf("created id %d max %d", maxTask.ID, maxID)
	}
	if deleteItem(h, cookie, returnTaskURL(maxTask.ID)+"?version=1").Code != http.StatusNoContent {
		t.Fatal("delete max task")
	}
	rec = postItem(h, cookie, itemReturnTasksURL(car.ID), `{"part_note":"新建"}`)
	recreated := assertTaskCreated(t, rec)
	if recreated.ID == maxID {
		t.Fatalf("reused deleted task id %d", recreated.ID)
	}
	rec = postItem(h, cookie, completeReturnTaskURL(maxID), `{"version":1}`)
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	rec = deleteItem(h, cookie, returnTaskURL(maxID)+"?version=1")
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	keptNew := assertTaskOK(t, getItem(h, cookie, returnTaskURL(recreated.ID)))
	if keptNew.PartNote == nil || *keptNew.PartNote != "新建" || keptNew.CompletedAt != nil {
		t.Fatalf("new task changed after deleted-id writes=%+v", keptNew)
	}
	if deleteItem(h, cookie, returnTaskURL(recreated.ID)+"?version=1").Code != http.StatusNoContent {
		t.Fatal("cleanup recreated")
	}

	taskCount := countQuery(t, db, `SELECT COUNT(*) FROM return_tasks WHERE item_id = ?`, car.ID)
	live := requireItem(t, h, cookie, car.ID)
	rec = deleteItem(h, cookie, itemURL(car.ID)+fmt.Sprintf("?version=%d", live.Version))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("trash car status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = postItem(h, cookie, itemReturnTasksURL(car.ID), `{"part_note":"回收站后"}`)
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	rec = postItem(h, cookie, itemReturnTasksURL(car.ID), fmt.Sprintf(`{"part_note":"%s"}`, longNote))
	if rec.Code == http.StatusBadRequest {
		t.Fatalf("trashed item long part_note returned 400: %s", rec.Body.String())
	}
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	rec = postItem(h, cookie, completeReturnTaskURL(charger.ID), `{"version":1}`)
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	rec = deleteItem(h, cookie, returnTaskURL(charger.ID)+"?version=1")
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	rec = getItem(h, cookie, returnTaskURL(charger.ID))
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	rec = getItem(h, cookie, returnTaskURL(whole.ID))
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	if countQuery(t, db, `SELECT COUNT(*) FROM return_tasks WHERE item_id = ?`, car.ID) != taskCount {
		t.Fatal("trash writes changed return_tasks")
	}
	hidden := mustReturnTaskPage(t, getItem(h, cookie, "/api/v1/return-tasks"))
	if pageHasTaskID(hidden, charger.ID) || pageHasTaskID(hidden, whole.ID) {
		t.Fatalf("trashed item tasks in open list=%+v", hidden.Data)
	}
	trashedRec := getItem(h, cookie, trashURL(car.ID))
	trashed := assertItemOK(t, trashedRec)
	if trashed.DeletedAt == nil || len(trashed.ReturnTasks) != 2 {
		t.Fatalf("trash item tasks=%+v", trashed.ReturnTasks)
	}
	if trashed.ReturnTasks[0].ID != charger.ID || trashed.ReturnTasks[0].CompletedAt != nil {
		t.Fatalf("trash incomplete first=%+v", trashed.ReturnTasks)
	}
	if trashed.ReturnTasks[1].ID != whole.ID || trashed.ReturnTasks[1].CompletedAt == nil {
		t.Fatalf("trash completed last=%+v", trashed.ReturnTasks)
	}

	rec = postItem(h, cookie, restoreURL(car.ID), fmt.Sprintf(`{"version":%d}`, trashed.Version))
	restored := assertItemOK(t, rec)
	if restored.DeletedAt != nil {
		t.Fatalf("restore deleted_at=%v", restored.DeletedAt)
	}
	if len(restored.ReturnTasks) != 2 || restored.ReturnTasks[0].ID != charger.ID || restored.ReturnTasks[0].CompletedAt != nil {
		t.Fatalf("restore tasks=%+v", restored.ReturnTasks)
	}
	if restored.ReturnTasks[1].ID != whole.ID || restored.ReturnTasks[1].CompletedAt == nil {
		t.Fatalf("restore completed=%+v", restored.ReturnTasks)
	}
	shown := mustReturnTaskPage(t, getItem(h, cookie, "/api/v1/return-tasks"))
	if !pageHasTaskID(shown, charger.ID) {
		t.Fatalf("restored incomplete missing from open list=%+v", shown.Data)
	}
	if pageHasTaskID(shown, whole.ID) {
		t.Fatalf("completed task in open list after restore=%+v", shown.Data)
	}
	if assertTaskOK(t, getItem(h, cookie, returnTaskURL(whole.ID))).CompletedAt == nil {
		t.Fatal("completed task reopened after restore")
	}

	rec = request(h, http.MethodPut, returnTaskURL(charger.ID), `{"version":1}`, webOrigin, testRemote, cookie)
	if rec.Code == http.StatusMethodNotAllowed {
		t.Fatalf("PUT task returned 405: %s", rec.Body.String())
	}
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	rec = request(h, http.MethodGet, itemReturnTasksURL(car.ID), "", "", testRemote, cookie)
	if rec.Code == http.StatusMethodNotAllowed {
		t.Fatalf("GET item return-tasks returned 405: %s", rec.Body.String())
	}
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	rec = getItem(h, cookie, "/api/v1/return-tasks?deleted=1")
	assertInvalidFields(t, rec, map[string]string{"deleted": "不支持的参数"})
	rec = getItem(h, cookie, "/api/v1/return-tasks?limit=0")
	assertInvalidFields(t, rec, map[string]string{"limit": "数量超出范围"})
	limited := mustReturnTaskPage(t, getItem(h, cookie, "/api/v1/return-tasks?limit=1"))
	if limited.Limit != 1 || len(limited.Data) != 1 || limited.Total < 1 {
		t.Fatalf("limit 1=%+v", limited)
	}
}

func TestPhotoUploadAndRead(t *testing.T) {
	db, h, _, dataDir := testHandlerDir(t, false)
	cookie := login(t, h)
	item := mustItem(t, h, cookie, `{"name":"遥控车"}`)
	taskRec := postItem(h, cookie, itemReturnTasksURL(item.ID), `{}`)
	task := assertTaskCreated(t, taskRec)
	if !strings.Contains(taskRec.Body.String(), `"cover_photo":null`) {
		t.Fatalf("cover_photo before photos: %s", taskRec.Body.String())
	}

	jpegBytes := encodeTestJPEG(t, 20, 10)
	pngBytes := encodeTestPNG(t, 8, 8)
	webpBytes := oneByOneWebP()
	sources := [][]byte{jpegBytes, pngBytes, webpBytes}
	uploaded := make([]photoBody, 0, 3)
	for i, src := range sources {
		rec := postPhoto(t, h, cookie, itemPhotosURL(item.ID), src)
		p := assertPhotoCreated(t, rec)
		if p.ItemID != item.ID || p.Position != i {
			t.Fatalf("upload %d photo=%+v", i, p)
		}
		origPath := filepath.Join(dataDir, "originals", fmt.Sprintf("%d.jpg", p.ID))
		thumbPath := filepath.Join(dataDir, "thumbnails", fmt.Sprintf("%d.jpg", p.ID))
		st, err := os.Stat(origPath)
		if err != nil {
			t.Fatalf("original %d: %v", p.ID, err)
		}
		if _, err := os.Stat(thumbPath); err != nil {
			t.Fatalf("thumbnail %d: %v", p.ID, err)
		}
		if p.ByteSize != st.Size() {
			t.Fatalf("byte_size=%d file=%d", p.ByteSize, st.Size())
		}
		assertPhotoJPEG(t, request(h, http.MethodGet, photoThumbURL(p.ID), "", "", testRemote, cookie))
		origRec := request(h, http.MethodGet, photoOriginalURL(p.ID), "", "", testRemote, cookie)
		assertPhotoJPEG(t, origRec)
		want, err := os.ReadFile(origPath)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(origRec.Body.Bytes(), want) {
			t.Fatalf("original bytes mismatch id=%d", p.ID)
		}
		uploaded = append(uploaded, p)
		if i == 0 {
			gotTask := assertTaskOK(t, getItem(h, cookie, returnTaskURL(task.ID)))
			if gotTask.CoverPhoto == nil || gotTask.CoverPhoto.ID != p.ID {
				t.Fatalf("cover_photo after first=%+v want id=%d body=%s", gotTask.CoverPhoto, p.ID, getItem(h, cookie, returnTaskURL(task.ID)).Body.String())
			}
		}
	}

	got := requireItem(t, h, cookie, item.ID)
	if got.Version != 1 {
		t.Fatalf("item version=%d", got.Version)
	}
	if len(got.Photos) != 3 {
		t.Fatalf("photos=%+v", got.Photos)
	}
	for i, p := range uploaded {
		if got.Photos[i].ID != p.ID || got.Photos[i].Position != i {
			t.Fatalf("order i=%d got=%+v want=%+v", i, got.Photos[i], p)
		}
	}
	if countQuery(t, db, `SELECT COUNT(*) FROM photos`) != 3 {
		t.Fatal("photo rows")
	}
}

func TestPhotoUploadErrors(t *testing.T) {
	db, h, _, _ := testHandlerDir(t, false)
	cookie := login(t, h)
	item := mustItem(t, h, cookie, `{"name":"锚点"}`)
	path := itemPhotosURL(item.ID)

	heic := []byte{
		0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p',
		'h', 'e', 'i', 'c', 0x00, 0x00, 0x00, 0x00,
		'm', 'i', 'f', '1', 'h', 'e', 'i', 'c',
	}
	rec := postPhoto(t, h, cookie, path, heic)
	assertInvalidFields(t, rec, map[string]string{"file": "暂不支持这种照片格式"})

	rec = postPhotoParts(t, h, cookie, path, webOrigin, false, nil)
	assertInvalidFields(t, rec, map[string]string{"file": "没有照片文件"})

	rec = postPhoto(t, h, cookie, path, []byte("not-an-image"))
	assertInvalidFields(t, rec, map[string]string{"file": "不是可用的照片"})

	rec = postPhoto(t, h, cookie, path, jpegSOF(10001, 5001))
	assertInvalidFields(t, rec, map[string]string{"file": "照片像素过多"})
	if countQuery(t, db, `SELECT COUNT(*) FROM photos`) != 0 {
		t.Fatal("invalid uploads wrote a row")
	}

	rec = postPhoto(t, h, "", path, encodeTestJPEG(t, 8, 8))
	assertError(t, rec, http.StatusUnauthorized, "unauthenticated", "未登录")
	if countQuery(t, db, `SELECT COUNT(*) FROM photos`) != 0 {
		t.Fatal("unauthenticated upload wrote a row")
	}

	rec = request(h, http.MethodGet, photoThumbURL(1), "", "", testRemote, "")
	assertError(t, rec, http.StatusUnauthorized, "unauthenticated", "未登录")

	rec = postPhotoParts(t, h, cookie, path, "http://evil.example", true, encodeTestJPEG(t, 8, 8))
	assertError(t, rec, http.StatusForbidden, "origin_rejected", "来源不被接受")
	if countQuery(t, db, `SELECT COUNT(*) FROM photos`) != 0 {
		t.Fatal("origin rejected upload wrote a row")
	}

	rec = postPhotoParts(t, h, cookie, "/api/v1/items/abc/photos", "http://evil.example", true, encodeTestJPEG(t, 8, 8))
	assertError(t, rec, http.StatusForbidden, "origin_rejected", "来源不被接受")

	rec = requestWithType(h, http.MethodPost, path, `{}`, webOrigin, cookie, "application/json")
	assertError(t, rec, http.StatusBadRequest, "invalid_body", "请求格式不正确")

	rec = postPhoto(t, h, cookie, itemPhotosURL(999999), []byte("not-an-image"))
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")

	rec = postPhoto(t, h, cookie, path+"?foo=1", encodeTestJPEG(t, 8, 8))
	assertInvalidFields(t, rec, map[string]string{"foo": "不支持的参数"})

	rec = request(h, http.MethodGet, photoThumbURL(1)+"?foo=1", "", "", testRemote, cookie)
	assertInvalidFields(t, rec, map[string]string{"foo": "不支持的参数"})
	rec = request(h, http.MethodGet, photoThumbURL(1)+"?foo=1", "", "", testRemote, "")
	assertError(t, rec, http.StatusUnauthorized, "unauthenticated", "未登录")

	created := assertPhotoCreated(t, postPhoto(t, h, cookie, path, encodeTestJPEG(t, 12, 8)))
	rec = patchItem(h, cookie, itemURL(item.ID), `{"photos":[],"version":1,"name":"锚点"}`)
	got := assertItemOK(t, rec)
	if len(got.Photos) != 1 || got.Photos[0].ID != created.ID {
		t.Fatalf("patch photos cleared=%+v", got.Photos)
	}
}

func TestPhotoUploadTooLarge(t *testing.T) {
	db, h, _, _ := testHandlerDir(t, false)
	cookie := login(t, h)
	item := mustItem(t, h, cookie, `{"name":"超大"}`)
	path := itemPhotosURL(item.ID)

	rec := postOversize(h, cookie, path, webOrigin)
	assertError(t, rec, http.StatusRequestEntityTooLarge, "body_too_large", "请求正文过大")
	if countQuery(t, db, `SELECT COUNT(*) FROM photos`) != 0 {
		t.Fatal("oversized upload wrote a row")
	}

	rec = postOversize(h, cookie, path, "http://evil.example")
	assertError(t, rec, http.StatusForbidden, "origin_rejected", "来源不被接受")
	if rec.Code == http.StatusRequestEntityTooLarge {
		t.Fatal("bad origin returned 413")
	}

	rec = postOversize(h, "", path, webOrigin)
	assertError(t, rec, http.StatusUnauthorized, "unauthenticated", "未登录")
	if rec.Code == http.StatusRequestEntityTooLarge {
		t.Fatal("unauthenticated returned 413")
	}
	if countQuery(t, db, `SELECT COUNT(*) FROM photos`) != 0 {
		t.Fatal("rejected oversized wrote a row")
	}
}

func TestPhotoFirstAndDelete(t *testing.T) {
	db, h, _, dataDir := testHandlerDir(t, false)
	cookie := login(t, h)
	item := mustItem(t, h, cookie, `{"name":"遥控车"}`)
	task := assertTaskCreated(t, postItem(h, cookie, itemReturnTasksURL(item.ID), `{}`))
	jpegBytes := encodeTestJPEG(t, 8, 8)
	first := assertPhotoCreated(t, postPhoto(t, h, cookie, itemPhotosURL(item.ID), jpegBytes))
	second := assertPhotoCreated(t, postPhoto(t, h, cookie, itemPhotosURL(item.ID), jpegBytes))
	if first.Position != 0 || first.Version != 1 || second.Position != 1 || second.Version != 1 {
		t.Fatalf("uploaded first=%+v second=%+v", first, second)
	}

	rec := postItem(h, cookie, photoFirstURL(second.ID), `{"version":0}`)
	if rec.Code == http.StatusConflict {
		t.Fatalf("version 0 set first returned 409: %s", rec.Body.String())
	}
	assertInvalidFields(t, rec, map[string]string{"version": "版本不正确"})
	rec = postItem(h, cookie, photoFirstURL(second.ID), `{}`)
	if rec.Code == http.StatusConflict {
		t.Fatalf("missing version set first returned 409: %s", rec.Body.String())
	}
	assertInvalidFields(t, rec, map[string]string{"version": "版本不正确"})
	rec = deleteItem(h, cookie, photoURL(second.ID)+"?version=0")
	if rec.Code == http.StatusNotFound {
		t.Fatalf("version 0 delete returned 404: %s", rec.Body.String())
	}
	assertInvalidFields(t, rec, map[string]string{"version": "版本不正确"})
	unchanged := requireItem(t, h, cookie, item.ID)
	if len(unchanged.Photos) != 2 || unchanged.Photos[0].ID != first.ID || unchanged.Photos[0].Version != 1 || unchanged.Photos[1].ID != second.ID || unchanged.Photos[1].Version != 1 {
		t.Fatalf("invalid version wrote=%+v", unchanged.Photos)
	}
	assertPhotoFiles(t, dataDir, first.ID, true)
	assertPhotoFiles(t, dataDir, second.ID, true)

	rec = postItem(h, cookie, photoFirstURL(second.ID), `{"version":1}`)
	moved := assertPhotoOK(t, rec)
	if moved.ID != second.ID || moved.Position != 0 || moved.Version != 2 {
		t.Fatalf("set first=%+v", moved)
	}
	got := requireItem(t, h, cookie, item.ID)
	if got.Version != 1 {
		t.Fatalf("item version=%d", got.Version)
	}
	if len(got.Photos) != 2 || got.Photos[0].ID != second.ID || got.Photos[0].Position != 0 || got.Photos[1].ID != first.ID || got.Photos[1].Position != 1 {
		t.Fatalf("after first photos=%+v", got.Photos)
	}
	listPage := mustItemPage(t, getItem(h, cookie, "/api/v1/items"))
	var listed itemBody
	for _, it := range listPage.Data {
		if it.ID == item.ID {
			listed = it
		}
	}
	if listed.ID != item.ID || len(listed.Photos) == 0 || listed.Photos[0].ID != second.ID {
		t.Fatalf("list photos[0]=%+v", listed.Photos)
	}
	if len(got.ReturnTasks) != 1 || got.ReturnTasks[0].CoverPhoto == nil || got.ReturnTasks[0].CoverPhoto.ID != second.ID {
		t.Fatalf("nested cover=%+v", got.ReturnTasks)
	}
	open := mustReturnTaskPage(t, getItem(h, cookie, "/api/v1/return-tasks"))
	var listedTask returnTaskBody
	for _, tk := range open.Data {
		if tk.ID == task.ID {
			listedTask = tk
		}
	}
	if listedTask.ID != task.ID || listedTask.CoverPhoto == nil || listedTask.CoverPhoto.ID != second.ID {
		t.Fatalf("open cover=%+v", listedTask.CoverPhoto)
	}

	rec = postItem(h, cookie, photoFirstURL(second.ID), `{"version":2}`)
	again := assertPhotoOK(t, rec)
	if again.ID != second.ID || again.Position != 0 || again.Version != 3 {
		t.Fatalf("set first again=%+v", again)
	}
	got = requireItem(t, h, cookie, item.ID)
	if got.Version != 1 || got.Photos[0].Version != 3 || got.Photos[1].ID != first.ID || got.Photos[1].Version != 1 {
		t.Fatalf("after second first=%+v item version=%d", got.Photos, got.Version)
	}

	rec = postItem(h, cookie, photoFirstURL(second.ID), `{"version":1}`)
	assertError(t, rec, http.StatusConflict, "version_conflict", "记录已被修改")
	assertNoFieldsKey(t, rec)
	stale := requireItem(t, h, cookie, item.ID)
	if stale.Photos[0].ID != second.ID || stale.Photos[0].Position != 0 || stale.Photos[0].Version != 3 || stale.Photos[1].ID != first.ID || stale.Photos[1].Position != 1 || stale.Photos[1].Version != 1 {
		t.Fatalf("stale first wrote=%+v", stale.Photos)
	}
	assertPhotoFiles(t, dataDir, first.ID, true)
	assertPhotoFiles(t, dataDir, second.ID, true)

	rec = deleteItem(h, cookie, photoURL(second.ID)+"?version=1")
	assertError(t, rec, http.StatusConflict, "version_conflict", "记录已被修改")
	assertNoFieldsKey(t, rec)
	stale = requireItem(t, h, cookie, item.ID)
	if stale.Photos[0].ID != second.ID || stale.Photos[0].Position != 0 || stale.Photos[0].Version != 3 || stale.Photos[1].ID != first.ID || stale.Photos[1].Version != 1 {
		t.Fatalf("stale delete wrote=%+v", stale.Photos)
	}
	assertPhotoFiles(t, dataDir, first.ID, true)
	assertPhotoFiles(t, dataDir, second.ID, true)

	rec = deleteItem(h, cookie, photoURL(second.ID)+"?version=3")
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("delete first status=%d body=%q", rec.Code, rec.Body.String())
	}
	assertPhotoFiles(t, dataDir, second.ID, false)
	assertPhotoFiles(t, dataDir, first.ID, true)
	got = requireItem(t, h, cookie, item.ID)
	if got.Version != 1 || len(got.Photos) != 1 || got.Photos[0].ID != first.ID || got.Photos[0].Position != 0 || got.Photos[0].Version != 1 {
		t.Fatalf("after delete cover=%+v version=%d", got.Photos, got.Version)
	}
	if got.ReturnTasks[0].CoverPhoto == nil || got.ReturnTasks[0].CoverPhoto.ID != first.ID {
		t.Fatalf("cover after delete first=%+v", got.ReturnTasks[0].CoverPhoto)
	}
	open = mustReturnTaskPage(t, getItem(h, cookie, "/api/v1/return-tasks"))
	for _, tk := range open.Data {
		if tk.ID == task.ID && (tk.CoverPhoto == nil || tk.CoverPhoto.ID != first.ID) {
			t.Fatalf("open cover after delete=%+v", tk.CoverPhoto)
		}
	}

	rec = deleteItem(h, cookie, photoURL(first.ID)+"?version=1")
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("delete last status=%d body=%q", rec.Code, rec.Body.String())
	}
	assertPhotoFiles(t, dataDir, first.ID, false)
	itemRec := getItem(h, cookie, itemURL(item.ID))
	got = decodeItem(t, itemRec)
	if len(got.Photos) != 0 {
		t.Fatalf("photos after last delete=%+v", got.Photos)
	}
	if !strings.Contains(itemRec.Body.String(), `"cover_photo":null`) {
		t.Fatalf("nested cover after last delete: %s", itemRec.Body.String())
	}
	taskRec := getItem(h, cookie, returnTaskURL(task.ID))
	emptyTask := decodeReturnTask(t, taskRec)
	if emptyTask.CoverPhoto != nil || !strings.Contains(taskRec.Body.String(), `"cover_photo":null`) {
		t.Fatalf("task cover after last delete=%+v body=%s", emptyTask.CoverPhoto, taskRec.Body.String())
	}
	openRec := getItem(h, cookie, "/api/v1/return-tasks")
	open = mustReturnTaskPage(t, openRec)
	for _, tk := range open.Data {
		if tk.ID == task.ID && (tk.CoverPhoto != nil || !strings.Contains(openRec.Body.String(), `"cover_photo":null`)) {
			t.Fatalf("open cover after last delete=%+v body=%s", tk.CoverPhoto, openRec.Body.String())
		}
	}

	rec = postItem(h, cookie, photoFirstURL(999999), `[]`)
	assertError(t, rec, http.StatusBadRequest, "invalid_body", "请求格式不正确")
	rec = deleteItem(h, cookie, "/api/v1/photos/999999?version=abc")
	if rec.Code == http.StatusNotFound {
		t.Fatalf("missing photo bad version returned 404: %s", rec.Body.String())
	}
	assertInvalidFields(t, rec, map[string]string{"version": "版本不正确"})
	if countQuery(t, db, `SELECT COUNT(*) FROM photos`) != 0 {
		t.Fatal("photo rows remained")
	}
}

func TestPhotoLimitAndReuse(t *testing.T) {
	db, h, _, dataDir := testHandlerDir(t, false)
	cookie := login(t, h)
	item := mustItem(t, h, cookie, `{"name":"相册"}`)
	tiny := encodeTestJPEG(t, 2, 2)
	var last photoBody
	for i := 0; i < 20; i++ {
		last = assertPhotoCreated(t, postPhoto(t, h, cookie, itemPhotosURL(item.ID), tiny))
		if last.Position != i {
			t.Fatalf("upload %d position=%d", i, last.Position)
		}
	}
	if countQuery(t, db, `SELECT COUNT(*) FROM photos WHERE item_id = ?`, item.ID) != 20 {
		t.Fatal("expected 20 photo rows")
	}
	if n := countJPEGDir(t, filepath.Join(dataDir, "originals")); n != 20 {
		t.Fatalf("originals=%d", n)
	}
	if n := countJPEGDir(t, filepath.Join(dataDir, "thumbnails")); n != 20 {
		t.Fatalf("thumbnails=%d", n)
	}

	rec := postPhoto(t, h, cookie, itemPhotosURL(item.ID), tiny)
	assertError(t, rec, http.StatusConflict, "photo_limit", "一件物品最多 20 张照片")
	if countQuery(t, db, `SELECT COUNT(*) FROM photos WHERE item_id = ?`, item.ID) != 20 {
		t.Fatal("21st upload wrote a row")
	}
	if n := countJPEGDir(t, filepath.Join(dataDir, "originals")); n != 20 {
		t.Fatalf("originals after 21st=%d", n)
	}
	if n := countJPEGDir(t, filepath.Join(dataDir, "thumbnails")); n != 20 {
		t.Fatalf("thumbnails after 21st=%d", n)
	}

	maxID := int64(countQuery(t, db, `SELECT MAX(id) FROM photos`))
	if maxID != last.ID {
		t.Fatalf("max=%d last=%d", maxID, last.ID)
	}
	rec = deleteItem(h, cookie, photoURL(maxID)+"?version=1")
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("delete max status=%d body=%q", rec.Code, rec.Body.String())
	}
	created := assertPhotoCreated(t, postPhoto(t, h, cookie, itemPhotosURL(item.ID), tiny))
	if created.ID == maxID {
		t.Fatalf("reused id=%d", created.ID)
	}
	rec = postItem(h, cookie, photoFirstURL(maxID), `{"version":1}`)
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	rec = deleteItem(h, cookie, photoURL(maxID)+"?version=1")
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	got := requireItem(t, h, cookie, item.ID)
	if len(got.Photos) != 20 {
		t.Fatalf("photos after reuse=%+v", got.Photos)
	}
	for _, p := range got.Photos {
		if p.ID == maxID {
			t.Fatalf("old id still present: %+v", got.Photos)
		}
	}
}

func TestPhotoTrash(t *testing.T) {
	db, h, _, dataDir := testHandlerDir(t, false)
	cookie := login(t, h)
	item := mustItem(t, h, cookie, `{"name":"遥控车"}`)
	other := mustItem(t, h, cookie, `{"name":"另一件"}`)
	jpegBytes := encodeTestJPEG(t, 8, 8)
	kept := assertPhotoCreated(t, postPhoto(t, h, cookie, itemPhotosURL(item.ID), jpegBytes))
	otherPhoto := assertPhotoCreated(t, postPhoto(t, h, cookie, itemPhotosURL(other.ID), jpegBytes))

	rec := deleteItem(h, cookie, itemURL(item.ID)+"?version=1")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("trash status=%d body=%s", rec.Code, rec.Body.String())
	}
	assertPhotoJPEG(t, request(h, http.MethodGet, photoThumbURL(kept.ID), "", "", testRemote, cookie))
	assertPhotoJPEG(t, request(h, http.MethodGet, photoOriginalURL(kept.ID), "", "", testRemote, cookie))

	rec = postPhoto(t, h, cookie, itemPhotosURL(item.ID), jpegBytes)
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	rec = postItem(h, cookie, photoFirstURL(kept.ID), `{"version":1}`)
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	rec = deleteItem(h, cookie, photoURL(kept.ID)+"?version=1")
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")
	if countQuery(t, db, `SELECT COUNT(*) FROM photos WHERE id = ?`, kept.ID) != 1 {
		t.Fatal("trashed item lost photo row")
	}
	assertPhotoFiles(t, dataDir, kept.ID, true)

	rec = postItem(h, cookie, restoreURL(item.ID), `{"version":2}`)
	restored := assertItemOK(t, rec)
	if len(restored.Photos) != 1 || restored.Photos[0].ID != kept.ID || restored.Photos[0].Position != 0 {
		t.Fatalf("restore photos=%+v", restored.Photos)
	}

	rec = deleteItem(h, cookie, itemURL(item.ID)+"?version=3")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("second trash status=%d body=%s", rec.Code, rec.Body.String())
	}
	trashed := requireTrash(t, h, cookie, item.ID)
	rec = deleteItem(h, cookie, trashURL(item.ID)+fmt.Sprintf("?version=%d", trashed.Version))
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("purge status=%d body=%q", rec.Code, rec.Body.String())
	}
	if countQuery(t, db, `SELECT COUNT(*) FROM photos WHERE id = ?`, kept.ID) != 0 {
		t.Fatal("purged photo row remained")
	}
	assertPhotoFiles(t, dataDir, kept.ID, false)
	assertPhotoFiles(t, dataDir, otherPhoto.ID, true)
	if countQuery(t, db, `SELECT COUNT(*) FROM photos WHERE id = ?`, otherPhoto.ID) != 1 {
		t.Fatal("other item photo row gone")
	}
	rec = request(h, http.MethodGet, photoThumbURL(kept.ID), "", "", testRemote, cookie)
	assertError(t, rec, http.StatusNotFound, "not_found", "未找到")

	orig, _ := photoFilePaths(dataDir, otherPhoto.ID)
	if err := os.Remove(orig); err != nil {
		t.Fatal(err)
	}
	rec = request(h, http.MethodGet, photoOriginalURL(otherPhoto.ID), "", "", testRemote, cookie)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("missing file status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("missing file body=%q", rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct == "image/jpeg" {
		t.Fatalf("missing file Content-Type=%q", ct)
	}
}

func postPhoto(t *testing.T, h http.Handler, cookie, path string, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	return postPhotoParts(t, h, cookie, path, webOrigin, true, data)
}

func postPhotoParts(t *testing.T, h http.Handler, cookie, path, origin string, includeFile bool, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if includeFile {
		part, err := mw.CreateFormFile("file", "photo.bin")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.RemoteAddr = testRemote
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: "youchu_session", Value: cookie})
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func requestWithType(h http.Handler, method, path, body, origin, cookie, contentType string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = testRemote
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: "youchu_session", Value: cookie})
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

type infiniteReader byte

func (r infiniteReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(r)
	}
	return len(p), nil
}

func postOversize(h http.Handler, cookie, path, origin string) *httptest.ResponseRecorder {
	const boundary = "x"
	head := "--" + boundary + "\r\nContent-Disposition: form-data; name=\"file\"; filename=\"big.bin\"\r\nContent-Type: application/octet-stream\r\n\r\n"
	payload := photo.MaxUploadBytes + 1 - int64(len(head))
	if payload < 1 {
		payload = photo.MaxUploadBytes + 1
	}
	body := io.MultiReader(strings.NewReader(head), io.LimitReader(infiniteReader('x'), payload))
	req := httptest.NewRequest(http.MethodPost, path, body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	req.ContentLength = int64(len(head)) + payload
	req.RemoteAddr = testRemote
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: "youchu_session", Value: cookie})
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func assertPhotoOK(t *testing.T, rec *httptest.ResponseRecorder) photoBody {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var p photoBody
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("body=%s err=%v", rec.Body.String(), err)
	}
	if p.ID < 1 || p.Version < 1 {
		t.Fatalf("photo=%+v body=%s", p, rec.Body.String())
	}
	return p
}

func photoFilePaths(dataDir string, id int64) (string, string) {
	name := fmt.Sprintf("%d.jpg", id)
	return filepath.Join(dataDir, "originals", name), filepath.Join(dataDir, "thumbnails", name)
}

func assertPhotoFiles(t *testing.T, dataDir string, id int64, exist bool) {
	t.Helper()
	orig, thumb := photoFilePaths(dataDir, id)
	for _, p := range []string{orig, thumb} {
		_, err := os.Stat(p)
		if exist {
			if err != nil {
				t.Fatalf("missing %s: %v", p, err)
			}
			continue
		}
		if err == nil {
			t.Fatalf("still exists %s", p)
		}
		if !os.IsNotExist(err) {
			t.Fatalf("stat %s: %v", p, err)
		}
	}
}

func countJPEGDir(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".jpg") {
			n++
		}
	}
	return n
}

func assertPhotoCreated(t *testing.T, rec *httptest.ResponseRecorder) photoBody {
	t.Helper()
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var p photoBody
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("body=%s err=%v", rec.Body.String(), err)
	}
	if p.ID < 1 || p.Version != 1 || p.Width < 1 || p.Height < 1 || p.ByteSize < 1 {
		t.Fatalf("photo=%+v body=%s", p, rec.Body.String())
	}
	if p.CreatedAt != frozenAt || p.UpdatedAt != frozenAt {
		t.Fatalf("photo times=%+v", p)
	}
	return p
}

func assertPhotoJPEG(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/jpeg" {
		t.Fatalf("Content-Type=%q", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "private, no-store" {
		t.Fatalf("Cache-Control=%q", cc)
	}
	if rec.Header().Get("Content-Disposition") != "" {
		t.Fatalf("Content-Disposition=%q", rec.Header().Get("Content-Disposition"))
	}
	if _, err := jpeg.Decode(bytes.NewReader(rec.Body.Bytes())); err != nil {
		t.Fatalf("jpeg: %v", err)
	}
}

func encodeTestJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, m, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func encodeTestPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := png.Encode(&buf, m); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func oneByOneWebP() []byte {
	return []byte{
		0x52, 0x49, 0x46, 0x46, 0x1c, 0x00, 0x00, 0x00, 0x57, 0x45, 0x42, 0x50,
		0x56, 0x50, 0x38, 0x4c, 0x0f, 0x00, 0x00, 0x00, 0x2f, 0x00, 0x00, 0x00,
		0x00, 0x07, 0x10, 0xfd, 0x8f, 0xfe, 0x07, 0x22, 0xa2, 0xff, 0x01, 0x00,
	}
}

func jpegSOF(w, h int) []byte {
	return []byte{
		0xFF, 0xD8,
		0xFF, 0xE0, 0x00, 0x10,
		'J', 'F', 'I', 'F', 0x00,
		0x01, 0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00,
		0xFF, 0xC0, 0x00, 0x0B, 0x08,
		byte(h >> 8), byte(h),
		byte(w >> 8), byte(w),
		0x01, 0x01, 0x11, 0x00,
		0xFF, 0xD9,
	}
}
