package application

import (
	"encoding/csv"
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
	OrganizationID string                   `json:"organization_id"`
	LocationID     string                   `json:"location_id,omitempty"`
	AsOfDate       string                   `json:"as_of_date,omitempty"`
	Rows           []TreasuryCashPositionRow `json:"rows"`
	Totals         map[string]float64       `json:"totals"`
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
	OrganizationID string                     `json:"organization_id"`
	LocationID     string                     `json:"location_id,omitempty"`
	AsOfDate       string                     `json:"as_of_date,omitempty"`
	Rows           []TreasuryClearingBalanceRow `json:"rows"`
	Totals         map[string]float64         `json:"totals"`
}

type TreasuryBankReconciliationLine struct {
	StatementLineID   string  `json:"statement_line_id"`
	StatementDate     string  `json:"statement_date"`
	Description       string  `json:"description"`
	Reference         string  `json:"reference"`
	Amount            float64 `json:"amount"`
	MatchedAmount     float64 `json:"matched_amount"`
	RemainingAmount   float64 `json:"remaining_amount"`
	MatchStatus       string  `json:"match_status"`
	MatchedSourceType string  `json:"matched_source_type,omitempty"`
	MatchedSourceID   string  `json:"matched_source_id,omitempty"`
}

type TreasuryBankReconciliationReport struct {
	ReconciliationID      string                            `json:"reconciliation_id,omitempty"`
	TreasuryAccountID     string                            `json:"treasury_account_id,omitempty"`
	StatementID           string                            `json:"statement_id,omitempty"`
	OrganizationID        string                            `json:"organization_id"`
	LocationID            string                            `json:"location_id,omitempty"`
	AsOfDate              string                            `json:"as_of_date,omitempty"`
	BookBalance           float64                           `json:"book_balance"`
	StatementBalance      float64                           `json:"statement_balance"`
	MatchedAmount         float64                           `json:"matched_amount"`
	OutstandingBookAmount float64                           `json:"outstanding_book_amount"`
	Difference            float64                           `json:"difference"`
	Lines                 []TreasuryBankReconciliationLine  `json:"lines"`
	Exceptions            []model.Record                    `json:"exceptions"`
}

type TreasuryTransferRegisterRow struct {
	TransferID        string  `json:"transfer_id"`
	TransferDate      string  `json:"transfer_date"`
	FromAccountID     string  `json:"from_account_id"`
	FromAccountCode   string  `json:"from_account_code"`
	ToAccountID       string  `json:"to_account_id"`
	ToAccountCode     string  `json:"to_account_code"`
	Amount            float64 `json:"amount"`
	Reference         string  `json:"reference,omitempty"`
	Status            string  `json:"status"`
	PostingID         string  `json:"posting_id,omitempty"`
}

type TreasuryTransferRegisterReport struct {
	OrganizationID string                       `json:"organization_id"`
	LocationID     string                       `json:"location_id,omitempty"`
	AsOfDate       string                       `json:"as_of_date,omitempty"`
	Rows           []TreasuryTransferRegisterRow `json:"rows"`
	Totals         map[string]float64           `json:"totals"`
}

type TreasuryExceptionReport struct {
	OrganizationID string         `json:"organization_id"`
	LocationID     string         `json:"location_id,omitempty"`
	AsOfDate       string         `json:"as_of_date,omitempty"`
	Items          []model.Record `json:"items"`
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

func (s *TreasuryCoreService) ImportStatementCSV(organizationID, locationID, treasuryAccountID, actorID string, payload map[string]any, rawCSV string) (map[string]any, error) {
	account, err := s.models.Get("treasury_account", strings.TrimSpace(treasuryAccountID))
	if err != nil {
		return nil, err
	}
	if err := s.validateTreasuryScope(account, organizationID, locationID); err != nil {
		return nil, err
	}
	reader := csv.NewReader(strings.NewReader(strings.TrimSpace(rawCSV)))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, shared.Validation("invalid treasury statement csv")
	}
	if len(records) < 2 {
		return nil, shared.Validation("statement csv must include a header and at least one row")
	}
	header := csvHeaderIndex(records[0])
	statementDate := firstNonEmptyString(textValue(payload["statement_date"]), time.Now().UTC().Format("2006-01-02"))
	statementValues := map[string]any{
		"organization_id":      organizationID,
		"location_id":          locationID,
		"treasury_account_id":  account.ID,
		"statement_number":     firstNonEmptyString(textValue(payload["statement_number"]), posNumber("STMT")),
		"statement_date":       statementDate,
		"from_date":            firstNonEmptyString(textValue(payload["from_date"]), csvDateFallback(records[1], header, "date", statementDate)),
		"to_date":              firstNonEmptyString(textValue(payload["to_date"]), csvDateFallback(records[len(records)-1], header, "date", statementDate)),
		"opening_balance":      roundMoney(numberValue(payload["opening_balance"])),
		"closing_balance":      roundMoney(numberValue(payload["closing_balance"])),
		"import_method":        "csv",
		"status":               "imported",
		"source_file_name":     textValue(payload["source_file_name"]),
	}
	statement, err := s.models.Create("bank_statement", actorID, statementValues)
	if err != nil {
		return nil, err
	}
	lines := make([]model.Record, 0, len(records)-1)
	for _, row := range records[1:] {
		if len(row) == 0 {
			continue
		}
		lineValues := s.parseStatementCSVRow(organizationID, locationID, statement, row, header)
		line, err := s.models.Create("bank_statement_line", actorID, lineValues)
		if err != nil {
			return nil, err
		}
		lines = append(lines, line)
	}
	if numberValue(statement.Values["closing_balance"]) == 0 {
		if updated, err := s.refreshStatementBalance(statement.ID, actorID); err == nil {
			statement = updated
		}
	}
	return map[string]any{"statement": statement, "lines": lines}, nil
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
		"status":              "imported",
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
			"match_status":        "open",
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
	if updated, err := s.models.Update("bank_statement", statement.ID, actorID, mergeModelValues(statement.Values, map[string]any{"status": "reconciling"}), statement.Version); err == nil {
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
	match, err := s.models.Create("bank_reconciliation_match", actorID, map[string]any{
		"organization_id":       textValue(reconciliation.Values["organization_id"]),
		"location_id":           textValue(reconciliation.Values["location_id"]),
		"bank_reconciliation_id": reconciliation.ID,
		"bank_statement_line_id": line.ID,
		"matched_source_type":   sourceType,
		"matched_source_id":     sourceID,
		"matched_amount":        amount,
		"match_kind":            firstNonEmptyString(textValue(payload["match_kind"]), "manual"),
		"notes":                 textValue(payload["notes"]),
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
			"status":            "open",
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
			_, _ = s.models.Update("bank_statement", statement.ID, actorID, mergeModelValues(statement.Values, map[string]any{"status": "reconciled"}), statement.Version)
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
		"organization_id":           organizationID,
		"location_id":               locationID,
		"transfer_date":             firstNonEmptyString(textValue(payload["transfer_date"]), time.Now().UTC().Format("2006-01-02")),
		"from_treasury_account_id":  fromID,
		"to_treasury_account_id":    toID,
		"from_account_code":         textValue(fromAccount.Values["account_code"]),
		"to_account_code":           textValue(toAccount.Values["account_code"]),
		"amount":                    amount,
		"reference":                 textValue(payload["reference"]),
		"notes":                     textValue(payload["notes"]),
		"status":                    "draft",
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
	account, err := s.models.Get("treasury_account", report.TreasuryAccountID)
	if err == nil {
		glAccount := textValue(account.Values["gl_account_code"])
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
	exceptions, _, err := s.models.List("treasury_exception", model.Query{Page: 1, PageSize: 500, Filters: map[string]string{"bank_statement_id": statement.ID, "status": "open"}})
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
				if record.Header.Status != "received" { continue }
				date := firstNonEmptyString(textValue(record.Body.Payload["payment_date"]), textValue(record.Body.Payload["receipt_date"]))
				if asOfDate != "" && date != "" && date > asOfDate { continue }
				open := roundMoney(numberValue(record.Body.Payload["unapplied_amount"]))
				if open == 0 { continue }
				row := TreasuryClearingBalanceRow{SourceKind: "payment_receipt", AccountCode: textValue(record.Body.Payload["clearing_account_code"]), ReferenceID: record.Header.ID, ReferenceNo: firstNonEmptyString(record.Header.Number, record.Header.ID), OpenAmount: open, Status: record.Header.Status, EffectiveDate: date}
				report.Rows = append(report.Rows, row)
				report.Totals[row.AccountCode] = roundMoney(report.Totals[row.AccountCode] + row.OpenAmount)
			case "payment_out":
				if record.Header.Status != "paid" { continue }
				date := firstNonEmptyString(textValue(record.Body.Payload["payment_date"]), textValue(record.Body.Payload["bill_date"]))
				if asOfDate != "" && date != "" && date > asOfDate { continue }
				open := roundMoney(numberValue(record.Body.Payload["unapplied_amount"]))
				if open == 0 { continue }
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
	values["status"] = firstNonEmptyString(strings.TrimSpace(status), "resolved")
	values["note"] = strings.TrimSpace(note)
	values["resolved_at"] = time.Now().UTC().Format(time.RFC3339)
	values["resolved_by"] = actorID
	return s.models.Update("treasury_exception", item.ID, actorID, values, item.Version)
}

func (s *TreasuryCoreService) syncStatementExceptions(reconciliation, statement model.Record, actorID string) error {
	lines, _, err := s.models.List("bank_statement_line", model.Query{Page: 1, PageSize: 500, Filters: map[string]string{"bank_statement_id": statement.ID}})
	if err != nil && !isMissingModelDefinitionError(err) {
		return err
	}
	for _, line := range lines {
		remaining := roundMoney(numberValue(line.Values["remaining_amount"]))
		if remaining > 0 {
			_, err := s.upsertTreasuryModel("treasury_exception", map[string]string{"bank_statement_line_id": line.ID}, actorID, map[string]any{
				"organization_id":       textValue(reconciliation.Values["organization_id"]),
				"location_id":           textValue(reconciliation.Values["location_id"]),
				"treasury_account_id":   textValue(reconciliation.Values["treasury_account_id"]),
				"bank_statement_id":     statement.ID,
				"bank_statement_line_id": line.ID,
				"exception_kind":        "unmatched_statement_line",
				"statement_date":        textValue(line.Values["statement_date"]),
				"description":           textValue(line.Values["description"]),
				"amount":                remaining,
				"status":                "open",
			})
			if err != nil {
				return err
			}
			continue
		}
		if current, ok := s.findTreasuryModelByFields("treasury_exception", map[string]string{"bank_statement_line_id": line.ID}); ok && textValue(current.Values["status"]) == "open" {
			if _, err := s.models.Update("treasury_exception", current.ID, actorID, mergeModelValues(current.Values, map[string]any{"status": "resolved", "resolved_at": time.Now().UTC().Format(time.RFC3339), "resolved_by": actorID}), current.Version); err != nil {
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
		"match_status":        "open",
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
