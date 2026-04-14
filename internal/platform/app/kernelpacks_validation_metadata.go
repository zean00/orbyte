package app

import (
	"strings"

	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
)

func applyBuiltInValidationMetadata(manifest module.Manifest) module.Manifest {
	for modelIndex := range manifest.Models {
		def := &manifest.Models[modelIndex]
		for fieldIndex := range def.Fields {
			field := &def.Fields[fieldIndex]
			if field.Reference == nil {
				if ref, ok := builtInReferenceForField(def.Key, field.Key); ok {
					ref := ref
					field.Reference = &ref
				}
			}
			if len(field.AllowedValues) == 0 {
				if values, ok := builtInAllowedValuesForField(def.Key, field.Key); ok {
					field.AllowedValues = values
				}
			}
		}
	}
	return manifest
}

func builtInReferenceForField(modelKey, fieldKey string) (model.ReferenceDefinition, bool) {
	if strings.TrimSpace(fieldKey) == "" {
		return model.ReferenceDefinition{}, false
	}
	if fieldKey == "policy_id" {
		switch modelKey {
		case "approval_policy_stage":
			return model.ReferenceDefinition{ModelKey: "approval_policy"}, true
		case "expense_rate_rule":
			return model.ReferenceDefinition{ModelKey: "expense_policy"}, true
		default:
			return model.ReferenceDefinition{}, false
		}
	}
	refs := map[string]model.ReferenceDefinition{
		"party_id":                                 {ModelKey: "party"},
		"payable_party_id":                         {ModelKey: "party"},
		"manager_party_id":                         {ModelKey: "party"},
		"vendor_party_id":                          {ModelKey: "party"},
		"customer_party_id":                        {ModelKey: "party"},
		"payer_party_id":                           {ModelKey: "party"},
		"employee_id":                              {ModelKey: "employee_profile"},
		"manager_employee_id":                      {ModelKey: "employee_profile"},
		"organization_unit_id":                     {ModelKey: "organization_unit"},
		"operating_unit_id":                        {ModelKey: "organization_unit"},
		"parent_unit_id":                           {ModelKey: "organization_unit"},
		"department_id":                            {ModelKey: "department"},
		"parent_department_id":                     {ModelKey: "department"},
		"cost_center_id":                           {ModelKey: "cost_center"},
		"labor_cost_center_id":                     {ModelKey: "cost_center"},
		"parent_cost_center_id":                    {ModelKey: "cost_center"},
		"calendar_id":                              {ModelKey: "work_calendar"},
		"shift_template_id":                        {ModelKey: "shift_template"},
		"roster_id":                                {ModelKey: "workforce_roster"},
		"roster_slot_id":                           {ModelKey: "workforce_roster_slot"},
		"attendance_day_id":                        {ModelKey: "attendance_day"},
		"absence_code_id":                          {ModelKey: "absence_code"},
		"leave_request_id":                         {ModelKey: "leave_request"},
		"overtime_request_id":                      {ModelKey: "overtime_request"},
		"approval_policy_id":                       {ModelKey: "approval_policy"},
		"approval_policy_stage_id":                 {ModelKey: "approval_policy_stage"},
		"stage_id":                                 {ModelKey: "approval_policy_stage"},
		"approver_group_id":                        {ModelKey: "approver_group"},
		"dimension_code":                           {ModelKey: "commercial_variant_dimension", LookupField: "values.code"},
		"product_code":                             {ModelKey: "commercial_product", LookupField: "values.code"},
		"item_code":                                {ModelKey: "commercial_item", LookupField: "values.sku"},
		"commercial_item_code":                     {ModelKey: "commercial_item", LookupField: "values.sku"},
		"finished_item_code":                       {ModelKey: "commercial_item", LookupField: "values.sku"},
		"produced_item_code":                       {ModelKey: "commercial_item", LookupField: "values.sku"},
		"output_item_code":                         {ModelKey: "commercial_item", LookupField: "values.sku"},
		"category_code":                            {ModelKey: "commercial_item_category", LookupField: "values.code"},
		"uom_code":                                 {ModelKey: "commercial_uom", LookupField: "values.code"},
		"tax_code":                                 {ModelKey: "commercial_tax_code", LookupField: "values.code"},
		"default_tax_code":                         {ModelKey: "commercial_tax_code", LookupField: "values.code"},
		"tax_profile_code":                         {ModelKey: "commercial_tax_profile", LookupField: "values.code"},
		"default_tax_profile_code":                 {ModelKey: "commercial_tax_profile", LookupField: "values.code"},
		"price_list_code":                          {ModelKey: "commercial_price_list", LookupField: "values.code"},
		"default_price_list_code":                  {ModelKey: "commercial_price_list", LookupField: "values.code"},
		"payment_method_code":                      {ModelKey: "payment_method", LookupField: "values.code"},
		"revenue_account_code":                     {ModelKey: "finance_account", LookupField: "values.code"},
		"inventory_asset_account_code":             {ModelKey: "finance_account", LookupField: "values.code"},
		"cogs_account_code":                        {ModelKey: "finance_account", LookupField: "values.code"},
		"wip_account_code":                         {ModelKey: "finance_account", LookupField: "values.code"},
		"clearing_account_code":                    {ModelKey: "finance_account", LookupField: "values.code"},
		"receivable_account_code":                  {ModelKey: "finance_account", LookupField: "values.code"},
		"liability_account_code":                   {ModelKey: "finance_account", LookupField: "values.code"},
		"payroll_payable_account_code":             {ModelKey: "finance_account", LookupField: "values.code"},
		"warehouse_code":                           {ModelKey: "warehouse", LookupField: "values.code"},
		"source_warehouse_code":                    {ModelKey: "warehouse", LookupField: "values.code"},
		"target_warehouse_code":                    {ModelKey: "warehouse", LookupField: "values.code"},
		"batch_code":                               {ModelKey: "inventory_batch", LookupField: "values.batch_code"},
		"store_code":                               {ModelKey: "pos_store", LookupField: "values.code"},
		"register_code":                            {ModelKey: "pos_register", LookupField: "values.code"},
		"tender_type_code":                         {ModelKey: "pos_tender_type", LookupField: "values.code"},
		"shift_id":                                 {ModelKey: "pos_shift"},
		"reconciliation_id":                        {ModelKey: "pos_tender_reconciliation"},
		"gift_card_id":                             {ModelKey: "gift_card"},
		"accounting_period_id":                     {ModelKey: "accounting_period"},
		"journal_template_id":                      {ModelKey: "journal_template"},
		"journal_run_id":                           {ModelKey: "journal_run"},
		"fixed_asset_id":                           {ModelKey: "fixed_asset"},
		"prepaid_expense_id":                       {ModelKey: "prepaid_expense"},
		"treasury_account_id":                      {ModelKey: "treasury_account"},
		"default_treasury_account_id":              {ModelKey: "treasury_account"},
		"bank_statement_id":                        {ModelKey: "bank_statement"},
		"bank_statement_line_id":                   {ModelKey: "bank_statement_line"},
		"bank_import_template_id":                  {ModelKey: "bank_import_template"},
		"bank_reconciliation_id":                   {ModelKey: "bank_reconciliation"},
		"from_treasury_account_id":                 {ModelKey: "treasury_account"},
		"to_treasury_account_id":                   {ModelKey: "treasury_account"},
		"expense_policy_id":                        {ModelKey: "expense_policy"},
		"expense_category_code":                    {ModelKey: "expense_category", LookupField: "values.code"},
		"travel_policy_id":                         {ModelKey: "travel_policy"},
		"salary_structure_id":                      {ModelKey: "salary_structure"},
		"pay_component_id":                         {ModelKey: "pay_component"},
		"component_code":                           {ModelKey: "pay_component", LookupField: "values.code"},
		"payroll_period_id":                        {ModelKey: "payroll_period"},
		"payroll_tax_rule_id":                      {ModelKey: "payroll_tax_rule"},
		"payroll_contribution_rule_id":             {ModelKey: "payroll_contribution_rule"},
		"remittance_authority_id":                  {ModelKey: "remittance_authority"},
		"remittance_obligation_type_id":            {ModelKey: "remittance_obligation_type"},
		"withholding_obligation_type_id":           {ModelKey: "remittance_obligation_type"},
		"employee_contribution_obligation_type_id": {ModelKey: "remittance_obligation_type"},
		"employer_contribution_obligation_type_id": {ModelKey: "remittance_obligation_type"},
		"leave_policy_id":                          {ModelKey: "leave_policy"},
		"leave_entitlement_rule_id":                {ModelKey: "leave_entitlement_rule"},
		"leave_balance_account_id":                 {ModelKey: "leave_balance_account"},
		"balance_account_id":                       {ModelKey: "leave_balance_account"},
		"vendor_profile_id":                        {ModelKey: "vendor_profile"},
		"vendor_id":                                {ModelKey: "vendor_profile"},
		"vendor_item_profile_id":                   {ModelKey: "vendor_item_profile"},
		"planning_run_id":                          {ModelKey: "planning_run"},
		"production_bom_id":                        {ModelKey: "production_bom"},
		"bom_id":                                   {ModelKey: "production_bom"},
		"production_bom_version_id":                {ModelKey: "production_bom_version"},
		"work_center_code":                         {ModelKey: "production_work_center", LookupField: "values.code"},
		"production_routing_id":                    {ModelKey: "production_routing"},
		"routing_id":                               {ModelKey: "production_routing"},
		"production_routing_step_id":               {ModelKey: "production_routing_step"},
		"promotion_campaign_code":                  {ModelKey: "promotion_campaign", LookupField: "values.code"},
		"promotion_code":                           {ModelKey: "promotion_code", LookupField: "values.code"},
		"store_credit_account_id":                  {ModelKey: "store_credit_account"},
	}
	ref, ok := refs[fieldKey]
	if !ok {
		return model.ReferenceDefinition{}, false
	}
	if ref.ModelKey == modelKey {
		return model.ReferenceDefinition{}, false
	}
	return ref, true
}

func builtInAllowedValuesForField(modelKey, fieldKey string) ([]string, bool) {
	if fieldKey == "event_type" {
		if modelKey == "attendance_event" {
			return []string{"clock_in", "clock_out", "break_start", "break_end", "adjustment", "manual_adjustment"}, true
		}
		return nil, false
	}
	values := map[string][]string{
		"status":                    {"active", "inactive", "blocked", "draft", "open", "opened", "closed", "submitted", "approved", "rejected", "cancelled", "posted", "generated", "completed", "pending", "processing", "ready", "settled", "void", "failed", "archived", "issued", "partially_paid", "paid", "refunded", "held", "published", "imported"},
		"approval_status":           {"draft", "pending", "submitted", "approved", "rejected", "cancelled"},
		"employment_status":         {"active", "inactive", "terminated", "on_leave"},
		"member_status":             {"active", "inactive", "expired", "suspended"},
		"attendance_status":         {"present", "absent", "late", "leave", "overtime", "incomplete"},
		"account_type":              {"asset", "liability", "equity", "revenue", "expense"},
		"normal_balance":            {"debit", "credit"},
		"item_type":                 {"product", "service", "fee"},
		"journal_kind":              {"manual", "recurring", "accrual"},
		"generated_posting_status":  {"none", "pending", "generated", "posted", "failed"},
		"task_type":                 {"checklist", "journal", "review", "approval"},
		"reversal_status":           {"none", "pending", "reversed", "corrected"},
		"obligation_class":          {"withholding", "employee_contribution", "employer_contribution", "other"},
		"pay_basis":                 {"hourly", "salary", "daily"},
		"assignment_type":           {"primary", "secondary", "temporary"},
		"eligibility_type":          {"pos", "warehouse", "production", "field", "general"},
		"contact_kind":              {"person", "department", "support", "billing"},
		"address_type":              {"billing", "shipping", "registered", "home", "work", "other"},
		"party_kind":                {"person", "organization"},
		"customer_type":             {"retail", "business", "member", "walk_in"},
		"unit_type":                 {"company", "division", "department", "team", "store", "warehouse", "work_center"},
		"shipment_status":           {"draft", "ready", "dispatched", "delivered", "failed", "cancelled"},
		"lifecycle_status":          {"draft", "active", "inactive", "disposed", "impaired", "revalued", "transferred", "retired"},
		"last_lifecycle_event_type": {"created", "capitalized", "transferred", "impaired", "revalued", "disposed", "retired"},
		"disposal_type":             {"sale", "scrap", "write_off", "donation", "other"},
		"mismatch_type":             {"quantity", "cost", "valuation", "location", "other"},
		"tender_kind":               {"cash", "card", "voucher", "store_credit", "gift_card", "other"},
		"transaction_type":          {"issue", "redeem", "refund", "adjust", "expire", "transfer"},
		"treasury_type":             {"cash", "bank", "wallet", "clearing"},
		"match_status":              {"unmatched", "matched", "partial", "ignored"},
		"matched_source_type":       {"payment", "receipt", "transfer", "journal", "settlement", "other"},
		"source_type":               {"payment", "receipt", "transfer", "journal", "settlement", "other"},
		"match_kind":                {"exact", "partial", "manual", "rule"},
		"exception_kind":            {"missing_match", "amount_mismatch", "duplicate", "timing", "other"},
		"entry_type":                {"accrual", "usage", "adjustment", "carryover", "expiry", "opening", "annual_grant", "monthly_accrual", "carry_forward", "reservation", "release", "consumption", "reversal"},
		"run_status":                {"pending", "processing", "completed", "failed", "cancelled"},
		"source_document_type":      {"sales_order", "sales_invoice", "invoice", "credit_note", "customer_payment", "customer_refund", "purchase_order", "purchase_invoice", "vendor_bill", "vendor_payment", "vendor_credit", "fulfillment_order", "sales_fulfillment", "delivery_order", "production_order", "production_issue", "production_output", "production_cost_capture", "journal_posting", "ledger_posting", "journal_template", "payroll_run", "payroll_remittance_payment", "pos_sale", "pos_tender_reconciliation", "retail_finance", "stock_movement", "stock_receipt", "stock_issue", "stock_adjustment", "stock_transfer", "goods_receipt", "treasury_transfer", "treasury_exception", "recall_case", "other"},
		"coverage_status":           {"covered", "short", "excess", "unknown"},
		"conversion_status":         {"pending", "converted", "rejected", "cancelled"},
		"rate_type":                 {"labor", "machine", "overhead", "setup", "other"},
		"capture_type":              {"labor", "material", "machine", "overhead", "other"},
		"variance_type":             {"price", "quantity", "usage", "efficiency", "mix", "other"},
		"rule_kind":                 {"line_percent", "line_amount", "order_percent", "order_amount", "bundle", "bogo"},
		"action_type":               {"recall", "hold", "release", "dispose", "inspect"},
		"severity":                  {"low", "medium", "high", "critical"},
		"containment_mode":          {"recalled", "held", "released"},
		"campaign_kind":             {"bundle", "discount", "loyalty", "markdown", "recovery"},
		"target_segment":            {"all", "gold", "silver", "bronze", "new", "lapsed"},
	}
	allowed, ok := values[fieldKey]
	if !ok {
		return nil, false
	}
	return append([]string(nil), allowed...), true
}
