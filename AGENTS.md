# go-monitoring

Long-running service that quotes swaps against multiple DEX aggregators and checks
whether expected Balancer V3 pools appear in routes.

- **Hourly loop**: hand-curated `config.BaseEndpoints` → expanded per enabled solver.
- **Daily loop**: Balancer API discovery → test set → same provider pipeline.
- **UI**: `/` dashboard (results), `/pools` (discovered catalog).
- **Deploy**: Fly.io (`fly.toml`), Docker multi-stage build.

## Commands

```bash
go build -o /tmp/go-monitoring .
go test ./...
go run .                    # needs .env with provider API keys for live checks
docker build -t go-monitoring .
```

No CI or Makefile in this repo — run `go test ./...` before finishing changes.

## Layout

| Package | Role |
|---------|------|
| `config/` | `BaseEndpoints`, `DiscoveryConfigs`, route solvers, env helpers |
| `handlers/` | HTTP: `/`, `/pools`, `/check/` |
| `internal/discovery/` | Balancer GraphQL, categorization, test set, state |
| `internal/monitor/` | Provider registry, `ExpandForSolvers`, monitoring loops |
| `internal/collector/` | In-memory endpoint + result stores |
| `internal/api/` | Generic HTTP client for provider APIs |
| `providers/` | Per-aggregator handlers, URL builders, parsers |
| `notifications/` | Resend email on failures / startup |

**Discovery domain rules**: see [`docs/discovery.md`](docs/discovery.md) before changing
discovery, test set selection, surge skip, or `BaseName` formatting.

## Invariants

- **Two goroutines, two cadences**: `monitor.MonitorAPIs` (hourly, BaseEndpoints only)
  and `discovery.Run` (daily, discovery + test set). Do not put discovered rows on the
  hourly loop.
- **Shared expansion**: `monitor.ExpandForSolvers` is used by BaseEndpoints startup and
  discovery. Do not duplicate solver×network filtering elsewhere.
- **In-memory only (v1)**: no DB. Each successful per-network fetch replaces that
  network's snapshot; failed fetches keep the previous snapshot for that network.
- **Test set ≠ discovered list**: only `unique`-tagged pools are tested; `highTVL`-only
  pools are catalogued on `/pools` only.
- **Row identity**: `(network, pool_address, token_in, token_out)` — boosted pools emit
  separate registered vs underlying rows.
- **WIP skips**: `internal/monitor/provider_registry.go` `isWIPCase` — prefer
  `PoolType` / `HookType` on discovered rows; keep `endpoint.Name` substring fallback
  for BaseEndpoints.
- **`balancer_sor`**: may run on-chain price follow-up after the API quote.

## Environment

Loads `.env` via godotenv if present. Do not commit secrets.

| Variable | Default | Purpose |
|----------|---------|---------|
| `CHECK_INTERVAL_HOURS` | 1 | BaseEndpoints monitoring cadence |
| `DISCOVERY_INTERVAL_HOURS` | 24 | Discovery + test set cadence |
| `DISCOVERY_TEST_POOLS_PER_GROUP` | 1 | Max pools per `(PoolType, HookType)` group |
| `EMAIL_NOTIFICATIONS` | off | Alert on check failures |
| `RESEND_API_KEY` | — | Email delivery |
| `DISABLE_<SOLVER>` | — | e.g. `DISABLE_0X=true` disables a route solver |
| Provider keys | — | `ZEROX_API_KEY`, `INCH_API_KEY`, `HYPERBLOOM_API_KEY`, `BARTER_API_KEY` |

Route solvers are registered in `internal/monitor/provider_registry.go` →
`InitializeRegistry()`. Enabled list: `config.GetEnabledRouteSolvers()`.

## Common tasks

### New route solver

1. Handler + URL builder in `providers/<name>_handler.go` (follow 0x / odos patterns).
2. Register in `InitializeRegistry()` with `Handler`, `URLBuilder`, optional
   `RequestBodyBuilder`, `APIKeyEnvVar`, `UsePOST`.
3. Add to `config.GetEnabledRouteSolvers()` with `SupportedNetworks`.
4. Unit tests in `providers/` for response parsing edge cases.

### Discovery change

1. Read [`docs/discovery.md`](docs/discovery.md).
2. Core code: `internal/discovery/discovery.go`, `testset.go`, `state.go`, `client.go`,
   `basename.go`.
3. Add tests in `internal/discovery/*_test.go`.
4. Display: `handlers/pools.go` — do not duplicate per-provider result tables on `/pools`.

### New network

1. `config.DiscoveryConfigs` + `config.BalancerAPIChain`.
2. Solver `SupportedNetworks` entries as needed.

## Conventions

- Go 1.24, module `go-monitoring`.
- Match existing style: focused diffs, minimal comments, no drive-by refactors.
- Tests are unit-level; avoid new tests that require live aggregator APIs unless
  clearly skipped.

## Deferred (do not add without updating docs)

- Persistence (SQLite / volumes)
- Manual discovery trigger
- `MaxTradeUSD` trade cap on discovery rows
- Per-provider results on `/pools`

## Deploy

- Fly app `go-monitoring`, region `ams`, port `8080`.
- `GO_VERSION=1.24` in `Dockerfile` / `fly.toml`.
