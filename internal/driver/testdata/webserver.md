# e2e fixture: a small web server

Serves forever on the port given by the `PORT` environment variable; the
test kills the process when it is done.

```inkdown
let srv = http.serve(int(env("PORT")))
while true {
  let req = http.next(srv)
  let path = http.path(req)
  if path == "/ping" {
    http.respond(req, 200, "pong")
  } else if path == "/echo" {
    http.set_header(req, "Content-Type", "application/json")
    let doc = ("{\"method\": " + json.escape(http.method(req))
      + ", \"body\": " + json.escape(http.body(req))
      + ", \"q\": " + json.escape(http.query(req, "q")) + "}")
    http.respond(req, 200, doc)
  } else {
    http.respond(req, 404, "not found: " + path)
  }
}
```
