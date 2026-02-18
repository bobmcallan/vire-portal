package common

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestRunner provides standardized test execution with artifact collection.
type TestRunner struct {
	suite      string
	resultsDir string
	mu         sync.Mutex

	// Counters
	passed  int
	failed  int
	skipped int

	// Failure details for summary
	failures []string

	// Log file
	logFile *os.File
}

var (
	globalRunner     *TestRunner
	globalRunnerOnce sync.Once
)

// NewTestRunner creates or returns the global test runner for a suite.
func NewTestRunner(suite string) *TestRunner {
	globalRunnerOnce.Do(func() {
		resultsDir := InitResultsDir()

		logPath := filepath.Join(resultsDir, suite+".log")
		logFile, err := os.Create(logPath)
		if err != nil {
			panic(fmt.Sprintf("failed to create log file: %v", err))
		}

		globalRunner = &TestRunner{
			suite:      suite,
			resultsDir: resultsDir,
			logFile:    logFile,
			failures:   make([]string, 0),
		}

		// Write header
		globalRunner.logf("# UI Test Log: %s\n", time.Now().Format("2006-01-02 15:04:05"))
		globalRunner.logf("# Suite: %s\n", suite)
		globalRunner.logf("# Server: %s\n", GetTestURL())
		globalRunner.logf("\n")
	})
	return globalRunner
}

// logf writes to both stdout and the log file.
func (r *TestRunner) logf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Print(msg)
	if r.logFile != nil {
		r.logFile.WriteString(msg)
	}
}

// RunTest executes a test function with automatic artifact collection.
// It returns the context and cancel function for the browser.
func (r *TestRunner) RunTest(t *testing.T, name string, testFn func(ctx context.Context) error) {
	ctx, cancel := NewBrowserContext(DefaultBrowserConfig())
	defer cancel()

	start := time.Now()
	r.logf("=== RUN   %s\n", name)

	err := testFn(ctx)
	duration := time.Since(start)

	if err != nil {
		r.logf("--- FAIL: %s (%.2fs)\n    %s\n", name, duration.Seconds(), err.Error())
		r.mu.Lock()
		r.failed++
		r.failures = append(r.failures, fmt.Sprintf("- **%s**: %s", name, err.Error()))
		r.mu.Unlock()

		// Take failure screenshot
		r.takeScreenshot(ctx, name+"_FAIL")
		t.Error(err.Error())
	} else {
		r.logf("--- PASS: %s (%.2fs)\n", name, duration.Seconds())
		r.mu.Lock()
		r.passed++
		r.mu.Unlock()
	}
}

// RunTestWithSkip executes a test that may skip.
func (r *TestRunner) RunTestWithSkip(t *testing.T, name string, testFn func(ctx context.Context) (skip bool, err error)) {
	ctx, cancel := NewBrowserContext(DefaultBrowserConfig())
	defer cancel()

	start := time.Now()
	r.logf("=== RUN   %s\n", name)

	skip, err := testFn(ctx)
	duration := time.Since(start)

	if skip {
		r.logf("--- SKIP: %s (%.2fs)\n", name, duration.Seconds())
		r.mu.Lock()
		r.skipped++
		r.mu.Unlock()
		t.SkipNow()
	} else if err != nil {
		r.logf("--- FAIL: %s (%.2fs)\n    %s\n", name, duration.Seconds(), err.Error())
		r.mu.Lock()
		r.failed++
		r.failures = append(r.failures, fmt.Sprintf("- **%s**: %s", name, err.Error()))
		r.mu.Unlock()

		// Take failure screenshot
		r.takeScreenshot(ctx, name+"_FAIL")
		t.Error(err.Error())
	} else {
		r.logf("--- PASS: %s (%.2fs)\n", name, duration.Seconds())
		r.mu.Lock()
		r.passed++
		r.mu.Unlock()
	}
}

// takeScreenshot captures a screenshot with the given name.
func (r *TestRunner) takeScreenshot(ctx context.Context, name string) {
	safeName := strings.ReplaceAll(name, " ", "_")
	safeName = strings.ReplaceAll(safeName, "/", "_")
	path := filepath.Join(r.resultsDir, safeName+".png")

	if err := Screenshot(ctx, path); err != nil {
		r.logf("    screenshot failed: %v\n", err)
	} else {
		r.logf("    screenshot: %s\n", path)
	}
}

// Finalize writes the summary and closes the log file.
// This should be called after all tests complete.
func (r *TestRunner) Finalize() {
	if r.logFile != nil {
		r.logFile.Close()
		r.logFile = nil
	}

	r.writeSummary()
}

// writeSummary generates the summary.md file.
func (r *TestRunner) writeSummary() {
	summaryPath := filepath.Join(r.resultsDir, "summary.md")

	f, err := os.Create(summaryPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create summary: %v\n", err)
		return
	}
	defer f.Close()

	status := "PASS"
	if r.failed > 0 {
		status = "FAIL"
	}

	fmt.Fprintf(f, "# Test Summary: %s\n\n", r.suite)
	fmt.Fprintf(f, "**Status:** %s\n", status)
	fmt.Fprintf(f, "**Timestamp:** %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(f, "**Server:** %s\n\n", GetTestURL())

	fmt.Fprintf(f, "| Metric | Count |\n")
	fmt.Fprintf(f, "|--------|-------|\n")
	fmt.Fprintf(f, "| Passed | %d |\n", r.passed)
	fmt.Fprintf(f, "| Failed | %d |\n", r.failed)
	fmt.Fprintf(f, "| Skipped | %d |\n\n", r.skipped)

	// List screenshots
	fmt.Fprintf(f, "## Artifacts\n\n")
	fmt.Fprintf(f, "Log: `%s.log`\n\n", r.suite)

	// Find screenshots
	screenshots := []string{}
	entries, _ := os.ReadDir(r.resultsDir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".png") {
			screenshots = append(screenshots, e.Name())
		}
	}
	if len(screenshots) > 0 {
		fmt.Fprintf(f, "### Screenshots\n\n")
		for _, s := range screenshots {
			fmt.Fprintf(f, "- `%s`\n", s)
		}
		fmt.Fprintf(f, "\n")
	}

	// Failure details
	if len(r.failures) > 0 {
		fmt.Fprintf(f, "## Failures\n\n")
		for _, failure := range r.failures {
			fmt.Fprintf(f, "%s\n\n", failure)
		}
	}

	// Also write to stdout for immediate visibility
	fmt.Printf("\n---\n")
	fmt.Printf("Summary written to: %s\n", summaryPath)
}

// TeeWriter returns an io.Writer that writes to both stdout and the log file.
func (r *TestRunner) TeeWriter() io.Writer {
	return &teeWriter{logFile: r.logFile}
}

type teeWriter struct {
	logFile *os.File
}

func (t *teeWriter) Write(p []byte) (n int, err error) {
	os.Stdout.Write(p)
	if t.logFile != nil {
		t.logFile.Write(p)
	}
	return len(p), nil
}

// GetRunner returns the global test runner, creating it if necessary.
func GetRunner(suite string) *TestRunner {
	if globalRunner == nil {
		return NewTestRunner(suite)
	}
	return globalRunner
}
