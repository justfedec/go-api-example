# Go API Example

Simple REST API with Go's standard library. No frameworks, no dependencies.

## Run

```bash
go run . --addr 0.0.0.0:8080
```

## Endpoints

- `GET /` - Index page
- `GET /todos` - List todos
- `POST /todos` - Create todo (`{"title": "..."}`)
- `GET /health` - Health check
