# Tasks API

A RESTful task API written in Go with [gin](https://github.com/gin-gonic/gin)
and an in-memory SQLite database.

## Quick start

```sh
make run           # run locally
make docker-run    # build and run the Docker image
make test          # unit tests with the race detector
make lint          # golangci-lint (includes go vet)
make generate      # regenerate the DAO mocks
```

CI (GitHub Actions) runs the tests with the race detector, golangci-lint (the
same version as `make lint`) and a Docker build.

Configuration is read from environment variables:


| Variable           | Default | Description                                                     |
| ------------------ | ------- | --------------------------------------------------------------- |
| `HTTP_ADDR`        | `:8080` | Address to listen on                                            |
| `SHUTDOWN_TIMEOUT` | `10s`   | How long in-flight requests get to finish on `SIGINT`/`SIGTERM` |


## API


| Method   | Path          | Description                 | Success |
| -------- | ------------- | --------------------------- | ------- |
| `GET`    | `/tasks`      | List tasks (paginated)      | `200`   |
| `POST`   | `/tasks`      | Create a task               | `201`   |
| `PUT`    | `/tasks/{id}` | Update a task               | `200`   |
| `DELETE` | `/tasks/{id}` | Delete a task (soft delete) | `204`   |
| `GET`    | `/healthz`    | Liveness probe              | `200`   |


### Task


| Field       | Type            | Notes                                                          |
| ----------- | --------------- | -------------------------------------------------------------- |
| `id`        | string (UUIDv7) | Server-generated, time-ordered                                 |
| `name`      | string          | Required on create, trimmed, 1–255 characters                  |
| `status`    | integer         | Required on create; `0` = incomplete, `1` = completed          |
| `createdAt` | string          | UTC                                                            |
| `updatedAt` | string          | UTC                                                            |
| `deletedAt` | string          | Only set on deleted tasks, which are never returned by the API |


### Examples

```sh
# Create (both fields required)
curl -i -X POST localhost:8080/tasks -d '{"name":"buy milk","status":0}'
# HTTP/1.1 201 Created
# {"id":"01a15c8d-...","name":"buy milk","status":0,"createdAt":"...","updatedAt":"..."}

# List, two at a time
curl 'localhost:8080/tasks?limit=2'
# {"items":[...],"nextKey":"eyJjdXJzb3IiOiIwMWExNWM4ZC0..."}
curl 'localhost:8080/tasks?limit=2&nextKey=eyJjdXJzb3IiOiIwMWExNWM4ZC0...'

# Update (only the fields given are changed; at least one is required)
curl -X PUT localhost:8080/tasks/01a15c8d-... -d '{"status":1}'
# {"id":"01a15c8d-...","name":"buy milk","status":1,"createdAt":"...","updatedAt":"..."}

# Delete
curl -i -X DELETE localhost:8080/tasks/01a15c8d-...
# HTTP/1.1 204 No Content
```

### Pagination

`GET /tasks` is paginated with `nextKey`, ordered by creation time (oldest first).


| Query param | Default | Notes                                                     |
| ----------- | ------- | --------------------------------------------------------- |
| `limit`     | `20`    | `1`–`100`                                                 |
| `nextKey`   | —       | The `nextKey` from the previous response; treat as opaque |


`nextKey` is omitted on the last page. Pass `limit` again on every request to
keep the same page size.

### Errors

Every error has the same shape:

```json
{"error": {"code": "validation_failed", "message": "status: is required", "field": "status"}}
```


| Status | `code`              | When                                                  |
| ------ | ------------------- | ----------------------------------------------------- |
| `400`  | `invalid_json`      | Empty, malformed, or wrongly typed body               |
| `400`  | `invalid_id`        | Path `id` is not a UUID                               |
| `400`  | `invalid_next_key`  | `nextKey` is invalid                                  |
| `404`  | `not_found`         | Task does not exist or was deleted                    |
| `413`  | `request_too_large` | Body is larger than 1 MiB                             |
| `422`  | `validation_failed` | A field or query parameter is missing or invalid      |
| `500`  | `internal_error`    | Unexpected failure (details are logged, not returned) |


## Design notes

### Layout

```
main/           entry point: env config, wiring, graceful shutdown
model/          domain types (Task, Status)
dao/            TaskDAO interface + SQLite implementation (plain database/sql)
migrations/     schema as goose SQL migrations, embedded into the binary
service/
  service.go    Service (dependencies) and Register (the task routes)
  main.go       error → HTTP status mapping, shared validation
  create_task.go list_tasks.go update_task.go delete_task.go
                one endpoint per file: XxxTask(c *gin.Context) binds the
                request, validates it, calls the DAO and writes the response
internal/
  daomock/      gomock mocks of the DAO interfaces (go generate)
  testutil/     NewSQLiteDB: a migrated in-memory database for tests
```

Dependencies point one way: service → dao → model. `Service` gets the DAO as
an exported field (`TaskDAO`), so moving to a persistent database only
means adding a new `TaskDAO` implementation, and tests can
inject a mock DAO.
Errors are mapped to an HTTP status and the API's error format in one place,
`service/main.go`.

### Decisions

- **gin** for routing, binding and panic recovery.
- **UUIDv7 IDs** sort by creation time, so the ID is also the pagination cursor.
- **Pagination**: `nextKey` holds the last ID, so deletes between requests
don't shift pages the way offsets would.
- **Soft delete**: `DELETE` sets `deletedAt`; deleted tasks behave as
nonexistent.
- **In-memory SQLite** (`modernc.org/sqlite`, pure Go): one idle connection
keeps the database alive if the pool drops its connection; the pool is
capped at one connection.
- **Migrations** (goose, `migrations/`) run at startup, since the database
lives inside the process.
- **ginkgo + gomega**: the service is tested via `httptest` with a gomock DAO,
the DAO against a real SQLite database.
