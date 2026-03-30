package application

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/shared"
)

type FinancePeriodEndCoreService struct {
	documents *document.Service
	models    *model.Service
	finance   *FinanceReportingCoreService
}

type FinancePeriodCloseTask struct {
	ID              string `json:"id"`
	TaskCode        string `json:"task_code"`
	Label           string `json:"label"`
	TaskType        string `json:"task_type"`
	Status          string `json:"status"`
	Required        bool   `json:"required"`
	JournalTemplate string `json:"journal_template_id,omitempty"`
	JournalRunID    string `json:"journal_run_id,omitempty"`
	PostingID       string `json:"posting_id,omitempty"`
	PostingNumber   string `json:"posting_number,omitempty"`
	Note            string `json:"note,omitempty"`
}

type FinanceJournalRunItem struct {
	ID             string `json:"id"`
	TemplateID     string `json:"template_id"`
	TemplateCode   string `json:"template_code"`
	TemplateName   string `json:"template_name"`
	JournalKind    string `json:"journal_kind"`
	Cadence        string `json:"cadence"`
	Status         string `json:"status"`
	PostingID      string `json:"posting_id,omitempty"`
	PostingNumber  string `json:"posting_number,omitempty"`
	PostingStatus  string `json:"posting_status,omitempty"`
	PostingDate    string `json:"posting_date,omitempty"`
	ReversalStatus string `json:"reversal_status,omitempty"`
}

type FinancePeriodClosePack struct {
	PeriodID    string                  `json:"period_id"`
	PeriodKey   string                  `json:"period_key"`
	Status      string                  `json:"status"`
	Ready       bool                    `json:"ready"`
	Blockers    []string                `json:"blockers"`
	Tasks       []FinancePeriodCloseTask`json:"tasks"`
	JournalRuns []FinanceJournalRunItem `json:"journal_runs"`
}

func NewFinancePeriodEndCoreService(documents *document.Service, models *model.Service, finance *FinanceReportingCoreService) *FinancePeriodEndCoreService {
	return &FinancePeriodEndCoreService{documents: documents, models: models, finance: finance}
}

func (s *FinancePeriodEndCoreService) ReadClosePack(periodID, organizationID, locationID string) (FinancePeriodClosePack, error) {
	period, err := s.periodRecord(periodID, organizationID, locationID)
	if err != nil {
		return FinancePeriodClosePack{}, err
	}
	return s.buildClosePackReadOnly(period)
}

func (s *FinancePeriodEndCoreService) ClosePack(periodID, actorID, organizationID, locationID string) (FinancePeriodClosePack, error) {
	period, err := s.periodRecord(periodID, organizationID, locationID)
	if err != nil {
		return FinancePeriodClosePack{}, err
	}
	if err := s.ensureClosePackTasks(period, actorID); err != nil {
		return FinancePeriodClosePack{}, err
	}
	if err := s.syncPeriodJournalRuns(period, actorID); err != nil {
		return FinancePeriodClosePack{}, err
	}
	return s.buildClosePack(period)
}

func (s *FinancePeriodEndCoreService) GenerateJournalRuns(periodID, actorID, organizationID, locationID string) (FinancePeriodClosePack, error) {
	period, err := s.periodRecord(periodID, organizationID, locationID)
	if err != nil {
		return FinancePeriodClosePack{}, err
	}
	templates, err := s.dueTemplates(period)
	if err != nil {
		return FinancePeriodClosePack{}, err
	}
	for _, tmpl := range templates {
		if err := s.ensureJournalRun(period, tmpl, actorID); err != nil {
			return FinancePeriodClosePack{}, err
		}
	}
	if err := s.ensureClosePackTasks(period, actorID); err != nil {
		return FinancePeriodClosePack{}, err
	}
	if err := s.syncPeriodJournalRuns(period, actorID); err != nil {
		return FinancePeriodClosePack{}, err
	}
	return s.buildClosePack(period)
}

func (s *FinancePeriodEndCoreService) CloseAccountingPeriod(periodID, actorID, organizationID, locationID string) (model.Record, error) {
	period, err := s.periodRecord(periodID, organizationID, locationID)
	if err != nil {
		return model.Record{}, err
	}
	if err := s.ensureClosePackTasks(period, actorID); err != nil {
		return model.Record{}, err
	}
	if err := s.syncPeriodJournalRuns(period, actorID); err != nil {
		return model.Record{}, err
	}
	pack, err := s.buildClosePack(period)
	if err != nil {
		return model.Record{}, err
	}
	if !pack.Ready {
		return model.Record{}, shared.Conflict(firstNonEmptyString(strings.Join(pack.Blockers, "; "), "period close checklist is not ready"))
	}
	if s.finance == nil {
		return model.Record{}, shared.Validation("finance reporting is unavailable")
	}
	return s.finance.CloseAccountingPeriod(periodID, actorID, organizationID, locationID)
}

func (s *FinancePeriodEndCoreService) ReopenAccountingPeriod(periodID, actorID, organizationID, locationID string) (model.Record, error) {
	if s.finance == nil {
		return model.Record{}, shared.Validation("finance reporting is unavailable")
	}
	return s.finance.ReopenAccountingPeriod(periodID, actorID, organizationID, locationID)
}

func (s *FinancePeriodEndCoreService) CompleteTask(taskID, actorID, organizationID, locationID string) (model.Record, error) {
	task, err := s.scopedTask(taskID, organizationID, locationID)
	if err != nil {
		return model.Record{}, err
	}
	if strings.EqualFold(textValue(task.Values["task_type"]), "journal") {
		return model.Record{}, shared.Conflict("journal-backed tasks complete automatically from posted journal runs")
	}
	values := cloneMap(task.Values)
	values["status"] = "completed"
	values["completed_at"] = time.Now().UTC().Format(time.RFC3339)
	values["completed_by"] = actorID
	return s.models.Update("accounting_period_task", task.ID, actorID, values, task.Version)
}

func (s *FinancePeriodEndCoreService) WaiveTask(taskID, actorID, organizationID, locationID string) (model.Record, error) {
	task, err := s.scopedTask(taskID, organizationID, locationID)
	if err != nil {
		return model.Record{}, err
	}
	if strings.EqualFold(textValue(task.Values["task_type"]), "journal") {
		return model.Record{}, shared.Conflict("journal-backed tasks cannot be waived directly")
	}
	values := cloneMap(task.Values)
	values["status"] = "waived"
	values["completed_at"] = time.Now().UTC().Format(time.RFC3339)
	values["completed_by"] = actorID
	return s.models.Update("accounting_period_task", task.ID, actorID, values, task.Version)
}

func (s *FinancePeriodEndCoreService) ReverseAccrualPosting(postingID, reversalDate, actorID, organizationID, locationID string) (document.Record, error) {
	return s.reverseJournalPosting(postingID, reversalDate, actorID, organizationID, locationID, map[string]struct{}{"accrual": {}})
}

func (s *FinancePeriodEndCoreService) ReverseJournalPosting(postingID, reversalDate, actorID, organizationID, locationID string) (document.Record, error) {
	return s.reverseJournalPosting(postingID, reversalDate, actorID, organizationID, locationID, map[string]struct{}{"accrual": {}, "manual": {}})
}

func (s *FinancePeriodEndCoreService) reverseJournalPosting(postingID, reversalDate, actorID, organizationID, locationID string, allowedKinds map[string]struct{}) (document.Record, error) {
	if s.documents == nil {
		return document.Record{}, shared.Validation("documents are unavailable")
	}
	source, err := s.documents.Get(strings.TrimSpace(postingID))
	if err != nil {
		return document.Record{}, err
	}
	if source.Header.Type != "ledger_posting" {
		return document.Record{}, shared.Validation("only ledger postings can be reversed")
	}
	if strings.TrimSpace(source.Header.OrganizationID) != strings.TrimSpace(organizationID) {
		return document.Record{}, shared.Forbidden("posting is outside the current organization scope")
	}
	if strings.TrimSpace(locationID) != "" && strings.TrimSpace(source.Header.LocationID) != strings.TrimSpace(locationID) {
		return document.Record{}, shared.Forbidden("posting is outside the current location scope")
	}
	if strings.TrimSpace(source.Header.Status) != "posted" {
		return document.Record{}, shared.Conflict("only posted journals can be reversed")
	}
	payload := cloneMap(source.Body.Payload)
	journalKind := strings.TrimSpace(textValue(payload["journal_source_kind"]))
	if _, ok := allowedKinds[journalKind]; !ok {
		return document.Record{}, shared.Conflict("journal type cannot be reversed from this action")
	}
	if strings.TrimSpace(textValue(payload["reversed_by_posting_id"])) != "" {
		return document.Record{}, shared.Conflict("journal already has a reversal")
	}
	reversalDate = strings.TrimSpace(reversalDate)
	if reversalDate == "" {
		reversalDate = s.defaultReversalDate(source)
	}
	if reversalDate == "" {
		return document.Record{}, shared.Validation("reversal date is required")
	}
	if s.finance != nil {
		if err := s.finance.ValidatePostingDateOpen(source.Header.OrganizationID, source.Header.LocationID, reversalDate); err != nil {
			return document.Record{}, err
		}
	}
	reversalPayload := map[string]any{
		"source_document_type":   "ledger_posting",
		"source_document_id":     source.Header.ID,
		"posting_date":           reversalDate,
		"currency_code":          firstNonEmptyString(textValue(payload["currency_code"]), source.Header.TotalAmount.Currency, "IDR"),
		"posting_rule_key":       "journal_reversal",
		"total_amount":           roundMoney(numberValue(payload["total_amount"])),
		"journal_lines":          reverseJournalLines(recordList(payload["journal_lines"])),
		"notes":                  fmt.Sprintf("Reversal of %s", firstNonEmptyString(source.Header.Number, source.Header.ID)),
		"journal_source_kind":    "reversal",
		"reversal_of_posting_id": source.Header.ID,
		"reversal_status":        "not_applicable",
		"accounting_period_id":   s.periodIDForDate(source.Header.OrganizationID, source.Header.LocationID, reversalDate),
	}
	reversal, err := s.documents.Create("ledger_posting", source.Header.OrganizationID, source.Header.LocationID, actorID, reversalPayload)
	if err != nil {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(reversal.Header.ID, source.Header.ID, "posting_for", map[string]any{"posting_reason": "reversal_of"}); err != nil && !isConflict(err) {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(source.Header.ID, reversal.Header.ID, "posting_for", map[string]any{"posting_reason": "reversal_pair"}); err != nil && !isConflict(err) {
		return document.Record{}, err
	}
	payload["reversed_by_posting_id"] = reversal.Header.ID
	payload["reversal_status"] = "generated"
	source.Body.Payload = payload
	source.Body.ContentHash = document.ContentHash(payload)
	source.Header.UpdatedAt = time.Now().UTC()
	source.Header.UpdatedBy = actorID
	if err := s.documents.Save(source); err != nil {
		return document.Record{}, err
	}
	return s.documents.Get(reversal.Header.ID)
}

func (s *FinancePeriodEndCoreService) HandleApprovedLedgerPosting(record document.Record, actorID string) error {
	if record.Header.Type != "ledger_posting" {
		return nil
	}
	payload := cloneMap(record.Body.Payload)
	if strings.TrimSpace(textValue(payload["journal_source_kind"])) == "reversal" {
		sourceID := strings.TrimSpace(textValue(payload["reversal_of_posting_id"]))
		if sourceID != "" {
			if err := s.markSourcePostingReversed(sourceID, record.Header.ID, actorID, true); err != nil {
				return err
			}
		}
	}
	if runID := strings.TrimSpace(textValue(payload["journal_run_id"])); runID != "" {
		return s.syncRunByID(runID, actorID)
	}
	return nil
}

func (s *FinancePeriodEndCoreService) HandleCanceledLedgerPosting(record document.Record, actorID string) error {
	if record.Header.Type != "ledger_posting" {
		return nil
	}
	payload := cloneMap(record.Body.Payload)
	if strings.TrimSpace(textValue(payload["journal_source_kind"])) == "reversal" {
		sourceID := strings.TrimSpace(textValue(payload["reversal_of_posting_id"]))
		if sourceID != "" {
			if err := s.markSourcePostingReversed(sourceID, record.Header.ID, actorID, false); err != nil {
				return err
			}
		}
	}
	if runID := strings.TrimSpace(textValue(payload["journal_run_id"])); runID != "" {
		return s.syncRunByID(runID, actorID)
	}
	return nil
}

func (s *FinancePeriodEndCoreService) periodRecord(periodID, organizationID, locationID string) (model.Record, error) {
	if s.models == nil {
		return model.Record{}, shared.Validation("models are unavailable")
	}
	record, err := s.models.Get("accounting_period", strings.TrimSpace(periodID))
	if err != nil {
		return model.Record{}, err
	}
	recordOrgID := strings.TrimSpace(textValue(record.Values["organization_id"]))
	recordLocationID := strings.TrimSpace(textValue(record.Values["location_id"]))
	if strings.TrimSpace(organizationID) != "" && recordOrgID != strings.TrimSpace(organizationID) {
		return model.Record{}, shared.Forbidden("accounting period is outside the current organization scope")
	}
	if strings.TrimSpace(locationID) != "" && recordLocationID != strings.TrimSpace(locationID) {
		return model.Record{}, shared.Forbidden("accounting period is outside the current location scope")
	}
	return record, nil
}

func (s *FinancePeriodEndCoreService) scopedTask(taskID, organizationID, locationID string) (model.Record, error) {
	task, err := s.models.Get("accounting_period_task", strings.TrimSpace(taskID))
	if err != nil {
		return model.Record{}, err
	}
	taskOrgID := strings.TrimSpace(textValue(task.Values["organization_id"]))
	taskLocationID := strings.TrimSpace(textValue(task.Values["location_id"]))
	if strings.TrimSpace(organizationID) != "" && taskOrgID != strings.TrimSpace(organizationID) {
		return model.Record{}, shared.Forbidden("task is outside the current organization scope")
	}
	if strings.TrimSpace(locationID) != "" && taskLocationID != strings.TrimSpace(locationID) {
		return model.Record{}, shared.Forbidden("task is outside the current location scope")
	}
	return task, nil
}

func (s *FinancePeriodEndCoreService) dueTemplates(period model.Record) ([]model.Record, error) {
	if s.models == nil {
		return nil, nil
	}
	items, _, err := s.models.List("journal_template", model.Query{
		Filters: map[string]string{
			"organization_id": strings.TrimSpace(textValue(period.Values["organization_id"])),
			"status":          "active",
		},
		Page:     1,
		PageSize: 1000,
	})
	if err != nil && !isMissingModelDefinitionError(err) {
		return nil, err
	}
	periodEnd := strings.TrimSpace(textValue(period.Values["end_date"]))
	endTime, _ := time.Parse("2006-01-02", periodEnd)
	results := make([]model.Record, 0, len(items))
	for _, item := range items {
		tmplLocation := strings.TrimSpace(textValue(item.Values["location_id"]))
		periodLocation := strings.TrimSpace(textValue(period.Values["location_id"]))
		if tmplLocation != "" && tmplLocation != periodLocation {
			continue
		}
		if !templateDueForDate(strings.TrimSpace(textValue(item.Values["cadence"])), endTime) {
			continue
		}
		nextDueDate := strings.TrimSpace(textValue(item.Values["next_due_date"]))
		if nextDueDate != "" {
			dueTime, parseErr := time.Parse("2006-01-02", nextDueDate)
			if parseErr == nil && dueTime.After(endTime) {
				continue
			}
		}
		results = append(results, item)
	}
	sort.Slice(results, func(i, j int) bool {
		return strings.TrimSpace(textValue(results[i].Values["code"])) < strings.TrimSpace(textValue(results[j].Values["code"]))
	})
	return results, nil
}

func templateDueForDate(cadence string, endTime time.Time) bool {
	if endTime.IsZero() {
		return false
	}
	switch strings.TrimSpace(strings.ToLower(cadence)) {
	case "", "monthly":
		return true
	case "quarterly":
		return endTime.Month() == time.March || endTime.Month() == time.June || endTime.Month() == time.September || endTime.Month() == time.December
	case "yearly":
		return endTime.Month() == time.December
	default:
		return false
	}
}

func (s *FinancePeriodEndCoreService) ensureJournalRun(period, tmpl model.Record, actorID string) error {
	existing, err := s.runForTemplatePeriod(period.ID, tmpl.ID)
	if err != nil {
		return err
	}
	if existing.ID != "" {
		if err := s.syncSingleRun(existing, actorID); err != nil {
			return err
		}
		return nil
	}
	journalLines := cloneRecordList(recordList(tmpl.Values["journal_lines"]))
	totalAmount := 0.0
	for _, line := range journalLines {
		totalAmount = roundMoney(totalAmount + numberValue(line["debit"]))
	}
	postingDate := strings.TrimSpace(textValue(period.Values["end_date"]))
	reversalStatus := "not_applicable"
	if strings.TrimSpace(textValue(tmpl.Values["journal_kind"])) == "accrual" {
		reversalStatus = "available"
	}
	postingPayload := map[string]any{
		"source_document_type": "journal_template",
		"source_document_id":   tmpl.ID,
		"posting_date":         postingDate,
		"currency_code":        firstNonEmptyString(textValue(tmpl.Values["currency_code"]), "IDR"),
		"posting_rule_key":     "period_end_" + strings.TrimSpace(textValue(tmpl.Values["journal_kind"])),
		"total_amount":         totalAmount,
		"journal_lines":        journalLines,
		"notes":                firstNonEmptyString(textValue(tmpl.Values["description_template"]), textValue(tmpl.Values["name"])),
		"journal_source_kind":  strings.TrimSpace(textValue(tmpl.Values["journal_kind"])),
		"journal_template_id":  tmpl.ID,
		"accounting_period_id": period.ID,
		"reversal_status":     reversalStatus,
	}
	posting, err := s.documents.Create("ledger_posting", textValue(period.Values["organization_id"]), textValue(period.Values["location_id"]), actorID, postingPayload)
	if err != nil {
		return err
	}
	runValues := map[string]any{
		"organization_id":      textValue(period.Values["organization_id"]),
		"location_id":          textValue(period.Values["location_id"]),
		"accounting_period_id": period.ID,
		"period_key":           textValue(period.Values["period_key"]),
		"journal_template_id":  tmpl.ID,
		"template_code":        textValue(tmpl.Values["code"]),
		"template_name":        textValue(tmpl.Values["name"]),
		"cadence":              textValue(tmpl.Values["cadence"]),
		"journal_kind":         textValue(tmpl.Values["journal_kind"]),
		"posting_date":         postingDate,
		"generated_posting_id": posting.Header.ID,
		"status":               "generated",
	}
	run, err := s.models.Create("journal_run", actorID, runValues)
	if err != nil {
		return err
	}
	postingPayload = cloneMap(posting.Body.Payload)
	postingPayload["journal_run_id"] = run.ID
	posting.Body.Payload = postingPayload
	posting.Body.ContentHash = document.ContentHash(postingPayload)
	posting.Header.UpdatedAt = time.Now().UTC()
	posting.Header.UpdatedBy = actorID
	if err := s.documents.Save(posting); err != nil {
		return err
	}
	return nil
}

func (s *FinancePeriodEndCoreService) ensureClosePackTasks(period model.Record, actorID string) error {
	if s.models == nil {
		return nil
	}
	requiredChecklist := []struct {
		code  string
		label string
	}{
		{code: "reconcile_ar", label: "Reconcile AR"},
		{code: "reconcile_ap", label: "Reconcile AP"},
		{code: "review_journal_ledger", label: "Review Journal Ledger"},
		{code: "review_tax_summary", label: "Review Tax Summary"},
	}
	existing, err := s.periodTasks(period.ID)
	if err != nil {
		return err
	}
	byCode := map[string]model.Record{}
	for _, item := range existing {
		byCode[strings.TrimSpace(textValue(item.Values["task_code"]))] = item
	}
	for _, task := range requiredChecklist {
		if _, ok := byCode[task.code]; ok {
			continue
		}
		if _, err := s.models.Create("accounting_period_task", actorID, map[string]any{
			"organization_id":      textValue(period.Values["organization_id"]),
			"location_id":          textValue(period.Values["location_id"]),
			"accounting_period_id": period.ID,
			"period_key":           textValue(period.Values["period_key"]),
			"task_code":            task.code,
			"label":                task.label,
			"task_type":            "checklist",
			"status":               "pending",
			"required":             true,
		}); err != nil {
			return err
		}
	}
	templates, err := s.dueTemplates(period)
	if err != nil {
		return err
	}
	for _, tmpl := range templates {
		if !boolValue(tmpl.Values["required_for_period_close"]) {
			continue
		}
		taskCode := "journal:" + tmpl.ID
		existingTask, ok := byCode[taskCode]
		if !ok {
			run, _ := s.runForTemplatePeriod(period.ID, tmpl.ID)
			values := map[string]any{
				"organization_id":      textValue(period.Values["organization_id"]),
				"location_id":          textValue(period.Values["location_id"]),
				"accounting_period_id": period.ID,
				"period_key":           textValue(period.Values["period_key"]),
				"task_code":            taskCode,
				"label":                firstNonEmptyString(textValue(tmpl.Values["name"]), textValue(tmpl.Values["code"])),
				"task_type":            "journal",
				"status":               "pending",
				"required":             true,
				"journal_template_id":  tmpl.ID,
				"journal_run_id":       run.ID,
				"posting_id":           textValue(run.Values["generated_posting_id"]),
			}
			if _, err := s.models.Create("accounting_period_task", actorID, values); err != nil {
				return err
			}
			continue
		}
		run, _ := s.runForTemplatePeriod(period.ID, tmpl.ID)
		values := cloneMap(existingTask.Values)
		values["journal_template_id"] = tmpl.ID
		values["journal_run_id"] = run.ID
		values["posting_id"] = textValue(run.Values["generated_posting_id"])
		if _, err := s.models.Update("accounting_period_task", existingTask.ID, actorID, values, existingTask.Version); err != nil {
			return err
		}
	}
	return nil
}

func (s *FinancePeriodEndCoreService) buildClosePack(period model.Record) (FinancePeriodClosePack, error) {
	tasks, err := s.periodTasks(period.ID)
	if err != nil {
		return FinancePeriodClosePack{}, err
	}
	runs, err := s.periodRuns(period.ID)
	if err != nil {
		return FinancePeriodClosePack{}, err
	}
	items := make([]FinancePeriodCloseTask, 0, len(tasks))
	blockers := make([]string, 0)
	ready := true
	for _, task := range tasks {
		status := strings.TrimSpace(textValue(task.Values["status"]))
		required := boolValue(task.Values["required"])
		if required && status != "completed" && status != "waived" {
			ready = false
			blockers = append(blockers, firstNonEmptyString(textValue(task.Values["label"]), textValue(task.Values["task_code"]))+" is still pending")
		}
		items = append(items, FinancePeriodCloseTask{
			ID:              task.ID,
			TaskCode:        textValue(task.Values["task_code"]),
			Label:           textValue(task.Values["label"]),
			TaskType:        textValue(task.Values["task_type"]),
			Status:          status,
			Required:        required,
			JournalTemplate: textValue(task.Values["journal_template_id"]),
			JournalRunID:    textValue(task.Values["journal_run_id"]),
			PostingID:       textValue(task.Values["posting_id"]),
			PostingNumber:   textValue(task.Values["posting_number"]),
			Note:            textValue(task.Values["note"]),
		})
	}
	runItems := make([]FinanceJournalRunItem, 0, len(runs))
	for _, run := range runs {
		status := strings.TrimSpace(textValue(run.Values["status"]))
		if status == "generated" || status == "submitted" {
			ready = false
			blockers = append(blockers, firstNonEmptyString(textValue(run.Values["template_name"]), textValue(run.Values["template_code"]))+" is not posted")
		}
		runItems = append(runItems, FinanceJournalRunItem{
			ID:             run.ID,
			TemplateID:     textValue(run.Values["journal_template_id"]),
			TemplateCode:   textValue(run.Values["template_code"]),
			TemplateName:   textValue(run.Values["template_name"]),
			JournalKind:    textValue(run.Values["journal_kind"]),
			Cadence:        textValue(run.Values["cadence"]),
			Status:         status,
			PostingID:      textValue(run.Values["generated_posting_id"]),
			PostingNumber:  textValue(run.Values["generated_posting_number"]),
			PostingStatus:  textValue(run.Values["generated_posting_status"]),
			PostingDate:    textValue(run.Values["posting_date"]),
			ReversalStatus: textValue(run.Values["reversal_status"]),
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].TaskCode < items[j].TaskCode })
	sort.Slice(runItems, func(i, j int) bool { return runItems[i].TemplateCode < runItems[j].TemplateCode })
	return FinancePeriodClosePack{
		PeriodID:    period.ID,
		PeriodKey:   textValue(period.Values["period_key"]),
		Status:      textValue(period.Values["status"]),
		Ready:       ready,
		Blockers:    uniqueStrings(blockers),
		Tasks:       items,
		JournalRuns: runItems,
	}, nil
}

func (s *FinancePeriodEndCoreService) buildClosePackReadOnly(period model.Record) (FinancePeriodClosePack, error) {
	tasks, err := s.periodTasks(period.ID)
	if err != nil {
		return FinancePeriodClosePack{}, err
	}
	runs, err := s.periodRuns(period.ID)
	if err != nil {
		return FinancePeriodClosePack{}, err
	}
	requiredChecklist := []struct {
		code  string
		label string
	}{
		{code: "reconcile_ap", label: "Reconcile AP"},
		{code: "reconcile_ar", label: "Reconcile AR"},
		{code: "review_journal_ledger", label: "Review Journal Ledger"},
		{code: "review_tax_summary", label: "Review Tax Summary"},
	}
	items := make([]FinancePeriodCloseTask, 0, len(tasks)+8)
	blockers := make([]string, 0)
	ready := true
	taskByCode := make(map[string]FinancePeriodCloseTask, len(tasks))
	runByTemplateID := make(map[string]model.Record, len(runs))
	for _, run := range runs {
		runByTemplateID[strings.TrimSpace(textValue(run.Values["journal_template_id"]))] = run
	}
	for _, task := range tasks {
		status := strings.TrimSpace(textValue(task.Values["status"]))
		required := boolValue(task.Values["required"])
		item := FinancePeriodCloseTask{
			ID:              task.ID,
			TaskCode:        textValue(task.Values["task_code"]),
			Label:           textValue(task.Values["label"]),
			TaskType:        textValue(task.Values["task_type"]),
			Status:          status,
			Required:        required,
			JournalTemplate: textValue(task.Values["journal_template_id"]),
			JournalRunID:    textValue(task.Values["journal_run_id"]),
			PostingID:       textValue(task.Values["posting_id"]),
			PostingNumber:   textValue(task.Values["posting_number"]),
			Note:            textValue(task.Values["note"]),
		}
		if required && status != "completed" && status != "waived" {
			ready = false
			blockers = append(blockers, firstNonEmptyString(item.Label, item.TaskCode)+" is still pending")
		}
		items = append(items, item)
		taskByCode[item.TaskCode] = item
	}
	for _, checklist := range requiredChecklist {
		if _, ok := taskByCode[checklist.code]; ok {
			continue
		}
		ready = false
		blockers = append(blockers, checklist.label+" is still pending")
		items = append(items, FinancePeriodCloseTask{
			TaskCode: checklist.code,
			Label:    checklist.label,
			TaskType: "checklist",
			Status:   "pending",
			Required: true,
		})
	}
	templates, err := s.dueTemplates(period)
	if err != nil {
		return FinancePeriodClosePack{}, err
	}
	for _, tmpl := range templates {
		if !boolValue(tmpl.Values["required_for_period_close"]) {
			continue
		}
		taskCode := "journal:" + tmpl.ID
		if _, ok := taskByCode[taskCode]; ok {
			continue
		}
		run, hasRun := runByTemplateID[tmpl.ID]
		status := "pending"
		var runID, postingID, postingNumber string
		if hasRun {
			runStatus := strings.TrimSpace(textValue(run.Values["status"]))
			runID = run.ID
			postingID = textValue(run.Values["generated_posting_id"])
			postingNumber = textValue(run.Values["generated_posting_number"])
			if runStatus == "posted" || runStatus == "reversed" {
				status = "completed"
			}
		}
		if status != "completed" {
			ready = false
			blockers = append(blockers, firstNonEmptyString(textValue(tmpl.Values["name"]), textValue(tmpl.Values["code"]))+" is still pending")
		}
		items = append(items, FinancePeriodCloseTask{
			TaskCode:        taskCode,
			Label:           firstNonEmptyString(textValue(tmpl.Values["name"]), textValue(tmpl.Values["code"])),
			TaskType:        "journal",
			Status:          status,
			Required:        true,
			JournalTemplate: tmpl.ID,
			JournalRunID:    runID,
			PostingID:       postingID,
			PostingNumber:   postingNumber,
		})
	}
	runItems := make([]FinanceJournalRunItem, 0, len(runs))
	for _, run := range runs {
		status := strings.TrimSpace(textValue(run.Values["status"]))
		if status == "generated" || status == "submitted" {
			ready = false
			blockers = append(blockers, firstNonEmptyString(textValue(run.Values["template_name"]), textValue(run.Values["template_code"]))+" is not posted")
		}
		runItems = append(runItems, FinanceJournalRunItem{
			ID:             run.ID,
			TemplateID:     textValue(run.Values["journal_template_id"]),
			TemplateCode:   textValue(run.Values["template_code"]),
			TemplateName:   textValue(run.Values["template_name"]),
			JournalKind:    textValue(run.Values["journal_kind"]),
			Cadence:        textValue(run.Values["cadence"]),
			Status:         status,
			PostingID:      textValue(run.Values["generated_posting_id"]),
			PostingNumber:  textValue(run.Values["generated_posting_number"]),
			PostingStatus:  textValue(run.Values["generated_posting_status"]),
			PostingDate:    textValue(run.Values["posting_date"]),
			ReversalStatus: textValue(run.Values["reversal_status"]),
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].TaskCode < items[j].TaskCode })
	sort.Slice(runItems, func(i, j int) bool { return runItems[i].TemplateCode < runItems[j].TemplateCode })
	return FinancePeriodClosePack{
		PeriodID:    period.ID,
		PeriodKey:   textValue(period.Values["period_key"]),
		Status:      textValue(period.Values["status"]),
		Ready:       ready,
		Blockers:    uniqueStrings(blockers),
		Tasks:       items,
		JournalRuns: runItems,
	}, nil
}

func (s *FinancePeriodEndCoreService) uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func (s *FinancePeriodEndCoreService) periodTasks(periodID string) ([]model.Record, error) {
	items, _, err := s.models.List("accounting_period_task", model.Query{
		Filters: map[string]string{"accounting_period_id": periodID},
		Page:     1,
		PageSize: 1000,
	})
	if err != nil && !isMissingModelDefinitionError(err) {
		return nil, err
	}
	return items, nil
}

func (s *FinancePeriodEndCoreService) periodRuns(periodID string) ([]model.Record, error) {
	items, _, err := s.models.List("journal_run", model.Query{
		Filters: map[string]string{"accounting_period_id": periodID},
		Page:     1,
		PageSize: 1000,
	})
	if err != nil && !isMissingModelDefinitionError(err) {
		return nil, err
	}
	return items, nil
}

func (s *FinancePeriodEndCoreService) runForTemplatePeriod(periodID, templateID string) (model.Record, error) {
	items, _, err := s.models.List("journal_run", model.Query{
		Filters: map[string]string{
			"accounting_period_id": periodID,
			"journal_template_id":  templateID,
		},
		Page:     1,
		PageSize: 2,
	})
	if err != nil && !isMissingModelDefinitionError(err) {
		return model.Record{}, err
	}
	if len(items) == 0 {
		return model.Record{}, nil
	}
	return items[0], nil
}

func (s *FinancePeriodEndCoreService) syncPeriodJournalRuns(period model.Record, actorID string) error {
	runs, err := s.periodRuns(period.ID)
	if err != nil {
		return err
	}
	for _, run := range runs {
		if err := s.syncSingleRun(run, actorID); err != nil {
			return err
		}
	}
	tasks, err := s.periodTasks(period.ID)
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if strings.TrimSpace(textValue(task.Values["task_type"])) != "journal" {
			continue
		}
		if err := s.syncJournalTask(task, actorID); err != nil {
			return err
		}
	}
	return nil
}

func (s *FinancePeriodEndCoreService) syncRunByID(runID, actorID string) error {
	run, err := s.models.Get("journal_run", strings.TrimSpace(runID))
	if err != nil {
		return err
	}
	return s.syncSingleRun(run, actorID)
}

func (s *FinancePeriodEndCoreService) syncSingleRun(run model.Record, actorID string) error {
	values := cloneMap(run.Values)
	postingID := strings.TrimSpace(textValue(values["generated_posting_id"]))
	status := "generated"
	postingNumber := ""
	postingStatus := ""
	reversalStatus := ""
	if postingID != "" {
		posting, err := s.documents.Get(postingID)
		if err == nil {
			postingStatus = posting.Header.Status
			postingNumber = posting.Header.Number
			reversalStatus = strings.TrimSpace(textValue(posting.Body.Payload["reversal_status"]))
			switch posting.Header.Status {
			case "draft":
				status = "generated"
			case "submitted":
				status = "submitted"
			case "posted":
				status = "posted"
				if strings.TrimSpace(textValue(posting.Body.Payload["journal_source_kind"])) == "accrual" && reversalStatus == "reversed" {
					status = "reversed"
				}
			case "cancelled":
				status = "cancelled"
			default:
				status = posting.Header.Status
			}
		}
	}
	values["status"] = status
	values["generated_posting_number"] = postingNumber
	values["generated_posting_status"] = postingStatus
	values["reversal_status"] = reversalStatus
	updated, err := s.models.Update("journal_run", run.ID, actorID, values, run.Version)
	if err != nil {
		return err
	}
	taskItems, _, err := s.models.List("accounting_period_task", model.Query{
		Filters: map[string]string{"journal_run_id": updated.ID},
		Page:     1,
		PageSize: 100,
	})
	if err != nil && !isMissingModelDefinitionError(err) {
		return err
	}
	for _, task := range taskItems {
		if err := s.syncJournalTask(task, actorID); err != nil {
			return err
		}
	}
	return nil
}

func (s *FinancePeriodEndCoreService) syncJournalTask(task model.Record, actorID string) error {
	values := cloneMap(task.Values)
	runID := strings.TrimSpace(textValue(values["journal_run_id"]))
	status := "pending"
	postingID := strings.TrimSpace(textValue(values["posting_id"]))
	postingNumber := strings.TrimSpace(textValue(values["posting_number"]))
	if runID != "" {
		run, err := s.models.Get("journal_run", runID)
		if err == nil {
			values["posting_id"] = textValue(run.Values["generated_posting_id"])
			values["posting_number"] = textValue(run.Values["generated_posting_number"])
			postingID = textValue(run.Values["generated_posting_id"])
			postingNumber = textValue(run.Values["generated_posting_number"])
			if strings.TrimSpace(textValue(run.Values["status"])) == "posted" || strings.TrimSpace(textValue(run.Values["status"])) == "reversed" {
				status = "completed"
			}
		}
	}
	values["status"] = status
	values["posting_id"] = postingID
	values["posting_number"] = postingNumber
	if status == "completed" {
		values["completed_at"] = time.Now().UTC().Format(time.RFC3339)
		values["completed_by"] = actorID
	} else {
		values["completed_at"] = ""
		values["completed_by"] = ""
	}
	_, err := s.models.Update("accounting_period_task", task.ID, actorID, values, task.Version)
	return err
}

func (s *FinancePeriodEndCoreService) markSourcePostingReversed(sourceID, reversalID, actorID string, reversed bool) error {
	source, err := s.documents.Get(sourceID)
	if err != nil {
		return err
	}
	payload := cloneMap(source.Body.Payload)
	currentReverse := strings.TrimSpace(textValue(payload["reversed_by_posting_id"]))
	if reversed {
		payload["reversed_by_posting_id"] = reversalID
		payload["reversal_status"] = "reversed"
	} else if currentReverse == reversalID {
		payload["reversed_by_posting_id"] = ""
		payload["reversal_status"] = "available"
	}
	source.Body.Payload = payload
	source.Body.ContentHash = document.ContentHash(payload)
	source.Header.UpdatedAt = time.Now().UTC()
	source.Header.UpdatedBy = actorID
	if err := s.documents.Save(source); err != nil {
		return err
	}
	if runID := strings.TrimSpace(textValue(payload["journal_run_id"])); runID != "" {
		return s.syncRunByID(runID, actorID)
	}
	return nil
}

func (s *FinancePeriodEndCoreService) defaultReversalDate(source document.Record) string {
	sourceDate := strings.TrimSpace(textValue(source.Body.Payload["posting_date"]))
	items, _, err := s.models.List("accounting_period", model.Query{
		Filters: map[string]string{
			"organization_id": source.Header.OrganizationID,
			"status":          "open",
		},
		Page:     1,
		PageSize: 1000,
	})
	if err != nil {
		return ""
	}
	nextDate := ""
	for _, item := range items {
		periodLocation := strings.TrimSpace(textValue(item.Values["location_id"]))
		if periodLocation != "" && periodLocation != strings.TrimSpace(source.Header.LocationID) {
			continue
		}
		startDate := strings.TrimSpace(textValue(item.Values["start_date"]))
		if startDate == "" || startDate <= sourceDate {
			continue
		}
		if nextDate == "" || startDate < nextDate {
			nextDate = startDate
		}
	}
	return nextDate
}

func (s *FinancePeriodEndCoreService) periodIDForDate(organizationID, locationID, postingDate string) string {
	items, _, err := s.models.List("accounting_period", model.Query{
		Filters: map[string]string{
			"organization_id": strings.TrimSpace(organizationID),
		},
		Page:     1,
		PageSize: 1000,
	})
	if err != nil {
		return ""
	}
	for _, item := range items {
		periodLocation := strings.TrimSpace(textValue(item.Values["location_id"]))
		if periodLocation != "" && periodLocation != strings.TrimSpace(locationID) {
			continue
		}
		startDate := strings.TrimSpace(textValue(item.Values["start_date"]))
		endDate := strings.TrimSpace(textValue(item.Values["end_date"]))
		if startDate != "" && postingDate < startDate {
			continue
		}
		if endDate != "" && postingDate > endDate {
			continue
		}
		return item.ID
	}
	return ""
}
