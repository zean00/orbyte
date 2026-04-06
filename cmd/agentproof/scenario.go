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
	if _, err := client.createModel(ctx, "commercial_item", map[string]any{
		"sku":                          itemCode,
		"name":                         "Field Router " + runID,
		"kind":                         "simple",
		"item_type":                    "product",
		"uom_code":                     "EA",
		"unit_price":                   100.0,
		"tax_code":                     taxCode,
		"revenue_account_code":         "4000-REV",
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
			"uom_code":       "EA",
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
			"uom_code":       "EA",
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
		"uom_code":                     "EA",
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
			"uom_code":             "EA",
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
			"uom_code":                             "EA",
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
			"uom_code":                             "EA",
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
			"uom_code":                             "EA",
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
			"uom_code":                             "EA",
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
		if _, err := client.createModel(ctx, "commercial_item", item); err != nil {
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
			"purchase_uom_code":      "EA",
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
			"purchase_uom_code":      "EA",
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
	if _, err := createAndApproveDocument(ctx, client, "stock_receipt", map[string]any{
		"receipt_date":   now.AddDate(0, -2, 0).Format("2006-01-02"),
		"warehouse_code": warehouseCode,
		"lines": []map[string]any{
			{"item_code": coldBrewCode, "description": "Opening stock", "warehouse_code": warehouseCode, "uom_code": "EA", "quantity": 46.0, "unit_cost": 172000.0},
			{"item_code": oatMilkCode, "description": "Opening stock", "warehouse_code": warehouseCode, "uom_code": "EA", "quantity": 36.0, "unit_cost": 39000.0},
			{"item_code": matchaCode, "description": "Opening stock", "warehouse_code": warehouseCode, "uom_code": "EA", "quantity": 30.0, "unit_cost": 120000.0},
			{"item_code": cupsCode, "description": "Opening stock", "warehouse_code": warehouseCode, "uom_code": "EA", "quantity": 40.0, "unit_cost": 1800.0},
		},
	}); err != nil {
		return scenarioManifest{}, fmt.Errorf("seed stock receipt: %w", err)
	}

	for week := 1; week <= 6; week++ {
		fulfillmentDate := now.AddDate(0, 0, -(7 * week)).Format("2006-01-02")
		if _, err := createAndApproveDocument(ctx, client, "sales_fulfillment", map[string]any{
			"source_order_number": fmt.Sprintf("SO-HIST-%s-%d", suffix, week),
			"party_name":          "Cafe Horizon " + runID,
			"fulfillment_date":    fulfillmentDate,
			"lines": []map[string]any{
				{"item_code": coldBrewCode, "description": "Cold brew beans historical issue", "warehouse_code": warehouseCode, "uom_code": "EA", "quantity": 7.0},
				{"item_code": oatMilkCode, "description": "Oat milk historical issue", "warehouse_code": warehouseCode, "uom_code": "EA", "quantity": 5.0},
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
			{"item_code": coldBrewCode, "description": "Cold brew beans urgent demand", "warehouse_code": warehouseCode, "uom_code": "EA", "quantity": 4.0, "unit_price": 180000.0, "line_total": 720000.0},
			{"item_code": oatMilkCode, "description": "Oat milk urgent demand", "warehouse_code": warehouseCode, "uom_code": "EA", "quantity": 3.0, "unit_price": 42000.0, "line_total": 126000.0},
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
- When the dashboard preview tool returns the <orbyte-dashboard-artifact> block, copy that block exactly into your final answer unchanged.
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
				"location_id":     item.locationID,
				"payload":         map[string]any{"title": item.title},
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
- When the dashboard preview tool returns the <orbyte-dashboard-artifact> block, copy that block exactly into your final answer unchanged.
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

	paymentMethod, err := client.createModel(ctx, "payment_method", map[string]any{
		"code":                  "CASHPROMO-" + suffix,
		"name":                  "Cash Promo " + runID,
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
			"uom_code":             "EA",
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
			"uom_code":             "EA",
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
			"uom_code":             "EA",
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
			"uom_code":             "EA",
			"unit_price":           18000.0,
			"revenue_account_code": "4000-REV",
			"is_sellable":          true,
			"inventory_enabled":    false,
			"allow_negative_stock": true,
			"status":               "active",
		},
	} {
		if _, err := client.createModel(ctx, "commercial_item", item); err != nil {
			return scenarioManifest{}, fmt.Errorf("create item %s: %w", stringValue(item["sku"]), err)
		}
	}

	goldOne, err := client.createModel(ctx, "customer_profile", map[string]any{
		"party_id":        "party_gold_1_" + suffix,
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
	goldTwo, err := client.createModel(ctx, "customer_profile", map[string]any{
		"party_id":        "party_gold_2_" + suffix,
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
	silverCustomer, err := client.createModel(ctx, "customer_profile", map[string]any{
		"party_id":        "party_silver_" + suffix,
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

	paymentMethod, err := client.createModel(ctx, "payment_method", map[string]any{
		"code":                  "CASHSHOW-" + suffix,
		"name":                  "Cash Showcase " + runID,
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
			"uom_code":             "EA",
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
			"uom_code":             "EA",
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
			"uom_code":             "EA",
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
			"uom_code":             "EA",
			"unit_price":           18000.0,
			"revenue_account_code": "4000-REV",
			"is_sellable":          true,
			"inventory_enabled":    false,
			"allow_negative_stock": true,
			"status":               "active",
		},
	} {
		if _, err := client.createModel(ctx, "commercial_item", item); err != nil {
			return scenarioManifest{}, fmt.Errorf("create item %s: %w", stringValue(item["sku"]), err)
		}
	}

	goldOne, err := client.createModel(ctx, "customer_profile", map[string]any{
		"party_id":        "party_showcase_gold_1_" + suffix,
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
	goldTwo, err := client.createModel(ctx, "customer_profile", map[string]any{
		"party_id":        "party_showcase_gold_2_" + suffix,
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
	silverCustomer, err := client.createModel(ctx, "customer_profile", map[string]any{
		"party_id":        "party_showcase_silver_" + suffix,
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
	if seededPIN {
		if err := client.enterPOSTerminal(ctx, storeCode, registerCode, shiftID, terminalPIN); err != nil {
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
				"location_id":     item.locationID,
				"payload":         map[string]any{"title": item.title},
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
- When the widgets preview tool returns the <orbyte-dashboard-artifact> blocks, copy each block exactly into your final answer unchanged.
- For planning questions, base the plan on both the dashboard evidence and the POS sales pattern, keep the answer stepwise as a numbered list using "1.", "2.", and "3." markers, and do not execute it.
- The recommended campaign should be a breakfast bundle for gold members focused on Loc Demo Central and Loc Demo West while using Loc Demo East as the benchmark.
- For execute questions, do not call analytics or generic business-info tools again. Immediately call business.document.draft.create with document_type "generic_request", location_id "loc_hq", organization_id "org_default", confirm_apply true, and a payload containing title, summary, target_branches, benchmark_branch, target_products, target_segment, replace_campaign, and follow_up.
- After creating the draft, restate it as a draft promotion recovery request including the exact draft title, draft id, open path, target branches, target products, target segment, benchmark, and next-week follow-up.`, storeCode))
	manifest.PromptPack = retailRecoveryShowcasePromptPack(runID, storeCode, draftTitle)
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
		{"sku": finishedCode, "name": "Finished Kit " + runID, "uom_code": "EA", "inventory_enabled": true, "status": "active"},
		{"sku": componentA, "name": "Component A " + runID, "uom_code": "EA", "inventory_enabled": true, "status": "active"},
		{"sku": componentB, "name": "Component B " + runID, "uom_code": "EA", "inventory_enabled": true, "inventory_tracking_mode": "batch", "status": "active"},
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
			{"component_item_code": componentA, "quantity_per_unit": 2.0, "uom_code": "EA", "warehouse_code": "MAIN"},
			{"component_item_code": componentB, "quantity_per_unit": 1.0, "uom_code": "EA", "warehouse_code": "MAIN", "batch_code": "BATCH-" + suffix},
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
				"uom_code":       "EA",
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
			{"component_item_code": componentA, "item_code": componentA, "quantity": 6.0, "uom_code": "EA", "warehouse_code": "MAIN", "unit_cost": 5.0},
			{"component_item_code": componentB, "item_code": componentB, "quantity": 3.0, "uom_code": "EA", "warehouse_code": "MAIN", "batch_code": "BATCH-" + suffix, "unit_cost": 10.0},
		},
	})
	if err != nil {
		return scenarioManifest{}, fmt.Errorf("create production order: %w", err)
	}
	issueDoc, err := createAndApproveDocument(ctx, client, "production_issue", map[string]any{
		"source_production_order_id": productionOrder.ID,
		"warehouse_code":             "MAIN",
		"lines": []map[string]any{
			{"item_code": componentA, "quantity": 6.0, "uom_code": "EA", "warehouse_code": "MAIN", "unit_cost": 5.0},
			{"item_code": componentB, "quantity": 3.0, "uom_code": "EA", "warehouse_code": "MAIN", "batch_code": "BATCH-" + suffix, "unit_cost": 10.0},
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
	runID := time.Now().UTC().Format("20060102-150405")
	return runID, strings.ReplaceAll(runID, "-", "")
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
				"timeout": 20000,
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
		"governance_enabled":                 true,
		"default_action_mode":                "draft_only",
		"tool_states_json":                   "{}",
		"blocked_action_classes_json":        "[]",
		"blocked_tool_keys_json":             "[]",
		"blocked_document_types_json":        "[]",
		"allowed_submit_document_types_json": "[]",
		"domain_policy_overrides_json":       "{}",
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

func mustJSONString(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(data)
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
		"item.list",
		"customer.list",
		"pos_sale.list",
		"promotion_campaign.list",
		"promotion_code.list",
		"promotion_redemption.list",
		"discount_rule.list",
	}
}
