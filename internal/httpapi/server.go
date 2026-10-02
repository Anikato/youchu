package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"youchu/internal/auth"
	"youchu/internal/config"
	"youchu/internal/mcp"
	"youchu/internal/webui"
)

type windowHit struct {
	start time.Time
	n     int
}

type Limiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	now    func() time.Time
	hits   map[string]windowHit
}

func NewLimiter(limit int, now func() time.Time) *Limiter {
	return &Limiter{limit: limit, window: time.Minute, now: now, hits: map[string]windowHit{}}
}

func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	hit := l.hits[key]
	if hit.start.IsZero() || now.Sub(hit.start) >= l.window {
		hit = windowHit{start: now}
	}
	if hit.n >= l.limit {
		l.hits[key] = hit
		return false
	}
	hit.n++
	l.hits[key] = hit
	return true
}

type handler struct {
	db        *sql.DB
	cfg       config.Config
	dummyHash string
	now       func() time.Time
	limit     *Limiter
	mcp       http.Handler
	web       fs.FS
}

func New(db *sql.DB, cfg config.Config, dummyHash string, now func() time.Time) http.Handler {
	return newHandler(db, cfg, dummyHash, now, webui.Dist)
}

func newHandler(db *sql.DB, cfg config.Config, dummyHash string, now func() time.Time, web fs.FS) http.Handler {
	h := &handler{db: db, cfg: cfg, dummyHash: dummyHash, now: now, limit: NewLimiter(cfg.LoginLimit, now), mcp: mcp.New(db, now, cfg.DataDir), web: webRoot(web)}
	mux := http.NewServeMux()
	mux.Handle("/mcp", http.HandlerFunc(h.serveMCP))
	mux.HandleFunc("/api/v1/session", h.session)
	mux.HandleFunc("/api/v1/me", h.me)
	mux.HandleFunc("/api/v1/me/password", h.mePassword)
	mux.HandleFunc("/api/v1/locations", h.locationsCollection)
	mux.HandleFunc("/api/v1/locations/{id}", h.locationByID)
	mux.HandleFunc("/api/v1/items", h.itemsCollection)
	mux.HandleFunc("/api/v1/items/{id}", h.itemByID)
	mux.HandleFunc("/api/v1/categories", h.categoriesCollection)
	mux.HandleFunc("/api/v1/categories/{id}", h.categoryByID)
	mux.HandleFunc("/api/v1/trash", h.trashCollection)
	mux.HandleFunc("/api/v1/trash/{id}", h.trashByID)
	mux.HandleFunc("/api/v1/trash/{id}/restore", h.restoreTrash)
	mux.HandleFunc("/api/v1/items/{id}/return-tasks", h.itemReturnTasks)
	mux.HandleFunc("/api/v1/return-tasks", h.returnTasksCollection)
	mux.HandleFunc("/api/v1/return-tasks/{id}", h.returnTaskByID)
	mux.HandleFunc("/api/v1/return-tasks/{id}/complete", h.completeReturnTask)
	mux.HandleFunc("/api/v1/items/{id}/photos", h.itemPhotos)
	mux.HandleFunc("/api/v1/photos/{id}", h.photoByID)
	mux.HandleFunc("/api/v1/photos/{id}/first", h.photoFirst)
	mux.HandleFunc("/api/v1/photos/{id}/thumbnail", h.photoThumbnail)
	mux.HandleFunc("/api/v1/photos/{id}/original", h.photoOriginal)
	mux.HandleFunc("/api/v1/tokens", h.tokensCollection)
	mux.HandleFunc("/api/v1/tokens/{id}", h.tokenByID)
	mux.HandleFunc("/", h.spa)
	return mux
}

func webRoot(web fs.FS) fs.FS {
	if web == nil {
		return nil
	}
	if _, err := fs.Stat(web, "index.html"); err == nil {
		return web
	}
	if sub, err := fs.Sub(web, "dist"); err == nil {
		return sub
	}
	return web
}

func (h *handler) spa(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	if p == "/api" || strings.HasPrefix(p, "/api/") {
		h.notFound(w, r)
		return
	}
	if p == "/mcp" || strings.HasPrefix(p, "/mcp/") {
		h.notFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		h.notFound(w, r)
		return
	}
	if h.web == nil {
		h.notFound(w, r)
		return
	}
	index, err := h.web.Open("index.html")
	if err != nil {
		h.notFound(w, r)
		return
	}
	index.Close()
	rel := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if rel == "" || rel == "." {
		rel = "index.html"
	}
	if st, err := fs.Stat(h.web, rel); err == nil && !st.IsDir() {
		http.FileServer(http.FS(h.web)).ServeHTTP(w, r)
		return
	}
	if strings.Contains(path.Base(rel), ".") {
		h.notFound(w, r)
		return
	}
	data, err := fs.ReadFile(h.web, "index.html")
	if err != nil {
		h.notFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(data)
	}
}

func (h *handler) session(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.login(w, r)
	case http.MethodDelete:
		h.logout(w, r)
	default:
		h.notFound(w, r)
	}
}

func (h *handler) login(w http.ResponseWriter, r *http.Request) {
	if !h.originOK(w, r) {
		return
	}
	body, ok := readLimited(w, r, 4096)
	if !ok {
		return
	}
	if !h.limit.Allow(h.loginKey(r)) {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "登录过于频繁")
		return
	}
	var creds struct {
		Username *string `json:"username"`
		Password *string `json:"password"`
	}
	if err := json.Unmarshal(body, &creds); err != nil || creds.Username == nil || creds.Password == nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求格式不正确")
		return
	}
	username := strings.TrimSpace(*creds.Username)
	if username == "" || *creds.Password == "" {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求格式不正确")
		return
	}
	var hash string
	err := h.db.QueryRowContext(r.Context(), `SELECT password_hash FROM users WHERE username = ?`, username).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		auth.Verify(h.dummyHash, *creds.Password)
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "用户名或密码错误")
		return
	}
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	if !auth.Verify(hash, *creds.Password) {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "用户名或密码错误")
		return
	}
	token, err := auth.Create(r.Context(), h.db, h.cfg.SessionTTL, h.now())
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: "youchu_session", Value: token, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Secure: h.cfg.CookieSecure,
		MaxAge: int(h.cfg.SessionTTL.Seconds()),
	})
	writeJSON(w, http.StatusOK, map[string]string{"username": username})
}

func (h *handler) logout(w http.ResponseWriter, r *http.Request) {
	if !h.originOK(w, r) {
		return
	}
	if _, ok := readLimited(w, r, 4096); !ok {
		return
	}
	if c, err := r.Cookie("youchu_session"); err == nil {
		_ = auth.Delete(r.Context(), h.db, c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name: "youchu_session", Value: "", Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Secure: h.cfg.CookieSecure, MaxAge: -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) me(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getMe(w, r)
	case http.MethodPatch:
		h.patchMe(w, r)
	default:
		h.notFound(w, r)
	}
}

func (h *handler) mePassword(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.postPassword(w, r)
	default:
		h.notFound(w, r)
	}
}

func (h *handler) getMe(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("youchu_session")
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "未登录")
		return
	}
	name, err := auth.Lookup(r.Context(), h.db, c.Value, h.now())
	if errors.Is(err, auth.ErrUnauthenticated) {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "未登录")
		return
	}
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"username": name})
}

func (h *handler) patchMe(w http.ResponseWriter, r *http.Request) {
	_, body, ok := h.accountWrite(w, r)
	if !ok {
		return
	}
	var in struct {
		Username *string `json:"username"`
	}
	if err := json.Unmarshal(body, &in); err != nil || in.Username == nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求格式不正确")
		return
	}
	name := strings.TrimSpace(*in.Username)
	switch {
	case name == "":
		writeFields(w, map[string]string{"username": "用户名不能为空"})
		return
	case utf8.RuneCountInString(name) > 64:
		writeFields(w, map[string]string{"username": "用户名过长"})
		return
	}
	for _, ch := range name {
		if unicode.IsSpace(ch) {
			writeFields(w, map[string]string{"username": "用户名不能包含空白"})
			return
		}
	}
	if err := auth.UpdateUsername(r.Context(), h.db, name, h.now()); err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"username": name})
}

func (h *handler) postPassword(w http.ResponseWriter, r *http.Request) {
	token, body, ok := h.accountWrite(w, r)
	if !ok {
		return
	}
	var in struct {
		CurrentPassword *string `json:"current_password"`
		NewPassword     *string `json:"new_password"`
	}
	if err := json.Unmarshal(body, &in); err != nil || in.CurrentPassword == nil || in.NewPassword == nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求格式不正确")
		return
	}
	current := *in.CurrentPassword
	next := *in.NewPassword
	fields := map[string]string{}
	if current == "" {
		fields["current_password"] = "请输入当前密码"
	}
	n := utf8.RuneCountInString(next)
	switch {
	case n < 8:
		fields["new_password"] = "密码至少 8 个字符"
	case n > 128:
		fields["new_password"] = "密码过长"
	case next == current:
		fields["new_password"] = "新密码不能与当前密码相同"
	}
	if len(fields) > 0 {
		writeFields(w, fields)
		return
	}
	err := auth.ChangePassword(r.Context(), h.db, token, current, next, h.now())
	if errors.Is(err, auth.ErrCurrentPassword) {
		writeFields(w, map[string]string{"current_password": "当前密码不正确"})
		return
	}
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) accountWrite(w http.ResponseWriter, r *http.Request) (string, []byte, bool) {
	if !h.originOK(w, r) {
		return "", nil, false
	}
	c, err := r.Cookie("youchu_session")
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "未登录")
		return "", nil, false
	}
	if _, err = auth.Lookup(r.Context(), h.db, c.Value, h.now()); err != nil {
		if errors.Is(err, auth.ErrUnauthenticated) {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "未登录")
			return "", nil, false
		}
		http.Error(w, "", http.StatusInternalServerError)
		return "", nil, false
	}
	body, ok := readLimited(w, r, 4096)
	if !ok {
		return "", nil, false
	}
	if r.URL.RawQuery != "" {
		writeFields(w, queryRejected(r))
		return "", nil, false
	}
	return c.Value, body, true
}

func (h *handler) originOK(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("Origin") != h.cfg.PublicOrigin {
		writeError(w, http.StatusForbidden, "origin_rejected", "来源不被接受")
		return false
	}
	return true
}

func (h *handler) notFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "not_found", "未找到")
}

func readLimited(w http.ResponseWriter, r *http.Request, limit int) ([]byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, int64(limit))
	body, err := io.ReadAll(r.Body)
	if err == nil {
		return body, true
	}
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		writeError(w, http.StatusRequestEntityTooLarge, "body_too_large", "请求正文过大")
		return nil, false
	}
	writeError(w, http.StatusBadRequest, "invalid_body", "请求格式不正确")
	return nil, false
}

func (h *handler) loginKey(r *http.Request) string {
	if h.cfg.TrustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			first := strings.TrimSpace(strings.Split(xff, ",")[0])
			if first != "" {
				return clientKey(first)
			}
		}
	}
	return clientKey(r.RemoteAddr)
}

func clientKey(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
