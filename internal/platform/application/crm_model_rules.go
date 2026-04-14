package application

import (
	"strings"

	"orbyte/internal/platform/model"
	"orbyte/internal/platform/shared"
)

func RegisterCRMModelRules(models *model.Service) {
	if models == nil {
		return
	}
	models.SetConstraintEvaluator("crm.ticket.lifecycle", func(input model.RuleInput) error {
		if input.ModelKey != "crm_ticket" {
			return nil
		}
		values := input.Values
		status := strings.TrimSpace(crmTextValue(values["status"]))
		existingStatus := strings.TrimSpace(crmTextValue(input.Existing["status"]))
		transitionedToStatus := existingStatus == "" || !strings.EqualFold(existingStatus, status)
		if status == "" {
			return nil
		}
		if existingStatus == "" {
			switch status {
			case "new", "open", "pending_customer", "pending_internal":
			default:
				return shared.Validation("crm tickets must start in an active status")
			}
		} else if !crmTicketTransitionAllowed(existingStatus, status) {
			return shared.Validation("crm ticket status transition is not allowed")
		}
		openedAt := parseRFC3339(values["opened_at"])
		firstResponseDueAt := parseRFC3339(values["first_response_due_at"])
		firstResponseAt := parseRFC3339(values["first_response_at"])
		dueAt := parseRFC3339(values["due_at"])
		resolvedAt := parseRFC3339(values["resolved_at"])
		if !openedAt.IsZero() {
			if !firstResponseDueAt.IsZero() && firstResponseDueAt.Before(openedAt) {
				return shared.Validation("crm ticket first_response_due_at must be on or after opened_at")
			}
			if !dueAt.IsZero() && dueAt.Before(openedAt) {
				return shared.Validation("crm ticket due_at must be on or after opened_at")
			}
			if !firstResponseAt.IsZero() && firstResponseAt.Before(openedAt) {
				return shared.Validation("crm ticket first_response_at must be on or after opened_at")
			}
			if !resolvedAt.IsZero() && resolvedAt.Before(openedAt) {
				return shared.Validation("crm ticket resolved_at must be on or after opened_at")
			}
		}
		if transitionedToStatus && (status == "resolved" || status == "closed") {
			if strings.TrimSpace(crmTextValue(values["resolution_notes"])) == "" {
				return shared.Validation("crm tickets require resolution_notes before resolved or closed")
			}
			if resolvedAt.IsZero() {
				return shared.Validation("crm tickets require resolved_at before resolved or closed")
			}
		} else if status != "resolved" && status != "closed" {
			if !resolvedAt.IsZero() {
				return shared.Validation("crm tickets may only set resolved_at in resolved or closed status")
			}
			if strings.TrimSpace(crmTextValue(values["resolution_notes"])) != "" {
				return shared.Validation("crm tickets may only set resolution_notes in resolved or closed status")
			}
		}
		return nil
	})
	models.SetConstraintEvaluator("crm.queue.active", func(input model.RuleInput) error {
		queueCode := strings.TrimSpace(crmTextValue(input.Values[input.FieldKey]))
		existingQueueCode := strings.TrimSpace(crmTextValue(input.Existing[input.FieldKey]))
		if queueCode == "" {
			return nil
		}
		if existingQueueCode != "" && strings.EqualFold(existingQueueCode, queueCode) {
			return nil
		}
		queues, _, err := models.List("crm_queue", model.Query{
			Filters:  map[string]string{"code": queueCode},
			Page:     1,
			PageSize: 2,
		})
		if err != nil {
			return err
		}
		if len(queues) == 0 {
			return nil
		}
		if strings.TrimSpace(crmTextValue(queues[0].Values["status"])) != "active" {
			return shared.Validation("crm queue references must point to an active queue")
		}
		return nil
	})
	models.SetConstraintEvaluator("crm.contact.party_link", func(input model.RuleInput) error {
		contactID := strings.TrimSpace(crmTextValue(input.Values["contact_id"]))
		if contactID == "" {
			return nil
		}
		partyID := strings.TrimSpace(crmTextValue(input.Values["party_id"]))
		if partyID == "" {
			return shared.Validation("crm records with contact_id must also set party_id")
		}
		contact, err := models.Get("party_contact", contactID)
		if err != nil {
			return err
		}
		if linkedPartyID := strings.TrimSpace(crmTextValue(contact.Values["party_id"])); linkedPartyID != "" && !strings.EqualFold(linkedPartyID, partyID) {
			return shared.Validation("crm contact_id must belong to the same party_id")
		}
		return nil
	})
	models.SetConstraintEvaluator("crm.lead.lifecycle", func(input model.RuleInput) error {
		if input.ModelKey != "crm_lead" {
			return nil
		}
		status := strings.TrimSpace(crmTextValue(input.Values["status"]))
		existingStatus := strings.TrimSpace(crmTextValue(input.Existing["status"]))
		if status == "" {
			return nil
		}
		if existingStatus != "" && !crmLeadTransitionAllowed(existingStatus, status) {
			return shared.Validation("crm lead status transition is not allowed")
		}
		return nil
	})
	models.SetConstraintEvaluator("crm.opportunity.source_lead_link", func(input model.RuleInput) error {
		sourceLeadID := strings.TrimSpace(crmTextValue(input.Values["source_lead_id"]))
		if sourceLeadID == "" {
			return nil
		}
		sourceLead, err := models.Get("crm_lead", sourceLeadID)
		if err != nil {
			return err
		}
		sourceLeadStatus := strings.TrimSpace(crmTextValue(sourceLead.Values["status"]))
		if sourceLeadStatus == "disqualified" || sourceLeadStatus == "closed" {
			return shared.Validation("crm opportunities cannot link to a disqualified or closed source lead")
		}
		if partyID := strings.TrimSpace(crmTextValue(input.Values["party_id"])); partyID != "" {
			if linkedPartyID := strings.TrimSpace(crmTextValue(sourceLead.Values["party_id"])); linkedPartyID != "" && !strings.EqualFold(linkedPartyID, partyID) {
				return shared.Validation("crm opportunity party_id must match the source lead party_id")
			}
		}
		if contactID := strings.TrimSpace(crmTextValue(input.Values["contact_id"])); contactID != "" {
			if linkedContactID := strings.TrimSpace(crmTextValue(sourceLead.Values["contact_id"])); linkedContactID != "" && !strings.EqualFold(linkedContactID, contactID) {
				return shared.Validation("crm opportunity contact_id must match the source lead contact_id")
			}
		}
		existingNumber := strings.TrimSpace(crmTextValue(input.Existing["opportunity_number"]))
		for page := 1; ; page++ {
			items, _, err := models.List("crm_opportunity", model.Query{
				Filters:  map[string]string{"source_lead_id": sourceLeadID},
				Page:     page,
				PageSize: model.MaxPageSize,
			})
			if err != nil {
				return err
			}
			for _, item := range items {
				itemNumber := strings.TrimSpace(crmTextValue(item.Values["opportunity_number"]))
				if existingNumber != "" && strings.EqualFold(existingNumber, itemNumber) {
					continue
				}
				if crmOpportunityIsOpen(crmTextValue(item.Values["stage"])) {
					return shared.Validation("crm opportunities allow only one open opportunity per source lead")
				}
			}
			if len(items) < model.MaxPageSize {
				break
			}
		}
		return nil
	})
	models.SetConstraintEvaluator("crm.opportunity.lifecycle", func(input model.RuleInput) error {
		if input.ModelKey != "crm_opportunity" {
			return nil
		}
		stage := strings.TrimSpace(crmTextValue(input.Values["stage"]))
		existingStage := strings.TrimSpace(crmTextValue(input.Existing["stage"]))
		if stage == "" {
			return nil
		}
		if existingStage != "" && !crmOpportunityTransitionAllowed(existingStage, stage) {
			return shared.Validation("crm opportunity stage transition is not allowed")
		}
		expectedStatus := crmOpportunityStatusForStage(stage)
		if status := strings.TrimSpace(crmTextValue(input.Values["status"])); status != "" && !strings.EqualFold(status, expectedStatus) {
			return shared.Validation("crm opportunity status must match the selected stage")
		}
		if stage == "lost" && strings.TrimSpace(crmTextValue(input.Values["loss_reason"])) == "" {
			return shared.Validation("crm opportunities require loss_reason when stage is lost")
		}
		if crmFloatValue(input.Values["estimated_value"]) < 0 {
			return shared.Validation("crm opportunity estimated_value must be zero or greater")
		}
		return nil
	})
	models.SetConstraintEvaluator("crm.activity.related_link", func(input model.RuleInput) error {
		if input.ModelKey != "crm_activity" {
			return nil
		}
		relatedKind := strings.TrimSpace(crmTextValue(input.Values["related_kind"]))
		relatedID := strings.TrimSpace(crmTextValue(input.Values["related_id"]))
		if !(relatedKind == "" && relatedID == "") {
			if relatedKind == "" || relatedID == "" {
				return shared.Validation("crm activities require both related_kind and related_id together")
			}
			targetModelKey := crmRelatedModelKey(relatedKind)
			if targetModelKey == "" {
				return shared.Validation("crm activity related_kind is invalid")
			}
			record, err := models.Get(targetModelKey, relatedID)
			if err != nil {
				return err
			}
			partyID := strings.TrimSpace(crmTextValue(input.Values["party_id"]))
			switch relatedKind {
			case "party":
				if partyID != "" && !strings.EqualFold(partyID, record.ID) {
					return shared.Validation("crm activity party_id must match the related party")
				}
			default:
				if partyID != "" {
					if linkedPartyID := strings.TrimSpace(crmTextValue(record.Values["party_id"])); linkedPartyID != "" && !strings.EqualFold(linkedPartyID, partyID) {
						return shared.Validation("crm activity party_id must match the related record party_id")
					}
				}
			}
		}
		status := strings.TrimSpace(crmTextValue(input.Values["status"]))
		completedAt := parseRFC3339(input.Values["completed_at"])
		if status == "completed" && completedAt.IsZero() {
			return shared.Validation("crm activities require completed_at when status is completed")
		}
		if status != "completed" && !completedAt.IsZero() {
			return shared.Validation("crm activities may only set completed_at when status is completed")
		}
		return nil
	})
	models.SetConstraintEvaluator("crm.ticket.comment.link", func(input model.RuleInput) error {
		if input.ModelKey != "crm_ticket_comment" {
			return nil
		}
		ticketID := strings.TrimSpace(crmTextValue(input.Values["ticket_id"]))
		if ticketID == "" {
			return nil
		}
		return validateCRMTicketLink(models, input.Values, "ticket_number", "party_id")
	})
	models.SetConstraintEvaluator("crm.ticket.activity.link", func(input model.RuleInput) error {
		if input.ModelKey != "crm_ticket_activity" {
			return nil
		}
		ticketID := strings.TrimSpace(crmTextValue(input.Values["ticket_id"]))
		if ticketID == "" {
			return nil
		}
		if err := validateCRMTicketLink(models, input.Values, "ticket_number", "party_id"); err != nil {
			return err
		}
		for _, key := range []string{"from_status", "to_status", "ticket_status_key"} {
			value := strings.TrimSpace(crmTextValue(input.Values[key]))
			if value == "" {
				continue
			}
			if !crmTicketStatusAllowed(value) {
				return shared.Validation("crm ticket activity " + key + " must be a valid ticket status")
			}
		}
		return nil
	})
}

func crmTicketTransitionAllowed(fromStatus, toStatus string) bool {
	fromStatus = strings.TrimSpace(fromStatus)
	toStatus = strings.TrimSpace(toStatus)
	if fromStatus == "" || toStatus == "" || strings.EqualFold(fromStatus, toStatus) {
		return true
	}
	allowed := map[string]map[string]bool{
		"new":              {"open": true, "pending_customer": true, "pending_internal": true, "resolved": true, "cancelled": true},
		"open":             {"pending_customer": true, "pending_internal": true, "resolved": true, "cancelled": true},
		"pending_customer": {"open": true, "pending_internal": true, "resolved": true, "cancelled": true},
		"pending_internal": {"open": true, "pending_customer": true, "resolved": true, "cancelled": true},
		"resolved":         {"open": true, "closed": true},
		"closed":           {},
		"cancelled":        {},
	}
	return allowed[strings.ToLower(fromStatus)][strings.ToLower(toStatus)]
}

func crmLeadTransitionAllowed(fromStatus, toStatus string) bool {
	fromStatus = strings.TrimSpace(strings.ToLower(fromStatus))
	toStatus = strings.TrimSpace(strings.ToLower(toStatus))
	if fromStatus == "" || toStatus == "" || fromStatus == toStatus {
		return true
	}
	allowed := map[string]map[string]bool{
		"new":          {"contacted": true, "qualified": true, "disqualified": true, "closed": true},
		"contacted":    {"qualified": true, "disqualified": true, "closed": true},
		"qualified":    {"converted": true, "disqualified": true, "closed": true},
		"disqualified": {},
		"converted":    {},
		"closed":       {},
	}
	return allowed[fromStatus][toStatus]
}

func crmOpportunityTransitionAllowed(fromStage, toStage string) bool {
	fromStage = strings.TrimSpace(strings.ToLower(fromStage))
	toStage = strings.TrimSpace(strings.ToLower(toStage))
	if fromStage == "" || toStage == "" || fromStage == toStage {
		return true
	}
	allowed := map[string]map[string]bool{
		"new":         {"qualified": true, "proposal": true, "negotiation": true, "won": true, "lost": true},
		"qualified":   {"proposal": true, "negotiation": true, "won": true, "lost": true},
		"proposal":    {"negotiation": true, "won": true, "lost": true},
		"negotiation": {"won": true, "lost": true},
		"won":         {},
		"lost":        {},
	}
	return allowed[fromStage][toStage]
}

func crmTicketStatusAllowed(status string) bool {
	switch strings.TrimSpace(status) {
	case "new", "open", "pending_customer", "pending_internal", "resolved", "closed", "cancelled":
		return true
	default:
		return false
	}
}

func crmRelatedModelKey(kind string) string {
	switch strings.TrimSpace(kind) {
	case "party":
		return "party"
	case "lead":
		return "crm_lead"
	case "opportunity":
		return "crm_opportunity"
	case "ticket":
		return "crm_ticket"
	default:
		return ""
	}
}

func validateCRMTicketLink(models *model.Service, values map[string]any, ticketNumberKey, partyIDKey string) error {
	ticketID := strings.TrimSpace(crmTextValue(values["ticket_id"]))
	ticket, err := models.Get("crm_ticket", ticketID)
	if err != nil {
		return err
	}
	if ticketNumber := strings.TrimSpace(crmTextValue(values[ticketNumberKey])); ticketNumber != "" && !strings.EqualFold(ticketNumber, strings.TrimSpace(crmTextValue(ticket.Values["ticket_number"]))) {
		return shared.Validation("crm ticket-linked records must keep ticket_number aligned with the linked ticket")
	}
	if partyID := strings.TrimSpace(crmTextValue(values[partyIDKey])); partyID != "" {
		if linkedPartyID := strings.TrimSpace(crmTextValue(ticket.Values["party_id"])); linkedPartyID != "" && !strings.EqualFold(partyID, linkedPartyID) {
			return shared.Validation("crm ticket-linked records must keep party_id aligned with the linked ticket")
		}
	}
	return nil
}
