# Go API Example

A Todo REST API on Go's standard library (`net/http` + `database/sql`), persisting
to **Postgres**. The single dependency is the pgx driver.

It is built to showcase Propie's **Databases** feature: attach a Database to the VM
and the platform injects its connection string as `DATABASE_URL` — the app then
stores todos durably, so they survive VM restarts and idle-reaps.

## Required env

| Var            | Notes                                                                 |
| -------------- | --------------------------------------------------------------------- |
| `DATABASE_URL` | Postgres connection string. **Required** — the app exits without it.  |

On Propie, you don't type this: in the deploy dialog pick one of your project's
Databases and the platform injects `DATABASE_URL` for you.

## Run locally

```bash
# throwaway Postgres
docker run -d --rm --name pg -e POSTGRES_PASSWORD=pg -p 5432:5432 postgres:16

DATABASE_URL='postgres://postgres:pg@localhost:5432/postgres?sslmode=disable' \
  go run . --addr 0.0.0.0:8080
```

Open http://localhost:8080 for the web UI.

## Endpoints

| Method   | Path           | Description                       |
| -------- | -------------- | --------------------------------- |
| `GET`    | `/`            | Web UI                            |
| `GET`    | `/todos`       | List all todos                    |
| `POST`   | `/todos`       | Create a todo                     |
| `PATCH`  | `/todos/{id}`  | Toggle completed                  |
| `DELETE` | `/todos/{id}`  | Delete a todo                     |
| `GET`    | `/health`      | DB-aware health (`store: postgres`) |

## Tests

```bash
go test ./...                       # MemoryStore suite (no DB needed)
DATABASE_URL=... go test ./...      # also runs the PostgresStore suite
```
