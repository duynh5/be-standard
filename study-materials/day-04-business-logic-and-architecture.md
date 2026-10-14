# Day 4: Business Logic and Architecture

This lesson expands Day 4 of the
[backend learning roadmap](../BACKEND_LEARNING_ROADMAP.md#day-4-business-logic-and-architecture).
It turns the HTTP and PostgreSQL work into feature-oriented modules and places the
order transaction behind one explicit application operation.

## Learning outcomes

By the end of this lesson, you should be able to:

- distinguish transport, application, domain, and persistence representations;
- organize code by business capability without adding ceremonial layers;
- state domain invariants independently of HTTP and SQL;
- define narrow interfaces at real substitution boundaries;
- inject dependencies with ordinary constructors;
- place a transaction around one complete business operation;
- map stable application errors at the HTTP boundary;
- test rules separately from database transaction behavior;
- explain why a modular monolith is the appropriate current deployment shape.

### How to use this lesson

Architecture becomes easier when you stop asking "how many layers should this
project have?" and ask three more concrete questions:

1. Which code interprets an external representation?
2. Which code decides whether the business action is valid?
3. Which code performs the durable side effect?

For order placement, the answers are the HTTP handler, order use case, and
PostgreSQL adapter. Follow one request through those three roles while reading this
lesson. A boundary earns its place when it protects a different responsibility or
testable risk, not when it makes a folder diagram look symmetrical.

### Before you start: architecture is assignment of responsibility

The word *architecture* can sound like diagrams and frameworks. In this lesson it
means deciding which code is responsible for each decision:

- the **handler** understands HTTP;
- the **use case** coordinates one user action and its business rules;
- the **domain model** names valid business state;
- the **adapter** translates between the application and an external mechanism such
  as PostgreSQL;
- `main` constructs the concrete pieces and connects them.

Use one change as a test. Suppose the public JSON field changes from `product_id` to
`productId`. The HTTP representation should change, but the inventory rule and SQL
transaction should not. Suppose the inventory reservation query changes. The
adapter should change, but the public JSON should not. A boundary is useful when it
contains that kind of change.

Whenever a new type or interface appears, ask: "Which change is this protecting the
rest of the program from?" If there is no concrete answer, the abstraction may be
ceremony rather than architecture.

## 1. Architecture manages reasons to change

Architecture is not a target number of folders. It keeps volatile concerns from
spreading through the system:

- HTTP changes because the public protocol evolves.
- Business rules change because product policy evolves.
- SQL changes because storage and query needs evolve.
- Process wiring changes because infrastructure and operations evolve.

Put code together when it changes for the same reason. Separate it when it has a
different owner, vocabulary, invariant, or dependency direction.

The service remains one deployable and one database. Clear modules reduce cognitive
coupling now and create evidence about possible future boundaries. They do not imply
that a network boundary should be added.

Imagine three likely changes:

| Change | Code that should primarily move |
| --- | --- |
| Rename JSON field `product_id` | HTTP transport types and contract tests |
| Reject duplicate products in an order | Order application/domain validation |
| Change the inventory reservation query | PostgreSQL adapter and integration tests |

If any one change requires editing all three concerns everywhere, the architecture
is spreading reasons to change. If every small feature requires several empty
wrappers with no protected responsibility, it is over-structured. The goal is
local, explainable change.

## 2. One concept can need several representations

An incoming order request is not automatically a domain object or database row.
Each representation has different responsibilities.

### Transport request

```go
type placeOrderRequest struct {
    Items []placeOrderItemRequest `json:"items"`
}

type placeOrderItemRequest struct {
    ProductID string `json:"product_id"`
    Quantity  int64  `json:"quantity"`
}
```

This type owns JSON names and request-shape validation. It must not accept fields
that the client is forbidden to choose, such as `CustomerID`, `Status`,
`TotalCents`, or `CreatedAt`.

### Application command

```go
type PlaceOrderCommand struct {
    CustomerID string
    Items      []OrderItemInput
}
```

The authenticated customer ID comes from the trusted principal, not the JSON body.
The command represents an application action after transport decoding.

### Domain result

```go
type Order struct {
    ID         string
    CustomerID string
    Status     Status
    Items      []Item
    TotalCents int64
    CreatedAt  time.Time
}
```

This representation uses domain vocabulary and valid states. It has no JSON or SQL
tags unless a deliberate local convention proves that sharing them remains safe.

### Persistence rows

SQL may return flat rows containing repeated order columns for each item. The
PostgreSQL adapter assembles those rows into the domain result. Database null types,
driver errors, and column naming remain in the adapter.

Separate representations are useful when they protect a boundary. Do not duplicate
types mechanically when they are genuinely identical and owned by the same module.

Follow a field through the boundary to see why conversion is useful:

```text
JSON "product_id"
  -> placeOrderItemRequest.ProductID   (untrusted transport string)
  -> OrderItemInput.ProductID          (validated application input)
  -> SQL parameter $1                  (adapter representation)
  -> order_items.product_id            (durable relational fact)
```

The values may look identical today, but each stage has a different owner. The
handler must not let the caller set `CustomerID`; it obtains that value from the
trusted principal when constructing the command. The adapter must not leak driver
null types back into the domain. Conversions make those trust and ownership changes
visible.

## 3. Organize vertical feature packages

Use this direction as a starting point:

```text
cmd/api/
    main.go
internal/
    platform/
        database/
        httpserver/
    product/
        handler.go
        service.go
        postgres.go
    order/
        handler.go
        service.go
        postgres.go
```

Feature packages keep one capability's HTTP translation, use cases, storage adapter,
and tests close together. Platform packages contain genuinely shared infrastructure,
not business rules.

Dependency direction matters more than filenames:

```text
HTTP handler -> order use case -> narrow persistence capability
                                      ^
                                      |
                              PostgreSQL adapter

main constructs all concrete values and connects them.
```

The order use case must not import `net/http`, `pgx`, router types, environment
configuration, or telemetry SDKs. The handler may import the order package because
it translates HTTP into order behavior. The PostgreSQL adapter may import pgx
because it translates order persistence needs into SQL.

Avoid packages named `utils`, `common`, or `helpers`. Put a function with the concept
that owns it. Create shared infrastructure only after two real consumers demonstrate
the same stable need.

## 4. Write invariants before orchestration

For `PlaceOrder`, start with rules rather than steps:

- the caller must be an identified customer;
- the order contains at least one item;
- every quantity is positive;
- a product appears at most once, or duplicates are combined by one documented rule;
- every product exists and has sufficient inventory;
- item price is captured when the order commits;
- total equals the sum of captured unit price multiplied by quantity;
- inventory reservation, order creation, and item creation commit atomically;
- the initial committed status is valid;
- arithmetic cannot overflow.

Rules involving only input can be checked before opening a transaction. Rules
involving current database state must be protected inside the transaction.

Classify the rules before writing the control flow:

| Rule | Earliest useful check | Final authority |
| --- | --- | --- |
| At least one item | Application validation | Application validation |
| Quantity is positive | Application validation | Database `CHECK` as defense in depth |
| Product exists | Inside the transaction | Foreign key or protected query/update |
| Inventory is sufficient | Inside the transaction | Conditional update or lock |
| Total does not overflow | Application/adapter calculation | Checked calculation before commit |
| Order and reservation are all-or-nothing | Transaction design | Database commit/rollback |

Checking inexpensive input rules before acquiring a database connection avoids
wasting a scarce resource. Rechecking critical durable rules in PostgreSQL protects
against races and other writers.

Represent order status with a defined type:

```go
type Status string

const (
    StatusPlaced   Status = "placed"
    StatusCanceled Status = "canceled"
)
```

If cancellation is added later, define its state transition explicitly:

```text
placed -> canceled
canceled -> no further transition
```

Do not allow arbitrary strings merely because the database column is text.

## 5. Keep interfaces narrow and consumer-owned

The `PlaceOrder` service needs one durable capability: commit an order while
reserving inventory. Express that need in the order package:

```go
type PlacementRepository interface {
    Place(ctx context.Context, request PlacementRequest) (Order, error)
}
```

The PostgreSQL implementation can use several tables and statements internally.
The interface does not expose `Begin`, `Commit`, SQL rows, or generic CRUD methods.
It names the business-sized persistence operation whose atomicity the application
requires.

This design places the transaction mechanics in the adapter while keeping the
transaction boundary selected by the use case. The service calls exactly one
atomic repository operation after validating the command.

Do not create a repository interface containing every possible product and order
method. A broad interface increases test setup and couples unrelated use cases.
Different use cases may depend on different small interfaces implemented by the
same concrete adapter.

A narrow interface also keeps a unit-test fake honest:

```go
type placementFake struct {
    got    PlacementRequest
    result Order
    err    error
}

func (f *placementFake) Place(_ context.Context, request PlacementRequest) (Order, error) {
    f.got = request
    return f.result, f.err
}
```

This fake can prove that validation prevents a call and that valid input is
translated correctly. It intentionally cannot prove transaction behavior; the
PostgreSQL integration test owns that risk.

## 6. Dependency injection is ordinary construction

The service receives required dependencies through a constructor:

```go
type IDGenerator interface {
    NewID() string
}

type Clock interface {
    Now() time.Time
}

type Service struct {
    repository PlacementRepository
    ids        IDGenerator
    clock      Clock
}

func NewService(repository PlacementRepository, ids IDGenerator, clock Clock) *Service {
    if repository == nil || ids == nil || clock == nil {
        panic("order service requires repository, ID generator, and clock")
    }

    return &Service{repository: repository, ids: ids, clock: clock}
}
```

Constructor parameters make requirements visible and enable deterministic tests.
No dependency-injection framework or service locator is needed. `main` creates the
real PostgreSQL adapter, system clock, ID generator, service, and handler.

The comparisons above reject nil interface values. An interface containing a typed
nil pointer is itself non-nil, so the composition root must not pass typed-nil
implementations. Constructor validation does not make an invalid implementation
safe.

The construction direction and runtime call direction are related but distinct:

```text
startup: main -> constructs repository -> service -> handler
runtime: handler -> calls service -> calls repository
```

`main` knows the concrete types because wiring is its job. The service knows only
the capabilities in its fields. Tests replace those capabilities directly without
a global registry or hidden environment lookup.

For internal programmer errors such as a missing mandatory dependency, failing
immediately during construction is clearer than allowing a nil dereference on the
first request. For runtime input and external failures, return errors instead.

## 7. Design the `PlaceOrder` use case

The application service coordinates policy without knowing SQL:

```go
func (s *Service) PlaceOrder(ctx context.Context, command PlaceOrderCommand) (Order, error) {
    if err := validateCommand(command); err != nil {
        return Order{}, err
    }

    request := PlacementRequest{
        OrderID:    s.ids.NewID(),
        CustomerID: command.CustomerID,
        Items:      append([]OrderItemInput(nil), command.Items...),
        PlacedAt:   s.clock.Now().UTC(),
    }

    order, err := s.repository.Place(ctx, request)
    if err != nil {
        return Order{}, fmt.Errorf("place order: %w", err)
    }
    return order, nil
}
```

The adapter's `Place` implementation:

1. Begins a transaction.
2. Sorts product IDs to obtain locks consistently.
3. Atomically reserves each requested quantity and returns its current price.
4. Calculates the total with overflow checks.
5. Inserts the order.
6. Inserts every item with its captured price.
7. Commits.

If any step fails, the deferred rollback keeps the durable invariant intact.

For a two-item order, walk through a concrete success and failure:

```text
Input: 2 x TEA-001 and 1 x CUP-004

1. Validate IDs and positive quantities before opening the transaction.
2. Sort IDs as CUP-004, TEA-001 so every request locks in the same order.
3. Reserve CUP-004 and capture its current unit price.
4. Reserve TEA-001 and capture its current unit price.
5. Calculate the total with overflow checks.
6. Insert the order and both captured-price items.
7. Commit once.
```

If step 4 reports insufficient tea, the transaction rolls back the cup reservation
from step 3. The use case returns a stable inventory error; it does not attempt a
second, compensating update because no part of the local transaction committed.

An alternative callback-based transaction abstraction can be valid, but it often
leaks transaction plumbing into the application and adds indirect control flow.
Use the smallest design that preserves the operation's atomicity and remains easy
to test.

## 8. Stable error identities cross boundaries

Define application errors for conditions callers may act on:

```go
var (
    ErrEmptyOrder            = errors.New("order must contain an item")
    ErrInvalidQuantity       = errors.New("quantity must be positive")
    ErrDuplicateProduct      = errors.New("product appears more than once")
    ErrProductNotFound       = errors.New("product not found")
    ErrInsufficientInventory = errors.New("insufficient inventory")
)
```

The PostgreSQL adapter translates known database outcomes into these identities and
wraps unexpected errors with operation context. The handler uses `errors.Is` to map
identities to HTTP problems:

| Application outcome | HTTP status | Public code |
| --- | --- | --- |
| Invalid items | 400 | `invalid_order` |
| Missing product | 400 or 404, according to the written contract | `product_not_found` |
| Insufficient inventory | 409 | `insufficient_inventory` |
| Unexpected failure | 500 | `internal_error` |

Choose one missing-product contract and test it. Never inspect error strings or let
the domain import HTTP constants.

Trace one failure through the layers:

```text
PostgreSQL conditional update returns no row
  -> adapter determines inventory is insufficient
  -> adapter returns ErrInsufficientInventory
  -> service wraps it as "place order: insufficient inventory"
  -> errors.Is still recognizes ErrInsufficientInventory
  -> handler writes 409 with code "insufficient_inventory"
```

The internal wrapped message helps an operator understand the failed operation. The
sentinel identity lets code make a stable decision. The public problem gives the
client only the information it needs. One error therefore has related but distinct
representations at each boundary.

## 9. Object ownership belongs in the query

Although identity middleware is added on Day 5, design the read boundary now:

```sql
SELECT id, customer_id, status, total_cents, created_at
FROM orders
WHERE id = $1
  AND customer_id = $2;
```

Filtering by both order and customer prevents the service from loading another
customer's order and relying on a later check that may be forgotten. Return the same
not-found outcome for a nonexistent order and an order owned by someone else unless
the public contract has a compelling, safe reason to reveal existence.

Suppose order `O-42` belongs to customer `C-2`:

```text
query arguments (O-42, C-2) -> one row
query arguments (O-42, C-9) -> no rows
query arguments (O-99, C-9) -> no rows
```

The final two cases are intentionally indistinguishable to `C-9`. The repository
does not first return `C-2`'s row and ask another layer to remember to conceal it.

## 10. Test at the boundary that owns the risk

### Unit tests

Use a fake `PlacementRepository`, deterministic clock, and deterministic ID source.
Test:

- empty item list;
- zero and negative quantities;
- duplicate product IDs;
- dependency is not called when command validation fails;
- valid normalized request reaches the repository;
- repository errors retain their identity after wrapping;
- result is returned unchanged on success.

Unit tests should not recreate SQL behavior in a complicated fake. The fake records
the request and returns a configured result.

### Integration tests

Use real PostgreSQL to prove:

- order, items, and inventory commit together;
- an item failure rolls back earlier inventory changes;
- two concurrent requests for the final unit produce one success;
- multi-product requests acquire locks consistently;
- a customer-scoped query cannot return another customer's order.

### Handler tests

Prove that decoding and error mapping are correct without retesting transaction
internals. Each test should fail for one clear reason.

The test layers form complementary evidence:

| Test boundary | Fast question it answers | What it deliberately does not prove |
| --- | --- | --- |
| Use-case unit test | Are input rules and dependency calls correct? | SQL, locks, or rollback |
| PostgreSQL integration test | Do constraints and the transaction preserve durable state? | HTTP decoding and status mapping |
| Handler test | Does the public request/response contract map correctly? | Real database concurrency |

Duplicating the full workflow in every layer makes tests slower without improving
diagnosis. Put each assertion at the narrowest boundary that owns the risk.

## 11. Guided refactor and build

Complete these changes sequentially:

1. Move product HTTP, application, and PostgreSQL code into the `product` feature.
2. Create the `order` feature without generic cross-feature repositories.
3. Define commands, results, errors, and the narrow placement interface.
4. Implement command validation with unit tests first.
5. Implement PostgreSQL placement using the Day 3 transaction and locking strategy.
6. Add `POST /v1/orders` as a thin translation layer.
7. Add customer-scoped order retrieval.
8. Run unit, handler, integration, race, and vet checks.

After the move, search old package paths, constructors, tests, mocks, and imports so
no duplicate or stale implementation remains.

## 12. Optional architecture decision record

Write `docs/adr/0001-order-inventory-locking.md` with:

- **Context:** concurrent buyers may request the same inventory;
- **Options:** application pre-check, row lock, conditional update, serializable
  transaction;
- **Decision:** the selected approach and transaction boundary;
- **Consequences:** lock duration, error mapping, retry behavior, query complexity;
- **Evidence:** last-unit integration test and expected query behavior.

An ADR records why a decision was made and when it should be reconsidered. It is not
a victory announcement or a copy of implementation details.

For example, the decision might state: use a conditional `UPDATE ... WHERE
inventory_quantity >= $1` because the operation is a decrement with one clear
invariant; reconsider if reservations gain expiration, partial fulfillment, or a
separate inventory owner. That explanation preserves both the current reason and
the conditions under which the decision may stop fitting.

## 13. Microservice counterfactual

If order and inventory were separate services, the current transaction could no
longer update both databases atomically. The design would need reservation identity,
timeouts, idempotency, intermediate states, compensation or reconciliation, message
delivery guarantees, and cross-service observability.

No present evidence justifies paying that cost. Record the boundary and its
consequences, but keep the modules in-process while a local transaction is the
simplest correct consistency mechanism.

The difficult case is an ambiguous partial failure:

```text
Ordering -> Inventory: reserve one TEA-001
Inventory commits the reservation
Inventory -> Ordering response is lost
Ordering cannot tell whether the reservation happened
```

Inside one PostgreSQL transaction, commit or rollback gives one answer. Across two
services, ordering needs a stable reservation ID and a way to retry or query that
exact operation without reserving twice. If its own order update later fails, it
also needs a durable release or reconciliation path. This is the concrete cost
hidden behind the phrase "split inventory into a service."

## 14. Knowledge check

1. Why is an HTTP request DTO not automatically a domain object?
2. Which dependency direction protects business logic from frameworks?
3. What belongs outside versus inside the placement transaction?
4. Why does the consumer own the placement interface?
5. Why is one business-sized repository method preferable to generic CRUD here?
6. Where are database errors translated into application errors?
7. Why must object ownership be included in the SQL predicate?
8. Which test proves that inventory and orders commit atomically?

### Expected answers

1. It represents untrusted protocol shape and may contain fields or rules that do
   not match valid domain state.
2. Handlers and adapters depend toward the use case/domain; the domain does not
   import HTTP, driver, or telemetry packages.
3. Input-only validation can happen before it; all current-state checks and durable
   changes must be protected inside it.
4. The consumer can express only the capability it needs.
5. It names and preserves the required atomic operation without leaking storage
   mechanics or unrelated methods.
6. At the PostgreSQL adapter boundary, before the use case or handler sees them.
7. It makes unauthorized data inaccessible rather than relying on a later check.
8. A real-database integration test that forces a mid-operation failure and inspects
   every affected table afterward.

## 15. Completion evidence

Day 4 is complete when:

- HTTP, business rules, and PostgreSQL concerns have clear dependency direction;
- `PlaceOrder` validates input and delegates one atomic persistence operation;
- public interfaces are narrow and justified by real consumers;
- deterministic unit tests cover all input-only rules;
- real-database tests prove rollback and last-unit contention;
- handlers map stable error identities without string matching;
- order retrieval scopes by both order ID and customer ID;
- the microservice counterfactual explains added consistency cost without adding
  distributed infrastructure;
- all configured checks pass.

Then continue to Day 5.
