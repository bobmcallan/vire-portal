# Portal: Portfolio & Cash Field Naming Refactor

**Date:** 2026-03-02
**Status:** Pending
**Upstream:** [vire-server refactor-portfolio-field-naming.md](/home/bobmc/development/vire/docs/refactor-portfolio-field-naming.md)
**Scope:** All portal files consuming Vire API JSON fields — Go models, JavaScript, HTML templates, and tests

---

## Summary

The Vire server is renaming all portfolio, holding, cash, and capital fields to follow a consistent `{qualifier}_{category}_{timescale}_{measure}_{suffix}` convention. This is a single-phase rename with no backward compatibility — old field names are removed immediately.

The portal must update all field references before deploying the server change. The portal is a thin proxy + UI layer — it does not compute these values, only reads and displays them.

---

## Affected Files

| File | Layer | Change Type |
|---|---|---|
| `internal/vire/models/portfolio.go` | Go models | Struct field + JSON tag renames |
| `internal/vire/models/navexa.go` | Go models | **No change** (excluded from server refactor) |
| `pages/static/common.js` | JavaScript | JSON field access renames + API URL change |
| `pages/dashboard.html` | HTML template | Holding field name renames in `x-text` bindings |
| `internal/server/proxy_stress_test.go` | Go test | JSON test fixtures + API path update |
| `internal/handlers/dashboard_stress_test.go` | Go test | Dashboard label assertions (if labels change) |
| `tests/ui/dashboard_test.go` | UI test | Dashboard label assertions (if labels change) |

---

## Change Inventory

### 1. `internal/vire/models/portfolio.go` — Go Struct Renames

#### Portfolio struct (lines 42–62)

| Line | Current Field | Current JSON Tag | New Field | New JSON Tag |
|---|---|---|---|---|
| 48 | `TotalValue` | `total_value` | `PortfolioValue` | `portfolio_value` |
| 49 | `TotalCost` | `total_cost` | `NetEquityCost` | `net_equity_cost` |
| 50 | `TotalGain` | `total_gain` | — | — |
| 51 | `TotalGainPct` | `total_gain_pct` | — | — |
| 52 | `TotalNetReturn` | `total_net_return` | `NetEquityReturn` | `net_equity_return` |
| 53 | `TotalNetReturnPct` | `total_net_return_pct` | `NetEquityReturnPct` | `net_equity_return_pct` |
| 54 | `AvailableCash` | `available_cash` | `NetCashBalance` | `net_cash_balance` |
| 55 | `CapitalGain` | `capital_gain` | `NetCapitalReturn` | `net_capital_return` |
| 56 | `CapitalGainPct` | `capital_gain_pct` | `NetCapitalReturnPct` | `net_capital_return_pct` |

> **Note on `TotalGain` / `TotalGainPct`:** These fields exist in the portal model but are NOT listed in the server's rename table. The server doc renames `total_value` but does not mention `total_gain`. These fields come from Navexa upstream and may have been removed or replaced. **Action:** Verify with the server whether `total_gain` / `total_gain_pct` survive the refactor. If removed, delete from Portal struct. If renamed, update accordingly.

#### Holding struct (lines 65–89)

| Line | Current Field | Current JSON Tag | New Field | New JSON Tag |
|---|---|---|---|---|
| 75 | `Weight` | `weight` | `PortfolioWeightPct` | `portfolio_weight_pct` |
| 76 | `TotalCost` | `total_cost` | `CostBasis` | `cost_basis` |
| 77 | `TotalProceeds` | `total_proceeds` | `GrossProceeds` | `gross_proceeds` |
| 79 | `CapitalGainPct` | `capital_gain_pct` | `AnnualizedCapitalReturnPct` | `annualized_capital_return_pct` |
| 82 | `TotalReturnPctTWRR` | `total_return_pct_twrr` | `TimeWeightedReturnPct` | `time_weighted_return_pct` |

**Kept unchanged:** `MarketValue`, `AvgCost`, `CurrentPrice`, `NetReturn`, `NetReturnPct`, `DividendReturn`

> **Note:** Holding fields `net_return` and `net_return_pct` are kept per the server doc. The holding `market_value` is also kept.

#### PortfolioReview struct (lines 97–113)

| Line | Current Field | Current JSON Tag | New Field | New JSON Tag |
|---|---|---|---|---|
| 101 | `TotalValue` | `total_value` | `PortfolioValue` | `portfolio_value` |
| 102 | `TotalCost` | `total_cost` | `NetEquityCost` | `net_equity_cost` |
| 103 | `TotalGain` | `total_gain` | — | — |
| 104 | `TotalGainPct` | `total_gain_pct` | — | — |
| 105 | `DayChange` | `day_change` | `PortfolioDayChange` | `portfolio_day_change` |
| 106 | `DayChangePct` | `day_change_pct` | `PortfolioDayChangePct` | `portfolio_day_change_pct` |

> **Note on `TotalGain` / `TotalGainPct`:** Same question as Portfolio struct above — verify with server.

#### PortfolioSnapshot struct (lines 157–168)

| Current Field | New Field | Rationale |
|---|---|---|
| `TotalValue` | `EquityValue` | Internal only (no JSON tag). Consistent with server. |
| `TotalCost` | `NetEquityCost` | Internal only. Consistent. |
| `TotalGain` | — | Verify with server |
| `TotalGainPct` | — | Verify with server |

#### SnapshotHolding struct (lines 171–177)

| Current Field | New Field |
|---|---|
| `TotalCost` | `CostBasis` |
| `Weight` | `PortfolioWeightPct` |

#### GrowthDataPoint struct (lines 179–188)

| Current Field | New Field | Rationale |
|---|---|---|
| `TotalValue` | `EquityValue` | Consistent with server. Internal only. |
| `TotalCost` | `NetEquityCost` | Consistent. Internal only. |

### 2. `internal/vire/models/navexa.go` — NO CHANGES

Per the server doc: *"Navexa upstream structs are excluded from this refactor. These structs mirror the external Navexa API shape."*

The portal's `NavexaPortfolio`, `NavexaHolding`, `NavexaTrade`, `NavexaPerformance` structs are **not changed**.

### 3. `pages/static/common.js` — JavaScript Field Access Renames

#### Portfolio data parsing (loadPortfolio, lines 255–282)

| Line | Current | New |
|---|---|---|
| 263 | `holdingsData.total_value` | `holdingsData.portfolio_value` |
| 264 | `holdingsData.total_net_return` | `holdingsData.net_equity_return` |
| 265 | `holdingsData.total_net_return_pct` | `holdingsData.net_equity_return_pct` |
| 266 | `holdingsData.total_cost` | `holdingsData.net_equity_cost` |
| 267 | `holdingsData.available_cash` | `holdingsData.net_cash_balance` |
| 272 | `holdingsData.capital_gain` | `holdingsData.net_capital_return` |
| 273 | `holdingsData.capital_gain_pct` | `holdingsData.net_capital_return_pct` |
| 274 | `cp.simple_return_pct` | `cp.simple_capital_return_pct` |
| 275 | `cp.annualized_return_pct` | `cp.annualized_capital_return_pct` |

#### Refresh portfolio parsing (refreshPortfolio, lines 486–514)

| Line | Current | New |
|---|---|---|
| 495 | `data.total_value` | `data.portfolio_value` |
| 496 | `data.total_net_return` | `data.net_equity_return` |
| 497 | `data.total_net_return_pct` | `data.net_equity_return_pct` |
| 498 | `data.total_cost` | `data.net_equity_cost` |
| 499 | `data.available_cash` | `data.net_cash_balance` |
| 504 | `data.capital_gain` | `data.net_capital_return` |
| 505 | `data.capital_gain_pct` | `data.net_capital_return_pct` |
| 506 | `cp.simple_return_pct` | `cp.simple_capital_return_pct` |
| 507 | `cp.annualized_return_pct` | `cp.annualized_capital_return_pct` |

#### Growth chart — API URL change (fetchGrowthData, line 314)

| Line | Current | New |
|---|---|---|
| 314 | `'/api/portfolios/' + ... + '/history'` | `'/api/portfolios/' + ... + '/timeline'` |

#### Growth chart — timeseries field renames (filterAnomalies + renderChart)

| Line | Current | New |
|---|---|---|
| 341 | `prev.total_capital` | `prev.portfolio_value` |
| 342 | `p.total_capital - prev.total_capital` / `prev.total_capital` | `p.portfolio_value - prev.portfolio_value` / `prev.portfolio_value` |
| 344 | `p.total_capital = prev.total_capital` | `p.portfolio_value = prev.portfolio_value` |
| 365 | `p.total_capital \|\| p.value` | `p.portfolio_value \|\| p.value` |
| 366 | `p.total_cost` | `p.net_equity_cost` |
| 367 | `p.net_capital_deployed` | `p.net_capital_deployed` (unchanged) |

#### Cash transactions — summary field renames (loadTransactions, lines 617–620)

| Line | Current | New |
|---|---|---|
| 618 | `summary.total_cash` | `summary.gross_cash_balance` |
| 620 | `summary.by_category` | `summary.net_cash_by_category` |

#### Holdings table filter (line 208)

`x.market_value !== 0` — **No change** (`market_value` is kept).

### 4. `pages/dashboard.html` — Template Binding Renames

| Line | Current | New |
|---|---|---|
| 137 | `pct(h.weight)` | `pct(h.portfolio_weight_pct)` |

**Kept unchanged:** `h.market_value` (line 136), `h.net_return` (line 138), `h.net_return_pct` (line 139)

### 5. `internal/server/proxy_stress_test.go` — Test Fixture Renames

All JSON fixtures in `TestDashboardCapitalPerformance_StressSuite` must update field names.

#### CapitalPerformance_UnexpectedValues test cases (lines 463–471)

Replace in all 9 test body strings:

| Current JSON Field | New JSON Field |
|---|---|
| `"total_cost"` | `"net_equity_cost"` |
| `"current_portfolio_value"` | `"equity_value"` |
| `"simple_return_pct"` | `"simple_capital_return_pct"` |
| `"annualized_return_pct"` | `"annualized_capital_return_pct"` |

> `"net_capital_deployed"` and `"transaction_count"` are **unchanged**.

#### ForceRefresh_RapidCalls test (line 599)

Same field renames in the mock response body.

### 6. Dashboard Label Assertions — Decision Required

The dashboard currently displays these labels:

**Portfolio summary row:** `TOTAL VALUE`, `NET EQUITY CAPITAL`, `AVAILABLE CASH`, `NET RETURN $`, `NET RETURN %`

**Capital summary row:** `TOTAL DEPOSITED`, `CAPITAL GAIN $`, `CAPITAL GAIN %`, `SIMPLE RETURN %`, `ANNUALIZED %`

The server field rename does NOT mandate UI label changes — labels are a portal-level concern. However, some labels now conflict with the new field semantics:

| Current Label | Underlying Field (New) | Semantic Match? | Proposed Label |
|---|---|---|---|
| `TOTAL VALUE` | `portfolio_value` | Yes | **Keep** |
| `NET EQUITY CAPITAL` | `net_equity_cost` | Yes | **Keep** |
| `AVAILABLE CASH` | `net_cash_balance` | Yes | **Keep** |
| `NET RETURN $` | `net_equity_return` | Yes | **Keep** |
| `NET RETURN %` | `net_equity_return_pct` | Yes | **Keep** |
| `TOTAL DEPOSITED` | `net_capital_deployed` | Acceptable | **Keep** |
| `CAPITAL GAIN $` | `net_capital_return` | Misleading (not tax capital gain) | **CAPITAL RETURN $** |
| `CAPITAL GAIN %` | `net_capital_return_pct` | Misleading | **CAPITAL RETURN %** |
| `SIMPLE RETURN %` | `simple_capital_return_pct` | Yes | **Keep** |
| `ANNUALIZED %` | `annualized_capital_return_pct` | Yes | **Keep** |

**Recommendation:** Rename `CAPITAL GAIN $` → `CAPITAL RETURN $` and `CAPITAL GAIN %` → `CAPITAL RETURN %` to match the new field semantics (net capital return, not realized capital gain). This affects:
- `pages/dashboard.html` lines 82, 86
- `internal/handlers/dashboard_stress_test.go` line 589
- `tests/ui/dashboard_test.go` lines 546, 557

If labels are kept as-is, no test changes needed for labels — only field access changes.

---

## API Endpoint Changes

The server is consolidating some endpoints. The portal must update any direct references.

| Change | Portal Impact | File |
|---|---|---|
| `/history` → `/timeline` | Growth chart URL | `pages/static/common.js:314` |
| `/cash-summary` removed | Not used by portal (portal uses `/cash-transactions`) | None |
| `/cash-transactions/performance` removed | Not used by portal (portal reads `capital_performance` from `/portfolios/{name}`) | None |
| `/screen` + `/screen/snipe` → `/screen/stocks` | Not used by portal frontend | None |
| `time_series` removed from `/indicators` response | Portal only reads `trend`, `rsi_signal`, `data_points` from indicators — no impact | None |

**Only 1 endpoint URL change needed:** `/history` → `/timeline` in common.js.

---

## Implementation Order

### Phase 1: Go Models (compile-time safety)

1. **Update `internal/vire/models/portfolio.go`** — Rename struct fields and JSON tags per the table above. This will produce compile errors in any Go code that references the old field names, making it easy to find all internal references.

2. **Fix compile errors** — Update any internal Go code that references the renamed struct fields (snapshot builders, growth data, etc.). The portal has minimal server-side logic — most of this is struct definitions only.

### Phase 2: JavaScript + Templates (runtime references)

3. **Update `pages/static/common.js`** — Rename all JSON field accesses per the table above. Update the `/history` → `/timeline` URL.

4. **Update `pages/dashboard.html`** — Rename `h.weight` → `h.portfolio_weight_pct` in the holdings table.

5. **(Optional) Update dashboard labels** — If renaming `CAPITAL GAIN` → `CAPITAL RETURN`, update `dashboard.html` lines 82 and 86.

### Phase 3: Tests

6. **Update `internal/server/proxy_stress_test.go`** — Rename JSON field names in all test fixture strings.

7. **Update `internal/handlers/dashboard_stress_test.go`** — Update label assertions if labels changed in Phase 2.

8. **Update `tests/ui/dashboard_test.go`** — Update label assertions if labels changed in Phase 2.

### Phase 4: Verify

9. **`go vet ./...`** — Ensure no linting issues.

10. **`go test ./...`** — Run all unit tests.

11. **UI test suite** — Run full UI test suite against the updated server.

---

## Open Questions

1. **`total_gain` / `total_gain_pct` fate** — These fields exist in portal's Portfolio, PortfolioReview, and PortfolioSnapshot structs but are NOT mentioned in the server's rename table. Are they removed? Renamed? Kept? Need server confirmation.

2. **Label rename decision** — Should `CAPITAL GAIN $` / `CAPITAL GAIN %` labels change to `CAPITAL RETURN $` / `CAPITAL RETURN %`? This is a portal-only decision but affects 3 test files.

3. **Holding `GainLoss` / `GainLossPct` / `TotalReturnValue` / `TotalReturnPct`** — These fields exist in portal's Holding struct but are not mentioned in the server rename table. Are they kept as-is? Renamed? Removed?

---

## Risk Assessment

**Low risk.** The portal is a thin proxy + UI layer:
- Go models are data transfer objects with no business logic — rename is mechanical
- JavaScript field accesses are string literals — searchable and verifiable
- Only 1 API endpoint URL changes
- Compile-time errors (Go) and runtime errors (JS) will surface any missed renames quickly
- No backward compatibility needed — portal and server deploy together

**Deployment note:** Portal and server must deploy simultaneously. The old field names will not exist in the new server response, so any portal instance running the old code against the new server will break.
