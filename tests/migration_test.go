package tests

// Migration validation tests for the rename + vire-mcp migration.
//
// These tests verify:
// 1. cmd/vire-portal builds (renamed from cmd/portal)
// 2. cmd/vire-mcp builds with rewritten import paths
// 3. internal/vire/ packages compile and are importable
// 4. Key types from migrated packages are accessible
//
// These tests will FAIL until the migration is complete.

import (
	"testing"

	// Verify internal/vire/common is importable and compiles
	virecommon "github.com/bobmcallan/vire-portal/internal/vire/common"

	// Verify internal/vire/models is importable and compiles
	viremodels "github.com/bobmcallan/vire-portal/internal/vire/models"

	// Verify internal/vire/interfaces is importable and compiles
	_ "github.com/bobmcallan/vire-portal/internal/vire/interfaces"
)

// TestVireCommonPackageCompiles verifies the migrated common package is accessible.
func TestVireCommonPackageCompiles(t *testing.T) {
	// Verify key functions exist and are callable
	_ = virecommon.FormatMoney(1234.56)
	_ = virecommon.FormatSignedMoney(1234.56)
	_ = virecommon.FormatSignedPct(1.5)
	_ = virecommon.FormatMarketCap(1e9)
	_ = virecommon.FormatMoneyWithCurrency(100.0, "AUD")
	_ = virecommon.FormatSignedMoneyWithCurrency(100.0, "USD")

	// Verify freshness constants exist
	_ = virecommon.FreshnessRealTimeQuote

	// Verify IsFresh function exists
	_ = virecommon.IsFresh

	// Verify version functions exist
	_ = virecommon.GetVersion()
	_ = virecommon.GetFullVersion()

	// Verify logger creation works
	logger := virecommon.NewLoggerFromConfig(virecommon.LoggingConfig{
		Level:   "error",
		Outputs: []string{"console"},
	})
	if logger == nil {
		t.Fatal("NewLoggerFromConfig returned nil")
	}
}

// TestVireModelsPackageCompiles verifies the migrated models package is accessible.
func TestVireModelsPackageCompiles(t *testing.T) {
	// Verify key types used by vire-mcp exist and can be instantiated
	_ = viremodels.RealTimeQuote{}
	_ = viremodels.Portfolio{}
	_ = viremodels.Holding{}
	_ = viremodels.HoldingReview{}
	_ = viremodels.PortfolioReview{}
	_ = viremodels.PortfolioStrategy{}
	_ = viremodels.PortfolioSnapshot{}
	_ = viremodels.GrowthDataPoint{}
	_ = viremodels.SnipeBuy{}
	_ = viremodels.ScreenCandidate{}
	_ = viremodels.StockData{}
	_ = viremodels.TickerSignals{}
	_ = viremodels.FunnelResult{}
	_ = viremodels.FunnelStage{}
	_ = viremodels.SearchRecord{}
	_ = viremodels.PortfolioPlan{}
	_ = viremodels.PlanItem{}
	_ = viremodels.PortfolioWatchlist{}
	_ = viremodels.Fundamentals{}
	_ = viremodels.NavexaTrade{}
	_ = viremodels.CompanyTimeline{}
	_ = viremodels.FilingSummary{}

	// Verify compliance constants exist
	_ = viremodels.ComplianceStatusCompliant
	_ = viremodels.ComplianceStatusNonCompliant
}

// TestVireCommonConfigTypes verifies config types from common package.
func TestVireCommonConfigTypes(t *testing.T) {
	// Verify Config struct and LoadConfig function exist
	cfg := virecommon.NewDefaultConfig()
	if cfg == nil {
		t.Fatal("NewDefaultConfig returned nil")
	}

	// Verify default values
	if cfg.Server.Port != 4242 {
		t.Errorf("Expected default port 4242, got %d", cfg.Server.Port)
	}
	if cfg.Environment != "development" {
		t.Errorf("Expected default environment 'development', got %q", cfg.Environment)
	}
}

// TestVireCommonFormatFunctions verifies format functions produce expected output.
func TestVireCommonFormatFunctions(t *testing.T) {
	tests := []struct {
		name     string
		got      string
		contains string
	}{
		{"FormatMoney positive", virecommon.FormatMoney(1234.56), "$1,234.56"},
		{"FormatMoney zero", virecommon.FormatMoney(0), "$0.00"},
		{"FormatSignedMoney positive", virecommon.FormatSignedMoney(100), "+$100.00"},
		{"FormatSignedMoney negative", virecommon.FormatSignedMoney(-50), "-$50.00"},
		{"FormatSignedPct positive", virecommon.FormatSignedPct(1.5), "+1.50%"},
		{"FormatSignedPct negative", virecommon.FormatSignedPct(-2.3), "-2.30%"},
		{"FormatMarketCap billions", virecommon.FormatMarketCap(5e9), "$5.00B"},
		{"FormatMarketCap millions", virecommon.FormatMarketCap(500e6), "$500.00M"},
		{"FormatMoneyWithCurrency AUD", virecommon.FormatMoneyWithCurrency(100, "AUD"), "A$100.00"},
		{"FormatMoneyWithCurrency USD", virecommon.FormatMoneyWithCurrency(100, "USD"), "US$100.00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.contains {
				t.Errorf("got %q, want %q", tt.got, tt.contains)
			}
		})
	}
}
