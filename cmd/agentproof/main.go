package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fatalf("usage: go run ./cmd/agentproof <list-scenarios|seed|analyze> [flags]")
	}
	switch strings.TrimSpace(os.Args[1]) {
	case "list-scenarios":
		runListScenarios()
	case "seed":
		if err := runSeed(os.Args[2:]); err != nil {
			fatalf("%v", err)
		}
	case "analyze":
		if err := runAnalyze(os.Args[2:]); err != nil {
			fatalf("%v", err)
		}
	default:
		fatalf("unknown subcommand %q", os.Args[1])
	}
}

func runListScenarios() {
	for _, def := range listScenarioDefinitions() {
		fmt.Printf("%s\t%s\t%s\n", def.Key, def.DomainBundle, def.Description)
	}
}

func runSeed(args []string) error {
	fs := flag.NewFlagSet("seed", flag.ContinueOnError)
	baseURL := fs.String("base-url", "http://127.0.0.1:18110", "Running Orbyte base URL")
	username := fs.String("username", "admin", "Bootstrap admin username")
	password := fs.String("password", "admin123!", "Bootstrap admin password")
	scenarioKey := fs.String("scenario", "employee_spend", "Scenario key to seed")
	opencodeCommand := fs.String("opencode-command", "opencode", "ACP provider command for opencode")
	output := fs.String("output", "", "Path to write the scenario manifest JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	def, err := lookupScenarioDefinition(*scenarioKey)
	if err != nil {
		return err
	}
	client, err := newAPIClient(*baseURL)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	if err := client.login(ctx, *username, *password); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}
	manifest, err := def.Seed(ctx, client, *baseURL, *opencodeCommand)
	if err != nil {
		return err
	}
	outPath := strings.TrimSpace(*output)
	if outPath == "" {
		outPath = defaultOutputPath(".", "agentproof-manifest", manifest.RunID)
	}
	if err := writeJSONFile(outPath, manifest); err != nil {
		return err
	}
	printSeedSummary(outPath, manifest)
	return nil
}

func runAnalyze(args []string) error {
	fs := flag.NewFlagSet("analyze", flag.ContinueOnError)
	baseURL := fs.String("base-url", "", "Running Orbyte base URL; defaults to the manifest base_url")
	username := fs.String("username", "admin", "Bootstrap admin username")
	password := fs.String("password", "admin123!", "Bootstrap admin password")
	manifestPath := fs.String("manifest", "", "Path to the scenario manifest JSON")
	sessionID := fs.String("session-id", "", "ACP session ID to analyze")
	titlePrefix := fs.String("session-title-prefix", "", "Fallback ACP session title prefix")
	output := fs.String("output", "", "Path to write the analysis report JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*manifestPath) == "" {
		return fmt.Errorf("--manifest is required")
	}
	manifest, err := loadManifest(*manifestPath)
	if err != nil {
		return err
	}
	if strings.TrimSpace(*baseURL) == "" {
		*baseURL = manifest.BaseURL
	}
	if strings.TrimSpace(*titlePrefix) == "" {
		*titlePrefix = manifest.SessionTitleHint
	}
	client, err := newAPIClient(*baseURL)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := client.login(ctx, *username, *password); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}
	report, err := analyzeSession(ctx, client, manifest, *sessionID, *titlePrefix)
	if err != nil {
		return err
	}
	outPath := strings.TrimSpace(*output)
	if outPath == "" {
		dir := filepath.Dir(*manifestPath)
		outPath = defaultOutputPath(dir, "agentproof-analysis", manifest.RunID)
	}
	if err := writeJSONFile(outPath, report); err != nil {
		return err
	}
	printAnalysisSummary(outPath, report)
	return nil
}

func loadManifest(path string) (scenarioManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return scenarioManifest{}, err
	}
	var manifest scenarioManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return scenarioManifest{}, err
	}
	return manifest, nil
}

func printSeedSummary(path string, manifest scenarioManifest) {
	fmt.Printf("Wrote scenario manifest: %s\n", path)
	fmt.Printf("Scenario: %s (%s)\n", manifest.Scenario, manifest.DomainBundle)
	fmt.Printf("Run ID: %s\n", manifest.RunID)
	fmt.Printf("Agent workspace: %s%s\n", manifest.BaseURL, manifest.AgentRoutePath)
	if strings.TrimSpace(manifest.WorkingDir) != "" {
		fmt.Printf("Working dir: %s\n", manifest.WorkingDir)
	}
	if strings.TrimSpace(manifest.SessionInstructions) != "" {
		fmt.Printf("Session instructions: %s\n", manifest.SessionInstructions)
	}
	fmt.Printf("Session title hint: %s\n", manifest.SessionTitleHint)
	if len(manifest.Entities) > 0 {
		keys := make([]string, 0, len(manifest.Entities))
		for key := range manifest.Entities {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			entity := manifest.Entities[key]
			name := valueOrDefault(stringValue(entity["name"]), stringValue(entity["code"]))
			if name == "" {
				continue
			}
			fmt.Printf("Entity[%s]: %s\n", key, name)
		}
	}
	fmt.Printf("Service principal: %s\n", manifest.ServicePrincipal.Key)
	fmt.Printf("Opencode MCP URL: %s\n", manifest.OpencodeConfig.URL)
}

func printAnalysisSummary(path string, report analysisReport) {
	fmt.Printf("Wrote analysis report: %s\n", path)
	fmt.Printf("Session: %s (%s)\n", report.SessionTitle, report.SessionID)
	fmt.Printf("Exact: %d  Reasonable: %d  Unacceptable: %d\n",
		report.Summary.ExactCount,
		report.Summary.ReasonableCount,
		report.Summary.UnacceptableCount,
	)
	for _, item := range report.Results {
		fmt.Printf("- %s: %s\n", item.PromptID, item.Classification)
		if len(item.MissingFacts) > 0 {
			fmt.Printf("  missing: %s\n", strings.Join(item.MissingFacts, ", "))
		}
		if len(item.Contradictions) > 0 {
			fmt.Printf("  contradictions: %s\n", strings.Join(item.Contradictions, ", "))
		}
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return ""
	}
}

func mapValue(record map[string]any, key string) map[string]any {
	value, _ := record[key].(map[string]any)
	return value
}

func valueOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
