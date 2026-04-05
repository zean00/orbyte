package application

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/shared"
)

const (
	financeCollectionsWriteoffThreshold = 1000.0
	defaultARWriteoffAccountCode        = "5900-BADDEBT"
	defaultAPWriteoffAccountCode        = "4800-AP-WRITEOFF"
)

type FinanceStatementLine struct {
	DocumentID     string  `json:"document_id"`
	DocumentNumber string  `json:"document_number"`
	DocumentType   string  `json:"document_type"`
	DocumentDate   string  `json:"document_date"`
	DueDate        string  `json:"due_date"`
	AccountCode    string  `json:"account_code"`
	TotalAmount    float64 `json:"total_amount"`
	SettledAmount  float64 `json:"settled_amount"`
	WriteoffAmount float64 `json:"writeoff_amount"`
	OpenAmount     float64 `json:"open_amount"`
	DaysOverdue    int     `json:"days_overdue"`
	AgingBucket    string  `json:"aging_bucket"`
	Status         string  `json:"status"`
}

type FinanceStatementReport struct {
	OrganizationID   string                 `json:"organization_id"`
	LocationID       string                 `json:"location_id,omitempty"`
	AsOfDate         string                 `json:"as_of_date"`
	Kind             string                 `json:"kind"`
	CounterpartyID   string                 `json:"counterparty_id"`
	CounterpartyName string                 `json:"counterparty_name"`
	Totals           map[string]float64     `json:"totals"`
	Rows             []FinanceStatementLine `json:"rows"`
}

type FinanceSettlementException struct {
	ID               string  `json:"id,omitempty"`
	SourceKey        string  `json:"source_key"`
	Kind             string  `json:"kind"`
	ExceptionType    string  `json:"exception_type"`
	AsOfDate         string  `json:"as_of_date"`
	CounterpartyID   string  `json:"counterparty_id"`
	CounterpartyName string  `json:"counterparty_name"`
	AccountCode      string  `json:"account_code,omitempty"`
	SourceDocumentID string  `json:"source_document_id,omitempty"`
	SourceDocumentNo string  `json:"source_document_number,omitempty"`
	SourcePaymentID  string  `json:"source_payment_id,omitempty"`
	SourcePaymentNo  string  `json:"source_payment_number,omitempty"`
	OpenAmount       float64 `json:"open_amount"`
	UnappliedAmount  float64 `json:"unapplied_amount"`
	Status           string  `json:"status"`
	CollectionCaseID string  `json:"collection_case_id,omitempty"`
	Note             string  `json:"note,omitempty"`
}

type FinanceSettlementExceptionReport struct {
	OrganizationID string                       `json:"organization_id"`
	LocationID     string                       `json:"location_id,omitempty"`
	AsOfDate       string                       `json:"as_of_date"`
	Kind           string                       `json:"kind,omitempty"`
	Totals         map[string]float64           `json:"totals"`
	Items          []FinanceSettlementException `json:"items"`
}

type FinanceCollectionCaseSummary struct {
	ID                string   `json:"id"`
	Kind              string   `json:"kind"`
	CounterpartyID    string   `json:"counterparty_id"`
	CounterpartyName  string   `json:"counterparty_name"`
	Status            string   `json:"status"`
	Priority          string   `json:"priority"`
	AssigneeUserID    string   `json:"assignee_user_id"`
	FollowUpDate      string   `json:"follow_up_date"`
	PromisedPayDate   string   `json:"promised_payment_date"`
	LastContactDate   string   `json:"last_contact_date"`
	TotalOpenAmount   float64  `json:"total_open_amount"`
	OverdueAmount     float64  `json:"overdue_amount"`
	OldestDueDate     string   `json:"oldest_due_date"`
	SourceDocumentIDs []string `json:"source_document_ids"`
	Note              string   `json:"note"`
}

type FinanceCollectionsCoreService struct {
	documents      *document.Service
	models         *model.Service
	reconciliation *FinanceReconciliationCoreService
	commercial     *CommercialCoreService
	procurement    *ProcurementCoreService
	finance        *FinanceReportingCoreService
}

func NewFinanceCollectionsCoreService(documents *document.Service, models *model.Service, reconciliation *FinanceReconciliationCoreService, commercial *CommercialCoreService, procurement *ProcurementCoreService, finance *FinanceReportingCoreService) *FinanceCollectionsCoreService {
	return &FinanceCollectionsCoreService{
		documents:      documents,
		models:         models,
		reconciliation: reconciliation,
		commercial:     commercial,
		procurement:    procurement,
		finance:        finance,
	}
}

func (s *FinanceCollectionsCoreService) ARStatement(organizationID, locationID, asOfDate, partyID string) FinanceStatementReport {
	return s.statementReport("ar", organizationID, locationID, asOfDate, partyID)
}

func (s *FinanceCollectionsCoreService) APStatement(organizationID, locationID, asOfDate, vendorID string) FinanceStatementReport {
	return s.statementReport("ap", organizationID, locationID, asOfDate, vendorID)
}

func (s *FinanceCollectionsCoreService) GenerateARStatementRun(organizationID, locationID, partyID, asOfDate, actorID string) (model.Record, error) {
	return s.generateStatementRun("ar", organizationID, locationID, partyID, asOfDate, actorID)
}

func (s *FinanceCollectionsCoreService) GenerateAPStatementRun(organizationID, locationID, vendorID, asOfDate, actorID string) (model.Record, error) {
	return s.generateStatementRun("ap", organizationID, locationID, vendorID, asOfDate, actorID)
}

func (s *FinanceCollectionsCoreService) SyncSettlementExceptions(organizationID, locationID, asOfDate, kind, actorID string) (FinanceSettlementExceptionReport, error) {
	report := s.settlementExceptions(organizationID, locationID, asOfDate, kind)
	if s.models == nil {
		return report, nil
	}
	existing, _, err := s.models.List("settlement_exception", model.Query{
		Page:     1,
		PageSize: 100,
		Filters: map[string]string{
			"organization_id": organizationID,
			"location_id":     locationID,
			"as_of_date":      report.AsOfDate,
		},
	})
	if err != nil {
		return FinanceSettlementExceptionReport{}, err
	}
	existingByKey := map[string]model.Record{}
	for _, record := range existing {
		existingByKey[textValue(record.Values["source_key"])] = record
	}
	seen := map[string]struct{}{}
	for _, item := range report.Items {
		seen[item.SourceKey] = struct{}{}
		values := map[string]any{
			"organization_id":        organizationID,
			"location_id":            locationID,
			"source_key":             item.SourceKey,
			"kind":                   item.Kind,
			"exception_type":         item.ExceptionType,
			"as_of_date":             item.AsOfDate,
			"counterparty_id":        item.CounterpartyID,
			"counterparty_name":      item.CounterpartyName,
			"account_code":           item.AccountCode,
			"source_document_id":     item.SourceDocumentID,
			"source_document_number": item.SourceDocumentNo,
			"source_payment_id":      item.SourcePaymentID,
			"source_payment_number":  item.SourcePaymentNo,
			"open_amount":            item.OpenAmount,
			"unapplied_amount":       item.UnappliedAmount,
			"status":                 "open",
			"collection_case_id":     item.CollectionCaseID,
			"note":                   item.Note,
		}
		if current, ok := existingByKey[item.SourceKey]; ok {
			record, err := s.models.Update("settlement_exception", current.ID, actorID, mergeModelValues(current.Values, values), current.Version)
			if err != nil {
				return FinanceSettlementExceptionReport{}, err
			}
			item.ID = record.ID
		} else {
			record, err := s.models.Create("settlement_exception", actorID, values)
			if err != nil {
				return FinanceSettlementExceptionReport{}, err
			}
			item.ID = record.ID
		}
	}
	for _, current := range existing {
		if kind != "" && textValue(current.Values["kind"]) != kind {
			continue
		}
		key := textValue(current.Values["source_key"])
		if _, ok := seen[key]; ok {
			continue
		}
		values := mergeModelValues(current.Values, map[string]any{
			"status": "closed",
		})
		if _, err := s.models.Update("settlement_exception", current.ID, actorID, values, current.Version); err != nil {
			return FinanceSettlementExceptionReport{}, err
		}
	}
	refreshed, _, err := s.models.List("settlement_exception", model.Query{
		Page:     1,
		PageSize: 100,
		Filters: map[string]string{
			"organization_id": organizationID,
			"location_id":     locationID,
			"as_of_date":      report.AsOfDate,
			"status":          "open",
		},
	})
	if err != nil {
		return FinanceSettlementExceptionReport{}, err
	}
	report.Items = make([]FinanceSettlementException, 0, len(refreshed))
	report.Totals = map[string]float64{}
	for _, record := range refreshed {
		if kind != "" && textValue(record.Values["kind"]) != kind {
			continue
		}
		item := settlementExceptionFromRecord(record)
		report.Items = append(report.Items, item)
		report.Totals[item.ExceptionType] = roundMoney(report.Totals[item.ExceptionType] + maxFloat(item.OpenAmount, item.UnappliedAmount))
	}
	sort.Slice(report.Items, func(i, j int) bool {
		if report.Items[i].CounterpartyName != report.Items[j].CounterpartyName {
			return report.Items[i].CounterpartyName < report.Items[j].CounterpartyName
		}
		if report.Items[i].ExceptionType != report.Items[j].ExceptionType {
			return report.Items[i].ExceptionType < report.Items[j].ExceptionType
		}
		return report.Items[i].SourceKey < report.Items[j].SourceKey
	})
	return report, nil
}

func (s *FinanceCollectionsCoreService) SettlementExceptions(organizationID, locationID, asOfDate, kind string) FinanceSettlementExceptionReport {
	return s.settlementExceptions(organizationID, locationID, asOfDate, kind)
}

func (s *FinanceCollectionsCoreService) SettlementExceptionRecords(organizationID, locationID, asOfDate, kind string) (FinanceSettlementExceptionReport, error) {
	if s.models == nil {
		return s.settlementExceptions(organizationID, locationID, asOfDate, kind), nil
	}
	asOfDate = normalizeAsOfDate(asOfDate)
	filters := map[string]string{
		"organization_id": organizationID,
		"location_id":     locationID,
		"as_of_date":      asOfDate,
		"status":          "open",
	}
	if kind != "" {
		filters["kind"] = kind
	}
	items, _, err := s.models.List("settlement_exception", model.Query{Page: 1, PageSize: 100, Filters: filters})
	if err != nil {
		return FinanceSettlementExceptionReport{}, err
	}
	report := FinanceSettlementExceptionReport{
		OrganizationID: organizationID,
		LocationID:     locationID,
		AsOfDate:       asOfDate,
		Kind:           kind,
		Totals:         map[string]float64{},
		Items:          make([]FinanceSettlementException, 0, len(items)),
	}
	for _, item := range items {
		exception := settlementExceptionFromRecord(item)
		report.Items = append(report.Items, exception)
		report.Totals[exception.ExceptionType] = roundMoney(report.Totals[exception.ExceptionType] + maxFloat(exception.OpenAmount, exception.UnappliedAmount))
	}
	return report, nil
}

func (s *FinanceCollectionsCoreService) OpenCollectionCaseFromException(exceptionID, actorID, organizationID, locationID string) (model.Record, error) {
	if s.models == nil {
		return model.Record{}, shared.NotFound("collection cases are not available")
	}
	exception, err := s.models.Get("settlement_exception", exceptionID)
	if err != nil {
		return model.Record{}, err
	}
	if textValue(exception.Values["organization_id"]) != organizationID || textValue(exception.Values["location_id"]) != locationID {
		return model.Record{}, shared.Forbidden("settlement exception is outside the active scope")
	}
	docIDs := []string{}
	if id := textValue(exception.Values["source_document_id"]); id != "" {
		docIDs = append(docIDs, id)
	}
	sourceNumbers := []string{}
	if number := textValue(exception.Values["source_document_number"]); number != "" {
		sourceNumbers = append(sourceNumbers, number)
	}
	caseValues := map[string]any{
		"organization_id":          organizationID,
		"location_id":              locationID,
		"kind":                     textValue(exception.Values["kind"]),
		"counterparty_id":          textValue(exception.Values["counterparty_id"]),
		"counterparty_name":        textValue(exception.Values["counterparty_name"]),
		"source_document_ids":      docIDs,
		"source_document_numbers":  sourceNumbers,
		"settlement_exception_ids": []string{exception.ID},
		"status":                   "open",
		"priority":                 "normal",
		"note":                     textValue(exception.Values["note"]),
	}
	enriched := s.enrichCollectionCaseValues(caseValues, organizationID, locationID)
	record, err := s.models.Create("collection_case", actorID, enriched)
	if err != nil {
		return model.Record{}, err
	}
	updatedException := mergeModelValues(exception.Values, map[string]any{
		"collection_case_id": record.ID,
		"status":             "in_case",
	})
	if _, err := s.models.Update("settlement_exception", exception.ID, actorID, updatedException, exception.Version); err != nil {
		return model.Record{}, err
	}
	return record, nil
}

func (s *FinanceCollectionsCoreService) RefreshCollectionCase(caseID, actorID, organizationID, locationID string) (model.Record, error) {
	if s.models == nil {
		return model.Record{}, shared.NotFound("collection cases are not available")
	}
	record, err := s.models.Get("collection_case", caseID)
	if err != nil {
		return model.Record{}, err
	}
	if textValue(record.Values["organization_id"]) != organizationID || textValue(record.Values["location_id"]) != locationID {
		return model.Record{}, shared.Forbidden("collection case is outside the active scope")
	}
	values := s.enrichCollectionCaseValues(record.Values, organizationID, locationID)
	return s.models.Update("collection_case", record.ID, actorID, values, record.Version)
}

func (s *FinanceCollectionsCoreService) CollectionCases(organizationID, locationID, kind, status string) ([]FinanceCollectionCaseSummary, error) {
	if s.models == nil {
		return nil, nil
	}
	filters := map[string]string{
		"organization_id": organizationID,
		"location_id":     locationID,
	}
	if kind != "" {
		filters["kind"] = kind
	}
	if status != "" {
		filters["status"] = status
	}
	items, _, err := s.models.List("collection_case", model.Query{Page: 1, PageSize: 100, Filters: filters})
	if err != nil {
		return nil, err
	}
	result := make([]FinanceCollectionCaseSummary, 0, len(items))
	for _, item := range items {
		values := s.enrichCollectionCaseValues(item.Values, organizationID, locationID)
		result = append(result, FinanceCollectionCaseSummary{
			ID:                item.ID,
			Kind:              textValue(values["kind"]),
			CounterpartyID:    textValue(values["counterparty_id"]),
			CounterpartyName:  textValue(values["counterparty_name"]),
			Status:            textValue(values["status"]),
			Priority:          textValue(values["priority"]),
			AssigneeUserID:    textValue(values["assignee_user_id"]),
			FollowUpDate:      textValue(values["follow_up_date"]),
			PromisedPayDate:   textValue(values["promised_payment_date"]),
			LastContactDate:   textValue(values["last_contact_date"]),
			TotalOpenAmount:   roundMoney(numberValue(values["total_open_amount"])),
			OverdueAmount:     roundMoney(numberValue(values["overdue_amount"])),
			OldestDueDate:     textValue(values["oldest_due_date"]),
			SourceDocumentIDs: stringListValue(values["source_document_ids"]),
			Note:              textValue(values["note"]),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CounterpartyName != result[j].CounterpartyName {
			return result[i].CounterpartyName < result[j].CounterpartyName
		}
		return result[i].ID < result[j].ID
	})
	return result, nil
}

func (s *FinanceCollectionsCoreService) ApplySettlementException(exceptionID, targetDocumentID string, amount float64, actorID, organizationID, locationID string) (model.Record, error) {
	if s.models == nil {
		return model.Record{}, shared.NotFound("settlement exceptions are not available")
	}
	exception, err := s.models.Get("settlement_exception", exceptionID)
	if err != nil {
		return model.Record{}, err
	}
	if textValue(exception.Values["organization_id"]) != organizationID || textValue(exception.Values["location_id"]) != locationID {
		return model.Record{}, shared.Forbidden("settlement exception is outside the active scope")
	}
	kind := textValue(exception.Values["kind"])
	if amount <= 0 {
		amount = roundMoney(numberValue(exception.Values["unapplied_amount"]))
	}
	switch kind {
	case "ar":
		if err := s.commercial.AllocatePaymentReceipt(textValue(exception.Values["source_payment_id"]), targetDocumentID, amount, actorID); err != nil {
			return model.Record{}, err
		}
	case "ap":
		if err := s.procurement.AllocatePaymentOut(textValue(exception.Values["source_payment_id"]), targetDocumentID, amount, actorID); err != nil {
			return model.Record{}, err
		}
	default:
		return model.Record{}, shared.Validation("unsupported settlement exception kind")
	}
	values := mergeModelValues(exception.Values, map[string]any{
		"status": "applied",
		"note":   fmt.Sprintf("Applied %.2f to %s", amount, targetDocumentID),
	})
	return s.models.Update("settlement_exception", exception.ID, actorID, values, exception.Version)
}

func (s *FinanceCollectionsCoreService) WriteOffSettlementException(exceptionID, postingDate string, amount float64, actorID, organizationID, locationID string) (document.Record, error) {
	if s.models == nil {
		return document.Record{}, shared.NotFound("settlement exceptions are not available")
	}
	exception, err := s.models.Get("settlement_exception", exceptionID)
	if err != nil {
		return document.Record{}, err
	}
	if textValue(exception.Values["organization_id"]) != organizationID || textValue(exception.Values["location_id"]) != locationID {
		return document.Record{}, shared.Forbidden("settlement exception is outside the active scope")
	}
	if amount <= 0 {
		amount = roundMoney(numberValue(exception.Values["open_amount"]))
	}
	var posting document.Record
	switch textValue(exception.Values["kind"]) {
	case "ar":
		posting, err = s.writeOffInvoice(textValue(exception.Values["source_document_id"]), postingDate, amount, actorID)
	case "ap":
		posting, err = s.writeOffVendorBill(textValue(exception.Values["source_document_id"]), postingDate, amount, actorID)
	default:
		err = shared.Validation("unsupported write-off kind")
	}
	if err != nil {
		return document.Record{}, err
	}
	values := mergeModelValues(exception.Values, map[string]any{
		"status": "written_off",
		"note":   fmt.Sprintf("Written off %.2f on %s", amount, textValue(posting.Body.Payload["posting_date"])),
	})
	if _, err := s.models.Update("settlement_exception", exception.ID, actorID, values, exception.Version); err != nil {
		return document.Record{}, err
	}
	return posting, nil
}

func (s *FinanceCollectionsCoreService) statementReport(kind, organizationID, locationID, asOfDate, counterpartyID string) FinanceStatementReport {
	asOfDate = normalizeAsOfDate(asOfDate)
	rows := []FinanceStatementLine{}
	counterpartyName := ""
	var aging FinanceAgingReport
	switch kind {
	case "ar":
		if s.reconciliation != nil {
			aging = s.reconciliation.ARAging(organizationID, locationID, asOfDate, counterpartyID, "")
		}
	case "ap":
		if s.reconciliation != nil {
			aging = s.reconciliation.APAging(organizationID, locationID, asOfDate, counterpartyID, "")
		}
	}
	totals := map[string]float64{
		"open_amount":    0,
		"overdue_amount": 0,
		"current_amount": 0,
		"due_today":      0,
	}
	for _, item := range aging.Items {
		if counterpartyName == "" {
			counterpartyName = item.CounterpartyName
		}
		writeoffAmount := s.documentWriteoffAmount(kind, item.DocumentID)
		rows = append(rows, FinanceStatementLine{
			DocumentID:     item.DocumentID,
			DocumentNumber: item.DocumentNumber,
			DocumentType:   item.DocumentType,
			DocumentDate:   item.DocumentDate,
			DueDate:        item.DueDate,
			AccountCode:    item.AccountCode,
			TotalAmount:    item.TotalAmount,
			SettledAmount:  item.SettledAmount,
			WriteoffAmount: writeoffAmount,
			OpenAmount:     item.OpenAmount,
			DaysOverdue:    item.DaysOverdue,
			AgingBucket:    item.AgingBucket,
			Status:         item.Status,
		})
		totals["open_amount"] = roundMoney(totals["open_amount"] + item.OpenAmount)
		if strings.HasPrefix(item.AgingBucket, "overdue_") {
			totals["overdue_amount"] = roundMoney(totals["overdue_amount"] + item.OpenAmount)
		}
		if item.AgingBucket == "current" {
			totals["current_amount"] = roundMoney(totals["current_amount"] + item.OpenAmount)
		}
		if item.AgingBucket == "due_today" {
			totals["due_today"] = roundMoney(totals["due_today"] + item.OpenAmount)
		}
		totals[item.AgingBucket] = roundMoney(totals[item.AgingBucket] + item.OpenAmount)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].DueDate != rows[j].DueDate {
			return rows[i].DueDate < rows[j].DueDate
		}
		return rows[i].DocumentNumber < rows[j].DocumentNumber
	})
	return FinanceStatementReport{
		OrganizationID:   organizationID,
		LocationID:       locationID,
		AsOfDate:         asOfDate,
		Kind:             kind,
		CounterpartyID:   counterpartyID,
		CounterpartyName: counterpartyName,
		Totals:           totals,
		Rows:             rows,
	}
}

func (s *FinanceCollectionsCoreService) statementReportForCounterparty(kind, organizationID, locationID, asOfDate, requestedCounterpartyID, canonicalCounterpartyID string) FinanceStatementReport {
	candidates := []string{}
	seen := map[string]struct{}{}
	addCandidate := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		candidates = append(candidates, value)
	}
	addCandidate(requestedCounterpartyID)
	addCandidate(canonicalCounterpartyID)
	if kind == "ap" {
		if record, ok := resolveExistingModelRecord(s.models, "vendor_profile", requestedCounterpartyID); ok {
			addCandidate(record.ID)
			addCandidate(textValue(record.Values["code"]))
		}
	} else {
		if record, ok := resolveExistingModelRecord(s.models, "party", requestedCounterpartyID); ok {
			addCandidate(record.ID)
			addCandidate(textValue(record.Values["code"]))
		}
	}
	if len(candidates) == 0 {
		return s.statementReport(kind, organizationID, locationID, asOfDate, "")
	}
	for _, candidate := range candidates {
		report := s.statementReport(kind, organizationID, locationID, asOfDate, candidate)
		if len(report.Rows) > 0 {
			report.CounterpartyID = canonicalCounterpartyID
			return report
		}
	}
	report := s.statementReport(kind, organizationID, locationID, asOfDate, candidates[0])
	report.CounterpartyID = canonicalCounterpartyID
	return report
}

func (s *FinanceCollectionsCoreService) generateStatementRun(kind, organizationID, locationID, counterpartyID, asOfDate, actorID string) (model.Record, error) {
	if s.models == nil {
		return model.Record{}, shared.NotFound("statement runs are not available")
	}
	if strings.TrimSpace(counterpartyID) == "" {
		return model.Record{}, shared.Validation("counterparty is required")
	}
	canonicalCounterpartyID := strings.TrimSpace(counterpartyID)
	if kind == "ap" {
		if err := validateVendorID(s.models, canonicalCounterpartyID); err != nil {
			return model.Record{}, err
		}
		if record, ok := resolveExistingModelRecord(s.models, "vendor_profile", canonicalCounterpartyID); ok {
			canonicalCounterpartyID = record.ID
		}
	} else {
		if err := validatePartyID(s.models, canonicalCounterpartyID); err != nil {
			return model.Record{}, err
		}
		if record, ok := resolveExistingModelRecord(s.models, "party", canonicalCounterpartyID); ok {
			canonicalCounterpartyID = record.ID
		}
	}
	report := s.statementReportForCounterparty(kind, organizationID, locationID, asOfDate, counterpartyID, canonicalCounterpartyID)
	if len(report.Rows) == 0 {
		return model.Record{}, shared.Validation("statement has no open items")
	}
	modelKey := "party_statement_run"
	counterpartyField := "party_id"
	if kind == "ap" {
		modelKey = "vendor_statement_run"
		counterpartyField = "vendor_id"
	}
	return s.models.Create(modelKey, actorID, map[string]any{
		"organization_id":      organizationID,
		"location_id":          locationID,
		counterpartyField:      canonicalCounterpartyID,
		"counterparty_name":    report.CounterpartyName,
		"as_of_date":           report.AsOfDate,
		"status":               "issued",
		"open_amount_total":    report.Totals["open_amount"],
		"overdue_amount_total": report.Totals["overdue_amount"],
		"statement_snapshot":   report,
	})
}

func (s *FinanceCollectionsCoreService) settlementExceptions(organizationID, locationID, asOfDate, kind string) FinanceSettlementExceptionReport {
	asOfDate = normalizeAsOfDate(asOfDate)
	report := FinanceSettlementExceptionReport{
		OrganizationID: organizationID,
		LocationID:     locationID,
		AsOfDate:       asOfDate,
		Kind:           kind,
		Totals:         map[string]float64{},
		Items:          []FinanceSettlementException{},
	}
	if s.documents == nil {
		return report
	}
	appendItem := func(item FinanceSettlementException) {
		if kind != "" && item.Kind != kind {
			return
		}
		report.Items = append(report.Items, item)
		report.Totals[item.ExceptionType] = roundMoney(report.Totals[item.ExceptionType] + maxFloat(item.OpenAmount, item.UnappliedAmount))
	}
	for _, item := range s.statementReport("ar", organizationID, locationID, asOfDate, "").Rows {
		if item.OpenAmount <= 0 {
			continue
		}
		exType := "short_payment"
		if item.OpenAmount <= financeCollectionsWriteoffThreshold {
			exType = "write_off_candidate"
		}
		if item.SettledAmount <= 0 && exType == "short_payment" {
			continue
		}
		record, err := s.documents.Get(item.DocumentID)
		if err != nil {
			continue
		}
		appendItem(FinanceSettlementException{
			SourceKey:        fmt.Sprintf("ar|%s|%s|%s", exType, item.DocumentID, asOfDate),
			Kind:             "ar",
			ExceptionType:    exType,
			AsOfDate:         asOfDate,
			CounterpartyID:   textValue(record.Body.Payload["party_id"]),
			CounterpartyName: textValue(record.Body.Payload["party_name"]),
			AccountCode:      item.AccountCode,
			SourceDocumentID: item.DocumentID,
			SourceDocumentNo: item.DocumentNumber,
			OpenAmount:       item.OpenAmount,
			Status:           "open",
		})
	}
	for _, item := range s.statementReport("ap", organizationID, locationID, asOfDate, "").Rows {
		if item.OpenAmount <= 0 {
			continue
		}
		exType := "short_payment"
		if item.OpenAmount <= financeCollectionsWriteoffThreshold {
			exType = "write_off_candidate"
		}
		if item.SettledAmount <= 0 && exType == "short_payment" {
			continue
		}
		record, err := s.documents.Get(item.DocumentID)
		if err != nil {
			continue
		}
		appendItem(FinanceSettlementException{
			SourceKey:        fmt.Sprintf("ap|%s|%s|%s", exType, item.DocumentID, asOfDate),
			Kind:             "ap",
			ExceptionType:    exType,
			AsOfDate:         asOfDate,
			CounterpartyID:   textValue(record.Body.Payload["vendor_id"]),
			CounterpartyName: textValue(record.Body.Payload["vendor_name"]),
			AccountCode:      item.AccountCode,
			SourceDocumentID: item.DocumentID,
			SourceDocumentNo: item.DocumentNumber,
			OpenAmount:       item.OpenAmount,
			Status:           "open",
		})
	}
	for _, record := range s.documents.List() {
		if !matchesCommercialScope(record, organizationID, locationID) {
			continue
		}
		payload := clonedPayload(record.Body.Payload)
		switch record.Header.Type {
		case "payment_receipt":
			if textValue(payload["receipt_date"]) == "" || textValue(payload["receipt_date"]) > asOfDate {
				continue
			}
			unapplied := roundMoney(numberValue(payload["unapplied_amount"]))
			if unapplied <= 0 {
				continue
			}
			exType := "unapplied_cash"
			if len(recordList(payload["allocations"])) > 0 {
				exType = "overpayment"
			}
			appendItem(FinanceSettlementException{
				SourceKey:        fmt.Sprintf("ar|%s|%s|%s", exType, record.Header.ID, asOfDate),
				Kind:             "ar",
				ExceptionType:    exType,
				AsOfDate:         asOfDate,
				CounterpartyID:   textValue(payload["party_id"]),
				CounterpartyName: textValue(payload["party_name"]),
				AccountCode:      textValue(payload["receivable_account_code"]),
				SourcePaymentID:  record.Header.ID,
				SourcePaymentNo:  firstNonEmptyString(record.Header.Number, record.Header.ID),
				UnappliedAmount:  unapplied,
				Status:           "open",
			})
		case "payment_out":
			if textValue(payload["payment_date"]) == "" || textValue(payload["payment_date"]) > asOfDate {
				continue
			}
			unapplied := roundMoney(numberValue(payload["unapplied_amount"]))
			if unapplied <= 0 {
				continue
			}
			exType := "unapplied_cash"
			if len(recordList(payload["allocations"])) > 0 {
				exType = "overpayment"
			}
			appendItem(FinanceSettlementException{
				SourceKey:        fmt.Sprintf("ap|%s|%s|%s", exType, record.Header.ID, asOfDate),
				Kind:             "ap",
				ExceptionType:    exType,
				AsOfDate:         asOfDate,
				CounterpartyID:   textValue(payload["vendor_id"]),
				CounterpartyName: textValue(payload["vendor_name"]),
				AccountCode:      textValue(payload["payable_account_code"]),
				SourcePaymentID:  record.Header.ID,
				SourcePaymentNo:  firstNonEmptyString(record.Header.Number, record.Header.ID),
				UnappliedAmount:  unapplied,
				Status:           "open",
			})
		}
	}
	sort.Slice(report.Items, func(i, j int) bool {
		if report.Items[i].CounterpartyName != report.Items[j].CounterpartyName {
			return report.Items[i].CounterpartyName < report.Items[j].CounterpartyName
		}
		if report.Items[i].ExceptionType != report.Items[j].ExceptionType {
			return report.Items[i].ExceptionType < report.Items[j].ExceptionType
		}
		return report.Items[i].SourceKey < report.Items[j].SourceKey
	})
	return report
}

func (s *FinanceCollectionsCoreService) documentWriteoffAmount(kind, documentID string) float64 {
	if s.documents == nil || strings.TrimSpace(documentID) == "" {
		return 0
	}
	record, err := s.documents.Get(documentID)
	if err != nil {
		return 0
	}
	switch kind {
	case "ar":
		return roundMoney(numberValue(record.Body.Payload["writeoff_amount"]))
	case "ap":
		return roundMoney(numberValue(record.Body.Payload["writeoff_amount"]))
	default:
		return 0
	}
}

func (s *FinanceCollectionsCoreService) enrichCollectionCaseValues(values map[string]any, organizationID, locationID string) map[string]any {
	next := mergeModelValues(nil, values)
	totalOpen := 0.0
	overdue := 0.0
	oldestDue := ""
	today := time.Now().UTC().Format("2006-01-02")
	for _, documentID := range stringListValue(next["source_document_ids"]) {
		record, err := s.documents.Get(documentID)
		if err != nil || !matchesCommercialScope(record, organizationID, locationID) {
			continue
		}
		payload := clonedPayload(record.Body.Payload)
		openAmount := roundMoney(numberValue(payload["balance_due_amount"]))
		if openAmount <= 0 {
			continue
		}
		totalOpen = roundMoney(totalOpen + openAmount)
		dueDate := firstNonEmptyString(textValue(payload["due_date"]), textValue(payload["invoice_date"]), textValue(payload["bill_date"]))
		if dueDate != "" && (oldestDue == "" || dueDate < oldestDue) {
			oldestDue = dueDate
		}
		if dueDate != "" && dateDiffDays(dueDate, today) > 0 {
			overdue = roundMoney(overdue + openAmount)
		}
	}
	next["total_open_amount"] = totalOpen
	next["overdue_amount"] = overdue
	next["oldest_due_date"] = oldestDue
	return next
}

func (s *FinanceCollectionsCoreService) writeOffInvoice(invoiceID, postingDate string, amount float64, actorID string) (document.Record, error) {
	if s.documents == nil {
		return document.Record{}, shared.NotFound("documents service is not available")
	}
	invoice, err := s.documents.Get(invoiceID)
	if err != nil {
		return document.Record{}, err
	}
	if invoice.Header.Type != "invoice" {
		return document.Record{}, shared.Validation("write-off source must be an invoice")
	}
	payload := clonedPayload(invoice.Body.Payload)
	openAmount := roundMoney(numberValue(payload["balance_due_amount"]))
	if amount <= 0 {
		amount = openAmount
	}
	if amount <= 0 || amount-openAmount > 0.0001 {
		return document.Record{}, shared.Validation("write-off exceeds invoice balance")
	}
	postingDate = firstNonEmptyString(strings.TrimSpace(postingDate), time.Now().UTC().Format("2006-01-02"))
	if s.finance != nil {
		if err := s.finance.ValidatePostingDateOpen(invoice.Header.OrganizationID, invoice.Header.LocationID, postingDate); err != nil {
			return document.Record{}, err
		}
	}
	writeoffAmount := roundMoney(numberValue(payload["writeoff_amount"]) + amount)
	paidAmount := roundMoney(numberValue(payload["paid_amount"]))
	creditedAmount := roundMoney(numberValue(payload["credited_amount"]))
	refundedAmount := roundMoney(numberValue(payload["refunded_amount"]))
	payload["writeoff_amount"] = writeoffAmount
	payload["balance_due_amount"] = roundMoney(maxFloat(numberValue(payload["total_amount"])-paidAmount-creditedAmount-writeoffAmount, 0))
	switch {
	case numberValue(payload["balance_due_amount"]) == 0 && writeoffAmount > 0:
		invoice.Header.Status = "written_off"
	case paidAmount > 0 || creditedAmount > 0 || refundedAmount > 0 || writeoffAmount > 0:
		invoice.Header.Status = "partially_paid"
	default:
		invoice.Header.Status = "issued"
	}
	if err := s.saveMutatedDocument(invoice, actorID, payload); err != nil {
		return document.Record{}, err
	}
	postingPayload := map[string]any{
		"source_document_type": "invoice",
		"source_document_id":   invoice.Header.ID,
		"posting_date":         postingDate,
		"currency_code":        firstNonEmptyString(textValue(payload["currency_code"]), invoice.Header.TotalAmount.Currency, "IDR"),
		"posting_rule_key":     "invoice_writeoff_default",
		"total_amount":         amount,
		"journal_lines": []map[string]any{
			{"account_code": defaultARWriteoffAccountCode, "account_name": "Bad Debt Expense", "description": fmt.Sprintf("Write-off %s", firstNonEmptyString(invoice.Header.Number, invoice.Header.ID)), "debit": amount, "credit": 0.0},
			{"account_code": firstNonEmptyString(textValue(payload["receivable_account_code"]), "1100-AR"), "account_name": "Accounts Receivable", "description": fmt.Sprintf("Write-off %s", firstNonEmptyString(invoice.Header.Number, invoice.Header.ID)), "debit": 0.0, "credit": amount},
		},
		"notes": fmt.Sprintf("Write-off for invoice %s", firstNonEmptyString(invoice.Header.Number, invoice.Header.ID)),
	}
	posting, err := s.documents.Create("ledger_posting", invoice.Header.OrganizationID, invoice.Header.LocationID, actorID, postingPayload)
	if err != nil {
		return document.Record{}, err
	}
	if err := s.finalizeCollectionsPosting(posting, actorID); err != nil {
		return document.Record{}, err
	}
	posting.Header.Status = "posted"
	if _, err := s.documents.AddLink(posting.Header.ID, invoice.Header.ID, "posting_for", map[string]any{"posting_reason": "invoice_writeoff"}); err != nil && !isConflict(err) {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(invoice.Header.ID, posting.Header.ID, "posting_for", map[string]any{"posting_reason": "invoice_writeoff"}); err != nil && !isConflict(err) {
		return document.Record{}, err
	}
	return posting, nil
}

func (s *FinanceCollectionsCoreService) writeOffVendorBill(billID, postingDate string, amount float64, actorID string) (document.Record, error) {
	if s.documents == nil {
		return document.Record{}, shared.NotFound("documents service is not available")
	}
	bill, err := s.documents.Get(billID)
	if err != nil {
		return document.Record{}, err
	}
	if bill.Header.Type != "vendor_bill" {
		return document.Record{}, shared.Validation("write-off source must be a vendor bill")
	}
	payload := clonedPayload(bill.Body.Payload)
	openAmount := roundMoney(numberValue(payload["balance_due_amount"]))
	if amount <= 0 {
		amount = openAmount
	}
	if amount <= 0 || amount-openAmount > 0.0001 {
		return document.Record{}, shared.Validation("write-off exceeds vendor bill balance")
	}
	postingDate = firstNonEmptyString(strings.TrimSpace(postingDate), time.Now().UTC().Format("2006-01-02"))
	if s.finance != nil {
		if err := s.finance.ValidatePostingDateOpen(bill.Header.OrganizationID, bill.Header.LocationID, postingDate); err != nil {
			return document.Record{}, err
		}
	}
	writeoffAmount := roundMoney(numberValue(payload["writeoff_amount"]) + amount)
	paidAmount := roundMoney(numberValue(payload["paid_amount"]))
	creditedAmount := roundMoney(numberValue(payload["credited_amount"]))
	payload["writeoff_amount"] = writeoffAmount
	payload["balance_due_amount"] = roundMoney(maxFloat(numberValue(payload["total_amount"])-paidAmount-creditedAmount-writeoffAmount, 0))
	switch {
	case numberValue(payload["balance_due_amount"]) == 0 && writeoffAmount > 0:
		bill.Header.Status = "written_off"
	case paidAmount > 0 || creditedAmount > 0 || writeoffAmount > 0:
		bill.Header.Status = "partially_paid"
	default:
		bill.Header.Status = "issued"
	}
	if err := s.saveMutatedDocument(bill, actorID, payload); err != nil {
		return document.Record{}, err
	}
	postingPayload := map[string]any{
		"source_document_type": "vendor_bill",
		"source_document_id":   bill.Header.ID,
		"posting_date":         postingDate,
		"currency_code":        firstNonEmptyString(textValue(payload["currency_code"]), bill.Header.TotalAmount.Currency, "IDR"),
		"posting_rule_key":     "vendor_bill_writeoff_default",
		"total_amount":         amount,
		"journal_lines": []map[string]any{
			{"account_code": firstNonEmptyString(textValue(payload["payable_account_code"]), "2000-AP"), "account_name": "Accounts Payable", "description": fmt.Sprintf("Write-off %s", firstNonEmptyString(bill.Header.Number, bill.Header.ID)), "debit": amount, "credit": 0.0},
			{"account_code": defaultAPWriteoffAccountCode, "account_name": "AP Write-off Gain", "description": fmt.Sprintf("Write-off %s", firstNonEmptyString(bill.Header.Number, bill.Header.ID)), "debit": 0.0, "credit": amount},
		},
		"notes": fmt.Sprintf("Write-off for vendor bill %s", firstNonEmptyString(bill.Header.Number, bill.Header.ID)),
	}
	posting, err := s.documents.Create("ledger_posting", bill.Header.OrganizationID, bill.Header.LocationID, actorID, postingPayload)
	if err != nil {
		return document.Record{}, err
	}
	if err := s.finalizeCollectionsPosting(posting, actorID); err != nil {
		return document.Record{}, err
	}
	posting.Header.Status = "posted"
	if _, err := s.documents.AddLink(posting.Header.ID, bill.Header.ID, "posting_for", map[string]any{"posting_reason": "vendor_bill_writeoff"}); err != nil && !isConflict(err) {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(bill.Header.ID, posting.Header.ID, "posting_for", map[string]any{"posting_reason": "vendor_bill_writeoff"}); err != nil && !isConflict(err) {
		return document.Record{}, err
	}
	return posting, nil
}

func (s *FinanceCollectionsCoreService) finalizeCollectionsPosting(record document.Record, actorID string) error {
	record.Header.Status = "posted"
	record.Header.TotalAmount = shared.Money{
		Currency:    firstNonEmptyString(textValue(record.Body.Payload["currency_code"]), record.Header.TotalAmount.Currency, "IDR"),
		AmountMinor: moneyMinor(derivedRecordAmount(record.Body.Payload)),
	}
	record.Header.Version++
	record.Header.ETag = fmt.Sprintf("%s:%d", record.Header.ID, record.Header.Version)
	record.Header.UpdatedBy = actorID
	record.Header.UpdatedAt = time.Now().UTC()
	return s.documents.Save(record)
}

func (s *FinanceCollectionsCoreService) saveMutatedDocument(record document.Record, actorID string, payload map[string]any) error {
	record.Body.Payload = clonedPayload(payload)
	record.Header.TotalAmount = shared.Money{
		Currency:    firstNonEmptyString(textValue(payload["currency_code"]), record.Header.TotalAmount.Currency, "IDR"),
		AmountMinor: moneyMinor(derivedRecordAmount(payload)),
	}
	record.Header.Version++
	record.Header.UpdatedBy = actorID
	record.Header.UpdatedAt = time.Now().UTC()
	record.Header.ETag = fmt.Sprintf("%s:%d", record.Header.ID, record.Header.Version)
	return s.documents.Save(record)
}

func mergeModelValues(base map[string]any, updates map[string]any) map[string]any {
	next := map[string]any{}
	for key, value := range base {
		next[key] = value
	}
	for key, value := range updates {
		next[key] = value
	}
	return next
}

func settlementExceptionFromRecord(record model.Record) FinanceSettlementException {
	return FinanceSettlementException{
		ID:               record.ID,
		SourceKey:        textValue(record.Values["source_key"]),
		Kind:             textValue(record.Values["kind"]),
		ExceptionType:    textValue(record.Values["exception_type"]),
		AsOfDate:         textValue(record.Values["as_of_date"]),
		CounterpartyID:   textValue(record.Values["counterparty_id"]),
		CounterpartyName: textValue(record.Values["counterparty_name"]),
		AccountCode:      textValue(record.Values["account_code"]),
		SourceDocumentID: textValue(record.Values["source_document_id"]),
		SourceDocumentNo: textValue(record.Values["source_document_number"]),
		SourcePaymentID:  textValue(record.Values["source_payment_id"]),
		SourcePaymentNo:  textValue(record.Values["source_payment_number"]),
		OpenAmount:       roundMoney(numberValue(record.Values["open_amount"])),
		UnappliedAmount:  roundMoney(numberValue(record.Values["unapplied_amount"])),
		Status:           textValue(record.Values["status"]),
		CollectionCaseID: textValue(record.Values["collection_case_id"]),
		Note:             textValue(record.Values["note"]),
	}
}

func stringListValue(value any) []string {
	result := []string{}
	if values, ok := value.([]string); ok {
		for _, item := range values {
			if strings.TrimSpace(item) != "" {
				result = append(result, strings.TrimSpace(item))
			}
		}
		return result
	}
	if values, ok := value.([]any); ok {
		for _, item := range values {
			if text := strings.TrimSpace(textValue(item)); text != "" {
				result = append(result, text)
			}
		}
		return result
	}
	rows := recordList(value)
	for _, row := range rows {
		if text := strings.TrimSpace(textValue(row)); text != "" {
			result = append(result, text)
		}
	}
	return result
}
