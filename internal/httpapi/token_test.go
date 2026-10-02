package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

func TestCreateAndListAccessToken(t *testing.T) {
	db, h, _ := testHandler(t, false)
	cookie := login(t, h)
	rec := request(h, http.MethodPost, "/api/v1/tokens", `{"name":"电脑上的 Cursor","scopes":["read","organize"]}`, "http://127.0.0.1:5173", "203.0.113.5:1", cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create=%d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID          int64    `json:"id"`
		Name        string   `json:"name"`
		Token       string   `json:"token"`
		TokenPrefix string   `json:"token_prefix"`
		Scopes      []string `json:"scopes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(created.Token, "yc_") || created.TokenPrefix != created.Token[:12] {
		t.Fatalf("token=%q prefix=%q", created.Token, created.TokenPrefix)
	}
	if created.Name != "电脑上的 Cursor" || strings.Join(created.Scopes, ",") != "read,organize" {
		t.Fatalf("created=%+v", created)
	}
	var hash string
	if err := db.QueryRow(`SELECT token_hash FROM access_tokens WHERE id = ?`, created.ID).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if hash == created.Token || hash == "" {
		t.Fatal("stored plaintext")
	}
	list := request(h, http.MethodGet, "/api/v1/tokens", "", "", "203.0.113.5:1", cookie)
	if list.Code != http.StatusOK || strings.Contains(list.Body.String(), `"token":`) {
		t.Fatalf("list=%d %s", list.Code, list.Body.String())
	}
	if !strings.Contains(list.Body.String(), `"token_prefix":"`+created.TokenPrefix) {
		t.Fatalf("list body=%s", list.Body.String())
	}
}

func TestAccessTokenScopeFields(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)
	origin := "http://127.0.0.1:5173"
	for _, body := range []string{
		`{"name":"a","scopes":["organize"]}`,
		`{"name":"a","scopes":["write"]}`,
		`{"name":"a","scopes":["read","read"]}`,
		`{"name":"a","scopes":["read","foo"]}`,
		`{"name":"a","scopes":"read"}`,
	} {
		rec := request(h, http.MethodPost, "/api/v1/tokens", body, origin, "203.0.113.5:1", cookie)
		assertInvalidFields(t, rec, map[string]string{"scopes": "权限范围不正确"})
	}
	blank := request(h, http.MethodPost, "/api/v1/tokens", `{"name":"  ","scopes":["read"]}`, origin, "203.0.113.5:1", cookie)
	assertInvalidFields(t, blank, map[string]string{"name": "名称不能为空"})
	missingName := request(h, http.MethodPost, "/api/v1/tokens", `{"scopes":["read"]}`, origin, "203.0.113.5:1", cookie)
	assertCode(t, missingName, http.StatusBadRequest, "invalid_body")
	nullName := request(h, http.MethodPost, "/api/v1/tokens", `{"name":null,"scopes":["read"]}`, origin, "203.0.113.5:1", cookie)
	assertCode(t, nullName, http.StatusBadRequest, "invalid_body")
	missingScopes := request(h, http.MethodPost, "/api/v1/tokens", `{"name":"a"}`, origin, "203.0.113.5:1", cookie)
	assertInvalidFields(t, missingScopes, map[string]string{"scopes": "权限范围不正确"})
	nullScopes := request(h, http.MethodPost, "/api/v1/tokens", `{"name":"a","scopes":null}`, origin, "203.0.113.5:1", cookie)
	assertInvalidFields(t, nullScopes, map[string]string{"scopes": "权限范围不正确"})
}

func TestAccessTokenLimitAndRevoke(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)
	origin := "http://127.0.0.1:5173"
	var lastID int64
	var lastToken string
	ids := map[int64]struct{}{}
	for i := 0; i < 20; i++ {
		rec := request(h, http.MethodPost, "/api/v1/tokens", `{"name":"t","scopes":["read"]}`, origin, "203.0.113.5:1", cookie)
		if rec.Code != http.StatusCreated {
			t.Fatalf("i=%d code=%d %s", i, rec.Code, rec.Body.String())
		}
		var created struct {
			ID    int64  `json:"id"`
			Token string `json:"token"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
			t.Fatal(err)
		}
		lastID, lastToken = created.ID, created.Token
		ids[created.ID] = struct{}{}
	}
	over := request(h, http.MethodPost, "/api/v1/tokens", `{"name":"t","scopes":["read"]}`, origin, "203.0.113.5:1", cookie)
	assertCode(t, over, http.StatusConflict, "token_limit")
	del := request(h, http.MethodDelete, "/api/v1/tokens/"+strconv.FormatInt(lastID, 10), "", origin, "203.0.113.5:1", cookie)
	if del.Code != http.StatusNoContent {
		t.Fatalf("del=%d %s", del.Code, del.Body.String())
	}
	_ = lastToken
	list := request(h, http.MethodGet, "/api/v1/tokens", "", "", "203.0.113.5:1", cookie)
	if list.Code != http.StatusOK {
		t.Fatalf("list=%d %s", list.Code, list.Body.String())
	}
	var page struct {
		Data []struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Data) != 19 {
		t.Fatalf("remaining=%d body=%s", len(page.Data), list.Body.String())
	}
	for _, row := range page.Data {
		if row.ID == lastID {
			t.Fatalf("deleted id %d still listed: %s", lastID, list.Body.String())
		}
		delete(ids, row.ID)
	}
	if len(ids) != 1 {
		t.Fatalf("unexpected remaining ids=%v", ids)
	}
	if _, ok := ids[lastID]; !ok {
		t.Fatalf("expected deleted id %d to be the missing one", lastID)
	}
}

func TestRevokeTokenMakesBearerUnknown(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)
	origin := "http://127.0.0.1:5173"
	created := request(h, http.MethodPost, "/api/v1/tokens", `{"name":"mcp","scopes":["read"]}`, origin, "203.0.113.5:1", cookie)
	if created.Code != http.StatusCreated {
		t.Fatalf("create=%d %s", created.Code, created.Body.String())
	}
	var tok struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &tok); err != nil {
		t.Fatal(err)
	}
	del := request(h, http.MethodDelete, "/api/v1/tokens/"+strconv.FormatInt(tok.ID, 10), "", origin, "203.0.113.5:1", cookie)
	if del.Code != http.StatusNoContent {
		t.Fatalf("del=%d %s", del.Code, del.Body.String())
	}
	list := request(h, http.MethodGet, "/api/v1/tokens", "", "", "203.0.113.5:1", cookie)
	if list.Code != http.StatusOK || list.Body.String() != "{\"data\":[]}\n" {
		t.Fatalf("list=%d %s", list.Code, list.Body.String())
	}
}

func TestAccessTokenWriteGuardOrder(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)
	big := `{"name":"` + strings.Repeat("n", 5000) + `","scopes":["read"]}`
	badOrigin := request(h, http.MethodPost, "/api/v1/tokens", `{"name":"t","scopes":["read"]}`, "http://evil.example", "203.0.113.5:1", cookie)
	assertCode(t, badOrigin, http.StatusForbidden, "origin_rejected")
	noSess := request(h, http.MethodPost, "/api/v1/tokens", `{"name":"t","scopes":["read"]}`, "http://127.0.0.1:5173", "203.0.113.5:1", "")
	assertCode(t, noSess, http.StatusUnauthorized, "unauthenticated")
	query := request(h, http.MethodPost, "/api/v1/tokens?x=1", `{"name":"t","scopes":["read"]}`, "http://127.0.0.1:5173", "203.0.113.5:1", cookie)
	if query.Code != http.StatusBadRequest {
		t.Fatalf("query=%d %s", query.Code, query.Body.String())
	}
	tooBig := request(h, http.MethodPost, "/api/v1/tokens", big, "http://127.0.0.1:5173", "203.0.113.5:1", cookie)
	assertCode(t, tooBig, http.StatusRequestEntityTooLarge, "body_too_large")
	badBig := request(h, http.MethodPost, "/api/v1/tokens", big, "http://evil.example", "203.0.113.5:1", cookie)
	assertCode(t, badBig, http.StatusForbidden, "origin_rejected")
	noSessBig := request(h, http.MethodPost, "/api/v1/tokens", big, "http://127.0.0.1:5173", "203.0.113.5:1", "")
	assertCode(t, noSessBig, http.StatusUnauthorized, "unauthenticated")
	wrong := request(h, http.MethodPut, "/api/v1/tokens", `{"name":"t","scopes":["read"]}`, "http://127.0.0.1:5173", "203.0.113.5:1", cookie)
	assertCode(t, wrong, http.StatusNotFound, "not_found")
	getOne := request(h, http.MethodGet, "/api/v1/tokens/1", "", "", "203.0.113.5:1", cookie)
	assertCode(t, getOne, http.StatusNotFound, "not_found")
	listQuery := request(h, http.MethodGet, "/api/v1/tokens?x=1", "", "", "203.0.113.5:1", cookie)
	assertInvalidFields(t, listQuery, map[string]string{"x": "不支持的参数"})
	delQuery := request(h, http.MethodDelete, "/api/v1/tokens/1?x=1", "", "http://127.0.0.1:5173", "203.0.113.5:1", cookie)
	assertInvalidFields(t, delQuery, map[string]string{"x": "不支持的参数"})
	missing := request(h, http.MethodDelete, "/api/v1/tokens/999", "", "http://127.0.0.1:5173", "203.0.113.5:1", cookie)
	assertCode(t, missing, http.StatusNotFound, "not_found")
	badID := request(h, http.MethodDelete, "/api/v1/tokens/abc", "", "http://127.0.0.1:5173", "203.0.113.5:1", cookie)
	assertCode(t, badID, http.StatusNotFound, "not_found")
}
