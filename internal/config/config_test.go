package config

import (
	"testing"
	"time"
)

func getenvFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func validEnv() map[string]string {
	return map[string]string{
		"YOUCHU_PUBLIC_ORIGIN": "http://127.0.0.1:5173",
		"YOUCHU_USERNAME":      "ada",
		"YOUCHU_PASSWORD":      "correct-horse",
	}
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(getenvFrom(validEnv()))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != "127.0.0.1:8080" || cfg.DataDir != "./data" || cfg.CookieSecure || cfg.TrustProxy {
		t.Fatalf("defaults: %+v", cfg)
	}
	if cfg.SessionTTL != 336*time.Hour || cfg.LoginLimit != 10 {
		t.Fatalf("ttl or limit: %+v", cfg)
	}
	if cfg.PublicOrigin != "http://127.0.0.1:5173" {
		t.Fatal(cfg.PublicOrigin)
	}
}

func TestLoadRejectsBadValues(t *testing.T) {
	cases := []map[string]string{
		{"YOUCHU_SESSION_TTL": "0s"},
		{"YOUCHU_SESSION_TTL": "-1s"},
		{"YOUCHU_SESSION_TTL": "500ms"},
		{"YOUCHU_LOGIN_LIMIT": "0"},
		{"YOUCHU_LOGIN_LIMIT": "-1"},
		{"YOUCHU_LOGIN_LIMIT": "1.5"},
		{"YOUCHU_COOKIE_SECURE": "yes"},
		{"YOUCHU_TRUST_PROXY": "yes"},
		{"YOUCHU_PUBLIC_ORIGIN": "http://127.0.0.1:5173/"},
		{"YOUCHU_PUBLIC_ORIGIN": "http://127.0.0.1:5173/app"},
		{},
	}
	for _, override := range cases {
		env := validEnv()
		for k, v := range override {
			env[k] = v
		}
		if _, ok := override["YOUCHU_PUBLIC_ORIGIN"]; !ok && len(override) == 0 {
			delete(env, "YOUCHU_PUBLIC_ORIGIN")
		}
		if _, err := Load(getenvFrom(env)); err == nil {
			t.Fatalf("accepted %v", override)
		}
	}
}

func TestValidateNewUser(t *testing.T) {
	name, err := ValidateNewUser("  ada  ", "correct-horse")
	if err != nil || name != "ada" {
		t.Fatalf("name=%q err=%v", name, err)
	}
	if _, err := ValidateNewUser("a b", "correct-horse"); err == nil {
		t.Fatal("accepted internal space")
	}
	if _, err := ValidateNewUser("ada", "short"); err == nil {
		t.Fatal("accepted short password")
	}
	long := make([]rune, 129)
	for i := range long {
		long[i] = 'a'
	}
	if _, err := ValidateNewUser("ada", string(long)); err == nil {
		t.Fatal("accepted long password")
	}
}
