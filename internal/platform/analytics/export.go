package analytics

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"time"

	"github.com/jung-kurt/gofpdf"
	"github.com/xuri/excelize/v2"

	"orbyte/internal/platform/shared"
)

func (s *Service) ExportDocumentReportingCSV(query FactQuery, dimension string) ([]byte, error) {
	rows := s.ReportingBreakdown(query, dimension)
	buf := &bytes.Buffer{}
	writer := csv.NewWriter(buf)
	if err := writer.Write([]string{"dimension_type", "dimension_key", "label", "created", "draft", "submitted", "approved", "rejected", "cancelled"}); err != nil {
		return nil, err
	}
	for _, row := range rows {
		record := []string{
			row.DimensionType,
			row.DimensionKey,
			row.Label,
			strconv.Itoa(row.Created),
			strconv.Itoa(row.Draft),
			strconv.Itoa(row.Submitted),
			strconv.Itoa(row.Approved),
			strconv.Itoa(row.Rejected),
			strconv.Itoa(row.Cancelled),
		}
		if err := writer.Write(record); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s *Service) ExportDocumentReportingXLSX(query FactQuery, dimension string) ([]byte, error) {
	rows := s.ReportingBreakdown(query, dimension)
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	headers := []string{"dimension_type", "dimension_key", "label", "created", "draft", "submitted", "approved", "rejected", "cancelled"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheet, cell, header); err != nil {
			return nil, err
		}
	}
	for rowIndex, row := range rows {
		values := []any{row.DimensionType, row.DimensionKey, row.Label, row.Created, row.Draft, row.Submitted, row.Approved, row.Rejected, row.Cancelled}
		for colIndex, value := range values {
			cell, _ := excelize.CoordinatesToCellName(colIndex+1, rowIndex+2)
			if err := f.SetCellValue(sheet, cell, value); err != nil {
				return nil, err
			}
		}
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s *Service) ExportDocumentReportingPDF(query FactQuery, dimension string) ([]byte, error) {
	rows := s.ReportingBreakdown(query, dimension)
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(10, 10, 10)
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 14)
	pdf.CellFormat(190, 8, "Analytics Reporting Documents", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(190, 6, "Dimension: "+dimension, "", 1, "L", false, 0, "")
	pdf.Ln(2)

	headers := []string{"Type", "Key", "Label", "Created", "Draft", "Submitted", "Approved", "Rejected", "Cancelled"}
	widths := []float64{20, 24, 42, 16, 14, 20, 18, 18, 18}
	pdf.SetFont("Arial", "B", 8)
	for i, header := range headers {
		pdf.CellFormat(widths[i], 7, header, "1", 0, "C", false, 0, "")
	}
	pdf.Ln(-1)
	pdf.SetFont("Arial", "", 8)
	for _, row := range rows {
		values := []string{
			row.DimensionType,
			row.DimensionKey,
			row.Label,
			strconv.Itoa(row.Created),
			strconv.Itoa(row.Draft),
			strconv.Itoa(row.Submitted),
			strconv.Itoa(row.Approved),
			strconv.Itoa(row.Rejected),
			strconv.Itoa(row.Cancelled),
		}
		for i, value := range values {
			pdf.CellFormat(widths[i], 6, value, "1", 0, "L", false, 0, "")
		}
		pdf.Ln(-1)
	}
	buf := &bytes.Buffer{}
	if err := pdf.Output(buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s *Service) CreateReportDefinition(def ReportDefinition) (ReportDefinition, error) {
	if def.ID == "" {
		def.ID = fmt.Sprintf("report:%d", time.Now().UTC().UnixNano())
	}
	if def.Name == "" {
		def.Name = def.Dimension + " report"
	}
	if def.Dimension == "" {
		def.Dimension = "document_type"
	}
	if def.Format == "" {
		def.Format = "csv"
	}
	if def.Window == "" {
		def.Window = "current_state"
	}
	if def.Schedule == "" {
		def.Schedule = "daily"
	}
	if def.NextRunAt.IsZero() {
		def.NextRunAt = nextRun(def.Schedule, time.Now().UTC())
	}
	def.Enabled = true
	if s == nil || s.repo == nil {
		return ReportDefinition{}, shared.Conflict("analytics repository is not configured")
	}
	if err := s.repo.SaveReportDefinition(def); err != nil {
		return ReportDefinition{}, err
	}
	return def, nil
}

func (s *Service) EnsureReportDefinition(def ReportDefinition) (ReportDefinition, error) {
	for _, existing := range s.ListReportDefinitions() {
		if existing.ID == def.ID && def.ID != "" {
			return existing, nil
		}
		if existing.Name == def.Name && def.Name != "" {
			return existing, nil
		}
	}
	return s.CreateReportDefinition(def)
}

func (s *Service) ListReportDefinitions() []ReportDefinition {
	if s.repo == nil {
		return nil
	}
	return s.repo.ListReportDefinitions()
}

func (s *Service) ListReportRuns() []ReportRun {
	if s.repo == nil {
		return nil
	}
	return s.repo.ListReportRuns()
}

func (s *Service) ListReportArtifacts() []ReportArtifact {
	if s.repo == nil {
		return nil
	}
	return s.repo.ListReportArtifacts()
}

func (s *Service) ListReportDeliveries() []ReportDelivery {
	if s.repo == nil {
		return nil
	}
	return s.repo.ListReportDeliveries()
}

func (s *Service) ListReportDeliveryDeadLetters() []ReportDeliveryDeadLetter {
	if s.repo == nil {
		return nil
	}
	return s.repo.ListReportDeliveryDeadLetters()
}

func (s *Service) DeliverArtifact(artifactID, channel, recipient string) (ReportDelivery, error) {
	artifact, ok := s.GetReportArtifact(artifactID)
	if !ok {
		return ReportDelivery{}, fmt.Errorf("report artifact not found")
	}
	if channel == "" {
		channel = "download"
	}
	attemptCount := 1
	for _, prior := range s.ListReportDeliveries() {
		if prior.ArtifactID == artifactID && prior.Channel == channel && prior.Recipient == recipient && prior.AttemptCount >= attemptCount {
			attemptCount = prior.AttemptCount + 1
		}
	}
	status := "queued"
	deliveredAt := time.Time{}
	lastError := ""
	adapter := s.deliveryAdapter(channel)
	if adapter == nil {
		lastError = "unsupported delivery channel"
	} else if err := adapter.Deliver(artifact, recipient); err != nil {
		lastError = err.Error()
	} else {
		status = "delivered"
		deliveredAt = time.Now().UTC()
	}
	if status != "delivered" {
		if attemptCount >= 3 {
			status = "dead_letter"
		} else {
			status = "failed"
		}
	}
	delivery := ReportDelivery{
		ID:           fmt.Sprintf("delivery:%d", time.Now().UTC().UnixNano()),
		ArtifactID:   artifact.ID,
		Channel:      channel,
		Recipient:    recipient,
		Status:       status,
		AttemptCount: attemptCount,
		LastError:    lastError,
		CreatedAt:    time.Now().UTC(),
		DeliveredAt:  deliveredAt,
	}
	if err := s.repo.SaveReportDelivery(delivery); err != nil {
		return ReportDelivery{}, err
	}
	if status == "dead_letter" {
		if err := s.repo.SaveReportDeliveryDeadLetter(ReportDeliveryDeadLetter{
			ID:           fmt.Sprintf("delivery-dead:%d", time.Now().UTC().UnixNano()),
			ArtifactID:   artifact.ID,
			Channel:      channel,
			Recipient:    recipient,
			Reason:       lastError,
			AttemptCount: attemptCount,
			CreatedAt:    time.Now().UTC(),
		}); err != nil {
			return ReportDelivery{}, err
		}
	}
	if status == "delivered" {
		return delivery, nil
	}
	return delivery, fmt.Errorf("%s", lastError)
}

func (s *Service) deliveryAdapter(channel string) DeliveryAdapter {
	switch channel {
	case "download":
		return DownloadAdapter{}
	case "filesystem":
		return FilesystemAdapter{}
	case "webhook":
		return WebhookAdapter{}
	case "email":
		return EmailAdapter{}
	case "object_store":
		return ObjectStoreAdapter{}
	default:
		return nil
	}
}

func (s *Service) GetReportArtifact(id string) (ReportArtifact, bool) {
	if s.repo == nil {
		return ReportArtifact{}, false
	}
	return s.repo.GetReportArtifact(id)
}

func (s *Service) RunReport(def ReportDefinition) (ReportRun, []byte, error) {
	query := FactQuery{LocationID: def.LocationID, DocumentType: def.DocumentType}
	var (
		content []byte
		err     error
	)
	switch def.Format {
	case "xlsx":
		content, err = s.ExportDocumentReportingXLSX(query, def.Dimension)
	case "pdf":
		content, err = s.ExportDocumentReportingPDF(query, def.Dimension)
	default:
		content, err = s.ExportDocumentReportingCSV(query, def.Dimension)
	}
	if err != nil {
		return ReportRun{}, nil, err
	}
	artifact := ReportArtifact{
		ID:          fmt.Sprintf("artifact:%d", time.Now().UTC().UnixNano()),
		ReportID:    def.ID,
		FileName:    "analytics_reporting_documents." + def.Format,
		ContentType: map[string]string{"xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "pdf": "application/pdf"}[def.Format],
		SizeBytes:   len(content),
		CreatedAt:   time.Now().UTC(),
		Content:     content,
	}
	if artifact.ContentType == "" {
		artifact.ContentType = "text/csv"
	}
	run := ReportRun{ID: fmt.Sprintf("report-run:%d", time.Now().UTC().UnixNano()), ReportID: def.ID, ArtifactID: artifact.ID, Format: def.Format, Status: "generated", GeneratedAt: time.Now().UTC(), RowCount: len(s.ReportingBreakdown(query, def.Dimension))}
	artifact.ReportRunID = run.ID
	if err := s.repo.SaveReportRun(run); err != nil {
		return ReportRun{}, nil, err
	}
	if err := s.repo.SaveReportArtifact(artifact); err != nil {
		return ReportRun{}, nil, err
	}
	if def.DeliveryChannel != "" {
		if _, err := s.DeliverArtifact(artifact.ID, def.DeliveryChannel, def.DeliveryTarget); err != nil {
			return ReportRun{}, nil, err
		}
	}
	return run, content, nil
}

func (s *Service) RunDueReports(now time.Time) error {
	for _, def := range s.ListReportDefinitions() {
		if !def.Enabled || def.NextRunAt.After(now) {
			continue
		}
		if _, _, err := s.RunReport(def); err != nil {
			return err
		}
		def.NextRunAt = nextRun(def.Schedule, now)
		if err := s.repo.UpdateReportDefinition(def); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) CleanupReportData(cutoff time.Time) error {
	if s.repo == nil {
		return nil
	}
	return s.repo.CleanupReportData(cutoff)
}
