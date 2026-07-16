# A todo REST API, written in Markdown

This repository used to contain a Go todo API; here it is again, this time
as an Inkdown program. Run it and talk to it with curl:

~~~
$ inkdown run examples/todo-api.md
todo-api listening on http://localhost:8080

$ curl -s -X POST localhost:8080/todos -d '{"title": "ship inkdown v2"}'
{"id": 1, "title": "ship inkdown v2", "completed": false}
$ curl -s -X PATCH localhost:8080/todos/1
{"id": 1, "title": "ship inkdown v2", "completed": true}
$ curl -s localhost:8080/todos
[{"id": 1, "title": "ship inkdown v2", "completed": true}]
$ curl -s -X DELETE localhost:8080/todos/1 -o /dev/null -w '%{http_code}\n'
204
~~~

## The store

Inkdown has no structs, so the store is three parallel lists plus an id
counter. Globals are visible inside every function.

```inkdown
var ids: [int] = []
var titles: [string] = []
var done: [bool] = []
var next_id = 1
```

`find` maps an id to its position, and `remove_at` rebuilds the lists
without one position (lists grow with `push`; removal is a rebuild).

```inkdown
func find(id: int) -> int {
  for i in range(0, len(ids)) {
    if ids[i] == id { return i }
  }
  return -1
}

func remove_at(at: int) {
  var new_ids: [int] = []
  var new_titles: [string] = []
  var new_done: [bool] = []
  for i in range(0, len(ids)) {
    if i != at {
      push(new_ids, ids[i])
      push(new_titles, titles[i])
      push(new_done, done[i])
    }
  }
  ids = new_ids
  titles = new_titles
  done = new_done
}
```

## Rendering JSON

Responses are built by concatenation; `json.escape` keeps user text safe.

```inkdown
func todo_json(i: int) -> string {
  return ("{\"id\": " + str(ids[i]) + ", \"title\": " + json.escape(titles[i])
    + ", \"completed\": " + str(done[i]) + "}")
}

func todos_json() -> string {
  var out = "["
  for i in range(0, len(ids)) {
    if i > 0 { out += ", " }
    out += todo_json(i)
  }
  return out + "]"
}
```

## The handlers

`request` is a handle type, so requests can be passed to helper functions.

```inkdown
func create(req: request) {
  let body = http.body(req)
  if not json.has(body, "title") {
    http.respond(req, 400, "{\"error\": \"the body must be JSON with a title field\"}")
    return
  }
  push(ids, next_id)
  push(titles, json.get(body, "title"))
  push(done, false)
  next_id += 1
  http.respond(req, 201, todo_json(len(ids) - 1))
}

func handle(req: request) {
  let method = http.method(req)
  let path = http.path(req)
  http.set_header(req, "Content-Type", "application/json")

  if path == "/health" {
    http.respond(req, 200, "{\"status\": \"ok\", \"todos\": " + str(len(ids)) + "}")
    return
  }
  if path == "/todos" {
    if method == "GET" {
      http.respond(req, 200, todos_json())
    } else if method == "POST" {
      create(req)
    } else {
      http.respond(req, 405, "{\"error\": \"method not allowed\"}")
    }
    return
  }
  if str.starts_with(path, "/todos/") {
    let parts = str.split(path, "/")
    if len(parts) == 3 and is_int(parts[2]) {
      let i = find(int(parts[2]))
      if i == -1 {
        http.respond(req, 404, "{\"error\": \"no such todo\"}")
      } else if method == "PATCH" {
        done[i] = not done[i]
        http.respond(req, 200, todo_json(i))
      } else if method == "DELETE" {
        remove_at(i)
        http.respond(req, 204, "")
      } else {
        http.respond(req, 405, "{\"error\": \"method not allowed\"}")
      }
      return
    }
  }
  http.respond(req, 404, "{\"error\": \"not found\"}")
}
```

## The accept loop

One request at a time, forever. Set `PORT` to override the default.

```inkdown
var port = 8080
if is_int(env("PORT")) {
  port = int(env("PORT"))
}
let srv = http.serve(port)
print("todo-api listening on http://localhost:" + str(port))
while true {
  handle(http.next(srv))
}
```
