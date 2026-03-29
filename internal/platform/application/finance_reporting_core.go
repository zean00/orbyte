package application

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"orbyte/internal/platform/config"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/shared"
)

type FinanceReportingCoreService struct {
	documents *document.Service
	models    *model.Service
	config    *config.Service
}

type FinanceTrialBalanceRow struct {
	AccountCode   string  `json:"account_code"`
	AccountName   string  `json:"account_name"`
	AccountType   string  `json:"account_type"`
	ReportGroup   string  `json:"report_group"`
	NormalBalance string  `json:"normal_balance"`
	Opening       float64 `json:"opening"`
	Debit         float64 `json:"debit"`
	Credit        float64 `json:"credit"`
	Ending        float64 `json:"ending"`
}

type FinanceTrialBalanceReport struct {
	OrganizationID string                   `json:"organization_id"`
	LocationID     string                   `json:"location_id,omitempty"`
	FromDate       string                   `json:"from_date,omitempty"`
	ToDate         string                   `json:"to_date,omitempty"`
	Rows           []FinanceTrialBalanceRow `json:"rows"`
	Totals         map[string]float64       `json:"totals"`
}

type FinanceStatementRow struct {
	AccountCode string  `json:"account_code"`
	AccountName string  `json:"account_name"`
	Amount      float64 `json:"amount"`
}

type FinanceStatementSection struct {
	Key    string                `json:"key"`
	Label  string                `json:"label"`
	Amount float64               `json:"amount"`
	Rows   []FinanceStatementRow `json:"rows"`
}

type FinanceProfitAndLossReport struct {
	OrganizationID string                    `json:"organization_id"`
	LocationID     string                    `json:"location_id,omitempty"`
	FromDate       string                    `json:"from_date,omitempty"`
	ToDate         string                    `json:"to_date,omitempty"`
	Sections       []FinanceStatementSection `json:"sections"`
	GrossProfit    float64                   `json:"gross_profit"`
	NetProfit      float64                   `json:"net_profit"`
}

type FinanceBalanceSheetReport struct {
	OrganizationID string                    `json:"organization_id"`
	LocationID     string                    `json:"location_id,omitempty"`
	AsOfDate       string                    `json:"as_of_date,omitempty"`
	Sections       []FinanceStatementSection `json:"sections"`
	RetainedEarnings float64                 `json:"retained_earnings"`
}

type FinanceTaxSummaryRow struct {
	AccountCode string  `json:"account_code"`
	AccountName string  `json:"account_name"`
	TaxBucket   string  `json:"tax_bucket"`
	Debit       float64 `json:"debit"`
	Credit      float64 `json:"credit"`
	NetAmount   float64 `json:"net_amount"`
}

type FinanceTaxSummaryReport struct {
	OrganizationID string                 `json:"organization_id"`
	LocationID     string                 `json:"location_id,omitempty"`
	FromDate       string                 `json:"from_date,omitempty"`
	ToDate         string                 `json:"to_date,omitempty"`
	Rows           []FinanceTaxSummaryRow `json:"rows"`
	Totals         map[string]float64     `json:"totals"`
}

type FinanceJournalLedgerRow struct {
	PostingID           string  `json:"posting_id"`
	PostingNumber       string  `json:"posting_number"`
	PostingDate         string  `json:"posting_date"`
	SourceDocumentType  string  `json:"source_document_type"`
	SourceDocumentID    string  `json:"source_document_id"`
	AccountCode         string  `json:"account_code"`
	AccountName         string  `json:"account_name"`
	Description         string  `json:"description"`
	Debit               float64 `json:"debit"`
	Credit              float64 `json:"credit"`
}

type FinanceJournalLedgerReport struct {
	OrganizationID string                    `json:"organization_id"`
	LocationID     string                    `json:"location_id,omitempty"`
	FromDate       string                    `json:"from_date,omitempty"`
	ToDate         string                    `json:"to_date,omitempty"`
	Rows           []FinanceJournalLedgerRow `json:"rows"`
	TotalDebit     float64                   `json:"total_debit"`
	TotalCredit    float64                   `json:"total_credit"`
}

type financeAccountMeta struct {
	Name          string
	AccountType   string
	ReportGroup   string
	NormalBalance string
}

type financeLedgerLine struct {
	PostingID          string
	PostingNumber      string
	PostingDate        string
	OrganizationID     string
	LocationID         string
	SourceDocumentType string
	SourceDocumentID   string
	AccountCode        string
	AccountName        string
	Description        string
	Debit              float64
	Credit             float64
}

func NewFinanceReportingCoreService(documents *document.Service, models *model.Service, configSvc *config.Service) *FinanceReportingCoreService {
	return &FinanceReportingCoreService{documents: documents, models: models, config: configSvc}
}

func (s *FinanceReportingCoreService) ValidatePostingDateOpen(organizationID, locationID, postingDate string) error {
	postingDate = strings.TrimSpace(postingDate)
	if postingDate == "" || s.models == nil {
		return nil
	}
	items, _, err := s.models.List("accounting_period", model.Query{
		Filters: map[string]string{
			"organization_id": strings.TrimSpace(organizationID),
			"status":          "closed",
		},
		Page:     1,
		PageSize: 1000,
	})
	if err != nil && !isMissingModelDefinitionError(err) {
		return err
	}
	for _, item := range items {
		periodLocationID := strings.TrimSpace(textValue(item.Values["location_id"]))
		currentLocationID := strings.TrimSpace(locationID)
		if periodLocationID != "" && periodLocationID != currentLocationID {
			continue
		}
		startDate := textValue(item.Values["start_date"])
		endDate := textValue(item.Values["end_date"])
		if startDate != "" && postingDate < startDate {
			continue
		}
		if endDate != "" && postingDate > endDate {
			continue
		}
		return shared.Conflict("posting date falls within a closed accounting period")
	}
	return nil
}

func (s *FinanceReportingCoreService) CloseAccountingPeriod(periodID, actorID, organizationID, locationID string) (model.Record, error) {
	if s.models == nil {
		return model.Record{}, shared.Validation("accounting periods are unavailable")
	}
	record, err := s.models.Get("accounting_period", strings.TrimSpace(periodID))
	if err != nil {
		return model.Record{}, err
	}
	if err := s.validatePeriodScope(record, organizationID, locationID); err != nil {
		return model.Record{}, err
	}
	values := cloneMap(record.Values)
	values["status"] = "closed"
	values["closed_at"] = time.Now().UTC().Format(time.RFC3339)
	values["closed_by"] = actorID
	return s.models.Update("accounting_period", record.ID, actorID, values, record.Version)
}

func (s *FinanceReportingCoreService) ReopenAccountingPeriod(periodID, actorID, organizationID, locationID string) (model.Record, error) {
	if s.models == nil {
		return model.Record{}, shared.Validation("accounting periods are unavailable")
	}
	record, err := s.models.Get("accounting_period", strings.TrimSpace(periodID))
	if err != nil {
		return model.Record{}, err
	}
	if err := s.validatePeriodScope(record, organizationID, locationID); err != nil {
		return model.Record{}, err
	}
	values := cloneMap(record.Values)
	values["status"] = "open"
	values["closed_at"] = ""
	values["closed_by"] = ""
	return s.models.Update("accounting_period", record.ID, actorID, values, record.Version)
}

func (s *FinanceReportingCoreService) validatePeriodScope(record model.Record, organizationID, locationID string) error {
	recordOrgID := strings.TrimSpace(textValue(record.Values["organization_id"]))
	recordLocationID := strings.TrimSpace(textValue(record.Values["location_id"]))
	currentOrgID := strings.TrimSpace(organizationID)
	currentLocationID := strings.TrimSpace(locationID)
	if currentOrgID != "" && recordOrgID != currentOrgID {
		return shared.Forbidden("accounting period is outside the current organization scope")
	}
	if currentLocationID != "" && recordLocationID != currentLocationID {
		return shared.Forbidden("accounting period is outside the current location scope")
	}
	return nil
}

func (s *FinanceReportingCoreService) TrialBalance(organizationID, locationID, fromDate, toDate string) FinanceTrialBalanceReport {
	lines := s.ledgerLines(organizationID, locationID, fromDate, toDate)
	accounts := s.accountMetaIndex()
	type bucket struct {
		openingDebit  float64
		openingCredit float64
		debit         float64
		credit        float64
	}
	buckets := map[string]*bucket{}
	for _, line := range lines {
		entry := buckets[line.AccountCode]
		if entry == nil {
			entry = &bucket{}
			buckets[line.AccountCode] = entry
		}
		if fromDate != "" && line.PostingDate < fromDate {
			entry.openingDebit = roundMoney(entry.openingDebit + line.Debit)
			entry.openingCredit = roundMoney(entry.openingCredit + line.Credit)
			continue
		}
		entry.debit = roundMoney(entry.debit + line.Debit)
		entry.credit = roundMoney(entry.credit + line.Credit)
	}
	rows := make([]FinanceTrialBalanceRow, 0, len(buckets))
	totalOpening := 0.0
	totalDebit := 0.0
	totalCredit := 0.0
	totalEnding := 0.0
	for accountCode, bucket := range buckets {
		meta := s.financeAccountMeta(accounts, accountCode)
		opening := s.balanceFor(meta.NormalBalance, bucket.openingDebit, bucket.openingCredit)
		ending := roundMoney(opening + s.balanceFor(meta.NormalBalance, bucket.debit, bucket.credit))
		rows = append(rows, FinanceTrialBalanceRow{
			AccountCode:   accountCode,
			AccountName:   firstNonEmptyString(meta.Name, accountCode),
			AccountType:   meta.AccountType,
			ReportGroup:   meta.ReportGroup,
			NormalBalance: meta.NormalBalance,
			Opening:       opening,
			Debit:         roundMoney(bucket.debit),
			Credit:        roundMoney(bucket.credit),
			Ending:        ending,
		})
		totalOpening = roundMoney(totalOpening + opening)
		totalDebit = roundMoney(totalDebit + bucket.debit)
		totalCredit = roundMoney(totalCredit + bucket.credit)
		totalEnding = roundMoney(totalEnding + ending)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].AccountCode < rows[j].AccountCode })
	return FinanceTrialBalanceReport{
		OrganizationID: organizationID,
		LocationID:     locationID,
		FromDate:       fromDate,
		ToDate:         toDate,
		Rows:           rows,
		Totals: map[string]float64{
			"opening": totalOpening,
			"debit":   totalDebit,
			"credit":  totalCredit,
			"ending":  totalEnding,
		},
	}
}

func (s *FinanceReportingCoreService) ProfitAndLoss(organizationID, locationID, fromDate, toDate string) FinanceProfitAndLossReport {
	rows := s.statementRows(organizationID, locationID, fromDate, toDate)
	revenue := s.statementSection("revenue", "Revenue", rows, func(meta financeAccountMeta) bool { return meta.AccountType == "revenue" })
	cogs := s.statementSection("cogs", "Cost of Goods Sold", rows, func(meta financeAccountMeta) bool {
		return meta.AccountType == "expense" && meta.ReportGroup == "cogs"
	})
	operating := s.statementSection("operating_expenses", "Operating Expenses", rows, func(meta financeAccountMeta) bool {
		return meta.AccountType == "expense" && meta.ReportGroup != "cogs"
	})
	grossProfit := roundMoney(revenue.Amount - cogs.Amount)
	netProfit := roundMoney(grossProfit - operating.Amount)
	return FinanceProfitAndLossReport{
		OrganizationID: organizationID,
		LocationID:     locationID,
		FromDate:       fromDate,
		ToDate:         toDate,
		Sections:       []FinanceStatementSection{revenue, cogs, operating},
		GrossProfit:    grossProfit,
		NetProfit:      netProfit,
	}
}

func (s *FinanceReportingCoreService) BalanceSheet(organizationID, locationID, asOfDate string) FinanceBalanceSheetReport {
	rows := s.statementRows(organizationID, locationID, "", asOfDate)
	assets := s.statementSection("assets", "Assets", rows, func(meta financeAccountMeta) bool { return meta.AccountType == "asset" })
	liabilities := s.statementSection("liabilities", "Liabilities", rows, func(meta financeAccountMeta) bool { return meta.AccountType == "liability" })
	equity := s.statementSection("equity", "Equity", rows, func(meta financeAccountMeta) bool { return meta.AccountType == "equity" })
	retained := s.statementNetIncome(organizationID, locationID, asOfDate)
	equity.Amount = roundMoney(equity.Amount + retained)
	return FinanceBalanceSheetReport{
		OrganizationID:   organizationID,
		LocationID:       locationID,
		AsOfDate:         asOfDate,
		Sections:         []FinanceStatementSection{assets, liabilities, equity},
		RetainedEarnings: retained,
	}
}

func (s *FinanceReportingCoreService) TaxSummary(organizationID, locationID, fromDate, toDate string) FinanceTaxSummaryReport {
	lines := s.ledgerLines(organizationID, locationID, fromDate, toDate)
	accounts := s.accountMetaIndex()
	type bucket struct {
		name   string
		bucket string
		debit  float64
		credit float64
	}
	grouped := map[string]*bucket{}
	for _, line := range lines {
		meta := s.financeAccountMeta(accounts, line.AccountCode)
		taxBucket := s.taxBucketFor(meta, line)
		if taxBucket == "" {
			continue
		}
		entry := grouped[line.AccountCode]
		if entry == nil {
			entry = &bucket{name: firstNonEmptyString(meta.Name, line.AccountName, line.AccountCode), bucket: taxBucket}
			grouped[line.AccountCode] = entry
		}
		entry.debit = roundMoney(entry.debit + line.Debit)
		entry.credit = roundMoney(entry.credit + line.Credit)
	}
	rows := make([]FinanceTaxSummaryRow, 0, len(grouped))
	inputTotal := 0.0
	outputTotal := 0.0
	for accountCode, entry := range grouped {
		netAmount := roundMoney(entry.credit - entry.debit)
		if entry.bucket == "input_tax" {
			netAmount = roundMoney(entry.debit - entry.credit)
			inputTotal = roundMoney(inputTotal + netAmount)
		} else {
			outputTotal = roundMoney(outputTotal + roundMoney(entry.credit-entry.debit))
		}
		rows = append(rows, FinanceTaxSummaryRow{
			AccountCode: accountCode,
			AccountName: entry.name,
			TaxBucket:   entry.bucket,
			Debit:       entry.debit,
			Credit:      entry.credit,
			NetAmount:   netAmount,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].AccountCode < rows[j].AccountCode })
	return FinanceTaxSummaryReport{
		OrganizationID: organizationID,
		LocationID:     locationID,
		FromDate:       fromDate,
		ToDate:         toDate,
		Rows:           rows,
		Totals: map[string]float64{
			"input_tax":  inputTotal,
			"output_tax": outputTotal,
			"net_tax":    roundMoney(outputTotal - inputTotal),
		},
	}
}

func (s *FinanceReportingCoreService) JournalLedger(organizationID, locationID, fromDate, toDate string) FinanceJournalLedgerReport {
	lines := s.ledgerLines(organizationID, locationID, fromDate, toDate)
	rows := make([]FinanceJournalLedgerRow, 0, len(lines))
	totalDebit := 0.0
	totalCredit := 0.0
	for _, line := range lines {
		rows = append(rows, FinanceJournalLedgerRow{
			PostingID:          line.PostingID,
			PostingNumber:      line.PostingNumber,
			PostingDate:        line.PostingDate,
			SourceDocumentType: line.SourceDocumentType,
			SourceDocumentID:   line.SourceDocumentID,
			AccountCode:        line.AccountCode,
			AccountName:        line.AccountName,
			Description:        line.Description,
			Debit:              line.Debit,
			Credit:             line.Credit,
		})
		totalDebit = roundMoney(totalDebit + line.Debit)
		totalCredit = roundMoney(totalCredit + line.Credit)
	}
	return FinanceJournalLedgerReport{
		OrganizationID: organizationID,
		LocationID:     locationID,
		FromDate:       fromDate,
		ToDate:         toDate,
		Rows:           rows,
		TotalDebit:     totalDebit,
		TotalCredit:    totalCredit,
	}
}

func (s *FinanceReportingCoreService) ledgerLines(organizationID, locationID, fromDate, toDate string) []financeLedgerLine {
	if s.documents == nil {
		return nil
	}
	records := s.documents.List()
	lines := make([]financeLedgerLine, 0)
	for _, record := range records {
		if record.Header.Type != "ledger_posting" || record.Header.Status != "posted" {
			continue
		}
		if organizationID != "" && record.Header.OrganizationID != organizationID {
			continue
		}
		if locationID != "" && record.Header.LocationID != "" && record.Header.LocationID != locationID {
			continue
		}
		payload := clonedPayload(record.Body.Payload)
		postingDate := textValue(payload["posting_date"])
		if postingDate == "" {
			continue
		}
		if fromDate != "" && postingDate < fromDate {
			// keep for opening only
		}
		if toDate != "" && postingDate > toDate {
			continue
		}
		for _, line := range recordList(payload["journal_lines"]) {
			lines = append(lines, financeLedgerLine{
				PostingID:          record.Header.ID,
				PostingNumber:      firstNonEmptyString(record.Header.Number, record.Header.ID),
				PostingDate:        postingDate,
				OrganizationID:     record.Header.OrganizationID,
				LocationID:         record.Header.LocationID,
				SourceDocumentType: textValue(payload["source_document_type"]),
				SourceDocumentID:   textValue(payload["source_document_id"]),
				AccountCode:        textValue(line["account_code"]),
				AccountName:        firstNonEmptyString(textValue(line["account_name"]), textValue(line["account_code"])),
				Description:        textValue(line["description"]),
				Debit:              roundMoney(numberValue(line["debit"])),
				Credit:             roundMoney(numberValue(line["credit"])),
			})
		}
	}
	sort.Slice(lines, func(i, j int) bool {
		if lines[i].PostingDate == lines[j].PostingDate {
			if lines[i].PostingNumber == lines[j].PostingNumber {
				return lines[i].AccountCode < lines[j].AccountCode
			}
			return lines[i].PostingNumber < lines[j].PostingNumber
		}
		return lines[i].PostingDate < lines[j].PostingDate
	})
	return lines
}

func (s *FinanceReportingCoreService) statementRows(organizationID, locationID, fromDate, toDate string) map[string]FinanceStatementRow {
	lines := s.ledgerLines(organizationID, locationID, fromDate, toDate)
	accounts := s.accountMetaIndex()
	rows := map[string]FinanceStatementRow{}
	for _, line := range lines {
		meta := s.financeAccountMeta(accounts, line.AccountCode)
		amount := s.balanceFor(meta.NormalBalance, line.Debit, line.Credit)
		entry := rows[line.AccountCode]
		entry.AccountCode = line.AccountCode
		entry.AccountName = firstNonEmptyString(meta.Name, line.AccountName, line.AccountCode)
		entry.Amount = roundMoney(entry.Amount + amount)
		rows[line.AccountCode] = entry
	}
	return rows
}

func (s *FinanceReportingCoreService) statementSection(key, label string, rows map[string]FinanceStatementRow, include func(financeAccountMeta) bool) FinanceStatementSection {
	accounts := s.accountMetaIndex()
	sectionRows := make([]FinanceStatementRow, 0)
	total := 0.0
	for accountCode, row := range rows {
		meta := s.financeAccountMeta(accounts, accountCode)
		if !include(meta) {
			continue
		}
		amount := row.Amount
		sectionRows = append(sectionRows, FinanceStatementRow{
			AccountCode: row.AccountCode,
			AccountName: row.AccountName,
			Amount:      amount,
		})
		total = roundMoney(total + amount)
	}
	sort.Slice(sectionRows, func(i, j int) bool { return sectionRows[i].AccountCode < sectionRows[j].AccountCode })
	return FinanceStatementSection{Key: key, Label: label, Amount: total, Rows: sectionRows}
}

func (s *FinanceReportingCoreService) statementNetIncome(organizationID, locationID, asOfDate string) float64 {
	pnl := s.ProfitAndLoss(organizationID, locationID, "", asOfDate)
	return pnl.NetProfit
}

func (s *FinanceReportingCoreService) accountMetaIndex() map[string]financeAccountMeta {
	index := map[string]financeAccountMeta{}
	if s.models == nil {
		return index
	}
	items, _, err := s.models.List("finance_account", model.Query{Page: 1, PageSize: 1000})
	if err != nil && !isMissingModelDefinitionError(err) {
		return index
	}
	for _, item := range items {
		code := textValue(item.Values["code"])
		if code == "" {
			continue
		}
		index[code] = financeAccountMeta{
			Name:          firstNonEmptyString(textValue(item.Values["name"]), code),
			AccountType:   firstNonEmptyString(textValue(item.Values["account_type"]), s.fallbackAccountType(code)),
			ReportGroup:   firstNonEmptyString(textValue(item.Values["report_group"]), s.fallbackReportGroup(code)),
			NormalBalance: firstNonEmptyString(textValue(item.Values["normal_balance"]), s.fallbackNormalBalance(code)),
		}
	}
	return index
}

func (s *FinanceReportingCoreService) financeAccountMeta(index map[string]financeAccountMeta, accountCode string) financeAccountMeta {
	if meta, ok := index[accountCode]; ok {
		return meta
	}
	return financeAccountMeta{
		Name:          accountCode,
		AccountType:   s.fallbackAccountType(accountCode),
		ReportGroup:   s.fallbackReportGroup(accountCode),
		NormalBalance: s.fallbackNormalBalance(accountCode),
	}
}

func (s *FinanceReportingCoreService) fallbackAccountType(accountCode string) string {
	switch prefix := leadingAccountPrefix(accountCode); prefix {
	case "1":
		return "asset"
	case "2":
		return "liability"
	case "3":
		return "equity"
	case "4":
		return "revenue"
	default:
		return "expense"
	}
}

func (s *FinanceReportingCoreService) fallbackReportGroup(accountCode string) string {
	upper := strings.ToUpper(strings.TrimSpace(accountCode))
	switch s.fallbackAccountType(accountCode) {
	case "asset":
		if strings.Contains(upper, "AR") {
			return "accounts_receivable"
		}
		if strings.Contains(upper, "INV") || strings.Contains(upper, "FG") || strings.Contains(upper, "RM") {
			return "inventory"
		}
		if strings.Contains(upper, "WIP") {
			return "wip"
		}
		if strings.Contains(upper, "VATIN") || strings.Contains(upper, "TAXIN") {
			return "tax_input"
		}
		return "assets"
	case "liability":
		if strings.Contains(upper, "AP") {
			return "accounts_payable"
		}
		if strings.Contains(upper, "VAT") || strings.Contains(upper, "TAX") {
			return "tax_output"
		}
		return "liabilities"
	case "equity":
		return "equity"
	case "revenue":
		return "revenue"
	default:
		if strings.Contains(upper, "COGS") {
			return "cogs"
		}
		return "operating_expense"
	}
}

func (s *FinanceReportingCoreService) fallbackNormalBalance(accountCode string) string {
	switch s.fallbackAccountType(accountCode) {
	case "asset", "expense":
		return "debit"
	default:
		return "credit"
	}
}

func (s *FinanceReportingCoreService) taxBucketFor(meta financeAccountMeta, line financeLedgerLine) string {
	group := strings.TrimSpace(meta.ReportGroup)
	if group == "tax_input" || group == "tax_output" {
		return group
	}
	desc := strings.ToLower(strings.TrimSpace(line.Description))
	code := strings.ToUpper(strings.TrimSpace(line.AccountCode))
	switch {
	case strings.Contains(desc, "input tax"), strings.Contains(desc, "tax receivable"), strings.Contains(code, "VATIN"):
		return "input_tax"
	case strings.Contains(desc, "tax payable"), strings.Contains(code, "VATOUT"), strings.Contains(code, "VAT"):
		return "output_tax"
	default:
		return ""
	}
}

func (s *FinanceReportingCoreService) balanceFor(normalBalance string, debit, credit float64) float64 {
	if strings.EqualFold(strings.TrimSpace(normalBalance), "credit") {
		return roundMoney(credit - debit)
	}
	return roundMoney(debit - credit)
}

func leadingAccountPrefix(accountCode string) string {
	accountCode = strings.TrimSpace(accountCode)
	if accountCode == "" {
		return ""
	}
	return string(accountCode[0])
}

func financeReportDateOrToday(value string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return time.Now().UTC().Format("2006-01-02")
}

func (s *FinanceReportingCoreService) ValidateAccountingPeriod(values map[string]any) error {
	startDate := strings.TrimSpace(textValue(values["start_date"]))
	endDate := strings.TrimSpace(textValue(values["end_date"]))
	if startDate == "" || endDate == "" {
		return nil
	}
	if startDate > endDate {
		return shared.Validation("accounting period start_date cannot be after end_date")
	}
	return nil
}

func (s *FinanceReportingCoreService) EnsureFinanceAccountDefaults(values map[string]any) map[string]any {
	next := cloneMap(values)
	code := strings.TrimSpace(textValue(next["code"]))
	if next["account_type"] == nil || textValue(next["account_type"]) == "" {
		next["account_type"] = s.fallbackAccountType(code)
	}
	if next["report_group"] == nil || textValue(next["report_group"]) == "" {
		next["report_group"] = s.fallbackReportGroup(code)
	}
	if next["normal_balance"] == nil || textValue(next["normal_balance"]) == "" {
		next["normal_balance"] = s.fallbackNormalBalance(code)
	}
	if next["status"] == nil || textValue(next["status"]) == "" {
		next["status"] = "active"
	}
	if next["name"] == nil || textValue(next["name"]) == "" {
		next["name"] = code
	}
	return next
}

func (s *FinanceReportingCoreService) EnsureAccountingPeriodDefaults(values map[string]any) map[string]any {
	next := cloneMap(values)
	next["organization_id"] = textValue(next["organization_id"])
	next["location_id"] = textValue(next["location_id"])
	next["start_date"] = textValue(next["start_date"])
	next["end_date"] = textValue(next["end_date"])
	if next["period_key"] == nil || textValue(next["period_key"]) == "" {
		startDate := textValue(next["start_date"])
		if len(startDate) >= 7 {
			next["period_key"] = startDate[:7]
		}
	}
	if next["status"] == nil || textValue(next["status"]) == "" {
		next["status"] = "open"
	}
	return next
}

func financeReportTitleFromPath(path string) string {
	switch {
	case strings.Contains(path, "trial-balance"):
		return "Trial Balance"
	case strings.Contains(path, "profit-and-loss"):
		return "Profit and Loss"
	case strings.Contains(path, "balance-sheet"):
		return "Balance Sheet"
	case strings.Contains(path, "tax-summary"):
		return "Tax Summary"
	default:
		return "Journal Ledger"
	}
}

func financeReportPathKey(path string) string {
	switch {
	case strings.Contains(path, "trial-balance"):
		return "trial-balance"
	case strings.Contains(path, "profit-and-loss"):
		return "profit-and-loss"
	case strings.Contains(path, "balance-sheet"):
		return "balance-sheet"
	case strings.Contains(path, "tax-summary"):
		return "tax-summary"
	default:
		return "journal-ledger"
	}
}

func financeReportEndpoint(path string) string {
	return "/ui/data/finance/" + financeReportPathKey(path)
}

func financeReportNavItems() []map[string]string {
	return []map[string]string{
		{"key": "trial-balance", "label": "Trial Balance", "path": "/ui/finance/trial-balance"},
		{"key": "profit-and-loss", "label": "Profit and Loss", "path": "/ui/finance/profit-and-loss"},
		{"key": "balance-sheet", "label": "Balance Sheet", "path": "/ui/finance/balance-sheet"},
		{"key": "tax-summary", "label": "Tax Summary", "path": "/ui/finance/tax-summary"},
		{"key": "journal-ledger", "label": "Journal Ledger", "path": "/ui/finance/journal-ledger"},
	}
}

func financeReportEmptyState(title string) map[string]any {
	return map[string]any{
		"title": title,
		"rows":  []map[string]any{},
		"meta":  map[string]any{"generated_at": time.Now().UTC().Format(time.RFC3339)},
	}
}

func financeReportError(err error) error {
	if err == nil {
		return nil
	}
	return shared.Validation(fmt.Sprintf("finance report error: %v", err))
}
