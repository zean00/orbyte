package app

import (
	"testing"

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
