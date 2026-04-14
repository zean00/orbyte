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
		"organization_id": organizationID,
		"location_id":     locationID,
		"status":          "active",
	}, actorID)
}

func ensureOrganizationUnitRecord(t *testing.T, models *model.Service, actorID, unitID, organizationID, locationID string) model.Record {
	t.Helper()
	if record, err := models.Get("organization_unit", unitID); err == nil {
		return record
	}
	return ensureModelByCode(t, models, "organization_unit", "organization_unit_id", unitID, map[string]any{
		"organization_unit_id": unitID,
		"organization_id":      organizationID,
		"location_id":          locationID,
		"name":                 "Unit " + unitID,
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

func ensurePOSTenderTypeRecord(t *testing.T, models *model.Service, actorID, code, kind string) model.Record {
	t.Helper()
	return ensureModelByCode(t, models, "pos_tender_type", "code", code, map[string]any{
		"code":                  code,
		"name":                  code,
		"kind":                  kind,
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
		PageSize: 1,
	})
	if err == nil && len(items) > 0 {
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
