# Study Materials

These materials complement the main
[backend learning roadmap](../BACKEND_LEARNING_ROADMAP.md). The roadmap remains the
source of truth for sequence, deliverables, and acceptance criteria. This directory
contains three different kinds of material; use each for its intended purpose.

## Choose the right learning path

If Go syntax feels rusty, begin with the
[Go language refresher](foundations/go-language-refresher.md). It rebuilds the
language fundamentals through one small product-catalog example and a guided
exercise. If the syntax and core value semantics are already comfortable, begin
with the seven daily lessons instead. They introduce one running product-and-order
service and teach the backend concepts needed before the longer implementation
phases.

The word **Day** indicates sequence, not a deadline. A lesson may take several
sessions when the concepts are new. Stop at section boundaries, run the examples,
and continue only when you can explain the previous checkpoint without copying its
wording.

After completing the seven-day foundation, follow the phase lessons sequentially.
They are project-based lessons: each phase starts from the system built in earlier
phases, introduces a production problem, develops a mental model, and then asks you
to implement and verify a solution.

Use the key-concept refresher only after learning a phase. It intentionally compresses
ideas and is unsuitable as the first explanation of a subject.

Every genuine lesson should be read in this order:

1. Start with the concrete failure or user need.
2. Build a plain-language mental model.
3. Trace one worked example.
4. Implement one bounded increment.
5. Make the failure happen deliberately.
6. Explain the result in your own words.
7. Keep the requested completion evidence.

Do not advance merely because the code compiles. Advance when you can predict the
important success and failure paths before running them.

## Detailed lessons

- [Go language refresher](foundations/go-language-refresher.md) - an optional pre-Day-1 lesson covering program structure, syntax, collections, functions, structs, methods, pointers, interfaces, generics, errors, concurrency syntax, testing, and common traps.
- [Day 1: Go backend foundations](day-01-go-backend-foundations.md) - a guided lesson on Go modules, types, ownership, interfaces, errors, context, concurrency, testing, and the in-memory product-store exercise.
- [Day 2: HTTP contracts and request lifecycle](day-02-http-contracts-and-lifecycle.md) - HTTP semantics, strict request handling, middleware, public errors, handler tests, and graceful shutdown.
- [Day 3: PostgreSQL and relational data](day-03-postgresql-and-relational-data.md) - relational constraints, migrations, pools, parameterized SQL, transactions, concurrency, integration tests, and query plans.
- [Day 4: Business logic and architecture](day-04-business-logic-and-architecture.md) - vertical features, representation boundaries, invariants, narrow interfaces, dependency injection, transaction ownership, and error mapping.
- [Day 5: Identity and API security](day-05-identity-and-api-security.md) - OAuth/OIDC roles, token and JWKS validation, principals, role and object authorization, browser security, redaction, and threat modeling.
- [Day 6: Testing, observability, and operations](day-06-testing-observability-and-operations.md) - test boundaries, deterministic fixtures, logs, metrics, traces, service objectives, limits, retries, failure drills, alerts, and runbooks.
- [Day 7: Delivery and production rehearsal](day-07-delivery-and-production-rehearsal.md) - containers, runtime configuration, CI, compatible migrations, rollback, restore testing, capacity, incident rehearsal, and clean-clone verification.

External links in the roadmap are optional primary references for deeper study. They are not prerequisites for completing a detailed lesson.

## Detailed implementation phases

- [Phases 0-26 lesson index](phase-lessons/README.md) - project-based lessons covering domain discovery, one production service, durable work, distributed systems, service extraction, scaling, security, cloud/platform operations, integrations, maintenance, and the capstone. They assume the foundation lessons or equivalent experience.

## Refresher guides

- [Backend phase key concepts](backend-phase-key-concepts.md) - concise recall notes for implementation Phases 0-26, including mental models, failure modes, and self-check questions.

## Suggested refresher method

1. Read the phase's **Core ideas** before starting its work.
2. Explain the **Memory anchors** without looking at the notes.
3. Answer the **Self-check** questions in your own words.
4. Apply the roadmap's **cross-cutting microservice lens** to the phase, even while its implementation remains in-process.
5. Return to the roadmap for the phase's deliverables and acceptance criteria.
