package catalog

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strings"
)

const maxIconSVG = 16384

var svgElems = map[string]struct{}{
	"svg": {}, "g": {}, "path": {}, "circle": {}, "ellipse": {},
	"rect": {}, "line": {}, "polyline": {}, "polygon": {},
}

var svgAttrs = map[string]struct{}{
	"viewbox": {}, "width": {}, "height": {}, "d": {},
	"cx": {}, "cy": {}, "r": {}, "rx": {}, "ry": {},
	"x": {}, "y": {}, "x1": {}, "y1": {}, "x2": {}, "y2": {},
	"points": {}, "transform": {}, "fill": {}, "stroke": {},
	"stroke-width": {}, "stroke-linecap": {}, "stroke-linejoin": {},
	"fill-rule": {}, "stroke-miterlimit": {}, "opacity": {},
	"fill-opacity": {}, "stroke-opacity": {},
}

func SanitizeSVG(raw string) (string, string) {
	if strings.TrimSpace(raw) == "" {
		return "", "图标不正确"
	}
	if len(raw) > maxIconSVG {
		return "", "图标过大"
	}
	dec := xml.NewDecoder(strings.NewReader(raw))
	dec.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		switch strings.ToLower(charset) {
		case "", "utf-8", "utf8":
			return input, nil
		default:
			return nil, errors.New("charset")
		}
	}
	var out bytes.Buffer
	depth := 0
	started := false
	hasBox := false
	hasW, hasH := false, false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", "图标不正确"
		}
		switch t := tok.(type) {
		case xml.StartElement:
			local := strings.ToLower(t.Name.Local)
			if badSVGName(t.Name) {
				return "", "图标不正确"
			}
			if _, ok := svgElems[local]; !ok {
				return "", "图标不正确"
			}
			if depth == 0 {
				if local != "svg" || started {
					return "", "图标不正确"
				}
				started = true
			}
			for _, a := range t.Attr {
				if forbiddenSVGAttr(a.Name) {
					return "", "图标不正确"
				}
			}
			out.WriteByte('<')
			out.WriteString(local)
			if local == "svg" && depth == 0 {
				out.WriteString(` xmlns="http://www.w3.org/2000/svg"`)
			}
			seen := map[string]struct{}{}
			for _, a := range t.Attr {
				canon, ok := canonSVGAttr(a.Name.Local)
				if !ok {
					continue
				}
				if _, dup := seen[canon]; dup {
					continue
				}
				seen[canon] = struct{}{}
				val := a.Value
				if canon == "fill" || canon == "stroke" {
					val = mapPaint(val)
				}
				if local == "svg" && depth == 0 {
					switch canon {
					case "viewBox":
						hasBox = true
					case "width":
						hasW = true
					case "height":
						hasH = true
					}
				}
				out.WriteByte(' ')
				out.WriteString(canon)
				out.WriteString(`="`)
				var esc bytes.Buffer
				if err := xml.EscapeText(&esc, []byte(val)); err != nil {
					return "", "图标不正确"
				}
				out.Write(esc.Bytes())
				out.WriteByte('"')
			}
			out.WriteByte('>')
			depth++
		case xml.EndElement:
			local := strings.ToLower(t.Name.Local)
			if _, ok := svgElems[local]; !ok {
				return "", "图标不正确"
			}
			depth--
			if depth < 0 {
				return "", "图标不正确"
			}
			out.WriteString("</")
			out.WriteString(local)
			out.WriteByte('>')
		case xml.CharData, xml.Comment, xml.ProcInst, xml.Directive:
			continue
		}
	}
	if !started || depth != 0 {
		return "", "图标不正确"
	}
	if !hasBox && !(hasW && hasH) {
		return "", "图标不正确"
	}
	return out.String(), ""
}

func badSVGName(n xml.Name) bool {
	if n.Space == "" || n.Space == "http://www.w3.org/2000/svg" {
		return false
	}
	return true
}

func forbiddenSVGAttr(n xml.Name) bool {
	local := strings.ToLower(n.Local)
	if local == "href" || strings.HasPrefix(local, "on") {
		return true
	}
	if n.Space == "http://www.w3.org/1999/xlink" || strings.Contains(strings.ToLower(n.Space), "xlink") {
		return true
	}
	return false
}

func canonSVGAttr(local string) (string, bool) {
	if local == "viewBox" || strings.ToLower(local) == "viewbox" {
		return "viewBox", true
	}
	low := strings.ToLower(local)
	if _, ok := svgAttrs[low]; !ok {
		return "", false
	}
	return low, true
}

func mapPaint(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || strings.EqualFold(v, "none") || strings.EqualFold(v, "currentColor") {
		return v
	}
	return "currentColor"
}
