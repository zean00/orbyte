package application

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"orbyte/internal/platform/config"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/shared"
)

const (
	defaultCashOverAccountCode      = "4890-CASH-OVER"
	defaultCashShortAccountCode     = "5890-CASH-SHORT"
	defaultGiftCardLiabilityAccount = "2250-GIFT-CARD"
	defaultStoreCreditLiabilityCode = "2260-STORE-CREDIT"
)

type RetailFinanceShiftReconciliationRow struct {
	ReconciliationID    string  `json:"reconciliation_id"`
	ShiftID             string  `json:"shift_id"`
	ShiftNumber         string  `json:"shift_number"`
	StoreCode           string  `json:"store_code"`
	RegisterCode        string  `json:"register_code"`
	CashierUserID       string  `json:"cashier_user_id"`
	ReconciliationDate  string  `json:"reconciliation_date"`
	ExpectedCashAmount  float64 `json:"expected_cash_amount"`
	ActualCashAmount    float64 `json:"actual_cash_amount"`
	OverShortAmount     float64 `json:"over_short_amount"`
	ExpectedTotalAmount float64 `json:"expected_total_amount"`
	CountedTotalAmount  float64 `json:"counted_total_amount"`
	Status              string  `json:"status"`
	PostingID           string  `json:"posting_id,omitempty"`
	ApprovedBy          string  `json:"approved_by,omitempty"`
	ApprovedAt          string  `json:"approved_at,omitempty"`
}

type RetailFinanceShiftReconciliationReport struct {
	OrganizationID string                                `json:"organization_id"`
	LocationID     string                                `json:"location_id,omitempty"`
	AsOfDate       string                                `json:"as_of_date,omitempty"`
	Rows           []RetailFinanceShiftReconciliationRow `json:"rows"`
	Totals         map[string]float64                    `json:"totals"`
}

type RetailFinanceTenderSettlementRow struct {
	SettlementID        string  `json:"settlement_id"`
	ReconciliationID    string  `json:"reconciliation_id"`
	ShiftID             string  `json:"shift_id"`
	ShiftNumber         string  `json:"shift_number"`
	StoreCode           string  `json:"store_code"`
	RegisterCode        string  `json:"register_code"`
	TenderTypeCode      string  `json:"tender_type_code"`
	TenderKind          string  `json:"tender_kind"`
	ClearingAccountCode string  `json:"clearing_account_code"`
	ExpectedAmount      float64 `json:"expected_amount"`
	SettledAmount       float64 `json:"settled_amount"`
	DifferenceAmount    float64 `json:"difference_amount"`
	SettlementDate      string  `json:"settlement_date,omitempty"`
	SettlementReference string  `json:"settlement_reference,omitempty"`
	Status              string  `json:"status"`
}

type RetailFinanceTenderSettlementReport struct {
	OrganizationID string                             `json:"organization_id"`
	LocationID     string                             `json:"location_id,omitempty"`
	AsOfDate       string                             `json:"as_of_date,omitempty"`
	Rows           []RetailFinanceTenderSettlementRow `json:"rows"`
	Totals         map[string]float64                 `json:"totals"`
}

type RetailFinanceCoreService struct {
	documents *document.Service
	models    *model.Service
	config    *config.Service
	finance   *FinanceReportingCoreService
}

func NewRetailFinanceCoreService(documents *document.Service, models *model.Service, configSvc *config.Service, finance *FinanceReportingCoreService) *RetailFinanceCoreService {
	return &RetailFinanceCoreService{documents: documents, models: models, config: configSvc, finance: finance}
}

func (s *RetailFinanceCoreService) SyncShiftReconciliation(organizationID, locationID, shiftID, actorID string) (model.Record, error) {
	shift, err := s.models.Get("pos_shift", strings.TrimSpace(shiftID))
	if err != nil {
		return model.Record{}, err
	}
	summary := s.shiftTenderSummary(shift.ID, numberValue(shift.Values["opening_cash_amount"]))
	values := map[string]any{
		"organization_id":       strings.TrimSpace(organizationID),
		"location_id":           strings.TrimSpace(locationID),
		"shift_id":              shift.ID,
		"shift_number":          textValue(shift.Values["shift_number"]),
		"store_code":            textValue(shift.Values["store_code"]),
		"register_code":         textValue(shift.Values["register_code"]),
		"cashier_user_id":       textValue(shift.Values["cashier_user_id"]),
		"reconciliation_date":   firstNonEmptyString(textValue(shift.Values["closed_at"]), textValue(shift.Values["opened_at"]), time.Now().UTC().Format(time.RFC3339)),
		"expected_cash_amount":  roundMoney(summary.ExpectedCash),
		"actual_cash_amount":    roundMoney(numberValue(shift.Values["actual_cash_amount"])),
		"over_short_amount":     roundMoney(numberValue(shift.Values["actual_cash_amount"]) - summary.ExpectedCash),
		"expected_total_amount": roundMoney(summary.ExpectedTotal),
		"counted_total_amount":  roundMoney(summary.ExpectedTotal - summary.ExpectedCash + numberValue(shift.Values["actual_cash_amount"])),
		"tender_summary_json":   marshalJSONString(summary.Rows),
		"status":                firstNonEmptyString(mapShiftToReconciliationStatus(textValue(shift.Values["status"])), "draft"),
		"posting_id":            "",
		"approved_by":           "",
		"approved_at":           "",
		"notes":                 textValue(shift.Values["notes"]),
	}
	record, err := s.upsertRetailModel("pos_tender_reconciliation", map[string]string{"shift_id": shift.ID}, actorID, values)
	if err != nil {
		return model.Record{}, err
	}
	if err := s.syncSettlementsForReconciliation(record, shift, organizationID, locationID, actorID, summary); err != nil {
		return model.Record{}, err
	}
	return s.models.Get("pos_tender_reconciliation", record.ID)
}

func (s *RetailFinanceCoreService) ApproveShiftReconciliation(reconciliationID, actorID string) (model.Record, error) {
	record, err := s.models.Get("pos_tender_reconciliation", strings.TrimSpace(reconciliationID))
	if err != nil {
		return model.Record{}, err
	}
	values := cloneMap(record.Values)
	postingID := textValue(values["posting_id"])
	if roundMoney(numberValue(values["over_short_amount"])) != 0 && postingID == "" {
		posting, err := s.createOverShortPosting(record, actorID)
		if err != nil {
			return model.Record{}, err
		}
		postingID = posting.Header.ID
		values["posting_id"] = postingID
	}
	values["status"] = "posted"
	values["approved_by"] = actorID
	values["approved_at"] = time.Now().UTC().Format(time.RFC3339)
	updated, err := s.models.Update("pos_tender_reconciliation", record.ID, actorID, values, record.Version)
	if err != nil {
		return model.Record{}, err
	}
	if err := s.markSettlementsPosted(record.ID, actorID); err != nil {
		return model.Record{}, err
	}
	return updated, nil
}

func (s *RetailFinanceCoreService) SettleTenderSettlement(settlementID, actorID string, settledAmount float64, settlementDate, reference, notes string) (model.Record, error) {
	record, err := s.models.Get("pos_tender_settlement", strings.TrimSpace(settlementID))
	if err != nil {
		return model.Record{}, err
	}
	values := cloneMap(record.Values)
	expected := roundMoney(numberValue(values["expected_amount"]))
	settled := roundMoney(settledAmount)
	values["settled_amount"] = settled
	values["difference_amount"] = roundMoney(expected - settled)
	values["settlement_date"] = firstNonEmptyString(strings.TrimSpace(settlementDate), time.Now().UTC().Format("2006-01-02"))
	values["settlement_reference"] = strings.TrimSpace(reference)
	values["notes"] = strings.TrimSpace(notes)
	switch {
	case settled <= 0:
		values["status"] = "open"
	case settled < expected:
		values["status"] = "partially_settled"
	default:
		values["status"] = "settled"
	}
	return s.models.Update("pos_tender_settlement", record.ID, actorID, values, record.Version)
}

func (s *RetailFinanceCoreService) ShiftReconciliationReport(organizationID, locationID, asOfDate, storeCode, registerCode string) RetailFinanceShiftReconciliationReport {
	report := RetailFinanceShiftReconciliationReport{
		OrganizationID: strings.TrimSpace(organizationID),
		LocationID:     strings.TrimSpace(locationID),
		AsOfDate:       normalizeAsOfDate(asOfDate),
		Rows:           []RetailFinanceShiftReconciliationRow{},
		Totals:         map[string]float64{},
	}
	items := s.listRetailModels("pos_tender_reconciliation", map[string]string{
		"organization_id": strings.TrimSpace(organizationID),
		"location_id":     strings.TrimSpace(locationID),
	})
	for _, item := range items {
		if storeCode != "" && textValue(item.Values["store_code"]) != storeCode {
			continue
		}
		if registerCode != "" && textValue(item.Values["register_code"]) != registerCode {
			continue
		}
		if report.AsOfDate != "" {
			reconDate := textValue(item.Values["reconciliation_date"])
			if reconDate != "" && reconDate[:10] > report.AsOfDate {
				continue
			}
		}
		row := RetailFinanceShiftReconciliationRow{
			ReconciliationID:    item.ID,
			ShiftID:             textValue(item.Values["shift_id"]),
			ShiftNumber:         textValue(item.Values["shift_number"]),
			StoreCode:           textValue(item.Values["store_code"]),
			RegisterCode:        textValue(item.Values["register_code"]),
			CashierUserID:       textValue(item.Values["cashier_user_id"]),
			ReconciliationDate:  textValue(item.Values["reconciliation_date"]),
			ExpectedCashAmount:  roundMoney(numberValue(item.Values["expected_cash_amount"])),
			ActualCashAmount:    roundMoney(numberValue(item.Values["actual_cash_amount"])),
			OverShortAmount:     roundMoney(numberValue(item.Values["over_short_amount"])),
			ExpectedTotalAmount: roundMoney(numberValue(item.Values["expected_total_amount"])),
			CountedTotalAmount:  roundMoney(numberValue(item.Values["counted_total_amount"])),
			Status:              textValue(item.Values["status"]),
			PostingID:           textValue(item.Values["posting_id"]),
			ApprovedBy:          textValue(item.Values["approved_by"]),
			ApprovedAt:          textValue(item.Values["approved_at"]),
		}
		report.Rows = append(report.Rows, row)
		report.Totals["expected_cash_amount"] = roundMoney(report.Totals["expected_cash_amount"] + row.ExpectedCashAmount)
		report.Totals["actual_cash_amount"] = roundMoney(report.Totals["actual_cash_amount"] + row.ActualCashAmount)
		report.Totals["over_short_amount"] = roundMoney(report.Totals["over_short_amount"] + row.OverShortAmount)
		report.Totals["expected_total_amount"] = roundMoney(report.Totals["expected_total_amount"] + row.ExpectedTotalAmount)
	}
	sort.Slice(report.Rows, func(i, j int) bool {
		if report.Rows[i].ReconciliationDate != report.Rows[j].ReconciliationDate {
			return report.Rows[i].ReconciliationDate > report.Rows[j].ReconciliationDate
		}
		return report.Rows[i].ShiftNumber > report.Rows[j].ShiftNumber
	})
	return report
}

func (s *RetailFinanceCoreService) TenderSettlementReport(organizationID, locationID, asOfDate, storeCode, registerCode, status string) RetailFinanceTenderSettlementReport {
	report := RetailFinanceTenderSettlementReport{
		OrganizationID: strings.TrimSpace(organizationID),
		LocationID:     strings.TrimSpace(locationID),
		AsOfDate:       normalizeAsOfDate(asOfDate),
		Rows:           []RetailFinanceTenderSettlementRow{},
		Totals:         map[string]float64{},
	}
	items := s.listRetailModels("pos_tender_settlement", map[string]string{
		"organization_id": strings.TrimSpace(organizationID),
		"location_id":     strings.TrimSpace(locationID),
	})
	for _, item := range items {
		if storeCode != "" && textValue(item.Values["store_code"]) != storeCode {
			continue
		}
		if registerCode != "" && textValue(item.Values["register_code"]) != registerCode {
			continue
		}
		if strings.TrimSpace(status) != "" && !strings.EqualFold(textValue(item.Values["status"]), strings.TrimSpace(status)) {
			continue
		}
		if report.AsOfDate != "" {
			settleDate := textValue(item.Values["settlement_date"])
			if settleDate != "" && settleDate > report.AsOfDate {
				continue
			}
		}
		row := RetailFinanceTenderSettlementRow{
			SettlementID:        item.ID,
			ReconciliationID:    textValue(item.Values["reconciliation_id"]),
			ShiftID:             textValue(item.Values["shift_id"]),
			ShiftNumber:         textValue(item.Values["shift_number"]),
			StoreCode:           textValue(item.Values["store_code"]),
			RegisterCode:        textValue(item.Values["register_code"]),
			TenderTypeCode:      textValue(item.Values["tender_type_code"]),
			TenderKind:          textValue(item.Values["tender_kind"]),
			ClearingAccountCode: textValue(item.Values["clearing_account_code"]),
			ExpectedAmount:      roundMoney(numberValue(item.Values["expected_amount"])),
			SettledAmount:       roundMoney(numberValue(item.Values["settled_amount"])),
			DifferenceAmount:    roundMoney(numberValue(item.Values["difference_amount"])),
			SettlementDate:      textValue(item.Values["settlement_date"]),
			SettlementReference: textValue(item.Values["settlement_reference"]),
			Status:              textValue(item.Values["status"]),
		}
		report.Rows = append(report.Rows, row)
		report.Totals["expected_amount"] = roundMoney(report.Totals["expected_amount"] + row.ExpectedAmount)
		report.Totals["settled_amount"] = roundMoney(report.Totals["settled_amount"] + row.SettledAmount)
		report.Totals["difference_amount"] = roundMoney(report.Totals["difference_amount"] + row.DifferenceAmount)
	}
	sort.Slice(report.Rows, func(i, j int) bool {
		if report.Rows[i].ShiftNumber != report.Rows[j].ShiftNumber {
			return report.Rows[i].ShiftNumber > report.Rows[j].ShiftNumber
		}
		return report.Rows[i].TenderTypeCode < report.Rows[j].TenderTypeCode
	})
	return report
}

func (s *RetailFinanceCoreService) CashOverShortReport(organizationID, locationID, asOfDate, storeCode, registerCode string) RetailFinanceShiftReconciliationReport {
	return s.ShiftReconciliationReport(organizationID, locationID, asOfDate, storeCode, registerCode)
}

func (s *RetailFinanceCoreService) LookupGiftCard(organizationID, locationID, code string) (model.Record, error) {
	record, ok := s.findRetailModelByFields("gift_card", map[string]string{
		"organization_id": strings.TrimSpace(organizationID),
		"location_id":     strings.TrimSpace(locationID),
		"code":            strings.TrimSpace(code),
	})
	if !ok {
		return model.Record{}, shared.NotFound("gift card not found")
	}
	return record, nil
}

func (s *RetailFinanceCoreService) LookupStoreCredit(organizationID, locationID, partyID string) (model.Record, error) {
	record, ok := s.findRetailModelByFields("store_credit_account", map[string]string{
		"organization_id": strings.TrimSpace(organizationID),
		"location_id":     strings.TrimSpace(locationID),
		"party_id":        strings.TrimSpace(partyID),
	})
	if !ok {
		return model.Record{}, shared.NotFound("store credit account not found")
	}
	return record, nil
}

func (s *RetailFinanceCoreService) IssueGiftCard(organizationID, locationID, actorID string, payload map[string]any) (map[string]any, error) {
	code := strings.TrimSpace(textValue(payload["code"]))
	if code == "" {
		code = posNumber("GC")
	}
	amount := roundMoney(numberValue(payload["amount"]))
	if amount <= 0 {
		return nil, shared.Validation("gift card amount must be greater than zero")
	}
	liabilityAccount := firstNonEmptyString(
		textValue(payload["liability_account_code"]),
		s.retailPostingConfig(organizationID, locationID)["gift_card_liability_account_code"],
		defaultGiftCardLiabilityAccount,
	)
	cardValues := map[string]any{
		"organization_id":        organizationID,
		"location_id":            locationID,
		"code":                   code,
		"store_code":             textValue(payload["store_code"]),
		"party_id":               textValue(payload["party_id"]),
		"party_name":             textValue(payload["party_name"]),
		"currency_code":          firstNonEmptyString(textValue(payload["currency_code"]), "IDR"),
		"issued_amount":          amount,
		"remaining_balance":      amount,
		"liability_account_code": liabilityAccount,
		"status":                 "active",
		"notes":                  textValue(payload["notes"]),
	}
	card, err := s.upsertRetailModel("gift_card", map[string]string{
		"organization_id": strings.TrimSpace(organizationID),
		"location_id":     strings.TrimSpace(locationID),
		"code":            code,
	}, actorID, cardValues)
	if err != nil {
		return nil, err
	}
	transaction, err := s.models.Create("gift_card_transaction", actorID, map[string]any{
		"organization_id":  organizationID,
		"location_id":      locationID,
		"gift_card_id":     card.ID,
		"gift_card_code":   code,
		"party_id":         textValue(payload["party_id"]),
		"transaction_type": "issue",
		"amount":           amount,
		"balance_after":    amount,
		"reference":        firstNonEmptyString(textValue(payload["reference"]), code),
		"status":           "posted",
	})
	if err != nil {
		return nil, err
	}
	posting, err := s.createStoredValuePosting(organizationID, locationID, actorID, time.Now().UTC().Format("2006-01-02"), firstNonEmptyString(textValue(payload["clearing_account_code"]), textValue(payload["payment_account_code"]), "1000-CASH"), liabilityAccount, amount, "gift_card_issue", code)
	if err != nil {
		return nil, err
	}
	return map[string]any{"gift_card": card, "transaction": transaction, "posting": posting}, nil
}

func (s *RetailFinanceCoreService) ResolveStoredValueTenders(organizationID, locationID, partyID string, tenders []normalizedTender) ([]normalizedTender, error) {
	resolved := make([]normalizedTender, 0, len(tenders))
	for _, tender := range tenders {
		next := tender
		switch strings.ToLower(strings.TrimSpace(tender.Kind)) {
		case "gift_card":
			card, err := s.LookupGiftCard(organizationID, locationID, strings.TrimSpace(tender.Reference))
			if err != nil {
				return nil, err
			}
			if strings.ToLower(textValue(card.Values["status"])) != "active" {
				return nil, shared.Validation("gift card is not active")
			}
			if roundMoney(numberValue(card.Values["remaining_balance"])) < roundMoney(tender.Amount) {
				return nil, shared.Validation("gift card balance is insufficient")
			}
			next.ClearingAccountCode = firstNonEmptyString(next.ClearingAccountCode, textValue(card.Values["liability_account_code"]), s.retailPostingConfig(organizationID, locationID)["gift_card_liability_account_code"], defaultGiftCardLiabilityAccount)
		case "store_credit":
			if strings.TrimSpace(partyID) == "" {
				return nil, shared.Validation("store credit requires a customer")
			}
			account, err := s.LookupStoreCredit(organizationID, locationID, partyID)
			if err != nil {
				return nil, err
			}
			if roundMoney(numberValue(account.Values["balance_amount"])) < roundMoney(tender.Amount) {
				return nil, shared.Validation("store credit balance is insufficient")
			}
			next.Reference = firstNonEmptyString(next.Reference, account.ID)
			next.ClearingAccountCode = firstNonEmptyString(next.ClearingAccountCode, textValue(account.Values["liability_account_code"]), s.retailPostingConfig(organizationID, locationID)["store_credit_liability_account_code"], defaultStoreCreditLiabilityCode)
		}
		resolved = append(resolved, next)
	}
	return resolved, nil
}

func (s *RetailFinanceCoreService) RecordStoredValueRedemptions(organizationID, locationID string, sale model.Record, payments []document.Record, tenders []normalizedTender, actorID, partyID string) error {
	for index, tender := range tenders {
		var paymentID string
		if index < len(payments) {
			paymentID = payments[index].Header.ID
		}
		switch strings.ToLower(strings.TrimSpace(tender.Kind)) {
		case "gift_card":
			card, err := s.LookupGiftCard(organizationID, locationID, strings.TrimSpace(tender.Reference))
			if err != nil {
				return err
			}
			if err := s.adjustGiftCardBalance(card, roundMoney(-tender.Amount), "redeem", organizationID, locationID, actorID, sale.ID, paymentID, tender.Reference); err != nil {
				return err
			}
		case "store_credit":
			account, err := s.LookupStoreCredit(organizationID, locationID, firstNonEmptyString(strings.TrimSpace(partyID), textValue(sale.Values["party_id"])))
			if err != nil {
				return err
			}
			if err := s.adjustStoreCreditBalance(account, roundMoney(-tender.Amount), "redeem", organizationID, locationID, actorID, sale.ID, paymentID, firstNonEmptyString(tender.Reference, sale.ID)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *RetailFinanceCoreService) CreateStoreCreditFromPOSRefund(organizationID, locationID string, sale model.Record, creditNote document.Record, actorID string) (map[string]any, error) {
	partyID := strings.TrimSpace(textValue(sale.Values["party_id"]))
	if partyID == "" {
		return nil, shared.Validation("store credit refund requires a customer")
	}
	account, err := s.ensureStoreCreditAccount(organizationID, locationID, partyID, textValue(sale.Values["party_name"]), actorID)
	if err != nil {
		return nil, err
	}
	amount := roundMoney(numberValue(creditNote.Body.Payload["total_amount"]))
	account, transaction, err := s.creditStoreCreditAccount(account, amount, organizationID, locationID, actorID, sale.ID, creditNote.Header.ID, firstNonEmptyString(creditNote.Header.Number, creditNote.Header.ID))
	if err != nil {
		return nil, err
	}
	posting, err := s.createStoredValuePosting(organizationID, locationID, actorID, time.Now().UTC().Format("2006-01-02"), textValue(creditNote.Body.Payload["receivable_account_code"]), textValue(account.Values["liability_account_code"]), amount, "store_credit_refund", sale.ID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"store_credit_account": account,
		"store_credit_txn":     transaction,
		"posting":              posting,
	}, nil
}

type shiftTenderSummary struct {
	ExpectedCash  float64
	ExpectedTotal float64
	Rows          []map[string]any
}

func (s *RetailFinanceCoreService) shiftTenderSummary(shiftID string, openingCash float64) shiftTenderSummary {
	totals := map[string]map[string]any{}
	expectedCash := roundMoney(openingCash)
	expectedTotal := roundMoney(openingCash)
	items, _, err := s.models.List("pos_sale", model.Query{Page: 1, PageSize: 200})
	if err != nil {
		return shiftTenderSummary{ExpectedCash: expectedCash, ExpectedTotal: expectedTotal}
	}
	for _, item := range items {
		if textValue(item.Values["shift_id"]) != shiftID || textValue(item.Values["status"]) != "completed" {
			continue
		}
		for _, tender := range unmarshalRetailJSONList(textValue(item.Values["tenders_json"])) {
			code := textValue(tender["tender_type_code"])
			if code == "" {
				continue
			}
			entry := totals[code]
			if entry == nil {
				entry = map[string]any{
					"tender_type_code":      code,
					"name":                  textValue(tender["name"]),
					"kind":                  textValue(tender["kind"]),
					"expected_amount":       0.0,
					"counted_amount":        0.0,
					"difference_amount":     0.0,
					"clearing_account_code": textValue(tender["clearing_account_code"]),
				}
				totals[code] = entry
			}
			entry["expected_amount"] = roundMoney(numberValue(entry["expected_amount"]) + numberValue(tender["amount"]))
			expectedTotal = roundMoney(expectedTotal + numberValue(tender["amount"]))
			if boolFieldValue(tender["is_cash_like"]) || strings.EqualFold(textValue(tender["kind"]), "cash") {
				expectedCash = roundMoney(expectedCash + numberValue(tender["amount"]))
			}
		}
	}
	rows := make([]map[string]any, 0, len(totals))
	for _, value := range totals {
		rows = append(rows, value)
	}
	sort.Slice(rows, func(i, j int) bool {
		return textValue(rows[i]["tender_type_code"]) < textValue(rows[j]["tender_type_code"])
	})
	return shiftTenderSummary{ExpectedCash: expectedCash, ExpectedTotal: expectedTotal, Rows: rows}
}

func (s *RetailFinanceCoreService) syncSettlementsForReconciliation(reconciliation model.Record, shift model.Record, organizationID, locationID, actorID string, summary shiftTenderSummary) error {
	existing, _, err := s.models.List("pos_tender_settlement", model.Query{
		Page:     1,
		PageSize: 100,
		Filters:  map[string]string{"reconciliation_id": reconciliation.ID},
	})
	if err != nil && !isMissingModelDefinitionError(err) {
		return err
	}
	byTender := map[string]model.Record{}
	for _, record := range existing {
		byTender[textValue(record.Values["tender_type_code"])] = record
	}
	for _, item := range summary.Rows {
		kind := textValue(item["kind"])
		if kind == "" || strings.EqualFold(kind, "cash") || strings.EqualFold(kind, "gift_card") || strings.EqualFold(kind, "store_credit") {
			continue
		}
		values := map[string]any{
			"organization_id":       strings.TrimSpace(organizationID),
			"location_id":           strings.TrimSpace(locationID),
			"reconciliation_id":     reconciliation.ID,
			"shift_id":              shift.ID,
			"shift_number":          textValue(shift.Values["shift_number"]),
			"store_code":            textValue(shift.Values["store_code"]),
			"register_code":         textValue(shift.Values["register_code"]),
			"tender_type_code":      textValue(item["tender_type_code"]),
			"tender_kind":           kind,
			"clearing_account_code": textValue(item["clearing_account_code"]),
			"expected_amount":       roundMoney(numberValue(item["expected_amount"])),
			"settled_amount":        0.0,
			"difference_amount":     roundMoney(numberValue(item["expected_amount"])),
			"status":                "open",
		}
		if current, ok := byTender[textValue(item["tender_type_code"])]; ok {
			if _, err := s.models.Update("pos_tender_settlement", current.ID, actorID, mergeModelValues(current.Values, values), current.Version); err != nil {
				return err
			}
			continue
		}
		if _, err := s.models.Create("pos_tender_settlement", actorID, values); err != nil {
			return err
		}
	}
	return nil
}

func (s *RetailFinanceCoreService) createOverShortPosting(reconciliation model.Record, actorID string) (document.Record, error) {
	amount := roundMoney(numberValue(reconciliation.Values["over_short_amount"]))
	if amount == 0 {
		return document.Record{}, nil
	}
	postingDate := textValue(reconciliation.Values["reconciliation_date"])
	if len(postingDate) >= 10 {
		postingDate = postingDate[:10]
	}
	if postingDate == "" {
		postingDate = time.Now().UTC().Format("2006-01-02")
	}
	organizationID := textValue(reconciliation.Values["organization_id"])
	locationID := textValue(reconciliation.Values["location_id"])
	if s.finance != nil {
		if err := s.finance.ValidatePostingDateOpen(organizationID, locationID, postingDate); err != nil {
			return document.Record{}, err
		}
	}
	cashAccount := s.cashAccountForReconciliation(reconciliation)
	configValues := s.retailPostingConfig(organizationID, locationID)
	gainAccount := firstNonEmptyString(configValues["cash_over_gain_account_code"], defaultCashOverAccountCode)
	lossAccount := firstNonEmptyString(configValues["cash_over_short_loss_account_code"], defaultCashShortAccountCode)
	lines := []map[string]any{}
	if amount > 0 {
		lines = append(lines,
			map[string]any{"account_code": cashAccount, "description": "Cash Over", "debit": amount, "credit": 0.0},
			map[string]any{"account_code": gainAccount, "description": "Cash Over Gain", "debit": 0.0, "credit": amount},
		)
	} else {
		lines = append(lines,
			map[string]any{"account_code": lossAccount, "description": "Cash Short Loss", "debit": roundMoney(-amount), "credit": 0.0},
			map[string]any{"account_code": cashAccount, "description": "Cash Short", "debit": 0.0, "credit": roundMoney(-amount)},
		)
	}
	payload := map[string]any{
		"posting_date":         postingDate,
		"currency_code":        "IDR",
		"source_document_type": "pos_tender_reconciliation",
		"source_document_id":   reconciliation.ID,
		"posting_rule_key":     "pos_cash_over_short",
		"journal_source_kind":  "system",
		"notes":                fmt.Sprintf("POS shift reconciliation %s", textValue(reconciliation.Values["shift_number"])),
		"journal_lines":        lines,
		"total_amount":         roundMoney(maxFloat(amount, -amount)),
	}
	posting, err := s.documents.Create("ledger_posting", organizationID, locationID, actorID, payload)
	if err != nil {
		return document.Record{}, err
	}
	if err := s.finalizeSystemPosting(posting, actorID, "posted"); err != nil {
		return document.Record{}, err
	}
	return s.documents.Get(posting.Header.ID)
}

func (s *RetailFinanceCoreService) createStoredValuePosting(organizationID, locationID, actorID, postingDate, debitAccount, creditAccount string, amount float64, ruleKey, sourceID string) (document.Record, error) {
	if amount <= 0 {
		return document.Record{}, shared.Validation("stored value amount must be greater than zero")
	}
	if s.finance != nil {
		if err := s.finance.ValidatePostingDateOpen(organizationID, locationID, postingDate); err != nil {
			return document.Record{}, err
		}
	}
	payload := map[string]any{
		"posting_date":         postingDate,
		"currency_code":        "IDR",
		"source_document_type": "retail_finance",
		"source_document_id":   sourceID,
		"posting_rule_key":     ruleKey,
		"journal_source_kind":  "system",
		"journal_lines": []map[string]any{
			{"account_code": debitAccount, "description": "Stored Value Debit", "debit": amount, "credit": 0.0},
			{"account_code": creditAccount, "description": "Stored Value Liability", "debit": 0.0, "credit": amount},
		},
		"total_amount": amount,
	}
	posting, err := s.documents.Create("ledger_posting", organizationID, locationID, actorID, payload)
	if err != nil {
		return document.Record{}, err
	}
	if err := s.finalizeSystemPosting(posting, actorID, "posted"); err != nil {
		return document.Record{}, err
	}
	return s.documents.Get(posting.Header.ID)
}

func (s *RetailFinanceCoreService) finalizeSystemPosting(record document.Record, actorID, status string) error {
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

func (s *RetailFinanceCoreService) cashAccountForReconciliation(reconciliation model.Record) string {
	registerCode := textValue(reconciliation.Values["register_code"])
	if registerCode != "" {
		if register, ok := s.findRetailModelByField("pos_register", "code", registerCode); ok {
			if account := textValue(register.Values["cash_account_code"]); account != "" {
				return account
			}
		}
	}
	return "1000-CASH"
}

func (s *RetailFinanceCoreService) ensureStoreCreditAccount(organizationID, locationID, partyID, partyName, actorID string) (model.Record, error) {
	if record, ok := s.findRetailModelByFields("store_credit_account", map[string]string{
		"organization_id": strings.TrimSpace(organizationID),
		"location_id":     strings.TrimSpace(locationID),
		"party_id":        strings.TrimSpace(partyID),
	}); ok {
		return record, nil
	}
	return s.models.Create("store_credit_account", actorID, map[string]any{
		"organization_id":        organizationID,
		"location_id":            locationID,
		"party_id":               partyID,
		"party_name":             partyName,
		"currency_code":          "IDR",
		"balance_amount":         0.0,
		"liability_account_code": firstNonEmptyString(s.retailPostingConfig(organizationID, locationID)["store_credit_liability_account_code"], defaultStoreCreditLiabilityCode),
		"status":                 "active",
	})
}

func (s *RetailFinanceCoreService) creditStoreCreditAccount(account model.Record, amount float64, organizationID, locationID, actorID, saleID, sourceDocumentID, reference string) (model.Record, model.Record, error) {
	values := cloneMap(account.Values)
	balance := roundMoney(numberValue(values["balance_amount"]) + amount)
	values["balance_amount"] = balance
	values["last_activity_at"] = time.Now().UTC().Format(time.RFC3339)
	updated, err := s.models.Update("store_credit_account", account.ID, actorID, values, account.Version)
	if err != nil {
		return model.Record{}, model.Record{}, err
	}
	txn, err := s.models.Create("store_credit_transaction", actorID, map[string]any{
		"organization_id":         organizationID,
		"location_id":             locationID,
		"store_credit_account_id": updated.ID,
		"party_id":                textValue(updated.Values["party_id"]),
		"transaction_type":        "credit",
		"amount":                  amount,
		"balance_after":           balance,
		"pos_sale_id":             saleID,
		"source_document_id":      sourceDocumentID,
		"reference":               reference,
		"status":                  "posted",
	})
	if err != nil {
		return model.Record{}, model.Record{}, err
	}
	return updated, txn, nil
}

func (s *RetailFinanceCoreService) adjustStoreCreditBalance(account model.Record, delta float64, transactionType, organizationID, locationID, actorID, saleID, sourceDocumentID, reference string) error {
	values := cloneMap(account.Values)
	balance := roundMoney(numberValue(values["balance_amount"]) + delta)
	if balance < 0 {
		return shared.Validation("store credit balance cannot be negative")
	}
	values["balance_amount"] = balance
	values["last_activity_at"] = time.Now().UTC().Format(time.RFC3339)
	updated, err := s.models.Update("store_credit_account", account.ID, actorID, values, account.Version)
	if err != nil {
		return err
	}
	_, err = s.models.Create("store_credit_transaction", actorID, map[string]any{
		"organization_id":         organizationID,
		"location_id":             locationID,
		"store_credit_account_id": updated.ID,
		"party_id":                textValue(updated.Values["party_id"]),
		"transaction_type":        transactionType,
		"amount":                  roundMoney(maxFloat(delta, -delta)),
		"balance_after":           balance,
		"pos_sale_id":             saleID,
		"source_document_id":      sourceDocumentID,
		"reference":               reference,
		"status":                  "posted",
	})
	return err
}

func (s *RetailFinanceCoreService) adjustGiftCardBalance(card model.Record, delta float64, transactionType, organizationID, locationID, actorID, saleID, sourceDocumentID, reference string) error {
	values := cloneMap(card.Values)
	balance := roundMoney(numberValue(values["remaining_balance"]) + delta)
	if balance < 0 {
		return shared.Validation("gift card balance cannot be negative")
	}
	values["remaining_balance"] = balance
	values["last_activity_at"] = time.Now().UTC().Format(time.RFC3339)
	if balance == 0 {
		values["status"] = "consumed"
	}
	updated, err := s.models.Update("gift_card", card.ID, actorID, values, card.Version)
	if err != nil {
		return err
	}
	_, err = s.models.Create("gift_card_transaction", actorID, map[string]any{
		"organization_id":    organizationID,
		"location_id":        locationID,
		"gift_card_id":       updated.ID,
		"gift_card_code":     textValue(updated.Values["code"]),
		"party_id":           textValue(updated.Values["party_id"]),
		"transaction_type":   transactionType,
		"amount":             roundMoney(maxFloat(delta, -delta)),
		"balance_after":      balance,
		"pos_sale_id":        saleID,
		"source_document_id": sourceDocumentID,
		"reference":          reference,
		"status":             "posted",
	})
	return err
}

func (s *RetailFinanceCoreService) markSettlementsPosted(reconciliationID, actorID string) error {
	items, _, err := s.models.List("pos_tender_settlement", model.Query{
		Page:     1,
		PageSize: 100,
		Filters:  map[string]string{"reconciliation_id": reconciliationID},
	})
	if err != nil && !isMissingModelDefinitionError(err) {
		return err
	}
	for _, item := range items {
		values := cloneMap(item.Values)
		if roundMoney(numberValue(values["settled_amount"])) >= roundMoney(numberValue(values["expected_amount"])) && roundMoney(numberValue(values["expected_amount"])) > 0 {
			values["status"] = "settled"
		} else if roundMoney(numberValue(values["settled_amount"])) > 0 {
			values["status"] = "partially_settled"
		} else {
			values["status"] = "open"
		}
		if _, err := s.models.Update("pos_tender_settlement", item.ID, actorID, values, item.Version); err != nil {
			return err
		}
	}
	return nil
}

func (s *RetailFinanceCoreService) upsertRetailModel(modelKey string, filters map[string]string, actorID string, values map[string]any) (model.Record, error) {
	items := s.listRetailModels(modelKey, filters)
	if len(items) == 0 {
		return s.models.Create(modelKey, actorID, values)
	}
	current := items[0]
	return s.models.Update(modelKey, current.ID, actorID, mergeModelValues(current.Values, values), current.Version)
}

func (s *RetailFinanceCoreService) listRetailModels(modelKey string, filters map[string]string) []model.Record {
	items, _, err := s.models.List(modelKey, model.Query{Page: 1, PageSize: 200, Filters: filters})
	if err != nil {
		return nil
	}
	return items
}

func (s *RetailFinanceCoreService) findRetailModelByField(modelKey, fieldKey, value string) (model.Record, bool) {
	if strings.TrimSpace(value) == "" {
		return model.Record{}, false
	}
	items := s.listRetailModels(modelKey, map[string]string{fieldKey: value})
	if len(items) == 0 {
		return model.Record{}, false
	}
	return items[0], true
}

func (s *RetailFinanceCoreService) findRetailModelByFields(modelKey string, filters map[string]string) (model.Record, bool) {
	items := s.listRetailModels(modelKey, filters)
	if len(items) == 0 {
		return model.Record{}, false
	}
	return items[0], true
}

func (s *RetailFinanceCoreService) retailPostingConfig(organizationID, locationID string) map[string]string {
	if s.config == nil {
		return map[string]string{
			"cash_over_gain_account_code":         defaultCashOverAccountCode,
			"cash_over_short_loss_account_code":   defaultCashShortAccountCode,
			"gift_card_liability_account_code":    defaultGiftCardLiabilityAccount,
			"store_credit_liability_account_code": defaultStoreCreditLiabilityCode,
		}
	}
	entry, ok := s.config.Resolve("retail_finance.posting", strings.TrimSpace(organizationID), strings.TrimSpace(locationID))
	if !ok {
		return map[string]string{
			"cash_over_gain_account_code":         defaultCashOverAccountCode,
			"cash_over_short_loss_account_code":   defaultCashShortAccountCode,
			"gift_card_liability_account_code":    defaultGiftCardLiabilityAccount,
			"store_credit_liability_account_code": defaultStoreCreditLiabilityCode,
		}
	}
	return map[string]string{
		"cash_over_gain_account_code":         firstNonEmptyString(textValue(entry.Value["cash_over_gain_account_code"]), defaultCashOverAccountCode),
		"cash_over_short_loss_account_code":   firstNonEmptyString(textValue(entry.Value["cash_over_short_loss_account_code"]), defaultCashShortAccountCode),
		"gift_card_liability_account_code":    firstNonEmptyString(textValue(entry.Value["gift_card_liability_account_code"]), defaultGiftCardLiabilityAccount),
		"store_credit_liability_account_code": firstNonEmptyString(textValue(entry.Value["store_credit_liability_account_code"]), defaultStoreCreditLiabilityCode),
	}
}

func mapShiftToReconciliationStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "closed":
		return "submitted"
	case "opened":
		return "draft"
	default:
		return "draft"
	}
}

func unmarshalRetailJSONList(raw string) []map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var items []map[string]any
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil
	}
	return items
}
