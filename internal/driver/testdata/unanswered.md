# e2e fixture: a server that takes a second request without answering the first

```inkdown
let srv = http.serve(int(env("PORT")))
let a = http.next(srv)
let b = http.next(srv)
print("unreachable")
```
