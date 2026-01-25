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

	"github.com/cookiespooky/notepub/internal/config"
	"github.com/cookiespooky/notepub/internal/indexer"
	"github.com/cookiespooky/notepub/internal/models"
	"github.com/cookiespooky/notepub/internal/rules"
	"github.com/cookiespooky/notepub/internal/s3util"
	"github.com/cookiespooky/notepub/internal/serve"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 15 * time.Second
	idleTimeout       = 60 * time.Second
	version           = "dev"
)

var ErrUsage = errors.New("usage error")

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
	cache := serve.NewHtmlCache(cfg.Paths.CacheRoot, cfg.Theme.Name)
	themeDir := filepath.Join(cfg.Theme.Dir, cfg.Theme.Name)
	theme, err := serve.LoadTheme(themeDir, cfg.Theme.TemplatesSubdir, cfg.Theme.AssetsSubdir)
	if err != nil {
		return fmt.Errorf("load theme: %w", err)
	}
	log.Printf("theme loaded: path=%s fallback=%t", themeDir, theme.UsedFallback())

	client, err := s3util.NewClient(context.Background(), s3util.Config{
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

	srv := serve.New(cfg, store, cache, theme, client, rulesCfg)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	listenAddr := cfg.Server.Listen
	if *addr != "" {
		listenAddr = *addr
	}
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
	fs, configPath, rulesPath, resolvePath, validateLinks := newValidateFlagSet()
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
	} else if *validateLinks {
		return fmt.Errorf("link validation: resolve.json not found (use --resolve)")
	}
	log.Println("validate completed")
	return nil
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
		fs, _, _, _, _ := newValidateFlagSet()
		newValidateUsageWriter(fs)(os.Stdout)
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

func newValidateFlagSet() (*flag.FlagSet, *string, *string, *string, *bool) {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	configPath := fs.String("config", "", "Path to config.yaml")
	rulesPath := fs.String("rules", "", "Path to rules.yaml (overrides config)")
	resolvePath := fs.String("resolve", "", "Path to resolve.json (optional)")
	validateLinks := fs.Bool("links", false, "Validate links using resolve.json")
	return fs, configPath, rulesPath, resolvePath, validateLinks
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
