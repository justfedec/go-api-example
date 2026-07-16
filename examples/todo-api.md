# A todo REST API, written in Markdown

This repository used to contain a Go todo API; here it is again, this time
as an Inkdown program. Run it and talk to it with curl:

~~~
$ inkdown run examples/todo-api.md
todo-api listening on http://localhost:8080

$ curl -s -X POST localhost:8080/todos -d '{"title": "ship inkdown v3"}'
{"id": 1, "title": "ship inkdown v3", "completed": false}
$ curl -s -X PATCH localhost:8080/todos/1
{"id": 1, "title": "ship inkdown v3", "completed": true}
$ curl -s localhost:8080/todos
[{"id": 1, "title": "ship inkdown v3", "completed": true}]
$ curl -s -X DELETE localhost:8080/todos/1 -o /dev/null -w '%{http_code}\n'
204
~~~

## The store

A `record` holds one todo, so the store is a single list of them plus an id
counter — no parallel arrays to keep in sync.

```inkdown
record Todo {
  id: int
  title: string
  completed: bool
}

var todos: [Todo] = []
var next_id = 1
```

`find` maps an id to its position, and `remove_at` rebuilds the list without
one position.

```inkdown
func find(id: int) -> int {
  for i in range(0, len(todos)) {
    if todos[i].id == id { return i }
  }
  return -1
}

func remove_at(at: int) {
  var kept: [Todo] = []
  for i in range(0, len(todos)) {
    if i != at { push(kept, todos[i]) }
  }
  todos = kept
}
```

## Rendering JSON

Responses are built by concatenation; `json.escape` keeps user text safe.

```inkdown
func todo_json(t: Todo) -> string {
  return ("{\"id\": " + str(t.id) + ", \"title\": " + json.escape(t.title)
    + ", \"completed\": " + str(t.completed) + "}")
}

func todos_json() -> string {
  var out = "["
  for i in range(0, len(todos)) {
    if i > 0 { out += ", " }
    out += todo_json(todos[i])
  }
  return out + "]"
}
```

## The handlers

The `title` field comes from an untrusted body, so `json.get` uses the
recoverable `let value, err = ...` form instead of panicking.

```inkdown
func create(req: request) {
  let title, err = json.get(http.body(req), "title")
  if err != "" {
    http.respond(req, 400, "{\"error\": \"the body must be JSON with a title field\"}")
    return
  }
  push(todos, Todo(id: next_id, title: title, completed: false))
  next_id += 1
  http.respond(req, 201, todo_json(todos[len(todos) - 1]))
}

func handle(req: request) {
  let method = http.method(req)
  let path = http.path(req)
  http.set_header(req, "Content-Type", "application/json")

  if path == "/health" {
    http.respond(req, 200, "{\"status\": \"ok\", \"todos\": " + str(len(todos)) + "}")
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
        todos[i].completed = not todos[i].completed
        http.respond(req, 200, todo_json(todos[i]))
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
