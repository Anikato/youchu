package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

var errInvalidBody = errors.New("invalid body")

type textValue struct {
	Present bool
	Null    bool
	Value   string
}

type itemLink struct {
	IDPresent bool
	IDNull    bool
	ID        int64
	Note      textValue
}

type itemWrite struct {
	Name           textValue
	Alias          textValue
	Model          textValue
	Spec           textValue
	QuantityNote   textValue
	Note           textValue
	VersionPresent bool
	Version        int64
	LocationsSet   bool
	LocationsNull  bool
	Locations      []itemLink
	CategoriesSet  bool
	CategoriesNull bool
	Categories     []itemLink
}

type locationWrite struct {
	Name           textValue
	Type           textValue
	Code           textValue
	ParentPresent  bool
	ParentNull     bool
	ParentID       int64
	Icon           textValue
	CustomPresent  bool
	CustomNull     bool
	CustomID       int64
	VersionPresent bool
	Version        int64
}

type categoryWrite struct {
	Name           textValue
	ParentPresent  bool
	ParentNull     bool
	ParentID       int64
	VersionPresent bool
	Version        int64
}

func parseObject(body []byte) (map[string]json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var obj map[string]json.RawMessage
	if err := dec.Decode(&obj); err != nil || obj == nil {
		return nil, errInvalidBody
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, errInvalidBody
	}
	return obj, nil
}

type versionWrite struct {
	VersionPresent bool
	Version        int64
}

type returnTaskWrite struct {
	PartNote        textValue
	Reason          textValue
	DestinationNote textValue
}

func decodeReturnTaskWrite(body []byte) (returnTaskWrite, error) {
	obj, err := parseObject(body)
	if err != nil {
		return returnTaskWrite{}, err
	}
	var out returnTaskWrite
	nullable := []struct {
		key string
		dst *textValue
	}{
		{"part_note", &out.PartNote},
		{"reason", &out.Reason},
		{"destination_note", &out.DestinationNote},
	}
	for _, f := range nullable {
		raw, ok := obj[f.key]
		if !ok {
			continue
		}
		*f.dst, err = decodeNullableString(raw)
		if err != nil {
			return returnTaskWrite{}, err
		}
	}
	return out, nil
}

func decodeVersionWrite(body []byte) (versionWrite, error) {
	obj, err := parseObject(body)
	if err != nil {
		return versionWrite{}, err
	}
	var out versionWrite
	if raw, ok := obj["version"]; ok {
		out.Version, err = decodeInteger(raw)
		if err != nil {
			return versionWrite{}, err
		}
		out.VersionPresent = true
	}
	return out, nil
}

func decodeItemWrite(body []byte) (itemWrite, error) {
	obj, err := parseObject(body)
	if err != nil {
		return itemWrite{}, err
	}
	var out itemWrite
	if raw, ok := obj["name"]; ok {
		out.Name, err = decodeRequiredString(raw)
		if err != nil {
			return itemWrite{}, err
		}
	}
	nullable := []struct {
		key string
		dst *textValue
	}{
		{"alias", &out.Alias},
		{"model", &out.Model},
		{"spec", &out.Spec},
		{"quantity_note", &out.QuantityNote},
		{"note", &out.Note},
	}
	for _, f := range nullable {
		raw, ok := obj[f.key]
		if !ok {
			continue
		}
		*f.dst, err = decodeNullableString(raw)
		if err != nil {
			return itemWrite{}, err
		}
	}
	if raw, ok := obj["version"]; ok {
		out.Version, err = decodeInteger(raw)
		if err != nil {
			return itemWrite{}, err
		}
		out.VersionPresent = true
	}
	if raw, ok := obj["locations"]; ok {
		set, isNull, links, err := decodeIDLinks(raw, "location_id")
		if err != nil {
			return itemWrite{}, err
		}
		out.LocationsSet = set
		out.LocationsNull = isNull
		out.Locations = links
	}
	if raw, ok := obj["categories"]; ok {
		set, isNull, links, err := decodeIDLinks(raw, "category_id")
		if err != nil {
			return itemWrite{}, err
		}
		out.CategoriesSet = set
		out.CategoriesNull = isNull
		out.Categories = links
	}
	return out, nil
}

type locationIconWrite struct {
	Name           textValue
	SVG            textValue
	VersionPresent bool
	Version        int64
}

func decodeLocationIconWrite(body []byte) (locationIconWrite, error) {
	obj, err := parseObject(body)
	if err != nil {
		return locationIconWrite{}, err
	}
	var out locationIconWrite
	if raw, ok := obj["name"]; ok {
		out.Name, err = decodeNullableString(raw)
		if err != nil {
			return locationIconWrite{}, err
		}
	}
	if raw, ok := obj["svg"]; ok {
		out.SVG, err = decodeNullableString(raw)
		if err != nil {
			return locationIconWrite{}, err
		}
	}
	if raw, ok := obj["version"]; ok {
		out.Version, err = decodeInteger(raw)
		if err != nil {
			return locationIconWrite{}, err
		}
		out.VersionPresent = true
	}
	return out, nil
}

func decodeLocationWrite(body []byte) (locationWrite, error) {
	obj, err := parseObject(body)
	if err != nil {
		return locationWrite{}, err
	}
	var out locationWrite
	if raw, ok := obj["name"]; ok {
		out.Name, err = decodeRequiredString(raw)
		if err != nil {
			return locationWrite{}, err
		}
	}
	if raw, ok := obj["type"]; ok {
		out.Type, err = decodeRequiredString(raw)
		if err != nil {
			return locationWrite{}, err
		}
	}
	if raw, ok := obj["code"]; ok {
		out.Code, err = decodeNullableString(raw)
		if err != nil {
			return locationWrite{}, err
		}
	}
	if raw, ok := obj["parent_id"]; ok {
		id, isNull, err := decodeOptionalID(raw)
		if err != nil {
			return locationWrite{}, err
		}
		out.ParentPresent = true
		out.ParentNull = isNull
		out.ParentID = id
	}
	if raw, ok := obj["icon"]; ok {
		out.Icon, err = decodeNullableString(raw)
		if err != nil {
			return locationWrite{}, err
		}
	}
	if raw, ok := obj["custom_icon_id"]; ok {
		id, isNull, err := decodeOptionalID(raw)
		if err != nil {
			return locationWrite{}, err
		}
		out.CustomPresent = true
		out.CustomNull = isNull
		out.CustomID = id
	}
	if raw, ok := obj["version"]; ok {
		out.Version, err = decodeInteger(raw)
		if err != nil {
			return locationWrite{}, err
		}
		out.VersionPresent = true
	}
	return out, nil
}

func decodeCategoryWrite(body []byte) (categoryWrite, error) {
	obj, err := parseObject(body)
	if err != nil {
		return categoryWrite{}, err
	}
	var out categoryWrite
	if raw, ok := obj["name"]; ok {
		out.Name, err = decodeRequiredString(raw)
		if err != nil {
			return categoryWrite{}, err
		}
	}
	if raw, ok := obj["parent_id"]; ok {
		id, isNull, err := decodeOptionalID(raw)
		if err != nil {
			return categoryWrite{}, err
		}
		out.ParentPresent = true
		out.ParentNull = isNull
		out.ParentID = id
	}
	if raw, ok := obj["version"]; ok {
		out.Version, err = decodeInteger(raw)
		if err != nil {
			return categoryWrite{}, err
		}
		out.VersionPresent = true
	}
	return out, nil
}

func decodeIDLinks(raw json.RawMessage, idKey string) (bool, bool, []itemLink, error) {
	s := strings.TrimSpace(string(raw))
	if s == "null" {
		return true, true, nil, nil
	}
	if s == "" || s[0] != '[' {
		return false, false, nil, errInvalidBody
	}
	var elems []json.RawMessage
	if err := json.Unmarshal(raw, &elems); err != nil {
		return false, false, nil, errInvalidBody
	}
	links := make([]itemLink, 0, len(elems))
	for _, el := range elems {
		es := strings.TrimSpace(string(el))
		if es == "" || es[0] != '{' {
			return false, false, nil, errInvalidBody
		}
		link, err := decodeLink(el, idKey)
		if err != nil {
			return false, false, nil, err
		}
		links = append(links, link)
	}
	return true, false, links, nil
}

func decodeLink(raw json.RawMessage, idKey string) (itemLink, error) {
	obj, err := parseObject(raw)
	if err != nil {
		return itemLink{}, err
	}
	var link itemLink
	if raw, ok := obj[idKey]; ok {
		id, isNull, err := decodeOptionalID(raw)
		if err != nil {
			return itemLink{}, err
		}
		link.IDPresent = true
		link.IDNull = isNull
		link.ID = id
	}
	if idKey == "location_id" {
		if raw, ok := obj["note"]; ok {
			link.Note, err = decodeNullableString(raw)
			if err != nil {
				return itemLink{}, err
			}
		}
	}
	return link, nil
}

func decodeOptionalID(raw json.RawMessage) (int64, bool, error) {
	if strings.TrimSpace(string(raw)) == "null" {
		return 0, true, nil
	}
	id, err := decodeInteger(raw)
	if err != nil {
		return 0, false, err
	}
	return id, false, nil
}

func decodeRequiredString(raw json.RawMessage) (textValue, error) {
	s := strings.TrimSpace(string(raw))
	if s == "" || s[0] != '"' {
		return textValue{}, errInvalidBody
	}
	var v string
	if err := json.Unmarshal(raw, &v); err != nil {
		return textValue{}, errInvalidBody
	}
	return textValue{Present: true, Value: v}, nil
}

func decodeNullableString(raw json.RawMessage) (textValue, error) {
	if strings.TrimSpace(string(raw)) == "null" {
		return textValue{Present: true, Null: true}, nil
	}
	return decodeRequiredString(raw)
}

func decodeInteger(raw json.RawMessage) (int64, error) {
	s := strings.TrimSpace(string(raw))
	if s == "" || (s[0] != '-' && (s[0] < '0' || s[0] > '9')) || strings.ContainsAny(s, ".eE") {
		return 0, errInvalidBody
	}
	v, err := json.Number(s).Int64()
	if err != nil {
		return 0, errInvalidBody
	}
	return v, nil
}
