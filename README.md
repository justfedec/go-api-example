# Go API Example

Simple Todo app with Go's standard library. No frameworks, no dependencies.

## Run

```bash
go run . --addr 0.0.0.0:8080
```

Open http://localhost:8080 for the web UI.

## Endpoints

| Method   | Path           | Description         |
|----------|----------------|---------------------|
| `GET`    | `/`            | Web UI              |
| `GET`    | `/todos`       | List all todos      |
| `POST`   | `/todos`       | Create a todo       |
| `PATCH`  | `/todos/{id}`  | Toggle completed    |
| `DELETE` | `/todos/{id}`  | Delete a todo       |
| `GET`    | `/health`      | Health check        |
