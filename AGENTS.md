# API Starter

Small Go API with one example resource: Task. Read README.md before changes.
Keep the starter easy to understand, stateless, and inexpensive to run. Prefer
Go's standard library and the existing net/http, Chi, Huma, and Onyx SDK patterns.

## Architectural boundaries

| Location | Responsibility |
| --- | --- |
| `cmd/api` | Composition root: load configuration, construct dependencies, configure HTTP, and own process shutdown. |
| `internal/config` | Parse and validate application configuration once at startup. |
| `internal/httpapi` | Typed HTTP contracts, input validation, handlers, status/error mapping, middleware, and docs delivery. |
| `internal/store/store.go` | Application models, repository contract, and transport-independent error categories. |
| `internal/store/onyx.go` | Onyx adapter: SDK calls, storage mapping, pagination, and SDK error classification. |
| `internal/gen/onyx` | Official generated table clients; regenerate rather than hand-edit. |
| `cmd/schema` | Explicit schema operations, isolated from the running API. |
| `cmd/openapi`, `cmd/docs` | Offline contract export and database-free documentation preview. |

- Wire dependencies explicitly in `cmd/api`; do not use service locators, global
  database clients, or hidden initialization side effects.
- HTTP handlers validate, delegate, and translate results. They must not construct
  SDK clients, execute database queries, or publish schemas.
- Repository contracts and application models must not depend on Chi, Huma, or
  generated SDK types. Keep SDK-specific behavior in the Onyx adapter.
- Persistence code must not choose HTTP response bodies or write to an HTTP
  response. Map application errors to HTTP once, at the HTTP boundary.
- The current simple CRUD flow needs no extra service layer. If business rules
  grow beyond CRUD, introduce a focused application package for those rules; it
  should depend on small repository interfaces, not HTTP or concrete SDK clients.
- Keep packages cohesive and dependencies acyclic. Avoid generic frameworks,
  pass-through layers, speculative abstractions, and catch-all utility packages.

## Contracts and data correctness

- Define request validation in Huma inputs. Validate and normalize input before
  writes; keep reusable business invariants in application code when introduced.
- Accept only client-editable fields. Generate IDs and UTC timestamps on the
  server; preserve ID and createdAt during updates.
- Create with insert, not save/upsert. Update and delete through filtered queries
  with affected counts so missing records return 404 without being recreated.
- Keep list limits bounded and use database cursor pagination with deterministic
  ordering. Never load an entire table to paginate in memory.
- Distinguish missing records, conflicts, invalid cursors, unavailable dependencies,
  authentication failures, and unexpected storage responses. Never convert a
  failed query into an empty successful response.
- Preserve wrapped error identity with `errors.Is`/`errors.As`; expose only safe
  application errors at HTTP boundaries. Do not log raw SDK errors or payloads.
- Current edits use last-write wins. If stronger concurrency guarantees are
  required, implement them using documented database capabilities and tests;
  do not imply a read-then-write sequence is atomic.
- Do not add automatic retries to mutations without an explicit idempotency
  strategy. Avoid retry loops that can duplicate records or exceed deadlines.
- Regenerate OpenAPI from the same typed routes after contract changes. Keep
  checked-in contracts, Onyx schema, and generated Go clients consistent.

## Stateless runtime and resource ownership

- Run one stateless Go HTTP process. Keep durable state in Onyx; do not rely on
  local files, process memory, sessions, or instance affinity for correctness.
- Reuse configured SDK/HTTP clients and connection pools. Resolve application
  configuration at startup, rather than reading environment variables in handlers.
- Pass request contexts through every storage or network operation. Do not replace
  them with background contexts or store contexts in long-lived structs.
- Bound downstream calls, readiness probes, request bodies, headers, and HTTP
  reads/writes. Downstream timeouts must fit inside the request deadline.
- Keep liveness independent of the database. Readiness must be bounded, read-only,
  and cheap; never use a probe to write data or alter schema.
- Every goroutine must have an owner, a termination path, and error handling.
  Close response bodies and other owned resources; avoid unbounded concurrency.
- On shutdown, stop accepting work and allow in-flight requests to finish within
  a bounded shutdown context. Do not use request contexts for process cleanup.
- Preserve the static binary, non-root runtime, read-only filesystem compatibility,
  and externally supplied credentials. Keep builds independent of database access.

## Security and operational scope

- Keep secrets out of source, images, docs examples, and logs. Use the official SDK
  resolver with environment variables or a read-only downloaded configuration.
- Log request IDs, methods, paths, status, duration, and safe error categories;
  never log credentials, request bodies, or full database responses.
- Keep loopback binding as the local default. When adding authentication, enforce
  authorization for every resource operation before accessing protected data.
- Schema publishing is a separate, explicit operation against the intended
  database. Preserve unrelated tables and refuse incompatible changes for review.
  Never publish schema during startup, builds, tests, or readiness checks.
- Keep Swagger UI pinned and self-hosted, its online validator disabled, and
  license notices intact. Do not add a hosted docs service, CDN assets, paid
  integration, or application frontend without a concrete user requirement.

## Verification and change discipline

- Keep changes focused. Add dependencies only for a concrete requirement; pin
  versions, run `go mod tidy`, and remove unused dependencies when scope changes.
- Test behavior at its owning boundary: HTTP status/validation contracts with
  httptest, application rules without a server, and adapter semantics with local
  SDK test servers. Test failure paths, cancellation, and pagination boundaries.
- Test doubles belong only in tests. The running API must use Onyx, and docs-only
  mode must clearly reject CRUD rather than simulate persistence.
- Run `make check lint vuln` for applicable code changes, plus schema validation
  and OpenAPI regeneration when those contracts change. Check generated diffs.
- Verify interactive documentation changes in a browser and confirm assets and
  API requests stay local. Keep ordinary tests and builds free of live writes.
- Run the live smoke test only against an explicitly selected development database;
  clean up only records created by that run. Report live checks separately from
  offline tests, including checks not run and the reason.
- Update README.md when setup, configuration, commands, or behavior changes.
  Add filename comments to authored source/config where the format supports them.
