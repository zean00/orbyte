package main

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var dashboardArtifactPattern = regexp.MustCompile(`(?s)<orbyte-dashboard-artifact>\s*(\{.*?\})\s*</orbyte-dashboard-artifact>`)
var planStepPattern = regexp.MustCompile(`(?m)^\s*(?:[-*]|\d+[.)])\s+`)

func analyzeSession(ctx context.Context, client *apiClient, manifest scenarioManifest, sessionID, titlePrefix string) (analysisReport, error) {
	session, err := findSession(ctx, client, sessionID, titlePrefix)
	if err != nil {
		return analysisReport{}, err
	}
	results := make([]promptAnalysisResult, 0, len(manifest.PromptPack))
	exactCount := 0
	reasonableCount := 0
	unacceptableCount := 0
	messageCursor := 0
	traceCursor := 0
	for _, prompt := range manifest.PromptPack {
		result := promptAnalysisResult{PromptID: prompt.ID, Prompt: prompt.Prompt}
		for ; messageCursor < len(session.Messages); messageCursor++ {
			if session.Messages[messageCursor].Role == "user" && strings.TrimSpace(session.Messages[messageCursor].Content) == strings.TrimSpace(prompt.Prompt) {
				result.UserMessageID = session.Messages[messageCursor].ID
				for next := messageCursor + 1; next < len(session.Messages); next++ {
					if session.Messages[next].Role != "assistant" {
						continue
					}
					result.AssistantMessageID = session.Messages[next].ID
					result.Answer = session.Messages[next].Content
					messageCursor = next
					break
				}
				break
			}
		}
		traceWindow, nextTraceCursor := traceWindowForPrompt(prompt.Prompt, session.Trace, traceCursor)
		if nextTraceCursor > traceCursor {
			traceCursor = nextTraceCursor
		}
		classifyPrompt(&result, prompt, traceWindow)
		if prompt.ExpectedArtifact != nil {
			verifyArtifact(&result, *prompt.ExpectedArtifact, session)
		}
		if prompt.ExpectedPlan != nil {
			verifyPlan(&result, *prompt.ExpectedPlan, session)
		}
		if prompt.ExpectedDraft != nil {
			verifyDraft(ctx, client, &result, *prompt.ExpectedDraft)
		}
		switch result.Classification {
		case "exact":
			exactCount++
		case "reasonable":
			reasonableCount++
		default:
			unacceptableCount++
		}
		results = append(results, result)
	}
	return analysisReport{
		Version:      "2026-04-02",
		Scenario:     manifest.Scenario,
		RunID:        manifest.RunID,
		BaseURL:      manifest.BaseURL,
		SessionID:    session.ID,
		SessionTitle: session.Title,
		GeneratedAt:  time.Now().UTC(),
		Results:      results,
		Summary: analysisSummary{
			ExactCount:        exactCount,
			ReasonableCount:   reasonableCount,
			UnacceptableCount: unacceptableCount,
		},
	}, nil
}

func findSession(ctx context.Context, client *apiClient, sessionID, titlePrefix string) (sessionTranscript, error) {
	if strings.TrimSpace(sessionID) != "" {
		return client.getSession(ctx, sessionID)
	}
	items, err := client.listSessions(ctx)
	if err != nil {
		return sessionTranscript{}, err
	}
	for _, item := range items {
		if strings.HasPrefix(strings.TrimSpace(item.Title), strings.TrimSpace(titlePrefix)) {
			return client.getSession(ctx, item.ID)
		}
	}
	return sessionTranscript{}, fmt.Errorf("session not found for title prefix %q", titlePrefix)
}

func classifyPrompt(result *promptAnalysisResult, prompt promptExpectation, trace []sessionTraceEvent) {
	answer := strings.ToLower(result.Answer)
	if strings.TrimSpace(answer) == "" {
		result.Classification = "unacceptable"
		result.Investigation = "No assistant answer was found after the matching user prompt."
		return
	}
	criticalMissing := false
	for _, fact := range prompt.RequiredFacts {
		matched := true
		for _, check := range fact.Checks {
			if !containsCheck(answer, check) {
				matched = false
				break
			}
		}
		if matched {
			result.MatchedFacts = append(result.MatchedFacts, fact.Key)
		} else {
			result.MissingFacts = append(result.MissingFacts, fact.Key)
			if fact.Severity == "critical" {
				criticalMissing = true
			}
		}
	}
	for _, forbidden := range prompt.ForbiddenPhrases {
		if containsContradiction(answer, forbidden) {
			result.Contradictions = append(result.Contradictions, forbidden)
		}
	}
	if len(result.MatchedFacts) > 0 && len(result.Contradictions) > 0 {
		filtered := result.Contradictions[:0]
		for _, contradiction := range result.Contradictions {
			if isNumericLike(normalizeComparable(contradiction)) {
				continue
			}
			filtered = append(filtered, contradiction)
		}
		result.Contradictions = filtered
	}
	result.ToolCallCount = countToolCalls(trace)
	switch {
	case len(result.Contradictions) > 0 || criticalMissing:
		result.Classification = "unacceptable"
	case len(result.MissingFacts) > 0:
		result.Classification = "reasonable"
	default:
		result.Classification = "exact"
	}
	result.Investigation = investigateResult(*result)
}

func verifyDraft(ctx context.Context, client *apiClient, result *promptAnalysisResult, expected draftExpectation) {
	items, err := client.listUIDocuments(ctx, expected.DocumentType, true)
	if err != nil {
		result.Classification = "unacceptable"
		result.MissingFacts = append(result.MissingFacts, "draft_lookup_failed")
		result.Investigation = "The answer required a draft artifact, but draft verification failed while listing documents."
		return
	}
	combinedPayloadText := ""
	combinedMatched := false
	for _, item := range items {
		if strings.ToLower(strings.TrimSpace(item.Header.Status)) != "draft" {
			continue
		}
		title := strings.ToLower(strings.TrimSpace(stringValue(item.Body.Payload["title"])))
		if len(expected.TitleChecks) > 0 && !allChecksMatch(title, expected.TitleChecks) {
			continue
		}
		payloadText := strings.ToLower(strings.TrimSpace(flattenPayloadText(item.Body.Payload)))
		combinedPayloadText += " " + payloadText
		if !allChecksMatch(payloadText, expected.PayloadChecks) {
			continue
		}
		combinedMatched = true
		result.DraftVerified = true
		result.DraftDocumentID = strings.TrimSpace(item.Header.ID)
		result.MissingFacts = removeFactKey(result.MissingFacts, "draft_title")
		if len(result.Contradictions) == 0 {
			criticalMissing := false
			for _, missing := range result.MissingFacts {
				if missing == "draft_created" || missing == "draft_title" {
					criticalMissing = true
					break
				}
			}
			switch {
			case !criticalMissing && len(result.MissingFacts) == 0:
				result.Classification = "exact"
			case !criticalMissing:
				result.Classification = "reasonable"
			}
		}
		return
	}
	if !combinedMatched && len(expected.PayloadChecks) > 0 && allChecksMatch(strings.ToLower(strings.TrimSpace(combinedPayloadText)), expected.PayloadChecks) {
		result.DraftVerified = true
		result.MissingFacts = removeFactKey(result.MissingFacts, "draft_title")
		if len(result.Contradictions) == 0 {
			criticalMissing := false
			for _, missing := range result.MissingFacts {
				if missing == "draft_created" || missing == "draft_title" {
					criticalMissing = true
					break
				}
			}
			switch {
			case !criticalMissing && len(result.MissingFacts) == 0:
				result.Classification = "exact"
			case !criticalMissing:
				result.Classification = "reasonable"
			}
		}
		return
	}
	result.DraftVerified = false
	result.Classification = "unacceptable"
	result.MissingFacts = append(result.MissingFacts, "expected_draft_document")
	result.Investigation = "The answer did not result in the expected draft artifact, or the created draft did not contain the required recommendation details."
}

func verifyArtifact(result *promptAnalysisResult, expected artifactExpectation, session sessionTranscript) {
	artifacts := extractArtifactsFromAnswer(result.Answer)
	if len(artifacts) == 0 && len(session.Artifacts) > 0 {
		for _, item := range session.Artifacts {
			artifacts = append(artifacts, map[string]any{
				"kind":     item.Kind,
				"title":    item.Title,
				"metadata": item.Metadata,
			})
		}
	}
	matchedArtifacts := 0
	combinedWidgetKeys := make([]string, 0)
	for _, artifact := range artifacts {
		kind := strings.TrimSpace(stringValue(artifact["kind"]))
		if strings.TrimSpace(expected.Kind) != "" && kind != strings.TrimSpace(expected.Kind) {
			continue
		}
		title := strings.ToLower(strings.TrimSpace(stringValue(artifact["title"])))
		if !allChecksMatch(title, expected.TitleChecks) {
			continue
		}
		metadata := mapValue(artifact, "metadata")
		widgets := anySlice(firstNonNil(artifact["widgets"], metadata["widgets"]))
		if len(widgets) == 0 {
			if widget := mapValue(metadata, "widget"); len(widget) > 0 {
				widgets = []any{widget}
			} else if widget := mapValue(artifact, "widget"); len(widget) > 0 {
				widgets = []any{widget}
			}
		}
		if expected.MinWidgets > 0 && len(widgets) < expected.MinWidgets {
			continue
		}
		widgetKeys := artifactWidgetKeys(widgets)
		matchedArtifacts++
		combinedWidgetKeys = append(combinedWidgetKeys, widgetKeys...)
		result.ArtifactKind = kind
	}
	if matchedArtifacts > 0 {
		if expected.MinArtifacts > 0 && matchedArtifacts < expected.MinArtifacts {
			result.ArtifactVerified = false
			result.Classification = "unacceptable"
			result.MissingFacts = append(result.MissingFacts, "expected_dashboard_artifact")
			result.Investigation = "The answer included dashboard artifacts, but not enough of them for the requested evidence set."
			return
		}
		if len(expected.WidgetKeys) > 0 && !allChecksMatch(strings.ToLower(strings.Join(combinedWidgetKeys, " ")), expected.WidgetKeys) {
			result.ArtifactVerified = false
			result.Classification = "unacceptable"
			result.MissingFacts = append(result.MissingFacts, "expected_dashboard_artifact")
			result.Investigation = "The answer included dashboard artifacts, but they did not contain the required widget set."
			return
		}
		result.ArtifactVerified = true
		return
	}
	result.ArtifactVerified = false
	result.Classification = "unacceptable"
	result.MissingFacts = append(result.MissingFacts, "expected_dashboard_artifact")
	result.Investigation = "The answer did not include the expected dashboard artifact block, or the artifact did not contain the required widget set."
}

func verifyPlan(result *promptAnalysisResult, expected planExpectation, session sessionTranscript) {
	answer := strings.TrimSpace(result.Answer)
	stepCount := len(planStepPattern.FindAllString(answer, -1))
	if stepCount == 0 {
		stepCount = len(session.CurrentPlan)
	}
	result.PlanStepCount = stepCount
	combined := strings.ToLower(strings.TrimSpace(answer))
	if combined == "" && len(session.CurrentPlan) > 0 {
		parts := make([]string, 0, len(session.CurrentPlan))
		for _, item := range session.CurrentPlan {
			parts = append(parts, item.Content)
		}
		combined = strings.ToLower(strings.Join(parts, " "))
	}
	if expected.MinSteps > 0 && stepCount < expected.MinSteps {
		result.PlanVerified = false
		result.Classification = "unacceptable"
		result.MissingFacts = append(result.MissingFacts, "expected_plan_steps")
		result.Investigation = "The planning answer did not produce enough explicit steps for the requested plan."
		return
	}
	if !allChecksMatch(combined, expected.ContentChecks) {
		result.PlanVerified = false
		result.Classification = "unacceptable"
		result.MissingFacts = append(result.MissingFacts, "expected_plan_content")
		result.Investigation = "The planning answer did not cover the expected plan focus areas."
		return
	}
	result.PlanVerified = true
}

func extractArtifactsFromAnswer(answer string) []map[string]any {
	matches := dashboardArtifactPattern.FindAllStringSubmatch(answer, -1)
	if len(matches) == 0 {
		return nil
	}
	artifacts := make([]map[string]any, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(strings.TrimSpace(match[1])), &payload); err != nil {
			continue
		}
		artifacts = append(artifacts, payload)
	}
	return artifacts
}

func artifactWidgetKeys(items []any) []string {
	keys := make([]string, 0, len(items))
	for _, item := range items {
		record, _ := item.(map[string]any)
		if record == nil {
			continue
		}
		key := stringValue(record["widget_key"])
		if key == "" {
			key = stringValue(mapValue(record, "definition")["key"])
		}
		if key == "" {
			continue
		}
		keys = append(keys, strings.ToLower(strings.TrimSpace(key)))
	}
	return keys
}

func anySlice(value any) []any {
	items, _ := value.([]any)
	return items
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func traceWindowForPrompt(prompt string, trace []sessionTraceEvent, cursor int) ([]sessionTraceEvent, int) {
	needle := strings.TrimSpace(prompt)
	if needle == "" {
		return nil, cursor
	}
	start := -1
	for index := max(cursor, 0); index < len(trace); index++ {
		if trace[index].Kind != "user_message" {
			continue
		}
		if strings.TrimSpace(tracePayloadString(trace[index].Payload, "content")) != needle {
			continue
		}
		start = index
		break
	}
	if start == -1 {
		return nil, cursor
	}
	end := len(trace)
	for index := start + 1; index < len(trace); index++ {
		if trace[index].Kind == "user_message" {
			end = index
			break
		}
	}
	return trace[start:end], end
}

func tracePayloadString(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	value, ok := payload[key]
	if !ok {
		return ""
	}
	text, _ := value.(string)
	return text
}

func countToolCalls(trace []sessionTraceEvent) int {
	total := 0
	for _, event := range trace {
		updateKind := strings.TrimSpace(tracePayloadString(event.Payload, "update_kind"))
		if event.Kind == "tool_call" || event.Kind == "tool_call_update" || updateKind == "tool_call" || updateKind == "tool_call_update" || strings.HasPrefix(updateKind, "tool_call_") {
			total++
		}
	}
	return total
}

func allChecksMatch(answer string, checks []string) bool {
	for _, check := range checks {
		if !containsCheck(answer, check) {
			return false
		}
	}
	return true
}

func flattenPayloadText(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, flattenPayloadText(item))
		}
		return strings.Join(parts, " ")
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, flattenPayloadText(item))
		}
		return strings.Join(parts, " ")
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

func investigateResult(result promptAnalysisResult) string {
	switch result.Classification {
	case "exact":
		if result.ToolCallCount > 0 {
			return "The answer matched all required facts and the trace showed live tool activity, so this looks like a healthy retrieval-and-summarization turn."
		}
		return "The answer matched all required facts. The transcript does not prove fresh tool usage for this turn, but the output is still consistent with the seeded ground truth."
	case "reasonable":
		if result.ToolCallCount == 0 {
			return "The core answer stayed close to the expected facts, but the session trace did not show tool calls. This suggests the model may have answered from prior context instead of fresh retrieval."
		}
		return "The answer stayed materially correct but omitted at least one non-critical fact. This is still usable, but the summarization was looser than the rubric expects."
	default:
		if result.ToolCallCount == 0 {
			return "The answer missed critical facts without any visible tool activity. The most likely cause is no retrieval rather than a bad seed."
		}
		if len(result.Contradictions) > 0 {
			return "The answer included a contradiction against seeded facts even though tools were available. This points to either incomplete retrieval, stale context carry-over, or poor synthesis."
		}
		return "The answer fell short of the rubric on critical facts. The next check should be whether the prompt was ambiguous or the tool selection missed the relevant records."
	}
}

func containsCheck(answer, check string) bool {
	needle := strings.ToLower(strings.TrimSpace(check))
	if needle == "" {
		return true
	}
	normalizedAnswer := normalizeComparable(answer)
	normalizedNeedle := normalizeComparable(needle)
	if isNumericLike(normalizedNeedle) {
		if numericCheckMatches(answer, normalizedNeedle) {
			return true
		}
		return regexp.MustCompile(`(^|[^0-9])`+regexp.QuoteMeta(normalizedNeedle)+`([^0-9]|$)`).FindStringIndex(normalizedAnswer) != nil
	}
	if strings.Contains(normalizedAnswer, normalizedNeedle) {
		return true
	}
	relaxedAnswer := stripRunIDSuffixes(normalizedAnswer)
	relaxedNeedle := stripRunIDSuffixes(normalizedNeedle)
	if numericCheckMatches(wordNumberNormalized(answer), relaxedNeedle) {
		return true
	}
	if strings.Contains(relaxedAnswer, relaxedNeedle) {
		return true
	}
	return strings.Contains(normalizeComparable(wordNumberNormalized(answer)), normalizeComparable(wordNumberNormalized(needle)))
}

func containsContradiction(answer, forbidden string) bool {
	needle := strings.ToLower(strings.TrimSpace(forbidden))
	if needle == "" || !containsCheck(answer, needle) {
		return false
	}
	for _, prefix := range []string{"no ", "not ", "without ", "none ", "zero ", "not been ", "was not ", "were not ", "did not ", "has not ", "have not "} {
		if strings.Contains(normalizeComparable(answer), normalizeComparable(prefix+needle)) {
			return false
		}
	}
	if strings.Contains(needle, "pending approval") && (strings.Contains(normalizeComparable(answer), "no pending approv") || strings.Contains(normalizeComparable(answer), "pending approval 0") || strings.Contains(normalizeComparable(answer), "pending approvals 0")) {
		return false
	}
	return true
}

func normalizeComparable(value string) string {
	lower := strings.ToLower(value)
	runes := []rune(lower)
	var b strings.Builder
	for i, r := range runes {
		prevDigit := i > 0 && unicode.IsDigit(runes[i-1])
		nextDigit := i+1 < len(runes) && unicode.IsDigit(runes[i+1])
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune(r)
		case (r == ',' || r == '.') && prevDigit && nextDigit:
			continue
		case r == ',', r == '.', r == ':', r == ';', r == '|', r == '-', r == '_', r == '/':
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func isNumericLike(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func numericCheckMatches(answer, needle string) bool {
	target, err := strconv.ParseFloat(needle, 64)
	if err != nil {
		return false
	}
	for _, match := range regexp.MustCompile(`\d[\d,.]*`).FindAllString(answer, -1) {
		normalized := strings.ReplaceAll(match, ",", "")
		value, parseErr := strconv.ParseFloat(normalized, 64)
		if parseErr != nil {
			continue
		}
		if value == target {
			return true
		}
	}
	return false
}

func wordNumberNormalized(value string) string {
	replacer := strings.NewReplacer(
		" zero ", " 0 ",
		" one ", " 1 ",
		" two ", " 2 ",
		" three ", " 3 ",
		" four ", " 4 ",
		" five ", " 5 ",
		" six ", " 6 ",
		" seven ", " 7 ",
		" eight ", " 8 ",
		" nine ", " 9 ",
		" ten ", " 10 ",
	)
	return replacer.Replace(" " + strings.ToLower(value) + " ")
}

func stripRunIDSuffixes(value string) string {
	return regexp.MustCompile(`\b20\d{6} \d{6}\b|\b20\d{6}-\d{6}\b`).ReplaceAllString(value, "")
}

func removeFactKey(items []string, key string) []string {
	if len(items) == 0 {
		return items
	}
	filtered := items[:0]
	for _, item := range items {
		if item == key {
			continue
		}
		filtered = append(filtered, item)
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}
