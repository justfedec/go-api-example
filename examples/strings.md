# Working with text

Inkdown v2 ships a set of flat string builtins. Positions count **runes**
(characters), not bytes, so `index_of` and `substring` compose safely on
UTF-8 text; only `len(s)` stays byte-based.

Whitespace trimming and case:

```inkdown
let sentence = "  The Quick Brown Fox  "
let clean = trim(sentence)
print(clean)
print(lower(clean))
print(upper(clean))
```

`split` and `join` convert between strings and lists:

```inkdown
let words = split(clean, " ")
print(len(words), "words")
for w in words {
  print("-", w)
}
print(join(words, "_"))
```

Searching and slicing:

```inkdown
print(contains(clean, "Quick"), starts_with(clean, "The"))
print(index_of(clean, "Brown"), index_of(clean, "Wolf"))
print(substring(clean, 4, 9))
print(replace(clean, "Fox", "Yak"))
```

The guards `is_int` and `is_float` test whether `int()` / `float()` would
succeed, so untrusted input never has to panic:

```inkdown
if is_int("42") {
  print(int("42") + 1)
}
print(is_float("3.5"), is_float("abc"))
```
