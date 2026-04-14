package application

import (
	"fmt"
	"strings"

	"orbyte/internal/platform/model"
	"orbyte/internal/platform/shared"
)

func modelRegistered(models *model.Service, modelKey string) bool {
	if models == nil {
		return false
	}
	_, ok := models.Definition(strings.TrimSpace(modelKey))
	return ok
}

func modelFieldRegistered(models *model.Service, modelKey, fieldKey string) bool {
	if models == nil {
		return false
	}
	def, ok := models.Definition(strings.TrimSpace(modelKey))
	if !ok {
		return false
	}
	for _, field := range def.Fields {
		if strings.EqualFold(strings.TrimSpace(field.Key), strings.TrimSpace(fieldKey)) {
			return true
		}
	}
	return false
}

func validateExistingModelID(models *model.Service, modelKey, recordID, label string) error {
	recordID = strings.TrimSpace(recordID)
	if recordID == "" || !modelRegistered(models, modelKey) {
		return nil
	}
	if _, err := models.Get(modelKey, recordID); err != nil {
		return shared.Validation(fmt.Sprintf("%s not found", label))
	}
	return nil
}

func resolveExistingModelRecord(models *model.Service, modelKey, reference string) (model.Record, bool) {
	reference = strings.TrimSpace(reference)
	if reference == "" || !modelRegistered(models, modelKey) {
		return model.Record{}, false
	}
	if record, err := models.Get(modelKey, reference); err == nil {
		return record, true
	}
	if modelFieldRegistered(models, modelKey, "code") {
		items, _, err := models.List(modelKey, model.Query{
			Filters:  map[string]string{"code": reference},
			Page:     1,
			PageSize: 1,
		})
		if err == nil && len(items) > 0 {
			return items[0], true
		}
	}
	return model.Record{}, false
}

func validateExistingModelField(models *model.Service, modelKey, fieldKey, fieldValue, label string) error {
	fieldValue = strings.TrimSpace(fieldValue)
	if fieldValue == "" || !modelRegistered(models, modelKey) {
		return nil
	}
	items, _, err := models.List(modelKey, model.Query{
		Filters:  map[string]string{fieldKey: fieldValue},
		Page:     1,
		PageSize: 1,
	})
	if err != nil || len(items) == 0 {
		return shared.Validation(fmt.Sprintf("%s not found", label))
	}
	return nil
}

func validateWarehouseCode(models *model.Service, warehouseCode string) error {
	return validateExistingModelField(models, "warehouse", "code", warehouseCode, "warehouse")
}

func validateCommercialItemCode(models *model.Service, itemCode string) error {
	return validateExistingModelField(models, "commercial_item", "sku", itemCode, "item")
}

func validateVendorID(models *model.Service, vendorID string) error {
	if !modelRegistered(models, "vendor_profile") {
		return nil
	}
	if _, ok := resolveExistingModelRecord(models, "vendor_profile", vendorID); ok {
		return nil
	}
	return shared.Validation("vendor not found")
}

func validatePartyID(models *model.Service, partyID string) error {
	if _, ok := resolveExistingModelRecord(models, "party", partyID); ok {
		return nil
	}
	return shared.Validation("party not found")
}

func validateTreasuryAccountID(models *model.Service, treasuryAccountID string) error {
	if _, ok := resolveExistingModelRecord(models, "treasury_account", treasuryAccountID); ok {
		return nil
	}
	return shared.Validation("treasury account not found")
}

func validateCommercialTaxCode(models *model.Service, taxCode string) error {
	return validateExistingModelField(models, "commercial_tax_code", "code", taxCode, "tax code")
}

func validateLocationID(models *model.Service, locationID string) error {
	if !modelRegistered(models, "location") {
		return nil
	}
	if _, ok := resolveExistingModelRecord(models, "location", locationID); ok {
		return nil
	}
	return shared.Validation("location not found")
}

func validateCostCenterID(models *model.Service, costCenterID string) error {
	if _, ok := resolveExistingModelRecord(models, "cost_center", costCenterID); ok {
		return nil
	}
	return shared.Validation("cost center not found")
}

func validateEmployeeID(models *model.Service, employeeID string) error {
	if _, ok := resolveExistingModelRecord(models, "employee_profile", employeeID); ok {
		return nil
	}
	if modelFieldRegistered(models, "employee_profile", "employee_code") {
		return validateExistingModelField(models, "employee_profile", "employee_code", employeeID, "employee")
	}
	return shared.Validation("employee not found")
}

func validateCustomerPartyID(models *model.Service, partyID string) error {
	partyID = strings.TrimSpace(partyID)
	if partyID == "" {
		return nil
	}
	if modelRegistered(models, "customer_profile") {
		items, _, err := models.List("customer_profile", model.Query{
			Filters:  map[string]string{"party_id": partyID},
			Page:     1,
			PageSize: 1,
		})
		if err == nil && len(items) > 0 {
			return nil
		}
	}
	if modelRegistered(models, "party") {
		if _, err := models.Get("party", partyID); err == nil {
			return nil
		}
		return shared.Validation("party not found")
	}
	if modelRegistered(models, "customer_profile") {
		return shared.Validation("customer not found")
	}
	return nil
}
