# Service Core

Complete these lessons sequentially. Each section adds one reviewable vertical
capability while preserving evidence from earlier work.

## Decisions and Executable Skeleton

### Goal and mental model

Build the smallest process that starts from validated configuration, exposes a
composition root, shuts down within a deadline, and can be verified locally and in
CI. A skeleton is executable evidence, not a tree of empty packages.

`cmd/api/main.go` owns process lifecycle: parse config, construct dependencies,
start the server, observe signals, drain, close dependencies, and select the exit
code. Business packages must not discover environment variables or initialize
global infrastructure.

Typed configuration converts strings at startup and rejects missing URLs, invalid
durations, nonpositive limits, or unsafe combinations. Errors identify the field
but never print secret values.

Architecture decision records capture context, alternatives, decision, consequences,
and reconsideration triggers. Record why the initial system is a modular monolith,
why PostgreSQL is chosen, why authentication is external, and why SQL remains
visible before code generation.

Local commands and CI must share one implementation. Make targets may wrap scripts,
but CI should not contain a second set of hidden build rules.

### Start with a concrete situation

A new developer clones the repository and runs the documented start command. There
are only two acceptable outcomes: the service starts with validated configuration,
or it exits with an error that identifies what must be fixed. It must not listen on
a port and fail later on the first request because a required dependency was never
configured.

Trace the successful lifecycle before creating packages:

```text
process starts
  -> parse and validate configuration
  -> construct database, services, handlers, and server
  -> listen and serve
  -> receive SIGTERM
  -> stop new work and drain within a deadline
  -> close dependencies
  -> exit
```

`main` is called the **composition root** because it is the place where concrete
parts are constructed and connected. An architecture decision record (ADR) is a
short document explaining why an important choice was made, which alternatives were
considered, and what would cause the choice to be revisited. Neither concept
requires a framework.

First make the lifecycle work with one trivial health handler. Only then add
PostgreSQL or business behavior. That isolates process-lifecycle mistakes from
feature mistakes.

### Guided work

1. Verify the existing module declaration and add the `cmd/api` command. Use
   `go mod init` only when starting outside this already-initialized repository.
2. Define typed config with explicit required fields and safe defaults.
3. Add a minimal server with explicit timeouts and signal handling.
4. Implement bounded shutdown and distinguish normal `http.ErrServerClosed` from
   startup/listen failure.
5. Add local PostgreSQL configuration without connecting business code globally.
6. Write four ADRs for architecture, database, auth boundary, and SQL approach.
7. Add repository commands for format check, vet, unit tests, race tests,
   vulnerability scan, and build.
8. Make CI call the same commands.
9. Verify a clean clone, missing configuration, SIGTERM, and a failing check.

### Failure checks

- Start without each required variable and confirm failure occurs before listening.
- Send SIGTERM during an in-flight request and confirm drain is bounded.
- Break formatting or a test and prove local and CI commands fail identically.
- Search business packages for environment access and global dependency discovery.

### Knowledge check and expected answers

1. **Why is `main` the composition root?** It owns concrete construction and
   lifecycle while feature code depends only on explicit inputs.
2. **Why fail startup on invalid config?** Serving with unknown preconditions creates
   later, less diagnosable runtime failure.
3. **What makes an ADR useful?** It preserves alternatives and tradeoffs so a
   decision can be revisited when its context changes.

### Completion evidence

A clean clone follows one documented setup path; invalid configuration fails with
field context and no secret; SIGTERM drains within its limit; local and CI checks
match; and the ADRs explain decisions without claiming future abstractions exist.

## HTTP Platform

### Goal and mental model

Create one bounded, consistent HTTP boundary used by feature handlers. Handlers
decode the public protocol, validate its representation, call a use case, and map
known outcomes. They do not contain SQL or core business rules.

HTTP method semantics, content type, body size, unknown-field rejection, exactly one
JSON value, status codes, and headers are part of the contract. A problem-details
shape gives clients a stable status and code while keeping internal errors private.

Middleware order is behavior. Request ID must run first. Access logging should wrap
recovery so it observes the final recovered status. Authentication will later run
inside recovery. Use route templates in logs and metrics, never raw ID-bearing paths.

Liveness checks only process life. Readiness checks whether the instance should
receive traffic. In this section it reflects application lifecycle, such as startup
completion and draining. The persistence section adds a bounded PostgreSQL probe
after that dependency exists. Both endpoints must have stable semantics documented
in OpenAPI, the machine-readable description of the HTTP API.

### Start with a concrete situation

Send one malformed request:

```text
POST /v1/products
Content-Type: application/json

{"sku":"TEA-001","unexpected":true}
```

Follow it through the boundary. The server limits how many bytes it will read, the
decoder rejects the unknown field, the handler creates a stable public problem, and
the access logger records the final 400 response. The product use case never runs.

That one path explains the platform's responsibilities:

```text
untrusted bytes
  -> protocol limits and decoding
  -> transport validation
  -> application call only when valid
  -> public response encoding
```

Build and test this negative path before adding several successful endpoints. It
forces headers, error shape, middleware order, and logging to become observable
instead of remaining architectural claims.

### Guided work

1. Define OpenAPI for health and the currently implemented routes.
2. Build response and problem encoders that set headers before status.
3. Build strict JSON decoding with media-type parsing and byte limits.
4. Add request-ID validation/generation, access logging, and panic recovery.
5. Compose middleware as request ID -> access log -> recovery -> route.
6. Add live and ready handlers with distinct lifecycle behavior; leave PostgreSQL
   readiness probing for the persistence section.
7. Configure server header, read, write, idle, and shutdown timeouts.
8. Test handlers with `httptest`; do not bind a port for unit-level HTTP tests.
9. Compare actual examples with the OpenAPI contract.

### Failure checks

Send wrong content type, malformed JSON, unknown fields, trailing JSON, an oversized
body, and a panic. Confirm deterministic 4xx outcomes, one generic 500, correlation
in logs, and no internal error text. Put the application into its draining state:
readiness should fail promptly while liveness remains healthy.

### Knowledge check and expected answers

1. **Why reject unknown fields?** Misspellings and stale clients fail visibly instead
   of silently losing intent.
2. **Why does access logging wrap recovery?** It must observe the 500 response that
   recovery writes after a panic.
3. **Why exclude PostgreSQL from liveness?** Restarting the app cannot repair the
   dependency and can amplify an outage.

### Completion evidence

The OpenAPI contract matches examples; every untrusted body is bounded and strictly
decoded; all negative cases are deterministic; panics correlate to a private log;
timeouts are explicit; and health tests prove liveness/readiness separation.

## Persistence Foundation

### Goal and mental model

Make PostgreSQL an explicit adapter with bounded connection capacity, ordered
migrations, authoritative constraints, contextual queries, and real integration
tests. Mocks cannot validate SQL, types, constraints, locks, or rollback.

Treat pool size as a system budget: instance count multiplied by per-instance maximum
must leave room for migrations, operators, monitoring, and failover. The same
context should bound connection acquisition and query execution.

Migrations must build the entire schema from empty state. Application checks create
good errors; unique, check, foreign-key, and not-null constraints remain the final
integrity barrier under concurrency or alternate writers.

Every query is parameterized. Rows are closed and terminal iteration errors checked.
Transactions roll back on every non-commit path. Driver errors are translated only
when their identity is known; unexpected failures remain visible with context.

### Start with a concrete situation

Two requests try to create `TEA-001` at nearly the same time. Both can check
"does this SKU exist?" before either inserts. Application code alone therefore
cannot make the uniqueness promise. A PostgreSQL unique constraint makes one insert
succeed and the other fail regardless of timing.

Use that race to separate responsibilities:

```text
application validation -> gives the caller an early, understandable answer
database constraint    -> remains correct when requests race
error translation      -> turns a known database outcome into ErrDuplicateSKU
integration test       -> proves the real database enforces it
```

Treat the connection pool like a small set of checkout tokens. A query must acquire
one token, use the connection for bounded work, and return it. If code leaks rows or
transactions, tokens disappear from circulation until new requests wait or time
out. Pool configuration is therefore part of correctness under load, not merely a
performance setting.

### Guided work

1. Add a pinned local PostgreSQL service and health check.
2. Configure pgx pool limits, lifetimes, and startup ping deadline.
3. Extend readiness with a tightly bounded PostgreSQL acquisition and ping check;
   keep liveness independent of PostgreSQL.
4. Add Goose migration commands and the initial product schema.
5. Encode SKU uniqueness, nonnegative price/inventory, required fields, and times.
6. Implement product insert/get/list with parameterized SQL.
7. Translate no-row and specific unique-constraint outcomes.
8. Build an integration harness that starts from migrated disposable state.
9. Test direct invalid SQL writes to prove constraints.
10. Test cancellation during pool wait or query execution.
11. Run empty-database migration verification in CI.

### Failure checks

Exhaust the pool within a bounded test and ensure acquisition ends with context.
Cancel a query and verify resources return to the pool. Inject duplicate and invalid
rows directly through SQL. Interrupt migration setup and confirm the next run has a
clear recovery path rather than silently assuming success.

### Knowledge check and expected answers

1. **Why enforce a rule twice?** Application logic communicates business intent;
   database constraints resolve races and protect every writer.
2. **What does SQL parameterization not protect?** Dynamic identifiers; columns and
   sort directions require a fixed allowlist.
3. **Why use real PostgreSQL tests?** Only PostgreSQL reproduces its syntax, types,
   constraints, locking, isolation, and driver lifecycle.

### Completion evidence

Migrations recreate the schema from empty state; constraints reject bypass writes;
all queries accept context and parameters; rows, transactions, and cancellations
release resources; pool limits are documented as a fleet budget; and integration
tests run in CI.

## Product Slice

### Goal and mental model

Deliver the first complete feature slice: transport -> use case -> persistence.
Keep product behavior together while dependency direction points inward. Add only
interfaces required by a real consumer.

The create use case owns product input rules and admin authorization. PostgreSQL owns
durable uniqueness. The handler maps stable error identities to public problems.
Authentication absence (401) and authenticated lack of permission (403) are distinct.

Cursor pagination represents a position in a total order. Use `(created_at, id)` or
another deterministic unique pair. Encode it opaquely, validate decoded shape, apply
a maximum page size, query strictly after the cursor, and request one extra row to
determine whether a next cursor exists.

Document ordering assumptions. A mutable sort key can move records between pages;
prefer immutable keys or define the consistency tradeoff.

### Start with a concrete situation

Trace one successful product creation all the way through:

```text
HTTP JSON
  -> createProductRequest
  -> CreateProduct command
  -> authorization and product rules
  -> narrow repository call
  -> parameterized INSERT
  -> Product result
  -> 201 response
```

At each arrow, identify what becomes trusted and what representation changes. The
JSON is caller-controlled. The command includes only fields the application accepts.
The use case decides whether the principal may create products. PostgreSQL makes SKU
uniqueness durable. The handler alone chooses status 201 and the response shape.

Implement this one path vertically before building a generic repository or global
layer hierarchy. Then add one failure at a time: anonymous, authenticated but
forbidden, duplicate SKU, and malformed cursor. Each failure should stop at the
boundary that owns it.

### Guided work

1. Define create/list commands and domain results without JSON or pgx types.
2. Define narrow persistence capabilities in the consuming product code.
3. Enforce admin authorization in the application operation.
4. Implement create and cursor-list SQL with stable ordering.
5. Translate unique SKU violation to `ErrDuplicateSKU`.
6. Add POST/GET handlers and update OpenAPI with examples and limits.
7. Test unauthenticated, forbidden, duplicate, malformed cursor, boundary page size,
   empty page, and multi-page traversal.
8. Insert equal timestamps and prove the ID tie-breaker prevents omission.
9. Inspect the query plan with representative data.

### Failure checks

Attempt product creation as anonymous, customer, and admin principals. Reuse a cursor
after inserting records around its position. Tamper with cursor bytes. Request zero,
negative, and excessive page sizes. Ensure errors stay stable and SQL details never
reach clients.

### Knowledge check and expected answers

1. **Why is an offset not a cursor?** It counts rows rather than naming a stable
   position and shifts when preceding rows change.
2. **Why add a unique tie-breaker?** Equal primary sort values otherwise make the
   boundary ambiguous and can skip or repeat records.
3. **Where is admin authorization enforced?** In the use case, with middleware only
   establishing the trusted principal.

### Completion evidence

The vertical slice has clear boundaries; duplicate SKU maps to a stable conflict;
401 and 403 are tested; cursor traversal is deterministic under documented
assumptions; public docs match behavior; and the query plan supports the chosen
filter/order.

## Order Transaction

### Goal and mental model

Commit one business invariant: order, items, captured prices, and inventory
reservation either all become durable or none do. An application read followed by
an update is not concurrency protection.

Use an atomic conditional update such as:

```sql
UPDATE products
SET inventory_quantity = inventory_quantity - $1,
    updated_at = $2
WHERE id = $3
  AND inventory_quantity >= $1
RETURNING price_cents;
```

For several products, acquire locks in sorted product-ID order to reduce deadlocks.
Calculate totals with checked integer arithmetic. Insert items with captured unit
prices so future catalog changes do not rewrite order history.

Scope reads in SQL by both order ID and authenticated customer ID. Wrong-owner and
nonexistent reads return the same public not-found response.

### Start with a concrete situation

Inventory contains one unit of `TEA-001`. Two customers place an order concurrently:

```text
Request A reads quantity 1
Request B reads quantity 1
Request A decides it can buy
Request B decides it can buy
```

If each request later writes `0`, both orders may succeed even though only one unit
existed. No Go data race is required; the race occurs between database operations.

The protected operation must combine the condition and change:

```sql
UPDATE products
SET inventory_quantity = inventory_quantity - 1
WHERE id = $1
  AND inventory_quantity >= 1
RETURNING price_cents;
```

Only one transaction can successfully obtain the returned row for the final unit.
The transaction then inserts the order and items before committing. If any insert
fails, rollback restores the inventory change. The lesson is not "transactions are
good"; it is that the database must protect the entire business promise at the
point where concurrent requests meet.

### Guided work

1. Add order, order-item, and inventory migrations with integrity constraints.
2. Define `PlaceOrder` input rules: nonempty, positive quantities, no duplicate
   product IDs, bounded item count, and valid principal.
3. Define one atomic placement persistence capability.
4. Begin a transaction, reserve products in deterministic order, capture prices,
   calculate total, insert order/items, and commit.
5. Defer rollback for every earlier exit.
6. Translate known missing-product, inventory, and conflict outcomes.
7. Add owner-scoped get and list queries.
8. Add handler mappings and OpenAPI examples.
9. Test a forced failure after reservation and before order insertion.
10. Test two buyers for the final unit with synchronization, not sleeps.

### Failure checks

Force item insertion to violate a constraint and verify inventory is unchanged.
Cancel the context before commit. Place a multi-product order concurrently in reverse
input order and verify internal sorting avoids inconsistent lock order. Inspect every
public error for leaked table, constraint, or SQL text.

### Knowledge check and expected answers

1. **Why does the transaction surround the business operation?** Independent
   repository commits can leave durable partial state.
2. **Why is race-detector success insufficient?** Database instances and requests can
   interleave without sharing Go memory; business races require database protection.
3. **Why scope ownership in SQL?** Unauthorized data never enters application memory
   and a later check cannot be forgotten.

### Completion evidence

Rollback tests prove zero partial state; contention produces exactly one final-unit
success and never negative inventory; lock order and error mapping are documented;
owner-scoped retrieval leaks no existence; and all tests remain deterministic under
race detection.
