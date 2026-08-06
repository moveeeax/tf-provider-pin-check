// Command tf-provider-pin-check scans Terraform files and fails when any
// provider or module reference is not pinned to an explicit version.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/moveeeax/tf-provider-pin-check/internal/config"
	"github.com/moveeeax/tf-provider-pin-check/internal/scanner"
)

// version is stamped at build time via -ldflags; defaults to "dev".
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("tf-provider-pin-check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		cfgPath    = fs.String("config", "", "path to exceptions file (default: .tf-pin-check.yml in the target)")
		format     = fs.String("format", "text", "output format: text or json")
		strict     = fs.Bool("strict", false, "reject the pessimistic ~> operator; require exact versions")
		quiet      = fs.Bool("quiet", false, "print nothing on success")
		showVer    = fs.Bool("version", false, "print version and exit")
		usageError = fmt.Sprintf("usage: tf-provider-pin-check [flags] [path ...]\n")
	)
	fs.Usage = func() {
		fmt.Fprint(stderr, usageError)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *showVer {
		fmt.Fprintln(stdout, version)
		return 0
	}

	paths := fs.Args()
	if len(paths) == 0 {
		paths = []string{"."}
	}

	var all []scanner.Finding
	for _, root := range paths {
		cfg, err := config.Load(*cfgPath, root)
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 2
		}
		cfg.Strict = cfg.Strict || *strict
		findings, err := scanner.New(root, cfg).Scan()
		if err != nil {
			// Parse errors are reported but do not mask real findings.
			fmt.Fprintln(stderr, "warning:", err)
		}
		all = append(all, findings...)
	}

	if len(all) == 0 {
		if !*quiet && *format == "text" {
			fmt.Fprintln(stdout, "ok: all provider and module references are pinned")
		} else if *format == "json" {
			fmt.Fprintln(stdout, "[]")
		}
		return 0
	}

	switch *format {
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(all); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 2
		}
	default:
		for _, f := range all {
			fmt.Fprintln(stdout, f.String())
		}
		fmt.Fprintf(stderr, "\n%d unpinned reference(s) found\n", len(all))
	}
	return 1
}
