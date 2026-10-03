package catalog

import "testing"

const simpleSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M4 8h16v12H4z" fill="#000"/></svg>`

func TestSanitizeSVGKeepsPathAndRewritesFill(t *testing.T) {
	got, msg := SanitizeSVG(simpleSVG)
	if msg != "" {
		t.Fatalf("msg=%q got=%q", msg, got)
	}
	if !containsAll(got, `<svg`, `viewBox="0 0 24 24"`, `<path`, `currentColor`) {
		t.Fatalf("got=%s", got)
	}
	if containsAll(got, `#000`) {
		t.Fatalf("left raw fill: %s", got)
	}
}

func TestSanitizeSVGRejectsScript(t *testing.T) {
	_, msg := SanitizeSVG(`<svg viewBox="0 0 24 24"><script>alert(1)</script><path d="M1 1h1"/></svg>`)
	if msg != "图标不正确" {
		t.Fatalf("msg=%q", msg)
	}
}

func TestSanitizeSVGRejectsEmptyAndHuge(t *testing.T) {
	if _, msg := SanitizeSVG("  "); msg != "图标不正确" {
		t.Fatalf("empty msg=%q", msg)
	}
	huge := `<svg viewBox="0 0 24 24"><path d="` + string(make([]byte, 20000)) + `"/></svg>`
	if _, msg := SanitizeSVG(huge); msg != "图标过大" {
		t.Fatalf("huge msg=%q", msg)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !containsFold(s, p) {
			return false
		}
	}
	return true
}

func containsFold(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || stringIndex(s, sub) >= 0)
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
