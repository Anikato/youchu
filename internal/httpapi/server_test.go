package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"youchu/internal/auth"
	"youchu/internal/config"
	"youchu/internal/migrate"
	"youchu/internal/photo"
	"youchu/internal/store"
	"youchu/migrations"
)

func TestLoginAndMe(t *testing.T) {
	db, h, _ := testHandler(t, false)
	rec := request(h, http.MethodPost, "/api/v1/session", `{"username":"ada","password":"correct-horse"}`, "http://127.0.0.1:5173", "203.0.113.5:1", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["username"] != "ada" {
		t.Fatalf("body=%s err=%v", rec.Body.String(), err)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "youchu_session" || cookies[0].Value == "" {
		t.Fatalf("cookies=%v", cookies)
	}
	c := cookies[0]
	if !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Secure || c.MaxAge != 3600 || c.Path != "/" {
		t.Fatalf("cookie=%+v", c)
	}
	me := request(h, http.MethodGet, "/api/v1/me", "", "", "203.0.113.5:1", c.Value)
	if me.Code != http.StatusOK || !strings.Contains(me.Body.String(), `"username":"ada"`) {
		t.Fatalf("me=%d %s", me.Code, me.Body.String())
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE token_hash = ?`, c.Value).Scan(&n); err != nil || n != 0 {
		t.Fatal("raw token was stored")
	}
}

func TestSecureCookieFollowsConfig(t *testing.T) {
	_, h, _ := testHandler(t, true)
	rec := request(h, http.MethodPost, "/api/v1/session", `{"username":"ada","password":"correct-horse"}`, "http://127.0.0.1:5173", "203.0.113.5:1", "")
	cookies := rec.Result().Cookies()
	if rec.Code != http.StatusOK || len(cookies) != 1 || !cookies[0].Secure {
		t.Fatalf("status=%d cookies=%v", rec.Code, cookies)
	}
}

func TestBadCredentialsMatch(t *testing.T) {
	_, h, _ := testHandler(t, false)
	wrong := request(h, http.MethodPost, "/api/v1/session", `{"username":"ada","password":"wrong-password"}`, "http://127.0.0.1:5173", "203.0.113.8:1", "")
	missing := request(h, http.MethodPost, "/api/v1/session", `{"username":"nobody","password":"wrong-password"}`, "http://127.0.0.1:5173", "203.0.113.9:1", "")
	if wrong.Code != http.StatusUnauthorized || missing.Code != http.StatusUnauthorized {
		t.Fatalf("wrong=%d missing=%d", wrong.Code, missing.Code)
	}
	if wrong.Body.String() != missing.Body.String() {
		t.Fatalf("bodies differ: %s vs %s", wrong.Body.String(), missing.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(wrong.Body.Bytes(), &body); err != nil || body["code"] != "invalid_credentials" || body["message"] != "用户名或密码错误" {
		t.Fatalf("body=%s", wrong.Body.String())
	}
	if len(wrong.Result().Cookies()) != 0 || len(missing.Result().Cookies()) != 0 {
		t.Fatal("failed login set a cookie")
	}
}

func TestMeRequiresSession(t *testing.T) {
	db, h, now := testHandler(t, false)
	rec := request(h, http.MethodGet, "/api/v1/me", "", "", "203.0.113.5:1", "")
	assertCode(t, rec, http.StatusUnauthorized, "unauthenticated")
	login := request(h, http.MethodPost, "/api/v1/session", `{"username":"ada","password":"correct-horse"}`, "http://127.0.0.1:5173", "203.0.113.5:1", "")
	token := login.Result().Cookies()[0].Value
	past := now.Add(-time.Second).Format(time.RFC3339Nano)
	if _, err := db.Exec(`UPDATE sessions SET expires_at = ?`, past); err != nil {
		t.Fatal(err)
	}
	expired := request(h, http.MethodGet, "/api/v1/me", "", "", "203.0.113.5:1", token)
	assertCode(t, expired, http.StatusUnauthorized, "unauthenticated")
}

func TestOriginRequired(t *testing.T) {
	db, h, _ := testHandler(t, false)
	missing := request(h, http.MethodPost, "/api/v1/session", `{"username":"ada","password":"correct-horse"}`, "", "203.0.113.5:1", "")
	wrong := request(h, http.MethodPost, "/api/v1/session", `{"username":"ada","password":"correct-horse"}`, "http://evil.example", "203.0.113.5:1", "")
	assertCode(t, missing, http.StatusForbidden, "origin_rejected")
	assertCode(t, wrong, http.StatusForbidden, "origin_rejected")
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("sessions=%d err=%v", n, err)
	}
}

func TestLoginRateLimitIsPerRemoteAddr(t *testing.T) {
	_, h, _ := testHandler(t, false)
	body := `{"username":"ada","password":"wrong-password"}`
	for i := 0; i < 10; i++ {
		rec := request(h, http.MethodPost, "/api/v1/session", body, "http://127.0.0.1:5173", "203.0.113.10:1", "")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status=%d", i, rec.Code)
		}
	}
	limited := request(h, http.MethodPost, "/api/v1/session", body, "http://127.0.0.1:5173", "203.0.113.10:9", "")
	assertCode(t, limited, http.StatusTooManyRequests, "rate_limited")
	other := request(h, http.MethodPost, "/api/v1/session", body, "http://127.0.0.1:5173", "203.0.113.11:1", "")
	if other.Code == http.StatusTooManyRequests {
		t.Fatal("other address shared the bucket")
	}
}

func TestLoginRateLimitIgnoresForwardedForByDefault(t *testing.T) {
	_, h, _ := testHandler(t, false)
	body := `{"username":"ada","password":"wrong-password"}`
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/session", strings.NewReader(body))
		req.RemoteAddr = "203.0.113.20:1"
		req.Header.Set("Origin", "http://127.0.0.1:5173")
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("198.51.100.%d", i+1))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status=%d", i, rec.Code)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/session", strings.NewReader(body))
	req.RemoteAddr = "203.0.113.20:1"
	req.Header.Set("Origin", "http://127.0.0.1:5173")
	req.Header.Set("X-Forwarded-For", "203.0.113.99")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assertCode(t, rec, http.StatusTooManyRequests, "rate_limited")
}

func TestLoginRateLimitTrustsForwardedForWhenEnabled(t *testing.T) {
	db, _, now, dir := testHandlerDir(t, false)
	dummy, err := auth.NewDummyHash()
	if err != nil {
		t.Fatal(err)
	}
	h := New(db, config.Config{
		DataDir:      dir,
		PublicOrigin: "http://127.0.0.1:5173",
		SessionTTL:   time.Hour,
		LoginLimit:   10,
		TrustProxy:   true,
	}, dummy, func() time.Time { return now })
	body := `{"username":"ada","password":"wrong-password"}`
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/session", strings.NewReader(body))
		req.RemoteAddr = "172.17.0.1:1"
		req.Header.Set("Origin", "http://127.0.0.1:5173")
		req.Header.Set("X-Forwarded-For", "203.0.113.40")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status=%d body=%s", i, rec.Code, rec.Body.String())
		}
	}
	limited := httptest.NewRequest(http.MethodPost, "/api/v1/session", strings.NewReader(body))
	limited.RemoteAddr = "172.17.0.1:1"
	limited.Header.Set("Origin", "http://127.0.0.1:5173")
	limited.Header.Set("X-Forwarded-For", "203.0.113.40")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, limited)
	assertCode(t, rec, http.StatusTooManyRequests, "rate_limited")
	other := httptest.NewRequest(http.MethodPost, "/api/v1/session", strings.NewReader(body))
	other.RemoteAddr = "172.17.0.1:1"
	other.Header.Set("Origin", "http://127.0.0.1:5173")
	other.Header.Set("X-Forwarded-For", "203.0.113.41")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, other)
	if rec.Code == http.StatusTooManyRequests {
		t.Fatal("other forwarded address shared the bucket")
	}
}

func TestRootIsJSONWhenWebMissing(t *testing.T) {
	_, h, _ := testHandler(t, false)
	rec := request(h, http.MethodGet, "/", "", "", "203.0.113.5:1", "")
	assertCode(t, rec, http.StatusNotFound, "not_found")
}

func TestSPAServesIndexAndAssets(t *testing.T) {
	db, _, now, dir := testHandlerDir(t, false)
	dummy, err := auth.NewDummyHash()
	if err != nil {
		t.Fatal(err)
	}
	web := fstest.MapFS{
		"index.html":            {Data: []byte("<!doctype html><title>有处</title>")},
		"assets/app.js":         {Data: []byte("console.log(1)")},
		"manifest.json":         {Data: []byte(`{"name":"有处","display":"standalone"}`)},
		"apple-touch-icon.png":  {Data: []byte("png")},
	}
	h := newHandler(db, config.Config{
		DataDir:      dir,
		PublicOrigin: "http://127.0.0.1:5173",
		SessionTTL:   time.Hour,
		LoginLimit:   10,
	}, dummy, func() time.Time { return now }, web)
	html := request(h, http.MethodGet, "/", "", "", "203.0.113.5:1", "")
	if html.Code != http.StatusOK || !strings.Contains(html.Body.String(), "有处") {
		t.Fatalf("GET / status=%d body=%s", html.Code, html.Body.String())
	}
	if !strings.Contains(html.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("content-type=%q", html.Header().Get("Content-Type"))
	}
	page := request(h, http.MethodGet, "/account", "", "", "203.0.113.5:1", "")
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "有处") {
		t.Fatalf("GET /account status=%d body=%s", page.Code, page.Body.String())
	}
	asset := request(h, http.MethodGet, "/assets/app.js", "", "", "203.0.113.5:1", "")
	if asset.Code != http.StatusOK || asset.Body.String() != "console.log(1)" {
		t.Fatalf("asset status=%d body=%s", asset.Code, asset.Body.String())
	}
	manifest := request(h, http.MethodGet, "/manifest.json", "", "", "203.0.113.5:1", "")
	if manifest.Code != http.StatusOK || !strings.Contains(manifest.Body.String(), `"standalone"`) {
		t.Fatalf("manifest status=%d body=%s", manifest.Code, manifest.Body.String())
	}
	icon := request(h, http.MethodGet, "/apple-touch-icon.png", "", "", "203.0.113.5:1", "")
	if icon.Code != http.StatusOK || icon.Body.String() != "png" {
		t.Fatalf("icon status=%d body=%s", icon.Code, icon.Body.String())
	}
	api404 := request(h, http.MethodGet, "/api/v1/does-not-exist", "", "", "203.0.113.5:1", "")
	assertCode(t, api404, http.StatusNotFound, "not_found")
	missingJS := request(h, http.MethodGet, "/missing.js", "", "", "203.0.113.5:1", "")
	assertCode(t, missingJS, http.StatusNotFound, "not_found")
	post := request(h, http.MethodPost, "/", "", "http://127.0.0.1:5173", "203.0.113.5:1", "")
	assertCode(t, post, http.StatusNotFound, "not_found")
}

func TestOriginFailuresDoNotConsumeRateLimit(t *testing.T) {
	_, h, _ := testHandler(t, false)
	for i := 0; i < 11; i++ {
		rec := request(h, http.MethodPost, "/api/v1/session", `{"username":"ada","password":"wrong-password"}`, "http://evil.example", "203.0.113.12:1", "")
		if rec.Code != http.StatusForbidden {
			t.Fatalf("attempt %d status=%d", i, rec.Code)
		}
	}
	rec := request(h, http.MethodPost, "/api/v1/session", `{"username":"ada","password":"wrong-password"}`, "http://127.0.0.1:5173", "203.0.113.12:1", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestLogoutDeletesOnlyThatSession(t *testing.T) {
	_, h, _ := testHandler(t, false)
	first := request(h, http.MethodPost, "/api/v1/session", `{"username":"ada","password":"correct-horse"}`, "http://127.0.0.1:5173", "198.51.100.1:1", "")
	second := request(h, http.MethodPost, "/api/v1/session", `{"username":"ada","password":"correct-horse"}`, "http://127.0.0.1:5173", "198.51.100.2:1", "")
	a := first.Result().Cookies()[0].Value
	b := second.Result().Cookies()[0].Value
	gone := request(h, http.MethodDelete, "/api/v1/session", "", "http://127.0.0.1:5173", "198.51.100.1:1", a)
	if gone.Code != http.StatusNoContent {
		t.Fatalf("logout=%d", gone.Code)
	}
	if me := request(h, http.MethodGet, "/api/v1/me", "", "", "198.51.100.1:1", a); me.Code != http.StatusUnauthorized {
		t.Fatalf("old cookie still worked: %d", me.Code)
	}
	if me := request(h, http.MethodGet, "/api/v1/me", "", "", "198.51.100.2:1", b); me.Code != http.StatusOK {
		t.Fatalf("other session died: %d %s", me.Code, me.Body.String())
	}
	empty := request(h, http.MethodDelete, "/api/v1/session", "", "http://127.0.0.1:5173", "198.51.100.3:1", "")
	if empty.Code != http.StatusNoContent {
		t.Fatalf("empty logout=%d", empty.Code)
	}
}

func TestInvalidBodies(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cases := []string{`{`, `{"username":"ada"}`, `{"username":"  ","password":"x"}`, `{"username":"ada","password":""}`}
	for _, body := range cases {
		rec := request(h, http.MethodPost, "/api/v1/session", body, "http://127.0.0.1:5173", "203.0.113.20:1", "")
		assertCode(t, rec, http.StatusBadRequest, "invalid_body")
	}
	big := `{"username":"ada","password":"` + strings.Repeat("a", 5000) + `"}`
	rec := request(h, http.MethodPost, "/api/v1/session", big, "http://127.0.0.1:5173", "203.0.113.21:1", "")
	assertCode(t, rec, http.StatusRequestEntityTooLarge, "body_too_large")
	missing := request(h, http.MethodGet, "/nope", "", "", "203.0.113.21:1", "")
	assertCode(t, missing, http.StatusNotFound, "not_found")
}

func TestPatchMeUsername(t *testing.T) {
	_, h, _ := testHandler(t, false)
	login := request(h, http.MethodPost, "/api/v1/session", `{"username":"ada","password":"correct-horse"}`, "http://127.0.0.1:5173", "203.0.113.5:1", "")
	cookie := login.Result().Cookies()[0].Value
	rec := request(h, http.MethodPatch, "/api/v1/me", `{"username":"kevin"}`, "http://127.0.0.1:5173", "203.0.113.5:1", cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"username":"kevin"`) {
		t.Fatalf("patch=%d %s", rec.Code, rec.Body.String())
	}
	me := request(h, http.MethodGet, "/api/v1/me", "", "", "203.0.113.5:1", cookie)
	if me.Code != http.StatusOK || !strings.Contains(me.Body.String(), `"username":"kevin"`) {
		t.Fatalf("me=%d %s", me.Code, me.Body.String())
	}
	old := request(h, http.MethodPost, "/api/v1/session", `{"username":"ada","password":"correct-horse"}`, "http://127.0.0.1:5173", "203.0.113.6:1", "")
	assertCode(t, old, http.StatusUnauthorized, "invalid_credentials")
	ok := request(h, http.MethodPost, "/api/v1/session", `{"username":"kevin","password":"correct-horse"}`, "http://127.0.0.1:5173", "203.0.113.6:1", "")
	if ok.Code != http.StatusOK {
		t.Fatalf("login kevin=%d %s", ok.Code, ok.Body.String())
	}
}

func TestPatchMeUsernameFields(t *testing.T) {
	_, h, _ := testHandler(t, false)
	login := request(h, http.MethodPost, "/api/v1/session", `{"username":"ada","password":"correct-horse"}`, "http://127.0.0.1:5173", "203.0.113.5:1", "")
	cookie := login.Result().Cookies()[0].Value
	origin := "http://127.0.0.1:5173"
	blank := request(h, http.MethodPatch, "/api/v1/me", `{"username":"  "}`, origin, "203.0.113.5:1", cookie)
	assertInvalidFields(t, blank, map[string]string{"username": "用户名不能为空"})
	space := request(h, http.MethodPatch, "/api/v1/me", `{"username":"a b"}`, origin, "203.0.113.5:1", cookie)
	assertInvalidFields(t, space, map[string]string{"username": "用户名不能包含空白"})
	long := request(h, http.MethodPatch, "/api/v1/me", `{"username":"`+strings.Repeat("名", 65)+`"}`, origin, "203.0.113.5:1", cookie)
	assertInvalidFields(t, long, map[string]string{"username": "用户名过长"})
	nullName := request(h, http.MethodPatch, "/api/v1/me", `{"username":null}`, origin, "203.0.113.5:1", cookie)
	assertCode(t, nullName, http.StatusBadRequest, "invalid_body")
	missing := request(h, http.MethodPatch, "/api/v1/me", `{}`, origin, "203.0.113.5:1", cookie)
	assertCode(t, missing, http.StatusBadRequest, "invalid_body")
	same := request(h, http.MethodPatch, "/api/v1/me", `{"username":"ada"}`, origin, "203.0.113.5:1", cookie)
	if same.Code != http.StatusOK || !strings.Contains(same.Body.String(), `"username":"ada"`) {
		t.Fatalf("same=%d %s", same.Code, same.Body.String())
	}
}

func TestChangePasswordRevokesOtherSessions(t *testing.T) {
	db, h, _ := testHandler(t, false)
	a := request(h, http.MethodPost, "/api/v1/session", `{"username":"ada","password":"correct-horse"}`, "http://127.0.0.1:5173", "203.0.113.5:1", "")
	b := request(h, http.MethodPost, "/api/v1/session", `{"username":"ada","password":"correct-horse"}`, "http://127.0.0.1:5173", "203.0.113.6:1", "")
	cookieA := a.Result().Cookies()[0].Value
	cookieB := b.Result().Cookies()[0].Value
	rec := request(h, http.MethodPost, "/api/v1/me/password", `{"current_password":"correct-horse","new_password":"new-horse-1"}`, "http://127.0.0.1:5173", "203.0.113.5:1", cookieA)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("change=%d %s", rec.Code, rec.Body.String())
	}
	meA := request(h, http.MethodGet, "/api/v1/me", "", "", "203.0.113.5:1", cookieA)
	if meA.Code != http.StatusOK {
		t.Fatalf("session A=%d %s", meA.Code, meA.Body.String())
	}
	meB := request(h, http.MethodGet, "/api/v1/me", "", "", "203.0.113.6:1", cookieB)
	assertCode(t, meB, http.StatusUnauthorized, "unauthenticated")
	old := request(h, http.MethodPost, "/api/v1/session", `{"username":"ada","password":"correct-horse"}`, "http://127.0.0.1:5173", "203.0.113.7:1", "")
	assertCode(t, old, http.StatusUnauthorized, "invalid_credentials")
	ok := request(h, http.MethodPost, "/api/v1/session", `{"username":"ada","password":"new-horse-1"}`, "http://127.0.0.1:5173", "203.0.113.7:1", "")
	if ok.Code != http.StatusOK {
		t.Fatalf("new login=%d %s", ok.Code, ok.Body.String())
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("sessions=%d want 2 (kept A plus new login)", n)
	}
}

func TestChangePasswordFields(t *testing.T) {
	db, h, _ := testHandler(t, false)
	login := request(h, http.MethodPost, "/api/v1/session", `{"username":"ada","password":"correct-horse"}`, "http://127.0.0.1:5173", "203.0.113.5:1", "")
	cookie := login.Result().Cookies()[0].Value
	origin := "http://127.0.0.1:5173"
	var hashBefore string
	if err := db.QueryRow(`SELECT password_hash FROM users WHERE id = 1`).Scan(&hashBefore); err != nil {
		t.Fatal(err)
	}
	wrong := request(h, http.MethodPost, "/api/v1/me/password", `{"current_password":"wrong-horse","new_password":"new-horse-1"}`, origin, "203.0.113.5:1", cookie)
	assertInvalidFields(t, wrong, map[string]string{"current_password": "当前密码不正确"})
	same := request(h, http.MethodPost, "/api/v1/me/password", `{"current_password":"correct-horse","new_password":"correct-horse"}`, origin, "203.0.113.5:1", cookie)
	assertInvalidFields(t, same, map[string]string{"new_password": "新密码不能与当前密码相同"})
	short := request(h, http.MethodPost, "/api/v1/me/password", `{"current_password":"correct-horse","new_password":"short"}`, origin, "203.0.113.5:1", cookie)
	assertInvalidFields(t, short, map[string]string{"new_password": "密码至少 8 个字符"})
	var hashAfter string
	if err := db.QueryRow(`SELECT password_hash FROM users WHERE id = 1`).Scan(&hashAfter); err != nil {
		t.Fatal(err)
	}
	if hashAfter != hashBefore {
		t.Fatal("hash changed after field errors")
	}
	other := request(h, http.MethodPost, "/api/v1/session", `{"username":"ada","password":"correct-horse"}`, origin, "203.0.113.8:1", "")
	if other.Code != http.StatusOK {
		t.Fatal("other session lost")
	}
	emptyCurrent := request(h, http.MethodPost, "/api/v1/me/password", `{"current_password":"","new_password":"new-horse-1"}`, origin, "203.0.113.5:1", cookie)
	assertInvalidFields(t, emptyCurrent, map[string]string{"current_password": "请输入当前密码"})
}

func TestAccountWriteGuardOrder(t *testing.T) {
	_, h, _ := testHandler(t, false)
	login := request(h, http.MethodPost, "/api/v1/session", `{"username":"ada","password":"correct-horse"}`, "http://127.0.0.1:5173", "203.0.113.5:1", "")
	cookie := login.Result().Cookies()[0].Value
	origin := "http://127.0.0.1:5173"
	oversize := strings.Repeat("a", 4100)
	bigUser := `{"username":"` + oversize + `"}`
	bigPass := `{"current_password":"correct-horse","new_password":"` + oversize + `"}`

	for _, tc := range []struct {
		method, path, body, origin, cookie string
		status                             int
		code                               string
	}{
		{http.MethodPatch, "/api/v1/me", `{"username":"kevin"}`, "http://evil.example", cookie, 403, "origin_rejected"},
		{http.MethodPost, "/api/v1/me/password", `{"current_password":"correct-horse","new_password":"new-horse-1"}`, "http://evil.example", cookie, 403, "origin_rejected"},
		{http.MethodPatch, "/api/v1/me", `{"username":"kevin"}`, origin, "", 401, "unauthenticated"},
		{http.MethodPost, "/api/v1/me/password", `{"current_password":"correct-horse","new_password":"new-horse-1"}`, origin, "", 401, "unauthenticated"},
		{http.MethodPatch, "/api/v1/me", bigUser, "http://evil.example", cookie, 403, "origin_rejected"},
		{http.MethodPost, "/api/v1/me/password", bigPass, "http://evil.example", cookie, 403, "origin_rejected"},
		{http.MethodPatch, "/api/v1/me", bigUser, origin, "", 401, "unauthenticated"},
		{http.MethodPost, "/api/v1/me/password", bigPass, origin, "", 401, "unauthenticated"},
		{http.MethodPut, "/api/v1/me", `{"username":"kevin"}`, origin, cookie, 404, "not_found"},
		{http.MethodPost, "/api/v1/me", `{"username":"kevin"}`, origin, cookie, 404, "not_found"},
		{http.MethodGet, "/api/v1/me/password", "", origin, cookie, 404, "not_found"},
	} {
		rec := request(h, tc.method, tc.path, tc.body, tc.origin, "203.0.113.5:1", tc.cookie)
		assertCode(t, rec, tc.status, tc.code)
	}

	query := request(h, http.MethodPatch, "/api/v1/me?foo=1", `{"username":"kevin"}`, origin, "203.0.113.5:1", cookie)
	assertInvalidFields(t, query, map[string]string{"foo": "不支持的参数"})
	queryPass := request(h, http.MethodPost, "/api/v1/me/password?foo=1", `{"current_password":"correct-horse","new_password":"new-horse-1"}`, origin, "203.0.113.5:1", cookie)
	assertInvalidFields(t, queryPass, map[string]string{"foo": "不支持的参数"})

	tooBig := request(h, http.MethodPatch, "/api/v1/me", bigUser, origin, "203.0.113.5:1", cookie)
	assertCode(t, tooBig, http.StatusRequestEntityTooLarge, "body_too_large")
	tooBigPass := request(h, http.MethodPost, "/api/v1/me/password", bigPass, origin, "203.0.113.5:1", cookie)
	assertCode(t, tooBigPass, http.StatusRequestEntityTooLarge, "body_too_large")

	me := request(h, http.MethodGet, "/api/v1/me", "", "", "203.0.113.5:1", cookie)
	if me.Code != http.StatusOK || !strings.Contains(me.Body.String(), `"username":"ada"`) {
		t.Fatalf("username changed after guard failures: %s", me.Body.String())
	}
}

func testHandler(t *testing.T, secure bool) (*sql.DB, http.Handler, time.Time) {
	t.Helper()
	db, h, now, _ := testHandlerDir(t, secure)
	return db, h, now
}

func testHandlerDir(t *testing.T, secure bool) (*sql.DB, http.Handler, time.Time, string) {
	t.Helper()
	dir := t.TempDir()
	if err := photo.EnsureDirs(dir); err != nil {
		t.Fatal(err)
	}
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
		DataDir:      dir,
		PublicOrigin: "http://127.0.0.1:5173",
		SessionTTL:   time.Hour,
		LoginLimit:   10,
		CookieSecure: secure,
	}, dummy, func() time.Time { return now })
	return db, h, now, dir
}

func request(h http.Handler, method, path, body, origin, remote, cookie string) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.RemoteAddr = remote
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

func migratedDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "youchu.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	var files []migrate.File
	err = fs.WalkDir(migrations.FS, ".", func(path string, d fs.DirEntry, err error) error {
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
	if err != nil {
		t.Fatal(err)
	}
	if err := migrate.Apply(context.Background(), db, files, time.Now()); err != nil {
		t.Fatal(err)
	}
	return db
}

func assertCode(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("status=%d body=%s err=%v", rec.Code, rec.Body.String(), err)
	}
	if rec.Code != status || body["code"] != code {
		t.Fatalf("status=%d code=%s body=%s", rec.Code, body["code"], rec.Body.String())
	}
}
