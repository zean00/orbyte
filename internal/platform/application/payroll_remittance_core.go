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

type PayrollRemittanceCoreService struct {
	documents *document.Service
	models    *model.Service
}

func NewPayrollRemittanceCoreService(documents *document.Service, models *model.Service) *PayrollRemittanceCoreService {
	return &PayrollRemittanceCoreService{documents: documents, models: models}
}

func (s *PayrollRemittanceCoreService) NormalizePayload(documentType string, payload map[string]any) map[string]any {
	next := cloneMap(payload)
	switch strings.TrimSpace(documentType) {
	case "payroll_remittance_liability":
		s.normalizeLiability(next)
	case "payroll_remittance_adjustment":
		s.normalizeAdjustment(next)
	case "payroll_remittance_batch":
		s.normalizeBatch(next)
	case "payroll_remittance_payment":
		s.normalizePayment(next)
	}
	if textValue(next["currency_code"]) == "" {
		next["currency_code"] = "IDR"
	}
	return next
}

func (s *PayrollRemittanceCoreService) GenerateLiabilitiesFromPayrollRun(runID, actorID string) ([]document.Record, document.Record, error) {
	if s == nil || s.documents == nil || s.models == nil {
		return nil, document.Record{}, shared.Validation("payroll remittance services are unavailable")
	}
	run, err := s.documents.Get(strings.TrimSpace(runID))
	if err != nil {
		return nil, document.Record{}, err
	}
	if run.Header.Type != "payroll_run" {
		return nil, document.Record{}, shared.Validation("source document must be a payroll run")
	}
	if run.Header.Status != "processed" {
		return nil, document.Record{}, shared.Conflict("payroll run must be processed before remittance generation")
	}
	if existing := s.existingLiabilitiesForRun(run.Header.ID); len(existing) > 0 {
		var posting document.Record
		postingID := textValue(existing[0].Body.Payload["generation_posting_id"])
		if postingID != "" {
			posting, _ = s.documents.Get(postingID)
		}
		return existing, posting, nil
	}

	runPayload := run.Body.Payload
	runLines := recordList(runPayload["payroll_lines"])
	if len(runLines) == 0 {
		return nil, document.Record{}, shared.Validation("payroll run has no payroll lines")
	}

	aggregates := map[string]*liabilityAggregate{}
	baseDueDate := firstNonEmptyString(textValue(runPayload["period_end_date"]), textValue(runPayload["pay_date"]))
	for _, line := range runLines {
		orgID := firstNonEmptyString(textValue(line["organization_id"]), run.Header.OrganizationID)
		locID := firstNonEmptyString(textValue(line["location_id"]), run.Header.LocationID)
		currency := firstNonEmptyString(textValue(line["currency_code"]), textValue(runPayload["currency_code"]), "IDR")
		taxRuleID := textValue(line["tax_rule_id"])
		contributionRuleID := textValue(line["contribution_rule_id"])
		if amount := roundMoney(numberValue(line["tax_withholding_total"])); amount > 0 {
			profile, obligation, dueDate, err := s.resolveProfileAndObligation(orgID, locID, taxRuleID, "", "withholding", baseDueDate)
			if err != nil {
				return nil, document.Record{}, err
			}
			s.accumulateLiability(aggregates, liabilityAggregate{
				AuthorityID:       textValue(profile.Values["remittance_authority_id"]),
				ObligationTypeID:  obligation.ID,
				OrganizationID:    orgID,
				LocationID:        locID,
				CurrencyCode:      currency,
				TreasuryAccountID: firstNonEmptyString(textValue(profile.Values["default_treasury_account_id"]), textValue(line["treasury_account_id"])),
				PaymentMethodCode: firstNonEmptyString(textValue(profile.Values["payment_method_code"]), textValue(line["payment_method_code"]), "BANK"),
				LiabilityAccount:  textValue(obligation.Values["liability_account_code"]),
				DueDate:           dueDate,
				TotalAmount:       amount,
				WithholdingAmount: amount,
				SourceLineEmployee: []string{
					textValue(line["employee_id"]),
				},
			})
		}
		if amount := roundMoney(numberValue(line["employee_contributions_total"])); amount > 0 {
			profile, obligation, dueDate, err := s.resolveProfileAndObligation(orgID, locID, "", contributionRuleID, "employee_contribution", baseDueDate)
			if err != nil {
				return nil, document.Record{}, err
			}
			s.accumulateLiability(aggregates, liabilityAggregate{
				AuthorityID:       textValue(profile.Values["remittance_authority_id"]),
				ObligationTypeID:  obligation.ID,
				OrganizationID:    orgID,
				LocationID:        locID,
				CurrencyCode:      currency,
				TreasuryAccountID: firstNonEmptyString(textValue(profile.Values["default_treasury_account_id"]), textValue(line["treasury_account_id"])),
				PaymentMethodCode: firstNonEmptyString(textValue(profile.Values["payment_method_code"]), textValue(line["payment_method_code"]), "BANK"),
				LiabilityAccount:  textValue(obligation.Values["liability_account_code"]),
				DueDate:           dueDate,
				TotalAmount:       amount,
				EmployeeAmount:    amount,
				SourceLineEmployee: []string{
					textValue(line["employee_id"]),
				},
			})
		}
		if amount := roundMoney(numberValue(line["employer_contributions_total"])); amount > 0 {
			profile, obligation, dueDate, err := s.resolveProfileAndObligation(orgID, locID, "", contributionRuleID, "employer_contribution", baseDueDate)
			if err != nil {
				return nil, document.Record{}, err
			}
			s.accumulateLiability(aggregates, liabilityAggregate{
				AuthorityID:       textValue(profile.Values["remittance_authority_id"]),
				ObligationTypeID:  obligation.ID,
				OrganizationID:    orgID,
				LocationID:        locID,
				CurrencyCode:      currency,
				TreasuryAccountID: firstNonEmptyString(textValue(profile.Values["default_treasury_account_id"]), textValue(line["treasury_account_id"])),
				PaymentMethodCode: firstNonEmptyString(textValue(profile.Values["payment_method_code"]), textValue(line["payment_method_code"]), "BANK"),
				LiabilityAccount:  textValue(obligation.Values["liability_account_code"]),
				DueDate:           dueDate,
				TotalAmount:       amount,
				EmployerAmount:    amount,
				SourceLineEmployee: []string{
					textValue(line["employee_id"]),
				},
			})
		}
	}
	if len(aggregates) == 0 {
		return nil, document.Record{}, shared.Validation("payroll run does not contain remittance-generating totals")
	}

	payableAccountCode := firstNonEmptyString(textValue(runPayload["payable_account_code"]), "2105-PAYROLL")
	totalAmount := 0.0
	journalLines := []map[string]any{}
	creditByAccount := map[string]float64{}
	keys := make([]string, 0, len(aggregates))
	for key := range aggregates {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		item := aggregates[key]
		totalAmount = roundMoney(totalAmount + item.TotalAmount)
		creditByAccount[item.LiabilityAccount] = roundMoney(creditByAccount[item.LiabilityAccount] + item.TotalAmount)
	}
	journalLines = append(journalLines, map[string]any{
		"account_code": payableAccountCode,
		"debit":        roundMoney(totalAmount),
		"credit":       0.0,
	})
	creditAccounts := make([]string, 0, len(creditByAccount))
	for account := range creditByAccount {
		creditAccounts = append(creditAccounts, account)
	}
	sort.Strings(creditAccounts)
	for _, account := range creditAccounts {
		journalLines = append(journalLines, map[string]any{
			"account_code": account,
			"debit":        0.0,
			"credit":       roundMoney(creditByAccount[account]),
		})
	}
	posting, err := s.documents.Create("ledger_posting", run.Header.OrganizationID, run.Header.LocationID, actorID, map[string]any{
		"posting_date":         firstNonEmptyString(textValue(runPayload["pay_date"]), textValue(runPayload["period_end_date"]), time.Now().UTC().Format("2006-01-02")),
		"currency_code":        firstNonEmptyString(textValue(runPayload["currency_code"]), "IDR"),
		"posting_rule_key":     "payroll_remittance_generation",
		"journal_source_kind":  "remittance",
		"source_document_type": "payroll_run",
		"source_document_id":   run.Header.ID,
		"notes":                fmt.Sprintf("Generated remittance liabilities for %s", firstNonEmptyString(run.Header.Number, run.Header.ID)),
		"journal_lines":        journalLines,
		"total_amount":         roundMoney(totalAmount),
	})
	if err != nil {
		return nil, document.Record{}, err
	}
	posting.Header.Status = "posted"
	posting.Header.Version++
	posting.Header.ETag = fmt.Sprintf("%s:%d", posting.Header.ID, posting.Header.Version)
	posting.Header.UpdatedAt = time.Now().UTC()
	posting.Header.UpdatedBy = actorID
	if err := s.documents.Save(posting); err != nil {
		return nil, document.Record{}, err
	}

	liabilities := make([]document.Record, 0, len(keys))
	for _, key := range keys {
		item := aggregates[key]
		payload := s.NormalizePayload("payroll_remittance_liability", map[string]any{
			"source_payroll_run_id":           run.Header.ID,
			"payroll_period_id":               textValue(runPayload["payroll_period_id"]),
			"remittance_authority_id":         item.AuthorityID,
			"remittance_obligation_type_id":   item.ObligationTypeID,
			"organization_id":                 item.OrganizationID,
			"location_id":                     item.LocationID,
			"currency_code":                   item.CurrencyCode,
			"treasury_account_id":             item.TreasuryAccountID,
			"payment_method_code":             item.PaymentMethodCode,
			"liability_account_code":          item.LiabilityAccount,
			"due_date":                        item.DueDate,
			"employee_withholding_amount":     roundMoney(item.WithholdingAmount),
			"employee_contribution_amount":    roundMoney(item.EmployeeAmount),
			"employer_contribution_amount":    roundMoney(item.EmployerAmount),
			"net_liability_amount":            roundMoney(item.TotalAmount),
			"amount":                          roundMoney(item.TotalAmount),
			"total_amount":                    roundMoney(item.TotalAmount),
			"outstanding_amount":              roundMoney(item.TotalAmount),
			"paid_amount":                     0.0,
			"generation_posting_id":           posting.Header.ID,
			"source_employee_ids":             item.SourceLineEmployee,
			"notes":                           fmt.Sprintf("Generated from payroll run %s", firstNonEmptyString(run.Header.Number, run.Header.ID)),
		})
		liability, err := s.documents.Create("payroll_remittance_liability", item.OrganizationID, item.LocationID, actorID, payload)
		if err != nil {
			return nil, document.Record{}, err
		}
		liability.Header.Status = "open"
		liability.Header.Version++
		liability.Header.ETag = fmt.Sprintf("%s:%d", liability.Header.ID, liability.Header.Version)
		liability.Header.UpdatedAt = time.Now().UTC()
		liability.Header.UpdatedBy = actorID
		if err := s.documents.Save(liability); err != nil {
			return nil, document.Record{}, err
		}
		liabilities = append(liabilities, liability)
	}

	return liabilities, posting, nil
}

func (s *PayrollRemittanceCoreService) CreatePaymentFromBatch(batchID, actorID string) (document.Record, document.Record, error) {
	if s == nil || s.documents == nil || s.models == nil {
		return document.Record{}, document.Record{}, shared.Validation("payroll remittance services are unavailable")
	}
	batch, err := s.documents.Get(strings.TrimSpace(batchID))
	if err != nil {
		return document.Record{}, document.Record{}, err
	}
	if batch.Header.Type != "payroll_remittance_batch" {
		return document.Record{}, document.Record{}, shared.Validation("source document must be a remittance batch")
	}
	if batch.Header.Status != "approved" {
		return document.Record{}, document.Record{}, shared.Conflict("remittance batch must be approved before payment creation")
	}
	if existing := s.existingPaymentForBatch(batch.Header.ID); existing != "" {
		return document.Record{}, document.Record{}, shared.Conflict("remittance payment already exists for this batch")
	}
	liabilityIDs := payrollStringList(batch.Body.Payload["liability_ids"])
	if len(liabilityIDs) == 0 {
		return document.Record{}, document.Record{}, shared.Validation("remittance batch has no liabilities")
	}
	liabilities := make([]document.Record, 0, len(liabilityIDs))
	totalAmount := 0.0
	debitByAccount := map[string]float64{}
	expectedOrgID := ""
	expectedLocID := ""
	expectedAuthorityID := ""
	expectedCurrencyCode := ""
	expectedTreasuryAccountID := ""
	expectedPaymentMethod := ""
	for _, liabilityID := range liabilityIDs {
		liability, err := s.documents.Get(liabilityID)
		if err != nil {
			return document.Record{}, document.Record{}, err
		}
		if liability.Header.Type != "payroll_remittance_liability" {
			return document.Record{}, document.Record{}, shared.Validation("batch contains a non-remittance liability document")
		}
		outstanding := roundMoney(numberValue(liability.Body.Payload["outstanding_amount"]))
		if outstanding <= 0 {
			return document.Record{}, document.Record{}, shared.Conflict("remittance batch contains a fully settled liability")
		}
		orgID := liability.Header.OrganizationID
		locID := liability.Header.LocationID
		authorityID := textValue(liability.Body.Payload["remittance_authority_id"])
		currencyCode := firstNonEmptyString(textValue(liability.Body.Payload["currency_code"]), "IDR")
		treasuryAccountID := textValue(liability.Body.Payload["treasury_account_id"])
		paymentMethodCode := firstNonEmptyString(textValue(liability.Body.Payload["payment_method_code"]), "BANK")
		if expectedOrgID == "" {
			expectedOrgID = orgID
			expectedLocID = locID
			expectedAuthorityID = authorityID
			expectedCurrencyCode = currencyCode
			expectedTreasuryAccountID = treasuryAccountID
			expectedPaymentMethod = paymentMethodCode
		} else if expectedOrgID != orgID || expectedLocID != locID || expectedAuthorityID != authorityID || expectedCurrencyCode != currencyCode || expectedTreasuryAccountID != treasuryAccountID || expectedPaymentMethod != paymentMethodCode {
			return document.Record{}, document.Record{}, shared.Validation("remittance batch contains incompatible liabilities")
		}
		liabilities = append(liabilities, liability)
		totalAmount = roundMoney(totalAmount + outstanding)
		accountCode := firstNonEmptyString(textValue(liability.Body.Payload["liability_account_code"]), "2310-STATUTORY-PAYABLE")
		debitByAccount[accountCode] = roundMoney(debitByAccount[accountCode] + outstanding)
	}

	paymentPayload := s.NormalizePayload("payroll_remittance_payment", map[string]any{
		"payroll_remittance_batch_id": batch.Header.ID,
		"liability_ids":               liabilityIDs,
		"remittance_authority_id":     expectedAuthorityID,
		"payment_date":                firstNonEmptyString(textValue(batch.Body.Payload["payment_date"]), time.Now().UTC().Format("2006-01-02")),
		"due_date":                    textValue(batch.Body.Payload["due_date"]),
		"currency_code":               expectedCurrencyCode,
		"treasury_account_id":         expectedTreasuryAccountID,
		"payment_method_code":         expectedPaymentMethod,
		"amount_paid":                 roundMoney(totalAmount),
		"total_amount":                roundMoney(totalAmount),
		"notes":                       fmt.Sprintf("Settled remittance batch %s", firstNonEmptyString(batch.Header.Number, batch.Header.ID)),
	})
	payment, err := s.documents.Create("payroll_remittance_payment", expectedOrgID, expectedLocID, actorID, paymentPayload)
	if err != nil {
		return document.Record{}, document.Record{}, err
	}

	treasuryAccountCode := s.resolveTreasuryGLAccount(expectedTreasuryAccountID)
	journalLines := []map[string]any{}
	debitAccounts := make([]string, 0, len(debitByAccount))
	for account := range debitByAccount {
		debitAccounts = append(debitAccounts, account)
	}
	sort.Strings(debitAccounts)
	for _, account := range debitAccounts {
		journalLines = append(journalLines, map[string]any{
			"account_code": account,
			"debit":        roundMoney(debitByAccount[account]),
			"credit":       0.0,
		})
	}
	journalLines = append(journalLines, map[string]any{
		"account_code": treasuryAccountCode,
		"debit":        0.0,
		"credit":       roundMoney(totalAmount),
	})
	posting, err := s.documents.Create("ledger_posting", expectedOrgID, expectedLocID, actorID, map[string]any{
		"posting_date":         firstNonEmptyString(textValue(paymentPayload["payment_date"]), time.Now().UTC().Format("2006-01-02")),
		"currency_code":        firstNonEmptyString(textValue(paymentPayload["currency_code"]), "IDR"),
		"posting_rule_key":     "payroll_remittance_payment",
		"journal_source_kind":  "remittance",
		"source_document_type": "payroll_remittance_payment",
		"source_document_id":   payment.Header.ID,
		"notes":                fmt.Sprintf("Remittance payment for %s", firstNonEmptyString(batch.Header.Number, batch.Header.ID)),
		"journal_lines":        journalLines,
		"total_amount":         roundMoney(totalAmount),
	})
	if err != nil {
		return document.Record{}, document.Record{}, err
	}
	posting.Header.Status = "posted"
	posting.Header.Version++
	posting.Header.ETag = fmt.Sprintf("%s:%d", posting.Header.ID, posting.Header.Version)
	posting.Header.UpdatedAt = time.Now().UTC()
	posting.Header.UpdatedBy = actorID
	if err := s.documents.Save(posting); err != nil {
		return document.Record{}, document.Record{}, err
	}

	payment.Body.Payload = document.NormalizePayload(cloneMap(payment.Body.Payload))
	payment.Body.Payload["posted_ledger_id"] = posting.Header.ID
	payment.Header.Status = "paid"
	payment.Header.Version++
	payment.Header.ETag = fmt.Sprintf("%s:%d", payment.Header.ID, payment.Header.Version)
	payment.Header.UpdatedAt = time.Now().UTC()
	payment.Header.UpdatedBy = actorID
	payment.Body.ContentHash = document.ContentHash(payment.Body.Payload)
	if err := s.documents.Save(payment); err != nil {
		return document.Record{}, document.Record{}, err
	}

	paymentIDs := []string{payment.Header.ID}
	for _, liability := range liabilities {
		payload := document.NormalizePayload(cloneMap(liability.Body.Payload))
		paidAmount := roundMoney(numberValue(payload["paid_amount"]) + numberValue(payload["outstanding_amount"]))
		payload["paid_amount"] = paidAmount
		payload["outstanding_amount"] = 0.0
		payload["payment_ids"] = paymentIDs
		liability.Body.Payload = payload
		liability.Body.ContentHash = document.ContentHash(liability.Body.Payload)
		liability.Header.Status = "paid"
		liability.Header.Version++
		liability.Header.ETag = fmt.Sprintf("%s:%d", liability.Header.ID, liability.Header.Version)
		liability.Header.UpdatedAt = time.Now().UTC()
		liability.Header.UpdatedBy = actorID
		if err := s.documents.Save(liability); err != nil {
			return document.Record{}, document.Record{}, err
		}
	}

	batch.Body.Payload = document.NormalizePayload(cloneMap(batch.Body.Payload))
	batch.Body.Payload["payment_ids"] = paymentIDs
	batch.Body.Payload["posted_ledger_id"] = posting.Header.ID
	batch.Body.Payload["paid_amount"] = roundMoney(totalAmount)
	batch.Body.Payload["outstanding_amount"] = 0.0
	batch.Header.Status = "paid"
	batch.Header.Version++
	batch.Header.ETag = fmt.Sprintf("%s:%d", batch.Header.ID, batch.Header.Version)
	batch.Header.UpdatedAt = time.Now().UTC()
	batch.Header.UpdatedBy = actorID
	batch.Body.ContentHash = document.ContentHash(batch.Body.Payload)
	if err := s.documents.Save(batch); err != nil {
		return document.Record{}, document.Record{}, err
	}

	return payment, posting, nil
}

func (s *PayrollRemittanceCoreService) normalizeLiability(next map[string]any) {
	amount := roundMoney(firstPositive(numberValue(next["amount"]), numberValue(next["total_amount"]), numberValue(next["net_liability_amount"])))
	next["amount"] = amount
	next["total_amount"] = amount
	next["net_liability_amount"] = amount
	if _, exists := next["paid_amount"]; !exists {
		next["paid_amount"] = 0.0
	}
	if numberValue(next["outstanding_amount"]) == 0 {
		next["outstanding_amount"] = roundMoney(amount - numberValue(next["paid_amount"]))
	}
	if textValue(next["due_date"]) == "" {
		next["due_date"] = firstNonEmptyString(textValue(next["payment_date"]), time.Now().UTC().Format("2006-01-02"))
	}
}

func (s *PayrollRemittanceCoreService) normalizeAdjustment(next map[string]any) {
	if textValue(next["adjustment_date"]) == "" {
		next["adjustment_date"] = time.Now().UTC().Format("2006-01-02")
	}
	amount := roundMoney(numberValue(next["amount"]))
	next["amount"] = amount
	next["total_amount"] = absMoney(amount)
}

func (s *PayrollRemittanceCoreService) normalizeBatch(next map[string]any) {
	if textValue(next["payment_date"]) == "" {
		next["payment_date"] = time.Now().UTC().Format("2006-01-02")
	}
	liabilityIDs := payrollStringList(next["liability_ids"])
	if len(liabilityIDs) == 0 || s == nil || s.documents == nil {
		return
	}
	total := 0.0
	authorityID := ""
	currency := ""
	treasuryAccountID := ""
	paymentMethod := ""
	dueDate := ""
	obligationTypeIDs := make([]string, 0, len(liabilityIDs))
	for _, liabilityID := range liabilityIDs {
		liability, err := s.documents.Get(liabilityID)
		if err != nil || liability.Header.Type != "payroll_remittance_liability" {
			continue
		}
		if !s.isOpenLiability(liability.Header.Status) {
			continue
		}
		authorityID = firstNonEmptyString(authorityID, textValue(liability.Body.Payload["remittance_authority_id"]))
		currency = firstNonEmptyString(currency, textValue(liability.Body.Payload["currency_code"]))
		treasuryAccountID = firstNonEmptyString(treasuryAccountID, textValue(liability.Body.Payload["treasury_account_id"]))
		paymentMethod = firstNonEmptyString(paymentMethod, textValue(liability.Body.Payload["payment_method_code"]))
		liabilityDueDate := textValue(liability.Body.Payload["due_date"])
		if dueDate == "" || (liabilityDueDate != "" && liabilityDueDate < dueDate) {
			dueDate = liabilityDueDate
		}
		obligationTypeIDs = append(obligationTypeIDs, textValue(liability.Body.Payload["remittance_obligation_type_id"]))
		total = roundMoney(total + numberValue(liability.Body.Payload["outstanding_amount"]))
	}
	next["liability_ids"] = liabilityIDs
	next["remittance_authority_id"] = authorityID
	next["remittance_obligation_type_ids"] = obligationTypeIDs
	next["currency_code"] = firstNonEmptyString(currency, textValue(next["currency_code"]), "IDR")
	next["treasury_account_id"] = firstNonEmptyString(treasuryAccountID, textValue(next["treasury_account_id"]))
	next["payment_method_code"] = firstNonEmptyString(paymentMethod, textValue(next["payment_method_code"]), "BANK")
	next["due_date"] = firstNonEmptyString(dueDate, textValue(next["due_date"]), textValue(next["payment_date"]))
	next["total_amount"] = roundMoney(total)
	next["outstanding_amount"] = roundMoney(total)
}

func (s *PayrollRemittanceCoreService) normalizePayment(next map[string]any) {
	if textValue(next["payment_date"]) == "" {
		next["payment_date"] = time.Now().UTC().Format("2006-01-02")
	}
	amount := roundMoney(firstPositive(numberValue(next["amount_paid"]), numberValue(next["total_amount"])))
	next["amount_paid"] = amount
	next["total_amount"] = amount
}

func (s *PayrollRemittanceCoreService) accumulateLiability(aggregates map[string]*liabilityAggregate, item liabilityAggregate) {
	key := strings.Join([]string{
		item.AuthorityID,
		item.ObligationTypeID,
		item.OrganizationID,
		item.LocationID,
		item.CurrencyCode,
		item.TreasuryAccountID,
		item.PaymentMethodCode,
		item.LiabilityAccount,
		item.DueDate,
	}, "|")
	if existing, ok := aggregates[key]; ok {
		existing.TotalAmount = roundMoney(existing.TotalAmount + item.TotalAmount)
		existing.WithholdingAmount = roundMoney(existing.WithholdingAmount + item.WithholdingAmount)
		existing.EmployeeAmount = roundMoney(existing.EmployeeAmount + item.EmployeeAmount)
		existing.EmployerAmount = roundMoney(existing.EmployerAmount + item.EmployerAmount)
		existing.SourceLineEmployee = append(existing.SourceLineEmployee, item.SourceLineEmployee...)
		return
	}
	copyItem := item
	copyItem.SourceLineEmployee = append([]string(nil), item.SourceLineEmployee...)
	aggregates[key] = &copyItem
}

type liabilityAggregate struct {
	AuthorityID        string
	ObligationTypeID   string
	OrganizationID     string
	LocationID         string
	CurrencyCode       string
	TreasuryAccountID  string
	PaymentMethodCode  string
	LiabilityAccount   string
	DueDate            string
	TotalAmount        float64
	WithholdingAmount  float64
	EmployeeAmount     float64
	EmployerAmount     float64
	SourceLineEmployee []string
}

func (s *PayrollRemittanceCoreService) resolveProfileAndObligation(orgID, locID, taxRuleID, contributionRuleID, target, baseDate string) (model.Record, model.Record, string, error) {
	profile, ok := s.findActiveRemittanceProfile(orgID, locID, taxRuleID, contributionRuleID)
	if !ok {
		return model.Record{}, model.Record{}, "", shared.Validation("no active payroll remittance profile matches the payroll rule configuration")
	}
	obligationKey := ""
	switch target {
	case "withholding":
		obligationKey = "withholding_obligation_type_id"
	case "employee_contribution":
		obligationKey = "employee_contribution_obligation_type_id"
	case "employer_contribution":
		obligationKey = "employer_contribution_obligation_type_id"
	default:
		return model.Record{}, model.Record{}, "", shared.Validation("unsupported remittance obligation target")
	}
	obligationID := textValue(profile.Values[obligationKey])
	if obligationID == "" {
		return model.Record{}, model.Record{}, "", shared.Validation("remittance profile is missing obligation mapping")
	}
	obligation, err := s.models.Get("remittance_obligation_type", obligationID)
	if err != nil || !strings.EqualFold(textValue(obligation.Values["status"]), "active") {
		return model.Record{}, model.Record{}, "", shared.Validation("remittance obligation type is not available")
	}
	dueDate := s.resolveDueDate(textValue(profile.Values["remittance_authority_id"]), obligation.ID, baseDate)
	return profile, obligation, dueDate, nil
}

func (s *PayrollRemittanceCoreService) findActiveRemittanceProfile(orgID, locID, taxRuleID, contributionRuleID string) (model.Record, bool) {
	if s == nil || s.models == nil {
		return model.Record{}, false
	}
	items, _, err := s.models.List("payroll_remittance_profile", model.Query{
		SortKey:  "updated_at",
		Desc:     true,
		Page:     1,
		PageSize: model.MaxPageSize,
	})
	if err != nil {
		return model.Record{}, false
	}
	for _, item := range items {
		if !strings.EqualFold(textValue(item.Values["status"]), "active") {
			continue
		}
		if value := textValue(item.Values["organization_id"]); value != "" && value != orgID {
			continue
		}
		if value := textValue(item.Values["location_id"]); value != "" && value != locID {
			continue
		}
		if taxRuleID != "" && textValue(item.Values["payroll_tax_rule_id"]) == taxRuleID {
			return item, true
		}
		if contributionRuleID != "" && textValue(item.Values["payroll_contribution_rule_id"]) == contributionRuleID {
			return item, true
		}
	}
	return model.Record{}, false
}

func (s *PayrollRemittanceCoreService) resolveDueDate(authorityID, obligationTypeID, baseDate string) string {
	base := time.Now().UTC()
	if parsed, err := time.Parse("2006-01-02", strings.TrimSpace(baseDate)); err == nil {
		base = parsed
	}
	if s == nil || s.models == nil {
		return base.Format("2006-01-02")
	}
	items, _, err := s.models.List("remittance_schedule_rule", model.Query{
		SortKey:  "updated_at",
		Desc:     true,
		Page:     1,
		PageSize: model.MaxPageSize,
	})
	if err != nil {
		return base.Format("2006-01-02")
	}
	for _, item := range items {
		if !strings.EqualFold(textValue(item.Values["status"]), "active") {
			continue
		}
		if value := textValue(item.Values["remittance_authority_id"]); value != "" && value != authorityID {
			continue
		}
		if value := textValue(item.Values["remittance_obligation_type_id"]); value != "" && value != obligationTypeID {
			continue
		}
		if dueDay := int(numberValue(item.Values["due_day_of_month"])); dueDay > 0 {
			year, month, _ := base.Date()
			month++
			if month > 12 {
				month = 1
				year++
			}
			maxDay := daysInMonth(time.Date(year, month, 1, 0, 0, 0, 0, time.UTC))
			if dueDay > maxDay {
				dueDay = maxDay
			}
			return time.Date(year, month, dueDay, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
		}
		if offset := int(numberValue(item.Values["due_days_after_period_end"])); offset > 0 {
			return base.AddDate(0, 0, offset).Format("2006-01-02")
		}
	}
	return base.Format("2006-01-02")
}

func daysInMonth(t time.Time) int {
	return time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func (s *PayrollRemittanceCoreService) existingLiabilitiesForRun(runID string) []document.Record {
	if s == nil || s.documents == nil || strings.TrimSpace(runID) == "" {
		return nil
	}
	items := []document.Record{}
	for _, record := range s.documents.List() {
		if record.Header.Type != "payroll_remittance_liability" {
			continue
		}
		if textValue(record.Body.Payload["source_payroll_run_id"]) != runID {
			continue
		}
		if strings.EqualFold(record.Header.Status, "cancelled") {
			continue
		}
		items = append(items, record)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Header.ID < items[j].Header.ID
	})
	return items
}

func (s *PayrollRemittanceCoreService) existingPaymentForBatch(batchID string) string {
	if s == nil || s.documents == nil || strings.TrimSpace(batchID) == "" {
		return ""
	}
	for _, record := range s.documents.List() {
		if record.Header.Type != "payroll_remittance_payment" {
			continue
		}
		if textValue(record.Body.Payload["payroll_remittance_batch_id"]) != batchID {
			continue
		}
		if strings.EqualFold(record.Header.Status, "cancelled") {
			continue
		}
		return record.Header.ID
	}
	return ""
}

func (s *PayrollRemittanceCoreService) resolveTreasuryGLAccount(treasuryAccountID string) string {
	if s == nil || s.models == nil || strings.TrimSpace(treasuryAccountID) == "" {
		return "1010-BANK"
	}
	account, err := s.models.Get("treasury_account", treasuryAccountID)
	if err != nil {
		return "1010-BANK"
	}
	return firstNonEmptyString(textValue(account.Values["gl_account_code"]), "1010-BANK")
}

func (s *PayrollRemittanceCoreService) isOpenLiability(status string) bool {
	normalized := strings.ToLower(strings.TrimSpace(status))
	return normalized == "open" || normalized == "partially_paid"
}
