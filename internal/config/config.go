package config

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type Config struct {
	HTTPAddr     string
	DataDir      string
	PublicOrigin string
	CookieSecure bool
	TrustProxy   bool
	SessionTTL   time.Duration
	LoginLimit   int
	Username     string
	Password     string
}

func Load(getenv func(string) string) (Config, error) {
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	cfg := Config{
		HTTPAddr:   "127.0.0.1:8080",
		DataDir:    "./data",
		SessionTTL: 336 * time.Hour,
		LoginLimit: 10,
	}
	if v := getenv("YOUCHU_HTTP_ADDR"); v != "" {
		cfg.HTTPAddr = v
	}
	if v := getenv("YOUCHU_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	origin := getenv("YOUCHU_PUBLIC_ORIGIN")
	if err := validateOrigin(origin); err != nil {
		return Config{}, err
	}
	cfg.PublicOrigin = origin
	secure, err := parseBool(getenv("YOUCHU_COOKIE_SECURE"))
	if err != nil {
		return Config{}, fmt.Errorf("YOUCHU_COOKIE_SECURE: %w", err)
	}
	cfg.CookieSecure = secure
	trust, err := parseBool(getenv("YOUCHU_TRUST_PROXY"))
	if err != nil {
		return Config{}, fmt.Errorf("YOUCHU_TRUST_PROXY: %w", err)
	}
	cfg.TrustProxy = trust
	if v := getenv("YOUCHU_SESSION_TTL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d < time.Second {
			return Config{}, fmt.Errorf("YOUCHU_SESSION_TTL must be at least 1s")
		}
		cfg.SessionTTL = d
	}
	if v := getenv("YOUCHU_LOGIN_LIMIT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return Config{}, fmt.Errorf("YOUCHU_LOGIN_LIMIT must be an integer >= 1")
		}
		cfg.LoginLimit = n
	}
	cfg.Username = getenv("YOUCHU_USERNAME")
	cfg.Password = getenv("YOUCHU_PASSWORD")
	return cfg, nil
}

func parseBool(v string) (bool, error) {
	switch v {
	case "", "false":
		return false, nil
	case "true":
		return true, nil
	default:
		return false, fmt.Errorf("must be true or false")
	}
}

func validateOrigin(origin string) error {
	u, err := url.Parse(origin)
	if err != nil || u.Scheme == "" || u.Host == "" || origin != u.Scheme+"://"+u.Host {
		return fmt.Errorf("YOUCHU_PUBLIC_ORIGIN must be scheme://host with no path, query, or trailing slash")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("YOUCHU_PUBLIC_ORIGIN scheme must be http or https")
	}
	return nil
}

func ValidateNewUser(username, password string) (string, error) {
	username = strings.TrimSpace(username)
	if username == "" || utf8.RuneCountInString(username) > 64 {
		return "", fmt.Errorf("invalid username")
	}
	for _, r := range username {
		if unicode.IsSpace(r) {
			return "", fmt.Errorf("invalid username")
		}
	}
	if n := utf8.RuneCountInString(password); n < 8 || n > 128 {
		return "", fmt.Errorf("invalid password")
	}
	return username, nil
}
