package httpx

import (
	"sort"
	"strings"
	"time"

	"orbyte/internal/platform/identity"
)

func hierarchyGraphData(ident *identity.Service, organizationID, locationID, operatingUnitID, status string) ([]adminHierarchyNode, []adminHierarchyEdge) {
	lines := ident.ReportingLines()
	filteredEdges := make([]adminHierarchyEdge, 0)
	includedUserIDs := map[string]struct{}{}
	status = strings.TrimSpace(status)
	for _, line := range lines {
		if !reportingLineMatchesAdminFilters(line, organizationID, locationID, operatingUnitID, status) {
			continue
		}
		filteredEdges = append(filteredEdges, adminHierarchyEdge{
			ID:               line.ID,
			SubjectUserID:    line.SubjectUserID,
			ManagerUserID:    line.ManagerUserID,
			RelationshipType: line.RelationshipType,
			Status:           line.Status,
			OrganizationID:   line.OrganizationID,
			LocationID:       line.LocationID,
			OperatingUnitID:  line.OperatingUnitID,
			Priority:         line.Priority,
			EffectiveFrom:    line.EffectiveFrom,
			EffectiveTo:      line.EffectiveTo,
		})
		includedUserIDs[line.SubjectUserID] = struct{}{}
		includedUserIDs[line.ManagerUserID] = struct{}{}
	}
	users := ident.Users()
	nodes := make([]adminHierarchyNode, 0)
	for _, user := range users {
		if locationID != "" && user.DefaultLocationID != "" && user.DefaultLocationID != locationID {
			if _, ok := includedUserIDs[user.ID]; !ok {
				continue
			}
		}
		if len(includedUserIDs) > 0 {
			if _, ok := includedUserIDs[user.ID]; !ok && locationID != "" {
				continue
			}
		}
		nodes = append(nodes, adminHierarchyNode{
			ID:                user.ID,
			Username:          user.Username,
			Status:            user.Status,
			DefaultLocationID: user.DefaultLocationID,
		})
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Username == nodes[j].Username {
			return nodes[i].ID < nodes[j].ID
		}
		return nodes[i].Username < nodes[j].Username
	})
	sort.Slice(filteredEdges, func(i, j int) bool {
		if filteredEdges[i].SubjectUserID == filteredEdges[j].SubjectUserID {
			if filteredEdges[i].Priority == filteredEdges[j].Priority {
				return filteredEdges[i].ID < filteredEdges[j].ID
			}
			return filteredEdges[i].Priority > filteredEdges[j].Priority
		}
		return filteredEdges[i].SubjectUserID < filteredEdges[j].SubjectUserID
	})
	return nodes, filteredEdges
}

func hierarchySummary(nodes []adminHierarchyNode, edges []adminHierarchyEdge) adminHierarchySummary {
	summary := adminHierarchySummary{TotalUsers: len(nodes)}
	resolvedManagers := map[string]bool{}
	now := time.Now().UTC()
	for _, edge := range edges {
		if edge.Status == "active" && !edge.EffectiveFrom.After(now) && (edge.EffectiveTo.IsZero() || !edge.EffectiveTo.Before(now)) {
			summary.ActiveLines++
			resolvedManagers[edge.SubjectUserID] = true
			if edge.RelationshipType == "acting_manager" {
				summary.ActingOverrides++
			}
		}
	}
	for _, node := range nodes {
		if !resolvedManagers[node.ID] {
			summary.OrphanUsers++
		}
	}
	return summary
}

func hierarchyChain(ident *identity.Service, userID, organizationID, locationID, operatingUnitID string) []map[string]any {
	items := make([]map[string]any, 0)
	visited := map[string]bool{}
	currentUserID := strings.TrimSpace(userID)
	for currentUserID != "" && !visited[currentUserID] {
		visited[currentUserID] = true
		user, ok := ident.FindUser(currentUserID)
		if !ok {
			break
		}
		entry := map[string]any{
			"user_id":   user.ID,
			"username":  user.Username,
			"user":      user,
			"is_origin": len(items) == 0,
		}
		resolution, ok := ident.ResolveManager(currentUserID, organizationID, locationID, operatingUnitID, time.Now().UTC())
		if ok {
			entry["manager_user_id"] = resolution.Manager.ID
			entry["manager_username"] = resolution.Manager.Username
			entry["resolved_via"] = resolution.Via
			entry["line"] = resolution.Line
			currentUserID = resolution.Manager.ID
		} else {
			currentUserID = ""
		}
		items = append(items, entry)
	}
	return items
}

func reportingLineMatchesAdminFilters(line identity.ReportingLine, organizationID, locationID, operatingUnitID, status string) bool {
	if organizationID != "" && line.OrganizationID != "" && line.OrganizationID != organizationID {
		return false
	}
	if locationID != "" && line.LocationID != "" && line.LocationID != locationID {
		return false
	}
	if operatingUnitID != "" && line.OperatingUnitID != "" && line.OperatingUnitID != operatingUnitID {
		return false
	}
	if status != "" && line.Status != status {
		return false
	}
	return true
}
