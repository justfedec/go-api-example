package stdlib

import "github.com/justfedec/inkdown/internal/types"

func init() {
	Roots["str"] = true

	str := types.String
	strList := &types.List{Elem: types.String}
	register(
		&Spec{Name: "str.len", Params: []types.Type{str}, Ret: types.Int, GoFunc: "_ink_strLen", Chunk: "strings"},
		&Spec{Name: "str.slice", Params: []types.Type{str, types.Int, types.Int}, Ret: str, GoFunc: "_ink_slice", Chunk: "strings"},
		&Spec{Name: "str.split", Params: []types.Type{str, str}, Ret: strList, GoFunc: "_ink_split", Chunk: "strings"},
		&Spec{Name: "str.join", Params: []types.Type{strList, str}, Ret: str, GoFunc: "_ink_join", Chunk: "strings"},
		&Spec{Name: "str.contains", Params: []types.Type{str, str}, Ret: types.Bool, GoFunc: "_ink_contains", Chunk: "strings"},
		&Spec{Name: "str.starts_with", Params: []types.Type{str, str}, Ret: types.Bool, GoFunc: "_ink_startsWith", Chunk: "strings"},
		&Spec{Name: "str.index_of", Params: []types.Type{str, str}, Ret: types.Int, GoFunc: "_ink_indexOf", Chunk: "strings"},
		&Spec{Name: "str.trim", Params: []types.Type{str}, Ret: str, GoFunc: "_ink_trim", Chunk: "strings"},
		&Spec{Name: "str.lower", Params: []types.Type{str}, Ret: str, GoFunc: "_ink_lower", Chunk: "strings"},
		&Spec{Name: "str.upper", Params: []types.Type{str}, Ret: str, GoFunc: "_ink_upper", Chunk: "strings"},
		&Spec{Name: "str.replace", Params: []types.Type{str, str, str}, Ret: str, GoFunc: "_ink_replace", Chunk: "strings"},
	)

	// Every str.* position counts runes, not bytes, so str.len, str.slice and
	// str.index_of compose safely on UTF-8 text (len() is list-only in v3).
	Chunks["strings"] = &Chunk{
		Imports: []string{"strings", "unicode/utf8"},
		Src: `func _ink_strLen(s string) int { return utf8.RuneCountInString(s) }

func _ink_split(s, sep string) []string { return strings.Split(s, sep) }

func _ink_join(xs []string, sep string) string { return strings.Join(xs, sep) }

func _ink_contains(s, sub string) bool { return strings.Contains(s, sub) }

func _ink_startsWith(s, prefix string) bool { return strings.HasPrefix(s, prefix) }

func _ink_indexOf(s, sub string) int {
	i := strings.Index(s, sub)
	if i < 0 {
		return -1
	}
	return utf8.RuneCountInString(s[:i])
}

func _ink_slice(s string, i, j int) string {
	runes := []rune(s)
	if i < 0 || j > len(runes) || i > j {
		panic("str.slice: invalid range [" + strconv.Itoa(i) + ":" + strconv.Itoa(j) + ") for a string of " + strconv.Itoa(len(runes)) + " characters")
	}
	return string(runes[i:j])
}

func _ink_trim(s string) string { return strings.TrimSpace(s) }

func _ink_lower(s string) string { return strings.ToLower(s) }

func _ink_upper(s string) string { return strings.ToUpper(s) }

func _ink_replace(s, old, new string) string { return strings.ReplaceAll(s, old, new) }
`,
	}
}
