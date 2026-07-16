# regression: JSON numbers keep their literal form

Large integers must not come back in scientific notation or lose precision.

```inkdown
let doc = "{\"id\": 1000000, \"ts\": 1721000000, \"big\": 9007199254740993, \"pi\": 3.5}"
print(int(json.get(doc, "id")) + 1)
print(int(json.get(doc, "ts")))
print(json.get(doc, "big"))
print(float(json.get(doc, "pi")))
```
