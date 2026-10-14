# Go Language Refresher

This lesson rebuilds practical fluency with Go itself before the backend lessons
add HTTP, databases, transactions, and concurrency. It is for someone who has seen
Go before but no longer remembers the syntax or the reasoning behind it.

The goal is not to memorize the language specification. The goal is to recover a
working mental model: what a value contains, what a function may change, how data
is shared, and how failure moves through a program.

## Learning outcomes

By the end of this lesson, you should be able to:

- distinguish modules, packages, and files, and use the standard toolchain;
- declare values and choose useful types;
- write branches, loops, functions, and closures;
- explain how arrays, slices, maps, strings, bytes, and runes differ;
- model data with structs, methods, embedding, and interfaces;
- use pointers without treating every value as a pointer;
- understand zero values, `nil`, `make`, and `new`;
- return errors and use `defer` for cleanup;
- recognize the syntax and ownership rules of basic concurrency;
- avoid common traps involving shadowing, aliasing, typed `nil`, and `range`;
- write and run a focused Go test.

This lesson uses a small product catalog throughout. Type the examples into a
scratch package if you want maximum recall. Reading creates familiarity; typing,
predicting, and running create fluency.

## 1. Modules, packages, and the shape of a Go program

Every Go source file declares a package near the top. A **package** is Go's basic
compilation and API unit: it is normally the `.go` files in one directory that
declare the same package name. An executable program uses `package main` and has a
`main` function:

```go
package main

import "fmt"

func main() {
	fmt.Println("catalog started")
}
```

The pieces have distinct jobs:

- `package main` says this package builds an executable.
- `import "fmt"` makes another package available in this file.
- `func main()` is where the executable starts.
- `fmt.Println` uses an exported function from `fmt`.

Names beginning with an uppercase letter are exported from their package. A name
beginning with a lowercase letter is package-private. Exported does not mean
globally available: another package must still import the defining package.

Go organizes code by package directory, not by class. All ordinary `.go` files in
one directory must use the same package name. Tests may use that package or an
external test package ending in `_test`.

A **module** is a collection of packages released, versioned, and distributed
together. The Go toolchain also uses modules to manage dependencies. A module's
root contains a `go.mod` file. This repository's `go.mod` begins with:

```text
module be-standard

go 1.26.3
```

The module path is therefore `be-standard`. The files in `internal/product/` form
the `product` package, whose import path is `be-standard/internal/product`.
Packages inside a module do not each need their own `go.mod` file.

So the distinction is:

- a module is a collection and dependency/versioning boundary, identified by
  `go.mod`;
- a package is a directory-level compilation and visibility boundary that code
  imports and uses.

One module commonly contains many packages. A repository can also contain more
than one module, although this repository currently contains one.

Useful commands from this repository's module root, the directory containing
`go.mod`, are:

```text
go run ./path/to/main-package
go test ./...
go test -race ./...
go vet ./...
gofmt -w path/to/file.go
go doc fmt.Errorf
```

`gofmt` is part of normal Go development, not an optional style preference. Let it
format the code instead of manually aligning or debating whitespace.

### Checkpoint

Before continuing, answer these without looking back:

1. What makes a Go name visible to another package?
2. What is the difference between a module and a package?
3. Which function starts an executable?

## 2. Variables, constants, and basic types

Go is statically typed: each value has a type known at compile time. Type inference
can save spelling, but it does not make the variable dynamically typed.

```go
var name string                 // Explicit type; zero value is "".
var stock int = 12              // Explicit type and initial value.
sku := "SKU-001"               // Inferred as string; functions only.
priceCents := int64(2_499)      // Explicit conversion to int64.
const defaultCurrency = "USD"  // Compile-time constant.
```

Use `:=` for a new local variable when the inferred type is clear. Use `var` when
the zero value is meaningful, when declaring outside a function, or when the type
would otherwise be unclear. Use `const` for values known at compile time.

Common built-in types include:

| Kind | Examples | Practical note |
|---|---|---|
| Boolean | `bool` | Only `true` and `false`; integers are not truthy. |
| Signed integer | `int`, `int64` | Use `int` for indexes/counts; choose a fixed width at storage or wire boundaries. |
| Unsigned integer | `uint`, `byte` | `byte` is an alias for `uint8`; unsigned values are rarely needed for ordinary counts. |
| Floating point | `float64` | Suitable for measurements, not exact money. |
| Text | `string` | Immutable bytes, conventionally UTF-8. |
| Unicode code point | `rune` | Alias for `int32`; represents one Unicode code point. |

Go does not silently convert numeric variables:

```go
quantity := 3
unitPrice := int64(499)
total := int64(quantity) * unitPrice
```

That explicit conversion forces you to notice representation boundaries. For money,
an integer number of the smallest currency unit is usually easier to reason about
than binary floating point.

### Constants and `iota`

Constants may be untyped until context gives them a concrete type. `iota` can define
a related sequence:

```go
type Status int

const (
	StatusDraft Status = iota
	StatusActive
	StatusArchived
)
```

Do not assume a zero enum value is invalid. Here, the zero value is deliberately
`StatusDraft`. If zero must mean unknown, name it explicitly as `StatusUnknown`.

### Short exercise

Declare a product SKU, name, stock count, and price in cents. Predict each inferred
type, then confirm with `fmt.Printf("%T\n", value)`.

## 3. Control flow: `if`, `switch`, `for`, and `range`

Go keeps control flow small. There is one loop keyword, `for`, and conditions do not
use parentheses.

### `if` and a scoped initializer

```go
if stock == 0 {
	fmt.Println("out of stock")
} else if stock < 5 {
	fmt.Println("low stock")
} else {
	fmt.Println("available")
}
```

An `if` may initialize a value whose scope is limited to the branches:

```go
if err := validateSKU(sku); err != nil {
	return err
}
```

This is why Go error handling often reads as a sequence of guarded steps.

### `switch`

Go cases do not fall through by default:

```go
switch status {
case StatusDraft:
	fmt.Println("not visible")
case StatusActive:
	fmt.Println("for sale")
case StatusArchived:
	fmt.Println("read-only history")
default:
	fmt.Println("unknown status")
}
```

A switch without an expression is a clean alternative to a long `if` chain:

```go
switch {
case stock == 0:
	return "none"
case stock < 5:
	return "low"
default:
	return "healthy"
}
```

### The four useful forms of `for`

```go
for i := 0; i < 3; i++ { // Traditional counted loop.
	fmt.Println(i)
}

for stock > 0 { // A while-style loop.
	stock--
}

for { // An intentional infinite loop; it must exit or block somewhere.
	break
}

for index, product := range products { // Iterate over a collection.
	fmt.Println(index, product.Name)
}
```

Use `_`, the blank identifier, when a returned or ranged value is intentionally not
needed:

```go
for _, product := range products {
	fmt.Println(product.Name)
}
```

Since Go 1.22, variables declared by a `range` clause with `:=` are distinct for
each iteration. Old advice to write `product := product` before launching a closure
is unnecessary in this module's Go version. A variable declared before the loop and
assigned with `=` is still reused, so read the declaration form carefully.

### Checkpoint

Write a loop that totals the stock of all products and skips archived products with
`continue`. What should an empty catalog total?

## 4. Functions, multiple results, and closures

A function's signature is its contract:

```go
func subtotal(priceCents int64, quantity int) (int64, error) {
	if quantity < 0 {
		return 0, fmt.Errorf("quantity must not be negative: %d", quantity)
	}

	return priceCents * int64(quantity), nil
}
```

Multiple return values make `(result, error)` ordinary. Callers must handle both:

```go
total, err := subtotal(499, 3)
if err != nil {
	return err
}
fmt.Println(total)
```

Prefer short functions with explicit parameters and results. Named return values
can document several same-typed results and support deferred error handling, but
they become hard to follow when used as mutable local variables throughout a long
function.

### Variadic functions

A variadic parameter accepts zero or more values:

```go
func hasAnyTag(product Product, wanted ...string) bool {
	for _, tag := range product.Tags {
		for _, candidate := range wanted {
			if tag == candidate {
				return true
			}
		}
	}
	return false
}

featured := hasAnyTag(product, "featured", "seasonal")
```

Inside the function, `wanted` is a slice. Pass an existing slice with `tags...`.

### Functions are values

Functions can be passed and returned:

```go
func filter(products []Product, keep func(Product) bool) []Product {
	result := make([]Product, 0, len(products))
	for _, product := range products {
		if keep(product) {
			result = append(result, product)
		}
	}
	return result
}

available := filter(products, func(product Product) bool {
	return product.Stock > 0
})
```

The anonymous function is a closure: it can refer to variables in the surrounding
scope. That is useful, but it also means the closure may share changing state. Keep
captured state obvious, especially when a closure runs later or in another goroutine.

## 5. Arrays, slices, and ownership

An array contains a fixed number of elements, and its length is part of its type:

```go
var weeklySales [7]int
```

Assigning or passing an array copies all its elements. Arrays are useful building
blocks, but most application APIs use slices.

A slice is a small descriptor referring to part of an underlying array. It contains
a pointer to storage, a length, and a capacity. Copying a slice copies the descriptor,
not necessarily the elements:

```go
names := []string{"keyboard", "mouse"}
alias := names
alias[0] = "mechanical keyboard"
fmt.Println(names[0]) // "mechanical keyboard"
```

Both slices refer to the same element storage. This is the central slice ownership
question: **is shared mutation intended?**

### Length, capacity, and `append`

```go
products := make([]Product, 0, 4)
products = append(products, Product{SKU: "SKU-001", Name: "Keyboard"})
```

- Length is the number of accessible elements: `len(products)`.
- Capacity is how many elements fit before a larger backing array is needed:
  `cap(products)`.
- `append` returns the resulting slice and may reuse the old backing array or
  allocate a new one. Always use its returned value.

Because allocation depends on capacity, code should not rely on `append` breaking
or preserving an alias. Copy deliberately at an ownership boundary:

```go
owned := append([]Product(nil), products...)
```

For nested reference-like fields, a shallow slice copy may not be enough:

```go
type Product struct {
	SKU   string
	Name  string
	Stock int
	Tags  []string
}

func cloneProduct(product Product) Product {
	product.Tags = append([]string(nil), product.Tags...)
	return product
}
```

### Slicing retains storage

```go
firstTen := allProducts[:10]
```

`firstTen` still refers to `allProducts`' backing array. Keeping a tiny subslice can
therefore keep a large allocation alive. Copy the needed elements when long-lived
data should not retain the original buffer.

### Short exercise

Predict the output before running this:

```go
original := []int{1, 2, 3}
copyOfHeader := original
independent := append([]int(nil), original...)

copyOfHeader[0] = 10
independent[1] = 20

fmt.Println(original)
fmt.Println(independent)
```

Explain the result using “slice descriptor” and “backing array.”

## 6. Maps

A map associates unique keys with values:

```go
stockBySKU := map[string]int{
	"SKU-001": 12,
	"SKU-002": 4,
}

stockBySKU["SKU-003"] = 8
```

A map's zero value is `nil`. Reading from a nil map is safe and returns the value
type's zero value. Writing to a nil map panics, so initialize a map before writing:

```go
stockBySKU = make(map[string]int)
```

Use the comma-ok form when absence differs from a stored zero:

```go
stock, found := stockBySKU[sku]
if !found {
	return fmt.Errorf("product %q not found", sku)
}
```

Without `found`, a missing SKU and a SKU with zero stock both appear as `0`.

Delete is safe even if the key is absent:

```go
delete(stockBySKU, sku)
```

Map iteration order is deliberately unspecified. Sort keys before producing stable
output, snapshots, signatures, or tests. Maps are reference-like and are not safe
for unsynchronized concurrent read/write access.

## 7. Strings, bytes, and runes

A string is an immutable sequence of bytes. Go source is UTF-8, and application
strings are commonly UTF-8, but a string can technically contain arbitrary bytes.

```go
name := "cafe\u0301"
fmt.Println(len(name)) // Byte count, not necessarily visible-character count.
```

There are three different units to keep straight:

- `len(text)` counts bytes.
- `for index, r := range text` decodes Unicode code points; `index` is a byte offset.
- What a person sees as one character may contain multiple code points, such as a
  letter followed by a combining mark.

Use `[]byte` for raw bytes or mutable byte-oriented processing. Use `[]rune` when
you intentionally need code-point indexing:

```go
for byteIndex, r := range "Go语言" {
	fmt.Printf("byte=%d rune=%q\n", byteIndex, r)
}
```

Converting between string and byte/rune slices normally allocates and copies:

```go
raw := []byte("SKU-001")
text := string(raw)
runes := []rune("Go语言")
```

Do not truncate user-facing text by arbitrary byte offsets; doing so can split a
UTF-8 encoding. Decide whether the actual requirement concerns bytes, code points,
or human-perceived characters.

## 8. Structs, methods, pointers, and embedding

A struct groups named fields into one value:

```go
type Product struct {
	SKU        string
	Name       string
	PriceCents int64
	Stock      int
	Tags       []string
}

product := Product{
	SKU:        "SKU-001",
	Name:       "Keyboard",
	PriceCents: 2_499,
	Stock:      12,
}
```

Prefer keyed literals outside tiny private types. They survive field reordering and
show what each value means.

A method is a function with a receiver:

```go
func (p Product) Available() bool {
	return p.Stock > 0
}

func (p *Product) Reserve(quantity int) error {
	if quantity <= 0 {
		return fmt.Errorf("quantity must be positive")
	}
	if p.Stock < quantity {
		return fmt.Errorf("insufficient stock")
	}

	p.Stock -= quantity
	return nil
}
```

`Available` uses a value receiver because it only observes the product. `Reserve`
uses a pointer receiver because it changes the original product.

A pointer stores the address of a value:

```go
productPointer := &product
fmt.Println(productPointer.Name) // Go automatically dereferences for field access.
```

Use a pointer when a function or method must mutate one shared value, when copying
a value would be semantically wrong, or when `nil` is a meaningful state. Do not use
pointers everywhere merely because they appear cheaper. Pointer sharing introduces
mutation and lifetime questions, and small values are intentionally easy to copy.

If a type has pointer-receiver methods, only `*Product` has those methods in its
method set for interface satisfaction. Go may automatically take an address for an
addressable local method call, but interface assignment follows the actual method
set.

### Embedding is promotion, not inheritance

```go
type AuditFields struct {
	CreatedBy string
}

type Product struct {
	AuditFields
	SKU  string
	Name string
}

fmt.Println(product.CreatedBy) // Promoted from product.AuditFields.CreatedBy.
```

Embedding can promote fields and methods for convenient composition. It does not
create an “is-a” inheritance hierarchy, and the embedded value still exists as a
real field.

## 9. Interfaces, assertions, type switches, and generics

An interface describes behavior as a method set:

```go
type ProductFinder interface {
	FindBySKU(sku string) (Product, error)
}
```

Types satisfy interfaces implicitly by having the required methods. There is no
`implements` declaration. This keeps providers independent from their consumers and
lets a consumer define the smallest behavior it actually needs.

Accept interfaces where multiple implementations or a test double are useful.
Return concrete types unless callers genuinely need an abstraction. Avoid creating
an interface for every struct “just in case.”

### Type assertions

When an interface value may contain different concrete values, inspect it safely:

```go
value, ok := anyValue.(Product)
if !ok {
	return fmt.Errorf("expected Product, got %T", anyValue)
}
```

The one-result form, `value := anyValue.(Product)`, panics on a mismatch. Use it only
when a mismatch truly represents an internal invariant violation.

A type switch handles several possible dynamic types:

```go
switch value := anyValue.(type) {
case Product:
	fmt.Println(value.SKU)
case *Product:
	fmt.Println(value.SKU)
case nil:
	fmt.Println("no value")
default:
	fmt.Printf("unsupported type %T\n", value)
}
```

### Generics

Generics express an algorithm that is identical across a family of types:

```go
func Contains[T comparable](values []T, wanted T) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
```

`T` is a type parameter. `comparable` is its constraint because `==` is required.
Use generics when they remove real type duplication without hiding domain meaning.
A product repository does not become clearer merely because its domain types can be
replaced with type parameters.

## 10. Zero values, `nil`, `make`, and `new`

Every Go variable starts with a zero value:

| Type | Zero value |
|---|---|
| Number | `0` |
| Boolean | `false` |
| String | `""` |
| Pointer, slice, map, channel, function, interface | `nil` |
| Struct | All fields at their zero values |
| Array | All elements at their zero values |

Designing a useful zero value makes types easier to use. `sync.Mutex` and
`bytes.Buffer`, for example, are ready without a constructor.

`make` initializes the runtime representation of slices, maps, and channels:

```go
products := make([]Product, 0, 8)
bySKU := make(map[string]Product)
updates := make(chan Product, 4)
```

`new(T)` allocates a zero `T` and returns `*T`:

```go
count := new(int) // *count is initially 0.
```

`new` is much less common than `&T{}` because a struct literal can initialize fields
at the same time.

### The typed-`nil` interface trap

An interface value contains a dynamic type and a dynamic value. It is `nil` only
when both are absent:

```go
type ProductError struct {
	SKU string
}

func (e *ProductError) Error() string { return "product error: " + e.SKU }

var productErr *ProductError = nil
var err error = productErr

fmt.Println(productErr == nil) // true
fmt.Println(err == nil)        // false: dynamic type is *ProductError.
```

Avoid returning a typed nil as an interface. Return a literal `nil` on success:

```go
func validateProduct(product Product) error {
	if product.SKU == "" {
		return &ProductError{SKU: product.SKU}
	}
	return nil
}
```

## 11. Errors, `defer`, `panic`, and `recover`

An `error` is a value describing an operation that failed. Ordinary expected
failures should be returned, enriched with context, and inspected when a caller can
respond differently.

```go
var ErrProductNotFound = errors.New("product not found")

func loadProduct(sku string) (Product, error) {
	product, found := productsBySKU[sku]
	if !found {
		return Product{}, fmt.Errorf("load product %q: %w", sku, ErrProductNotFound)
	}
	return cloneProduct(product), nil
}
```

`%w` preserves the wrapped error. Callers use `errors.Is` for a particular error
identity and `errors.As` for a particular error type or interface:

```go
if errors.Is(err, ErrProductNotFound) {
	// Handle the known not-found condition.
}

var productErr *ProductError
if errors.As(err, &productErr) {
	// Use fields or methods from whichever wrapped error matched *ProductError.
}
```

The deeper backend treatment, including custom matching and Go 1.26's
`errors.AsType`, is in
[Day 1: Go backend foundations](../day-01-go-backend-foundations.md#5-errors-are-values-with-identity).

### `defer` schedules cleanup at the owner boundary

A deferred call runs when the surrounding function returns, including an early
return. This lets the function that successfully acquired a resource immediately
record how that resource will be released:

```go
func readConfig(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config: %w", err)
	}
	defer file.Close()

	return io.ReadAll(file)
}
```

For a read-only file, a close failure is rarely actionable. When closing or flushing
a writer can reveal lost output, capture and return that error instead of discarding
it.

The point is not that cleanup is postponed for its own sake. The point is that
every later return path now shares one reliable cleanup rule. Deferred calls run in
last-in-first-out order, which naturally releases nested resources in reverse order.

Arguments to a deferred call are evaluated when `defer` executes:

```go
name := "first"
defer fmt.Println(name)
name = "second"
// Prints "first" when the function returns.
```

Do not put `defer` in a large loop when every iteration must release a scarce
resource promptly. Move one iteration into a helper function so its defer runs at
the end of that iteration.

### `panic` and `recover`

Use returned errors for operational failures such as invalid input, unavailable
dependencies, and missing records. A panic indicates that ordinary control flow
cannot meaningfully continue, often because of a programmer error or violated
internal invariant.

`recover` can stop a panic only when called directly inside a deferred function in
the same goroutine. It is most useful at a deliberate process boundary, such as server
middleware that logs the panic and fails one request. It should not turn arbitrary
bugs into successful responses or silently continue with corrupted state.

## 12. Concurrency syntax and the ownership behind it

Concurrency lets independent work make progress during overlapping time. It does
not automatically make work faster, and shared mutable data still needs a clear
owner or synchronization rule.

Start a goroutine with `go`:

```go
go refreshCatalog()
```

The caller does not wait automatically. Production code therefore needs a way to
observe completion, cancellation, and failure.

A channel passes values between goroutines:

```go
updates := make(chan Product)

go func() {
	updates <- Product{SKU: "SKU-001", Name: "Keyboard"}
}()

product := <-updates
```

An unbuffered send waits for a receiver, and an unbuffered receive waits for a
sender. A buffered channel allows up to its capacity to wait in the channel; it is
not an unlimited queue.

Only the goroutine responsible for producing all values should normally close a
channel. Closing means “no more values will ever be sent”; it is not a way for a
receiver to request cancellation. Receiving from a closed channel returns remaining
buffered values and then the element type's zero value with `ok == false`:

```go
product, ok := <-updates
if !ok {
	return
}
```

`select` waits on multiple channel operations:

```go
select {
case product := <-updates:
	fmt.Println(product.SKU)
case <-ctx.Done():
	return ctx.Err()
}
```

Use a mutex when several operations must protect shared in-memory state:

```go
type Catalog struct {
	mu    sync.RWMutex
	bySKU map[string]Product
}

func (c *Catalog) Find(sku string) (Product, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	product, found := c.bySKU[sku]
	return cloneProduct(product), found
}
```

The mutex protects an invariant, not merely a map operation. If “check stock and
reserve it” must be atomic, both steps belong under one lock. Always decide how a
goroutine ends before starting it; leaked goroutines are leaked resources.

[Day 1](../day-01-go-backend-foundations.md#8-concurrency-goroutines-synchronization-and-correctness)
develops these rules through a concurrency-safe store exercise.

## 13. Testing and everyday tools

Tests live in files ending with `_test.go`. A table-driven test expresses several
inputs against one behavior:

```go
func TestSubtotal(t *testing.T) {
	tests := []struct {
		name       string
		priceCents int64
		quantity   int
		want       int64
		wantErr    bool
	}{
		{name: "one item", priceCents: 499, quantity: 1, want: 499},
		{name: "several items", priceCents: 499, quantity: 3, want: 1_497},
		{name: "negative quantity", priceCents: 499, quantity: -1, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := subtotal(test.priceCents, test.quantity)
			if (err != nil) != test.wantErr {
				t.Fatalf("subtotal() error = %v, wantErr %v", err, test.wantErr)
			}
			if got != test.want {
				t.Errorf("subtotal() = %d, want %d", got, test.want)
			}
		})
	}
}
```

Good tests report the behavior, input, actual result, and expected result. Test
observable contracts rather than private implementation steps.

Use the toolchain in progressively broader scopes:

```text
gofmt -w ./path/to/changed.go
go test ./path/to/changed/package
go test ./...
go vet ./...
go test -race ./...
```

The race detector finds unsynchronized memory access during the execution paths the
tests actually run. Passing it does not prove a business operation is atomic, and
it cannot inspect an unexecuted path.

## 14. Practical traps and habits worth remembering

### Watch `:=` for accidental shadowing

`:=` declares at least one new variable in the current scope and may reuse other
names in that same scope. Inside a nested block, it can create a new variable that
hides the outer one:

```go
var product Product

if useCache {
	product, err := cache.Find(sku) // This product exists only in the if block.
	if err != nil {
		return err
	}
	fmt.Println(product.Name)
}

fmt.Println(product.Name) // Still the outer zero-value Product.
```

When you intend to assign an existing variable, declare the other result first or
use `=` deliberately.

### Treat copies according to what they contain

Copying a struct copies its fields. String and number fields become independent
values; slice, map, pointer, function, channel, and interface fields may still refer
to shared state. “Passed by value” does not imply “deeply independent.”

### Prefer useful zero values

Avoid constructors that only fill fields with their ordinary zero values. Add a
constructor when it establishes an invariant, initializes a map/channel, chooses
required dependencies, or hides a representation that callers should not build
directly.

### Keep setup explicit

Package-level state and `init` functions can hide ordering and make tests interfere
with each other. Prefer constructing dependencies in `main` and passing them to the
code that needs them.

### Keep APIs unsurprising

- Accept `context.Context` as the first parameter when an operation may wait or do
  I/O; do not store it in a struct.
- Return errors instead of logging and returning the same error at every layer.
- Define interfaces near the consumer.
- Copy mutable data at boundaries when callers must not mutate internal state.
- Sort map-derived output when stable order matters.
- Document who owns closing a channel or resource.
- Keep the happy path visually clear with early returns for failure.

### Let the compiler help

Unused imports and local variables are compile errors. Exhaustive-looking switches
still need deliberate defaults when new values may appear. Run `gofmt`, tests,
`go vet`, and the race detector rather than relying on visual review alone.

## 15. Guided recall exercise: build a small catalog summary

Create a temporary `main` package and implement this behavior without copying code
from earlier sections:

1. Define a `Product` with SKU, name, price in cents, stock, status, and tags.
2. Give it `Available()` and `Reserve(quantity)` methods.
3. Create a slice containing at least three products.
4. Build a map keyed by SKU and reject duplicate SKUs.
5. Filter active products with stock greater than zero.
6. Sort the selected products by SKU before printing them.
7. Print each SKU, name, and subtotal for a requested quantity.
8. Return a wrapped sentinel error when a requested SKU is absent.
9. Write table-driven tests for `Reserve` and subtotal calculation.

Deliberately make these failures happen:

- request a missing SKU and prove `errors.Is` still finds the sentinel through the
  wrapper;
- mutate a product's tags after inserting it, observe whether storage is shared,
  and then fix the ownership boundary with a clone;
- run a negative reservation and confirm stock remains unchanged;
- print map values without sorting twice, then explain why a test must not rely on
  that order.

Use this validation sequence:

```text
gofmt -w ./path/to/exercise
go test ./path/to/exercise
go vet ./path/to/exercise
go test -race ./path/to/exercise
```

The exercise is complete when you can explain why each type, receiver, copy,
branch, and error check exists. A passing command is evidence, not the explanation.

## 16. Knowledge check

Answer before opening the explanations below:

1. Why can changing one slice appear to change another slice?
2. Why does a map lookup often need two results?
3. When is a pointer receiver appropriate?
4. What does an interface contain that makes a typed nil non-nil?
5. What is the difference between `errors.Is` and `errors.As`?
6. Why is `defer` usually placed immediately after successful acquisition?
7. Who should close a channel?
8. Does passing a struct by value make all nested data independent?
9. What does ranging over a string produce?
10. What can the race detector prove, and what can it not prove?

<details>
<summary>Explanations</summary>

1. Two slice descriptors can refer to the same backing array, so element mutation
   is visible through both.
2. The value alone cannot distinguish a missing key from a stored zero value; the
   second result reports whether the key existed.
3. Use one when the method must mutate the original value, copying is semantically
   wrong, or nil is a meaningful receiver state. Consistency across a type's method
   set also matters.
4. An interface stores both a dynamic type and a dynamic value. A nil pointer still
   supplies a dynamic type when stored in an interface.
5. `errors.Is` asks whether an error chain represents a particular error value or
   identity. `errors.As` finds a value assignable to a requested error type or
   interface so the caller can use that matched value.
6. The acquisition and cleanup rule remain adjacent, and every subsequent return
   path is covered.
7. Normally the producer that knows no more values will be sent. Receivers should
   not close a channel merely because they are finished receiving.
8. No. The struct fields are copied, but reference-like fields may still point at
   shared data.
9. Each iteration produces a byte offset and a decoded Unicode code point (`rune`).
10. It can report memory races that occur on executed paths. It cannot prove all
    paths are race-free or that a multi-step business rule is logically atomic.

</details>

## Where to continue

Continue with [Day 1: Go backend foundations](../day-01-go-backend-foundations.md).
That lesson applies these language mechanics to boundary design, resource ownership,
error contracts, cancellation, concurrency, and a tested in-memory backend store.
