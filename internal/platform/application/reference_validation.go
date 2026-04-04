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
	return validateExistingModelID(models, "vendor_profile", vendorID, "vendor")
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
