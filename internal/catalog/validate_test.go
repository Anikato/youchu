package catalog

import (
	"strings"
	"testing"
)

func TestValidateNameCodeNote(t *testing.T) {
	_, err := NormalizeName("  厨房\n")
	if err == nil || err.Error() != "名称不能包含控制字符" {
		t.Fatalf("name control: %v", err)
	}
	got, err := NormalizeName("  厨房  ")
	if err != nil || got != "厨房" {
		t.Fatalf("name trim: %q %v", got, err)
	}
	_, err = NormalizeName(strings.Repeat("a", 81))
	if err == nil || err.Error() != "名称过长" {
		t.Fatalf("name long: %v", err)
	}
	got, err = NormalizeCode(" 001 ")
	if err != nil || got != "001" {
		t.Fatalf("code zeros: %q %v", got, err)
	}
	_, err = NormalizeCode("0 01")
	if err == nil || err.Error() != "编号不能包含空白" {
		t.Fatalf("code space: %v", err)
	}
	got, err = NormalizeNote("一行\n二行")
	if err != nil || got != "一行\n二行" {
		t.Fatalf("note newline: %q %v", got, err)
	}
	_, err = NormalizeNote("备\u0000注")
	if err == nil || err.Error() != "备注不能包含控制字符" {
		t.Fatalf("note nul: %v", err)
	}
}

func TestValidateTextRules(t *testing.T) {
	got, err := NormalizeName(strings.Repeat("a", 80))
	if err != nil || got != strings.Repeat("a", 80) {
		t.Fatalf("name 80: %q %v", got, err)
	}
	got, err = NormalizeName(strings.Repeat("厨", 80))
	if err != nil || got != strings.Repeat("厨", 80) {
		t.Fatalf("name 80 runes: %q %v", got, err)
	}
	_, err = NormalizeName(strings.Repeat("厨", 81))
	if err == nil || err.Error() != "名称过长" {
		t.Fatalf("name 81 runes: %v", err)
	}
	_, err = NormalizeName("")
	if err == nil || err.Error() != "请填写名称" {
		t.Fatalf("name empty: %v", err)
	}
	_, err = NormalizeName("   ")
	if err == nil || err.Error() != "请填写名称" {
		t.Fatalf("name spaces: %v", err)
	}
	_, err = NormalizeName(strings.Repeat(" ", 81))
	if err == nil || err.Error() != "请填写名称" {
		t.Fatalf("name only spaces long: %v", err)
	}
	_, err = NormalizeName("\n")
	if err == nil || err.Error() != "名称不能包含控制字符" {
		t.Fatalf("name only newline: %v", err)
	}
	_, err = NormalizeName("\n" + strings.Repeat("a", 81))
	if err == nil || err.Error() != "名称不能包含控制字符" {
		t.Fatalf("name control before length: %v", err)
	}
	got, err = NormalizeName("  厨 房  ")
	if err != nil || got != "厨 房" {
		t.Fatalf("name internal space: %q %v", got, err)
	}

	_, err = NormalizeCode("")
	if err == nil || err.Error() != "请填写编号" {
		t.Fatalf("code empty: %v", err)
	}
	_, err = NormalizeCode("   ")
	if err == nil || err.Error() != "请填写编号" {
		t.Fatalf("code spaces: %v", err)
	}
	_, err = NormalizeCode(strings.Repeat(" ", 33))
	if err == nil || err.Error() != "请填写编号" {
		t.Fatalf("code only spaces long: %v", err)
	}
	got, err = NormalizeCode(strings.Repeat("a", 32))
	if err != nil || got != strings.Repeat("a", 32) {
		t.Fatalf("code 32: %q %v", got, err)
	}
	_, err = NormalizeCode(strings.Repeat("a", 33))
	if err == nil || err.Error() != "编号过长" {
		t.Fatalf("code 33: %v", err)
	}
	_, err = NormalizeCode(strings.Repeat("a", 20) + " " + strings.Repeat("b", 20))
	if err == nil || err.Error() != "编号过长" {
		t.Fatalf("code long before space: %v", err)
	}
	_, err = NormalizeCode("0\x0001")
	if err == nil || err.Error() != "编号不能包含控制字符" {
		t.Fatalf("code control: %v", err)
	}
	_, err = NormalizeCode("00\u00a01")
	if err == nil || err.Error() != "编号不能包含空白" {
		t.Fatalf("code nbsp: %v", err)
	}
	_, err = NormalizeCode("0\n1")
	if err == nil || err.Error() != "编号不能包含控制字符" {
		t.Fatalf("code newline: %v", err)
	}

	got, err = NormalizeNote("  一行\n二行  ")
	if err != nil || got != "一行\n二行" {
		t.Fatalf("note trim keeps inner newline: %q %v", got, err)
	}
	got, err = NormalizeNote("一\r\n二")
	if err != nil || got != "一\r\n二" {
		t.Fatalf("note crlf: %q %v", got, err)
	}
	got, err = NormalizeNote(" \n\r ")
	if err != nil || got != "" {
		t.Fatalf("note blank: %q %v", got, err)
	}
	got, err = NormalizeNote(strings.Repeat("注", 2000))
	if err != nil || got != strings.Repeat("注", 2000) {
		t.Fatalf("note 2000: len %d %v", len([]rune(got)), err)
	}
	_, err = NormalizeNote(strings.Repeat("注", 2001))
	if err == nil || err.Error() != "备注过长" {
		t.Fatalf("note 2001: %v", err)
	}
	_, err = NormalizeNote("一\t二")
	if err == nil || err.Error() != "备注不能包含控制字符" {
		t.Fatalf("note tab: %v", err)
	}

	checkOptional := func(name string, fn func(string) (string, error), max int, longMsg, ctrlMsg string, allowBreak bool) {
		t.Helper()
		got, err := fn("  x  ")
		if err != nil || got != "x" {
			t.Fatalf("%s trim: %q %v", name, got, err)
		}
		got, err = fn("   ")
		if err != nil || got != "" {
			t.Fatalf("%s blank: %q %v", name, got, err)
		}
		got, err = fn(strings.Repeat("a", max))
		if err != nil || got != strings.Repeat("a", max) {
			t.Fatalf("%s max: %v", name, err)
		}
		_, err = fn(strings.Repeat("a", max+1))
		if err == nil || err.Error() != longMsg {
			t.Fatalf("%s long: %v", name, err)
		}
		_, err = fn("a\x00b")
		if err == nil || err.Error() != ctrlMsg {
			t.Fatalf("%s nul: %v", name, err)
		}
		_, err = fn("\n")
		if allowBreak {
			if err != nil {
				t.Fatalf("%s newline: %v", name, err)
			}
			return
		}
		if err == nil || err.Error() != ctrlMsg {
			t.Fatalf("%s newline: %v", name, err)
		}
	}
	checkOptional("alias", NormalizeAlias, 200, "别名过长", "别名不能包含控制字符", false)
	checkOptional("model", NormalizeModel, 120, "型号过长", "型号不能包含控制字符", false)
	checkOptional("spec", NormalizeSpec, 200, "规格过长", "规格不能包含控制字符", false)
	checkOptional("quantity", NormalizeQuantityNote, 200, "数量说明过长", "数量说明不能包含控制字符", false)
	checkOptional("part_note", NormalizePartNote, 200, "配件说明过长", "配件说明不能包含控制字符", false)
	checkOptional("reason", NormalizeReason, 200, "原因过长", "原因不能包含控制字符", false)

	got, err = NormalizePlacementNote("  一层\n二层  ")
	if err != nil || got != "一层\n二层" {
		t.Fatalf("placement newline: %q %v", got, err)
	}
	got, err = NormalizePlacementNote(" \r ")
	if err != nil || got != "" {
		t.Fatalf("placement blank: %q %v", got, err)
	}
	_, err = NormalizePlacementNote(strings.Repeat("放", 2001))
	if err == nil || err.Error() != "放置说明过长" {
		t.Fatalf("placement long: %v", err)
	}
	got, err = NormalizePlacementNote(strings.Repeat("放", 2000))
	if err != nil || utf8RuneCount(got) != 2000 {
		t.Fatalf("placement 2000: %v", err)
	}
	_, err = NormalizePlacementNote("放\x00")
	if err == nil || err.Error() != "放置说明不能包含控制字符" {
		t.Fatalf("placement nul: %v", err)
	}
	_, err = NormalizePlacementNote("放\t")
	if err == nil || err.Error() != "放置说明不能包含控制字符" {
		t.Fatalf("placement tab: %v", err)
	}

	got, err = NormalizeDestinationNote("  楼上\n楼下  ")
	if err != nil || got != "楼上\n楼下" {
		t.Fatalf("destination newline: %q %v", got, err)
	}
	got, err = NormalizeDestinationNote(" \r ")
	if err != nil || got != "" {
		t.Fatalf("destination blank: %q %v", got, err)
	}
	got, err = NormalizeDestinationNote(strings.Repeat("去", 2000))
	if err != nil || utf8RuneCount(got) != 2000 {
		t.Fatalf("destination 2000: %v", err)
	}
	_, err = NormalizeDestinationNote(strings.Repeat("去", 2001))
	if err == nil || err.Error() != "临时去向过长" {
		t.Fatalf("destination long: %v", err)
	}
	_, err = NormalizeDestinationNote("去\x00")
	if err == nil || err.Error() != "临时去向不能包含控制字符" {
		t.Fatalf("destination nul: %v", err)
	}
	_, err = NormalizeDestinationNote("去\t")
	if err == nil || err.Error() != "临时去向不能包含控制字符" {
		t.Fatalf("destination tab: %v", err)
	}
}

func TestNormalizeKeyword(t *testing.T) {
	got, err := NormalizeKeyword("  usb  ")
	if err != nil || got != "usb" {
		t.Fatalf("trim: %q %v", got, err)
	}
	_, err = NormalizeKeyword("usb\n")
	if err == nil || err.Error() != "关键词不能包含控制字符" {
		t.Fatalf("newline: %v", err)
	}
	_, err = NormalizeKeyword(strings.Repeat("a", 201))
	if err == nil || err.Error() != "关键词过长" {
		t.Fatalf("long: %v", err)
	}
	got, err = NormalizeKeyword("   ")
	if err != nil || got != "" {
		t.Fatalf("blank: %q %v", got, err)
	}
}

func utf8RuneCount(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}
