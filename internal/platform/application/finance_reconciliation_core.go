package application

import (
	"sort"
	"strings"
	"time"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
)

type FinanceAgingItem struct {
	DocumentID      string  `json:"document_id"`
	DocumentNumber  string  `json:"document_number"`
	DocumentType    string  `json:"document_type"`
	AccountCode     string  `json:"account_code"`
	CounterpartyID  string  `json:"counterparty_id"`
	CounterpartyName string `json:"counterparty_name"`
	DocumentDate    string  `json:"document_date"`
	DueDate         string  `json:"due_date"`
	TotalAmount     float64 `json:"total_amount"`
	SettledAmount   float64 `json:"settled_amount"`
	OpenAmount      float64 `json:"open_amount"`
	DaysOverdue     int     `json:"days_overdue"`
	AgingBucket     string  `json:"aging_bucket"`
	Status          string  `json:"status"`
}

type FinanceAgingGroup struct {
	CounterpartyID   string                 `json:"counterparty_id"`
	CounterpartyName string                 `json:"counterparty_name"`
	OpenAmount       float64                `json:"open_amount"`
	Items            []FinanceAgingItem     `json:"items"`
	Aging            map[string]float64     `json:"aging"`
}

type FinanceAgingReport struct {
	OrganizationID string               `json:"organization_id"`
	LocationID     string               `json:"location_id,omitempty"`
	AsOfDate       string               `json:"as_of_date,omitempty"`
	Kind           string               `json:"kind"`
	Totals         map[string]float64   `json:"totals"`
	Groups         []FinanceAgingGroup  `json:"groups"`
	Items          []FinanceAgingItem   `json:"items"`
}

type FinanceReconciliationAccountRow struct {
	AccountCode     string  `json:"account_code"`
	AccountName     string  `json:"account_name"`
	SubledgerAmount float64 `json:"subledger_amount"`
	GLAmount        float64 `json:"gl_amount"`
	Difference      float64 `json:"difference"`
}

type FinanceReconciliationMismatch struct {
	DocumentID      string  `json:"document_id,omitempty"`
	DocumentNumber  string  `json:"document_number,omitempty"`
	DocumentType    string  `json:"document_type,omitempty"`
	AccountCode     string  `json:"account_code,omitempty"`
	CounterpartyID  string  `json:"counterparty_id,omitempty"`
	CounterpartyName string `json:"counterparty_name,omitempty"`
	SubledgerAmount float64 `json:"subledger_amount"`
	GLAmount        float64 `json:"gl_amount"`
	Difference      float64 `json:"difference"`
	Reason          string  `json:"reason"`
}

type FinanceReconciliationReport struct {
	OrganizationID string                             `json:"organization_id"`
	LocationID     string                             `json:"location_id,omitempty"`
	AsOfDate       string                             `json:"as_of_date,omitempty"`
	Kind           string                             `json:"kind"`
	SubledgerTotal float64                            `json:"subledger_total"`
	GLTotal        float64                            `json:"gl_total"`
	Difference     float64                            `json:"difference"`
	Accounts       []FinanceReconciliationAccountRow  `json:"accounts"`
	Mismatches     []FinanceReconciliationMismatch    `json:"mismatches"`
	Items          []FinanceAgingItem                 `json:"items"`
}

type FinanceReconciliationCoreService struct {
	documents *document.Service
	models    *model.Service
	finance   *FinanceReportingCoreService
}

func NewFinanceReconciliationCoreService(documents *document.Service, models *model.Service, finance *FinanceReportingCoreService) *FinanceReconciliationCoreService {
	return &FinanceReconciliationCoreService{documents: documents, models: models, finance: finance}
}

func (s *FinanceReconciliationCoreService) ARAging(organizationID, locationID, asOfDate, partyID, agingBucket string) FinanceAgingReport {
	return s.agingReport("ar", organizationID, locationID, asOfDate, partyID, agingBucket)
}

func (s *FinanceReconciliationCoreService) APAging(organizationID, locationID, asOfDate, vendorID, agingBucket string) FinanceAgingReport {
	return s.agingReport("ap", organizationID, locationID, asOfDate, vendorID, agingBucket)
}

func (s *FinanceReconciliationCoreService) ARReconciliation(organizationID, locationID, asOfDate, partyID, accountCode string) FinanceReconciliationReport {
	return s.reconciliationReport("ar", organizationID, locationID, asOfDate, partyID, accountCode)
}

func (s *FinanceReconciliationCoreService) APReconciliation(organizationID, locationID, asOfDate, vendorID, accountCode string) FinanceReconciliationReport {
	return s.reconciliationReport("ap", organizationID, locationID, asOfDate, vendorID, accountCode)
}

func (s *FinanceReconciliationCoreService) agingReport(kind, organizationID, locationID, asOfDate, counterpartyID, requestedBucket string) FinanceAgingReport {
	asOfDate = normalizeAsOfDate(asOfDate)
	items := s.collectAgingItems(kind, organizationID, locationID, asOfDate, counterpartyID, requestedBucket)
	report := FinanceAgingReport{
		OrganizationID: organizationID,
		LocationID:     locationID,
		AsOfDate:       asOfDate,
		Kind:           kind,
		Totals:         financeAgingTotals(),
		Items:          items,
		Groups:         make([]FinanceAgingGroup, 0),
	}
	groupMap := map[string]*FinanceAgingGroup{}
	for _, item := range items {
		report.Totals["open_amount"] = roundMoney(report.Totals["open_amount"] + item.OpenAmount)
		report.Totals[item.AgingBucket] = roundMoney(report.Totals[item.AgingBucket] + item.OpenAmount)
		groupKey := firstNonEmptyString(item.CounterpartyID, item.CounterpartyName, "_unknown")
		entry := groupMap[groupKey]
		if entry == nil {
			entry = &FinanceAgingGroup{
				CounterpartyID:   item.CounterpartyID,
				CounterpartyName: item.CounterpartyName,
				Items:            make([]FinanceAgingItem, 0),
				Aging:            financeAgingTotals(),
			}
			groupMap[groupKey] = entry
		}
		entry.OpenAmount = roundMoney(entry.OpenAmount + item.OpenAmount)
		entry.Aging[item.AgingBucket] = roundMoney(entry.Aging[item.AgingBucket] + item.OpenAmount)
		entry.Items = append(entry.Items, item)
	}
	for _, entry := range groupMap {
		sort.Slice(entry.Items, func(i, j int) bool {
			if entry.Items[i].DueDate != entry.Items[j].DueDate {
				return entry.Items[i].DueDate < entry.Items[j].DueDate
			}
			return entry.Items[i].DocumentNumber < entry.Items[j].DocumentNumber
		})
		report.Groups = append(report.Groups, *entry)
	}
	sort.Slice(report.Groups, func(i, j int) bool {
		if report.Groups[i].CounterpartyName != report.Groups[j].CounterpartyName {
			return report.Groups[i].CounterpartyName < report.Groups[j].CounterpartyName
		}
		return report.Groups[i].CounterpartyID < report.Groups[j].CounterpartyID
	})
	return report
}

func (s *FinanceReconciliationCoreService) reconciliationReport(kind, organizationID, locationID, asOfDate, counterpartyID, accountCode string) FinanceReconciliationReport {
	asOfDate = normalizeAsOfDate(asOfDate)
	items := s.collectAgingItems(kind, organizationID, locationID, asOfDate, counterpartyID, "")
	if accountCode != "" {
		filtered := make([]FinanceAgingItem, 0, len(items))
		for _, item := range items {
			if item.AccountCode == accountCode {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	subledgerByAccount := map[string]float64{}
	for _, item := range items {
		subledgerByAccount[item.AccountCode] = roundMoney(subledgerByAccount[item.AccountCode] + item.OpenAmount)
	}
	accountIndex := s.accountMetaIndex()
	glByAccount := s.glBalances(kind, organizationID, locationID, asOfDate, counterpartyID, accountCode, accountIndex)
	report := FinanceReconciliationReport{
		OrganizationID: organizationID,
		LocationID:     locationID,
		AsOfDate:       asOfDate,
		Kind:           kind,
		Accounts:       make([]FinanceReconciliationAccountRow, 0),
		Mismatches:     make([]FinanceReconciliationMismatch, 0),
		Items:          items,
	}
	accountCodes := map[string]struct{}{}
	for code := range subledgerByAccount {
		accountCodes[code] = struct{}{}
	}
	for code := range glByAccount {
		accountCodes[code] = struct{}{}
	}
	for code := range accountCodes {
		subledgerAmount := roundMoney(subledgerByAccount[code])
		glAmount := roundMoney(glByAccount[code])
		diff := roundMoney(subledgerAmount - glAmount)
		row := FinanceReconciliationAccountRow{
			AccountCode:     code,
			AccountName:     s.accountNameFromIndex(accountIndex, code),
			SubledgerAmount: subledgerAmount,
			GLAmount:        glAmount,
			Difference:      diff,
		}
		report.Accounts = append(report.Accounts, row)
		report.SubledgerTotal = roundMoney(report.SubledgerTotal + subledgerAmount)
		report.GLTotal = roundMoney(report.GLTotal + glAmount)
		if diff != 0 {
			report.Mismatches = append(report.Mismatches, FinanceReconciliationMismatch{
				AccountCode:     code,
				SubledgerAmount: subledgerAmount,
				GLAmount:        glAmount,
				Difference:      diff,
				Reason:          "subledger and ledger balances differ for the account",
			})
		}
	}
	report.Difference = roundMoney(report.SubledgerTotal - report.GLTotal)
	for _, item := range items {
		if item.AccountCode == "" {
			report.Mismatches = append(report.Mismatches, FinanceReconciliationMismatch{
				DocumentID:       item.DocumentID,
				DocumentNumber:   item.DocumentNumber,
				DocumentType:     item.DocumentType,
				CounterpartyID:   item.CounterpartyID,
				CounterpartyName: item.CounterpartyName,
				SubledgerAmount:  item.OpenAmount,
				GLAmount:         0,
				Difference:       item.OpenAmount,
				Reason:           "source document is missing receivable/payable account code",
			})
		}
	}
	sort.Slice(report.Accounts, func(i, j int) bool { return report.Accounts[i].AccountCode < report.Accounts[j].AccountCode })
	sort.Slice(report.Mismatches, func(i, j int) bool {
		if report.Mismatches[i].AccountCode != report.Mismatches[j].AccountCode {
			return report.Mismatches[i].AccountCode < report.Mismatches[j].AccountCode
		}
		return report.Mismatches[i].DocumentNumber < report.Mismatches[j].DocumentNumber
	})
	return report
}

func (s *FinanceReconciliationCoreService) collectAgingItems(kind, organizationID, locationID, asOfDate, counterpartyID, requestedBucket string) []FinanceAgingItem {
	if s.documents == nil {
		return nil
	}
	items := make([]FinanceAgingItem, 0)
	for _, record := range s.documents.List() {
		if !matchesCommercialScope(record, organizationID, locationID) {
			continue
		}
		item, ok := s.agingItemForRecord(kind, record, asOfDate)
		if !ok {
			continue
		}
		if counterpartyID != "" && item.CounterpartyID != counterpartyID {
			continue
		}
		if requestedBucket != "" && item.AgingBucket != requestedBucket {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CounterpartyName != items[j].CounterpartyName {
			return items[i].CounterpartyName < items[j].CounterpartyName
		}
		if items[i].DueDate != items[j].DueDate {
			return items[i].DueDate < items[j].DueDate
		}
		return items[i].DocumentNumber < items[j].DocumentNumber
	})
	return items
}

func (s *FinanceReconciliationCoreService) agingItemForRecord(kind string, record document.Record, asOfDate string) (FinanceAgingItem, bool) {
	payload := clonedPayload(record.Body.Payload)
	switch kind {
	case "ar":
		if record.Header.Type != "invoice" {
			return FinanceAgingItem{}, false
		}
		if record.Header.Status != "issued" && record.Header.Status != "partially_paid" {
			return FinanceAgingItem{}, false
		}
		documentDate := textValue(payload["invoice_date"])
		if documentDate == "" || documentDate > asOfDate {
			return FinanceAgingItem{}, false
		}
		openAmount := roundMoney(numberValue(payload["balance_due_amount"]))
		if openAmount <= 0 {
			return FinanceAgingItem{}, false
		}
		totalAmount := roundMoney(numberValue(payload["total_amount"]))
		credited := roundMoney(numberValue(payload["credited_amount"]))
		refunded := roundMoney(numberValue(payload["refunded_amount"]))
		paid := roundMoney(numberValue(payload["paid_amount"]))
		settledAmount := roundMoney(paid + credited + refunded)
		dueDate := firstNonEmptyString(textValue(payload["due_date"]), documentDate)
		daysOverdue := maxInt(dateDiffDays(dueDate, asOfDate), 0)
		return FinanceAgingItem{
			DocumentID:       record.Header.ID,
			DocumentNumber:   firstNonEmptyString(record.Header.Number, record.Header.ID),
			DocumentType:     record.Header.Type,
			AccountCode:      textValue(payload["receivable_account_code"]),
			CounterpartyID:   textValue(payload["party_id"]),
			CounterpartyName: textValue(payload["party_name"]),
			DocumentDate:     documentDate,
			DueDate:          dueDate,
			TotalAmount:      totalAmount,
			SettledAmount:    settledAmount,
			OpenAmount:       openAmount,
			DaysOverdue:      daysOverdue,
			AgingBucket:      financeAgingBucket(dueDate, asOfDate),
			Status:           record.Header.Status,
		}, true
	case "ap":
		if record.Header.Type != "vendor_bill" {
			return FinanceAgingItem{}, false
		}
		if record.Header.Status != "issued" && record.Header.Status != "partially_paid" {
			return FinanceAgingItem{}, false
		}
		documentDate := textValue(payload["bill_date"])
		if documentDate == "" || documentDate > asOfDate {
			return FinanceAgingItem{}, false
		}
		openAmount := roundMoney(numberValue(payload["balance_due_amount"]))
		if openAmount <= 0 {
			return FinanceAgingItem{}, false
		}
		totalAmount := roundMoney(numberValue(payload["total_amount"]))
		credited := roundMoney(numberValue(payload["credited_amount"]))
		paid := roundMoney(numberValue(payload["paid_amount"]))
		settledAmount := roundMoney(paid + credited)
		dueDate := firstNonEmptyString(textValue(payload["due_date"]), documentDate)
		daysOverdue := maxInt(dateDiffDays(dueDate, asOfDate), 0)
		return FinanceAgingItem{
			DocumentID:       record.Header.ID,
			DocumentNumber:   firstNonEmptyString(record.Header.Number, record.Header.ID),
			DocumentType:     record.Header.Type,
			AccountCode:      textValue(payload["payable_account_code"]),
			CounterpartyID:   textValue(payload["vendor_id"]),
			CounterpartyName: textValue(payload["vendor_name"]),
			DocumentDate:     documentDate,
			DueDate:          dueDate,
			TotalAmount:      totalAmount,
			SettledAmount:    settledAmount,
			OpenAmount:       openAmount,
			DaysOverdue:      daysOverdue,
			AgingBucket:      financeAgingBucket(dueDate, asOfDate),
			Status:           record.Header.Status,
		}, true
	default:
		return FinanceAgingItem{}, false
	}
}

func (s *FinanceReconciliationCoreService) glBalances(kind, organizationID, locationID, asOfDate, counterpartyID, accountFilter string, accountIndex map[string]financeAccountMeta) map[string]float64 {
	balances := map[string]float64{}
	if s.documents == nil {
		return balances
	}
	sourceDocumentCache := map[string]document.Record{}
	for _, record := range s.documents.List() {
		if !matchesCommercialScope(record, organizationID, locationID) {
			continue
		}
		if record.Header.Type != "ledger_posting" || record.Header.Status != "posted" {
			continue
		}
		payload := clonedPayload(record.Body.Payload)
		postingDate := textValue(payload["posting_date"])
		if postingDate == "" || postingDate > asOfDate {
			continue
		}
		if counterpartyID != "" && !s.ledgerPostingMatchesCounterparty(kind, payload, counterpartyID, sourceDocumentCache) {
			continue
		}
		for _, rawLine := range recordList(payload["journal_lines"]) {
			code := textValue(rawLine["account_code"])
			if code == "" {
				continue
			}
			if accountFilter != "" && code != accountFilter {
				continue
			}
			meta := s.financeAccountMeta(accountIndex, code)
			accountKind := strings.ToLower(firstNonEmptyString(meta.ReportGroup, meta.AccountType))
			switch kind {
			case "ar":
				if !strings.Contains(accountKind, "receivable") {
					continue
				}
				balances[code] = roundMoney(balances[code] + numberValue(rawLine["debit"]) - numberValue(rawLine["credit"]))
			case "ap":
				if !strings.Contains(accountKind, "payable") {
					continue
				}
				balances[code] = roundMoney(balances[code] + numberValue(rawLine["credit"]) - numberValue(rawLine["debit"]))
			}
		}
	}
	return balances
}

func (s *FinanceReconciliationCoreService) ledgerPostingMatchesCounterparty(kind string, payload map[string]any, counterpartyID string, cache map[string]document.Record) bool {
	sourceDocumentID := strings.TrimSpace(textValue(payload["source_document_id"]))
	if sourceDocumentID == "" || counterpartyID == "" || s.documents == nil {
		return sourceDocumentID == "" && counterpartyID == ""
	}
	record, ok := cache[sourceDocumentID]
	if !ok {
		loaded, err := s.documents.Get(sourceDocumentID)
		if err != nil {
			return false
		}
		record = loaded
		cache[sourceDocumentID] = record
	}
	switch kind {
	case "ar":
		return textValue(record.Body.Payload["party_id"]) == counterpartyID
	case "ap":
		return textValue(record.Body.Payload["vendor_id"]) == counterpartyID
	default:
		return false
	}
}

func (s *FinanceReconciliationCoreService) accountNameFromIndex(index map[string]financeAccountMeta, accountCode string) string {
	if strings.TrimSpace(accountCode) == "" {
		return ""
	}
	meta := s.financeAccountMeta(index, accountCode)
	return firstNonEmptyString(meta.Name, accountCode)
}

func (s *FinanceReconciliationCoreService) accountMetaIndex() map[string]financeAccountMeta {
	if s.finance != nil {
		return s.finance.accountMetaIndex()
	}
	return map[string]financeAccountMeta{}
}

func (s *FinanceReconciliationCoreService) financeAccountMeta(index map[string]financeAccountMeta, accountCode string) financeAccountMeta {
	if s.finance != nil {
		return s.finance.financeAccountMeta(index, accountCode)
	}
	return financeAccountMeta{}
}

func financeAgingTotals() map[string]float64 {
	return map[string]float64{
		"open_amount":     0,
		"current":         0,
		"due_today":       0,
		"overdue_1_30":    0,
		"overdue_31_60":   0,
		"overdue_61_90":   0,
		"overdue_91_up":   0,
	}
}

func financeAgingBucket(dueDate, asOfDate string) string {
	if dueDate == "" {
		return "current"
	}
	if dueDate == asOfDate {
		return "due_today"
	}
	overdueDays := dateDiffDays(dueDate, asOfDate)
	if overdueDays <= 0 {
		return "current"
	}
	switch {
	case overdueDays <= 30:
		return "overdue_1_30"
	case overdueDays <= 60:
		return "overdue_31_60"
	case overdueDays <= 90:
		return "overdue_61_90"
	default:
		return "overdue_91_up"
	}
}

func normalizeAsOfDate(value string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return time.Now().UTC().Format("2006-01-02")
}
