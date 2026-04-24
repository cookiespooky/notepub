package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/cookiespooky/notepub/internal/config"
	"github.com/cookiespooky/notepub/internal/indexer"
	"github.com/cookiespooky/notepub/internal/models"
	"github.com/cookiespooky/notepub/internal/rules"
	"github.com/cookiespooky/notepub/internal/s3util"
	"github.com/cookiespooky/notepub/internal/serve"
	"github.com/cookiespooky/notepub/internal/templateupdate"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 15 * time.Second
	idleTimeout       = 60 * time.Second
)

var ErrUsage = errors.New("usage error")

var version = "dev"

func main() {
	code := run()
	os.Exit(code)
}

func run() int {
	if len(os.Args) < 2 {
		err := usageError("missing command", usageWriter)
		printError(err)
		return codeFromErr(err)
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	if cmd == "-h" || cmd == "--help" || cmd == "help" {
		helpCmd(args)
		return 0
	}
	if cmd == "-v" || cmd == "--version" || cmd == "version" {
		printVersion()
		return 0
	}

	var err error
	switch cmd {
	case "index":
		err = indexCmd(args)
	case "serve":
		err = serveCmd(args)
	case "build":
		err = buildCmd(args)
	case "validate":
		err = validateCmd(args)
	case "template":
		err = templateCmd(args)
	default:
		err = usageError(fmt.Sprintf("unknown command: %s", cmd), usageWriter)
	}

	if err == nil {
		return 0
	}
	printError(err)
	return codeFromErr(err)
}

func indexCmd(args []string) error {
	fs, configPath, rulesPath := newIndexFlagSet()
	helped, err := parseFlags(fs, args, newIndexUsageWriter(fs))
	if err != nil {
		return err
	}
	if helped {
		return nil
	}

	if *rulesPath != "" {
		if _, err := validateRulesPath(*rulesPath); err != nil {
			return err
		}
	}

	configPathResolved := resolveConfigPath(*configPath)
	cfg, err := config.Load(configPathResolved)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("config file not found: %s", configPathResolved)
		}
		return fmt.Errorf("load config: %w", err)
	}
	resolvedRules, err := resolveRulesPath(configPathResolved, cfg.RulesPath, *rulesPath)
	if err != nil {
		return err
	}
	cfg.RulesPath = resolvedRules

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := indexer.Run(ctx, cfg); err != nil {
		return fmt.Errorf("index: %w", err)
	}
	log.Println("index completed")
	return nil
}

func serveCmd(args []string) error {
	fs, configPath, rulesPath, addr := newServeFlagSet()
	helped, err := parseFlags(fs, args, newServeUsageWriter(fs))
	if err != nil {
		return err
	}
	if helped {
		return nil
	}

	if *rulesPath != "" {
		if _, err := validateRulesPath(*rulesPath); err != nil {
			return err
		}
	}

	configPathResolved := resolveConfigPath(*configPath)
	cfg, err := config.Load(configPathResolved)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("config file not found: %s", configPathResolved)
		}
		return fmt.Errorf("load config: %w", err)
	}
	if *addr != "" {
		cfg.Server.Listen = *addr
		cfg.Runtime.Dev.BaseURL = ""
		cfg.Runtime.Dev.MediaBaseURL = ""
		cfg.Site.MediaBaseURL = ""
		if err := config.ApplyRuntimeURLs(&cfg); err != nil {
			return fmt.Errorf("apply serve address: %w", err)
		}
	}
	resolvedRules, err := resolveRulesPath(configPathResolved, cfg.RulesPath, *rulesPath)
	if err != nil {
		return err
	}
	cfg.RulesPath = resolvedRules

	rulesCfg, err := rules.Load(cfg.RulesPath)
	if err != nil {
		return fmt.Errorf("load rules: %w", err)
	}

	resolvePath := filepath.Join(cfg.Paths.ArtifactsDir, "resolve.json")
	store := serve.NewResolveStore(resolvePath, rulesCfg, cfg.Media.ExposeAllUnderPrefix)
	cache := serve.NewHtmlCache(cfg.Paths.CacheRoot, cfg.Theme.Name, cfg.Site.BaseURL+"|"+cfg.Site.MediaBaseURL)
	themeDir := filepath.Join(cfg.Theme.Dir, cfg.Theme.Name)
	theme, err := serve.LoadTheme(themeDir, cfg.Theme.TemplatesSubdir, cfg.Theme.AssetsSubdir)
	if err != nil {
		return fmt.Errorf("load theme: %w", err)
	}
	log.Printf("theme loaded: path=%s fallback=%t", themeDir, theme.UsedFallback())

	var client *s3.Client
	if cfg.Content.Source == "s3" {
		client, err = s3util.NewClient(context.Background(), s3util.Config{
			Endpoint:       cfg.S3.Endpoint,
			Region:         cfg.S3.Region,
			ForcePathStyle: cfg.S3.ForcePathStyle,
			Bucket:         cfg.S3.Bucket,
			Prefix:         cfg.S3.Prefix,
			AccessKey:      cfg.S3.AccessKey,
			SecretKey:      cfg.S3.SecretKey,
			Anonymous:      cfg.S3.Anonymous,
		})
		if err != nil {
			return fmt.Errorf("s3 client: %w", err)
		}
	}

	srv := serve.New(cfg, store, cache, theme, client, rulesCfg)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	listenAddr := cfg.Server.Listen
	server := &http.Server{
		Addr:              listenAddr,
		Handler:           srv.Router(),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("notepub serve listening on %s", listenAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("server error: %w", err)
		}
	}
	log.Println("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
	return nil
}

func buildCmd(args []string) error {
	fs, configPath, rulesPath, distDir, artifactsDir, noIndex, generateSearch := newBuildFlagSet()
	helped, err := parseFlags(fs, args, newBuildUsageWriter(fs))
	if err != nil {
		return err
	}
	if helped {
		return nil
	}

	if *rulesPath != "" {
		if _, err := validateRulesPath(*rulesPath); err != nil {
			return err
		}
	}

	configPathResolved := resolveConfigPath(*configPath)
	cfg, err := config.Load(configPathResolved)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("config file not found: %s", configPathResolved)
		}
		return fmt.Errorf("load config: %w", err)
	}
	resolvedRules, err := resolveRulesPath(configPathResolved, cfg.RulesPath, *rulesPath)
	if err != nil {
		return err
	}
	cfg.RulesPath = resolvedRules
	rulesCfg, err := rules.Load(cfg.RulesPath)
	if err != nil {
		return fmt.Errorf("load rules: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	opts := serve.BuildOptions{
		DistDir:        *distDir,
		ArtifactsDir:   *artifactsDir,
		NoIndex:        *noIndex,
		GenerateSearch: *generateSearch,
	}
	if err := serve.Build(ctx, cfg, rulesCfg, opts); err != nil {
		return fmt.Errorf("build: %w", err)
	}
	log.Println("build completed")
	return nil
}

func validateCmd(args []string) error {
	fs, configPath, rulesPath, resolvePath, validateLinks, validateMarkdown, markdownStrict, markdownFormat, markdownOutput := newValidateFlagSet()
	helped, err := parseFlags(fs, args, newValidateUsageWriter(fs))
	if err != nil {
		return err
	}
	if helped {
		return nil
	}

	if *rulesPath != "" {
		if _, err := validateRulesPath(*rulesPath); err != nil {
			return err
		}
	}

	configPathResolved := resolveConfigPath(*configPath)
	cfg, err := config.Load(configPathResolved)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("config file not found: %s", configPathResolved)
		}
		return fmt.Errorf("load config: %w", err)
	}
	resolvedRules, err := resolveRulesPath(configPathResolved, cfg.RulesPath, *rulesPath)
	if err != nil {
		return err
	}
	cfg.RulesPath = resolvedRules
	rulesCfg, err := rules.Load(cfg.RulesPath)
	if err != nil {
		return fmt.Errorf("load rules: %w", err)
	}
	if err := indexer.ValidateRules(rulesCfg); err != nil {
		return fmt.Errorf("rules validation: %w", err)
	}

	path := *resolvePath
	if path == "" {
		candidate := filepath.Join(cfg.Paths.ArtifactsDir, "resolve.json")
		if _, err := os.Stat(candidate); err == nil {
			path = candidate
		}
	}
	if path != "" {
		idx, err := validateResolve(path)
		if err != nil {
			return fmt.Errorf("resolve validation: %w", err)
		}
		if *validateLinks {
			if err := indexer.ValidateResolveLinks(idx, rulesCfg, cfg.S3.Prefix); err != nil {
				return fmt.Errorf("link validation: %w", err)
			}
		}
		if *validateMarkdown {
			ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()
			diags, caps, err := indexer.ValidateMarkdownWithCapabilities(ctx, cfg, idx)
			if err != nil {
				return fmt.Errorf("markdown validation: %w", err)
			}
			format := normalizeMarkdownFormat(*markdownFormat)
			if format == "" {
				return fmt.Errorf("markdown validation: unsupported markdown format %q (use text or json)", *markdownFormat)
			}
			rendered, err := renderMarkdownDiagnostics(diags, caps, format)
			if err != nil {
				return fmt.Errorf("markdown validation output: %w", err)
			}
			if err := writeMarkdownDiagnostics(rendered, *markdownOutput); err != nil {
				return fmt.Errorf("markdown validation output: %w", err)
			}
			errCount, warnCount := indexer.CountDiagnostics(diags)
			log.Printf("markdown validation: %d error(s), %d warning(s)", errCount, warnCount)
			if errCount > 0 {
				return fmt.Errorf("markdown validation failed (%d errors)", errCount)
			}
			if *markdownStrict && warnCount > 0 {
				return fmt.Errorf("markdown validation strict failed (%d warnings)", warnCount)
			}
		}
	} else if *validateLinks {
		return fmt.Errorf("link validation: resolve.json not found (use --resolve)")
	} else if *validateMarkdown {
		return fmt.Errorf("markdown validation: resolve.json not found (use --resolve or run index)")
	}
	log.Println("validate completed")
	return nil
}

func templateCmd(args []string) error {
	if len(args) == 0 {
		return usageError("missing template subcommand", templateUsageWriter)
	}
	subcmd := args[0]
	subargs := args[1:]
	switch subcmd {
	case "-h", "--help", "help":
		templateUsageWriter(os.Stdout)
		return nil
	case "check":
		fs, root := newTemplateCheckFlagSet()
		helped, err := parseFlags(fs, subargs, newTemplateCheckUsageWriter(fs))
		if err != nil {
			return err
		}
		if helped {
			return nil
		}
		report, err := templateupdate.Check(*root)
		if err != nil {
			return err
		}
		fmt.Print(report)
		return nil
	case "update":
		fs, root, apply := newTemplateUpdateFlagSet()
		helped, err := parseFlags(fs, subargs, newTemplateUpdateUsageWriter(fs))
		if err != nil {
			return err
		}
		if helped {
			return nil
		}
		report, err := templateupdate.Update(templateupdate.UpdateOptions{
			Root:  *root,
			Apply: *apply,
		})
		if err != nil {
			return err
		}
		fmt.Print(report)
		return nil
	default:
		return usageError(fmt.Sprintf("unknown template subcommand: %s", subcmd), templateUsageWriter)
	}
}

func validateResolve(path string) (models.ResolveIndex, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return models.ResolveIndex{}, fmt.Errorf("read resolve: %w", err)
	}
	var idx models.ResolveIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return models.ResolveIndex{}, fmt.Errorf("parse resolve: %w", err)
	}
	if len(idx.Routes) == 0 {
		return models.ResolveIndex{}, fmt.Errorf("resolve routes empty")
	}
	if len(idx.Meta) == 0 {
		return models.ResolveIndex{}, fmt.Errorf("resolve meta empty")
	}
	paths := make([]string, 0, len(idx.Routes))
	for p := range idx.Routes {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		route := idx.Routes[p]
		if route.Status == 200 {
			if _, ok := idx.Meta[p]; !ok {
				return models.ResolveIndex{}, fmt.Errorf("route %q missing meta", p)
			}
		}
	}
	return idx, nil
}

func usageWriter(w io.Writer) {
	fmt.Fprintln(w, "notepub index")
	fmt.Fprintln(w, "notepub serve --addr :8081")
	fmt.Fprintln(w, "notepub build --dist ./dist")
	fmt.Fprintln(w, "notepub validate")
	fmt.Fprintln(w, "notepub template check")
	fmt.Fprintln(w, "notepub template update --apply")
	fmt.Fprintln(w, "notepub version")
}

func helpCmd(args []string) {
	if len(args) == 0 {
		usageWriter(os.Stdout)
		return
	}
	switch args[0] {
	case "index":
		fs, _, _ := newIndexFlagSet()
		newIndexUsageWriter(fs)(os.Stdout)
	case "serve":
		fs, _, _, _ := newServeFlagSet()
		newServeUsageWriter(fs)(os.Stdout)
	case "build":
		fs, _, _, _, _, _, _ := newBuildFlagSet()
		newBuildUsageWriter(fs)(os.Stdout)
	case "validate":
		fs, _, _, _, _, _, _, _, _ := newValidateFlagSet()
		newValidateUsageWriter(fs)(os.Stdout)
	case "template":
		templateUsageWriter(os.Stdout)
	default:
		usageWriter(os.Stdout)
	}
}

func printVersion() {
	fmt.Println(version)
}

func newIndexFlagSet() (*flag.FlagSet, *string, *string) {
	fs := flag.NewFlagSet("index", flag.ContinueOnError)
	configPath := fs.String("config", "", "Path to config.yaml")
	rulesPath := fs.String("rules", "", "Path to rules.yaml (overrides config)")
	return fs, configPath, rulesPath
}

func newServeFlagSet() (*flag.FlagSet, *string, *string, *string) {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	configPath := fs.String("config", "", "Path to config.yaml")
	rulesPath := fs.String("rules", "", "Path to rules.yaml (overrides config)")
	addr := fs.String("addr", "", "HTTP listen address (overrides config)")
	return fs, configPath, rulesPath, addr
}

func newBuildFlagSet() (*flag.FlagSet, *string, *string, *string, *string, *bool, *bool) {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	configPath := fs.String("config", "", "Path to config.yaml")
	rulesPath := fs.String("rules", "", "Path to rules.yaml (overrides config)")
	distDir := fs.String("dist", "", "Output directory for static site")
	artifactsDir := fs.String("artifacts", "", "Artifacts directory (resolve.json, sitemap, robots)")
	noIndex := fs.Bool("no-index", false, "Do not run index if resolve.json is missing")
	generateSearch := fs.Bool("generate-search", false, "Generate search.json if missing")
	return fs, configPath, rulesPath, distDir, artifactsDir, noIndex, generateSearch
}

func newValidateFlagSet() (*flag.FlagSet, *string, *string, *string, *bool, *bool, *bool, *string, *string) {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	configPath := fs.String("config", "", "Path to config.yaml")
	rulesPath := fs.String("rules", "", "Path to rules.yaml (overrides config)")
	resolvePath := fs.String("resolve", "", "Path to resolve.json (optional)")
	validateLinks := fs.Bool("links", false, "Validate links using resolve.json")
	validateMarkdown := fs.Bool("markdown", false, "Validate markdown diagnostics (wikilinks/embeds/raw html)")
	markdownStrict := fs.Bool("markdown-strict", false, "Fail on markdown warnings as well as errors")
	markdownFormat := fs.String("markdown-format", "text", "Markdown diagnostics output format: text|json")
	markdownOutput := fs.String("output", "", "Write markdown diagnostics output to file path")
	return fs, configPath, rulesPath, resolvePath, validateLinks, validateMarkdown, markdownStrict, markdownFormat, markdownOutput
}

func newTemplateCheckFlagSet() (*flag.FlagSet, *string) {
	fs := flag.NewFlagSet("template check", flag.ContinueOnError)
	root := fs.String("root", ".", "Project root")
	return fs, root
}

func newTemplateUpdateFlagSet() (*flag.FlagSet, *string, *bool) {
	fs := flag.NewFlagSet("template update", flag.ContinueOnError)
	root := fs.String("root", ".", "Project root")
	apply := fs.Bool("apply", false, "Write changes instead of showing a dry run")
	return fs, root, apply
}

func normalizeMarkdownFormat(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "text":
		return "text"
	case "json":
		return "json"
	default:
		return ""
	}
}

func renderMarkdownDiagnostics(diags []indexer.MarkdownDiagnostic, caps indexer.MarkdownCapabilities, format string) ([]byte, error) {
	switch format {
	case "text":
		var b strings.Builder
		for _, d := range diags {
			b.WriteString(fmt.Sprintf("[%s] %s %s:%d %s\n", strings.ToUpper(d.Severity), d.Code, d.File, d.Line, d.Message))
		}
		b.WriteString("\nCapabilities:\n")
		names := capabilityNames(caps)
		for _, name := range names {
			used := caps.Used[name]
			supported := caps.Supported[name]
			b.WriteString(fmt.Sprintf("- %s: used=%t supported=%t\n", name, used, supported))
		}
		if len(caps.UnsupportedUsed) > 0 {
			b.WriteString("Unsupported used:\n")
			for _, name := range caps.UnsupportedUsed {
				b.WriteString(fmt.Sprintf("- %s\n", name))
			}
		}
		return []byte(b.String()), nil
	case "json":
		errCount, warnCount := indexer.CountDiagnostics(diags)
		payload := struct {
			Diagnostics  []indexer.MarkdownDiagnostic `json:"diagnostics"`
			Capabilities indexer.MarkdownCapabilities `json:"capabilities"`
			Summary      struct {
				Errors   int `json:"errors"`
				Warnings int `json:"warnings"`
			} `json:"summary"`
		}{
			Diagnostics:  diags,
			Capabilities: caps,
		}
		payload.Summary.Errors = errCount
		payload.Summary.Warnings = warnCount
		var b strings.Builder
		enc := json.NewEncoder(&b)
		enc.SetIndent("", "  ")
		if err := enc.Encode(payload); err != nil {
			return nil, err
		}
		return []byte(b.String()), nil
	default:
		return nil, fmt.Errorf("unsupported format %q", format)
	}
}

func capabilityNames(caps indexer.MarkdownCapabilities) []string {
	names := make([]string, 0, len(caps.Supported))
	for name := range caps.Supported {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func writeMarkdownDiagnostics(out []byte, filePath string) error {
	if strings.TrimSpace(filePath) == "" {
		_, err := os.Stdout.Write(out)
		return err
	}
	return os.WriteFile(filePath, out, 0o644)
}

func newIndexUsageWriter(fs *flag.FlagSet) func(io.Writer) {
	return func(w io.Writer) {
		fs.SetOutput(w)
		fmt.Fprintln(w, "notepub index")
		fs.PrintDefaults()
	}
}

func newServeUsageWriter(fs *flag.FlagSet) func(io.Writer) {
	return func(w io.Writer) {
		fs.SetOutput(w)
		fmt.Fprintln(w, "notepub serve --addr :8081")
		fs.PrintDefaults()
	}
}

func newBuildUsageWriter(fs *flag.FlagSet) func(io.Writer) {
	return func(w io.Writer) {
		fs.SetOutput(w)
		fmt.Fprintln(w, "notepub build --dist ./dist")
		fs.PrintDefaults()
	}
}

func newValidateUsageWriter(fs *flag.FlagSet) func(io.Writer) {
	return func(w io.Writer) {
		fs.SetOutput(w)
		fmt.Fprintln(w, "notepub validate")
		fs.PrintDefaults()
	}
}

func templateUsageWriter(w io.Writer) {
	fmt.Fprintln(w, "notepub template check")
	fmt.Fprintln(w, "notepub template update [--apply]")
}

func newTemplateCheckUsageWriter(fs *flag.FlagSet) func(io.Writer) {
	return func(w io.Writer) {
		fs.SetOutput(w)
		fmt.Fprintln(w, "notepub template check")
		fs.PrintDefaults()
	}
}

func newTemplateUpdateUsageWriter(fs *flag.FlagSet) func(io.Writer) {
	return func(w io.Writer) {
		fs.SetOutput(w)
		fmt.Fprintln(w, "notepub template update [--apply]")
		fs.PrintDefaults()
	}
}

type usageErr struct {
	msg   string
	usage func(io.Writer)
}

func (u usageErr) Error() string {
	return u.msg
}

func (u usageErr) Unwrap() error {
	return ErrUsage
}

func usageError(msg string, usage func(io.Writer)) error {
	return usageErr{msg: msg, usage: usage}
}

func parseFlags(fs *flag.FlagSet, args []string, usage func(io.Writer)) (bool, error) {
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			usage(os.Stdout)
			return true, nil
		}
		return false, usageError(normalizeFlagError(err), usage)
	}
	return false, nil
}

func normalizeFlagError(err error) string {
	msg := err.Error()
	const prefix = "flag provided but not defined: "
	if strings.HasPrefix(msg, prefix) {
		name := strings.TrimSpace(strings.TrimPrefix(msg, prefix))
		if strings.HasPrefix(name, "-") && !strings.HasPrefix(name, "--") && len(name) > 2 {
			name = "-" + name
		}
		return "unknown flag: " + name
	}
	return msg
}

func printError(err error) {
	fmt.Fprintln(os.Stderr, err.Error())
	if u, ok := err.(usageErr); ok && u.usage != nil {
		u.usage(os.Stderr)
	}
}

func codeFromErr(err error) int {
	if errors.Is(err, ErrUsage) {
		return 2
	}
	return 1
}

func resolveRulesPath(configPathFlag, cfgRulesPath, flagRulesPath string) (string, error) {
	if flagRulesPath != "" {
		return validateRulesPath(flagRulesPath)
	}
	if env := os.Getenv("RULES_PATH"); env != "" {
		return validateRulesPath(env)
	}
	if cfgRulesPath != "" {
		return validateRulesPath(cfgRulesPath)
	}
	if configPathFlag != "" {
		return validateRulesPath(filepath.Join(filepath.Dir(configPathFlag), "rules.yaml"))
	}
	return validateRulesPath("rules.yaml")
}

func resolveConfigPath(flagPath string) string {
	if flagPath != "" {
		return flagPath
	}
	if env := os.Getenv("CONFIG_PATH"); env != "" {
		return env
	}
	return "config.yaml"
}

func validateRulesPath(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("rules file not found: %s", path)
		}
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("rules path is a directory: %s", path)
	}
	return path, nil
}
