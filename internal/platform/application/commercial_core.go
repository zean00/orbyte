package application

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"orbyte/internal/platform/config"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/shared"
)

type CommercialCoreService struct {
	documents *document.Service
	config    *config.Service
	models    *model.Service
	search    *search.Service
}

type ReceivablesSummary struct {
	OpenInvoiceCount    int                `json:"open_invoice_count"`
	OpenBalanceTotal    float64            `json:"open_balance_total"`
	OverdueInvoiceCount int                `json:"overdue_invoice_count"`
	OverdueBalanceTotal float64            `json:"overdue_balance_total"`
	DueTodayCount       int                `json:"due_today_count"`
	DueTodayTotal       float64            `json:"due_today_total"`
	CurrentBalanceTotal float64            `json:"current_balance_total"`
	PaidAmountTotal     float64            `json:"paid_amount_total"`
	CreditedAmountTotal float64            `json:"credited_amount_total"`
	RefundedAmountTotal float64            `json:"refunded_amount_total"`
	Aging               map[string]float64 `json:"aging"`
	Items               []map[string]any   `json:"items"`
}

type PartyCommercialSummary struct {
	PartyID             string           `json:"party_id"`
	OpenInvoiceCount    int              `json:"open_invoice_count"`
	OpenBalanceTotal    float64          `json:"open_balance_total"`
	PaidAmountTotal     float64          `json:"paid_amount_total"`
	CreditedAmountTotal float64          `json:"credited_amount_total"`
	RefundedAmountTotal float64          `json:"refunded_amount_total"`
	OpenInvoices        []map[string]any `json:"open_invoices"`
	Activities          []map[string]any `json:"activities"`
}

type refundablePaymentOption struct {
	record          document.Record
	remainingAmount float64
}

type refundAllocation struct {
	paymentID     string
	paymentNumber string
	amount        float64
	note          string
}

type VariantDimensionSelection struct {
	DimensionCode string   `json:"dimension_code"`
	ValueCodes    []string `json:"value_codes"`
}

func NewCommercialCoreService(documents *document.Service, configSvc *config.Service, models *model.Service, searchSvc *search.Service) *CommercialCoreService {
	return &CommercialCoreService{documents: documents, config: configSvc, models: models, search: searchSvc}
}

func (s *CommercialCoreService) GenerateVariantsForProduct(productID, actorID string, selections []VariantDimensionSelection) ([]model.Record, error) {
	product, err := s.models.Get("commercial_product", strings.TrimSpace(productID))
	if err != nil {
		return nil, err
	}
	productCode := textValue(product.Values["code"])
	if productCode == "" {
		return nil, shared.Validation("product code is required")
	}
	dimensionOrder := orderedVariantDimensions(product.Values["variant_dimension_codes"])
	if len(dimensionOrder) == 0 {
		for _, selection := range selections {
			if code := strings.TrimSpace(selection.DimensionCode); code != "" {
				dimensionOrder = append(dimensionOrder, code)
			}
		}
	}
	if len(dimensionOrder) == 0 {
		return nil, shared.Validation("at least one variant dimension is required")
	}
	valuesByDimension := map[string][]model.Record{}
	for _, selection := range selections {
		dimensionCode := strings.TrimSpace(selection.DimensionCode)
		if dimensionCode == "" {
			continue
		}
		for _, valueCode := range selection.ValueCodes {
			valueCode = strings.TrimSpace(valueCode)
			if valueCode == "" {
				continue
			}
			items, _, listErr := s.models.List("commercial_variant_value", model.Query{
				Filters: map[string]string{
					"dimension_code": dimensionCode,
					"code":           valueCode,
					"status":         "active",
				},
				PageSize: 10,
			})
			if listErr != nil {
				return nil, listErr
			}
			if len(items) == 0 {
				return nil, shared.Validation("variant value not found for selected dimension")
			}
			valuesByDimension[dimensionCode] = append(valuesByDimension[dimensionCode], items[0])
		}
	}
	for _, dimensionCode := range dimensionOrder {
		if len(valuesByDimension[dimensionCode]) == 0 {
			return nil, shared.Validation("each variant dimension must have at least one selected value")
		}
	}

	combos := buildVariantValueCombos(dimensionOrder, valuesByDimension)
	if len(combos) == 0 {
		return nil, shared.Validation("no variants were generated")
	}
	created := make([]model.Record, 0, len(combos))
	for _, combo := range combos {
		signature, label, skuSuffix, valuesJSON := variantDescriptor(combo)
		existing, _, listErr := s.models.List("commercial_item", model.Query{
			Filters: map[string]string{
				"product_code":      productCode,
				"variant_signature": signature,
			},
			PageSize: 2,
		})
		if listErr != nil {
			return nil, listErr
		}
		if len(existing) > 0 {
			continue
		}
		nextValues := map[string]any{
			"product_code":            productCode,
			"sku":                     variantSKU(productCode, skuSuffix),
			"name":                    variantName(textValue(product.Values["name"]), label),
			"description":             textValue(product.Values["description"]),
			"is_variant":              true,
			"variant_signature":       signature,
			"variant_label":           label,
			"variant_values":          valuesJSON,
			"item_type":               firstNonEmptyString(textValue(product.Values["item_type"]), "product"),
			"kind":                    "variant",
			"category_code":           textValue(product.Values["category_code"]),
			"tags":                    textValue(product.Values["tags"]),
			"is_sellable":             true,
			"uom_code":                textValue(product.Values["uom_code"]),
			"currency_code":           textValue(product.Values["currency_code"]),
			"tax_code":                textValue(product.Values["tax_code"]),
			"revenue_account_code":    textValue(product.Values["revenue_account_code"]),
			"inventory_enabled":       boolFieldValue(product.Values["inventory_enabled"]),
			"inventory_tracking_mode": firstNonEmptyString(textValue(product.Values["inventory_tracking_mode"]), "none"),
			"expiry_tracking_enabled": boolFieldValue(product.Values["expiry_tracking_enabled"]),
			"allow_negative_stock":    boolFieldValue(product.Values["allow_negative_stock"]),
			"default_issue_strategy":  firstNonEmptyString(textValue(product.Values["default_issue_strategy"]), "manual"),
			"status":                  firstNonEmptyString(textValue(product.Values["status"]), "active"),
		}
		record, createErr := s.models.Create("commercial_item", actorID, nextValues)
		if createErr != nil {
			return nil, createErr
		}
		created = append(created, record)
	}
	return created, nil
}

func (s *CommercialCoreService) GenerateInvoiceFromOrder(orderID, actorID string) (document.Record, error) {
	order, err := s.documents.Get(strings.TrimSpace(orderID))
	if err != nil {
		return document.Record{}, err
	}
	if order.Header.Type != "sales_order" {
		return document.Record{}, shared.Validation("source document must be a sales order")
	}
	if order.Header.Status != "confirmed" {
		return document.Record{}, shared.Conflict("invoice can only be generated from a confirmed order")
	}
	payload := s.NormalizePayload(order.Header.Type, order.Body.Payload)
	now := time.Now().UTC()
	paymentTermDays := int(numberValue(payload["payment_term_days"]))
	if paymentTermDays <= 0 {
		paymentTermDays = 30
	}
	lines := recordList(payload["lines"])
	invoicePayload := map[string]any{
		"party_id":                textValue(payload["party_id"]),
		"party_name":              textValue(payload["party_name"]),
		"invoice_date":            now.Format("2006-01-02"),
		"due_date":                now.AddDate(0, 0, paymentTermDays).Format("2006-01-02"),
		"currency_code":           firstNonEmptyString(textValue(payload["currency_code"]), order.Header.TotalAmount.Currency, "IDR"),
		"source_order_id":         order.Header.ID,
		"source_order_number":     order.Header.Number,
		"price_list_code":         textValue(payload["price_list_code"]),
		"tax_profile_code":        textValue(payload["tax_profile_code"]),
		"payment_term_days":       paymentTermDays,
		"default_tax_code":        textValue(payload["default_tax_code"]),
		"receivable_account_code": firstNonEmptyString(textValue(payload["receivable_account_code"]), "1100-AR"),
		"subtotal_amount":         numberValue(payload["subtotal_amount"]),
		"tax_amount":              numberValue(payload["tax_amount"]),
		"total_amount":            numberValue(payload["total_amount"]),
		"paid_amount":             0.0,
		"balance_due_amount":      numberValue(payload["total_amount"]),
		"lines":                   lines,
		"notes":                   textValue(payload["notes"]),
	}
	record, err := s.documents.Create("invoice", order.Header.OrganizationID, order.Header.LocationID, actorID, invoicePayload)
	if err != nil {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(record.Header.ID, order.Header.ID, "source_order", map[string]any{"source_type": "sales_order"}); err != nil {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(order.Header.ID, record.Header.ID, "invoice_for", map[string]any{"generated_document_type": "invoice"}); err != nil {
		return document.Record{}, err
	}
	created, err := s.documents.Get(record.Header.ID)
	if err != nil {
		return document.Record{}, err
	}
	s.refreshDocuments(created, order)
	return created, nil
}

func (s *CommercialCoreService) NormalizePayload(documentType string, payload map[string]any) map[string]any {
	switch strings.TrimSpace(documentType) {
	case "sales_order", "invoice", "credit_note":
		return s.normalizeCommercialLines(payload)
	case "payment_receipt":
		return s.normalizeCommercialAllocations(payload)
	case "payment_refund":
		return s.normalizeCommercialRefund(payload)
	case "ledger_posting":
		return s.normalizeJournalLines(payload)
	default:
		return document.NormalizePayload(cloneMap(payload))
	}
}

func (s *CommercialCoreService) CreatePaymentReceiptFromInvoice(invoiceID, actorID string) (document.Record, error) {
	invoice, err := s.documents.Get(strings.TrimSpace(invoiceID))
	if err != nil {
		return document.Record{}, err
	}
	if invoice.Header.Type != "invoice" {
		return document.Record{}, shared.Validation("source document must be an invoice")
	}
	if invoice.Header.Status != "issued" && invoice.Header.Status != "partially_paid" {
		return document.Record{}, shared.Conflict("payment can only be registered for issued invoices")
	}
	payload := invoice.Body.Payload
	openAmount := roundMoney(numberValue(payload["balance_due_amount"]))
	if openAmount <= 0 {
		openAmount = roundMoney(numberValue(payload["total_amount"]) - numberValue(payload["paid_amount"]))
	}
	if openAmount <= 0 {
		return document.Record{}, shared.Conflict("invoice has no remaining balance")
	}
	now := time.Now().UTC()
	paymentPayload := map[string]any{
		"party_id":            textValue(payload["party_id"]),
		"party_name":          textValue(payload["party_name"]),
		"receipt_date":        now.Format("2006-01-02"),
		"payment_method_code": "",
		"payment_reference":   firstNonEmptyString(invoice.Header.Number, invoice.Header.ID),
		"receivable_account_code": textValue(payload["receivable_account_code"]),
		"currency_code":       firstNonEmptyString(textValue(payload["currency_code"]), invoice.Header.TotalAmount.Currency, "IDR"),
		"amount_received":     openAmount,
		"unapplied_amount":    0.0,
		"allocations": []map[string]any{{
			"invoice_number": invoice.Header.Number,
			"invoice_id":     invoice.Header.ID,
			"amount":         openAmount,
			"note":           "Generated from invoice",
		}},
		"notes": "",
	}
	record, err := s.documents.Create("payment_receipt", invoice.Header.OrganizationID, invoice.Header.LocationID, actorID, paymentPayload)
	if err != nil {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(record.Header.ID, invoice.Header.ID, "payment_for", map[string]any{"source_type": "invoice"}); err != nil {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(invoice.Header.ID, record.Header.ID, "payment_for", map[string]any{"generated_document_type": "payment_receipt"}); err != nil {
		return document.Record{}, err
	}
	created, err := s.documents.Get(record.Header.ID)
	if err != nil {
		return document.Record{}, err
	}
	s.refreshDocuments(created, invoice)
	return created, nil
}

func (s *CommercialCoreService) CreateCreditNoteFromInvoice(invoiceID, actorID string) (document.Record, error) {
	invoice, err := s.documents.Get(strings.TrimSpace(invoiceID))
	if err != nil {
		return document.Record{}, err
	}
	if invoice.Header.Type != "invoice" {
		return document.Record{}, shared.Validation("source document must be an invoice")
	}
	if invoice.Header.Status != "issued" && invoice.Header.Status != "partially_paid" && invoice.Header.Status != "paid" {
		return document.Record{}, shared.Conflict("credit note can only be generated from an issued, partially paid, or paid invoice")
	}
	payload := clonedPayload(invoice.Body.Payload)
	creditableAmount := roundMoney(numberValue(payload["balance_due_amount"]))
	if invoice.Header.Status == "paid" || creditableAmount <= 0 {
		creditableAmount = roundMoney(numberValue(payload["paid_amount"]) - numberValue(payload["refunded_amount"]))
	}
	if creditableAmount <= 0 {
		creditableAmount = roundMoney(numberValue(payload["total_amount"]) - numberValue(payload["credited_amount"]))
	}
	if creditableAmount <= 0 {
		return document.Record{}, shared.Conflict("invoice has no remaining creditable balance")
	}
	now := time.Now().UTC()
	lines := scaledCommercialLines(recordList(payload["lines"]), creditableAmount, roundMoney(numberValue(payload["total_amount"])))
	lineSubtotal, lineTax, lineTotal := commercialLineTotals(lines)
	creditPayload := map[string]any{
		"party_id":                textValue(payload["party_id"]),
		"party_name":              textValue(payload["party_name"]),
		"credit_date":             now.Format("2006-01-02"),
		"currency_code":           firstNonEmptyString(textValue(payload["currency_code"]), invoice.Header.TotalAmount.Currency, "IDR"),
		"source_invoice_id":       invoice.Header.ID,
		"source_invoice_number":   invoice.Header.Number,
		"receivable_account_code": textValue(payload["receivable_account_code"]),
		"subtotal_amount":         lineSubtotal,
		"tax_amount":              lineTax,
		"total_amount":            lineTotal,
		"refunded_amount":         0.0,
		"reason":                  fmt.Sprintf("Credit note for invoice %s", firstNonEmptyString(invoice.Header.Number, invoice.Header.ID)),
		"lines":                   lines,
		"notes":                   textValue(payload["notes"]),
	}
	record, err := s.documents.Create("credit_note", invoice.Header.OrganizationID, invoice.Header.LocationID, actorID, creditPayload)
	if err != nil {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(record.Header.ID, invoice.Header.ID, "invoice_for", map[string]any{"source_type": "invoice"}); err != nil {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(invoice.Header.ID, record.Header.ID, "invoice_for", map[string]any{"generated_document_type": "credit_note"}); err != nil {
		return document.Record{}, err
	}
	created, err := s.documents.Get(record.Header.ID)
	if err != nil {
		return document.Record{}, err
	}
	s.refreshDocuments(created, invoice)
	return created, nil
}

func (s *CommercialCoreService) HandleApprovedDocument(record document.Record, actorID string) error {
	switch record.Header.Type {
	case "invoice":
		return s.handleIssuedInvoice(record, actorID)
	case "credit_note":
		return s.handleIssuedCreditNote(record, actorID)
	case "payment_receipt":
		return s.handleReceivedPayment(record, actorID)
	case "payment_refund":
		return s.handleRefundedPayment(record, actorID)
	default:
		return nil
	}
}

func (s *CommercialCoreService) ValidateApprove(record document.Record) error {
	switch record.Header.Type {
	case "credit_note":
		if strings.TrimSpace(textValue(record.Body.Payload["source_invoice_id"])) == "" {
			return shared.Validation("credit note source invoice is required")
		}
	case "payment_refund":
		if strings.TrimSpace(textValue(record.Body.Payload["source_credit_note_id"])) == "" {
			return shared.Validation("refund source credit note is required")
		}
		if strings.TrimSpace(textValue(record.Body.Payload["source_invoice_id"])) == "" {
			return shared.Validation("refund source invoice is required")
		}
	}
	return nil
}

func (s *CommercialCoreService) ValidateCancel(record document.Record) error {
	switch record.Header.Type {
	case "invoice":
		if record.Header.Status != "issued" {
			return nil
		}
		if roundMoney(numberValue(record.Body.Payload["paid_amount"])) > 0 {
			return shared.Conflict("invoice with applied payments cannot be cancelled; reverse payments first")
		}
	case "payment_receipt":
		if record.Header.Status != "received" {
			return nil
		}
	case "payment_refund":
		if record.Header.Status != "refunded" {
			return nil
		}
	}
	return nil
}

func (s *CommercialCoreService) HandleCanceledDocument(record document.Record, actorID string) error {
	current, err := s.documents.Get(record.Header.ID)
	if err == nil {
		record = current
	}
	switch record.Header.Type {
	case "invoice":
		return s.handleCancelledInvoice(record, actorID)
	case "payment_receipt":
		return s.handleCancelledPayment(record, actorID)
	case "payment_refund":
		return s.handleCancelledRefund(record, actorID)
	default:
		return nil
	}
}

func (s *CommercialCoreService) CreateRefundFromCreditNote(creditNoteID, actorID string) (document.Record, error) {
	creditNote, err := s.documents.Get(strings.TrimSpace(creditNoteID))
	if err != nil {
		return document.Record{}, err
	}
	if creditNote.Header.Type != "credit_note" {
		return document.Record{}, shared.Validation("source document must be a credit note")
	}
	if creditNote.Header.Status != "issued" {
		return document.Record{}, shared.Conflict("refund can only be registered from an issued credit note")
	}
	payload := clonedPayload(creditNote.Body.Payload)
	invoiceID := textValue(payload["source_invoice_id"])
	if invoiceID == "" {
		return document.Record{}, shared.Validation("credit note source invoice is required")
	}
	invoice, err := s.documents.Get(invoiceID)
	if err != nil {
		return document.Record{}, err
	}
	invoicePayload := clonedPayload(invoice.Body.Payload)
	invoiceRefundableAmount := roundMoney(numberValue(invoicePayload["paid_amount"]) - numberValue(invoicePayload["refunded_amount"]))
	creditNoteRefundableAmount := roundMoney(numberValue(payload["total_amount"]) - numberValue(payload["refunded_amount"]))
	refundableAmount := roundMoney(invoiceRefundableAmount)
	if creditNoteRefundableAmount < refundableAmount {
		refundableAmount = roundMoney(creditNoteRefundableAmount)
	}
	if refundableAmount <= 0 {
		return document.Record{}, shared.Conflict("invoice has no refundable balance")
	}
	creditAmount := roundMoney(numberValue(payload["total_amount"]))
	if creditAmount > 0 && creditAmount < refundableAmount {
		refundableAmount = creditAmount
	}
	selectedPayment, methodCode, clearingAccount := s.resolveRefundPaymentDefaults(invoice)
	refundAllocations := buildRefundAllocationRows(s.refundablePaymentsForInvoice(invoice), refundableAmount)
	sourcePaymentID := selectedPayment.record.Header.ID
	sourcePaymentNumber := firstNonEmptyString(selectedPayment.record.Header.Number, selectedPayment.record.Header.ID)
	if len(refundAllocations) != 1 {
		sourcePaymentID = ""
		sourcePaymentNumber = ""
	}
	now := time.Now().UTC()
	refundPayload := map[string]any{
		"party_id":                  textValue(payload["party_id"]),
		"party_name":                textValue(payload["party_name"]),
		"refund_date":               now.Format("2006-01-02"),
		"payment_method_code":       methodCode,
		"clearing_account_code":     clearingAccount,
		"refund_reference":          firstNonEmptyString(creditNote.Header.Number, creditNote.Header.ID),
		"currency_code":             firstNonEmptyString(textValue(payload["currency_code"]), invoice.Header.TotalAmount.Currency, "IDR"),
		"amount_refunded":           refundableAmount,
		"source_credit_note_id":     creditNote.Header.ID,
		"source_credit_note_number": firstNonEmptyString(creditNote.Header.Number, creditNote.Header.ID),
		"source_invoice_id":         invoice.Header.ID,
		"source_invoice_number":     firstNonEmptyString(invoice.Header.Number, invoice.Header.ID),
		"source_payment_id":         sourcePaymentID,
		"source_payment_number":     sourcePaymentNumber,
		"refund_allocations":        refundAllocations,
		"receivable_account_code":   firstNonEmptyString(textValue(invoicePayload["receivable_account_code"]), textValue(payload["receivable_account_code"])),
		"reason":                    fmt.Sprintf("Refund for credit note %s", firstNonEmptyString(creditNote.Header.Number, creditNote.Header.ID)),
		"notes":                     textValue(payload["reason"]),
	}
	record, err := s.documents.Create("payment_refund", creditNote.Header.OrganizationID, creditNote.Header.LocationID, actorID, refundPayload)
	if err != nil {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(record.Header.ID, creditNote.Header.ID, "refund_for", map[string]any{"source_type": "credit_note"}); err != nil {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(creditNote.Header.ID, record.Header.ID, "refund_for", map[string]any{"generated_document_type": "payment_refund"}); err != nil {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(record.Header.ID, invoice.Header.ID, "refund_for", map[string]any{"source_type": "invoice"}); err != nil && !isConflict(err) {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(invoice.Header.ID, record.Header.ID, "refund_for", map[string]any{"generated_document_type": "payment_refund"}); err != nil && !isConflict(err) {
		return document.Record{}, err
	}
	created, err := s.documents.Get(record.Header.ID)
	if err != nil {
		return document.Record{}, err
	}
	s.refreshDocuments(created, creditNote, invoice)
	return created, nil
}

func (s *CommercialCoreService) ReceivablesSummary(now time.Time) ReceivablesSummary {
	return s.ReceivablesSummaryScoped("", "", now)
}

func (s *CommercialCoreService) ReceivablesSummaryScoped(organizationID, locationID string, now time.Time) ReceivablesSummary {
	summary := ReceivablesSummary{
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
		if record.Header.Type != "invoice" {
			continue
		}
		if record.Header.Status != "issued" && record.Header.Status != "partially_paid" {
			continue
		}
		payload := clonedPayload(record.Body.Payload)
		summary.PaidAmountTotal = roundMoney(summary.PaidAmountTotal + roundMoney(numberValue(payload["paid_amount"])))
		summary.CreditedAmountTotal = roundMoney(summary.CreditedAmountTotal + roundMoney(numberValue(payload["credited_amount"])))
		summary.RefundedAmountTotal = roundMoney(summary.RefundedAmountTotal + roundMoney(numberValue(payload["refunded_amount"])))
		balance := roundMoney(numberValue(payload["balance_due_amount"]))
		if balance <= 0 {
			continue
		}
		summary.OpenInvoiceCount++
		summary.OpenBalanceTotal = roundMoney(summary.OpenBalanceTotal + balance)

		dueDate := textValue(payload["due_date"])
		bucket := "current"
		if dueDate == today {
			summary.DueTodayCount++
			summary.DueTodayTotal = roundMoney(summary.DueTodayTotal + balance)
			bucket = "due_today"
		} else if overdueDays := dateDiffDays(dueDate, today); overdueDays > 0 {
			summary.OverdueInvoiceCount++
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
			"party_name":   textValue(payload["party_name"]),
			"status":       record.Header.Status,
			"invoice_date": textValue(payload["invoice_date"]),
			"due_date":     dueDate,
			"total_amount": roundMoney(numberValue(payload["total_amount"])),
			"paid_amount":  roundMoney(numberValue(payload["paid_amount"])),
			"credited":     roundMoney(numberValue(payload["credited_amount"])),
			"refunded":     roundMoney(numberValue(payload["refunded_amount"])),
			"balance_due":  balance,
			"aging_bucket": bucket,
			"days_overdue": maxInt(dateDiffDays(dueDate, today), 0),
		})
	}
	return summary
}

func (s *CommercialCoreService) PartyCommercialSummary(partyID, fromDate, toDate string) PartyCommercialSummary {
	return s.PartyCommercialSummaryScoped("", "", partyID, fromDate, toDate)
}

func (s *CommercialCoreService) PartyCommercialSummaryScoped(organizationID, locationID, partyID, fromDate, toDate string) PartyCommercialSummary {
	summary := PartyCommercialSummary{
		PartyID:      strings.TrimSpace(partyID),
		OpenInvoices: make([]map[string]any, 0),
		Activities:   make([]map[string]any, 0),
	}
	if summary.PartyID == "" {
		return summary
	}
	for _, record := range s.documents.List() {
		if !matchesCommercialScope(record, organizationID, locationID) {
			continue
		}
		payload := clonedPayload(record.Body.Payload)
		if textValue(payload["party_id"]) != summary.PartyID {
			continue
		}
		dateValue, amountValue := commercialActivityValues(record, payload)
		if amountValue != 0 && withinDateRange(dateValue, fromDate, toDate) {
			summary.Activities = append(summary.Activities, map[string]any{
				"id":      record.Header.ID,
				"type":    record.Header.Type,
				"number":  record.Header.Number,
				"status":  record.Header.Status,
				"date":    dateValue,
				"amount":  amountValue,
				"counter": textValue(payload["source_invoice_number"]),
			})
		}
		if record.Header.Type != "invoice" {
			continue
		}
		summary.PaidAmountTotal = roundMoney(summary.PaidAmountTotal + roundMoney(numberValue(payload["paid_amount"])))
		summary.CreditedAmountTotal = roundMoney(summary.CreditedAmountTotal + roundMoney(numberValue(payload["credited_amount"])))
		summary.RefundedAmountTotal = roundMoney(summary.RefundedAmountTotal + roundMoney(numberValue(payload["refunded_amount"])))
		balance := roundMoney(numberValue(payload["balance_due_amount"]))
		if balance > 0 && (record.Header.Status == "issued" || record.Header.Status == "partially_paid") {
			summary.OpenInvoiceCount++
			summary.OpenBalanceTotal = roundMoney(summary.OpenBalanceTotal + balance)
			summary.OpenInvoices = append(summary.OpenInvoices, map[string]any{
				"id":           record.Header.ID,
				"number":       record.Header.Number,
				"status":       record.Header.Status,
				"invoice_date": textValue(payload["invoice_date"]),
				"due_date":     textValue(payload["due_date"]),
				"total_amount": roundMoney(numberValue(payload["total_amount"])),
				"paid_amount":  roundMoney(numberValue(payload["paid_amount"])),
				"credited":     roundMoney(numberValue(payload["credited_amount"])),
				"refunded":     roundMoney(numberValue(payload["refunded_amount"])),
				"balance_due":  balance,
			})
		}
	}
	sort.Slice(summary.OpenInvoices, func(i, j int) bool {
		left := textValue(summary.OpenInvoices[i]["due_date"])
		right := textValue(summary.OpenInvoices[j]["due_date"])
		if left != right {
			return left < right
		}
		return textValue(summary.OpenInvoices[i]["number"]) < textValue(summary.OpenInvoices[j]["number"])
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

func matchesCommercialScope(record document.Record, organizationID, locationID string) bool {
	organizationID = strings.TrimSpace(organizationID)
	locationID = strings.TrimSpace(locationID)
	if organizationID != "" && strings.TrimSpace(record.Header.OrganizationID) != "" && strings.TrimSpace(record.Header.OrganizationID) != organizationID {
		return false
	}
	if locationID != "" && strings.TrimSpace(record.Header.LocationID) != "" && strings.TrimSpace(record.Header.LocationID) != locationID {
		return false
	}
	return true
}

func withinDateRange(value, fromDate, toDate string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	fromDate = strings.TrimSpace(fromDate)
	toDate = strings.TrimSpace(toDate)
	if fromDate != "" && value < fromDate {
		return false
	}
	if toDate != "" && value > toDate {
		return false
	}
	return true
}

func commercialActivityValues(record document.Record, payload map[string]any) (string, float64) {
	switch record.Header.Type {
	case "invoice":
		return textValue(payload["invoice_date"]), roundMoney(numberValue(payload["total_amount"]))
	case "payment_receipt":
		return textValue(payload["receipt_date"]), roundMoney(numberValue(payload["amount_received"]))
	case "credit_note":
		return textValue(payload["credit_date"]), roundMoney(numberValue(payload["total_amount"]))
	case "payment_refund":
		return textValue(payload["refund_date"]), roundMoney(numberValue(payload["amount_refunded"]))
	default:
		return "", 0
	}
}

func (s *CommercialCoreService) handleIssuedInvoice(invoice document.Record, actorID string) error {
	payload := clonedPayload(invoice.Body.Payload)
	totalAmount := roundMoney(numberValue(payload["total_amount"]))
	paidAmount := roundMoney(numberValue(payload["paid_amount"]))
	if paidAmount < 0 {
		paidAmount = 0
	}
	payload["paid_amount"] = paidAmount
	payload["balance_due_amount"] = roundMoney(maxFloat(totalAmount-paidAmount, 0))
	if err := s.updateDocumentPayload(invoice, actorID, payload); err != nil {
		return err
	}
	if s.hasPostingLink(invoice, "invoice_issue") {
		return nil
	}
	journalLines := s.invoicePostingLines(payload)
	postingPayload := map[string]any{
		"source_document_type": invoice.Header.Type,
		"source_document_id":   invoice.Header.ID,
		"posting_date":         time.Now().UTC().Format("2006-01-02"),
		"currency_code":        firstNonEmptyString(textValue(payload["currency_code"]), invoice.Header.TotalAmount.Currency, "IDR"),
		"posting_rule_key":     "invoice_issue_default",
		"total_amount":         totalAmount,
		"journal_lines":        journalLines,
		"notes":                fmt.Sprintf("Auto-posted from invoice %s", firstNonEmptyString(invoice.Header.Number, invoice.Header.ID)),
	}
	posting, err := s.documents.Create("ledger_posting", invoice.Header.OrganizationID, invoice.Header.LocationID, actorID, postingPayload)
	if err != nil {
		return err
	}
	if err := s.finalizeSystemPosting(posting, actorID, "posted"); err != nil {
		return err
	}
	if _, err := s.documents.AddLink(posting.Header.ID, invoice.Header.ID, "posting_for", map[string]any{"posting_reason": "invoice_issue"}); err != nil {
		return err
	}
	_, err = s.documents.AddLink(invoice.Header.ID, posting.Header.ID, "posting_for", map[string]any{"posting_reason": "invoice_issue"})
	return err
}

func (s *CommercialCoreService) handleIssuedCreditNote(creditNote document.Record, actorID string) error {
	payload := clonedPayload(creditNote.Body.Payload)
	invoiceID := textValue(payload["source_invoice_id"])
	if strings.TrimSpace(invoiceID) == "" {
		return shared.Validation("credit note source invoice is required")
	}
	invoice, err := s.documents.Get(invoiceID)
	if err != nil {
		return err
	}
	if invoice.Header.Type != "invoice" {
		return shared.Validation("credit note source must be an invoice")
	}
	invoicePayload := clonedPayload(invoice.Body.Payload)
	creditAmount := roundMoney(numberValue(payload["total_amount"]))
	creditableAmount := roundMoney(numberValue(invoicePayload["balance_due_amount"]))
	if invoice.Header.Status == "paid" || creditableAmount <= 0 {
		creditableAmount = roundMoney(numberValue(invoicePayload["paid_amount"]) - roundMoney(numberValue(invoicePayload["refunded_amount"])))
	}
	if creditableAmount <= 0 {
		creditableAmount = roundMoney(numberValue(invoicePayload["total_amount"]) - roundMoney(numberValue(invoicePayload["credited_amount"])))
	}
	if creditAmount <= 0 {
		return shared.Validation("credit note amount must be greater than zero")
	}
	if creditAmount-creditableAmount > 0.0001 {
		return shared.Validation("credit note exceeds invoice creditable balance")
	}
	creditedAmount := roundMoney(numberValue(invoicePayload["credited_amount"]) + creditAmount)
	invoicePayload["credited_amount"] = creditedAmount
	paidAmount := roundMoney(numberValue(invoicePayload["paid_amount"]))
	balanceDue := roundMoney(maxFloat(numberValue(invoicePayload["total_amount"])-paidAmount-creditedAmount, 0))
	invoicePayload["balance_due_amount"] = balanceDue
	if balanceDue == 0 {
		if paidAmount > 0 {
			invoice.Header.Status = "paid"
		} else {
			invoice.Header.Status = "cancelled"
		}
	} else if paidAmount > 0 {
		invoice.Header.Status = "partially_paid"
	} else {
		invoice.Header.Status = "issued"
	}
	if err := s.saveMutatedDocument(invoice, actorID, invoicePayload); err != nil {
		return err
	}
	if s.hasPostingLink(creditNote, "credit_note_issue") {
		return nil
	}
	journalLines := reverseJournalLines(s.invoicePostingLines(payload))
	postingPayload := map[string]any{
		"source_document_type": creditNote.Header.Type,
		"source_document_id":   creditNote.Header.ID,
		"posting_date":         time.Now().UTC().Format("2006-01-02"),
		"currency_code":        firstNonEmptyString(textValue(payload["currency_code"]), creditNote.Header.TotalAmount.Currency, "IDR"),
		"posting_rule_key":     "credit_note_issue_default",
		"total_amount":         creditAmount,
		"journal_lines":        journalLines,
		"notes":                fmt.Sprintf("Auto-posted from credit note %s", firstNonEmptyString(creditNote.Header.Number, creditNote.Header.ID)),
	}
	posting, err := s.documents.Create("ledger_posting", creditNote.Header.OrganizationID, creditNote.Header.LocationID, actorID, postingPayload)
	if err != nil {
		return err
	}
	if err := s.finalizeSystemPosting(posting, actorID, "posted"); err != nil {
		return err
	}
	if _, err := s.documents.AddLink(posting.Header.ID, creditNote.Header.ID, "posting_for", map[string]any{"posting_reason": "credit_note_issue"}); err != nil {
		return err
	}
	if _, err := s.documents.AddLink(creditNote.Header.ID, posting.Header.ID, "posting_for", map[string]any{"posting_reason": "credit_note_issue"}); err != nil {
		return err
	}
	return nil
}

func (s *CommercialCoreService) handleReceivedPayment(payment document.Record, actorID string) error {
	payload := clonedPayload(payment.Body.Payload)
	allocations := recordList(payload["allocations"])
	amountReceived := roundMoney(numberValue(payload["amount_received"]))
	appliedAmount := 0.0
	for _, allocation := range allocations {
		appliedAmount += roundMoney(numberValue(allocation["amount"]))
	}
	payload["unapplied_amount"] = roundMoney(maxFloat(amountReceived-appliedAmount, 0))
	if err := s.updateDocumentPayload(payment, actorID, payload); err != nil {
		return err
	}
	for _, allocation := range allocations {
		invoiceID := textValue(allocation["invoice_id"])
		if strings.TrimSpace(invoiceID) == "" {
			continue
		}
		if err := s.applyAllocationToInvoice(payment, invoiceID, roundMoney(numberValue(allocation["amount"])), actorID); err != nil {
			return err
		}
	}
	if s.hasPostingLink(payment, "payment_receipt") {
		return nil
	}
	journalLines := s.paymentPostingLines(payload)
	postingPayload := map[string]any{
		"source_document_type": payment.Header.Type,
		"source_document_id":   payment.Header.ID,
		"posting_date":         time.Now().UTC().Format("2006-01-02"),
		"currency_code":        firstNonEmptyString(textValue(payload["currency_code"]), payment.Header.TotalAmount.Currency, "IDR"),
		"posting_rule_key":     "payment_receipt_default",
		"total_amount":         amountReceived,
		"journal_lines":        journalLines,
		"notes":                fmt.Sprintf("Auto-posted from payment %s", firstNonEmptyString(payment.Header.Number, payment.Header.ID)),
	}
	posting, err := s.documents.Create("ledger_posting", payment.Header.OrganizationID, payment.Header.LocationID, actorID, postingPayload)
	if err != nil {
		return err
	}
	if err := s.finalizeSystemPosting(posting, actorID, "posted"); err != nil {
		return err
	}
	if _, err := s.documents.AddLink(posting.Header.ID, payment.Header.ID, "posting_for", map[string]any{"posting_reason": "payment_receipt"}); err != nil {
		return err
	}
	_, err = s.documents.AddLink(payment.Header.ID, posting.Header.ID, "posting_for", map[string]any{"posting_reason": "payment_receipt"})
	return err
}

func (s *CommercialCoreService) handleRefundedPayment(refund document.Record, actorID string) error {
	payload := clonedPayload(refund.Body.Payload)
	invoiceID := textValue(payload["source_invoice_id"])
	if invoiceID == "" {
		return shared.Validation("refund source invoice is required")
	}
	invoice, err := s.documents.Get(invoiceID)
	if err != nil {
		return err
	}
	if invoice.Header.Type != "invoice" {
		return shared.Validation("refund source must be an invoice")
	}
	creditNoteID := textValue(payload["source_credit_note_id"])
	if creditNoteID == "" {
		return shared.Validation("refund source credit note is required")
	}
	creditNote, err := s.documents.Get(creditNoteID)
	if err != nil {
		return err
	}
	if creditNote.Header.Type != "credit_note" {
		return shared.Validation("refund source credit note must be a credit note")
	}
	invoicePayload := clonedPayload(invoice.Body.Payload)
	creditNotePayload := clonedPayload(creditNote.Body.Payload)
	refundAmount := roundMoney(numberValue(payload["amount_refunded"]))
	if refundAmount <= 0 {
		return shared.Validation("refund amount must be greater than zero")
	}
	creditNoteRefunded := roundMoney(numberValue(creditNotePayload["refunded_amount"]) + refundAmount)
	if creditNoteRefunded-roundMoney(numberValue(creditNotePayload["total_amount"])) > 0.0001 {
		return shared.Validation("refund exceeds credit note amount")
	}
	refundedAmount := roundMoney(numberValue(invoicePayload["refunded_amount"]) + refundAmount)
	paidAmount := roundMoney(numberValue(invoicePayload["paid_amount"]))
	if refundedAmount-paidAmount > 0.0001 {
		return shared.Validation("refund exceeds paid amount")
	}
	allocations := refundAllocationsFromPayload(payload)
	if len(allocations) > 0 {
		totalAllocated := 0.0
		for _, allocation := range allocations {
			if allocation.amount <= 0 {
				continue
			}
			totalAllocated = roundMoney(totalAllocated + allocation.amount)
			payment, err := s.documents.Get(allocation.paymentID)
			if err != nil {
				return err
			}
			if payment.Header.Type != "payment_receipt" {
				return shared.Validation("refund source payment must be a payment receipt")
			}
			paymentPayload := clonedPayload(payment.Body.Payload)
			refundableAmount := roundMoney(numberValue(paymentPayload["amount_received"]) - numberValue(paymentPayload["refunded_amount"]))
			if allocation.amount-refundableAmount > 0.0001 {
				return shared.Validation("refund exceeds source payment amount")
			}
			paymentPayload["refunded_amount"] = roundMoney(numberValue(paymentPayload["refunded_amount"]) + allocation.amount)
			if err := s.saveMutatedDocument(payment, actorID, paymentPayload); err != nil {
				return err
			}
		}
		if roundMoney(totalAllocated-refundAmount) > 0.0001 || roundMoney(refundAmount-totalAllocated) > 0.0001 {
			return shared.Validation("refund allocations must equal refunded amount")
		}
	}
	invoicePayload["refunded_amount"] = refundedAmount
	creditNotePayload["refunded_amount"] = creditNoteRefunded
	if refundedAmount > 0 && refundedAmount == paidAmount {
		invoice.Header.Status = "refunded"
	}
	if err := s.saveMutatedDocument(invoice, actorID, invoicePayload); err != nil {
		return err
	}
	if err := s.saveMutatedDocument(creditNote, actorID, creditNotePayload); err != nil {
		return err
	}
	if s.hasPostingLink(refund, "payment_refund") {
		return nil
	}
	journalLines := s.refundPostingLines(payload)
	postingPayload := map[string]any{
		"source_document_type": refund.Header.Type,
		"source_document_id":   refund.Header.ID,
		"posting_date":         time.Now().UTC().Format("2006-01-02"),
		"currency_code":        firstNonEmptyString(textValue(payload["currency_code"]), refund.Header.TotalAmount.Currency, "IDR"),
		"posting_rule_key":     "payment_refund_default",
		"total_amount":         refundAmount,
		"journal_lines":        journalLines,
		"notes":                fmt.Sprintf("Auto-posted from refund %s", firstNonEmptyString(refund.Header.Number, refund.Header.ID)),
	}
	posting, err := s.documents.Create("ledger_posting", refund.Header.OrganizationID, refund.Header.LocationID, actorID, postingPayload)
	if err != nil {
		return err
	}
	if err := s.finalizeSystemPosting(posting, actorID, "posted"); err != nil {
		return err
	}
	if _, err := s.documents.AddLink(posting.Header.ID, refund.Header.ID, "posting_for", map[string]any{"posting_reason": "payment_refund"}); err != nil {
		return err
	}
	_, err = s.documents.AddLink(refund.Header.ID, posting.Header.ID, "posting_for", map[string]any{"posting_reason": "payment_refund"})
	return err
}

func (s *CommercialCoreService) applyAllocationToInvoice(payment document.Record, invoiceID string, amount float64, actorID string) error {
	if amount <= 0 {
		return nil
	}
	invoice, err := s.documents.Get(invoiceID)
	if err != nil {
		return err
	}
	if invoice.Header.Type != "invoice" {
		return shared.Validation("allocation target must be an invoice")
	}
	payload := clonedPayload(invoice.Body.Payload)
	totalAmount := roundMoney(numberValue(payload["total_amount"]))
	paidAmount := roundMoney(numberValue(payload["paid_amount"]) + amount)
	if paidAmount-totalAmount > 0.0001 {
		return shared.Validation("allocation exceeds invoice balance")
	}
	payload["paid_amount"] = paidAmount
	balanceDue := roundMoney(maxFloat(totalAmount-paidAmount, 0))
	payload["balance_due_amount"] = balanceDue
	if balanceDue == 0 {
		invoice.Header.Status = "paid"
	} else if paidAmount > 0 && (invoice.Header.Status == "issued" || invoice.Header.Status == "partially_paid") {
		invoice.Header.Status = "partially_paid"
	}
	if err := s.saveMutatedDocument(invoice, actorID, payload); err != nil {
		return err
	}
	if _, err := s.documents.AddLink(payment.Header.ID, invoice.Header.ID, "payment_for", map[string]any{"allocated_amount": amount}); err != nil && !isConflict(err) {
		return err
	}
	if _, err := s.documents.AddLink(invoice.Header.ID, payment.Header.ID, "payment_for", map[string]any{"allocated_amount": amount}); err != nil && !isConflict(err) {
		return err
	}
	return nil
}

func (s *CommercialCoreService) reverseAllocationOnInvoice(payment document.Record, invoiceID string, amount float64, actorID string) error {
	if amount <= 0 {
		return nil
	}
	invoice, err := s.documents.Get(invoiceID)
	if err != nil {
		return err
	}
	if invoice.Header.Type != "invoice" {
		return shared.Validation("allocation target must be an invoice")
	}
	payload := clonedPayload(invoice.Body.Payload)
	totalAmount := roundMoney(numberValue(payload["total_amount"]))
	paidAmount := roundMoney(numberValue(payload["paid_amount"]) - amount)
	if paidAmount < 0 {
		paidAmount = 0
	}
	payload["paid_amount"] = paidAmount
	balanceDue := roundMoney(maxFloat(totalAmount-paidAmount, 0))
	payload["balance_due_amount"] = balanceDue
	switch {
	case balanceDue == 0:
		invoice.Header.Status = "paid"
	case paidAmount > 0:
		invoice.Header.Status = "partially_paid"
	default:
		invoice.Header.Status = "issued"
	}
	return s.saveMutatedDocument(invoice, actorID, payload)
}

func (s *CommercialCoreService) updateDocumentPayload(record document.Record, actorID string, payload map[string]any) error {
	return s.saveMutatedDocument(record, actorID, payload)
}

func (s *CommercialCoreService) saveMutatedDocument(record document.Record, actorID string, payload map[string]any) error {
	record.Body.Payload = document.NormalizePayload(payload)
	record.Body.ContentHash = document.ContentHash(record.Body.Payload)
	record.Header.Version++
	record.Header.ETag = fmt.Sprintf("%s:%d", record.Header.ID, record.Header.Version)
	record.Header.UpdatedBy = actorID
	record.Header.UpdatedAt = time.Now().UTC()
	record.Header.TotalAmount = shared.Money{
		Currency:    firstNonEmptyString(textValue(record.Body.Payload["currency_code"]), record.Header.TotalAmount.Currency, "IDR"),
		AmountMinor: moneyMinor(derivedRecordAmount(record.Body.Payload)),
	}
	if err := s.documents.Save(record); err != nil {
		return err
	}
	s.refreshDocuments(record)
	return nil
}

func (s *CommercialCoreService) finalizeSystemPosting(record document.Record, actorID, status string) error {
	record.Header.Status = status
	record.Header.TotalAmount = shared.Money{
		Currency:    firstNonEmptyString(textValue(record.Body.Payload["currency_code"]), record.Header.TotalAmount.Currency, "IDR"),
		AmountMinor: moneyMinor(derivedRecordAmount(record.Body.Payload)),
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

func (s *CommercialCoreService) handleCancelledInvoice(invoice document.Record, actorID string) error {
	if s.hasPostingLink(invoice, "invoice_cancel_reversal") {
		return nil
	}
	return s.createReversalPosting(invoice, actorID, "invoice_issue", "invoice_cancel_reversal", "invoice_issue_reversal")
}

func (s *CommercialCoreService) handleCancelledPayment(payment document.Record, actorID string) error {
	payload := clonedPayload(payment.Body.Payload)
	for _, allocation := range recordList(payload["allocations"]) {
		invoiceID := textValue(allocation["invoice_id"])
		if strings.TrimSpace(invoiceID) == "" {
			continue
		}
		if err := s.reverseAllocationOnInvoice(payment, invoiceID, roundMoney(numberValue(allocation["amount"])), actorID); err != nil {
			return err
		}
	}
	payload["unapplied_amount"] = roundMoney(numberValue(payload["amount_received"]))
	if err := s.updateDocumentPayload(payment, actorID, payload); err != nil {
		return err
	}
	if s.hasPostingLink(payment, "payment_cancel_reversal") {
		return nil
	}
	return s.createReversalPosting(payment, actorID, "payment_receipt", "payment_cancel_reversal", "payment_receipt_reversal")
}

func (s *CommercialCoreService) handleCancelledRefund(refund document.Record, actorID string) error {
	payload := clonedPayload(refund.Body.Payload)
	for _, allocation := range refundAllocationsFromPayload(payload) {
		payment, err := s.documents.Get(allocation.paymentID)
		if err != nil {
			return err
		}
		paymentPayload := clonedPayload(payment.Body.Payload)
		refundedAmount := roundMoney(numberValue(paymentPayload["refunded_amount"]) - allocation.amount)
		if refundedAmount < 0 {
			refundedAmount = 0
		}
		paymentPayload["refunded_amount"] = refundedAmount
		if err := s.saveMutatedDocument(payment, actorID, paymentPayload); err != nil {
			return err
		}
	}
	creditNoteID := textValue(payload["source_credit_note_id"])
	if creditNoteID != "" {
		creditNote, err := s.documents.Get(creditNoteID)
		if err != nil {
			return err
		}
		creditNotePayload := clonedPayload(creditNote.Body.Payload)
		refundAmount := roundMoney(numberValue(payload["amount_refunded"]))
		refundedAmount := roundMoney(numberValue(creditNotePayload["refunded_amount"]) - refundAmount)
		if refundedAmount < 0 {
			refundedAmount = 0
		}
		creditNotePayload["refunded_amount"] = refundedAmount
		if err := s.saveMutatedDocument(creditNote, actorID, creditNotePayload); err != nil {
			return err
		}
	}
	invoiceID := textValue(payload["source_invoice_id"])
	if invoiceID != "" {
		invoice, err := s.documents.Get(invoiceID)
		if err != nil {
			return err
		}
		invoicePayload := clonedPayload(invoice.Body.Payload)
		refundAmount := roundMoney(numberValue(payload["amount_refunded"]))
		refundedAmount := roundMoney(numberValue(invoicePayload["refunded_amount"]) - refundAmount)
		if refundedAmount < 0 {
			refundedAmount = 0
		}
		invoicePayload["refunded_amount"] = refundedAmount
		if invoice.Header.Status == "refunded" {
			invoice.Header.Status = "paid"
		}
		if err := s.saveMutatedDocument(invoice, actorID, invoicePayload); err != nil {
			return err
		}
	}
	if s.hasPostingLink(refund, "payment_refund_reversal") {
		return nil
	}
	return s.createReversalPosting(refund, actorID, "payment_refund", "payment_refund_reversal", "payment_refund_reversal")
}

func (s *CommercialCoreService) resolveRefundPaymentDefaults(invoice document.Record) (refundablePaymentOption, string, string) {
	payments := s.refundablePaymentsForInvoice(invoice)
	if len(payments) == 0 {
		return refundablePaymentOption{}, "", ""
	}
	selected := payments[0]
	payload := clonedPayload(selected.record.Body.Payload)
	methodCode := textValue(payload["payment_method_code"])
	clearingAccount := firstNonEmptyString(textValue(payload["clearing_account_code"]), s.resolvePaymentClearingAccount(payload))
	return selected, methodCode, clearingAccount
}

func (s *CommercialCoreService) refundablePaymentsForInvoice(invoice document.Record) []refundablePaymentOption {
	options := make([]refundablePaymentOption, 0)
	for _, link := range invoice.Links {
		if link.LinkType != "payment_for" {
			continue
		}
		payment, err := s.documents.Get(link.LinkedDocumentID)
		if err != nil || payment.Header.Type != "payment_receipt" || payment.Header.Status != "received" {
			continue
		}
		payload := clonedPayload(payment.Body.Payload)
		remainingAmount := roundMoney(numberValue(payload["amount_received"]) - numberValue(payload["refunded_amount"]))
		if remainingAmount <= 0 {
			continue
		}
		options = append(options, refundablePaymentOption{record: payment, remainingAmount: remainingAmount})
	}
	return options
}

func (s *CommercialCoreService) createReversalPosting(source document.Record, actorID, originalReason, reversalReason, postingRuleKey string) error {
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
	if _, err := s.documents.AddLink(reversal.Header.ID, originalPosting.Header.ID, "posting_for", map[string]any{
		"posting_reason":  "reversal_of",
		"source_document": source.Header.ID,
	}); err != nil && !isConflict(err) {
		return err
	}
	_, err = s.documents.AddLink(originalPosting.Header.ID, reversal.Header.ID, "posting_for", map[string]any{
		"posting_reason":  "reversal_pair",
		"source_document": source.Header.ID,
	})
	if err != nil && !isConflict(err) {
		return err
	}
	return nil
}

func (s *CommercialCoreService) hasPostingLink(record document.Record, reason string) bool {
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

func (s *CommercialCoreService) findPostingForReason(record document.Record, reason string) (document.Record, bool) {
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

func (s *CommercialCoreService) invoicePostingLines(payload map[string]any) []map[string]any {
	subtotal := roundMoney(numberValue(payload["subtotal_amount"]))
	taxAmount := roundMoney(numberValue(payload["tax_amount"]))
	totalAmount := roundMoney(numberValue(payload["total_amount"]))
	postingConfig := s.postingConfig()
	receivableAccount := firstNonEmptyString(
		textValue(payload["receivable_account_code"]),
		postingConfig["invoice_issue_receivable_account_code"],
		"1100-AR",
	)
	lines := []map[string]any{{
		"account_code": receivableAccount,
		"description":  "Accounts Receivable",
		"debit":        totalAmount,
		"credit":       0.0,
	}}
	revenueAccountDefault := firstNonEmptyString(
		s.resolveRevenueAccount(payload),
		postingConfig["invoice_issue_revenue_account_code"],
		"4000-REV",
	)
	revenueByAccount := map[string]float64{}
	taxByAccount := map[string]float64{}
	revenueTotal := 0.0
	taxTotal := 0.0
	for _, line := range recordList(payload["lines"]) {
		revenueAccount := firstNonEmptyString(textValue(line["revenue_account_code"]), revenueAccountDefault)
		lineSubtotal := roundMoney(numberValue(line["line_subtotal"]))
		revenueByAccount[revenueAccount] = roundMoney(revenueByAccount[revenueAccount] + lineSubtotal)
		revenueTotal = roundMoney(revenueTotal + lineSubtotal)
		taxLineAmount := roundMoney(numberValue(line["tax_amount"]))
		if taxLineAmount <= 0 {
			continue
		}
		taxAccount := firstNonEmptyString(
			textValue(line["tax_account_code"]),
			s.lookupAccountCode("commercial_tax_code", "code", textValue(line["tax_code"]), "tax_account_code"),
			postingConfig["invoice_issue_tax_account_code"],
			"2100-TAX",
		)
		taxByAccount[taxAccount] = roundMoney(taxByAccount[taxAccount] + taxLineAmount)
		taxTotal = roundMoney(taxTotal + taxLineAmount)
	}
	if revenueTotal == 0 && subtotal > 0 {
		revenueByAccount[revenueAccountDefault] = subtotal
	}
	revenueAccounts := make([]string, 0, len(revenueByAccount))
	for accountCode := range revenueByAccount {
		revenueAccounts = append(revenueAccounts, accountCode)
	}
	sort.Strings(revenueAccounts)
	for _, accountCode := range revenueAccounts {
		amount := revenueByAccount[accountCode]
		if amount <= 0 {
			continue
		}
		lines = append(lines, map[string]any{
			"account_code": accountCode,
			"description":  "Revenue",
			"debit":        0.0,
			"credit":       amount,
		})
	}
	if taxTotal == 0 && taxAmount > 0 {
		taxAccount := firstNonEmptyString(
			s.resolveTaxAccount(payload),
			postingConfig["invoice_issue_tax_account_code"],
			"2100-TAX",
		)
		taxByAccount[taxAccount] = taxAmount
	}
	taxAccounts := make([]string, 0, len(taxByAccount))
	for accountCode := range taxByAccount {
		taxAccounts = append(taxAccounts, accountCode)
	}
	sort.Strings(taxAccounts)
	for _, accountCode := range taxAccounts {
		amount := taxByAccount[accountCode]
		if amount <= 0 {
			continue
		}
		lines = append(lines, map[string]any{
			"account_code": accountCode,
			"description":  "Tax Payable",
			"debit":        0.0,
			"credit":       amount,
		})
	}
	return lines
}

func (s *CommercialCoreService) paymentPostingLines(payload map[string]any) []map[string]any {
	totalAmount := roundMoney(numberValue(payload["amount_received"]))
	postingConfig := s.postingConfig()
	clearingAccount := firstNonEmptyString(
		s.resolvePaymentClearingAccount(payload),
		postingConfig["payment_receipt_clearing_account_code"],
		"1000-CASH",
	)
	receivableAccount := firstNonEmptyString(
		textValue(payload["receivable_account_code"]),
		postingConfig["payment_receipt_receivable_account_code"],
		"1100-AR",
	)
	return []map[string]any{{
		"account_code": clearingAccount,
		"description":  "Cash / Clearing",
		"debit":        totalAmount,
		"credit":       0.0,
	}, {
		"account_code": receivableAccount,
		"description":  "Accounts Receivable",
		"debit":        0.0,
		"credit":       totalAmount,
	}}
}

func (s *CommercialCoreService) refundPostingLines(payload map[string]any) []map[string]any {
	totalAmount := roundMoney(numberValue(payload["amount_refunded"]))
	postingConfig := s.postingConfig()
	receivableAccount := firstNonEmptyString(
		textValue(payload["receivable_account_code"]),
		postingConfig["payment_refund_receivable_account_code"],
		postingConfig["payment_receipt_receivable_account_code"],
		"1100-AR",
	)
	clearingAccount := firstNonEmptyString(
		s.resolvePaymentClearingAccount(payload),
		postingConfig["payment_refund_clearing_account_code"],
		postingConfig["payment_receipt_clearing_account_code"],
		"1000-CASH",
	)
	return []map[string]any{{
		"account_code": receivableAccount,
		"description":  "Accounts Receivable",
		"debit":        totalAmount,
		"credit":       0.0,
	}, {
		"account_code": clearingAccount,
		"description":  "Cash / Clearing",
		"debit":        0.0,
		"credit":       totalAmount,
	}}
}

func (s *CommercialCoreService) postingConfig() map[string]string {
	defaults := map[string]string{
		"invoice_issue_receivable_account_code":   "1100-AR",
		"invoice_issue_revenue_account_code":      "4000-REV",
		"invoice_issue_tax_account_code":          "2100-TAX",
		"payment_receipt_clearing_account_code":   "1000-CASH",
		"payment_receipt_receivable_account_code": "1100-AR",
		"payment_refund_clearing_account_code":    "1000-CASH",
		"payment_refund_receivable_account_code":  "1100-AR",
	}
	if s.config == nil {
		return defaults
	}
	value, ok := s.config.Resolve("commercial.posting", "", "")
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

func (s *CommercialCoreService) normalizeCommercialLines(payload map[string]any) map[string]any {
	next := document.NormalizePayload(cloneMap(payload))
	next = s.applyPartyCommercialDefaults(next)
	priceListCode := textValue(next["price_list_code"])
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
		baseDate := firstNonEmptyString(textValue(next["invoice_date"]), textValue(next["order_date"]))
		if baseDate != "" && numberValue(next["payment_term_days"]) > 0 {
			if dueDate, ok := addDaysToDate(baseDate, int(numberValue(next["payment_term_days"]))); ok {
				next["due_date"] = dueDate
			}
		}
	}
	rows := recordList(next["lines"])
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
			normalized["uom_code"] = firstNonEmptyString(
				s.lookupAccountCode("commercial_item", "sku", itemCode, "uom_code"),
				s.lookupAccountCode("commercial_product", "code", textValue(normalized["product_code"]), "uom_code"),
			)
		}
		if numberValue(normalized["unit_price"]) == 0 {
			if to := s.lookupPriceListUnitPrice(priceListCode, itemCode); to > 0 {
				normalized["unit_price"] = to
			} else if to := s.lookupNumberValue("commercial_item", "sku", itemCode, "base_price"); to > 0 {
				normalized["unit_price"] = to
			} else if to := s.lookupNumberValue("commercial_item", "sku", itemCode, "unit_price"); to > 0 {
				normalized["unit_price"] = to
			} else if to := s.lookupProductNumberValue(textValue(normalized["product_code"]), "base_price"); to > 0 {
				normalized["unit_price"] = to
			} else if to := s.lookupProductNumberValue(textValue(normalized["product_code"]), "unit_price"); to > 0 {
				normalized["unit_price"] = to
			}
		}
		if textValue(normalized["tax_code"]) == "" {
			normalized["tax_code"] = firstNonEmptyString(
				defaultTaxCode,
				s.lookupAccountCode("commercial_price_list_item", "price_list_code|item_code", priceListCode+"|"+itemCode, "tax_code"),
				s.lookupAccountCode("commercial_item", "sku", itemCode, "tax_code"),
				s.lookupAccountCode("commercial_product", "code", textValue(normalized["product_code"]), "tax_code"),
			)
		}
		if textValue(normalized["revenue_account_code"]) == "" {
			normalized["revenue_account_code"] = firstNonEmptyString(
				s.lookupAccountCode("commercial_price_list_item", "price_list_code|item_code", priceListCode+"|"+itemCode, "revenue_account_code"),
				s.lookupAccountCode("commercial_item", "sku", itemCode, "revenue_account_code"),
				s.lookupAccountCode("commercial_product", "code", textValue(normalized["product_code"]), "revenue_account_code"),
			)
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
		normalized["unit_price"] = unitPrice
		normalized["discount_amount"] = discountAmount
		normalized["tax_rate"] = taxRate
		normalized["tax_mode"] = taxMode
		normalized["line_subtotal"] = lineSubtotal
		normalized["tax_amount"] = lineTax
		normalized["line_total"] = lineTotal
		subtotalAmount += lineSubtotal
		taxAmount += lineTax
		totalAmount += lineTotal
		normalizedRows = append(normalizedRows, normalized)
	}
	next["lines"] = normalizedRows
	next["subtotal_amount"] = roundMoney(subtotalAmount)
	next["tax_amount"] = roundMoney(taxAmount)
	next["total_amount"] = roundMoney(totalAmount)
	if strings.TrimSpace(textValue(next["currency_code"])) == "" {
		next["currency_code"] = "IDR"
	}
	return next
}

func (s *CommercialCoreService) resolveVariantItemCode(productCode, variantSignature string) string {
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

func orderedVariantDimensions(value any) []string {
	parts := strings.Split(textValue(value), ",")
	result := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		code := strings.TrimSpace(part)
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		result = append(result, code)
	}
	return result
}

func buildVariantValueCombos(order []string, valuesByDimension map[string][]model.Record) [][]model.Record {
	combos := [][]model.Record{{}}
	for _, dimensionCode := range order {
		values := valuesByDimension[dimensionCode]
		next := make([][]model.Record, 0, len(combos)*maxInt(len(values), 1))
		for _, combo := range combos {
			for _, value := range values {
				item := append([]model.Record{}, combo...)
				item = append(item, value)
				next = append(next, item)
			}
		}
		combos = next
	}
	return combos
}

func variantDescriptor(combo []model.Record) (signature, label, skuSuffix, valuesJSON string) {
	signatureParts := make([]string, 0, len(combo))
	labelParts := make([]string, 0, len(combo))
	skuParts := make([]string, 0, len(combo))
	jsonParts := make([]string, 0, len(combo))
	for _, value := range combo {
		dimensionCode := textValue(value.Values["dimension_code"])
		valueCode := textValue(value.Values["code"])
		valueName := firstNonEmptyString(textValue(value.Values["name"]), valueCode)
		signatureParts = append(signatureParts, dimensionCode+"="+valueCode)
		labelParts = append(labelParts, valueName)
		skuParts = append(skuParts, strings.ToUpper(strings.ReplaceAll(valueCode, " ", "_")))
		jsonParts = append(jsonParts, fmt.Sprintf(`{"dimension_code":"%s","value_code":"%s","value_name":"%s"}`, escapeJSONString(dimensionCode), escapeJSONString(valueCode), escapeJSONString(valueName)))
	}
	return strings.Join(signatureParts, "|"), strings.Join(labelParts, " / "), strings.Join(skuParts, "-"), "[" + strings.Join(jsonParts, ",") + "]"
}

func variantSKU(productCode, suffix string) string {
	productCode = strings.TrimSpace(productCode)
	suffix = strings.TrimSpace(suffix)
	if suffix == "" {
		return productCode
	}
	return productCode + "-" + suffix
}

func variantName(productName, label string) string {
	productName = strings.TrimSpace(productName)
	label = strings.TrimSpace(label)
	if productName == "" {
		return label
	}
	if label == "" {
		return productName
	}
	return productName + " / " + label
}

func escapeJSONString(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return replacer.Replace(value)
}

func boolFieldValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	default:
		return false
	}
}

func (s *CommercialCoreService) applyPartyCommercialDefaults(payload map[string]any) map[string]any {
	next := document.NormalizePayload(cloneMap(payload))
	partyID := textValue(next["party_id"])
	if partyID == "" {
		return next
	}
	if textValue(next["party_name"]) == "" {
		next["party_name"] = firstNonEmptyString(
			s.lookupModelValueByID("party", partyID, "display_name"),
			s.lookupModelValueByID("party", partyID, "name"),
		)
	}
	if textValue(next["currency_code"]) == "" {
		next["currency_code"] = s.lookupModelValueByID("party", partyID, "currency_code")
	}
	if textValue(next["tax_profile_code"]) == "" {
		next["tax_profile_code"] = s.lookupModelValueByID("party", partyID, "tax_profile_code")
	}
	if textValue(next["price_list_code"]) == "" {
		next["price_list_code"] = s.lookupModelValueByID("party", partyID, "default_price_list_code")
	}
	if numberValue(next["payment_term_days"]) == 0 {
		if days := s.lookupModelNumberByID("party", partyID, "payment_term_days"); days > 0 {
			next["payment_term_days"] = days
		}
	}
	return next
}

func commercialTaxBreakdown(grossAmount, taxRate float64, taxMode string) (float64, float64, float64) {
	mode := strings.ToLower(strings.TrimSpace(taxMode))
	switch mode {
	case "inclusive":
		if taxRate <= 0 {
			return roundMoney(grossAmount), 0, roundMoney(grossAmount)
		}
		subtotal := roundMoney(grossAmount / (1 + taxRate/100))
		tax := roundMoney(grossAmount - subtotal)
		return subtotal, tax, roundMoney(grossAmount)
	case "exempt":
		return roundMoney(grossAmount), 0, roundMoney(grossAmount)
	default:
		subtotal := roundMoney(grossAmount)
		tax := roundMoney(subtotal * taxRate / 100)
		return subtotal, tax, roundMoney(subtotal + tax)
	}
}

func (s *CommercialCoreService) normalizeCommercialAllocations(payload map[string]any) map[string]any {
	next := document.NormalizePayload(cloneMap(payload))
	rows := recordList(next["allocations"])
	normalizedRows := make([]map[string]any, 0, len(rows))
	appliedAmount := 0.0
	for _, row := range rows {
		normalized := cloneMap(row)
		invoiceID := textValue(normalized["invoice_id"])
		if invoiceID != "" {
			if invoice, err := s.documents.Get(invoiceID); err == nil && invoice.Header.Type == "invoice" {
				if textValue(normalized["invoice_number"]) == "" {
					normalized["invoice_number"] = firstNonEmptyString(invoice.Header.Number, invoice.Header.ID)
				}
				if amount := roundMoney(numberValue(normalized["amount"])); amount == 0 {
					openAmount := roundMoney(numberValue(invoice.Body.Payload["balance_due_amount"]))
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
	amountReceived := roundMoney(numberValue(next["amount_received"]))
	if amountReceived <= 0 && appliedAmount > 0 {
		amountReceived = roundMoney(appliedAmount)
	}
	next["allocations"] = normalizedRows
	next["amount_received"] = amountReceived
	next["unapplied_amount"] = roundMoney(maxFloat(amountReceived-appliedAmount, 0))
	if methodCode := textValue(next["payment_method_code"]); methodCode != "" && textValue(next["clearing_account_code"]) == "" {
		next["clearing_account_code"] = s.lookupAccountCode("payment_method", "code", methodCode, "clearing_account_code")
	}
	return next
}

func (s *CommercialCoreService) normalizeCommercialRefund(payload map[string]any) map[string]any {
	next := document.NormalizePayload(cloneMap(payload))
	if methodCode := textValue(next["payment_method_code"]); methodCode != "" && textValue(next["clearing_account_code"]) == "" {
		next["clearing_account_code"] = s.lookupAccountCode("payment_method", "code", methodCode, "clearing_account_code")
	}
	rows := recordList(next["refund_allocations"])
	normalizedRows := make([]map[string]any, 0, len(rows))
	totalRefunded := 0.0
	for _, row := range rows {
		normalized := cloneMap(row)
		paymentID := textValue(normalized["payment_id"])
		if paymentID != "" {
			if payment, err := s.documents.Get(paymentID); err == nil && payment.Header.Type == "payment_receipt" {
				if textValue(normalized["payment_number"]) == "" {
					normalized["payment_number"] = firstNonEmptyString(payment.Header.Number, payment.Header.ID)
				}
				if amount := roundMoney(numberValue(normalized["amount"])); amount == 0 {
					remaining := roundMoney(numberValue(payment.Body.Payload["amount_received"]) - numberValue(payment.Body.Payload["refunded_amount"]))
					if remaining > 0 {
						normalized["amount"] = remaining
					}
				}
			}
		}
		amount := roundMoney(numberValue(normalized["amount"]))
		normalized["amount"] = amount
		totalRefunded += amount
		normalizedRows = append(normalizedRows, normalized)
	}
	if len(normalizedRows) > 0 {
		next["refund_allocations"] = normalizedRows
		if len(normalizedRows) == 1 {
			next["source_payment_id"] = textValue(normalizedRows[0]["payment_id"])
			next["source_payment_number"] = textValue(normalizedRows[0]["payment_number"])
		} else {
			next["source_payment_id"] = ""
			next["source_payment_number"] = ""
		}
	}
	refundAmount := roundMoney(numberValue(next["amount_refunded"]))
	if len(normalizedRows) > 0 {
		refundAmount = roundMoney(totalRefunded)
	}
	next["amount_refunded"] = refundAmount
	return next
}

func buildRefundAllocationRows(payments []refundablePaymentOption, requestedAmount float64) []map[string]any {
	remaining := roundMoney(requestedAmount)
	allocateAll := remaining <= 0
	rows := make([]map[string]any, 0)
	for _, payment := range payments {
		if payment.remainingAmount <= 0 {
			continue
		}
		allocationAmount := payment.remainingAmount
		if !allocateAll && remaining < allocationAmount {
			allocationAmount = roundMoney(remaining)
		}
		if allocationAmount <= 0 {
			continue
		}
		rows = append(rows, map[string]any{
			"payment_id":     payment.record.Header.ID,
			"payment_number": firstNonEmptyString(payment.record.Header.Number, payment.record.Header.ID),
			"amount":         allocationAmount,
			"note":           "",
		})
		if !allocateAll {
			remaining = roundMoney(maxFloat(remaining-allocationAmount, 0))
			if remaining <= 0 {
				break
			}
		}
	}
	return rows
}

func refundAllocationsFromPayload(payload map[string]any) []refundAllocation {
	rows := recordList(payload["refund_allocations"])
	allocations := make([]refundAllocation, 0, len(rows))
	for _, row := range rows {
		paymentID := textValue(row["payment_id"])
		amount := roundMoney(numberValue(row["amount"]))
		if paymentID == "" || amount <= 0 {
			continue
		}
		allocations = append(allocations, refundAllocation{
			paymentID:     paymentID,
			paymentNumber: textValue(row["payment_number"]),
			amount:        amount,
			note:          textValue(row["note"]),
		})
	}
	if len(allocations) > 0 {
		return allocations
	}
	sourcePaymentID := textValue(payload["source_payment_id"])
	refundAmount := roundMoney(numberValue(payload["amount_refunded"]))
	if sourcePaymentID != "" && refundAmount > 0 {
		return []refundAllocation{{
			paymentID:     sourcePaymentID,
			paymentNumber: textValue(payload["source_payment_number"]),
			amount:        refundAmount,
		}}
	}
	return nil
}

func (s *CommercialCoreService) normalizeJournalLines(payload map[string]any) map[string]any {
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

func reverseJournalLines(rows []map[string]any) []map[string]any {
	reversed := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		next := cloneMap(row)
		debit := roundMoney(numberValue(row["debit"]))
		credit := roundMoney(numberValue(row["credit"]))
		next["debit"] = credit
		next["credit"] = debit
		reversed = append(reversed, next)
	}
	return reversed
}

func commercialLineTotals(lines []map[string]any) (float64, float64, float64) {
	subtotal := 0.0
	tax := 0.0
	total := 0.0
	for _, line := range lines {
		subtotal += roundMoney(numberValue(line["line_subtotal"]))
		tax += roundMoney(numberValue(line["tax_amount"]))
		total += roundMoney(numberValue(line["line_total"]))
	}
	return roundMoney(subtotal), roundMoney(tax), roundMoney(total)
}

func scaledCommercialLines(lines []map[string]any, targetTotal float64, sourceTotal float64) []map[string]any {
	cloned := recordList(lines)
	if len(cloned) == 0 {
		return cloned
	}
	if sourceTotal <= 0 || targetTotal <= 0 {
		return cloned
	}
	if targetTotal >= sourceTotal {
		return cloned
	}
	ratio := targetTotal / sourceTotal
	scaled := make([]map[string]any, 0, len(cloned))
	runningTotal := 0.0
	for index, line := range cloned {
		next := cloneMap(line)
		if index == len(cloned)-1 {
			remaining := roundMoney(maxFloat(targetTotal-runningTotal, 0))
			sourceLineTotal := roundMoney(numberValue(next["line_total"]))
			sourceLineSubtotal := roundMoney(numberValue(next["line_subtotal"]))
			sourceLineTax := roundMoney(numberValue(next["tax_amount"]))
			if sourceLineTotal > 0 {
				lineRatio := remaining / sourceLineTotal
				sub := roundMoney(sourceLineSubtotal * lineRatio)
				tax := roundMoney(remaining - sub)
				if sourceLineTax > 0 {
					tax = roundMoney(sourceLineTax * lineRatio)
					sub = roundMoney(remaining - tax)
				}
				next["line_subtotal"] = sub
				next["tax_amount"] = tax
				next["line_total"] = remaining
			} else {
				next["line_subtotal"] = remaining
				next["tax_amount"] = 0.0
				next["line_total"] = remaining
			}
			scaled = append(scaled, next)
			break
		}
		sub := roundMoney(numberValue(next["line_subtotal"]) * ratio)
		tax := roundMoney(numberValue(next["tax_amount"]) * ratio)
		total := roundMoney(numberValue(next["line_total"]) * ratio)
		next["line_subtotal"] = sub
		next["tax_amount"] = tax
		next["line_total"] = total
		scaled = append(scaled, next)
		runningTotal = roundMoney(runningTotal + total)
	}
	return scaled
}

func (s *CommercialCoreService) resolveRevenueAccount(payload map[string]any) string {
	if account := textValue(payload["revenue_account_code"]); account != "" {
		return account
	}
	lines := recordList(payload["lines"])
	for _, line := range lines {
		if account := textValue(line["revenue_account_code"]); account != "" {
			return account
		}
		if account := s.lookupAccountCode("commercial_item", "sku", textValue(line["item_code"]), "revenue_account_code"); account != "" {
			return account
		}
	}
	return ""
}

func (s *CommercialCoreService) resolveTaxAccount(payload map[string]any) string {
	if account := textValue(payload["tax_account_code"]); account != "" {
		return account
	}
	if account := s.lookupAccountCode("commercial_tax_code", "code", textValue(payload["tax_code"]), "tax_account_code"); account != "" {
		return account
	}
	lines := recordList(payload["lines"])
	for _, line := range lines {
		if account := textValue(line["tax_account_code"]); account != "" {
			return account
		}
		if account := s.lookupAccountCode("commercial_tax_code", "code", textValue(line["tax_code"]), "tax_account_code"); account != "" {
			return account
		}
		if taxCode := s.lookupAccountCode("commercial_item", "sku", textValue(line["item_code"]), "tax_code"); taxCode != "" {
			if account := s.lookupAccountCode("commercial_tax_code", "code", taxCode, "tax_account_code"); account != "" {
				return account
			}
		}
	}
	return ""
}

func (s *CommercialCoreService) resolvePaymentClearingAccount(payload map[string]any) string {
	if account := textValue(payload["clearing_account_code"]); account != "" {
		return account
	}
	return s.lookupAccountCode("payment_method", "code", textValue(payload["payment_method_code"]), "clearing_account_code")
}

func (s *CommercialCoreService) lookupAccountCode(modelKey, filterKey, filterValue, valueKey string) string {
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

func (s *CommercialCoreService) lookupNumberValue(modelKey, filterKey, filterValue, valueKey string) float64 {
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

func (s *CommercialCoreService) lookupProductNumberValue(productCode, valueKey string) float64 {
	return s.lookupNumberValue("commercial_product", "code", productCode, valueKey)
}

func (s *CommercialCoreService) lookupModelValueByID(modelKey, id, valueKey string) string {
	if s.models == nil || strings.TrimSpace(id) == "" {
		return ""
	}
	record, err := s.models.Get(modelKey, id)
	if err != nil {
		return ""
	}
	return textValue(record.Values[valueKey])
}

func (s *CommercialCoreService) lookupModelNumberByID(modelKey, id, valueKey string) float64 {
	if s.models == nil || strings.TrimSpace(id) == "" {
		return 0
	}
	record, err := s.models.Get(modelKey, id)
	if err != nil {
		return 0
	}
	return roundMoney(numberValue(record.Values[valueKey]))
}

func (s *CommercialCoreService) lookupPriceListUnitPrice(priceListCode, itemCode string) float64 {
	if s.models == nil || strings.TrimSpace(priceListCode) == "" || strings.TrimSpace(itemCode) == "" {
		return 0
	}
	items, _, err := s.models.List("commercial_price_list_item", model.Query{
		Filters: map[string]string{
			"price_list_code": strings.TrimSpace(priceListCode),
			"item_code":       strings.TrimSpace(itemCode),
		},
		Page:     1,
		PageSize: 1,
	})
	if err != nil || len(items) == 0 {
		return 0
	}
	return roundMoney(numberValue(items[0].Values["unit_price"]))
}

func lookupFilters(filterKey, filterValue string) map[string]string {
	if !strings.Contains(filterKey, "|") {
		return map[string]string{filterKey: filterValue}
	}
	keys := strings.Split(filterKey, "|")
	values := strings.Split(filterValue, "|")
	filters := map[string]string{}
	for index, key := range keys {
		if index < len(values) && strings.TrimSpace(key) != "" {
			filters[key] = values[index]
		}
	}
	return filters
}

func recordList(value any) []map[string]any {
	items, ok := value.([]any)
	if !ok {
		if typed, ok := value.([]map[string]any); ok {
			rows := make([]map[string]any, 0, len(typed))
			for _, row := range typed {
				rows = append(rows, cloneMap(row))
			}
			return rows
		}
		return nil
	}
	rows := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if row, ok := item.(map[string]any); ok {
			rows = append(rows, cloneMap(row))
		}
	}
	return rows
}

func clonedPayload(input map[string]any) map[string]any {
	return document.NormalizePayload(cloneMap(input))
}

func derivedRecordAmount(payload map[string]any) float64 {
	if total := roundMoney(numberValue(payload["total_amount"])); total > 0 {
		return total
	}
	return roundMoney(numberValue(payload["amount_received"]))
}

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func textValue(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func numberValue(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case int32:
		return float64(typed)
	case string:
		if strings.TrimSpace(typed) == "" {
			return 0
		}
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

func roundMoney(value float64) float64 {
	return float64(int(value*100+0.5)) / 100
}

func addDaysToDate(base string, days int) (string, bool) {
	if strings.TrimSpace(base) == "" || days <= 0 {
		return "", false
	}
	parsed, err := time.Parse("2006-01-02", base)
	if err != nil {
		return "", false
	}
	return parsed.AddDate(0, 0, days).Format("2006-01-02"), true
}

func moneyMinor(value float64) int64 {
	return int64(roundMoney(value) * 100)
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func dateDiffDays(from string, to string) int {
	if strings.TrimSpace(from) == "" || strings.TrimSpace(to) == "" {
		return 0
	}
	fromTime, err := time.Parse("2006-01-02", from)
	if err != nil {
		return 0
	}
	toTime, err := time.Parse("2006-01-02", to)
	if err != nil {
		return 0
	}
	return int(toTime.Sub(fromTime).Hours() / 24)
}

func isConflict(err error) bool {
	var platformErr shared.Error
	return errors.As(err, &platformErr) && platformErr.Kind == shared.KindConflict
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (s *CommercialCoreService) refreshDocuments(records ...document.Record) {
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
