package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"
)

func runValidateMCP(args []string) error {
	fs := flag.NewFlagSet("validate-mcp", flag.ContinueOnError)
	baseURL := fs.String("base-url", "http://127.0.0.1:18110", "Running Orbyte base URL")
	username := fs.String("username", "admin", "Bootstrap admin username")
	password := fs.String("password", "admin123!", "Bootstrap admin password")
	opencodeCommand := fs.String("opencode-command", "opencode", "ACP provider command for opencode")
	output := fs.String("output", "", "Path to write the MCP validation report JSON")
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

	report := mcpValidationReport{
		Version:     "2026-04-10",
		RunID:       runID,
		GeneratedAt: time.Now().UTC(),
		BaseURL:     *baseURL,
		Modes:       make([]mcpValidationModeResult, 0, 2),
	}

	modes := splitCSVOrDefault(*modeFilter, []string{"full", "minimal"})
	scenarios := splitCSVOrDefault(*scenarioFilter, []string{"retail_recovery_showcase", "inventory_replenishment_execute"})
	for _, mode := range modes {
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
			"playbooks_json":                     defaultMCPPlaybooksJSON(),
		}); err != nil {
			return fmt.Errorf("set platform.mcp %s mode: %w", mode, err)
		}
		modeResult := mcpValidationModeResult{
			ExposureMode: mode,
			ScenarioRuns: make([]mcpValidationScenarioRun, 0, len(scenarios)),
		}
		for _, scenarioKey := range scenarios {
			run, runErr := validateScenarioWithMode(ctx, client, *baseURL, *opencodeCommand, mode, runID, scenarioKey)
			if runErr != nil {
				run = mcpValidationScenarioRun{
					Scenario:      scenarioKey,
					Success:       false,
					FailureReason: runErr.Error(),
				}
			}
			modeResult.ScenarioRuns = append(modeResult.ScenarioRuns, run)
		}
		report.Modes = append(report.Modes, modeResult)
	}

	outPath := strings.TrimSpace(*output)
	if outPath == "" {
		outPath = defaultOutputPath(".", "agentproof-mcp-validation", runID)
	}
	if err := writeJSONFile(outPath, report); err != nil {
		return err
	}
	fmt.Printf("Wrote MCP validation report: %s\n", outPath)
	return nil
}

func splitCSVOrDefault(value string, defaults []string) []string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return defaults
	}
	parts := strings.Split(trimmed, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		items = append(items, part)
	}
	if len(items) == 0 {
		return defaults
	}
	return items
}

func validateScenarioWithMode(ctx context.Context, client *apiClient, baseURL, opencodeCommand, exposureMode, runID, scenarioKey string) (mcpValidationScenarioRun, error) {
	def, err := lookupScenarioDefinition(scenarioKey)
	if err != nil {
		return mcpValidationScenarioRun{}, err
	}
	manifest, err := def.Seed(ctx, client, baseURL, opencodeCommand)
	if err != nil {
		return mcpValidationScenarioRun{}, err
	}
	manifestPath := defaultOutputPath("/tmp", "agentproof-"+scenarioKey+"-"+exposureMode, manifest.RunID)
	if err := writeJSONFile(manifestPath, manifest); err != nil {
		return mcpValidationScenarioRun{}, err
	}

	sessionTitle := fmt.Sprintf("%s [%s]", manifest.SessionTitleHint, exposureMode)
	session, err := client.startSession(ctx, map[string]any{
		"provider_key": "opencode",
		"shell":        "workspace",
		"route_path":   manifest.AgentRoutePath,
		"title":        sessionTitle,
		"working_dir":  manifest.WorkingDir,
	})
	if err != nil {
		return mcpValidationScenarioRun{}, fmt.Errorf("start session: %w", err)
	}

	for _, prompt := range manifest.PromptPack {
		mode := "ask"
		if prompt.ExpectedDraft != nil {
			mode = "execute"
		} else if prompt.ExpectedPlan != nil {
			mode = "plan"
		}
		if _, err := promptSessionWithRetry(ctx, client, session.ID, map[string]any{
			"content":         composeValidationPrompt(prompt.Prompt, mode, exposureMode, manifest.SessionInstructions),
			"display_content": prompt.Prompt,
			"mode":            mode,
		}); err != nil {
			return mcpValidationScenarioRun{}, fmt.Errorf("prompt %s: %w", prompt.ID, err)
		}
		if err := waitForSessionTurn(ctx, client, session.ID); err != nil {
			return mcpValidationScenarioRun{}, fmt.Errorf("wait for prompt %s: %w", prompt.ID, err)
		}
	}

	analysis, err := analyzeSession(ctx, client, manifest, session.ID, "")
	if err != nil {
		return mcpValidationScenarioRun{}, fmt.Errorf("analyze session: %w", err)
	}
	success := analysis.Summary.UnacceptableCount == 0
	matchedPlaybooks := collectMatchedPlaybookIDs(analysis.Results)
	requiredArtifactsMet := true
	artifactEventCount := 0
	for _, item := range analysis.Results {
		if item.ArtifactEventCount > artifactEventCount {
			artifactEventCount = item.ArtifactEventCount
		}
		if !item.RequiredArtifactsPresent {
			requiredArtifactsMet = false
		}
	}
	return mcpValidationScenarioRun{
		Scenario:             scenarioKey,
		ManifestPath:         filepath.Clean(manifestPath),
		SessionID:            session.ID,
		SessionTitle:         sessionTitle,
		Analysis:             analysis,
		Success:              success,
		MatchedPlaybookIDs:   matchedPlaybooks,
		RequiredArtifactsMet: requiredArtifactsMet,
		ArtifactEventCount:   artifactEventCount,
	}, nil
}

func waitForSessionTurn(ctx context.Context, client *apiClient, sessionID string) error {
	deadline := time.Now().Add(90 * time.Second)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for session turn")
		}
		session, err := client.getLiveSession(ctx, sessionID)
		if err != nil {
			return err
		}
		if !session.TurnInProgress {
			if strings.EqualFold(strings.TrimSpace(session.Status), "error") {
				return fmt.Errorf("session entered error status")
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1500 * time.Millisecond):
		}
	}
}

func promptSessionWithRetry(ctx context.Context, client *apiClient, sessionID string, req map[string]any) (acpSession, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		item, err := client.promptSession(ctx, sessionID, req)
		if err == nil {
			return item, nil
		}
		lastErr = err
		if !isRetryablePromptError(err) || attempt == 2 {
			break
		}
		select {
		case <-ctx.Done():
			return acpSession{}, ctx.Err()
		case <-time.After(time.Duration(attempt+1) * time.Second):
		}
	}
	return acpSession{}, lastErr
}

func isRetryablePromptError(err error) bool {
	if err == nil {
		return false
	}
	if err == io.EOF {
		return true
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, " eof") || strings.HasSuffix(message, "eof")
}

func composeValidationPrompt(prompt, mode, exposureMode, sessionInstructions string) string {
	sections := []string{}
	if strings.TrimSpace(sessionInstructions) != "" {
		sections = append(sections, "Scenario instructions:\n"+strings.TrimSpace(sessionInstructions))
	}
	if exposureMode == "minimal" {
		sections = append(sections,
			"Use Orbyte MCP as the source of truth. Search playbooks first when the request matches a business workflow. If no playbook fits, use tools.search or tools.list, then tools.describe, then tools.call. Use exact discovered tool ids only.",
		)
	} else {
		sections = append(sections,
			"Use Orbyte MCP as the source of truth. If the right tool is not already obvious, use tools/list discovery before calling tools.",
		)
	}
	switch mode {
	case "plan":
		sections = append(sections,
			"Planning mode is active. Gather evidence, produce a concise stepwise plan, and do not execute or create records unless the user explicitly asks to execute later.",
		)
	case "execute":
		sections = append(sections,
			"Execute mode is active. Act on the current plan or explicit user request, report created artifacts and links, and avoid unnecessary re-planning.",
		)
	}
	lower := strings.ToLower(prompt)
	if strings.Contains(lower, "dashboard widget") || strings.Contains(lower, "dashboard widgets") {
		sections = append(sections,
			"For dashboard requests, if widget tools are needed, include every returned <orbyte-dashboard-artifact> block verbatim in the final answer.",
		)
	}
	if strings.Contains(lower, "crm") && strings.Contains(lower, "widget") {
		sections = append(sections,
			"For CRM widget requests, call analytics dashboard widget tools with surface set to dashboard. Prefer widget keys crm.ticketing.open_tickets, crm.ticketing.overdue_tickets, and crm.ticketing.queue_backlog for service backlog evidence.",
		)
	}
	if strings.Contains(lower, "crm service backlog") || (strings.Contains(lower, "queue") && strings.Contains(lower, "customer need the most attention")) {
		sections = append(sections,
			"For CRM service backlog questions, call crm.ticket.summary first and include the exact priority queue code and overdue ticket title in the final answer before showing widgets.",
		)
	}
	if strings.Contains(lower, "crm") && strings.Contains(lower, "pipeline") {
		sections = append(sections,
			"For CRM pipeline review, include the stale opportunity title and its specific prioritized value, not only the total portfolio or open-opportunity count.",
		)
	}
	if strings.Contains(lower, "customer 360") {
		sections = append(sections,
			"For CRM customer 360 questions, call crm.customer.summary with query set to the exact customer name from the prompt, then name the current ticket title, active opportunity title, and opportunity stage explicitly.",
		)
	}
	if strings.Contains(lower, "combined crm service and sales overview") {
		sections = append(sections,
			"For combined CRM overview questions, call crm.ticket.summary, crm.customer.summary for each named customer, and crm.opportunity.pipeline.summary. Include the exact backlog queue code and explicitly mention the pipeline in the final answer.",
		)
	}
	if strings.Contains(lower, "underperforming compared with the strongest branch") || (strings.Contains(lower, "underperforming") && strings.Contains(lower, "strongest branch")) {
		sections = append(sections,
			"Explicitly name the strongest benchmark branch and each underperforming branch in the final answer instead of referring to them generically.",
			"For the retail recovery dashboard insight, preview exactly these dashboard widgets when available: analytics.demo.sales.target_attainment, analytics.demo.sales.branch_mix, and analytics.demo.sales.daily_trend.",
		)
	}
	if strings.Contains(lower, "recovery plan") && strings.Contains(lower, "loc demo central") && strings.Contains(lower, "loc demo west") {
		sections = append(sections,
			"For the retail recovery plan, explicitly state that Beans Boost should be replaced by the Espresso Double + Butter Croissant bundle for gold members.",
		)
	}
	sections = append(sections, prompt)
	return strings.Join(sections, "\n\n")
}
