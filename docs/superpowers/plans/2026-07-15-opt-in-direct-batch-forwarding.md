# Opt-in Direct Batch Forwarding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an opt-in standalone-proxy mode that processes and immediately forwards complete gophertunnel batches in order, reducing request/response ping by about 50 ms on average.

**Architecture:** `proxy.Config.EnableBatchForwarding` selects the existing packet-at-a-time loops or new batch loops. Batch mode is capability-based for custom backends, processes packets sequentially, and uses `WritePacketImmediate` so previously buffered Oomph output cannot be overtaken.

**Tech Stack:** Go, gophertunnel PR #80 `ReadBatch`, gophertunnel `WritePacketImmediate`, standard `testing` package.

## Global Constraints

- The option defaults to false and does not change existing proxy behavior.
- The native Dragonfly integration and `player.Player` remain unchanged.
- Packet order is preserved within batches, between batches, and relative to Oomph-generated output.
- Batch mode must not use `WritePacketDirect`.
- Public documentation must state that each forwarding direction loses a 0–50 ms wait, reducing request/response ping by about 50 ms on average, with a timing-dependent 0–100 ms range.

---

### Task 1: Dependency and opt-in connection capabilities

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`
- Modify: `integration/proxy/proxy.go`
- Test: `integration/proxy/proxy_test.go`

**Interfaces:**
- Produces: `Config.EnableBatchForwarding bool`
- Produces: internal `batchReader` and `immediatePacketWriter` capability interfaces.
- Produces: `defaultDial(time.Duration, bool) DialFunc`.

- [ ] **Step 1: Write failing configuration and capability tests**

Add tests that call a capability helper with the existing fake backend and expect an error containing `batch forwarding requires`. Verify that disabled configuration retains the ordinary backend surface.

- [ ] **Step 2: Run the focused tests and verify RED**

Run: `go test ./integration/proxy -run 'TestBatchForwardingCapabilities|TestBatchForwardingDefaultsDisabled'`

Expected: compilation failure because the option and helper do not exist.

- [ ] **Step 3: Upgrade gophertunnel and implement capability plumbing**

Run: `go get github.com/sandertv/gophertunnel@23b8e112bf2004f5525b894d3f3986cd90cf2ab5`

Add:

```go
type batchReader interface {
	ReadBatch() ([]packet.Packet, error)
}

type immediatePacketWriter interface {
	WritePacketImmediate(...packet.Packet) error
}
```

Add the config field with a doc comment. In `Listen`, set `cfg.Listen.EnableBatchReading = true` only when the option is enabled. Pass the option to `defaultDial`, which sets `Dialer.EnableBatchReading` on default backend connections. Keep `Backend` and `DialFunc` source-compatible.

- [ ] **Step 4: Run focused and package tests and verify GREEN**

Run: `go test ./integration/proxy`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum integration/proxy/proxy.go integration/proxy/proxy_test.go
git commit -m "feat(proxy): add opt-in batch forwarding capabilities"
```

### Task 2: Client-to-backend batch processing

**Files:**
- Modify: `integration/proxy/proxy.go`
- Test: `integration/proxy/proxy_test.go`

**Interfaces:**
- Consumes: `batchReader`, `immediatePacketWriter`, and `Config.EnableBatchForwarding` from Task 1.
- Produces: a client batch loop that preserves packet order and immediately flushes the backend queue.

- [ ] **Step 1: Write failing client-batch tests**

Extend fakes to supply batches and record calls as `[][]packet.Packet`. Test that two packets are forwarded in one immediate call in original order. Add a test where handling cancels every packet but the destination still receives one zero-packet immediate call, proving its existing queue is flushed.

- [ ] **Step 2: Run the client-batch tests and verify RED**

Run: `go test ./integration/proxy -run 'TestClientBatch'`

Expected: FAIL because `clientLoop` still calls `ReadPacket` and `WritePacket`.

- [ ] **Step 3: Implement the batch-selected client loop**

Keep the legacy loop unchanged when the option is false. In enabled mode, assert client `batchReader` and backend `immediatePacketWriter`, process the returned slice sequentially under `routeMu`, append non-cancelled packets, then call:

```go
err = writer.WritePacketImmediate(forwarded...)
```

Call it even when `forwarded` is empty.

- [ ] **Step 4: Run focused and package tests and verify GREEN**

Run: `go test ./integration/proxy -run 'TestClientBatch' && go test ./integration/proxy`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add integration/proxy/proxy.go integration/proxy/proxy_test.go
git commit -m "feat(proxy): immediately forward client batches"
```

### Task 3: Backend-to-client batches and transfers

**Files:**
- Modify: `integration/proxy/proxy.go`
- Test: `integration/proxy/proxy_test.go`

**Interfaces:**
- Consumes: Task 1 capability interfaces.
- Produces: an ordered backend batch loop with transfer-boundary flushing.

- [ ] **Step 1: Write failing backend batch and transfer tests**

Test that server packets are rewritten, tracked, and sent in one immediate call in order. Test a batch `[ordinary, Transfer, tail]`: the ordinary prefix is immediately written before transfer begins, while `tail` is not forwarded from the old backend.

- [ ] **Step 2: Run the backend-batch tests and verify RED**

Run: `go test ./integration/proxy -run 'TestBackendBatch'`

Expected: FAIL because `backendLoop` still reads and writes one packet at a time.

- [ ] **Step 3: Implement backend batch processing**

Select the legacy loop when disabled. In batch mode, read from the current generation, process ordinary packets sequentially into `forwarded`, and use `WritePacketImmediate(forwarded...)`. On `Transfer`, flush the prefix first, perform the existing transfer, discard the old batch tail, and resume by reading the current backend generation.

- [ ] **Step 4: Run focused, race, and package tests and verify GREEN**

Run: `go test ./integration/proxy -run 'TestBackendBatch' && go test -race ./integration/proxy`

Expected: PASS with no race reports.

- [ ] **Step 5: Commit**

```bash
git add integration/proxy/proxy.go integration/proxy/proxy_test.go
git commit -m "feat(proxy): immediately forward backend batches"
```

### Task 4: Public latency documentation and final verification

**Files:**
- Modify: `docs/setup.md`
- Modify: `example/default/default.go`
- Modify: `docs/superpowers/specs/2026-07-15-opt-in-direct-batch-forwarding-design.md`

**Interfaces:**
- Consumes: `proxy.Config.EnableBatchForwarding`.
- Produces: a runnable opt-in example and an accurate latency/cost explanation.

- [ ] **Step 1: Document and enable the option in the standalone example**

Set `EnableBatchForwarding: true` in `example/default/default.go`. Add a setup section explaining that enabled mode removes the independent 0–50 ms proxy flush wait in each direction, reduces request/response ping by about 50 ms on average (0–100 ms depending on tick alignment), and may use more CPU/bandwidth because separate inbound batches are no longer combined by the proxy's 50 ms tick.

- [ ] **Step 2: Update the approved design with the same quantified claim**

Add the quantified average and range to the goal and ordering sections without claiming a guaranteed fixed reduction.

- [ ] **Step 3: Format and verify all affected modules**

Run:

```bash
gofmt -w integration/proxy/proxy.go integration/proxy/proxy_test.go example/default/default.go
go test ./integration/proxy ./integration/dragonfly ./player/...
(cd example/default && go test ./...)
git diff --check
```

Expected: all tests PASS and `git diff --check` prints nothing.

- [ ] **Step 4: Commit**

```bash
git add docs/setup.md example/default/default.go docs/superpowers/specs/2026-07-15-opt-in-direct-batch-forwarding-design.md
git commit -m "docs: explain batch forwarding latency tradeoff"
```
