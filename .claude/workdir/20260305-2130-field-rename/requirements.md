# v0.3.166 Canonical Field Rename Migration

## Scope
- UPDATE all JSON field name references to match new v0.3.166 canonical naming
- DO NOT change internal JS property names (this.grossCashBalance, this.equityValue etc.)
- DO NOT rename `portfolio_value` or `changes.yesterday.portfolio_value` — these are UNCHANGED

## Full Rename Mapping

### PORTFOLIO LEVEL (equity)
| Old JSON Field | New JSON Field |
|---|---|
| `equity_value` | `equity_holdings_value` |
| `net_equity_cost` | `equity_holdings_cost` |
| `net_equity_return` | `equity_holdings_return` |
| `net_equity_return_pct` | `equity_holdings_return_pct` |
| `realized_equity_return` | `equity_holdings_realized` |
| `unrealized_equity_return` | `equity_holdings_unrealized` |

### PORTFOLIO LEVEL (capital)
| Old JSON Field | New JSON Field |
|---|---|
| `gross_cash_balance` | `capital_gross` |
| `net_cash_balance` | `capital_available` |

### PORTFOLIO LEVEL (portfolio)
| Old JSON Field | New JSON Field |
|---|---|
| `net_capital_return` | `portfolio_return` |
| `net_capital_return_pct` | `portfolio_return_pct` |

### PORTFOLIO LEVEL (income)
| Old JSON Field | New JSON Field |
|---|---|
| `ledger_dividend_return` | `income_dividends_received` |
| `dividend_forecast` | `income_dividends_forecast` |
| `dividend_return` | `income_dividends_navexa` |

### HOLDING LEVEL
| Old JSON Field | New JSON Field |
|---|---|
| `market_value` | `holding_value_market` |
| `avg_cost` | `holding_cost_avg` |
| `portfolio_weight_pct` | `holding_weight_pct` |
| `net_return` | `holding_return_net` |
| `net_return_pct` | `holding_return_net_pct` |

### CAPITAL PERFORMANCE
| Old JSON Field | New JSON Field |
|---|---|
| `gross_capital_deposited` | `capital_contributions_gross` |
| `gross_capital_withdrawn` | `capital_withdrawals_gross` |
| `net_capital_deployed` | `capital_contributions_net` |
| `simple_capital_return_pct` | `capital_return_simple_pct` |
| `annualized_capital_return_pct` | `capital_return_xirr_pct` |

### TIMELINE/GROWTH (within data_points arrays)
| Old JSON Field | New JSON Field |
|---|---|
| `equity_value` | `equity_holdings_value` |
| `net_equity_cost` | `equity_holdings_cost` |
| `net_capital_deployed` | `capital_contributions_net` |
| `cumulative_dividend_return` | `income_dividends_cumulative` |

### PERIOD CHANGES (within changes.yesterday/week/month)
| Old JSON Field | New JSON Field |
|---|---|
| `equity_value` | `equity_holdings_value` |
| `gross_cash` | `capital_gross` |
| `dividend` | `income_dividends` (not currently used in portal) |

### CASH TRANSACTIONS (summary context)
| Old JSON Field | New JSON Field |
|---|---|
| `gross_cash_balance` | `capital_gross` |

---

## File-by-File Changes

### 1. pages/static/common.js

**loadPortfolio() — portfolio field parsing:**
- `holdingsData.dividend_forecast` → `holdingsData.income_dividends_forecast`
- `holdingsData.ledger_dividend_return` → `holdingsData.income_dividends_received`
- `changes.yesterday?.gross_cash` → `changes.yesterday?.capital_gross` (and week/month)
- `changes.yesterday?.equity_value` → `changes.yesterday?.equity_holdings_value` (and week/month, both raw_change and pct_change)
- `holdingsData.net_equity_return` → `holdingsData.equity_holdings_return`
- `holdingsData.net_equity_return_pct` → `holdingsData.equity_holdings_return_pct`
- `holdingsData.net_equity_cost` → `holdingsData.equity_holdings_cost`
- `holdingsData.equity_value` → `holdingsData.equity_holdings_value`
- `holdingsData.gross_cash_balance` → `holdingsData.capital_gross`
- `holdingsData.net_cash_balance` → `holdingsData.capital_available`
- `cp.net_capital_deployed` → `cp.capital_contributions_net`
- `cp.gross_capital_deposited` → `cp.capital_contributions_gross`

**refreshPortfolio():** Mirror ALL changes from loadPortfolio() above (same fields, different variable names: `data.*` instead of `holdingsData.*`).

**filteredHoldings getter:**
- `x.market_value` → `x.holding_value_market`

**renderChart():**
- `p.equity_value` → `p.equity_holdings_value`
- `p.net_capital_deployed` → `p.capital_contributions_net`

**loadTransactions():**
- `summary.gross_cash_balance` → `summary.capital_gross`

### 2. pages/dashboard.html

**Glossary tooltip bindings (6 changes):**
- `glossaryDef('gross_cash_balance')` → `glossaryDef('capital_gross')`
- `glossaryDef('net_cash_balance')` → `glossaryDef('capital_available')`
- `glossaryDef('equity_value')` → `glossaryDef('equity_holdings_value')`
- `glossaryDef('net_equity_return')` → `glossaryDef('equity_holdings_return')`
- `glossaryDef('net_equity_return_pct')` → `glossaryDef('equity_holdings_return_pct')`
- `glossaryDef('dividend_forecast')` → `glossaryDef('income_dividends_forecast')`

**Holdings table bindings (6 changes):**
- `h.market_value` → `h.holding_value_market`
- `h.portfolio_weight_pct` → `h.holding_weight_pct`
- `h.net_return` (2 occurrences: x-text and :class) → `h.holding_return_net`
- `h.net_return_pct` (2 occurrences: x-text and :class) → `h.holding_return_net_pct`

### 3. internal/vire/models/portfolio.go

**Portfolio struct:**
- `NetEquityCost float64 json:"net_equity_cost"` → `EquityHoldingsCost float64 json:"equity_holdings_cost"`
- `NetEquityReturn float64 json:"net_equity_return"` → `EquityHoldingsReturn float64 json:"equity_holdings_return"`
- `NetEquityReturnPct float64 json:"net_equity_return_pct"` → `EquityHoldingsReturnPct float64 json:"equity_holdings_return_pct"`
- `GrossCashBalance float64 json:"gross_cash_balance,omitempty"` → `CapitalGross float64 json:"capital_gross,omitempty"`
- `NetCashBalance float64 json:"net_cash_balance,omitempty"` → `CapitalAvailable float64 json:"capital_available,omitempty"`
- `NetCapitalReturn float64 json:"net_capital_return,omitempty"` → `PortfolioReturn float64 json:"portfolio_return,omitempty"`
- `NetCapitalReturnPct float64 json:"net_capital_return_pct,omitempty"` → `PortfolioReturnPct float64 json:"portfolio_return_pct,omitempty"`

**Holding struct:**
- `AvgCost float64 json:"avg_cost"` → `HoldingCostAvg float64 json:"holding_cost_avg"`
- `MarketValue float64 json:"market_value"` → `HoldingValueMarket float64 json:"holding_value_market"`
- `PortfolioWeightPct float64 json:"portfolio_weight_pct"` → `HoldingWeightPct float64 json:"holding_weight_pct"`
- `DividendReturn float64 json:"dividend_return"` → `IncomeDividendsNavexa float64 json:"income_dividends_navexa"`
- `AnnualizedCapitalReturnPct float64 json:"annualized_capital_return_pct"` → `CapitalReturnXirrPct float64 json:"capital_return_xirr_pct"`
- `NetReturn float64 json:"net_return"` → `HoldingReturnNet float64 json:"holding_return_net"`
- `NetReturnPct float64 json:"net_return_pct"` → `HoldingReturnNetPct float64 json:"holding_return_net_pct"`

**PortfolioReview struct:**
- `NetEquityCost float64 json:"net_equity_cost"` → `EquityHoldingsCost float64 json:"equity_holdings_cost"`

**PortfolioSnapshot struct:**
- `EquityValue float64` → `EquityHoldingsValue float64`
- `NetEquityCost float64` → `EquityHoldingsCost float64`

**SnapshotHolding struct:**
- `AvgCost` → `HoldingCostAvg`
- `MarketValue` → `HoldingValueMarket`
- `PortfolioWeightPct` → `HoldingWeightPct`

**GrowthDataPoint struct:**
- `EquityValue float64` → `EquityHoldingsValue float64`
- `NetEquityCost float64` → `EquityHoldingsCost float64`

### 4. internal/vire/models/navexa.go

**NavexaHolding struct:**
- `AvgCost float64 json:"avg_cost"` → `HoldingCostAvg float64 json:"holding_cost_avg"`
- `MarketValue float64 json:"market_value"` → `HoldingValueMarket float64 json:"holding_value_market"`
- `DividendReturn float64 json:"dividend_return"` → `IncomeDividendsNavexa float64 json:"income_dividends_navexa"`

### 5. internal/handlers/dashboard_stress_test.go

**TestDashboardHandler_StressSummaryGainColorBindings:**
- `h.net_return` → `h.holding_return_net`
- `h.net_return_pct` → `h.holding_return_net_pct`

**TestDashboardHandler_StressGlossaryTooltipBindings:**
- `glossaryDef('gross_cash_balance')` → `glossaryDef('capital_gross')`
- `glossaryDef('net_cash_balance')` → `glossaryDef('capital_available')`
- `glossaryDef('equity_value')` → `glossaryDef('equity_holdings_value')`
- `glossaryDef('net_equity_return')` → `glossaryDef('equity_holdings_return')`
- `glossaryDef('net_equity_return_pct')` → `glossaryDef('equity_holdings_return_pct')`
- `glossaryDef('dividend_forecast')` → `glossaryDef('income_dividends_forecast')`

### 6. internal/server/proxy_stress_test.go

Update all JSON fixture strings (lines 463-471, 599):
- `"net_equity_cost"` → `"equity_holdings_cost"`
- `"net_capital_deployed"` → `"capital_contributions_net"`
- `"equity_value"` → `"equity_holdings_value"`
- `"simple_capital_return_pct"` → `"capital_return_simple_pct"`
- `"annualized_capital_return_pct"` → `"capital_return_xirr_pct"`

---

## After all changes
1. `go vet ./...` — verify no compile errors
2. `go test ./...` — verify all tests pass
3. Grep for any remaining old field names to catch missed references
