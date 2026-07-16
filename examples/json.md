# Reading JSON with paths

Inkdown has no maps, so JSON stays a string and `json.get` walks it with a
dot-separated path — object keys by name, array elements by index.

```inkdown
let doc = "{\"todos\": [{\"title\": \"buy milk\", \"done\": false}, {\"title\": \"ship v2\", \"done\": true}], \"count\": 2}"

print(json.get(doc, "todos.0.title"))
print(json.get(doc, "todos.1.done"))
```

Scalars come back in string form; `int()` and `float()` convert onward:

```inkdown
print(int(json.get(doc, "count")) + 1)
```

`json.has` is the guard for optional fields (a missing path makes `json.get`
panic), and `json.len` sizes arrays so `range` can iterate them:

```inkdown
print(json.has(doc, "todos.2.title"), json.has(doc, "count"))
for i in range(0, json.len(doc, "todos")) {
  print(json.get(doc, "todos." + str(i) + ".title"))
}
```

Getting an object or array returns compact JSON, ready to be walked again:

```inkdown
let second = json.get(doc, "todos.1")
print(second)
print(json.get(second, "title"))
```

Going the other way, `json.escape` quotes a string as a JSON literal, so
documents can be built safely by concatenation:

```inkdown
print("{\"msg\": " + json.escape("he said \"hi\"") + "}")
```
