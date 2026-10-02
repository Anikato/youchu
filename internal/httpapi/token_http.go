package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"youchu/internal/auth"
)

type accessTokenJSON struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Token       string   `json:"token,omitempty"`
	TokenPrefix string   `json:"token_prefix"`
	Scopes      []string `json:"scopes"`
	CreatedAt   string   `json:"created_at"`
}

type accessTokenListJSON struct {
	Data []accessTokenJSON `json:"data"`
}

func (h *handler) tokensCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listAccessTokens(w, r)
	case http.MethodPost:
		h.createAccessToken(w, r)
	default:
		h.notFound(w, r)
	}
}

func (h *handler) tokenByID(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodDelete:
		h.deleteAccessToken(w, r)
	default:
		h.notFound(w, r)
	}
}

func (h *handler) listAccessTokens(w http.ResponseWriter, r *http.Request) {
	if !h.currentUser(w, r) {
		return
	}
	if r.URL.RawQuery != "" {
		writeFields(w, queryRejected(r))
		return
	}
	tokens, err := auth.ListAccessTokens(r.Context(), h.db)
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	data := make([]accessTokenJSON, 0, len(tokens))
	for _, tok := range tokens {
		data = append(data, toAccessTokenJSON(tok))
	}
	writeJSON(w, http.StatusOK, accessTokenListJSON{Data: data})
}

func (h *handler) createAccessToken(w http.ResponseWriter, r *http.Request) {
	_, body, ok := h.accountWrite(w, r)
	if !ok {
		return
	}
	rawName, rawScopes, scopesOK, err := decodeTokenWrite(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求格式不正确")
		return
	}
	fields := map[string]string{}
	name, err := auth.NormalizeTokenName(rawName)
	if err != nil {
		fields["name"] = err.Error()
	}
	var scopes []string
	if !scopesOK {
		fields["scopes"] = "权限范围不正确"
	} else {
		scopes, err = auth.ParseScopes(rawScopes)
		if err != nil {
			fields["scopes"] = "权限范围不正确"
		}
	}
	if len(fields) > 0 {
		writeFields(w, fields)
		return
	}
	tok, err := auth.CreateAccessToken(r.Context(), h.db, name, scopes, h.now())
	if err != nil {
		if errors.Is(err, auth.ErrTokenLimit) {
			writeError(w, http.StatusConflict, "token_limit", "最多 20 个接入令牌")
			return
		}
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, toAccessTokenJSON(tok))
}

func (h *handler) deleteAccessToken(w http.ResponseWriter, r *http.Request) {
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
	id, ok := parseDecimalID(r.PathValue("id"))
	if !ok {
		h.notFound(w, r)
		return
	}
	err := auth.DeleteAccessToken(r.Context(), h.db, id)
	if errors.Is(err, auth.ErrNotFound) {
		h.notFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func decodeTokenWrite(body []byte) (string, []string, bool, error) {
	obj, err := parseObject(body)
	if err != nil {
		return "", nil, false, err
	}
	rawName, ok := obj["name"]
	if !ok {
		return "", nil, false, errInvalidBody
	}
	name, err := decodeRequiredString(rawName)
	if err != nil {
		return "", nil, false, err
	}
	rawScopes, ok := obj["scopes"]
	if !ok {
		return name.Value, nil, false, nil
	}
	s := strings.TrimSpace(string(rawScopes))
	if s == "" || s == "null" || s[0] != '[' {
		return name.Value, nil, false, nil
	}
	var scopes []string
	if err := json.Unmarshal(rawScopes, &scopes); err != nil {
		return name.Value, nil, false, nil
	}
	return name.Value, scopes, true, nil
}

func toAccessTokenJSON(tok auth.AccessToken) accessTokenJSON {
	scopes := tok.Scopes
	if scopes == nil {
		scopes = []string{}
	}
	return accessTokenJSON{
		ID:          tok.ID,
		Name:        tok.Name,
		Token:       tok.Token,
		TokenPrefix: tok.TokenPrefix,
		Scopes:      scopes,
		CreatedAt:   tok.CreatedAt,
	}
}
