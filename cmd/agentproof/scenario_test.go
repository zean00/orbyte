package main

import (
	"strings"
	"testing"
)

func TestDefaultMCPPlaybooksJSONIncludesCRMPlaybooks(t *testing.T) {
	items := defaultMCPPlaybooksJSON()
	for _, marker := range []string{`"id":"crm_service_backlog_triage"`, `"id":"crm_customer_360_review"`, `"id":"crm_sales_pipeline_review"`, `"id":"crm_service_sales_overview"`} {
		if !strings.Contains(items, marker) {
			t.Fatalf("expected %s in default MCP playbooks json, got %s", marker, items)
		}
	}
	for _, marker := range []string{`"crm.ticket.summary"`, `"crm.customer.summary"`, `"crm.customer.timeline"`, `"crm.opportunity.pipeline.summary"`} {
		if !strings.Contains(items, marker) {
			t.Fatalf("expected %s in default MCP playbooks json, got %s", marker, items)
		}
	}
}

func TestAgentproofMCPOperationsIncludeCRMReadAndWriteScopes(t *testing.T) {
	ops := agentproofMCPOperations()
	for _, op := range []string{
		"crm_ticket.list",
		"crm_ticket.read",
		"crm_ticket.create",
		"crm_ticket.update",
		"crm_ticket_comment.create",
		"crm_lead.list",
		"crm_lead.read",
		"crm_opportunity.list",
		"crm_opportunity.read",
	} {
		found := false
		for _, current := range ops {
			if current == op {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected %s in agentproof MCP operations", op)
		}
	}
}
