package app

import (
	"fmt"
	"testing"
	"time"

	"orbyte/internal/platform/model"
)

func ensurePartyRecord(t *testing.T, models *model.Service, actorID, partyID, name string) model.Record {
	t.Helper()
	if record, err := models.Get("party", partyID); err == nil {
		return record
	}
	return ensureModelByCode(t, models, "party", "name", name, map[string]any{
		"id":         partyID,
		"party_type": "organization",
		"name":       name,
		"status":     "active",
	}, actorID)
}

func ensureWarehouseRecord(t *testing.T, models *model.Service, actorID, code, organizationID, locationID string) model.Record {
	t.Helper()
	return ensureModelByCode(t, models, "warehouse", "code", code, map[string]any{
		"code":            code,
		"name":            code + " Warehouse",
		"kind":            "storage",
		"organization_id": organizationID,
		"location_id":     locationID,
		"status":          "active",
	}, actorID)
}

func ensureLocationRecord(t *testing.T, models *model.Service, actorID, locationID string) model.Record {
	t.Helper()
	if record, err := models.Get("location", locationID); err == nil {
		return record
	}
	return ensureModelByCode(t, models, "location", "code", locationID, map[string]any{
		"id":     locationID,
		"code":   locationID,
		"name":   "Location " + locationID,
		"status": "active",
	}, actorID)
}

func ensureOrganizationUnitRecord(t *testing.T, models *model.Service, actorID, unitID, organizationID, locationID string) model.Record {
	t.Helper()
	if record, err := models.Get("organization_unit", unitID); err == nil {
		return record
	}
	return ensureModelByCode(t, models, "organization_unit", "code", unitID, map[string]any{
		"id":              unitID,
		"organization_id": organizationID,
		"location_id":     locationID,
		"code":            unitID,
		"name":            "Unit " + unitID,
		"status":          "active",
	}, actorID)
}

func ensureDepartmentRecord(t *testing.T, models *model.Service, actorID, departmentID, organizationID, locationID, organizationUnitID string) model.Record {
	t.Helper()
	if record, err := models.Get("department", departmentID); err == nil {
		return record
	}
	return ensureModelByCode(t, models, "department", "code", departmentID, map[string]any{
		"id":                   departmentID,
		"organization_id":      organizationID,
		"location_id":          locationID,
		"organization_unit_id": organizationUnitID,
		"code":                 departmentID,
		"name":                 "Department " + departmentID,
		"status":               "active",
	}, actorID)
}

func ensureCostCenterRecord(t *testing.T, models *model.Service, actorID, costCenterID, organizationID, locationID, organizationUnitID, departmentID string) model.Record {
	t.Helper()
	if record, err := models.Get("cost_center", costCenterID); err == nil {
		return record
	}
	return ensureModelByCode(t, models, "cost_center", "code", costCenterID, map[string]any{
		"id":                   costCenterID,
		"organization_id":      organizationID,
		"location_id":          locationID,
		"organization_unit_id": organizationUnitID,
		"department_id":        departmentID,
		"code":                 costCenterID,
		"name":                 "Cost Center " + costCenterID,
		"status":               "active",
	}, actorID)
}

func ensureCommercialUOMRecord(t *testing.T, models *model.Service, actorID, code string) model.Record {
	t.Helper()
	return ensureModelByCode(t, models, "commercial_uom", "code", code, map[string]any{
		"code":   code,
		"name":   code,
		"symbol": code,
		"status": "active",
	}, actorID)
}

func ensureFinanceAccountRecord(t *testing.T, models *model.Service, actorID, code, name, accountType, reportGroup, normalBalance string) model.Record {
	t.Helper()
	return ensureModelByCode(t, models, "finance_account", "code", code, map[string]any{
		"code":           code,
		"name":           name,
		"account_type":   accountType,
		"report_group":   reportGroup,
		"normal_balance": normalBalance,
		"status":         "active",
	}, actorID)
}

func ensurePaymentMethodRecord(t *testing.T, models *model.Service, actorID, code, kind, clearingAccountCode string) model.Record {
	t.Helper()
	ensureFinanceAccountRecord(t, models, actorID, clearingAccountCode, "Clearing "+code, "asset", "cash_and_bank", "debit")
	return ensureModelByCode(t, models, "payment_method", "code", code, map[string]any{
		"code":                  code,
		"name":                  code,
		"kind":                  kind,
		"clearing_account_code": clearingAccountCode,
		"status":                "active",
	}, actorID)
}

func ensurePOSTenderTypeRecord(t *testing.T, models *model.Service, actorID, code, kind string) model.Record {
	t.Helper()
	ensurePaymentMethodRecord(t, models, actorID, code, kind, "1000-CLEAR-"+code)
	return ensureModelByCode(t, models, "pos_tender_type", "code", code, map[string]any{
		"code":                  code,
		"name":                  code,
		"kind":                  kind,
		"payment_method_code":   code,
		"clearing_account_code": "1000-CLEAR-" + code,
		"status":                "active",
	}, actorID)
}

func ensureAccountingPeriodForDate(t *testing.T, models *model.Service, actorID, organizationID, locationID, postingDate string) model.Record {
	t.Helper()
	parsed, err := time.Parse("2006-01-02", postingDate)
	if err != nil {
		t.Fatalf("parse posting date %q: %v", postingDate, err)
	}
	periodKey := parsed.Format("2006-01")
	items, _, err := models.List("accounting_period", model.Query{
		Filters:  map[string]string{"organization_id": organizationID, "location_id": locationID, "period_key": periodKey},
		Page:     1,
		PageSize: 100,
	})
	if err == nil && len(items) > 0 {
		for i, item := range items {
			if text, _ := item.Values["status"].(string); text != "open" {
				updated, updateErr := models.Update("accounting_period", item.ID, actorID, mergeTestValues(item.Values, map[string]any{
					"organization_id": organizationID,
					"location_id":     locationID,
					"period_key":      periodKey,
					"status":          "open",
				}), item.Version)
				if updateErr != nil {
					t.Fatalf("reopen accounting period %s: %v", item.ID, updateErr)
				}
				items[i] = updated
			}
		}
		return items[0]
	}
	start := time.Date(parsed.Year(), parsed.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, -1)
	return ensureModelByCode(t, models, "accounting_period", "period_key", periodKey, map[string]any{
		"organization_id": organizationID,
		"location_id":     locationID,
		"period_key":      periodKey,
		"start_date":      start.Format("2006-01-02"),
		"end_date":        end.Format("2006-01-02"),
		"status":          "open",
		"name":            fmt.Sprintf("%s Period", periodKey),
	}, actorID)
}

func mergeTestValues(base map[string]any, updates map[string]any) map[string]any {
	merged := map[string]any{}
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range updates {
		merged[key] = value
	}
	return merged
}
