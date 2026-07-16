# e2e fixture: HTTP client

Talks to the fake server whose URL arrives in `TEST_URL`. The last request
targets a closed port to show that transport failures never panic.

```inkdown
let base = env("TEST_URL")

let hello = http.get(base + "/hello")
print(http.ok(hello), http.status(hello))
print(http.text(hello))

let created = http.post(base + "/items", "{\"name\": \"x\"}")
print(http.status(created), json.get(http.text(created), "id"))

let bad = http.get("http://127.0.0.1:1/unreachable")
print(http.ok(bad), http.status(bad))
```
