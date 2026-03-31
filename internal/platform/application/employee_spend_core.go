package application

import (
	"fmt"
	"strings"
	"time"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/shared"
)

type EmployeeSpendCoreService struct {
	documents *document.Service
	models    *model.Service
	workforce *EmployeeWorkforceCoreService
}

func NewEmployeeSpendCoreService(documents *document.Service, models *model.Service) *EmployeeSpendCoreService {
	return &EmployeeSpendCoreService{
		documents: documents,
		models:    models,
		workforce: NewEmployeeWorkforceCoreService(models),
	}
}

func (s *EmployeeSpendCoreService) NormalizePayload(documentType string, payload map[string]any) map[string]any {
	next := cloneMap(payload)
	s.applyEmployeeDefaults(next)
	s.applySpendProfileDefaults(next)
	switch strings.TrimSpace(documentType) {
	case "travel_request":
		if textValue(next["request_date"]) == "" {
			next["request_date"] = time.Now().UTC().Format("2006-01-02")
		}
		next["estimated_total_amount"] = roundMoney(s.claimLineTotal(recordList(next["estimated_lines"])))
		next["total_amount"] = roundMoney(numberValue(next["estimated_total_amount"]))
	case "cash_advance":
		if textValue(next["request_date"]) == "" {
			next["request_date"] = time.Now().UTC().Format("2006-01-02")
		}
		requested := firstPositive(numberValue(next["requested_amount"]), numberValue(next["total_amount"]), s.claimLineTotal(recordList(next["advance_lines"])))
		next["requested_amount"] = roundMoney(requested)
		next["total_amount"] = roundMoney(requested)
		if numberValue(next["approved_amount"]) <= 0 {
			next["approved_amount"] = roundMoney(requested)
		}
		next["outstanding_amount"] = roundMoney(firstPositive(numberValue(next["outstanding_amount"]), numberValue(next["approved_amount"]), requested))
	case "expense_claim":
		if textValue(next["claim_date"]) == "" {
			next["claim_date"] = time.Now().UTC().Format("2006-01-02")
		}
		total := s.claimLineTotal(recordList(next["claim_lines"]))
		next["total_amount"] = roundMoney(total)
		if numberValue(next["approved_amount"]) <= 0 {
			next["approved_amount"] = roundMoney(total)
		}
		if numberValue(next["reimbursable_amount"]) <= 0 {
			next["reimbursable_amount"] = roundMoney(numberValue(next["approved_amount"]))
		}
	case "advance_liquidation":
		s.normalizeLiquidation(next)
	case "reimbursement_payment":
		if textValue(next["payment_date"]) == "" {
			next["payment_date"] = time.Now().UTC().Format("2006-01-02")
		}
		amountPaid := firstPositive(numberValue(next["amount_paid"]), numberValue(next["net_settlement_amount"]), numberValue(next["reimbursable_amount"]), numberValue(next["total_amount"]))
		next["amount_paid"] = roundMoney(amountPaid)
		next["total_amount"] = roundMoney(amountPaid)
	}
	if textValue(next["currency_code"]) == "" {
		next["currency_code"] = "IDR"
	}
	return next
}

func (s *EmployeeSpendCoreService) CreateReimbursementPaymentFromLiquidation(liquidationID, actorID string) (document.Record, error) {
	if s == nil || s.documents == nil || strings.TrimSpace(liquidationID) == "" {
		return document.Record{}, shared.Validation("advance liquidation is required")
	}
	liquidation, err := s.documents.Get(strings.TrimSpace(liquidationID))
	if err != nil {
		return document.Record{}, err
	}
	if liquidation.Header.Type != "advance_liquidation" {
		return document.Record{}, shared.Validation("source document must be an advance liquidation")
	}
	if liquidation.Header.Status != "approved" {
		return document.Record{}, shared.Conflict("advance liquidation must be approved before reimbursement payment")
	}
	payload := s.NormalizePayload("reimbursement_payment", map[string]any{
		"employee_id":           textValue(liquidation.Body.Payload["employee_id"]),
		"party_id":              textValue(liquidation.Body.Payload["party_id"]),
		"travel_request_id":     textValue(liquidation.Body.Payload["travel_request_id"]),
		"cash_advance_id":       textValue(liquidation.Body.Payload["cash_advance_id"]),
		"source_liquidation_id": liquidation.Header.ID,
		"currency_code":         firstNonEmptyString(textValue(liquidation.Body.Payload["currency_code"]), "IDR"),
		"net_settlement_amount": roundMoney(maxFloat(0, numberValue(liquidation.Body.Payload["net_settlement_amount"]))),
		"amount_paid":           roundMoney(maxFloat(0, numberValue(liquidation.Body.Payload["net_settlement_amount"]))),
		"notes":                 firstNonEmptyString(textValue(liquidation.Body.Payload["notes"]), "Generated from advance liquidation "+firstNonEmptyString(liquidation.Header.Number, liquidation.Header.ID)),
	})
	if numberValue(payload["amount_paid"]) <= 0 {
		return document.Record{}, shared.Validation("advance liquidation does not require reimbursement payment")
	}
	return s.documents.Create("reimbursement_payment", liquidation.Header.OrganizationID, liquidation.Header.LocationID, actorID, payload)
}

func (s *EmployeeSpendCoreService) applyEmployeeDefaults(next map[string]any) {
	employeeID := strings.TrimSpace(textValue(next["employee_id"]))
	if employeeID == "" || s == nil || s.models == nil {
		return
	}
	employee, err := s.models.Get("employee_profile", employeeID)
	if err == nil {
		if textValue(next["party_id"]) == "" {
			next["party_id"] = textValue(employee.Values["party_id"])
		}
		if textValue(next["employee_code"]) == "" {
			next["employee_code"] = textValue(employee.Values["employee_code"])
		}
	}
	if s.workforce == nil {
		return
	}
	if assignment, ok, err := s.workforce.ResolveCurrentAssignment(employeeID, time.Now().UTC()); err == nil && ok {
		assignIfEmpty(next, "organization_id", assignment.Values["organization_id"])
		assignIfEmpty(next, "location_id", assignment.Values["location_id"])
		assignIfEmpty(next, "organization_unit_id", assignment.Values["organization_unit_id"])
		assignIfEmpty(next, "department_id", assignment.Values["department_id"])
		assignIfEmpty(next, "cost_center_id", assignment.Values["cost_center_id"])
	}
}

func (s *EmployeeSpendCoreService) applySpendProfileDefaults(next map[string]any) {
	employeeID := strings.TrimSpace(textValue(next["employee_id"]))
	if employeeID == "" || s == nil || s.models == nil {
		return
	}
	items, _, err := s.models.List("employee_spend_profile", model.Query{
		Filters:  map[string]string{"employee_id": employeeID},
		SortKey:  "updated_at",
		Desc:     true,
		Page:     1,
		PageSize: model.MaxPageSize,
	})
	if err != nil {
		return
	}
	for _, item := range items {
		if !strings.EqualFold(textValue(item.Values["status"]), "active") {
			continue
		}
		assignIfEmpty(next, "currency_code", item.Values["default_currency_code"])
		assignIfEmpty(next, "payment_method_code", item.Values["default_payment_method_code"])
		assignIfEmpty(next, "payable_account_code", item.Values["payable_account_code"])
		assignIfEmpty(next, "expense_account_code", item.Values["expense_account_code"])
		assignIfEmpty(next, "treasury_account_id", item.Values["treasury_account_id"])
		return
	}
}

func (s *EmployeeSpendCoreService) normalizeLiquidation(next map[string]any) {
	claimTotal := firstPositive(numberValue(next["claim_total_amount"]), numberValue(next["reimbursable_amount"]))
	if claimID := strings.TrimSpace(textValue(next["expense_claim_id"])); claimID != "" && s != nil && s.documents != nil {
		if claim, err := s.documents.Get(claimID); err == nil {
			assignIfEmpty(next, "employee_id", claim.Body.Payload["employee_id"])
			assignIfEmpty(next, "party_id", claim.Body.Payload["party_id"])
			assignIfEmpty(next, "travel_request_id", claim.Body.Payload["travel_request_id"])
			assignIfEmpty(next, "currency_code", claim.Body.Payload["currency_code"])
			claimTotal = firstPositive(claimTotal, numberValue(claim.Body.Payload["reimbursable_amount"]), numberValue(claim.Body.Payload["approved_amount"]), numberValue(claim.Body.Payload["total_amount"]))
		}
	}
	advanceAmount := firstPositive(numberValue(next["advance_amount"]), numberValue(next["advance_applied_amount"]))
	if advanceID := strings.TrimSpace(textValue(next["cash_advance_id"])); advanceID != "" && s != nil && s.documents != nil {
		if advance, err := s.documents.Get(advanceID); err == nil {
			assignIfEmpty(next, "employee_id", advance.Body.Payload["employee_id"])
			assignIfEmpty(next, "party_id", advance.Body.Payload["party_id"])
			assignIfEmpty(next, "travel_request_id", advance.Body.Payload["travel_request_id"])
			assignIfEmpty(next, "currency_code", advance.Body.Payload["currency_code"])
			totalAdvance := firstPositive(numberValue(advance.Body.Payload["outstanding_amount"]), numberValue(advance.Body.Payload["approved_amount"]), numberValue(advance.Body.Payload["total_amount"]))
			advanceAmount = firstPositive(advanceAmount, roundMoney(maxFloat(0, totalAdvance-s.liquidatedAdvanceAmount(advanceID))), totalAdvance)
		}
	}
	net := roundMoney(claimTotal - advanceAmount)
	settlementDirection := "balanced"
	if net > 0 {
		settlementDirection = "company_owes_employee"
	} else if net < 0 {
		settlementDirection = "employee_owes_company"
	}
	next["claim_total_amount"] = roundMoney(claimTotal)
	next["advance_amount"] = roundMoney(advanceAmount)
	next["advance_applied_amount"] = roundMoney(minFloat(claimTotal, advanceAmount))
	next["net_settlement_amount"] = net
	next["settlement_direction"] = settlementDirection
	next["outstanding_amount_after"] = roundMoney(maxFloat(0, advanceAmount-claimTotal))
	next["total_amount"] = roundMoney(claimTotal)
	if textValue(next["liquidation_date"]) == "" {
		next["liquidation_date"] = time.Now().UTC().Format("2006-01-02")
	}
}

func (s *EmployeeSpendCoreService) liquidatedAdvanceAmount(advanceID string) float64 {
	if s == nil || s.documents == nil || strings.TrimSpace(advanceID) == "" {
		return 0
	}
	total := 0.0
	for _, record := range s.documents.List() {
		if record.Header.Type != "advance_liquidation" {
			continue
		}
		if record.Header.Status != "approved" {
			continue
		}
		if textValue(record.Body.Payload["cash_advance_id"]) != advanceID {
			continue
		}
		total += numberValue(record.Body.Payload["advance_applied_amount"])
	}
	return roundMoney(total)
}

func (s *EmployeeSpendCoreService) claimLineTotal(lines []map[string]any) float64 {
	total := 0.0
	for _, line := range lines {
		amount := numberValue(line["amount"])
		if amount <= 0 {
			amount = numberValue(line["quantity"]) * numberValue(line["unit_price"])
		}
		total += amount
	}
	return roundMoney(total)
}

func assignIfEmpty(next map[string]any, key string, value any) {
	if strings.TrimSpace(textValue(next[key])) != "" {
		return
	}
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) != "" {
			next[key] = typed
		}
	case nil:
	default:
		if text := strings.TrimSpace(fmt.Sprint(typed)); text != "" {
			next[key] = value
		}
	}
}

func firstPositive(values ...float64) float64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
