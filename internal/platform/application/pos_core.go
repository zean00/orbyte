package application

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/shared"
)

type POSCoreService struct {
	documents   *document.Service
	models      *model.Service
	search      *search.Service
	actions     *DocumentActions
	commercial  *CommercialCoreService
	inventory   *InventoryCoreService
	fulfillment *FulfillmentCoreService
	returns     *ReturnsCoreService
	retail      *RetailFinanceCoreService
	attendance  *WorkforceAttendanceCoreService
}

func NewPOSCoreService(documents *document.Service, models *model.Service, searchSvc *search.Service, actions *DocumentActions, commercialSvc *CommercialCoreService, inventorySvc *InventoryCoreService, fulfillmentSvc *FulfillmentCoreService, returnsSvc *ReturnsCoreService) *POSCoreService {
	return &POSCoreService{
		documents:   documents,
		models:      models,
		search:      searchSvc,
		actions:     actions,
		commercial:  commercialSvc,
		inventory:   inventorySvc,
		fulfillment: fulfillmentSvc,
		returns:     returnsSvc,
	}
}

func (s *POSCoreService) SetRetailFinance(retail *RetailFinanceCoreService) {
	s.retail = retail
}

func (s *POSCoreService) AttachWorkforceAttendance(attendance *WorkforceAttendanceCoreService) {
	s.attendance = attendance
}

type POSCatalogItem struct {
	ItemCode          string  `json:"item_code"`
	ProductCode       string  `json:"product_code,omitempty"`
	Name              string  `json:"name"`
	Description       string  `json:"description,omitempty"`
	ItemType          string  `json:"item_type,omitempty"`
	VariantLabel      string  `json:"variant_label,omitempty"`
	VariantSignature  string  `json:"variant_signature,omitempty"`
	UOMCode           string  `json:"uom_code,omitempty"`
	UnitPrice         float64 `json:"unit_price"`
	TaxCode           string  `json:"tax_code,omitempty"`
	TaxRate           float64 `json:"tax_rate"`
	TaxMode           string  `json:"tax_mode,omitempty"`
	CurrencyCode      string  `json:"currency_code,omitempty"`
	InventoryEnabled  bool    `json:"inventory_enabled"`
	AvailableQuantity float64 `json:"available_quantity"`
	OnHandQuantity    float64 `json:"on_hand_quantity"`
}

type POSBootstrap struct {
	Stores               []model.Record      `json:"stores"`
	Registers            []model.Record      `json:"registers"`
	TenderTypes          []model.Record      `json:"tender_types"`
	OpenShift            *model.Record       `json:"open_shift,omitempty"`
	CurrentStore         *model.Record       `json:"current_store,omitempty"`
	CurrentCashier       *POSCashierSummary  `json:"current_cashier,omitempty"`
	TerminalContext      *POSTerminalContext `json:"terminal_context,omitempty"`
	CashierPINConfigured bool                `json:"cashier_pin_configured"`
}

type POSCashierSummary struct {
	UserID   string `json:"user_id"`
	Username string `json:"username,omitempty"`
	Name     string `json:"name,omitempty"`
}

type POSTerminalContext struct {
	CashierUserID string `json:"cashier_user_id"`
	StoreCode     string `json:"store_code"`
	RegisterCode  string `json:"register_code"`
	ShiftID       string `json:"shift_id"`
	VerifiedAt    string `json:"verified_at,omitempty"`
}

type POSCustomerLookupItem struct {
	PartyID       string `json:"party_id"`
	CustomerName  string `json:"customer_name"`
	CustomerType  string `json:"customer_type,omitempty"`
	MemberStatus  string `json:"member_status,omitempty"`
	MemberTier    string `json:"member_tier,omitempty"`
	MemberValidTo string `json:"member_valid_to,omitempty"`
}

type POSStoredValueLookupResult struct {
	Kind             string  `json:"kind"`
	Code             string  `json:"code,omitempty"`
	PartyID          string  `json:"party_id,omitempty"`
	PartyName        string  `json:"party_name,omitempty"`
	CurrencyCode     string  `json:"currency_code,omitempty"`
	Status           string  `json:"status,omitempty"`
	RemainingBalance float64 `json:"remaining_balance,omitempty"`
	BalanceAmount    float64 `json:"balance_amount,omitempty"`
}

type POSCartLineInput struct {
	ProductCode      string  `json:"product_code,omitempty"`
	VariantSignature string  `json:"variant_signature,omitempty"`
	ItemCode         string  `json:"item_code,omitempty"`
	Description      string  `json:"description,omitempty"`
	Quantity         float64 `json:"quantity"`
	DiscountAmount   float64 `json:"discount_amount,omitempty"`
	Note             string  `json:"note,omitempty"`
}

type POSTenderInput struct {
	TenderTypeCode string  `json:"tender_type_code"`
	Amount         float64 `json:"amount"`
	Reference      string  `json:"reference,omitempty"`
	Notes          string  `json:"notes,omitempty"`
}

type POSHoldSaleInput struct {
	SaleID         string             `json:"sale_id,omitempty"`
	StoreCode      string             `json:"store_code"`
	RegisterCode   string             `json:"register_code"`
	ShiftID        string             `json:"shift_id"`
	PartyID        string             `json:"party_id,omitempty"`
	PartyName      string             `json:"party_name,omitempty"`
	Notes          string             `json:"notes,omitempty"`
	CheckoutMode   string             `json:"checkout_mode,omitempty"`
	Lines          []POSCartLineInput `json:"lines"`
	Tenders        []POSTenderInput   `json:"tenders,omitempty"`
	PromotionCodes []string           `json:"promotion_codes,omitempty"`
	Reference      string             `json:"reference,omitempty"`
	DeviceID       string             `json:"device_id,omitempty"`
	OfflineCached  bool               `json:"offline_cached,omitempty"`
}

type POSCheckoutInput struct {
	SaleID         string             `json:"sale_id,omitempty"`
	StoreCode      string             `json:"store_code"`
	RegisterCode   string             `json:"register_code"`
	ShiftID        string             `json:"shift_id"`
	PartyID        string             `json:"party_id,omitempty"`
	PartyName      string             `json:"party_name,omitempty"`
	Notes          string             `json:"notes,omitempty"`
	CheckoutMode   string             `json:"checkout_mode,omitempty"`
	Lines          []POSCartLineInput `json:"lines"`
	Tenders        []POSTenderInput   `json:"tenders"`
	PromotionCodes []string           `json:"promotion_codes,omitempty"`
	Reference      string             `json:"reference,omitempty"`
	DeviceID       string             `json:"device_id,omitempty"`
	OfflineCached  bool               `json:"offline_cached,omitempty"`
}

type POSCheckoutResult struct {
	Sale         model.Record      `json:"sale"`
	Order        *document.Record  `json:"order,omitempty"`
	Invoice      *document.Record  `json:"invoice,omitempty"`
	Fulfillment  *document.Record  `json:"fulfillment,omitempty"`
	Payments     []document.Record `json:"payments,omitempty"`
	PrimaryDocID string            `json:"primary_document_id,omitempty"`
	ReceiptTitle string            `json:"receipt_title,omitempty"`
}

type POSPromotionCodeValidation struct {
	Code                  string   `json:"code"`
	Status                string   `json:"status"`
	Message               string   `json:"message"`
	PromotionCampaignCode string   `json:"promotion_campaign_code,omitempty"`
	DiscountAmount        float64  `json:"discount_amount,omitempty"`
	RuleCodes             []string `json:"rule_codes,omitempty"`
}

type POSPromotionValidationResult struct {
	Valid               bool                         `json:"valid"`
	CurrencyCode        string                       `json:"currency_code,omitempty"`
	DiscountAmountTotal float64                      `json:"discount_amount_total,omitempty"`
	Codes               []POSPromotionCodeValidation `json:"codes"`
}

func (s *POSCoreService) Bootstrap(locationID, storeCode, registerCode, cashierUserID string) (POSBootstrap, error) {
	stores := s.listActiveModels("pos_store")
	registers := s.listActiveModels("pos_register")
	tenderTypes := s.listActiveModels("pos_tender_type")
	filteredStores := make([]model.Record, 0, len(stores))
	for _, store := range stores {
		if locationID != "" && strings.TrimSpace(textValue(store.Values["location_id"])) != "" && strings.TrimSpace(textValue(store.Values["location_id"])) != strings.TrimSpace(locationID) {
			continue
		}
		filteredStores = append(filteredStores, store)
	}
	bootstrap := POSBootstrap{
		Stores:      filteredStores,
		Registers:   registers,
		TenderTypes: tenderTypes,
	}
	if storeCode != "" {
		if store, ok := s.findModelByField("pos_store", "code", storeCode); ok {
			bootstrap.CurrentStore = &store
		}
	}
	if registerCode != "" && cashierUserID != "" {
		if shift, ok := s.findOpenShift(registerCode, cashierUserID); ok {
			bootstrap.OpenShift = &shift
		}
	}
	return bootstrap, nil
}

func (s *POSCoreService) ValidateTerminalContext(storeCode, registerCode, shiftID, cashierUserID string) (model.Record, error) {
	shift, _, _, err := s.validateSaleContext(storeCode, registerCode, shiftID, cashierUserID)
	if err != nil {
		return model.Record{}, err
	}
	return shift, nil
}

func (s *POSCoreService) ResumeShift(storeCode, registerCode, shiftID, cashierUserID string) (model.Record, error) {
	shift, err := s.ValidateTerminalContext(storeCode, registerCode, shiftID, cashierUserID)
	if err != nil {
		return model.Record{}, err
	}
	if textValue(shift.Values["status"]) != "opened" {
		return model.Record{}, shared.Validation("shift is not open")
	}
	return shift, nil
}

func (s *POSCoreService) SearchCatalog(organizationID, locationID, storeCode, query string) ([]POSCatalogItem, error) {
	store, ok := s.findModelByField("pos_store", "code", storeCode)
	if !ok {
		return nil, shared.NotFound("pos store not found")
	}
	warehouseCode := textValue(store.Values["warehouse_code"])
	priceListCode := textValue(store.Values["price_list_code"])
	taxProfileCode := textValue(store.Values["tax_profile_code"])
	defaultTaxCode := textValue(store.Values["default_tax_code"])

	candidates := make([]model.Record, 0)
	trimmed := strings.TrimSpace(query)
	if trimmed != "" && s.search != nil {
		result, err := s.search.Query("commercial.items.search", organizationID, locationID, search.QueryRequest{
			Query:    trimmed,
			Page:     1,
			PageSize: 20,
		})
		if err == nil {
			for _, hit := range result.Hits {
				record, getErr := s.models.Get("commercial_item", hit.SourceID)
				if getErr == nil {
					candidates = append(candidates, record)
				}
			}
		}
	}
	if len(candidates) == 0 {
		items, _, err := s.models.List("commercial_item", model.Query{Page: 1, PageSize: 50})
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if !matchesCatalogQuery(item, trimmed) {
				continue
			}
			candidates = append(candidates, item)
		}
	}

	results := make([]POSCatalogItem, 0, len(candidates))
	seen := map[string]struct{}{}
	for _, item := range candidates {
		itemCode := textValue(item.Values["sku"])
		if itemCode == "" {
			continue
		}
		if _, exists := seen[itemCode]; exists {
			continue
		}
		seen[itemCode] = struct{}{}
		payload := s.commercial.NormalizePayload("sales_order", map[string]any{
			"price_list_code":  priceListCode,
			"tax_profile_code": taxProfileCode,
			"default_tax_code": defaultTaxCode,
			"sales_channel":    "pos",
			"store_code":       textValue(store.Values["code"]),
			"lines": []map[string]any{{
				"item_code": itemCode,
				"quantity":  1,
			}},
		})
		line := firstRecord(payload["lines"])
		stock := s.inventory.ItemStockScoped(organizationID, locationID, itemCode, time.Now().UTC())
		available := stockSummaryForWarehouse(stock, warehouseCode, "available_quantity")
		onHand := stockSummaryForWarehouse(stock, warehouseCode, "on_hand_quantity")
		results = append(results, POSCatalogItem{
			ItemCode:          itemCode,
			ProductCode:       textValue(item.Values["product_code"]),
			Name:              firstNonEmptyString(textValue(item.Values["name"]), textValue(line["description"])),
			Description:       textValue(line["description"]),
			ItemType:          textValue(item.Values["item_type"]),
			VariantLabel:      textValue(item.Values["variant_label"]),
			VariantSignature:  textValue(item.Values["variant_signature"]),
			UOMCode:           firstNonEmptyString(textValue(line["uom_code"]), textValue(item.Values["uom_code"])),
			UnitPrice:         numberValue(line["unit_price"]),
			TaxCode:           textValue(line["tax_code"]),
			TaxRate:           numberValue(line["tax_rate"]),
			TaxMode:           textValue(line["tax_mode"]),
			CurrencyCode:      firstNonEmptyString(textValue(payload["currency_code"]), "IDR"),
			InventoryEnabled:  boolFieldValue(item.Values["inventory_enabled"]),
			AvailableQuantity: available,
			OnHandQuantity:    onHand,
		})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})
	return results, nil
}

func (s *POSCoreService) ValidatePromotionCodes(organizationID, locationID, storeCode, partyID, partyName string, promotionCodes []string, lines []POSCartLineInput) (POSPromotionValidationResult, error) {
	result := POSPromotionValidationResult{Valid: true, Codes: []POSPromotionCodeValidation{}}
	normalizedCodes := make([]string, 0, len(promotionCodes))
	seen := map[string]struct{}{}
	for _, item := range promotionCodes {
		code := strings.TrimSpace(item)
		if code == "" {
			continue
		}
		key := strings.ToLower(code)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalizedCodes = append(normalizedCodes, code)
	}
	if len(normalizedCodes) == 0 {
		return result, nil
	}
	if strings.TrimSpace(storeCode) == "" {
		return result, shared.Validation("store is required for promotion validation")
	}
	store, ok := s.findModelByField("pos_store", "code", storeCode)
	if !ok {
		return result, shared.NotFound("pos store not found")
	}
	payload, normalizedLines, err := s.buildOrderPayload(store, partyID, partyName, "", normalizedCodes, lines)
	if err != nil {
		return result, err
	}
	payload = s.commercial.NormalizePayload("sales_order", payload)
	result.CurrencyCode = firstNonEmptyString(textValue(payload["currency_code"]), "IDR")
	result.DiscountAmountTotal = roundMoney(numberValue(payload["discount_amount_total"]))

	appliedByCode := map[string]*POSPromotionCodeValidation{}
	for _, row := range normalizedLines {
		for _, entry := range recordList(row["promotion_breakdown"]) {
			code := strings.TrimSpace(textValue(entry["promotion_code"]))
			if code == "" {
				continue
			}
			key := strings.ToLower(code)
			current := appliedByCode[key]
			if current == nil {
				current = &POSPromotionCodeValidation{
					Code:                  code,
					Status:                "applied",
					Message:               "Code applied successfully.",
					PromotionCampaignCode: textValue(entry["promotion_campaign_code"]),
					RuleCodes:             []string{},
				}
				appliedByCode[key] = current
			}
			current.DiscountAmount = roundMoney(current.DiscountAmount + numberValue(entry["discount_value"]))
			ruleCode := strings.TrimSpace(textValue(entry["rule_code"]))
			if ruleCode != "" && !containsString(current.RuleCodes, ruleCode) {
				current.RuleCodes = append(current.RuleCodes, ruleCode)
			}
		}
	}

	evalTime := s.commercial.discountEvaluationTime(payload)
	partyValues := s.commercial.partyDiscountFields(partyID)
	campaigns := s.commercial.listActivePromotionCampaigns()
	activeCodes := s.commercial.listPromotionCodes()
	for _, code := range normalizedCodes {
		key := strings.ToLower(code)
		if applied := appliedByCode[key]; applied != nil {
			result.Codes = append(result.Codes, *applied)
			continue
		}
		status := POSPromotionCodeValidation{
			Code:    code,
			Status:  "invalid",
			Message: "Code is invalid.",
		}
		record, found := s.findModelByField("promotion_code", "code", code)
		if !found {
			status.Message = "Code was not found."
			result.Valid = false
			result.Codes = append(result.Codes, status)
			continue
		}
		recordStatus := strings.ToLower(strings.TrimSpace(textValue(record.Values["status"])))
		if recordStatus != "" && recordStatus != "active" {
			status.Message = "Code is inactive."
			result.Valid = false
			result.Codes = append(result.Codes, status)
			continue
		}
		promoCode, active := activeCodes[key]
		if !active {
			status.Message = "Code is inactive."
			result.Valid = false
			result.Codes = append(result.Codes, status)
			continue
		}
		status.PromotionCampaignCode = promoCode.promotionCampaignCode
		campaign, ok := campaigns[strings.ToLower(strings.TrimSpace(promoCode.promotionCampaignCode))]
		if !ok {
			status.Message = "Campaign is inactive or unavailable."
			result.Valid = false
			result.Codes = append(result.Codes, status)
			continue
		}
		if !withinDateWindow(evalTime, campaign.startAt, campaign.endAt) || !withinDateWindow(evalTime, promoCode.startAt, promoCode.endAt) {
			status.Message = "Code is outside its active date window."
			result.Valid = false
			result.Codes = append(result.Codes, status)
			continue
		}
		if len(campaign.salesChannels) > 0 {
			if _, ok := campaign.salesChannels["pos"]; !ok {
				status.Message = "Code is not valid for POS sales."
				result.Valid = false
				result.Codes = append(result.Codes, status)
				continue
			}
		}
		if len(campaign.storeCodes) > 0 {
			if _, ok := campaign.storeCodes[strings.ToLower(strings.TrimSpace(storeCode))]; !ok {
				status.Message = "Code is not valid for this store."
				result.Valid = false
				result.Codes = append(result.Codes, status)
				continue
			}
		}
		if !promotionCodeMatchesParty(promoCode, partyValues, evalTime) {
			status.Message = "Code is not eligible for the attached customer."
			result.Valid = false
			result.Codes = append(result.Codes, status)
			continue
		}
		if promoCode.totalRedemptionLimit > 0 && s.commercial.promotionCodeUsageCount(promoCode.code, "") >= promoCode.totalRedemptionLimit {
			status.Message = "Code redemption limit has been reached."
			result.Valid = false
			result.Codes = append(result.Codes, status)
			continue
		}
		if strings.TrimSpace(partyID) != "" && promoCode.perCustomerRedemptionLimit > 0 && s.commercial.promotionCodeUsageCount(promoCode.code, partyID) >= promoCode.perCustomerRedemptionLimit {
			status.Message = "Customer redemption limit has been reached for this code."
			result.Valid = false
			result.Codes = append(result.Codes, status)
			continue
		}
		if campaign.globalUsageCap > 0 && s.commercial.promotionCampaignUsageCount(campaign.code, "") >= campaign.globalUsageCap {
			status.Message = "Campaign usage limit has been reached."
			result.Valid = false
			result.Codes = append(result.Codes, status)
			continue
		}
		if strings.TrimSpace(partyID) != "" && campaign.perCustomerUsageCap > 0 && s.commercial.promotionCampaignUsageCount(campaign.code, partyID) >= campaign.perCustomerUsageCap {
			status.Message = "Customer usage limit has been reached for this campaign."
			result.Valid = false
			result.Codes = append(result.Codes, status)
			continue
		}
		status.Status = "not_applicable"
		status.Message = "Code is valid, but it does not apply to the current basket."
		result.Valid = false
		result.Codes = append(result.Codes, status)
	}
	return result, nil
}

func posPromotionValidationMessage(result POSPromotionValidationResult) string {
	parts := make([]string, 0, len(result.Codes))
	for _, item := range result.Codes {
		if strings.EqualFold(item.Status, "applied") {
			continue
		}
		parts = append(parts, strings.TrimSpace(item.Code)+": "+strings.TrimSpace(item.Message))
	}
	if len(parts) == 0 {
		return "promotion codes are invalid"
	}
	return "promotion validation failed: " + strings.Join(parts, "; ")
}

func (s *POSCoreService) SearchCustomers(query string) ([]POSCustomerLookupItem, error) {
	needle := strings.ToLower(strings.TrimSpace(query))
	results := make([]POSCustomerLookupItem, 0, 20)
	for page := 1; ; page++ {
		items, total, err := s.models.List("customer_profile", model.Query{Page: page, PageSize: model.MaxPageSize})
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			name := firstNonEmptyString(textValue(item.Values["customer_name"]), textValue(item.Values["party_name"]), textValue(item.Values["party_id"]))
			partyID := textValue(item.Values["party_id"])
			if needle != "" {
				haystack := strings.ToLower(strings.Join([]string{
					name,
					partyID,
					textValue(item.Values["customer_type"]),
					textValue(item.Values["member_status"]),
					textValue(item.Values["member_tier"]),
				}, " "))
				if !strings.Contains(haystack, needle) {
					continue
				}
			}
			results = append(results, POSCustomerLookupItem{
				PartyID:       partyID,
				CustomerName:  name,
				CustomerType:  textValue(item.Values["customer_type"]),
				MemberStatus:  textValue(item.Values["member_status"]),
				MemberTier:    textValue(item.Values["member_tier"]),
				MemberValidTo: textValue(item.Values["member_valid_to"]),
			})
		}
		if page*model.MaxPageSize >= total || len(items) == 0 {
			break
		}
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].CustomerName == results[j].CustomerName {
			return results[i].PartyID < results[j].PartyID
		}
		return strings.ToLower(results[i].CustomerName) < strings.ToLower(results[j].CustomerName)
	})
	if len(results) > 20 {
		results = results[:20]
	}
	return results, nil
}

func (s *POSCoreService) LookupStoredValue(organizationID, locationID, kind, reference, partyID string) (POSStoredValueLookupResult, error) {
	if s.retail == nil {
		return POSStoredValueLookupResult{}, shared.Validation("stored value lookup is unavailable")
	}
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "gift_card":
		if strings.TrimSpace(reference) == "" {
			return POSStoredValueLookupResult{}, shared.Validation("gift card code is required")
		}
		record, err := s.retail.LookupGiftCard(organizationID, locationID, reference)
		if err != nil {
			return POSStoredValueLookupResult{}, err
		}
		return POSStoredValueLookupResult{
			Kind:             "gift_card",
			Code:             textValue(record.Values["code"]),
			PartyID:          textValue(record.Values["party_id"]),
			PartyName:        textValue(record.Values["party_name"]),
			CurrencyCode:     textValue(record.Values["currency_code"]),
			Status:           textValue(record.Values["status"]),
			RemainingBalance: numberValue(record.Values["remaining_balance"]),
		}, nil
	case "store_credit":
		lookupPartyID := strings.TrimSpace(partyID)
		if lookupPartyID == "" {
			lookupPartyID = strings.TrimSpace(reference)
		}
		if lookupPartyID == "" {
			return POSStoredValueLookupResult{}, shared.Validation("customer is required for store credit")
		}
		record, err := s.retail.LookupStoreCredit(organizationID, locationID, lookupPartyID)
		if err != nil {
			return POSStoredValueLookupResult{}, err
		}
		return POSStoredValueLookupResult{
			Kind:          "store_credit",
			PartyID:       textValue(record.Values["party_id"]),
			PartyName:     textValue(record.Values["party_name"]),
			CurrencyCode:  textValue(record.Values["currency_code"]),
			Status:        textValue(record.Values["status"]),
			BalanceAmount: numberValue(record.Values["balance_amount"]),
		}, nil
	default:
		return POSStoredValueLookupResult{}, shared.Validation("unsupported stored value kind")
	}
}

func (s *POSCoreService) OpenShift(organizationID, locationID, storeCode, registerCode, cashierUserID, actorID string, openingCash float64, notes string) (model.Record, error) {
	if cashierUserID == "" {
		return model.Record{}, shared.Validation("cashier user is required")
	}
	if _, ok := s.findModelByField("pos_store", "code", storeCode); !ok {
		return model.Record{}, shared.NotFound("pos store not found")
	}
	if _, ok := s.findModelByField("pos_register", "code", registerCode); !ok {
		return model.Record{}, shared.NotFound("pos register not found")
	}
	if _, ok := s.findOpenShift(registerCode, cashierUserID); ok {
		return model.Record{}, shared.Conflict("an open shift already exists for this register and cashier")
	}
	var cashierEmployeeID string
	if s.attendance != nil && s.attendance.workforce != nil {
		if employee, ok, err := s.attendance.workforce.ResolveEmployeeByUser(cashierUserID); err != nil {
			return model.Record{}, err
		} else if ok {
			cashierEmployeeID = employee.ID
		}
	}
	record, err := s.models.Create("pos_shift", actorID, map[string]any{
		"organization_id":      organizationID,
		"location_id":          locationID,
		"shift_number":         posNumber("SHIFT"),
		"store_code":           storeCode,
		"register_code":        registerCode,
		"cashier_user_id":      cashierUserID,
		"cashier_employee_id":  cashierEmployeeID,
		"roster_slot_id":       "",
		"attendance_day_id":    "",
		"opened_at":            time.Now().UTC().Format(time.RFC3339),
		"opening_cash_amount":  roundMoney(openingCash),
		"expected_cash_amount": roundMoney(openingCash),
		"actual_cash_amount":   0.0,
		"over_short_amount":    0.0,
		"status":               "opened",
		"notes":                notes,
	})
	if err != nil {
		return model.Record{}, err
	}
	if s.attendance != nil && cashierEmployeeID != "" {
		scope := AttendanceScope{OrganizationID: organizationID, LocationID: locationID, StoreCode: storeCode, RegisterCode: registerCode}
		event, day, previousDay, attendanceErr := s.attendance.recordAttendanceEventWithState(AttendanceEventInput{
			EmployeeID: cashierEmployeeID,
			EventType:  "clock_in",
			OccurredAt: time.Now().UTC(),
			Source:     "pos_shift",
			Notes:      notes,
			Scope:      scope,
		}, actorID)
		if attendanceErr != nil {
			if rollbackErr := s.deleteModelRecord("pos_shift", record.ID); rollbackErr != nil {
				return model.Record{}, fmt.Errorf("record attendance event: %w (rollback failed: %v)", attendanceErr, rollbackErr)
			}
			return model.Record{}, attendanceErr
		}
		values := cloneMap(record.Values)
		values["roster_slot_id"] = textValue(event.Values["roster_slot_id"])
		values["attendance_day_id"] = day.ID
		linked, updateErr := s.models.Update("pos_shift", record.ID, actorID, values, record.Version)
		if updateErr != nil {
			rollbackErr := s.attendance.rollbackAttendanceEventState(event.ID, previousDay, day)
			if deleteErr := s.deleteModelRecord("pos_shift", record.ID); rollbackErr == nil {
				rollbackErr = deleteErr
			}
			if rollbackErr != nil {
				return model.Record{}, fmt.Errorf("link shift attendance context: %w (rollback failed: %v)", updateErr, rollbackErr)
			}
			return model.Record{}, updateErr
		}
		record = linked
	}
	return record, nil
}

func (s *POSCoreService) CloseShift(organizationID, locationID, shiftID, actorID string, actualCash float64, notes string) (model.Record, error) {
	shift, err := s.models.Get("pos_shift", shiftID)
	if err != nil {
		return model.Record{}, err
	}
	if textValue(shift.Values["status"]) != "opened" {
		return model.Record{}, shared.Conflict("pos shift is not open")
	}
	if actorID != "" && textValue(shift.Values["cashier_user_id"]) != "" && textValue(shift.Values["cashier_user_id"]) != actorID {
		return model.Record{}, shared.Forbidden("shift belongs to a different cashier")
	}
	expectedCash := s.shiftExpectedCash(shift.ID, numberValue(shift.Values["opening_cash_amount"]))
	values := cloneMap(shift.Values)
	values["closed_at"] = time.Now().UTC().Format(time.RFC3339)
	values["expected_cash_amount"] = expectedCash
	values["actual_cash_amount"] = roundMoney(actualCash)
	values["over_short_amount"] = roundMoney(actualCash - expectedCash)
	values["status"] = "closed"
	values["notes"] = strings.TrimSpace(notes)
	var attendanceEvent model.Record
	var attendanceDay model.Record
	var previousAttendanceDay model.Record
	if s.attendance != nil {
		employeeID := textValue(values["cashier_employee_id"])
		if employeeID != "" {
			scope := AttendanceScope{OrganizationID: organizationID, LocationID: locationID, StoreCode: textValue(values["store_code"]), RegisterCode: textValue(values["register_code"])}
			event, day, previousDay, err := s.attendance.recordAttendanceEventWithState(AttendanceEventInput{
				EmployeeID:      employeeID,
				EventType:       "clock_out",
				OccurredAt:      time.Now().UTC(),
				Source:          "pos_shift",
				Notes:           notes,
				RosterSlotID:    textValue(values["roster_slot_id"]),
				AttendanceDayID: textValue(values["attendance_day_id"]),
				Scope:           scope,
			}, actorID)
			if err != nil {
				return model.Record{}, err
			}
			attendanceEvent = event
			attendanceDay = day
			previousAttendanceDay = previousDay
			if textValue(values["roster_slot_id"]) == "" {
				values["roster_slot_id"] = textValue(event.Values["roster_slot_id"])
			}
			if day.ID != "" {
				values["attendance_day_id"] = day.ID
			}
		}
	}
	record, err := s.models.Update("pos_shift", shift.ID, actorID, values, shift.Version)
	if err != nil {
		if s.attendance != nil && attendanceEvent.ID != "" {
			if rollbackErr := s.attendance.rollbackAttendanceEventState(attendanceEvent.ID, previousAttendanceDay, attendanceDay); rollbackErr != nil {
				return model.Record{}, fmt.Errorf("close shift: %w (attendance rollback failed: %v)", err, rollbackErr)
			}
		}
		return model.Record{}, err
	}
	if s.retail != nil {
		if _, syncErr := s.retail.SyncShiftReconciliation(organizationID, locationID, record.ID, actorID); syncErr != nil {
			return model.Record{}, syncErr
		}
	}
	return record, nil
}

func (s *POSCoreService) deleteModelRecord(modelKey, id string) error {
	repo := s.models.Repository()
	if repo == nil {
		return nil
	}
	return repo.DeleteRecord(modelKey, strings.TrimSpace(id))
}

func (s *POSCoreService) HoldSale(input POSHoldSaleInput, actorID string) (model.Record, error) {
	shift, register, store, err := s.validateSaleContext(input.StoreCode, input.RegisterCode, input.ShiftID, actorID)
	if err != nil {
		return model.Record{}, err
	}
	if validation, validateErr := s.ValidatePromotionCodes(textValue(shift.Values["organization_id"]), textValue(shift.Values["location_id"]), input.StoreCode, input.PartyID, input.PartyName, input.PromotionCodes, input.Lines); validateErr != nil {
		return model.Record{}, validateErr
	} else if !validation.Valid {
		return model.Record{}, shared.Validation(posPromotionValidationMessage(validation))
	}
	orderPayload, cartSummary, err := s.buildOrderPayload(store, input.PartyID, input.PartyName, input.Notes, input.PromotionCodes, input.Lines)
	if err != nil {
		return model.Record{}, err
	}
	values := map[string]any{
		"organization_id":      textValue(shift.Values["organization_id"]),
		"location_id":          textValue(shift.Values["location_id"]),
		"sale_number":          posNumber("SALE"),
		"store_code":           input.StoreCode,
		"register_code":        input.RegisterCode,
		"shift_id":             shift.ID,
		"cashier_user_id":      actorID,
		"party_id":             input.PartyID,
		"party_name":           firstNonEmptyString(input.PartyName, textValue(orderPayload["party_name"])),
		"checkout_mode":        s.effectiveCheckoutMode(store, register, input.CheckoutMode),
		"status":               "held",
		"reference":            input.Reference,
		"notes":                input.Notes,
		"currency_code":        firstNonEmptyString(textValue(orderPayload["currency_code"]), "IDR"),
		"subtotal_amount":      numberValue(orderPayload["subtotal_amount"]),
		"tax_amount":           numberValue(orderPayload["tax_amount"]),
		"total_amount":         numberValue(orderPayload["total_amount"]),
		"warehouse_code":       textValue(store.Values["warehouse_code"]),
		"price_list_code":      textValue(orderPayload["price_list_code"]),
		"tax_profile_code":     textValue(orderPayload["tax_profile_code"]),
		"promotion_codes_json": marshalJSONString(anyStringSlice(orderPayload["promotion_codes"])),
		"lines_json":           marshalJSONString(cartSummary),
		"tenders_json":         marshalJSONString(input.Tenders),
		"source_document_type": "",
		"source_document_id":   "",
		"invoice_id":           "",
		"fulfillment_id":       "",
		"payment_ids_json":     "[]",
		"device_id":            input.DeviceID,
		"offline_cached":       input.OfflineCached,
	}
	if strings.TrimSpace(input.SaleID) != "" {
		current, getErr := s.models.Get("pos_sale", input.SaleID)
		if getErr != nil {
			return model.Record{}, getErr
		}
		values["sale_number"] = firstNonEmptyString(textValue(current.Values["sale_number"]), textValue(values["sale_number"]))
		record, updateErr := s.models.Update("pos_sale", current.ID, actorID, values, current.Version)
		if updateErr != nil {
			return model.Record{}, updateErr
		}
		return record, nil
	}
	record, err := s.models.Create("pos_sale", actorID, values)
	if err != nil {
		return model.Record{}, err
	}
	return record, nil
}

func (s *POSCoreService) HeldSales(cashierUserID, registerCode, shiftID string) ([]model.Record, error) {
	items, _, err := s.models.List("pos_sale", model.Query{Filters: map[string]string{"status": "held"}, Page: 1, PageSize: 100})
	if err != nil {
		return nil, err
	}
	filtered := make([]model.Record, 0, len(items))
	for _, item := range items {
		if cashierUserID != "" && textValue(item.Values["cashier_user_id"]) != cashierUserID {
			continue
		}
		if registerCode != "" && textValue(item.Values["register_code"]) != registerCode {
			continue
		}
		if shiftID != "" && textValue(item.Values["shift_id"]) != shiftID {
			continue
		}
		filtered = append(filtered, item)
	}
	sort.Slice(filtered, func(i, j int) bool {
		return textValue(filtered[i].Values["sale_number"]) < textValue(filtered[j].Values["sale_number"])
	})
	return filtered, nil
}

func (s *POSCoreService) Checkout(organizationID, locationID string, input POSCheckoutInput, actorID string) (POSCheckoutResult, error) {
	shift, register, store, err := s.validateSaleContext(input.StoreCode, input.RegisterCode, input.ShiftID, actorID)
	if err != nil {
		return POSCheckoutResult{}, err
	}
	if len(input.Lines) == 0 {
		return POSCheckoutResult{}, shared.Validation("pos sale requires at least one line")
	}
	if len(input.Tenders) == 0 {
		return POSCheckoutResult{}, shared.Validation("pos sale requires at least one tender")
	}
	if validation, validateErr := s.ValidatePromotionCodes(organizationID, locationID, input.StoreCode, input.PartyID, input.PartyName, input.PromotionCodes, input.Lines); validateErr != nil {
		return POSCheckoutResult{}, validateErr
	} else if !validation.Valid {
		return POSCheckoutResult{}, shared.Validation(posPromotionValidationMessage(validation))
	}
	orderPayload, cartSummary, err := s.buildOrderPayload(store, input.PartyID, input.PartyName, input.Notes, input.PromotionCodes, input.Lines)
	if err != nil {
		return POSCheckoutResult{}, err
	}
	normalizedTenders, totalTendered, err := s.normalizeTenders(input.Tenders)
	if err != nil {
		return POSCheckoutResult{}, err
	}
	if s.retail != nil {
		normalizedTenders, err = s.retail.ResolveStoredValueTenders(organizationID, locationID, input.PartyID, normalizedTenders)
		if err != nil {
			return POSCheckoutResult{}, err
		}
	}
	totalAmount := roundMoney(numberValue(orderPayload["total_amount"]))
	if totalTendered < totalAmount {
		return POSCheckoutResult{}, shared.Validation("tendered amount is less than total")
	}
	changeDue := roundMoney(totalTendered - totalAmount)
	checkoutMode := s.effectiveCheckoutMode(store, register, input.CheckoutMode)

	order, err := s.documents.Create("sales_order", organizationID, locationID, actorID, orderPayload)
	if err != nil {
		return POSCheckoutResult{}, err
	}
	order, err = s.confirmDocument(order.Header.ID, actorID)
	if err != nil {
		return POSCheckoutResult{}, err
	}
	invoice, err := s.commercial.GenerateInvoiceFromOrder(order.Header.ID, actorID)
	if err != nil {
		return POSCheckoutResult{}, err
	}
	invoice, err = s.submitAndApprove(invoice.Header.ID, actorID)
	if err != nil {
		return POSCheckoutResult{}, err
	}

	var fulfillment *document.Record
	if s.hasInventorySaleLines(order.Body.Payload["lines"]) {
		record, fulfillmentErr := s.fulfillment.GenerateFulfillmentFromOrder(order.Header.ID, actorID)
		if fulfillmentErr != nil {
			return POSCheckoutResult{}, fulfillmentErr
		}
		approved, approveErr := s.submitAndApprove(record.Header.ID, actorID)
		if approveErr != nil {
			return POSCheckoutResult{}, approveErr
		}
		fulfillment = &approved
	}

	payments := make([]document.Record, 0, len(normalizedTenders))
	remaining := totalAmount
	for _, tender := range normalizedTenders {
		applied := roundMoney(minFloat(remaining, tender.Amount))
		remaining = roundMoney(remaining - applied)
		paymentPayload := map[string]any{
			"party_id":                textValue(orderPayload["party_id"]),
			"party_name":              textValue(orderPayload["party_name"]),
			"receipt_date":            time.Now().UTC().Format("2006-01-02"),
			"payment_method_code":     tender.PaymentMethodCode,
			"payment_reference":       firstNonEmptyString(tender.Reference, input.Reference, textValue(invoice.Body.Payload["payment_reference"])),
			"currency_code":           firstNonEmptyString(textValue(orderPayload["currency_code"]), "IDR"),
			"amount_received":         roundMoney(tender.Amount),
			"unapplied_amount":        roundMoney(maxFloat(tender.Amount-applied, 0)),
			"receivable_account_code": textValue(invoice.Body.Payload["receivable_account_code"]),
			"clearing_account_code":   tender.ClearingAccountCode,
			"notes":                   tender.Notes,
			"allocations": []map[string]any{{
				"invoice_id":     invoice.Header.ID,
				"invoice_number": firstNonEmptyString(invoice.Header.Number, invoice.Header.ID),
				"amount":         applied,
				"note":           fmt.Sprintf("POS %s", firstNonEmptyString(textValue(orderPayload["reference"]), order.Header.Number)),
			}},
		}
		payment, createErr := s.documents.Create("payment_receipt", organizationID, locationID, actorID, paymentPayload)
		if createErr != nil {
			return POSCheckoutResult{}, createErr
		}
		payment, err = s.submitAndApprove(payment.Header.ID, actorID)
		if err != nil {
			return POSCheckoutResult{}, err
		}
		payments = append(payments, payment)
	}

	saleValues := map[string]any{
		"organization_id":      organizationID,
		"location_id":          locationID,
		"sale_number":          posNumber("SALE"),
		"store_code":           input.StoreCode,
		"register_code":        input.RegisterCode,
		"shift_id":             shift.ID,
		"cashier_user_id":      actorID,
		"party_id":             input.PartyID,
		"party_name":           firstNonEmptyString(input.PartyName, textValue(orderPayload["party_name"])),
		"checkout_mode":        checkoutMode,
		"status":               "completed",
		"reference":            input.Reference,
		"notes":                input.Notes,
		"currency_code":        firstNonEmptyString(textValue(orderPayload["currency_code"]), "IDR"),
		"subtotal_amount":      numberValue(orderPayload["subtotal_amount"]),
		"tax_amount":           numberValue(orderPayload["tax_amount"]),
		"total_amount":         totalAmount,
		"tendered_amount":      totalTendered,
		"change_due_amount":    changeDue,
		"warehouse_code":       textValue(store.Values["warehouse_code"]),
		"price_list_code":      textValue(orderPayload["price_list_code"]),
		"tax_profile_code":     textValue(orderPayload["tax_profile_code"]),
		"promotion_codes_json": marshalJSONString(anyStringSlice(orderPayload["promotion_codes"])),
		"lines_json":           marshalJSONString(cartSummary),
		"tenders_json":         marshalJSONString(normalizedTenders),
		"source_document_type": firstNonEmptyString(mapCheckoutPrimaryType(checkoutMode), "invoice"),
		"source_document_id":   primaryDocumentID(checkoutMode, order.Header.ID, invoice.Header.ID),
		"order_id":             order.Header.ID,
		"order_number":         firstNonEmptyString(order.Header.Number, order.Header.ID),
		"invoice_id":           invoice.Header.ID,
		"invoice_number":       firstNonEmptyString(invoice.Header.Number, invoice.Header.ID),
		"fulfillment_id":       documentIDOrEmpty(fulfillment),
		"fulfillment_number":   documentNumberOrEmpty(fulfillment),
		"payment_ids_json":     marshalJSONString(documentIDs(payments)),
		"device_id":            input.DeviceID,
		"offline_cached":       input.OfflineCached,
	}
	var sale model.Record
	if strings.TrimSpace(input.SaleID) != "" {
		current, getErr := s.models.Get("pos_sale", input.SaleID)
		if getErr != nil {
			return POSCheckoutResult{}, getErr
		}
		if textValue(current.Values["status"]) != "held" {
			return POSCheckoutResult{}, shared.Validation("pos sale is not in held status")
		}
		saleValues["sale_number"] = firstNonEmptyString(textValue(current.Values["sale_number"]), textValue(saleValues["sale_number"]))
		sale, err = s.models.Update("pos_sale", current.ID, actorID, saleValues, current.Version)
		if err != nil {
			return POSCheckoutResult{}, err
		}
	} else {
		sale, err = s.models.Create("pos_sale", actorID, saleValues)
		if err != nil {
			return POSCheckoutResult{}, err
		}
	}
	if s.retail != nil {
		if err := s.retail.RecordStoredValueRedemptions(organizationID, locationID, sale, payments, normalizedTenders, actorID, input.PartyID); err != nil {
			return POSCheckoutResult{}, err
		}
	}
	result := POSCheckoutResult{
		Sale:         sale,
		Order:        &order,
		Invoice:      &invoice,
		Fulfillment:  fulfillment,
		Payments:     payments,
		PrimaryDocID: primaryDocumentID(checkoutMode, order.Header.ID, invoice.Header.ID),
		ReceiptTitle: firstNonEmptyString(invoice.Header.Number, order.Header.Number, textValue(sale.Values["sale_number"])),
	}
	return result, nil
}

func (s *POSCoreService) TransactionLookup(query, cashierUserID, storeCode, registerCode string) ([]model.Record, error) {
	items, _, err := s.models.List("pos_sale", model.Query{Page: 1, PageSize: 100})
	if err != nil {
		return nil, err
	}
	trimmed := strings.ToLower(strings.TrimSpace(query))
	filtered := make([]model.Record, 0, len(items))
	for _, item := range items {
		if textValue(item.Values["status"]) != "completed" {
			continue
		}
		if cashierUserID != "" && textValue(item.Values["cashier_user_id"]) != cashierUserID {
			continue
		}
		if storeCode != "" && textValue(item.Values["store_code"]) != storeCode {
			continue
		}
		if registerCode != "" && textValue(item.Values["register_code"]) != registerCode {
			continue
		}
		if trimmed != "" {
			haystack := strings.ToLower(strings.Join([]string{
				textValue(item.Values["sale_number"]),
				textValue(item.Values["party_name"]),
				textValue(item.Values["order_number"]),
				textValue(item.Values["invoice_number"]),
			}, " "))
			if !strings.Contains(haystack, trimmed) {
				continue
			}
		}
		filtered = append(filtered, item)
	}
	sort.Slice(filtered, func(i, j int) bool {
		return textValue(filtered[i].Values["sale_number"]) > textValue(filtered[j].Values["sale_number"])
	})
	return filtered, nil
}

func (s *POSCoreService) RefundSale(saleID, actorID string) (map[string]any, error) {
	if s.returns == nil {
		return nil, shared.Validation("returns service is unavailable")
	}
	sale, err := s.models.Get("pos_sale", saleID)
	if err != nil {
		return nil, err
	}
	fulfillmentID := textValue(sale.Values["fulfillment_id"])
	if fulfillmentID == "" {
		return nil, shared.Validation("pos sale has no fulfillment to return")
	}
	salesReturn, err := s.returns.GenerateReturnFromFulfillment(fulfillmentID, actorID)
	if err != nil {
		return nil, err
	}
	salesReturn, err = s.updateReturnResolution(salesReturn, "refund", actorID)
	if err != nil {
		return nil, err
	}
	salesReturn, err = s.submitAndApprove(salesReturn.Header.ID, actorID)
	if err != nil {
		return nil, err
	}
	returnReceipt, err := s.returns.CreateReturnReceiptFromReturn(salesReturn.Header.ID, actorID)
	if err != nil {
		return nil, err
	}
	returnReceipt, err = s.submitAndApprove(returnReceipt.Header.ID, actorID)
	if err != nil {
		return nil, err
	}
	creditNote, err := s.returns.CreateCreditNoteFromReturn(salesReturn.Header.ID, actorID)
	if err != nil {
		return nil, err
	}
	creditNote, err = s.submitAndApprove(creditNote.Header.ID, actorID)
	if err != nil {
		return nil, err
	}
	refund, err := s.returns.CreateRefundFromReturn(salesReturn.Header.ID, actorID)
	if err != nil {
		return nil, err
	}
	refund, err = s.submitAndApprove(refund.Header.ID, actorID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"sales_return":   salesReturn,
		"return_receipt": returnReceipt,
		"credit_note":    creditNote,
		"payment_refund": refund,
	}, nil
}

func (s *POSCoreService) RefundSaleToStoreCredit(organizationID, locationID, saleID, actorID string) (map[string]any, error) {
	if s.returns == nil || s.retail == nil {
		return nil, shared.Validation("store credit refund is unavailable")
	}
	sale, err := s.models.Get("pos_sale", saleID)
	if err != nil {
		return nil, err
	}
	fulfillmentID := textValue(sale.Values["fulfillment_id"])
	if fulfillmentID == "" {
		return nil, shared.Validation("pos sale has no fulfillment to return")
	}
	salesReturn, err := s.returns.GenerateReturnFromFulfillment(fulfillmentID, actorID)
	if err != nil {
		return nil, err
	}
	salesReturn, err = s.updateReturnResolution(salesReturn, "refund", actorID)
	if err != nil {
		return nil, err
	}
	salesReturn, err = s.submitAndApprove(salesReturn.Header.ID, actorID)
	if err != nil {
		return nil, err
	}
	returnReceipt, err := s.returns.CreateReturnReceiptFromReturn(salesReturn.Header.ID, actorID)
	if err != nil {
		return nil, err
	}
	returnReceipt, err = s.submitAndApprove(returnReceipt.Header.ID, actorID)
	if err != nil {
		return nil, err
	}
	creditNote, err := s.returns.CreateCreditNoteFromReturn(salesReturn.Header.ID, actorID)
	if err != nil {
		return nil, err
	}
	creditNote, err = s.submitAndApprove(creditNote.Header.ID, actorID)
	if err != nil {
		return nil, err
	}
	storeCredit, err := s.retail.CreateStoreCreditFromPOSRefund(organizationID, locationID, sale, creditNote, actorID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"sales_return":         salesReturn,
		"return_receipt":       returnReceipt,
		"credit_note":          creditNote,
		"store_credit_account": storeCredit["store_credit_account"],
		"store_credit_txn":     storeCredit["store_credit_txn"],
		"posting":              storeCredit["posting"],
	}, nil
}

func (s *POSCoreService) ExchangeSale(saleID, actorID string) (map[string]any, error) {
	if s.returns == nil {
		return nil, shared.Validation("returns service is unavailable")
	}
	sale, err := s.models.Get("pos_sale", saleID)
	if err != nil {
		return nil, err
	}
	fulfillmentID := textValue(sale.Values["fulfillment_id"])
	if fulfillmentID == "" {
		return nil, shared.Validation("pos sale has no fulfillment to exchange")
	}
	salesReturn, err := s.returns.GenerateReturnFromFulfillment(fulfillmentID, actorID)
	if err != nil {
		return nil, err
	}
	salesReturn, err = s.updateReturnResolution(salesReturn, "exchange", actorID)
	if err != nil {
		return nil, err
	}
	salesReturn, err = s.submitAndApprove(salesReturn.Header.ID, actorID)
	if err != nil {
		return nil, err
	}
	returnReceipt, err := s.returns.CreateReturnReceiptFromReturn(salesReturn.Header.ID, actorID)
	if err != nil {
		return nil, err
	}
	returnReceipt, err = s.submitAndApprove(returnReceipt.Header.ID, actorID)
	if err != nil {
		return nil, err
	}
	replacementOrder, err := s.returns.CreateReplacementOrderFromReturn(salesReturn.Header.ID, actorID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"sales_return":      salesReturn,
		"return_receipt":    returnReceipt,
		"replacement_order": replacementOrder,
	}, nil
}

type normalizedTender struct {
	TenderTypeCode      string  `json:"tender_type_code"`
	PaymentMethodCode   string  `json:"payment_method_code"`
	Name                string  `json:"name"`
	Kind                string  `json:"kind"`
	Amount              float64 `json:"amount"`
	Reference           string  `json:"reference,omitempty"`
	Notes               string  `json:"notes,omitempty"`
	ClearingAccountCode string  `json:"clearing_account_code,omitempty"`
	IsCashLike          bool    `json:"is_cash_like,omitempty"`
}

func (s *POSCoreService) normalizeTenders(inputs []POSTenderInput) ([]normalizedTender, float64, error) {
	items := make([]normalizedTender, 0, len(inputs))
	total := 0.0
	for _, input := range inputs {
		if roundMoney(input.Amount) <= 0 {
			continue
		}
		record, ok := s.findModelByField("pos_tender_type", "code", input.TenderTypeCode)
		if !ok {
			return nil, 0, shared.Validation("unknown tender type: " + input.TenderTypeCode)
		}
		items = append(items, normalizedTender{
			TenderTypeCode:      input.TenderTypeCode,
			PaymentMethodCode:   firstNonEmptyString(textValue(record.Values["payment_method_code"]), input.TenderTypeCode),
			Name:                firstNonEmptyString(textValue(record.Values["name"]), input.TenderTypeCode),
			Kind:                textValue(record.Values["kind"]),
			Amount:              roundMoney(input.Amount),
			Reference:           input.Reference,
			Notes:               input.Notes,
			ClearingAccountCode: textValue(record.Values["clearing_account_code"]),
			IsCashLike:          boolFieldValue(record.Values["is_cash_like"]) || strings.EqualFold(textValue(record.Values["kind"]), "cash"),
		})
		total += roundMoney(input.Amount)
	}
	if len(items) == 0 {
		return nil, 0, shared.Validation("no valid tenders supplied")
	}
	return items, roundMoney(total), nil
}

func (s *POSCoreService) buildOrderPayload(store model.Record, partyID, partyName, notes string, promotionCodes []string, lines []POSCartLineInput) (map[string]any, []map[string]any, error) {
	if err := validateCustomerPartyID(s.models, partyID); err != nil {
		return nil, nil, err
	}
	orderLines := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if roundMoney(line.Quantity) <= 0 {
			continue
		}
		if err := validateCommercialItemCode(s.models, line.ItemCode); err != nil {
			return nil, nil, err
		}
		orderLines = append(orderLines, map[string]any{
			"product_code":      line.ProductCode,
			"variant_signature": line.VariantSignature,
			"item_code":         line.ItemCode,
			"description":       line.Description,
			"quantity":          roundMoney(line.Quantity),
			"discount_amount":   roundMoney(line.DiscountAmount),
			"note":              line.Note,
		})
	}
	if len(orderLines) == 0 {
		return nil, nil, shared.Validation("pos sale requires at least one valid line")
	}
	now := s.businessTime()
	payload := map[string]any{
		"party_id":         strings.TrimSpace(partyID),
		"party_name":       strings.TrimSpace(partyName),
		"order_date":       now.Format("2006-01-02"),
		"order_datetime":   now.Format(time.RFC3339),
		"price_list_code":  textValue(store.Values["price_list_code"]),
		"tax_profile_code": textValue(store.Values["tax_profile_code"]),
		"default_tax_code": textValue(store.Values["default_tax_code"]),
		"currency_code":    firstNonEmptyString(textValue(store.Values["currency_code"]), "IDR"),
		"reference":        firstNonEmptyString(textValue(store.Values["name"]), textValue(store.Values["code"])),
		"sales_channel":    "pos",
		"store_code":       textValue(store.Values["code"]),
		"promotion_codes":  promotionCodes,
		"notes":            notes,
		"lines":            orderLines,
	}
	normalized := s.commercial.NormalizePayload("sales_order", payload)
	return normalized, recordList(normalized["lines"]), nil
}

func (s *POSCoreService) businessTime() time.Time {
	now := time.Now()
	if s.commercial == nil {
		return now
	}
	policy := s.commercial.discountPolicy()
	if policy.TimeZone == "" {
		return now
	}
	location, err := time.LoadLocation(policy.TimeZone)
	if err != nil {
		return now
	}
	return now.In(location)
}

func (s *POSCoreService) validateSaleContext(storeCode, registerCode, shiftID, actorID string) (model.Record, model.Record, model.Record, error) {
	if storeCode == "" || registerCode == "" || shiftID == "" {
		return model.Record{}, model.Record{}, model.Record{}, shared.Validation("store, register, and shift are required")
	}
	store, ok := s.findModelByField("pos_store", "code", storeCode)
	if !ok {
		return model.Record{}, model.Record{}, model.Record{}, shared.NotFound("pos store not found")
	}
	register, ok := s.findModelByField("pos_register", "code", registerCode)
	if !ok {
		return model.Record{}, model.Record{}, model.Record{}, shared.NotFound("pos register not found")
	}
	if textValue(register.Values["store_code"]) != storeCode {
		return model.Record{}, model.Record{}, model.Record{}, shared.Validation("register does not belong to the selected store")
	}
	shift, err := s.models.Get("pos_shift", shiftID)
	if err != nil {
		return model.Record{}, model.Record{}, model.Record{}, err
	}
	if textValue(shift.Values["status"]) != "opened" {
		return model.Record{}, model.Record{}, model.Record{}, shared.Conflict("pos shift is not open")
	}
	if textValue(shift.Values["store_code"]) != storeCode {
		return model.Record{}, model.Record{}, model.Record{}, shared.Validation("shift does not belong to the selected store")
	}
	if textValue(shift.Values["register_code"]) != registerCode {
		return model.Record{}, model.Record{}, model.Record{}, shared.Validation("shift does not belong to the selected register")
	}
	if actorID != "" && textValue(shift.Values["cashier_user_id"]) != "" && textValue(shift.Values["cashier_user_id"]) != actorID {
		return model.Record{}, model.Record{}, model.Record{}, shared.Forbidden("shift belongs to a different cashier")
	}
	return shift, register, store, nil
}

func (s *POSCoreService) submitAndApprove(documentID, actorID string) (document.Record, error) {
	if s.actions == nil {
		return document.Record{}, shared.Validation("document actions are unavailable")
	}
	acting := ActingContext{ActorID: actorID, EffectiveUserID: actorID}
	record, err := s.documents.Get(documentID)
	if err != nil {
		return document.Record{}, err
	}
	record, err = s.actions.Submit(documentID, acting, record.Header.Version, record.Header.ETag)
	if err != nil {
		return document.Record{}, err
	}
	record, err = s.actions.Approve(documentID, acting, record.Header.Version, record.Header.ETag)
	if err != nil {
		return document.Record{}, err
	}
	switch record.Header.Type {
	case "invoice", "credit_note", "payment_receipt", "payment_refund":
		if s.commercial != nil {
			if err := s.commercial.HandleApprovedDocument(record, actorID); err != nil {
				return document.Record{}, err
			}
		}
	case "sales_fulfillment":
		if s.fulfillment != nil {
			if err := s.fulfillment.HandleApprovedDocument(record, actorID); err != nil {
				return document.Record{}, err
			}
		}
	case "sales_return", "return_receipt":
		if s.returns != nil {
			if err := s.returns.HandleApprovedDocument(record, actorID); err != nil {
				return document.Record{}, err
			}
		}
	}
	updated, err := s.documents.Get(documentID)
	if err == nil {
		record = updated
	}
	if normalized, normalizeErr := s.normalizeApprovedDocumentStatus(record, actorID); normalizeErr == nil {
		record = normalized
	} else {
		return document.Record{}, normalizeErr
	}
	return record, nil
}

func (s *POSCoreService) confirmDocument(documentID, actorID string) (document.Record, error) {
	record, err := s.documents.Get(documentID)
	if err != nil {
		return document.Record{}, err
	}
	record.Header.Status = "confirmed"
	record.Header.Version++
	record.Header.ETag = fmt.Sprintf("%s:%d", record.Header.ID, record.Header.Version)
	record.Header.UpdatedBy = actorID
	record.Header.UpdatedAt = time.Now().UTC()
	if err := s.documents.Save(record); err != nil {
		return document.Record{}, err
	}
	return s.documents.Get(documentID)
}

func (s *POSCoreService) normalizeApprovedDocumentStatus(record document.Record, actorID string) (document.Record, error) {
	targetStatus := ""
	switch record.Header.Type {
	case "invoice", "credit_note":
		targetStatus = "issued"
	case "payment_receipt":
		targetStatus = "received"
	case "payment_refund":
		targetStatus = "refunded"
	case "sales_fulfillment":
		targetStatus = "issued"
	case "return_receipt":
		targetStatus = "received"
	}
	if targetStatus == "" || record.Header.Status == targetStatus {
		return record, nil
	}
	record.Header.Status = targetStatus
	record.Header.Version++
	record.Header.ETag = fmt.Sprintf("%s:%d", record.Header.ID, record.Header.Version)
	record.Header.UpdatedBy = actorID
	record.Header.UpdatedAt = time.Now().UTC()
	if err := s.documents.Save(record); err != nil {
		return document.Record{}, err
	}
	return s.documents.Get(record.Header.ID)
}

func (s *POSCoreService) updateReturnResolution(record document.Record, resolutionType, actorID string) (document.Record, error) {
	if record.Header.Type != "sales_return" {
		return record, nil
	}
	updated := record
	payload := cloneMap(updated.Body.Payload)
	payload["resolution_type"] = resolutionType
	updated.Body.Payload = document.NormalizePayload(payload)
	updated.Body.ContentHash = document.ContentHash(updated.Body.Payload)
	updated.Header.Version++
	updated.Header.ETag = fmt.Sprintf("%s:%d", updated.Header.ID, updated.Header.Version)
	updated.Header.UpdatedBy = actorID
	updated.Header.UpdatedAt = time.Now().UTC()
	if err := s.documents.Save(updated); err != nil {
		return document.Record{}, err
	}
	return updated, nil
}

func (s *POSCoreService) effectiveCheckoutMode(store, register model.Record, requested string) string {
	return firstNonEmptyString(strings.TrimSpace(requested), textValue(register.Values["checkout_mode"]), textValue(store.Values["checkout_mode"]), "invoice_first")
}

func (s *POSCoreService) hasInventorySaleLines(raw any) bool {
	for _, line := range recordList(raw) {
		itemCode := textValue(line["item_code"])
		if itemCode == "" {
			continue
		}
		if item, ok := s.findModelByField("commercial_item", "sku", itemCode); ok && boolFieldValue(item.Values["inventory_enabled"]) {
			return true
		}
	}
	return false
}

func (s *POSCoreService) listActiveModels(modelKey string) []model.Record {
	items, _, err := s.models.List(modelKey, model.Query{Page: 1, PageSize: 100})
	if err != nil {
		return nil
	}
	filtered := make([]model.Record, 0, len(items))
	for _, item := range items {
		status := strings.ToLower(strings.TrimSpace(textValue(item.Values["status"])))
		if status == "" || status == "active" || status == "opened" || status == "held" || status == "completed" {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func (s *POSCoreService) findModelByField(modelKey, fieldKey, value string) (model.Record, bool) {
	if strings.TrimSpace(value) == "" {
		return model.Record{}, false
	}
	items, _, err := s.models.List(modelKey, model.Query{
		Filters:  map[string]string{fieldKey: value},
		Page:     1,
		PageSize: 10,
	})
	if err != nil || len(items) == 0 {
		return model.Record{}, false
	}
	return items[0], true
}

func (s *POSCoreService) findOpenShift(registerCode, cashierUserID string) (model.Record, bool) {
	items, _, err := s.models.List("pos_shift", model.Query{
		Filters:  map[string]string{"register_code": registerCode, "cashier_user_id": cashierUserID, "status": "opened"},
		Page:     1,
		PageSize: 10,
	})
	if err != nil || len(items) == 0 {
		return model.Record{}, false
	}
	return items[0], true
}

func (s *POSCoreService) shiftExpectedCash(shiftID string, openingCash float64) float64 {
	items, _, err := s.models.List("pos_sale", model.Query{Page: 1, PageSize: 100})
	if err != nil {
		return roundMoney(openingCash)
	}
	total := roundMoney(openingCash)
	for _, item := range items {
		if textValue(item.Values["shift_id"]) != shiftID || textValue(item.Values["status"]) != "completed" {
			continue
		}
		var tenders []normalizedTender
		_ = json.Unmarshal([]byte(textValue(item.Values["tenders_json"])), &tenders)
		for _, tender := range tenders {
			if tender.IsCashLike {
				total = roundMoney(total + roundMoney(tender.Amount) - roundMoney(numberValue(item.Values["change_due_amount"])))
			}
		}
	}
	return total
}

func mapCheckoutPrimaryType(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "sales_order_first":
		return "sales_order"
	default:
		return "invoice"
	}
}

func primaryDocumentID(mode, orderID, invoiceID string) string {
	if mapCheckoutPrimaryType(mode) == "sales_order" {
		return orderID
	}
	return invoiceID
}

func documentIDOrEmpty(record *document.Record) string {
	if record == nil {
		return ""
	}
	return record.Header.ID
}

func documentNumberOrEmpty(record *document.Record) string {
	if record == nil {
		return ""
	}
	return firstNonEmptyString(record.Header.Number, record.Header.ID)
}

func documentIDs(records []document.Record) []string {
	ids := make([]string, 0, len(records))
	for _, record := range records {
		ids = append(ids, record.Header.ID)
	}
	return ids
}

func stockSummaryForWarehouse(summary map[string]any, warehouseCode, field string) float64 {
	total := 0.0
	for _, row := range recordList(summary["warehouse_batches"]) {
		if textValue(row["warehouse_code"]) != warehouseCode {
			continue
		}
		total = roundMoney(total + numberValue(row[field]))
	}
	return total
}

func matchesCatalogQuery(record model.Record, query string) bool {
	if strings.TrimSpace(query) == "" {
		return boolFieldValue(record.Values["is_sellable"])
	}
	haystack := strings.ToLower(strings.Join([]string{
		textValue(record.Values["sku"]),
		textValue(record.Values["name"]),
		textValue(record.Values["description"]),
		textValue(record.Values["variant_label"]),
		textValue(record.Values["product_code"]),
	}, " "))
	return boolFieldValue(record.Values["is_sellable"]) && strings.Contains(haystack, strings.ToLower(strings.TrimSpace(query)))
}

func posNumber(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, time.Now().UTC().Format("20060102150405"))
}

func marshalJSONString(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "[]"
	}
	return string(data)
}

func firstRecord(raw any) map[string]any {
	items := recordList(raw)
	if len(items) == 0 {
		return map[string]any{}
	}
	return items[0]
}
