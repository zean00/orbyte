package app

import "testing"

func TestLeavePolicyCoreManifestIncludesSelfServiceAndBalanceModels(t *testing.T) {
	manifest := leavePolicyCoreKernelPackManifest()
	for _, modelKey := range []string{
		"leave_policy",
		"leave_entitlement_rule",
		"employee_leave_profile",
		"leave_balance_account",
		"leave_balance_entry",
		"leave_accrual_run",
		"leave_balance_adjustment",
	} {
		if !modelHasField(manifest, modelKey, "status") {
			t.Fatalf("expected %s to be registered with status field", modelKey)
		}
	}
	if len(manifest.SelfService.APIs) == 0 {
		t.Fatal("expected leave policy core to publish self-service APIs")
	}
	foundBalances := false
	foundSubmit := false
	for _, item := range manifest.SelfService.APIs {
		switch item.Key {
		case "leave.self_service.balances.list":
			foundBalances = true
		case "leave.self_service.requests.submit":
			foundSubmit = true
		}
	}
	if !foundBalances {
		t.Fatal("expected leave.self_service.balances.list API")
	}
	if !foundSubmit {
		t.Fatal("expected leave.self_service.requests.submit API")
	}
}

func TestWorkforceAttendanceLeaveRequestIncludesPolicyFields(t *testing.T) {
	manifest := workforceAttendanceKernelPackManifest()
	for _, field := range []string{
		"leave_policy_id",
		"employee_leave_profile_id",
		"balance_account_id",
		"request_unit",
		"requested_days",
		"half_day_session",
		"request_source",
		"self_service_actor_user_id",
		"approved_days",
		"reservation_entry_ids_json",
		"consumption_entry_ids_json",
	} {
		if !modelHasField(manifest, "leave_request", field) {
			t.Fatalf("expected leave_request to include %s", field)
		}
	}
}
