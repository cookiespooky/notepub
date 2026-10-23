package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cookiespooky/notepub/internal/audit"
	"github.com/cookiespooky/notepub/internal/config"
)

func newAuditFlagSet() (*flag.FlagSet, *string, *string, *string, *string, *bool) {
	fs := flag.NewFlagSet("audit", flag.ContinueOnError)
	configPath := fs.String("config", "", "Path to config.yaml (base_url is read from it)")
	distDir := fs.String("dist", "", "Built site to check (default: ./dist)")
	baseURL := fs.String("base-url", "", "Address the site is served at (overrides config)")
	format := fs.String("format", "text", "Output format: text|json")
	strict := fs.Bool("strict", false, "Fail on warnings as well as errors")
	return fs, configPath, distDir, baseURL, format, strict
}

func newAuditUsageWriter(fs *flag.FlagSet) func(io.Writer) {
	return func(w io.Writer) {
		fs.SetOutput(w)
		fmt.Fprintln(w, "notepub audit [--dist ./dist] [--base-url https://example.com/]")
		fmt.Fprintln(w, "Checks the built site against an SEO checklist, without network:")
		fmt.Fprintln(w, "titles, descriptions, headings, canonical, OpenGraph and the og:image size, favicon,")
		fmt.Fprintln(w, "viewport, JSON-LD, alt, links and images that lead nowhere or only redirect,")
		fmt.Fprintln(w, "orphan pages, robots.txt, sitemap, llms.txt, 404.html, a counter.")
		fs.PrintDefaults()
	}
}

func auditCmd(args []string) error {
	fs, configPath, distDir, baseURL, format, strict := newAuditFlagSet()
	helped, err := parseFlags(fs, args, newAuditUsageWriter(fs))
	if err != nil || helped {
		return err
	}

	base := strings.TrimSpace(*baseURL)
	if base == "" {
		cfgPath := resolveConfigPath(*configPath)
		cfg, err := config.Load(cfgPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("config file not found: %s (or pass --base-url)", cfgPath)
			}
			return fmt.Errorf("load config: %w", err)
		}
		base = cfg.Site.BaseURL
	}
	dist := strings.TrimSpace(*distDir)
	if dist == "" {
		dist = "./dist"
	}

	report, err := audit.Run(audit.Options{Dist: dist, BaseURL: base})
	if err != nil {
		return fmt.Errorf("audit: %w", err)
	}
	errs, warns, infos := report.Count("error"), report.Count("warning"), report.Count("info")

	switch strings.ToLower(strings.TrimSpace(*format)) {
	case "", "text":
		for _, f := range report.Findings {
			where := f.Path
			if where == "" {
				where = "(site)"
			}
			fmt.Fprintf(os.Stdout, "[%s] %s %s %s\n", strings.ToUpper(f.Severity), f.Code, where, f.Message)
		}
		fmt.Fprintf(os.Stdout, "\naudit: %d page(s), %d error(s), %d warning(s), %d info\n", report.Pages, errs, warns, infos)
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(struct {
			audit.Report
			Errors   int `json:"errors"`
			Warnings int `json:"warnings"`
		}{report, errs, warns}); err != nil {
			return fmt.Errorf("audit output: %w", err)
		}
	default:
		return usageError(fmt.Sprintf("unsupported format %q (use text or json)", *format), newAuditUsageWriter(fs))
	}

	if errs > 0 {
		return fmt.Errorf("audit failed: %d error(s)", errs)
	}
	if *strict && warns > 0 {
		return fmt.Errorf("audit strict failed: %d warning(s)", warns)
	}
	return nil
}
