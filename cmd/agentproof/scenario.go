package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"orbyte/internal/platform/analytics"
	application "orbyte/internal/platform/application"
	"orbyte/internal/platform/shared"
)

const (
	defaultOrgID = "org_default"
	defaultLocID = "loc_hq"
)

func seedEmployeeSpendScenario(ctx context.Context, client *apiClient, baseURL, opencodeCommand string) (scenarioManifest, error) {
	runID, suffix := newRunContext()
	normalizer := application.NewEmployeeSpendCoreService(nil, nil)
	manifest, err := newScenarioManifest(ctx, client, baseURL, opencodeCommand, "employee_spend", "workforce-expense", runID, suffix)
	if err != nil {
		return scenarioManifest{}, err
	}

	primaryName := "Rina Hartono " + runID
	primaryCode := "EMP-SPEND-" + suffix
	primaryPartyID := "party_emp_" + suffix
	primaryDeptID := "dept_spend_" + suffix
	primaryCostCenterID := "cc_spend_" + suffix
	primaryTreasuryID := "treasury_main_" + suffix

	employeeRecord, err := client.createModel(ctx, "employee_profile", map[string]any{
		"party_id":          primaryPartyID,
		"name":              primaryName,
		"user_id":           "user_admin",
		"employee_code":     primaryCode,
		"employment_status": "active",
		"status":            "active",
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create employee profile: %w", err)
	}
	employeeID := stringValue(employeeRecord["id"])
	if _, err := client.createModel(ctx, "employee_assignment", map[string]any{
		"employee_id":          employeeID,
		"organization_id":      defaultOrgID,
		"location_id":          defaultLocID,
		"organization_unit_id": "ou_spend_" + suffix,
		"department_id":        primaryDeptID,
		"cost_center_id":       primaryCostCenterID,
		"effective_from":       time.Now().UTC().Format("2006-01-02"),
		"status":               "active",
	}); err != nil {
		return scenarioManifest{}, fmt.Errorf("create employee assignment: %w", err)
	}

	categoryRecord, err := client.createModel(ctx, "expense_category", map[string]any{
		"code":                 "TRAVEL-" + suffix,
		"name":                 "Travel Expense " + runID,
		"expense_account_code": "6100-TRAVEL",
		"payable_account_code": "2100-EMP",
		"status":               "active",
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create expense category: %w", err)
	}
	categoryID := stringValue(categoryRecord["id"])
	categoryCode := stringValue(mapValue(categoryRecord, "values")["code"])

	policyRecord, err := client.createModel(ctx, "expense_policy", map[string]any{
		"code":                         "POL-" + suffix,
		"name":                         "Travel Policy " + runID,
		"organization_id":              defaultOrgID,
		"location_id":                  defaultLocID,
		"default_currency_code":        "IDR",
		"default_payment_method_code":  "BANK",
		"default_payable_account_code": "2100-EMP",
		"default_expense_account_code": "6100-TRAVEL",
		"default_treasury_account_id":  primaryTreasuryID,
		"status":                       "active",
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create expense policy: %w", err)
	}
	policyID := stringValue(policyRecord["id"])
	if _, err := client.createModel(ctx, "travel_policy", map[string]any{
		"code":                      "TRV-" + suffix,
		"name":                      "Domestic Travel " + runID,
		"organization_id":           defaultOrgID,
		"location_id":               defaultLocID,
		"default_expense_policy_id": policyID,
		"status":                    "active",
	}); err != nil {
		return scenarioManifest{}, fmt.Errorf("create travel policy: %w", err)
	}
	if _, err := client.createModel(ctx, "employee_spend_profile", map[string]any{
		"employee_id":                 employeeID,
		"expense_policy_id":           policyID,
		"default_currency_code":       "IDR",
		"default_payment_method_code": "BANK",
		"payable_account_code":        "2100-EMP",
		"expense_account_code":        "6100-TRAVEL",
		"treasury_account_id":         primaryTreasuryID,
		"status":                      "active",
	}); err != nil {
		return scenarioManifest{}, fmt.Errorf("create employee spend profile: %w", err)
	}

	travelPayload := normalizer.NormalizePayload("travel_request", map[string]any{
		"employee_id":         employeeID,
		"party_id":            primaryPartyID,
		"employee_code":       primaryCode,
		"organization_id":     defaultOrgID,
		"location_id":         defaultLocID,
		"department_id":       primaryDeptID,
		"cost_center_id":      primaryCostCenterID,
		"currency_code":       "IDR",
		"expense_category_id": categoryID,
		"request_date":        "2099-10-01",
		"travel_start_date":   "2099-10-10",
		"travel_end_date":     "2099-10-12",
		"destination":         "Jakarta",
		"purpose":             "Customer visit for enterprise renewal",
		"estimated_lines": []map[string]any{
			{"description": "Hotel", "amount": 120.0},
			{"description": "Meals", "amount": 30.0},
		},
	})
	travelDoc, err := createAndApproveDocument(ctx, client, "travel_request", travelPayload)
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("seed travel request: %w", err)
	}

	advanceDoc, err := createAndApproveDocument(ctx, client, "cash_advance", normalizer.NormalizePayload("cash_advance", map[string]any{
		"employee_id":          employeeID,
		"party_id":             primaryPartyID,
		"employee_code":        primaryCode,
		"organization_id":      defaultOrgID,
		"location_id":          defaultLocID,
		"department_id":        primaryDeptID,
		"cost_center_id":       primaryCostCenterID,
		"currency_code":        "IDR",
		"payment_method_code":  "BANK",
		"payable_account_code": "2100-EMP",
		"expense_account_code": "6100-TRAVEL",
		"treasury_account_id":  primaryTreasuryID,
		"travel_request_id":    travelDoc.ID,
		"requested_amount":     100.0,
		"notes":                "Travel cash advance for Jakarta visit",
	}))
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("seed cash advance: %w", err)
	}

	claimDoc, err := createAndApproveDocument(ctx, client, "expense_claim", normalizer.NormalizePayload("expense_claim", map[string]any{
		"employee_id":       employeeID,
		"party_id":          primaryPartyID,
		"employee_code":     primaryCode,
		"organization_id":   defaultOrgID,
		"location_id":       defaultLocID,
		"department_id":     primaryDeptID,
		"cost_center_id":    primaryCostCenterID,
		"currency_code":     "IDR",
		"travel_request_id": travelDoc.ID,
		"claim_date":        "2099-10-14",
		"claim_lines": []map[string]any{
			{"expense_category_code": categoryCode, "description": "Hotel", "amount": 150.0},
			{"expense_category_code": categoryCode, "description": "Meals", "amount": 20.0},
		},
	}))
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("seed expense claim: %w", err)
	}

	liquidationDoc, err := createAndApproveDocument(ctx, client, "advance_liquidation", normalizer.NormalizePayload("advance_liquidation", map[string]any{
		"employee_id":            employeeID,
		"party_id":               primaryPartyID,
		"employee_code":          primaryCode,
		"organization_id":        defaultOrgID,
		"location_id":            defaultLocID,
		"department_id":          primaryDeptID,
		"cost_center_id":         primaryCostCenterID,
		"currency_code":          "IDR",
		"travel_request_id":      travelDoc.ID,
		"cash_advance_id":        advanceDoc.ID,
		"expense_claim_id":       claimDoc.ID,
		"claim_total_amount":     170.0,
		"advance_amount":         30.0,
		"advance_applied_amount": 30.0,
		"net_settlement_amount":  140.0,
		"settlement_direction":   "company_owes_employee",
		"liquidation_date":       "2099-10-15",
	}))
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("seed advance liquidation: %w", err)
	}

	reimbursementDoc, err := createAndApproveDocument(ctx, client, "reimbursement_payment", normalizer.NormalizePayload("reimbursement_payment", map[string]any{
		"employee_id":           employeeID,
		"party_id":              primaryPartyID,
		"employee_code":         primaryCode,
		"organization_id":       defaultOrgID,
		"location_id":           defaultLocID,
		"department_id":         primaryDeptID,
		"cost_center_id":        primaryCostCenterID,
		"currency_code":         "IDR",
		"travel_request_id":     travelDoc.ID,
		"cash_advance_id":       advanceDoc.ID,
		"source_liquidation_id": liquidationDoc.ID,
		"net_settlement_amount": 140.0,
		"amount_paid":           140.0,
		"payment_date":          "2099-10-16",
		"notes":                 "Generated from advance liquidation " + liquidationDoc.ID,
	}))
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("seed reimbursement payment: %w", err)
	}

	noiseName := "Adi Santoso " + runID
	noiseCode := "EMP-NOISE-" + suffix
	noisePartyID := "party_noise_" + suffix
	noiseDeptID := "dept_noise_" + suffix
	noiseEmployeeRecord, err := client.createModel(ctx, "employee_profile", map[string]any{
		"party_id":          noisePartyID,
		"name":              noiseName,
		"user_id":           "user_admin",
		"employee_code":     noiseCode,
		"employment_status": "active",
		"status":            "active",
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create noise employee profile: %w", err)
	}
	noiseEmployeeID := stringValue(noiseEmployeeRecord["id"])
	noiseTravelDoc, err := createAndApproveDocument(ctx, client, "travel_request", normalizer.NormalizePayload("travel_request", map[string]any{
		"employee_id":         noiseEmployeeID,
		"party_id":            noisePartyID,
		"employee_code":       noiseCode,
		"organization_id":     defaultOrgID,
		"location_id":         defaultLocID,
		"department_id":       noiseDeptID,
		"cost_center_id":      "cc_noise_" + suffix,
		"currency_code":       "IDR",
		"expense_category_id": categoryID,
		"request_date":        "2099-10-20",
		"travel_start_date":   "2099-10-22",
		"travel_end_date":     "2099-10-23",
		"destination":         "Bandung",
		"purpose":             "Internal workshop",
		"estimated_lines":     []map[string]any{{"description": "Train", "amount": 55.0}},
	}))
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("seed noise travel request: %w", err)
	}

	manifest.Entities = map[string]map[string]any{
		"primary_employee": {"id": employeeID, "code": primaryCode, "party_id": primaryPartyID, "name": primaryName, "department_id": primaryDeptID},
		"noise_employee":   {"id": noiseEmployeeID, "code": noiseCode, "party_id": noisePartyID, "name": noiseName, "department_id": noiseDeptID},
	}
	manifest.Documents = map[string]documentFacts{
		"travel_request":        travelDoc,
		"cash_advance":          advanceDoc,
		"expense_claim":         claimDoc,
		"advance_liquidation":   liquidationDoc,
		"reimbursement_payment": reimbursementDoc,
		"noise_travel_request":  noiseTravelDoc,
	}
	manifest.GroundTruth = map[string]any{
		"approved_advance_amount":   100.0,
		"approved_claim_total":      170.0,
		"net_settlement_amount":     140.0,
		"settlement_direction":      "company_owes_employee",
		"reimbursement_status":      "paid",
		"reimbursement_amount_paid": 140.0,
		"pending_approval_count":    0,
		"open_workflow_task_count":  0,
	}
	manifest.PromptPack = employeeSpendPromptPack(primaryName)
	return manifest, nil
}

func seedOrderToCashScenario(ctx context.Context, client *apiClient, baseURL, opencodeCommand string) (scenarioManifest, error) {
	runID, suffix := newRunContext()
	manifest, err := newScenarioManifest(ctx, client, baseURL, opencodeCommand, "order_to_cash", "commercial-fulfillment", runID, suffix)
	if err != nil {
		return scenarioManifest{}, err
	}

	customer, err := client.createModel(ctx, "party", map[string]any{
		"name":          "Atlas Retail " + runID,
		"member_status": "active",
		"status":        "active",
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create customer: %w", err)
	}
	taxCode := "VAT11-" + suffix
	if _, err := client.createModel(ctx, "commercial_tax_code", map[string]any{
		"code":             taxCode,
		"name":             "VAT 11",
		"rate_percent":     11.0,
		"mode":             "exclusive",
		"tax_account_code": "2100-VATOUT",
		"status":           "active",
	}); err != nil {
		return scenarioManifest{}, fmt.Errorf("create tax code: %w", err)
	}
	itemCode := "OTC-ITEM-" + suffix
	if err := createSeedCommercialItem(ctx, client, map[string]any{
		"sku":                          itemCode,
		"name":                         "Field Router " + runID,
		"kind":                         "simple",
		"item_type":                    "product",
		"uom_code":                     "ea",
		"unit_price":                   100.0,
		"tax_code":                     taxCode,
		"revenue_account_code":         "4000-REV",
		"is_sellable":                  true,
		"inventory_enabled":            true,
		"inventory_tracking_mode":      "quantity",
		"inventory_asset_account_code": "1200-INV",
		"status":                       "active",
	}); err != nil {
		return scenarioManifest{}, fmt.Errorf("create item: %w", err)
	}
	if _, err := createAndApproveDocument(ctx, client, "stock_receipt", map[string]any{
		"receipt_date":   "2099-09-28",
		"warehouse_code": "MAIN",
		"lines": []map[string]any{{
			"item_code":      itemCode,
			"description":    "Opening stock",
			"warehouse_code": "MAIN",
			"uom_code":       "ea",
			"quantity":       12.0,
			"unit_cost":      60.0,
		}},
	}); err != nil {
		return scenarioManifest{}, fmt.Errorf("seed stock receipt: %w", err)
	}

	orderDoc, err := createAndApproveDocument(ctx, client, "sales_order", map[string]any{
		"party_id":          stringValue(customer["id"]),
		"party_name":        stringValue(mapValue(customer, "values")["name"]),
		"currency_code":     "IDR",
		"order_date":        "2099-10-01",
		"payment_term_days": 14.0,
		"subtotal_amount":   500.0,
		"tax_amount":        55.0,
		"total_amount":      555.0,
		"lines": []map[string]any{{
			"item_code":      itemCode,
			"description":    "Field Router",
			"warehouse_code": "MAIN",
			"uom_code":       "ea",
			"quantity":       5.0,
			"unit_price":     100.0,
			"tax_code":       taxCode,
			"tax_rate":       11.0,
			"line_subtotal":  500.0,
			"tax_amount":     55.0,
			"line_total":     555.0,
		}},
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create sales order: %w", err)
	}

	fulfillmentDoc, err := client.postDocument(ctx, "/commercial/orders/"+orderDoc.ID+"/generate-fulfillment", nil)
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("generate fulfillment: %w", err)
	}
	fulfillmentDoc, err = submitApproveDocument(ctx, client, fulfillmentDoc.ID)
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("approve fulfillment: %w", err)
	}

	deliveryDoc, err := client.postDocument(ctx, "/delivery/fulfillments/"+fulfillmentDoc.ID+"/register-delivery", nil)
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("generate delivery: %w", err)
	}
	deliveryDoc, err = submitApproveAndActionDocument(ctx, client, deliveryDoc.ID, "mark_delivered")
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("deliver order: %w", err)
	}

	invoiceDoc, err := client.postDocument(ctx, "/commercial/orders/"+orderDoc.ID+"/generate-invoice", nil)
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("generate invoice: %w", err)
	}
	invoiceDoc, err = submitApproveDocument(ctx, client, invoiceDoc.ID)
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("approve invoice: %w", err)
	}

	paymentDoc, err := client.postDocument(ctx, "/commercial/invoices/"+invoiceDoc.ID+"/register-payment", nil)
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("register payment: %w", err)
	}
	paymentDoc, err = submitApproveDocument(ctx, client, paymentDoc.ID)
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("approve payment: %w", err)
	}

	returnDoc, err := client.postDocument(ctx, "/returns/fulfillments/"+fulfillmentDoc.ID+"/register-return", nil)
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("generate return: %w", err)
	}
	returnDoc, err = submitApproveDocument(ctx, client, returnDoc.ID)
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("approve return: %w", err)
	}

	creditNoteDoc, err := client.postDocument(ctx, "/returns/returns/"+returnDoc.ID+"/issue-credit-note", nil)
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("issue credit note: %w", err)
	}
	creditNoteDoc, err = submitApproveDocument(ctx, client, creditNoteDoc.ID)
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("approve credit note: %w", err)
	}

	manifest.Entities = map[string]map[string]any{
		"customer": {"id": stringValue(customer["id"]), "name": stringValue(mapValue(customer, "values")["name"]), "code": "OTC-" + suffix},
		"item":     {"code": itemCode, "name": "Field Router " + runID},
	}
	manifest.Documents = map[string]documentFacts{
		"sales_order":       orderDoc,
		"sales_fulfillment": fulfillmentDoc,
		"delivery_order":    deliveryDoc,
		"invoice":           invoiceDoc,
		"payment_receipt":   paymentDoc,
		"sales_return":      returnDoc,
		"credit_note":       creditNoteDoc,
	}
	manifest.GroundTruth = map[string]any{
		"delivered_quantity": 5.0,
		"invoice_total":      555.0,
		"payment_status":     paymentDoc.Status,
		"return_exists":      true,
		"credit_note_status": creditNoteDoc.Status,
		"open_work_count":    0,
	}
	manifest.PromptPack = orderToCashPromptPack(stringValue(mapValue(customer, "values")["name"]), itemCode)
	return manifest, nil
}

func seedProcureToPayInventoryScenario(ctx context.Context, client *apiClient, baseURL, opencodeCommand string) (scenarioManifest, error) {
	runID, suffix := newRunContext()
	manifest, err := newScenarioManifest(ctx, client, baseURL, opencodeCommand, "procure_to_pay_inventory", "procurement-inventory", runID, suffix)
	if err != nil {
		return scenarioManifest{}, err
	}

	vendor, err := client.createModel(ctx, "party", map[string]any{
		"name":   "Nova Supplies " + runID,
		"status": "active",
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create vendor: %w", err)
	}
	itemCode := "P2P-ITEM-" + suffix
	if _, err := client.createModel(ctx, "commercial_item", map[string]any{
		"sku":                          itemCode,
		"name":                         "Medical Cuff " + runID,
		"uom_code":                     "ea",
		"inventory_enabled":            true,
		"inventory_tracking_mode":      "quantity",
		"inventory_asset_account_code": "1200-INV-MED",
		"status":                       "active",
	}); err != nil {
		return scenarioManifest{}, fmt.Errorf("create procurement item: %w", err)
	}

	poDoc, err := createAndApproveDocument(ctx, client, "purchase_order", map[string]any{
		"vendor_id":     stringValue(vendor["id"]),
		"vendor_name":   stringValue(mapValue(vendor, "values")["name"]),
		"currency_code": "IDR",
		"lines": []map[string]any{{
			"item_code":            itemCode,
			"description":          "Medical Cuff",
			"warehouse_code":       "MAIN",
			"uom_code":             "ea",
			"ordered_qty":          15.0,
			"quantity":             15.0,
			"unit_price":           100.0,
			"tax_code":             "VAT11",
			"tax_rate":             11.0,
			"tax_mode":             "exclusive",
			"tax_account_code":     "1510-VAT-IN",
			"expense_account_code": "5100-COGS",
		}},
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create purchase order: %w", err)
	}

	receiptDoc, err := client.postDocument(ctx, "/procurement/orders/"+poDoc.ID+"/register-receipt", nil)
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("register goods receipt: %w", err)
	}
	receiptDoc, err = submitApproveDocument(ctx, client, receiptDoc.ID)
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("approve goods receipt: %w", err)
	}

	billDoc, err := client.postDocument(ctx, "/procurement/receipts/"+receiptDoc.ID+"/register-vendor-bill", nil)
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("register vendor bill: %w", err)
	}
	billDoc, err = submitApproveDocument(ctx, client, billDoc.ID)
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("approve vendor bill: %w", err)
	}

	paymentDoc, err := client.postDocument(ctx, "/procurement/bills/"+billDoc.ID+"/register-payment", nil)
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("register payment out: %w", err)
	}
	paymentDoc, err = submitApproveDocument(ctx, client, paymentDoc.ID)
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("approve payment out: %w", err)
	}

	supplierReturnDoc, err := client.postDocument(ctx, "/procurement/bills/"+billDoc.ID+"/register-supplier-return", nil)
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("register supplier return: %w", err)
	}
	supplierReturnDoc, err = submitApproveDocument(ctx, client, supplierReturnDoc.ID)
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("approve supplier return: %w", err)
	}

	vendorCreditDoc, err := client.postDocument(ctx, "/supplier-returns/returns/"+supplierReturnDoc.ID+"/issue-vendor-credit", nil)
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("issue vendor credit: %w", err)
	}
	vendorCreditDoc, err = submitApproveDocument(ctx, client, vendorCreditDoc.ID)
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("approve vendor credit: %w", err)
	}

	manifest.Entities = map[string]map[string]any{
		"vendor": {"id": stringValue(vendor["id"]), "name": stringValue(mapValue(vendor, "values")["name"]), "code": "VEN-" + suffix},
		"item":   {"code": itemCode, "name": "Medical Cuff " + runID},
	}
	manifest.Documents = map[string]documentFacts{
		"purchase_order":  poDoc,
		"goods_receipt":   receiptDoc,
		"vendor_bill":     billDoc,
		"payment_out":     paymentDoc,
		"supplier_return": supplierReturnDoc,
		"vendor_credit":   vendorCreditDoc,
	}
	manifest.GroundTruth = map[string]any{
		"received_quantity":     15.0,
		"vendor_bill_total":     numberValue(billDoc.Payload["total_amount"]),
		"payment_status":        paymentDoc.Status,
		"supplier_return_state": supplierReturnDoc.Status,
		"vendor_credit_state":   vendorCreditDoc.Status,
	}
	manifest.PromptPack = procureToPayPromptPack(stringValue(mapValue(vendor, "values")["name"]), itemCode)
	return manifest, nil
}

func seedInventoryReplenishmentExecuteScenario(ctx context.Context, client *apiClient, baseURL, opencodeCommand string) (scenarioManifest, error) {
	runID, suffix := newRunContext()
	manifest, err := newScenarioManifest(ctx, client, baseURL, opencodeCommand, "inventory_replenishment_execute", "planning-procurement", runID, suffix)
	if err != nil {
		return scenarioManifest{}, err
	}

	warehouseCode := "WH-REPL-" + suffix
	coldBrewCode := "REPL-CB-" + suffix
	oatMilkCode := "REPL-OAT-" + suffix
	matchaCode := "REPL-MATCHA-" + suffix
	cupsCode := "REPL-CUPS-" + suffix
	vendorName := "North Roast Supply " + runID

	vendorParty, err := client.createModel(ctx, "party", map[string]any{
		"name":   vendorName,
		"status": "active",
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create vendor party: %w", err)
	}
	vendor, err := client.createModel(ctx, "vendor_profile", map[string]any{
		"party_id":    stringValue(vendorParty["id"]),
		"vendor_name": vendorName,
		"status":      "active",
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create vendor profile: %w", err)
	}
	customer, err := client.createModel(ctx, "party", map[string]any{
		"name":   "Cafe Horizon " + runID,
		"status": "active",
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create customer: %w", err)
	}
	if _, err := client.createModel(ctx, "warehouse", map[string]any{
		"code":   warehouseCode,
		"name":   "Replenishment Warehouse " + runID,
		"kind":   "storage",
		"status": "active",
	}); err != nil {
		return scenarioManifest{}, fmt.Errorf("create warehouse: %w", err)
	}

	for _, item := range []map[string]any{
		{
			"sku":                                  coldBrewCode,
			"name":                                 "Cold Brew Beans 1kg " + runID,
			"kind":                                 "item",
			"item_type":                            "product",
			"uom_code":                             "ea",
			"base_price":                           180000.0,
			"inventory_enabled":                    true,
			"inventory_tracking_mode":              "quantity",
			"inventory_asset_account_code":         "1200-INV",
			"revenue_account_code":                 "4000-REV",
			"replenishment_enabled":                true,
			"replenishment_mode":                   "reorder_point_target",
			"reorder_point_quantity":               10.0,
			"target_stock_quantity":                24.0,
			"default_replenishment_warehouse_code": warehouseCode,
			"status":                               "active",
		},
		{
			"sku":                                  oatMilkCode,
			"name":                                 "Oat Milk Barista 1L " + runID,
			"kind":                                 "item",
			"item_type":                            "product",
			"uom_code":                             "ea",
			"base_price":                           42000.0,
			"inventory_enabled":                    true,
			"inventory_tracking_mode":              "quantity",
			"inventory_asset_account_code":         "1200-INV",
			"revenue_account_code":                 "4000-REV",
			"replenishment_enabled":                true,
			"replenishment_mode":                   "reorder_point_target",
			"reorder_point_quantity":               12.0,
			"target_stock_quantity":                22.0,
			"default_replenishment_warehouse_code": warehouseCode,
			"status":                               "active",
		},
		{
			"sku":                                  matchaCode,
			"name":                                 "Matcha Powder 500g " + runID,
			"kind":                                 "item",
			"item_type":                            "product",
			"uom_code":                             "ea",
			"base_price":                           125000.0,
			"inventory_enabled":                    true,
			"inventory_tracking_mode":              "quantity",
			"inventory_asset_account_code":         "1200-INV",
			"revenue_account_code":                 "4000-REV",
			"replenishment_enabled":                true,
			"replenishment_mode":                   "reorder_point_target",
			"reorder_point_quantity":               8.0,
			"target_stock_quantity":                16.0,
			"default_replenishment_warehouse_code": warehouseCode,
			"status":                               "active",
		},
		{
			"sku":                                  cupsCode,
			"name":                                 "Paper Cups 16oz " + runID,
			"kind":                                 "item",
			"item_type":                            "product",
			"uom_code":                             "ea",
			"base_price":                           2500.0,
			"inventory_enabled":                    true,
			"inventory_tracking_mode":              "quantity",
			"inventory_asset_account_code":         "1200-INV",
			"revenue_account_code":                 "4000-REV",
			"replenishment_enabled":                true,
			"replenishment_mode":                   "reorder_point_target",
			"reorder_point_quantity":               15.0,
			"target_stock_quantity":                30.0,
			"default_replenishment_warehouse_code": warehouseCode,
			"status":                               "active",
		},
	} {
		if err := createSeedCommercialItem(ctx, client, item); err != nil {
			return scenarioManifest{}, fmt.Errorf("create item %s: %w", stringValue(item["sku"]), err)
		}
	}

	for _, vendorItem := range []map[string]any{
		{
			"vendor_id":              stringValue(vendor["id"]),
			"vendor_name":            vendorName,
			"item_code":              coldBrewCode,
			"preferred":              true,
			"priority":               1.0,
			"purchase_uom_code":      "ea",
			"lead_time_days":         5.0,
			"minimum_order_quantity": 10.0,
			"pack_size":              5.0,
			"last_purchase_price":    172000.0,
			"status":                 "active",
		},
		{
			"vendor_id":              stringValue(vendor["id"]),
			"vendor_name":            vendorName,
			"item_code":              oatMilkCode,
			"preferred":              true,
			"priority":               1.0,
			"purchase_uom_code":      "ea",
			"lead_time_days":         4.0,
			"minimum_order_quantity": 4.0,
			"pack_size":              4.0,
			"last_purchase_price":    39000.0,
			"status":                 "active",
		},
	} {
		if _, err := client.createModel(ctx, "vendor_item_profile", vendorItem); err != nil {
			return scenarioManifest{}, fmt.Errorf("create vendor item profile for %s: %w", stringValue(vendorItem["item_code"]), err)
		}
	}

	now := time.Now().UTC()
	for _, movement := range []map[string]any{
		{"item_code": coldBrewCode, "description": "Opening stock", "warehouse_code": warehouseCode, "uom_code": "ea", "quantity_delta": 46.0, "unit_cost": 172000.0, "total_cost": 7912000.0},
		{"item_code": oatMilkCode, "description": "Opening stock", "warehouse_code": warehouseCode, "uom_code": "ea", "quantity_delta": 36.0, "unit_cost": 39000.0, "total_cost": 1404000.0},
		{"item_code": matchaCode, "description": "Opening stock", "warehouse_code": warehouseCode, "uom_code": "ea", "quantity_delta": 60.0, "unit_cost": 120000.0, "total_cost": 7200000.0},
		{"item_code": cupsCode, "description": "Opening stock", "warehouse_code": warehouseCode, "uom_code": "ea", "quantity_delta": 100.0, "unit_cost": 1800.0, "total_cost": 180000.0},
	} {
		record, err := client.createDocument(ctx, map[string]any{
			"type":            "stock_movement",
			"organization_id": defaultOrgID,
			"location_id":     defaultLocID,
			"payload": map[string]any{
				"movement_date":      now.AddDate(0, -2, 0).Format("2006-01-02"),
				"movement_reason":    "opening_balance",
				"movement_direction": "in",
				"currency_code":      "IDR",
				"item_code":          movement["item_code"],
				"description":        movement["description"],
				"warehouse_code":     movement["warehouse_code"],
				"uom_code":           movement["uom_code"],
				"quantity_delta":     movement["quantity_delta"],
				"unit_cost":          movement["unit_cost"],
				"total_cost":         movement["total_cost"],
			},
		})
		if err != nil {
			return scenarioManifest{}, fmt.Errorf("seed opening stock movement for %s: %w", stringValue(movement["item_code"]), err)
		}
		current, _, _, err := client.getDocument(ctx, record.ID)
		if err != nil {
			return scenarioManifest{}, fmt.Errorf("load opening stock movement for %s: %w", stringValue(movement["item_code"]), err)
		}
		if current.Status != "posted" {
			_, version, etag, err := client.getDocument(ctx, record.ID)
			if err != nil {
				return scenarioManifest{}, fmt.Errorf("reload opening stock movement for %s: %w", stringValue(movement["item_code"]), err)
			}
			if _, _, _, err := client.actionDocument(ctx, record.ID, version, etag, "approve"); err != nil {
				reloaded, _, _, reloadErr := client.getDocument(ctx, record.ID)
				if reloadErr != nil || reloaded.Status != "posted" {
					return scenarioManifest{}, fmt.Errorf("post opening stock movement for %s: %w", stringValue(movement["item_code"]), err)
				}
			}
		}
	}

	for week := 1; week <= 6; week++ {
		fulfillmentDate := now.AddDate(0, 0, -(7 * week)).Format("2006-01-02")
		if _, err := createAndApproveDocument(ctx, client, "sales_fulfillment", map[string]any{
			"source_order_number": fmt.Sprintf("SO-HIST-%s-%d", suffix, week),
			"party_name":          "Cafe Horizon " + runID,
			"fulfillment_date":    fulfillmentDate,
			"lines": []map[string]any{
				{"item_code": coldBrewCode, "description": "Cold brew beans historical issue", "warehouse_code": warehouseCode, "uom_code": "ea", "quantity": 7.0},
				{"item_code": oatMilkCode, "description": "Oat milk historical issue", "warehouse_code": warehouseCode, "uom_code": "ea", "quantity": 5.0},
			},
		}); err != nil {
			return scenarioManifest{}, fmt.Errorf("seed historical fulfillment %d: %w", week, err)
		}
	}

	if _, err := createAndApproveDocument(ctx, client, "sales_order", map[string]any{
		"party_id":      stringValue(customer["id"]),
		"party_name":    stringValue(mapValue(customer, "values")["name"]),
		"currency_code": "IDR",
		"order_date":    now.Format("2006-01-02"),
		"total_amount":  846000.0,
		"lines": []map[string]any{
			{"item_code": coldBrewCode, "description": "Cold brew beans urgent demand", "warehouse_code": warehouseCode, "uom_code": "ea", "quantity": 4.0, "unit_price": 180000.0, "line_total": 720000.0},
			{"item_code": oatMilkCode, "description": "Oat milk urgent demand", "warehouse_code": warehouseCode, "uom_code": "ea", "quantity": 3.0, "unit_price": 42000.0, "line_total": 126000.0},
		},
	}); err != nil {
		return scenarioManifest{}, fmt.Errorf("seed confirmed sales order: %w", err)
	}

	manifest.SessionInstructions = strings.TrimSpace(defaultSessionInstructions("inventory_replenishment_execute") + fmt.Sprintf(`
- Review replenishment risk through the dedicated planning MCP tools before using generic business record search.
- Start with planning_core.replenishment.insight.summary for warehouse %s.
- Then use planning_core.replenishment.plan.summary to turn the risk signal into recommended quantities and vendor grouping.
- If asked to execute, use planning_core.purchase_requests.draft.create with the recommended item_code, warehouse_code, and quantity selections, and do not submit the resulting purchase request drafts.
- If the MCP server prefixes tool names, choose the tools whose names end with planning_core_replenishment_insight_summary, planning_core_replenishment_plan_summary, and planning_core_purchase_requests_draft_create.`, warehouseCode))
	manifest.Entities = map[string]map[string]any{
		"warehouse":       {"code": warehouseCode},
		"vendor":          {"id": stringValue(vendor["id"]), "name": vendorName},
		"cold_brew_beans": {"sku": coldBrewCode, "name": "Cold Brew Beans 1kg " + runID},
		"oat_milk":        {"sku": oatMilkCode, "name": "Oat Milk Barista 1L " + runID},
		"matcha":          {"sku": matchaCode, "name": "Matcha Powder 500g " + runID},
		"paper_cups":      {"sku": cupsCode, "name": "Paper Cups 16oz " + runID},
	}
	manifest.GroundTruth = map[string]any{
		"warehouse_code":         warehouseCode,
		"recommended_items":      []string{"Cold Brew Beans 1kg " + runID, "Oat Milk Barista 1L " + runID},
		"recommended_quantities": map[string]float64{coldBrewCode: 20.0, oatMilkCode: 16.0},
		"recommended_vendor":     vendorName,
		"skip_items":             []string{"Matcha Powder 500g " + runID, "Paper Cups 16oz " + runID},
		"forecast_signal":        "six weeks of repeated fulfillment demand plus one current confirmed sales order",
		"draft_document_type":    "purchase_request",
	}
	manifest.PromptPack = inventoryReplenishmentExecutePromptPack(runID, warehouseCode, vendorName, coldBrewCode, oatMilkCode)
	return manifest, nil
}

func seedInventoryDashboardReplenishmentExecuteScenario(ctx context.Context, client *apiClient, baseURL, opencodeCommand string) (scenarioManifest, error) {
	manifest, err := seedInventoryReplenishmentExecuteScenario(ctx, client, baseURL, opencodeCommand)
	if err != nil {
		return scenarioManifest{}, err
	}
	warehouseCode := stringValue(manifest.Entities["warehouse"]["code"])
	vendorName := stringValue(manifest.Entities["vendor"]["name"])
	coldBrewCode := stringValue(manifest.Entities["cold_brew_beans"]["sku"])
	oatMilkCode := stringValue(manifest.Entities["oat_milk"]["sku"])
	manifest.Scenario = "inventory_dashboard_replenishment_execute"
	manifest.DomainBundle = "planning-procurement-dashboard"
	manifest.SessionInstructions = strings.TrimSpace(defaultSessionInstructions("inventory_dashboard_replenishment_execute") + fmt.Sprintf(`
- For replenishment insight questions, first call analytics.dashboard.widget_catalog with surface "dashboard".
- Then call analytics.dashboard.board.preview with title "Replenishment Risk Dashboard", surface "dashboard", and a description covering warehouse %s replenishment risk, shortage candidates, suggested request quantity, and due-soon replenishment.
- For this replenishment dashboard scenario, use the planning.replenishment widget family and do not use unrelated sales or demo widgets in the dashboard artifact.
- Do not hand-pick dashboard widget keys for the insight turn. Let analytics.dashboard.board.preview infer the planning widget set from the title and description.
- In the first paragraph of the insight answer, explicitly identify the at-risk items and the healthy items to skip for warehouse %s when the dashboard evidence supports it.
- When the dashboard preview tool returns widget/session artifacts, rely on those artifacts for rendering and briefly explain which widgets matter in the final answer.
- For planning questions, base the plan on the dashboard evidence, keep the answer stepwise as a numbered list using "1.", "2.", and "3." style markers, and do not execute the plan.
- For execute questions, do not call dashboard or analytics tools again. Immediately call planning_core.purchase_requests.draft.create with the recommended selections for warehouse %s, do not submit the drafts, and restate the created purchase request draft ids, vendor, and line quantities.`, warehouseCode, warehouseCode, warehouseCode))
	manifest.GroundTruth["dashboard_widget_keys"] = []string{
		"planning.replenishment.shortages",
		"planning.replenishment.items",
	}
	manifest.PromptPack = inventoryDashboardReplenishmentExecutePromptPack(manifest.RunID, warehouseCode, vendorName, coldBrewCode, oatMilkCode)
	return manifest, nil
}

func seedSalesDashboardRecoveryExecuteScenario(ctx context.Context, client *apiClient, baseURL, opencodeCommand string) (scenarioManifest, error) {
	runID, suffix := newRunContext()
	manifest, err := newScenarioManifest(ctx, client, baseURL, opencodeCommand, "sales_dashboard_recovery_execute", "analytics-recovery", runID, suffix)
	if err != nil {
		return scenarioManifest{}, err
	}

	draftTitle := "Sales Recovery Plan " + runID
	type documentSeed struct {
		documentType string
		locationID   string
		status       string
		title        string
	}
	batches := [][]documentSeed{
		{
			{documentType: "generic_request", locationID: "loc_demo_central", status: "submitted", title: "Central soft demand " + runID},
			{documentType: "generic_request", locationID: "loc_demo_west", status: "approved", title: "West branch sale " + runID},
			{documentType: "generic_request", locationID: "loc_demo_east", status: "approved", title: "East branch flagship sale " + runID},
			{documentType: "generic_request", locationID: "loc_demo_east", status: "approved", title: "East branch receivable " + runID},
		},
		{
			{documentType: "generic_request", locationID: "loc_demo_central", status: "submitted", title: "Central recovery escalation " + runID},
			{documentType: "generic_request", locationID: "loc_demo_west", status: "submitted", title: "West catch-up order " + runID},
			{documentType: "generic_request", locationID: "loc_demo_west", status: "approved", title: "West branch invoice " + runID},
			{documentType: "generic_request", locationID: "loc_demo_east", status: "approved", title: "East expansion sale " + runID},
			{documentType: "generic_request", locationID: "loc_demo_east", status: "approved", title: "East branch billing " + runID},
		},
		{
			{documentType: "generic_request", locationID: "loc_demo_central", status: "draft", title: "Central tentative order " + runID},
			{documentType: "generic_request", locationID: "loc_demo_west", status: "approved", title: "West branch renewal " + runID},
			{documentType: "generic_request", locationID: "loc_demo_east", status: "approved", title: "East enterprise uplift " + runID},
			{documentType: "generic_request", locationID: "loc_demo_east", status: "approved", title: "East month-end invoice " + runID},
		},
	}

	for batchIndex, batch := range batches {
		for _, item := range batch {
			created, err := client.createDocument(ctx, map[string]any{
				"type":            item.documentType,
				"organization_id": defaultOrgID,
				"location_id":     defaultLocID,
				"payload": map[string]any{
					"title":              item.title,
					"branch_location_id": item.locationID,
				},
			})
			if err != nil {
				return scenarioManifest{}, fmt.Errorf("seed %s: %w", item.documentType, err)
			}
			switch item.status {
			case "approved":
				if _, err := submitApproveAndMaybeStop(ctx, client, created.ID, true); err != nil {
					return scenarioManifest{}, fmt.Errorf("seed approved %s: %w", item.documentType, err)
				}
			case "submitted":
				if _, err := submitApproveAndMaybeStop(ctx, client, created.ID, false); err != nil {
					return scenarioManifest{}, fmt.Errorf("submit %s: %w", item.documentType, err)
				}
			}
		}
		if err := client.captureAnalyticsSnapshot(ctx); err != nil {
			return scenarioManifest{}, fmt.Errorf("capture analytics snapshot batch %d: %w", batchIndex+1, err)
		}
		time.Sleep(15 * time.Millisecond)
	}

	manifest.SessionInstructions = strings.TrimSpace(defaultSessionInstructions("sales_dashboard_recovery_execute") + `
- For insight questions about branch performance, first call analytics.dashboard.widget_catalog with surface "dashboard".
- Then call analytics.dashboard.board.preview with title "Sales Performance Dashboard", surface "dashboard", and explicit widget_keys from the analytics.demo.sales widget family.
- Use the analytics.demo.sales widget family for this synthetic scenario and do not mix in other widget families.
- In the first paragraph of the insight answer, explicitly state that Loc Demo Central and Loc Demo West are underperforming compared with Loc Demo East as the benchmark leader when the data supports it.
- When the dashboard preview tool returns widget/session artifacts, rely on those artifacts for rendering and briefly explain which widgets matter in the final answer.
- Name Loc Demo Central and Loc Demo West explicitly as underperforming when they trail the benchmark, and explicitly name Loc Demo East as the benchmark when it leads.
- For planning questions, base the plan on the dashboard evidence, keep the answer stepwise as a numbered list using "1.", "2.", and "3." style markers, and do not execute the plan.
- For execute questions, do not call analytics or business-info tools again. Immediately call business.document.draft.create with document_type "generic_request", location_id "loc_hq", organization_id "org_default", confirm_apply true, and a payload containing title, summary, branches, benchmark, and follow_up.
- After creating the draft, restate it as a draft generic request including the exact draft title, draft id, open path, and the seeded branch/benchmark/follow-up details.`)
	manifest.Entities = map[string]map[string]any{
		"central_branch": {"location_id": "loc_demo_central", "label": "Loc Demo Central"},
		"east_branch":    {"location_id": "loc_demo_east", "label": "Loc Demo East"},
		"west_branch":    {"location_id": "loc_demo_west", "label": "Loc Demo West"},
	}
	manifest.GroundTruth = map[string]any{
		"underperforming_branches": []string{"Loc Demo Central", "Loc Demo West"},
		"benchmark_branch":         "Loc Demo East",
		"dashboard_widget_keys": []string{
			"analytics.demo.sales.net_sales",
			"analytics.demo.sales.target_attainment",
			"analytics.demo.sales.daily_trend",
			"analytics.demo.sales.branch_mix",
			"analytics.demo.sales.branch_table",
			"analytics.demo.sales.branch_map",
		},
		"draft_title":         draftTitle,
		"draft_document_type": "generic_request",
	}
	manifest.PromptPack = salesDashboardRecoveryExecutePromptPack(runID, draftTitle)
	return manifest, nil
}

func seedPOSPromotionStrategyScenario(ctx context.Context, client *apiClient, baseURL, opencodeCommand string) (scenarioManifest, error) {
	runID, suffix := newRunContext()
	manifest, err := newScenarioManifest(ctx, client, baseURL, opencodeCommand, "pos_promotion_strategy", "retail-promotion", runID, suffix)
	if err != nil {
		return scenarioManifest{}, err
	}

	storeCode := "PROMO-STORE-" + suffix
	registerCode := "PROMO-REG-" + suffix
	espressoCode := "PROMO-ESPRESSO-" + suffix
	croissantCode := "PROMO-CROISSANT-" + suffix
	beansCode := "PROMO-BEANS-" + suffix
	teaCode := "PROMO-TEA-" + suffix
	beansCampaignCode := "BEANS-BOOST-" + suffix
	beansPromoCode := "BEANS10-" + suffix
	beansRuleCode := "RULE-BEANS10-" + suffix
	draftTitle := "Promotion Plan " + runID

	if err := ensureSeedPOSReferences(ctx, client); err != nil {
		return scenarioManifest{}, err
	}
	paymentMethod, err := client.createModel(ctx, "payment_method", map[string]any{
		"code":                  "CASHPROMO-" + suffix,
		"name":                  "Cash Promo " + runID,
		"kind":                  "cash",
		"clearing_account_code": "1000-CASH",
		"status":                "active",
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create payment method: %w", err)
	}
	if _, err := client.createModel(ctx, "pos_store", map[string]any{
		"code":             storeCode,
		"name":             "Promotion Test Store " + runID,
		"warehouse_code":   "MAIN",
		"default_tax_code": "",
		"currency_code":    "IDR",
		"checkout_mode":    "invoice_first",
		"status":           "active",
	}); err != nil {
		return scenarioManifest{}, fmt.Errorf("create pos store: %w", err)
	}
	if _, err := client.createModel(ctx, "pos_register", map[string]any{
		"code":              registerCode,
		"name":              "Promotion Register " + runID,
		"store_code":        storeCode,
		"checkout_mode":     "invoice_first",
		"cash_account_code": "1000-CASH",
		"status":            "active",
	}); err != nil {
		return scenarioManifest{}, fmt.Errorf("create pos register: %w", err)
	}
	if _, err := client.createModel(ctx, "pos_tender_type", map[string]any{
		"code":                  "CASH-" + suffix,
		"name":                  "Cash",
		"kind":                  "cash",
		"payment_method_code":   stringValue(mapValue(paymentMethod, "values")["code"]),
		"clearing_account_code": "1000-CASH",
		"is_cash_like":          true,
		"status":                "active",
	}); err != nil {
		return scenarioManifest{}, fmt.Errorf("create pos tender type: %w", err)
	}

	for _, item := range []map[string]any{
		{
			"sku":                  espressoCode,
			"name":                 "Espresso Double " + runID,
			"description":          "Frequent coffee purchase",
			"kind":                 "product",
			"uom_code":             "ea",
			"unit_price":           28000.0,
			"revenue_account_code": "4000-REV",
			"is_sellable":          true,
			"inventory_enabled":    false,
			"allow_negative_stock": true,
			"status":               "active",
		},
		{
			"sku":                  croissantCode,
			"name":                 "Butter Croissant " + runID,
			"description":          "Frequent pastry attachment",
			"kind":                 "product",
			"uom_code":             "ea",
			"unit_price":           22000.0,
			"revenue_account_code": "4000-REV",
			"is_sellable":          true,
			"inventory_enabled":    false,
			"allow_negative_stock": true,
			"status":               "active",
		},
		{
			"sku":                  beansCode,
			"name":                 "House Beans 1kg " + runID,
			"description":          "Higher-value packaged beans",
			"kind":                 "product",
			"uom_code":             "ea",
			"unit_price":           95000.0,
			"revenue_account_code": "4000-REV",
			"is_sellable":          true,
			"inventory_enabled":    false,
			"allow_negative_stock": true,
			"status":               "active",
		},
		{
			"sku":                  teaCode,
			"name":                 "Iced Tea " + runID,
			"description":          "Secondary beverage",
			"kind":                 "product",
			"uom_code":             "ea",
			"unit_price":           18000.0,
			"revenue_account_code": "4000-REV",
			"is_sellable":          true,
			"inventory_enabled":    false,
			"allow_negative_stock": true,
			"status":               "active",
		},
	} {
		if err := createSeedCommercialItem(ctx, client, item); err != nil {
			return scenarioManifest{}, fmt.Errorf("create item %s: %w", stringValue(item["sku"]), err)
		}
	}

	goldOne, err := createSeedCustomerProfile(ctx, client, "Alya Santoso "+runID, map[string]any{
		"customer_name":   "Alya Santoso " + runID,
		"customer_type":   "member",
		"member_status":   "active",
		"member_tier":     "gold",
		"member_valid_to": "2099-12-31",
		"status":          "active",
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create gold customer 1: %w", err)
	}
	goldTwo, err := createSeedCustomerProfile(ctx, client, "Bima Pratama "+runID, map[string]any{
		"customer_name":   "Bima Pratama " + runID,
		"customer_type":   "member",
		"member_status":   "active",
		"member_tier":     "gold",
		"member_valid_to": "2099-12-31",
		"status":          "active",
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create gold customer 2: %w", err)
	}
	silverCustomer, err := createSeedCustomerProfile(ctx, client, "Citra Lestari "+runID, map[string]any{
		"customer_name":   "Citra Lestari " + runID,
		"customer_type":   "member",
		"member_status":   "active",
		"member_tier":     "silver",
		"member_valid_to": "2099-12-31",
		"status":          "active",
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create silver customer: %w", err)
	}

	if _, err := client.createModel(ctx, "promotion_campaign", map[string]any{
		"code":           beansCampaignCode,
		"name":           "Beans Boost " + runID,
		"trigger_mode":   "code",
		"sales_channels": "pos",
		"store_codes":    storeCode,
		"status":         "active",
	}); err != nil {
		return scenarioManifest{}, fmt.Errorf("create promotion campaign: %w", err)
	}
	if _, err := client.createModel(ctx, "promotion_code", map[string]any{
		"code":                    beansPromoCode,
		"promotion_campaign_code": beansCampaignCode,
		"status":                  "active",
	}); err != nil {
		return scenarioManifest{}, fmt.Errorf("create promotion code: %w", err)
	}
	if _, err := client.createModel(ctx, "discount_rule", map[string]any{
		"code":                    beansRuleCode,
		"name":                    "Beans 10 Percent " + runID,
		"promotion_campaign_code": beansCampaignCode,
		"scope":                   "line",
		"rule_kind":               "line_percent",
		"item_codes":              beansCode,
		"discount_percent":        10.0,
		"status":                  "active",
	}); err != nil {
		return scenarioManifest{}, fmt.Errorf("create discount rule: %w", err)
	}

	shift, err := client.openPOSShift(ctx, storeCode, registerCode, 500000.0, "Agentproof promotion strategy seed")
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("open pos shift: %w", err)
	}
	shiftID := stringValue(shift["id"])
	if shiftID == "" {
		return scenarioManifest{}, fmt.Errorf("open pos shift: missing shift id")
	}

	checkout := func(party map[string]any, lines []map[string]any, promotionCodes []string, tenderAmount float64, reference string) error {
		req := map[string]any{
			"store_code":      storeCode,
			"register_code":   registerCode,
			"shift_id":        shiftID,
			"promotion_codes": promotionCodes,
			"lines":           lines,
			"tenders": []map[string]any{{
				"tender_type_code": "CASH-" + suffix,
				"amount":           tenderAmount,
			}},
			"reference": reference,
		}
		if party != nil {
			req["party_id"] = stringValue(mapValue(party, "values")["party_id"])
			req["party_name"] = stringValue(mapValue(party, "values")["customer_name"])
		}
		_, err := client.posCheckout(ctx, req)
		return err
	}

	comboLines := []map[string]any{
		{"item_code": espressoCode, "quantity": 1.0},
		{"item_code": croissantCode, "quantity": 1.0},
	}
	for index, party := range []map[string]any{goldOne, goldOne, goldTwo, goldTwo, goldOne, goldTwo} {
		if err := checkout(party, comboLines, nil, 50000.0, fmt.Sprintf("PROMO-COMBO-%s-%d", suffix, index+1)); err != nil {
			return scenarioManifest{}, fmt.Errorf("seed combo sale %d: %w", index+1, err)
		}
	}
	if err := checkout(goldOne, []map[string]any{{"item_code": espressoCode, "quantity": 1.0}}, nil, 28000.0, "PROMO-ESPRESSO-"+suffix); err != nil {
		return scenarioManifest{}, fmt.Errorf("seed espresso sale: %w", err)
	}
	if err := checkout(silverCustomer, []map[string]any{{"item_code": teaCode, "quantity": 1.0}}, nil, 18000.0, "PROMO-TEA-"+suffix); err != nil {
		return scenarioManifest{}, fmt.Errorf("seed tea sale: %w", err)
	}
	if err := checkout(goldOne, []map[string]any{{"item_code": beansCode, "quantity": 1.0}}, []string{beansPromoCode}, 85500.0, "PROMO-BEANS-"+suffix); err != nil {
		return scenarioManifest{}, fmt.Errorf("seed beans promo sale: %w", err)
	}
	if err := checkout(nil, []map[string]any{{"item_code": beansCode, "quantity": 1.0}}, nil, 95000.0, "PROMO-BEANS-WALKIN-"+suffix); err != nil {
		return scenarioManifest{}, fmt.Errorf("seed beans walk-in sale: %w", err)
	}

	manifest.SessionInstructions = strings.TrimSpace(defaultSessionInstructions("pos_promotion_strategy") + `
- Review POS sales, customer/member profiles, promotion campaigns, promotion redemptions, and discount rules before recommending a campaign.
- Start with MCP tools pos_core.sales.strategy.summary and promotion_core.performance.summary before using generic business record search.
- If the MCP server prefixes tool names, choose the tools whose names end with pos_core_sales_strategy_summary, promotion_core_performance_summary, and promotion_core_strategy_plan_draft_create.
- Prefer a concrete campaign design with target products, target member/customer segment, and a specific current promotion to replace if the data supports it.
- If asked to create a draft, use promotion_core.strategy.plan.draft.create to create a draft generic_request and do not submit it.`)
	manifest.Entities = map[string]map[string]any{
		"store":           {"code": storeCode, "register_code": registerCode},
		"gold_member_one": {"party_id": stringValue(mapValue(goldOne, "values")["party_id"]), "name": stringValue(mapValue(goldOne, "values")["customer_name"]), "member_tier": "gold"},
		"gold_member_two": {"party_id": stringValue(mapValue(goldTwo, "values")["party_id"]), "name": stringValue(mapValue(goldTwo, "values")["customer_name"]), "member_tier": "gold"},
		"silver_member":   {"party_id": stringValue(mapValue(silverCustomer, "values")["party_id"]), "name": stringValue(mapValue(silverCustomer, "values")["customer_name"]), "member_tier": "silver"},
		"espresso":        {"sku": espressoCode, "name": "Espresso Double " + runID},
		"croissant":       {"sku": croissantCode, "name": "Butter Croissant " + runID},
		"beans":           {"sku": beansCode, "name": "House Beans 1kg " + runID},
		"tea":             {"sku": teaCode, "name": "Iced Tea " + runID},
	}
	manifest.GroundTruth = map[string]any{
		"recommended_campaign_kind":    "member_bundle_discount",
		"recommended_products":         []string{"Espresso Double " + runID, "Butter Croissant " + runID},
		"recommended_segment":          "gold members",
		"supporting_pattern":           "espresso and croissant were repeatedly purchased together by gold members",
		"underperforming_campaign":     "Beans Boost " + runID,
		"underperforming_promo_code":   beansPromoCode,
		"underperforming_reason":       "only one redemption and weak demand signal compared with the breakfast bundle pattern",
		"combo_sale_count":             6,
		"beans_promo_redemption_count": 1,
		"draft_title":                  draftTitle,
		"draft_document_type":          "generic_request",
	}
	manifest.PromptPack = posPromotionStrategyPromptPack(runID, storeCode, espressoCode, croissantCode, beansPromoCode, draftTitle)
	manifest.SessionInstructions = strings.TrimSpace(manifest.SessionInstructions + fmt.Sprintf(`
- For this scenario, use store code %s when calling POS sales and promotion summary tools.
- Leave date_from and date_to empty unless the prompt explicitly asks for a date window.
- Start with pos_core.sales.strategy.summary and promotion_core.performance.summary, then answer directly if they already provide enough evidence.
- If those tools return a clear recommendation signal and underperforming campaign, do not fall back to generic business record search.`, storeCode))
	return manifest, nil
}

func seedRetailRecoveryShowcaseScenario(ctx context.Context, client *apiClient, baseURL, opencodeCommand string) (scenarioManifest, error) {
	runID, suffix := newRunContext()
	manifest, err := newScenarioManifest(ctx, client, baseURL, opencodeCommand, "retail_recovery_showcase", "retail-dashboard-agent", runID, suffix)
	if err != nil {
		return scenarioManifest{}, err
	}

	storeCode := "SHOWCASE-STORE-" + suffix
	registerCode := "SHOWCASE-REG-" + suffix
	espressoCode := "SHOWCASE-ESPRESSO-" + suffix
	croissantCode := "SHOWCASE-CROISSANT-" + suffix
	beansCode := "SHOWCASE-BEANS-" + suffix
	teaCode := "SHOWCASE-TEA-" + suffix
	campaignCode := "SHOWCASE-BEANS-BOOST-" + suffix
	promoCode := "SHOWCASE-BEANS10-" + suffix
	ruleCode := "SHOWCASE-RULE-BEANS10-" + suffix
	draftTitle := "Promotion Recovery Plan " + runID

	if err := ensureSeedPOSReferences(ctx, client); err != nil {
		return scenarioManifest{}, err
	}
	paymentMethod, err := client.createModel(ctx, "payment_method", map[string]any{
		"code":                  "CASHSHOW-" + suffix,
		"name":                  "Cash Showcase " + runID,
		"kind":                  "cash",
		"clearing_account_code": "1000-CASH",
		"status":                "active",
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create payment method: %w", err)
	}
	if _, err := client.createModel(ctx, "pos_store", map[string]any{
		"code":             storeCode,
		"name":             "Retail Recovery Store " + runID,
		"warehouse_code":   "MAIN",
		"default_tax_code": "",
		"currency_code":    "IDR",
		"checkout_mode":    "invoice_first",
		"status":           "active",
	}); err != nil {
		return scenarioManifest{}, fmt.Errorf("create pos store: %w", err)
	}
	if _, err := client.createModel(ctx, "pos_register", map[string]any{
		"code":              registerCode,
		"name":              "Front Register " + runID,
		"store_code":        storeCode,
		"checkout_mode":     "invoice_first",
		"cash_account_code": "1000-CASH",
		"status":            "active",
	}); err != nil {
		return scenarioManifest{}, fmt.Errorf("create pos register: %w", err)
	}
	if _, err := client.createModel(ctx, "pos_tender_type", map[string]any{
		"code":                  "CASH-" + suffix,
		"name":                  "Cash",
		"kind":                  "cash",
		"payment_method_code":   stringValue(mapValue(paymentMethod, "values")["code"]),
		"clearing_account_code": "1000-CASH",
		"is_cash_like":          true,
		"status":                "active",
	}); err != nil {
		return scenarioManifest{}, fmt.Errorf("create pos tender type: %w", err)
	}
	if _, err := client.createModel(ctx, "pos_tender_type", map[string]any{
		"code":                  "CARD-" + suffix,
		"name":                  "Card",
		"kind":                  "card",
		"payment_method_code":   stringValue(mapValue(paymentMethod, "values")["code"]),
		"clearing_account_code": "1010-BANK",
		"is_cash_like":          false,
		"status":                "active",
	}); err != nil {
		return scenarioManifest{}, fmt.Errorf("create card tender type: %w", err)
	}
	for _, item := range []map[string]any{
		{
			"sku":                  espressoCode,
			"name":                 "Espresso Double " + runID,
			"description":          "Repeat breakfast beverage",
			"kind":                 "product",
			"uom_code":             "ea",
			"unit_price":           28000.0,
			"revenue_account_code": "4000-REV",
			"is_sellable":          true,
			"inventory_enabled":    false,
			"allow_negative_stock": true,
			"status":               "active",
		},
		{
			"sku":                  croissantCode,
			"name":                 "Butter Croissant " + runID,
			"description":          "Repeat breakfast attachment",
			"kind":                 "product",
			"uom_code":             "ea",
			"unit_price":           22000.0,
			"revenue_account_code": "4000-REV",
			"is_sellable":          true,
			"inventory_enabled":    false,
			"allow_negative_stock": true,
			"status":               "active",
		},
		{
			"sku":                  beansCode,
			"name":                 "House Beans 1kg " + runID,
			"description":          "Weak current promoted product",
			"kind":                 "product",
			"uom_code":             "ea",
			"unit_price":           95000.0,
			"revenue_account_code": "4000-REV",
			"is_sellable":          true,
			"inventory_enabled":    false,
			"allow_negative_stock": true,
			"status":               "active",
		},
		{
			"sku":                  teaCode,
			"name":                 "Iced Tea " + runID,
			"description":          "Secondary beverage control item",
			"kind":                 "product",
			"uom_code":             "ea",
			"unit_price":           18000.0,
			"revenue_account_code": "4000-REV",
			"is_sellable":          true,
			"inventory_enabled":    false,
			"allow_negative_stock": true,
			"status":               "active",
		},
	} {
		if err := createSeedCommercialItem(ctx, client, item); err != nil {
			return scenarioManifest{}, fmt.Errorf("create item %s: %w", stringValue(item["sku"]), err)
		}
	}

	goldOne, err := createSeedCustomerProfile(ctx, client, "Alya Santoso "+runID, map[string]any{
		"customer_name":   "Alya Santoso " + runID,
		"customer_type":   "member",
		"member_status":   "active",
		"member_tier":     "gold",
		"member_valid_to": "2099-12-31",
		"status":          "active",
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create gold customer 1: %w", err)
	}
	goldTwo, err := createSeedCustomerProfile(ctx, client, "Bima Pratama "+runID, map[string]any{
		"customer_name":   "Bima Pratama " + runID,
		"customer_type":   "member",
		"member_status":   "active",
		"member_tier":     "gold",
		"member_valid_to": "2099-12-31",
		"status":          "active",
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create gold customer 2: %w", err)
	}
	silverCustomer, err := createSeedCustomerProfile(ctx, client, "Citra Lestari "+runID, map[string]any{
		"customer_name":   "Citra Lestari " + runID,
		"customer_type":   "member",
		"member_status":   "active",
		"member_tier":     "silver",
		"member_valid_to": "2099-12-31",
		"status":          "active",
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create silver customer: %w", err)
	}

	if _, err := client.createModel(ctx, "promotion_campaign", map[string]any{
		"code":           campaignCode,
		"name":           "Beans Boost " + runID,
		"trigger_mode":   "code",
		"sales_channels": "pos",
		"store_codes":    storeCode,
		"status":         "active",
	}); err != nil {
		return scenarioManifest{}, fmt.Errorf("create promotion campaign: %w", err)
	}
	if _, err := client.createModel(ctx, "promotion_code", map[string]any{
		"code":                    promoCode,
		"promotion_campaign_code": campaignCode,
		"status":                  "active",
	}); err != nil {
		return scenarioManifest{}, fmt.Errorf("create promotion code: %w", err)
	}
	if _, err := client.createModel(ctx, "discount_rule", map[string]any{
		"code":                    ruleCode,
		"name":                    "Beans 10 Percent " + runID,
		"promotion_campaign_code": campaignCode,
		"scope":                   "line",
		"rule_kind":               "line_percent",
		"item_codes":              beansCode,
		"discount_percent":        10.0,
		"status":                  "active",
	}); err != nil {
		return scenarioManifest{}, fmt.Errorf("create discount rule: %w", err)
	}

	shift, err := client.openPOSShift(ctx, storeCode, registerCode, 500000.0, "Retail recovery showcase seed")
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("open pos shift: %w", err)
	}
	shiftID := stringValue(shift["id"])
	if shiftID == "" {
		return scenarioManifest{}, fmt.Errorf("open pos shift: missing shift id")
	}
	terminalPIN, seededPIN, err := client.ensureCashierPIN(ctx, "123456")
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("ensure cashier pin: %w", err)
	}
	if terminalPIN != "" {
		if err := client.enterPOSTerminal(ctx, storeCode, registerCode, shiftID, terminalPIN); err != nil {
			if !seededPIN && strings.Contains(strings.ToLower(err.Error()), "invalid") {
				return scenarioManifest{}, fmt.Errorf("enter pos terminal: existing cashier PIN does not match showcase default 123456: %w", err)
			}
			return scenarioManifest{}, fmt.Errorf("enter pos terminal: %w", err)
		}
	}

	checkout := func(party map[string]any, lines []map[string]any, promotionCodes []string, tenders []map[string]any, reference string) error {
		req := map[string]any{
			"store_code":      storeCode,
			"register_code":   registerCode,
			"shift_id":        shiftID,
			"promotion_codes": promotionCodes,
			"lines":           lines,
			"tenders":         tenders,
			"reference":       reference,
		}
		if party != nil {
			req["party_id"] = stringValue(mapValue(party, "values")["party_id"])
			req["party_name"] = stringValue(mapValue(party, "values")["customer_name"])
		}
		_, err := client.posCheckout(ctx, req)
		return err
	}
	comboLines := []map[string]any{
		{"item_code": espressoCode, "quantity": 1.0},
		{"item_code": croissantCode, "quantity": 1.0},
	}
	for index, party := range []map[string]any{goldOne, goldOne, goldTwo, goldTwo, goldOne, goldTwo} {
		if err := checkout(party, comboLines, nil, []map[string]any{{"tender_type_code": "CASH-" + suffix, "amount": 50000.0}}, fmt.Sprintf("SHOWCASE-COMBO-%s-%d", suffix, index+1)); err != nil {
			return scenarioManifest{}, fmt.Errorf("seed combo sale %d: %w", index+1, err)
		}
	}
	if err := checkout(goldOne, []map[string]any{{"item_code": beansCode, "quantity": 1.0}}, []string{promoCode}, []map[string]any{{"tender_type_code": "CARD-" + suffix, "amount": 85500.0, "reference": "APPROVAL-" + suffix}}, "SHOWCASE-BEANS-"+suffix); err != nil {
		return scenarioManifest{}, fmt.Errorf("seed beans promo sale: %w", err)
	}
	if err := checkout(silverCustomer, []map[string]any{{"item_code": teaCode, "quantity": 1.0}}, nil, []map[string]any{{"tender_type_code": "CASH-" + suffix, "amount": 18000.0}}, "SHOWCASE-TEA-"+suffix); err != nil {
		return scenarioManifest{}, fmt.Errorf("seed tea sale: %w", err)
	}
	heldSale, err := client.posHoldSale(ctx, map[string]any{
		"store_code":    storeCode,
		"register_code": registerCode,
		"shift_id":      shiftID,
		"party_id":      stringValue(mapValue(goldOne, "values")["party_id"]),
		"party_name":    stringValue(mapValue(goldOne, "values")["customer_name"]),
		"lines": []map[string]any{
			{"item_code": beansCode, "quantity": 1.0},
			{"item_code": espressoCode, "quantity": 1.0},
		},
		"tenders": []map[string]any{
			{"tender_type_code": "CASH-" + suffix, "amount": 20000.0},
		},
		"reference": "SHOWCASE-HOLD-" + suffix,
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("hold sale: %w", err)
	}

	type documentSeed struct {
		documentType string
		locationID   string
		status       string
		title        string
	}
	batches := [][]documentSeed{
		{
			{documentType: "generic_request", locationID: "loc_demo_central", status: "submitted", title: "Central soft demand " + runID},
			{documentType: "generic_request", locationID: "loc_demo_west", status: "approved", title: "West branch sale " + runID},
			{documentType: "generic_request", locationID: "loc_demo_east", status: "approved", title: "East branch flagship sale " + runID},
			{documentType: "generic_request", locationID: "loc_demo_east", status: "approved", title: "East branch receivable " + runID},
		},
		{
			{documentType: "generic_request", locationID: "loc_demo_central", status: "submitted", title: "Central recovery escalation " + runID},
			{documentType: "generic_request", locationID: "loc_demo_west", status: "submitted", title: "West catch-up order " + runID},
			{documentType: "generic_request", locationID: "loc_demo_west", status: "approved", title: "West branch invoice " + runID},
			{documentType: "generic_request", locationID: "loc_demo_east", status: "approved", title: "East expansion sale " + runID},
			{documentType: "generic_request", locationID: "loc_demo_east", status: "approved", title: "East branch billing " + runID},
		},
		{
			{documentType: "generic_request", locationID: "loc_demo_central", status: "draft", title: "Central tentative order " + runID},
			{documentType: "generic_request", locationID: "loc_demo_west", status: "approved", title: "West branch renewal " + runID},
			{documentType: "generic_request", locationID: "loc_demo_east", status: "approved", title: "East enterprise uplift " + runID},
			{documentType: "generic_request", locationID: "loc_demo_east", status: "approved", title: "East month-end invoice " + runID},
		},
	}
	for batchIndex, batch := range batches {
		for _, item := range batch {
			created, err := client.createDocument(ctx, map[string]any{
				"type":            item.documentType,
				"organization_id": defaultOrgID,
				"location_id":     defaultLocID,
				"payload": map[string]any{
					"title":              item.title,
					"branch_location_id": item.locationID,
				},
			})
			if err != nil {
				return scenarioManifest{}, fmt.Errorf("seed %s: %w", item.documentType, err)
			}
			switch item.status {
			case "approved":
				if _, err := submitApproveAndMaybeStop(ctx, client, created.ID, true); err != nil {
					return scenarioManifest{}, fmt.Errorf("seed approved %s: %w", item.documentType, err)
				}
			case "submitted":
				if _, err := submitApproveAndMaybeStop(ctx, client, created.ID, false); err != nil {
					return scenarioManifest{}, fmt.Errorf("submit %s: %w", item.documentType, err)
				}
			}
		}
		if err := client.captureAnalyticsSnapshot(ctx); err != nil {
			return scenarioManifest{}, fmt.Errorf("capture analytics snapshot batch %d: %w", batchIndex+1, err)
		}
		time.Sleep(15 * time.Millisecond)
	}

	board, err := client.createDashboardBoard(ctx, analytics.Dashboard{
		Name:        "Retail Recovery Board " + runID,
		Description: "Synthetic branch performance board for the retail recovery showcase across dashboard and agent surfaces.",
		Surface:     "dashboard",
		IsDefault:   true,
		Visibility:  "private",
		Status:      "active",
		Widgets: []analytics.DashboardWidget{
			{WidgetKey: "analytics.demo.sales.net_sales", Title: "Demo Net Sales", Kind: "metric", Width: 3, Height: 1, Order: 1},
			{WidgetKey: "analytics.demo.sales.target_attainment", Title: "Demo Target Attainment", Kind: "gauge", Width: 3, Height: 2, Order: 2},
			{WidgetKey: "analytics.demo.sales.daily_trend", Title: "Demo Daily Sales Trend", Kind: "chart_line", Width: 6, Height: 2, Order: 3},
			{WidgetKey: "analytics.demo.sales.branch_mix", Title: "Demo Branch Sales Mix", Kind: "chart_bar", Width: 6, Height: 2, Order: 4},
			{WidgetKey: "analytics.demo.sales.branch_table", Title: "Demo Branch Sales Breakdown", Kind: "table", Width: 6, Height: 2, Order: 5},
			{WidgetKey: "analytics.demo.sales.branch_map", Title: "Demo Branch Performance", Kind: "map", Width: 6, Height: 2, Order: 6},
		},
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create dashboard board: %w", err)
	}

	manifest.Routes = map[string]string{
		"pos_terminal": "/ui/pos/terminal",
		"dashboard":    "/ui/dashboard",
		"agent":        "/ui/agent/workspace",
	}
	manifest.Walkthrough = []showcaseChapter{
		{
			Surface: "pos",
			Title:   "POS terminal walkthrough",
			Steps: []string{
				fmt.Sprintf("Open /ui/pos/terminal, unlock the seeded terminal for store %s and register %s, and verify the active shift %s.", storeCode, registerCode, shiftID),
				fmt.Sprintf("Resume the held sale %s or create a fresh basket with Espresso Double %s and Butter Croissant %s.", stringValue(heldSale["id"]), runID, runID),
				"Complete checkout to demonstrate cashier flow, held sale recovery, and promotion-aware retail operations.",
			},
		},
		{
			Surface: "dashboard",
			Title:   "Dashboard walkthrough",
			Steps: []string{
				fmt.Sprintf("Open /ui/dashboard and verify the default board %q renders all six demo sales widgets.", board.Name),
				"Use the board to show that Loc Demo East leads, while Loc Demo Central and Loc Demo West are below the benchmark.",
			},
		},
		{
			Surface: "agent",
			Title:   "Agent continuity walkthrough",
			Steps: []string{
				"Ask the seeded insight question and confirm the agent returns an inline dashboard artifact with only the most relevant evidence widgets rather than the full board.",
				"Ask the planning question and confirm Current Plan is populated with branch, segment, and campaign actions.",
				"Ask the execute question and confirm the agent creates a draft promotion recovery request with an open link.",
			},
		},
	}
	manifest.Entities = map[string]map[string]any{
		"store": {
			"code":          storeCode,
			"register_code": registerCode,
			"shift_id":      shiftID,
		},
		"central_branch": {"location_id": "loc_demo_central", "label": "Loc Demo Central"},
		"east_branch":    {"location_id": "loc_demo_east", "label": "Loc Demo East"},
		"west_branch":    {"location_id": "loc_demo_west", "label": "Loc Demo West"},
		"gold_member_one": {
			"party_id":    stringValue(mapValue(goldOne, "values")["party_id"]),
			"name":        stringValue(mapValue(goldOne, "values")["customer_name"]),
			"member_tier": "gold",
		},
		"gold_member_two": {
			"party_id":    stringValue(mapValue(goldTwo, "values")["party_id"]),
			"name":        stringValue(mapValue(goldTwo, "values")["customer_name"]),
			"member_tier": "gold",
		},
		"silver_member": {
			"party_id":    stringValue(mapValue(silverCustomer, "values")["party_id"]),
			"name":        stringValue(mapValue(silverCustomer, "values")["customer_name"]),
			"member_tier": "silver",
		},
		"espresso":  {"sku": espressoCode, "name": "Espresso Double " + runID},
		"croissant": {"sku": croissantCode, "name": "Butter Croissant " + runID},
		"beans":     {"sku": beansCode, "name": "House Beans 1kg " + runID},
		"tea":       {"sku": teaCode, "name": "Iced Tea " + runID},
		"campaign": {
			"code":       campaignCode,
			"promo_code": promoCode,
			"name":       "Beans Boost " + runID,
		},
		"dashboard_board": {
			"id":   board.ID,
			"name": board.Name,
			"path": "/ui/dashboard",
		},
		"held_sale": {
			"id":        stringValue(heldSale["id"]),
			"reference": "SHOWCASE-HOLD-" + suffix,
		},
	}
	manifest.GroundTruth = map[string]any{
		"underperforming_branches": []string{"Loc Demo Central", "Loc Demo West"},
		"benchmark_branch":         "Loc Demo East",
		"insight_widget_keys": []string{
			"analytics.demo.sales.target_attainment",
			"analytics.demo.sales.branch_mix",
			"analytics.demo.sales.daily_trend",
		},
		"dashboard_widget_keys": []string{
			"analytics.demo.sales.net_sales",
			"analytics.demo.sales.target_attainment",
			"analytics.demo.sales.daily_trend",
			"analytics.demo.sales.branch_mix",
			"analytics.demo.sales.branch_table",
			"analytics.demo.sales.branch_map",
		},
		"recommended_campaign_kind":    "member_breakfast_bundle",
		"recommended_products":         []string{"Espresso Double " + runID, "Butter Croissant " + runID},
		"recommended_segment":          "gold members",
		"supporting_pattern":           "espresso and croissant were repeatedly purchased together by gold members",
		"underperforming_campaign":     "Beans Boost " + runID,
		"underperforming_promo_code":   promoCode,
		"underperforming_reason":       "only one redemption and weak demand signal compared with the breakfast bundle pattern",
		"combo_sale_count":             6,
		"beans_promo_redemption_count": 1,
		"draft_title":                  draftTitle,
		"draft_document_type":          "generic_request",
	}
	manifest.SessionInstructions = strings.TrimSpace(defaultSessionInstructions("retail_recovery_showcase") + fmt.Sprintf(`
- This is a unified retail showcase. Keep POS evidence, dashboard evidence, and the final draft request consistent with one another.
- For insight questions, start with pos_core.sales.strategy.summary for store code %s.
- Then call analytics.dashboard.widgets.preview with surface "dashboard" and explicit widget_keys for analytics.demo.sales.target_attainment, analytics.demo.sales.branch_mix, and analytics.demo.sales.daily_trend.
- In the first paragraph of the insight answer, explicitly state that Loc Demo Central and Loc Demo West are underperforming compared with Loc Demo East as the benchmark leader, and explicitly mention the Espresso Double + Butter Croissant pattern among gold members.
- For the insight turn, keep the inline dashboard artifacts focused on only those most relevant widgets rather than the full six-widget board.
- When the widgets preview tool returns widget/session artifacts, rely on those artifacts for rendering and briefly explain which widgets matter in the final answer.
- For planning questions, base the plan on both the dashboard evidence and the POS sales pattern, keep the answer stepwise as a numbered list using "1.", "2.", and "3." markers, and do not execute it.
- The recommended campaign should be a breakfast bundle for gold members focused on Loc Demo Central and Loc Demo West while using Loc Demo East as the benchmark.
- For execute questions, do not call analytics or generic business-info tools again. Immediately call business.document.draft.create with document_type "generic_request", location_id "loc_hq", organization_id "org_default", confirm_apply true, and a payload containing title, summary, target_branches, benchmark_branch, target_products, target_segment, replace_campaign, and follow_up.
- After creating the draft, restate it as a draft promotion recovery request including the exact draft title, draft id, open path, target branches, target products, target segment, benchmark, and next-week follow-up.`, storeCode))
	manifest.PromptPack = retailRecoveryShowcasePromptPack(runID, storeCode, draftTitle)
	return manifest, nil
}

func seedCRMServiceSalesOverviewScenario(ctx context.Context, client *apiClient, baseURL, opencodeCommand string) (scenarioManifest, error) {
	runID, suffix := newRunContext()
	manifest, err := newScenarioManifest(ctx, client, baseURL, opencodeCommand, "crm_service_sales_overview", "crm-service-sales", runID, suffix)
	if err != nil {
		return scenarioManifest{}, err
	}

	now := time.Now().UTC()
	queueSupportCode := "CRM-SUPPORT-" + suffix
	queueSuccessCode := "CRM-SUCCESS-" + suffix
	customerName := "CRM Demo Customer " + runID
	healthyCustomerName := "Healthy Beans Co " + runID
	contactName := "Alya CRM " + runID
	healthyContactName := "Bima CRM " + runID
	ticketTitle := "Damaged shipment replacement"
	overdueTicketTitle := "Refund follow-up"
	opportunityTitle := "Quarterly catering contract"
	healthyOpportunityTitle := "Loyalty bundle expansion"

	queueSupport, err := client.createModel(ctx, "crm_queue", map[string]any{
		"code":                 queueSupportCode,
		"name":                 "CRM Support " + runID,
		"triage_sla_hours":     2,
		"resolution_sla_hours": 12,
		"status":               "active",
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create support queue: %w", err)
	}
	queueSuccess, err := client.createModel(ctx, "crm_queue", map[string]any{
		"code":                 queueSuccessCode,
		"name":                 "CRM Success " + runID,
		"triage_sla_hours":     4,
		"resolution_sla_hours": 24,
		"status":               "active",
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create success queue: %w", err)
	}
	if _, err := client.createModel(ctx, "crm_sla_policy", map[string]any{
		"code":                 "CRM-SLA-HIGH-" + suffix,
		"name":                 "CRM High Priority SLA " + runID,
		"queue_code":           queueSupportCode,
		"priority":             "high",
		"first_response_hours": 1,
		"resolution_hours":     8,
		"status":               "active",
	}); err != nil {
		return scenarioManifest{}, fmt.Errorf("create crm sla policy: %w", err)
	}
	if _, err := client.createModel(ctx, "crm_assignment_rule", map[string]any{
		"code":              "CRM-ASSIGN-" + suffix,
		"name":              "Support Queue Default " + runID,
		"queue_code":        queueSupportCode,
		"assign_queue_code": queueSupportCode,
		"assign_user_id":    "user_admin",
		"rank":              10,
		"status":            "active",
	}); err != nil {
		return scenarioManifest{}, fmt.Errorf("create crm assignment rule: %w", err)
	}

	party, err := client.createModel(ctx, "party", map[string]any{
		"party_type": "organization",
		"name":       customerName,
		"status":     "active",
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create crm customer party: %w", err)
	}
	healthyParty, err := client.createModel(ctx, "party", map[string]any{
		"party_type": "organization",
		"name":       healthyCustomerName,
		"status":     "active",
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create healthy crm party: %w", err)
	}
	if _, err := client.createModel(ctx, "customer_profile", map[string]any{
		"party_id":         stringValue(party["id"]),
		"customer_name":    customerName,
		"customer_type":    "member",
		"customer_segment": "strategic",
		"member_status":    "active",
		"member_tier":      "gold",
		"status":           "active",
	}); err != nil {
		return scenarioManifest{}, fmt.Errorf("create crm customer profile: %w", err)
	}
	if _, err := client.createModel(ctx, "customer_profile", map[string]any{
		"party_id":         stringValue(healthyParty["id"]),
		"customer_name":    healthyCustomerName,
		"customer_type":    "member",
		"customer_segment": "growth",
		"member_status":    "active",
		"member_tier":      "silver",
		"status":           "active",
	}); err != nil {
		return scenarioManifest{}, fmt.Errorf("create healthy customer profile: %w", err)
	}
	contact, err := client.createModel(ctx, "party_contact", map[string]any{
		"party_id":     stringValue(party["id"]),
		"name":         contactName,
		"contact_kind": "person",
		"email":        "alya+" + suffix + "@example.com",
		"status":       "active",
		"is_primary":   true,
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create crm contact: %w", err)
	}
	healthyContact, err := client.createModel(ctx, "party_contact", map[string]any{
		"party_id":     stringValue(healthyParty["id"]),
		"name":         healthyContactName,
		"contact_kind": "person",
		"email":        "bima+" + suffix + "@example.com",
		"status":       "active",
		"is_primary":   true,
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create healthy crm contact: %w", err)
	}

	overdueTicket, err := client.createModel(ctx, "crm_ticket", map[string]any{
		"ticket_number":         "CRM-TKT-OD-" + suffix,
		"title":                 overdueTicketTitle,
		"description":           "Customer still waiting for refund confirmation.",
		"party_id":              stringValue(party["id"]),
		"party_name":            customerName,
		"queue_code":            queueSupportCode,
		"priority":              "urgent",
		"severity":              "high",
		"status":                "open",
		"assignee_user_id":      "user_admin",
		"opened_at":             now.Add(-72 * time.Hour).Format(time.RFC3339),
		"first_response_due_at": now.Add(-48 * time.Hour).Format(time.RFC3339),
		"first_response_at":     now.Add(-47 * time.Hour).Format(time.RFC3339),
		"due_at":                now.Add(-24 * time.Hour).Format(time.RFC3339),
		"source_channel":        "email",
		"issue_category":        "refund",
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create overdue crm ticket: %w", err)
	}
	currentTicket, err := client.createModel(ctx, "crm_ticket", map[string]any{
		"ticket_number":         "CRM-TKT-OP-" + suffix,
		"title":                 ticketTitle,
		"description":           "Customer reports damaged cartons on arrival.",
		"party_id":              stringValue(party["id"]),
		"party_name":            customerName,
		"queue_code":            queueSupportCode,
		"priority":              "high",
		"severity":              "high",
		"status":                "open",
		"assignee_user_id":      "user_admin",
		"opened_at":             now.Add(-18 * time.Hour).Format(time.RFC3339),
		"first_response_due_at": now.Add(-16 * time.Hour).Format(time.RFC3339),
		"first_response_at":     now.Add(-15 * time.Hour).Format(time.RFC3339),
		"due_at":                now.Add(24 * time.Hour).Format(time.RFC3339),
		"source_channel":        "email",
		"issue_category":        "logistics",
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create current crm ticket: %w", err)
	}
	if _, err := client.createModel(ctx, "crm_ticket", map[string]any{
		"ticket_number":         "CRM-TKT-RS-" + suffix,
		"title":                 "Welcome package follow-up",
		"description":           "Member activation package delivered.",
		"party_id":              stringValue(healthyParty["id"]),
		"party_name":            healthyCustomerName,
		"queue_code":            queueSuccessCode,
		"priority":              "medium",
		"severity":              "low",
		"status":                "resolved",
		"assignee_user_id":      "user_admin",
		"opened_at":             now.Add(-96 * time.Hour).Format(time.RFC3339),
		"first_response_due_at": now.Add(-90 * time.Hour).Format(time.RFC3339),
		"first_response_at":     now.Add(-92 * time.Hour).Format(time.RFC3339),
		"due_at":                now.Add(-72 * time.Hour).Format(time.RFC3339),
		"resolved_at":           now.Add(-48 * time.Hour).Format(time.RFC3339),
		"resolution_notes":      "Package resent and confirmed delivered.",
		"source_channel":        "chat",
		"issue_category":        "onboarding",
	}); err != nil {
		return scenarioManifest{}, fmt.Errorf("create resolved crm ticket: %w", err)
	}
	for extra := 0; extra < 10; extra++ {
		if _, err := client.createModel(ctx, "crm_ticket", map[string]any{
			"ticket_number":         fmt.Sprintf("CRM-TKT-EX-%d-%s", extra+1, suffix),
			"title":                 fmt.Sprintf("Backlog escalation %d", extra+1),
			"description":           "Extra seeded support backlog to make the scenario queue dominant.",
			"party_id":              stringValue(party["id"]),
			"party_name":            customerName,
			"queue_code":            queueSupportCode,
			"priority":              "medium",
			"severity":              "low",
			"status":                "open",
			"assignee_user_id":      "user_admin",
			"opened_at":             now.Add(time.Duration(-10-extra) * time.Hour).Format(time.RFC3339),
			"first_response_due_at": now.Add(time.Duration(-9-extra) * time.Hour).Format(time.RFC3339),
			"first_response_at":     now.Add(time.Duration(-8-extra) * time.Hour).Format(time.RFC3339),
			"due_at":                now.Add(time.Duration(12-extra) * time.Hour).Format(time.RFC3339),
			"source_channel":        "email",
			"issue_category":        "service_recovery",
		}); err != nil {
			return scenarioManifest{}, fmt.Errorf("create extra crm backlog ticket: %w", err)
		}
	}

	if _, err := client.createModel(ctx, "crm_ticket_comment", map[string]any{
		"ticket_id":      stringValue(currentTicket["id"]),
		"ticket_number":  stringValue(mapValue(currentTicket, "values")["ticket_number"]),
		"comment_type":   "internal_note",
		"body":           "Replacement review started.",
		"author_user_id": "user_admin",
		"created_at":     now.Add(-14 * time.Hour).Format(time.RFC3339),
		"party_id":       stringValue(party["id"]),
		"party_name":     customerName,
	}); err != nil {
		return scenarioManifest{}, fmt.Errorf("create crm ticket comment: %w", err)
	}
	if _, err := client.createModel(ctx, "crm_ticket_activity", map[string]any{
		"ticket_id":         stringValue(currentTicket["id"]),
		"ticket_number":     stringValue(mapValue(currentTicket, "values")["ticket_number"]),
		"activity_type":     "assignment_note",
		"actor_user_id":     "user_admin",
		"assignee_user_id":  "user_admin",
		"queue_code":        queueSupportCode,
		"from_status":       "new",
		"to_status":         "open",
		"occurred_at":       now.Add(-14 * time.Hour).Format(time.RFC3339),
		"note":              "Assigned to support lead for recovery follow-up.",
		"party_id":          stringValue(party["id"]),
		"party_name":        customerName,
		"severity":          "high",
		"priority":          "high",
		"source_channel":    "email",
		"issue_category":    "logistics",
		"ticket_status_key": "open",
	}); err != nil {
		return scenarioManifest{}, fmt.Errorf("create crm ticket activity: %w", err)
	}

	lead, err := client.createModel(ctx, "crm_lead", map[string]any{
		"lead_number":         "CRM-LEAD-" + suffix,
		"title":               "Upsell catering program",
		"party_id":            stringValue(party["id"]),
		"party_name":          customerName,
		"contact_id":          stringValue(contact["id"]),
		"owner_user_id":       "user_admin",
		"source_channel":      "referral",
		"status":              "qualified",
		"rating":              "hot",
		"estimated_value":     18000000,
		"expected_close_date": now.AddDate(0, 1, 0).Format("2006-01-02"),
		"next_action_at":      now.Add(-24 * time.Hour).Format(time.RFC3339),
		"notes":               "Customer open to quarterly contract.",
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create crm lead: %w", err)
	}
	opportunity, err := client.createModel(ctx, "crm_opportunity", map[string]any{
		"opportunity_number":  "CRM-OPP-" + suffix,
		"title":               opportunityTitle,
		"party_id":            stringValue(party["id"]),
		"party_name":          customerName,
		"contact_id":          stringValue(contact["id"]),
		"owner_user_id":       "user_admin",
		"source_lead_id":      stringValue(lead["id"]),
		"stage":               "proposal",
		"status":              "open",
		"estimated_value":     24000000,
		"expected_close_date": now.AddDate(0, 1, 15).Format("2006-01-02"),
		"next_action_at":      now.Add(-48 * time.Hour).Format(time.RFC3339),
		"notes":               "Proposal draft in review.",
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create crm opportunity: %w", err)
	}
	if _, err := client.createModel(ctx, "crm_activity", map[string]any{
		"activity_type": "meeting",
		"subject":       "Review service recovery and catering proposal",
		"related_kind":  "opportunity",
		"related_id":    stringValue(opportunity["id"]),
		"party_id":      stringValue(party["id"]),
		"party_name":    customerName,
		"owner_user_id": "user_admin",
		"status":        "open",
		"due_at":        now.Add(72 * time.Hour).Format(time.RFC3339),
		"note":          "Prepare both service recovery and upsell summary.",
	}); err != nil {
		return scenarioManifest{}, fmt.Errorf("create crm activity: %w", err)
	}
	healthyLead, err := client.createModel(ctx, "crm_lead", map[string]any{
		"lead_number":         "CRM-LEAD-HEALTHY-" + suffix,
		"title":               "Loyalty bundle expansion",
		"party_id":            stringValue(healthyParty["id"]),
		"party_name":          healthyCustomerName,
		"contact_id":          stringValue(healthyContact["id"]),
		"owner_user_id":       "user_admin",
		"source_channel":      "web",
		"status":              "qualified",
		"rating":              "warm",
		"estimated_value":     12000000,
		"expected_close_date": now.AddDate(0, 2, 0).Format("2006-01-02"),
		"next_action_at":      now.Add(48 * time.Hour).Format(time.RFC3339),
		"notes":               "Healthy account with expansion potential.",
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create healthy crm lead: %w", err)
	}
	if _, err := client.createModel(ctx, "crm_opportunity", map[string]any{
		"opportunity_number":  "CRM-OPP-HEALTHY-" + suffix,
		"title":               healthyOpportunityTitle,
		"party_id":            stringValue(healthyParty["id"]),
		"party_name":          healthyCustomerName,
		"contact_id":          stringValue(healthyContact["id"]),
		"owner_user_id":       "user_admin",
		"source_lead_id":      stringValue(healthyLead["id"]),
		"stage":               "qualified",
		"status":              "open",
		"estimated_value":     12000000,
		"expected_close_date": now.AddDate(0, 2, 10).Format("2006-01-02"),
		"next_action_at":      now.Add(72 * time.Hour).Format(time.RFC3339),
		"notes":               "Healthy account expansion next quarter.",
	}); err != nil {
		return scenarioManifest{}, fmt.Errorf("create healthy crm opportunity: %w", err)
	}

	manifest.Routes = map[string]string{
		"tickets":       "/ui/crm/tickets",
		"queues":        "/ui/crm/queues",
		"leads":         "/ui/crm/leads",
		"opportunities": "/ui/crm/opportunities",
		"customer_360":  "/ui/crm/customers/360?party_id=" + stringValue(party["id"]),
		"dashboard":     "/ui/dashboard",
		"agent":         "/ui/agent/workspace",
	}
	manifest.Entities = map[string]map[string]any{
		"queue_support":    {"code": queueSupportCode, "name": stringValue(mapValue(queueSupport, "values")["name"])},
		"queue_success":    {"code": queueSuccessCode, "name": stringValue(mapValue(queueSuccess, "values")["name"])},
		"customer":         {"party_id": stringValue(party["id"]), "name": customerName, "contact_name": contactName},
		"healthy_customer": {"party_id": stringValue(healthyParty["id"]), "name": healthyCustomerName, "contact_name": healthyContactName},
		"overdue_ticket":   {"id": stringValue(overdueTicket["id"]), "title": overdueTicketTitle},
		"ticket":           {"id": stringValue(currentTicket["id"]), "title": ticketTitle},
		"opportunity":      {"id": stringValue(opportunity["id"]), "title": opportunityTitle},
	}
	manifest.GroundTruth = map[string]any{
		"priority_queue_code":       queueSupportCode,
		"priority_customer_name":    customerName,
		"overdue_ticket_title":      overdueTicketTitle,
		"open_ticket_title":         ticketTitle,
		"stale_opportunity_title":   opportunityTitle,
		"stale_pipeline_value":      24000000,
		"healthy_customer_name":     healthyCustomerName,
		"healthy_opportunity_title": healthyOpportunityTitle,
	}
	manifest.SessionInstructions = strings.TrimSpace(defaultSessionInstructions("crm_service_sales_overview") + fmt.Sprintf(`
- This CRM scenario validates service backlog, customer 360, and sales pipeline reasoning for %s and %s.
- In minimal mode, use CRM skill discovery first and load the matching CRM skill before any business tool call.
- For CRM backlog questions, name the exact queue code and mention the overdue ticket that makes the queue risky.
- For CRM pipeline questions, include the stale opportunity title and the prioritized opportunity value, not only aggregate pipeline totals.
- When asked to show dashboard widgets, preview CRM widgets on the dashboard surface and rely on the returned artifacts instead of substituting plain text widget names.`, customerName, healthyCustomerName))
	manifest.PromptPack = crmServiceSalesOverviewPromptPack(customerName, healthyCustomerName, queueSupportCode, ticketTitle, overdueTicketTitle, opportunityTitle)
	return manifest, nil
}

func seedLeaveToPayrollScenario(ctx context.Context, client *apiClient, baseURL, opencodeCommand string) (scenarioManifest, error) {
	runID, suffix := newRunContext()
	manifest, err := newScenarioManifest(ctx, client, baseURL, opencodeCommand, "leave_to_payroll", "workforce-payroll", runID, suffix)
	if err != nil {
		return scenarioManifest{}, err
	}

	employee, err := client.createModel(ctx, "employee_profile", map[string]any{
		"party_id":          "party_leave_" + suffix,
		"name":              "Maya Putri " + runID,
		"user_id":           "user_admin",
		"employee_code":     "EMP-LV-" + suffix,
		"employment_status": "active",
		"status":            "active",
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create employee profile: %w", err)
	}
	employeeID := stringValue(employee["id"])
	if _, err := client.createModel(ctx, "employee_assignment", map[string]any{
		"employee_id":          employeeID,
		"organization_id":      defaultOrgID,
		"location_id":          defaultLocID,
		"organization_unit_id": "ou_lv_" + suffix,
		"department_id":        "dept_lv_" + suffix,
		"cost_center_id":       "cc_lv_" + suffix,
		"effective_from":       "2099-10-01",
		"status":               "active",
	}); err != nil {
		return scenarioManifest{}, err
	}
	absenceCode, err := client.createModel(ctx, "absence_code", map[string]any{
		"code":                "LV-" + suffix,
		"name":                "Annual Leave",
		"category":            "leave",
		"deduct_from_payroll": true,
		"status":              "active",
	})
	if err != nil {
		return scenarioManifest{}, err
	}
	leavePolicy, err := client.createModel(ctx, "leave_policy", map[string]any{
		"code":             "POL-LV-" + suffix,
		"name":             "Annual Leave Policy",
		"absence_code_id":  stringValue(absenceCode["id"]),
		"paid_leave":       false,
		"requires_balance": true,
		"organization_id":  defaultOrgID,
		"location_id":      defaultLocID,
		"status":           "active",
	})
	if err != nil {
		return scenarioManifest{}, err
	}
	balanceAccount, err := client.createModel(ctx, "leave_balance_account", map[string]any{
		"employee_id":                employeeID,
		"leave_policy_id":            stringValue(leavePolicy["id"]),
		"current_balance_days":       12.0,
		"reserved_days":              0.0,
		"available_days":             10.0,
		"carry_forward_balance_days": 0.0,
		"status":                     "active",
	})
	if err != nil {
		return scenarioManifest{}, err
	}
	leaveRequest, err := client.createModel(ctx, "leave_request", map[string]any{
		"employee_id":     employeeID,
		"organization_id": defaultOrgID,
		"location_id":     defaultLocID,
		"absence_code_id": stringValue(absenceCode["id"]),
		"start_date":      "2099-10-06",
		"end_date":        "2099-10-07",
		"requested_days":  2.0,
		"approval_status": "approved",
		"status":          "active",
	})
	if err != nil {
		return scenarioManifest{}, err
	}
	period, err := client.createModel(ctx, "payroll_period", map[string]any{
		"code":            "PR-" + suffix,
		"name":            "October 2099",
		"organization_id": defaultOrgID,
		"location_id":     defaultLocID,
		"start_date":      "2099-10-01",
		"end_date":        "2099-10-31",
		"pay_date":        "2099-10-31",
		"status":          "open",
	})
	if err != nil {
		return scenarioManifest{}, err
	}
	runDoc, err := createAndApproveDocument(ctx, client, "payroll_run", map[string]any{
		"payroll_period_id": periodID(period),
		"pay_date":          "2099-10-31",
		"currency_code":     "IDR",
		"employee_ids":      []string{employeeID},
		"payroll_lines": []map[string]any{{
			"employee_id":                  employeeID,
			"party_id":                     stringValue(mapValue(employee, "values")["party_id"]),
			"organization_id":              defaultOrgID,
			"location_id":                  defaultLocID,
			"department_id":                "dept_lv_" + suffix,
			"cost_center_id":               "cc_lv_" + suffix,
			"currency_code":                "IDR",
			"leave_days":                   2.0,
			"gross_pay":                    1200.0,
			"employee_deductions_total":    40.0,
			"employee_contributions_total": 0.0,
			"tax_withholding_total":        100.0,
			"reimbursements_total":         0.0,
			"net_pay":                      1060.0,
		}},
		"gross_pay_total":              1200.0,
		"employee_deductions_total":    40.0,
		"employee_contributions_total": 0.0,
		"tax_withholding_total":        100.0,
		"reimbursements_total":         0.0,
		"net_pay_total":                1060.0,
		"employer_cost_total":          1200.0,
		"expense_account_code":         "6200-PAYROLL",
		"payable_account_code":         "2105-PAYROLL",
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create payroll run: %w", err)
	}
	paymentDoc, err := createAndApproveDocument(ctx, client, "payroll_payment", map[string]any{
		"payroll_run_id":      runDoc.ID,
		"payroll_period_id":   periodID(period),
		"employee_id":         employeeID,
		"party_id":            stringValue(mapValue(employee, "values")["party_id"]),
		"payment_date":        "2099-10-31",
		"currency_code":       "IDR",
		"treasury_account_id": "treasury_payroll_" + suffix,
		"payment_method_code": "BANK",
		"net_pay":             1060.0,
		"total_amount":        1060.0,
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create payroll payment: %w", err)
	}

	manifest.Entities = map[string]map[string]any{
		"employee": {"id": employeeID, "name": stringValue(mapValue(employee, "values")["name"]), "code": stringValue(mapValue(employee, "values")["employee_code"])},
	}
	manifest.Documents = map[string]documentFacts{
		"payroll_run":     runDoc,
		"payroll_payment": paymentDoc,
	}
	manifest.GroundTruth = map[string]any{
		"approved_leave_days":     2.0,
		"remaining_leave_balance": 10.0,
		"payroll_deduction":       40.0,
		"net_pay":                 1060.0,
		"payment_status":          paymentDoc.Status,
		"leave_request_id":        stringValue(leaveRequest["id"]),
		"balance_account_id":      stringValue(balanceAccount["id"]),
	}
	manifest.PromptPack = leaveToPayrollPromptPack(stringValue(mapValue(employee, "values")["name"]))
	return manifest, nil
}

func seedPayrollRemittanceScenario(ctx context.Context, client *apiClient, baseURL, opencodeCommand string) (scenarioManifest, error) {
	runID, suffix := newRunContext()
	manifest, err := newScenarioManifest(ctx, client, baseURL, opencodeCommand, "payroll_remittance", "payroll-treasury", runID, suffix)
	if err != nil {
		return scenarioManifest{}, err
	}

	authority, err := client.createModel(ctx, "remittance_authority", map[string]any{
		"code":                        "AUTH-" + suffix,
		"name":                        "Tax Authority " + runID,
		"organization_id":             defaultOrgID,
		"location_id":                 defaultLocID,
		"default_currency_code":       "IDR",
		"default_treasury_account_id": "treasury_remit_" + suffix,
		"payment_method_code":         "BANK",
		"status":                      "active",
	})
	if err != nil {
		return scenarioManifest{}, err
	}
	withholding, err := client.createModel(ctx, "remittance_obligation_type", map[string]any{
		"remittance_authority_id": stringValue(authority["id"]),
		"code":                    "WHT-" + suffix,
		"name":                    "Withholding",
		"obligation_class":        "withholding",
		"liability_account_code":  "2310-WHT",
		"status":                  "active",
	})
	if err != nil {
		return scenarioManifest{}, err
	}
	liabilityDoc, err := createAndApproveDocument(ctx, client, "payroll_remittance_liability", map[string]any{
		"remittance_authority_id":       stringValue(authority["id"]),
		"remittance_obligation_type_id": stringValue(withholding["id"]),
		"organization_id":               defaultOrgID,
		"location_id":                   defaultLocID,
		"currency_code":                 "IDR",
		"treasury_account_id":           "treasury_remit_" + suffix,
		"payment_method_code":           "BANK",
		"liability_account_code":        "2310-WHT",
		"due_date":                      "2099-11-07",
		"employee_withholding_amount":   120.0,
		"total_amount":                  120.0,
		"open_amount":                   120.0,
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create remittance liability: %w", err)
	}
	batchDoc, err := createAndApproveDocument(ctx, client, "payroll_remittance_batch", map[string]any{
		"liability_ids":       []string{liabilityDoc.ID},
		"organization_id":     defaultOrgID,
		"location_id":         defaultLocID,
		"currency_code":       "IDR",
		"treasury_account_id": "treasury_remit_" + suffix,
		"payment_method_code": "BANK",
		"payment_date":        "2099-11-05",
		"total_amount":        120.0,
		"open_amount":         120.0,
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create remittance batch: %w", err)
	}
	paymentDoc, err := createAndApproveDocument(ctx, client, "payroll_remittance_payment", map[string]any{
		"payroll_remittance_batch_id": batchDoc.ID,
		"organization_id":             defaultOrgID,
		"location_id":                 defaultLocID,
		"currency_code":               "IDR",
		"treasury_account_id":         "treasury_remit_" + suffix,
		"payment_method_code":         "BANK",
		"payment_date":                "2099-11-05",
		"amount_paid":                 120.0,
		"total_amount":                120.0,
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create remittance payment: %w", err)
	}

	manifest.Entities = map[string]map[string]any{
		"authority": {"id": stringValue(authority["id"]), "name": stringValue(mapValue(authority, "values")["name"]), "code": stringValue(mapValue(authority, "values")["code"])},
	}
	manifest.Documents = map[string]documentFacts{
		"payroll_remittance_liability": liabilityDoc,
		"payroll_remittance_batch":     batchDoc,
		"payroll_remittance_payment":   paymentDoc,
	}
	manifest.GroundTruth = map[string]any{
		"liability_total":       120.0,
		"authority_name":        stringValue(mapValue(authority, "values")["name"]),
		"obligation_class":      "withholding",
		"batch_status":          batchDoc.Status,
		"payment_status":        paymentDoc.Status,
		"remaining_open_amount": 0.0,
	}
	manifest.PromptPack = payrollRemittancePromptPack(stringValue(mapValue(authority, "values")["name"]))
	return manifest, nil
}

func seedProductionCostingScenario(ctx context.Context, client *apiClient, baseURL, opencodeCommand string) (scenarioManifest, error) {
	runID, suffix := newRunContext()
	manifest, err := newScenarioManifest(ctx, client, baseURL, opencodeCommand, "production_costing", "production-inventory", runID, suffix)
	if err != nil {
		return scenarioManifest{}, err
	}

	finishedCode := "FG-" + suffix
	componentA := "COMP-A-" + suffix
	componentB := "COMP-B-" + suffix
	for _, item := range []map[string]any{
		{"sku": finishedCode, "name": "Finished Kit " + runID, "uom_code": "ea", "inventory_enabled": true, "status": "active"},
		{"sku": componentA, "name": "Component A " + runID, "uom_code": "ea", "inventory_enabled": true, "status": "active"},
		{"sku": componentB, "name": "Component B " + runID, "uom_code": "ea", "inventory_enabled": true, "inventory_tracking_mode": "batch", "status": "active"},
	} {
		if _, err := client.createModel(ctx, "commercial_item", item); err != nil {
			return scenarioManifest{}, err
		}
	}
	bom, err := client.createModel(ctx, "production_bom", map[string]any{
		"code":                 "BOM-" + suffix,
		"name":                 "Proof BOM " + runID,
		"finished_item_code":   finishedCode,
		"default_version_code": "v1",
		"status":               "active",
	})
	if err != nil {
		return scenarioManifest{}, err
	}
	if _, err := client.createModel(ctx, "production_bom_version", map[string]any{
		"bom_id":         stringValue(bom["id"]),
		"bom_code":       stringValue(mapValue(bom, "values")["code"]),
		"version_code":   "v1",
		"yield_quantity": 1.0,
		"is_active":      true,
		"status":         "active",
		"lines": []map[string]any{
			{"component_item_code": componentA, "quantity_per_unit": 2.0, "uom_code": "ea", "warehouse_code": "MAIN"},
			{"component_item_code": componentB, "quantity_per_unit": 1.0, "uom_code": "ea", "warehouse_code": "MAIN", "batch_code": "BATCH-" + suffix},
		},
	}); err != nil {
		return scenarioManifest{}, err
	}
	for _, seed := range []map[string]any{
		{"item_code": componentA, "warehouse_code": "MAIN", "quantity": 20.0, "unit_cost": 5.0},
		{"item_code": componentB, "warehouse_code": "MAIN", "batch_code": "BATCH-" + suffix, "quantity": 10.0, "unit_cost": 10.0},
	} {
		if _, err := createAndApproveDocument(ctx, client, "stock_receipt", map[string]any{
			"receipt_date":   "2099-09-28",
			"warehouse_code": seed["warehouse_code"],
			"lines": []map[string]any{{
				"item_code":      seed["item_code"],
				"warehouse_code": seed["warehouse_code"],
				"batch_code":     seed["batch_code"],
				"uom_code":       "ea",
				"quantity":       seed["quantity"],
				"unit_cost":      seed["unit_cost"],
			}},
		}); err != nil {
			return scenarioManifest{}, err
		}
	}
	productionOrder, err := createAndApproveDocument(ctx, client, "production_order", map[string]any{
		"finished_item_code": finishedCode,
		"production_pattern": "make_to_stock",
		"warehouse_code":     "MAIN",
		"planned_quantity":   3.0,
		"stages":             []map[string]any{{"code": "mix", "name": "Mix", "required": true, "status": "completed"}, {"code": "pack", "name": "Pack", "required": true, "status": "completed"}},
		"lines": []map[string]any{
			{"component_item_code": componentA, "item_code": componentA, "quantity": 6.0, "uom_code": "ea", "warehouse_code": "MAIN", "unit_cost": 5.0},
			{"component_item_code": componentB, "item_code": componentB, "quantity": 3.0, "uom_code": "ea", "warehouse_code": "MAIN", "batch_code": "BATCH-" + suffix, "unit_cost": 10.0},
		},
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create production order: %w", err)
	}
	issueDoc, err := createAndApproveDocument(ctx, client, "production_issue", map[string]any{
		"source_production_order_id": productionOrder.ID,
		"warehouse_code":             "MAIN",
		"lines": []map[string]any{
			{"item_code": componentA, "quantity": 6.0, "uom_code": "ea", "warehouse_code": "MAIN", "unit_cost": 5.0},
			{"item_code": componentB, "quantity": 3.0, "uom_code": "ea", "warehouse_code": "MAIN", "batch_code": "BATCH-" + suffix, "unit_cost": 10.0},
		},
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create production issue: %w", err)
	}
	outputDoc, err := createAndApproveDocument(ctx, client, "production_output", map[string]any{
		"source_production_order_id": productionOrder.ID,
		"finished_item_code":         finishedCode,
		"warehouse_code":             "MAIN",
		"output_quantity":            3.0,
		"production_lot_code":        "LOT-" + suffix,
		"total_production_cost":      60.0,
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create production output: %w", err)
	}

	manifest.Entities = map[string]map[string]any{
		"finished_item": {"code": finishedCode, "name": "Finished Kit " + runID},
	}
	manifest.Documents = map[string]documentFacts{
		"production_order":  productionOrder,
		"production_issue":  issueDoc,
		"production_output": outputDoc,
	}
	manifest.GroundTruth = map[string]any{
		"consumed_component_a":     6.0,
		"consumed_component_b":     3.0,
		"output_quantity":          3.0,
		"total_production_cost":    60.0,
		"production_output_status": outputDoc.Status,
	}
	manifest.PromptPack = productionCostingPromptPack(finishedCode)
	return manifest, nil
}

func newRunContext() (string, string) {
	now := time.Now().UTC()
	runID := now.Format("20060102-150405")
	suffix := fmt.Sprintf("%s%09d", strings.ReplaceAll(runID, "-", ""), now.Nanosecond())
	return runID, suffix
}

func createScenarioWorkingDir(scenario, runID string) (string, error) {
	base := filepath.Join(os.TempDir(), "orbyte-agentproof", strings.TrimSpace(scenario), strings.TrimSpace(runID))
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", err
	}
	return base, nil
}

func createOpencodeHome(suffix string, snippet map[string]any) (string, error) {
	home := filepath.Join(os.TempDir(), "orbyte-agentproof", "opencode", strings.ToLower(strings.TrimSpace(suffix)))
	configDir := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return "", err
	}
	configPath := filepath.Join(configDir, "opencode.json")
	data, err := json.MarshalIndent(snippet, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(configPath, append(data, '\n'), 0o644); err != nil {
		return "", err
	}
	return home, nil
}

func newScenarioManifest(ctx context.Context, client *apiClient, baseURL, opencodeCommand, scenario, domainBundle, runID, suffix string) (scenarioManifest, error) {
	servicePrincipal, opencodeConfig, err := configureAgentAccess(ctx, client, baseURL, opencodeCommand, suffix)
	if err != nil {
		return scenarioManifest{}, err
	}
	workingDir, err := createScenarioWorkingDir(scenario, runID)
	if err != nil {
		return scenarioManifest{}, err
	}
	return scenarioManifest{
		Version:             "2026-04-02",
		Scenario:            scenario,
		DomainBundle:        domainBundle,
		RunID:               runID,
		GeneratedAt:         time.Now().UTC(),
		BaseURL:             baseURL,
		WorkingDir:          workingDir,
		AgentRoutePath:      "/ui/agent/workspace",
		SessionTitleHint:    "agentproof-" + scenario + "-" + runID,
		SessionInstructions: defaultSessionInstructions(scenario),
		ServicePrincipal:    servicePrincipal,
		OpencodeConfig:      opencodeConfig,
		Entities:            map[string]map[string]any{},
		Documents:           map[string]documentFacts{},
		GroundTruth:         map[string]any{},
	}, nil
}

func defaultSessionInstructions(scenario string) string {
	label := strings.ReplaceAll(strings.TrimSpace(scenario), "_", " ")
	return strings.TrimSpace(fmt.Sprintf(`You are validating the Orbyte business system through its connected MCP tools.

Rules for this session:
- Ignore the local working directory and do not use local files as evidence. The working directory may be empty on purpose.
- Use the connected Orbyte MCP tools. Prefer tools whose names start with orbyte_.
- If a question asks for a status, amount, quantity, or open-work count, verify it from Orbyte data before answering.
- If you cannot find evidence, say which Orbyte data you checked rather than guessing.
- Briefly mention the retrieved document or record type in your answer.
- This is the %s scenario.`, label))
}

func configureAgentAccess(ctx context.Context, client *apiClient, baseURL, opencodeCommand, suffix string) (servicePrincipalOutput, opencodeConfigOutput, error) {
	servicePrincipalID := shared.NewID("sp")
	servicePrincipalKey := "agentproof_mcp_" + strings.ToLower(suffix)
	servicePrincipal, err := client.createServicePrincipal(ctx, servicePrincipalID, servicePrincipalKey, agentproofMCPOperations())
	if err != nil {
		return servicePrincipalOutput{}, opencodeConfigOutput{}, fmt.Errorf("create service principal: %w", err)
	}
	token, err := client.issueServicePrincipalToken(ctx, servicePrincipal.ID, 24*60*60)
	if err != nil {
		return servicePrincipalOutput{}, opencodeConfigOutput{}, fmt.Errorf("issue service principal token: %w", err)
	}
	servicePrincipal.Token = token
	serverName := "orbyte-agentproof-" + strings.ToLower(suffix)
	mcpURL := strings.TrimRight(baseURL, "/") + "/mcp"
	snippet := map[string]any{
		"$schema": "https://opencode.ai/config.json",
		"permission": map[string]any{
			"*": "allow",
		},
		"mcp": map[string]any{
			serverName: map[string]any{
				"type":    "remote",
				"url":     mcpURL,
				"enabled": true,
				"timeout": 120000,
				"headers": map[string]any{
					"Authorization": "Bearer " + token,
				},
			},
		},
	}
	opencodeHome, err := createOpencodeHome(suffix, snippet)
	if err != nil {
		return servicePrincipalOutput{}, opencodeConfigOutput{}, fmt.Errorf("create opencode config: %w", err)
	}
	acpProvider := map[string]any{
		"key":         "opencode",
		"name":        "OpenCode ACP",
		"description": "Agent proof ACP provider",
		"command":     opencodeCommand,
		"args":        []string{"acp", "--print-logs"},
		"transport":   "jsonl",
		"mcp_servers": []map[string]any{{
			"name":    serverName,
			"type":    "http",
			"url":     mcpURL,
			"enabled": true,
			"timeout": 120000,
			"headers": []map[string]any{{
				"name":  "Authorization",
				"value": "Bearer " + token,
			}},
		}},
		"env": map[string]any{
			"HOME":            opencodeHome,
			"XDG_CONFIG_HOME": filepath.Join(opencodeHome, ".config"),
		},
	}
	if err := client.putConfig(ctx, "platform.acp", map[string]any{
		"enabled":        true,
		"providers_json": mustJSONString([]map[string]any{acpProvider}),
	}); err != nil {
		return servicePrincipalOutput{}, opencodeConfigOutput{}, fmt.Errorf("enable acp: %w", err)
	}
	if err := client.putConfig(ctx, "platform.mcp", map[string]any{
		"enabled":                            true,
		"exposure_mode":                      "full",
		"governance_enabled":                 true,
		"default_action_mode":                "draft_only",
		"tool_states_json":                   "{}",
		"blocked_action_classes_json":        "[]",
		"blocked_tool_keys_json":             "[]",
		"blocked_document_types_json":        "[]",
		"allowed_submit_document_types_json": "[]",
		"domain_policy_overrides_json":       "{}",
		"playbooks_json":                     defaultMCPPlaybooksJSON(),
	}); err != nil {
		return servicePrincipalOutput{}, opencodeConfigOutput{}, fmt.Errorf("enable mcp: %w", err)
	}
	return servicePrincipal, opencodeConfigOutput{
		ServerName: serverName,
		URL:        mcpURL,
		Bearer:     token,
		Snippet:    snippet,
		Provider:   acpProvider,
	}, nil
}

func defaultMCPPlaybooksJSON() string {
	return mustJSONString([]map[string]any{
		{
			"id":          "retail_recovery_dashboard",
			"name":        "Retail Recovery Dashboard",
			"description": "Diagnose underperforming branches, compare the strongest branch, surface POS patterns, and preview dashboard widgets that explain the gap.",
			"domains":     []string{"analytics", "retail", "promotion"},
			"labels":      []string{"dashboard", "recovery", "showcase"},
			"keywords":    []string{"store", "branch", "underperforming", "widgets", "sales pattern"},
			"use_when":    "The user asks why some branches are underperforming and wants dashboard evidence or a dashboard-backed recovery plan.",
			"workflow_steps": []map[string]any{
				{"step": "discover_dashboard_widgets", "title": "Discover Dashboard Widgets", "tool_id": "analytics.dashboard.widget_catalog", "required": true, "description": "Find dashboard widgets that explain branch performance, branch mix, target attainment, and daily sales trend.", "output": "Candidate dashboard widget keys and titles."},
				{"step": "preview_dashboard_widgets", "title": "Preview Recovery Widgets", "tool_id": "analytics.dashboard.widgets.preview", "required": true, "description": "Preview the exact retail recovery widgets for surface dashboard.", "arguments": map[string]any{"surface": "dashboard", "widget_keys": []string{"analytics.demo.sales.target_attainment", "analytics.demo.sales.branch_mix", "analytics.demo.sales.daily_trend"}}, "output": "Dashboard widget/session artifacts for the relevant widgets, plus titles the final answer should reference."},
				{"step": "name_branch_gaps", "title": "Name Branch Gaps", "required": true, "description": "Name the strongest benchmark branch plus each underperforming branch explicitly in the final answer."},
				{"step": "summarize_pos_pattern", "title": "Summarize POS Pattern", "tool_id": "pos_core.sales.strategy.summary", "required": true, "description": "Identify the strongest basket or product pairing and target customer segment.", "output": "Recommended bundle products and target segment."},
				{"step": "check_promotion_replacement", "title": "Check Promotion Replacement", "tool_id": "promotion_core.performance.summary", "required": false, "when": "Use when the user asks for a recovery plan or campaign replacement.", "description": "Identify weak promotion performance and replacement candidates.", "output": "Promotion that should be replaced and why."},
				{"step": "return_widget_artifacts", "title": "Return Widget Artifacts", "required": false, "when": "Use when widgets were requested.", "description": "Rely on the returned session artifacts for rendering and do not answer a widget request with text-only widget names."},
			},
			"tool_inventory": []string{
				"analytics.dashboard.widget_catalog",
				"analytics.dashboard.widgets.preview",
				"pos_core.sales.strategy.summary",
				"promotion_core.performance.summary",
			},
			"required_final_facts": []string{
				"Strongest benchmark branch name.",
				"Each underperforming branch name.",
				"POS pattern to lean into, including Espresso Double and Butter Croissant.",
				"Target segment, especially gold members.",
				"Whether Beans Boost should be replaced when planning recovery.",
			},
			"required_artifacts": []string{
				"dashboard widget/session artifact for analytics.demo.sales.target_attainment",
				"dashboard widget/session artifact for analytics.demo.sales.branch_mix",
				"dashboard widget/session artifact for analytics.demo.sales.daily_trend",
			},
			"guardrails": []string{
				"Do not create or submit promotion documents unless the user explicitly asks to execute.",
				"Do not invent branch names when dashboard or POS data is insufficient; call the relevant tools instead.",
				"If the user asks to show dashboard widgets, do not finish with text-only widget names; ensure the widget/session artifacts were actually produced.",
			},
			"success_checks": []string{
				"Final answer names Loc Demo East as benchmark when it is the strongest branch.",
				"Final answer names Loc Demo Central and Loc Demo West as underperforming branches when those are the weak branches.",
				"Insight answer includes dashboard artifacts for target attainment, daily trend, and branch mix when widgets are requested.",
				"Plan answer says Beans Boost should be replaced by the breakfast bundle campaign.",
			},
			"pitfalls": []string{
				"Do not say only that the gold segment is strongest; the user asked for branch comparison.",
				"Do not summarize widgets as text-only names without actually producing the preview artifacts.",
			},
			"examples": []string{
				"Which branches are underperforming compared with the strongest branch, and what dashboard widgets explain why?",
			},
		},
		{
			"id":          "inventory_replenishment_execute",
			"name":        "Inventory Replenishment Execute",
			"description": "Review inventory replenishment signals, inspect plan recommendations, and prepare or execute the replenishment follow-up flow.",
			"domains":     []string{"inventory", "planning", "procurement"},
			"labels":      []string{"replenishment", "planning", "execute"},
			"keywords":    []string{"warehouse", "replenishment", "shortage", "vendor", "purchase"},
			"use_when":    "The user asks for replenishment analysis or wants the system to execute a replenishment workflow.",
			"workflow_steps": []map[string]any{
				{"step": "inspect_replenishment_risk", "title": "Inspect Replenishment Risk", "tool_id": "planning_core.replenishment.insight.summary", "required": true, "description": "Identify at-risk and healthy items for the requested warehouse.", "output": "At-risk item names and healthy skip items."},
				{"step": "summarize_replenishment_plan", "title": "Summarize Replenishment Plan", "tool_id": "planning_core.replenishment.plan.summary", "required": true, "description": "Get exact recommended quantities, warehouse, and vendor grouping.", "output": "Recommended item quantities and vendor."},
				{"step": "gate_execution", "title": "Gate Execution", "required": true, "description": "Only create or execute follow-up documents when the user explicitly asks to proceed."},
				{"step": "create_purchase_request_draft", "title": "Create Purchase Request Draft", "tool_id": "planning_core.purchase_requests.draft.create", "required": false, "when": "Only when the user explicitly asks to create drafts or execute the plan.", "description": "Create draft purchase request documents for recommended replenishment lines.", "output": "Draft document ids and review links."},
			},
			"tool_inventory": []string{
				"planning_core.replenishment.insight.summary",
				"planning_core.replenishment.plan.summary",
				"planning_core.purchase_requests.draft.create",
			},
			"required_final_facts": []string{
				"At-risk item names.",
				"Healthy items to skip.",
				"Exact recommended quantities.",
				"Recommended vendor.",
			},
			"required_draft_outputs": []string{
				"purchase_request draft ids",
				"vendor name",
				"item names and quantities",
				"review links when available",
			},
			"guardrails": []string{
				"Do not submit purchase requests during validation or when the user says draft only.",
				"Do not create drafts unless the user explicitly asks to create or execute.",
				"Do not include healthy skip items in purchase request drafts.",
			},
			"success_checks": []string{
				"Plan output includes Cold Brew quantity 20 and Oat Milk quantity 16 when those are the current recommendations.",
				"Draft output names North Roast as vendor when it is the selected vendor.",
				"Execute output says the documents are draft and not submitted.",
			},
			"pitfalls": []string{
				"Do not reuse an old plan if the warehouse code changed; call the replenishment tools for the current warehouse.",
				"Do not treat zero recommendations as success when the tool output includes at-risk lines.",
			},
			"examples": []string{
				"Review the inventory replenishment situation and prepare the next steps for execution.",
			},
		},
		{
			"id":          "crm_service_interest_lead_capture",
			"name":        "CRM Service Interest Lead Capture",
			"description": "Capture a sales lead when a customer service interaction reveals product or commercial interest, while checking for duplicates before creating anything.",
			"domains":     []string{"crm", "service", "sales"},
			"labels":      []string{"crm", "lead-capture", "handoff", "service-to-sales"},
			"keywords":    []string{"customer interest", "service agent handoff", "create lead", "sales lead", "product interest", "upsell", "cross-sell"},
			"use_when":    "The user or connected service agent wants to turn a customer service conversation into a CRM sales lead because the customer showed interest in a product, offer, package, or broader commercial follow-up.",
			"workflow_steps": []map[string]any{
				{"step": "resolve_customer_context", "title": "Resolve Customer Context", "tool_id": "crm.customer.summary", "required": true, "description": "Resolve the named customer or party first so the lead is attached to the correct account context. Execute this step first and wait for the resolved customer context before any duplicate checks.", "output": "Customer account, profile, service context, and party id."},
				{"step": "check_existing_leads", "title": "Check Existing Leads", "tool_id": "crm.lead.search", "required": true, "description": "After the customer context is resolved, search for open or recent leads for the same customer before creating a new one. Execute this after resolve_customer_context, not in parallel with it.", "output": "Existing lead matches and ownership context."},
				{"step": "check_existing_opportunities", "title": "Check Existing Opportunities", "tool_id": "crm.opportunity.search", "required": true, "description": "After checking leads, search for open opportunities for the same customer so the agent does not duplicate active pipeline. Execute this after resolve_customer_context and the lead check, not as an initial parallel batch.", "output": "Existing opportunity matches and current pipeline stage."},
				{"step": "decide_create_or_update", "title": "Decide Create Or Update", "required": true, "description": "Only after the customer, lead, and opportunity checks are completed, decide whether to reuse an existing record or create a new lead."},
				{"step": "create_lead", "title": "Create Lead", "tool_id": "crm.lead.create", "required": false, "when": "Use only when the customer is resolved, no matching active lead or opportunity already covers the same product interest, and the user or automation policy explicitly wants lead creation.", "description": "Create a CRM lead that captures the product interest, source context, next action, and owner.", "output": "Created lead id and summary of captured interest."},
				{"step": "log_follow_up_activity", "title": "Log Follow-up Activity", "tool_id": "crm.activity.create", "required": false, "when": "Use when the workflow needs an internal follow-up task or handoff activity after creating or updating the lead.", "description": "Create a CRM activity so sales can follow up on the detected interest.", "output": "Activity id and next action context."},
			},
			"tool_inventory": []string{
				"crm.customer.summary",
				"crm.lead.search",
				"crm.opportunity.search",
				"crm.lead.create",
				"crm.activity.create",
			},
			"required_final_facts": []string{
				"Resolved customer or party context.",
				"Detected product or commercial interest.",
				"Whether an existing lead or opportunity already covers that interest.",
				"Whether the workflow should create a new lead or update/reference an existing record.",
			},
			"required_draft_outputs": []string{
				"lead id when a new lead is created",
				"activity id when an internal follow-up activity is created",
			},
			"guardrails": []string{
				"Do not create a duplicate lead when an active lead or open opportunity already covers the same customer interest.",
				"Do not create a lead until the customer or party is resolved to the correct CRM account context.",
				"Do not assume every support conversation should create a lead; the customer must show explicit product or commercial interest.",
				"Do not parallelize the initial customer resolution and duplicate-check steps; execute them in order so each step can use the previous result.",
			},
			"success_checks": []string{
				"Final answer clearly states whether a new lead was created or an existing lead/opportunity should be reused.",
				"When a lead is created, the answer includes the created lead id and the interest context that justified the handoff.",
			},
			"pitfalls": []string{
				"Do not create a lead from vague satisfaction signals alone; the customer must show buying or expansion interest.",
				"Do not skip duplicate checks just because the service issue and sales context look urgent.",
				"Do not start customer summary, lead search, and opportunity search as one parallel batch; that can stall the session and weakens duplicate decisions.",
			},
			"examples": []string{
				"The customer asked about a catering package during a support call. Should we create a CRM lead?",
				"A support agent detected interest in a premium product. Capture it as a CRM lead if nothing active already exists.",
			},
		},
		{
			"id":          "crm_customer_complaint_ticket_intake",
			"name":        "CRM Customer Complaint Ticket Intake",
			"description": "Capture a customer complaint as a CRM service ticket after resolving the customer, checking for duplicates, and routing it to the right queue and severity.",
			"domains":     []string{"crm", "service"},
			"labels":      []string{"crm", "ticket-intake", "complaint", "service-desk"},
			"keywords":    []string{"customer complaint", "create ticket", "support case", "issue intake", "service ticket", "complain ticket"},
			"use_when":    "The user or connected customer service agent wants to turn a customer complaint, service issue, refund problem, damaged order, or support escalation into a CRM ticket.",
			"workflow_steps": []map[string]any{
				{"step": "resolve_customer_context", "title": "Resolve Customer Context", "tool_id": "crm.customer.summary", "required": true, "description": "Resolve the named customer or party first so the complaint is attached to the correct CRM account context. Execute this step first and wait for the resolved customer context before any duplicate checks.", "output": "Customer account, profile, and party id."},
				{"step": "check_existing_tickets", "title": "Check Existing Tickets", "tool_id": "crm.ticket.search", "required": true, "description": "After the customer context is resolved, search for existing open tickets for the same customer and complaint pattern before creating a new ticket. Execute this after resolve_customer_context, not in parallel with it.", "output": "Possible duplicate tickets, current queue, and ticket status."},
				{"step": "decide_queue_priority_and_severity", "title": "Decide Queue, Priority, And Severity", "required": true, "description": "Only after the customer and duplicate-ticket checks are completed, choose the right queue, priority, severity, and issue category based on the complaint type and urgency."},
				{"step": "create_ticket", "title": "Create Ticket", "tool_id": "crm.ticket.create", "required": false, "when": "Use only when the customer is resolved, no matching open ticket already covers the same complaint, and the user or automation policy explicitly wants ticket creation.", "description": "Create the CRM complaint ticket with title, description, customer, queue, priority, severity, and source channel.", "output": "Created ticket id and queue assignment context."},
				{"step": "assign_ticket", "title": "Assign Ticket", "tool_id": "crm.ticket.assign", "required": false, "when": "Use when the workflow wants to route the complaint immediately to a named agent or owner after creation.", "description": "Assign or reassign the complaint ticket to the correct owner.", "output": "Updated ticket assignee and routing note."},
				{"step": "capture_original_complaint_note", "title": "Capture Original Complaint Note", "tool_id": "crm.ticket.comment.create", "required": false, "when": "Use when the original complaint detail, transcript, or escalation note should be preserved as a ticket comment.", "description": "Attach the original complaint narrative or escalation note to the ticket.", "output": "Ticket comment id and preserved complaint context."},
			},
			"tool_inventory": []string{
				"crm.customer.summary",
				"crm.ticket.search",
				"crm.ticket.create",
				"crm.ticket.assign",
				"crm.ticket.comment.create",
			},
			"required_final_facts": []string{
				"Resolved customer or party context.",
				"Complaint summary or issue type.",
				"Whether an existing open ticket already covers the complaint.",
				"Chosen queue, priority, and severity when creating a new ticket.",
				"Whether a new ticket should be created or an existing ticket should be reused/updated.",
			},
			"required_draft_outputs": []string{
				"ticket id when a new complaint ticket is created",
				"assigned owner when the ticket is routed immediately",
				"comment id when the original complaint note is attached",
			},
			"guardrails": []string{
				"Do not create a duplicate complaint ticket when an active open ticket already covers the same customer issue.",
				"Do not create a complaint ticket until the customer or party is resolved to the correct CRM account context.",
				"Do not guess queue, priority, or severity from vague wording; when urgency is unclear, say that the classification needs confirmation or use the safest supported default.",
				"Do not parallelize the initial customer resolution and duplicate-ticket check; execute them in order so the duplicate check uses the resolved customer context.",
			},
			"success_checks": []string{
				"Final answer clearly states whether a new complaint ticket was created or an existing one should be reused.",
				"When a new ticket is created, the answer includes the ticket id plus the selected queue and severity context.",
			},
			"pitfalls": []string{
				"Do not treat every complaint as a high-severity escalation without evidence from the complaint context.",
				"Do not skip duplicate-ticket checks just because the customer sounds frustrated.",
				"Do not start customer summary and ticket search as one parallel batch; that can stall the session and weaken duplicate detection.",
			},
			"examples": []string{
				"The customer called to complain about a damaged shipment. Create a CRM complaint ticket if nothing open already covers it.",
				"A support agent needs to log a refund complaint from a customer. Capture it as the right CRM ticket and assign it if needed.",
			},
		},
		{
			"id":          "crm_service_backlog_triage",
			"name":        "CRM Service Backlog Triage",
			"description": "Review CRM service backlog, identify the highest-risk customer or queue, and show the best service widgets when the user asks for dashboard evidence.",
			"domains":     []string{"crm", "service"},
			"labels":      []string{"crm", "service", "backlog", "sla"},
			"keywords":    []string{"crm backlog", "ticket backlog", "overdue tickets", "sla", "queue health"},
			"use_when":    "The user asks about CRM service backlog, ticket pressure, overdue tickets, queue health, or asks for widget evidence for service operations.",
			"workflow_steps": []map[string]any{
				{"step": "summarize_service_backlog", "title": "Summarize Service Backlog", "tool_id": "crm.ticket.summary", "required": true, "description": "Load CRM service backlog, overdue tickets, and queue health.", "output": "Open tickets, overdue tickets, queue backlog, and SLA context."},
				{"step": "discover_crm_widgets", "title": "Discover CRM Widgets", "tool_id": "analytics.dashboard.widget_catalog", "required": false, "when": "Use when the user asks for widgets or dashboard evidence.", "arguments": map[string]any{"surface": "dashboard"}, "description": "Discover CRM dashboard widgets for service backlog.", "output": "Relevant CRM service widget keys."},
				{"step": "preview_crm_widgets", "title": "Preview CRM Widgets", "tool_id": "analytics.dashboard.widgets.preview", "required": false, "when": "Use when the user asks for widgets or dashboard evidence.", "arguments": map[string]any{"surface": "dashboard", "widget_keys": []string{"crm.ticketing.open_tickets", "crm.ticketing.overdue_tickets", "crm.ticketing.queue_backlog"}}, "description": "Preview the CRM service widgets that explain the backlog.", "output": "Dashboard widget artifacts for CRM service backlog."},
				{"step": "name_risk_target", "title": "Name Risk Target", "required": true, "description": "Name the most at-risk queue or customer explicitly in the final answer."},
			},
			"tool_inventory": []string{
				"crm.ticket.summary",
				"analytics.dashboard.widget_catalog",
				"analytics.dashboard.widgets.preview",
			},
			"required_final_facts": []string{
				"Most pressured queue or backlog area.",
				"Most at-risk customer or open-ticket cluster when identifiable.",
				"Overdue ticket or SLA risk context.",
			},
			"required_artifacts": []string{
				"dashboard widget/session artifact for crm.ticketing.open_tickets when widgets are requested",
				"dashboard widget/session artifact for crm.ticketing.overdue_tickets when widgets are requested",
				"dashboard widget/session artifact for crm.ticketing.queue_backlog when widgets are requested",
			},
			"guardrails": []string{
				"Do not answer a service backlog question from generic business search when crm.ticket.summary is available.",
				"Do not claim dashboard evidence unless widget preview artifacts were actually returned.",
			},
			"success_checks": []string{
				"Final answer names the priority queue or at-risk customer explicitly.",
				"Widget requests produce CRM service widget artifacts instead of text-only widget names.",
			},
			"pitfalls": []string{
				"Do not stop at queue counts without identifying the customer or overdue pressure point when the user asks who needs attention most.",
			},
		},
		{
			"id":          "crm_customer_360_review",
			"name":        "CRM Customer 360 Review",
			"description": "Load a single customer’s CRM 360 summary or timeline and explain how service issues, activities, and opportunities connect.",
			"domains":     []string{"crm", "service", "sales"},
			"labels":      []string{"crm", "customer-360", "timeline"},
			"keywords":    []string{"customer 360", "customer timeline", "crm customer", "service issue", "opportunity link"},
			"use_when":    "The user names one customer or asks for CRM customer 360 context, history, timeline, or service-to-sales linkage.",
			"workflow_steps": []map[string]any{
				{"step": "load_customer_summary", "title": "Load Customer Summary", "tool_id": "crm.customer.summary", "required": true, "description": "Load customer 360 summary for the named party.", "output": "Customer service, profile, and opportunity summary."},
				{"step": "load_customer_timeline", "title": "Load Customer Timeline", "tool_id": "crm.customer.timeline", "required": false, "when": "Use when the user asks for sequence, history, or follow-up details.", "description": "Load timeline entries for the named customer.", "output": "Activity chronology across service and sales."},
				{"step": "connect_service_and_sales", "title": "Connect Service And Sales", "required": true, "description": "Explain how the current ticket, customer health, and active opportunity relate to each other."},
			},
			"tool_inventory": []string{
				"crm.customer.summary",
				"crm.customer.timeline",
			},
			"required_final_facts": []string{
				"Customer name.",
				"Open ticket or service issue summary.",
				"Active opportunity and stage when present.",
				"Clear explanation of how service and sales context connect.",
			},
			"guardrails": []string{
				"Do not answer a named-customer question with only aggregate backlog metrics.",
				"Do not use crm.customer.health as the sole source when the user explicitly asked for one customer’s 360 context.",
			},
			"success_checks": []string{
				"Final answer names the customer and the linked ticket/opportunity records or summaries.",
			},
			"pitfalls": []string{
				"Do not ignore the active opportunity when the ticket and pipeline are linked in the customer summary.",
			},
		},
		{
			"id":          "crm_sales_pipeline_review",
			"name":        "CRM Sales Pipeline Review",
			"description": "Summarize the CRM sales pipeline, highlight stale opportunities, and recommend the next internal sales action.",
			"domains":     []string{"crm", "sales"},
			"labels":      []string{"crm", "sales", "pipeline"},
			"keywords":    []string{"crm pipeline", "stale opportunity", "sales pipeline", "next action"},
			"use_when":    "The user asks for a CRM pipeline review, stale opportunities, or the next action for active opportunities.",
			"workflow_steps": []map[string]any{
				{"step": "summarize_pipeline", "title": "Summarize Pipeline", "tool_id": "crm.opportunity.pipeline.summary", "required": true, "description": "Load CRM sales pipeline value, stale opportunities, and activity coverage.", "output": "Pipeline value, stage mix, stale opportunities, and recommended focus."},
				{"step": "recommend_next_action", "title": "Recommend Next Action", "required": true, "description": "If one stale opportunity is called out, name it explicitly and recommend the next internal action."},
			},
			"tool_inventory": []string{
				"crm.opportunity.pipeline.summary",
				"crm.opportunity.search",
				"crm.lead.search",
			},
			"required_final_facts": []string{
				"Current pipeline value or stage concentration.",
				"Stale opportunity name when present.",
				"Recommended next internal action.",
			},
			"guardrails": []string{
				"Do not infer pipeline health from customer health alone; use crm.opportunity.pipeline.summary.",
			},
			"success_checks": []string{
				"Final answer names the stale opportunity explicitly when one exists.",
			},
			"pitfalls": []string{
				"Do not stop at total pipeline value without calling out stale or overdue next-action pressure.",
			},
		},
		{
			"id":          "crm_service_sales_overview",
			"name":        "CRM Service and Sales Overview",
			"description": "Summarize CRM service backlog, customer health, customer 360 context, and active sales pipeline for internal CRM review.",
			"domains":     []string{"crm", "service", "sales"},
			"labels":      []string{"crm", "customer-360", "backlog", "pipeline"},
			"keywords":    []string{"crm", "ticket", "backlog", "customer health", "customer 360", "pipeline", "opportunity", "lead"},
			"use_when":    "The user asks about CRM ticket backlog, customer health, customer history, opportunity pipeline, or wants a combined service-and-sales overview.",
			"workflow_steps": []map[string]any{
				{"step": "select_overview_skill", "title": "Select Overview Skill", "required": true, "description": "Search skills first and use this skill when the request spans ticketing, customer health, and sales pipeline in CRM."},
				{"step": "summarize_service_backlog", "title": "Summarize Service Backlog", "tool_id": "crm.ticket.summary", "required": true, "description": "Load CRM service backlog, queue health, and open-ticket pressure.", "output": "Open tickets, overdue tickets, queue backlog, and SLA context."},
				{"step": "summarize_customer_health", "title": "Summarize Customer Health", "tool_id": "crm.customer.health", "required": false, "when": "Use when the request asks about customer risk, health, or at-risk customers.", "description": "Load at-risk customers and CRM service risk indicators.", "output": "Customer health indicators and at-risk accounts."},
				{"step": "load_customer_360", "title": "Load Customer 360", "tool_id": "crm.customer.summary", "required": false, "when": "Use when the request names a customer or asks for customer-specific CRM context.", "description": "Load one customer 360 summary with tickets, opportunities, and profile context.", "output": "Customer-specific CRM summary."},
				{"step": "summarize_pipeline", "title": "Summarize Pipeline", "tool_id": "crm.opportunity.pipeline.summary", "required": true, "description": "Load active CRM sales pipeline status.", "output": "Pipeline value, stage mix, stale opportunities, and activity coverage."},
				{"step": "favor_named_customer_context", "title": "Favor Named Customer Context", "required": true, "description": "If the request is about one named customer, load the customer summary or timeline directly instead of only aggregate summaries."},
				{"step": "name_checked_records", "title": "Name Checked Records", "required": true, "description": "Name the CRM record type or summary type you checked in the final answer."},
			},
			"tool_inventory": []string{
				"crm.ticket.summary",
				"crm.customer.health",
				"crm.customer.summary",
				"crm.customer.timeline",
				"crm.opportunity.pipeline.summary",
				"crm.opportunity.search",
				"crm.lead.search",
			},
			"required_final_facts": []string{
				"Current CRM service backlog or open-ticket count.",
				"Customer health or at-risk customer signal when requested.",
				"Current active CRM pipeline value or stage summary when requested.",
				"Specific customer name and CRM context when the question names one customer.",
			},
			"guardrails": []string{
				"Do not create or update CRM records unless the user explicitly asks to do so.",
				"Do not answer a customer-specific question with only aggregate backlog metrics.",
				"Do not rely on generic business record wrappers when a dedicated CRM tool covers the request.",
			},
			"success_checks": []string{
				"Final answer references the CRM summary types or record families that were checked.",
				"Customer-specific questions use crm.customer.summary or crm.customer.timeline instead of only aggregate search.",
				"Backlog-and-pipeline questions use both crm.ticket.summary and crm.opportunity.pipeline.summary.",
			},
			"pitfalls": []string{
				"Do not stop at crm_core.business.info.get or crm_core.business.records.search when dedicated CRM tools exist.",
				"Do not describe service backlog without checking crm.ticket.summary first.",
			},
			"examples": []string{
				"Summarize the CRM service backlog, customer health, and active pipeline for the seeded CRM demo customer.",
				"Which customers are most at risk in CRM, and what does the active opportunity pipeline look like?",
			},
		},
	})
}

func createAndApproveDocument(ctx context.Context, client *apiClient, documentType string, payload map[string]any) (documentFacts, error) {
	created, err := client.createDocument(ctx, map[string]any{
		"type":            documentType,
		"organization_id": defaultOrgID,
		"location_id":     defaultLocID,
		"payload":         payload,
	})
	if err != nil {
		return documentFacts{}, err
	}
	return submitApproveDocument(ctx, client, created.ID)
}

func submitApproveDocument(ctx context.Context, client *apiClient, id string) (documentFacts, error) {
	current, version, etag, err := client.getDocument(ctx, id)
	if err != nil {
		return documentFacts{}, err
	}
	current, version, etag, err = client.actionDocument(ctx, current.ID, version, etag, "submit")
	if err != nil {
		return documentFacts{}, err
	}
	current, _, _, err = client.actionDocument(ctx, current.ID, version, etag, "approve")
	if err != nil {
		return documentFacts{}, err
	}
	return current, nil
}

func submitApproveAndActionDocument(ctx context.Context, client *apiClient, id, action string) (documentFacts, error) {
	current, err := submitApproveDocument(ctx, client, id)
	if err != nil {
		return documentFacts{}, err
	}
	_, version, etag, err := client.getDocument(ctx, current.ID)
	if err != nil {
		return documentFacts{}, err
	}
	current, _, _, err = client.actionDocument(ctx, current.ID, version, etag, action)
	if err != nil {
		return documentFacts{}, err
	}
	return current, nil
}

func submitApproveAndMaybeStop(ctx context.Context, client *apiClient, id string, approve bool) (documentFacts, error) {
	current, version, etag, err := client.getDocument(ctx, id)
	if err != nil {
		return documentFacts{}, err
	}
	current, version, etag, err = client.actionDocument(ctx, current.ID, version, etag, "submit")
	if err != nil {
		return documentFacts{}, err
	}
	if !approve {
		return current, nil
	}
	current, _, _, err = client.actionDocument(ctx, current.ID, version, etag, "approve")
	if err != nil {
		return documentFacts{}, err
	}
	return current, nil
}

func employeeSpendPromptPack(name string) []promptExpectation {
	return []promptExpectation{
		{ID: "summary", Prompt: fmt.Sprintf("Give a concise summary of %s's employee-spend flow and current end state.", name), RequiredFacts: []requiredFact{{Key: "travel_request_approved", Severity: "high", Checks: []string{"travel request", "approved"}}, {Key: "cash_advance_approved", Severity: "high", Checks: []string{"cash advance", "100"}}, {Key: "expense_claim_approved", Severity: "high", Checks: []string{"expense claim", "170"}}, {Key: "reimbursement_paid", Severity: "high", Checks: []string{"reimbursement", "paid"}}}, ForbiddenPhrases: []string{"employee owes company", "pending approval"}},
		{ID: "advance_amount", Prompt: fmt.Sprintf("What cash advance amount was approved for %s?", name), RequiredFacts: []requiredFact{{Key: "approved_advance_amount", Severity: "critical", Checks: []string{"100"}}}, ForbiddenPhrases: []string{"30", "140"}},
		{ID: "claim_total", Prompt: fmt.Sprintf("What was the approved expense-claim total for %s?", name), RequiredFacts: []requiredFact{{Key: "approved_claim_total", Severity: "critical", Checks: []string{"170"}}}, ForbiddenPhrases: []string{"100", "140"}},
		{ID: "settlement", Prompt: fmt.Sprintf("Did %s owe the company or did the company reimburse the employee, and by how much?", name), RequiredFacts: []requiredFact{{Key: "settlement_direction", Severity: "critical", Checks: []string{"company", "reimburse"}}, {Key: "settlement_amount", Severity: "critical", Checks: []string{"140"}}}, ForbiddenPhrases: []string{"employee owes company", "-30"}},
		{ID: "payment_doc", Prompt: fmt.Sprintf("What reimbursement or payment document exists for %s, and what is its status?", name), RequiredFacts: []requiredFact{{Key: "payment_document_type", Severity: "high", Checks: []string{"reimbursement", "payment"}}, {Key: "payment_document_status", Severity: "critical", Checks: []string{"paid"}}}, ForbiddenPhrases: []string{"draft", "submitted", "approved only"}},
		{ID: "pending_work", Prompt: fmt.Sprintf("Are there any pending approvals or open workflow tasks for %s's spend scenario?", name), RequiredFacts: []requiredFact{{Key: "pending_approvals", Severity: "critical", Checks: []string{"no"}}, {Key: "open_tasks", Severity: "critical", Checks: []string{"no"}}}, ForbiddenPhrases: []string{"pending approval", "open task"}},
	}
}

func orderToCashPromptPack(customerName, itemCode string) []promptExpectation {
	return []promptExpectation{
		{ID: "summary", Prompt: fmt.Sprintf("Summarize the order-to-cash scenario for customer %s, including fulfillment and cash collection.", customerName), RequiredFacts: []requiredFact{{Key: "sales_order", Severity: "high", Checks: []string{"sales order"}}, {Key: "delivery", Severity: "high", Checks: []string{"delivered"}}, {Key: "invoice", Severity: "high", Checks: []string{"invoice"}}, {Key: "payment", Severity: "high", Checks: []string{"paid"}}}, ForbiddenPhrases: []string{"unpaid", "undelivered"}},
		{ID: "delivered_qty", Prompt: fmt.Sprintf("How many units of %s were delivered to %s?", itemCode, customerName), RequiredFacts: []requiredFact{{Key: "delivered_qty", Severity: "critical", Checks: []string{"5"}}}, ForbiddenPhrases: []string{"3", "15"}},
		{ID: "invoice_total", Prompt: fmt.Sprintf("What was the invoice total for %s?", customerName), RequiredFacts: []requiredFact{{Key: "invoice_total", Severity: "critical", Checks: []string{"555"}}}, ForbiddenPhrases: []string{"500", "155"}},
		{ID: "payment_status", Prompt: fmt.Sprintf("What is the payment status for %s's invoice?", customerName), RequiredFacts: []requiredFact{{Key: "payment_status", Severity: "critical", Checks: []string{"paid"}}}, ForbiddenPhrases: []string{"open", "draft"}},
		{ID: "return_reversal", Prompt: "Did this order have a return or reversal, and if so what document was created?", RequiredFacts: []requiredFact{{Key: "return_exists", Severity: "high", Checks: []string{"return"}}, {Key: "credit_note", Severity: "critical", Checks: []string{"credit note"}}}, ForbiddenPhrases: []string{"no return"}},
		{ID: "pending_work", Prompt: fmt.Sprintf("Are there any open commercial workflow items left in %s's scenario?", customerName), RequiredFacts: []requiredFact{{Key: "no_open_work", Severity: "critical", Checks: []string{"no"}}}, ForbiddenPhrases: []string{"pending", "open task"}},
	}
}

func procureToPayPromptPack(vendorName, itemCode string) []promptExpectation {
	return []promptExpectation{
		{ID: "summary", Prompt: fmt.Sprintf("Summarize the procure-to-pay scenario with vendor %s and item %s.", vendorName, itemCode), RequiredFacts: []requiredFact{{Key: "purchase_order", Severity: "high", Checks: []string{"purchase order"}}, {Key: "receipt", Severity: "high", Checks: []string{"goods receipt"}}, {Key: "bill", Severity: "high", Checks: []string{"vendor bill"}}, {Key: "payment", Severity: "high", Checks: []string{"paid"}}}, ForbiddenPhrases: []string{"unpaid", "not received"}},
		{ID: "received_qty", Prompt: fmt.Sprintf("How many units of %s were received from %s?", itemCode, vendorName), RequiredFacts: []requiredFact{{Key: "received_qty", Severity: "critical", Checks: []string{"15"}}}, ForbiddenPhrases: []string{"5", "12"}},
		{ID: "bill_total", Prompt: fmt.Sprintf("What was the vendor bill total for %s?", vendorName), RequiredFacts: []requiredFact{{Key: "bill_total", Severity: "critical", Checks: []string{"1665"}}}, ForbiddenPhrases: []string{"1500", "555"}},
		{ID: "payment_status", Prompt: fmt.Sprintf("What is the payment status for the vendor bill from %s?", vendorName), RequiredFacts: []requiredFact{{Key: "payment_status", Severity: "critical", Checks: []string{"paid"}}}, ForbiddenPhrases: []string{"open", "draft"}},
		{ID: "supplier_return", Prompt: "Was there a supplier return or vendor credit in this scenario?", RequiredFacts: []requiredFact{{Key: "supplier_return", Severity: "high", Checks: []string{"supplier return"}}, {Key: "vendor_credit", Severity: "critical", Checks: []string{"vendor credit"}}}, ForbiddenPhrases: []string{"no return"}},
		{ID: "inventory_effect", Prompt: "What happened to inventory in this procurement scenario?", RequiredFacts: []requiredFact{{Key: "inventory_effect", Severity: "critical", Checks: []string{"received"}}}, ForbiddenPhrases: []string{"no inventory impact"}},
	}
}

func leaveToPayrollPromptPack(employeeName string) []promptExpectation {
	return []promptExpectation{
		{ID: "summary", Prompt: fmt.Sprintf("Summarize %s's leave-to-payroll scenario.", employeeName), RequiredFacts: []requiredFact{{Key: "approved_leave", Severity: "high", Checks: []string{"approved", "leave"}}, {Key: "payroll", Severity: "high", Checks: []string{"payroll"}}, {Key: "payment", Severity: "high", Checks: []string{"paid"}}}, ForbiddenPhrases: []string{"pending leave"}},
		{ID: "leave_days", Prompt: fmt.Sprintf("How many leave days were approved for %s?", employeeName), RequiredFacts: []requiredFact{{Key: "leave_days", Severity: "critical", Checks: []string{"2"}}}, ForbiddenPhrases: []string{"1", "3"}},
		{ID: "balance", Prompt: fmt.Sprintf("What remaining leave balance is recorded for %s?", employeeName), RequiredFacts: []requiredFact{{Key: "leave_balance", Severity: "critical", Checks: []string{"10"}}}, ForbiddenPhrases: []string{"12", "2"}},
		{ID: "payroll_impact", Prompt: fmt.Sprintf("Did payroll reflect the leave impact for %s, and by how much?", employeeName), RequiredFacts: []requiredFact{{Key: "payroll_deduction", Severity: "critical", Checks: []string{"40"}}}, ForbiddenPhrases: []string{"0 deduction"}},
		{ID: "payment_status", Prompt: fmt.Sprintf("What is the payroll payment state for %s?", employeeName), RequiredFacts: []requiredFact{{Key: "payment_state", Severity: "critical", Checks: []string{"paid"}}}, ForbiddenPhrases: []string{"draft", "pending"}},
	}
}

func payrollRemittancePromptPack(authorityName string) []promptExpectation {
	return []promptExpectation{
		{ID: "summary", Prompt: fmt.Sprintf("Summarize the payroll remittance scenario with authority %s.", authorityName), RequiredFacts: []requiredFact{{Key: "liability", Severity: "high", Checks: []string{"liability"}}, {Key: "batch", Severity: "high", Checks: []string{"batch"}}, {Key: "payment", Severity: "high", Checks: []string{"paid"}}}, ForbiddenPhrases: []string{"no remittance"}},
		{ID: "liability_total", Prompt: fmt.Sprintf("What total remittance liability was due to %s?", authorityName), RequiredFacts: []requiredFact{{Key: "liability_total", Severity: "critical", Checks: []string{"120"}}}, ForbiddenPhrases: []string{"12", "220"}},
		{ID: "obligation_type", Prompt: fmt.Sprintf("What obligation type was remitted to %s?", authorityName), RequiredFacts: []requiredFact{{Key: "obligation_type", Severity: "critical", Checks: []string{"withholding"}}}, ForbiddenPhrases: []string{"employer contribution only"}},
		{ID: "batch_state", Prompt: fmt.Sprintf("What is the state of the remittance batch for %s?", authorityName), RequiredFacts: []requiredFact{{Key: "batch_state", Severity: "critical", Checks: []string{"approved"}}}, ForbiddenPhrases: []string{"draft", "cancelled"}},
		{ID: "payment_state", Prompt: fmt.Sprintf("What is the remittance payment state for %s?", authorityName), RequiredFacts: []requiredFact{{Key: "payment_state", Severity: "critical", Checks: []string{"paid"}}}, ForbiddenPhrases: []string{"open", "draft"}},
	}
}

func productionCostingPromptPack(finishedCode string) []promptExpectation {
	return []promptExpectation{
		{ID: "summary", Prompt: fmt.Sprintf("Summarize the production-costing scenario for finished item %s.", finishedCode), RequiredFacts: []requiredFact{{Key: "production_order", Severity: "high", Checks: []string{"production order"}}, {Key: "issue", Severity: "high", Checks: []string{"issue"}}, {Key: "output", Severity: "high", Checks: []string{"output"}}}, ForbiddenPhrases: []string{"not produced"}},
		{ID: "materials", Prompt: fmt.Sprintf("What materials were consumed to produce %s?", finishedCode), RequiredFacts: []requiredFact{{Key: "component_a", Severity: "critical", Checks: []string{"6"}}, {Key: "component_b", Severity: "critical", Checks: []string{"3"}}}, ForbiddenPhrases: []string{"0"}},
		{ID: "output_qty", Prompt: fmt.Sprintf("How many units of %s were produced?", finishedCode), RequiredFacts: []requiredFact{{Key: "output_qty", Severity: "critical", Checks: []string{"3"}}}, ForbiddenPhrases: []string{"1", "6"}},
		{ID: "cost_total", Prompt: fmt.Sprintf("What total production cost was recorded for %s?", finishedCode), RequiredFacts: []requiredFact{{Key: "cost_total", Severity: "critical", Checks: []string{"60"}}}, ForbiddenPhrases: []string{"30", "600"}},
		{ID: "status", Prompt: fmt.Sprintf("What is the status of the production output for %s?", finishedCode), RequiredFacts: []requiredFact{{Key: "output_state", Severity: "critical", Checks: []string{"posted"}}}, ForbiddenPhrases: []string{"draft", "cancelled"}},
	}
}

func inventoryReplenishmentExecutePromptPack(runID, warehouseCode, vendorName, coldBrewCode, oatMilkCode string) []promptExpectation {
	coldBrewName := "Cold Brew Beans 1kg " + runID
	oatMilkName := "Oat Milk Barista 1L " + runID
	matchaName := "Matcha Powder 500g " + runID
	cupsName := "Paper Cups 16oz " + runID
	return []promptExpectation{
		{
			ID:     "insight",
			Prompt: fmt.Sprintf("For warehouse %s, which inventory items are currently at replenishment risk, which items are healthy enough to skip, and why?", warehouseCode),
			RequiredFacts: []requiredFact{
				{Key: "risk_cold_brew", Severity: "critical", Checks: []string{coldBrewName}},
				{Key: "risk_oat_milk", Severity: "critical", Checks: []string{oatMilkName}},
				{Key: "healthy_matcha", Severity: "high", Checks: []string{matchaName}},
				{Key: "healthy_cups", Severity: "high", Checks: []string{cupsName}},
			},
			ForbiddenPhrases: []string{"reorder matcha", "reorder paper cups"},
		},
		{
			ID:     "plan",
			Prompt: fmt.Sprintf("For warehouse %s, what replenishment plan should we run next, including exact quantities and vendor?", warehouseCode),
			RequiredFacts: []requiredFact{
				{Key: "plan_cold_brew_qty", Severity: "critical", Checks: []string{coldBrewName, "20"}},
				{Key: "plan_oat_milk_qty", Severity: "critical", Checks: []string{oatMilkName, "16"}},
				{Key: "plan_vendor", Severity: "critical", Checks: []string{vendorName}},
			},
			ForbiddenPhrases: []string{"reorder matcha", "reorder paper cups", "all vendors"},
		},
		{
			ID:     "execute",
			Prompt: fmt.Sprintf("Create draft purchase request documents for that replenishment plan for warehouse %s. Use the recommended quantities and do not submit them. After creating them, tell me the draft ids and summarize what was created.", warehouseCode),
			RequiredFacts: []requiredFact{
				{Key: "draft_created", Severity: "critical", Checks: []string{"draft", "purchase request"}},
				{Key: "draft_items", Severity: "critical", Checks: []string{"cold brew", "oat milk"}},
				{Key: "draft_vendor", Severity: "high", Checks: []string{"north roast"}},
			},
			ForbiddenPhrases: []string{"submitted", matchaName, cupsName},
			ExpectedDraft: &draftExpectation{
				DocumentType:  "purchase_request",
				PayloadChecks: []string{"cold brew", "20", "oat milk", "16", "north roast"},
			},
		},
	}
}

func inventoryDashboardReplenishmentExecutePromptPack(runID, warehouseCode, vendorName, coldBrewCode, oatMilkCode string) []promptExpectation {
	coldBrewName := "Cold Brew Beans 1kg " + runID
	oatMilkName := "Oat Milk Barista 1L " + runID
	matchaName := "Matcha Powder 500g " + runID
	cupsName := "Paper Cups 16oz " + runID
	return []promptExpectation{
		{
			ID:     "insight",
			Prompt: fmt.Sprintf("For warehouse %s, show me the most relevant dashboard widgets for replenishment risk and tell me which inventory items are at active replenishment risk, which items are healthy enough to skip, and why.", warehouseCode),
			RequiredFacts: []requiredFact{
				{Key: "risk_cold_brew", Severity: "critical", Checks: []string{coldBrewName}},
				{Key: "risk_oat_milk", Severity: "critical", Checks: []string{oatMilkName}},
				{Key: "skip_matcha", Severity: "high", Checks: []string{matchaName}},
				{Key: "skip_cups", Severity: "high", Checks: []string{cupsName}},
			},
			ForbiddenPhrases: []string{"all items are healthy", "no replenishment risk"},
			ExpectedArtifact: &artifactExpectation{
				Kind:        "dashboard_board",
				TitleChecks: []string{"replenishment", "dashboard"},
				WidgetKeys:  []string{"planning.replenishment.shortages", "planning.replenishment.items"},
				MinWidgets:  2,
			},
		},
		{
			ID:     "plan",
			Prompt: fmt.Sprintf("Based on that dashboard, create a stepwise replenishment plan for warehouse %s. Keep it focused on the at-risk items, include exact quantities and vendor, and do not execute it.", warehouseCode),
			RequiredFacts: []requiredFact{
				{Key: "plan_cold_brew_qty", Severity: "critical", Checks: []string{coldBrewName, "20"}},
				{Key: "plan_oat_milk_qty", Severity: "critical", Checks: []string{oatMilkName, "16"}},
				{Key: "plan_vendor", Severity: "critical", Checks: []string{vendorName}},
			},
			ExpectedPlan: &planExpectation{
				MinSteps:      2,
				ContentChecks: []string{coldBrewName, oatMilkName, vendorName, "20", "16"},
			},
			ForbiddenPhrases: []string{"submit now"},
		},
		{
			ID:     "execute",
			Prompt: fmt.Sprintf("Create draft purchase request documents from that replenishment plan for warehouse %s. Use the recommended quantities, do not submit them, and then tell me the draft ids, vendor, and what was created.", warehouseCode),
			RequiredFacts: []requiredFact{
				{Key: "draft_created", Severity: "critical", Checks: []string{"draft", "purchase request"}},
				{Key: "draft_vendor", Severity: "critical", Checks: []string{vendorName}},
				{Key: "draft_items", Severity: "critical", Checks: []string{"cold brew", "20", "oat milk", "16"}},
			},
			ForbiddenPhrases: []string{"submitted", matchaName, cupsName},
		},
	}
}

func salesDashboardRecoveryExecutePromptPack(runID, draftTitle string) []promptExpectation {
	return []promptExpectation{
		{
			ID:     "insight",
			Prompt: "Which branches are underperforming this week compared with the strongest branch, and show me the most relevant dashboard widgets for why?",
			RequiredFacts: []requiredFact{
				{Key: "underperforming_central", Severity: "critical", Checks: []string{"loc demo central"}},
				{Key: "underperforming_west", Severity: "critical", Checks: []string{"loc demo west"}},
				{Key: "benchmark_east", Severity: "high", Checks: []string{"loc demo east"}},
			},
			ForbiddenPhrases: []string{"all branches are performing evenly", "loc demo east is underperforming"},
			ExpectedArtifact: &artifactExpectation{
				Kind: "dashboard_widget",
				WidgetKeys: []string{
					"analytics.demo.sales.target_attainment",
					"analytics.demo.sales.daily_trend",
					"analytics.demo.sales.branch_mix",
				},
				MinArtifacts: 3,
			},
		},
		{
			ID:     "plan",
			Prompt: "Based on that dashboard, create a stepwise branch recovery plan. Keep it focused on Loc Demo Central and Loc Demo West, use Loc Demo East as the benchmark, and do not execute it.",
			RequiredFacts: []requiredFact{
				{Key: "focus_central", Severity: "critical", Checks: []string{"loc demo central"}},
				{Key: "focus_west", Severity: "critical", Checks: []string{"loc demo west"}},
				{Key: "benchmark_east", Severity: "high", Checks: []string{"loc demo east"}},
			},
			ExpectedPlan: &planExpectation{
				MinSteps:      3,
				ContentChecks: []string{"loc demo central", "loc demo west", "loc demo east", "target"},
			},
			ForbiddenPhrases: []string{"execute immediately", "submit the request now"},
		},
		{
			ID:     "execute",
			Prompt: fmt.Sprintf("Create a draft generic request titled %q from that plan. Include Loc Demo Central, Loc Demo West, Loc Demo East as the benchmark, and a next-week target-attainment follow-up. Do not submit it. After creating it, tell me the draft id and link.", draftTitle),
			RequiredFacts: []requiredFact{
				{Key: "draft_created", Severity: "critical", Checks: []string{"draft", "generic request", draftTitle}},
				{Key: "draft_title", Severity: "critical", Checks: []string{draftTitle}},
			},
			ForbiddenPhrases: []string{"submitted", "approved"},
			ExpectedDraft: &draftExpectation{
				DocumentType:  "generic_request",
				TitleChecks:   []string{draftTitle},
				PayloadChecks: []string{"loc demo central", "loc demo west", "loc demo east", "target-attainment"},
			},
		},
	}
}

func posPromotionStrategyPromptPack(runID, storeCode, espressoCode, croissantCode, beansPromoCode, draftTitle string) []promptExpectation {
	espressoName := "Espresso Double " + runID
	croissantName := "Butter Croissant " + runID
	beansCampaignName := "Beans Boost " + runID
	return []promptExpectation{
		{
			ID:     "strategy",
			Prompt: fmt.Sprintf("For store %s, review the current POS sales trend, member behavior, and promotion setup. What promotion campaign should we run next, for which products and customer segment, and why?", storeCode),
			RequiredFacts: []requiredFact{
				{Key: "campaign_shape", Severity: "critical", Checks: []string{"bundle"}},
				{Key: "target_product_espresso", Severity: "critical", Checks: []string{espressoName}},
				{Key: "target_product_croissant", Severity: "critical", Checks: []string{croissantName}},
				{Key: "target_segment", Severity: "critical", Checks: []string{"gold", "member"}},
			},
			ForbiddenPhrases: []string{"all customers", "discount espresso only"},
		},
		{
			ID:     "evidence",
			Prompt: fmt.Sprintf("For store %s, what sales pattern is the strongest evidence for that recommendation?", storeCode),
			RequiredFacts: []requiredFact{
				{Key: "repeat_combo_pattern", Severity: "critical", Checks: []string{"espresso", "croissant", "together"}},
				{Key: "member_pattern", Severity: "high", Checks: []string{"gold", "member"}},
			},
			ForbiddenPhrases: []string{"beans was the top bundle", "no repeat pattern"},
		},
		{
			ID:     "retire",
			Prompt: fmt.Sprintf("For store %s, which current promotion should we retire or replace, and why?", storeCode),
			RequiredFacts: []requiredFact{
				{Key: "underperforming_campaign", Severity: "critical", Checks: []string{beansCampaignName}},
				{Key: "underperforming_reason", Severity: "critical", Checks: []string{"one", "redemption"}},
			},
			ForbiddenPhrases: []string{"keep beans boost", "espresso bundle is underperforming"},
		},
		{
			ID:     "products_segment",
			Prompt: fmt.Sprintf("For store %s, name the exact products and customer segment you would target in the next campaign.", storeCode),
			RequiredFacts: []requiredFact{
				{Key: "target_product_espresso", Severity: "critical", Checks: []string{espressoName}},
				{Key: "target_product_croissant", Severity: "critical", Checks: []string{croissantName}},
				{Key: "target_segment", Severity: "critical", Checks: []string{"gold", "member"}},
			},
			ForbiddenPhrases: []string{"house beans 1kg", "walk-in only"},
		},
		{
			ID:     "draft",
			Prompt: fmt.Sprintf("Create a draft generic request titled %q that captures the recommended promotion plan. Include the target products, the gold-member segment, and that %s should be replaced. Do not submit it. After creating it, tell me the draft id and restate the recommendation briefly.", draftTitle, beansCampaignName),
			RequiredFacts: []requiredFact{
				{Key: "draft_created", Severity: "critical", Checks: []string{"draft"}},
				{Key: "draft_title", Severity: "critical", Checks: []string{draftTitle}},
				{Key: "recommendation_restated", Severity: "high", Checks: []string{"bundle", "gold"}},
			},
			ForbiddenPhrases: []string{"submitted", beansPromoCode + " should continue"},
			ExpectedDraft: &draftExpectation{
				DocumentType:  "generic_request",
				TitleChecks:   []string{draftTitle},
				PayloadChecks: []string{"espresso", "croissant", "gold", "beans boost"},
			},
		},
	}
}

func retailRecoveryShowcasePromptPack(runID, storeCode, draftTitle string) []promptExpectation {
	espressoName := "Espresso Double " + runID
	croissantName := "Butter Croissant " + runID
	beansCampaignName := "Beans Boost " + runID
	return []promptExpectation{
		{
			ID:     "insight",
			Prompt: fmt.Sprintf("For store %s, which branches are underperforming compared with the strongest branch, what POS sales pattern should we lean into, and show me the most relevant dashboard widgets for why?", storeCode),
			RequiredFacts: []requiredFact{
				{Key: "underperforming_central", Severity: "critical", Checks: []string{"loc demo central"}},
				{Key: "underperforming_west", Severity: "critical", Checks: []string{"loc demo west"}},
				{Key: "benchmark_east", Severity: "high", Checks: []string{"loc demo east"}},
				{Key: "espresso_pattern", Severity: "critical", Checks: []string{espressoName}},
				{Key: "croissant_pattern", Severity: "critical", Checks: []string{croissantName}},
				{Key: "gold_segment", Severity: "high", Checks: []string{"gold", "member"}},
			},
			ForbiddenPhrases: []string{"all branches are performing evenly", "loc demo east is underperforming"},
			ExpectedArtifact: &artifactExpectation{
				Kind: "dashboard_widget",
				WidgetKeys: []string{
					"analytics.demo.sales.target_attainment",
					"analytics.demo.sales.daily_trend",
					"analytics.demo.sales.branch_mix",
				},
				MinArtifacts: 3,
			},
		},
		{
			ID:     "plan",
			Prompt: "Based on that dashboard and POS evidence, create a stepwise recovery plan. Focus on Loc Demo Central and Loc Demo West, use Loc Demo East as the benchmark, target gold members, and do not execute it.",
			RequiredFacts: []requiredFact{
				{Key: "focus_central", Severity: "critical", Checks: []string{"loc demo central"}},
				{Key: "focus_west", Severity: "critical", Checks: []string{"loc demo west"}},
				{Key: "benchmark_east", Severity: "high", Checks: []string{"loc demo east"}},
				{Key: "campaign_products", Severity: "critical", Checks: []string{espressoName, croissantName}},
				{Key: "replace_campaign", Severity: "high", Checks: []string{beansCampaignName}},
			},
			ExpectedPlan: &planExpectation{
				MinSteps:      3,
				ContentChecks: []string{"loc demo central", "loc demo west", "loc demo east", "gold", "espresso", "croissant"},
			},
			ForbiddenPhrases: []string{"execute immediately", "submit the request now"},
		},
		{
			ID:     "execute",
			Prompt: fmt.Sprintf("Create a draft generic request titled %q from that plan. Include Loc Demo Central and Loc Demo West as the target branches, Loc Demo East as the benchmark, Espresso Double and Butter Croissant for gold members, replace Beans Boost, and add a next-week target-attainment follow-up. Do not submit it. After creating it, tell me the draft id and link.", draftTitle),
			RequiredFacts: []requiredFact{
				{Key: "draft_created", Severity: "critical", Checks: []string{"draft", draftTitle}},
				{Key: "draft_type", Severity: "critical", Checks: []string{"generic request"}},
			},
			ForbiddenPhrases: []string{"submitted", "approved"},
			ExpectedDraft: &draftExpectation{
				DocumentType:  "generic_request",
				TitleChecks:   []string{draftTitle},
				PayloadChecks: []string{"loc demo central", "loc demo west", "loc demo east", "espresso", "croissant", "gold", "beans boost", "target-attainment"},
			},
		},
	}
}

func crmServiceSalesOverviewPromptPack(customerName, healthyCustomerName, queueSupportCode, ticketTitle, overdueTicketTitle, opportunityTitle string) []promptExpectation {
	return []promptExpectation{
		{
			ID:     "service_backlog_widgets",
			Prompt: "Review the CRM service backlog. Which queue and customer need the most attention right now, and show me the most relevant dashboard widgets for why?",
			RequiredFacts: []requiredFact{
				{Key: "priority_queue", Severity: "critical", Checks: []string{queueSupportCode}},
				{Key: "priority_customer", Severity: "critical", Checks: []string{customerName}},
				{Key: "overdue_ticket", Severity: "high", Checks: []string{overdueTicketTitle}},
			},
			ExpectedArtifact: &artifactExpectation{
				Kind: "dashboard_widget",
				WidgetKeys: []string{
					"crm.ticketing.open_tickets",
					"crm.ticketing.overdue_tickets",
					"crm.ticketing.queue_backlog",
				},
				MinArtifacts: 3,
			},
		},
		{
			ID:     "customer_360",
			Prompt: fmt.Sprintf("Open the CRM customer 360 context for %s and explain the link between the current service issue and the active opportunity.", customerName),
			RequiredFacts: []requiredFact{
				{Key: "customer_name", Severity: "critical", Checks: []string{customerName}},
				{Key: "ticket_title", Severity: "critical", Checks: []string{ticketTitle}},
				{Key: "opportunity_title", Severity: "critical", Checks: []string{opportunityTitle}},
				{Key: "proposal_stage", Severity: "high", Checks: []string{"proposal"}},
			},
		},
		{
			ID:     "pipeline_review",
			Prompt: "Summarize the active CRM sales pipeline. Which opportunity is stale, what is the pipeline value we should prioritize, and what should happen next?",
			RequiredFacts: []requiredFact{
				{Key: "stale_opportunity", Severity: "critical", Checks: []string{opportunityTitle}},
				{Key: "priority_customer", Severity: "high", Checks: []string{customerName}},
				{Key: "pipeline_value", Severity: "critical", Checks: []string{"24000000"}},
			},
		},
		{
			ID:     "service_sales_overview",
			Prompt: fmt.Sprintf("Give me a combined CRM service and sales overview for %s and %s. Tell me who is at risk, who looks healthy, and the next internal action we should take.", customerName, healthyCustomerName),
			RequiredFacts: []requiredFact{
				{Key: "at_risk_customer", Severity: "critical", Checks: []string{customerName}},
				{Key: "healthy_customer", Severity: "critical", Checks: []string{healthyCustomerName}},
				{Key: "backlog_reference", Severity: "high", Checks: []string{queueSupportCode}},
				{Key: "pipeline_reference", Severity: "high", Checks: []string{"pipeline"}},
			},
		},
	}
}

func mustJSONString(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func createSeedCommercialItem(ctx context.Context, client *apiClient, values map[string]any) error {
	item := cloneScenarioValues(values)
	itemCode := strings.TrimSpace(stringValue(item["sku"]))
	productCode := strings.TrimSpace(stringValue(item["product_code"]))
	kind := strings.ToLower(strings.TrimSpace(stringValue(item["kind"])))
	itemType := strings.ToLower(strings.TrimSpace(stringValue(item["item_type"])))
	isSellable := true
	if raw, ok := item["is_sellable"]; ok {
		isSellable = boolValue(raw)
	}
	needsProduct := strings.TrimSpace(stringValue(item["variant_signature"])) != "" ||
		boolValue(item["is_variant"]) ||
		kind == "variant" ||
		(isSellable && (itemType == "product" || kind == "product" || kind == "item" || kind == "simple"))
	if err := ensureSeedCommercialReferences(ctx, client, item); err != nil {
		return err
	}
	if needsProduct {
		if productCode == "" {
			productCode = itemCode
			item["product_code"] = productCode
		}
		if itemType == "" {
			item["item_type"] = "product"
		}
		if _, err := client.createModel(ctx, "commercial_product", map[string]any{
			"code":                                 productCode,
			"name":                                 stringValue(item["name"]),
			"item_type":                            stringValue(item["item_type"]),
			"category_code":                        stringValue(item["category_code"]),
			"uom_code":                             stringValue(item["uom_code"]),
			"currency_code":                        stringValue(item["currency_code"]),
			"tax_code":                             stringValue(item["tax_code"]),
			"revenue_account_code":                 stringValue(item["revenue_account_code"]),
			"inventory_asset_account_code":         stringValue(item["inventory_asset_account_code"]),
			"cogs_account_code":                    stringValue(item["cogs_account_code"]),
			"wip_account_code":                     stringValue(item["wip_account_code"]),
			"inventory_enabled":                    item["inventory_enabled"],
			"inventory_tracking_mode":              item["inventory_tracking_mode"],
			"expiry_tracking_enabled":              item["expiry_tracking_enabled"],
			"allow_negative_stock":                 item["allow_negative_stock"],
			"default_issue_strategy":               item["default_issue_strategy"],
			"replenishment_enabled":                item["replenishment_enabled"],
			"replenishment_mode":                   item["replenishment_mode"],
			"reorder_point_quantity":               item["reorder_point_quantity"],
			"target_stock_quantity":                item["target_stock_quantity"],
			"default_replenishment_warehouse_code": item["default_replenishment_warehouse_code"],
			"status":                               firstNonEmptyString(stringValue(item["status"]), "active"),
		}); err != nil {
			return err
		}
	}
	_, err := client.createModel(ctx, "commercial_item", item)
	return err
}

func ensureSeedPOSReferences(ctx context.Context, client *apiClient) error {
	for _, account := range []map[string]any{
		{"code": "1000-CASH", "name": "Cash", "account_type": "asset", "normal_balance": "debit", "status": "active"},
		{"code": "1010-BANK", "name": "Bank Clearing", "account_type": "asset", "normal_balance": "debit", "status": "active"},
		{"code": "4000-REV", "name": "Sales Revenue", "account_type": "revenue", "normal_balance": "credit", "status": "active"},
		{"code": "2100-VATOUT", "name": "VAT Output", "account_type": "liability", "normal_balance": "credit", "status": "active"},
	} {
		if err := createModelIgnoreConflict(ctx, client, "finance_account", account); err != nil {
			return err
		}
	}
	return createModelIgnoreConflict(ctx, client, "warehouse", map[string]any{
		"code":   "MAIN",
		"name":   "Main Warehouse",
		"kind":   "storage",
		"status": "active",
	})
}

func createSeedCustomerProfile(ctx context.Context, client *apiClient, partyName string, values map[string]any) (map[string]any, error) {
	party, err := client.createModel(ctx, "party", map[string]any{
		"party_type": "person",
		"name":       partyName,
		"status":     "active",
	})
	if err != nil {
		return nil, err
	}
	profile := cloneScenarioValues(values)
	profile["party_id"] = stringValue(party["id"])
	if strings.TrimSpace(stringValue(profile["customer_name"])) == "" {
		profile["customer_name"] = partyName
	}
	return client.createModel(ctx, "customer_profile", profile)
}

func ensureSeedCommercialReferences(ctx context.Context, client *apiClient, item map[string]any) error {
	if uomCode := strings.TrimSpace(stringValue(item["uom_code"])); uomCode != "" {
		if err := createModelIgnoreConflict(ctx, client, "commercial_uom", map[string]any{
			"code":   uomCode,
			"name":   strings.ToUpper(uomCode),
			"symbol": strings.ToUpper(uomCode),
			"status": "active",
		}); err != nil {
			return err
		}
	}
	if taxCode := strings.TrimSpace(stringValue(item["tax_code"])); taxCode != "" {
		if err := createModelIgnoreConflict(ctx, client, "commercial_tax_code", map[string]any{
			"code":   taxCode,
			"name":   taxCode,
			"mode":   "exclusive",
			"rate":   11,
			"status": "active",
		}); err != nil {
			return err
		}
	}
	accountDefaults := map[string]map[string]string{
		"revenue_account_code":         {"account_type": "revenue", "normal_balance": "credit"},
		"inventory_asset_account_code": {"account_type": "asset", "normal_balance": "debit"},
		"cogs_account_code":            {"account_type": "expense", "normal_balance": "debit"},
		"wip_account_code":             {"account_type": "asset", "normal_balance": "debit"},
	}
	for field, defaults := range accountDefaults {
		code := strings.TrimSpace(stringValue(item[field]))
		if code == "" {
			continue
		}
		if err := createModelIgnoreConflict(ctx, client, "finance_account", map[string]any{
			"code":           code,
			"name":           code,
			"account_type":   defaults["account_type"],
			"normal_balance": defaults["normal_balance"],
			"status":         "active",
		}); err != nil {
			return err
		}
	}
	return nil
}

func createModelIgnoreConflict(ctx context.Context, client *apiClient, key string, values map[string]any) error {
	if _, err := client.createModel(ctx, key, values); err != nil {
		if strings.Contains(err.Error(), "status=409") || strings.Contains(strings.ToLower(err.Error()), "duplicate key value") {
			return nil
		}
		return err
	}
	return nil
}

func periodID(record map[string]any) string {
	return stringValue(record["id"])
}

func numberValue(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		f, _ := v.Float64()
		return f
	default:
		return 0
	}
}

func boolValue(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	default:
		return false
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func cloneScenarioValues(values map[string]any) map[string]any {
	cloned := map[string]any{}
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func agentproofMCPOperations() []string {
	return []string{
		"agent.workspace.use",
		"platform.context.read",
		"module.read",
		"document.create",
		"document.read",
		"document.list",
		"document.update_draft",
		"configuration.read",
		"analytics.read",
		"monitoring.read",
		"template.read",
		"audit.read",
		"event.read",
		"outbox.read",
		"deadletter.read",
		"crm_queue.list",
		"crm_queue.read",
		"crm_sla_policy.list",
		"crm_sla_policy.read",
		"crm_assignment_rule.list",
		"crm_assignment_rule.read",
		"crm_ticket.list",
		"crm_ticket.read",
		"crm_ticket.create",
		"crm_ticket.update",
		"crm_ticket_comment.list",
		"crm_ticket_comment.read",
		"crm_ticket_comment.create",
		"crm_ticket_activity.list",
		"crm_ticket_activity.read",
		"crm_lead.list",
		"crm_lead.read",
		"crm_lead.create",
		"crm_lead.update",
		"crm_opportunity.list",
		"crm_opportunity.read",
		"crm_opportunity.create",
		"crm_opportunity.update",
		"crm_activity.list",
		"crm_activity.read",
		"party.list",
		"party.read",
		"party_contact.list",
		"party_contact.read",
		"item.list",
		"customer.list",
		"customer.read",
		"pos_sale.list",
		"promotion_campaign.list",
		"promotion_code.list",
		"promotion_redemption.list",
		"discount_rule.list",
	}
}
