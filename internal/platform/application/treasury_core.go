package application

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"orbyte/internal/platform/config"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/shared"
)

const (
	defaultBankFeeExpenseAccount     = "6300-BANK-FEE"
	defaultBankInterestIncomeAccount = "4800-BANK-INT"
	defaultTransferClearingAccount   = "1095-TRANSFER-CLEARING"
	defaultTreasurySuspenseAccount   = "1099-TREASURY-SUSPENSE"
)

type TreasuryCashPositionRow struct {
	TreasuryAccountID    string  `json:"treasury_account_id"`
	AccountCode          string  `json:"account_code"`
	AccountName          string  `json:"account_name"`
	TreasuryType         string  `json:"treasury_type"`
	CurrencyCode         string  `json:"currency_code"`
	GLAccountCode        string  `json:"gl_account_code"`
	BookBalance          float64 `json:"book_balance"`
	StatementBalance     float64 `json:"statement_balance"`
	UnreconciledAmount   float64 `json:"unreconciled_amount"`
	LastStatementDate    string  `json:"last_statement_date,omitempty"`
	LastReconciliationID string  `json:"last_reconciliation_id,omitempty"`
}

type TreasuryCashPositionReport struct {
	OrganizationID string                    `json:"organization_id"`
	LocationID     string                    `json:"location_id,omitempty"`
	AsOfDate       string                    `json:"as_of_date,omitempty"`
	Rows           []TreasuryCashPositionRow `json:"rows"`
	Totals         map[string]float64        `json:"totals"`
}

type TreasuryClearingBalanceRow struct {
	SourceKind    string  `json:"source_kind"`
	AccountCode   string  `json:"account_code"`
	ReferenceID   string  `json:"reference_id,omitempty"`
	ReferenceNo   string  `json:"reference_number,omitempty"`
	OpenAmount    float64 `json:"open_amount"`
	Status        string  `json:"status,omitempty"`
	EffectiveDate string  `json:"effective_date,omitempty"`
}

type TreasuryClearingBalanceReport struct {
	OrganizationID string                       `json:"organization_id"`
	LocationID     string                       `json:"location_id,omitempty"`
	AsOfDate       string                       `json:"as_of_date,omitempty"`
	Rows           []TreasuryClearingBalanceRow `json:"rows"`
	Totals         map[string]float64           `json:"totals"`
}

type TreasuryBankReconciliationLine struct {
	StatementLineID   string                       `json:"statement_line_id"`
	StatementDate     string                       `json:"statement_date"`
	Description       string                       `json:"description"`
	Reference         string                       `json:"reference"`
	Amount            float64                      `json:"amount"`
	MatchedAmount     float64                      `json:"matched_amount"`
	RemainingAmount   float64                      `json:"remaining_amount"`
	MatchStatus       string                       `json:"match_status"`
	MatchedSourceType string                       `json:"matched_source_type,omitempty"`
	MatchedSourceID   string                       `json:"matched_source_id,omitempty"`
	Candidates        []TreasuryBankMatchCandidate `json:"candidates,omitempty"`
}

type TreasuryBankReconciliationReport struct {
	ReconciliationID      string                           `json:"reconciliation_id,omitempty"`
	TreasuryAccountID     string                           `json:"treasury_account_id,omitempty"`
	StatementID           string                           `json:"statement_id,omitempty"`
	OrganizationID        string                           `json:"organization_id"`
	LocationID            string                           `json:"location_id,omitempty"`
	AsOfDate              string                           `json:"as_of_date,omitempty"`
	BookBalance           float64                          `json:"book_balance"`
	StatementBalance      float64                          `json:"statement_balance"`
	MatchedAmount         float64                          `json:"matched_amount"`
	OutstandingBookAmount float64                          `json:"outstanding_book_amount"`
	Difference            float64                          `json:"difference"`
	Lines                 []TreasuryBankReconciliationLine `json:"lines"`
	Exceptions            []model.Record                   `json:"exceptions"`
}

type TreasuryBankMatchCandidate struct {
	SourceType    string   `json:"source_type"`
	SourceID      string   `json:"source_id"`
	Reference     string   `json:"reference,omitempty"`
	Description   string   `json:"description,omitempty"`
	AccountCode   string   `json:"account_code,omitempty"`
	EffectiveDate string   `json:"effective_date,omitempty"`
	Amount        float64  `json:"amount"`
	Score         float64  `json:"score"`
	Reasons       []string `json:"reasons,omitempty"`
}

type TreasuryImportPreview struct {
	TreasuryAccountID string           `json:"treasury_account_id,omitempty"`
	TemplateID        string           `json:"template_id,omitempty"`
	PresetKey         string           `json:"preset_key,omitempty"`
	StatementValues   map[string]any   `json:"statement_values"`
	Lines             []map[string]any `json:"lines"`
	Warnings          []string         `json:"warnings"`
	RowCount          int              `json:"row_count"`
	DuplicateCount    int              `json:"duplicate_count"`
}

type TreasuryTransferRegisterRow struct {
	TransferID      string  `json:"transfer_id"`
	TransferDate    string  `json:"transfer_date"`
	FromAccountID   string  `json:"from_account_id"`
	FromAccountCode string  `json:"from_account_code"`
	ToAccountID     string  `json:"to_account_id"`
	ToAccountCode   string  `json:"to_account_code"`
	Amount          float64 `json:"amount"`
	Reference       string  `json:"reference,omitempty"`
	Status          string  `json:"status"`
	PostingID       string  `json:"posting_id,omitempty"`
}

type TreasuryTransferRegisterReport struct {
	OrganizationID string                        `json:"organization_id"`
	LocationID     string                        `json:"location_id,omitempty"`
	AsOfDate       string                        `json:"as_of_date,omitempty"`
	Rows           []TreasuryTransferRegisterRow `json:"rows"`
	Totals         map[string]float64            `json:"totals"`
}

type TreasuryExceptionReport struct {
	OrganizationID string             `json:"organization_id"`
	LocationID     string             `json:"location_id,omitempty"`
	AsOfDate       string             `json:"as_of_date,omitempty"`
	Items          []model.Record     `json:"items"`
	Totals         map[string]float64 `json:"totals"`
}

type TreasuryCoreService struct {
	documents *document.Service
	models    *model.Service
	config    *config.Service
	finance   *FinanceReportingCoreService
	retail    *RetailFinanceCoreService
}

func NewTreasuryCoreService(documents *document.Service, models *model.Service, configSvc *config.Service, finance *FinanceReportingCoreService, retail *RetailFinanceCoreService) *TreasuryCoreService {
	return &TreasuryCoreService{documents: documents, models: models, config: configSvc, finance: finance, retail: retail}
}

func (s *TreasuryCoreService) PreviewStatementImport(organizationID, locationID, treasuryAccountID string, payload map[string]any, rawCSV string) (TreasuryImportPreview, error) {
	account, err := s.models.Get("treasury_account", strings.TrimSpace(treasuryAccountID))
	if err != nil {
		return TreasuryImportPreview{}, err
	}
	if err := s.validateTreasuryScope(account, organizationID, locationID); err != nil {
		return TreasuryImportPreview{}, err
	}
	statementValues, lines, warnings, duplicateCount, err := s.previewStatementImport(account, organizationID, locationID, payload, rawCSV)
	if err != nil {
		return TreasuryImportPreview{}, err
	}
	return TreasuryImportPreview{
		TreasuryAccountID: account.ID,
		TemplateID:        strings.TrimSpace(textValue(payload["bank_import_template_id"])),
		PresetKey:         strings.TrimSpace(textValue(payload["preset_key"])),
		StatementValues:   statementValues,
		Lines:             lines,
		Warnings:          warnings,
		RowCount:          len(lines),
		DuplicateCount:    duplicateCount,
	}, nil
}

func (s *TreasuryCoreService) ImportStatementCSV(organizationID, locationID, treasuryAccountID, actorID string, payload map[string]any, rawCSV string) (map[string]any, error) {
	account, err := s.models.Get("treasury_account", strings.TrimSpace(treasuryAccountID))
	if err != nil {
		return nil, err
	}
	if err := s.validateTreasuryScope(account, organizationID, locationID); err != nil {
		return nil, err
	}
	statementValues, linesPayload, warnings, duplicateCount, err := s.previewStatementImport(account, organizationID, locationID, payload, rawCSV)
	if err != nil {
		return nil, err
	}
	statement, err := s.models.Create("bank_statement", actorID, statementValues)
	if err != nil {
		return nil, err
	}
	lines := make([]model.Record, 0, len(linesPayload))
	for _, lineValues := range linesPayload {
		lineValues = cloneMap(lineValues)
		lineValues["bank_statement_id"] = statement.ID
		line, err := s.models.Create("bank_statement_line", actorID, lineValues)
		if err != nil {
			return nil, err
		}
		lines = append(lines, line)
	}
	if _, ok := s.models.Definition("bank_statement_import_run"); ok {
		_, _ = s.models.Create("bank_statement_import_run", actorID, map[string]any{
			"organization_id":         organizationID,
			"location_id":             locationID,
			"treasury_account_id":     account.ID,
			"bank_statement_id":       statement.ID,
			"bank_import_template_id": strings.TrimSpace(textValue(payload["bank_import_template_id"])),
			"preset_key":              strings.TrimSpace(textValue(payload["preset_key"])),
			"source_file_name":        textValue(payload["source_file_name"]),
			"row_count":               len(linesPayload),
			"duplicate_count":         duplicateCount,
			"warning_count":           len(warnings),
			"warnings_json":           marshalJSONString(warnings),
			"status":                  "imported",
		})
	}
	if numberValue(statement.Values["closing_balance"]) == 0 {
		if updated, err := s.refreshStatementBalance(statement.ID, actorID); err == nil {
			statement = updated
		}
	}
	return map[string]any{"statement": statement, "lines": lines, "warnings": warnings, "duplicate_count": duplicateCount}, nil
}

func (s *TreasuryCoreService) CreateManualStatement(organizationID, locationID, treasuryAccountID, actorID string, payload map[string]any) (map[string]any, error) {
	account, err := s.models.Get("treasury_account", strings.TrimSpace(treasuryAccountID))
	if err != nil {
		return nil, err
	}
	if err := s.validateTreasuryScope(account, organizationID, locationID); err != nil {
		return nil, err
	}
	statementValues := map[string]any{
		"organization_id":     organizationID,
		"location_id":         locationID,
		"treasury_account_id": account.ID,
		"statement_number":    firstNonEmptyString(textValue(payload["statement_number"]), posNumber("STMT")),
		"statement_date":      firstNonEmptyString(textValue(payload["statement_date"]), time.Now().UTC().Format("2006-01-02")),
		"from_date":           firstNonEmptyString(textValue(payload["from_date"]), textValue(payload["statement_date"])),
		"to_date":             firstNonEmptyString(textValue(payload["to_date"]), textValue(payload["statement_date"])),
		"opening_balance":     roundMoney(numberValue(payload["opening_balance"])),
		"closing_balance":     roundMoney(numberValue(payload["closing_balance"])),
		"import_method":       "manual",
		"status":              "opened",
	}
	statement, err := s.models.Create("bank_statement", actorID, statementValues)
	if err != nil {
		return nil, err
	}
	createdLines := []model.Record{}
	for _, line := range recordList(payload["lines"]) {
		lineValues := map[string]any{
			"organization_id":     organizationID,
			"location_id":         locationID,
			"bank_statement_id":   statement.ID,
			"treasury_account_id": account.ID,
			"statement_date":      firstNonEmptyString(textValue(line["statement_date"]), textValue(statement.Values["statement_date"])),
			"value_date":          textValue(line["value_date"]),
			"reference":           textValue(line["reference"]),
			"description":         textValue(line["description"]),
			"debit_amount":        roundMoney(numberValue(line["debit_amount"])),
			"credit_amount":       roundMoney(numberValue(line["credit_amount"])),
			"signed_amount":       roundMoney(resolveSignedStatementAmount(line)),
			"running_balance":     roundMoney(numberValue(line["running_balance"])),
			"matched_amount":      0.0,
			"remaining_amount":    roundMoney(maxFloat(resolveSignedStatementAmount(line), -resolveSignedStatementAmount(line))),
			"match_status":        "unmatched",
		}
		created, err := s.models.Create("bank_statement_line", actorID, lineValues)
		if err != nil {
			return nil, err
		}
		createdLines = append(createdLines, created)
	}
	if updated, err := s.refreshStatementBalance(statement.ID, actorID); err == nil {
		statement = updated
	}
	return map[string]any{"statement": statement, "lines": createdLines}, nil
}

func (s *TreasuryCoreService) SyncBankReconciliation(organizationID, locationID, statementID, actorID string) (model.Record, error) {
	statement, err := s.models.Get("bank_statement", strings.TrimSpace(statementID))
	if err != nil {
		return model.Record{}, err
	}
	if err := s.validateTreasuryScope(statement, organizationID, locationID); err != nil {
		return model.Record{}, err
	}
	report := s.BankReconciliation(organizationID, locationID, statement.ID)
	values := map[string]any{
		"organization_id":         organizationID,
		"location_id":             locationID,
		"treasury_account_id":     textValue(statement.Values["treasury_account_id"]),
		"bank_statement_id":       statement.ID,
		"reconciliation_date":     firstNonEmptyString(textValue(statement.Values["to_date"]), textValue(statement.Values["statement_date"])),
		"book_balance":            roundMoney(report.BookBalance),
		"statement_balance":       roundMoney(report.StatementBalance),
		"matched_amount":          roundMoney(report.MatchedAmount),
		"outstanding_book_amount": roundMoney(report.OutstandingBookAmount),
		"difference_amount":       roundMoney(report.Difference),
		"status":                  reconciliationStatusForDifference(report.Difference),
	}
	reconciliation, err := s.upsertTreasuryModel("bank_reconciliation", map[string]string{"bank_statement_id": statement.ID}, actorID, values)
	if err != nil {
		return model.Record{}, err
	}
	if err := s.syncStatementExceptions(reconciliation, statement, actorID); err != nil {
		return model.Record{}, err
	}
	if updated, err := s.models.Update("bank_statement", statement.ID, actorID, mergeModelValues(statement.Values, map[string]any{"status": "processing"}), statement.Version); err == nil {
		statement = updated
	}
	_ = statement
	return s.models.Get("bank_reconciliation", reconciliation.ID)
}

func (s *TreasuryCoreService) MatchStatementLine(reconciliationID, lineID, actorID string, payload map[string]any) (map[string]any, error) {
	reconciliation, err := s.models.Get("bank_reconciliation", strings.TrimSpace(reconciliationID))
	if err != nil {
		return nil, err
	}
	line, err := s.models.Get("bank_statement_line", strings.TrimSpace(lineID))
	if err != nil {
		return nil, err
	}
	if textValue(line.Values["bank_statement_id"]) != textValue(reconciliation.Values["bank_statement_id"]) {
		return nil, shared.Validation("statement line does not belong to reconciliation")
	}
	remaining := roundMoney(numberValue(line.Values["remaining_amount"]))
	amount := roundMoney(numberValue(payload["amount"]))
	if amount <= 0 || amount > remaining {
		amount = remaining
	}
	if amount <= 0 {
		return nil, shared.Validation("no remaining statement amount to match")
	}
	sourceType := strings.TrimSpace(textValue(payload["source_type"]))
	sourceID := strings.TrimSpace(textValue(payload["source_id"]))
	if sourceType == "" || sourceID == "" {
		return nil, shared.Validation("source_type and source_id are required")
	}
	sourceType = normalizeMatchedSourceType(sourceType)
	match, err := s.models.Create("bank_reconciliation_match", actorID, map[string]any{
		"organization_id":        textValue(reconciliation.Values["organization_id"]),
		"location_id":            textValue(reconciliation.Values["location_id"]),
		"bank_reconciliation_id": reconciliation.ID,
		"bank_statement_line_id": line.ID,
		"matched_source_type":    sourceType,
		"matched_source_id":      sourceID,
		"matched_amount":         amount,
		"match_kind":             firstNonEmptyString(textValue(payload["match_kind"]), "manual"),
		"notes":                  textValue(payload["notes"]),
	})
	if err != nil {
		return nil, err
	}
	lineValues := cloneMap(line.Values)
	matchedAmount := roundMoney(numberValue(lineValues["matched_amount"]) + amount)
	lineValues["matched_amount"] = matchedAmount
	lineValues["remaining_amount"] = roundMoney(maxFloat(numberValue(lineValues["signed_amount"]), -numberValue(lineValues["signed_amount"])) - matchedAmount)
	if roundMoney(numberValue(lineValues["remaining_amount"])) <= 0 {
		lineValues["remaining_amount"] = 0.0
		lineValues["match_status"] = "matched"
	} else {
		lineValues["match_status"] = "partial"
	}
	lineValues["matched_source_type"] = sourceType
	lineValues["matched_source_id"] = sourceID
	updatedLine, err := s.models.Update("bank_statement_line", line.ID, actorID, lineValues, line.Version)
	if err != nil {
		return nil, err
	}
	if sourceType == "pos_tender_settlement" && s.retail != nil {
		_, _ = s.retail.SettleTenderSettlement(sourceID, actorID, amount, firstNonEmptyString(textValue(updatedLine.Values["statement_date"]), time.Now().UTC().Format("2006-01-02")), firstNonEmptyString(textValue(updatedLine.Values["reference"]), updatedLine.ID), "matched from bank reconciliation")
	}
	reconciliation, err = s.SyncBankReconciliation(textValue(reconciliation.Values["organization_id"]), textValue(reconciliation.Values["location_id"]), textValue(reconciliation.Values["bank_statement_id"]), actorID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"match": match, "line": updatedLine, "reconciliation": reconciliation}, nil
}

func (s *TreasuryCoreService) ApproveBankReconciliation(reconciliationID, actorID string) (model.Record, error) {
	reconciliation, err := s.models.Get("bank_reconciliation", strings.TrimSpace(reconciliationID))
	if err != nil {
		return model.Record{}, err
	}
	if strings.EqualFold(textValue(reconciliation.Values["status"]), "closed") {
		return model.Record{}, shared.Conflict("bank reconciliation is already closed")
	}
	if roundMoney(numberValue(reconciliation.Values["difference_amount"])) != 0 {
		return model.Record{}, shared.Conflict("bank reconciliation cannot be approved while differences remain")
	}
	openExceptions, _, err := s.models.List("treasury_exception", model.Query{
		Page:     1,
		PageSize: 1,
		Filters: map[string]string{
			"bank_statement_id": textValue(reconciliation.Values["bank_statement_id"]),
			"status":            "pending",
		},
	})
	if err != nil && !isMissingModelDefinitionError(err) {
		return model.Record{}, err
	}
	if len(openExceptions) > 0 {
		return model.Record{}, shared.Conflict("bank reconciliation cannot be approved while treasury exceptions remain open")
	}
	values := cloneMap(reconciliation.Values)
	values["status"] = "closed"
	values["approved_by"] = actorID
	values["approved_at"] = time.Now().UTC().Format(time.RFC3339)
	updated, err := s.models.Update("bank_reconciliation", reconciliation.ID, actorID, values, reconciliation.Version)
	if err != nil {
		return model.Record{}, err
	}
	statementID := textValue(updated.Values["bank_statement_id"])
	if statementID != "" {
		if statement, err := s.models.Get("bank_statement", statementID); err == nil {
			_, _ = s.models.Update("bank_statement", statement.ID, actorID, mergeModelValues(statement.Values, map[string]any{"status": "completed"}), statement.Version)
		}
	}
	return updated, nil
}

func (s *TreasuryCoreService) CreateTransfer(organizationID, locationID, actorID string, payload map[string]any) (model.Record, error) {
	amount := roundMoney(numberValue(payload["amount"]))
	if amount <= 0 {
		return model.Record{}, shared.Validation("transfer amount must be greater than zero")
	}
	fromID := strings.TrimSpace(textValue(payload["from_treasury_account_id"]))
	toID := strings.TrimSpace(textValue(payload["to_treasury_account_id"]))
	if fromID == "" || toID == "" || fromID == toID {
		return model.Record{}, shared.Validation("from and to treasury accounts are required and must differ")
	}
	fromAccount, err := s.models.Get("treasury_account", fromID)
	if err != nil {
		return model.Record{}, err
	}
	toAccount, err := s.models.Get("treasury_account", toID)
	if err != nil {
		return model.Record{}, err
	}
	if err := s.validateTreasuryScope(fromAccount, organizationID, locationID); err != nil {
		return model.Record{}, err
	}
	if err := s.validateTreasuryScope(toAccount, organizationID, locationID); err != nil {
		return model.Record{}, err
	}
	return s.models.Create("treasury_transfer", actorID, map[string]any{
		"organization_id":          organizationID,
		"location_id":              locationID,
		"transfer_date":            firstNonEmptyString(textValue(payload["transfer_date"]), time.Now().UTC().Format("2006-01-02")),
		"from_treasury_account_id": fromID,
		"to_treasury_account_id":   toID,
		"from_account_code":        textValue(fromAccount.Values["account_code"]),
		"to_account_code":          textValue(toAccount.Values["account_code"]),
		"amount":                   amount,
		"reference":                textValue(payload["reference"]),
		"notes":                    textValue(payload["notes"]),
		"status":                   "draft",
	})
}

func (s *TreasuryCoreService) ApproveTransfer(transferID, actorID string) (map[string]any, error) {
	transfer, err := s.models.Get("treasury_transfer", strings.TrimSpace(transferID))
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(textValue(transfer.Values["status"]), "draft") {
		return nil, shared.Conflict("treasury transfer can only be approved from draft status")
	}
	if strings.TrimSpace(textValue(transfer.Values["posting_id"])) != "" {
		return nil, shared.Conflict("treasury transfer is already posted")
	}
	fromAccount, err := s.models.Get("treasury_account", textValue(transfer.Values["from_treasury_account_id"]))
	if err != nil {
		return nil, err
	}
	toAccount, err := s.models.Get("treasury_account", textValue(transfer.Values["to_treasury_account_id"]))
	if err != nil {
		return nil, err
	}
	postDate := firstNonEmptyString(textValue(transfer.Values["transfer_date"]), time.Now().UTC().Format("2006-01-02"))
	organizationID := textValue(transfer.Values["organization_id"])
	locationID := textValue(transfer.Values["location_id"])
	if s.finance != nil {
		if err := s.finance.ValidatePostingDateOpen(organizationID, locationID, postDate); err != nil {
			return nil, err
		}
	}
	amount := roundMoney(numberValue(transfer.Values["amount"]))
	payload := map[string]any{
		"posting_date":         postDate,
		"currency_code":        firstNonEmptyString(textValue(fromAccount.Values["currency_code"]), textValue(toAccount.Values["currency_code"]), "IDR"),
		"source_document_type": "treasury_transfer",
		"source_document_id":   transfer.ID,
		"posting_rule_key":     "treasury_transfer",
		"journal_source_kind":  "system",
		"journal_lines": []map[string]any{
			{"account_code": textValue(toAccount.Values["gl_account_code"]), "description": "Treasury Transfer In", "debit": amount, "credit": 0.0},
			{"account_code": textValue(fromAccount.Values["gl_account_code"]), "description": "Treasury Transfer Out", "debit": 0.0, "credit": amount},
		},
		"total_amount": amount,
	}
	posting, err := s.documents.Create("ledger_posting", organizationID, locationID, actorID, payload)
	if err != nil {
		return nil, err
	}
	if err := s.finalizeSystemPosting(posting, actorID, "posted"); err != nil {
		return nil, err
	}
	updatedTransfer, err := s.models.Update("treasury_transfer", transfer.ID, actorID, mergeModelValues(transfer.Values, map[string]any{"status": "posted", "posting_id": posting.Header.ID}), transfer.Version)
	if err != nil {
		return nil, err
	}
	return map[string]any{"transfer": updatedTransfer, "posting": posting}, nil
}

func (s *TreasuryCoreService) finalizeSystemPosting(record document.Record, actorID, status string) error {
	record.Header.Status = status
	record.Header.TotalAmount = shared.Money{
		Currency:    firstNonEmptyString(textValue(record.Body.Payload["currency_code"]), record.Header.TotalAmount.Currency, "IDR"),
		AmountMinor: moneyMinor(numberValue(record.Body.Payload["total_amount"])),
	}
	record.Header.Version++
	record.Header.ETag = fmt.Sprintf("%s:%d", record.Header.ID, record.Header.Version)
	record.Header.UpdatedBy = actorID
	record.Header.UpdatedAt = time.Now().UTC()
	return s.documents.Save(record)
}

func (s *TreasuryCoreService) BankReconciliation(organizationID, locationID, statementID string) TreasuryBankReconciliationReport {
	report := TreasuryBankReconciliationReport{OrganizationID: organizationID, LocationID: locationID, Lines: []TreasuryBankReconciliationLine{}, Exceptions: []model.Record{}}
	statement, err := s.models.Get("bank_statement", strings.TrimSpace(statementID))
	if err != nil {
		return report
	}
	report.StatementID = statement.ID
	report.TreasuryAccountID = textValue(statement.Values["treasury_account_id"])
	report.AsOfDate = firstNonEmptyString(textValue(statement.Values["to_date"]), textValue(statement.Values["statement_date"]))
	report.StatementBalance = roundMoney(numberValue(statement.Values["closing_balance"]))
	reconciliation, ok := s.findTreasuryModelByFields("bank_reconciliation", map[string]string{"bank_statement_id": statement.ID})
	if ok {
		report.ReconciliationID = reconciliation.ID
	}
	var account model.Record
	glAccount := ""
	account, err = s.models.Get("treasury_account", report.TreasuryAccountID)
	if err == nil {
		glAccount = textValue(account.Values["gl_account_code"])
		ledger := s.finance.JournalLedger(organizationID, locationID, "", report.AsOfDate)
		for _, row := range ledger.Rows {
			if row.AccountCode != glAccount {
				continue
			}
			report.BookBalance = roundMoney(report.BookBalance + row.Debit - row.Credit)
		}
	}
	lines, _, err := s.models.List("bank_statement_line", model.Query{Page: 1, PageSize: 500, Filters: map[string]string{"bank_statement_id": statement.ID}})
	if err == nil {
		for _, line := range lines {
			row := TreasuryBankReconciliationLine{
				StatementLineID:   line.ID,
				StatementDate:     textValue(line.Values["statement_date"]),
				Description:       textValue(line.Values["description"]),
				Reference:         textValue(line.Values["reference"]),
				Amount:            roundMoney(numberValue(line.Values["signed_amount"])),
				MatchedAmount:     roundMoney(numberValue(line.Values["matched_amount"])),
				RemainingAmount:   roundMoney(numberValue(line.Values["remaining_amount"])),
				MatchStatus:       textValue(line.Values["match_status"]),
				MatchedSourceType: textValue(line.Values["matched_source_type"]),
				MatchedSourceID:   textValue(line.Values["matched_source_id"]),
			}
			if row.RemainingAmount > 0 {
				row.Candidates = s.matchCandidatesForLine(organizationID, locationID, account, statement, line, report.AsOfDate, glAccount)
			}
			report.MatchedAmount = roundMoney(report.MatchedAmount + row.MatchedAmount)
			report.Lines = append(report.Lines, row)
		}
	}
	openLines := 0.0
	for _, line := range report.Lines {
		openLines = roundMoney(openLines + line.RemainingAmount)
	}
	report.OutstandingBookAmount = roundMoney(report.BookBalance - report.MatchedAmount)
	report.Difference = roundMoney(report.StatementBalance - report.BookBalance)
	exceptions, _, err := s.models.List("treasury_exception", model.Query{Page: 1, PageSize: 500, Filters: map[string]string{"bank_statement_id": statement.ID, "status": "pending"}})
	if err == nil {
		report.Exceptions = exceptions
	}
	_ = openLines
	sort.Slice(report.Lines, func(i, j int) bool {
		if report.Lines[i].StatementDate != report.Lines[j].StatementDate {
			return report.Lines[i].StatementDate < report.Lines[j].StatementDate
		}
		return report.Lines[i].StatementLineID < report.Lines[j].StatementLineID
	})
	return report
}

func (s *TreasuryCoreService) CashPositionReport(organizationID, locationID, asOfDate string) TreasuryCashPositionReport {
	asOfDate = normalizeAsOfDate(asOfDate)
	if asOfDate == "" {
		asOfDate = time.Now().UTC().Format("2006-01-02")
	}
	report := TreasuryCashPositionReport{OrganizationID: organizationID, LocationID: locationID, AsOfDate: asOfDate, Rows: []TreasuryCashPositionRow{}, Totals: map[string]float64{}}
	accounts, _, err := s.models.List("treasury_account", model.Query{Page: 1, PageSize: 500, Filters: map[string]string{"organization_id": organizationID, "location_id": locationID}})
	if err != nil && !isMissingModelDefinitionError(err) {
		return report
	}
	ledger := s.finance.JournalLedger(organizationID, locationID, "", asOfDate)
	for _, account := range accounts {
		row := TreasuryCashPositionRow{
			TreasuryAccountID: account.ID,
			AccountCode:       textValue(account.Values["account_code"]),
			AccountName:       textValue(account.Values["name"]),
			TreasuryType:      textValue(account.Values["treasury_type"]),
			CurrencyCode:      firstNonEmptyString(textValue(account.Values["currency_code"]), "IDR"),
			GLAccountCode:     textValue(account.Values["gl_account_code"]),
		}
		for _, line := range ledger.Rows {
			if line.AccountCode == row.GLAccountCode {
				row.BookBalance = roundMoney(row.BookBalance + line.Debit - line.Credit)
			}
		}
		statement := s.latestStatementForAccount(account.ID, asOfDate)
		if statement.ID != "" {
			row.StatementBalance = roundMoney(numberValue(statement.Values["closing_balance"]))
			row.LastStatementDate = firstNonEmptyString(textValue(statement.Values["to_date"]), textValue(statement.Values["statement_date"]))
		}
		if reconciliation, ok := s.findLatestReconciliationForAccount(account.ID, asOfDate); ok {
			row.LastReconciliationID = reconciliation.ID
		}
		row.UnreconciledAmount = roundMoney(row.StatementBalance - row.BookBalance)
		report.Rows = append(report.Rows, row)
		report.Totals["book_balance"] = roundMoney(report.Totals["book_balance"] + row.BookBalance)
		report.Totals["statement_balance"] = roundMoney(report.Totals["statement_balance"] + row.StatementBalance)
		report.Totals["unreconciled_amount"] = roundMoney(report.Totals["unreconciled_amount"] + row.UnreconciledAmount)
	}
	sort.Slice(report.Rows, func(i, j int) bool { return report.Rows[i].AccountCode < report.Rows[j].AccountCode })
	return report
}

func (s *TreasuryCoreService) ClearingBalanceReport(organizationID, locationID, asOfDate string) TreasuryClearingBalanceReport {
	asOfDate = normalizeAsOfDate(asOfDate)
	if asOfDate == "" {
		asOfDate = time.Now().UTC().Format("2006-01-02")
	}
	report := TreasuryClearingBalanceReport{OrganizationID: organizationID, LocationID: locationID, AsOfDate: asOfDate, Rows: []TreasuryClearingBalanceRow{}, Totals: map[string]float64{}}
	if s.documents != nil {
		for _, record := range s.documents.List() {
			if record.Header.OrganizationID != organizationID {
				continue
			}
			if locationID != "" && record.Header.LocationID != "" && record.Header.LocationID != locationID {
				continue
			}
			switch record.Header.Type {
			case "payment_receipt":
				if record.Header.Status != "received" {
					continue
				}
				date := firstNonEmptyString(textValue(record.Body.Payload["payment_date"]), textValue(record.Body.Payload["receipt_date"]))
				if asOfDate != "" && date != "" && date > asOfDate {
					continue
				}
				open := roundMoney(numberValue(record.Body.Payload["unapplied_amount"]))
				if open == 0 {
					continue
				}
				row := TreasuryClearingBalanceRow{SourceKind: "payment_receipt", AccountCode: textValue(record.Body.Payload["clearing_account_code"]), ReferenceID: record.Header.ID, ReferenceNo: firstNonEmptyString(record.Header.Number, record.Header.ID), OpenAmount: open, Status: record.Header.Status, EffectiveDate: date}
				report.Rows = append(report.Rows, row)
				report.Totals[row.AccountCode] = roundMoney(report.Totals[row.AccountCode] + row.OpenAmount)
			case "payment_out":
				if record.Header.Status != "paid" {
					continue
				}
				date := firstNonEmptyString(textValue(record.Body.Payload["payment_date"]), textValue(record.Body.Payload["bill_date"]))
				if asOfDate != "" && date != "" && date > asOfDate {
					continue
				}
				open := roundMoney(numberValue(record.Body.Payload["unapplied_amount"]))
				if open == 0 {
					continue
				}
				row := TreasuryClearingBalanceRow{SourceKind: "payment_out", AccountCode: textValue(record.Body.Payload["clearing_account_code"]), ReferenceID: record.Header.ID, ReferenceNo: firstNonEmptyString(record.Header.Number, record.Header.ID), OpenAmount: open, Status: record.Header.Status, EffectiveDate: date}
				report.Rows = append(report.Rows, row)
				report.Totals[row.AccountCode] = roundMoney(report.Totals[row.AccountCode] + row.OpenAmount)
			}
		}
	}
	if s.retail != nil {
		settlements := s.retail.TenderSettlementReport(organizationID, locationID, asOfDate, "", "", "")
		for _, row := range settlements.Rows {
			if row.Status == "settled" || roundMoney(row.DifferenceAmount) == 0 {
				continue
			}
			item := TreasuryClearingBalanceRow{SourceKind: "pos_tender_settlement", AccountCode: row.ClearingAccountCode, ReferenceID: row.SettlementID, ReferenceNo: row.ShiftNumber, OpenAmount: roundMoney(row.ExpectedAmount - row.SettledAmount), Status: row.Status, EffectiveDate: row.SettlementDate}
			report.Rows = append(report.Rows, item)
			report.Totals[item.AccountCode] = roundMoney(report.Totals[item.AccountCode] + item.OpenAmount)
		}
	}
	sort.Slice(report.Rows, func(i, j int) bool {
		if report.Rows[i].AccountCode != report.Rows[j].AccountCode {
			return report.Rows[i].AccountCode < report.Rows[j].AccountCode
		}
		return report.Rows[i].ReferenceNo < report.Rows[j].ReferenceNo
	})
	return report
}

func (s *TreasuryCoreService) TransferRegister(organizationID, locationID, asOfDate, status string) TreasuryTransferRegisterReport {
	asOfDate = normalizeAsOfDate(asOfDate)
	report := TreasuryTransferRegisterReport{OrganizationID: organizationID, LocationID: locationID, AsOfDate: asOfDate, Rows: []TreasuryTransferRegisterRow{}, Totals: map[string]float64{}}
	items, _, err := s.models.List("treasury_transfer", model.Query{Page: 1, PageSize: 500, Filters: map[string]string{"organization_id": organizationID, "location_id": locationID}})
	if err != nil && !isMissingModelDefinitionError(err) {
		return report
	}
	for _, item := range items {
		if status != "" && !strings.EqualFold(textValue(item.Values["status"]), status) {
			continue
		}
		if asOfDate != "" && textValue(item.Values["transfer_date"]) > asOfDate {
			continue
		}
		row := TreasuryTransferRegisterRow{TransferID: item.ID, TransferDate: textValue(item.Values["transfer_date"]), FromAccountID: textValue(item.Values["from_treasury_account_id"]), FromAccountCode: textValue(item.Values["from_account_code"]), ToAccountID: textValue(item.Values["to_treasury_account_id"]), ToAccountCode: textValue(item.Values["to_account_code"]), Amount: roundMoney(numberValue(item.Values["amount"])), Reference: textValue(item.Values["reference"]), Status: textValue(item.Values["status"]), PostingID: textValue(item.Values["posting_id"])}
		report.Rows = append(report.Rows, row)
		report.Totals["amount"] = roundMoney(report.Totals["amount"] + row.Amount)
	}
	sort.Slice(report.Rows, func(i, j int) bool { return report.Rows[i].TransferDate < report.Rows[j].TransferDate })
	return report
}

func (s *TreasuryCoreService) ExceptionReport(organizationID, locationID, asOfDate, status string) TreasuryExceptionReport {
	asOfDate = normalizeAsOfDate(asOfDate)
	report := TreasuryExceptionReport{OrganizationID: organizationID, LocationID: locationID, AsOfDate: asOfDate, Items: []model.Record{}, Totals: map[string]float64{}}
	filters := map[string]string{"organization_id": organizationID, "location_id": locationID}
	if status != "" {
		if status == "open" {
			status = "pending"
		}
		filters["status"] = status
	}
	items, _, err := s.models.List("treasury_exception", model.Query{Page: 1, PageSize: 500, Filters: filters})
	if err != nil && !isMissingModelDefinitionError(err) {
		return report
	}
	for _, item := range items {
		if asOfDate != "" {
			effectiveDate := firstNonEmptyString(textValue(item.Values["exception_date"]), textValue(item.Values["statement_date"]))
			if effectiveDate != "" && effectiveDate > asOfDate {
				continue
			}
		}
		report.Items = append(report.Items, item)
		report.Totals[textValue(item.Values["exception_kind"])] = roundMoney(report.Totals[textValue(item.Values["exception_kind"])] + numberValue(item.Values["amount"]))
	}
	return report
}

func (s *TreasuryCoreService) ResolveException(exceptionID, actorID, status, note string) (model.Record, error) {
	item, err := s.models.Get("treasury_exception", strings.TrimSpace(exceptionID))
	if err != nil {
		return model.Record{}, err
	}
	values := cloneMap(item.Values)
	values["status"] = firstNonEmptyString(strings.TrimSpace(status), "completed")
	values["note"] = strings.TrimSpace(note)
	values["resolved_at"] = time.Now().UTC().Format(time.RFC3339)
	values["resolved_by"] = actorID
	return s.models.Update("treasury_exception", item.ID, actorID, values, item.Version)
}

func (s *TreasuryCoreService) CreateExceptionJournal(exceptionID, actorID string, payload map[string]any) (map[string]any, error) {
	item, err := s.models.Get("treasury_exception", strings.TrimSpace(exceptionID))
	if err != nil {
		return nil, err
	}
	kind := strings.TrimSpace(textValue(item.Values["exception_kind"]))
	journalKind := s.exceptionJournalKind(kind, item)
	if journalKind == "" {
		return nil, shared.Validation("only bank fee or interest candidates can create draft journals")
	}
	statementLine, err := s.models.Get("bank_statement_line", strings.TrimSpace(textValue(item.Values["bank_statement_line_id"])))
	if err != nil {
		return nil, err
	}
	account, err := s.models.Get("treasury_account", strings.TrimSpace(textValue(item.Values["treasury_account_id"])))
	if err != nil {
		return nil, err
	}
	organizationID := textValue(item.Values["organization_id"])
	locationID := textValue(item.Values["location_id"])
	postingConfig := s.treasuryPostingConfig(organizationID, locationID)
	statementAmount := roundMoney(maxFloat(numberValue(statementLine.Values["signed_amount"]), -numberValue(statementLine.Values["signed_amount"])))
	if statementAmount <= 0 {
		statementAmount = roundMoney(numberValue(item.Values["amount"]))
	}
	if statementAmount <= 0 {
		return nil, shared.Validation("exception amount must be greater than zero")
	}
	postingDate := firstNonEmptyString(textValue(payload["posting_date"]), textValue(item.Values["exception_date"]), textValue(item.Values["statement_date"]), time.Now().UTC().Format("2006-01-02"))
	if s.finance != nil {
		if err := s.finance.ValidatePostingDateOpen(organizationID, locationID, postingDate); err != nil {
			return nil, err
		}
	}
	currencyCode := firstNonEmptyString(textValue(payload["currency_code"]), "IDR")
	var journalLines []map[string]any
	if journalKind == "bank_fee" {
		journalLines = []map[string]any{
			{"account_code": firstNonEmptyString(textValue(payload["expense_account_code"]), postingConfig["bank_fee_expense_account_code"]), "debit": statementAmount, "credit": 0.0, "description": "Bank fee"},
			{"account_code": firstNonEmptyString(textValue(payload["offset_account_code"]), textValue(account.Values["gl_account_code"]), postingConfig["treasury_suspense_account_code"]), "debit": 0.0, "credit": statementAmount, "description": "Bank fee offset"},
		}
	} else {
		journalLines = []map[string]any{
			{"account_code": firstNonEmptyString(textValue(payload["offset_account_code"]), textValue(account.Values["gl_account_code"]), postingConfig["treasury_suspense_account_code"]), "debit": statementAmount, "credit": 0.0, "description": "Bank interest receipt"},
			{"account_code": firstNonEmptyString(textValue(payload["income_account_code"]), postingConfig["bank_interest_income_account_code"]), "debit": 0.0, "credit": statementAmount, "description": "Bank interest income"},
		}
	}
	journalPayload := normalizeManualJournalLines(map[string]any{
		"posting_date":           postingDate,
		"currency_code":          currencyCode,
		"journal_source_kind":    "manual",
		"manual_journal_type":    "other",
		"posting_rule_key":       "treasury_exception_adjustment",
		"supporting_reference":   firstNonEmptyString(textValue(statementLine.Values["reference"]), item.ID),
		"notes":                  firstNonEmptyString(textValue(payload["notes"]), fmt.Sprintf("Treasury exception journal for %s", item.ID)),
		"journal_lines":          journalLines,
		"source_document_type":   "treasury_exception",
		"source_document_id":     item.ID,
		"treasury_exception_id":  item.ID,
		"bank_statement_line_id": statementLine.ID,
	})
	record, err := s.documents.Create("ledger_posting", organizationID, locationID, actorID, journalPayload)
	if err != nil {
		return nil, err
	}
	record.Header.TotalAmount = shared.Money{
		Currency:    currencyCode,
		AmountMinor: moneyMinor(numberValue(journalPayload["total_amount"])),
	}
	if err := s.documents.Save(record); err != nil {
		return nil, err
	}
	updatedException, err := s.models.Update("treasury_exception", item.ID, actorID, mergeModelValues(item.Values, map[string]any{
		"suggested_journal_id": record.Header.ID,
		"note":                 firstNonEmptyString(textValue(payload["notes"]), textValue(item.Values["note"])),
	}), item.Version)
	if err != nil {
		return nil, err
	}
	return map[string]any{"record": record, "exception": updatedException}, nil
}

func (s *TreasuryCoreService) syncStatementExceptions(reconciliation, statement model.Record, actorID string) error {
	lines, _, err := s.models.List("bank_statement_line", model.Query{Page: 1, PageSize: 500, Filters: map[string]string{"bank_statement_id": statement.ID}})
	if err != nil && !isMissingModelDefinitionError(err) {
		return err
	}
	for _, line := range lines {
		remaining := roundMoney(numberValue(line.Values["remaining_amount"]))
		exceptionKind := s.classifyStatementException(line)
		if remaining > 0 {
			_, err := s.upsertTreasuryModel("treasury_exception", map[string]string{"bank_statement_line_id": line.ID}, actorID, map[string]any{
				"organization_id":        textValue(reconciliation.Values["organization_id"]),
				"location_id":            textValue(reconciliation.Values["location_id"]),
				"treasury_account_id":    textValue(reconciliation.Values["treasury_account_id"]),
				"bank_statement_id":      statement.ID,
				"bank_statement_line_id": line.ID,
				"exception_kind":         exceptionKind,
				"exception_date":         firstNonEmptyString(textValue(line.Values["value_date"]), textValue(line.Values["statement_date"])),
				"statement_date":         textValue(line.Values["statement_date"]),
				"description":            textValue(line.Values["description"]),
				"reference":              textValue(line.Values["reference"]),
				"amount":                 remaining,
				"status":                 "pending",
			})
			if err != nil {
				return err
			}
			continue
		}
		if current, ok := s.findTreasuryModelByFields("treasury_exception", map[string]string{"bank_statement_line_id": line.ID}); ok && textValue(current.Values["status"]) == "pending" {
			if _, err := s.models.Update("treasury_exception", current.ID, actorID, mergeModelValues(current.Values, map[string]any{"status": "completed", "resolved_at": time.Now().UTC().Format(time.RFC3339), "resolved_by": actorID}), current.Version); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *TreasuryCoreService) refreshStatementBalance(statementID, actorID string) (model.Record, error) {
	statement, err := s.models.Get("bank_statement", statementID)
	if err != nil {
		return model.Record{}, err
	}
	lines, _, err := s.models.List("bank_statement_line", model.Query{Page: 1, PageSize: 500, Filters: map[string]string{"bank_statement_id": statement.ID}})
	if err != nil {
		return model.Record{}, err
	}
	opening := roundMoney(numberValue(statement.Values["opening_balance"]))
	closing := opening
	for _, line := range lines {
		closing = roundMoney(closing + numberValue(line.Values["signed_amount"]))
	}
	values := cloneMap(statement.Values)
	values["closing_balance"] = closing
	return s.models.Update("bank_statement", statement.ID, actorID, values, statement.Version)
}

func (s *TreasuryCoreService) latestStatementForAccount(treasuryAccountID, asOfDate string) model.Record {
	items, _, err := s.models.List("bank_statement", model.Query{Page: 1, PageSize: 500, Filters: map[string]string{"treasury_account_id": treasuryAccountID}})
	if err != nil {
		return model.Record{}
	}
	var best model.Record
	bestDate := ""
	for _, item := range items {
		current := firstNonEmptyString(textValue(item.Values["to_date"]), textValue(item.Values["statement_date"]))
		if current == "" || (asOfDate != "" && current > asOfDate) {
			continue
		}
		if best.ID == "" || current > bestDate {
			best = item
			bestDate = current
		}
	}
	return best
}

func (s *TreasuryCoreService) findLatestReconciliationForAccount(treasuryAccountID, asOfDate string) (model.Record, bool) {
	items, _, err := s.models.List("bank_reconciliation", model.Query{Page: 1, PageSize: 500, Filters: map[string]string{"treasury_account_id": treasuryAccountID}})
	if err != nil {
		return model.Record{}, false
	}
	var best model.Record
	bestDate := ""
	for _, item := range items {
		current := textValue(item.Values["reconciliation_date"])
		if current == "" || (asOfDate != "" && current > asOfDate) {
			continue
		}
		if best.ID == "" || current > bestDate {
			best = item
			bestDate = current
		}
	}
	return best, best.ID != ""
}

func (s *TreasuryCoreService) validateTreasuryScope(record model.Record, organizationID, locationID string) error {
	if organizationID != "" && textValue(record.Values["organization_id"]) != organizationID {
		return shared.Forbidden("record is outside the current organization scope")
	}
	if locationID != "" && textValue(record.Values["location_id"]) != "" && textValue(record.Values["location_id"]) != locationID {
		return shared.Forbidden("record is outside the current location scope")
	}
	return nil
}

func (s *TreasuryCoreService) upsertTreasuryModel(modelKey string, filters map[string]string, actorID string, values map[string]any) (model.Record, error) {
	if current, ok := s.findTreasuryModelByFields(modelKey, filters); ok {
		return s.models.Update(modelKey, current.ID, actorID, mergeModelValues(current.Values, values), current.Version)
	}
	return s.models.Create(modelKey, actorID, values)
}

func (s *TreasuryCoreService) findTreasuryModelByFields(modelKey string, filters map[string]string) (model.Record, bool) {
	items, _, err := s.models.List(modelKey, model.Query{Page: 1, PageSize: 100, Filters: filters})
	if err != nil || len(items) == 0 {
		return model.Record{}, false
	}
	return items[0], true
}

func (s *TreasuryCoreService) previewStatementImport(account model.Record, organizationID, locationID string, payload map[string]any, rawCSV string) (map[string]any, []map[string]any, []string, int, error) {
	templateValues, err := s.resolveImportTemplate(organizationID, locationID, payload)
	if err != nil {
		return nil, nil, nil, 0, err
	}
	reader := csv.NewReader(strings.NewReader(strings.TrimSpace(rawCSV)))
	records, err := readCSVAll(reader)
	if err != nil {
		return nil, nil, nil, 0, shared.Validation("invalid treasury statement csv")
	}
	headerRow := intAnyValue(templateValues["header_row_index"], 0)
	if headerRow < 0 || headerRow >= len(records) {
		headerRow = 0
	}
	if len(records) <= headerRow+1 {
		return nil, nil, nil, 0, shared.Validation("statement csv must include a header and at least one row")
	}
	header := csvHeaderIndexWithAliases(records[headerRow], stringMap(templateValues["header_aliases_json"]))
	statementDate := firstNonEmptyString(normalizeTemplateDate(textValue(payload["statement_date"]), templateValues), time.Now().UTC().Format("2006-01-02"))
	lines := make([]map[string]any, 0, len(records)-headerRow-1)
	warnings := []string{}
	duplicateCount := 0
	for idx, row := range records[headerRow+1:] {
		if len(row) == 0 || isBlankCSVRow(row) {
			continue
		}
		lineValues, skipReason := s.parseStatementCSVRowNormalized(organizationID, locationID, account, row, header, templateValues)
		if skipReason != "" {
			warnings = append(warnings, fmt.Sprintf("row %d skipped: %s", headerRow+idx+2, skipReason))
			continue
		}
		if s.statementLineDuplicateExists(account.ID, lineValues) {
			duplicateCount++
			warnings = append(warnings, fmt.Sprintf("row %d skipped: duplicate statement line", headerRow+idx+2))
			continue
		}
		lines = append(lines, lineValues)
	}
	fromDate := firstNonEmptyString(normalizeTemplateDate(textValue(payload["from_date"]), templateValues), firstStatementLineDate(lines), statementDate)
	toDate := firstNonEmptyString(normalizeTemplateDate(textValue(payload["to_date"]), templateValues), lastStatementLineDate(lines), statementDate)
	statementValues := map[string]any{
		"organization_id":         organizationID,
		"location_id":             locationID,
		"treasury_account_id":     account.ID,
		"statement_number":        firstNonEmptyString(textValue(payload["statement_number"]), posNumber("STMT")),
		"statement_date":          statementDate,
		"from_date":               fromDate,
		"to_date":                 toDate,
		"opening_balance":         roundMoney(numberValue(payload["opening_balance"])),
		"closing_balance":         roundMoney(numberValue(payload["closing_balance"])),
		"import_method":           "csv",
		"source_file_name":        textValue(payload["source_file_name"]),
		"status":                  "imported",
		"bank_import_template_id": strings.TrimSpace(textValue(payload["bank_import_template_id"])),
		"preset_key":              strings.TrimSpace(textValue(payload["preset_key"])),
	}
	return statementValues, lines, warnings, duplicateCount, nil
}

func (s *TreasuryCoreService) resolveImportTemplate(organizationID, locationID string, payload map[string]any) (map[string]any, error) {
	templateID := strings.TrimSpace(textValue(payload["bank_import_template_id"]))
	if templateID != "" {
		record, err := s.models.Get("bank_import_template", templateID)
		if err != nil {
			return nil, err
		}
		if err := s.validateTreasuryScope(record, organizationID, locationID); err != nil {
			return nil, err
		}
		return cloneMap(record.Values), nil
	}
	presetKey := strings.TrimSpace(textValue(payload["preset_key"]))
	if presetKey != "" {
		if preset, ok := s.findTreasuryModelByFields("bank_import_preset", map[string]string{"preset_key": presetKey}); ok {
			return cloneMap(preset.Values), nil
		}
		return builtInBankImportPreset(presetKey), nil
	}
	return builtInBankImportPreset("generic_debit_credit"), nil
}

func builtInBankImportPreset(presetKey string) map[string]any {
	switch strings.TrimSpace(presetKey) {
	case "generic_signed_amount":
		return map[string]any{
			"preset_key":         "generic_signed_amount",
			"header_row_index":   0,
			"date_column":        "date",
			"value_date_column":  "value_date",
			"reference_column":   "reference",
			"description_column": "description",
			"amount_column":      "amount",
			"balance_column":     "balance",
			"sign_convention":    "signed",
			"date_format":        "2006-01-02",
		}
	default:
		return map[string]any{
			"preset_key":                "generic_debit_credit",
			"header_row_index":          0,
			"date_column":               "date",
			"value_date_column":         "value_date",
			"reference_column":          "reference",
			"external_reference_column": "external_reference",
			"description_column":        "description",
			"debit_column":              "debit",
			"credit_column":             "credit",
			"balance_column":            "balance",
			"sign_convention":           "credit_minus_debit",
			"date_format":               "2006-01-02",
		}
	}
}

func (s *TreasuryCoreService) parseStatementCSVRowNormalized(organizationID, locationID string, account model.Record, row []string, header map[string]int, template map[string]any) (map[string]any, string) {
	dateColumn := firstNonEmptyString(textValue(template["date_column"]), "date")
	statementDate := normalizeTemplateDate(csvString(row, header, dateColumn), template)
	if statementDate == "" {
		return nil, "statement date is required"
	}
	valueDate := normalizeTemplateDate(csvString(row, header, firstNonEmptyString(textValue(template["value_date_column"]), "value_date")), template)
	reference := firstNonEmptyString(csvString(row, header, firstNonEmptyString(textValue(template["reference_column"]), "reference")), csvString(row, header, firstNonEmptyString(textValue(template["external_reference_column"]), "external_reference")))
	description := csvString(row, header, firstNonEmptyString(textValue(template["description_column"]), "description"))
	debit := normalizeTemplateNumber(csvString(row, header, firstNonEmptyString(textValue(template["debit_column"]), "debit")), template)
	credit := normalizeTemplateNumber(csvString(row, header, firstNonEmptyString(textValue(template["credit_column"]), "credit")), template)
	amount := normalizeTemplateNumber(csvString(row, header, firstNonEmptyString(textValue(template["amount_column"]), "amount")), template)
	signedAmount := roundMoney(resolveTemplateSignedAmount(amount, debit, credit, textValue(template["sign_convention"])))
	if signedAmount == 0 {
		return nil, "amount is required"
	}
	return map[string]any{
		"organization_id":     organizationID,
		"location_id":         locationID,
		"treasury_account_id": account.ID,
		"statement_date":      statementDate,
		"value_date":          valueDate,
		"reference":           reference,
		"description":         description,
		"debit_amount":        roundMoney(debit),
		"credit_amount":       roundMoney(credit),
		"signed_amount":       signedAmount,
		"running_balance":     roundMoney(normalizeTemplateNumber(csvString(row, header, firstNonEmptyString(textValue(template["balance_column"]), "balance")), template)),
		"matched_amount":      0.0,
		"remaining_amount":    roundMoney(maxFloat(signedAmount, -signedAmount)),
		"match_status":        "unmatched",
	}, ""
}

func (s *TreasuryCoreService) statementLineDuplicateExists(treasuryAccountID string, values map[string]any) bool {
	items, _, err := s.models.List("bank_statement_line", model.Query{
		Page:     1,
		PageSize: 25,
		Filters: map[string]string{
			"treasury_account_id": treasuryAccountID,
			"statement_date":      textValue(values["statement_date"]),
			"reference":           textValue(values["reference"]),
		},
	})
	if err != nil {
		return false
	}
	for _, item := range items {
		if roundMoney(numberValue(item.Values["signed_amount"])) != roundMoney(numberValue(values["signed_amount"])) {
			continue
		}
		if normalizeMatchText(textValue(item.Values["description"])) != normalizeMatchText(textValue(values["description"])) {
			continue
		}
		return true
	}
	return false
}

func normalizeMatchedSourceType(sourceType string) string {
	switch strings.TrimSpace(sourceType) {
	case "payment_receipt", "receipt":
		return "receipt"
	case "payment_refund", "payment_out", "payment":
		return "payment"
	case "treasury_transfer", "transfer":
		return "transfer"
	case "ledger_posting", "journal":
		return "journal"
	case "pos_tender_settlement", "settlement":
		return "settlement"
	default:
		return "other"
	}
}

func (s *TreasuryCoreService) classifyStatementException(line model.Record) string {
	if s.exceptionJournalKind("", line) != "" {
		return "other"
	}
	return "other"
}

func (s *TreasuryCoreService) exceptionJournalKind(kind string, line model.Record) string {
	switch strings.TrimSpace(kind) {
	case "bank_fee_candidate":
		return "bank_fee"
	case "interest_candidate":
		return "interest"
	}
	description := normalizeMatchText(textValue(line.Values["description"]) + " " + textValue(line.Values["reference"]))
	switch {
	case strings.Contains(description, "fee") || strings.Contains(description, "charge") || strings.Contains(description, "admin"):
		return "bank_fee"
	case strings.Contains(description, "interest") || strings.Contains(description, "bunga"):
		return "interest"
	default:
		return ""
	}
}

func (s *TreasuryCoreService) matchCandidatesForLine(organizationID, locationID string, account, statement, line model.Record, asOfDate, glAccount string) []TreasuryBankMatchCandidate {
	candidates := []TreasuryBankMatchCandidate{}
	for _, candidate := range s.documentMatchCandidates(organizationID, locationID, line, glAccount, asOfDate) {
		candidates = append(candidates, candidate)
	}
	for _, candidate := range s.modelMatchCandidates(organizationID, locationID, line, account.ID, glAccount, asOfDate) {
		candidates = append(candidates, candidate)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].SourceID < candidates[j].SourceID
		}
		return candidates[i].Score > candidates[j].Score
	})
	if len(candidates) > 5 {
		candidates = candidates[:5]
	}
	return candidates
}

func (s *TreasuryCoreService) documentMatchCandidates(organizationID, locationID string, line model.Record, glAccount, asOfDate string) []TreasuryBankMatchCandidate {
	candidates := []TreasuryBankMatchCandidate{}
	targetAmount := roundMoney(maxFloat(numberValue(line.Values["signed_amount"]), -numberValue(line.Values["signed_amount"])))
	targetDate := firstNonEmptyString(textValue(line.Values["value_date"]), textValue(line.Values["statement_date"]))
	targetRef := textValue(line.Values["reference"])
	targetDesc := textValue(line.Values["description"])
	for _, item := range s.documents.List() {
		if item.Header.OrganizationID != organizationID {
			continue
		}
		if locationID != "" && item.Header.LocationID != "" && item.Header.LocationID != locationID {
			continue
		}
		if item.Header.Type == "ledger_posting" && item.Header.Status != "posted" {
			continue
		}
		amount, accountCode, effectiveDate, reference, description, ok := treasuryDocumentCandidate(item, glAccount)
		if !ok || amount <= 0 {
			continue
		}
		if asOfDate != "" && effectiveDate != "" && effectiveDate > asOfDate {
			continue
		}
		score, reasons := scoreTreasuryMatch(targetAmount, amount, targetDate, effectiveDate, targetRef, reference, targetDesc, description)
		if score <= 0 {
			continue
		}
		candidates = append(candidates, TreasuryBankMatchCandidate{
			SourceType:    item.Header.Type,
			SourceID:      item.Header.ID,
			Reference:     reference,
			Description:   description,
			AccountCode:   accountCode,
			EffectiveDate: effectiveDate,
			Amount:        amount,
			Score:         score,
			Reasons:       reasons,
		})
	}
	return candidates
}

func (s *TreasuryCoreService) modelMatchCandidates(organizationID, locationID string, line model.Record, treasuryAccountID, glAccount, asOfDate string) []TreasuryBankMatchCandidate {
	candidates := []TreasuryBankMatchCandidate{}
	targetAmount := roundMoney(maxFloat(numberValue(line.Values["signed_amount"]), -numberValue(line.Values["signed_amount"])))
	targetDate := firstNonEmptyString(textValue(line.Values["value_date"]), textValue(line.Values["statement_date"]))
	targetRef := textValue(line.Values["reference"])
	targetDesc := textValue(line.Values["description"])
	if s.retail != nil {
		settlements := s.retail.TenderSettlementReport(organizationID, locationID, asOfDate, "", "", "")
		for _, item := range settlements.Rows {
			if item.ClearingAccountCode != glAccount || roundMoney(item.ExpectedAmount-item.SettledAmount) <= 0 {
				continue
			}
			score, reasons := scoreTreasuryMatch(targetAmount, roundMoney(item.ExpectedAmount-item.SettledAmount), targetDate, item.SettlementDate, targetRef, item.SettlementReference, targetDesc, item.ShiftNumber)
			if score <= 0 {
				continue
			}
			candidates = append(candidates, TreasuryBankMatchCandidate{
				SourceType:    "pos_tender_settlement",
				SourceID:      item.SettlementID,
				Reference:     item.SettlementReference,
				Description:   item.ShiftNumber,
				AccountCode:   item.ClearingAccountCode,
				EffectiveDate: item.SettlementDate,
				Amount:        roundMoney(item.ExpectedAmount - item.SettledAmount),
				Score:         score,
				Reasons:       reasons,
			})
		}
	}
	transfers := s.TransferRegister(organizationID, locationID, asOfDate, "posted")
	for _, item := range transfers.Rows {
		if treasuryAccountID != "" && item.FromAccountID != treasuryAccountID && item.ToAccountID != treasuryAccountID {
			continue
		}
		score, reasons := scoreTreasuryMatch(targetAmount, item.Amount, targetDate, item.TransferDate, targetRef, item.Reference, targetDesc, item.Reference)
		if score <= 0 {
			continue
		}
		candidates = append(candidates, TreasuryBankMatchCandidate{
			SourceType:    "treasury_transfer",
			SourceID:      item.TransferID,
			Reference:     item.Reference,
			Description:   item.Reference,
			AccountCode:   "",
			EffectiveDate: item.TransferDate,
			Amount:        item.Amount,
			Score:         score,
			Reasons:       reasons,
		})
	}
	return candidates
}

func (s *TreasuryCoreService) parseStatementCSVRow(organizationID, locationID string, statement model.Record, row []string, header map[string]int) map[string]any {
	signedAmount := roundMoney(resolveSignedStatementAmount(map[string]any{
		"debit_amount":  csvNumber(row, header, "debit"),
		"credit_amount": csvNumber(row, header, "credit"),
		"amount":        csvNumber(row, header, "amount"),
	}))
	return map[string]any{
		"organization_id":     organizationID,
		"location_id":         locationID,
		"bank_statement_id":   statement.ID,
		"treasury_account_id": textValue(statement.Values["treasury_account_id"]),
		"statement_date":      csvString(row, header, "date"),
		"value_date":          csvString(row, header, "value_date"),
		"reference":           firstNonEmptyString(csvString(row, header, "reference"), csvString(row, header, "external_reference")),
		"description":         csvString(row, header, "description"),
		"debit_amount":        roundMoney(csvNumber(row, header, "debit")),
		"credit_amount":       roundMoney(csvNumber(row, header, "credit")),
		"signed_amount":       signedAmount,
		"running_balance":     roundMoney(csvNumber(row, header, "balance")),
		"matched_amount":      0.0,
		"remaining_amount":    roundMoney(maxFloat(signedAmount, -signedAmount)),
		"match_status":        "unmatched",
	}
}

func (s *TreasuryCoreService) treasuryPostingConfig(organizationID, locationID string) map[string]string {
	if s.config == nil {
		return map[string]string{
			"bank_transfer_clearing_account_code": defaultTransferClearingAccount,
			"bank_fee_expense_account_code":       defaultBankFeeExpenseAccount,
			"bank_interest_income_account_code":   defaultBankInterestIncomeAccount,
			"treasury_suspense_account_code":      defaultTreasurySuspenseAccount,
		}
	}
	entry, ok := s.config.Resolve("treasury.posting", strings.TrimSpace(organizationID), strings.TrimSpace(locationID))
	if !ok {
		return map[string]string{
			"bank_transfer_clearing_account_code": defaultTransferClearingAccount,
			"bank_fee_expense_account_code":       defaultBankFeeExpenseAccount,
			"bank_interest_income_account_code":   defaultBankInterestIncomeAccount,
			"treasury_suspense_account_code":      defaultTreasurySuspenseAccount,
		}
	}
	return map[string]string{
		"bank_transfer_clearing_account_code": firstNonEmptyString(textValue(entry.Value["bank_transfer_clearing_account_code"]), defaultTransferClearingAccount),
		"bank_fee_expense_account_code":       firstNonEmptyString(textValue(entry.Value["bank_fee_expense_account_code"]), defaultBankFeeExpenseAccount),
		"bank_interest_income_account_code":   firstNonEmptyString(textValue(entry.Value["bank_interest_income_account_code"]), defaultBankInterestIncomeAccount),
		"treasury_suspense_account_code":      firstNonEmptyString(textValue(entry.Value["treasury_suspense_account_code"]), defaultTreasurySuspenseAccount),
	}
}

func csvHeaderIndex(row []string) map[string]int {
	index := map[string]int{}
	for i, value := range row {
		index[strings.ToLower(strings.TrimSpace(value))] = i
	}
	return index
}

func csvHeaderIndexWithAliases(row []string, aliases map[string]string) map[string]int {
	index := csvHeaderIndex(row)
	for key, alias := range aliases {
		alias = strings.ToLower(strings.TrimSpace(alias))
		key = strings.ToLower(strings.TrimSpace(key))
		if alias == "" || key == "" {
			continue
		}
		if idx, ok := index[alias]; ok {
			index[key] = idx
		}
	}
	return index
}

func csvString(row []string, header map[string]int, key string) string {
	idx, ok := header[strings.ToLower(strings.TrimSpace(key))]
	if !ok || idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

func csvNumber(row []string, header map[string]int, key string) float64 {
	return numberValue(csvString(row, header, key))
}

func csvDateFallback(row []string, header map[string]int, key, fallback string) string {
	if value := csvString(row, header, key); value != "" {
		return value
	}
	return fallback
}

func resolveSignedStatementAmount(values map[string]any) float64 {
	amount := roundMoney(numberValue(values["amount"]))
	if amount != 0 {
		return amount
	}
	credit := roundMoney(numberValue(values["credit_amount"]))
	debit := roundMoney(numberValue(values["debit_amount"]))
	if credit != 0 || debit != 0 {
		return roundMoney(credit - debit)
	}
	return 0
}

func resolveTemplateSignedAmount(amount, debit, credit float64, signConvention string) float64 {
	switch strings.TrimSpace(signConvention) {
	case "signed":
		return roundMoney(amount)
	default:
		if amount != 0 {
			return roundMoney(amount)
		}
		return roundMoney(credit - debit)
	}
}

func normalizeTemplateNumber(raw string, template map[string]any) float64 {
	next := strings.TrimSpace(raw)
	if next == "" {
		return 0
	}
	thousands := textValue(template["thousands_separator"])
	decimal := textValue(template["decimal_separator"])
	if thousands != "" {
		next = strings.ReplaceAll(next, thousands, "")
	}
	if decimal != "" && decimal != "." {
		next = strings.ReplaceAll(next, decimal, ".")
	}
	return numberValue(next)
}

func normalizeTemplateDate(raw string, template map[string]any) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if layout := strings.TrimSpace(textValue(template["date_format"])); layout != "" {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed.Format("2006-01-02")
		}
	}
	for _, layout := range []string{"2006-01-02", "02/01/2006", "01/02/2006", "02-01-2006", "2006/01/02"} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed.Format("2006-01-02")
		}
	}
	return raw
}

func firstStatementLineDate(lines []map[string]any) string {
	best := ""
	for _, line := range lines {
		current := firstNonEmptyString(textValue(line["statement_date"]), textValue(line["value_date"]))
		if current == "" {
			continue
		}
		if best == "" || current < best {
			best = current
		}
	}
	return best
}

func lastStatementLineDate(lines []map[string]any) string {
	best := ""
	for _, line := range lines {
		current := firstNonEmptyString(textValue(line["statement_date"]), textValue(line["value_date"]))
		if current == "" {
			continue
		}
		if best == "" || current > best {
			best = current
		}
	}
	return best
}

func isBlankCSVRow(row []string) bool {
	for _, item := range row {
		if strings.TrimSpace(item) != "" {
			return false
		}
	}
	return true
}

func stringMap(raw any) map[string]string {
	out := map[string]string{}
	if values, ok := raw.(string); ok {
		values = strings.TrimSpace(values)
		if values == "" {
			return out
		}
		decoded := map[string]string{}
		if err := json.Unmarshal([]byte(values), &decoded); err == nil {
			for key, value := range decoded {
				if strings.TrimSpace(key) == "" {
					continue
				}
				out[key] = strings.TrimSpace(value)
			}
			return out
		}
		generic := map[string]any{}
		if err := json.Unmarshal([]byte(values), &generic); err == nil {
			for key, value := range generic {
				if strings.TrimSpace(key) == "" {
					continue
				}
				out[key] = textValue(value)
			}
		}
		return out
	}
	if values, ok := raw.(map[string]any); ok {
		for key, value := range values {
			if strings.TrimSpace(key) == "" {
				continue
			}
			out[key] = textValue(value)
		}
		return out
	}
	if values, ok := raw.(map[string]string); ok {
		for key, value := range values {
			if strings.TrimSpace(key) == "" {
				continue
			}
			out[key] = strings.TrimSpace(value)
		}
	}
	return out
}

func intAnyValue(value any, fallback int) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		if strings.TrimSpace(typed) == "" {
			return fallback
		}
		return int(numberValue(typed))
	default:
		if numberValue(value) == 0 {
			return fallback
		}
		return int(numberValue(value))
	}
}

func normalizeMatchText(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	replacer := strings.NewReplacer("-", " ", "_", " ", "/", " ", ".", " ", ",", " ", "  ", " ")
	return strings.Join(strings.Fields(replacer.Replace(raw)), " ")
}

func scoreTreasuryMatch(targetAmount, amount float64, targetDate, effectiveDate, targetRef, reference, targetDesc, description string) (float64, []string) {
	score := 0.0
	reasons := []string{}
	if roundMoney(targetAmount) == roundMoney(amount) {
		score += 60
		reasons = append(reasons, "exact_amount")
	} else {
		return 0, nil
	}
	if targetDate != "" && effectiveDate != "" {
		diff := dateDistanceDays(targetDate, effectiveDate)
		switch {
		case diff == 0:
			score += 20
			reasons = append(reasons, "same_day")
		case diff <= 3:
			score += 10
			reasons = append(reasons, "near_date")
		}
	}
	if targetRef != "" && reference != "" && strings.Contains(normalizeMatchText(reference), normalizeMatchText(targetRef)) {
		score += 25
		reasons = append(reasons, "reference_match")
	}
	if targetDesc != "" && description != "" {
		desc := normalizeMatchText(description)
		for _, token := range strings.Fields(normalizeMatchText(targetDesc)) {
			if len(token) >= 4 && strings.Contains(desc, token) {
				score += 3
				reasons = append(reasons, "description_hint")
				break
			}
		}
	}
	return score, reasons
}

func dateDistanceDays(left, right string) int {
	if left == "" || right == "" {
		return 999
	}
	ld, err := time.Parse("2006-01-02", left)
	if err != nil {
		return 999
	}
	rd, err := time.Parse("2006-01-02", right)
	if err != nil {
		return 999
	}
	diff := ld.Sub(rd)
	if diff < 0 {
		diff = -diff
	}
	return int(diff.Hours() / 24)
}

func treasuryDocumentCandidate(record document.Record, glAccount string) (float64, string, string, string, string, bool) {
	payload := record.Body.Payload
	switch record.Header.Type {
	case "payment_receipt":
		if record.Header.Status != "received" {
			return 0, "", "", "", "", false
		}
		account := textValue(payload["clearing_account_code"])
		if glAccount != "" && account != glAccount {
			return 0, "", "", "", "", false
		}
		return roundMoney(numberValue(payload["amount_received"])), account, textValue(payload["receipt_date"]), firstNonEmptyString(textValue(payload["payment_reference"]), record.Header.Number), firstNonEmptyString(textValue(payload["party_name"]), textValue(payload["notes"])), true
	case "payment_out":
		if record.Header.Status != "paid" {
			return 0, "", "", "", "", false
		}
		account := textValue(payload["clearing_account_code"])
		if glAccount != "" && account != glAccount {
			return 0, "", "", "", "", false
		}
		return roundMoney(numberValue(payload["amount_paid"])), account, textValue(payload["payment_date"]), firstNonEmptyString(textValue(payload["payment_reference"]), record.Header.Number), firstNonEmptyString(textValue(payload["vendor_name"]), textValue(payload["notes"])), true
	case "payment_refund":
		if record.Header.Status != "refunded" {
			return 0, "", "", "", "", false
		}
		account := textValue(payload["clearing_account_code"])
		if glAccount != "" && account != glAccount {
			return 0, "", "", "", "", false
		}
		return roundMoney(numberValue(payload["amount_refunded"])), account, textValue(payload["refund_date"]), firstNonEmptyString(textValue(payload["payment_reference"]), record.Header.Number), firstNonEmptyString(textValue(payload["party_name"]), textValue(payload["notes"])), true
	case "ledger_posting":
		lines := recordList(payload["journal_lines"])
		amount := 0.0
		for _, line := range lines {
			if textValue(line["account_code"]) != glAccount {
				continue
			}
			amount = roundMoney(maxFloat(numberValue(line["debit"]), numberValue(line["credit"])))
			break
		}
		if amount <= 0 {
			return 0, "", "", "", "", false
		}
		return amount, glAccount, textValue(payload["posting_date"]), firstNonEmptyString(textValue(payload["supporting_reference"]), record.Header.Number), firstNonEmptyString(textValue(payload["notes"]), textValue(payload["posting_rule_key"])), true
	default:
		return 0, "", "", "", "", false
	}
}

func reconciliationStatusForDifference(difference float64) string {
	if roundMoney(difference) == 0 {
		return "ready"
	}
	return "draft"
}

func readCSVAll(reader *csv.Reader) ([][]string, error) {
	rows := [][]string{}
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		rows = append(rows, record)
	}
	return rows, nil
}
