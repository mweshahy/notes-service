# Notes API

A small REST API for creating, listing, searching, updating, and deleting notes. It uses Go's `net/http`, SQLite persistence, and explicit constructor injection.

## Architecture

```text
cmd/notes-api       composition root, HTTP server and shutdown
internal/domain     Note model and domain errors
internal/service    note operations and repository contract
internal/storage    SQLite and in-memory repository adapters
internal/transport/http  HTTP handlers and request logging middleware
tests/              service and HTTP tests
```

The service owns application behavior and depends on a repository interface. Storage adapters implement that interface. HTTP depends on a narrow service interface, so transport tests can use the real service with the in-memory adapter.

## Run

Requires the Go version 1.27.4 declared in `go.mod` or newer.

```sh
go run ./cmd/notes-api
```

The server listens on `:8080` by default and stores data in `./data/notes.db`. Set `NOTES_SERVICE_PORT` to use a different port. For example, in PowerShell:

```powershell
$env:NOTES_SERVICE_PORT = "9090"
go run ./cmd/notes-api
```

If needed, set the database path by changing the path used in `NewSQLiteRepository` initialization in `internal/storage/sqlite.go`.

Build a container with:

```sh
docker build -t notes-api .
```

SQLite stores data directly in a local file. If you run the container without a volume, your database will be wiped out every time the container stops, restarts, or updates. Create a local data folder for example (C:\Docker\data) and mount it into the container using a volume option (-v) as follows:

```sh

docker run --rm -p 8080:8080 -v ./data:/app/data notes-api
```

To run on a different container port, set `NOTES_SERVICE_PORT` and publish that port. For example:
```sh
docker run --rm -e NOTES_SERVICE_PORT=9090 -p 9090:9090 -v ./data:/app/data notes-api
```
.
OR
```sh
docker run -d --name notes-api-instance -p 8080:8080 -v C:\Docker\data:/app/data notes-api
docker logs notes-api-instance
```

## Endpoints

- `POST /api/v1/notes` — JSON `{"title":"...","content":"..."}`, returns `201` and the created note.

Invoke-RestMethod -Method Post -Uri "http://localhost:8080/api/v1/notes" -ContentType "application/json" -Body '{"title": "Architectural Notes", "content": "Keep dependencies small and favor standard library features."}'

- `GET /api/v1/notes` — returns all notes. Optional `?q=...` searches title and content, case-insensitively.

Invoke-RestMethod -Method Get -Uri "http://localhost:8080/api/v1/notes"

- `GET /api/v1/notes/{id}` — returns one note.

Invoke-RestMethod -Method Get -Uri "http://localhost:8080/api/v1/notes/ef7ba3c9-7e3b-4214-a6d6-9ee55f68bd53"

- `PUT /api/v1/notes/{id}` — JSON `{"title":"...","content":"..."}`, returns the updated note.

Invoke-RestMethod -Method Put -Uri "http://localhost:8080/api/v1/notes/367185f9-ee88-4176-adbd-3a3c24020712" -ContentType "application/json" -Body '{"title": "topic1", "content": "Happy birthday to me"}'

- `DELETE /api/v1/notes/{id}` — returns `204`.

Invoke-RestMethod -Method Delete -Uri "http://localhost:8080/api/v1/notes/367185f9-ee88-4176-adbd-3a3c24020712"

Notes include `id`, `title`, `content`, `created_at`, and `updated_at`. Both timestamps are stored in SQLite in UTC.

## Verification

```sh
go test ./tests/...
```
