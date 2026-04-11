package main

import (
	"strings"
	"testing"
)

func TestDefaultMCPPlaybooksJSONIncludesCRMOverview(t *testing.T) {
	items := defaultMCPPlaybooksJSON()
	if !strings.Contains(items, `"id":"crm_service_sales_overview"`) {
		t.Fatalf("expected CRM playbook in default MCP playbooks json, got %s", items)
	}
	if !strings.Contains(items, `"crm.ticket.summary"`) {
		t.Fatalf("expected CRM ticket summary tool in default MCP playbooks json, got %s", items)
	}
	if !strings.Contains(items, `"crm.opportunity.pipeline.summary"`) {
		t.Fatalf("expected CRM opportunity pipeline tool in default MCP playbooks json, got %s", items)
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
