package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type measurementReport struct {
	Version       string             `json:"version"`
	RunID         string             `json:"run_id"`
	GeneratedAt   time.Time          `json:"generated_at"`
	BaseURL       string             `json:"base_url"`
	OpencodeCmd   string             `json:"opencode_command"`
	ModeResults   []modeMeasurement  `json:"mode_results"`
	Comparison    modeComparison     `json:"comparison"`
}

type modeMeasurement struct {
	ExposureMode   string                      `json:"exposure_mode"`
	ScenarioRuns   []scenarioModeMeasurement   `json:"scenario_runs"`
	AvgToolCalls   float64                     `json:"avg_tool_calls"`
	AvgAccuracy    float64                     `json:"avg_accuracy"`
	TotalPrompts   int                         `json:"total_prompts"`
	TotalToolCalls int                         `json:"total_tool_calls"`
	TotalExact     int                         `json:"total_exact"`
	TotalReasonable int                        `json:"total_reasonable"`
	TotalUnacceptable int                      `json:"total_unacceptable"`
	PlaybookHits   int                         `json:"playbook_hits"`
	PlaybookMisses int                         `json:"playbook_misses"`
}

type scenarioModeMeasurement struct {
	Scenario            string  `json:"scenario"`
	SessionID           string  `json:"session_id"`
	ManifestPath        string  `json:"manifest_path"`
	PromptCount         int     `json:"prompt_count"`
	ExactCount          int     `json:"exact_count"`
	ReasonableCount     int     `json:"reasonable_count"`
	UnacceptableCount   int     `json:"unacceptable_count"`
	Accuracy            float64 `json:"accuracy"`
	TotalToolCalls      int     `json:"total_tool_calls"`
	AvgToolCallsPerPrompt float64 `json:"avg_tool_calls_per_prompt"`
	MatchedPlaybooks    int     `json:"matched_playbooks"`
	Success             bool    `json:"success"`
	PlaybookHits        int     `json:"playbook_hits"`
	PlaybookMisses      int     `json:"playbook_misses"`
}

type modeComparison struct {
	FullAvgToolCalls     float64 `json:"full_avg_tool_calls"`
	MinimalAvgToolCalls  float64 `json:"minimal_avg_tool_calls"`
	ToolCallReduction    float64 `json:"tool_call_reduction_pct"`
	FullAccuracy         float64 `json:"full_accuracy"`
	MinimalAccuracy      float64 `json:"minimal_accuracy"`
	FullUnacceptable     int     `json:"full_unacceptable"`
	MinimalUnacceptable  int     `json:"minimal_unacceptable"`
}

func runMeasureMCP(args []string) error {
	fs := flag.NewFlagSet("measure-mcp", flag.ContinueOnError)
	baseURL := fs.String("base-url", "http://127.0.0.1:18110", "Running Orbyte base URL")
	username := fs.String("username", "admin", "Bootstrap admin username")
	password := fs.String("password", "admin123!", "Bootstrap admin password")
	opencodeCommand := fs.String("opencode-command", "opencode", "ACP provider command for opencode")
	output := fs.String("output", "", "Path to write the measurement report JSON")
	modeFilter := fs.String("modes", "", "Comma-separated exposure modes to run (default: full,minimal)")
	scenarioFilter := fs.String("scenarios", "", "Comma-separated scenarios to run (default: retail_recovery_showcase,inventory_replenishment_execute)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	client, err := newAPIClient(*baseURL)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	if err := client.login(ctx, *username, *password); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	runID, suffix := newRunContext()
	if _, _, err := configureAgentAccess(ctx, client, *baseURL, *opencodeCommand, suffix); err != nil {
		return fmt.Errorf("configure runtime: %w", err)
	}

	report := measurementReport{
		Version:     "2026-04-11",
		RunID:       runID,
		GeneratedAt: time.Now().UTC(),
		BaseURL:     *baseURL,
		OpencodeCmd: *opencodeCommand,
		ModeResults: make([]modeMeasurement, 0, 2),
	}

	modes := splitCSVOrDefault(*modeFilter, []string{"full", "minimal"})
	scenarios := splitCSVOrDefault(*scenarioFilter, []string{"retail_recovery_showcase", "inventory_replenishment_execute"})

	for _, mode := range modes {
		fmt.Printf("\n=== Running mode: %s ===\n", mode)

		// Set MCP config for this mode
		playbooksJSON := defaultMCPPlaybooksJSON()
		if err := client.putConfig(ctx, "platform.mcp", map[string]any{
			"enabled":                            true,
			"exposure_mode":                      mode,
			"governance_enabled":                 true,
			"default_action_mode":                "draft_only",
			"tool_states_json":                   "{}",
			"blocked_action_classes_json":        "[]",
			"blocked_tool_keys_json":             "[]",
			"blocked_document_types_json":        "[]",
			"allowed_submit_document_types_json": "[]",
			"domain_policy_overrides_json":       "{}",
			"playbooks_json":                     playbooksJSON,
		}); err != nil {
			return fmt.Errorf("set platform.mcp %s mode: %w", mode, err)
		}

		modeResult := modeMeasurement{
			ExposureMode: mode,
			ScenarioRuns: make([]scenarioModeMeasurement, 0, len(scenarios)),
		}

		for _, scenarioKey := range scenarios {
			fmt.Printf("  Running scenario: %s\n", scenarioKey)
			run, runErr := validateScenarioWithMode(ctx, client, *baseURL, *opencodeCommand, mode, runID, scenarioKey)
			if runErr != nil {
				fmt.Printf("  Scenario %s failed: %v\n", scenarioKey, runErr)
				run = mcpValidationScenarioRun{
					Scenario:      scenarioKey,
					Success:       false,
					FailureReason: runErr.Error(),
				}
			}

			// Enrich with session-level metrics
			enriched := enrichScenarioMeasurement(run)
			modeResult.ScenarioRuns = append(modeResult.ScenarioRuns, enriched)

			// Accumulate totals
			modeResult.TotalPrompts += len(run.Analysis.Results)
			modeResult.TotalExact += run.Analysis.Summary.ExactCount
			modeResult.TotalReasonable += run.Analysis.Summary.ReasonableCount
			modeResult.TotalUnacceptable += run.Analysis.Summary.UnacceptableCount

			for _, r := range run.Analysis.Results {
				modeResult.TotalToolCalls += r.ToolCallCount
			}

			// Count playbook matches
			for _, r := range run.Analysis.Results {
				if r.MatchedPlaybookID != "" {
					modeResult.PlaybookHits++
				} else {
					modeResult.PlaybookMisses++
				}
			}
		}

		// Compute averages
		if modeResult.TotalPrompts > 0 {
			modeResult.AvgToolCalls = float64(modeResult.TotalToolCalls) / float64(modeResult.TotalPrompts)
			modeResult.AvgAccuracy = float64(modeResult.TotalExact+modeResult.TotalReasonable) / float64(modeResult.TotalPrompts)
		}

		report.ModeResults = append(report.ModeResults, modeResult)
	}

	// Compute comparison
	report.Comparison = computeComparison(report.ModeResults)

	// Print summary
	printMeasurementSummary(report)

	outPath := strings.TrimSpace(*output)
	if outPath == "" {
		outPath = defaultOutputPath(".", "agentproof-mcp-measurement", runID)
	}
	if err := writeJSONFile(outPath, report); err != nil {
		return err
	}
	fmt.Printf("\nWrote measurement report: %s\n", outPath)
	return nil
}

func enrichScenarioMeasurement(run mcpValidationScenarioRun) scenarioModeMeasurement {
	totalPrompts := len(run.Analysis.Results)
	totalToolCalls := 0
	for _, r := range run.Analysis.Results {
		totalToolCalls += r.ToolCallCount
	}

	avgToolCalls := 0.0
	if totalPrompts > 0 {
		avgToolCalls = float64(totalToolCalls) / float64(totalPrompts)
	}

	accuracy := 0.0
	if totalPrompts > 0 {
		accuracy = float64(run.Analysis.Summary.ExactCount+run.Analysis.Summary.ReasonableCount) / float64(totalPrompts)
	}

	return scenarioModeMeasurement{
		Scenario:              run.Scenario,
		SessionID:             run.SessionID,
		ManifestPath:          run.ManifestPath,
		PromptCount:           totalPrompts,
		ExactCount:            run.Analysis.Summary.ExactCount,
		ReasonableCount:       run.Analysis.Summary.ReasonableCount,
		UnacceptableCount:     run.Analysis.Summary.UnacceptableCount,
		Accuracy:              accuracy,
		TotalToolCalls:        totalToolCalls,
		AvgToolCallsPerPrompt: avgToolCalls,
		MatchedPlaybooks:      len(run.MatchedPlaybookIDs),
		Success:               run.Success,
		PlaybookHits:          run.Analysis.Summary.ExactCount, // proxies: exact matches often correlate with playbook use
	}
}

func computeComparison(modeResults []modeMeasurement) modeComparison {
	var fullResult, minimalResult *modeMeasurement
	for i := range modeResults {
		switch modeResults[i].ExposureMode {
		case "full":
			fullResult = &modeResults[i]
		case "minimal":
			minimalResult = &modeResults[i]
		}
	}

	comp := modeComparison{}
	if fullResult != nil {
		comp.FullAvgToolCalls = fullResult.AvgToolCalls
		comp.FullAccuracy = fullResult.AvgAccuracy
		comp.FullUnacceptable = fullResult.TotalUnacceptable
	}
	if minimalResult != nil {
		comp.MinimalAvgToolCalls = minimalResult.AvgToolCalls
		comp.MinimalAccuracy = minimalResult.AvgAccuracy
		comp.MinimalUnacceptable = minimalResult.TotalUnacceptable
	}

	if fullResult != nil && minimalResult != nil && fullResult.AvgToolCalls > 0 {
		comp.ToolCallReduction = (fullResult.AvgToolCalls - minimalResult.AvgToolCalls) / fullResult.AvgToolCalls * 100
	}

	return comp
}

func printMeasurementSummary(report measurementReport) {
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("MCP MODE EFFECTIVENESS MEASUREMENT")
	fmt.Println(strings.Repeat("=", 70))

	for _, mode := range report.ModeResults {
		fmt.Printf("\n--- Mode: %s ---\n", mode.ExposureMode)
		for _, run := range mode.ScenarioRuns {
			fmt.Printf("  Scenario: %s\n", run.Scenario)
			fmt.Printf("    Prompts: %d | Exact: %d | Reasonable: %d | Unacceptable: %d\n",
				run.PromptCount, run.ExactCount, run.ReasonableCount, run.UnacceptableCount)
			fmt.Printf("    Accuracy: %.1f%%\n", run.Accuracy*100)
			fmt.Printf("    Tool calls: %d total, %.1f avg/prompt\n", run.TotalToolCalls, run.AvgToolCallsPerPrompt)
			if run.SessionID != "" {
				fmt.Printf("    Session: %s\n", run.SessionID)
			}
		}
		fmt.Printf("  Overall: Accuracy=%.1f%% | Avg tool calls/prompt=%.1f | Unacceptable=%d\n",
			mode.AvgAccuracy*100, mode.AvgToolCalls, mode.TotalUnacceptable)
	}

	if report.Comparison.FullAvgToolCalls > 0 {
		fmt.Printf("\n--- Comparison ---\n")
		fmt.Printf("  Tool call reduction (minimal vs full): %.1f%%\n", report.Comparison.ToolCallReduction)
		fmt.Printf("  Full mode accuracy: %.1f%%\n", report.Comparison.FullAccuracy*100)
		fmt.Printf("  Minimal mode accuracy: %.1f%%\n", report.Comparison.MinimalAccuracy*100)
		fmt.Printf("  Full unacceptable: %d | Minimal unacceptable: %d\n",
			report.Comparison.FullUnacceptable, report.Comparison.MinimalUnacceptable)
	}

	fmt.Println(strings.Repeat("=", 70))
}

func fetchSessionMessages(ctx context.Context, client *apiClient, sessionID string) ([]sessionMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+"/agent/api/sessions/"+sessionID, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var session struct {
		Messages []sessionMessage `json:"messages"`
	}
	if err := json.Unmarshal(body, &session); err != nil {
		return nil, err
	}

	return session.Messages, nil
}

func estimateTokenCount(content string) int {
	// Rough estimate: ~4 chars per token for English text
	// This is a lower-bound estimate; actual tokenization varies by model
	if len(content) == 0 {
		return 0
	}
	words := len(strings.Fields(content))
	// Average ~1.3 tokens per word for English
	return int(float64(words) * 1.3)
}
