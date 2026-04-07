package app

import (
	"strings"
	"testing"
)

func TestBuiltInModuleManifestsUseVersionedDependencyRequirements(t *testing.T) {
	for _, manifest := range builtInModuleManifests() {
		if len(manifest.Dependencies) != 0 {
			t.Fatalf("module %s still uses legacy Dependencies", manifest.Key)
		}
		for _, dependency := range manifest.DependencyRequirements {
			if strings.TrimSpace(dependency.ModuleKey) == "" {
				t.Fatalf("module %s has dependency requirement without module key", manifest.Key)
			}
			if strings.TrimSpace(dependency.VersionRange) == "" {
				t.Fatalf("module %s dependency %s is missing version_range", manifest.Key, dependency.ModuleKey)
			}
			if strings.TrimSpace(string(dependency.Kind)) == "" {
				t.Fatalf("module %s dependency %s is missing kind", manifest.Key, dependency.ModuleKey)
			}
		}
	}
}

func TestBuiltInValidationMetadataReferencesKnownModels(t *testing.T) {
	knownModels := map[string]bool{}
	for _, manifest := range builtInModuleManifests() {
		for _, def := range manifest.Models {
			knownModels[def.Key] = true
		}
	}
	for _, manifest := range builtInModuleManifests() {
		for _, def := range manifest.Models {
			for _, field := range def.Fields {
				if field.Reference == nil {
					continue
				}
				if !knownModels[field.Reference.ModelKey] {
					t.Fatalf("%s.%s references unknown model %s", def.Key, field.Key, field.Reference.ModelKey)
				}
			}
		}
	}
}

func TestBuiltInValidationMetadataScopesAmbiguousFields(t *testing.T) {
	ref, ok := builtInReferenceForField("approval_policy_stage", "policy_id")
	if !ok || ref.ModelKey != "approval_policy" {
		t.Fatalf("expected approval policy stage policy_id to reference approval_policy, got %+v ok=%v", ref, ok)
	}
	ref, ok = builtInReferenceForField("expense_rate_rule", "policy_id")
	if !ok || ref.ModelKey != "expense_policy" {
		t.Fatalf("expected expense rate rule policy_id to reference expense_policy, got %+v ok=%v", ref, ok)
	}
	if _, ok := builtInReferenceForField("leave_policy", "policy_id"); ok {
		t.Fatal("expected unrecognized policy_id owner to remain unclassified")
	}

	statuses, ok := builtInAllowedValuesForField("status")
	if !ok {
		t.Fatal("expected built-in status allowlist")
	}
	for _, required := range []string{"issued", "held", "partially_paid", "paid", "refunded"} {
		if !containsTestString(statuses, required) {
			t.Fatalf("expected status allowlist to include %q, got %+v", required, statuses)
		}
	}
}

func TestRequiredReferenceLikeFieldsAreClassified(t *testing.T) {
	allowed := map[string]string{
		"organization_unit.organization_id":                        "organization_id is a tenant/org scope key; organization is not a model yet",
		"department.organization_id":                               "organization_id is a tenant/org scope key; organization is not a model yet",
		"cost_center.organization_id":                              "organization_id is a tenant/org scope key; organization is not a model yet",
		"employee_profile.employee_code":                           "employee_code is a business key for the same record",
		"employee_assignment.organization_id":                      "organization_id is a tenant/org scope key; organization is not a model yet",
		"workforce_roster.organization_id":                         "organization_id is a tenant/org scope key; organization is not a model yet",
		"workforce_roster_slot.organization_id":                    "organization_id is a tenant/org scope key; organization is not a model yet",
		"attendance_adjustment.reason_code":                        "reason_code is governed by attendance policy/service logic",
		"approver_group_member.user_id":                            "user_id is owned by identity, not the generic model registry",
		"approval_delegation_rule.approver_user_id":                "user_id is owned by identity, not the generic model registry",
		"approval_delegation_rule.delegate_user_id":                "user_id is owned by identity, not the generic model registry",
		"promotion_redemption.source_document_id":                  "source_document_id is polymorphic and validated by source document type",
		"accounting_period.organization_id":                        "organization_id is a tenant/org scope key; organization is not a model yet",
		"journal_template.organization_id":                         "organization_id is a tenant/org scope key; organization is not a model yet",
		"journal_run.organization_id":                              "organization_id is a tenant/org scope key; organization is not a model yet",
		"accounting_period_task.organization_id":                   "organization_id is a tenant/org scope key; organization is not a model yet",
		"accounting_period_task.task_code":                         "task_code is a business key for the task definition",
		"party_statement_run.organization_id":                      "organization_id is a tenant/org scope key; organization is not a model yet",
		"vendor_statement_run.organization_id":                     "organization_id is a tenant/org scope key; organization is not a model yet",
		"collection_case.organization_id":                          "organization_id is a tenant/org scope key; organization is not a model yet",
		"collection_case.counterparty_id":                          "counterparty can be customer or vendor and is validated by service logic",
		"settlement_exception.organization_id":                     "organization_id is a tenant/org scope key; organization is not a model yet",
		"fixed_asset.organization_id":                              "organization_id is a tenant/org scope key; organization is not a model yet",
		"fixed_asset_schedule.organization_id":                     "organization_id is a tenant/org scope key; organization is not a model yet",
		"prepaid_expense.organization_id":                          "organization_id is a tenant/org scope key; organization is not a model yet",
		"prepaid_schedule.organization_id":                         "organization_id is a tenant/org scope key; organization is not a model yet",
		"asset_disposal.organization_id":                           "organization_id is a tenant/org scope key; organization is not a model yet",
		"asset_transfer.organization_id":                           "organization_id is a tenant/org scope key; organization is not a model yet",
		"asset_impairment.organization_id":                         "organization_id is a tenant/org scope key; organization is not a model yet",
		"asset_revaluation.organization_id":                        "organization_id is a tenant/org scope key; organization is not a model yet",
		"inventory_count_session.organization_id":                  "organization_id is a tenant/org scope key; organization is not a model yet",
		"inventory_reconciliation_case.organization_id":            "organization_id is a tenant/org scope key; organization is not a model yet",
		"pos_tender_reconciliation.organization_id":                "organization_id is a tenant/org scope key; organization is not a model yet",
		"pos_tender_settlement.organization_id":                    "organization_id is a tenant/org scope key; organization is not a model yet",
		"gift_card.organization_id":                                "organization_id is a tenant/org scope key; organization is not a model yet",
		"gift_card_transaction.organization_id":                    "organization_id is a tenant/org scope key; organization is not a model yet",
		"store_credit_account.organization_id":                     "organization_id is a tenant/org scope key; organization is not a model yet",
		"store_credit_transaction.organization_id":                 "organization_id is a tenant/org scope key; organization is not a model yet",
		"treasury_account.organization_id":                         "organization_id is a tenant/org scope key; organization is not a model yet",
		"treasury_account.account_code":                            "account_code is a treasury account business key",
		"treasury_account.gl_account_code":                         "gl_account_code is service/config governed until finance accounts are mandatory in treasury setup",
		"bank_statement.organization_id":                           "organization_id is a tenant/org scope key; organization is not a model yet",
		"bank_statement_line.organization_id":                      "organization_id is a tenant/org scope key; organization is not a model yet",
		"bank_import_template.organization_id":                     "organization_id is a tenant/org scope key; organization is not a model yet",
		"bank_import_template.template_code":                       "template_code is a business key for the same template record",
		"bank_statement_import_run.organization_id":                "organization_id is a tenant/org scope key; organization is not a model yet",
		"bank_reconciliation.organization_id":                      "organization_id is a tenant/org scope key; organization is not a model yet",
		"bank_match_rule.organization_id":                          "organization_id is a tenant/org scope key; organization is not a model yet",
		"bank_reconciliation_match.organization_id":                "organization_id is a tenant/org scope key; organization is not a model yet",
		"bank_reconciliation_match.matched_source_id":              "matched_source_id is polymorphic and validated by matched_source_type",
		"treasury_transfer.organization_id":                        "organization_id is a tenant/org scope key; organization is not a model yet",
		"treasury_exception.organization_id":                       "organization_id is a tenant/org scope key; organization is not a model yet",
		"payroll_period.organization_id":                           "organization_id is a tenant/org scope key; organization is not a model yet",
		"inventory_batch.batch_code":                               "batch_code is a business key for the same record",
		"production_bom_version.version_code":                      "version_code is a business key for the same record",
		"production_routing.organization_id":                       "organization_id is a tenant/org scope key; organization is not a model yet",
		"production_routing_step.organization_id":                  "organization_id is a tenant/org scope key; organization is not a model yet",
		"production_cost_rate.organization_id":                     "organization_id is a tenant/org scope key; organization is not a model yet",
		"production_cost_capture.organization_id":                  "organization_id is a tenant/org scope key; organization is not a model yet",
		"production_cost_capture.production_order_id":              "production_order_id points to a document, not a generic model",
		"production_variance_case.organization_id":                 "organization_id is a tenant/org scope key; organization is not a model yet",
		"production_variance_case.production_order_id":             "production_order_id points to a document, not a generic model",
		"production_output_allocation.organization_id":             "organization_id is a tenant/org scope key; organization is not a model yet",
		"production_output_allocation.source_production_output_id": "source_production_output_id points to production output documents",
		"production_output_allocation.production_order_id":         "production_order_id points to a document, not a generic model",
		"pos_shift.cashier_user_id":                                "user_id is owned by identity, not the generic model registry",
		"pos_sale.cashier_user_id":                                 "user_id is owned by identity, not the generic model registry",
	}
	for _, manifest := range builtInModuleManifests() {
		for _, def := range manifest.Models {
			for _, field := range def.Fields {
				if !field.Required || !(strings.HasSuffix(field.Key, "_id") || strings.HasSuffix(field.Key, "_code")) {
					continue
				}
				key := def.Key + "." + field.Key
				if field.Reference == nil && len(field.ConstraintRuleKeys) == 0 {
					if _, ok := allowed[key]; !ok {
						t.Fatalf("%s must declare reference metadata, constraint rules, or an allowlist reason", key)
					}
				}
			}
		}
	}
}

func containsTestString(items []string, candidate string) bool {
	for _, item := range items {
		if item == candidate {
			return true
		}
	}
	return false
}
