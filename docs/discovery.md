# Balancer V3 discovery

Domain reference for `internal/discovery/`. For agent ops and repo layout, see
[`AGENTS.md`](../AGENTS.md).

## Overview

Discover Balancer V3 pools via the Balancer GraphQL API, catalogue them on `/pools`,
select a daily **test set** of `unique`-tagged pools, and run that set through the
same provider monitoring pipeline as `config.BaseEndpoints`.

## Execution model

Two independent goroutines in `main.go`:

| Loop | Cadence | Env var | What runs |
|------|---------|---------|-----------|
| Price check | Hourly (default 1h) | `CHECK_INTERVAL_HOURS` | `BaseEndpoints` only |
| Discovery | Daily (default 24h) | `DISCOVERY_INTERVAL_HOURS` | Fetch → test set → provider checks |

Discovery runs once immediately on startup (`discovery.Run`). The hourly loop does
not wait for the first discovery run.

After each discovery cycle:

1. Per-network pool snapshots are updated (see Failure handling).
2. Test set is rebuilt and pushed to the collector's discovered-endpoints store.
3. `monitor.RunDiscoveredOnce` runs provider checks (registered via `SetTestSetRunner`).

## Configuration

Per-network settings in `config.DiscoveryConfigs`:

| Field | Purpose |
|-------|---------|
| `Network` | Numeric chain ID (e.g. `"1"`, `"8453"`, `"100"`) |
| `Enabled` | Skip when false |
| `TVLThresholdUSD` | Threshold for `highTVL` tag (default $1M on enabled networks) |
| `TradePercent` | Trade size as % of `TokenIn` balance (default 5) |

Global env vars:

- `DISCOVERY_INTERVAL_HOURS` — default **24**
- `DISCOVERY_TEST_POOLS_PER_GROUP` — default **1** when unset (`config.GetDiscoveryTestPoolsPerGroup`)

## Balancer API

- Endpoint: `https://api-v3.balancer.fi/graphql` (hardcoded in `client.go`, no env override).
- Chain mapping: `config.BalancerAPIChain(network)` → `GqlChain` enum; empty string skips the network.
- Query: `poolsQuery` in `client.go`, `first: 1000`, no pagination. Add cursor pagination if a network exceeds 1000 V3 pools.
- Retries: up to 3 attempts per network per run (`maxFetchAttempts`) for cold-cache timeouts.

## Pool categories

After skip filters, each pool may be tagged:

| Tag | Meaning |
|-----|---------|
| `unique` | At least one registered token pair exists in exactly one V3 pool on that network (Balancer-only scope). **Required for test set candidacy.** |
| `highTVL` | `totalLiquidity` > network `TVLThresholdUSD`. Display/catalogue only unless also `unique`. |

A pool may have both tags, one, or neither (neither → not listed).

### Skip conditions (not added to catalogue)

- `dynamicData.isPaused` or `isInRecoveryMode`
- `totalLiquidity` < **$10,000** (`minTotalLiquidityUSD`)
- Hook type `AKRON` (excluded entirely)
- Pool types in `excludedPoolTypes` (`FIXED_LBP`, `LIQUIDITY_BOOTSTRAPPING`)
- Addresses in `excludedPoolAddresses`

### Per-pool captured fields

See `discovery.Pool` and `discovery.PoolToken` in `state.go`. StableSurge pools also store
`surgeThreshold`, `surgeImbalance`, and `surging` (see Surge skip).

## Test set selection

Implemented in `testset.Build`. Only pools with the `unique` category are candidates.

### Grouping

Group candidates by `(PoolType, HookType)` within each network.

### Per-group selection (`selectFromGroup`)

1. Classify each candidate's chosen unique pair as **boosted** (both tokens have
   `underlyingToken`) or **non_boosted**.
2. Sort each sub-bucket by `totalLiquidityUSD` descending.
3. Pick up to `N` pools (`DISCOVERY_TEST_POOLS_PER_GROUP`):
   - Both sub-buckets non-empty → top **1** boosted + top **1** non-boosted.
   - Only one sub-bucket → top `N` from that bucket.

Skip surging StableSurge pools from candidacy (`pool.Surging`); they remain on `/pools`.

### Pair selection (per selected pool)

One canonical pair per pool: among V3-unique pairs in that pool, pick the pair with
the **highest combined `balanceUSD`**. Direction: token with **smaller `balanceUSD`** is
`TokenIn`; tie or missing USD → lexicographic address order (lower = `TokenIn`).

### Rows emitted

| Pool kind | Rows |
|-----------|------|
| Non-boosted | 1 registered-token row |
| Boosted | 2 rows: registered (`-boosted` suffix) + underlying (`-underlying` suffix) |

Row key (implicit variant): `(network, pool_address, token_in, token_out)`.

### Trade sizing

- Semantic: `tradeUSD = (TradePercent / 100) × TokenIn.balanceUSD`, converted to raw units.
- Implementation: `(TradePercent / 100) × TokenIn.balance × 10^decimals` when `balanceUSD` > 0.
- Drop row when `balanceUSD` is missing/zero, human balance invalid, or raw amount rounds to zero.
- Underlying rows reuse the registered token's `balanceUSD` with underlying decimals.

**Deferred:** absolute cap via `MaxTradeUSD` in `DiscoveryConfig`.

## Surge skip

For `STABLE_SURGE` hooks, compute imbalance on USD balances (proxy for on-chain math):

```
median     = median(balanceUSD[])
totalDiff  = Σ |balanceUSD[i] - median|
imb        = totalDiff / Σ balanceUSD[i]
```

Treat as surging (excluded from test set) when:

```
imb >= surgeThresholdPercentage - stableSurgeImbalanceBuffer
```

`stableSurgeImbalanceBuffer = 0.10` in `state.go`. Missing/zero USD balances → `imb = 0`
(not surging; row may still drop at sizing).

## Discovered BaseName format

`formatDiscoveredBaseName` in `basename.go`:

```
{NETWORK}-{POOL_TYPE}-{HOOK_OR_NO_HOOK}-{TOKEN_IN}-{TOKEN_OUT}-{SHORT_ADDR}{SUFFIX}
```

- `NETWORK`: slugged `config.NetworkName`
- `HOOK_OR_NO_HOOK`: slugged hook type, or `NO-HOOK`
- `SHORT_ADDR`: first 10 chars of pool address (lowercase)
- `SUFFIX`: `-underlying` for underlying row; `-boosted` for boosted registered row; none otherwise

Provider `Name` = `{solver}-{BaseName}`.

## Display (`/pools`)

- Route: `GET /pools` (`handlers/pools.go`).
- Shows all catalogued pools; "in test set" badge; per-provider results stay on `/`.
- `discovery.LastSuccessAt()` shown as freshness (oldest per-network success timestamp).
- Sortable/filterable table — match existing dashboard patterns when extending.

## Failure handling

- **Per-network**: fetch/process failures log and **keep the previous snapshot** for that
  network. Other networks still update.
- **Panic**: `safeRunOnce` recovers, logs stack, emails alert; ticker continues.
- No in-run backoff beyond per-network fetch retries.
- No persistence across process restarts — full re-fetch on boot.

## Integration with monitoring

- Test set uses `monitor.ExpandForSolvers` → same providers as BaseEndpoints.
- `collector.Endpoint` has `PoolType` / `HookType` for discovered rows (empty for BaseEndpoints).
- `isWIPCase` uses type/hook when set, else name substring matching for legacy rows.
- Alerting: same email path as BaseEndpoints when `EMAIL_NOTIFICATIONS` is enabled.

## Manual trigger

Not implemented. Discovery runs on schedule only.

## Persistence

Not implemented (v1). All discovery state is in-memory.
