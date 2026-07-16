package stdlib

import "github.com/justfedec/inkdown/internal/types"

// The json namespace reads JSON documents through dot-separated string paths
// ("todos.0.title") instead of introducing maps or a dynamic value type:
// json.has is the guard, json.get panics when the path is missing (panics
// stay reserved for programmer errors, per the v1 philosophy), and
// json.escape makes building JSON by string concatenation safe.
func init() {
	Roots["json"] = true

	str := types.String
	register(
		&Spec{Name: "json.get", Params: []types.Type{str, str}, Ret: str, GoFunc: "_ink_jsonGet", GoFuncErr: "_ink_jsonGetErr", Chunk: "json"},
		&Spec{Name: "json.has", Params: []types.Type{str, str}, Ret: types.Bool, GoFunc: "_ink_jsonHas", Chunk: "json"},
		&Spec{Name: "json.len", Params: []types.Type{str, str}, Ret: types.Int, GoFunc: "_ink_jsonLen", GoFuncErr: "_ink_jsonLenErr", Chunk: "json"},
		&Spec{Name: "json.escape", Params: []types.Type{str}, Ret: str, GoFunc: "_ink_jsonEscape", Chunk: "json"},
	)

	Chunks["json"] = &Chunk{
		Imports: []string{"encoding/json", "strings"},
		Src: `// _ink_jsonDecode parses with UseNumber so numbers keep their original
// literal form: no scientific notation, no float64 precision loss.
func _ink_jsonDecode(doc string) (any, error) {
	dec := json.NewDecoder(strings.NewReader(doc))
	dec.UseNumber()
	var v any
	err := dec.Decode(&v)
	return v, err
}

// _ink_jsonRoot is the panicking decode used by json.get and json.len; an
// unparsable document is a programmer error, distinct from a missing path.
func _ink_jsonRoot(fn, doc string) any {
	v, err := _ink_jsonDecode(doc)
	if err != nil {
		panic(fn + ": the document is not valid JSON: " + err.Error())
	}
	return v
}

// _ink_jsonWalk follows a dot-separated path ("" is the document root;
// numeric segments index arrays) and reports whether a value exists there.
func _ink_jsonWalk(v any, path string) (any, bool) {
	if path == "" {
		return v, true
	}
	for _, seg := range strings.Split(path, ".") {
		switch cur := v.(type) {
		case map[string]any:
			nv, ok := cur[seg]
			if !ok {
				return nil, false
			}
			v = nv
		case []any:
			i, err := strconv.Atoi(seg)
			if err != nil || i < 0 || i >= len(cur) {
				return nil, false
			}
			v = cur[i]
		default:
			return nil, false
		}
	}
	return v, true
}

// _ink_jsonGet returns scalars in string form (int() and float() convert
// them onward — numbers come back exactly as written in the document);
// objects and arrays come back as compact JSON, ready to be fed to json.get
// again.
func _ink_jsonGet(doc, path string) string {
	v, ok := _ink_jsonWalk(_ink_jsonRoot("json.get", doc), path)
	if !ok {
		panic("json.get: no value at path " + strconv.Quote(path) + " (guard with json.has)")
	}
	switch t := v.(type) {
	case string:
		return t
	case json.Number:
		return string(t)
	case bool:
		return strconv.FormatBool(t)
	case nil:
		return "null"
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

// _ink_jsonGetErr is the recoverable form for 'let x, err = json.get(...)':
// both an unparsable document and a missing path come back as an error
// string instead of a panic — the shape you want for untrusted request
// bodies.
func _ink_jsonGetErr(doc, path string) (string, string) {
	v, err := _ink_jsonDecode(doc)
	if err != nil {
		return "", "json.get: the document is not valid JSON: " + err.Error()
	}
	val, ok := _ink_jsonWalk(v, path)
	if !ok {
		return "", "json.get: no value at path " + strconv.Quote(path)
	}
	switch t := val.(type) {
	case string:
		return t, ""
	case json.Number:
		return string(t), ""
	case bool:
		return strconv.FormatBool(t), ""
	case nil:
		return "null", ""
	default:
		b, _ := json.Marshal(val)
		return string(b), ""
	}
}

func _ink_jsonLenErr(doc, path string) (int, string) {
	v, err := _ink_jsonDecode(doc)
	if err != nil {
		return 0, "json.len: the document is not valid JSON: " + err.Error()
	}
	val, ok := _ink_jsonWalk(v, path)
	if !ok {
		return 0, "json.len: no value at path " + strconv.Quote(path)
	}
	arr, isArr := val.([]any)
	if !isArr {
		return 0, "json.len: the value at path " + strconv.Quote(path) + " is not an array"
	}
	return len(arr), ""
}

func _ink_jsonHas(doc, path string) bool {
	v, err := _ink_jsonDecode(doc)
	if err != nil {
		return false
	}
	_, ok := _ink_jsonWalk(v, path)
	return ok
}

func _ink_jsonLen(doc, path string) int {
	v, ok := _ink_jsonWalk(_ink_jsonRoot("json.len", doc), path)
	if !ok {
		panic("json.len: no value at path " + strconv.Quote(path) + " (guard with json.has)")
	}
	arr, isArr := v.([]any)
	if !isArr {
		panic("json.len: the value at path " + strconv.Quote(path) + " is not an array")
	}
	return len(arr)
}

// _ink_jsonEscape returns s as a quoted JSON string literal (quotes
// included), so values embed directly: "{\"title\": " + json.escape(t) + "}".
func _ink_jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
`,
	}
}
