package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cookiespooky/notepub/internal/config"
	"github.com/cookiespooky/notepub/internal/scaffold"
)

type typeAddFlags struct {
	config, rules, permalink, template, from *string
	noSitemap, noSearch                      *bool
}

func newTypeAddFlagSet() (*flag.FlagSet, typeAddFlags) {
	fs := flag.NewFlagSet("type add", flag.ContinueOnError)
	f := typeAddFlags{
		config:    fs.String("config", "", "Path to config.yaml (the theme folder is read from it)"),
		rules:     fs.String("rules", "", "Path to rules.yaml (overrides config)"),
		permalink: fs.String("permalink", "", `Address pattern (default "/<name>/{{ slug }}/")`),
		template:  fs.String("template", "", `Template file (default "<name>.html")`),
		from:      fs.String("from", "", `Template to copy (default "page.html")`),
		noSitemap: fs.Bool("no-sitemap", false, "Leave pages of this type out of the sitemap"),
		noSearch:  fs.Bool("no-search", false, "Leave pages of this type out of search"),
	}
	return fs, f
}

func typeUsageWriter(fs *flag.FlagSet) func(io.Writer) {
	return func(w io.Writer) {
		fmt.Fprintln(w, "notepub type add <name> [--permalink \"/services/{{ slug }}/\"] [--from page.html]")
		fmt.Fprintln(w, "Adds a kind of page: the type in rules.yaml and its template in the theme.")
		fmt.Fprintln(w, "A note with `type: <name>` becomes a page at the permalink on the next build.")
		if fs != nil {
			fs.SetOutput(w)
			fs.PrintDefaults()
		}
	}
}

func typeCmd(args []string) error {
	if len(args) == 0 || args[0] != "add" {
		if len(args) > 0 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help") {
			typeUsageWriter(nil)(os.Stdout)
			return nil
		}
		return usageError("usage: notepub type add <name>", typeUsageWriter(nil))
	}
	args = args[1:]
	fs, f := newTypeAddFlagSet()
	// The name may come before or after the flags.
	name := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		name, args = args[0], args[1:]
	}
	helped, err := parseFlags(fs, args, typeUsageWriter(fs))
	if err != nil || helped {
		return err
	}
	if name == "" && fs.NArg() > 0 {
		name = fs.Arg(0)
	}
	if name == "" {
		return usageError("missing type name", typeUsageWriter(fs))
	}

	cfgPath := resolveConfigPath(*f.config)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("config file not found: %s", cfgPath)
		}
		return fmt.Errorf("load config: %w", err)
	}
	rulesPath, err := resolveRulesPath(cfgPath, cfg.RulesPath, *f.rules)
	if err != nil {
		return err
	}
	templatesDir := filepath.Join(cfg.Theme.Dir, cfg.Theme.Name, cfg.Theme.TemplatesSubdir)

	res, err := scaffold.AddType(scaffold.TypeOptions{
		Name:         name,
		RulesPath:    rulesPath,
		TemplatesDir: templatesDir,
		Permalink:    *f.permalink,
		Template:     *f.template,
		From:         *f.from,
		Sitemap:      !*f.noSitemap,
		Search:       !*f.noSearch,
	})
	if err != nil {
		return err
	}
	fmt.Printf("type %q added to %s, pages at %s\n", name, rulesPath, res.Permalink)
	if res.TemplateCreated {
		fmt.Printf("template %s created from %s — edit it to give the page its own layout\n", res.TemplatePath, res.TemplateSource)
	} else {
		fmt.Printf("template %s already existed and was kept\n", res.TemplatePath)
	}
	fmt.Printf("\nA note becomes such a page with this frontmatter:\n\n---\ntype: %s\nslug: my-first-%s\ntitle: \"…\"\ndescription: \"…\"\n---\n", name, name)
	return nil
}
