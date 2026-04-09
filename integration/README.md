# Integration Test Harness

This package provides deterministic anti-cheat integration tests for:

- correctness (scoring/buffer behavior stability),
- network correctness (ACK execution order),
- performance (hot paths + packet replay benchmarks).

## Run Correctness Tests

```bash
go test ./integration -run Test -count=1
```

## Run Performance Benchmarks

```bash
go test ./integration -run ^$ -bench . -benchmem -count=3
```

## Included Benchmarks

- `BenchmarkEntityRewindExactAndNearest`: entity rewind lookup hot path.
- `BenchmarkKeyValsToString`: detection/log key-value formatting hot path.
- `BenchmarkHandleClientPacketReplayAuthInput`: end-to-end `PlayerAuthInput` replay through `HandleClientPacket`.
- `BenchmarkHandleClientPacketNetworkStackLatency`: end-to-end network ACK packet handling path.

## Suggested Regression Workflow

1. Run tests and benchmarks on `main` and save output.
2. Apply anti-cheat changes.
3. Re-run the same commands.
4. Compare benchmark medians and memory allocations.
5. Keep changes only if correctness tests pass and benchmark trends are not worse.
