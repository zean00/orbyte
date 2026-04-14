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

type assetLifecycleState struct {
	CurrentLocationID     string
	CurrentCostCenterCode string
	LifecycleStatus       string
	LastEventDate         string
	LastEventType         string
	LastEventID           string
	DisposalDate          string
	DisposalType          string
	DisposalPostingID     string
	NetRevaluationAmount  float64
	ImpairmentAmountTotal float64
	AccumulatedImpairment float64
	AdjustmentAmountTotal float64
	GrossAmount           float64
	CarryingAmount        float64
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

func (s *FinanceAssetCoreService) DisposeFixedAsset(id, organizationID, locationID, actorID string, payload map[string]any) (map[string]any, error) {
	asset, schedule, state, err := s.fixedAssetContext(id, organizationID, locationID)
	if err != nil {
		return nil, err
	}
	disposalDate := firstNonEmptyString(textValue(payload["disposal_date"]), time.Now().UTC().Format("2006-01-02"))
	if err := s.validateFixedAssetEventDate(asset, disposalDate); err != nil {
		return nil, err
	}
	disposalType := strings.ToLower(firstNonEmptyString(textValue(payload["disposal_type"]), "retirement"))
	if disposalType != "retirement" && disposalType != "sale" {
		return nil, shared.Validation("disposal type must be retirement or sale")
	}
	if state.LifecycleStatus == "disposed" || state.LifecycleStatus == "retired" {
		return nil, shared.Conflict("fixed asset is already disposed")
	}
	proceedsAmount := roundMoney(numberValue(payload["proceeds_amount"]))
	if disposalType == "retirement" {
		proceedsAmount = 0
	}
	if disposalType == "sale" && proceedsAmount < 0 {
		return nil, shared.Validation("sale proceeds must be zero or greater")
	}
	if state.CarryingAmount < 0 {
		state.CarryingAmount = 0
	}
	cfg := s.assetPostingConfig(textValue(asset.Values["organization_id"]), textValue(asset.Values["location_id"]))
	proceedsAccount := firstNonEmptyString(textValue(payload["proceeds_account_code"]), cfg["fixed_asset_sale_proceeds_account_code"], "1000-CASH")
	gainAccount := firstNonEmptyString(textValue(payload["disposal_gain_account_code"]), cfg["fixed_asset_disposal_gain_account_code"], "7200-FA-GAIN")
	lossAccount := firstNonEmptyString(textValue(payload["disposal_loss_account_code"]), cfg["fixed_asset_disposal_loss_account_code"], "6205-FA-LOSS")
	grossAmount := roundMoney(numberValue(asset.Values["basis_amount"]) + state.NetRevaluationAmount)
	accumDep := roundMoney(numberValue(asset.Values["booked_amount"]))
	accumImpairment := roundMoney(state.AccumulatedImpairment)
	gainLoss := roundMoney(grossAmount - accumDep - accumImpairment - proceedsAmount)
	eventValues := map[string]any{
		"organization_id":                 textValue(asset.Values["organization_id"]),
		"location_id":                     textValue(asset.Values["location_id"]),
		"fixed_asset_id":                  asset.ID,
		"fixed_asset_schedule_id":         schedule.ID,
		"disposal_date":                   disposalDate,
		"disposal_type":                   disposalType,
		"status":                          "posted",
		"proceeds_amount":                 proceedsAmount,
		"gross_amount":                    grossAmount,
		"accumulated_depreciation_amount": accumDep,
		"accumulated_impairment_amount":   accumImpairment,
		"carrying_amount":                 state.CarryingAmount,
		"gain_loss_amount":                roundMoney(proceedsAmount - state.CarryingAmount),
		"proceeds_account_code":           proceedsAccount,
		"disposal_gain_account_code":      gainAccount,
		"disposal_loss_account_code":      lossAccount,
		"notes":                           textValue(payload["notes"]),
	}
	event, err := s.models.Create("asset_disposal", actorID, eventValues)
	if err != nil {
		return nil, err
	}
	lines := make([]map[string]any, 0, 4)
	if accumDep > 0 {
		lines = append(lines, map[string]any{"account_code": textValue(asset.Values["accumulated_depreciation_account_code"]), "account_name": "Accumulated Depreciation", "description": "Asset disposal", "debit": accumDep, "credit": 0.0})
	}
	if accumImpairment > 0 {
		lines = append(lines, map[string]any{"account_code": firstNonEmptyString(textValue(payload["accumulated_impairment_account_code"]), cfg["fixed_asset_default_accumulated_impairment_account_code"], "1595-ACC-IMP"), "account_name": "Accumulated Impairment", "description": "Asset disposal", "debit": accumImpairment, "credit": 0.0})
	}
	if proceedsAmount > 0 {
		lines = append(lines, map[string]any{"account_code": proceedsAccount, "account_name": "Disposal Proceeds", "description": "Asset disposal", "debit": proceedsAmount, "credit": 0.0})
	}
	lines = append(lines, map[string]any{"account_code": textValue(asset.Values["asset_account_code"]), "account_name": "Fixed Asset", "description": "Asset disposal", "debit": 0.0, "credit": grossAmount})
	if gainLoss > 0 {
		lines = append(lines, map[string]any{"account_code": lossAccount, "account_name": "Disposal Loss", "description": "Asset disposal", "debit": gainLoss, "credit": 0.0})
	} else if gainLoss < 0 {
		lines = append(lines, map[string]any{"account_code": gainAccount, "account_name": "Disposal Gain", "description": "Asset disposal", "debit": 0.0, "credit": roundMoney(-gainLoss)})
	}
	posting, err := s.createAssetLifecyclePosting(asset, actorID, "asset_disposal", event.ID, disposalDate, grossAmount, lines, schedule.ID)
	if err != nil {
		return nil, err
	}
	if event, err = s.models.Update("asset_disposal", event.ID, actorID, mergeRecordValues(event.Values, map[string]any{"posting_id": posting.Header.ID, "posting_date": disposalDate}), event.Version); err != nil {
		return nil, err
	}
	if err := s.recomputeFixedAssetState(asset.ID, actorID); err != nil {
		return nil, err
	}
	return map[string]any{"event": event, "posting": posting}, nil
}

func (s *FinanceAssetCoreService) TransferFixedAsset(id, organizationID, locationID, actorID string, payload map[string]any) (model.Record, error) {
	asset, _, state, err := s.fixedAssetContext(id, organizationID, locationID)
	if err != nil {
		return model.Record{}, err
	}
	effectiveDate := firstNonEmptyString(textValue(payload["effective_date"]), time.Now().UTC().Format("2006-01-02"))
	if err := s.validateFixedAssetEventDate(asset, effectiveDate); err != nil {
		return model.Record{}, err
	}
	if state.LifecycleStatus == "disposed" || state.LifecycleStatus == "retired" {
		return model.Record{}, shared.Conflict("disposed fixed assets cannot be transferred")
	}
	fromLocation := firstNonEmptyString(textValue(asset.Values["current_location_id"]), textValue(asset.Values["location_id"]))
	fromCostCenter := firstNonEmptyString(textValue(asset.Values["current_cost_center_code"]), textValue(asset.Values["cost_center_code"]))
	toLocation := firstNonEmptyString(textValue(payload["to_location_id"]), fromLocation)
	toCostCenter := firstNonEmptyString(textValue(payload["to_cost_center_code"]), fromCostCenter)
	if err := validateLocationID(s.models, toLocation); err != nil {
		return model.Record{}, err
	}
	if err := validateCostCenterID(s.models, toCostCenter); err != nil {
		return model.Record{}, err
	}
	if toLocation == fromLocation && toCostCenter == fromCostCenter {
		return model.Record{}, shared.Validation("transfer target must change location or cost center")
	}
	event, err := s.models.Create("asset_transfer", actorID, map[string]any{
		"organization_id":       textValue(asset.Values["organization_id"]),
		"location_id":           textValue(asset.Values["location_id"]),
		"fixed_asset_id":        asset.ID,
		"effective_date":        effectiveDate,
		"from_location_id":      fromLocation,
		"to_location_id":        toLocation,
		"from_cost_center_code": fromCostCenter,
		"to_cost_center_code":   toCostCenter,
		"status":                "posted",
		"notes":                 textValue(payload["notes"]),
	})
	if err != nil {
		return model.Record{}, err
	}
	if err := s.recomputeFixedAssetState(asset.ID, actorID); err != nil {
		return model.Record{}, err
	}
	return event, nil
}

func (s *FinanceAssetCoreService) ImpairFixedAsset(id, organizationID, locationID, actorID string, payload map[string]any) (map[string]any, error) {
	asset, schedule, state, err := s.fixedAssetContext(id, organizationID, locationID)
	if err != nil {
		return nil, err
	}
	impairmentDate := firstNonEmptyString(textValue(payload["impairment_date"]), time.Now().UTC().Format("2006-01-02"))
	if err := s.validateFixedAssetEventDate(asset, impairmentDate); err != nil {
		return nil, err
	}
	if state.LifecycleStatus == "disposed" || state.LifecycleStatus == "retired" {
		return nil, shared.Conflict("disposed fixed assets cannot be impaired")
	}
	amount := roundMoney(numberValue(payload["impairment_amount"]))
	if amount <= 0 {
		return nil, shared.Validation("impairment amount must be greater than zero")
	}
	if amount > state.CarryingAmount {
		return nil, shared.Validation("impairment amount exceeds current carrying amount")
	}
	cfg := s.assetPostingConfig(textValue(asset.Values["organization_id"]), textValue(asset.Values["location_id"]))
	expenseAccount := firstNonEmptyString(textValue(payload["impairment_expense_account_code"]), cfg["fixed_asset_default_impairment_expense_account_code"], "6210-IMP")
	accumulatedAccount := firstNonEmptyString(textValue(payload["accumulated_impairment_account_code"]), cfg["fixed_asset_default_accumulated_impairment_account_code"], "1595-ACC-IMP")
	event, err := s.models.Create("asset_impairment", actorID, map[string]any{
		"organization_id":                     textValue(asset.Values["organization_id"]),
		"location_id":                         textValue(asset.Values["location_id"]),
		"fixed_asset_id":                      asset.ID,
		"fixed_asset_schedule_id":             schedule.ID,
		"impairment_date":                     impairmentDate,
		"impairment_amount":                   amount,
		"status":                              "posted",
		"impairment_expense_account_code":     expenseAccount,
		"accumulated_impairment_account_code": accumulatedAccount,
		"notes":                               textValue(payload["notes"]),
	})
	if err != nil {
		return nil, err
	}
	lines := []map[string]any{
		{"account_code": expenseAccount, "account_name": "Impairment Loss", "description": "Asset impairment", "debit": amount, "credit": 0.0},
		{"account_code": accumulatedAccount, "account_name": "Accumulated Impairment", "description": "Asset impairment", "debit": 0.0, "credit": amount},
	}
	posting, err := s.createAssetLifecyclePosting(asset, actorID, "asset_impairment", event.ID, impairmentDate, amount, lines, schedule.ID)
	if err != nil {
		return nil, err
	}
	if event, err = s.models.Update("asset_impairment", event.ID, actorID, mergeRecordValues(event.Values, map[string]any{"posting_id": posting.Header.ID, "posting_date": impairmentDate}), event.Version); err != nil {
		return nil, err
	}
	if err := s.recomputeFixedAssetState(asset.ID, actorID); err != nil {
		return nil, err
	}
	return map[string]any{"event": event, "posting": posting}, nil
}

func (s *FinanceAssetCoreService) RevalueFixedAsset(id, organizationID, locationID, actorID string, payload map[string]any) (map[string]any, error) {
	asset, schedule, state, err := s.fixedAssetContext(id, organizationID, locationID)
	if err != nil {
		return nil, err
	}
	revaluationDate := firstNonEmptyString(textValue(payload["revaluation_date"]), time.Now().UTC().Format("2006-01-02"))
	if err := s.validateFixedAssetEventDate(asset, revaluationDate); err != nil {
		return nil, err
	}
	if state.LifecycleStatus == "disposed" || state.LifecycleStatus == "retired" {
		return nil, shared.Conflict("disposed fixed assets cannot be revalued")
	}
	targetCarrying := roundMoney(numberValue(payload["target_carrying_amount"]))
	delta := roundMoney(numberValue(payload["revaluation_amount"]))
	if targetCarrying > 0 {
		delta = roundMoney(targetCarrying - state.CarryingAmount)
	}
	if delta == 0 {
		return nil, shared.Validation("revaluation amount must change the carrying amount")
	}
	if roundMoney(state.CarryingAmount+delta) < 0 {
		return nil, shared.Validation("revaluation would reduce carrying amount below zero")
	}
	cfg := s.assetPostingConfig(textValue(asset.Values["organization_id"]), textValue(asset.Values["location_id"]))
	reserveAccount := firstNonEmptyString(textValue(payload["revaluation_reserve_account_code"]), cfg["fixed_asset_default_revaluation_reserve_account_code"], "3105-FA-REVAL")
	lossAccount := firstNonEmptyString(textValue(payload["revaluation_loss_account_code"]), cfg["fixed_asset_default_revaluation_loss_account_code"], "6215-FA-REVAL-LOSS")
	event, err := s.models.Create("asset_revaluation", actorID, map[string]any{
		"organization_id":                  textValue(asset.Values["organization_id"]),
		"location_id":                      textValue(asset.Values["location_id"]),
		"fixed_asset_id":                   asset.ID,
		"fixed_asset_schedule_id":          schedule.ID,
		"revaluation_date":                 revaluationDate,
		"revaluation_amount":               delta,
		"target_carrying_amount":           roundMoney(state.CarryingAmount + delta),
		"status":                           "posted",
		"revaluation_reserve_account_code": reserveAccount,
		"revaluation_loss_account_code":    lossAccount,
		"notes":                            textValue(payload["notes"]),
	})
	if err != nil {
		return nil, err
	}
	lines := []map[string]any{}
	if delta > 0 {
		lines = append(lines,
			map[string]any{"account_code": textValue(asset.Values["asset_account_code"]), "account_name": "Fixed Asset", "description": "Asset revaluation", "debit": delta, "credit": 0.0},
			map[string]any{"account_code": reserveAccount, "account_name": "Revaluation Reserve", "description": "Asset revaluation", "debit": 0.0, "credit": delta},
		)
	} else {
		amount := roundMoney(-delta)
		lines = append(lines,
			map[string]any{"account_code": lossAccount, "account_name": "Revaluation Loss", "description": "Asset revaluation", "debit": amount, "credit": 0.0},
			map[string]any{"account_code": textValue(asset.Values["asset_account_code"]), "account_name": "Fixed Asset", "description": "Asset revaluation", "debit": 0.0, "credit": amount},
		)
	}
	posting, err := s.createAssetLifecyclePosting(asset, actorID, "asset_revaluation", event.ID, revaluationDate, roundMoney(maxFloat(delta, -delta)), lines, schedule.ID)
	if err != nil {
		return nil, err
	}
	if event, err = s.models.Update("asset_revaluation", event.ID, actorID, mergeRecordValues(event.Values, map[string]any{"posting_id": posting.Header.ID, "posting_date": revaluationDate}), event.Version); err != nil {
		return nil, err
	}
	if err := s.recomputeFixedAssetState(asset.ID, actorID); err != nil {
		return nil, err
	}
	return map[string]any{"event": event, "posting": posting}, nil
}

func (s *FinanceAssetCoreService) HandleApprovedLedgerPosting(record document.Record, actorID string) error {
	if record.Header.Type != "ledger_posting" {
		return nil
	}
	if assetID := strings.TrimSpace(textValue(record.Body.Payload["fixed_asset_id"])); assetID != "" {
		return s.recomputeFixedAssetState(assetID, actorID)
	}
	return s.applyPostingToSchedule(record, actorID, false)
}

func (s *FinanceAssetCoreService) HandleCanceledLedgerPosting(record document.Record, actorID string) error {
	if record.Header.Type != "ledger_posting" {
		return nil
	}
	if assetID := strings.TrimSpace(textValue(record.Body.Payload["fixed_asset_id"])); assetID != "" {
		return s.recomputeFixedAssetState(assetID, actorID)
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
	assetUpdate["current_location_id"] = textValue(assetRecord.Values["location_id"])
	assetUpdate["origin_location_id"] = textValue(assetRecord.Values["origin_location_id"])
	assetUpdate["current_cost_center_code"] = textValue(assetRecord.Values["cost_center_code"])
	assetUpdate["origin_cost_center_code"] = textValue(assetRecord.Values["origin_cost_center_code"])
	assetUpdate["gross_amount_current"] = roundMoney(numberValue(assetRecord.Values["basis_amount"]))
	assetUpdate["carrying_amount"] = roundMoney(numberValue(scheduleRecord.Values["remaining_amount"]))
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
	next["origin_location_id"] = firstNonEmptyString(textValue(next["origin_location_id"]), strings.TrimSpace(locationID))
	next["current_location_id"] = firstNonEmptyString(textValue(next["current_location_id"]), strings.TrimSpace(locationID))
	next["cost_center_code"] = textValue(next["cost_center_code"])
	next["origin_cost_center_code"] = firstNonEmptyString(textValue(next["origin_cost_center_code"]), textValue(next["cost_center_code"]))
	next["current_cost_center_code"] = firstNonEmptyString(textValue(next["current_cost_center_code"]), textValue(next["cost_center_code"]))
	next["lifecycle_status"] = firstNonEmptyString(textValue(next["lifecycle_status"]), "active")
	next["impairment_amount_total"] = roundMoney(numberValue(next["impairment_amount_total"]))
	next["revaluation_amount_total"] = roundMoney(numberValue(next["revaluation_amount_total"]))
	next["gross_amount_current"] = basisAmount
	next["carrying_amount"] = roundMoney(basisAmount - salvageAmount)
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
		"organization_id":               strings.TrimSpace(organizationID),
		"location_id":                   strings.TrimSpace(locationID),
		"status":                        status,
		"method":                        method,
		"cadence":                       cadence,
		"total_periods":                 totalPeriods,
		"periods_booked":                0,
		"basis_amount":                  basisAmount,
		"depreciable_basis_amount":      roundMoney(basisAmount - salvageAmount),
		"booked_amount":                 0.0,
		"remaining_amount":              roundMoney(basisAmount - salvageAmount),
		"accumulated_adjustment_amount": 0.0,
		"declining_rate_percent":        roundMoney(numberValue(next["declining_rate_percent"])),
		"next_posting_date":             scheduleFirstDueDate(startDate, cadence),
		"linked_journal_template_id":    "",
		"last_rebase_date":              "",
		"last_rebase_reason":            "",
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
		"organization_id":           textValue(schedule.Values["organization_id"]),
		"location_id":               textValue(schedule.Values["location_id"]),
		"code":                      scheduleTemplateCode(scheduleModelKey, parent),
		"name":                      scheduleTemplateName(scheduleModelKey, parent),
		"journal_kind":              scheduleTemplateKind(scheduleModelKey),
		"cadence":                   firstNonEmptyString(textValue(schedule.Values["cadence"]), "monthly"),
		"currency_code":             "IDR",
		"description_template":      scheduleTemplateDescription(scheduleModelKey, parent),
		"required_for_period_close": true,
		"source_model_type":         sourceModelType,
		"source_model_id":           sourceModelID,
		"next_due_date":             preview.NextPostingDate,
		"status":                    "active",
		"notes":                     fmt.Sprintf("Auto-managed from %s %s", scheduleModelKey, parent.ID),
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
	return s.recomputeScheduledAsset(sourceType, sourceID, actorID)
}

func (s *FinanceAssetCoreService) recomputeScheduleValues(templateID, scheduleModelKey string, schedule model.Record) (map[string]any, assetLifecycleState, error) {
	values := cloneMap(schedule.Values)
	parent, err := s.parentForSchedule(scheduleModelKey, schedule)
	if err != nil {
		return nil, assetLifecycleState{}, err
	}
	state := assetLifecycleState{
		CurrentLocationID:     firstNonEmptyString(textValue(parent.Values["origin_location_id"]), textValue(parent.Values["location_id"])),
		CurrentCostCenterCode: firstNonEmptyString(textValue(parent.Values["origin_cost_center_code"]), textValue(parent.Values["cost_center_code"])),
		LifecycleStatus:       "active",
	}
	baseRemaining := roundMoney(numberValue(values["basis_amount"]))
	if scheduleModelKey == "fixed_asset_schedule" {
		baseRemaining = roundMoney(baseRemaining - numberValue(values["salvage_amount"]))
		lifecycleState, lifecycleErr := s.fixedAssetLifecycleState(parent, schedule)
		if lifecycleErr != nil {
			return nil, assetLifecycleState{}, lifecycleErr
		}
		state = lifecycleState
		baseRemaining = roundMoney(maxFloat(0, baseRemaining+state.AdjustmentAmountTotal))
		values["depreciable_basis_amount"] = baseRemaining
		values["accumulated_adjustment_amount"] = roundMoney(state.AdjustmentAmountTotal)
		values["last_rebase_date"] = state.LastEventDate
		values["last_rebase_reason"] = state.LastEventType
		values["location_id"] = state.CurrentLocationID
		if state.LifecycleStatus == "disposed" || state.LifecycleStatus == "retired" {
			values["status"] = "completed"
		}
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
		return nil, assetLifecycleState{}, err
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
	if state.LifecycleStatus == "disposed" || state.LifecycleStatus == "retired" {
		values["status"] = "completed"
		values["remaining_amount"] = 0.0
		values["next_posting_date"] = ""
	} else if assetIntValue(values["periods_booked"], 0) >= assetIntValue(values["total_periods"], 0) || roundMoney(numberValue(values["remaining_amount"])) <= 0 {
		values["status"] = "completed"
		values["next_posting_date"] = ""
	}
	return values, state, nil
}

func (s *FinanceAssetCoreService) updateParentFromSchedule(scheduleModelKey string, schedule, template model.Record, state assetLifecycleState, actorID string) error {
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
	if scheduleModelKey == "fixed_asset_schedule" {
		values["location_id"] = state.CurrentLocationID
		values["current_location_id"] = state.CurrentLocationID
		values["cost_center_code"] = state.CurrentCostCenterCode
		values["current_cost_center_code"] = state.CurrentCostCenterCode
		values["lifecycle_status"] = firstNonEmptyString(state.LifecycleStatus, "active")
		values["impairment_amount_total"] = roundMoney(state.ImpairmentAmountTotal)
		values["revaluation_amount_total"] = roundMoney(state.NetRevaluationAmount)
		values["gross_amount_current"] = roundMoney(state.GrossAmount)
		values["carrying_amount"] = roundMoney(state.CarryingAmount)
		values["last_lifecycle_event_type"] = state.LastEventType
		values["last_lifecycle_event_id"] = state.LastEventID
		values["disposal_date"] = state.DisposalDate
		values["disposal_type"] = state.DisposalType
		values["disposal_posting_id"] = state.DisposalPostingID
		if state.LifecycleStatus == "disposed" || state.LifecycleStatus == "retired" {
			values["status"] = state.LifecycleStatus
		}
	}
	_, err = s.models.Update(parentModelKey, parent.ID, actorID, values, parent.Version)
	return err
}

func (s *FinanceAssetCoreService) recomputeScheduledAsset(scheduleModelKey, scheduleID, actorID string) error {
	if s.models == nil {
		return shared.Validation("models are unavailable")
	}
	schedule, err := s.models.Get(scheduleModelKey, scheduleID)
	if err != nil {
		return err
	}
	templateID := strings.TrimSpace(textValue(schedule.Values["linked_journal_template_id"]))
	values, state, err := s.recomputeScheduleValues(templateID, scheduleModelKey, schedule)
	if err != nil {
		return err
	}
	schedule, err = s.models.Update(scheduleModelKey, schedule.ID, actorID, values, schedule.Version)
	if err != nil {
		return err
	}
	template, schedule, err := s.syncScheduleTemplate(scheduleModelKey, schedule, actorID)
	if err != nil {
		return err
	}
	return s.updateParentFromSchedule(scheduleModelKey, schedule, template, state, actorID)
}

func (s *FinanceAssetCoreService) recomputeFixedAssetState(assetID, actorID string) error {
	asset, err := s.models.Get("fixed_asset", strings.TrimSpace(assetID))
	if err != nil {
		return err
	}
	scheduleID := strings.TrimSpace(textValue(asset.Values["schedule_id"]))
	if scheduleID == "" {
		return nil
	}
	return s.recomputeScheduledAsset("fixed_asset_schedule", scheduleID, actorID)
}

func (s *FinanceAssetCoreService) fixedAssetContext(id, organizationID, locationID string) (model.Record, model.Record, assetLifecycleState, error) {
	asset, err := s.scopedModelRecord("fixed_asset", strings.TrimSpace(id), organizationID, locationID)
	if err != nil {
		return model.Record{}, model.Record{}, assetLifecycleState{}, err
	}
	scheduleID := strings.TrimSpace(textValue(asset.Values["schedule_id"]))
	if scheduleID == "" {
		return model.Record{}, model.Record{}, assetLifecycleState{}, shared.Validation("fixed asset schedule is not initialized")
	}
	schedule, err := s.scopedModelRecord("fixed_asset_schedule", scheduleID, organizationID, locationID)
	if err != nil {
		return model.Record{}, model.Record{}, assetLifecycleState{}, err
	}
	state, err := s.fixedAssetLifecycleState(asset, schedule)
	if err != nil {
		return model.Record{}, model.Record{}, assetLifecycleState{}, err
	}
	return asset, schedule, state, nil
}

func (s *FinanceAssetCoreService) fixedAssetLifecycleState(asset, schedule model.Record) (assetLifecycleState, error) {
	state := assetLifecycleState{
		CurrentLocationID:     firstNonEmptyString(textValue(asset.Values["origin_location_id"]), textValue(asset.Values["location_id"])),
		CurrentCostCenterCode: firstNonEmptyString(textValue(asset.Values["origin_cost_center_code"]), textValue(asset.Values["cost_center_code"])),
		LifecycleStatus:       "active",
	}
	grossAmount := roundMoney(numberValue(asset.Values["basis_amount"]))
	accumulatedDep := roundMoney(numberValue(schedule.Values["booked_amount"]))
	accumulatedImpairment := 0.0
	netRevaluation := 0.0
	events, err := s.fixedAssetLifecycleEvents(asset.ID)
	if err != nil {
		return state, err
	}
	lastEventDate := ""
	for _, event := range events {
		eventDate := firstNonEmptyString(textValue(event.Values["effective_date"]), textValue(event.Values["disposal_date"]), textValue(event.Values["impairment_date"]), textValue(event.Values["revaluation_date"]))
		switch event.ModelKey {
		case "asset_transfer":
			state.CurrentLocationID = firstNonEmptyString(textValue(event.Values["to_location_id"]), state.CurrentLocationID)
			state.CurrentCostCenterCode = firstNonEmptyString(textValue(event.Values["to_cost_center_code"]), state.CurrentCostCenterCode)
			state.LifecycleStatus = "transferred"
		case "asset_impairment":
			amount := roundMoney(numberValue(event.Values["impairment_amount"]))
			state.ImpairmentAmountTotal = roundMoney(state.ImpairmentAmountTotal + amount)
			accumulatedImpairment = roundMoney(accumulatedImpairment + amount)
			state.AdjustmentAmountTotal = roundMoney(state.AdjustmentAmountTotal - amount)
			state.LifecycleStatus = "impaired"
		case "asset_revaluation":
			amount := roundMoney(numberValue(event.Values["revaluation_amount"]))
			netRevaluation = roundMoney(netRevaluation + amount)
			state.NetRevaluationAmount = netRevaluation
			state.AdjustmentAmountTotal = roundMoney(state.AdjustmentAmountTotal + amount)
			state.LifecycleStatus = "revalued"
		case "asset_disposal":
			state.DisposalDate = textValue(event.Values["disposal_date"])
			state.DisposalType = textValue(event.Values["disposal_type"])
			state.DisposalPostingID = textValue(event.Values["posting_id"])
			if strings.EqualFold(state.DisposalType, "retirement") {
				state.LifecycleStatus = "retired"
			} else {
				state.LifecycleStatus = "disposed"
			}
		}
		if eventDate >= lastEventDate {
			lastEventDate = eventDate
			state.LastEventDate = eventDate
			state.LastEventType = event.ModelKey
			state.LastEventID = event.ID
		}
	}
	grossAmount = roundMoney(grossAmount + netRevaluation)
	carrying := roundMoney(maxFloat(0, grossAmount-accumulatedDep-accumulatedImpairment))
	if state.LifecycleStatus == "disposed" || state.LifecycleStatus == "retired" {
		carrying = 0
	}
	state.GrossAmount = grossAmount
	state.AccumulatedImpairment = accumulatedImpairment
	state.CarryingAmount = carrying
	return state, nil
}

func (s *FinanceAssetCoreService) fixedAssetLifecycleEvents(assetID string) ([]model.Record, error) {
	eventModelKeys := []string{"asset_transfer", "asset_impairment", "asset_revaluation", "asset_disposal"}
	records := make([]model.Record, 0)
	for _, modelKey := range eventModelKeys {
		items, _, err := s.models.List(modelKey, model.Query{
			Filters:  map[string]string{"fixed_asset_id": strings.TrimSpace(assetID)},
			Page:     1,
			PageSize: 500,
		})
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if !s.assetLifecycleEventPosted(item) {
				continue
			}
			records = append(records, item)
		}
	}
	sort.Slice(records, func(i, j int) bool {
		leftDate := firstNonEmptyString(textValue(records[i].Values["effective_date"]), textValue(records[i].Values["disposal_date"]), textValue(records[i].Values["impairment_date"]), textValue(records[i].Values["revaluation_date"]))
		rightDate := firstNonEmptyString(textValue(records[j].Values["effective_date"]), textValue(records[j].Values["disposal_date"]), textValue(records[j].Values["impairment_date"]), textValue(records[j].Values["revaluation_date"]))
		if leftDate == rightDate {
			return records[i].CreatedAt.Before(records[j].CreatedAt)
		}
		return leftDate < rightDate
	})
	return records, nil
}

func (s *FinanceAssetCoreService) assetLifecycleEventPosted(record model.Record) bool {
	if !strings.EqualFold(textValue(record.Values["status"]), "posted") {
		return false
	}
	postingID := strings.TrimSpace(textValue(record.Values["posting_id"]))
	if postingID == "" {
		return true
	}
	if s.documents == nil {
		return false
	}
	posting, err := s.documents.Get(postingID)
	return err == nil && posting.Header.Status == "posted"
}

func (s *FinanceAssetCoreService) validateFixedAssetEventDate(asset model.Record, eventDate string) error {
	eventDate = strings.TrimSpace(eventDate)
	if eventDate == "" {
		return shared.Validation("event date is required")
	}
	if s.finance != nil {
		if err := s.finance.ValidatePostingDateOpen(textValue(asset.Values["organization_id"]), textValue(asset.Values["location_id"]), eventDate); err != nil {
			return err
		}
	}
	templateID := strings.TrimSpace(textValue(asset.Values["linked_journal_template_id"]))
	if templateID != "" {
		latest := s.latestPostedRun(templateID, "")
		if latest.Header.ID != "" {
			latestDate := strings.TrimSpace(textValue(latest.Body.Payload["posting_date"]))
			if latestDate != "" && eventDate < latestDate {
				return shared.Conflict("asset lifecycle events must not predate posted depreciation")
			}
		}
	}
	events, err := s.fixedAssetLifecycleEvents(asset.ID)
	if err != nil {
		return err
	}
	for _, event := range events {
		existingDate := firstNonEmptyString(textValue(event.Values["effective_date"]), textValue(event.Values["disposal_date"]), textValue(event.Values["impairment_date"]), textValue(event.Values["revaluation_date"]))
		if existingDate != "" && eventDate < existingDate {
			return shared.Conflict("asset lifecycle events must be posted in chronological order")
		}
	}
	return nil
}

func (s *FinanceAssetCoreService) createAssetLifecyclePosting(asset model.Record, actorID, sourceType, sourceID, postingDate string, totalAmount float64, lines []map[string]any, scheduleID string) (document.Record, error) {
	if s.documents == nil {
		return document.Record{}, shared.Validation("documents are unavailable")
	}
	payload := map[string]any{
		"posting_date":         strings.TrimSpace(postingDate),
		"currency_code":        "IDR",
		"posting_rule_key":     sourceType,
		"journal_source_kind":  "system",
		"source_document_type": sourceType,
		"source_document_id":   sourceID,
		"fixed_asset_id":       asset.ID,
		"asset_schedule_id":    scheduleID,
		"total_amount":         roundMoney(totalAmount),
		"journal_lines":        lines,
	}
	posting, err := s.documents.Create("ledger_posting", textValue(asset.Values["organization_id"]), textValue(asset.Values["location_id"]), actorID, payload)
	if err != nil {
		return document.Record{}, err
	}
	posting.Header.Status = "posted"
	posting.Header.UpdatedBy = actorID
	posting.Header.UpdatedAt = time.Now().UTC()
	if err := s.documents.Save(posting); err != nil {
		return document.Record{}, err
	}
	return posting, nil
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
		"fixed_asset_default_accumulated_impairment_account_code":   "1595-ACC-IMP",
		"fixed_asset_default_impairment_expense_account_code":       "6210-IMP",
		"fixed_asset_default_revaluation_reserve_account_code":      "3105-FA-REVAL",
		"fixed_asset_default_revaluation_loss_account_code":         "6215-FA-REVAL-LOSS",
		"fixed_asset_disposal_gain_account_code":                    "7200-FA-GAIN",
		"fixed_asset_disposal_loss_account_code":                    "6205-FA-LOSS",
		"fixed_asset_sale_proceeds_account_code":                    "1000-CASH",
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
		month := ((int(parsed.Month())-1)/3 + 1) * 3
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
	return "accrual"
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
