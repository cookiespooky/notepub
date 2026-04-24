package indexer

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/cookiespooky/notepub/internal/config"
	"github.com/cookiespooky/notepub/internal/linkutil"
	"github.com/cookiespooky/notepub/internal/localutil"
	"github.com/cookiespooky/notepub/internal/mdproc"
	"github.com/cookiespooky/notepub/internal/models"
	"github.com/cookiespooky/notepub/internal/rules"
	"github.com/cookiespooky/notepub/internal/s3util"
)

type MarkdownDiagnostic struct {
	Code     string
	Severity string
	File     string
	Line     int
	Message  string
}

type MarkdownCapabilities struct {
	Supported       map[string]bool `json:"supported"`
	Used            map[string]bool `json:"used"`
	UnsupportedUsed []string        `json:"unsupported_used,omitempty"`
}

func defaultMarkdownCapabilities() MarkdownCapabilities {
	return MarkdownCapabilities{
		Supported: map[string]bool{
			"obsidian.wikilinks":  true,
			"obsidian.embeds":     true,
			"obsidian.tags":       true,
			"obsidian.callouts":   true,
			"obsidian.footnotes":  true,
			"obsidian.math":       true,
			"raw_html":            true,
			"obsidian.block_refs": false,
		},
		Used: map[string]bool{},
	}
}

func ValidateMarkdown(ctx context.Context, cfg config.Config, idx models.ResolveIndex) ([]MarkdownDiagnostic, error) {
	diags, _, err := ValidateMarkdownWithCapabilities(ctx, cfg, idx)
	return diags, err
}

func ValidateMarkdownWithCapabilities(ctx context.Context, cfg config.Config, idx models.ResolveIndex) ([]MarkdownDiagnostic, MarkdownCapabilities, error) {
	resolver, err := buildResolverIndex(idx, cfg.S3.Prefix)
	if err != nil {
		return nil, MarkdownCapabilities{}, fmt.Errorf("build resolver index: %w", err)
	}

	var (
		objects  []s3util.Object
		s3client *s3.Client
	)
	switch cfg.Content.Source {
	case "local":
		objects, err = localutil.ListMarkdown(cfg.Content.LocalDir, cfg.S3.Prefix)
		if err != nil {
			return nil, MarkdownCapabilities{}, fmt.Errorf("list local markdown: %w", err)
		}
	case "s3":
		s3client, err = s3util.NewClient(ctx, s3util.Config{
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
			return nil, MarkdownCapabilities{}, fmt.Errorf("s3 client: %w", err)
		}
		objects, err = s3util.ListObjects(ctx, s3client, cfg.S3.Bucket, cfg.S3.Prefix)
		if err != nil {
			return nil, MarkdownCapabilities{}, fmt.Errorf("list s3 objects: %w", err)
		}
	default:
		return nil, MarkdownCapabilities{}, fmt.Errorf("unsupported content source: %s", cfg.Content.Source)
	}

	keys := make([]string, 0, len(objects))
	for _, obj := range objects {
		if !strings.HasSuffix(strings.ToLower(obj.Key), ".md") {
			continue
		}
		keys = append(keys, obj.Key)
	}
	sort.Strings(keys)

	rule := rules.ResolveRule{
		Order:     []string{"path", "filename", "slug"},
		Ambiguity: "error",
		Missing:   "error",
		Case:      "insensitive",
	}

	diagnostics := make([]MarkdownDiagnostic, 0)
	capabilities := defaultMarkdownCapabilities()
	for _, key := range keys {
		var body []byte
		switch cfg.Content.Source {
		case "local":
			body, err = localutil.FetchObject(cfg.Content.LocalDir, key)
		case "s3":
			body, err = s3util.FetchObject(ctx, s3client, cfg.S3.Bucket, key)
		}
		if err != nil {
			diagnostics = append(diagnostics, MarkdownDiagnostic{
				Code:     "NP-MD-READ-ERROR",
				Severity: "error",
				File:     key,
				Line:     1,
				Message:  err.Error(),
			})
			continue
		}
		_, content, err := parseFrontmatter(body)
		if err != nil {
			diagnostics = append(diagnostics, MarkdownDiagnostic{
				Code:     "NP-MD-FRONTMATTER-ERROR",
				Severity: "error",
				File:     key,
				Line:     1,
				Message:  err.Error(),
			})
			continue
		}
		mergeCapabilities(&capabilities, detectMarkdownCapabilities(string(content)))
		diagnostics = append(diagnostics, diagnoseMarkdownContent(key, string(content), resolver, rule, cfg.Markdown.HTMLPolicy)...)
	}
	capabilities.UnsupportedUsed = buildUnsupportedList(capabilities)
	return diagnostics, capabilities, nil
}

func diagnoseMarkdownContent(fileKey string, markdown string, res resolverIndex, rule rules.ResolveRule, htmlPolicy string) []MarkdownDiagnostic {
	lines := strings.Split(mdproc.MaskCodeWithSpaces(mdproc.NormalizeLineEndings(markdown)), "\n")
	out := make([]MarkdownDiagnostic, 0)
	htmlWarnedLine := map[int]struct{}{}
	policy := strings.ToLower(strings.TrimSpace(htmlPolicy))
	if policy == "" {
		policy = "safe"
	}

	for i, line := range lines {
		lineNo := i + 1
		for _, match := range wikiRefLineRe.FindAllStringSubmatch(line, -1) {
			if len(match) < 2 {
				continue
			}
			raw := strings.TrimSpace(match[0])
			inner := strings.TrimSpace(match[1])
			if inner == "" {
				continue
			}
			isEmbed := strings.HasPrefix(raw, "![[")
			targetPart := inner
			if segs := strings.SplitN(inner, "|", 2); len(segs) > 0 {
				targetPart = strings.TrimSpace(segs[0])
			}
			embedTarget := stripWikiAnchor(targetPart)
			if isEmbed && isMediaTarget(embedTarget) {
				continue
			}
			if strings.Contains(targetPart, "#^") {
				out = append(out, MarkdownDiagnostic{
					Code:     "NP-OBSIDIAN-UNSUPPORTED",
					Severity: "warn",
					File:     fileKey,
					Line:     lineNo,
					Message:  fmt.Sprintf("%s: block reference anchors are not fully supported", raw),
				})
			}

			_, _, err := ResolveLink(inner, "wikimap", rule, res)
			if err == nil {
				continue
			}
			msg := err.Error()
			code := "NP-MD-WIKI-UNRESOLVED"
			severity := "warn"
			if strings.Contains(msg, "ambiguous") {
				code = "NP-MD-WIKI-AMBIGUOUS"
				severity = "error"
			} else if strings.Contains(msg, "missing") {
				if isEmbed {
					code = "NP-MD-EMBED-MISSING"
				} else {
					code = "NP-MD-WIKI-MISSING"
				}
			}
			out = append(out, MarkdownDiagnostic{
				Code:     code,
				Severity: severity,
				File:     fileKey,
				Line:     lineNo,
				Message:  fmt.Sprintf("%s: %s", raw, msg),
			})
		}

		if htmlTagLineRe.MatchString(line) {
			if _, ok := htmlWarnedLine[lineNo]; ok {
				continue
			}
			htmlWarnedLine[lineNo] = struct{}{}
			code := "NP-MD-RAW-HTML"
			severity := "warn"
			message := "raw HTML found in markdown body"
			if htmlDangerousTagRe.MatchString(line) {
				code = "NP-MD-HTML-DANGEROUS"
				severity = "error"
				message = "dangerous HTML tag detected"
			} else if policy == "deny" {
				code = "NP-MD-RAW-HTML-DENY"
				severity = "error"
				message = "raw HTML is denied by markdown.html_policy=deny"
			} else if policy == "safe" {
				code = "NP-MD-HTML-SANITIZED"
				severity = "warn"
				message = "raw HTML will be sanitized by markdown.html_policy=safe"
			} else if policy == "unsafe" {
				code = "NP-MD-RAW-HTML-UNSAFE"
				severity = "warn"
				message = "raw HTML will be rendered as-is by markdown.html_policy=unsafe"
			}
			out = append(out, MarkdownDiagnostic{
				Code:     code,
				Severity: severity,
				File:     fileKey,
				Line:     lineNo,
				Message:  message,
			})
		}
	}
	return out
}

func CountDiagnostics(diags []MarkdownDiagnostic) (errors int, warnings int) {
	for _, d := range diags {
		switch strings.ToLower(strings.TrimSpace(d.Severity)) {
		case "error":
			errors++
		case "warn", "warning":
			warnings++
		}
	}
	return errors, warnings
}

func LogDiagnostics(diags []MarkdownDiagnostic) {
	for _, d := range diags {
		log.Printf("[%s] %s %s:%d %s", strings.ToUpper(d.Severity), d.Code, d.File, d.Line, d.Message)
	}
}

func stripWikiAnchor(target string) string {
	return linkutil.StripWikiAnchor(target)
}

func detectMarkdownCapabilities(markdown string) MarkdownCapabilities {
	text := mdproc.MaskCodeWithSpaces(mdproc.NormalizeLineEndings(markdown))
	out := defaultMarkdownCapabilities()
	if wikiRefLineRe.MatchString(text) {
		out.Used["obsidian.wikilinks"] = true
	}
	if obsidianEmbedLineRe.MatchString(text) || mdImageRe.MatchString(text) {
		out.Used["obsidian.embeds"] = true
	}
	if len(extractObsidianTags([]byte(text))) > 0 {
		out.Used["obsidian.tags"] = true
	}
	if calloutLineRe.MatchString(text) {
		out.Used["obsidian.callouts"] = true
	}
	if footnoteRefRe.MatchString(text) {
		out.Used["obsidian.footnotes"] = true
	}
	if blockMathFenceRe.MatchString(text) || inlineMathUsageRe.MatchString(text) {
		out.Used["obsidian.math"] = true
	}
	if htmlTagLineRe.MatchString(text) {
		out.Used["raw_html"] = true
	}
	if blockRefWikiRe.MatchString(text) {
		out.Used["obsidian.block_refs"] = true
	}
	return out
}

func mergeCapabilities(dst *MarkdownCapabilities, src MarkdownCapabilities) {
	if dst == nil {
		return
	}
	if dst.Supported == nil {
		dst.Supported = map[string]bool{}
	}
	if dst.Used == nil {
		dst.Used = map[string]bool{}
	}
	for k, v := range src.Supported {
		if _, ok := dst.Supported[k]; !ok {
			dst.Supported[k] = v
		}
	}
	for k, v := range src.Used {
		if v {
			dst.Used[k] = true
		}
	}
}

func buildUnsupportedList(cap MarkdownCapabilities) []string {
	out := make([]string, 0)
	for name, used := range cap.Used {
		if !used {
			continue
		}
		if supported, ok := cap.Supported[name]; ok && !supported {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

var (
	wikiRefLineRe       = regexp.MustCompile(`!?\[\[([^\]]+)\]\]`)
	obsidianEmbedLineRe = regexp.MustCompile(`!\[\[[^\]]+\]\]`)
	htmlTagLineRe       = regexp.MustCompile(`<[A-Za-z][^>]*>`)
	htmlDangerousTagRe  = regexp.MustCompile(`(?i)<\s*(script|style|iframe|object|embed)\b`)
	calloutLineRe       = regexp.MustCompile(`(?m)^\s*>\s*\[![A-Za-z0-9_-]+\]`)
	footnoteRefRe       = regexp.MustCompile(`\[\^[^\]]+\]`)
	blockMathFenceRe    = regexp.MustCompile(`(?m)^\s*\$\$\s*$`)
	inlineMathUsageRe   = regexp.MustCompile(`\$(?:[^$\n]|\\\$)+\$`)
	blockRefWikiRe      = regexp.MustCompile(`!?\[\[[^\]]*#\^[^\]]+\]\]`)
)
