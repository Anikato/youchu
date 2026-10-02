package catalog

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

func NormalizeName(s string) (string, error) {
	return normalizeText(s, 80, "请填写名称", "名称过长", "名称不能包含控制字符", true, false, false)
}

func NormalizeCode(s string) (string, error) {
	return normalizeText(s, 32, "请填写编号", "编号过长", "编号不能包含控制字符", true, false, true)
}

func NormalizeNote(s string) (string, error) {
	return normalizeText(s, 2000, "", "备注过长", "备注不能包含控制字符", false, true, false)
}

func NormalizeAlias(s string) (string, error) {
	return normalizeText(s, 200, "", "别名过长", "别名不能包含控制字符", false, false, false)
}

func NormalizeModel(s string) (string, error) {
	return normalizeText(s, 120, "", "型号过长", "型号不能包含控制字符", false, false, false)
}

func NormalizeSpec(s string) (string, error) {
	return normalizeText(s, 200, "", "规格过长", "规格不能包含控制字符", false, false, false)
}

func NormalizeQuantityNote(s string) (string, error) {
	return normalizeText(s, 200, "", "数量说明过长", "数量说明不能包含控制字符", false, false, false)
}

func NormalizePlacementNote(s string) (string, error) {
	return normalizeText(s, 2000, "", "放置说明过长", "放置说明不能包含控制字符", false, true, false)
}

func NormalizeKeyword(s string) (string, error) {
	return normalizeText(s, 200, "", "关键词过长", "关键词不能包含控制字符", false, false, false)
}

func NormalizePartNote(s string) (string, error) {
	return normalizeText(s, 200, "", "配件说明过长", "配件说明不能包含控制字符", false, false, false)
}

func NormalizeReason(s string) (string, error) {
	return normalizeText(s, 200, "", "原因过长", "原因不能包含控制字符", false, false, false)
}

func NormalizeDestinationNote(s string) (string, error) {
	return normalizeText(s, 2000, "", "临时去向过长", "临时去向不能包含控制字符", false, true, false)
}

// Control characters are judged on the raw text. TrimSpace would hide a trailing newline.
func normalizeText(s string, max int, emptyMsg, longMsg, ctrlMsg string, required, allowBreaks, rejectSpace bool) (string, error) {
	for _, r := range s {
		if !unicode.Is(unicode.Cc, r) {
			continue
		}
		if allowBreaks && (r == '\n' || r == '\r') {
			continue
		}
		return "", errors.New(ctrlMsg)
	}
	s = strings.TrimSpace(s)
	if s == "" {
		if required {
			return "", errors.New(emptyMsg)
		}
		return "", nil
	}
	if utf8.RuneCountInString(s) > max {
		return "", errors.New(longMsg)
	}
	if rejectSpace {
		for _, r := range s {
			if unicode.IsSpace(r) {
				return "", errors.New("编号不能包含空白")
			}
		}
	}
	return s, nil
}
