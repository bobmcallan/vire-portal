// Package models defines data structures for Vire
package models

import (
	"strings"
	"time"
)

// eodhExchange maps Navexa exchange names (e.g. "ASX", "NYSE") to EODHD
// exchange codes (e.g. "AU", "US"). Returns "AU" for empty/unknown exchanges.
func eodhExchange(exchange string) string {
	switch strings.ToUpper(exchange) {
	case "ASX", "AU":
		return "AU"
	case "NYSE", "NASDAQ", "US", "BATS", "AMEX", "ARCA":
		return "US"
	case "LSE", "LON":
		return "LSE"
	case "":
		return "AU"
	default:
		return exchange
	}
}

// ComplianceStatus indicates whether a holding complies with the portfolio strategy
type ComplianceStatus string

const (
	ComplianceStatusCompliant    ComplianceStatus = "compliant"
	ComplianceStatusNonCompliant ComplianceStatus = "non_compliant"
	ComplianceStatusUnknown      ComplianceStatus = "unknown"
)

// ComplianceResult captures per-holding compliance with the portfolio strategy
type ComplianceResult struct {
	Status   ComplianceStatus `json:"status"`
	Reasons  []string         `json:"reasons,omitempty"`
	RuleHits []string         `json:"rule_hits,omitempty"` // which rule names triggered
}

// Portfolio represents a stock portfolio
type Portfolio struct {
	ID                      string    `json:"id"`
	Name                    string    `json:"name"`
	NavexaID                string    `json:"navexa_id,omitempty"`
	Holdings                []Holding `json:"holdings"`
	PortfolioValue          float64   `json:"portfolio_value"`
	EquityHoldingsCost      float64   `json:"equity_holdings_cost"`
	EquityHoldingsReturn    float64   `json:"equity_holdings_return"`
	EquityHoldingsReturnPct float64   `json:"equity_holdings_return_pct"`
	CapitalGross            float64   `json:"capital_gross,omitempty"`
	CapitalAvailable        float64   `json:"capital_available,omitempty"`
	PortfolioReturn         float64   `json:"portfolio_return,omitempty"`
	PortfolioReturnPct      float64   `json:"portfolio_return_pct,omitempty"`
	Currency                string    `json:"currency"`
	FXRate                  float64   `json:"fx_rate,omitempty"` // AUDUSD rate used for currency conversion at sync time
	LastSynced              time.Time `json:"last_synced"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

// Holding represents a portfolio position
type Holding struct {
	Ticker                string         `json:"ticker"`
	Exchange              string         `json:"exchange"`
	Name                  string         `json:"name"`
	Units                 float64        `json:"units"`
	HoldingCostAvg        float64        `json:"holding_cost_avg"`
	CurrentPrice          float64        `json:"current_price"`
	HoldingValueMarket    float64        `json:"holding_value_market"`
	HoldingWeightPct      float64        `json:"holding_weight_pct"` // Portfolio weight percentage
	CostBasis             float64        `json:"cost_basis"`
	GrossProceeds         float64        `json:"gross_proceeds,omitempty"`
	IncomeDividendsNavexa float64        `json:"income_dividends_navexa"`
	CapitalReturnXirrPct  float64        `json:"capital_return_xirr_pct"`
	TimeWeightedReturnPct float64        `json:"time_weighted_return_pct"`
	HoldingReturnNet      float64        `json:"holding_return_net"`
	HoldingReturnNetPct   float64        `json:"holding_return_net_pct"`
	Currency              string         `json:"currency"`          // Holding currency (AUD, USD)
	Country               string         `json:"country,omitempty"` // Domicile country ISO code (e.g. "AU", "US")
	Trades                []*NavexaTrade `json:"trades,omitempty"`
	LastUpdated           time.Time      `json:"last_updated"`
}

// EODHDTicker returns the full EODHD-format ticker (e.g. "BHP.AU", "CBOE.US").
// Maps Navexa exchange names to EODHD codes and falls back to ".AU" if empty.
func (h Holding) EODHDTicker() string {
	return h.Ticker + "." + eodhExchange(h.Exchange)
}

// PortfolioReview contains the analysis results for a portfolio
type PortfolioReview struct {
	PortfolioName         string            `json:"portfolio_name"`
	ReviewDate            time.Time         `json:"review_date"`
	PortfolioValue        float64           `json:"portfolio_value"`
	EquityHoldingsCost    float64           `json:"equity_holdings_cost"`
	PortfolioDayChange    float64           `json:"portfolio_day_change"`
	PortfolioDayChangePct float64           `json:"portfolio_day_change_pct"`
	FXRate                float64           `json:"fx_rate,omitempty"` // AUDUSD rate used for currency conversion
	HoldingReviews        []HoldingReview   `json:"holding_reviews"`
	Alerts                []Alert           `json:"alerts"`
	Summary               string            `json:"summary"` // AI-generated summary
	Recommendations       []string          `json:"recommendations"`
	PortfolioBalance      *PortfolioBalance `json:"portfolio_balance,omitempty"`
}

// PortfolioBalance contains sector/industry allocation analysis
type PortfolioBalance struct {
	SectorAllocations   []SectorAllocation `json:"sector_allocations"`
	DefensiveWeight     float64            `json:"defensive_weight"`     // % in defensive sectors
	GrowthWeight        float64            `json:"growth_weight"`        // % in growth sectors
	IncomeWeight        float64            `json:"income_weight"`        // % in high-dividend stocks
	ConcentrationRisk   string             `json:"concentration_risk"`   // low, medium, high
	DiversificationNote string             `json:"diversification_note"` // Analysis note
}

// SectorAllocation represents allocation to a sector
type SectorAllocation struct {
	Sector   string   `json:"sector"`
	Weight   float64  `json:"weight"`
	Holdings []string `json:"holdings"`
}

// HoldingReview contains the analysis for a single holding
type HoldingReview struct {
	Holding          Holding           `json:"holding"`
	Signals          *TickerSignals    `json:"signals,omitempty"`
	Fundamentals     *Fundamentals     `json:"fundamentals,omitempty"`
	OvernightMove    float64           `json:"overnight_move"`
	OvernightPct     float64           `json:"overnight_pct"`
	NewsImpact       string            `json:"news_impact,omitempty"`
	NewsIntelligence *NewsIntelligence `json:"news_intelligence,omitempty"`
	FilingSummaries  []FilingSummary   `json:"filing_summaries,omitempty"`
	Timeline         *CompanyTimeline  `json:"timeline,omitempty"`
	ActionRequired   string            `json:"action_required"` // BUY, SELL, HOLD, WATCH
	ActionReason     string            `json:"action_reason"`
	Compliance       *ComplianceResult `json:"compliance,omitempty"`
}

// Alert represents a portfolio alert
type Alert struct {
	Type     AlertType `json:"type"`
	Severity string    `json:"severity"` // high, medium, low
	Ticker   string    `json:"ticker,omitempty"`
	Message  string    `json:"message"`
	Signal   string    `json:"signal,omitempty"`
}

// PortfolioSnapshot represents the reconstructed state of a portfolio at a historical date.
// Computed on demand from trade history and EOD prices — not stored.
type PortfolioSnapshot struct {
	PortfolioName       string
	AsOfDate            time.Time
	PriceDate           time.Time // actual trading day used for prices (may differ on weekends/holidays)
	Holdings            []SnapshotHolding
	EquityHoldingsValue float64
	EquityHoldingsCost  float64
}

// SnapshotHolding represents a single position within a historical portfolio snapshot.
type SnapshotHolding struct {
	Ticker, Name                     string
	Units, HoldingCostAvg, CostBasis float64
	ClosePrice, HoldingValueMarket   float64
	HoldingWeightPct                 float64
}

// GrowthDataPoint represents a single point in the portfolio growth time series.
// Computed on demand from monthly snapshots — not stored.
type GrowthDataPoint struct {
	Date                time.Time
	EquityHoldingsValue float64
	EquityHoldingsCost  float64
	HoldingCount        int
}

// AlertType categorizes alerts
type AlertType string

const (
	AlertTypeSignal   AlertType = "signal"
	AlertTypePrice    AlertType = "price"
	AlertTypeNews     AlertType = "news"
	AlertTypeVolume   AlertType = "volume"
	AlertTypeRisk     AlertType = "risk"
	AlertTypeStrategy AlertType = "strategy"
)
