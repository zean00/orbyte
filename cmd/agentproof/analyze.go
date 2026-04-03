package main

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

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
	return strings.Contains(normalizedAnswer, normalizedNeedle)
}

func containsContradiction(answer, forbidden string) bool {
	needle := strings.ToLower(strings.TrimSpace(forbidden))
	if needle == "" || !containsCheck(answer, needle) {
		return false
	}
	for _, prefix := range []string{"no ", "not ", "without ", "none ", "zero "} {
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
