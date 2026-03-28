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

func mustRegisterCommercialDocumentTypes(t *testing.T, docs *document.Service) {
	t.Helper()
	for _, def := range []document.Definition{
		{Type: "sales_order", DisplayName: "Sales Order", SchemaVersion: "v1", AllowedLinkTypes: []string{"source_order", "invoice_for", "payment_for", "refund_for", "posting_for"}},
		{Type: "invoice", DisplayName: "Invoice", SchemaVersion: "v1", AllowedLinkTypes: []string{"source_order", "invoice_for", "payment_for", "refund_for", "posting_for"}},
		{Type: "credit_note", DisplayName: "Credit Note", SchemaVersion: "v1", AllowedLinkTypes: []string{"source_order", "invoice_for", "payment_for", "refund_for", "posting_for"}},
		{Type: "payment_receipt", DisplayName: "Payment Receipt", SchemaVersion: "v1", AllowedLinkTypes: []string{"source_order", "invoice_for", "payment_for", "refund_for", "posting_for"}},
		{Type: "payment_refund", DisplayName: "Payment Refund", SchemaVersion: "v1", AllowedLinkTypes: []string{"source_order", "invoice_for", "payment_for", "refund_for", "posting_for"}},
		{Type: "ledger_posting", DisplayName: "Ledger Posting", SchemaVersion: "v1", AllowedLinkTypes: []string{"source_order", "invoice_for", "payment_for", "refund_for", "posting_for"}},
	} {
		if err := docs.Register(def); err != nil {
			t.Fatalf("register document definition %s: %v", def.Type, err)
		}
	}
}

func mustRegisterCommercialModels(t *testing.T, models *model.Service) {
	t.Helper()
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
				{Key: "tax_profile_code", Type: "string"},
				{Key: "default_price_list_code", Type: "string"},
				{Key: "payment_term_days", Type: "number"},
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
