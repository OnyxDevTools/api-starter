# API Starter

A small Go REST API with **Tasks**, full CRUD, and Onyx persistence.
One server, one container, no frontend toolchain.

Uses `net/http`, Chi, Huma, and the official Onyx Go SDK. `/docs` serves embedded
Swagger UI with editable requests, response inspection, and curl examples. Its
pinned assets require no CDN or hosted service; the online validator is disabled.
Asset licenses and update instructions are in
[docs-assets](internal/httpapi/docs-assets/README.md).

## Start

Requires Go 1.27.3 (pinned in go.mod). Preview the documentation without a database:

```sh
make docs
```

Open **http://127.0.0.1:8080/docs**. Stop this preview before starting the API on
the same port. The preview serves documentation only; it does not simulate CRUD.

For the real API, download a development database config from Onyx, place it in
`config/onyx-database.json` (ignored by Git/Docker), and export:

```sh
export ONYX_CONFIG_PATH="$PWD/config/onyx-database.json"
export ONYX_DATABASE_ID="your-development-database-id"
make schema-diff
make schema-publish
make api
```

`make schema-publish` is an explicit setup operation. It preserves unrelated
remote tables and adds missing starter tables. Compatible existing tables remain
unchanged; incompatible tables require manual review. Coordinate publishing with
other schema operators. Startup, tests, and builds never publish schemas.

The SDK also accepts `ONYX_DATABASE_API_KEY`, `ONYX_DATABASE_API_SECRET`, and
`ONYX_DATABASE_BASE_URL` instead of a downloaded file. It uses the official SDK
configuration resolver. The app does not auto-load `.env` files.
Onyx 4.2.9+ is required for insert-only creation.

## Endpoints

| Method | Route |
| --- | --- |
| POST | `/api/v1/tasks` |
| GET list | `/api/v1/tasks` |
| GET one | `/api/v1/tasks/{id}` |
| PUT | `/api/v1/tasks/{id}` |
| DELETE | `/api/v1/tasks/{id}` |

- Tasks accept `title`, `body`, and `completed` (all required).
- Titles are trimmed, nonblank, and at most 120 characters. Body is required but
  may be empty; maximum 10,000 characters.
- IDs and UTC timestamps are server-owned. Updates preserve ID and createdAt.
- Lists use `limit` (1–100, default 20) and an optional opaque continuation `cursor`.
- Errors use `application/problem+json`; dependency failures never become empty lists.
- `/healthz`: process liveness. `/readyz`: bounded, read-only database check.
- `/docs`: interactive Swagger UI. `/openapi.json` and `/openapi.yaml`: generated contracts.

```sh
curl -i http://127.0.0.1:8080/api/v1/tasks \
  -H 'Content-Type: application/json' \
  -d '{"title":"Shopping list","body":"Bread and coffee","completed":false}'
```

## Structure

```text
cmd/api       startup and graceful shutdown
internal/httpapi  routes, typed contracts, middleware, local docs
internal/store    repository interfaces and Onyx persistence
internal/gen/onyx checked-in official generated table clients
api/          Onyx schema and exported OpenAPI
cmd/docs      database-free docs preview
cmd/openapi   offline specification export
cmd/schema    explicit schema tooling
```

Handlers validate and delegate to one repository interface; the API reuses one
Onyx client. Creates use insert, never upsert. Updates/deletes use affected counts
for missing records. Concurrent edits use last-write wins. Pagination stays in
the database. Test doubles exist only in tests.

The API container keeps no durable local state. SDK requests have an eight-second
timeout within a ten-second request deadline. Readiness reads at most one Task
under a two-second deadline; liveness does not access the database.

## Build and check

```sh
make check              # race tests, go vet, static Go binary
make lint vuln          # optional pinned development tools, installed on demand
make openapi            # regenerate OpenAPI without database access
make generate-onyx      # official CLI, local schema -> typed Go client
make schema-validate    # local structural schema check
```

`make schema-diff` and `make schema-validate-remote` are read-only remote commands.
Tool versions are pinned in Makefile. Ordinary builds require only Go, with no
database connection. Dependency installation and vulnerability lookup require
network access. CLI 0.11.14 checks
configuration during file-only generation; the helper supplies non-secret
`offline-codegen` values only for that offline command.

To opt into the live CRUD test against a running API and dedicated development
database, set the same ONYX_DATABASE_ID as the server and run:

```sh
LIVE_SMOKE_CONFIRM=development make live-smoke
```

It cleans up only its own record IDs. Normal tests skip this check.
Set API_URL if the running server is at a different address.

## Stateless container

```sh
docker build -t api-starter .
docker run --rm --read-only --cap-drop=ALL --security-opt=no-new-privileges \
  -p 127.0.0.1:8080:8080 \
  --mount "type=bind,src=$ONYX_CONFIG_PATH,dst=/run/onyx-config,readonly" \
  -e ONYX_CONFIG_PATH=/run/onyx-config api-starter
```

One non-root process, a static Go binary, and TLS certificates. No local database,
sessions, writable application storage, or container-specific hosting adapter.
Replicas share Onyx persistence. Schema changes run separately from container startup.

API_HOST defaults to `127.0.0.1` locally and `0.0.0.0` in the image. API_PORT
defaults to 8080. The API includes deadlines, HTTP timeouts, body limits, request
IDs, structured logs, panic recovery, and graceful shutdown. Secrets and record
bodies are not logged. This example is unauthenticated; add authentication and
request authorization before exposing private data.

If `/readyz` returns 503, check credentials, connectivity, table setup, and database
permissions. A missing table is a dependency failure. If startup reports missing
configuration, export the Onyx settings before starting the API.
