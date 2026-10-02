package catalog

var locationIcons = map[string]struct{}{
	"home": {}, "kitchen": {}, "living": {}, "bedroom": {}, "bathroom": {},
	"balcony": {}, "garage": {}, "cabinet": {}, "drawer": {}, "shelf": {},
	"fridge": {}, "washer": {}, "wardrobe": {}, "bookcase": {}, "box": {},
	"crate": {}, "basket": {}, "toolbox": {}, "bin": {}, "safe": {},
}

func ValidLocationIcon(name string) bool {
	_, ok := locationIcons[name]
	return ok
}

func parseLocationIcon(in OptionalText) (*string, string) {
	if !in.Present {
		return nil, ""
	}
	if in.Null {
		return nil, ""
	}
	if !ValidLocationIcon(in.Value) {
		return nil, "图标不正确"
	}
	v := in.Value
	return &v, ""
}
