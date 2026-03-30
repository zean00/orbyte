package application

import (
	"fmt"
	"strings"
	"time"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/shared"
)

type FinanceManualJournalCoreService struct {
	documents *document.Service
	finance   *FinanceReportingCoreService
	periodEnd *FinancePeriodEndCoreService
}

func NewFinanceManualJournalCoreService(documents *document.Service, finance *FinanceReportingCoreService) *FinanceManualJournalCoreService {
	return &FinanceManualJournalCoreService{documents: documents, finance: finance}
}

func (s *FinanceManualJournalCoreService) SetPeriodEnd(periodEnd *FinancePeriodEndCoreService) {
	s.periodEnd = periodEnd
}

func (s *FinanceManualJournalCoreService) IsManualJournal(record document.Record) bool {
	return record.Header.Type == "ledger_posting" && strings.TrimSpace(textValue(record.Body.Payload["journal_source_kind"])) == "manual"
}

func (s *FinanceManualJournalCoreService) IsManualJournalPayload(documentType string, payload map[string]any) bool {
	if strings.TrimSpace(documentType) != "ledger_posting" {
		return false
	}
	kind := strings.TrimSpace(textValue(payload["journal_source_kind"]))
	return kind == "" || kind == "manual"
}

func (s *FinanceManualJournalCoreService) NormalizePayload(payload map[string]any) map[string]any {
	next := document.NormalizePayload(cloneMap(payload))
	kind := strings.TrimSpace(textValue(next["journal_source_kind"]))
	if kind == "" {
		kind = "manual"
	}
	if kind != "manual" {
		return next
	}
	next = normalizeManualJournalLines(next)
	next["journal_source_kind"] = "manual"
	next["requires_approval"] = true
	if strings.TrimSpace(textValue(next["manual_journal_type"])) == "" {
		next["manual_journal_type"] = "other"
	}
	if strings.TrimSpace(textValue(next["reversal_status"])) == "" {
		next["reversal_status"] = "available"
	}
	return next
}

func (s *FinanceManualJournalCoreService) ValidateAction(record document.Record, action, actorID string) error {
	if !s.IsManualJournal(record) {
		return nil
	}
	payload := record.Body.Payload
	switch strings.TrimSpace(action) {
	case "submit":
		if strings.TrimSpace(textValue(payload["posting_date"])) == "" {
			return shared.Validation("manual journal posting date is required")
		}
		lines := recordList(payload["journal_lines"])
		if len(lines) == 0 {
			return shared.Validation("manual journal requires at least one journal line")
		}
		debitTotal := 0.0
		creditTotal := 0.0
		for _, line := range lines {
			debitTotal = roundMoney(debitTotal + roundMoney(numberValue(line["debit"])))
			creditTotal = roundMoney(creditTotal + roundMoney(numberValue(line["credit"])))
		}
		if roundMoney(debitTotal) <= 0 && roundMoney(creditTotal) <= 0 {
			return shared.Validation("manual journal total must be greater than zero")
		}
		if roundMoney(debitTotal) != roundMoney(creditTotal) {
			return shared.Validation("manual journal lines must balance")
		}
	case "approve":
		if actorID != "" {
			requesterID := firstNonEmptyString(textValue(payload["submitted_by"]), record.Header.CreatedBy)
			if requesterID != "" && requesterID == actorID {
				return shared.Forbidden("manual journal approval requires a different user than the creator")
			}
		}
		if s.finance != nil {
			if err := s.finance.ValidatePostingDateOpen(record.Header.OrganizationID, record.Header.LocationID, textValue(payload["posting_date"])); err != nil {
				return err
			}
		}
	case "reopen":
		if strings.TrimSpace(record.Header.Status) == "posted" {
			return shared.Conflict("posted manual journals cannot be reopened; use reversal or correction journal")
		}
	}
	return nil
}

func (s *FinanceManualJournalCoreService) HandleAction(record document.Record, action, actorID, note string) error {
	if !s.IsManualJournal(record) {
		return nil
	}
	current, err := s.documents.Get(record.Header.ID)
	if err == nil {
		record = current
	}
	payload := cloneMap(record.Body.Payload)
	now := time.Now().UTC()
	changed := false
	switch strings.TrimSpace(action) {
	case "submit":
		payload["requires_approval"] = true
		payload["submitted_by"] = actorID
		payload["submitted_at"] = now.Format(time.RFC3339)
		if strings.TrimSpace(note) != "" {
			payload["submission_note"] = strings.TrimSpace(note)
		}
		if strings.TrimSpace(textValue(payload["reversal_status"])) == "" {
			payload["reversal_status"] = "available"
		}
		changed = true
	case "approve":
		payload["requires_approval"] = true
		payload["approved_by"] = actorID
		payload["approved_at"] = now.Format(time.RFC3339)
		if strings.TrimSpace(note) != "" {
			payload["approval_note"] = strings.TrimSpace(note)
		}
		if strings.TrimSpace(textValue(payload["reversal_status"])) == "" {
			payload["reversal_status"] = "available"
		}
		changed = true
		if sourceID := strings.TrimSpace(textValue(payload["correction_of_posting_id"])); sourceID != "" {
			if err := s.markSourceCorrected(sourceID, record.Header.ID, actorID); err != nil {
				return err
			}
		}
	case "reject":
		if strings.TrimSpace(note) != "" {
			payload["approval_note"] = strings.TrimSpace(note)
			changed = true
		}
	}
	if !changed {
		return nil
	}
	record.Body.Payload = document.NormalizePayload(payload)
	record.Body.ContentHash = document.ContentHash(record.Body.Payload)
	record.Header.Version++
	record.Header.ETag = fmt.Sprintf("%s:%d", record.Header.ID, record.Header.Version)
	record.Header.UpdatedAt = now
	record.Header.UpdatedBy = actorID
	return s.documents.Save(record)
}

func (s *FinanceManualJournalCoreService) CreateCorrectionJournal(postingID, actorID, organizationID, locationID string) (document.Record, error) {
	if s.documents == nil {
		return document.Record{}, shared.Validation("documents are unavailable")
	}
	source, err := s.documents.Get(strings.TrimSpace(postingID))
	if err != nil {
		return document.Record{}, err
	}
	if source.Header.Type != "ledger_posting" {
		return document.Record{}, shared.Validation("only ledger postings can be corrected")
	}
	if strings.TrimSpace(source.Header.OrganizationID) != strings.TrimSpace(organizationID) {
		return document.Record{}, shared.Forbidden("posting is outside the current organization scope")
	}
	if strings.TrimSpace(locationID) != "" && strings.TrimSpace(source.Header.LocationID) != strings.TrimSpace(locationID) {
		return document.Record{}, shared.Forbidden("posting is outside the current location scope")
	}
	if strings.TrimSpace(source.Header.Status) != "posted" {
		return document.Record{}, shared.Conflict("only posted journals can be corrected")
	}
	sourcePayload := cloneMap(source.Body.Payload)
	payload := s.NormalizePayload(map[string]any{
		"source_document_type":     "ledger_posting",
		"source_document_id":       source.Header.ID,
		"posting_date":             textValue(sourcePayload["posting_date"]),
		"currency_code":            firstNonEmptyString(textValue(sourcePayload["currency_code"]), source.Header.TotalAmount.Currency, "IDR"),
		"posting_rule_key":         "manual_correction",
		"journal_source_kind":      "manual",
		"manual_journal_type":      "correction",
		"supporting_reference":     firstNonEmptyString(source.Header.Number, source.Header.ID),
		"correction_of_posting_id": source.Header.ID,
		"journal_lines":            recordList(sourcePayload["journal_lines"]),
		"notes":                    fmt.Sprintf("Correction journal for %s", firstNonEmptyString(source.Header.Number, source.Header.ID)),
	})
	record, err := s.documents.Create("ledger_posting", source.Header.OrganizationID, source.Header.LocationID, actorID, payload)
	if err != nil {
		return document.Record{}, err
	}
	record.Header.TotalAmount = shared.Money{
		Currency:    firstNonEmptyString(textValue(payload["currency_code"]), "IDR"),
		AmountMinor: moneyMinor(numberValue(payload["total_amount"])),
	}
	if err := s.documents.Save(record); err != nil {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(record.Header.ID, source.Header.ID, "posting_for", map[string]any{"posting_reason": "correction_of"}); err != nil && !isConflict(err) {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(source.Header.ID, record.Header.ID, "posting_for", map[string]any{"posting_reason": "correction_pair"}); err != nil && !isConflict(err) {
		return document.Record{}, err
	}
	return s.documents.Get(record.Header.ID)
}

func (s *FinanceManualJournalCoreService) markSourceCorrected(sourceID, correctedByPostingID, actorID string) error {
	if s.documents == nil || strings.TrimSpace(sourceID) == "" {
		return nil
	}
	source, err := s.documents.Get(strings.TrimSpace(sourceID))
	if err != nil {
		return err
	}
	payload := cloneMap(source.Body.Payload)
	payload["corrected_by_posting_id"] = strings.TrimSpace(correctedByPostingID)
	source.Body.Payload = document.NormalizePayload(payload)
	source.Body.ContentHash = document.ContentHash(source.Body.Payload)
	source.Header.Version++
	source.Header.ETag = fmt.Sprintf("%s:%d", source.Header.ID, source.Header.Version)
	source.Header.UpdatedAt = time.Now().UTC()
	source.Header.UpdatedBy = actorID
	return s.documents.Save(source)
}

func normalizeManualJournalLines(payload map[string]any) map[string]any {
	next := document.NormalizePayload(cloneMap(payload))
	rows := recordList(next["journal_lines"])
	normalizedRows := make([]map[string]any, 0, len(rows))
	debitTotal := 0.0
	creditTotal := 0.0
	for _, row := range rows {
		normalized := cloneMap(row)
		debit := roundMoney(numberValue(normalized["debit"]))
		credit := roundMoney(numberValue(normalized["credit"]))
		normalized["debit"] = debit
		normalized["credit"] = credit
		debitTotal += debit
		creditTotal += credit
		normalizedRows = append(normalizedRows, normalized)
	}
	next["journal_lines"] = normalizedRows
	next["total_amount"] = roundMoney(maxFloat(debitTotal, creditTotal))
	return next
}
