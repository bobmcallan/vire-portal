//go:build ignore

package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/bobmcallan/vire-portal/tests/common"
)

func main() {
	url := flag.String("url", "", "URL to capture (required)")
	out := flag.String("out", "", "Output path for screenshot (required)")
	login := flag.Bool("login", false, "Login before capture")
	flag.Parse()

	if *url == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "ERROR: -url and -out are required")
		os.Exit(2)
	}

	ctx, cancel := common.NewBrowserContext(common.DefaultBrowserConfig())
	defer cancel()

	if *login {
		if err := common.LoginAndNavigate(ctx, *url, 1000); err != nil {
			fmt.Fprintf(os.Stderr, "FAIL: %v\n", err)
			os.Exit(1)
		}
	} else {
		if err := common.NavigateAndWait(ctx, *url, 1000); err != nil {
			fmt.Fprintf(os.Stderr, "FAIL: %v\n", err)
			os.Exit(1)
		}
	}

	time.Sleep(500 * time.Millisecond)

	if err := common.Screenshot(ctx, *out); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: screenshot: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("screenshot: %s\n", *out)
}
