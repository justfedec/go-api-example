# Hello, web

The smallest possible Inkdown web server: no callbacks, no handlers to
register — just a loop that pulls requests and answers them.

```inkdown
let srv = http.serve(8080)
print("Listening on http://localhost:8080 — Ctrl-C to stop")
while true {
  let req = http.next(srv)
  http.respond(req, 200, "Hello from a Markdown file!\n")
}
```
