# Integrations, Maintenance, and Leadership

These lessons exercise engineering judgment where systems meet unreliable providers,
existing behavior, and team review. Completion requires evidence and handoff, not
only code.

## Common Production Integration Boundaries

### Goal and mental model

Third-party systems have independent quotas, latency, deployment, versions, outages,
and support processes. Place each behind a narrow adapter that translates provider
details into application-owned operations and stable errors.

For outbound calls define total deadline, connection/body limits, auth, idempotency,
retry ownership, rate/quota behavior, request identity, redaction, metrics, sandbox
tests, and provider outage policy. A fake supports deterministic application tests;
at least one sandbox/component test proves the real protocol.

Webhooks are untrusted inbound messages. Reject bodies beyond a strict byte limit,
then verify the signature over the exact bounded raw body before parsing. Use the
documented algorithm and key, compare signatures safely, check a bounded timestamp/
replay window, deduplicate event identity durably, and return quickly after recording
durable work. Key rotation requires overlapping verification keys. Outbound webhook
delivery needs attempt history, signature, idempotent event identity, backoff,
disable/quarantine rules, and customer-visible diagnostics. Treat subscriber URLs as
SSRF input: require HTTPS, resolve and reject private/link-local destinations, prevent
or revalidate redirects, defend against DNS rebinding, and restrict runtime egress.
Retry network failures, throttling, and selected server failures; treat most client
errors as permanent, honor `Retry-After`, and use bounded backoff with jitter.

Large uploads should go directly from an authorized client to object storage using a
short-lived, narrowly scoped upload grant. Use random object keys, size/type limits,
checksum, server-observed metadata, encryption, private access, lifecycle cleanup,
and asynchronous malware/content processing. Do not trust filename or client MIME as
proof of content.

Email/notification is asynchronous. Store preference and suppression state, stable
message identity, provider ID, attempts, delivery/bounce/complaint callbacks, and
audit. A provider acceptance means queued by provider, not read by a person.

Search and analytics are derived views. Authoritative data/events must rebuild them.
Track source position/version, lag, failed documents, and reconciliation differences.
Do not write business truth only to search.

### Start with a concrete situation

The service asks a payment provider to charge operation `pay-O-42`. The provider
times out after receiving the request. Retrying without a stable provider
idempotency key may charge twice; reporting failure may hide a successful charge.
The adapter needs a way to retry or query the same provider operation and translate
its outcomes into application-owned states.

Now reverse the direction. The provider later sends a webhook saying the payment
succeeded. Treat the body as hostile until its size, signature, timestamp, and event
identity are verified. Store durable work before replying if the provider interprets
a successful response as final acknowledgement.

Server-side request forgery (SSRF) occurs when an attacker makes the service connect
to an unintended address, often through a supplied webhook URL. HTTPS syntax alone
does not make a destination safe: DNS can resolve to private or link-local systems,
and redirects can change the destination after validation. Destination policy,
resolution checks, redirect handling, and restricted network egress must work
together.

Learn one outbound call and one inbound webhook completely before adding uploads,
email, search, and analytics. The same boundary questions—identity, limits,
ambiguity, retry, durable state, and observability—then become recognizable.

### Guided work

1. Choose one outbound provider and define an application-owned adapter interface.
2. Implement bounded client behavior, stable error classes, a fake, and sandbox test.
3. Add inbound webhook byte limits, raw-body signature, timestamp, key rotation,
   durable dedupe, and asynchronous processing.
4. Add outbound webhook subscription, safe destination validation, signed delivery,
   history, classified retry, and quarantine.
5. Design direct object upload grants and server-side completion verification.
6. Stream processing with checksum and bounded buffers; add scan/quarantine states.
7. Add notification preferences and authenticated provider callback handling.
8. Build a search/read model from authoritative events with checkpoints.
9. Add complete rebuild and incremental reconciliation commands.
10. Instrument provider latency/quota, webhook age/delivery, upload state, notification
   outcomes, and search lag.

### Failure checks

Return provider timeout, throttle, malformed/oversized body, and ambiguous write.
Forge, duplicate, reorder, delay, and rotate webhook signatures. Take the subscriber
endpoint down; try private and DNS-rebound destinations; return permanent `4xx`,
throttling, and transient `5xx` responses. Upload oversized, mislabeled,
checksum-invalid, unauthorized, and malicious objects. Duplicate bounce callbacks.
Delete the search index and rebuild it while new events arrive.

### Knowledge check and expected answers

1. **Why verify a webhook before parsing?** Signature usually covers exact raw bytes;
   parsing/re-encoding changes them and unverified input is untrusted.
2. **Why direct-upload to object storage?** It avoids buffering large files through
   application memory while preserving scoped authorization and validation.
3. **Why must a read model be rebuildable?** It is eventually consistent derived
   state and can drift, corrupt, or need a schema/index replacement.

### Completion evidence

Provider timeout/throttle/outage outcomes are deterministic; webhook forgery,
replay, duplicates, reorder, downtime, rotation, unsafe destinations, and retry
classification are tested; file streaming stays bounded and unauthorized objects
remain inaccessible; notification callbacks are idempotent; and derived views
rebuild/reconcile entirely from authoritative state.

## Maintenance and Legacy-System Evolution

### Goal and mental model

Most backend work changes a system with users, data, undocumented behavior, and
operational history. Discover actual behavior before improving it. Sources include
code, tests, API examples, telemetry, database shape, feature flags, incident notes,
support reports, and user workflows. Distinguish intentional contract from accidental
behavior and unknown risk.

Characterization tests pin important existing behavior without claiming it is ideal.
Place them at the narrowest boundary that observes the contract. Once behavior and
risk are understood, introduce a seam and refactor in small steps while tests remain
green.

Upgrades require release notes and compatibility analysis:

- language/toolchain: compiler, runtime, race, standard-library behavior;
- dependency: API, defaults, security, transitive dependencies;
- database: SQL/planner/extension/replication/backup compatibility;
- infrastructure: state, provider behavior, rollout and rollback.

Feature flags make release reversible only when default, ownership, targeting,
telemetry, dependency interactions, and removal date are explicit. A completed flag
is debt: remove old branch, tests, config, dashboard dimensions, and documentation.

Canary rollout exposes a small real cohort with abort signals. Shadow traffic copies
requests only when privacy and side effects are controlled; shadow writes must not
produce real effects.

A data repair tool needs explicit scope, dry-run, idempotency, bounded batches,
resume checkpoint, rate limit, authorization, audit, expected-state guard,
postcondition verification, and kill switch. Never accept an ambiguous production
target or broad unreviewed selector.

Regression diagnosis starts from evidence: reproduce, bound first bad version/time,
compare code/config/schema/data/dependency changes, form one hypothesis, test it,
add regression protection, and verify recovery. Avoid masking the symptom with a
broad fallback.

### Start with a concrete situation

A legacy endpoint returns an empty list when a customer has no orders. The intended
contract was never written, and a proposed refactor would return `null`. Before
calling either behavior correct, inspect clients, tests, examples, telemetry, and
support history. If consumers rely on `[]`, capture that behavior with a
characterization test.

The safe sequence is:

```text
observe actual behavior
  -> identify relied-on contract
  -> pin it with a focused test
  -> create a small seam
  -> refactor without changing the contract
  -> deliberately migrate later if the contract should change
```

A characterization test is not praise for the old design. It is an alarm that
tells you whether a supposedly behavior-preserving change altered something real.
Use the same discipline for upgrades, feature flags, canaries, and data repairs:
first define the boundary and evidence, then make the reversible change.

### Guided work

1. Select a mature area and create a behavior/risk inventory from every evidence
   source.
2. Mark confirmed contracts, accidental behavior, assumptions, and unknowns.
3. Add characterization tests for critical public/data/operational behavior.
4. Refactor one coupled area through small reviewable moves with no behavior change.
5. Upgrade Go plus one significant dependency or infrastructure component.
6. Run compatibility, performance, migration, and rollback/roll-forward checks.
7. Release one change behind a flag to a canary cohort with abort criteria.
8. Roll it back once, then complete rollout and remove the flag and old path.
9. Build/rehearse a data repair in dry-run and isolated representative state.
10. Diagnose a seeded regression using only repository and production-like evidence;
   document the handoff and residual risk.

### Failure checks

Run characterization tests against old and new versions. Interrupt the repair and
resume it. Attempt a repair outside allowed tenant/time/ID scope. Change data between
dry-run and execution and require expected-state rejection. Trigger canary abort.
Search after completion for flag names, compatibility branches, stale dependencies,
and documentation.

### Knowledge check and expected answers

1. **Why characterize before refactoring?** Unknown but relied-on behavior otherwise
   changes without evidence or deliberate decision.
2. **Why is a feature flag temporary?** Two behavior paths multiply tests,
   operations, and future change risk after migration value ends.
3. **What prevents data repair overreach?** Explicit target/scope, dry-run, expected-
   state guards, bounded/resumable execution, audit, and verification.

### Completion evidence

Before/after contract, data, performance, and operations are verified; upgrades have
executed recovery paths; canary rollback works; repair cannot escape scope and leaves
durable audit; the seeded regression is diagnosed and prevented; temporary flags and
compatibility code are removed; and another engineer understands remaining risk.

## Capstone Delivery and Engineering Leadership

### Goal and mental model

The capstone demonstrates end-to-end judgment: clarify a valuable problem, compare
alternatives, expose risk, seek critical review, deliver compatible increments,
operate the result, and make the system easier for others to own.

Choose a feature that exercises real risks without adding arbitrary technologies.
Examples include order cancellation with inventory compensation, administrator stock
adjustment with audit and asynchronous notification, or tenant-scoped bulk ordering.
The problem should require concurrency, authorization, failure handling, rollout,
and observability relevant to its behavior.

The design document should contain:

1. problem, users, outcomes, non-goals, and success measures;
2. facts, assumptions, open questions, and decision owners;
3. domain examples, states, invariants, and authorization;
4. alternatives including the smallest viable design;
5. API/event contracts and compatibility policy;
6. data ownership, schema, migrations, transaction/distributed workflow;
7. idempotency, timeouts, retries, concurrency, and failure matrix;
8. threat/privacy model and least privilege;
9. logs, metrics, traces, SLIs, alerts, and runbooks;
10. capacity/cost assumptions and measurement plan;
11. incremental rollout, mixed-version behavior, rollback/roll-forward, and cleanup;
12. test evidence and unresolved/residual risk.

Request critical review before implementation. Record each finding, decision,
rationale, owner, and resulting design change. Disagreement should become explicit
tradeoff, not silent avoidance.

Split delivery into independently reviewable vertical increments. Each preserves
compatibility, passes verification, updates owning documentation, and has a safe
stopping point. Avoid a long-lived rewrite branch.

Create a requirement traceability table:

```text
requirement -> code/contract -> test -> telemetry -> runbook/operation
```

Run evidence relevant to the design: concurrency, dependency failure, duplicate
delivery, restart, timeout, mixed versions, migration/backfill, load, privacy,
security, and recovery. Do not claim proof from irrelevant checks.

Leadership includes reviewing someone else's change for data safety, security,
operability, simplicity, and missing tests; writing a concise operational handoff;
and teaching one concept so another engineer can operate or extend it.

### Start with a concrete situation

Choose a capstone such as order cancellation with inventory release. Begin with a
single concrete case:

```text
Customer C-17 owns placed order O-42.
O-42 reserved two units of TEA-001.
C-17 requests cancellation twice while Inventory is briefly unavailable.
```

From that case derive the questions the design must answer: who may cancel, which
states allow it, whether repeated cancellation is safe, how inventory release is
identified, what happens during an ambiguous timeout, how operators find stuck
work, and what the customer sees while completion is pending.

Trace every important requirement across the system:

```text
requirement
  -> API or event contract
  -> state transition and implementation
  -> deterministic test
  -> production signal
  -> operator recovery step
```

The capstone is not a contest to use every earlier technology. Strong engineering
judgment includes rejecting mechanisms the requirement does not need and naming
what could be deleted if the problem became simpler. Another engineer should be
able to review, deploy, diagnose, and extend the result without private knowledge
from its author.

### Guided work

1. Select a user problem and define measurable outcomes/non-goals.
2. Complete discovery examples and the design document before implementation.
3. Obtain critical domain, backend, security, and operations review.
4. Revise the design and publish the finding/decision log.
5. Plan small compatible increments with explicit dependencies and exit criteria.
6. Deliver each increment with tests, telemetry, docs, and review.
7. Run migration and staged rollout while watching defined signals.
8. Inject relevant failures and rehearse rollback or roll-forward.
9. Complete the traceability matrix and close or own every review finding.
10. Review a peer change, write the operational handoff, and teach one system concept.
11. Remove rollout flags, compatibility scaffolding, unused abstraction, and temporary
   dashboards after verification.
12. Present which complexity would be deleted if requirements became simpler.

### Failure checks

Repeat the feature request concurrently, lose each dependency at each transition,
duplicate and reorder messages, restart every owning process, deploy old/new versions
in both orders, interrupt migrations/backfills, exhaust the first predicted resource,
and execute the recovery path from the handoff without author assistance.

### Knowledge check and expected answers

1. **What distinguishes leadership from implementing the code?** Reducing shared
   uncertainty through decisions, review, evidence, operational ownership, and
   enabling other engineers.
2. **Why trace requirements to telemetry/runbooks?** A requirement is not fully owned
   if the team cannot detect, diagnose, or recover when it fails in operation.
3. **Why explain removable complexity?** Good judgment matches architecture to
   requirements and can simplify when constraints disappear.

### Completion evidence

Reviewers can trace every requirement to implementation, tests, telemetry, and
operations; relevant concurrency/distribution/restart/mixed-version tests pass;
reliability, performance, and cost claims have production-like evidence; migration
and recovery are rehearsed; findings are resolved or explicitly owned; another
engineer can operate the feature from the handoff; and temporary complexity is gone.
