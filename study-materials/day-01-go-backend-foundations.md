# Day 1: Go Backend Foundations

This lesson is the detailed companion to Day 1 of the
[backend learning roadmap](../BACKEND_LEARNING_ROADMAP.md#day-1-go-as-a-backend-language).
It is self-contained: you can complete the required lesson without searching for
another tutorial. External references are optional sources for deeper study.

## Learning outcomes

By the end of this lesson, you should be able to:

- create and navigate a Go module;
- explain packages, exported names, zero values, structs, and methods;
- choose deliberately between values and pointers;
- recognize when slices and maps share mutable storage;
- define a small interface at the point where it is consumed;
- create, wrap, inspect, and preserve errors;
- use `defer` to release an acquired resource;
- pass cancellation and deadlines through `context.Context`;
- protect shared state with a mutex;
- distinguish data-race freedom from business correctness;
- implement and test a concurrency-safe in-memory product store.

Budget about three focused hours. Type the examples instead of copying them when
possible. The small mistakes you make while typing are useful practice with the
compiler and test output.

### How to use this lesson

Do not try to memorize every Go feature before writing code. Use one small product
store to answer four recurring backend questions:

1. **Who owns this data?** A value, slice, or map may be copied or may still share
   storage with its caller.
2. **Who may change it?** Exported fields, pointers, and methods determine where
   mutation is possible.
3. **How does failure travel?** Errors retain identity as they gain context on the
   way back to the caller.
4. **What happens when two operations overlap?** Synchronization must protect the
   whole business rule, not just keep individual reads and writes race-free.

Read a section, predict what its example will do, and then run or test that
prediction. The lesson builds vocabulary first and then combines it in the guided
store exercise.

## 1. Modules, packages, files, and names

A **module** is the versioned unit of Go source code. Its root contains `go.mod`,
which declares the module path and the Go language version. A module contains one
or more **packages**. A package is a group of `.go` files in the same directory
that declare the same package name.

Initialize this repository from its root:

```bash
go mod init be-standard
```

For this local learning project, `be-standard` is a sufficient module path. If the
repository is later published, it can use its canonical repository path.

Create application packages below `internal/`. Go's `internal` rule prevents code
outside the parent module tree from importing them. That makes the intended API
boundary explicit without a framework.

```text
internal/
`-- product/
    |-- store.go
    `-- store_test.go
```

Every Go file begins with a package declaration:

```go
package product
```

Go controls visibility at the **package** boundary. An identifier whose first
letter is uppercase is exported; code in another package may refer to it. An
identifier whose first letter is lowercase is unexported; only code in the same
package may refer to it.

This rule applies to types, functions, methods, variables, constants, and struct
fields. "Unexported" does not mean "visible only in this file." All files declaring
`package product` are compiled together and can use the same unexported names.

Suppose `internal/product/product.go` contains:

```go
package product

import "strings"

type Product struct { // Exported type: another package can write product.Product.
    SKU  string       // Exported field: another package can read and set it.
    name string       // Unexported field: only package product can refer to it.
}

func New(sku, name string) Product { // Exported function.
    return Product{SKU: sku, name: name}
}

func (p Product) Name() string { // Exported method.
    return p.name
}

func normalizeSKU(sku string) string { // Unexported helper.
    return strings.ToUpper(strings.TrimSpace(sku))
}
```

Another file in the same directory, such as `internal/product/store.go`, also says
`package product`. It can use every name above, including the lowercase ones:

```go
package product

func displayName(p Product) string {
    return p.name + " (" + normalizeSKU(p.SKU) + ")" // Both are accessible here.
}
```

By contrast, `cmd/api/main.go` belongs to package `main` and imports the product
package:

```go
package main

import (
    "fmt"

    "be-standard/internal/product"
)

func main() {
    tea := product.New("tea-001", "Green tea")

    fmt.Println(tea.SKU)    // Allowed: SKU is exported.
    fmt.Println(tea.Name()) // Allowed: Name is exported.

    // fmt.Println(tea.name)          // Does not compile: name is unexported.
    // product.normalizeSKU(tea.SKU)  // Does not compile: normalizeSKU is unexported.
}
```

The importing package uses the imported package name as a qualifier:
`product.Product`, `product.New`, and `tea.Name`. It cannot reach through that
qualifier to lowercase identifiers.

Exporting a struct type does not automatically export its fields. Code outside the
package could construct `Product{SKU: "TEA-001"}`, but it could not set `name` in
that literal. Providing `product.New` lets the owning package initialize and later
validate the private field while exposing `Name()` for read access. This is one way
to keep an invariant behind a package API.

Package visibility and the `internal/` directory solve different problems:

- capitalization controls which names an importing package may use;
- the `internal/` rule controls which code is allowed to import the package at all.

Tests also make this distinction visible. A test declaring `package product` can
inspect unexported details; a test declaring `package product_test` behaves like an
outside consumer and sees only exported names. Prefer the latter when testing the
public package contract, and use the former only when a focused internal test needs
private access.

Export only what a caller needs. Every exported name becomes part of the package API
that callers can depend on and that future changes may need to preserve.

The remaining store exercise uses an exported `Name` field to keep its data model
simple. That is not a contradiction: export a field when callers are deliberately
allowed to read and construct it directly; keep it unexported when the package must
control access or preserve rules around it.

## 2. Zero values, structs, and methods

Every Go variable has a value immediately. If no value is assigned explicitly,
Go uses the type's **zero value**.

| Type | Zero value |
| --- | --- |
| `bool` | `false` |
| integers and floats | `0` |
| `string` | `""` |
| pointer, slice, map, function, interface, channel | `nil` |
| struct | each field's zero value |

Good Go types are useful in their zero state when that is practical. For example,
`sync.Mutex` is ready to use without a constructor. A map is different: reading a
nil map is safe, but writing to it panics, so a type that owns a map normally needs
a constructor.

```go
type Product struct {
    ID   string
    SKU  string
    Name string
}

type Store struct {
    products map[string]Product
}

func NewStore() *Store {
    return &Store{products: make(map[string]Product)}
}
```

A method is a function with a receiver:

```go
func (p Product) DisplayName() string {
    return p.SKU + " - " + p.Name
}
```

The receiver appears between `func` and the method name. A value receiver receives
a copy of the value. A pointer receiver receives an address through which the
method can modify the original value.

For example, suppose `p.Name` is initially `"Green tea"`:

```go
func (p Product) RenameCopy(name string) {
    p.Name = name
}

func (p *Product) RenameOriginal(name string) {
    p.Name = name
}

p.RenameCopy("Black tea")
// p.Name is still "Green tea": RenameCopy changed only its local copy.

p.RenameOriginal("Black tea")
// p.Name is now "Black tea": RenameOriginal followed a pointer to p.
```

This distinction is about meaning before performance. A value receiver says the
method can work from a copy. A pointer receiver says it operates on the particular
instance supplied by the caller. Go may take the address automatically for a
convenient method call, but it does not change those semantics.

## 3. Values, pointers, slices, and maps

### Choose values by default for small data

Use a value when:

- the data is small and copying it is clear;
- the function should not mutate the caller's value;
- absence is represented separately, often by an error or boolean;
- value semantics make the API easier to reason about.

```go
func Rename(p Product, name string) Product {
    p.Name = name
    return p
}
```

Use a pointer when:

- the function or method must mutate the original value;
- copying would be expensive or incorrect, such as copying a type containing a
  mutex;
- identity matters independently of field equality;
- `nil` has a deliberate, documented meaning.

```go
func (p *Product) Rename(name string) {
    p.Name = name
}
```

Do not use pointers automatically merely because the code is in a backend. Values
often make ownership and mutation much clearer.

One useful way to decide is to describe the contract in a sentence:

| Contract | Natural representation |
| --- | --- |
| "Produce a renamed product and leave the input alone" | Take and return values |
| "Rename this existing product" | Take a pointer |
| "Look up a product; absence is an error" | Return `(Product, error)` |
| "Look up an optional product; nil means absent" | Possibly return `*Product`, if that meaning is documented |

Avoid using `*Product` merely to avoid copying three strings. That choice makes
aliasing and `nil` part of the API, which is a larger semantic cost than this small
copy.

### A slice is a view over an array

A slice contains metadata pointing to an underlying array. Assigning or passing a
slice copies the metadata, not necessarily the elements. Two slices can therefore
observe mutations to the same storage.

```go
names := []string{"tea", "coffee"}
copyOfHeader := names
copyOfHeader[0] = "water"
// names[0] is now "water" too.
```

Make an independent copy at an ownership boundary:

```go
independent := append([]string(nil), names...)
```

Length and capacity explain a common surprise. Appending may reuse the same
underlying array when capacity remains, or allocate a new one when it does not:

```go
base := make([]string, 1, 2)
base[0] = "tea"
alias := base

alias = append(alias, "coffee") // Reuses the array because capacity is 2.
// base still has length 1, but both slice headers refer to the same array.

copyOfBase := append([]string(nil), base...)
copyOfBase[0] = "water"
// base[0] remains "tea" because the elements were copied.
```

At a package boundary, decide whether the caller is allowed to share your backing
storage. If not, copy deliberately instead of relying on current capacity.

### A map is shared mutable state

Assigning a map does not copy its entries. Maps are not safe for concurrent writes,
or for a read concurrent with a write. A store that owns a map must prevent callers
from accessing that map directly and synchronize all access.

Returning a new slice from `List` protects the store's collection from callers.
Because the exercise's `Product` contains only strings, copying each `Product`
value is enough. If it later contains slices, maps, or pointers, decide whether a
deeper copy is required.

## 4. Interfaces describe required behavior

An interface is a set of method signatures. A type implements an interface
implicitly by having those methods; there is no `implements` declaration.

```go
type ProductReader interface {
    Get(sku string) (Product, error)
}

func PrintProduct(reader ProductReader, sku string) error {
    product, err := reader.Get(sku)
    if err != nil {
        return err
    }

    fmt.Println(product.DisplayName())
    return nil
}
```

The code that needs to read a product should declare the smallest interface it
needs. This is called defining the interface on the consumer side. It avoids a
large provider-owned interface that forces every consumer and test double to
depend on unrelated methods.

Do not introduce an interface just because one implementation exists. Introduce
one where a caller needs substitution, isolation in a test, or a real boundary.

Follow the dependency direction in the example:

```text
PrintProduct needs Get
        |
        +-- declares ProductReader
                         ^
                         |
              *Store happens to satisfy it
```

`Store` does not need to know that `PrintProduct` exists. A test can provide a tiny
fake with only `Get`, and a future PostgreSQL adapter can satisfy the same consumer
without inheriting unrelated store methods. This is why the interface normally
lives beside the code that consumes it.

You can ask the compiler to verify an implementation:

```go
var _ ProductReader = (*Store)(nil)
```

This declaration allocates nothing. It fails compilation if `*Store` does not
implement `ProductReader`.

## 5. Errors are values with identity

Go functions commonly return a useful result plus an error:

```go
product, err := store.Get("TEA-001")
if err != nil {
    // Handle or return the error.
}
```

Use sentinel errors when callers need to distinguish a stable condition:

```go
var (
    ErrDuplicateSKU = errors.New("duplicate SKU")
    ErrNotFound     = errors.New("product not found")
)
```

Add context while preserving the original error with `%w`:

```go
return Product{}, fmt.Errorf("get product %q: %w", sku, ErrNotFound)
```

The wrapped message is useful to a human, while `errors.Is` follows the wrapping
chain and retains machine-checkable identity:

```go
if errors.Is(err, ErrNotFound) {
    // Map this condition deliberately, such as to HTTP 404 later.
}
```

Use `errors.As` when you need a particular error type and its fields:

```go
var validationErr *ValidationError
if errors.As(err, &validationErr) {
    fmt.Println(validationErr.Field)
}
```

Avoid comparing error text. Messages are for people and may change; identities and
types are contracts. Handle an error once: either recover from it meaningfully or
return it with useful context.

Think of wrapping as building a chain rather than replacing the error:

```text
ErrNotFound
  <- "get product TEA-001: product not found"
  <- "render product page: get product TEA-001: product not found"
```

Printing the outer error gives a useful story. `errors.Is(err, ErrNotFound)` walks
the chain and still finds the stable cause. By contrast, `%v` formats the inner
error but does not add it to the chain:

```go
wrapped := fmt.Errorf("load product: %w", ErrNotFound) // errors.Is succeeds.
formatted := fmt.Errorf("load product: %v", ErrNotFound) // Identity is lost.
```

At each layer, add context that layer uniquely knows. A store can name the failed
operation and SKU; an HTTP handler later decides that `ErrNotFound` maps to a public
404. The store should not make that HTTP decision itself.

## 6. Resource ownership and `defer`

Whoever successfully acquires a resource is normally responsible for releasing
it. Place the `defer` immediately after confirming acquisition succeeded:

```go
file, err := os.Open("products.json")
if err != nil {
    return err
}
defer file.Close()
```

Deferred calls run when the surrounding function returns, in last-in-first-out
order. Typical backend uses include closing response bodies and query rows,
rolling back a transaction unless it commits, unlocking a mutex, and stopping a
timer.

Do not put an unbounded number of defers inside a long-running loop: they remain
pending until the surrounding function returns. Extract one iteration into a
small function when each iteration owns a resource.

For resources where close errors matter, such as buffered writers, explicitly
check the close or flush error instead of discarding it.

## 7. Context carries cancellation across boundaries

`context.Context` communicates cancellation, deadlines, and request-scoped values
across API boundaries.

```go
func LoadProduct(ctx context.Context, store ProductReader, sku string) (Product, error) {
    select {
    case <-ctx.Done():
        return Product{}, ctx.Err()
    default:
        return store.Get(sku)
    }
}
```

Rules to follow:

- accept `context.Context` as the first parameter;
- do not store it in a struct;
- pass it to downstream calls that support cancellation;
- do not pass `nil`; use `context.Background()` when no parent exists;
- call the returned cancel function when creating a timeout or cancellation scope;
- use context values only for request-scoped metadata such as trace identity, not
  ordinary optional function parameters.

```go
ctx, cancel := context.WithTimeout(parent, 500*time.Millisecond)
defer cancel()
```

Cancellation is cooperative. Calling `cancel` closes `ctx.Done()`, but work stops
only if the receiving code observes the signal or calls an API that does.

A deadline normally travels down the call stack while the error travels back up:

```text
HTTP request context (deadline)
        -> use case
            -> PostgreSQL query

PostgreSQL observes cancellation
        <- returns context deadline error
            <- use case adds operation context
                <- handler maps the outcome
```

Creating a fresh `context.Background()` in the middle would break that chain. The
database query could continue after the client disconnected or the server began
shutdown. Pass the received context unless the work is intentionally detached and
has a separately owned lifetime.

The in-memory store does not perform blocking I/O, so its first version does not
need context parameters. PostgreSQL and HTTP operations added later will.

## 8. Concurrency: goroutines, synchronization, and correctness

A goroutine is a function executing concurrently with other goroutines:

```go
done := make(chan struct{})

go func() {
    defer close(done)
    // Perform bounded work.
}()

<-done
```

The channel gives the goroutine a termination signal that the caller can observe.
Every goroutine should have a clear owner, a stopping condition, and a way for its
completion or error to be observed. Starting a goroutine and forgetting it risks a
leak.

### Channels and mutexes solve different ownership problems

Use a channel when goroutines need to transfer values, signal completion, or hand
off ownership. Use a mutex when multiple goroutines need short, synchronized access
to shared in-memory state.

```go
type Store struct {
    mu       sync.RWMutex
    products map[string]Product
}
```

A read lock permits other readers. A write lock excludes readers and writers:

```go
s.mu.RLock()
defer s.mu.RUnlock()

s.mu.Lock()
defer s.mu.Unlock()
```

Keep critical sections small, but large enough to preserve the complete invariant.
For duplicate prevention, checking and inserting must occur under one write lock:

```go
s.mu.Lock()
defer s.mu.Unlock()

if _, exists := s.products[product.SKU]; exists {
    return ErrDuplicateSKU
}
s.products[product.SKU] = product
```

If the check and insertion used separate lock sections, two goroutines could both
observe that the SKU is absent before either inserts it.

Write the unsafe interleaving out explicitly:

```text
Goroutine A: check SKU -> absent
Goroutine B: check SKU -> absent
Goroutine A: insert SKU -> success
Goroutine B: insert SKU -> success or overwrite
```

The bug is not that the map operation itself takes too long. The check and insert
together form one decision. Holding the write lock across both steps makes every
other goroutine observe either the state before that decision or the state after it,
never the half-finished point between them.

### Atomics and happens-before

An atomic operation safely reads or writes one supported value without a data race.
Atomics are useful for simple counters and flags, but they do not automatically
protect a rule involving several values or steps. Prefer a mutex when expressing
the invariant with atomics would be difficult to explain.

The Go memory model defines **happens-before** relationships: synchronization such
as unlocking a mutex before another goroutine locks it makes preceding writes
visible to the later goroutine. Without synchronization, assumptions based on
source-code order or wall-clock timing are invalid.

### Data-race freedom is not business correctness

A data race is unsynchronized concurrent access to the same memory where at least
one access writes. The race detector can find many such execution paths.

A program can be race-free and still be wrong. Consider inventory stored behind
properly locked methods:

```go
if store.Quantity(sku) > 0 {
    store.Decrement(sku)
}
```

Each call may be individually race-free, but two callers can both observe quantity
`1`, then both decrement. The business operation requires one atomic check-and-
decrement operation. Synchronize the whole invariant, not merely individual field
accesses. Day 3 applies the same principle with database transactions and locks.

Compare the two guarantees:

| Guarantee | Question answered |
| --- | --- |
| Data-race freedom | Are conflicting memory accesses synchronized? |
| Business atomicity | Can another operation observe or change state between the steps of this rule? |

The race detector helps with the first question. A contention test with a precise
expected outcome helps with the second. Production code normally needs both forms
of evidence.

## 9. Guided exercise: in-memory product store

Create `internal/product/store_test.go` first. Tests define the behavior before the
implementation exists.

### Required public behavior

Use this product model:

```go
type Product struct {
    ID   string
    SKU  string
    Name string
}
```

Implement this store API:

```go
func NewStore() *Store
func (s *Store) Create(product Product) error
func (s *Store) Get(sku string) (Product, error)
func (s *Store) List() []Product
```

Required invariants:

1. Each SKU identifies at most one product.
2. `Create` returns an error matching `ErrDuplicateSKU` for an existing SKU.
3. `Get` returns an error matching `ErrNotFound` for an unknown SKU.
4. Concurrent calls must not race or corrupt the map.
5. `List` returns a newly allocated snapshot so callers cannot change the store's
   collection.
6. `List` uses a deterministic order so tests and later API behavior are stable.

For Day 1, order the list lexicographically by SKU using `sort.Slice` or
`slices.SortFunc`. Stable order matters because map iteration order is deliberately
unspecified.

### Step 1: write the basic tests

Write tests for:

- creating and retrieving one product;
- retrieving an unknown SKU;
- creating the same SKU twice;
- listing products in SKU order;
- changing the returned list without changing the next returned list.

A useful test shape is:

```go
func TestStoreCreateAndGet(t *testing.T) {
    store := NewStore()
    want := Product{ID: "product-1", SKU: "TEA-001", Name: "Green tea"}

    if err := store.Create(want); err != nil {
        t.Fatalf("Create() error = %v", err)
    }

    got, err := store.Get(want.SKU)
    if err != nil {
        t.Fatalf("Get() error = %v", err)
    }
    if got != want {
        t.Fatalf("Get() = %#v, want %#v", got, want)
    }
}
```

Run the test now. Compilation failure is expected because the implementation does
not exist yet:

```bash
go test ./internal/product
```

This is the red step of red-green-refactor: observe the test fail for the expected
reason, add the smallest correct implementation, and then improve structure while
the tests remain green.

### Step 2: implement the sequential behavior

Create `store.go` with the model, sentinel errors, `Store`, and its methods. Use a
map keyed by SKU. Add deterministic sorting in `List`.

Run:

```bash
go test ./internal/product
```

Do not continue until the basic tests pass.

### Step 3: add concurrent tests

Start a fixed number of goroutines. Each must signal completion, and the test must
wait for all of them. `sync.WaitGroup` is appropriate:

```go
func TestStoreConcurrentCreateAndGet(t *testing.T) {
    store := NewStore()
    const workers = 100

    var wg sync.WaitGroup
    wg.Add(workers)

    for i := 0; i < workers; i++ {
        i := i
        go func() {
            defer wg.Done()

            sku := fmt.Sprintf("SKU-%03d", i)
            product := Product{ID: sku, SKU: sku, Name: "Product " + sku}
            if err := store.Create(product); err != nil {
                t.Errorf("Create(%q) error = %v", sku, err)
                return
            }
            if _, err := store.Get(sku); err != nil {
                t.Errorf("Get(%q) error = %v", sku, err)
            }
        }()
    }

    wg.Wait()

    if got := len(store.List()); got != workers {
        t.Fatalf("List() length = %d, want %d", got, workers)
    }
}
```

The number of goroutines is explicitly bounded. Every goroutine executes
`wg.Done()` with `defer`, including early-return paths, so the test has a clear
termination condition.

Now run the race detector:

```bash
go test -race ./internal/product
```

If you initially omit the mutex, expect a failure similar to:

```text
WARNING: DATA RACE
Read at ...
Previous write at ...
```

Do not silence the test. Add one `sync.RWMutex` owned by `Store`, use its write lock
for the entire duplicate-check-and-insert operation, and use its read lock while
getting or copying products.

### Step 4: test a business-level concurrency rule

Have many goroutines attempt to create the same SKU. Exactly one call must succeed;
every other result must match `ErrDuplicateSKU`. Protect only the test's result
counters with a separate mutex or use a results channel.

This test proves more than the absence of a data race: it proves that the uniqueness
invariant remains true under contention.

### Step 5: complete the verification loop

Run the repository's Day 1 checks:

```bash
gofmt -w .
go test ./...
go test -race ./...
go vet ./...
```

Interpret them as different evidence:

- `gofmt` makes source formatting deterministic;
- `go test` checks the specified examples and invariants;
- `go test -race` instruments memory access to detect exercised data races;
- `go vet` detects suspicious constructs that compile but are commonly erroneous.

Passing these commands does not prove the program is bug-free. It does mean the
current implementation passed four useful, repeatable checks.

## 10. Optional stretch: fuzz a validator

After the required work, write a small SKU parser or validator and fuzz it. A fuzz
test supplies generated inputs to find panics and invariant violations.

For example, define a rule that a SKU is 3-32 ASCII uppercase letters, digits, or
hyphens, starts with a letter, and ends with a letter or digit. Seed the fuzz test
with valid and invalid examples, then assert only properties that must always hold:
the validator never panics, accepted input respects every rule, and normalization
does not change already valid input.

Run:

```bash
go test -fuzz=FuzzValidateSKU -fuzztime=20s ./internal/product
```

Keep fuzzing optional on the first pass. The product store and required verification
commands are the priority.

## 11. Knowledge check

Answer these without looking back:

1. What is the difference between a module and a package?
2. Why can a nil map be read but not written?
3. When is a pointer receiver required?
4. Why can modifying one slice affect another slice?
5. How does a type declare that it implements an interface?
6. Why should the consumer usually own an interface?
7. What information does `%w` preserve?
8. When should `errors.Is` be used instead of comparing error messages?
9. Where should `defer` appear after resource acquisition?
10. Why does calling a context cancel function not forcibly stop a goroutine?
11. What must every goroutine have to avoid becoming an unowned leak?
12. Why can code pass the race detector and still violate an inventory rule?

### Expected answers

1. A module is the versioned source unit declared by `go.mod`; a package is a
   directory-level compilation and API unit within a module.
2. A nil map has no allocated table. Lookup is defined to return the zero value,
   but assignment has nowhere to store an entry and panics.
3. Use one when the method must mutate the original, copying is unsafe or costly,
   identity matters, or nil is intentional.
4. A slice copies a descriptor that can still point to the same underlying array.
5. It does not declare implementation; its method set satisfies the interface
   implicitly.
6. The consumer can describe only the behavior it needs, keeping coupling small.
7. It adds context while preserving the wrapped error in the error chain.
8. Use `errors.Is` when behavior depends on a stable error identity.
9. Immediately after checking that acquisition succeeded.
10. Cancellation is cooperative; the goroutine or downstream API must observe it.
11. An owner, a bounded or cancellable stopping condition, and observable completion
    or error handling.
12. The detector finds memory races on executed paths, not multi-step business
    invariants such as atomic check-and-decrement.

## 12. Completion evidence

Day 1 is complete when all of the following are true:

- the four verification commands pass;
- the duplicate-SKU contention test proves exactly one success;
- every started goroutine has a bounded count and a completion path;
- you can explain value versus pointer semantics without relying on "performance" as
  the only reason;
- you can explain implicit interface satisfaction;
- you can demonstrate that `errors.Is` recognizes a sentinel through a `%w` wrapper;
- you record one mistake, why it occurred, and the test or design rule that now
  prevents it.

Then continue to Day 2. Do not add HTTP handlers during this lesson; keeping the
exercise in memory makes the language, ownership, errors, and concurrency behavior
easier to see.
