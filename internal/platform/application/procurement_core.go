package application

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"orbyte/internal/platform/config"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/shared"
)

type ProcurementCoreService struct {
	documents *document.Service
	config    *config.Service
	models    *model.Service
	search    *search.Service
	finance   *FinanceReportingCoreService
	inventory *InventoryCoreService
}

type PayablesSummary struct {
	OpenBillCount       int                `json:"open_bill_count"`
	OpenBalanceTotal    float64            `json:"open_balance_total"`
	OverdueBillCount    int                `json:"overdue_bill_count"`
	OverdueBalanceTotal float64            `json:"overdue_balance_total"`
	DueTodayCount       int                `json:"due_today_count"`
	DueTodayTotal       float64            `json:"due_today_total"`
	CurrentBalanceTotal float64            `json:"current_balance_total"`
	PaidAmountTotal     float64            `json:"paid_amount_total"`
	CreditedAmountTotal float64            `json:"credited_amount_total"`
	Aging               map[string]float64 `json:"aging"`
	Items               []map[string]any   `json:"items"`
}

type VendorPayablesSummary struct {
	VendorID            string           `json:"vendor_id"`
	OpenBillCount       int              `json:"open_bill_count"`
	OpenBalanceTotal    float64          `json:"open_balance_total"`
	PaidAmountTotal     float64          `json:"paid_amount_total"`
	CreditedAmountTotal float64          `json:"credited_amount_total"`
	OpenBills           []map[string]any `json:"open_bills"`
	Activities          []map[string]any `json:"activities"`
}

func NewProcurementCoreService(documents *document.Service, configSvc *config.Service, models *model.Service, searchSvc *search.Service) *ProcurementCoreService {
	return &ProcurementCoreService{documents: documents, config: configSvc, models: models, search: searchSvc}
}

func (s *ProcurementCoreService) SetFinanceReporting(finance *FinanceReportingCoreService) {
	s.finance = finance
}

func (s *ProcurementCoreService) SetInventoryCore(inventory *InventoryCoreService) {
	s.inventory = inventory
}

func (s *ProcurementCoreService) NormalizePayload(documentType string, payload map[string]any) map[string]any {
	switch strings.TrimSpace(documentType) {
	case "purchase_request", "purchase_order", "vendor_bill", "vendor_credit_note":
		return s.normalizeProcurementLines(payload)
	case "goods_receipt":
		return s.normalizeReceiptLines(payload)
	case "payment_out":
		return s.normalizeBillAllocations(payload)
	default:
		return document.NormalizePayload(cloneMap(payload))
	}
}

func (s *ProcurementCoreService) GeneratePurchaseOrderFromRequest(requestID, actorID string) (document.Record, error) {
	req, err := s.documents.Get(strings.TrimSpace(requestID))
	if err != nil {
		return document.Record{}, err
	}
	if req.Header.Type != "purchase_request" {
		return document.Record{}, shared.Validation("source document must be a purchase request")
	}
	if req.Header.Status != "approved" {
		return document.Record{}, shared.Conflict("purchase order can only be generated from an approved request")
	}
	payload := s.NormalizePayload(req.Header.Type, req.Body.Payload)
	now := time.Now().UTC()
	poPayload := map[string]any{
		"vendor_id":                      textValue(payload["vendor_id"]),
		"vendor_name":                    textValue(payload["vendor_name"]),
		"order_date":                     now.Format("2006-01-02"),
		"currency_code":                  firstNonEmptyString(textValue(payload["currency_code"]), "IDR"),
		"tax_profile_code":               textValue(payload["tax_profile_code"]),
		"payment_term_days":              numberValue(payload["payment_term_days"]),
		"default_tax_code":               textValue(payload["default_tax_code"]),
		"source_purchase_request_id":     req.Header.ID,
		"source_purchase_request_number": firstNonEmptyString(req.Header.Number, req.Header.ID),
		"expected_receipt_date":          textValue(payload["needed_by_date"]),
		"subtotal_amount":                numberValue(payload["subtotal_amount"]),
		"tax_amount":                     numberValue(payload["tax_amount"]),
		"total_amount":                   numberValue(payload["total_amount"]),
		"lines":                          incrementProcurementLineMetrics(recordList(payload["lines"]), "ordered_qty", "quantity"),
		"notes":                          textValue(payload["notes"]),
	}
	record, err := s.documents.Create("purchase_order", req.Header.OrganizationID, req.Header.LocationID, actorID, poPayload)
	if err != nil {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(record.Header.ID, req.Header.ID, "source_request", map[string]any{"source_type": "purchase_request"}); err != nil {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(req.Header.ID, record.Header.ID, "purchase_order_for", map[string]any{"generated_document_type": "purchase_order"}); err != nil {
		return document.Record{}, err
	}
	created, err := s.documents.Get(record.Header.ID)
	if err != nil {
		return document.Record{}, err
	}
	s.refreshDocuments(created, req)
	return created, nil
}

func (s *ProcurementCoreService) CreateGoodsReceiptFromOrder(orderID, actorID string) (document.Record, error) {
	order, err := s.documents.Get(strings.TrimSpace(orderID))
	if err != nil {
		return document.Record{}, err
	}
	if order.Header.Type != "purchase_order" {
		return document.Record{}, shared.Validation("source document must be a purchase order")
	}
	if order.Header.Status != "approved" && order.Header.Status != "partially_received" {
		return document.Record{}, shared.Conflict("goods receipt can only be registered from an approved or partially received purchase order")
	}
	payload := s.NormalizePayload(order.Header.Type, order.Body.Payload)
	rows := make([]map[string]any, 0)
	for _, line := range recordList(payload["lines"]) {
		orderedQty := roundMoney(numberValue(line["ordered_qty"]))
		if orderedQty <= 0 {
			orderedQty = roundMoney(numberValue(line["quantity"]))
		}
		receivedQty := roundMoney(numberValue(line["received_qty"]))
		remaining := roundMoney(maxFloat(orderedQty-receivedQty, 0))
		if remaining <= 0 {
			continue
		}
		rows = append(rows, map[string]any{
			"item_code":               textValue(line["item_code"]),
			"description":             textValue(line["description"]),
			"uom_code":                textValue(line["uom_code"]),
			"warehouse_code":          textValue(line["warehouse_code"]),
			"unit_price":              roundMoney(numberValue(line["unit_price"])),
			"discount_amount":         roundMoney(numberValue(line["discount_amount"])),
			"tax_code":                textValue(line["tax_code"]),
			"tax_rate":                roundMoney(numberValue(line["tax_rate"])),
			"tax_mode":                textValue(line["tax_mode"]),
			"tax_account_code":        textValue(line["tax_account_code"]),
			"expense_account_code":    textValue(line["expense_account_code"]),
			"ordered_qty":             orderedQty,
			"receipt_qty":             remaining,
			"cumulative_received_qty": receivedQty,
			"line_total":              roundMoney(numberValue(line["line_total"])),
			"note":                    "",
		})
	}
	if len(rows) == 0 {
		return document.Record{}, shared.Conflict("purchase order has no remaining quantity to receive")
	}
	now := time.Now().UTC()
	receiptPayload := map[string]any{
		"vendor_id":                    textValue(payload["vendor_id"]),
		"vendor_name":                  textValue(payload["vendor_name"]),
		"receipt_date":                 now.Format("2006-01-02"),
		"currency_code":                firstNonEmptyString(textValue(payload["currency_code"]), "IDR"),
		"source_purchase_order_id":     order.Header.ID,
		"source_purchase_order_number": firstNonEmptyString(order.Header.Number, order.Header.ID),
		"lines":                        rows,
		"notes":                        textValue(payload["notes"]),
	}
	record, err := s.documents.Create("goods_receipt", order.Header.OrganizationID, order.Header.LocationID, actorID, receiptPayload)
	if err != nil {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(record.Header.ID, order.Header.ID, "receipt_for", map[string]any{"source_type": "purchase_order"}); err != nil {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(order.Header.ID, record.Header.ID, "receipt_for", map[string]any{"generated_document_type": "goods_receipt"}); err != nil {
		return document.Record{}, err
	}
	created, err := s.documents.Get(record.Header.ID)
	if err != nil {
		return document.Record{}, err
	}
	s.refreshDocuments(created, order)
	return created, nil
}

func (s *ProcurementCoreService) CreateVendorBillFromOrder(orderID, actorID string) (document.Record, error) {
	order, err := s.documents.Get(strings.TrimSpace(orderID))
	if err != nil {
		return document.Record{}, err
	}
	if order.Header.Type != "purchase_order" {
		return document.Record{}, shared.Validation("source document must be a purchase order")
	}
	if order.Header.Status != "approved" && order.Header.Status != "partially_received" && order.Header.Status != "received" {
		return document.Record{}, shared.Conflict("vendor bill can only be registered from an approved purchase order")
	}
	return s.createVendorBillFromSource(order, actorID, "purchase_order")
}

func (s *ProcurementCoreService) CreateVendorBillFromReceipt(receiptID, actorID string) (document.Record, error) {
	receipt, err := s.documents.Get(strings.TrimSpace(receiptID))
	if err != nil {
		return document.Record{}, err
	}
	if receipt.Header.Type != "goods_receipt" {
		return document.Record{}, shared.Validation("source document must be a goods receipt")
	}
	if receipt.Header.Status != "received" {
		return document.Record{}, shared.Conflict("vendor bill can only be registered from a received goods receipt")
	}
	return s.createVendorBillFromSource(receipt, actorID, "goods_receipt")
}

func (s *ProcurementCoreService) CreatePaymentOutFromBill(billID, actorID string) (document.Record, error) {
	bill, err := s.documents.Get(strings.TrimSpace(billID))
	if err != nil {
		return document.Record{}, err
	}
	if bill.Header.Type != "vendor_bill" {
		return document.Record{}, shared.Validation("source document must be a vendor bill")
	}
	if bill.Header.Status != "issued" && bill.Header.Status != "partially_paid" {
		return document.Record{}, shared.Conflict("payment out can only be registered from an issued vendor bill")
	}
	payload := clonedPayload(bill.Body.Payload)
	openAmount := roundMoney(numberValue(payload["balance_due_amount"]))
	if openAmount <= 0 {
		openAmount = roundMoney(numberValue(payload["total_amount"]) - numberValue(payload["paid_amount"]) - numberValue(payload["credited_amount"]))
	}
	if openAmount <= 0 {
		return document.Record{}, shared.Conflict("vendor bill has no remaining balance")
	}
	now := time.Now().UTC()
	paymentPayload := map[string]any{
		"vendor_id":           textValue(payload["vendor_id"]),
		"vendor_name":         textValue(payload["vendor_name"]),
		"payment_date":        now.Format("2006-01-02"),
		"payment_method_code": firstNonEmptyString(textValue(payload["default_payment_method_code"]), s.lookupVendorValue(textValue(payload["vendor_id"]), "default_payment_method_code")),
		"payment_reference":   firstNonEmptyString(bill.Header.Number, bill.Header.ID),
		"currency_code":       firstNonEmptyString(textValue(payload["currency_code"]), bill.Header.TotalAmount.Currency, "IDR"),
		"amount_paid":         openAmount,
		"unapplied_amount":    0.0,
		"allocations": []map[string]any{{
			"bill_number": firstNonEmptyString(bill.Header.Number, bill.Header.ID),
			"bill_id":     bill.Header.ID,
			"amount":      openAmount,
			"note":        "Generated from vendor bill",
		}},
		"payable_account_code": firstNonEmptyString(textValue(payload["payable_account_code"]), s.lookupVendorValue(textValue(payload["vendor_id"]), "payable_account_code")),
		"notes":                "",
	}
	paymentPayload = s.normalizeBillAllocations(paymentPayload)
	record, err := s.documents.Create("payment_out", bill.Header.OrganizationID, bill.Header.LocationID, actorID, paymentPayload)
	if err != nil {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(record.Header.ID, bill.Header.ID, "payment_for", map[string]any{"source_type": "vendor_bill"}); err != nil {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(bill.Header.ID, record.Header.ID, "payment_for", map[string]any{"generated_document_type": "payment_out"}); err != nil {
		return document.Record{}, err
	}
	created, err := s.documents.Get(record.Header.ID)
	if err != nil {
		return document.Record{}, err
	}
	s.refreshDocuments(created, bill)
	return created, nil
}

func (s *ProcurementCoreService) CreateVendorCreditFromBill(billID, actorID string) (document.Record, error) {
	bill, err := s.documents.Get(strings.TrimSpace(billID))
	if err != nil {
		return document.Record{}, err
	}
	if bill.Header.Type != "vendor_bill" {
		return document.Record{}, shared.Validation("source document must be a vendor bill")
	}
	if bill.Header.Status != "issued" && bill.Header.Status != "partially_paid" && bill.Header.Status != "paid" {
		return document.Record{}, shared.Conflict("vendor credit can only be generated from an issued, partially paid, or paid bill")
	}
	payload := clonedPayload(bill.Body.Payload)
	creditableAmount := roundMoney(numberValue(payload["balance_due_amount"]))
	if creditableAmount <= 0 {
		creditableAmount = roundMoney(numberValue(payload["total_amount"]) - numberValue(payload["credited_amount"]))
	}
	if creditableAmount <= 0 {
		return document.Record{}, shared.Conflict("vendor bill has no remaining creditable balance")
	}
	lines := scaledCommercialLines(recordList(payload["lines"]), creditableAmount, roundMoney(numberValue(payload["total_amount"])))
	subtotal, taxAmount, totalAmount := commercialLineTotals(lines)
	now := time.Now().UTC()
	creditPayload := map[string]any{
		"vendor_id":                 textValue(payload["vendor_id"]),
		"vendor_name":               textValue(payload["vendor_name"]),
		"credit_date":               now.Format("2006-01-02"),
		"currency_code":             firstNonEmptyString(textValue(payload["currency_code"]), bill.Header.TotalAmount.Currency, "IDR"),
		"source_vendor_bill_id":     bill.Header.ID,
		"source_vendor_bill_number": firstNonEmptyString(bill.Header.Number, bill.Header.ID),
		"payable_account_code":      textValue(payload["payable_account_code"]),
		"subtotal_amount":           subtotal,
		"tax_amount":                taxAmount,
		"total_amount":              totalAmount,
		"reason":                    fmt.Sprintf("Vendor credit for bill %s", firstNonEmptyString(bill.Header.Number, bill.Header.ID)),
		"lines":                     lines,
		"notes":                     textValue(payload["notes"]),
	}
	record, err := s.documents.Create("vendor_credit_note", bill.Header.OrganizationID, bill.Header.LocationID, actorID, creditPayload)
	if err != nil {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(record.Header.ID, bill.Header.ID, "credit_for", map[string]any{"source_type": "vendor_bill"}); err != nil {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(bill.Header.ID, record.Header.ID, "credit_for", map[string]any{"generated_document_type": "vendor_credit_note"}); err != nil {
		return document.Record{}, err
	}
	created, err := s.documents.Get(record.Header.ID)
	if err != nil {
		return document.Record{}, err
	}
	s.refreshDocuments(created, bill)
	return created, nil
}

func (s *ProcurementCoreService) ValidateApprove(record document.Record) error {
	switch record.Header.Type {
	case "vendor_bill":
		if strings.TrimSpace(textValue(record.Body.Payload["vendor_id"])) == "" {
			return shared.Validation("vendor bill vendor is required")
		}
	case "vendor_credit_note":
		if strings.TrimSpace(textValue(record.Body.Payload["source_vendor_bill_id"])) == "" {
			return shared.Validation("vendor credit source bill is required")
		}
	case "payment_out":
		if len(recordList(record.Body.Payload["allocations"])) == 0 {
			return shared.Validation("payment out allocations are required")
		}
	}
	return nil
}

func (s *ProcurementCoreService) ValidateCancel(record document.Record) error {
	switch record.Header.Type {
	case "purchase_order":
		if record.Header.Status == "approved" || record.Header.Status == "partially_received" || record.Header.Status == "received" {
			payload := clonedPayload(record.Body.Payload)
			for _, line := range recordList(payload["lines"]) {
				if roundMoney(numberValue(line["received_qty"])) > 0 || roundMoney(numberValue(line["billed_qty"])) > 0 {
					return shared.Conflict("purchase order with receipts or bills cannot be cancelled; reverse downstream activity first")
				}
			}
		}
	case "vendor_bill":
		if record.Header.Status != "issued" {
			return nil
		}
		if roundMoney(numberValue(record.Body.Payload["paid_amount"])) > 0 || roundMoney(numberValue(record.Body.Payload["credited_amount"])) > 0 {
			return shared.Conflict("vendor bill with payments or credits cannot be cancelled; reverse them first")
		}
	case "payment_out":
		if record.Header.Status != "paid" {
			return nil
		}
	case "vendor_credit_note":
		if record.Header.Status != "issued" {
			return nil
		}
	}
	return nil
}

func (s *ProcurementCoreService) HandleApprovedDocument(record document.Record, actorID string) error {
	switch record.Header.Type {
	case "goods_receipt":
		return s.handleReceivedGoodsReceipt(record, actorID)
	case "vendor_bill":
		return s.handleIssuedVendorBill(record, actorID)
	case "payment_out":
		return s.handlePaidOut(record, actorID)
	case "vendor_credit_note":
		return s.handleIssuedVendorCredit(record, actorID)
	default:
		return nil
	}
}

func (s *ProcurementCoreService) HandleCanceledDocument(record document.Record, actorID string) error {
	current, err := s.documents.Get(record.Header.ID)
	if err == nil {
		record = current
	}
	switch record.Header.Type {
	case "goods_receipt":
		return s.handleCancelledGoodsReceipt(record, actorID)
	case "vendor_bill":
		return s.handleCancelledVendorBill(record, actorID)
	case "payment_out":
		return s.handleCancelledPaymentOut(record, actorID)
	case "vendor_credit_note":
		return s.handleCancelledVendorCredit(record, actorID)
	default:
		return nil
	}
}

func (s *ProcurementCoreService) PayablesSummary(now time.Time) PayablesSummary {
	return s.PayablesSummaryScoped("", "", now)
}

func (s *ProcurementCoreService) PayablesSummaryScoped(organizationID, locationID string, now time.Time) PayablesSummary {
	summary := PayablesSummary{
		Aging: map[string]float64{
			"current":       0,
			"due_today":     0,
			"overdue_1_30":  0,
			"overdue_31_60": 0,
			"overdue_61_up": 0,
		},
		Items: make([]map[string]any, 0),
	}
	today := now.UTC().Format("2006-01-02")
	for _, record := range s.documents.List() {
		if !matchesCommercialScope(record, organizationID, locationID) {
			continue
		}
		if record.Header.Type != "vendor_bill" {
			continue
		}
		if record.Header.Status != "issued" && record.Header.Status != "partially_paid" {
			continue
		}
		payload := clonedPayload(record.Body.Payload)
		summary.PaidAmountTotal = roundMoney(summary.PaidAmountTotal + roundMoney(numberValue(payload["paid_amount"])))
		summary.CreditedAmountTotal = roundMoney(summary.CreditedAmountTotal + roundMoney(numberValue(payload["credited_amount"])))
		balance := roundMoney(numberValue(payload["balance_due_amount"]))
		if balance <= 0 {
			continue
		}
		summary.OpenBillCount++
		summary.OpenBalanceTotal = roundMoney(summary.OpenBalanceTotal + balance)
		dueDate := textValue(payload["due_date"])
		bucket := "current"
		if dueDate == today {
			summary.DueTodayCount++
			summary.DueTodayTotal = roundMoney(summary.DueTodayTotal + balance)
			bucket = "due_today"
		} else if overdueDays := dateDiffDays(dueDate, today); overdueDays > 0 {
			summary.OverdueBillCount++
			summary.OverdueBalanceTotal = roundMoney(summary.OverdueBalanceTotal + balance)
			switch {
			case overdueDays <= 30:
				bucket = "overdue_1_30"
			case overdueDays <= 60:
				bucket = "overdue_31_60"
			default:
				bucket = "overdue_61_up"
			}
		} else {
			summary.CurrentBalanceTotal = roundMoney(summary.CurrentBalanceTotal + balance)
		}
		summary.Aging[bucket] = roundMoney(summary.Aging[bucket] + balance)
		summary.Items = append(summary.Items, map[string]any{
			"id":           record.Header.ID,
			"number":       record.Header.Number,
			"vendor_name":  textValue(payload["vendor_name"]),
			"status":       record.Header.Status,
			"bill_date":    textValue(payload["bill_date"]),
			"due_date":     dueDate,
			"total_amount": roundMoney(numberValue(payload["total_amount"])),
			"paid_amount":  roundMoney(numberValue(payload["paid_amount"])),
			"credited":     roundMoney(numberValue(payload["credited_amount"])),
			"balance_due":  balance,
			"aging_bucket": bucket,
			"days_overdue": maxInt(dateDiffDays(dueDate, today), 0),
		})
	}
	return summary
}

func (s *ProcurementCoreService) VendorSummary(vendorID, fromDate, toDate string) VendorPayablesSummary {
	return s.VendorSummaryScoped("", "", vendorID, fromDate, toDate)
}

func (s *ProcurementCoreService) VendorSummaryScoped(organizationID, locationID, vendorID, fromDate, toDate string) VendorPayablesSummary {
	summary := VendorPayablesSummary{
		VendorID:   strings.TrimSpace(vendorID),
		OpenBills:  make([]map[string]any, 0),
		Activities: make([]map[string]any, 0),
	}
	if summary.VendorID == "" {
		return summary
	}
	for _, record := range s.documents.List() {
		if !matchesCommercialScope(record, organizationID, locationID) {
			continue
		}
		payload := clonedPayload(record.Body.Payload)
		if textValue(payload["vendor_id"]) != summary.VendorID {
			continue
		}
		dateValue, amountValue := procurementActivityValues(record, payload)
		if amountValue != 0 && withinDateRange(dateValue, fromDate, toDate) {
			summary.Activities = append(summary.Activities, map[string]any{
				"id":      record.Header.ID,
				"type":    record.Header.Type,
				"number":  record.Header.Number,
				"status":  record.Header.Status,
				"date":    dateValue,
				"amount":  amountValue,
				"counter": textValue(payload["source_vendor_bill_number"]),
			})
		}
		if record.Header.Type != "vendor_bill" {
			continue
		}
		summary.PaidAmountTotal = roundMoney(summary.PaidAmountTotal + roundMoney(numberValue(payload["paid_amount"])))
		summary.CreditedAmountTotal = roundMoney(summary.CreditedAmountTotal + roundMoney(numberValue(payload["credited_amount"])))
		balance := roundMoney(numberValue(payload["balance_due_amount"]))
		if balance > 0 && (record.Header.Status == "issued" || record.Header.Status == "partially_paid") {
			summary.OpenBillCount++
			summary.OpenBalanceTotal = roundMoney(summary.OpenBalanceTotal + balance)
			summary.OpenBills = append(summary.OpenBills, map[string]any{
				"id":           record.Header.ID,
				"number":       record.Header.Number,
				"status":       record.Header.Status,
				"bill_date":    textValue(payload["bill_date"]),
				"due_date":     textValue(payload["due_date"]),
				"total_amount": roundMoney(numberValue(payload["total_amount"])),
				"paid_amount":  roundMoney(numberValue(payload["paid_amount"])),
				"credited":     roundMoney(numberValue(payload["credited_amount"])),
				"balance_due":  balance,
			})
		}
	}
	sort.Slice(summary.OpenBills, func(i, j int) bool {
		left := textValue(summary.OpenBills[i]["due_date"])
		right := textValue(summary.OpenBills[j]["due_date"])
		if left != right {
			return left < right
		}
		return textValue(summary.OpenBills[i]["number"]) < textValue(summary.OpenBills[j]["number"])
	})
	sort.Slice(summary.Activities, func(i, j int) bool {
		left := textValue(summary.Activities[i]["date"])
		right := textValue(summary.Activities[j]["date"])
		if left != right {
			return left > right
		}
		return textValue(summary.Activities[i]["number"]) > textValue(summary.Activities[j]["number"])
	})
	return summary
}

func procurementActivityValues(record document.Record, payload map[string]any) (string, float64) {
	switch record.Header.Type {
	case "purchase_request":
		return textValue(payload["request_date"]), roundMoney(numberValue(payload["total_amount"]))
	case "purchase_order":
		return textValue(payload["order_date"]), roundMoney(numberValue(payload["total_amount"]))
	case "goods_receipt":
		return textValue(payload["receipt_date"]), roundMoney(numberValue(payload["total_amount"]))
	case "vendor_bill":
		return textValue(payload["bill_date"]), roundMoney(numberValue(payload["total_amount"]))
	case "payment_out":
		return textValue(payload["payment_date"]), roundMoney(numberValue(payload["amount_paid"]))
	case "vendor_credit_note":
		return textValue(payload["credit_date"]), roundMoney(numberValue(payload["total_amount"]))
	default:
		return "", 0
	}
}

func (s *ProcurementCoreService) createVendorBillFromSource(source document.Record, actorID, sourceType string) (document.Record, error) {
	payload := s.NormalizePayload(source.Header.Type, source.Body.Payload)
	now := time.Now().UTC()
	termDays := int(numberValue(payload["payment_term_days"]))
	if termDays <= 0 {
		termDays = 30
	}
	lines := s.vendorBillLinesFromSource(payload, sourceType)
	subtotalAmount, taxAmount, totalAmount := commercialLineTotals(lines)
	billPayload := map[string]any{
		"vendor_id":                    textValue(payload["vendor_id"]),
		"vendor_name":                  textValue(payload["vendor_name"]),
		"bill_date":                    now.Format("2006-01-02"),
		"due_date":                     now.AddDate(0, 0, termDays).Format("2006-01-02"),
		"currency_code":                firstNonEmptyString(textValue(payload["currency_code"]), source.Header.TotalAmount.Currency, "IDR"),
		"payment_term_days":            termDays,
		"tax_profile_code":             textValue(payload["tax_profile_code"]),
		"default_tax_code":             textValue(payload["default_tax_code"]),
		"source_purchase_order_id":     "",
		"source_purchase_order_number": "",
		"source_goods_receipt_id":      "",
		"source_goods_receipt_number":  "",
		"payable_account_code":         firstNonEmptyString(textValue(payload["payable_account_code"]), s.lookupVendorValue(textValue(payload["vendor_id"]), "payable_account_code")),
		"expense_account_code":         firstNonEmptyString(textValue(payload["expense_account_code"]), s.lookupVendorValue(textValue(payload["vendor_id"]), "expense_account_code")),
		"subtotal_amount":              subtotalAmount,
		"tax_amount":                   taxAmount,
		"total_amount":                 totalAmount,
		"paid_amount":                  0.0,
		"credited_amount":              0.0,
		"balance_due_amount":           totalAmount,
		"lines":                        lines,
		"landed_cost_lines":            recordList(payload["landed_cost_lines"]),
		"notes":                        textValue(payload["notes"]),
	}
	switch sourceType {
	case "purchase_order":
		billPayload["source_purchase_order_id"] = source.Header.ID
		billPayload["source_purchase_order_number"] = firstNonEmptyString(source.Header.Number, source.Header.ID)
	case "goods_receipt":
		billPayload["source_goods_receipt_id"] = source.Header.ID
		billPayload["source_goods_receipt_number"] = firstNonEmptyString(source.Header.Number, source.Header.ID)
		billPayload["source_purchase_order_id"] = textValue(payload["source_purchase_order_id"])
		billPayload["source_purchase_order_number"] = textValue(payload["source_purchase_order_number"])
		for idx, row := range recordList(payload["lines"]) {
			if idx >= len(lines) {
				break
			}
			lines[idx]["source_goods_receipt_id"] = source.Header.ID
			lines[idx]["source_goods_receipt_line_index"] = idx
			lines[idx]["receipt_unit_cost"] = firstPositiveNumber(row["effective_unit_cost"], row["unit_cost"], row["unit_price"])
			lines[idx]["receipt_total_cost"] = roundMoney(firstPositiveNumber(row["receipt_total_cost"], numberValue(row["receipt_qty"])*firstPositiveNumber(row["effective_unit_cost"], row["unit_cost"], row["unit_price"])))
		}
	}
	billPayload["lines"] = lines
	billPayload = s.normalizeProcurementLines(billPayload)
	billPayload["paid_amount"] = 0.0
	billPayload["credited_amount"] = 0.0
	billPayload["balance_due_amount"] = roundMoney(numberValue(billPayload["total_amount"]))
	record, err := s.documents.Create("vendor_bill", source.Header.OrganizationID, source.Header.LocationID, actorID, billPayload)
	if err != nil {
		return document.Record{}, err
	}
	linkType := "bill_for"
	if _, err := s.documents.AddLink(record.Header.ID, source.Header.ID, linkType, map[string]any{"source_type": sourceType}); err != nil {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(source.Header.ID, record.Header.ID, linkType, map[string]any{"generated_document_type": "vendor_bill"}); err != nil {
		return document.Record{}, err
	}
	created, err := s.documents.Get(record.Header.ID)
	if err != nil {
		return document.Record{}, err
	}
	s.refreshDocuments(created, source)
	return created, nil
}

func (s *ProcurementCoreService) vendorBillLinesFromSource(payload map[string]any, sourceType string) []map[string]any {
	rawLines := recordList(payload["lines"])
	switch sourceType {
	case "goods_receipt":
		rows := make([]map[string]any, 0, len(rawLines))
		for _, line := range rawLines {
			next := cloneMap(line)
			receiptQty := roundMoney(numberValue(next["receipt_qty"]))
			if receiptQty <= 0 {
				receiptQty = roundMoney(numberValue(next["quantity"]))
			}
			if receiptQty <= 0 {
				continue
			}
			next["quantity"] = receiptQty
			next["billed_qty"] = receiptQty
			rows = append(rows, next)
		}
		normalized := s.normalizeProcurementLines(map[string]any{
			"currency_code":        textValue(payload["currency_code"]),
			"default_tax_code":     textValue(payload["default_tax_code"]),
			"tax_profile_code":     textValue(payload["tax_profile_code"]),
			"expense_account_code": textValue(payload["expense_account_code"]),
			"lines":                rows,
		})
		return recordList(normalized["lines"])
	default:
		return incrementProcurementLineMetrics(rawLines, "billed_qty", "quantity")
	}
}

func incrementProcurementLineMetrics(lines []map[string]any, fieldKey, quantityKey string) []map[string]any {
	next := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		row := cloneMap(line)
		qty := roundMoney(numberValue(row[quantityKey]))
		if qty <= 0 {
			qty = 1
		}
		row[fieldKey] = qty
		if fieldKey == "ordered_qty" && roundMoney(numberValue(row["received_qty"])) == 0 {
			row["received_qty"] = 0.0
		}
		if fieldKey == "ordered_qty" && roundMoney(numberValue(row["billed_qty"])) == 0 {
			row["billed_qty"] = 0.0
		}
		next = append(next, row)
	}
	return next
}

func (s *ProcurementCoreService) handleReceivedGoodsReceipt(receipt document.Record, actorID string) error {
	payload := clonedPayload(receipt.Body.Payload)
	orderID := textValue(payload["source_purchase_order_id"])
	if orderID == "" {
		return shared.Validation("goods receipt source purchase order is required")
	}
	order, err := s.documents.Get(orderID)
	if err != nil {
		return err
	}
	orderPayload := clonedPayload(order.Body.Payload)
	linesByItem := map[string]float64{}
	for _, row := range recordList(payload["lines"]) {
		linesByItem[textValue(row["item_code"])] += roundMoney(numberValue(row["receipt_qty"]))
	}
	updatedLines := make([]map[string]any, 0)
	allReceived := true
	anyReceived := false
	for _, line := range recordList(orderPayload["lines"]) {
		next := cloneMap(line)
		itemCode := textValue(next["item_code"])
		orderedQty := roundMoney(numberValue(next["ordered_qty"]))
		if orderedQty <= 0 {
			orderedQty = roundMoney(numberValue(next["quantity"]))
		}
		receivedQty := roundMoney(numberValue(next["received_qty"]) + linesByItem[itemCode])
		if receivedQty > orderedQty {
			receivedQty = orderedQty
		}
		next["ordered_qty"] = orderedQty
		next["received_qty"] = receivedQty
		if receivedQty > 0 {
			anyReceived = true
		}
		if receivedQty < orderedQty {
			allReceived = false
		}
		updatedLines = append(updatedLines, next)
	}
	orderPayload["lines"] = updatedLines
	switch {
	case allReceived:
		order.Header.Status = "received"
	case anyReceived:
		order.Header.Status = "partially_received"
	default:
		order.Header.Status = "approved"
	}
	return s.saveMutatedDocument(order, actorID, orderPayload)
}

func (s *ProcurementCoreService) handleIssuedVendorBill(bill document.Record, actorID string) error {
	payload := clonedPayload(bill.Body.Payload)
	breakdown := s.vendorBillCostBreakdown(bill.Header.OrganizationID, bill.Header.LocationID, payload)
	payload["purchase_price_variance_amount"] = breakdown.varianceTotal
	totalAmount := roundMoney(numberValue(payload["total_amount"]))
	paidAmount := roundMoney(numberValue(payload["paid_amount"]))
	creditedAmount := roundMoney(numberValue(payload["credited_amount"]))
	payload["balance_due_amount"] = roundMoney(maxFloat(totalAmount-paidAmount-creditedAmount, 0))
	if err := s.updateDocumentPayload(bill, actorID, payload); err != nil {
		return err
	}
	postingLinked := s.hasPostingLink(bill, "vendor_bill_issue")
	costApplied := s.hasReceiptBackedCostAdjustmentApplied(bill.Header.ID)
	if orderID := textValue(payload["source_purchase_order_id"]); orderID != "" {
		_ = s.bumpOrderBilledQuantities(orderID, recordList(payload["lines"]), actorID, false)
	}
	if !postingLinked {
		postingPayload := map[string]any{
			"source_document_type": bill.Header.Type,
			"source_document_id":   bill.Header.ID,
			"posting_date":         time.Now().UTC().Format("2006-01-02"),
			"currency_code":        firstNonEmptyString(textValue(payload["currency_code"]), bill.Header.TotalAmount.Currency, "IDR"),
			"posting_rule_key":     "vendor_bill_issue_default",
			"total_amount":         totalAmount,
			"journal_lines":        s.vendorBillPostingLinesFromBreakdown(payload, breakdown),
			"notes":                fmt.Sprintf("Auto-posted from vendor bill %s", firstNonEmptyString(bill.Header.Number, bill.Header.ID)),
		}
		if s.finance != nil {
			if err := s.finance.ValidatePostingDateOpen(bill.Header.OrganizationID, bill.Header.LocationID, textValue(postingPayload["posting_date"])); err != nil {
				return err
			}
		}
		posting, err := s.documents.Create("ledger_posting", bill.Header.OrganizationID, bill.Header.LocationID, actorID, postingPayload)
		if err != nil {
			return err
		}
		if err := s.finalizeSystemPosting(posting, actorID, "posted"); err != nil {
			return err
		}
		if _, err := s.documents.AddLink(posting.Header.ID, bill.Header.ID, "posting_for", map[string]any{"posting_reason": "vendor_bill_issue"}); err != nil {
			return err
		}
		if _, err := s.documents.AddLink(bill.Header.ID, posting.Header.ID, "posting_for", map[string]any{"posting_reason": "vendor_bill_issue"}); err != nil {
			return err
		}
	}
	if !costApplied {
		if err := s.applyReceiptBackedCostAdjustments(bill.Header.ID, bill.Header.OrganizationID, bill.Header.LocationID, payload, actorID); err != nil {
			return err
		}
	}
	return nil
}

func (s *ProcurementCoreService) handlePaidOut(payment document.Record, actorID string) error {
	payload := clonedPayload(payment.Body.Payload)
	allocations := recordList(payload["allocations"])
	amountPaid := roundMoney(numberValue(payload["amount_paid"]))
	appliedAmount := 0.0
	for _, allocation := range allocations {
		appliedAmount += roundMoney(numberValue(allocation["amount"]))
	}
	payload["unapplied_amount"] = roundMoney(maxFloat(amountPaid-appliedAmount, 0))
	if err := s.updateDocumentPayload(payment, actorID, payload); err != nil {
		return err
	}
	for _, allocation := range allocations {
		if err := s.applyAllocationToBill(payment, textValue(allocation["bill_id"]), roundMoney(numberValue(allocation["amount"])), actorID); err != nil {
			return err
		}
	}
	if s.hasPostingLink(payment, "payment_out") {
		return nil
	}
	postingPayload := map[string]any{
		"source_document_type": payment.Header.Type,
		"source_document_id":   payment.Header.ID,
		"posting_date":         time.Now().UTC().Format("2006-01-02"),
		"currency_code":        firstNonEmptyString(textValue(payload["currency_code"]), payment.Header.TotalAmount.Currency, "IDR"),
		"posting_rule_key":     "payment_out_default",
		"total_amount":         amountPaid,
		"journal_lines":        s.paymentOutPostingLines(payload),
		"notes":                fmt.Sprintf("Auto-posted from payment out %s", firstNonEmptyString(payment.Header.Number, payment.Header.ID)),
	}
	if s.finance != nil {
		if err := s.finance.ValidatePostingDateOpen(payment.Header.OrganizationID, payment.Header.LocationID, textValue(postingPayload["posting_date"])); err != nil {
			return err
		}
	}
	posting, err := s.documents.Create("ledger_posting", payment.Header.OrganizationID, payment.Header.LocationID, actorID, postingPayload)
	if err != nil {
		return err
	}
	if err := s.finalizeSystemPosting(posting, actorID, "posted"); err != nil {
		return err
	}
	if _, err := s.documents.AddLink(posting.Header.ID, payment.Header.ID, "posting_for", map[string]any{"posting_reason": "payment_out"}); err != nil {
		return err
	}
	_, err = s.documents.AddLink(payment.Header.ID, posting.Header.ID, "posting_for", map[string]any{"posting_reason": "payment_out"})
	return err
}

func (s *ProcurementCoreService) handleIssuedVendorCredit(credit document.Record, actorID string) error {
	payload := clonedPayload(credit.Body.Payload)
	billID := textValue(payload["source_vendor_bill_id"])
	if billID == "" {
		return shared.Validation("vendor credit source bill is required")
	}
	bill, err := s.documents.Get(billID)
	if err != nil {
		return err
	}
	billPayload := clonedPayload(bill.Body.Payload)
	creditAmount := roundMoney(numberValue(payload["total_amount"]))
	creditableAmount := roundMoney(numberValue(billPayload["total_amount"]) - numberValue(billPayload["credited_amount"]))
	if creditAmount <= 0 {
		return shared.Validation("vendor credit amount must be greater than zero")
	}
	if creditAmount-creditableAmount > 0.0001 {
		return shared.Validation("vendor credit exceeds bill creditable balance")
	}
	creditedAmount := roundMoney(numberValue(billPayload["credited_amount"]) + creditAmount)
	paidAmount := roundMoney(numberValue(billPayload["paid_amount"]))
	billPayload["credited_amount"] = creditedAmount
	balanceDue := roundMoney(maxFloat(numberValue(billPayload["total_amount"])-paidAmount-creditedAmount, 0))
	billPayload["balance_due_amount"] = balanceDue
	switch {
	case balanceDue == 0 && paidAmount > 0:
		bill.Header.Status = "paid"
	case balanceDue == 0:
		bill.Header.Status = "cancelled"
	case paidAmount > 0:
		bill.Header.Status = "partially_paid"
	default:
		bill.Header.Status = "issued"
	}
	if err := s.saveMutatedDocument(bill, actorID, billPayload); err != nil {
		return err
	}
	if s.hasPostingLink(credit, "vendor_credit_issue") {
		return nil
	}
	postingPayload := map[string]any{
		"source_document_type": credit.Header.Type,
		"source_document_id":   credit.Header.ID,
		"posting_date":         time.Now().UTC().Format("2006-01-02"),
		"currency_code":        firstNonEmptyString(textValue(payload["currency_code"]), credit.Header.TotalAmount.Currency, "IDR"),
		"posting_rule_key":     "vendor_credit_issue_default",
		"total_amount":         creditAmount,
		"journal_lines":        reverseJournalLines(s.vendorBillPostingLines(payload)),
		"notes":                fmt.Sprintf("Auto-posted from vendor credit %s", firstNonEmptyString(credit.Header.Number, credit.Header.ID)),
	}
	if s.finance != nil {
		if err := s.finance.ValidatePostingDateOpen(credit.Header.OrganizationID, credit.Header.LocationID, textValue(postingPayload["posting_date"])); err != nil {
			return err
		}
	}
	posting, err := s.documents.Create("ledger_posting", credit.Header.OrganizationID, credit.Header.LocationID, actorID, postingPayload)
	if err != nil {
		return err
	}
	if err := s.finalizeSystemPosting(posting, actorID, "posted"); err != nil {
		return err
	}
	if _, err := s.documents.AddLink(posting.Header.ID, credit.Header.ID, "posting_for", map[string]any{"posting_reason": "vendor_credit_issue"}); err != nil {
		return err
	}
	_, err = s.documents.AddLink(credit.Header.ID, posting.Header.ID, "posting_for", map[string]any{"posting_reason": "vendor_credit_issue"})
	return err
}

func (s *ProcurementCoreService) applyAllocationToBill(payment document.Record, billID string, amount float64, actorID string) error {
	if amount <= 0 || billID == "" {
		return nil
	}
	bill, err := s.documents.Get(billID)
	if err != nil {
		return err
	}
	if bill.Header.Type != "vendor_bill" {
		return shared.Validation("allocation target must be a vendor bill")
	}
	payload := clonedPayload(bill.Body.Payload)
	totalAmount := roundMoney(numberValue(payload["total_amount"]))
	paidAmount := roundMoney(numberValue(payload["paid_amount"]) + amount)
	creditedAmount := roundMoney(numberValue(payload["credited_amount"]))
	if paidAmount+creditedAmount-totalAmount > 0.0001 {
		return shared.Validation("allocation exceeds vendor bill balance")
	}
	payload["paid_amount"] = paidAmount
	balanceDue := roundMoney(maxFloat(totalAmount-paidAmount-creditedAmount, 0))
	payload["balance_due_amount"] = balanceDue
	if balanceDue == 0 {
		bill.Header.Status = "paid"
	} else if paidAmount > 0 && (bill.Header.Status == "issued" || bill.Header.Status == "partially_paid") {
		bill.Header.Status = "partially_paid"
	}
	if err := s.saveMutatedDocument(bill, actorID, payload); err != nil {
		return err
	}
	if _, err := s.documents.AddLink(payment.Header.ID, bill.Header.ID, "payment_for", map[string]any{"allocated_amount": amount}); err != nil && !isConflict(err) {
		return err
	}
	if _, err := s.documents.AddLink(bill.Header.ID, payment.Header.ID, "payment_for", map[string]any{"allocated_amount": amount}); err != nil && !isConflict(err) {
		return err
	}
	return nil
}

func (s *ProcurementCoreService) reverseAllocationOnBill(payment document.Record, billID string, amount float64, actorID string) error {
	if amount <= 0 || billID == "" {
		return nil
	}
	bill, err := s.documents.Get(billID)
	if err != nil {
		return err
	}
	payload := clonedPayload(bill.Body.Payload)
	totalAmount := roundMoney(numberValue(payload["total_amount"]))
	paidAmount := roundMoney(numberValue(payload["paid_amount"]) - amount)
	if paidAmount < 0 {
		paidAmount = 0
	}
	creditedAmount := roundMoney(numberValue(payload["credited_amount"]))
	payload["paid_amount"] = paidAmount
	balanceDue := roundMoney(maxFloat(totalAmount-paidAmount-creditedAmount, 0))
	payload["balance_due_amount"] = balanceDue
	switch {
	case balanceDue == 0:
		bill.Header.Status = "paid"
	case paidAmount > 0:
		bill.Header.Status = "partially_paid"
	default:
		bill.Header.Status = "issued"
	}
	return s.saveMutatedDocument(bill, actorID, payload)
}

func (s *ProcurementCoreService) handleCancelledGoodsReceipt(receipt document.Record, actorID string) error {
	payload := clonedPayload(receipt.Body.Payload)
	orderID := textValue(payload["source_purchase_order_id"])
	if orderID == "" {
		return nil
	}
	order, err := s.documents.Get(orderID)
	if err != nil {
		return err
	}
	orderPayload := clonedPayload(order.Body.Payload)
	receivedByItem := map[string]float64{}
	for _, row := range recordList(payload["lines"]) {
		receivedByItem[textValue(row["item_code"])] += roundMoney(numberValue(row["receipt_qty"]))
	}
	updatedLines := make([]map[string]any, 0)
	allReceived := true
	anyReceived := false
	for _, line := range recordList(orderPayload["lines"]) {
		next := cloneMap(line)
		itemCode := textValue(next["item_code"])
		orderedQty := roundMoney(numberValue(next["ordered_qty"]))
		receivedQty := roundMoney(numberValue(next["received_qty"]) - receivedByItem[itemCode])
		if receivedQty < 0 {
			receivedQty = 0
		}
		next["received_qty"] = receivedQty
		if receivedQty > 0 {
			anyReceived = true
		}
		if receivedQty < orderedQty {
			allReceived = false
		}
		updatedLines = append(updatedLines, next)
	}
	orderPayload["lines"] = updatedLines
	switch {
	case allReceived && len(updatedLines) > 0:
		order.Header.Status = "received"
	case anyReceived:
		order.Header.Status = "partially_received"
	default:
		order.Header.Status = "approved"
	}
	return s.saveMutatedDocument(order, actorID, orderPayload)
}

func (s *ProcurementCoreService) handleCancelledVendorBill(bill document.Record, actorID string) error {
	if orderID := textValue(bill.Body.Payload["source_purchase_order_id"]); orderID != "" {
		_ = s.bumpOrderBilledQuantities(orderID, recordList(bill.Body.Payload["lines"]), actorID, true)
	}
	if s.hasPostingLink(bill, "vendor_bill_cancel_reversal") {
		return nil
	}
	return s.createReversalPosting(bill, actorID, "vendor_bill_issue", "vendor_bill_cancel_reversal", "vendor_bill_issue_reversal")
}

func (s *ProcurementCoreService) handleCancelledPaymentOut(payment document.Record, actorID string) error {
	payload := clonedPayload(payment.Body.Payload)
	for _, allocation := range recordList(payload["allocations"]) {
		if err := s.reverseAllocationOnBill(payment, textValue(allocation["bill_id"]), roundMoney(numberValue(allocation["amount"])), actorID); err != nil {
			return err
		}
	}
	payload["unapplied_amount"] = roundMoney(numberValue(payload["amount_paid"]))
	if err := s.updateDocumentPayload(payment, actorID, payload); err != nil {
		return err
	}
	if s.hasPostingLink(payment, "payment_out_reversal") {
		return nil
	}
	return s.createReversalPosting(payment, actorID, "payment_out", "payment_out_reversal", "payment_out_reversal")
}

func (s *ProcurementCoreService) handleCancelledVendorCredit(credit document.Record, actorID string) error {
	payload := clonedPayload(credit.Body.Payload)
	billID := textValue(payload["source_vendor_bill_id"])
	if billID != "" {
		bill, err := s.documents.Get(billID)
		if err != nil {
			return err
		}
		billPayload := clonedPayload(bill.Body.Payload)
		creditAmount := roundMoney(numberValue(payload["total_amount"]))
		creditedAmount := roundMoney(numberValue(billPayload["credited_amount"]) - creditAmount)
		if creditedAmount < 0 {
			creditedAmount = 0
		}
		paidAmount := roundMoney(numberValue(billPayload["paid_amount"]))
		billPayload["credited_amount"] = creditedAmount
		balanceDue := roundMoney(maxFloat(numberValue(billPayload["total_amount"])-paidAmount-creditedAmount, 0))
		billPayload["balance_due_amount"] = balanceDue
		switch {
		case balanceDue == 0:
			bill.Header.Status = "paid"
		case paidAmount > 0:
			bill.Header.Status = "partially_paid"
		default:
			bill.Header.Status = "issued"
		}
		if err := s.saveMutatedDocument(bill, actorID, billPayload); err != nil {
			return err
		}
	}
	if s.hasPostingLink(credit, "vendor_credit_reversal") {
		return nil
	}
	return s.createReversalPosting(credit, actorID, "vendor_credit_issue", "vendor_credit_reversal", "vendor_credit_issue_reversal")
}

func (s *ProcurementCoreService) bumpOrderBilledQuantities(orderID string, billLines []map[string]any, actorID string, reverse bool) error {
	order, err := s.documents.Get(strings.TrimSpace(orderID))
	if err != nil {
		return err
	}
	orderPayload := clonedPayload(order.Body.Payload)
	billedByItem := map[string]float64{}
	for _, line := range billLines {
		billedByItem[textValue(line["item_code"])] += roundMoney(numberValue(line["billed_qty"]))
		if billedByItem[textValue(line["item_code"])] == 0 {
			billedByItem[textValue(line["item_code"])] += roundMoney(numberValue(line["quantity"]))
		}
	}
	updatedLines := make([]map[string]any, 0)
	for _, line := range recordList(orderPayload["lines"]) {
		next := cloneMap(line)
		itemCode := textValue(next["item_code"])
		current := roundMoney(numberValue(next["billed_qty"]))
		delta := billedByItem[itemCode]
		if reverse {
			current -= delta
			if current < 0 {
				current = 0
			}
		} else {
			current += delta
		}
		next["billed_qty"] = current
		updatedLines = append(updatedLines, next)
	}
	orderPayload["lines"] = updatedLines
	return s.saveMutatedDocument(order, actorID, orderPayload)
}

type vendorBillCostBreakdown struct {
	debitByAccount         map[string]float64
	taxAmount              float64
	totalAmount            float64
	payableAccount         string
	varianceTotal          float64
	landedCostCapitalized  float64
	inventoryAdjustedTotal float64
}

func (s *ProcurementCoreService) vendorBillPostingLines(payload map[string]any) []map[string]any {
	breakdown := s.vendorBillCostBreakdown("", "", payload)
	return s.vendorBillPostingLinesFromBreakdown(payload, breakdown)
}

func (s *ProcurementCoreService) vendorBillPostingLinesFromBreakdown(payload map[string]any, breakdown vendorBillCostBreakdown) []map[string]any {
	lines := make([]map[string]any, 0, len(breakdown.debitByAccount)+2)
	for accountCode, amount := range breakdown.debitByAccount {
		if amount == 0 {
			continue
		}
		lines = append(lines, map[string]any{
			"account_code": accountCode,
			"description":  "Expense / Inventory / Variance",
			"debit":        amount,
			"credit":       0.0,
		})
	}
	if breakdown.taxAmount > 0 {
		lines = append(lines, map[string]any{
			"account_code": firstNonEmptyString(s.resolveTaxAccount(payload), s.postingConfig()["vendor_bill_tax_account_code"], "2100-TAX"),
			"description":  "Tax Receivable / Input Tax",
			"debit":        breakdown.taxAmount,
			"credit":       0.0,
		})
	}
	lines = append(lines, map[string]any{
		"account_code": breakdown.payableAccount,
		"description":  "Accounts Payable",
		"debit":        0.0,
		"credit":       breakdown.totalAmount,
	})
	return lines
}

func (s *ProcurementCoreService) vendorBillCostBreakdown(organizationID, locationID string, payload map[string]any) vendorBillCostBreakdown {
	taxAmount := roundMoney(numberValue(payload["tax_amount"]))
	totalAmount := roundMoney(numberValue(payload["total_amount"]))
	postingConfig := s.postingConfig()
	payableAccount := firstNonEmptyString(textValue(payload["payable_account_code"]), postingConfig["vendor_bill_payable_account_code"], "2000-AP")
	expenseAccount := firstNonEmptyString(s.resolveExpenseAccount(payload), postingConfig["vendor_bill_expense_account_code"], "5000-EXP")
	inventoryAccountDefault := firstNonEmptyString(postingConfig["vendor_bill_inventory_account_code"], "1200-INV")
	varianceAccount := firstNonEmptyString(postingConfig["purchase_price_variance_account_code"], "5100-PPV")
	debitByAccount := map[string]float64{}
	varianceTotal := 0.0
	receiptPayload := s.sourceReceiptPayload(payload)
	for _, line := range recordList(payload["lines"]) {
		lineSubtotal := roundMoney(numberValue(line["line_subtotal"]))
		allocatedLandedCost := roundMoney(numberValue(line["allocated_landed_cost"]))
		if lineSubtotal <= 0 && allocatedLandedCost <= 0 {
			continue
		}
		accountCode := expenseAccount
		if s.shouldCapitalizeVendorBillLine(payload, line) {
			accountCode = firstNonEmptyString(textValue(line["inventory_asset_account_code"]), inventoryAccountDefault)
			provisionalTotal, inventoryAdjustment, purchaseVariance := s.receiptBackedLineCostPlan(organizationID, locationID, line, receiptPayload)
			debitByAccount[accountCode] = roundMoney(debitByAccount[accountCode] + provisionalTotal + inventoryAdjustment)
			varianceTotal = roundMoney(varianceTotal + purchaseVariance)
			if purchaseVariance > 0 {
				debitByAccount[varianceAccount] = roundMoney(debitByAccount[varianceAccount] + purchaseVariance)
			} else if purchaseVariance < 0 {
				debitByAccount[accountCode] = roundMoney(debitByAccount[accountCode] + purchaseVariance)
				debitByAccount[varianceAccount] = roundMoney(debitByAccount[varianceAccount] - purchaseVariance)
			}
			continue
		}
		debitByAccount[accountCode] = roundMoney(debitByAccount[accountCode] + lineSubtotal + allocatedLandedCost)
	}
	if len(debitByAccount) == 0 {
		debitByAccount[expenseAccount] = roundMoney(numberValue(payload["subtotal_amount"]))
	}
	return vendorBillCostBreakdown{
		debitByAccount: debitByAccount,
		taxAmount:      taxAmount,
		totalAmount:    totalAmount,
		payableAccount: payableAccount,
		varianceTotal:  varianceTotal,
	}
}

func (s *ProcurementCoreService) shouldCapitalizeVendorBillLine(payload map[string]any, line map[string]any) bool {
	if !s.isInventoryPurchaseLine(line) {
		return false
	}
	return strings.TrimSpace(textValue(payload["source_goods_receipt_id"])) != ""
}

func (s *ProcurementCoreService) sourceReceiptPayload(payload map[string]any) map[string]any {
	receiptID := strings.TrimSpace(textValue(payload["source_goods_receipt_id"]))
	if receiptID == "" {
		return nil
	}
	receipt, err := s.documents.Get(receiptID)
	if err != nil || receipt.Header.Type != "goods_receipt" {
		return nil
	}
	return clonedPayload(receipt.Body.Payload)
}

func (s *ProcurementCoreService) receiptBackedLineCostPlan(organizationID, locationID string, line map[string]any, receiptPayload map[string]any) (float64, float64, float64) {
	quantity := roundMoney(numberValue(line["quantity"]))
	if quantity <= 0 {
		quantity = roundMoney(numberValue(line["billed_qty"]))
	}
	if quantity <= 0 {
		quantity = 1
	}
	provisionalUnit := firstPositiveNumber(line["receipt_unit_cost"])
	if provisionalUnit <= 0 {
		provisionalUnit = s.lookupReceiptUnitCost(line, receiptPayload)
	}
	billedTotal := roundMoney(numberValue(line["line_subtotal"]) + numberValue(line["allocated_landed_cost"]))
	provisionalTotal := roundMoney(quantity * provisionalUnit)
	varianceTotal := roundMoney(billedTotal - provisionalTotal)
	if varianceTotal == 0 {
		return provisionalTotal, 0, 0
	}
	onHandQty := quantity
	if s.inventory != nil {
		onHandQty = minFloat(quantity, s.inventory.CurrentOnHandQuantity(
			organizationID,
			locationID,
			textValue(line["item_code"]),
			textValue(line["warehouse_code"]),
			textValue(line["batch_code"]),
		))
	}
	inventoryAdjustment := 0.0
	if quantity > 0 && onHandQty > 0 {
		inventoryAdjustment = roundMoney(varianceTotal * onHandQty / quantity)
	}
	return provisionalTotal, inventoryAdjustment, roundMoney(varianceTotal - inventoryAdjustment)
}

func (s *ProcurementCoreService) lookupReceiptUnitCost(line map[string]any, receiptPayload map[string]any) float64 {
	if receiptPayload == nil {
		return 0
	}
	rows := recordList(receiptPayload["lines"])
	indexValue := line["source_goods_receipt_line_index"]
	if textValue(indexValue) != "" || numberValue(indexValue) != 0 {
		index := int(numberValue(indexValue))
		if index >= 0 && index < len(rows) {
			return roundMoney(firstPositiveNumber(rows[index]["effective_unit_cost"], rows[index]["unit_cost"], rows[index]["unit_price"]))
		}
	}
	itemCode := textValue(line["item_code"])
	warehouseCode := textValue(line["warehouse_code"])
	batchCode := textValue(line["batch_code"])
	for _, row := range rows {
		if textValue(row["item_code"]) != itemCode {
			continue
		}
		if warehouseCode != "" && textValue(row["warehouse_code"]) != warehouseCode {
			continue
		}
		if batchCode != "" && textValue(row["batch_code"]) != batchCode {
			continue
		}
		return roundMoney(firstPositiveNumber(row["effective_unit_cost"], row["unit_cost"], row["unit_price"]))
	}
	return 0
}

func (s *ProcurementCoreService) applyReceiptBackedCostAdjustments(billID, organizationID, locationID string, payload map[string]any, actorID string) error {
	if s.inventory == nil || strings.TrimSpace(textValue(payload["source_goods_receipt_id"])) == "" {
		return nil
	}
	receiptPayload := s.sourceReceiptPayload(payload)
	for _, line := range recordList(payload["lines"]) {
		if !s.shouldCapitalizeVendorBillLine(payload, line) {
			continue
		}
		_, inventoryAdjustment, _ := s.receiptBackedLineCostPlan(organizationID, locationID, line, receiptPayload)
		if inventoryAdjustment == 0 {
			continue
		}
		if err := s.inventory.ApplyCostAdjustment(actorID, organizationID, locationID, map[string]any{
			"item_code":            textValue(line["item_code"]),
			"warehouse_code":       textValue(line["warehouse_code"]),
			"batch_code":           textValue(line["batch_code"]),
			"source_document_type": "vendor_bill",
			"source_document_id":   billID,
			"event_type":           "vendor_bill_variance",
			"quantity_basis":       roundMoney(numberValue(line["quantity"])),
			"unit_cost":            0.0,
			"total_cost":           inventoryAdjustment,
			"currency_code":        firstNonEmptyString(textValue(payload["currency_code"]), "IDR"),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *ProcurementCoreService) hasReceiptBackedCostAdjustmentApplied(billID string) bool {
	if s.models == nil || strings.TrimSpace(billID) == "" {
		return false
	}
	items, _, err := s.models.List("inventory_cost_layer", model.Query{
		Filters: map[string]string{
			"source_document_type": "vendor_bill",
			"source_document_id":   strings.TrimSpace(billID),
			"event_type":           "vendor_bill_variance",
		},
		Page:     1,
		PageSize: 1,
	})
	return err == nil && len(items) > 0
}

func (s *ProcurementCoreService) paymentOutPostingLines(payload map[string]any) []map[string]any {
	totalAmount := roundMoney(numberValue(payload["amount_paid"]))
	postingConfig := s.postingConfig()
	payableAccount := firstNonEmptyString(textValue(payload["payable_account_code"]), postingConfig["payment_out_payable_account_code"], postingConfig["vendor_bill_payable_account_code"], "2000-AP")
	clearingAccount := firstNonEmptyString(
		textValue(payload["clearing_account_code"]),
		s.lookupAccountCode("payment_method", "code", textValue(payload["payment_method_code"]), "clearing_account_code"),
		postingConfig["payment_out_clearing_account_code"],
		"1000-CASH",
	)
	return []map[string]any{{
		"account_code": payableAccount,
		"description":  "Accounts Payable",
		"debit":        totalAmount,
		"credit":       0.0,
	}, {
		"account_code": clearingAccount,
		"description":  "Cash / Clearing",
		"debit":        0.0,
		"credit":       totalAmount,
	}}
}

func (s *ProcurementCoreService) postingConfig() map[string]string {
	defaults := map[string]string{
		"vendor_bill_payable_account_code":     "2000-AP",
		"vendor_bill_expense_account_code":     "5000-EXP",
		"vendor_bill_inventory_account_code":   "1200-INV",
		"purchase_price_variance_account_code": "5100-PPV",
		"vendor_bill_tax_account_code":         "2100-TAX",
		"payment_out_clearing_account_code":    "1000-CASH",
		"payment_out_payable_account_code":     "2000-AP",
		"vendor_credit_payable_account_code":   "2000-AP",
		"vendor_credit_expense_account_code":   "5000-EXP",
		"vendor_credit_tax_account_code":       "2100-TAX",
		"goods_receipt_clearing_account_code":  "2050-GRIR",
	}
	if s.config == nil {
		return defaults
	}
	value, ok := s.config.Resolve("procurement.posting", "", "")
	if !ok {
		return defaults
	}
	for key := range defaults {
		if current := textValue(value.Value[key]); current != "" {
			defaults[key] = current
		}
	}
	return defaults
}

func (s *ProcurementCoreService) isInventoryPurchaseLine(line map[string]any) bool {
	itemCode := textValue(line["item_code"])
	if itemCode == "" || s.models == nil {
		return false
	}
	items, _, err := s.models.List("commercial_item", model.Query{
		Filters:  map[string]string{"sku": itemCode},
		Page:     1,
		PageSize: 1,
	})
	if err != nil || len(items) == 0 {
		return false
	}
	return boolValue(items[0].Values["inventory_enabled"])
}

func (s *ProcurementCoreService) normalizeProcurementLines(payload map[string]any) map[string]any {
	next := document.NormalizePayload(cloneMap(payload))
	next = s.applyVendorDefaults(next)
	profileCode := textValue(next["tax_profile_code"])
	profileTaxCode := s.lookupAccountCode("commercial_tax_profile", "code", profileCode, "default_tax_code")
	profileTaxMode := strings.ToLower(s.lookupAccountCode("commercial_tax_profile", "code", profileCode, "price_tax_mode"))
	profileTermDays := s.lookupNumberValue("commercial_tax_profile", "code", profileCode, "payment_term_days")
	if textValue(next["default_tax_code"]) == "" && profileTaxCode != "" {
		next["default_tax_code"] = profileTaxCode
	}
	if numberValue(next["payment_term_days"]) == 0 && profileTermDays > 0 {
		next["payment_term_days"] = profileTermDays
	}
	if strings.TrimSpace(textValue(next["due_date"])) == "" {
		baseDate := firstNonEmptyString(textValue(next["bill_date"]), textValue(next["order_date"]), textValue(next["request_date"]))
		if baseDate != "" && numberValue(next["payment_term_days"]) > 0 {
			if dueDate, ok := addDaysToDate(baseDate, int(numberValue(next["payment_term_days"]))); ok {
				next["due_date"] = dueDate
			}
		}
	}
	rows := recordList(next["lines"])
	landedCostRows, landedCostTotal := normalizeLandedCostLines(recordList(next["landed_cost_lines"]))
	next["landed_cost_lines"] = landedCostRows
	normalizedRows := make([]map[string]any, 0, len(rows))
	subtotalAmount := 0.0
	taxAmount := 0.0
	totalAmount := 0.0
	for _, row := range rows {
		normalized := cloneMap(row)
		defaultTaxCode := textValue(next["default_tax_code"])
		if resolvedItemCode := s.resolveVariantItemCode(textValue(normalized["product_code"]), textValue(normalized["variant_signature"])); resolvedItemCode != "" {
			normalized["item_code"] = resolvedItemCode
		}
		itemCode := textValue(normalized["item_code"])
		if textValue(normalized["description"]) == "" {
			normalized["description"] = firstNonEmptyString(
				s.lookupAccountCode("commercial_item", "sku", itemCode, "description"),
				s.lookupAccountCode("commercial_item", "sku", itemCode, "name"),
			)
		}
		if textValue(normalized["uom_code"]) == "" {
			normalized["uom_code"] = s.lookupAccountCode("commercial_item", "sku", itemCode, "uom_code")
		}
		if numberValue(normalized["unit_price"]) == 0 {
			if price := s.lookupNumberValue("commercial_item", "sku", itemCode, "base_price"); price > 0 {
				normalized["unit_price"] = price
			}
		}
		if textValue(normalized["tax_code"]) == "" {
			normalized["tax_code"] = firstNonEmptyString(defaultTaxCode, s.lookupAccountCode("commercial_item", "sku", itemCode, "tax_code"))
		}
		if textValue(normalized["expense_account_code"]) == "" {
			normalized["expense_account_code"] = firstNonEmptyString(textValue(next["expense_account_code"]), s.lookupVendorValue(textValue(next["vendor_id"]), "expense_account_code"))
		}
		if textValue(normalized["inventory_asset_account_code"]) == "" {
			normalized["inventory_asset_account_code"] = s.lookupAccountCode("commercial_item", "sku", itemCode, "inventory_asset_account_code")
		}
		taxMode := strings.ToLower(s.lookupAccountCode("commercial_tax_code", "code", textValue(normalized["tax_code"]), "mode"))
		if taxMode == "" {
			taxMode = firstNonEmptyString(profileTaxMode, textValue(normalized["tax_mode"]), "exclusive")
		}
		if numberValue(normalized["tax_rate"]) == 0 {
			if rate := s.lookupNumberValue("commercial_tax_code", "code", textValue(normalized["tax_code"]), "rate_percent"); rate > 0 {
				normalized["tax_rate"] = rate
			}
		}
		if textValue(normalized["tax_account_code"]) == "" {
			normalized["tax_account_code"] = s.lookupAccountCode("commercial_tax_code", "code", textValue(normalized["tax_code"]), "tax_account_code")
		}
		quantity := numberValue(normalized["quantity"])
		if quantity == 0 {
			quantity = 1
		}
		unitPrice := numberValue(normalized["unit_price"])
		discountAmount := numberValue(normalized["discount_amount"])
		taxRate := numberValue(normalized["tax_rate"])
		grossAmount := roundMoney(maxFloat(quantity*unitPrice-discountAmount, 0))
		lineSubtotal, lineTax, lineTotal := commercialTaxBreakdown(grossAmount, taxRate, taxMode)
		normalized["quantity"] = quantity
		normalized["ordered_qty"] = maxFloat(numberValue(normalized["ordered_qty"]), quantity)
		normalized["unit_price"] = unitPrice
		normalized["discount_amount"] = discountAmount
		normalized["tax_rate"] = taxRate
		normalized["tax_mode"] = taxMode
		normalized["allocated_landed_cost"] = roundMoney(numberValue(normalized["allocated_landed_cost"]))
		normalized["unit_landed_cost"] = roundMoney(numberValue(normalized["unit_landed_cost"]))
		normalized["effective_unit_cost"] = roundMoney(firstPositiveNumber(normalized["effective_unit_cost"], unitPrice))
		normalized["line_subtotal"] = lineSubtotal
		normalized["tax_amount"] = lineTax
		normalized["line_total"] = lineTotal
		subtotalAmount += lineSubtotal
		taxAmount += lineTax
		totalAmount += lineTotal
		normalizedRows = append(normalizedRows, normalized)
	}
	normalizedRows = s.allocateLandedCostToLines(normalizedRows, landedCostRows, "quantity")
	next["lines"] = normalizedRows
	subtotalAmount, taxAmount, totalAmount = commercialLineTotals(normalizedRows)
	next["landed_cost_amount"] = landedCostTotal
	next["subtotal_amount"] = roundMoney(subtotalAmount + landedCostTotal)
	next["tax_amount"] = roundMoney(taxAmount)
	next["total_amount"] = roundMoney(totalAmount + landedCostTotal)
	if strings.TrimSpace(textValue(next["currency_code"])) == "" {
		next["currency_code"] = "IDR"
	}
	return next
}

func (s *ProcurementCoreService) resolveVariantItemCode(productCode, variantSignature string) string {
	productCode = strings.TrimSpace(productCode)
	variantSignature = strings.TrimSpace(variantSignature)
	if productCode == "" || variantSignature == "" {
		return ""
	}
	items, _, err := s.models.List("commercial_item", model.Query{
		Filters: map[string]string{
			"product_code":      productCode,
			"variant_signature": variantSignature,
		},
		PageSize: 2,
	})
	if err != nil || len(items) == 0 {
		return ""
	}
	return textValue(items[0].Values["sku"])
}

func (s *ProcurementCoreService) normalizeReceiptLines(payload map[string]any) map[string]any {
	next := document.NormalizePayload(cloneMap(payload))
	rows := recordList(next["lines"])
	landedCostRows, landedCostTotal := normalizeLandedCostLines(recordList(next["landed_cost_lines"]))
	next["landed_cost_lines"] = landedCostRows
	totalAmount := 0.0
	normalizedRows := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		normalized := cloneMap(row)
		receiptQty := roundMoney(numberValue(normalized["receipt_qty"]))
		if receiptQty <= 0 {
			receiptQty = roundMoney(numberValue(normalized["ordered_qty"]) - numberValue(normalized["cumulative_received_qty"]))
		}
		if receiptQty < 0 {
			receiptQty = 0
		}
		unitPrice := roundMoney(numberValue(normalized["unit_price"]))
		discountAmount := roundMoney(numberValue(normalized["discount_amount"]))
		taxRate := roundMoney(numberValue(normalized["tax_rate"]))
		taxMode := firstNonEmptyString(textValue(normalized["tax_mode"]), "exclusive")
		grossAmount := roundMoney(maxFloat(receiptQty*unitPrice-discountAmount, 0))
		lineSubtotal, lineTax, lineTotal := commercialTaxBreakdown(grossAmount, taxRate, taxMode)
		normalized["receipt_qty"] = receiptQty
		normalized["line_subtotal"] = lineSubtotal
		normalized["tax_amount"] = lineTax
		normalized["line_total"] = lineTotal
		normalized["allocated_landed_cost"] = roundMoney(numberValue(normalized["allocated_landed_cost"]))
		normalized["unit_landed_cost"] = roundMoney(numberValue(normalized["unit_landed_cost"]))
		normalized["effective_unit_cost"] = roundMoney(firstPositiveNumber(normalized["effective_unit_cost"], unitPrice))
		totalAmount += lineTotal
		normalizedRows = append(normalizedRows, normalized)
	}
	normalizedRows = s.allocateLandedCostToLines(normalizedRows, landedCostRows, "receipt_qty")
	next["lines"] = normalizedRows
	totalAmount = landedCostTotal
	for _, row := range normalizedRows {
		totalAmount = roundMoney(totalAmount + numberValue(row["line_total"]))
	}
	next["landed_cost_amount"] = landedCostTotal
	next["total_amount"] = roundMoney(totalAmount)
	if strings.TrimSpace(textValue(next["currency_code"])) == "" {
		next["currency_code"] = "IDR"
	}
	return next
}

func (s *ProcurementCoreService) normalizeBillAllocations(payload map[string]any) map[string]any {
	next := document.NormalizePayload(cloneMap(payload))
	rows := recordList(next["allocations"])
	normalizedRows := make([]map[string]any, 0, len(rows))
	appliedAmount := 0.0
	for _, row := range rows {
		normalized := cloneMap(row)
		billID := textValue(normalized["bill_id"])
		if billID != "" {
			if bill, err := s.documents.Get(billID); err == nil && bill.Header.Type == "vendor_bill" {
				if textValue(normalized["bill_number"]) == "" {
					normalized["bill_number"] = firstNonEmptyString(bill.Header.Number, bill.Header.ID)
				}
				if amount := roundMoney(numberValue(normalized["amount"])); amount == 0 {
					openAmount := roundMoney(numberValue(bill.Body.Payload["balance_due_amount"]))
					if openAmount > 0 {
						normalized["amount"] = openAmount
					}
				}
			}
		}
		amount := roundMoney(numberValue(normalized["amount"]))
		normalized["amount"] = amount
		appliedAmount += amount
		normalizedRows = append(normalizedRows, normalized)
	}
	amountPaid := roundMoney(numberValue(next["amount_paid"]))
	if amountPaid <= 0 && appliedAmount > 0 {
		amountPaid = roundMoney(appliedAmount)
	}
	next["allocations"] = normalizedRows
	next["amount_paid"] = amountPaid
	next["unapplied_amount"] = roundMoney(maxFloat(amountPaid-appliedAmount, 0))
	if methodCode := textValue(next["payment_method_code"]); methodCode != "" && textValue(next["clearing_account_code"]) == "" {
		next["clearing_account_code"] = s.lookupAccountCode("payment_method", "code", methodCode, "clearing_account_code")
	}
	return next
}

func normalizeLandedCostLines(rows []map[string]any) ([]map[string]any, float64) {
	normalized := make([]map[string]any, 0, len(rows))
	total := 0.0
	for _, row := range rows {
		next := cloneMap(row)
		amount := roundMoney(numberValue(next["amount"]))
		if amount <= 0 {
			continue
		}
		next["amount"] = amount
		next["allocation_basis"] = firstNonEmptyString(textValue(next["allocation_basis"]), "line_value")
		total = roundMoney(total + amount)
		normalized = append(normalized, next)
	}
	return normalized, total
}

func (s *ProcurementCoreService) allocateLandedCostToLines(lines []map[string]any, landedCostRows []map[string]any, qtyKey string) []map[string]any {
	totalLanded := 0.0
	for _, row := range landedCostRows {
		totalLanded = roundMoney(totalLanded + numberValue(row["amount"]))
	}
	if totalLanded <= 0 || len(lines) == 0 {
		for idx := range lines {
			lines[idx]["allocated_landed_cost"] = roundMoney(numberValue(lines[idx]["allocated_landed_cost"]))
			lines[idx]["unit_landed_cost"] = roundMoney(numberValue(lines[idx]["unit_landed_cost"]))
			baseUnit := firstPositiveNumber(lines[idx]["unit_price"], lines[idx]["unit_cost"], lines[idx]["receipt_unit_cost"])
			lines[idx]["effective_unit_cost"] = roundMoney(baseUnit + numberValue(lines[idx]["unit_landed_cost"]))
		}
		return lines
	}
	type eligibleLine struct {
		index int
		base  float64
	}
	eligible := make([]eligibleLine, 0)
	baseTotal := 0.0
	for idx, line := range lines {
		if !s.isInventoryPurchaseLine(line) {
			continue
		}
		base := roundMoney(numberValue(line["line_subtotal"]))
		if base <= 0 {
			base = roundMoney(numberValue(line[qtyKey]) * firstPositiveNumber(line["unit_price"], line["unit_cost"], line["receipt_unit_cost"]))
		}
		if base <= 0 {
			continue
		}
		baseTotal = roundMoney(baseTotal + base)
		eligible = append(eligible, eligibleLine{index: idx, base: base})
	}
	if len(eligible) == 0 || baseTotal <= 0 {
		return lines
	}
	remaining := totalLanded
	for pos, item := range eligible {
		allocated := 0.0
		if pos == len(eligible)-1 {
			allocated = remaining
		} else {
			allocated = roundMoney(totalLanded * item.base / baseTotal)
			remaining = roundMoney(remaining - allocated)
		}
		qty := roundMoney(numberValue(lines[item.index][qtyKey]))
		unitLanded := 0.0
		if qty > 0 {
			unitLanded = roundMoney(allocated / qty)
		}
		baseUnit := firstPositiveNumber(lines[item.index]["unit_price"], lines[item.index]["unit_cost"], lines[item.index]["receipt_unit_cost"])
		lines[item.index]["allocated_landed_cost"] = allocated
		lines[item.index]["unit_landed_cost"] = unitLanded
		lines[item.index]["effective_unit_cost"] = roundMoney(baseUnit + unitLanded)
	}
	return lines
}

func (s *ProcurementCoreService) applyVendorDefaults(payload map[string]any) map[string]any {
	next := document.NormalizePayload(cloneMap(payload))
	vendorID := textValue(next["vendor_id"])
	if vendorID == "" {
		return next
	}
	if textValue(next["vendor_name"]) == "" {
		next["vendor_name"] = firstNonEmptyString(s.lookupVendorValue(vendorID, "vendor_name"), s.lookupVendorValue(vendorID, "display_name"))
	}
	if textValue(next["currency_code"]) == "" {
		next["currency_code"] = s.lookupVendorValue(vendorID, "currency_code")
	}
	if textValue(next["tax_profile_code"]) == "" {
		next["tax_profile_code"] = s.lookupVendorValue(vendorID, "tax_profile_code")
	}
	if numberValue(next["payment_term_days"]) == 0 {
		if days := s.lookupVendorNumber(vendorID, "payment_term_days"); days > 0 {
			next["payment_term_days"] = days
		}
	}
	if textValue(next["payable_account_code"]) == "" {
		next["payable_account_code"] = s.lookupVendorValue(vendorID, "payable_account_code")
	}
	if textValue(next["expense_account_code"]) == "" {
		next["expense_account_code"] = s.lookupVendorValue(vendorID, "expense_account_code")
	}
	if textValue(next["default_payment_method_code"]) == "" {
		next["default_payment_method_code"] = s.lookupVendorValue(vendorID, "default_payment_method_code")
	}
	return next
}

func (s *ProcurementCoreService) resolveExpenseAccount(payload map[string]any) string {
	if account := textValue(payload["expense_account_code"]); account != "" {
		return account
	}
	lines := recordList(payload["lines"])
	for _, line := range lines {
		if account := textValue(line["expense_account_code"]); account != "" {
			return account
		}
	}
	return s.lookupVendorValue(textValue(payload["vendor_id"]), "expense_account_code")
}

func (s *ProcurementCoreService) resolveTaxAccount(payload map[string]any) string {
	if account := textValue(payload["tax_account_code"]); account != "" {
		return account
	}
	if account := s.lookupAccountCode("commercial_tax_code", "code", textValue(payload["tax_code"]), "tax_account_code"); account != "" {
		return account
	}
	for _, line := range recordList(payload["lines"]) {
		if account := textValue(line["tax_account_code"]); account != "" {
			return account
		}
		if account := s.lookupAccountCode("commercial_tax_code", "code", textValue(line["tax_code"]), "tax_account_code"); account != "" {
			return account
		}
	}
	return ""
}

func (s *ProcurementCoreService) lookupVendorValue(vendorID, key string) string {
	if s.models == nil || strings.TrimSpace(vendorID) == "" {
		return ""
	}
	record, err := s.models.Get("vendor_profile", vendorID)
	if err != nil {
		return ""
	}
	if current := textValue(record.Values[key]); current != "" {
		return current
	}
	partyID := textValue(record.Values["party_id"])
	if partyID == "" {
		return ""
	}
	switch key {
	case "vendor_name", "display_name":
		return firstNonEmptyString(s.lookupModelValueByID("party", partyID, "display_name"), s.lookupModelValueByID("party", partyID, "name"))
	case "currency_code":
		return s.lookupModelValueByID("party", partyID, "currency_code")
	case "tax_profile_code":
		return s.lookupModelValueByID("party", partyID, "tax_profile_code")
	default:
		return ""
	}
}

func (s *ProcurementCoreService) lookupVendorNumber(vendorID, key string) float64 {
	if s.models == nil || strings.TrimSpace(vendorID) == "" {
		return 0
	}
	record, err := s.models.Get("vendor_profile", vendorID)
	if err != nil {
		return 0
	}
	if value := roundMoney(numberValue(record.Values[key])); value > 0 {
		return value
	}
	partyID := textValue(record.Values["party_id"])
	if partyID == "" {
		return 0
	}
	switch key {
	case "payment_term_days":
		return s.lookupModelNumberByID("party", partyID, "payment_term_days")
	default:
		return 0
	}
}

func (s *ProcurementCoreService) lookupAccountCode(modelKey, filterKey, filterValue, valueKey string) string {
	if s.models == nil || strings.TrimSpace(filterValue) == "" {
		return ""
	}
	items, _, err := s.models.List(modelKey, model.Query{
		Filters:  lookupFilters(filterKey, filterValue),
		Page:     1,
		PageSize: 1,
	})
	if err != nil || len(items) == 0 {
		return ""
	}
	return textValue(items[0].Values[valueKey])
}

func (s *ProcurementCoreService) lookupNumberValue(modelKey, filterKey, filterValue, valueKey string) float64 {
	if s.models == nil || strings.TrimSpace(filterValue) == "" {
		return 0
	}
	items, _, err := s.models.List(modelKey, model.Query{
		Filters:  lookupFilters(filterKey, filterValue),
		Page:     1,
		PageSize: 1,
	})
	if err != nil || len(items) == 0 {
		return 0
	}
	return roundMoney(numberValue(items[0].Values[valueKey]))
}

func (s *ProcurementCoreService) lookupModelValueByID(modelKey, id, valueKey string) string {
	if s.models == nil || strings.TrimSpace(id) == "" {
		return ""
	}
	record, err := s.models.Get(modelKey, id)
	if err != nil {
		return ""
	}
	return textValue(record.Values[valueKey])
}

func (s *ProcurementCoreService) lookupModelNumberByID(modelKey, id, valueKey string) float64 {
	if s.models == nil || strings.TrimSpace(id) == "" {
		return 0
	}
	record, err := s.models.Get(modelKey, id)
	if err != nil {
		return 0
	}
	return roundMoney(numberValue(record.Values[valueKey]))
}

func (s *ProcurementCoreService) updateDocumentPayload(record document.Record, actorID string, payload map[string]any) error {
	return s.saveMutatedDocument(record, actorID, payload)
}

func (s *ProcurementCoreService) saveMutatedDocument(record document.Record, actorID string, payload map[string]any) error {
	record.Body.Payload = document.NormalizePayload(payload)
	record.Body.ContentHash = document.ContentHash(record.Body.Payload)
	record.Header.Version++
	record.Header.ETag = fmt.Sprintf("%s:%d", record.Header.ID, record.Header.Version)
	record.Header.UpdatedBy = actorID
	record.Header.UpdatedAt = time.Now().UTC()
	record.Header.TotalAmount = shared.Money{
		Currency:    firstNonEmptyString(textValue(record.Body.Payload["currency_code"]), record.Header.TotalAmount.Currency, "IDR"),
		AmountMinor: moneyMinor(derivedProcurementRecordAmount(record.Body.Payload)),
	}
	if err := s.documents.Save(record); err != nil {
		return err
	}
	s.refreshDocuments(record)
	return nil
}

func (s *ProcurementCoreService) finalizeSystemPosting(record document.Record, actorID, status string) error {
	record.Header.Status = status
	record.Header.TotalAmount = shared.Money{
		Currency:    firstNonEmptyString(textValue(record.Body.Payload["currency_code"]), record.Header.TotalAmount.Currency, "IDR"),
		AmountMinor: moneyMinor(derivedProcurementRecordAmount(record.Body.Payload)),
	}
	record.Header.Version++
	record.Header.ETag = fmt.Sprintf("%s:%d", record.Header.ID, record.Header.Version)
	record.Header.UpdatedBy = actorID
	record.Header.UpdatedAt = time.Now().UTC()
	if err := s.documents.Save(record); err != nil {
		return err
	}
	s.refreshDocuments(record)
	return nil
}

func derivedProcurementRecordAmount(payload map[string]any) float64 {
	if total := roundMoney(numberValue(payload["total_amount"])); total > 0 {
		return total
	}
	return roundMoney(numberValue(payload["amount_paid"]))
}

func (s *ProcurementCoreService) createReversalPosting(source document.Record, actorID, originalReason, reversalReason, postingRuleKey string) error {
	originalPosting, ok := s.findPostingForReason(source, originalReason)
	if !ok {
		return nil
	}
	lines := reverseJournalLines(recordList(originalPosting.Body.Payload["journal_lines"]))
	payload := clonedPayload(originalPosting.Body.Payload)
	payload["source_document_type"] = source.Header.Type
	payload["source_document_id"] = source.Header.ID
	payload["posting_date"] = time.Now().UTC().Format("2006-01-02")
	payload["posting_rule_key"] = postingRuleKey
	payload["journal_lines"] = lines
	payload["notes"] = fmt.Sprintf("Reversal of %s", firstNonEmptyString(originalPosting.Header.Number, originalPosting.Header.ID))
	payload["total_amount"] = roundMoney(numberValue(originalPosting.Body.Payload["total_amount"]))
	if s.finance != nil {
		if err := s.finance.ValidatePostingDateOpen(source.Header.OrganizationID, source.Header.LocationID, textValue(payload["posting_date"])); err != nil {
			return err
		}
	}
	reversal, err := s.documents.Create("ledger_posting", source.Header.OrganizationID, source.Header.LocationID, actorID, payload)
	if err != nil {
		return err
	}
	if err := s.finalizeSystemPosting(reversal, actorID, "posted"); err != nil {
		return err
	}
	if _, err := s.documents.AddLink(reversal.Header.ID, source.Header.ID, "posting_for", map[string]any{
		"posting_reason": reversalReason,
		"reversal_of":    originalPosting.Header.ID,
	}); err != nil {
		return err
	}
	if _, err := s.documents.AddLink(source.Header.ID, reversal.Header.ID, "posting_for", map[string]any{
		"posting_reason": reversalReason,
		"reversal_of":    originalPosting.Header.ID,
	}); err != nil {
		return err
	}
	return nil
}

func (s *ProcurementCoreService) hasPostingLink(record document.Record, reason string) bool {
	for _, link := range record.Links {
		if link.LinkType != "posting_for" {
			continue
		}
		if textValue(link.Metadata["posting_reason"]) == reason {
			return true
		}
	}
	return false
}

func (s *ProcurementCoreService) findPostingForReason(record document.Record, reason string) (document.Record, bool) {
	for _, link := range record.Links {
		if link.LinkType != "posting_for" {
			continue
		}
		if textValue(link.Metadata["posting_reason"]) != reason {
			continue
		}
		posting, err := s.documents.Get(link.LinkedDocumentID)
		if err == nil && posting.Header.Type == "ledger_posting" {
			return posting, true
		}
	}
	return document.Record{}, false
}

func (s *ProcurementCoreService) refreshDocuments(records ...document.Record) {
	if s.search == nil {
		return
	}
	for _, record := range records {
		if strings.TrimSpace(record.Header.ID) == "" {
			continue
		}
		s.search.RefreshDocument(record)
	}
}
