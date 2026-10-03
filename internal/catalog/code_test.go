package catalog

import (
	"errors"
	"testing"
)

func TestNextBoxCode(t *testing.T) {
	got, err := NextBoxCode("B102")
	if err != nil || got != "B103" {
		t.Fatalf("B102: got=%q err=%v", got, err)
	}
	got, err = NextBoxCode("B109")
	if err != nil || got != "B110" {
		t.Fatalf("B109: got=%q err=%v", got, err)
	}
	got, err = NextBoxCode("B199")
	if err != nil || got != "B1100" {
		t.Fatalf("B199: got=%q err=%v", got, err)
	}
	got, err = NextBoxCode("B112")
	if err != nil || got != "B113" {
		t.Fatalf("B112: got=%q err=%v", got, err)
	}
}

func TestNextBoxCodeRejects(t *testing.T) {
	_, err := NextBoxCode("B1")
	var fe *FieldError
	if !errors.As(err, &fe) || fe.Fields["code"] != "编号无法递增" {
		t.Fatalf("short err=%v", err)
	}
	_, err = NextBoxCode("B1AB")
	if !errors.As(err, &fe) || fe.Fields["code"] != "编号无法递增" {
		t.Fatalf("letters err=%v", err)
	}
}
