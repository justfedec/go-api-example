# Working with text

Inkdown's string builtins live in the `str.` namespace. Every position counts
**runes** (characters), not bytes, so `str.len`, `str.slice`, and
`str.index_of` compose safely on UTF-8 text. (`len()` is for lists only.)

Whitespace trimming and case:

```inkdown
let sentence = "  The Quick Brown Fox  "
let clean = str.trim(sentence)
print(clean)
print(str.lower(clean))
print(str.upper(clean))
```

`str.split` and `str.join` convert between strings and lists:

```inkdown
let words = str.split(clean, " ")
print(len(words), "words")
for w in words {
  print("-", w)
}
print(str.join(words, "_"))
```

Searching and slicing (`str.slice` takes rune indices, `str.len` a rune
count, so they line up even on accented text):

```inkdown
print(str.contains(clean, "Quick"), str.starts_with(clean, "The"))
print(str.index_of(clean, "Brown"), str.index_of(clean, "Wolf"))
print(str.slice(clean, 4, 9))
print(str.replace(clean, "Fox", "Yak"))

let accented = "café"
print(str.len(accented), str.slice(accented, 0, str.len(accented)))
```

The guards `is_int` and `is_float` test whether `int()` / `float()` would
succeed; the recoverable `let x, err = int(s)` form is another way to handle
untrusted input without panicking:

```inkdown
if is_int("42") {
  print(int("42") + 1)
}
print(is_float("3.5"), is_float("abc"))

let n, err = int("not a number")
print(err)
```
