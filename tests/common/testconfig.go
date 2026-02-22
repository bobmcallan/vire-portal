package common

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pelletier/go-toml/v2"
)

type TestConfig struct {
	Results struct {
		Dir string `toml:"dir"`
	} `toml:"results"`
	Server struct {
		URL string `toml:"url"`
	} `toml:"server"`
	Browser struct {
		Headless    bool `toml:"headless"`
		TimeoutSecs int  `toml:"timeout_seconds"`
	} `toml:"browser"`
}

var (
	globalConfig     *TestConfig
	globalConfigOnce sync.Once
	resultsDir       string
	resultsDirOnce   sync.Once
	logWriter        io.Writer
	logFile          *os.File
)

func LoadTestConfig() *TestConfig {
	globalConfigOnce.Do(func() {
		globalConfig = &TestConfig{}
		globalConfig.Results.Dir = "tests/results"
		globalConfig.Server.URL = "http://localhost:8883"
		globalConfig.Browser.Headless = true
		globalConfig.Browser.TimeoutSecs = 30

		configPaths := []string{
			"tests/ui/test_config.toml",
			"test_config.toml",
		}

		if wd, err := os.Getwd(); err == nil {
			if filepath.Base(wd) == "ui" {
				configPaths = append([]string{"test_config.toml"}, configPaths...)
			}
		}

		for _, path := range configPaths {
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			if err := toml.Unmarshal(data, globalConfig); err == nil {
				return
			}
		}
	})
	return globalConfig
}

func InitResultsDir() string {
	resultsDirOnce.Do(func() {
		cfg := LoadTestConfig()
		baseDir := cfg.Results.Dir

		if !filepath.IsAbs(baseDir) {
			if wd, err := os.Getwd(); err == nil {
				if filepath.Base(wd) == "ui" {
					baseDir = filepath.Join("..", "..", baseDir)
				}
			}
		}

		timestamp := time.Now().Format("2006-01-02-15-04-05")
		resultsDir = filepath.Join(baseDir, timestamp)

		if err := os.MkdirAll(resultsDir, 0755); err != nil {
			panic("failed to create results dir: " + err.Error())
		}
	})
	return resultsDir
}

func GetResultsDir() string {
	// First check if wrapper script set the results directory
	if dir := os.Getenv("VIRE_TEST_RESULTS_DIR"); dir != "" {
		// Make absolute if not already
		if !filepath.IsAbs(dir) {
			if absDir, err := filepath.Abs(dir); err == nil {
				return absDir
			}
		}
		return dir
	}
	// Fall back to creating our own
	if resultsDir == "" {
		return InitResultsDir()
	}
	return resultsDir
}

func GetScreenshotDir(subdir string) string {
	dir := filepath.Join(GetResultsDir(), subdir)
	os.MkdirAll(dir, 0755)
	return dir
}

func GetTestURL() string {
	if url := os.Getenv("VIRE_TEST_URL"); url != "" {
		return url
	}
	return LoadTestConfig().Server.URL
}

func GetLogPath(suite string) string {
	return filepath.Join(GetResultsDir(), suite+".log")
}

func WriteResultsSummary(suite string, passed, failed, skipped int) {
	resultsPath := GetResultsDir()
	summaryPath := filepath.Join(resultsPath, "summary.md")

	f, err := os.OpenFile(summaryPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	status := "PASS"
	if failed > 0 {
		status = "FAIL"
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	f.WriteString(fmt.Sprintf("# Test Results: %s\n\n", timestamp))
	f.WriteString(fmt.Sprintf("## %s\n", suite))
	f.WriteString(fmt.Sprintf("- Status: %s\n", status))
	f.WriteString(fmt.Sprintf("- Passed: %d\n", passed))
	f.WriteString(fmt.Sprintf("- Failed: %d\n", failed))
	f.WriteString(fmt.Sprintf("- Skipped: %d\n", skipped))
}
