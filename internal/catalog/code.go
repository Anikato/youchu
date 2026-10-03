package catalog

import (
	"math/big"
	"strings"
	"unicode/utf8"
)

func NextBoxCode(code string) (string, error) {
	runes := []rune(code)
	if len(runes) < 3 {
		return "", fieldError(map[string]string{"code": "编号无法递增"})
	}
	serial := string(runes[2:])
	for _, r := range serial {
		if r < '0' || r > '9' {
			return "", fieldError(map[string]string{"code": "编号无法递增"})
		}
	}
	n := new(big.Int)
	if _, ok := n.SetString(serial, 10); !ok {
		return "", fieldError(map[string]string{"code": "编号无法递增"})
	}
	n.Add(n, big.NewInt(1))
	next := n.Text(10)
	if len(next) < len(serial) {
		next = strings.Repeat("0", len(serial)-len(next)) + next
	}
	out := string(runes[:2]) + next
	if utf8.RuneCountInString(out) > 32 {
		return "", fieldError(map[string]string{"code": "编号过长"})
	}
	return out, nil
}
