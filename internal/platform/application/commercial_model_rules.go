package application

import (
	"strings"

	"orbyte/internal/platform/model"
	"orbyte/internal/platform/shared"
)

func RegisterCommercialModelRules(models *model.Service) {
	if models == nil {
		return
	}
	models.SetConstraintEvaluator("commercial.item.product_link", func(input model.RuleInput) error {
		if input.ModelKey != "commercial_item" {
			return nil
		}
		values := input.Values
		productCode := strings.TrimSpace(textValue(values["product_code"]))
		kind := strings.ToLower(strings.TrimSpace(textValue(values["kind"])))
		itemType := strings.ToLower(strings.TrimSpace(textValue(values["item_type"])))
		isSellable := boolFieldValue(values["is_sellable"])
		hasVariantData := boolFieldValue(values["is_variant"]) ||
			strings.TrimSpace(textValue(values["variant_signature"])) != "" ||
			strings.TrimSpace(textValue(values["variant_label"])) != "" ||
			strings.TrimSpace(textValue(values["variant_values"])) != ""
		requiresProduct := hasVariantData || kind == "variant" || (isSellable && (itemType == "product" || kind == "product" || kind == "item" || kind == "simple"))
		if requiresProduct && productCode == "" {
			return shared.Validation("sellable product items and variants must belong to a commercial product")
		}
		if productCode == "" {
			return nil
		}
		products, _, err := models.List("commercial_product", model.Query{
			Filters:  map[string]string{"code": productCode},
			Page:     1,
			PageSize: 2,
		})
		if err != nil {
			return err
		}
		if len(products) == 0 {
			return shared.Validation("product_code must reference an existing commercial product")
		}
		product := products[0]
		if status := strings.ToLower(strings.TrimSpace(textValue(product.Values["status"]))); status == "inactive" {
			return shared.Validation("product_code must reference an active commercial product")
		}
		if itemType != "" {
			if productItemType := strings.TrimSpace(textValue(product.Values["item_type"])); productItemType != "" && !strings.EqualFold(productItemType, itemType) {
				return shared.Validation("commercial item item_type must match the linked commercial product")
			}
		}
		for _, fieldKey := range []string{"uom_code", "category_code", "tax_code"} {
			itemValue := strings.TrimSpace(textValue(values[fieldKey]))
			productValue := strings.TrimSpace(textValue(product.Values[fieldKey]))
			if itemValue != "" && productValue != "" && !strings.EqualFold(itemValue, productValue) {
				return shared.Validation("commercial item " + fieldKey + " must match the linked commercial product")
			}
		}
		return nil
	})
	models.SetConstraintEvaluator("commercial.item.variant_signature.unique", func(input model.RuleInput) error {
		if input.ModelKey != "commercial_item" {
			return nil
		}
		productCode := strings.TrimSpace(textValue(input.Values["product_code"]))
		variantSignature := strings.TrimSpace(textValue(input.Values["variant_signature"]))
		sku := strings.TrimSpace(textValue(input.Values["sku"]))
		existingSKU := strings.TrimSpace(textValue(input.Existing["sku"]))
		if productCode == "" || variantSignature == "" {
			return nil
		}
		items, _, err := models.List("commercial_item", model.Query{
			Filters: map[string]string{
				"product_code":      productCode,
				"variant_signature": variantSignature,
			},
			Page:     1,
			PageSize: 10,
		})
		if err != nil {
			return err
		}
		for _, item := range items {
			itemSKU := strings.TrimSpace(textValue(item.Values["sku"]))
			if existingSKU != "" && strings.EqualFold(itemSKU, existingSKU) {
				continue
			}
			if !strings.EqualFold(itemSKU, sku) {
				return shared.Validation("variant_signature must be unique within the linked commercial product")
			}
		}
		return nil
	})
}
