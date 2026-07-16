# error binding, end to end

```inkdown
let n, err = int("42")
print(n, err == "")

let m, e2 = int("oops")
print(m, e2)

let doc = "{\"count\": 3, \"tags\": [\"a\", \"b\"]}"
let c, gerr = json.get(doc, "count")
print(c, gerr == "")

let missing, merr = json.get(doc, "nope")
print(merr)

let ln, lerr = json.len(doc, "tags")
print(ln, lerr == "")

let bad, berr = json.get("{ broken", "x")
print(berr)
```
