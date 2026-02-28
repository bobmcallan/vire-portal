# API Legacy Removal — Vire Server v0.3.107

**Server commit:** `c3088e6`
**Date:** 2026-02-28
**Portal status:** Cleaned up in v0.2.68

## Summary

Three categories of legacy code were removed from the Vire server API. Portal models have been updated to match.

## 1. FilingsIntelligence Removed

The `FilingsIntelligence` struct and all associated types have been removed from the server. This data is fully replaced by `FilingSummaries` + `CompanyTimeline` (the 3-layer assessment system).

### API response changes

The following JSON fields **no longer appear** in API responses:

| Endpoint | Removed field |
|----------|---------------|
| `get_portfolio` (holding reviews) | `filings_intelligence` |
| `get_stock_data` (market data) | `filings_intelligence` |

The replacement fields (`filing_summaries`, `timeline`) are unchanged and continue to be populated.

### Portal model changes

**`internal/vire/models/market.go`** — removed:
- `FilingsIntelligence` field from `MarketData` struct
- `FilingsIntelligence` struct definition
- `FilingMetric` struct definition
- `YearOverYearEntry` struct definition

**`internal/vire/models/portfolio.go`** — removed:
- `FilingsIntelligence` field from `HoldingReview` struct

### Removed JSON fields

```
filings_intelligence.summary
filings_intelligence.financial_health
filings_intelligence.growth_outlook
filings_intelligence.growth_rationale
filings_intelligence.key_metrics[]
filings_intelligence.year_over_year[]
filings_intelligence.risk_factors[]
filings_intelligence.positive_factors[]
filings_intelligence.filings_analyzed
filings_intelligence.generated_at
```

### Replacement fields (unchanged)

```
filing_summaries[]          — per-filing analysis (headline, type, summary)
timeline                    — structured company history (business_model, events)
```

## 2. Cash Flow Migration Endpoint Removed

| Removed | Detail |
|---------|--------|
| `POST /api/admin/migrate-cashflow` | One-time migration endpoint — no longer needed |

No portal impact. The portal does not use this endpoint.

## 3. Config Field Cleanup

| Removed | Detail |
|---------|--------|
| `JobManagerConfig.Interval` | Use `watcher_interval` instead |
| `FileConfig` struct | Was unused compatibility shim |

Server-internal only. No portal impact.
