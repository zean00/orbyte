package application

import (
	"testing"
	"time"

	"orbyte/internal/platform/config"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/search"
)

func TestGenerateInvoiceFromConfirmedOrder(t *testing.T) {
	docs := document.NewService()
	mustRegisterCommercialDocumentTypes(t, docs)
	service := NewCommercialCoreService(docs, nil, nil, nil)

	order, err := docs.Create("sales_order", "org_default", "loc_main", "user_admin", map[string]any{
		"party_id":          "party_1",
		"party_name":        "Walk In Customer",
		"currency_code":     "USD",
		"payment_term_days": 14.0,
		"subtotal_amount":   200.0,
		"tax_amount":        20.0,
		"total_amount":      220.0,
		"lines": []map[string]any{{
			"item_code":     "CONSULT",
			"description":   "Consultation",
			"quantity":      2.0,
			"unit_price":    100.0,
			"tax_rate":      10.0,
			"line_total":    220.0,
			"line_subtotal": 200.0,
			"tax_amount":    20.0,
		}},
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	order.Header.Status = "confirmed"
	order.Header.Number = "SO-001"
	order.Header.UpdatedAt = time.Now().UTC()
	if err := docs.Save(order); err != nil {
		t.Fatalf("save order: %v", err)
	}

	invoice, err := service.GenerateInvoiceFromOrder(order.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("generate invoice: %v", err)
	}
	if invoice.Header.Type != "invoice" {
		t.Fatalf("expected invoice type, got %s", invoice.Header.Type)
	}
	if got := invoice.Body.Payload["source_order_id"]; got != order.Header.ID {
		t.Fatalf("expected source order id %s, got %v", order.Header.ID, got)
	}
	if got := invoice.Body.Payload["balance_due_amount"]; got != 220.0 {
		t.Fatalf("expected balance due 220, got %v", got)
	}
	if got := numberValue(invoice.Body.Payload["payment_term_days"]); got != 14.0 {
		t.Fatalf("expected payment term days 14, got %v", got)
	}
	if len(invoice.Links) == 0 {
		t.Fatalf("expected generated invoice to have links")
	}
}

func TestGenerateInvoiceFromOrderPreservesDiscountedLines(t *testing.T) {
	models := model.NewService()
	docs := document.NewService()
	mustRegisterCommercialModels(t, models)
	mustRegisterCommercialDocumentTypes(t, docs)
	service := NewCommercialCoreService(docs, config.NewService(), models, nil)

	party, err := models.Create("party", "user_admin", map[string]any{
		"name":          "Member Customer",
		"member_status": "active",
		"status":        "active",
	})
	if err != nil {
		t.Fatalf("create party: %v", err)
	}
	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                     "DISC-ITEM",
		"name":                    "Discounted Item",
		"kind":                    "simple",
		"item_type":               "product",
		"inventory_enabled":       true,
		"inventory_tracking_mode": "quantity",
		"unit_price":              100,
		"tax_code":                "VAT11",
		"revenue_account_code":    "4000-REV",
		"status":                  "active",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	if _, err := models.Create("commercial_tax_code", "user_admin", map[string]any{
		"code":             "VAT11",
		"rate_percent":     11.0,
		"mode":             "exclusive",
		"tax_account_code": "2100-VATOUT",
		"status":           "active",
	}); err != nil {
		t.Fatalf("create tax code: %v", err)
	}
	if _, err := models.Create("discount_rule", "user_admin", map[string]any{
		"code":             "DISC10",
		"name":             "Disc 10",
		"scope":            "line",
		"rule_kind":        "line_percent",
		"item_codes":       "DISC-ITEM",
		"member_statuses":  "active",
		"discount_percent": 10,
		"status":           "active",
	}); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	orderPayload := service.NormalizePayload("sales_order", map[string]any{
		"party_id":   party.ID,
		"order_date": "2026-03-29",
		"lines": []map[string]any{{
			"item_code": "DISC-ITEM",
			"quantity":  2.0,
		}},
	})
	order, err := docs.Create("sales_order", "org_default", "loc_main", "user_admin", orderPayload)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	order.Header.Status = "confirmed"
	order.Header.Number = "SO-DISC-001"
	order.Header.UpdatedAt = time.Now().UTC()
	if err := docs.Save(order); err != nil {
		t.Fatalf("save order: %v", err)
	}

	invoice, err := service.GenerateInvoiceFromOrder(order.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("generate invoice: %v", err)
	}

	orderLine := recordList(order.Body.Payload["lines"])[0]
	invoiceLine := recordList(invoice.Body.Payload["lines"])[0]
	if got, want := numberValue(invoice.Body.Payload["total_amount"]), numberValue(order.Body.Payload["total_amount"]); got != want {
		t.Fatalf("expected invoice total %v to match order total %v", got, want)
	}
	if got, want := numberValue(invoice.Body.Payload["discount_amount_total"]), numberValue(order.Body.Payload["discount_amount_total"]); got != want {
		t.Fatalf("expected invoice discount total %v to match order discount total %v", got, want)
	}
	if got, want := numberValue(invoiceLine["discount_amount"]), numberValue(orderLine["discount_amount"]); got != want {
		t.Fatalf("expected invoice line discount %v to match order line discount %v", got, want)
	}
	if got, want := numberValue(invoiceLine["auto_discount_amount"]), numberValue(orderLine["auto_discount_amount"]); got != want {
		t.Fatalf("expected invoice auto discount %v to match order auto discount %v", got, want)
	}
}

func TestNormalizeCommercialLinesAppliesPromotionCodeForScopedCampaign(t *testing.T) {
	models := model.NewService()
	docs := document.NewService()
	mustRegisterCommercialModels(t, models)
	mustRegisterCommercialDocumentTypes(t, docs)
	service := NewCommercialCoreService(docs, config.NewService(), models, nil)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                  "PROMO-ITEM",
		"name":                 "Promo Item",
		"kind":                 "simple",
		"item_type":            "product",
		"unit_price":           100,
		"tax_code":             "VAT11",
		"revenue_account_code": "4000-REV",
		"status":               "active",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	if _, err := models.Create("commercial_tax_code", "user_admin", map[string]any{
		"code":             "VAT11",
		"name":             "VAT 11",
		"rate_percent":     11.0,
		"mode":             "exclusive",
		"tax_account_code": "2100-VATOUT",
		"status":           "active",
	}); err != nil {
		t.Fatalf("create tax code: %v", err)
	}
	if _, err := models.Create("promotion_campaign", "user_admin", map[string]any{
		"code":           "LUNCH",
		"name":           "Lunch Promo",
		"trigger_mode":   "code",
		"sales_channels": "pos",
		"store_codes":    "STORE1",
		"status":         "active",
	}); err != nil {
		t.Fatalf("create campaign: %v", err)
	}
	if _, err := models.Create("promotion_code", "user_admin", map[string]any{
		"code":                    "LUNCH10",
		"promotion_campaign_code": "LUNCH",
		"status":                  "active",
	}); err != nil {
		t.Fatalf("create code: %v", err)
	}
	if _, err := models.Create("discount_rule", "user_admin", map[string]any{
		"code":                    "PROMO-LINE10",
		"name":                    "Promo Line 10",
		"promotion_campaign_code": "LUNCH",
		"scope":                   "line",
		"rule_kind":               "line_percent",
		"item_codes":              "PROMO-ITEM",
		"discount_percent":        10,
		"status":                  "active",
	}); err != nil {
		t.Fatalf("create discount rule: %v", err)
	}

	normalized := service.NormalizePayload("sales_order", map[string]any{
		"sales_channel":   "pos",
		"store_code":      "STORE1",
		"promotion_codes": []string{"LUNCH10"},
		"order_datetime":  "2026-03-29T12:30:00+07:00",
		"lines": []map[string]any{{
			"item_code": "PROMO-ITEM",
			"quantity":  1,
		}},
	})
	if got := numberValue(normalized["discount_amount_total"]); got != 10 {
		t.Fatalf("expected promotion discount 10, got %v", got)
	}
	if len(recordList(normalized["promotion_breakdown"])) == 0 {
		t.Fatal("expected promotion breakdown on normalized payload")
	}
	line := firstRecord(normalized["lines"])
	if got := textValue(firstRecord(recordList(line["promotion_breakdown"]))["promotion_code"]); got != "LUNCH10" {
		t.Fatalf("expected applied promo code LUNCH10, got %q", got)
	}
}

func TestRecordPromotionRedemptionsSumsMatchingPromotionDiscounts(t *testing.T) {
	models := model.NewService()
	docs := document.NewService()
	mustRegisterCommercialModels(t, models)
	mustRegisterCommercialDocumentTypes(t, docs)
	service := NewCommercialCoreService(docs, config.NewService(), models, nil)

	invoice, err := docs.Create("invoice", "org_default", "loc_main", "user_admin", map[string]any{
		"party_id":      "party_1",
		"sales_channel": "pos",
		"store_code":    "STORE1",
		"promotion_breakdown": []map[string]any{
			{
				"rules": []map[string]any{
					{
						"promotion_campaign_code": "PROMO1",
						"promotion_code":          "PROMO-CODE",
						"discount_value":          10.0,
					},
				},
			},
			{
				"rules": []map[string]any{
					{
						"promotion_campaign_code": "PROMO1",
						"promotion_code":          "PROMO-CODE",
						"discount_value":          5.0,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}

	if err := service.recordPromotionRedemptions(invoice, "user_admin"); err != nil {
		t.Fatalf("record promotion redemptions: %v", err)
	}

	items, _, err := models.List("promotion_redemption", model.Query{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list redemptions: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 promotion redemption, got %d", len(items))
	}
	if got := numberValue(items[0].Values["discount_amount_total"]); got != 15 {
		t.Fatalf("expected aggregated redemption discount 15, got %v", got)
	}
}

func TestGenerateAndApproveCreditNoteFromIssuedInvoice(t *testing.T) {
	docs := document.NewService()
	mustRegisterCommercialDocumentTypes(t, docs)
	service := NewCommercialCoreService(docs, nil, nil, nil)

	invoice, err := docs.Create("invoice", "org_default", "loc_main", "user_admin", map[string]any{
		"party_id":                "party_1",
		"party_name":              "Walk In Customer",
		"currency_code":           "USD",
		"invoice_date":            "2026-03-27",
		"due_date":                "2026-04-26",
		"subtotal_amount":         200.0,
		"tax_amount":              20.0,
		"total_amount":            220.0,
		"paid_amount":             0.0,
		"balance_due_amount":      220.0,
		"receivable_account_code": "1100-AR",
		"lines": []map[string]any{{
			"item_code":     "CONSULT",
			"description":   "Consultation",
			"quantity":      2.0,
			"unit_price":    100.0,
			"tax_rate":      10.0,
			"line_total":    220.0,
			"line_subtotal": 200.0,
			"tax_amount":    20.0,
		}},
	})
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	invoice.Header.Status = "issued"
	invoice.Header.Number = "INV-CN-001"
	if err := docs.Save(invoice); err != nil {
		t.Fatalf("save invoice: %v", err)
	}
	if err := service.HandleApprovedDocument(invoice, "user_admin"); err != nil {
		t.Fatalf("handle issued invoice: %v", err)
	}

	creditNote, err := service.CreateCreditNoteFromInvoice(invoice.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("create credit note: %v", err)
	}
	if creditNote.Header.Type != "credit_note" {
		t.Fatalf("expected credit_note type, got %s", creditNote.Header.Type)
	}
	creditNote.Header.Status = "issued"
	if err := docs.Save(creditNote); err != nil {
		t.Fatalf("save credit note: %v", err)
	}
	if err := service.HandleApprovedDocument(creditNote, "user_admin"); err != nil {
		t.Fatalf("handle issued credit note: %v", err)
	}

	updatedInvoice, err := docs.Get(invoice.Header.ID)
	if err != nil {
		t.Fatalf("reload invoice: %v", err)
	}
	if updatedInvoice.Header.Status != "cancelled" {
		t.Fatalf("expected invoice cancelled after full credit, got %s", updatedInvoice.Header.Status)
	}
	if got := numberValue(updatedInvoice.Body.Payload["credited_amount"]); got != 220 {
		t.Fatalf("expected credited amount 220, got %v", got)
	}
	if got := numberValue(updatedInvoice.Body.Payload["balance_due_amount"]); got != 0 {
		t.Fatalf("expected balance due 0, got %v", got)
	}

	postings := 0
	for _, item := range docs.List() {
		if item.Header.Type == "ledger_posting" {
			postings++
		}
	}
	if postings != 2 {
		t.Fatalf("expected 2 ledger postings after credit note, got %d", postings)
	}
}

func TestCreatePaymentReceiptFromInvoiceCopiesReceivableAccount(t *testing.T) {
	docs := document.NewService()
	mustRegisterCommercialDocumentTypes(t, docs)
	service := NewCommercialCoreService(docs, nil, nil, nil)

	invoice, err := docs.Create("invoice", "org_default", "loc_main", "user_admin", map[string]any{
		"party_id":                "party_1",
		"party_name":              "Walk In Customer",
		"currency_code":           "USD",
		"invoice_date":            "2026-03-29",
		"due_date":                "2026-04-28",
		"subtotal_amount":         36.0,
		"tax_amount":              3.96,
		"total_amount":            39.96,
		"paid_amount":             0.0,
		"balance_due_amount":      39.96,
		"receivable_account_code": "1105-AR-TRADE",
		"lines": []map[string]any{{
			"item_code":            "BURGER",
			"description":          "Burger",
			"quantity":             3.0,
			"unit_price":           12.0,
			"tax_rate":             11.0,
			"line_subtotal":        36.0,
			"tax_amount":           3.96,
			"line_total":           39.96,
			"revenue_account_code": "4000-REV",
			"tax_account_code":     "2100-VAT",
		}},
	})
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	invoice.Header.Status = "issued"
	invoice.Header.Number = "INV-PAY-001"
	if err := docs.Save(invoice); err != nil {
		t.Fatalf("save invoice: %v", err)
	}

	payment, err := service.CreatePaymentReceiptFromInvoice(invoice.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("create payment receipt: %v", err)
	}
	if got := textValue(payment.Body.Payload["receivable_account_code"]); got != "1105-AR-TRADE" {
		t.Fatalf("expected receivable account copied from invoice, got %s", got)
	}
}

func TestGenerateAndApprovePartialCreditNoteFromPartiallyPaidInvoice(t *testing.T) {
	docs := document.NewService()
	mustRegisterCommercialDocumentTypes(t, docs)
	service := NewCommercialCoreService(docs, nil, nil, nil)

	invoice, err := docs.Create("invoice", "org_default", "loc_main", "user_admin", map[string]any{
		"party_id":                "party_1",
		"party_name":              "Walk In Customer",
		"currency_code":           "USD",
		"invoice_date":            "2026-03-27",
		"due_date":                "2026-04-26",
		"subtotal_amount":         200.0,
		"tax_amount":              20.0,
		"total_amount":            220.0,
		"paid_amount":             100.0,
		"balance_due_amount":      120.0,
		"receivable_account_code": "1100-AR",
		"lines": []map[string]any{{
			"item_code":     "CONSULT",
			"description":   "Consultation",
			"quantity":      2.0,
			"unit_price":    100.0,
			"tax_rate":      10.0,
			"line_total":    220.0,
			"line_subtotal": 200.0,
			"tax_amount":    20.0,
		}},
	})
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	invoice.Header.Status = "partially_paid"
	invoice.Header.Number = "INV-CN-002"
	if err := docs.Save(invoice); err != nil {
		t.Fatalf("save invoice: %v", err)
	}
	if err := service.HandleApprovedDocument(invoice, "user_admin"); err != nil {
		t.Fatalf("handle invoice posting: %v", err)
	}

	creditNote, err := service.CreateCreditNoteFromInvoice(invoice.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("create partial credit note: %v", err)
	}
	if got := numberValue(creditNote.Body.Payload["total_amount"]); got != 120 {
		t.Fatalf("expected credit note total 120, got %v", got)
	}
	creditNote.Header.Status = "issued"
	if err := docs.Save(creditNote); err != nil {
		t.Fatalf("save credit note: %v", err)
	}
	if err := service.HandleApprovedDocument(creditNote, "user_admin"); err != nil {
		t.Fatalf("approve partial credit note: %v", err)
	}

	updatedInvoice, err := docs.Get(invoice.Header.ID)
	if err != nil {
		t.Fatalf("reload invoice: %v", err)
	}
	if updatedInvoice.Header.Status != "paid" {
		t.Fatalf("expected invoice paid after payment + credit settles balance, got %s", updatedInvoice.Header.Status)
	}
	if got := numberValue(updatedInvoice.Body.Payload["credited_amount"]); got != 120 {
		t.Fatalf("expected credited amount 120, got %v", got)
	}
	if got := numberValue(updatedInvoice.Body.Payload["balance_due_amount"]); got != 0 {
		t.Fatalf("expected balance due 0, got %v", got)
	}
}

func TestGenerateAndApproveRefundFromCreditNoteOnPaidInvoice(t *testing.T) {
	docs := document.NewService()
	mustRegisterCommercialDocumentTypes(t, docs)
	service := NewCommercialCoreService(docs, nil, nil, nil)

	invoice, err := docs.Create("invoice", "org_default", "loc_main", "user_admin", map[string]any{
		"party_id":                "party_1",
		"party_name":              "Walk In Customer",
		"currency_code":           "USD",
		"invoice_date":            "2026-03-27",
		"due_date":                "2026-04-26",
		"subtotal_amount":         200.0,
		"tax_amount":              20.0,
		"total_amount":            220.0,
		"paid_amount":             220.0,
		"balance_due_amount":      0.0,
		"receivable_account_code": "1100-AR",
		"lines": []map[string]any{{
			"item_code":     "CONSULT",
			"description":   "Consultation",
			"quantity":      2.0,
			"unit_price":    100.0,
			"tax_rate":      10.0,
			"line_total":    220.0,
			"line_subtotal": 200.0,
			"tax_amount":    20.0,
		}},
	})
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	invoice.Header.Status = "paid"
	invoice.Header.Number = "INV-RF-001"
	if err := docs.Save(invoice); err != nil {
		t.Fatalf("save invoice: %v", err)
	}
	if err := service.HandleApprovedDocument(invoice, "user_admin"); err != nil {
		t.Fatalf("handle invoice posting: %v", err)
	}

	creditNote, err := service.CreateCreditNoteFromInvoice(invoice.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("create credit note from paid invoice: %v", err)
	}
	if got := numberValue(creditNote.Body.Payload["total_amount"]); got != 220 {
		t.Fatalf("expected paid-invoice credit note total 220, got %v", got)
	}
	creditNote.Header.Status = "issued"
	creditNote.Header.Number = "CN-RF-001"
	if err := docs.Save(creditNote); err != nil {
		t.Fatalf("save credit note: %v", err)
	}
	if err := service.HandleApprovedDocument(creditNote, "user_admin"); err != nil {
		t.Fatalf("approve credit note: %v", err)
	}

	refund, err := service.CreateRefundFromCreditNote(creditNote.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("create refund: %v", err)
	}
	if refund.Header.Type != "payment_refund" {
		t.Fatalf("expected payment_refund type, got %s", refund.Header.Type)
	}
	if got := numberValue(refund.Body.Payload["amount_refunded"]); got != 220 {
		t.Fatalf("expected refund amount 220, got %v", got)
	}
	refund.Header.Status = "refunded"
	refund.Header.Number = "RF-001"
	if err := docs.Save(refund); err != nil {
		t.Fatalf("save refund: %v", err)
	}
	if err := service.HandleApprovedDocument(refund, "user_admin"); err != nil {
		t.Fatalf("approve refund: %v", err)
	}

	updatedInvoice, err := docs.Get(invoice.Header.ID)
	if err != nil {
		t.Fatalf("reload invoice: %v", err)
	}
	if updatedInvoice.Header.Status != "refunded" {
		t.Fatalf("expected invoice refunded after full refund, got %s", updatedInvoice.Header.Status)
	}
	if got := numberValue(updatedInvoice.Body.Payload["credited_amount"]); got != 220 {
		t.Fatalf("expected credited amount 220, got %v", got)
	}
	if got := numberValue(updatedInvoice.Body.Payload["refunded_amount"]); got != 220 {
		t.Fatalf("expected refunded amount 220, got %v", got)
	}
	updatedCreditNote, err := docs.Get(creditNote.Header.ID)
	if err != nil {
		t.Fatalf("reload credit note: %v", err)
	}
	if got := numberValue(updatedCreditNote.Body.Payload["refunded_amount"]); got != 220 {
		t.Fatalf("expected credit note refunded amount 220, got %v", got)
	}

	postings := 0
	for _, item := range docs.List() {
		if item.Header.Type == "ledger_posting" {
			postings++
		}
	}
	if postings != 3 {
		t.Fatalf("expected 3 ledger postings after invoice, credit note, and refund, got %d", postings)
	}
}

func TestPartialRefundTracksCreditNoteAndSeedsRemainingRefund(t *testing.T) {
	docs := document.NewService()
	mustRegisterCommercialDocumentTypes(t, docs)
	service := NewCommercialCoreService(docs, nil, nil, nil)

	invoice, err := docs.Create("invoice", "org_default", "loc_main", "user_admin", map[string]any{
		"party_id":                "party_1",
		"party_name":              "Walk In Customer",
		"currency_code":           "USD",
		"invoice_date":            "2026-03-27",
		"due_date":                "2026-04-26",
		"subtotal_amount":         200.0,
		"tax_amount":              20.0,
		"total_amount":            220.0,
		"paid_amount":             220.0,
		"balance_due_amount":      0.0,
		"receivable_account_code": "1100-AR",
		"lines": []map[string]any{{
			"item_code":     "CONSULT",
			"description":   "Consultation",
			"quantity":      2.0,
			"unit_price":    100.0,
			"tax_rate":      10.0,
			"line_total":    220.0,
			"line_subtotal": 200.0,
			"tax_amount":    20.0,
		}},
	})
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	invoice.Header.Status = "paid"
	invoice.Header.Number = "INV-RF-002"
	if err := docs.Save(invoice); err != nil {
		t.Fatalf("save invoice: %v", err)
	}
	if err := service.HandleApprovedDocument(invoice, "user_admin"); err != nil {
		t.Fatalf("handle invoice posting: %v", err)
	}

	payment, err := docs.Create("payment_receipt", "org_default", "loc_main", "user_admin", map[string]any{
		"party_id":              "party_1",
		"party_name":            "Walk In Customer",
		"currency_code":         "USD",
		"receipt_date":          "2026-03-27",
		"payment_method_code":   "bank_transfer",
		"clearing_account_code": "1010-BANK",
		"amount_received":       220.0,
		"unapplied_amount":      0.0,
		"allocations": []map[string]any{{
			"invoice_number": invoice.Header.Number,
			"invoice_id":     invoice.Header.ID,
			"amount":         220.0,
		}},
	})
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	payment.Header.Status = "received"
	payment.Header.Number = "PAY-RF-002"
	if err := docs.Save(payment); err != nil {
		t.Fatalf("save payment: %v", err)
	}

	if _, err := docs.AddLink(invoice.Header.ID, payment.Header.ID, "payment_for", map[string]any{"allocated_amount": 220.0}); err != nil {
		t.Fatalf("link invoice to payment: %v", err)
	}

	creditNote, err := service.CreateCreditNoteFromInvoice(invoice.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("create credit note: %v", err)
	}
	creditNote.Header.Status = "issued"
	if err := docs.Save(creditNote); err != nil {
		t.Fatalf("save credit note: %v", err)
	}
	if err := service.HandleApprovedDocument(creditNote, "user_admin"); err != nil {
		t.Fatalf("approve credit note: %v", err)
	}

	refund, err := service.CreateRefundFromCreditNote(creditNote.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("create refund: %v", err)
	}
	if got := textValue(refund.Body.Payload["payment_method_code"]); got != "bank_transfer" {
		t.Fatalf("expected seeded refund payment method, got %q", got)
	}
	if got := textValue(refund.Body.Payload["clearing_account_code"]); got != "1010-BANK" {
		t.Fatalf("expected seeded refund clearing account, got %q", got)
	}
	if got := textValue(refund.Body.Payload["source_payment_id"]); got != payment.Header.ID {
		t.Fatalf("expected seeded refund source payment id %s, got %q", payment.Header.ID, got)
	}
	refundAllocations := recordList(refund.Body.Payload["refund_allocations"])
	if len(refundAllocations) != 1 {
		t.Fatalf("expected 1 refund allocation row, got %d", len(refundAllocations))
	}
	refund.Body.Payload["amount_refunded"] = 100.0
	refundAllocations[0]["amount"] = 100.0
	refund.Body.Payload["refund_allocations"] = refundAllocations
	refund.Header.Status = "refunded"
	if err := docs.Save(refund); err != nil {
		t.Fatalf("save refund: %v", err)
	}
	if err := service.HandleApprovedDocument(refund, "user_admin"); err != nil {
		t.Fatalf("approve partial refund: %v", err)
	}

	updatedInvoice, err := docs.Get(invoice.Header.ID)
	if err != nil {
		t.Fatalf("reload invoice: %v", err)
	}
	if updatedInvoice.Header.Status != "paid" {
		t.Fatalf("expected invoice to remain paid after partial refund, got %s", updatedInvoice.Header.Status)
	}
	if got := numberValue(updatedInvoice.Body.Payload["refunded_amount"]); got != 100 {
		t.Fatalf("expected invoice refunded amount 100, got %v", got)
	}

	updatedCredit, err := docs.Get(creditNote.Header.ID)
	if err != nil {
		t.Fatalf("reload credit note: %v", err)
	}
	if got := numberValue(updatedCredit.Body.Payload["refunded_amount"]); got != 100 {
		t.Fatalf("expected credit note refunded amount 100, got %v", got)
	}
	updatedPayment, err := docs.Get(payment.Header.ID)
	if err != nil {
		t.Fatalf("reload payment: %v", err)
	}
	if got := numberValue(updatedPayment.Body.Payload["refunded_amount"]); got != 100 {
		t.Fatalf("expected source payment refunded amount 100, got %v", got)
	}

	secondRefund, err := service.CreateRefundFromCreditNote(creditNote.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("create second refund: %v", err)
	}
	if got := numberValue(secondRefund.Body.Payload["amount_refunded"]); got != 120 {
		t.Fatalf("expected remaining refundable amount 120, got %v", got)
	}
	if got := textValue(secondRefund.Body.Payload["source_payment_id"]); got != payment.Header.ID {
		t.Fatalf("expected second refund to target same payment %s, got %q", payment.Header.ID, got)
	}
}

func TestCreateRefundSplitsAcrossMultiplePayments(t *testing.T) {
	docs := document.NewService()
	mustRegisterCommercialDocumentTypes(t, docs)
	service := NewCommercialCoreService(docs, nil, nil, nil)

	invoice, err := docs.Create("invoice", "org_default", "loc_main", "user_admin", map[string]any{
		"party_id":                "party_1",
		"party_name":              "Walk In Customer",
		"currency_code":           "USD",
		"invoice_date":            "2026-03-27",
		"due_date":                "2026-04-26",
		"subtotal_amount":         200.0,
		"tax_amount":              20.0,
		"total_amount":            220.0,
		"paid_amount":             220.0,
		"refunded_amount":         0.0,
		"balance_due_amount":      0.0,
		"receivable_account_code": "1100-AR",
		"lines": []map[string]any{{
			"item_code":     "CONSULT",
			"description":   "Consultation",
			"quantity":      2.0,
			"unit_price":    100.0,
			"tax_rate":      10.0,
			"line_total":    220.0,
			"line_subtotal": 200.0,
			"tax_amount":    20.0,
		}},
	})
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	invoice.Header.Status = "paid"
	invoice.Header.Number = "INV-RF-MULTI"
	if err := docs.Save(invoice); err != nil {
		t.Fatalf("save invoice: %v", err)
	}

	paymentA, err := docs.Create("payment_receipt", "org_default", "loc_main", "user_admin", map[string]any{
		"party_id":              "party_1",
		"party_name":            "Walk In Customer",
		"currency_code":         "USD",
		"receipt_date":          "2026-03-27",
		"payment_method_code":   "bank_transfer",
		"clearing_account_code": "1010-BANK",
		"amount_received":       100.0,
		"refunded_amount":       0.0,
	})
	if err != nil {
		t.Fatalf("create payment A: %v", err)
	}
	paymentA.Header.Status = "received"
	paymentA.Header.Number = "PAY-RF-A"
	if err := docs.Save(paymentA); err != nil {
		t.Fatalf("save payment A: %v", err)
	}
	paymentB, err := docs.Create("payment_receipt", "org_default", "loc_main", "user_admin", map[string]any{
		"party_id":              "party_1",
		"party_name":            "Walk In Customer",
		"currency_code":         "USD",
		"receipt_date":          "2026-03-28",
		"payment_method_code":   "bank_transfer",
		"clearing_account_code": "1010-BANK",
		"amount_received":       120.0,
		"refunded_amount":       0.0,
	})
	if err != nil {
		t.Fatalf("create payment B: %v", err)
	}
	paymentB.Header.Status = "received"
	paymentB.Header.Number = "PAY-RF-B"
	if err := docs.Save(paymentB); err != nil {
		t.Fatalf("save payment B: %v", err)
	}
	for _, payment := range []document.Record{paymentA, paymentB} {
		if _, err := docs.AddLink(invoice.Header.ID, payment.Header.ID, "payment_for", map[string]any{"allocated_amount": payment.Body.Payload["amount_received"]}); err != nil {
			t.Fatalf("link invoice to payment: %v", err)
		}
	}

	creditNote, err := service.CreateCreditNoteFromInvoice(invoice.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("create credit note: %v", err)
	}
	creditNote.Header.Status = "issued"
	if err := docs.Save(creditNote); err != nil {
		t.Fatalf("save credit note: %v", err)
	}
	if err := service.HandleApprovedDocument(creditNote, "user_admin"); err != nil {
		t.Fatalf("approve credit note: %v", err)
	}

	refund, err := service.CreateRefundFromCreditNote(creditNote.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("create refund: %v", err)
	}
	if got := textValue(refund.Body.Payload["source_payment_id"]); got != "" {
		t.Fatalf("expected no singular source payment id for multi-payment refund, got %q", got)
	}
	refundAllocations := recordList(refund.Body.Payload["refund_allocations"])
	if len(refundAllocations) != 2 {
		t.Fatalf("expected 2 refund allocation rows, got %d", len(refundAllocations))
	}
	if got := numberValue(refundAllocations[0]["amount"]); got != 100 {
		t.Fatalf("expected first refund allocation 100, got %v", got)
	}
	if got := numberValue(refundAllocations[1]["amount"]); got != 120 {
		t.Fatalf("expected second refund allocation 120, got %v", got)
	}
	refund.Header.Status = "refunded"
	if err := docs.Save(refund); err != nil {
		t.Fatalf("save refund: %v", err)
	}
	if err := service.HandleApprovedDocument(refund, "user_admin"); err != nil {
		t.Fatalf("approve refund: %v", err)
	}

	updatedPaymentA, err := docs.Get(paymentA.Header.ID)
	if err != nil {
		t.Fatalf("reload payment A: %v", err)
	}
	if got := numberValue(updatedPaymentA.Body.Payload["refunded_amount"]); got != 100 {
		t.Fatalf("expected payment A refunded amount 100, got %v", got)
	}
	updatedPaymentB, err := docs.Get(paymentB.Header.ID)
	if err != nil {
		t.Fatalf("reload payment B: %v", err)
	}
	if got := numberValue(updatedPaymentB.Body.Payload["refunded_amount"]); got != 120 {
		t.Fatalf("expected payment B refunded amount 120, got %v", got)
	}
}

func TestHandleApprovedCommercialDocumentsCreatesPostingsAndAllocations(t *testing.T) {
	docs := document.NewService()
	mustRegisterCommercialDocumentTypes(t, docs)
	service := NewCommercialCoreService(docs, nil, nil, nil)

	invoice, err := docs.Create("invoice", "org_default", "loc_main", "user_admin", map[string]any{
		"party_id":                "party_1",
		"party_name":              "Walk In Customer",
		"currency_code":           "USD",
		"invoice_date":            "2026-03-27",
		"due_date":                "2026-04-26",
		"subtotal_amount":         200.0,
		"tax_amount":              20.0,
		"total_amount":            220.0,
		"paid_amount":             0.0,
		"balance_due_amount":      220.0,
		"receivable_account_code": "1100-AR",
		"lines": []map[string]any{{
			"item_code":     "CONSULT",
			"description":   "Consultation",
			"quantity":      2.0,
			"unit_price":    100.0,
			"tax_rate":      10.0,
			"line_total":    220.0,
			"line_subtotal": 200.0,
			"tax_amount":    20.0,
		}},
	})
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	invoice.Header.Status = "issued"
	invoice.Header.Number = "INV-001"
	if err := docs.Save(invoice); err != nil {
		t.Fatalf("save invoice: %v", err)
	}
	if err := service.HandleApprovedDocument(invoice, "user_admin"); err != nil {
		t.Fatalf("handle issued invoice: %v", err)
	}

	payment, err := docs.Create("payment_receipt", "org_default", "loc_main", "user_admin", map[string]any{
		"party_id":            "party_1",
		"party_name":          "Walk In Customer",
		"currency_code":       "USD",
		"receipt_date":        "2026-03-27",
		"payment_method_code": "cash",
		"amount_received":     220.0,
		"unapplied_amount":    0.0,
		"allocations": []map[string]any{{
			"invoice_number": "INV-001",
			"invoice_id":     invoice.Header.ID,
			"amount":         220.0,
		}},
	})
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	payment.Header.Status = "received"
	payment.Header.Number = "PAY-001"
	if err := docs.Save(payment); err != nil {
		t.Fatalf("save payment: %v", err)
	}
	if err := service.HandleApprovedDocument(payment, "user_admin"); err != nil {
		t.Fatalf("handle received payment: %v", err)
	}

	updatedInvoice, err := docs.Get(invoice.Header.ID)
	if err != nil {
		t.Fatalf("reload invoice: %v", err)
	}
	if updatedInvoice.Header.Status != "paid" {
		t.Fatalf("expected invoice status paid, got %s", updatedInvoice.Header.Status)
	}
	if got := updatedInvoice.Body.Payload["paid_amount"]; got != 220.0 {
		t.Fatalf("expected paid amount 220, got %v", got)
	}
	if got := updatedInvoice.Body.Payload["balance_due_amount"]; got != 0.0 {
		t.Fatalf("expected balance due 0, got %v", got)
	}

	postings := 0
	for _, item := range docs.List() {
		if item.Header.Type == "ledger_posting" {
			postings++
			if item.Header.Status != "posted" {
				t.Fatalf("expected posted ledger status, got %s", item.Header.Status)
			}
		}
	}
	if postings != 2 {
		t.Fatalf("expected 2 ledger postings, got %d", postings)
	}
}

func TestHandleCancelledPaymentReopensInvoiceAndCreatesReversalPosting(t *testing.T) {
	docs := document.NewService()
	mustRegisterCommercialDocumentTypes(t, docs)
	service := NewCommercialCoreService(docs, nil, nil, nil)

	invoice, err := docs.Create("invoice", "org_default", "loc_main", "user_admin", map[string]any{
		"party_id":                "party_1",
		"party_name":              "Walk In Customer",
		"currency_code":           "USD",
		"invoice_date":            "2026-03-27",
		"due_date":                "2026-04-26",
		"subtotal_amount":         200.0,
		"tax_amount":              20.0,
		"total_amount":            220.0,
		"paid_amount":             0.0,
		"balance_due_amount":      220.0,
		"receivable_account_code": "1100-AR",
	})
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	invoice.Header.Status = "issued"
	invoice.Header.Number = "INV-002"
	if err := docs.Save(invoice); err != nil {
		t.Fatalf("save invoice: %v", err)
	}
	if err := service.HandleApprovedDocument(invoice, "user_admin"); err != nil {
		t.Fatalf("handle issued invoice: %v", err)
	}

	payment, err := docs.Create("payment_receipt", "org_default", "loc_main", "user_admin", map[string]any{
		"party_id":            "party_1",
		"party_name":          "Walk In Customer",
		"currency_code":       "USD",
		"receipt_date":        "2026-03-27",
		"payment_method_code": "cash",
		"amount_received":     220.0,
		"unapplied_amount":    0.0,
		"allocations": []map[string]any{{
			"invoice_number": "INV-002",
			"invoice_id":     invoice.Header.ID,
			"amount":         220.0,
		}},
	})
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	payment.Header.Status = "received"
	payment.Header.Number = "PAY-002"
	if err := docs.Save(payment); err != nil {
		t.Fatalf("save payment: %v", err)
	}
	if err := service.HandleApprovedDocument(payment, "user_admin"); err != nil {
		t.Fatalf("handle received payment: %v", err)
	}

	payment.Header.Status = "cancelled"
	if err := docs.Save(payment); err != nil {
		t.Fatalf("save cancelled payment: %v", err)
	}
	if err := service.HandleCanceledDocument(payment, "user_admin"); err != nil {
		t.Fatalf("handle cancelled payment: %v", err)
	}

	updatedInvoice, err := docs.Get(invoice.Header.ID)
	if err != nil {
		t.Fatalf("reload invoice: %v", err)
	}
	if updatedInvoice.Header.Status != "issued" {
		t.Fatalf("expected invoice status issued after reversal, got %s", updatedInvoice.Header.Status)
	}
	if got := numberValue(updatedInvoice.Body.Payload["paid_amount"]); got != 0 {
		t.Fatalf("expected paid amount reset to 0, got %v", got)
	}
	if got := numberValue(updatedInvoice.Body.Payload["balance_due_amount"]); got != 220 {
		t.Fatalf("expected balance due reset to 220, got %v", got)
	}
	reloadedPayment, err := docs.Get(payment.Header.ID)
	if err != nil {
		t.Fatalf("reload payment: %v", err)
	}
	if got := numberValue(reloadedPayment.Body.Payload["unapplied_amount"]); got != 220 {
		t.Fatalf("expected unapplied amount 220, got %v", got)
	}

	postings := 0
	for _, item := range docs.List() {
		if item.Header.Type == "ledger_posting" {
			postings++
		}
	}
	if postings != 3 {
		t.Fatalf("expected 3 ledger postings after payment reversal, got %d", postings)
	}
}

func TestHandleCancelledInvoiceCreatesReversalPosting(t *testing.T) {
	docs := document.NewService()
	mustRegisterCommercialDocumentTypes(t, docs)
	service := NewCommercialCoreService(docs, nil, nil, nil)

	invoice, err := docs.Create("invoice", "org_default", "loc_main", "user_admin", map[string]any{
		"party_id":                "party_1",
		"party_name":              "Walk In Customer",
		"currency_code":           "USD",
		"invoice_date":            "2026-03-27",
		"due_date":                "2026-04-26",
		"subtotal_amount":         150.0,
		"tax_amount":              15.0,
		"total_amount":            165.0,
		"paid_amount":             0.0,
		"balance_due_amount":      165.0,
		"receivable_account_code": "1100-AR",
	})
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	invoice.Header.Status = "issued"
	invoice.Header.Number = "INV-003"
	if err := docs.Save(invoice); err != nil {
		t.Fatalf("save invoice: %v", err)
	}
	if err := service.HandleApprovedDocument(invoice, "user_admin"); err != nil {
		t.Fatalf("handle issued invoice: %v", err)
	}

	invoice.Header.Status = "cancelled"
	if err := docs.Save(invoice); err != nil {
		t.Fatalf("save cancelled invoice: %v", err)
	}
	if err := service.HandleCanceledDocument(invoice, "user_admin"); err != nil {
		t.Fatalf("handle cancelled invoice: %v", err)
	}

	postings := make([]document.Record, 0)
	for _, item := range docs.List() {
		if item.Header.Type == "ledger_posting" {
			postings = append(postings, item)
		}
	}
	if len(postings) != 2 {
		t.Fatalf("expected 2 ledger postings after invoice cancellation, got %d", len(postings))
	}
	last := postings[len(postings)-1]
	lines := recordList(last.Body.Payload["journal_lines"])
	if len(lines) < 2 {
		t.Fatalf("expected reversal posting journal lines, got %+v", lines)
	}
	if numberValue(lines[0]["credit"]) == 0 {
		t.Fatalf("expected reversal posting to swap debit into credit, got %+v", lines[0])
	}
}

func TestAllocatePaymentReceiptDoesNotMutatePaymentWhenTargetValidationFails(t *testing.T) {
	docs := document.NewService()
	mustRegisterCommercialDocumentTypes(t, docs)
	service := NewCommercialCoreService(docs, nil, nil, nil)

	payment, err := docs.Create("payment_receipt", "org_default", "loc_main", "user_admin", map[string]any{
		"party_id":                "party_1",
		"party_name":              "Walk In Customer",
		"receipt_date":            "2026-03-27",
		"amount_received":         220.0,
		"refunded_amount":         0.0,
		"unapplied_amount":        220.0,
		"receivable_account_code": "1100-AR",
		"allocations":             []map[string]any{},
	})
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	payment.Header.Status = "received"
	if err := docs.Save(payment); err != nil {
		t.Fatalf("save payment: %v", err)
	}

	err = service.AllocatePaymentReceipt(payment.Header.ID, "doc_missing_invoice", 100.0, "user_admin")
	if err == nil {
		t.Fatal("expected allocation validation error")
	}

	reloadedPayment, err := docs.Get(payment.Header.ID)
	if err != nil {
		t.Fatalf("reload payment: %v", err)
	}
	if got := numberValue(reloadedPayment.Body.Payload["unapplied_amount"]); got != 220.0 {
		t.Fatalf("expected unapplied amount 220, got %v", got)
	}
	if got := len(recordList(reloadedPayment.Body.Payload["allocations"])); got != 0 {
		t.Fatalf("expected no persisted allocations, got %d", got)
	}
}

func TestPostingLinesResolveCommercialAccountsFromCatalogs(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	configSvc := config.NewService()
	mustRegisterCommercialDocumentTypes(t, docs)
	mustRegisterCommercialModels(t, models)
	service := NewCommercialCoreService(docs, configSvc, models, nil)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                  "CONSULT",
		"name":                 "Consultation",
		"kind":                 "service",
		"tax_code":             "PPN",
		"revenue_account_code": "4100-SVC",
		"status":               "active",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	if _, err := models.Create("commercial_tax_code", "user_admin", map[string]any{
		"code":             "PPN",
		"name":             "VAT",
		"mode":             "exclusive",
		"rate_percent":     11.0,
		"tax_account_code": "2110-VAT",
		"status":           "active",
	}); err != nil {
		t.Fatalf("create tax code: %v", err)
	}
	if _, err := models.Create("payment_method", "user_admin", map[string]any{
		"code":                  "bank_transfer",
		"name":                  "Bank Transfer",
		"kind":                  "bank_transfer",
		"clearing_account_code": "1010-BANK",
		"status":                "active",
	}); err != nil {
		t.Fatalf("create payment method: %v", err)
	}

	invoiceLines := service.invoicePostingLines(map[string]any{
		"subtotal_amount":         200.0,
		"tax_amount":              20.0,
		"total_amount":            220.0,
		"receivable_account_code": "1105-AR-TRADE",
		"lines": []map[string]any{{
			"item_code": "CONSULT",
			"tax_code":  "PPN",
		}},
	})
	if got := textValue(invoiceLines[0]["account_code"]); got != "1105-AR-TRADE" {
		t.Fatalf("expected receivable account from invoice payload, got %s", got)
	}
	if got := textValue(invoiceLines[1]["account_code"]); got != "4100-SVC" {
		t.Fatalf("expected revenue account from item catalog, got %s", got)
	}
	if got := textValue(invoiceLines[2]["account_code"]); got != "2110-VAT" {
		t.Fatalf("expected tax account from tax code catalog, got %s", got)
	}

	paymentLines := service.paymentPostingLines(map[string]any{
		"amount_received":         220.0,
		"receivable_account_code": "1105-AR-TRADE",
		"payment_method_code":     "bank_transfer",
	})
	if got := textValue(paymentLines[0]["account_code"]); got != "1010-BANK" {
		t.Fatalf("expected clearing account from payment method catalog, got %s", got)
	}
	if got := textValue(paymentLines[1]["account_code"]); got != "1105-AR-TRADE" {
		t.Fatalf("expected receivable account from payment payload, got %s", got)
	}
}

func TestPostingLinesUseCommercialConfigDefaults(t *testing.T) {
	docs := document.NewService()
	configSvc := config.NewService()
	mustRegisterCommercialDocumentTypes(t, docs)
	service := NewCommercialCoreService(docs, configSvc, nil, nil)
	if err := configSvc.Save(config.Entry{
		Key:       "commercial.posting",
		ModuleKey: "commercial_core",
		Category:  "finance",
		Scope:     "deployment",
		Value: map[string]any{
			"invoice_issue_receivable_account_code":   "1199-AR-DEFAULT",
			"invoice_issue_revenue_account_code":      "4999-REV-DEFAULT",
			"invoice_issue_tax_account_code":          "2199-TAX-DEFAULT",
			"payment_receipt_clearing_account_code":   "1099-CLEARING-DEFAULT",
			"payment_receipt_receivable_account_code": "1199-AR-DEFAULT",
		},
	}); err != nil {
		t.Fatalf("save config: %v", err)
	}

	invoiceLines := service.invoicePostingLines(map[string]any{
		"subtotal_amount": 150.0,
		"tax_amount":      15.0,
		"total_amount":    165.0,
	})
	if got := textValue(invoiceLines[0]["account_code"]); got != "1199-AR-DEFAULT" {
		t.Fatalf("expected receivable account from config, got %s", got)
	}
	if got := textValue(invoiceLines[1]["account_code"]); got != "4999-REV-DEFAULT" {
		t.Fatalf("expected revenue account from config, got %s", got)
	}
	if got := textValue(invoiceLines[2]["account_code"]); got != "2199-TAX-DEFAULT" {
		t.Fatalf("expected tax account from config, got %s", got)
	}

	paymentLines := service.paymentPostingLines(map[string]any{
		"amount_received": 165.0,
	})
	if got := textValue(paymentLines[0]["account_code"]); got != "1099-CLEARING-DEFAULT" {
		t.Fatalf("expected clearing account from config, got %s", got)
	}
	if got := textValue(paymentLines[1]["account_code"]); got != "1199-AR-DEFAULT" {
		t.Fatalf("expected receivable account from config, got %s", got)
	}
}

func TestInvoicePostingLinesGroupRevenueByLineAccount(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	configSvc := config.NewService()
	mustRegisterCommercialDocumentTypes(t, docs)
	mustRegisterCommercialModels(t, models)
	service := NewCommercialCoreService(docs, configSvc, models, nil)

	lines := service.invoicePostingLines(map[string]any{
		"subtotal_amount":         300.0,
		"tax_amount":              33.0,
		"total_amount":            333.0,
		"receivable_account_code": "1105-AR-TRADE",
		"lines": []map[string]any{
			{
				"item_code":            "CONSULT",
				"line_subtotal":        100.0,
				"tax_amount":           11.0,
				"revenue_account_code": "4100-SVC",
				"tax_account_code":     "2110-VAT-OUT",
			},
			{
				"item_code":            "AMOX",
				"line_subtotal":        200.0,
				"tax_amount":           22.0,
				"revenue_account_code": "4200-DRUG",
				"tax_account_code":     "2110-VAT-OUT",
			},
		},
	})
	if len(lines) != 4 {
		t.Fatalf("expected 4 posting lines, got %+v", lines)
	}
	if got := textValue(lines[1]["account_code"]); got != "4100-SVC" || numberValue(lines[1]["credit"]) != 100.0 {
		t.Fatalf("expected service revenue line, got %+v", lines[1])
	}
	if got := textValue(lines[2]["account_code"]); got != "4200-DRUG" || numberValue(lines[2]["credit"]) != 200.0 {
		t.Fatalf("expected drug revenue line, got %+v", lines[2])
	}
	if got := textValue(lines[3]["account_code"]); got != "2110-VAT-OUT" || numberValue(lines[3]["credit"]) != 33.0 {
		t.Fatalf("expected grouped tax line, got %+v", lines[3])
	}
}

func TestNormalizeCommercialLinesFallsBackToProductDefaultsForVariant(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	configSvc := config.NewService()
	mustRegisterCommercialDocumentTypes(t, docs)
	mustRegisterCommercialModels(t, models)
	service := NewCommercialCoreService(docs, configSvc, models, nil)

	if _, err := models.Create("commercial_product", "user_admin", map[string]any{
		"code":                 "TSHIRT",
		"name":                 "T-Shirt",
		"uom_code":             "EA",
		"base_price":           150000.0,
		"tax_code":             "PPN",
		"revenue_account_code": "4100-APPAREL",
		"status":               "active",
	}); err != nil {
		t.Fatalf("create product: %v", err)
	}
	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":          "TSHIRT-BLACK-L",
		"name":         "T-Shirt Black L",
		"kind":         "product",
		"product_code": "TSHIRT",
		"is_variant":   true,
		"status":       "active",
	}); err != nil {
		t.Fatalf("create variant item: %v", err)
	}

	normalized := service.normalizeCommercialLines(map[string]any{
		"lines": []map[string]any{{
			"item_code":         "TSHIRT-BLACK-L",
			"product_code":      "TSHIRT",
			"quantity":          2.0,
			"variant_signature": "color=black|size=l",
		}},
	})
	lines := recordList(normalized["lines"])
	if len(lines) != 1 {
		t.Fatalf("expected 1 normalized line, got %d", len(lines))
	}
	line := lines[0]
	if got := numberValue(line["unit_price"]); got != 150000.0 {
		t.Fatalf("expected parent fallback unit price, got %v", got)
	}
	if got := textValue(line["uom_code"]); got != "EA" {
		t.Fatalf("expected parent fallback uom, got %s", got)
	}
	if got := textValue(line["tax_code"]); got != "PPN" {
		t.Fatalf("expected parent fallback tax code, got %s", got)
	}
	if got := textValue(line["revenue_account_code"]); got != "4100-APPAREL" {
		t.Fatalf("expected parent fallback revenue account, got %s", got)
	}
}

func TestCommercialItemValidationRequiresProductForSellableMerchandise(t *testing.T) {
	models := model.NewService()
	mustRegisterCommercialModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":         "SELLABLE-ITEM",
		"name":        "Sellable Item",
		"kind":        "product",
		"item_type":   "product",
		"is_sellable": true,
		"status":      "active",
	}); err == nil {
		t.Fatal("expected sellable product item without product_code to fail")
	}
}

func TestCommercialItemValidationAllowsStandaloneService(t *testing.T) {
	models := model.NewService()
	mustRegisterCommercialModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":         "SERVICE-ITEM",
		"name":        "Standalone Service",
		"kind":        "service",
		"item_type":   "service",
		"is_sellable": true,
		"status":      "active",
	}); err != nil {
		t.Fatalf("expected standalone service item to remain allowed: %v", err)
	}
}

func TestCommercialItemValidationRejectsUnknownProductCode(t *testing.T) {
	models := model.NewService()
	mustRegisterCommercialModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":          "UNKNOWN-PRODUCT-ITEM",
		"name":         "Unknown Product Item",
		"kind":         "product",
		"item_type":    "product",
		"is_sellable":  true,
		"product_code": "MISSING-PRODUCT",
		"status":       "active",
	}); err == nil {
		t.Fatal("expected unknown product_code to fail")
	}
}

func TestCommercialItemValidationAllowsVariantSKURename(t *testing.T) {
	models := model.NewService()
	mustRegisterCommercialModels(t, models)

	if _, err := models.Create("commercial_product", "user_admin", map[string]any{
		"code":      "VARIANT-PARENT",
		"name":      "Variant Parent",
		"item_type": "product",
		"status":    "active",
	}); err != nil {
		t.Fatalf("create product: %v", err)
	}
	item, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":               "VARIANT-OLD",
		"name":              "Variant Item",
		"kind":              "variant",
		"item_type":         "product",
		"is_sellable":       true,
		"product_code":      "VARIANT-PARENT",
		"variant_signature": "color=black|size=m",
		"status":            "active",
	})
	if err != nil {
		t.Fatalf("create variant item: %v", err)
	}

	updatedValues := map[string]any{}
	for key, value := range item.Values {
		updatedValues[key] = value
	}
	updatedValues["sku"] = "VARIANT-NEW"
	if _, err := models.Update("commercial_item", item.ID, "user_admin", updatedValues, item.Version); err != nil {
		t.Fatalf("expected SKU-only variant rename to pass: %v", err)
	}
}

func TestNormalizeCommercialLinesSupportsHeaderDefaultAndInclusiveTax(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	configSvc := config.NewService()
	mustRegisterCommercialDocumentTypes(t, docs)
	mustRegisterCommercialModels(t, models)
	service := NewCommercialCoreService(docs, configSvc, models, nil)

	if _, err := models.Create("commercial_tax_code", "user_admin", map[string]any{
		"code":             "VAT11INC",
		"name":             "VAT 11 Inclusive",
		"mode":             "inclusive",
		"rate_percent":     11.0,
		"tax_account_code": "2110-VAT",
		"status":           "active",
	}); err != nil {
		t.Fatalf("create tax code: %v", err)
	}
	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":        "CONSULT",
		"name":       "Consultation",
		"kind":       "service",
		"unit_price": 111.0,
		"status":     "active",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}

	normalized := service.NormalizePayload("invoice", map[string]any{
		"default_tax_code": "VAT11INC",
		"lines": []map[string]any{{
			"item_code": "CONSULT",
			"quantity":  1.0,
		}},
	})
	lines := recordList(normalized["lines"])
	if len(lines) != 1 {
		t.Fatalf("expected 1 normalized line, got %d", len(lines))
	}
	if got := textValue(lines[0]["tax_code"]); got != "VAT11INC" {
		t.Fatalf("expected default tax code applied, got %s", got)
	}
	if got := textValue(lines[0]["tax_mode"]); got != "inclusive" {
		t.Fatalf("expected inclusive tax mode, got %s", got)
	}
	if got := numberValue(lines[0]["line_subtotal"]); got != 100.0 {
		t.Fatalf("expected inclusive subtotal 100, got %v", got)
	}
	if got := numberValue(lines[0]["tax_amount"]); got != 11.0 {
		t.Fatalf("expected inclusive tax 11, got %v", got)
	}
	if got := numberValue(lines[0]["line_total"]); got != 111.0 {
		t.Fatalf("expected inclusive total 111, got %v", got)
	}
}

func TestNormalizeCommercialLinesUsesTaxProfileDefaults(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	configSvc := config.NewService()
	mustRegisterCommercialDocumentTypes(t, docs)
	mustRegisterCommercialModels(t, models)
	service := NewCommercialCoreService(docs, configSvc, models, nil)

	if _, err := models.Create("commercial_tax_code", "user_admin", map[string]any{
		"code":             "VAT11",
		"name":             "VAT 11",
		"mode":             "exclusive",
		"rate_percent":     11.0,
		"tax_account_code": "2110-VAT",
		"status":           "active",
	}); err != nil {
		t.Fatalf("create tax code: %v", err)
	}
	if _, err := models.Create("commercial_tax_profile", "user_admin", map[string]any{
		"code":              "STANDARD_ID",
		"name":              "Standard Indonesia",
		"default_tax_code":  "VAT11",
		"payment_term_days": 21.0,
		"price_tax_mode":    "exclusive",
		"status":            "active",
	}); err != nil {
		t.Fatalf("create tax profile: %v", err)
	}
	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":        "CONSULT",
		"name":       "Consultation",
		"kind":       "service",
		"unit_price": 100.0,
		"status":     "active",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}

	normalized := service.NormalizePayload("invoice", map[string]any{
		"tax_profile_code": "STANDARD_ID",
		"invoice_date":     "2026-03-27",
		"lines": []map[string]any{{
			"item_code": "CONSULT",
			"quantity":  1.0,
		}},
	})
	lines := recordList(normalized["lines"])
	if len(lines) != 1 {
		t.Fatalf("expected 1 normalized line, got %d", len(lines))
	}
	if got := textValue(normalized["default_tax_code"]); got != "VAT11" {
		t.Fatalf("expected profile default tax code VAT11, got %s", got)
	}
	if got := numberValue(normalized["payment_term_days"]); got != 21.0 {
		t.Fatalf("expected payment term days 21, got %v", got)
	}
	if got := textValue(normalized["due_date"]); got != "2026-04-17" {
		t.Fatalf("expected due date 2026-04-17, got %s", got)
	}
	if got := textValue(lines[0]["tax_code"]); got != "VAT11" {
		t.Fatalf("expected line tax code VAT11, got %s", got)
	}
	if got := numberValue(lines[0]["tax_amount"]); got != 11.0 {
		t.Fatalf("expected line tax 11, got %v", got)
	}
}

func TestNormalizeCommercialLinesUsesPartyCommercialDefaults(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	configSvc := config.NewService()
	mustRegisterCommercialDocumentTypes(t, docs)
	mustRegisterCommercialModels(t, models)
	service := NewCommercialCoreService(docs, configSvc, models, nil)

	partyRecord, err := models.Create("party", "user_admin", map[string]any{
		"name":                    "Acme Corp",
		"email":                   "billing@acme.test",
		"currency_code":           "USD",
		"tax_profile_code":        "STANDARD_21",
		"default_price_list_code": "B2B_STD",
		"payment_term_days":       21.0,
		"status":                  "active",
	})
	if err != nil {
		t.Fatalf("create party: %v", err)
	}
	if _, err := models.Create("commercial_tax_code", "user_admin", map[string]any{
		"code":             "VAT11",
		"name":             "VAT 11",
		"mode":             "exclusive",
		"rate_percent":     11.0,
		"tax_account_code": "2110-VAT",
		"status":           "active",
	}); err != nil {
		t.Fatalf("create tax code: %v", err)
	}
	if _, err := models.Create("commercial_tax_profile", "user_admin", map[string]any{
		"code":              "STANDARD_21",
		"name":              "Standard 21 Days",
		"default_tax_code":  "VAT11",
		"payment_term_days": 21.0,
		"price_tax_mode":    "exclusive",
		"status":            "active",
	}); err != nil {
		t.Fatalf("create tax profile: %v", err)
	}
	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":         "CONSULT",
		"name":        "Consultation",
		"kind":        "service",
		"item_type":   "service",
		"base_price":  100.0,
		"unit_price":  100.0,
		"status":      "active",
		"is_sellable": true,
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	if _, err := models.Create("commercial_price_list", "user_admin", map[string]any{
		"code":          "B2B_STD",
		"name":          "B2B Standard",
		"currency_code": "USD",
		"status":        "active",
	}); err != nil {
		t.Fatalf("create price list: %v", err)
	}
	if _, err := models.Create("commercial_price_list_item", "user_admin", map[string]any{
		"price_list_code": "B2B_STD",
		"item_code":       "CONSULT",
		"unit_price":      95.0,
		"status":          "active",
	}); err != nil {
		t.Fatalf("create price list item: %v", err)
	}

	normalized := service.NormalizePayload("invoice", map[string]any{
		"party_id":     partyRecord.ID,
		"invoice_date": "2026-03-27",
		"lines":        []map[string]any{{"item_code": "CONSULT", "quantity": 1.0}},
	})
	lines := recordList(normalized["lines"])
	if got := textValue(normalized["party_name"]); got != "Acme Corp" {
		t.Fatalf("expected party name Acme Corp, got %s", got)
	}
	if got := textValue(normalized["currency_code"]); got != "USD" {
		t.Fatalf("expected currency USD, got %s", got)
	}
	if got := textValue(normalized["tax_profile_code"]); got != "STANDARD_21" {
		t.Fatalf("expected tax profile STANDARD_21, got %s", got)
	}
	if got := textValue(normalized["price_list_code"]); got != "B2B_STD" {
		t.Fatalf("expected price list B2B_STD, got %s", got)
	}
	if got := numberValue(normalized["payment_term_days"]); got != 21.0 {
		t.Fatalf("expected payment term days 21, got %v", got)
	}
	if got := textValue(normalized["due_date"]); got != "2026-04-17" {
		t.Fatalf("expected due date 2026-04-17, got %s", got)
	}
	if len(lines) != 1 || textValue(lines[0]["tax_code"]) != "VAT11" {
		t.Fatalf("expected line tax code VAT11, got %+v", lines)
	}
	if got := numberValue(lines[0]["unit_price"]); got != 95.0 {
		t.Fatalf("expected price list unit price 95, got %v", got)
	}
}

func TestNormalizeCommercialLinesUsesPriceListItemPricing(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	configSvc := config.NewService()
	mustRegisterCommercialDocumentTypes(t, docs)
	mustRegisterCommercialModels(t, models)
	service := NewCommercialCoreService(docs, configSvc, models, nil)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                  "SUBSCRIPTION",
		"name":                 "Subscription",
		"description":          "Annual subscription",
		"kind":                 "service",
		"item_type":            "service",
		"uom_code":             "YEAR",
		"base_price":           1200.0,
		"unit_price":           1200.0,
		"tax_code":             "VAT11",
		"revenue_account_code": "4100-SVC",
		"status":               "active",
		"is_sellable":          true,
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	if _, err := models.Create("commercial_tax_code", "user_admin", map[string]any{
		"code":             "VAT11",
		"name":             "VAT 11",
		"mode":             "exclusive",
		"rate_percent":     11.0,
		"tax_account_code": "2110-VAT",
		"status":           "active",
	}); err != nil {
		t.Fatalf("create tax code: %v", err)
	}
	if _, err := models.Create("commercial_price_list", "user_admin", map[string]any{
		"code":          "CORP2026",
		"name":          "Corporate 2026",
		"currency_code": "USD",
		"status":        "active",
	}); err != nil {
		t.Fatalf("create price list: %v", err)
	}
	if _, err := models.Create("commercial_price_list_item", "user_admin", map[string]any{
		"price_list_code":      "CORP2026",
		"item_code":            "SUBSCRIPTION",
		"unit_price":           999.0,
		"tax_code":             "VAT11",
		"revenue_account_code": "4190-SUB",
		"status":               "active",
	}); err != nil {
		t.Fatalf("create price list item: %v", err)
	}

	normalized := service.NormalizePayload("sales_order", map[string]any{
		"price_list_code": "CORP2026",
		"lines": []map[string]any{{
			"item_code": "SUBSCRIPTION",
			"quantity":  1.0,
		}},
	})
	lines := recordList(normalized["lines"])
	if len(lines) != 1 {
		t.Fatalf("expected 1 normalized line, got %d", len(lines))
	}
	if got := textValue(lines[0]["description"]); got != "Annual subscription" {
		t.Fatalf("expected item description, got %s", got)
	}
	if got := textValue(lines[0]["uom_code"]); got != "YEAR" {
		t.Fatalf("expected uom YEAR, got %s", got)
	}
	if got := numberValue(lines[0]["unit_price"]); got != 999.0 {
		t.Fatalf("expected price list unit price 999, got %v", got)
	}
	if got := textValue(lines[0]["revenue_account_code"]); got != "4190-SUB" {
		t.Fatalf("expected price list revenue account 4190-SUB, got %s", got)
	}
}

func TestNormalizeCommercialAllocationsUsesInvoiceDefaults(t *testing.T) {
	docs := document.NewService()
	mustRegisterCommercialDocumentTypes(t, docs)
	service := NewCommercialCoreService(docs, nil, nil, nil)

	invoice, err := docs.Create("invoice", "org_default", "loc_main", "user_admin", map[string]any{
		"party_id":           "party_1",
		"party_name":         "Walk In Customer",
		"currency_code":      "USD",
		"invoice_date":       "2026-03-27",
		"subtotal_amount":    200.0,
		"tax_amount":         20.0,
		"total_amount":       220.0,
		"paid_amount":        50.0,
		"balance_due_amount": 170.0,
	})
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	invoice.Header.Status = "partially_paid"
	invoice.Header.Number = "INV-OPEN-1"
	if err := docs.Save(invoice); err != nil {
		t.Fatalf("save invoice: %v", err)
	}

	normalized := service.NormalizePayload("payment_receipt", map[string]any{
		"currency_code":   "USD",
		"amount_received": 170.0,
		"allocations": []map[string]any{{
			"invoice_id": invoice.Header.ID,
		}},
	})
	allocations := recordList(normalized["allocations"])
	if len(allocations) != 1 {
		t.Fatalf("expected 1 allocation, got %d", len(allocations))
	}
	if got := textValue(allocations[0]["invoice_number"]); got != "INV-OPEN-1" {
		t.Fatalf("expected invoice number INV-OPEN-1, got %s", got)
	}
	if got := numberValue(allocations[0]["amount"]); got != 170.0 {
		t.Fatalf("expected default allocation amount 170, got %v", got)
	}
	if got := numberValue(normalized["unapplied_amount"]); got != 0.0 {
		t.Fatalf("expected unapplied amount 0, got %v", got)
	}
}

func TestReceivablesSummaryBucketsOpenInvoices(t *testing.T) {
	docs := document.NewService()
	mustRegisterCommercialDocumentTypes(t, docs)
	svc := NewCommercialCoreService(docs, config.NewService(), model.NewService(), search.NewService())

	createInvoice := func(invoiceDate, dueDate string, total, paid float64) {
		balance := total - paid
		record, err := docs.Create("invoice", "org_default", "loc_hq", "user_admin", map[string]any{
			"party_id":           "party_walk_in",
			"party_name":         "Walk In Customer",
			"invoice_date":       invoiceDate,
			"due_date":           dueDate,
			"currency_code":      "USD",
			"total_amount":       total,
			"paid_amount":        paid,
			"balance_due_amount": balance,
			"lines": []map[string]any{{
				"description": "Service",
				"quantity":    1,
				"unit_price":  total,
				"subtotal":    total,
				"tax_amount":  0,
				"total":       total,
			}},
		})
		if err != nil {
			t.Fatalf("create invoice failed: %v", err)
		}
		record.Header.Status = "issued"
		if err := docs.Save(record); err != nil {
			t.Fatalf("save invoice failed: %v", err)
		}
	}

	createInvoice("2026-03-01", "2026-03-20", 100, 0)
	createInvoice("2026-03-10", "2026-03-27", 200, 50)
	createInvoice("2026-03-25", "2026-04-10", 300, 0)

	summary := svc.ReceivablesSummary(time.Date(2026, 3, 27, 12, 0, 0, 0, time.UTC))

	if summary.OpenInvoiceCount != 3 {
		t.Fatalf("expected 3 open invoices, got %d", summary.OpenInvoiceCount)
	}
	if summary.OpenBalanceTotal != 550 {
		t.Fatalf("expected open balance 550, got %v", summary.OpenBalanceTotal)
	}
	if summary.OverdueInvoiceCount != 1 || summary.OverdueBalanceTotal != 100 {
		t.Fatalf("expected one overdue invoice totaling 100, got count=%d total=%v", summary.OverdueInvoiceCount, summary.OverdueBalanceTotal)
	}
	if summary.DueTodayCount != 1 || summary.DueTodayTotal != 150 {
		t.Fatalf("expected one due-today invoice totaling 150, got count=%d total=%v", summary.DueTodayCount, summary.DueTodayTotal)
	}
	if summary.CurrentBalanceTotal != 300 {
		t.Fatalf("expected current balance 300, got %v", summary.CurrentBalanceTotal)
	}
	if summary.Aging["overdue_1_30"] != 100 || summary.Aging["due_today"] != 150 || summary.Aging["current"] != 300 {
		t.Fatalf("unexpected aging buckets: %#v", summary.Aging)
	}
}

func TestReceivablesSummaryScopedByLocation(t *testing.T) {
	docs := document.NewService()
	mustRegisterCommercialDocumentTypes(t, docs)
	svc := NewCommercialCoreService(docs, nil, nil, nil)

	for _, tc := range []struct {
		locationID string
		partyName  string
		total      float64
	}{
		{locationID: "loc_hq", partyName: "HQ Party", total: 100},
		{locationID: "loc_other", partyName: "Other Party", total: 200},
	} {
		record, err := docs.Create("invoice", "org_default", tc.locationID, "user_admin", map[string]any{
			"party_id":           tc.partyName,
			"party_name":         tc.partyName,
			"invoice_date":       "2026-03-28",
			"due_date":           "2026-03-28",
			"total_amount":       tc.total,
			"paid_amount":        0.0,
			"credited_amount":    0.0,
			"refunded_amount":    0.0,
			"balance_due_amount": tc.total,
		})
		if err != nil {
			t.Fatalf("create invoice: %v", err)
		}
		record.Header.Status = "issued"
		if err := docs.Save(record); err != nil {
			t.Fatalf("save invoice: %v", err)
		}
	}

	summary := svc.ReceivablesSummaryScoped("org_default", "loc_hq", time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC))
	if summary.OpenInvoiceCount != 1 {
		t.Fatalf("expected one visible invoice, got %+v", summary)
	}
	if summary.OpenBalanceTotal != 100 {
		t.Fatalf("expected open balance 100, got %+v", summary)
	}
	if len(summary.Items) != 1 || summary.Items[0]["party_name"] != "HQ Party" {
		t.Fatalf("expected only HQ invoice item, got %+v", summary.Items)
	}
}

func TestNormalizeCommercialLinesAppliesVariantAndBulkDiscountRules(t *testing.T) {
	models := model.NewService()
	mustRegisterCommercialModels(t, models)
	configSvc := config.NewService()
	service := NewCommercialCoreService(document.NewService(), configSvc, models, nil)

	if _, err := models.Create("commercial_product", "user_admin", map[string]any{
		"code": "TEE",
		"name": "Tee",
	}); err != nil {
		t.Fatalf("create product: %v", err)
	}
	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":               "TEE-BLACK-M",
		"name":              "Tee Black M",
		"product_code":      "TEE",
		"variant_signature": "color=black|size=m",
		"category_code":     "APPAREL",
		"kind":              "variant",
		"item_type":         "product",
		"unit_price":        100,
		"status":            "active",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	if _, err := models.Create("discount_rule", "user_admin", map[string]any{
		"code":               "VAR10",
		"name":               "Variant 10",
		"scope":              "line",
		"rule_kind":          "line_percent",
		"variant_signatures": "color=black|size=m",
		"discount_percent":   10,
		"priority":           10,
		"status":             "active",
	}); err != nil {
		t.Fatalf("create rule: %v", err)
	}
	if _, err := models.Create("discount_rule", "user_admin", map[string]any{
		"code":                  "BULK5",
		"name":                  "Bulk 5",
		"scope":                 "line",
		"rule_kind":             "bulk_percent",
		"item_codes":            "TEE-BLACK-M",
		"minimum_line_quantity": 5,
		"discount_percent":      5,
		"priority":              20,
		"status":                "active",
	}); err != nil {
		t.Fatalf("create rule: %v", err)
	}
	if err := configSvc.Save(config.Entry{
		Key:       "discount.policy",
		Scope:     "deployment",
		Value:     map[string]any{"stacking_mode": "fully_stackable", "time_zone": "Asia/Jakarta"},
		UpdatedAt: time.Now().UTC(),
		UpdatedBy: "user_admin",
	}); err != nil {
		t.Fatalf("save config: %v", err)
	}

	normalized := service.NormalizePayload("sales_order", map[string]any{
		"order_date": "2026-03-29",
		"lines": []map[string]any{{
			"item_code":         "TEE-BLACK-M",
			"product_code":      "TEE",
			"variant_signature": "color=black|size=m",
			"quantity":          5.0,
		}},
	})

	lines := recordList(normalized["lines"])
	if got := numberValue(lines[0]["discount_amount"]); got != 75 {
		t.Fatalf("expected stacked discount 75, got %v", got)
	}
	if got := numberValue(normalized["discount_amount_total"]); got != 75 {
		t.Fatalf("expected discount total 75, got %v", got)
	}
}

func TestNormalizeCommercialLinesAppliesBuyXGetYRule(t *testing.T) {
	models := model.NewService()
	mustRegisterCommercialModels(t, models)
	service := NewCommercialCoreService(document.NewService(), config.NewService(), models, nil)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":        "SNACK",
		"name":       "Snack",
		"kind":       "simple",
		"item_type":  "product",
		"unit_price": 10,
		"status":     "active",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	if _, err := models.Create("discount_rule", "user_admin", map[string]any{
		"code":            "B2G1",
		"name":            "Buy 2 Get 1",
		"scope":           "line",
		"rule_kind":       "bxgy",
		"item_codes":      "SNACK",
		"buy_quantity":    2,
		"reward_quantity": 1,
		"reward_percent":  100,
		"status":          "active",
	}); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	normalized := service.NormalizePayload("sales_order", map[string]any{
		"order_date": "2026-03-29",
		"lines": []map[string]any{{
			"item_code": "SNACK",
			"quantity":  3.0,
		}},
	})
	lines := recordList(normalized["lines"])
	if got := numberValue(lines[0]["discount_amount"]); got != 10 {
		t.Fatalf("expected bxgy discount 10, got %v", got)
	}
}

func TestNormalizeCommercialLinesDoesNotCompoundAutoDiscountOnRenormalize(t *testing.T) {
	models := model.NewService()
	mustRegisterCommercialModels(t, models)
	service := NewCommercialCoreService(document.NewService(), config.NewService(), models, nil)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":        "AUTO-DISC",
		"name":       "Auto Disc",
		"kind":       "simple",
		"item_type":  "product",
		"unit_price": 100,
		"status":     "active",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	if _, err := models.Create("discount_rule", "user_admin", map[string]any{
		"code":             "AUTO10",
		"name":             "Auto 10",
		"scope":            "line",
		"rule_kind":        "line_percent",
		"item_codes":       "AUTO-DISC",
		"discount_percent": 10,
		"status":           "active",
	}); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	first := service.NormalizePayload("sales_order", map[string]any{
		"order_date": "2026-03-29",
		"lines": []map[string]any{{
			"item_code": "AUTO-DISC",
			"quantity":  1.0,
		}},
	})
	second := service.NormalizePayload("sales_order", first)
	firstLine := recordList(first["lines"])[0]
	secondLine := recordList(second["lines"])[0]
	if got := numberValue(firstLine["discount_amount"]); got != 10 {
		t.Fatalf("expected first discount 10, got %v", got)
	}
	if got := numberValue(secondLine["discount_amount"]); got != 10 {
		t.Fatalf("expected re-normalized discount to stay 10, got %v", got)
	}
	if got := numberValue(secondLine["manual_discount_amount"]); got != 0 {
		t.Fatalf("expected manual discount seed 0, got %v", got)
	}
}

func TestDiscountEvaluationTimePrefersCurrentOrderTimestamp(t *testing.T) {
	models := model.NewService()
	mustRegisterCommercialModels(t, models)
	service := NewCommercialCoreService(document.NewService(), config.NewService(), models, nil)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":        "TIME-DISC",
		"name":       "Time Disc",
		"kind":       "simple",
		"item_type":  "product",
		"unit_price": 100,
		"status":     "active",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	if _, err := models.Create("discount_rule", "user_admin", map[string]any{
		"code":             "LUNCH10",
		"name":             "Lunch 10",
		"scope":            "line",
		"rule_kind":        "line_percent",
		"item_codes":       "TIME-DISC",
		"discount_percent": 10,
		"weekdays":         "sunday",
		"start_time":       "12:00",
		"end_time":         "15:00",
		"status":           "active",
	}); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	normalized := service.NormalizePayload("sales_order", map[string]any{
		"order_date":            "2026-03-29",
		"order_datetime":        "2026-03-29T13:00:00+07:00",
		"discount_evaluated_at": "2026-03-29T09:00:00+07:00",
		"lines": []map[string]any{{
			"item_code": "TIME-DISC",
			"quantity":  1.0,
		}},
	})
	line := recordList(normalized["lines"])[0]
	if got := numberValue(line["discount_amount"]); got != 10 {
		t.Fatalf("expected rule to use current order timestamp and discount 10, got %v", got)
	}
}

func TestWithinClockWindowSupportsOvernightWindow(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	atLate := time.Date(2026, 3, 29, 23, 30, 0, 0, loc)
	atEarly := time.Date(2026, 3, 30, 1, 30, 0, 0, loc)
	atOutside := time.Date(2026, 3, 30, 3, 0, 0, 0, loc)

	if !withinClockWindow(atLate, "22:00", "02:00") {
		t.Fatalf("expected 23:30 to match overnight window")
	}
	if !withinClockWindow(atEarly, "22:00", "02:00") {
		t.Fatalf("expected 01:30 to match overnight window")
	}
	if withinClockWindow(atOutside, "22:00", "02:00") {
		t.Fatalf("expected 03:00 to fall outside overnight window")
	}
}

func TestNormalizeCommercialLinesAppliesMemberAndOrderExclusionDiscounts(t *testing.T) {
	models := model.NewService()
	mustRegisterCommercialModels(t, models)
	configSvc := config.NewService()
	service := NewCommercialCoreService(document.NewService(), configSvc, models, nil)

	if err := configSvc.Save(config.Entry{
		Key:       "discount.policy",
		Scope:     "deployment",
		Value:     map[string]any{"stacking_mode": "stack_by_scope", "time_zone": "Asia/Jakarta"},
		UpdatedAt: time.Now().UTC(),
		UpdatedBy: "user_admin",
	}); err != nil {
		t.Fatalf("save config: %v", err)
	}
	party, err := models.Create("party", "user_admin", map[string]any{
		"name":          "Alice",
		"member_status": "active",
		"member_tier":   "gold",
		"status":        "active",
	})
	if err != nil {
		t.Fatalf("create party: %v", err)
	}
	for _, item := range []map[string]any{
		{"sku": "MEMBER-ITEM", "name": "Member Item", "kind": "simple", "item_type": "product", "unit_price": 100, "category_code": "APPAREL", "status": "active"},
		{"sku": "EXCLUDED", "name": "Excluded Item", "kind": "simple", "item_type": "product", "unit_price": 50, "category_code": "APPAREL", "status": "active"},
	} {
		if _, err := models.Create("commercial_item", "user_admin", item); err != nil {
			t.Fatalf("create item: %v", err)
		}
	}
	if _, err := models.Create("discount_rule", "user_admin", map[string]any{
		"code":             "MEMBER5",
		"name":             "Member 5",
		"scope":            "line",
		"rule_kind":        "line_percent",
		"member_statuses":  "active",
		"item_codes":       "MEMBER-ITEM",
		"discount_percent": 5,
		"status":           "active",
	}); err != nil {
		t.Fatalf("create rule: %v", err)
	}
	if _, err := models.Create("discount_rule", "user_admin", map[string]any{
		"code":                "ORDER10",
		"name":                "Order 10",
		"scope":               "order",
		"rule_kind":           "order_percent",
		"discount_percent":    10,
		"excluded_item_codes": "EXCLUDED",
		"status":              "active",
	}); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	normalized := service.NormalizePayload("sales_order", map[string]any{
		"party_id":   party.ID,
		"order_date": "2026-03-29",
		"lines": []map[string]any{
			{"item_code": "MEMBER-ITEM", "quantity": 1.0},
			{"item_code": "EXCLUDED", "quantity": 1.0},
		},
	})
	lines := recordList(normalized["lines"])
	if got := numberValue(lines[0]["discount_amount"]); got != 15 {
		t.Fatalf("expected member item discount 15, got %v", got)
	}
	if got := numberValue(lines[1]["discount_amount"]); got != 0 {
		t.Fatalf("expected excluded line discount 0, got %v", got)
	}
}

func TestNormalizeCommercialLinesAppliesTimeAndCategoryDiscount(t *testing.T) {
	models := model.NewService()
	mustRegisterCommercialModels(t, models)
	configSvc := config.NewService()
	service := NewCommercialCoreService(document.NewService(), configSvc, models, nil)

	if err := configSvc.Save(config.Entry{
		Key:       "discount.policy",
		Scope:     "deployment",
		Value:     map[string]any{"stacking_mode": "best_one_only", "time_zone": "Asia/Jakarta"},
		UpdatedAt: time.Now().UTC(),
		UpdatedBy: "user_admin",
	}); err != nil {
		t.Fatalf("save config: %v", err)
	}
	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":           "LUNCH-ITEM",
		"name":          "Lunch Item",
		"kind":          "simple",
		"item_type":     "product",
		"unit_price":    100,
		"category_code": "FOOD",
		"status":        "active",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	if _, err := models.Create("discount_rule", "user_admin", map[string]any{
		"code":             "LUNCH20",
		"name":             "Lunch 20",
		"scope":            "line",
		"rule_kind":        "line_percent",
		"item_codes":       "LUNCH-ITEM",
		"weekdays":         "monday,tuesday,wednesday,thursday,friday",
		"start_time":       "12:00",
		"end_time":         "15:00",
		"discount_percent": 20,
		"status":           "active",
	}); err != nil {
		t.Fatalf("create rule: %v", err)
	}
	if _, err := models.Create("discount_rule", "user_admin", map[string]any{
		"code":             "FOOD10",
		"name":             "Food 10",
		"scope":            "line",
		"rule_kind":        "category_percent",
		"category_codes":   "FOOD",
		"discount_percent": 10,
		"status":           "active",
	}); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	normalized := service.NormalizePayload("sales_order", map[string]any{
		"order_datetime": "2026-03-27T13:00:00+07:00",
		"lines": []map[string]any{{
			"item_code": "LUNCH-ITEM",
			"quantity":  1.0,
		}},
	})
	lines := recordList(normalized["lines"])
	if got := numberValue(lines[0]["discount_amount"]); got != 20 {
		t.Fatalf("expected best-one time discount 20, got %v", got)
	}
}

func mustRegisterCommercialDocumentTypes(t *testing.T, docs *document.Service) {
	t.Helper()
	for _, def := range []document.Definition{
		{Type: "sales_order", DisplayName: "Sales Order", SchemaVersion: "v1", AllowedLinkTypes: []string{"source_order", "invoice_for", "payment_for", "refund_for", "posting_for", "fulfillment_for"}},
		{Type: "invoice", DisplayName: "Invoice", SchemaVersion: "v1", AllowedLinkTypes: []string{"source_order", "invoice_for", "payment_for", "refund_for", "posting_for", "fulfillment_for"}},
		{Type: "credit_note", DisplayName: "Credit Note", SchemaVersion: "v1", AllowedLinkTypes: []string{"source_order", "invoice_for", "payment_for", "refund_for", "posting_for", "fulfillment_for"}},
		{Type: "payment_receipt", DisplayName: "Payment Receipt", SchemaVersion: "v1", AllowedLinkTypes: []string{"source_order", "invoice_for", "payment_for", "refund_for", "posting_for", "fulfillment_for"}},
		{Type: "payment_refund", DisplayName: "Payment Refund", SchemaVersion: "v1", AllowedLinkTypes: []string{"source_order", "invoice_for", "payment_for", "refund_for", "posting_for", "fulfillment_for"}},
		{Type: "ledger_posting", DisplayName: "Ledger Posting", SchemaVersion: "v1", AllowedLinkTypes: []string{"source_order", "invoice_for", "payment_for", "refund_for", "posting_for", "fulfillment_for"}},
	} {
		if err := docs.Register(def); err != nil {
			t.Fatalf("register document definition %s: %v", def.Type, err)
		}
	}
}

func TestApplyPartyCommercialDefaultsPrefersCustomerProfile(t *testing.T) {
	models := model.NewService()
	mustRegisterCommercialModels(t, models)
	service := NewCommercialCoreService(document.NewService(), config.NewService(), models, nil)

	party, err := models.Create("party", "user_admin", map[string]any{
		"name":                    "PT Example",
		"display_name":            "Example Root",
		"tax_profile_code":        "PARTY-TAX",
		"default_price_list_code": "PARTY-PL",
		"payment_term_days":       14,
		"status":                  "active",
	})
	if err != nil {
		t.Fatalf("create party: %v", err)
	}
	if _, err := models.Create("customer_profile", "user_admin", map[string]any{
		"party_id":                party.ID,
		"customer_name":           "Example Customer",
		"tax_profile_code":        "CUST-TAX",
		"default_price_list_code": "CUST-PL",
		"payment_term_days":       30,
		"status":                  "active",
	}); err != nil {
		t.Fatalf("create customer profile: %v", err)
	}

	normalized := service.NormalizePayload("sales_order", map[string]any{
		"party_id": party.ID,
		"lines": []map[string]any{{
			"item_code": "IGNORED",
			"quantity":  1.0,
		}},
	})

	if got := textValue(normalized["tax_profile_code"]); got != "CUST-TAX" {
		t.Fatalf("expected customer tax profile, got %q", got)
	}
	if got := textValue(normalized["price_list_code"]); got != "CUST-PL" {
		t.Fatalf("expected customer price list, got %q", got)
	}
	if got := numberValue(normalized["payment_term_days"]); got != 30 {
		t.Fatalf("expected customer payment terms 30, got %v", got)
	}
}

func TestApplyPartyCommercialDefaultsPrefersActiveCustomerProfile(t *testing.T) {
	models := model.NewService()
	mustRegisterCommercialModels(t, models)
	service := NewCommercialCoreService(document.NewService(), config.NewService(), models, search.NewService())

	party, err := models.Create("party", "user_admin", map[string]any{
		"name":                    "Dual Profile Customer",
		"tax_profile_code":        "LEGACY-TAX",
		"default_price_list_code": "LEGACY-PRICE",
		"payment_term_days":       7,
		"customer_type":           "legacy",
		"member_status":           "inactive",
		"member_tier":             "bronze",
		"status":                  "active",
	})
	if err != nil {
		t.Fatalf("create party: %v", err)
	}
	if _, err := models.Create("customer_profile", "user_admin", map[string]any{
		"party_id":                party.ID,
		"customer_name":           "AAA Inactive",
		"tax_profile_code":        "INACTIVE-TAX",
		"default_price_list_code": "INACTIVE-PRICE",
		"payment_term_days":       14,
		"customer_type":           "inactive-type",
		"member_status":           "inactive",
		"member_tier":             "silver",
		"status":                  "inactive",
	}); err != nil {
		t.Fatalf("create inactive customer profile: %v", err)
	}
	if _, err := models.Create("customer_profile", "user_admin", map[string]any{
		"party_id":                party.ID,
		"customer_name":           "ZZZ Active",
		"tax_profile_code":        "ACTIVE-TAX",
		"default_price_list_code": "ACTIVE-PRICE",
		"payment_term_days":       45,
		"customer_type":           "active-type",
		"member_status":           "active",
		"member_tier":             "gold",
		"status":                  "active",
	}); err != nil {
		t.Fatalf("create active customer profile: %v", err)
	}

	normalized := service.NormalizePayload("sales_order", map[string]any{
		"party_id": party.ID,
		"lines":    []map[string]any{{"description": "Service", "quantity": 1.0, "unit_price": 100.0}},
	})

	if got := textValue(normalized["tax_profile_code"]); got != "ACTIVE-TAX" {
		t.Fatalf("expected active tax profile, got %q", got)
	}
	if got := textValue(normalized["price_list_code"]); got != "ACTIVE-PRICE" {
		t.Fatalf("expected active price list, got %q", got)
	}
	if got := numberValue(normalized["payment_term_days"]); got != 45 {
		t.Fatalf("expected active payment terms, got %v", got)
	}
	fields := service.partyDiscountFields(party.ID)
	if got := textValue(fields["customer_type"]); got != "active-type" {
		t.Fatalf("expected active customer type, got %q", got)
	}
	if got := textValue(fields["member_status"]); got != "active" {
		t.Fatalf("expected active member status, got %q", got)
	}
	if got := textValue(fields["member_tier"]); got != "gold" {
		t.Fatalf("expected active member tier, got %q", got)
	}
}

func TestCommercialValidateApproveRejectsUnknownReferences(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterCommercialDocumentTypes(t, docs)
	mustRegisterCommercialModels(t, models)

	service := NewCommercialCoreService(docs, config.NewService(), models, nil)
	record, err := docs.Create("sales_order", "org_default", "loc_main", "user_admin", map[string]any{
		"party_id":         "missing_party",
		"default_tax_code": "missing_tax",
		"lines": []map[string]any{{
			"item_code": "missing_item",
			"quantity":  1.0,
			"tax_code":  "missing_tax",
		}},
	})
	if err != nil {
		t.Fatalf("create sales order: %v", err)
	}
	if err := service.ValidateApprove(record); err == nil {
		t.Fatal("expected unknown commercial references to be rejected")
	}
}

func mustRegisterCommercialModels(t *testing.T, models *model.Service) {
	t.Helper()
	RegisterCommercialModelRules(models)
	for _, def := range []model.Definition{
		{
			Key:         "party",
			DisplayName: "Party",
			DefaultSort: "name",
			Fields: []model.FieldDefinition{
				{Key: "name", Type: "string", Required: true},
				{Key: "display_name", Type: "string"},
				{Key: "email", Type: "string"},
				{Key: "currency_code", Type: "string"},
				{Key: "customer_type", Type: "string"},
				{Key: "member_status", Type: "string"},
				{Key: "member_tier", Type: "string"},
				{Key: "member_valid_from", Type: "string"},
				{Key: "member_valid_to", Type: "string"},
				{Key: "tax_profile_code", Type: "string"},
				{Key: "default_price_list_code", Type: "string"},
				{Key: "payment_term_days", Type: "number"},
				{Key: "status", Type: "string"},
			},
		},
		{
			Key:         "customer_profile",
			DisplayName: "Customer Profile",
			DefaultSort: "customer_name",
			Fields: []model.FieldDefinition{
				{Key: "party_id", Type: "string", Required: true},
				{Key: "customer_name", Type: "string"},
				{Key: "customer_type", Type: "string"},
				{Key: "member_status", Type: "string"},
				{Key: "member_tier", Type: "string"},
				{Key: "member_valid_from", Type: "string"},
				{Key: "member_valid_to", Type: "string"},
				{Key: "tax_profile_code", Type: "string"},
				{Key: "default_price_list_code", Type: "string"},
				{Key: "payment_term_days", Type: "number"},
				{Key: "status", Type: "string"},
			},
		},
		{
			Key:         "commercial_product",
			DisplayName: "Commercial Product",
			DefaultSort: "code",
			Fields: []model.FieldDefinition{
				{Key: "code", Type: "string", Required: true},
				{Key: "name", Type: "string", Required: true},
				{Key: "item_type", Type: "string"},
				{Key: "category_code", Type: "string"},
				{Key: "uom_code", Type: "string"},
				{Key: "base_price", Type: "number"},
				{Key: "unit_price", Type: "number"},
				{Key: "tax_code", Type: "string"},
				{Key: "revenue_account_code", Type: "string"},
				{Key: "status", Type: "string"},
			},
		},
		{
			Key:         "commercial_item",
			DisplayName: "Commercial Item",
			DefaultSort: "sku",
			Fields: []model.FieldDefinition{
				{Key: "sku", Type: "string", Required: true},
				{Key: "name", Type: "string", Required: true},
				{Key: "description", Type: "string"},
				{Key: "product_code", Type: "string", ConstraintRuleKeys: []string{"commercial.item.product_link"}},
				{Key: "is_variant", Type: "bool"},
				{Key: "variant_signature", Type: "string", ConstraintRuleKeys: []string{"commercial.item.variant_signature.unique"}},
				{Key: "variant_label", Type: "string"},
				{Key: "variant_values", Type: "string"},
				{Key: "category_code", Type: "string"},
				{Key: "item_type", Type: "string"},
				{Key: "kind", Type: "string", Required: true},
				{Key: "uom_code", Type: "string"},
				{Key: "base_price", Type: "number"},
				{Key: "unit_price", Type: "number"},
				{Key: "tax_code", Type: "string"},
				{Key: "revenue_account_code", Type: "string"},
				{Key: "is_sellable", Type: "bool"},
				{Key: "status", Type: "string"},
			},
		},
		{
			Key:         "commercial_price_list",
			DisplayName: "Commercial Price List",
			DefaultSort: "code",
			Fields: []model.FieldDefinition{
				{Key: "code", Type: "string", Required: true},
				{Key: "name", Type: "string", Required: true},
				{Key: "currency_code", Type: "string"},
				{Key: "status", Type: "string"},
			},
		},
		{
			Key:         "commercial_price_list_item",
			DisplayName: "Commercial Price List Item",
			DefaultSort: "item_code",
			Fields: []model.FieldDefinition{
				{Key: "price_list_code", Type: "string", Required: true},
				{Key: "item_code", Type: "string", Required: true},
				{Key: "unit_price", Type: "number"},
				{Key: "currency_code", Type: "string"},
				{Key: "tax_code", Type: "string"},
				{Key: "revenue_account_code", Type: "string"},
				{Key: "status", Type: "string"},
			},
		},
		{
			Key:         "discount_rule",
			DisplayName: "Discount Rule",
			DefaultSort: "priority",
			Fields: []model.FieldDefinition{
				{Key: "code", Type: "string", Required: true},
				{Key: "name", Type: "string", Required: true},
				{Key: "promotion_campaign_code", Type: "string"},
				{Key: "campaign_name", Type: "string"},
				{Key: "event_code", Type: "string"},
				{Key: "scope", Type: "string"},
				{Key: "rule_kind", Type: "string"},
				{Key: "priority", Type: "number"},
				{Key: "start_at", Type: "string"},
				{Key: "end_at", Type: "string"},
				{Key: "weekdays", Type: "string"},
				{Key: "start_time", Type: "string"},
				{Key: "end_time", Type: "string"},
				{Key: "party_ids", Type: "string"},
				{Key: "customer_types", Type: "string"},
				{Key: "member_statuses", Type: "string"},
				{Key: "member_tiers", Type: "string"},
				{Key: "item_codes", Type: "string"},
				{Key: "product_codes", Type: "string"},
				{Key: "variant_signatures", Type: "string"},
				{Key: "category_codes", Type: "string"},
				{Key: "reward_item_codes", Type: "string"},
				{Key: "excluded_item_codes", Type: "string"},
				{Key: "excluded_product_codes", Type: "string"},
				{Key: "excluded_category_codes", Type: "string"},
				{Key: "minimum_order_total", Type: "number"},
				{Key: "minimum_line_quantity", Type: "number"},
				{Key: "buy_quantity", Type: "number"},
				{Key: "reward_quantity", Type: "number"},
				{Key: "discount_percent", Type: "number"},
				{Key: "discount_amount", Type: "number"},
				{Key: "fixed_price", Type: "number"},
				{Key: "reward_percent", Type: "number"},
				{Key: "status", Type: "string"},
			},
		},
		{
			Key:         "promotion_campaign",
			DisplayName: "Promotion Campaign",
			DefaultSort: "code",
			Fields: []model.FieldDefinition{
				{Key: "code", Type: "string", Required: true},
				{Key: "name", Type: "string", Required: true},
				{Key: "trigger_mode", Type: "string"},
				{Key: "start_at", Type: "string"},
				{Key: "end_at", Type: "string"},
				{Key: "sales_channels", Type: "string"},
				{Key: "store_codes", Type: "string"},
				{Key: "global_usage_cap", Type: "number"},
				{Key: "per_customer_usage_cap", Type: "number"},
				{Key: "status", Type: "string"},
			},
		},
		{
			Key:         "promotion_code",
			DisplayName: "Promotion Code",
			DefaultSort: "code",
			Fields: []model.FieldDefinition{
				{Key: "code", Type: "string", Required: true},
				{Key: "promotion_campaign_code", Type: "string", Required: true},
				{Key: "start_at", Type: "string"},
				{Key: "end_at", Type: "string"},
				{Key: "party_ids", Type: "string"},
				{Key: "member_statuses", Type: "string"},
				{Key: "member_tiers", Type: "string"},
				{Key: "total_redemption_limit", Type: "number"},
				{Key: "per_customer_redemption_limit", Type: "number"},
				{Key: "total_redemptions", Type: "number"},
				{Key: "status", Type: "string"},
			},
		},
		{
			Key:         "promotion_redemption",
			DisplayName: "Promotion Redemption",
			DefaultSort: "redeemed_at",
			Fields: []model.FieldDefinition{
				{Key: "promotion_campaign_code", Type: "string", Required: true},
				{Key: "promotion_code", Type: "string"},
				{Key: "source_document_type", Type: "string", Required: true},
				{Key: "source_document_id", Type: "string", Required: true},
				{Key: "party_id", Type: "string"},
				{Key: "sales_channel", Type: "string"},
				{Key: "store_code", Type: "string"},
				{Key: "discount_amount_total", Type: "number"},
				{Key: "redeemed_at", Type: "string"},
				{Key: "status", Type: "string"},
			},
		},
		{
			Key:         "commercial_tax_code",
			DisplayName: "Commercial Tax Code",
			DefaultSort: "code",
			Fields: []model.FieldDefinition{
				{Key: "code", Type: "string", Required: true},
				{Key: "name", Type: "string", Required: true},
				{Key: "mode", Type: "string"},
				{Key: "rate_percent", Type: "number"},
				{Key: "tax_account_code", Type: "string"},
				{Key: "status", Type: "string"},
			},
		},
		{
			Key:         "commercial_tax_profile",
			DisplayName: "Commercial Tax Profile",
			DefaultSort: "code",
			Fields: []model.FieldDefinition{
				{Key: "code", Type: "string", Required: true},
				{Key: "name", Type: "string", Required: true},
				{Key: "default_tax_code", Type: "string"},
				{Key: "payment_term_days", Type: "number"},
				{Key: "price_tax_mode", Type: "string"},
				{Key: "status", Type: "string"},
			},
		},
		{
			Key:         "payment_method",
			DisplayName: "Payment Method",
			DefaultSort: "code",
			Fields: []model.FieldDefinition{
				{Key: "code", Type: "string", Required: true},
				{Key: "name", Type: "string", Required: true},
				{Key: "kind", Type: "string", Required: true},
				{Key: "clearing_account_code", Type: "string"},
				{Key: "status", Type: "string"},
			},
		},
	} {
		if err := models.Register(def); err != nil {
			t.Fatalf("register model %s: %v", def.Key, err)
		}
	}
}
