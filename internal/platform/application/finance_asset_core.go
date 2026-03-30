package application

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"orbyte/internal/platform/config"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/shared"
)

type FinanceAssetCoreService struct {
	documents *document.Service
	models    *model.Service
	config    *config.Service
	finance   *FinanceReportingCoreService
}

type FinanceSchedulePreview struct {
	SourceID        string  `json:"source_id"`
	SourceType      string  `json:"source_type"`
	Status          string  `json:"status"`
	Method          string  `json:"method"`
	Cadence         string  `json:"cadence"`
	PeriodsTotal    int     `json:"periods_total"`
	PeriodsBooked   int     `json:"periods_booked"`
	BookToDate      float64 `json:"booked_amount"`
	RemainingAmount float64 `json:"remaining_amount"`
	NextPostingDate string  `json:"next_posting_date,omitempty"`
	NextAmount      float64 `json:"next_amount"`
}

func NewFinanceAssetCoreService(documents *document.Service, models *model.Service, configSvc *config.Service, finance *FinanceReportingCoreService) *FinanceAssetCoreService {
	return &FinanceAssetCoreService{documents: documents, models: models, config: configSvc, finance: finance}
}

func (s *FinanceAssetCoreService) CreateFixedAsset(organizationID, locationID, actorID string, payload map[string]any) (map[string]any, error) {
	return s.createScheduledAsset("fixed_asset", "fixed_asset_schedule", organizationID, locationID, actorID, payload)
}

func (s *FinanceAssetCoreService) CreatePrepaidExpense(organizationID, locationID, actorID string, payload map[string]any) (map[string]any, error) {
	return s.createScheduledAsset("prepaid_expense", "prepaid_schedule", organizationID, locationID, actorID, payload)
}

func (s *FinanceAssetCoreService) CreateFixedAssetFromVendorBill(vendorBillID string, lineIndex int, organizationID, locationID, actorID string, payload map[string]any) (map[string]any, error) {
	values, err := s.assetPayloadFromVendorBill("fixed_asset", vendorBillID, lineIndex, organizationID, locationID, payload)
	if err != nil {
		return nil, err
	}
	return s.createScheduledAsset("fixed_asset", "fixed_asset_schedule", organizationID, locationID, actorID, values)
}

func (s *FinanceAssetCoreService) CreatePrepaidFromVendorBill(vendorBillID string, lineIndex int, organizationID, locationID, actorID string, payload map[string]any) (map[string]any, error) {
	values, err := s.assetPayloadFromVendorBill("prepaid_expense", vendorBillID, lineIndex, organizationID, locationID, payload)
	if err != nil {
		return nil, err
	}
	return s.createScheduledAsset("prepaid_expense", "prepaid_schedule", organizationID, locationID, actorID, values)
}

func (s *FinanceAssetCoreService) FixedAssetPreview(id, organizationID, locationID string) (FinanceSchedulePreview, error) {
	return s.previewScheduledAsset("fixed_asset", "fixed_asset_schedule", id, organizationID, locationID)
}

func (s *FinanceAssetCoreService) PrepaidPreview(id, organizationID, locationID string) (FinanceSchedulePreview, error) {
	return s.previewScheduledAsset("prepaid_expense", "prepaid_schedule", id, organizationID, locationID)
}

func (s *FinanceAssetCoreService) HandleApprovedLedgerPosting(record document.Record, actorID string) error {
	if record.Header.Type != "ledger_posting" {
		return nil
	}
	return s.applyPostingToSchedule(record, actorID, false)
}

func (s *FinanceAssetCoreService) HandleCanceledLedgerPosting(record document.Record, actorID string) error {
	if record.Header.Type != "ledger_posting" {
		return nil
	}
	return s.applyPostingToSchedule(record, actorID, true)
}

func (s *FinanceAssetCoreService) ValidateAction(record document.Record, action string) error {
	if record.Header.Type != "ledger_posting" {
		return nil
	}
	if strings.TrimSpace(action) != "approve" {
		return nil
	}
	templateID := strings.TrimSpace(textValue(record.Body.Payload["journal_template_id"]))
	if templateID == "" || s.models == nil {
		return nil
	}
	template, err := s.models.Get("journal_template", templateID)
	if err != nil {
		return err
	}
	sourceType := strings.TrimSpace(textValue(template.Values["source_model_type"]))
	sourceID := strings.TrimSpace(textValue(template.Values["source_model_id"]))
	if sourceID == "" || (sourceType != "fixed_asset_schedule" && sourceType != "prepaid_schedule") {
		return nil
	}
	latest := s.latestPostedRun(templateID, record.Header.ID)
	if latest.Header.ID == "" {
		return nil
	}
	postingDate := strings.TrimSpace(textValue(record.Body.Payload["posting_date"]))
	latestDate := strings.TrimSpace(textValue(latest.Body.Payload["posting_date"]))
	if postingDate == "" || latestDate == "" {
		return nil
	}
	if postingDate < latestDate {
		return shared.Conflict("asset or prepaid journals must be approved in posting-date order")
	}
	return nil
}

func (s *FinanceAssetCoreService) createScheduledAsset(assetModelKey, scheduleModelKey, organizationID, locationID, actorID string, payload map[string]any) (map[string]any, error) {
	if s.models == nil {
		return nil, shared.Validation("models are unavailable")
	}
	assetValues, scheduleValues, err := s.normalizeScheduledAssetPayload(assetModelKey, organizationID, locationID, payload)
	if err != nil {
		return nil, err
	}
	assetRecord, err := s.models.Create(assetModelKey, actorID, assetValues)
	if err != nil {
		return nil, err
	}
	parentIDKey := "fixed_asset_id"
	templateName := "Depreciation"
	if assetModelKey == "prepaid_expense" {
		parentIDKey = "prepaid_expense_id"
		templateName = "Amortization"
	}
	scheduleValues[parentIDKey] = assetRecord.ID
	scheduleRecord, err := s.models.Create(scheduleModelKey, actorID, scheduleValues)
	if err != nil {
		return nil, err
	}
	templateRecord, scheduleRecord, err := s.syncScheduleTemplate(scheduleModelKey, scheduleRecord, actorID)
	if err != nil {
		return nil, err
	}
	assetUpdate := cloneMap(assetRecord.Values)
	assetUpdate["schedule_id"] = scheduleRecord.ID
	assetUpdate["linked_journal_template_id"] = templateRecord.ID
	assetUpdate["next_posting_date"] = textValue(scheduleRecord.Values["next_posting_date"])
	assetUpdate["periods_booked"] = numberValue(scheduleRecord.Values["periods_booked"])
	assetUpdate["booked_amount"] = numberValue(scheduleRecord.Values["booked_amount"])
	assetUpdate["remaining_amount"] = numberValue(scheduleRecord.Values["remaining_amount"])
	assetUpdate["status"] = firstNonEmptyString(textValue(scheduleRecord.Values["status"]), "active")
	assetRecord, err = s.models.Update(assetModelKey, assetRecord.ID, actorID, assetUpdate, assetRecord.Version)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(textValue(templateRecord.Values["name"])) == "" {
		templateValues := cloneMap(templateRecord.Values)
		templateValues["name"] = fmt.Sprintf("%s %s", templateName, firstNonEmptyString(textValue(assetRecord.Values["name"]), textValue(assetRecord.Values["code"]), assetRecord.ID))
		templateRecord, _ = s.models.Update("journal_template", templateRecord.ID, actorID, templateValues, templateRecord.Version)
	}
	return map[string]any{
		"asset":     assetRecord,
		"schedule":  scheduleRecord,
		"template":  templateRecord,
		"preview":   s.previewFromSchedule(scheduleModelKey, scheduleRecord),
		"asset_key": assetModelKey,
	}, nil
}

func (s *FinanceAssetCoreService) previewScheduledAsset(assetModelKey, scheduleModelKey, id, organizationID, locationID string) (FinanceSchedulePreview, error) {
	record, err := s.scopedModelRecord(assetModelKey, strings.TrimSpace(id), organizationID, locationID)
	if err != nil {
		return FinanceSchedulePreview{}, err
	}
	scheduleID := strings.TrimSpace(textValue(record.Values["schedule_id"]))
	if scheduleID == "" {
		return FinanceSchedulePreview{}, shared.Validation("schedule is not initialized")
	}
	schedule, err := s.scopedModelRecord(scheduleModelKey, scheduleID, organizationID, locationID)
	if err != nil {
		return FinanceSchedulePreview{}, err
	}
	return s.previewFromSchedule(scheduleModelKey, schedule), nil
}

func (s *FinanceAssetCoreService) normalizeScheduledAssetPayload(assetModelKey, organizationID, locationID string, payload map[string]any) (map[string]any, map[string]any, error) {
	next := document.NormalizePayload(cloneMap(payload))
	startDate := firstNonEmptyString(textValue(next["recognition_start_date"]), textValue(next["capitalization_date"]), textValue(next["acquisition_date"]), time.Now().UTC().Format("2006-01-02"))
	method := strings.ToLower(firstNonEmptyString(textValue(next["method"]), "straight_line"))
	if method != "straight_line" && method != "declining_balance" {
		return nil, nil, shared.Validation("supported methods are straight_line and declining_balance")
	}
	cadence := strings.ToLower(firstNonEmptyString(textValue(next["cadence"]), "monthly"))
	if cadence != "monthly" && cadence != "quarterly" && cadence != "yearly" {
		return nil, nil, shared.Validation("supported cadence values are monthly, quarterly, and yearly")
	}
	totalPeriods := assetIntValue(next["total_periods"], 0)
	if totalPeriods <= 0 {
		totalPeriods = assetIntValue(next["useful_life_periods"], 0)
	}
	if totalPeriods <= 0 {
		return nil, nil, shared.Validation("total periods must be greater than zero")
	}
	basisAmount := roundMoney(numberValue(next["basis_amount"]))
	if basisAmount <= 0 {
		return nil, nil, shared.Validation("basis amount must be greater than zero")
	}
	salvageAmount := 0.0
	if assetModelKey == "fixed_asset" {
		salvageAmount = roundMoney(numberValue(next["salvage_amount"]))
		if salvageAmount < 0 || salvageAmount >= basisAmount {
			return nil, nil, shared.Validation("salvage amount must be zero or less than basis amount")
		}
	}
	cfg := s.assetPostingConfig(organizationID, locationID)
	status := strings.ToLower(firstNonEmptyString(textValue(next["status"]), "active"))
	if status == "draft" {
		status = "active"
	}
	next["organization_id"] = strings.TrimSpace(organizationID)
	next["location_id"] = strings.TrimSpace(locationID)
	next["status"] = status
	next["basis_amount"] = basisAmount
	next["booked_amount"] = 0.0
	next["remaining_amount"] = roundMoney(basisAmount - salvageAmount)
	next["periods_booked"] = 0
	next["method"] = method
	next["cadence"] = cadence
	next["total_periods"] = totalPeriods
	next["declining_rate_percent"] = roundMoney(numberValue(next["declining_rate_percent"]))
	next["schedule_id"] = ""
	next["linked_journal_template_id"] = ""
	if assetModelKey == "fixed_asset" {
		next["acquisition_date"] = startDate
		next["capitalization_date"] = firstNonEmptyString(textValue(next["capitalization_date"]), startDate)
		next["asset_account_code"] = firstNonEmptyString(textValue(next["asset_account_code"]), cfg["fixed_asset_default_asset_account_code"], "1500-FA")
		next["accumulated_depreciation_account_code"] = firstNonEmptyString(textValue(next["accumulated_depreciation_account_code"]), cfg["fixed_asset_default_accumulated_depreciation_account_code"], "1590-ACC-DEPR")
		next["depreciation_expense_account_code"] = firstNonEmptyString(textValue(next["depreciation_expense_account_code"]), cfg["fixed_asset_default_depreciation_expense_account_code"], "6100-DEPR")
		next["salvage_amount"] = salvageAmount
	} else {
		next["recognition_start_date"] = startDate
		next["prepaid_asset_account_code"] = firstNonEmptyString(textValue(next["prepaid_asset_account_code"]), cfg["prepaid_default_asset_account_code"], "1600-PREPAID")
		next["expense_account_code"] = firstNonEmptyString(textValue(next["expense_account_code"]), cfg["prepaid_default_expense_account_code"], "6200-AMORT")
	}
	scheduleValues := map[string]any{
		"organization_id": strings.TrimSpace(organizationID),
		"location_id":     strings.TrimSpace(locationID),
		"status":          status,
		"method":          method,
		"cadence":         cadence,
		"total_periods":   totalPeriods,
		"periods_booked":  0,
		"basis_amount":    basisAmount,
		"booked_amount":   0.0,
		"remaining_amount": roundMoney(basisAmount - salvageAmount),
		"declining_rate_percent": roundMoney(numberValue(next["declining_rate_percent"])),
		"next_posting_date":      scheduleFirstDueDate(startDate, cadence),
		"linked_journal_template_id": "",
	}
	if assetModelKey == "fixed_asset" {
		scheduleValues["start_date"] = startDate
		scheduleValues["salvage_amount"] = salvageAmount
		scheduleValues["asset_account_code"] = textValue(next["asset_account_code"])
		scheduleValues["accumulated_depreciation_account_code"] = textValue(next["accumulated_depreciation_account_code"])
		scheduleValues["depreciation_expense_account_code"] = textValue(next["depreciation_expense_account_code"])
	} else {
		scheduleValues["start_date"] = startDate
		scheduleValues["prepaid_asset_account_code"] = textValue(next["prepaid_asset_account_code"])
		scheduleValues["expense_account_code"] = textValue(next["expense_account_code"])
	}
	return next, scheduleValues, nil
}

func (s *FinanceAssetCoreService) assetPayloadFromVendorBill(assetModelKey, vendorBillID string, lineIndex int, organizationID, locationID string, payload map[string]any) (map[string]any, error) {
	if s.documents == nil {
		return nil, shared.Validation("documents are unavailable")
	}
	bill, err := s.documents.Get(strings.TrimSpace(vendorBillID))
	if err != nil {
		return nil, err
	}
	if bill.Header.Type != "vendor_bill" {
		return nil, shared.Validation("source document must be a vendor bill")
	}
	if strings.TrimSpace(organizationID) != "" && strings.TrimSpace(bill.Header.OrganizationID) != strings.TrimSpace(organizationID) {
		return nil, shared.Forbidden("vendor bill is outside the current organization scope")
	}
	if strings.TrimSpace(locationID) != "" && strings.TrimSpace(bill.Header.LocationID) != strings.TrimSpace(locationID) {
		return nil, shared.Forbidden("vendor bill is outside the current location scope")
	}
	lines := recordList(bill.Body.Payload["lines"])
	if lineIndex < 0 || lineIndex >= len(lines) {
		return nil, shared.Validation("vendor bill line index is out of range")
	}
	line := cloneMap(lines[lineIndex])
	if s.isInventoryEnabledItem(textValue(line["item_code"])) {
		return nil, shared.Validation("inventory-enabled vendor bill lines are not eligible for fixed assets or prepaids")
	}
	basisAmount := roundMoney(numberValue(line["line_subtotal"]))
	if basisAmount <= 0 {
		basisAmount = roundMoney(numberValue(line["line_total"]))
	}
	if basisAmount <= 0 {
		return nil, shared.Validation("vendor bill line amount must be greater than zero")
	}
	values := cloneMap(payload)
	values["basis_amount"] = basisAmount
	values["organization_id"] = bill.Header.OrganizationID
	values["location_id"] = bill.Header.LocationID
	values["source_vendor_bill_id"] = bill.Header.ID
	values["source_vendor_bill_number"] = firstNonEmptyString(bill.Header.Number, bill.Header.ID)
	values["source_vendor_bill_line_index"] = lineIndex
	values["capitalization_date"] = firstNonEmptyString(textValue(values["capitalization_date"]), textValue(bill.Body.Payload["bill_date"]), time.Now().UTC().Format("2006-01-02"))
	values["recognition_start_date"] = firstNonEmptyString(textValue(values["recognition_start_date"]), textValue(values["capitalization_date"]))
	values["name"] = firstNonEmptyString(textValue(values["name"]), textValue(line["description"]), textValue(line["item_code"]), fmt.Sprintf("%s line %d", assetModelKey, lineIndex+1))
	values["code"] = firstNonEmptyString(textValue(values["code"]), strings.ToUpper(assetCodePrefix(assetModelKey))+"-"+sanitizeCodeFragment(firstNonEmptyString(bill.Header.Number, bill.Header.ID))+"-"+strconv.Itoa(lineIndex+1))
	return values, nil
}

func (s *FinanceAssetCoreService) syncScheduleTemplate(scheduleModelKey string, schedule model.Record, actorID string) (model.Record, model.Record, error) {
	if s.models == nil {
		return model.Record{}, model.Record{}, shared.Validation("models are unavailable")
	}
	sourceModelType := scheduleModelKey
	sourceModelID := schedule.ID
	templateID := strings.TrimSpace(textValue(schedule.Values["linked_journal_template_id"]))
	parent, err := s.parentForSchedule(scheduleModelKey, schedule)
	if err != nil {
		return model.Record{}, model.Record{}, err
	}
	preview := s.previewFromSchedule(scheduleModelKey, schedule)
	templateValues := map[string]any{
		"organization_id":         textValue(schedule.Values["organization_id"]),
		"location_id":             textValue(schedule.Values["location_id"]),
		"code":                    scheduleTemplateCode(scheduleModelKey, parent),
		"name":                    scheduleTemplateName(scheduleModelKey, parent),
		"journal_kind":            scheduleTemplateKind(scheduleModelKey),
		"cadence":                 firstNonEmptyString(textValue(schedule.Values["cadence"]), "monthly"),
		"currency_code":           "IDR",
		"description_template":    scheduleTemplateDescription(scheduleModelKey, parent),
		"required_for_period_close": true,
		"source_model_type":       sourceModelType,
		"source_model_id":         sourceModelID,
		"next_due_date":           preview.NextPostingDate,
		"status":                  "active",
		"notes":                   fmt.Sprintf("Auto-managed from %s %s", scheduleModelKey, parent.ID),
	}
	if preview.Status == "completed" || preview.NextAmount <= 0 || preview.NextPostingDate == "" {
		templateValues["status"] = "inactive"
		templateValues["required_for_period_close"] = false
		templateValues["journal_lines"] = []map[string]any{}
	} else {
		templateValues["journal_lines"] = s.scheduleJournalLines(scheduleModelKey, schedule, parent, preview.NextAmount)
	}
	var template model.Record
	if templateID == "" {
		template, err = s.models.Create("journal_template", actorID, templateValues)
		if err != nil {
			return model.Record{}, model.Record{}, err
		}
		nextSchedule := cloneMap(schedule.Values)
		nextSchedule["linked_journal_template_id"] = template.ID
		schedule, err = s.models.Update(scheduleModelKey, schedule.ID, actorID, nextSchedule, schedule.Version)
		if err != nil {
			return model.Record{}, model.Record{}, err
		}
		return template, schedule, nil
	}
	template, err = s.models.Get("journal_template", templateID)
	if err != nil {
		return model.Record{}, model.Record{}, err
	}
	template, err = s.models.Update("journal_template", template.ID, actorID, mergeRecordValues(template.Values, templateValues), template.Version)
	if err != nil {
		return model.Record{}, model.Record{}, err
	}
	return template, schedule, nil
}

func (s *FinanceAssetCoreService) applyPostingToSchedule(record document.Record, actorID string, reversed bool) error {
	templateID := strings.TrimSpace(textValue(record.Body.Payload["journal_template_id"]))
	if templateID == "" || s.models == nil {
		return nil
	}
	template, err := s.models.Get("journal_template", templateID)
	if err != nil {
		return err
	}
	sourceType := strings.TrimSpace(textValue(template.Values["source_model_type"]))
	sourceID := strings.TrimSpace(textValue(template.Values["source_model_id"]))
	if sourceID == "" || (sourceType != "fixed_asset_schedule" && sourceType != "prepaid_schedule") {
		return nil
	}
	schedule, err := s.models.Get(sourceType, sourceID)
	if err != nil {
		return err
	}
	values, err := s.recomputeScheduleValues(templateID, sourceType, schedule)
	if err != nil {
		return err
	}
	schedule, err = s.models.Update(sourceType, schedule.ID, actorID, values, schedule.Version)
	if err != nil {
		return err
	}
	template, schedule, err = s.syncScheduleTemplate(sourceType, schedule, actorID)
	if err != nil {
		return err
	}
	return s.updateParentFromSchedule(sourceType, schedule, template, actorID)
}

func (s *FinanceAssetCoreService) recomputeScheduleValues(templateID, scheduleModelKey string, schedule model.Record) (map[string]any, error) {
	values := cloneMap(schedule.Values)
	baseRemaining := roundMoney(numberValue(values["basis_amount"]))
	if scheduleModelKey == "fixed_asset_schedule" {
		baseRemaining = roundMoney(baseRemaining - numberValue(values["salvage_amount"]))
	}
	baseStartDate := firstNonEmptyString(textValue(values["start_date"]), textValue(values["last_posting_date"]))
	baseNextPostingDate := ""
	if baseStartDate != "" {
		baseNextPostingDate = scheduleFirstDueDate(baseStartDate, textValue(values["cadence"]))
	}
	values["periods_booked"] = 0
	values["booked_amount"] = 0.0
	values["remaining_amount"] = baseRemaining
	values["last_posting_id"] = ""
	values["last_posting_date"] = ""
	values["last_posting_amount"] = 0.0
	values["next_posting_date"] = baseNextPostingDate
	values["status"] = "active"
	history, err := s.postedRunsInOrder(templateID)
	if err != nil {
		return nil, err
	}
	for _, posting := range history {
		amount := roundMoney(numberValue(posting.Body.Payload["total_amount"]))
		values["periods_booked"] = assetIntValue(values["periods_booked"], 0) + 1
		values["booked_amount"] = roundMoney(numberValue(values["booked_amount"]) + amount)
		values["remaining_amount"] = roundMoney(maxFloat(0, numberValue(values["remaining_amount"])-amount))
		values["last_posting_id"] = posting.Header.ID
		values["last_posting_date"] = textValue(posting.Body.Payload["posting_date"])
		values["last_posting_amount"] = amount
		values["next_posting_date"] = scheduleAdvanceDueDate(textValue(posting.Body.Payload["posting_date"]), textValue(values["cadence"]))
	}
	if assetIntValue(values["periods_booked"], 0) >= assetIntValue(values["total_periods"], 0) || roundMoney(numberValue(values["remaining_amount"])) <= 0 {
		values["status"] = "completed"
		values["next_posting_date"] = ""
	}
	return values, nil
}

func (s *FinanceAssetCoreService) updateParentFromSchedule(scheduleModelKey string, schedule, template model.Record, actorID string) error {
	parent, err := s.parentForSchedule(scheduleModelKey, schedule)
	if err != nil {
		return err
	}
	parentModelKey := "fixed_asset"
	if scheduleModelKey == "prepaid_schedule" {
		parentModelKey = "prepaid_expense"
	}
	values := cloneMap(parent.Values)
	values["schedule_id"] = schedule.ID
	values["linked_journal_template_id"] = template.ID
	values["periods_booked"] = assetIntValue(schedule.Values["periods_booked"], 0)
	values["booked_amount"] = roundMoney(numberValue(schedule.Values["booked_amount"]))
	values["remaining_amount"] = roundMoney(numberValue(schedule.Values["remaining_amount"]))
	values["next_posting_date"] = textValue(schedule.Values["next_posting_date"])
	values["status"] = firstNonEmptyString(textValue(schedule.Values["status"]), textValue(parent.Values["status"]), "active")
	_, err = s.models.Update(parentModelKey, parent.ID, actorID, values, parent.Version)
	return err
}

func (s *FinanceAssetCoreService) scheduleJournalLines(scheduleModelKey string, schedule, parent model.Record, amount float64) []map[string]any {
	description := scheduleTemplateDescription(scheduleModelKey, parent)
	if scheduleModelKey == "fixed_asset_schedule" {
		return []map[string]any{
			{"account_code": textValue(schedule.Values["depreciation_expense_account_code"]), "account_name": "Depreciation Expense", "description": description, "debit": amount, "credit": 0.0},
			{"account_code": textValue(schedule.Values["accumulated_depreciation_account_code"]), "account_name": "Accumulated Depreciation", "description": description, "debit": 0.0, "credit": amount},
		}
	}
	return []map[string]any{
		{"account_code": textValue(schedule.Values["expense_account_code"]), "account_name": "Amortization Expense", "description": description, "debit": amount, "credit": 0.0},
		{"account_code": textValue(schedule.Values["prepaid_asset_account_code"]), "account_name": "Prepaid Asset", "description": description, "debit": 0.0, "credit": amount},
	}
}

func (s *FinanceAssetCoreService) previewFromSchedule(scheduleModelKey string, schedule model.Record) FinanceSchedulePreview {
	method := firstNonEmptyString(textValue(schedule.Values["method"]), "straight_line")
	totalPeriods := assetIntValue(schedule.Values["total_periods"], 0)
	periodsBooked := assetIntValue(schedule.Values["periods_booked"], 0)
	remainingAmount := roundMoney(numberValue(schedule.Values["remaining_amount"]))
	nextAmount := 0.0
	status := firstNonEmptyString(textValue(schedule.Values["status"]), "active")
	if status != "completed" && remainingAmount > 0 && periodsBooked < totalPeriods {
		switch method {
		case "declining_balance":
			rate := numberValue(schedule.Values["declining_rate_percent"])
			if rate <= 0 {
				rate = roundMoney(100.0 / float64(maxInt(1, totalPeriods)))
			}
			nextAmount = roundMoney(remainingAmount * rate / 100.0)
			if nextAmount <= 0 {
				nextAmount = remainingAmount
			}
			if periodsBooked+1 >= totalPeriods || nextAmount > remainingAmount {
				nextAmount = remainingAmount
			}
		default:
			remainingPeriods := maxInt(1, totalPeriods-periodsBooked)
			nextAmount = roundMoney(remainingAmount / float64(remainingPeriods))
		}
	}
	if roundMoney(nextAmount) <= 0 || remainingAmount <= 0 || periodsBooked >= totalPeriods {
		status = "completed"
		nextAmount = 0
	}
	return FinanceSchedulePreview{
		SourceID:        schedule.ID,
		SourceType:      scheduleModelKey,
		Status:          status,
		Method:          method,
		Cadence:         textValue(schedule.Values["cadence"]),
		PeriodsTotal:    totalPeriods,
		PeriodsBooked:   periodsBooked,
		BookToDate:      roundMoney(numberValue(schedule.Values["booked_amount"])),
		RemainingAmount: remainingAmount,
		NextPostingDate: textValue(schedule.Values["next_posting_date"]),
		NextAmount:      nextAmount,
	}
}

func (s *FinanceAssetCoreService) parentForSchedule(scheduleModelKey string, schedule model.Record) (model.Record, error) {
	parentModelKey := "fixed_asset"
	parentID := strings.TrimSpace(textValue(schedule.Values["fixed_asset_id"]))
	if scheduleModelKey == "prepaid_schedule" {
		parentModelKey = "prepaid_expense"
		parentID = strings.TrimSpace(textValue(schedule.Values["prepaid_expense_id"]))
	}
	if parentID == "" {
		return model.Record{}, shared.Validation("schedule parent reference is missing")
	}
	return s.models.Get(parentModelKey, parentID)
}

func (s *FinanceAssetCoreService) scopedModelRecord(modelKey, id, organizationID, locationID string) (model.Record, error) {
	record, err := s.models.Get(modelKey, strings.TrimSpace(id))
	if err != nil {
		return model.Record{}, err
	}
	if strings.TrimSpace(organizationID) != "" && strings.TrimSpace(textValue(record.Values["organization_id"])) != strings.TrimSpace(organizationID) {
		return model.Record{}, shared.Forbidden("record is outside the current organization scope")
	}
	if strings.TrimSpace(locationID) != "" {
		recordLocation := strings.TrimSpace(textValue(record.Values["location_id"]))
		if recordLocation != "" && recordLocation != strings.TrimSpace(locationID) {
			return model.Record{}, shared.Forbidden("record is outside the current location scope")
		}
	}
	return record, nil
}

func (s *FinanceAssetCoreService) latestPostedRun(templateID, exceptPostingID string) document.Record {
	history, err := s.postedRunsInOrder(templateID)
	if err != nil || len(history) == 0 {
		return document.Record{}
	}
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Header.ID != strings.TrimSpace(exceptPostingID) {
			return history[i]
		}
	}
	return document.Record{}
}

func (s *FinanceAssetCoreService) postedRunsInOrder(templateID string) ([]document.Record, error) {
	if s.models == nil || s.documents == nil {
		return nil, nil
	}
	runs, _, err := s.models.List("journal_run", model.Query{
		Filters:  map[string]string{"journal_template_id": strings.TrimSpace(templateID)},
		Page:     1,
		PageSize: 500,
	})
	if err != nil {
		return nil, err
	}
	records := make([]document.Record, 0, len(runs))
	for _, run := range runs {
		postingID := strings.TrimSpace(textValue(run.Values["generated_posting_id"]))
		if postingID == "" {
			continue
		}
		record, getErr := s.documents.Get(postingID)
		if getErr != nil || record.Header.Status != "posted" {
			continue
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		leftDate := strings.TrimSpace(textValue(records[i].Body.Payload["posting_date"]))
		rightDate := strings.TrimSpace(textValue(records[j].Body.Payload["posting_date"]))
		if leftDate == rightDate {
			return records[i].Header.CreatedAt.Before(records[j].Header.CreatedAt)
		}
		return leftDate < rightDate
	})
	return records, nil
}

func (s *FinanceAssetCoreService) assetPostingConfig(organizationID, locationID string) map[string]string {
	defaults := map[string]string{
		"fixed_asset_default_asset_account_code":                    "1500-FA",
		"fixed_asset_default_accumulated_depreciation_account_code": "1590-ACC-DEPR",
		"fixed_asset_default_depreciation_expense_account_code":     "6100-DEPR",
		"prepaid_default_asset_account_code":                        "1600-PREPAID",
		"prepaid_default_expense_account_code":                      "6200-AMORT",
	}
	if s.config == nil {
		return defaults
	}
	entry, ok := s.config.Resolve("finance_asset.posting", strings.TrimSpace(organizationID), strings.TrimSpace(locationID))
	if !ok {
		return defaults
	}
	for key := range defaults {
		if current := textValue(entry.Value[key]); current != "" {
			defaults[key] = current
		}
	}
	return defaults
}

func (s *FinanceAssetCoreService) isInventoryEnabledItem(itemCode string) bool {
	itemCode = strings.TrimSpace(itemCode)
	if itemCode == "" || s.models == nil {
		return false
	}
	items, _, err := s.models.List("commercial_item", model.Query{
		Filters:  map[string]string{"sku": itemCode},
		Page:     1,
		PageSize: 1,
	})
	if err != nil || len(items) == 0 {
		return false
	}
	return boolValue(items[0].Values["inventory_enabled"])
}

func scheduleFirstDueDate(startDate, cadence string) string {
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(startDate))
	if err != nil {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(cadence)) {
	case "quarterly":
		month := ((int(parsed.Month())-1)/3+1)*3
		return time.Date(parsed.Year(), time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	case "yearly":
		return time.Date(parsed.Year()+1, time.January, 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	default:
		return time.Date(parsed.Year(), parsed.Month()+1, 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	}
}

func scheduleAdvanceDueDate(currentDate, cadence string) string {
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(currentDate))
	if err != nil {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(cadence)) {
	case "quarterly":
		return time.Date(parsed.Year(), parsed.Month()+4, 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	case "yearly":
		return time.Date(parsed.Year()+2, time.January, 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	default:
		return time.Date(parsed.Year(), parsed.Month()+2, 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	}
}

func scheduleTemplateKind(scheduleModelKey string) string {
	if scheduleModelKey == "prepaid_schedule" {
		return "prepaid_amortization"
	}
	return "asset_depreciation"
}

func scheduleTemplateCode(scheduleModelKey string, parent model.Record) string {
	prefix := "FA-DEPR"
	if scheduleModelKey == "prepaid_schedule" {
		prefix = "PRE-AMORT"
	}
	return prefix + "-" + sanitizeCodeFragment(firstNonEmptyString(textValue(parent.Values["code"]), parent.ID))
}

func scheduleTemplateName(scheduleModelKey string, parent model.Record) string {
	label := "Depreciation"
	if scheduleModelKey == "prepaid_schedule" {
		label = "Amortization"
	}
	return fmt.Sprintf("%s %s", label, firstNonEmptyString(textValue(parent.Values["name"]), textValue(parent.Values["code"]), parent.ID))
}

func scheduleTemplateDescription(scheduleModelKey string, parent model.Record) string {
	label := "Depreciation"
	if scheduleModelKey == "prepaid_schedule" {
		label = "Amortization"
	}
	return fmt.Sprintf("%s for %s", label, firstNonEmptyString(textValue(parent.Values["name"]), textValue(parent.Values["code"]), parent.ID))
}

func assetCodePrefix(assetModelKey string) string {
	if assetModelKey == "prepaid_expense" {
		return "pre"
	}
	return "fa"
}

func sanitizeCodeFragment(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return "AUTO"
	}
	replacer := strings.NewReplacer(" ", "-", "/", "-", "_", "-", ".", "-", ":", "-", "#", "-", ",", "-")
	value = replacer.Replace(value)
	for strings.Contains(value, "--") {
		value = strings.ReplaceAll(value, "--", "-")
	}
	return strings.Trim(value, "-")
}

func mergeRecordValues(current, updates map[string]any) map[string]any {
	next := cloneMap(current)
	for key, value := range updates {
		next[key] = value
	}
	return next
}

func assetIntValue(value any, fallback int) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int8:
		return int(typed)
	case int16:
		return int(typed)
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float32:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(typed)); err == nil {
			return parsed
		}
	}
	return fallback
}
