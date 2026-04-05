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

type EmployeePayrollCoreService struct {
	documents  *document.Service
	models     *model.Service
	workforce  *EmployeeWorkforceCoreService
	attendance *WorkforceAttendanceCoreService
	spend      *EmployeeSpendCoreService
}

func NewEmployeePayrollCoreService(documents *document.Service, models *model.Service, attendance *WorkforceAttendanceCoreService, spend *EmployeeSpendCoreService) *EmployeePayrollCoreService {
	return &EmployeePayrollCoreService{
		documents:  documents,
		models:     models,
		workforce:  NewEmployeeWorkforceCoreService(models),
		attendance: attendance,
		spend:      spend,
	}
}

func (s *EmployeePayrollCoreService) NormalizePayload(documentType string, payload map[string]any) map[string]any {
	next := cloneMap(payload)
	switch strings.TrimSpace(documentType) {
	case "payroll_run":
		s.normalizePayrollRun(next)
	case "payroll_adjustment":
		s.normalizePayrollAdjustment(next)
	case "payroll_payment_batch":
		s.normalizePayrollPaymentBatch(next)
	case "payroll_payment":
		s.normalizePayrollPayment(next)
	}
	if textValue(next["currency_code"]) == "" {
		next["currency_code"] = "IDR"
	}
	return next
}

func (s *EmployeePayrollCoreService) CreatePaymentBatchFromRun(runID, actorID string) (document.Record, []document.Record, document.Record, error) {
	if s == nil || s.documents == nil {
		return document.Record{}, nil, document.Record{}, shared.Validation("payroll documents are unavailable")
	}
	run, err := s.documents.Get(strings.TrimSpace(runID))
	if err != nil {
		return document.Record{}, nil, document.Record{}, err
	}
	if run.Header.Type != "payroll_run" {
		return document.Record{}, nil, document.Record{}, shared.Validation("source document must be a payroll run")
	}
	if run.Header.Status != "processed" {
		return document.Record{}, nil, document.Record{}, shared.Conflict("payroll run must be processed before payment batch creation")
	}
	if existingBatchID := s.existingPaymentBatchForRun(run.Header.ID); existingBatchID != "" {
		return document.Record{}, nil, document.Record{}, shared.Conflict("payroll payment batch already exists for this payroll run")
	}
	runPayload := run.Body.Payload
	runLines := recordList(runPayload["payroll_lines"])
	if len(runLines) == 0 {
		return document.Record{}, nil, document.Record{}, shared.Validation("payroll run has no payroll lines")
	}
	if err := validateTreasuryAccountID(s.models, textValue(runPayload["treasury_account_id"])); err != nil {
		return document.Record{}, nil, document.Record{}, err
	}
	for _, row := range runLines {
		if err := validateEmployeeID(s.models, textValue(row["employee_id"])); err != nil {
			return document.Record{}, nil, document.Record{}, err
		}
		if err := validatePartyID(s.models, textValue(row["party_id"])); err != nil {
			return document.Record{}, nil, document.Record{}, err
		}
		if err := validateCostCenterID(s.models, textValue(row["cost_center_id"])); err != nil {
			return document.Record{}, nil, document.Record{}, err
		}
		if err := validateTreasuryAccountID(s.models, firstNonEmptyString(textValue(row["treasury_account_id"]), textValue(runPayload["treasury_account_id"]))); err != nil {
			return document.Record{}, nil, document.Record{}, err
		}
	}

	batchPayload := s.NormalizePayload("payroll_payment_batch", map[string]any{
		"payroll_run_id":      run.Header.ID,
		"payroll_period_id":   textValue(runPayload["payroll_period_id"]),
		"payment_date":        firstNonEmptyString(textValue(runPayload["pay_date"]), time.Now().UTC().Format("2006-01-02")),
		"currency_code":       firstNonEmptyString(textValue(runPayload["currency_code"]), "IDR"),
		"treasury_account_id": textValue(runPayload["treasury_account_id"]),
		"payment_method_code": textValue(runPayload["payment_method_code"]),
		"employee_count":      len(runLines),
		"total_amount":        roundMoney(numberValue(runPayload["net_pay_total"])),
		"net_pay_total":       roundMoney(numberValue(runPayload["net_pay_total"])),
		"notes":               fmt.Sprintf("Generated from payroll run %s", firstNonEmptyString(run.Header.Number, run.Header.ID)),
	})
	batch, err := s.documents.Create("payroll_payment_batch", run.Header.OrganizationID, run.Header.LocationID, actorID, batchPayload)
	if err != nil {
		return document.Record{}, nil, document.Record{}, err
	}

	payments := make([]document.Record, 0, len(runLines))
	paymentIDs := make([]string, 0, len(runLines))
	for _, row := range runLines {
		paymentPayload := s.NormalizePayload("payroll_payment", map[string]any{
			"payroll_run_id":           run.Header.ID,
			"payroll_payment_batch_id": batch.Header.ID,
			"payroll_period_id":        textValue(runPayload["payroll_period_id"]),
			"employee_id":              textValue(row["employee_id"]),
			"party_id":                 textValue(row["party_id"]),
			"organization_unit_id":     textValue(row["organization_unit_id"]),
			"department_id":            textValue(row["department_id"]),
			"cost_center_id":           textValue(row["cost_center_id"]),
			"payment_date":             firstNonEmptyString(textValue(runPayload["pay_date"]), time.Now().UTC().Format("2006-01-02")),
			"currency_code":            firstNonEmptyString(textValue(runPayload["currency_code"]), "IDR"),
			"treasury_account_id":      firstNonEmptyString(textValue(row["treasury_account_id"]), textValue(runPayload["treasury_account_id"])),
			"payment_method_code":      firstNonEmptyString(textValue(row["payment_method_code"]), textValue(runPayload["payment_method_code"])),
			"net_pay":                  roundMoney(numberValue(row["net_pay"])),
			"total_amount":             roundMoney(numberValue(row["net_pay"])),
			"notes":                    fmt.Sprintf("Generated from payroll run %s for employee %s", firstNonEmptyString(run.Header.Number, run.Header.ID), textValue(row["employee_id"])),
		})
		payment, createErr := s.documents.Create("payroll_payment", run.Header.OrganizationID, run.Header.LocationID, actorID, paymentPayload)
		if createErr != nil {
			return document.Record{}, nil, document.Record{}, createErr
		}
		payments = append(payments, payment)
		paymentIDs = append(paymentIDs, payment.Header.ID)
	}

	postingPayload := map[string]any{
		"posting_date":         firstNonEmptyString(textValue(runPayload["pay_date"]), time.Now().UTC().Format("2006-01-02")),
		"currency_code":        firstNonEmptyString(textValue(runPayload["currency_code"]), "IDR"),
		"posting_rule_key":     "payroll_run",
		"journal_source_kind":  "payroll",
		"source_document_type": "payroll_run",
		"source_document_id":   run.Header.ID,
		"notes":                fmt.Sprintf("Payroll posting for %s", firstNonEmptyString(run.Header.Number, run.Header.ID)),
		"journal_lines": []map[string]any{
			{"account_code": firstNonEmptyString(textValue(runPayload["expense_account_code"]), "6200-PAYROLL"), "debit": roundMoney(numberValue(runPayload["employer_cost_total"])), "credit": 0.0},
			{"account_code": firstNonEmptyString(textValue(runPayload["payable_account_code"]), "2105-PAYROLL"), "debit": 0.0, "credit": roundMoney(numberValue(runPayload["employer_cost_total"]))},
		},
		"total_amount": roundMoney(numberValue(runPayload["employer_cost_total"])),
	}
	posting, err := s.documents.Create("ledger_posting", run.Header.OrganizationID, run.Header.LocationID, actorID, postingPayload)
	if err != nil {
		return document.Record{}, nil, document.Record{}, err
	}
	posting.Header.Status = "posted"
	posting.Header.Version++
	posting.Header.ETag = fmt.Sprintf("%s:%d", posting.Header.ID, posting.Header.Version)
	posting.Header.UpdatedAt = time.Now().UTC()
	posting.Header.UpdatedBy = actorID
	if err := s.documents.Save(posting); err != nil {
		return document.Record{}, nil, document.Record{}, err
	}

	updatedBatch := batch
	updatedBatch.Body.Payload = document.NormalizePayload(cloneMap(updatedBatch.Body.Payload))
	updatedBatch.Body.Payload["payroll_payment_ids"] = paymentIDs
	updatedBatch.Body.Payload["ledger_posting_id"] = posting.Header.ID
	updatedBatch.Body.ContentHash = document.ContentHash(updatedBatch.Body.Payload)
	updatedBatch.Header.Version++
	updatedBatch.Header.ETag = fmt.Sprintf("%s:%d", updatedBatch.Header.ID, updatedBatch.Header.Version)
	updatedBatch.Header.UpdatedAt = time.Now().UTC()
	updatedBatch.Header.UpdatedBy = actorID
	if err := s.documents.Save(updatedBatch); err != nil {
		return document.Record{}, nil, document.Record{}, err
	}

	return updatedBatch, payments, posting, nil
}

func (s *EmployeePayrollCoreService) existingPaymentBatchForRun(runID string) string {
	if s == nil || s.documents == nil || strings.TrimSpace(runID) == "" {
		return ""
	}
	for _, record := range s.documents.List() {
		if record.Header.Type != "payroll_payment_batch" {
			continue
		}
		if textValue(record.Body.Payload["payroll_run_id"]) != runID {
			continue
		}
		if strings.EqualFold(record.Header.Status, "cancelled") {
			continue
		}
		return record.Header.ID
	}
	return ""
}

func (s *EmployeePayrollCoreService) normalizePayrollRun(next map[string]any) {
	if textValue(next["run_date"]) == "" {
		next["run_date"] = time.Now().UTC().Format("2006-01-02")
	}
	periodStart, periodEnd, payDate := s.resolvePayrollPeriod(next)
	if textValue(next["period_start_date"]) == "" {
		next["period_start_date"] = periodStart
	}
	if textValue(next["period_end_date"]) == "" {
		next["period_end_date"] = periodEnd
	}
	if textValue(next["pay_date"]) == "" {
		next["pay_date"] = payDate
	}
	employeeIDs := payrollStringList(next["employee_ids"])
	if len(employeeIDs) == 0 && textValue(next["employee_id"]) != "" {
		employeeIDs = []string{textValue(next["employee_id"])}
	}

	lines := make([]map[string]any, 0, len(employeeIDs))
	employeeCount := 0
	grossTotal := 0.0
	deductionTotal := 0.0
	contributionTotal := 0.0
	taxTotal := 0.0
	reimbursementTotal := 0.0
	netTotal := 0.0
	employerContributionTotal := 0.0
	employerCostTotal := 0.0

	for _, employeeID := range employeeIDs {
		line, ok := s.buildPayrollLine(strings.TrimSpace(employeeID), periodStart, periodEnd)
		if !ok {
			continue
		}
		lines = append(lines, line)
		employeeCount++
		grossTotal = roundMoney(grossTotal + numberValue(line["gross_pay"]))
		deductionTotal = roundMoney(deductionTotal + numberValue(line["employee_deductions_total"]))
		contributionTotal = roundMoney(contributionTotal + numberValue(line["employee_contributions_total"]))
		taxTotal = roundMoney(taxTotal + numberValue(line["tax_withholding_total"]))
		reimbursementTotal = roundMoney(reimbursementTotal + numberValue(line["reimbursement_total"]))
		netTotal = roundMoney(netTotal + numberValue(line["net_pay"]))
		employerContributionTotal = roundMoney(employerContributionTotal + numberValue(line["employer_contributions_total"]))
		employerCostTotal = roundMoney(employerCostTotal + numberValue(line["employer_cost_total"]))
		if textValue(next["organization_id"]) == "" {
			assignIfEmpty(next, "organization_id", line["organization_id"])
			assignIfEmpty(next, "location_id", line["location_id"])
			assignIfEmpty(next, "treasury_account_id", line["treasury_account_id"])
			assignIfEmpty(next, "payment_method_code", line["payment_method_code"])
		}
	}
	next["employee_ids"] = employeeIDs
	next["employee_count"] = employeeCount
	next["payroll_lines"] = lines
	next["gross_pay_total"] = roundMoney(grossTotal)
	next["employee_deductions_total"] = roundMoney(deductionTotal)
	next["employee_contributions_total"] = roundMoney(contributionTotal)
	next["tax_withholding_total"] = roundMoney(taxTotal)
	next["reimbursement_total"] = roundMoney(reimbursementTotal)
	next["net_pay_total"] = roundMoney(netTotal)
	next["employer_contributions_total"] = roundMoney(employerContributionTotal)
	next["employer_cost_total"] = roundMoney(employerCostTotal)
	next["total_amount"] = roundMoney(netTotal)
	next["currency_code"] = firstNonEmptyString(textValue(next["currency_code"]), "IDR")
	next["payable_account_code"] = firstNonEmptyString(textValue(next["payable_account_code"]), "2105-PAYROLL")
	next["expense_account_code"] = firstNonEmptyString(textValue(next["expense_account_code"]), "6200-PAYROLL")
}

func (s *EmployeePayrollCoreService) normalizePayrollAdjustment(next map[string]any) {
	if textValue(next["adjustment_date"]) == "" {
		next["adjustment_date"] = time.Now().UTC().Format("2006-01-02")
	}
	amount := roundMoney(numberValue(next["amount"]))
	next["amount"] = amount
	next["total_amount"] = amount
}

func (s *EmployeePayrollCoreService) normalizePayrollPaymentBatch(next map[string]any) {
	if textValue(next["payment_date"]) == "" {
		next["payment_date"] = time.Now().UTC().Format("2006-01-02")
	}
	amount := roundMoney(firstPositive(numberValue(next["total_amount"]), numberValue(next["net_pay_total"])))
	next["total_amount"] = amount
	next["net_pay_total"] = amount
}

func (s *EmployeePayrollCoreService) normalizePayrollPayment(next map[string]any) {
	if textValue(next["payment_date"]) == "" {
		next["payment_date"] = time.Now().UTC().Format("2006-01-02")
	}
	amount := roundMoney(firstPositive(numberValue(next["net_pay"]), numberValue(next["total_amount"])))
	next["net_pay"] = amount
	next["total_amount"] = amount
}

func (s *EmployeePayrollCoreService) buildPayrollLine(employeeID, periodStart, periodEnd string) (map[string]any, bool) {
	if s == nil || s.models == nil || employeeID == "" {
		return nil, false
	}
	employee, err := s.models.Get("employee_profile", employeeID)
	if err != nil || !strings.EqualFold(textValue(employee.Values["status"]), "active") {
		return nil, false
	}
	profile, hasProfile := s.findActivePayrollProfile(employeeID)
	compProfile, _ := s.findActiveCompensationProfile(employeeID)
	assignment, _, _ := s.workforce.ResolveCurrentAssignment(employeeID, attendanceDate(periodEnd))
	salaryStructureID := textValue(profile.Values["salary_structure_id"])
	structureLines := s.listSalaryStructureLines(salaryStructureID)
	workedHours, overtimeHours, deductibleLeaveDays := s.collectAttendanceMetrics(employeeID, periodStart, periodEnd)
	sourceReimbursementTotal := 0.0
	if hasProfile && payrollBoolValue(profile.Values["reimbursement_in_payroll"]) {
		sourceReimbursementTotal = s.collectPayrollReimbursements(employeeID, periodStart, periodEnd)
	}
	lineItems := make([]map[string]any, 0, len(structureLines)+3)
	grossPay := 0.0
	employeeDeductions := 0.0
	employeeContrib := 0.0
	employerContrib := 0.0
	reimbursementTotal := 0.0

	standardRate := numberValue(compProfile.Values["standard_hourly_rate"])
	overtimeRate := firstPositive(numberValue(compProfile.Values["overtime_hourly_rate"]), standardRate)
	leaveDailyRate := firstPositive(numberValue(profile.Values["leave_deduction_daily_rate"]), standardRate*8)

	for _, structureLine := range structureLines {
		componentCode := textValue(structureLine.Values["component_code"])
		component, _ := s.findPayComponent(componentCode)
		componentClass := firstNonEmptyString(textValue(component.Values["component_class"]), "earning")
		formulaKey := firstNonEmptyString(textValue(structureLine.Values["formula_key"]), "fixed_amount")
		amount := roundMoney(numberValue(structureLine.Values["fixed_amount"]))
		switch formulaKey {
		case "hourly_work":
			amount = roundMoney(workedHours * standardRate)
		case "overtime_hours":
			amount = roundMoney(overtimeHours * overtimeRate)
		case "leave_deduction":
			amount = roundMoney(-1 * deductibleLeaveDays * leaveDailyRate)
		case "reimbursement":
			amount = roundMoney(sourceReimbursementTotal)
		}
		if amount == 0 {
			continue
		}
		lineItems = append(lineItems, map[string]any{
			"component_code":  componentCode,
			"component_name":  firstNonEmptyString(textValue(component.Values["name"]), componentCode),
			"component_class": componentClass,
			"formula_key":     formulaKey,
			"amount":          amount,
		})
		switch componentClass {
		case "deduction":
			employeeDeductions = roundMoney(employeeDeductions + absMoney(amount))
		case "employee_contribution":
			employeeContrib = roundMoney(employeeContrib + absMoney(amount))
		case "employer_contribution":
			employerContrib = roundMoney(employerContrib + absMoney(amount))
		case "reimbursement":
			reimbursementTotal = roundMoney(reimbursementTotal + amount)
		default:
			grossPay = roundMoney(grossPay + amount)
		}
	}

	taxWithholding := s.computeRuleAmount(textValue(profile.Values["tax_rule_id"]), grossPay, "employee")
	contribEmployeeFromRule := s.computeContributionRuleAmount(textValue(profile.Values["contribution_rule_id"]), grossPay, "employee")
	contribEmployerFromRule := s.computeContributionRuleAmount(textValue(profile.Values["contribution_rule_id"]), grossPay, "employer")
	if contribEmployeeFromRule > 0 {
		employeeContrib = roundMoney(employeeContrib + contribEmployeeFromRule)
	}
	if contribEmployerFromRule > 0 {
		employerContrib = roundMoney(employerContrib + contribEmployerFromRule)
	}
	if taxWithholding > 0 {
		lineItems = append(lineItems, map[string]any{
			"component_code":  "tax_withholding",
			"component_name":  "Tax Withholding",
			"component_class": "tax",
			"formula_key":     "tax_rule",
			"amount":          roundMoney(-taxWithholding),
		})
	}

	netPay := roundMoney(grossPay + reimbursementTotal - employeeDeductions - employeeContrib - taxWithholding)
	employerCost := roundMoney(grossPay + reimbursementTotal + employerContrib)

	return map[string]any{
		"employee_id":                  employeeID,
		"party_id":                     firstNonEmptyString(textValue(profile.Values["payroll_party_id"]), textValue(employee.Values["party_id"])),
		"organization_id":              firstNonEmptyString(textValue(assignment.Values["organization_id"]), textValue(employee.Values["organization_id"])),
		"location_id":                  firstNonEmptyString(textValue(assignment.Values["location_id"]), textValue(employee.Values["location_id"])),
		"organization_unit_id":         firstNonEmptyString(textValue(assignment.Values["organization_unit_id"]), textValue(employee.Values["organization_unit_id"])),
		"department_id":                firstNonEmptyString(textValue(assignment.Values["department_id"]), textValue(employee.Values["department_id"])),
		"cost_center_id":               firstNonEmptyString(textValue(assignment.Values["cost_center_id"]), textValue(employee.Values["cost_center_id"])),
		"currency_code":                firstNonEmptyString(textValue(profile.Values["currency_code"]), textValue(compProfile.Values["currency_code"]), "IDR"),
		"payment_method_code":          firstNonEmptyString(textValue(profile.Values["payment_method_code"]), "BANK"),
		"treasury_account_id":          textValue(profile.Values["treasury_account_id"]),
		"tax_rule_id":                  textValue(profile.Values["tax_rule_id"]),
		"contribution_rule_id":         textValue(profile.Values["contribution_rule_id"]),
		"salary_structure_id":          salaryStructureID,
		"worked_hours":                 roundMoney(workedHours),
		"overtime_hours":               roundMoney(overtimeHours),
		"deductible_leave_days":        roundMoney(deductibleLeaveDays),
		"gross_pay":                    roundMoney(grossPay),
		"employee_deductions_total":    roundMoney(employeeDeductions),
		"employee_contributions_total": roundMoney(employeeContrib),
		"tax_withholding_total":        roundMoney(taxWithholding),
		"reimbursement_total":          roundMoney(reimbursementTotal),
		"net_pay":                      roundMoney(netPay),
		"employer_contributions_total": roundMoney(employerContrib),
		"employer_cost_total":          roundMoney(employerCost),
		"component_lines":              lineItems,
	}, true
}

func (s *EmployeePayrollCoreService) resolvePayrollPeriod(payload map[string]any) (string, string, string) {
	now := time.Now().UTC()
	start := textValue(payload["period_start_date"])
	end := textValue(payload["period_end_date"])
	payDate := textValue(payload["pay_date"])
	if periodID := textValue(payload["payroll_period_id"]); periodID != "" && s != nil && s.models != nil {
		if period, err := s.models.Get("payroll_period", periodID); err == nil {
			start = firstNonEmptyString(start, textValue(period.Values["start_date"]))
			end = firstNonEmptyString(end, textValue(period.Values["end_date"]))
			payDate = firstNonEmptyString(payDate, textValue(period.Values["pay_date"]))
			assignIfEmpty(payload, "organization_id", period.Values["organization_id"])
			assignIfEmpty(payload, "location_id", period.Values["location_id"])
		}
	}
	if start == "" {
		start = now.Format("2006-01-01")
	}
	if end == "" {
		end = now.Format("2006-01-02")
	}
	if payDate == "" {
		payDate = end
	}
	return start, end, payDate
}

func (s *EmployeePayrollCoreService) findActivePayrollProfile(employeeID string) (model.Record, bool) {
	if s == nil || s.models == nil || employeeID == "" {
		return model.Record{}, false
	}
	items, _, err := s.models.List("employee_payroll_profile", model.Query{
		Filters:  map[string]string{"employee_id": employeeID},
		SortKey:  "updated_at",
		Desc:     true,
		Page:     1,
		PageSize: model.MaxPageSize,
	})
	if err != nil {
		return model.Record{}, false
	}
	for _, item := range items {
		if strings.EqualFold(textValue(item.Values["status"]), "active") {
			return item, true
		}
	}
	return model.Record{}, false
}

func (s *EmployeePayrollCoreService) findActiveCompensationProfile(employeeID string) (model.Record, bool) {
	if s == nil || s.models == nil || employeeID == "" {
		return model.Record{}, false
	}
	items, _, err := s.models.List("employee_compensation_profile", model.Query{
		Filters:  map[string]string{"employee_id": employeeID},
		SortKey:  "updated_at",
		Desc:     true,
		Page:     1,
		PageSize: model.MaxPageSize,
	})
	if err != nil {
		return model.Record{}, false
	}
	for _, item := range items {
		if strings.EqualFold(textValue(item.Values["status"]), "active") {
			return item, true
		}
	}
	return model.Record{}, false
}

func (s *EmployeePayrollCoreService) listSalaryStructureLines(structureID string) []model.Record {
	if s == nil || s.models == nil || structureID == "" {
		return nil
	}
	items, _, err := s.models.List("salary_structure_line", model.Query{
		Filters:  map[string]string{"salary_structure_id": structureID},
		SortKey:  "sequence",
		Desc:     false,
		Page:     1,
		PageSize: model.MaxPageSize,
	})
	if err != nil {
		return nil
	}
	filtered := make([]model.Record, 0, len(items))
	for _, item := range items {
		if strings.EqualFold(textValue(item.Values["status"]), "active") {
			filtered = append(filtered, item)
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return int(numberValue(filtered[i].Values["sequence"])) < int(numberValue(filtered[j].Values["sequence"]))
	})
	return filtered
}

func (s *EmployeePayrollCoreService) findPayComponent(code string) (model.Record, bool) {
	if s == nil || s.models == nil || code == "" {
		return model.Record{}, false
	}
	items, _, err := s.models.List("pay_component", model.Query{
		Filters:  map[string]string{"code": code},
		SortKey:  "updated_at",
		Desc:     true,
		Page:     1,
		PageSize: model.MaxPageSize,
	})
	if err != nil || len(items) == 0 {
		return model.Record{}, false
	}
	return items[0], true
}

func (s *EmployeePayrollCoreService) collectAttendanceMetrics(employeeID, startDate, endDate string) (float64, float64, float64) {
	if s == nil || s.models == nil {
		return 0, 0, 0
	}
	items, _, err := s.models.List("attendance_day", model.Query{
		Filters:  map[string]string{"employee_id": employeeID},
		SortKey:  "attendance_date",
		Desc:     false,
		Page:     1,
		PageSize: model.MaxPageSize,
	})
	if err != nil {
		return 0, 0, 0
	}
	worked := 0.0
	overtime := 0.0
	leaveDays := 0.0
	for _, item := range items {
		day := textValue(item.Values["attendance_date"])
		if !dateWithin(day, startDate, endDate) || !strings.EqualFold(textValue(item.Values["status"]), "active") {
			continue
		}
		worked = roundMoney(worked + numberValue(item.Values["worked_hours"]))
		overtime = roundMoney(overtime + numberValue(item.Values["overtime_hours"]))
		if leaveID := textValue(item.Values["leave_request_id"]); leaveID != "" && s.leaveDeductsFromPayroll(leaveID) {
			leaveDays = roundMoney(leaveDays + 1)
		}
	}
	return worked, overtime, leaveDays
}

func (s *EmployeePayrollCoreService) leaveDeductsFromPayroll(leaveID string) bool {
	if s == nil || s.models == nil || leaveID == "" {
		return false
	}
	return NewLeavePolicyCoreService(s.models, nil, nil, nil).LeaveDeductsFromPayroll(leaveID)
}

func (s *EmployeePayrollCoreService) collectPayrollReimbursements(employeeID, startDate, endDate string) float64 {
	if s == nil || s.documents == nil {
		return 0
	}
	total := 0.0
	for _, record := range s.documents.List() {
		if record.Header.Type != "reimbursement_payment" {
			continue
		}
		if !inDocumentFinalState(record.Header.Status, "paid") {
			continue
		}
		if textValue(record.Body.Payload["employee_id"]) != employeeID {
			continue
		}
		if !payrollBoolValue(record.Body.Payload["include_in_payroll"]) {
			continue
		}
		if !dateWithin(textValue(record.Body.Payload["payment_date"]), startDate, endDate) {
			continue
		}
		total = roundMoney(total + numberValue(record.Body.Payload["amount_paid"]))
	}
	return total
}

func (s *EmployeePayrollCoreService) computeRuleAmount(ruleID string, grossPay float64, role string) float64 {
	if s == nil || s.models == nil || ruleID == "" {
		return 0
	}
	rule, err := s.models.Get("payroll_tax_rule", ruleID)
	if err != nil || !strings.EqualFold(textValue(rule.Values["status"]), "active") {
		return 0
	}
	base := maxFloat(0, grossPay-numberValue(rule.Values["threshold_amount"]))
	fixed := numberValue(rule.Values["fixed_amount"])
	rate := 0.0
	if role == "employer" {
		rate = numberValue(rule.Values["employer_rate_percent"])
	} else {
		rate = numberValue(rule.Values["employee_rate_percent"])
	}
	return roundMoney(fixed + (base * rate / 100))
}

func (s *EmployeePayrollCoreService) computeContributionRuleAmount(ruleID string, grossPay float64, role string) float64 {
	if s == nil || s.models == nil || ruleID == "" {
		return 0
	}
	rule, err := s.models.Get("payroll_contribution_rule", ruleID)
	if err != nil || !strings.EqualFold(textValue(rule.Values["status"]), "active") {
		return 0
	}
	base := maxFloat(0, grossPay-numberValue(rule.Values["threshold_amount"]))
	rate := 0.0
	fixed := 0.0
	if role == "employer" {
		rate = numberValue(rule.Values["employer_rate_percent"])
		fixed = numberValue(rule.Values["employer_fixed_amount"])
	} else {
		rate = numberValue(rule.Values["employee_rate_percent"])
		fixed = numberValue(rule.Values["employee_fixed_amount"])
	}
	return roundMoney(fixed + (base * rate / 100))
}

func payrollStringList(value any) []string {
	seen := map[string]struct{}{}
	items := []string{}
	switch typed := value.(type) {
	case []string:
		for _, item := range typed {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			items = append(items, item)
		}
	case []any:
		for _, item := range typed {
			text := strings.TrimSpace(textValue(item))
			if text == "" {
				continue
			}
			if _, ok := seen[text]; ok {
				continue
			}
			seen[text] = struct{}{}
			items = append(items, text)
		}
	}
	sort.Strings(items)
	return items
}

func dateWithin(day, startDate, endDate string) bool {
	day = strings.TrimSpace(day)
	if day == "" {
		return false
	}
	if startDate != "" && day < startDate {
		return false
	}
	if endDate != "" && day > endDate {
		return false
	}
	return true
}

func inDocumentFinalState(status string, states ...string) bool {
	normalized := strings.ToLower(strings.TrimSpace(status))
	for _, state := range states {
		if normalized == strings.ToLower(strings.TrimSpace(state)) {
			return true
		}
	}
	return false
}

func absMoney(value float64) float64 {
	if value < 0 {
		return roundMoney(-value)
	}
	return roundMoney(value)
}

func payrollBoolValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		normalized := strings.ToLower(strings.TrimSpace(typed))
		return normalized == "true" || normalized == "1" || normalized == "yes"
	default:
		return false
	}
}
