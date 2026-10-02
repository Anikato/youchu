package httpapi

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"youchu/internal/auth"
	"youchu/internal/mcp"
)

func (h *handler) serveMCP(w http.ResponseWriter, r *http.Request) {
	if !h.mcpOriginOK(w, r) {
		return
	}
	tok, ok := h.mcpBearer(w, r)
	if !ok {
		return
	}
	r = r.WithContext(mcp.WithAccessToken(r.Context(), tok))
	h.mcp.ServeHTTP(w, r)
}

func (h *handler) mcpOriginOK(w http.ResponseWriter, r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	if origin == h.cfg.PublicOrigin {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Hostname() == "" {
		writeError(w, http.StatusForbidden, "origin_rejected", "来源不被接受")
		return false
	}
	host := u.Hostname()
	if host == "127.0.0.1" || host == "localhost" {
		return true
	}
	if pub, err := url.Parse(h.cfg.PublicOrigin); err == nil && host == pub.Hostname() {
		return true
	}
	writeError(w, http.StatusForbidden, "origin_rejected", "来源不被接受")
	return false
}

func (h *handler) mcpBearer(w http.ResponseWriter, r *http.Request) (auth.AccessToken, bool) {
	token, ok := parseBearer(r.Header.Get("Authorization"))
	if !ok {
		mcpUnauthenticated(w)
		return auth.AccessToken{}, false
	}
	tok, err := auth.LookupAccessToken(r.Context(), h.db, token)
	if errors.Is(err, auth.ErrUnauthenticated) {
		mcpUnauthenticated(w)
		return auth.AccessToken{}, false
	}
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		return auth.AccessToken{}, false
	}
	return tok, true
}

func parseBearer(header string) (string, bool) {
	scheme, token, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return "", false
	}
	return token, true
}

func mcpUnauthenticated(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="youchu"`)
	writeError(w, http.StatusUnauthorized, "unauthenticated", "未登录")
}
