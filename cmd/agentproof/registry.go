package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type scenarioDefinition struct {
	Key          string
	DomainBundle string
	Description  string
	Seed         func(context.Context, *apiClient, string, string) (scenarioManifest, error)
}

func scenarioDefinitions() []scenarioDefinition {
	return []scenarioDefinition{
		{Key: "employee_spend", DomainBundle: "workforce-expense", Description: "Employee spend from travel request through reimbursement.", Seed: seedEmployeeSpendScenario},
		{Key: "order_to_cash", DomainBundle: "commercial-fulfillment", Description: "Sales order through fulfillment, delivery, invoice, payment, and return reversal.", Seed: seedOrderToCashScenario},
		{Key: "procure_to_pay_inventory", DomainBundle: "procurement-inventory", Description: "Purchase order through receipt, vendor bill, payment, and supplier return credit.", Seed: seedProcureToPayInventoryScenario},
		{Key: "pos_promotion_strategy", DomainBundle: "retail-promotion", Description: "Retail POS sales and promotion setup seeded for campaign-design strategy and draft creation.", Seed: seedPOSPromotionStrategyScenario},
		{Key: "leave_to_payroll", DomainBundle: "workforce-payroll", Description: "Approved leave reflected into payroll and payment state.", Seed: seedLeaveToPayrollScenario},
		{Key: "payroll_remittance", DomainBundle: "payroll-treasury", Description: "Payroll remittance liabilities, batching, and remittance payment.", Seed: seedPayrollRemittanceScenario},
		{Key: "production_costing", DomainBundle: "production-inventory", Description: "Production order, issue, output, and resulting cost summary facts.", Seed: seedProductionCostingScenario},
	}
}

func lookupScenarioDefinition(key string) (scenarioDefinition, error) {
	needle := strings.TrimSpace(strings.ToLower(key))
	for _, def := range scenarioDefinitions() {
		if def.Key == needle {
			return def, nil
		}
	}
	return scenarioDefinition{}, fmt.Errorf("unknown scenario %q", key)
}

func listScenarioDefinitions() []scenarioDefinition {
	defs := append([]scenarioDefinition(nil), scenarioDefinitions()...)
	sort.Slice(defs, func(i, j int) bool { return defs[i].Key < defs[j].Key })
	return defs
}
