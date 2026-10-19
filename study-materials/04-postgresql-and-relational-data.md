# PostgreSQL and Relational Data

This chapter expands the relational-data competency module in the
[overview and competency map](00-overview-and-competency-map.md#relational-data-and-postgresql).
It explains the required relational and transaction concepts and guides the first
PostgreSQL-backed implementation.

## Learning outcomes

By the end of this lesson, you should be able to:

- model products, orders, order items, and idempotency records relationally;
- choose primary, foreign, unique, check, and nullability constraints;
- explain the read/write/storage tradeoff of an index;
- execute parameterized SQL through a bounded pool;
- define a business-sized transaction boundary;
- prevent overselling under concurrent requests;
- recognize common deadlock causes and respond safely;
- apply and test ordered migrations;
- use `EXPLAIN` to inspect a query plan;
- distinguish unit tests from database integration tests.

### How to use this lesson

Move through this lesson from rules to storage, not from tables to application
code:

```text
business rule -> relational constraint -> query/transaction -> failure test
```

For example, "inventory never becomes negative" first becomes an invariant, then a
`CHECK` constraint and conditional update, and finally a two-buyer contention test.
The database is not replacing application logic. It is the last shared authority
when concurrent requests or another writer bypass an earlier application check.

### Before you start: translate database words into responsibilities

Keep this small vocabulary beside you:

| Term | Plain-language meaning |
| --- | --- |
| Database | The durable collection of related application data |
| Table | A named collection of rows describing one kind of fact |
| Row | One stored fact, such as one product |
| Column | One attribute of every row, such as `sku` or `price_cents` |
| Constraint | A rule PostgreSQL refuses to let stored data violate |
| Query | A request to read or change data |
| Connection | One live conversation between the application and PostgreSQL |
| Transaction | Several changes that become durable together or not at all |
| Migration | A versioned change to the database structure |
| Index | An extra lookup structure that trades write/storage cost for faster reads |

Follow one business promise through the whole lesson:

> Two customers competing for the final unit must never create two successful
> orders or make inventory negative.

The application can check inventory, but two requests can check at the same time.
PostgreSQL therefore becomes the final coordinator. Modeling, constraints, queries,
transactions, locks, and integration tests are different tools used to preserve
that one promise. Do not study them as unrelated database features.

## 1. Relational modeling begins with invariants

A table is not merely a serialized Go struct. It represents durable facts and
constraints that must remain true even when requests race or another tool writes
to the database.

For the learning service:

- `products` owns product identity, SKU, display data, price, and available stock;
- `orders` owns one customer's order and its lifecycle state;
- `order_items` records which product, quantity, and captured unit price belong to
  an order;
- `idempotency_records` links one caller and key to one logical request outcome.

Normalize facts that have independent identity and lifecycle. Preserve historical
facts deliberately: an order item stores the unit price charged at order time
instead of reading the product's current price later.

Consider a product whose current price changes after an order is placed:

```text
products
  TEA-001 | current price 700

order_items
  order O-42 | TEA-001 | quantity 2 | captured unit price 500
```

The order total remains 1,000 cents because `order_items` records the historical
fact. Joining to the product's current price would silently rewrite commercial
history. Relational modeling is therefore about deciding which facts belong
together and which must remain independently durable, not merely eliminating
duplicate fields.

## 2. Keys, constraints, and nullability

A primary key provides stable row identity. A foreign key ensures that a referenced
row exists. A unique constraint protects a business key. A check constraint rejects
invalid values. `NOT NULL` says absence is not part of the model.

An initial migration can express the rules directly:

```sql
CREATE TABLE products (
    id uuid PRIMARY KEY,
    sku text NOT NULL UNIQUE,
    name text NOT NULL,
    price_cents bigint NOT NULL CHECK (price_cents >= 0),
    inventory_quantity bigint NOT NULL CHECK (inventory_quantity >= 0),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE TABLE orders (
    id uuid PRIMARY KEY,
    customer_id text NOT NULL,
    status text NOT NULL CHECK (status IN ('pending', 'placed', 'canceled')),
    total_cents bigint NOT NULL CHECK (total_cents >= 0),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE TABLE order_items (
    order_id uuid NOT NULL REFERENCES orders(id),
    product_id uuid NOT NULL REFERENCES products(id),
    quantity bigint NOT NULL CHECK (quantity > 0),
    unit_price_cents bigint NOT NULL CHECK (unit_price_cents >= 0),
    PRIMARY KEY (order_id, product_id)
);

CREATE TABLE idempotency_records (
    customer_id text NOT NULL,
    idempotency_key text NOT NULL,
    request_fingerprint bytea NOT NULL,
    order_id uuid REFERENCES orders(id),
    response_status integer,
    response_body jsonb,
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    PRIMARY KEY (customer_id, idempotency_key)
);
```

The database uses `snake_case`; Go can use idiomatic field names. Mapping between
them is an adapter responsibility.

Do not use nullable columns by default. `NULL` adds a third state distinct from a
zero value and must have a business meaning. For an idempotency record, response
fields may be null while work is in progress; for a product SKU, absence has no
valid meaning.

Application validation produces friendly errors. Database constraints remain the
authoritative final barrier when concurrent callers pass the same earlier check.

Each constraint answers a different question:

| Constraint | Question it makes the database answer |
| --- | --- |
| Primary key | Which durable row is this? |
| Foreign key | Does the referenced owner or parent exist? |
| Unique | Can two rows claim the same business identity? |
| Check | Is this value or combination of values allowed? |
| `NOT NULL` | Is absence a valid state for this fact? |

Constraints should also reflect cross-row ownership where possible. If data later
becomes tenant-scoped, a foreign key that checks only an object ID may prove that
the object exists without proving it belongs to the same tenant. Schema design must
encode the actual invariant, not just create syntactically valid relationships.

## 3. Indexes are not free

An index is an additional ordered data structure that helps PostgreSQL locate rows
without scanning the whole table. It costs storage and makes inserts, updates, and
deletes maintain another structure.

Indexes should follow actual access patterns. Initial useful examples are:

```sql
CREATE INDEX orders_customer_created_id_idx
    ON orders (customer_id, created_at DESC, id DESC);

CREATE INDEX products_created_id_idx
    ON products (created_at ASC, id ASC);
```

Column order matters. The first index supports filtering by `customer_id` and then
cursor pagination ordered by `created_at, id`. An index on every column is not a
strategy; it increases write cost without guaranteeing a useful query path.

Think of a composite index as a sorted directory. An index on
`(customer_id, created_at, id)` groups one customer's entries first and orders that
group by time and ID. A query beginning with `customer_id` can enter the relevant
part of the directory directly. A query filtering only by `created_at` may not be
able to use that ordering efficiently because entries are grouped by customer
first. Query shape and index column order must be designed together.

Unique constraints usually create a unique index automatically. Do not add a second
identical index.

## 4. Migrations are ordered production changes

A migration gives a schema change a stable order and a repeatable application path.
Use the repository's chosen tool, Goose, and keep SQL migrations under `migrations/`.

```text
migrations/
|-- 00001_create_products.sql
|-- 00002_create_orders.sql
`-- 00003_create_idempotency_records.sql
```

A Goose SQL file separates directions:

```sql
-- +goose Up
CREATE TABLE example (...);

-- +goose Down
DROP TABLE example;
```

Production rollback is not always a down migration. Dropping a column can destroy
data. Prefer a new forward migration that restores compatibility, and use down
migrations only when their safety is understood.

Suppose products gain an optional description. A safe first migration is concrete
and additive:

```sql
-- +goose Up
ALTER TABLE products ADD COLUMN description text;

-- +goose Down
ALTER TABLE products DROP COLUMN description;
```

The existing application keeps working because old inserts may omit the nullable
column. A later application version can start writing it. Making the column `NOT
NULL` belongs in a later migration only after existing rows are backfilled and every
supported writer supplies a value. This is the practical meaning of a migration
being compatible with more than one application version.

Required migration evidence:

- applying every migration to an empty database succeeds;
- applying migrations in the documented deployment path succeeds;
- constraints reject invalid direct SQL writes;
- the application and migration can coexist with any version expected during a
  rolling deployment;
- migrations run as an explicit deployment step, not unpredictably in every app
  instance at startup.

## 5. Run a disposable local PostgreSQL

Define PostgreSQL in a local Compose file with an explicit version, health check,
port, database, and credentials used only for development. Keep real secrets out of
Git.

```yaml
services:
  postgres:
    image: postgres:17
    environment:
      POSTGRES_DB: be_standard
      POSTGRES_USER: be_standard
      POSTGRES_PASSWORD: local-development-only
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U be_standard -d be_standard"]
      interval: 2s
      timeout: 2s
      retries: 15
```

Pin the image version selected when implementation begins rather than relying on
`latest`. A named volume is convenient for manual development. Integration tests
should start from disposable state so old local data cannot hide migration errors.

## 6. A connection pool is a bounded shared resource

`pgxpool.Pool` manages several PostgreSQL connections. It is not one connection and
it is not unlimited. Every query competes for pool capacity.

At startup:

1. Parse the database URL into pool configuration.
2. Set a deliberate maximum connection count and lifetimes.
3. Create the pool with a startup context.
4. Ping with a tight deadline.
5. Fail startup with useful context if the database is unavailable.
6. Close the pool during process shutdown.

If the service has 10 instances and each permits 20 connections, it may consume 200
database connections before migrations, administration, and failover capacity.
Choose the pool size as a system budget, not an arbitrary per-process default.

Pool saturation appears as time waiting to acquire a connection. A query deadline
must cover acquisition and execution by passing the same context:

```go
row := pool.QueryRow(ctx, query, sku)
```

A pool does not make PostgreSQL infinitely parallel. It creates a queue in front of
a limited resource:

```text
40 concurrent requests
        |
        v
10 pool connections -> PostgreSQL
30 requests wait for a connection or reach their deadline
```

Increasing the pool may reduce waiting in one instance while overwhelming the
database across the fleet. Measure acquisition wait, query latency, and database
saturation before changing the limit. A canceled waiting request should leave the
queue promptly because the same context covers both acquisition and execution.

## 7. Parameterized SQL keeps code separate from data

Never concatenate untrusted values into SQL:

```go
// Unsafe: input can change SQL syntax.
query := "SELECT id FROM products WHERE sku = '" + sku + "'"
```

Use parameters:

```go
const getProductSQL = `
    SELECT id, sku, name, price_cents, inventory_quantity, created_at, updated_at
    FROM products
    WHERE sku = $1
`

var product Product
err := pool.QueryRow(ctx, getProductSQL, sku).Scan(
    &product.ID,
    &product.SKU,
    &product.Name,
    &product.PriceCents,
    &product.InventoryQuantity,
    &product.CreatedAt,
    &product.UpdatedAt,
)
```

The driver sends SQL structure and parameter data separately. Parameters protect
values, not dynamic identifiers such as a column name. Implement sortable columns
with a fixed allowlist, never client text interpolation.

If `sku` contains `TEA-001' OR true --`, concatenation turns those characters into
part of the SQL program. With `$1`, PostgreSQL receives the entire string as one
value to compare:

```text
SQL structure: SELECT ... WHERE sku = $1
parameter $1:  TEA-001' OR true --
```

The quote and comment marker lose their SQL meaning because they are data. A table
or column name cannot be supplied through `$1`, so user-selected sorting must map a
small public option such as `created` to a hard-coded SQL fragment.

Translate `pgx.ErrNoRows` into a stable product-level not-found error at the adapter
boundary. Translate the specific SKU uniqueness violation into duplicate SKU.
Avoid turning every database error into not-found or conflict; unexpected failures
must remain visible.

Close `Rows` and check its terminal error:

```go
rows, err := pool.Query(ctx, query, args...)
if err != nil {
    return nil, fmt.Errorf("query products: %w", err)
}
defer rows.Close()

for rows.Next() {
    // Scan one row.
}
if err := rows.Err(); err != nil {
    return nil, fmt.Errorf("iterate products: %w", err)
}
```

## 8. Transactions preserve a complete business operation

A transaction groups statements into one atomic commit. If order creation requires
inventory reservation, order insertion, and item insertion, those steps belong in
one transaction. Repository methods should not each start and commit independent
transactions because partial success would become durable.

Before commit, changes are tentative. After a successful commit, they become one
durable outcome visible according to the database's isolation rules:

```text
BEGIN
  reserve inventory       -- tentative
  insert order            -- tentative
  insert order items      -- tentative
COMMIT                     -- all become durable together
```

If item insertion fails, rollback removes the preceding inventory change as though
this transaction never committed. A transaction cannot roll back a separate email,
HTTP request, or message already sent to another system, which is why remote calls
do not belong inside this local atomic boundary.

A safe shape is:

```go
tx, err := pool.Begin(ctx)
if err != nil {
    return fmt.Errorf("begin place-order transaction: %w", err)
}
defer tx.Rollback(ctx)

// Execute every step through tx.

if err := tx.Commit(ctx); err != nil {
    return fmt.Errorf("commit place-order transaction: %w", err)
}
```

Calling rollback after a successful commit is harmless for pgx and ensures every
earlier return attempts cleanup. Keep the transaction short: do not call remote
services or wait for user input while holding database locks.

## 9. Prevent overselling atomically

This sequence is unsafe even inside application code with no data races:

1. Read inventory quantity.
2. If enough, calculate a new quantity.
3. Update the row.

Two transactions can both read the old value. Protect the check and change in the
database with one conditional update:

```sql
UPDATE products
SET inventory_quantity = inventory_quantity - $1,
    updated_at = $2
WHERE id = $3
  AND inventory_quantity >= $1
RETURNING price_cents;
```

With one unit remaining, PostgreSQL serializes the conflicting row updates:

```text
Transaction A evaluates quantity >= 1, decrements 1 -> 0, and commits
Transaction B then evaluates quantity >= 1 against the current row -> false
Transaction B receives no row and cannot decrement below zero
```

The important property is that the condition and write are one database statement
on the same row. An application-side `SELECT` followed later by `UPDATE` leaves a
gap in which another transaction can make the earlier observation stale.

If no row is returned, either the product does not exist or inventory is
insufficient. Decide whether the public contract needs to distinguish those cases;
if it does, issue a protected follow-up query within the same transaction.

For orders containing several products, process product IDs in a deterministic
sorted order. Consistent lock acquisition reduces deadlocks. It cannot eliminate
all deadlocks, so recognize PostgreSQL's deadlock/serialization error and retry the
entire transaction only with a strict attempt limit, backoff, and an idempotent
operation boundary.

Without a shared order, two transactions can form a cycle:

```text
Transaction A locks TEA, then waits for CUP
Transaction B locks CUP, then waits for TEA
```

Neither can continue, so PostgreSQL detects the deadlock and aborts one. Sorting
both requests as `CUP, TEA` means one transaction waits before holding the second
transaction's needed lock; the cycle does not form for this code path. The retry
still covers deadlocks caused by other access paths, and it must repeat the whole
transaction because the aborted transaction committed none of its work.

Isolation levels describe which concurrent anomalies a transaction may observe.
Do not select `SERIALIZABLE` as a magic correctness switch. It can reject conflicting
transactions, which means the application needs bounded whole-transaction retry.
For today's reservation, the conditional update directly expresses the invariant.

## 10. Prove rollback behavior

An integration test should force a failure after inventory reservation but before
order completion. One deterministic technique is to supply an order item that
violates a database constraint after the inventory update.

The test must verify:

- `PlaceOrder` returns an error;
- no order row exists;
- no order-item row exists;
- the original inventory quantity remains unchanged.

This is the day's must-finish proof. Merely inspecting the transaction code is not
enough; the real database must demonstrate rollback.

Add a second integration test that launches two attempts for the final unit. Expect
one committed order, one insufficient-inventory result, and final inventory zero.

## 11. Integration-test boundaries

Unit tests remain useful for calculations and state rules. Database behavior needs
a real PostgreSQL because a mock cannot reproduce:

- SQL syntax and types;
- unique, foreign-key, and check constraints;
- isolation and row locks;
- deadlocks and serialization failures;
- transaction commit and rollback behavior;
- query plans and indexes.

Each test should own deterministic data. Use unique IDs, reset schema state between
tests, or isolate tests in transactions when doing so does not hide the behavior
under test. Bound database startup and readiness waits; never loop forever.

## 12. Pagination and query plans

For stable product pagination, use a total order such as `(created_at, id)`:

```sql
SELECT id, sku, name, price_cents, inventory_quantity, created_at, updated_at
FROM products
WHERE (created_at, id) > ($1, $2)
ORDER BY created_at ASC, id ASC
LIMIT $3;
```

The unique `id` tie-breaker prevents rows with equal timestamps from being skipped
or repeated. Request one extra row to determine whether another page exists, then
return at most the public page limit.

Inspect the plan with representative data:

```sql
EXPLAIN (ANALYZE, BUFFERS)
SELECT ...;
```

`EXPLAIN` estimates without running the query. `EXPLAIN ANALYZE` executes it, so do
not casually use it on mutating statements or expensive production workloads.
Look at actual versus estimated rows, scan type, loop counts, sort operations, and
buffer activity. A sequential scan is not automatically bad for a small table.

Read a plan from the most deeply indented operation outward. Ask:

1. How many rows did this operation expect and actually process?
2. Did it scan the whole relation or use an index condition matching the query?
3. Did a sort or nested loop repeat far more work than expected?
4. Were most rows discarded by a filter after being read?
5. Does the test data resemble production volume and distribution?

A plan on ten uniformly distributed rows proves little about a million-row table
with one unusually large customer. Representative data shape is part of the
evidence.

Avoid N+1 access, where one query loads orders and one additional query runs for
every order's items. Fetch related data in a bounded number of queries or a join,
then assemble results deliberately.

## 13. Guided build

Complete these steps in order:

1. Add a pinned local PostgreSQL service and documented startup command.
2. Add Goose migrations for all four tables and required indexes.
3. Prove migrations succeed from an empty database.
4. Implement PostgreSQL product create, get, and paginated list operations.
5. Map only known constraint and no-row conditions to stable domain errors.
6. Add `/health/ready` with a tight database ping timeout.
7. Implement the order transaction with atomic conditional inventory updates.
8. Add rollback and last-unit concurrency integration tests.
9. Inspect the product-list query plan with representative rows.

Readiness should return failure when the pool cannot acquire and ping promptly. It
should not expose database addresses or error strings publicly.

## 14. Knowledge check

1. Why enforce duplicate SKU in both application logic and a unique constraint?
2. When should a column be nullable?
3. Why is a connection pool a system-wide capacity concern?
4. What does parameterization protect, and what does it not protect?
5. Why must the transaction boundary surround the full order operation?
6. How does the conditional inventory update prevent overselling?
7. Why sort product IDs before reserving several products?
8. Why can only a real database test prove transaction and lock behavior?
9. Why is a sequential scan not always a defect?

### Expected answers

1. Application checks provide useful feedback; the database resolves races and
   protects against other writers.
2. Only when absence has a defined domain meaning distinct from every value.
3. Instance count multiplied by per-instance maximum must fit database capacity
   with room for operations and failover.
4. It separates values from SQL syntax; dynamic identifiers still require an
   allowlist.
5. Otherwise an intermediate repository commit can leave partial durable state.
6. The database evaluates availability and decrements it as one protected statement.
7. Consistent lock order reduces cyclic waits and deadlocks.
8. Mocks do not implement PostgreSQL constraints, isolation, locks, or driver rules.
9. Reading a small portion of a tiny table can be cheaper than using an index.

## 15. Completion evidence

This chapter is complete when:

- an empty disposable database reaches the full schema through migrations alone;
- direct SQL cannot insert duplicate SKU, negative inventory, or an orphan item;
- product queries are parameterized and honor cancellation;
- rows, transactions, and pools are released on every path;
- a forced mid-operation failure leaves inventory and order tables unchanged;
- two buyers competing for the last unit produce exactly one committed order;
- the product pagination plan and index choice are explained in writing;
- formatting, unit tests, integration tests, race checks, and vet pass.

Record the observed failure mode and the database rule that prevents it. Then move
to [Business logic and architecture](05-business-logic-and-architecture.md).
