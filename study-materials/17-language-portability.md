# Adapting the Material to Other Backend Languages

Go is the reference language because its standard library, explicit errors,
concurrency model, and small deployment artifacts expose important backend
concerns clearly. The senior-backend standard in this repository is not "write
Go-shaped code elsewhere." It is preserve the same guarantees, failure behavior,
tests, and operational evidence using the target ecosystem's idioms.

## The portability contract

An implementation in another language is equivalent when it preserves:

- public HTTP, event, and data contracts;
- domain invariants and transaction boundaries;
- cancellation and deadline propagation;
- bounded bodies, concurrency, queues, pools, retries, and shutdown time;
- stable error identities without exposing internals;
- authentication, role authorization, and object-level authorization;
- database constraints, migration compatibility, and recovery evidence;
- deterministic unit, integration, contract, and acceptance tests;
- structured logs, metrics, traces, health behavior, SLOs, and runbooks;
- reproducible builds, dependency checks, non-root deployment, and safe rollout;
- the same contention, timeout, duplicate-delivery, restart, and partial-failure
  experiments required by the applied chapters.

Framework defaults do not count as evidence. Confirm effective timeouts, pool
sizes, retry behavior, serialization rules, middleware order, and shutdown behavior
in repository-owned tests or configuration.

## Concept mapping

| Concern | Go | Java/Kotlin | C#/.NET | TypeScript/Node.js | Python | Rust |
| --- | --- | --- | --- | --- | --- | --- |
| HTTP | `net/http` | Spring Boot, Micronaut, Javalin | ASP.NET Core | Fastify, Express, NestJS | FastAPI, Django, Flask | Axum, Actix Web |
| Cancellation | `context.Context` | structured concurrency, interrupts, timeouts | `CancellationToken` | `AbortSignal` | task cancellation and timeouts | cancellation-safe futures and tokens |
| Errors | returned errors, `errors.Is/As` | typed exceptions or results | typed exceptions or results | typed errors/results | typed exceptions | `Result` and error enums |
| Concurrency | goroutines, channels, mutexes | threads, virtual threads, coroutines, executors | tasks, channels, locks | event loop, promises, workers | `asyncio`, threads, processes | Tokio tasks, channels, locks |
| PostgreSQL | `database/sql`, `pgx` | JDBC, jOOQ, configured ORM | ADO.NET, Dapper, EF Core | node-postgres, Kysely, Prisma | psycopg, SQLAlchemy, Django ORM | SQLx, Diesel |
| Wiring | constructors and small interfaces | constructors, optional DI | built-in DI with explicit lifetimes | factories or framework DI | factories or framework DI | constructors, traits, app state |
| Logging | `log/slog` | SLF4J plus JSON backend | `ILogger` | Pino or equivalent | logging plus JSON formatter | `tracing` |
| Tests | `testing`, `httptest`, race/fuzz tools | JUnit, Testcontainers | xUnit/NUnit, TestServer, Testcontainers | test runner, HTTP injection, Testcontainers | pytest, test client, Testcontainers | built-in tests, proptest, Testcontainers |

These are examples, not mandatory choices. Prefer the team's established stack,
then make implicit behavior explicit at service boundaries.

## Universal and Go-specific material

Universal material includes HTTP semantics, domain modeling, SQL, transactions,
locking, migrations, idempotency, authentication and authorization, observability,
delivery, distributed systems, privacy, cloud operations, incident response, and
engineering leadership.

Go-specific material includes package visibility, zero values, slices, maps,
implicit interfaces, error wrapping APIs, `defer`, goroutines, channels, the Go
memory model, `context.Context`, `go test`, `go vet`, `gofmt`, and the race detector.
In another language, replace syntax exercises with native equivalents but retain
the backend exercise and its completion evidence.

## Adapting the core chapters

### Language and backend foundations

Learn the target language's module model, value/reference semantics, nullability,
collections, generics, error model, cleanup mechanism, concurrency model, testing,
static analysis, formatter, and race or thread-safety tooling.

Rebuild the in-memory product store idiomatically. Preserve explicit not-found and
duplicate-SKU outcomes, safe ownership of mutable values, thread-safe access, a
business-level contention test, and deterministic termination of spawned work.

### HTTP contracts and lifecycle

Use the framework's lowest practical layer until you can explain routing,
middleware order, body limits, content types, strict decoding, validation, problem
details, health checks, timeouts, cancellation, and graceful shutdown.

Test unknown fields, multiple JSON values, oversized bodies, wrong content types,
client cancellation, unhandled exception behavior, and drain deadlines. High-level
frameworks often add coercion or permissive decoding that must be made deliberate.

### PostgreSQL and relational data

SQL behavior is portable; drivers and ORMs are not. Verify parameter binding, pool
acquisition deadlines, transaction ownership, rollback on every error path,
row/stream disposal, isolation, lock behavior, affected-row counts, and translation
of actual PostgreSQL error codes.

Do not replace real-PostgreSQL tests with an in-memory database. Required evidence
depends on PostgreSQL constraints, MVCC, locks, query plans, and migrations.

### Business logic and architecture

Keep transport DTOs, application commands, domain concepts, and persistence models
separate when they change for different reasons. Use the language's normal
dependency mechanism; do not recreate Go interfaces or add a container for
symmetry. The application operation still owns the business transaction.

### Identity and API security

Use a maintained OAuth/OIDC resource-server library, but test issuer, audience,
algorithm, signature, expiry, clock skew, key rotation, missing credentials,
wrong-role access, and wrong-owner access. Framework annotations do not replace
object ownership in the query or use case. Keep secrets and personal data out of
logs and traces.

### Testing, observability, and operations

Choose test boundaries by risk rather than framework convention. Control clocks,
IDs, and randomness. Export OpenTelemetry-compatible traces and metrics where
practical, and keep labels bounded. The same SLO, runbook, cancellation,
saturation, and failure-drill requirements apply.

### Delivery and production readiness

Produce one immutable artifact, validate configuration before serving, run as a
non-root user, expose health endpoints with correct semantics, and drain within a
fixed deadline. Language-specific packaging changes; compatible schema rollout,
restore evidence, capacity budgets, artifact identity, and incident rehearsal do
not.

## Ecosystem-specific traps

### Java and Kotlin

- Proxy-based transaction annotations can be bypassed by self-invocation or
  incorrect visibility; prove the effective transaction boundary.
- Bound executors, virtual-thread carriers, connection pools, and HTTP clients.
- Preserve interruption or coroutine cancellation instead of swallowing it.
- Inspect generated ORM SQL for N+1 queries, lazy access, and accidental locks.

### C# and .NET

- Pass `CancellationToken` through controllers, application services, database
  access, and downstream clients.
- Choose DI lifetimes explicitly; never capture a scoped dependency in a singleton.
- Define timeout and retry ownership for managed `HttpClient` instances.
- Inspect EF Core tracking, generated SQL, concurrency tokens, and transaction scope.

### TypeScript and Node.js

- The event loop does not make multi-step operations atomic; use transactions and
  synchronization for shared state.
- Bound promises, worker queues, sockets, body sizes, and database pools.
- Propagate `AbortSignal` and do not let detached promises hide failures.
- Validate runtime input even when TypeScript types exist.

### Python

- Type hints do not validate untrusted runtime input by themselves.
- Know whether the server uses processes, threads, async tasks, or a mixture.
- Never perform blocking network or database work on an async event loop.
- Bound workers, task queues, request time, and connection pools explicitly.

### Rust

- Memory safety does not prove business atomicity or distributed correctness.
- Understand cancellation safety before dropping futures with partial work.
- Keep blocking work off async executors and bound spawned tasks and channels.
- Use domain error enums and translate infrastructure errors at the edge.

## Equivalent verification pipeline

Every alternative stack needs repository-owned commands for:

1. formatting and style;
2. compilation or type checking with warnings treated as failures;
3. unit tests;
4. real-PostgreSQL integration tests;
5. available race, concurrency, or stress checks;
6. fuzz or property tests for parsers and invariants;
7. dependency vulnerability and license checks;
8. API and event contract validation;
9. container build and smoke testing; and
10. the failure drills required by the current applied chapter.

Document exact commands in the root README if the reference implementation is
ported. A clean clone must reproduce them without hidden IDE state.

## Completion standard in another language

The port is complete only when another engineer familiar with that ecosystem can:

- explain the contracts, invariants, and ownership boundaries;
- reproduce the build and tests from a clean clone;
- prove transaction and authorization behavior under contention and negative cases;
- diagnose a failed request from telemetry before reading source;
- deploy compatible application and schema changes;
- restore data and rehearse the documented incident path;
- defend why the chosen framework features and abstractions are necessary; and
- change the system safely without relying on Go-specific assumptions.

This keeps the repository Go-first without confusing language fluency with backend
engineering judgment.
