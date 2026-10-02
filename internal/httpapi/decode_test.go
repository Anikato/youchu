package httpapi

import (
	"errors"
	"strings"
	"testing"
)

func TestDecodeItemLocations(t *testing.T) {
	cases := []struct {
		body    string
		typeErr bool
		nullSet bool
		set     bool
	}{
		{`{"name":"钳","locations":"x"}`, true, false, false},
		{`{"name":"钳","locations":[1]}`, true, false, false},
		{`{"name":"钳","locations":[null]}`, true, false, false},
		{`{"name":"钳","locations":null}`, false, true, true},
		{`{"name":"钳"}`, false, false, false},
		{`{"name":"钳","version":1.0}`, true, false, false},
		{`{"name":"钳","version":0}`, false, false, false},
	}
	for _, tc := range cases {
		got, err := decodeItemWrite([]byte(tc.body))
		if tc.typeErr {
			if !errors.Is(err, errInvalidBody) {
				t.Fatalf("%s err=%v", tc.body, err)
			}
			continue
		}
		if err != nil || got.LocationsNull != tc.nullSet || got.LocationsSet != tc.set {
			t.Fatalf("%s got=%+v err=%v", tc.body, got, err)
		}
	}
	loc, err := decodeLocationWrite([]byte(`{"parent_id":null}`))
	if err != nil || !loc.ParentNull {
		t.Fatalf("parent null: %+v %v", loc, err)
	}
}

func TestDecodeItemCategories(t *testing.T) {
	cases := []struct {
		body    string
		typeErr bool
		nullSet bool
		set     bool
	}{
		{`{"name":"钳","categories":"x"}`, true, false, false},
		{`{"name":"钳","categories":[1]}`, true, false, false},
		{`{"name":"钳","categories":[null]}`, true, false, false},
		{`{"name":"钳","categories":null}`, false, true, true},
		{`{"name":"钳"}`, false, false, false},
		{`{"name":"钳","categories":[]}`, false, false, true},
	}
	for _, tc := range cases {
		got, err := decodeItemWrite([]byte(tc.body))
		if tc.typeErr {
			if !errors.Is(err, errInvalidBody) {
				t.Fatalf("%s err=%v", tc.body, err)
			}
			continue
		}
		if err != nil || got.CategoriesNull != tc.nullSet || got.CategoriesSet != tc.set {
			t.Fatalf("%s got=%+v err=%v", tc.body, got, err)
		}
	}
}

func TestDecodeItemTextVersionAndLinks(t *testing.T) {
	got, err := decodeItemWrite([]byte("{\"name\":\"  钳  \",\"alias\":null,\"model\":\"M\",\"spec\":\"S\",\"quantity_note\":\"一\",\"note\":\"一\\n二\",\"ignored\":true}"))
	if err != nil {
		t.Fatal(err)
	}
	if !got.Name.Present || got.Name.Null || got.Name.Value != "  钳  " {
		t.Fatalf("name %+v", got.Name)
	}
	if !got.Alias.Present || !got.Alias.Null || got.Alias.Value != "" {
		t.Fatalf("alias %+v", got.Alias)
	}
	if !got.Model.Present || got.Model.Null || got.Model.Value != "M" {
		t.Fatalf("model %+v", got.Model)
	}
	if !got.Spec.Present || got.Spec.Value != "S" {
		t.Fatalf("spec %+v", got.Spec)
	}
	if !got.QuantityNote.Present || got.QuantityNote.Value != "一" {
		t.Fatalf("quantity %+v", got.QuantityNote)
	}
	if !got.Note.Present || got.Note.Null || got.Note.Value != "一\n二" {
		t.Fatalf("note %+v", got.Note)
	}
	if got.VersionPresent || got.LocationsSet || got.LocationsNull {
		t.Fatalf("flags %+v", got)
	}

	for _, body := range []string{
		`{"name":null}`,
		`{"name":1}`,
		`{"name":true}`,
		`{"name":[]}`,
		`{"name":{}}`,
		`{"alias":1}`,
		`{"alias":true}`,
		`{"alias":[]}`,
		`{"alias":{}}`,
		`{"model":1}`,
		`{"spec":false}`,
		`{"quantity_note":[]}`,
		`{"note":{}}`,
		`{"version":null}`,
		`{"version":"1"}`,
		`{"version":true}`,
		`{"version":[]}`,
		`{"version":{}}`,
		`{"version":1.5}`,
		`{"version":1e2}`,
		`{"version":1E2}`,
		`{"version":9223372036854775808}`,
		`{"locations":true}`,
		`{"locations":1}`,
		`{"locations":{}}`,
		`{"locations":[ "x" ]}`,
		`{"locations":[{"location_id":1.5}]}`,
		`{"locations":[{"location_id":1e1}]}`,
		`{"locations":[{"location_id":"1"}]}`,
		`{"locations":[{"location_id":true}]}`,
		`{"locations":[{"location_id":{}}]}`,
		`{"locations":[{"location_id":[]}]}`,
		`{"locations":[{"note":1}]}`,
		`[]`,
		`null`,
		`{"name":"钳"}{"name":"x"}`,
		``,
	} {
		if _, err := decodeItemWrite([]byte(body)); !errors.Is(err, errInvalidBody) {
			t.Fatalf("%s err=%v", body, err)
		}
	}

	got, err = decodeItemWrite([]byte(`{"version":-2}`))
	if err != nil || !got.VersionPresent || got.Version != -2 {
		t.Fatalf("neg version %+v %v", got, err)
	}
	got, err = decodeItemWrite([]byte("{\"locations\" : null}"))
	if err != nil || !got.LocationsSet || !got.LocationsNull {
		t.Fatalf("locations null space %+v %v", got, err)
	}
	got, err = decodeItemWrite([]byte(`{"model":null,"spec":null,"quantity_note":null,"note":null}`))
	if err != nil || !got.Model.Null || !got.Spec.Null || !got.QuantityNote.Null || !got.Note.Null {
		t.Fatalf("nullable text %+v %v", got, err)
	}
	got, err = decodeItemWrite([]byte("{\"locations\":\nnull}"))
	if err != nil || !got.LocationsSet || !got.LocationsNull {
		t.Fatalf("locations null newline %+v %v", got, err)
	}
	got, err = decodeItemWrite([]byte(`{"locations":[]}`))
	if err != nil || !got.LocationsSet || got.LocationsNull || len(got.Locations) != 0 {
		t.Fatalf("empty locations %+v %v", got, err)
	}
	got, err = decodeItemWrite([]byte(`{"locations":[{},{"location_id":null,"note":null},{"location_id":0},{"location_id":-4,"note":"  放  "}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !got.LocationsSet || got.LocationsNull || len(got.Locations) != 4 {
		t.Fatalf("links %+v", got)
	}
	if got.Locations[0].IDPresent || got.Locations[0].IDNull || got.Locations[0].Note.Present {
		t.Fatalf("missing id %+v", got.Locations[0])
	}
	if !got.Locations[1].IDPresent || !got.Locations[1].IDNull || !got.Locations[1].Note.Present || !got.Locations[1].Note.Null {
		t.Fatalf("null id %+v", got.Locations[1])
	}
	if !got.Locations[2].IDPresent || got.Locations[2].IDNull || got.Locations[2].ID != 0 {
		t.Fatalf("zero id %+v", got.Locations[2])
	}
	if got.Locations[3].ID != -4 || got.Locations[3].Note.Value != "  放  " || got.Locations[3].Note.Null {
		t.Fatalf("neg id %+v", got.Locations[3])
	}

	long := strings.Repeat("名", 81)
	got, err = decodeItemWrite([]byte(`{"name":"` + long + `"}`))
	if err != nil || got.Name.Value != long {
		t.Fatalf("decoder does not validate length: %v %q", err, got.Name.Value)
	}
}

func TestDecodeLocationWrite(t *testing.T) {
	loc, err := decodeLocationWrite([]byte(`{"parent_id":null}`))
	if err != nil || !loc.ParentNull || !loc.ParentPresent || loc.ParentID != 0 {
		t.Fatalf("parent null %+v %v", loc, err)
	}
	loc, err = decodeLocationWrite([]byte(`{}`))
	if err != nil || loc.ParentNull || loc.ParentPresent || loc.VersionPresent || loc.Name.Present || loc.Code.Present {
		t.Fatalf("empty %+v %v", loc, err)
	}
	loc, err = decodeLocationWrite([]byte(`{"parent_id":0,"name":"  厨房  ","type":"area","code":null,"version":0}`))
	if err != nil {
		t.Fatal(err)
	}
	if loc.ParentNull || !loc.ParentPresent || loc.ParentID != 0 {
		t.Fatalf("parent 0 %+v", loc)
	}
	if !loc.Name.Present || loc.Name.Null || loc.Name.Value != "  厨房  " {
		t.Fatalf("name %+v", loc.Name)
	}
	if !loc.Type.Present || loc.Type.Value != "area" {
		t.Fatalf("type %+v", loc.Type)
	}
	if !loc.Code.Present || !loc.Code.Null {
		t.Fatalf("code %+v", loc.Code)
	}
	if !loc.VersionPresent || loc.Version != 0 {
		t.Fatalf("version %+v", loc)
	}
	loc, err = decodeLocationWrite([]byte(`{"parent_id":-5,"code":" 001 "}`))
	if err != nil || loc.ParentNull || loc.ParentID != -5 || loc.Code.Value != " 001 " || loc.Code.Null {
		t.Fatalf("parent neg %+v %v", loc, err)
	}

	for _, body := range []string{
		`{"parent_id":1.0}`,
		`{"parent_id":"1"}`,
		`{"parent_id":true}`,
		`{"parent_id":[]}`,
		`{"parent_id":{}}`,
		`{"parent_id":1e2}`,
		`{"name":null}`,
		`{"name":1}`,
		`{"name":true}`,
		`{"type":null}`,
		`{"type":false}`,
		`{"type":[]}`,
		`{"code":1}`,
		`{"code":true}`,
		`{"version":1.0}`,
		`{"version":null}`,
		`[]`,
		`null`,
	} {
		if _, err := decodeLocationWrite([]byte(body)); !errors.Is(err, errInvalidBody) {
			t.Fatalf("%s err=%v", body, err)
		}
	}
}
