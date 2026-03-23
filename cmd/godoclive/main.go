package main

import (
	"encoding/json"
	"fmt"
	"os"
	"bufio"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"github.com/dicki/godoclive/internal/config"
	"github.com/dicki/godoclive/internal/generator"
	"github.com/dicki/godoclive/internal/model"
	"github.com/dicki/godoclive/internal/openapi"
	"github.com/dicki/godoclive/internal/pipeline"
)

var version = "dev"

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:     "godoclive",
	Short:   "Auto-generating, interactive API documentation for Go HTTP services",
	Long:    "GoDoc Live statically analyzes Go source code, extracts HTTP route definitions, and generates interactive API documentation.",
	Version: version,
}

// generate flags
var (
	flagOutput  string
	flagFormat  string
	flagServe   string
	flagTitle   string
	flagBaseURL string
	flagTheme   string
)

// analyze flags
var (
	flagAnalyzeJSON    bool
	flagAnalyzeVerbose bool
)

// validate flags
var (
	flagValidateJSON    bool
	flagValidateVerbose bool
)

// openapi flags
var (
	flagOpenAPIOutput string
	flagOpenAPIFormat string
	flagOpenAPITitle  string
	flagOpenAPIServer string
	flagGenerateOpenAPI string
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze [packages]",
	Short: "Run analysis only — print contract summary to stdout",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runAnalyze,
}

var generateCmd = &cobra.Command{
	Use:   "generate [packages]",
	Short: "Run analysis and generate documentation site",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runGenerate,
}

var watchCmd = &cobra.Command{
	Use:   "watch [packages]",
	Short: "Watch for file changes and regenerate",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runWatch,
}

var validateCmd = &cobra.Command{
	Use:   "validate [packages]",
	Short: "Report analysis coverage: % of endpoints fully resolved",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runValidate,
}

var openapiCmd = &cobra.Command{
	Use:   "openapi [packages]",
	Short: "Generate OpenAPI 3.1.0 specification from analyzed endpoints",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runOpenAPI,
}

func init() {
	// generate flags
	generateCmd.Flags().StringVar(&flagOutput, "output", "./docs", "Output path")
	generateCmd.Flags().StringVar(&flagFormat, "format", "folder", "Output format: folder | single")
	generateCmd.Flags().StringVar(&flagServe, "serve", "", "Address to serve after generation, e.g. :8080")
	generateCmd.Flags().StringVar(&flagTitle, "title", "", "Override project title displayed in docs")
	generateCmd.Flags().StringVar(&flagBaseURL, "base-url", "", "Pre-fill base URL in Try It")
	generateCmd.Flags().StringVar(&flagTheme, "theme", "", "Theme: light | dark (default: light)")

	// analyze flags
	analyzeCmd.Flags().BoolVar(&flagAnalyzeJSON, "json", false, "Output contract as JSON (machine-readable)")
	analyzeCmd.Flags().BoolVar(&flagAnalyzeVerbose, "verbose", false, "Show full Unresolved list per endpoint")

	// validate flags
	validateCmd.Flags().BoolVar(&flagValidateJSON, "json", false, "Output as JSON")
	validateCmd.Flags().BoolVar(&flagValidateVerbose, "verbose", false, "Show full Unresolved list per endpoint")

	// generate --openapi flag
	generateCmd.Flags().StringVar(&flagGenerateOpenAPI, "openapi", "", "Also generate OpenAPI spec at this path")

	// watch flags (same as generate)
	watchCmd.Flags().StringVar(&flagOutput, "output", "./docs", "Output path")
	watchCmd.Flags().StringVar(&flagFormat, "format", "folder", "Output format: folder | single")
	watchCmd.Flags().StringVar(&flagServe, "serve", "", "Address to serve after generation, e.g. :8080")
	watchCmd.Flags().StringVar(&flagTitle, "title", "", "Override project title displayed in docs")
	watchCmd.Flags().StringVar(&flagBaseURL, "base-url", "", "Pre-fill base URL in Try It")
	watchCmd.Flags().StringVar(&flagTheme, "theme", "", "Theme: light | dark (default: light)")

	// openapi flags
	openapiCmd.Flags().StringVar(&flagOpenAPIOutput, "output", "./openapi.json", "Output file path (.json or .yaml)")
	openapiCmd.Flags().StringVar(&flagOpenAPIFormat, "format", "", "Output format: json | yaml (inferred from extension if omitted)")
	openapiCmd.Flags().StringVar(&flagOpenAPITitle, "title", "", "API title")
	openapiCmd.Flags().StringVar(&flagOpenAPIServer, "server", "", "Server URL to include in spec")

	rootCmd.AddCommand(analyzeCmd, generateCmd, watchCmd, validateCmd, openapiCmd)
}

// runAnalyze implements the analyze command.
func runAnalyze(cmd *cobra.Command, args []string) error {
	endpoints, err := runPipeline(args[0])
	if err != nil {
		return err
	}

	if flagAnalyzeJSON {
		return printJSON(endpoints)
	}

	return printSummaryTable(endpoints, flagAnalyzeVerbose)
}

// runValidate implements the validate command.
func runValidate(cmd *cobra.Command, args []string) error {
	endpoints, err := runPipeline(args[0])
	if err != nil {
		return err
	}

	if flagValidateJSON {
		return printValidateJSON(endpoints)
	}

	return printCoverageReport(endpoints, flagValidateVerbose)
}

// runPipeline loads config and runs the analysis pipeline.
func runPipeline(pattern string) ([]model.EndpointDef, error) {
	// Extract directory and package pattern from the argument.
	// "./testdata/chi-basic/..." → dir="./testdata/chi-basic", pattern="./..."
	dir, pkgPattern := splitDirPattern(pattern)

	// Resolve dir to absolute path.
	if dir != "" {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			return nil, fmt.Errorf("resolving directory %q: %w", dir, err)
		}
		dir = absDir
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("getting working directory: %w", err)
		}
		dir = cwd
	}

	// Load config from the target directory (optional).
	cfg, err := config.LoadConfig(dir)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	return pipeline.RunPipeline(dir, pkgPattern, cfg)
}

// splitDirPattern splits a CLI argument like "./testdata/chi-basic/..." into
// a directory and a package pattern. If the argument ends with "/..." or "/.",
// the directory is everything before that suffix.
func splitDirPattern(arg string) (dir, pattern string) {
	// Handle "./..." — means current directory, all packages.
	if arg == "./..." {
		return ".", "./..."
	}

	// Check for trailing "/..." or "/."
	if strings.HasSuffix(arg, "/...") {
		dir = strings.TrimSuffix(arg, "/...")
		return dir, "./..."
	}

	if strings.HasSuffix(arg, "/.") {
		dir = strings.TrimSuffix(arg, "/.")
		return dir, "."
	}

	// If it's a directory path without a pattern suffix, assume "./...".
	info, err := os.Stat(arg)
	if err == nil && info.IsDir() {
		return arg, "./..."
	}

	// Otherwise treat the whole thing as a pattern from cwd.
	return ".", arg
}

// printJSON serializes endpoints as indented JSON to stdout.
func printJSON(endpoints []model.EndpointDef) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(endpoints)
}

// printSummaryTable prints a human-readable summary table.
func printSummaryTable(endpoints []model.EndpointDef, verbose bool) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "METHOD\tPATH\tSUMMARY\tAUTH\tSTATUS")
	_, _ = fmt.Fprintln(w, "------\t----\t-------\t----\t------")

	for _, ep := range endpoints {
		authStr := "-"
		if ep.Auth.Required && len(ep.Auth.Schemes) > 0 {
			schemes := make([]string, len(ep.Auth.Schemes))
			for i, s := range ep.Auth.Schemes {
				schemes[i] = string(s)
			}
			authStr = strings.Join(schemes, ",")
		}

		status := "\u2713 complete"
		if len(ep.Unresolved) > 0 {
			status = fmt.Sprintf("\u26a0 partial (%d unresolved)", len(ep.Unresolved))
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			ep.Method, ep.Path, ep.Summary, authStr, status)

		if verbose && len(ep.Unresolved) > 0 {
			for _, u := range ep.Unresolved {
				_, _ = fmt.Fprintf(w, "\t\t  -> %s\t\t\n", u)
			}
		}
	}

	return w.Flush()
}

// ValidateReport is the JSON structure for the validate command.
type ValidateReport struct {
	Total     int              `json:"total"`
	Resolved  int              `json:"resolved"`
	Partial   int              `json:"partial"`
	Endpoints []ValidateEntry  `json:"endpoints"`
}

// ValidateEntry represents one endpoint in the validate report.
type ValidateEntry struct {
	Method     string   `json:"method"`
	Path       string   `json:"path"`
	Complete   bool     `json:"complete"`
	Unresolved []string `json:"unresolved,omitempty"`
}

func printValidateJSON(endpoints []model.EndpointDef) error {
	report := buildReport(endpoints)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func printCoverageReport(endpoints []model.EndpointDef, verbose bool) error {
	report := buildReport(endpoints)

	fmt.Println("GoDoc Live — Analysis Coverage Report")
	fmt.Println()
	fmt.Printf("Total endpoints:     %d\n", report.Total)
	if report.Total > 0 {
		pctResolved := float64(report.Resolved) / float64(report.Total) * 100
		pctPartial := float64(report.Partial) / float64(report.Total) * 100
		fmt.Printf("Fully resolved:      %d  (%.1f%%)\n", report.Resolved, pctResolved)
		fmt.Printf("Partially resolved:  %d  (%.1f%%)\n", report.Partial, pctPartial)
	}

	if report.Partial > 0 || verbose {
		fmt.Println()
		fmt.Println("Unresolved items:")
		for _, entry := range report.Endpoints {
			if len(entry.Unresolved) > 0 {
				for _, u := range entry.Unresolved {
					fmt.Printf("  %-7s %-20s -> %s\n", entry.Method, entry.Path, u)
				}
			}
		}
	}

	return nil
}

func buildReport(endpoints []model.EndpointDef) ValidateReport {
	report := ValidateReport{
		Total: len(endpoints),
	}
	for _, ep := range endpoints {
		entry := ValidateEntry{
			Method:     ep.Method,
			Path:       ep.Path,
			Complete:   len(ep.Unresolved) == 0,
			Unresolved: ep.Unresolved,
		}
		report.Endpoints = append(report.Endpoints, entry)
		if len(ep.Unresolved) == 0 {
			report.Resolved++
		} else {
			report.Partial++
		}
	}
	return report
}

// runGenerate implements the generate command.
func runGenerate(cmd *cobra.Command, args []string) error {
	endpoints, err := runPipeline(args[0])
	if err != nil {
		return err
	}

	genCfg := buildGeneratorConfig(args[0])

	if err := generator.Generate(endpoints, genCfg); err != nil {
		return err
	}

	absOut, _ := filepath.Abs(flagOutput)
	fmt.Printf("Documentation generated at %s/\n", absOut)

	// Optionally generate OpenAPI spec alongside HTML.
	if flagGenerateOpenAPI != "" {
		if err := generateOpenAPISpec(endpoints, args[0], flagGenerateOpenAPI); err != nil {
			return err
		}
	}

	if flagServe != "" {
		return generator.Serve(flagOutput, flagServe)
	}
	return nil
}

// runOpenAPI implements the openapi command.
func runOpenAPI(cmd *cobra.Command, args []string) error {
	endpoints, err := runPipeline(args[0])
	if err != nil {
		return err
	}

	output := flagOpenAPIOutput
	return generateOpenAPISpec(endpoints, args[0], output)
}

// generateOpenAPISpec generates an OpenAPI spec file from endpoints.
func generateOpenAPISpec(endpoints []model.EndpointDef, pattern, output string) error {
	dir, _ := splitDirPattern(pattern)
	absDir, _ := filepath.Abs(dir)
	cfg, _ := config.LoadConfig(absDir)

	oapiCfg := buildOpenAPIConfig(cfg)

	doc := openapi.Generate(endpoints, oapiCfg)

	var format openapi.OutputFormat
	if flagOpenAPIFormat != "" {
		format = openapi.OutputFormat(flagOpenAPIFormat)
	}

	if err := openapi.Write(doc, openapi.WriteConfig{
		OutputPath: output,
		Format:     format,
		Indent:     true,
	}); err != nil {
		return err
	}

	absPath, _ := filepath.Abs(output)
	fmt.Printf("OpenAPI spec generated at %s\n", absPath)
	return nil
}

// buildOpenAPIConfig creates an openapi.Config from CLI flags and .godoclive.yaml.
func buildOpenAPIConfig(cfg *config.Config) openapi.Config {
	oapiCfg := openapi.Config{
		Title:   coalesce(flagOpenAPITitle, flagTitle, cfgStr(cfg, func(c *config.Config) string { return c.Title })),
		Version: cfgStr(cfg, func(c *config.Config) string { return c.Version }),
	}

	if flagOpenAPIServer != "" {
		oapiCfg.Servers = []openapi.Server{{URL: flagOpenAPIServer}}
	}

	// Apply .godoclive.yaml openapi section if available.
	if cfg != nil {
		if cfg.OpenAPI.Description != "" {
			oapiCfg.Description = cfg.OpenAPI.Description
		}
		if cfg.OpenAPI.Contact.Name != "" || cfg.OpenAPI.Contact.Email != "" {
			oapiCfg.Contact = &openapi.Contact{
				Name:  cfg.OpenAPI.Contact.Name,
				URL:   cfg.OpenAPI.Contact.URL,
				Email: cfg.OpenAPI.Contact.Email,
			}
		}
		if cfg.OpenAPI.License.Name != "" {
			oapiCfg.License = &openapi.License{
				Name: cfg.OpenAPI.License.Name,
				URL:  cfg.OpenAPI.License.URL,
			}
		}
		if len(cfg.OpenAPI.Servers) > 0 && len(oapiCfg.Servers) == 0 {
			for _, s := range cfg.OpenAPI.Servers {
				oapiCfg.Servers = append(oapiCfg.Servers, openapi.Server{
					URL:         s.URL,
					Description: s.Description,
				})
			}
		}
	}

	return oapiCfg
}

// buildGeneratorConfig creates a GeneratorConfig merging CLI flags with
// .env and .godoclive.yaml values. Precedence: CLI flag > .env > yaml > default.
func buildGeneratorConfig(pattern string) generator.GeneratorConfig {
	dir, _ := splitDirPattern(pattern)
	absDir, _ := filepath.Abs(dir)
	cfg, _ := config.LoadConfig(absDir)
	env := loadDotEnv(absDir)

	return generator.GeneratorConfig{
		OutputPath: flagOutput,
		Format:     flagFormat,
		Title:      coalesce(flagTitle, cfgStr(cfg, func(c *config.Config) string { return c.Title })),
		Version:    cfgStr(cfg, func(c *config.Config) string { return c.Version }),
		BaseURL:    coalesce(flagBaseURL, env["API_URL"], cfgStr(cfg, func(c *config.Config) string { return c.BaseURL })),
		Theme:      coalesce(flagTheme, cfgStr(cfg, func(c *config.Config) string { return c.Theme }), "light"),
	}
}

func coalesce(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func cfgStr(cfg *config.Config, fn func(*config.Config) string) string {
	if cfg == nil {
		return ""
	}
	return fn(cfg)
}

// loadDotEnv reads a .env file from the given directory and returns a map of
// key=value pairs. Returns an empty map if the file doesn't exist.
func loadDotEnv(dir string) map[string]string {
	result := make(map[string]string)
	f, err := os.Open(filepath.Join(dir, ".env"))
	if err != nil {
		return result
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		// Strip surrounding quotes.
		if len(value) >= 2 &&
			((value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}
		result[key] = value
	}
	return result
}

// runWatch implements the watch command.
func runWatch(cmd *cobra.Command, args []string) error {
	pattern := args[0]
	dir, _ := splitDirPattern(pattern)
	if dir != "" {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			return fmt.Errorf("resolving directory %q: %w", dir, err)
		}
		dir = absDir
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}
		dir = cwd
	}

	// Initial generation.
	genCfg := buildGeneratorConfig(pattern)

	doGenerate := func() {
		endpoints, err := runPipeline(pattern)
		if err != nil {
			fmt.Fprintf(os.Stderr, "analysis error: %v\n", err)
			return
		}
		if err := generator.Generate(endpoints, genCfg); err != nil {
			fmt.Fprintf(os.Stderr, "generation error: %v\n", err)
			return
		}
		fmt.Println("Documentation regenerated.")
	}

	doGenerate()

	// Start SSE server if --serve is set.
	var reloadCh chan struct{}
	if flagServe != "" {
		var err error
		reloadCh, err = generator.ServeWithSSE(flagOutput, flagServe)
		if err != nil {
			return err
		}
	}

	// Set up file watcher.
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("creating file watcher: %w", err)
	}
	defer func() { _ = watcher.Close() }()

	// Walk dir and add all directories containing .go files.
	if err := addWatchDirs(watcher, dir); err != nil {
		return fmt.Errorf("setting up watch: %w", err)
	}

	fmt.Printf("Watching %s for changes...\n", dir)

	// Debounce timer.
	var mu sync.Mutex
	var timer *time.Timer

	triggerRegenerate := func() {
		mu.Lock()
		defer mu.Unlock()
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(500*time.Millisecond, func() {
			doGenerate()
			if reloadCh != nil {
				select {
				case reloadCh <- struct{}{}:
				default:
				}
			}
		})
	}

	// Signal handling.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if strings.HasSuffix(event.Name, ".go") &&
				(event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove)) != 0 {
				triggerRegenerate()
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintf(os.Stderr, "watcher error: %v\n", err)
		case <-sigCh:
			fmt.Println("\nShutting down.")
			return nil
		}
	}
}

// addWatchDirs recursively adds directories containing .go files to the watcher.
func addWatchDirs(watcher *fsnotify.Watcher, root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if info.IsDir() {
			name := info.Name()
			// Skip hidden dirs, vendor, testdata, .git.
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return watcher.Add(path)
		}
		return nil
	})
}
