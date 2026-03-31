package httpx

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	application "orbyte/internal/platform/application"
	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/config"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/observability"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/securityfields"
	"orbyte/internal/platform/shared"
)

type createDocumentRequest struct {
	Type           string         `json:"type"`
	OrganizationID string         `json:"organization_id"`
	LocationID     string         `json:"location_id"`
	Payload        map[string]any `json:"payload"`
}

type updateDocumentRequest struct {
	ExpectedVersion int            `json:"expected_version,omitempty"`
	ExpectedETag    string         `json:"expected_etag,omitempty"`
	Payload         map[string]any `json:"payload"`
}

type actionRequest struct {
	Action          string `json:"action"`
	ExpectedVersion int    `json:"expected_version,omitempty"`
	ExpectedETag    string `json:"expected_etag,omitempty"`
	Note            string `json:"note,omitempty"`
}

type updateDocumentExtensionRequest struct {
	ExpectedVersion int            `json:"expected_version,omitempty"`
	ExpectedETag    string         `json:"expected_etag,omitempty"`
	Payload         map[string]any `json:"payload"`
}

type createDocumentLinkRequest struct {
	LinkedDocumentID string         `json:"linked_document_id"`
	LinkType         string         `json:"link_type"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

type createDocumentAttachmentRequest struct {
	AttachmentType string `json:"attachment_type"`
	FileName       string `json:"file_name"`
	ContentType    string `json:"content_type"`
	StorageKey     string `json:"storage_key"`
	SizeBytes      int64  `json:"size_bytes"`
}

func registerDocumentRoutes(mux *http.ServeMux, cfg *config.Service, ident *identity.Service, modules *module.Service, docs *document.Service, docActions *application.DocumentActions, commercialSvc *application.CommercialCoreService, procurementSvc *application.ProcurementCoreService, inventorySvc *application.InventoryCoreService, fulfillmentSvc *application.FulfillmentCoreService, deliverySvc *application.DeliveryCoreService, returnsSvc *application.ReturnsCoreService, supplierReturnsSvc *application.SupplierReturnsCoreService, productionSvc *application.ProductionCoreService, traceabilitySvc *application.TraceabilityCoreService, recallSvc *application.RecallCoreService, payrollSvc *application.EmployeePayrollCoreService, auditSvc *audit.Service, policySvc *policy.Service, searchSvc *search.Service, fieldSecurity *securityfields.Service, obs *observability.Service) {
	_ = traceabilitySvc
	if commercialSvc != nil {
		mux.HandleFunc("POST /commercial/products/", func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasSuffix(r.URL.Path, "/generate-variants") {
				http.NotFound(w, r)
				return
			}
			p, ok := requireInteractivePrincipal(w, r)
			if !ok {
				return
			}
			if !principalAllowsAll(ident, p, []string{"product.read", "item.create"}) {
				respondError(w, shared.Forbidden("variant generation is not allowed"))
				return
			}
			productID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/commercial/products/"), "/generate-variants")
			var req struct {
				Dimensions []application.VariantDimensionSelection `json:"dimensions"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				respondError(w, shared.Validation("invalid variant generation payload"))
				return
			}
			records, err := commercialSvc.GenerateVariantsForProduct(productID, principalEffectiveUserID(p), req.Dimensions)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusCreated, map[string]any{"items": records})
		})

		mux.HandleFunc("POST /commercial/orders/", func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.HasSuffix(r.URL.Path, "/generate-invoice"):
				documentID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/commercial/orders/"), "/generate-invoice")
				order, err := docs.Get(documentID)
				if err != nil {
					respondError(w, err)
					return
				}
				p, ok := requireAuthorization(w, r, ident, "document.create", order.Header.LocationID, "")
				if !ok {
					return
				}
				record, err := commercialSvc.GenerateInvoiceFromOrder(documentID, principalEffectiveUserID(p))
				if err != nil {
					respondError(w, err)
					return
				}
				refreshDocumentSearch(searchSvc, record)
				rendered := docs.Render(record, document.ViewExpanded, modules.EnabledMap())
				respondJSON(w, http.StatusCreated, sanitizeDocumentRecord(fieldSecurity, ident, p, rendered, "api"))
			case strings.HasSuffix(r.URL.Path, "/generate-fulfillment"):
				if fulfillmentSvc == nil {
					http.NotFound(w, r)
					return
				}
				documentID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/commercial/orders/"), "/generate-fulfillment")
				order, err := docs.Get(documentID)
				if err != nil {
					respondError(w, err)
					return
				}
				p, ok := requireAuthorization(w, r, ident, "document.create", order.Header.LocationID, "")
				if !ok {
					return
				}
				record, err := fulfillmentSvc.GenerateFulfillmentFromOrder(documentID, principalEffectiveUserID(p))
				if err != nil {
					respondError(w, err)
					return
				}
				refreshDocumentSearch(searchSvc, record)
				rendered := docs.Render(record, document.ViewExpanded, modules.EnabledMap())
				respondJSON(w, http.StatusCreated, sanitizeDocumentRecord(fieldSecurity, ident, p, rendered, "api"))
			case strings.HasSuffix(r.URL.Path, "/generate-production-order"):
				if productionSvc == nil {
					http.NotFound(w, r)
					return
				}
				documentID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/commercial/orders/"), "/generate-production-order")
				order, err := docs.Get(documentID)
				if err != nil {
					respondError(w, err)
					return
				}
				p, ok := requireAuthorization(w, r, ident, "document.create", order.Header.LocationID, "")
				if !ok {
					return
				}
				records, err := productionSvc.GenerateProductionOrdersFromSalesOrder(documentID, principalEffectiveUserID(p))
				if err != nil {
					respondError(w, err)
					return
				}
				items := make([]document.Record, 0, len(records))
				for _, item := range records {
					refreshDocumentSearch(searchSvc, item)
					items = append(items, sanitizeDocumentRecord(fieldSecurity, ident, p, docs.Render(item, document.ViewExpanded, modules.EnabledMap()), "api"))
				}
				respondJSON(w, http.StatusCreated, map[string]any{"items": items})
			default:
				http.NotFound(w, r)
			}
		})

		mux.HandleFunc("POST /commercial/invoices/", func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.HasSuffix(r.URL.Path, "/register-payment"):
				documentID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/commercial/invoices/"), "/register-payment")
				invoice, err := docs.Get(documentID)
				if err != nil {
					respondError(w, err)
					return
				}
				p, ok := requireAuthorization(w, r, ident, "document.create", invoice.Header.LocationID, "")
				if !ok {
					return
				}
				record, err := commercialSvc.CreatePaymentReceiptFromInvoice(documentID, principalEffectiveUserID(p))
				if err != nil {
					respondError(w, err)
					return
				}
				refreshDocumentSearch(searchSvc, record)
				rendered := docs.Render(record, document.ViewExpanded, modules.EnabledMap())
				respondJSON(w, http.StatusCreated, sanitizeDocumentRecord(fieldSecurity, ident, p, rendered, "api"))
			case strings.HasSuffix(r.URL.Path, "/issue-credit-note"):
				documentID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/commercial/invoices/"), "/issue-credit-note")
				invoice, err := docs.Get(documentID)
				if err != nil {
					respondError(w, err)
					return
				}
				p, ok := requireAuthorization(w, r, ident, "document.create", invoice.Header.LocationID, "")
				if !ok {
					return
				}
				record, err := commercialSvc.CreateCreditNoteFromInvoice(documentID, principalEffectiveUserID(p))
				if err != nil {
					respondError(w, err)
					return
				}
				refreshDocumentSearch(searchSvc, record)
				rendered := docs.Render(record, document.ViewExpanded, modules.EnabledMap())
				respondJSON(w, http.StatusCreated, sanitizeDocumentRecord(fieldSecurity, ident, p, rendered, "api"))
			default:
				http.NotFound(w, r)
			}
		})

		mux.HandleFunc("POST /commercial/credit-notes/", func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasSuffix(r.URL.Path, "/register-refund") {
				http.NotFound(w, r)
				return
			}
			documentID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/commercial/credit-notes/"), "/register-refund")
			creditNote, err := docs.Get(documentID)
			if err != nil {
				respondError(w, err)
				return
			}
			p, ok := requireAuthorization(w, r, ident, "document.create", creditNote.Header.LocationID, "")
			if !ok {
				return
			}
			record, err := commercialSvc.CreateRefundFromCreditNote(documentID, principalEffectiveUserID(p))
			if err != nil {
				respondError(w, err)
				return
			}
			refreshDocumentSearch(searchSvc, record)
			rendered := docs.Render(record, document.ViewExpanded, modules.EnabledMap())
			respondJSON(w, http.StatusCreated, sanitizeDocumentRecord(fieldSecurity, ident, p, rendered, "api"))
		})
	}
	if procurementSvc != nil {
		mux.HandleFunc("POST /procurement/requests/", func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasSuffix(r.URL.Path, "/generate-purchase-order") {
				http.NotFound(w, r)
				return
			}
			documentID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/procurement/requests/"), "/generate-purchase-order")
			source, err := docs.Get(documentID)
			if err != nil {
				respondError(w, err)
				return
			}
			p, ok := requireAuthorization(w, r, ident, "document.create", source.Header.LocationID, "")
			if !ok {
				return
			}
			record, err := procurementSvc.GeneratePurchaseOrderFromRequest(documentID, principalEffectiveUserID(p))
			if err != nil {
				respondError(w, err)
				return
			}
			refreshDocumentSearch(searchSvc, record)
			rendered := docs.Render(record, document.ViewExpanded, modules.EnabledMap())
			respondJSON(w, http.StatusCreated, sanitizeDocumentRecord(fieldSecurity, ident, p, rendered, "api"))
		})
		mux.HandleFunc("POST /procurement/orders/", func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.HasSuffix(r.URL.Path, "/register-receipt"):
				documentID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/procurement/orders/"), "/register-receipt")
				source, err := docs.Get(documentID)
				if err != nil {
					respondError(w, err)
					return
				}
				p, ok := requireAuthorization(w, r, ident, "document.create", source.Header.LocationID, "")
				if !ok {
					return
				}
				record, err := procurementSvc.CreateGoodsReceiptFromOrder(documentID, principalEffectiveUserID(p))
				if err != nil {
					respondError(w, err)
					return
				}
				refreshDocumentSearch(searchSvc, record)
				rendered := docs.Render(record, document.ViewExpanded, modules.EnabledMap())
				respondJSON(w, http.StatusCreated, sanitizeDocumentRecord(fieldSecurity, ident, p, rendered, "api"))
			case strings.HasSuffix(r.URL.Path, "/register-vendor-bill"):
				documentID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/procurement/orders/"), "/register-vendor-bill")
				source, err := docs.Get(documentID)
				if err != nil {
					respondError(w, err)
					return
				}
				p, ok := requireAuthorization(w, r, ident, "document.create", source.Header.LocationID, "")
				if !ok {
					return
				}
				record, err := procurementSvc.CreateVendorBillFromOrder(documentID, principalEffectiveUserID(p))
				if err != nil {
					respondError(w, err)
					return
				}
				refreshDocumentSearch(searchSvc, record)
				rendered := docs.Render(record, document.ViewExpanded, modules.EnabledMap())
				respondJSON(w, http.StatusCreated, sanitizeDocumentRecord(fieldSecurity, ident, p, rendered, "api"))
			default:
				http.NotFound(w, r)
			}
		})
		mux.HandleFunc("POST /procurement/receipts/", func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.HasSuffix(r.URL.Path, "/register-vendor-bill"):
				documentID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/procurement/receipts/"), "/register-vendor-bill")
				source, err := docs.Get(documentID)
				if err != nil {
					respondError(w, err)
					return
				}
				p, ok := requireAuthorization(w, r, ident, "document.create", source.Header.LocationID, "")
				if !ok {
					return
				}
				record, err := procurementSvc.CreateVendorBillFromReceipt(documentID, principalEffectiveUserID(p))
				if err != nil {
					respondError(w, err)
					return
				}
				refreshDocumentSearch(searchSvc, record)
				rendered := docs.Render(record, document.ViewExpanded, modules.EnabledMap())
				respondJSON(w, http.StatusCreated, sanitizeDocumentRecord(fieldSecurity, ident, p, rendered, "api"))
			case strings.HasSuffix(r.URL.Path, "/register-supplier-return"):
				if supplierReturnsSvc == nil {
					http.NotFound(w, r)
					return
				}
				documentID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/procurement/receipts/"), "/register-supplier-return")
				source, err := docs.Get(documentID)
				if err != nil {
					respondError(w, err)
					return
				}
				p, ok := requireAuthorization(w, r, ident, "document.create", source.Header.LocationID, "")
				if !ok {
					return
				}
				record, err := supplierReturnsSvc.GenerateSupplierReturnFromReceipt(documentID, principalEffectiveUserID(p))
				if err != nil {
					respondError(w, err)
					return
				}
				refreshDocumentSearch(searchSvc, record)
				rendered := docs.Render(record, document.ViewExpanded, modules.EnabledMap())
				respondJSON(w, http.StatusCreated, sanitizeDocumentRecord(fieldSecurity, ident, p, rendered, "api"))
			default:
				http.NotFound(w, r)
			}
		})
		mux.HandleFunc("POST /procurement/bills/", func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.HasSuffix(r.URL.Path, "/register-payment"):
				documentID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/procurement/bills/"), "/register-payment")
				source, err := docs.Get(documentID)
				if err != nil {
					respondError(w, err)
					return
				}
				p, ok := requireAuthorization(w, r, ident, "document.create", source.Header.LocationID, "")
				if !ok {
					return
				}
				record, err := procurementSvc.CreatePaymentOutFromBill(documentID, principalEffectiveUserID(p))
				if err != nil {
					respondError(w, err)
					return
				}
				refreshDocumentSearch(searchSvc, record)
				rendered := docs.Render(record, document.ViewExpanded, modules.EnabledMap())
				respondJSON(w, http.StatusCreated, sanitizeDocumentRecord(fieldSecurity, ident, p, rendered, "api"))
			case strings.HasSuffix(r.URL.Path, "/issue-credit-note"):
				documentID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/procurement/bills/"), "/issue-credit-note")
				source, err := docs.Get(documentID)
				if err != nil {
					respondError(w, err)
					return
				}
				p, ok := requireAuthorization(w, r, ident, "document.create", source.Header.LocationID, "")
				if !ok {
					return
				}
				record, err := procurementSvc.CreateVendorCreditFromBill(documentID, principalEffectiveUserID(p))
				if err != nil {
					respondError(w, err)
					return
				}
				refreshDocumentSearch(searchSvc, record)
				rendered := docs.Render(record, document.ViewExpanded, modules.EnabledMap())
				respondJSON(w, http.StatusCreated, sanitizeDocumentRecord(fieldSecurity, ident, p, rendered, "api"))
			case strings.HasSuffix(r.URL.Path, "/register-supplier-return"):
				if supplierReturnsSvc == nil {
					http.NotFound(w, r)
					return
				}
				documentID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/procurement/bills/"), "/register-supplier-return")
				source, err := docs.Get(documentID)
				if err != nil {
					respondError(w, err)
					return
				}
				p, ok := requireAuthorization(w, r, ident, "document.create", source.Header.LocationID, "")
				if !ok {
					return
				}
				record, err := supplierReturnsSvc.GenerateSupplierReturnFromBill(documentID, principalEffectiveUserID(p))
				if err != nil {
					respondError(w, err)
					return
				}
				refreshDocumentSearch(searchSvc, record)
				rendered := docs.Render(record, document.ViewExpanded, modules.EnabledMap())
				respondJSON(w, http.StatusCreated, sanitizeDocumentRecord(fieldSecurity, ident, p, rendered, "api"))
			default:
				http.NotFound(w, r)
			}
		})
	}
	if supplierReturnsSvc != nil {
		mux.HandleFunc("POST /supplier-returns/returns/", func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasSuffix(r.URL.Path, "/issue-vendor-credit") {
				http.NotFound(w, r)
				return
			}
			documentID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/supplier-returns/returns/"), "/issue-vendor-credit")
			source, err := docs.Get(documentID)
			if err != nil {
				respondError(w, err)
				return
			}
			p, ok := requireAuthorization(w, r, ident, "document.create", source.Header.LocationID, "")
			if !ok {
				return
			}
			record, err := supplierReturnsSvc.CreateVendorCreditFromReturn(documentID, principalEffectiveUserID(p))
			if err != nil {
				respondError(w, err)
				return
			}
			refreshDocumentSearch(searchSvc, record)
			rendered := docs.Render(record, document.ViewExpanded, modules.EnabledMap())
			respondJSON(w, http.StatusCreated, sanitizeDocumentRecord(fieldSecurity, ident, p, rendered, "api"))
		})
	}
	if deliverySvc != nil {
		mux.HandleFunc("POST /delivery/fulfillments/", func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasSuffix(r.URL.Path, "/register-delivery") {
				http.NotFound(w, r)
				return
			}
			documentID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/delivery/fulfillments/"), "/register-delivery")
			source, err := docs.Get(documentID)
			if err != nil {
				respondError(w, err)
				return
			}
			p, ok := requireAuthorization(w, r, ident, "document.create", source.Header.LocationID, "")
			if !ok {
				return
			}
			record, err := deliverySvc.GenerateDeliveryFromFulfillment(documentID, principalEffectiveUserID(p))
			if err != nil {
				respondError(w, err)
				return
			}
			refreshDocumentSearch(searchSvc, record)
			rendered := docs.Render(record, document.ViewExpanded, modules.EnabledMap())
			respondJSON(w, http.StatusCreated, sanitizeDocumentRecord(fieldSecurity, ident, p, rendered, "api"))
		})
	}
	if returnsSvc != nil {
		mux.HandleFunc("POST /returns/fulfillments/", func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasSuffix(r.URL.Path, "/register-return") {
				http.NotFound(w, r)
				return
			}
			documentID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/returns/fulfillments/"), "/register-return")
			source, err := docs.Get(documentID)
			if err != nil {
				respondError(w, err)
				return
			}
			p, ok := requireAuthorization(w, r, ident, "document.create", source.Header.LocationID, "")
			if !ok {
				return
			}
			record, err := returnsSvc.GenerateReturnFromFulfillment(documentID, principalEffectiveUserID(p))
			if err != nil {
				respondError(w, err)
				return
			}
			refreshDocumentSearch(searchSvc, record)
			rendered := docs.Render(record, document.ViewExpanded, modules.EnabledMap())
			respondJSON(w, http.StatusCreated, sanitizeDocumentRecord(fieldSecurity, ident, p, rendered, "api"))
		})
		mux.HandleFunc("POST /returns/returns/", func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.HasSuffix(r.URL.Path, "/register-receipt"):
				documentID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/returns/returns/"), "/register-receipt")
				source, err := docs.Get(documentID)
				if err != nil {
					respondError(w, err)
					return
				}
				p, ok := requireAuthorization(w, r, ident, "document.create", source.Header.LocationID, "")
				if !ok {
					return
				}
				record, err := returnsSvc.CreateReturnReceiptFromReturn(documentID, principalEffectiveUserID(p))
				if err != nil {
					respondError(w, err)
					return
				}
				refreshDocumentSearch(searchSvc, record)
				rendered := docs.Render(record, document.ViewExpanded, modules.EnabledMap())
				respondJSON(w, http.StatusCreated, sanitizeDocumentRecord(fieldSecurity, ident, p, rendered, "api"))
			case strings.HasSuffix(r.URL.Path, "/issue-credit-note"):
				documentID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/returns/returns/"), "/issue-credit-note")
				source, err := docs.Get(documentID)
				if err != nil {
					respondError(w, err)
					return
				}
				p, ok := requireAuthorization(w, r, ident, "document.create", source.Header.LocationID, "")
				if !ok {
					return
				}
				record, err := returnsSvc.CreateCreditNoteFromReturn(documentID, principalEffectiveUserID(p))
				if err != nil {
					respondError(w, err)
					return
				}
				refreshDocumentSearch(searchSvc, record)
				rendered := docs.Render(record, document.ViewExpanded, modules.EnabledMap())
				respondJSON(w, http.StatusCreated, sanitizeDocumentRecord(fieldSecurity, ident, p, rendered, "api"))
			case strings.HasSuffix(r.URL.Path, "/register-refund"):
				documentID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/returns/returns/"), "/register-refund")
				source, err := docs.Get(documentID)
				if err != nil {
					respondError(w, err)
					return
				}
				p, ok := requireAuthorization(w, r, ident, "document.create", source.Header.LocationID, "")
				if !ok {
					return
				}
				record, err := returnsSvc.CreateRefundFromReturn(documentID, principalEffectiveUserID(p))
				if err != nil {
					respondError(w, err)
					return
				}
				refreshDocumentSearch(searchSvc, record)
				rendered := docs.Render(record, document.ViewExpanded, modules.EnabledMap())
				respondJSON(w, http.StatusCreated, sanitizeDocumentRecord(fieldSecurity, ident, p, rendered, "api"))
			case strings.HasSuffix(r.URL.Path, "/create-replacement-order"):
				documentID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/returns/returns/"), "/create-replacement-order")
				source, err := docs.Get(documentID)
				if err != nil {
					respondError(w, err)
					return
				}
				p, ok := requireAuthorization(w, r, ident, "document.create", source.Header.LocationID, "")
				if !ok {
					return
				}
				record, err := returnsSvc.CreateReplacementOrderFromReturn(documentID, principalEffectiveUserID(p))
				if err != nil {
					respondError(w, err)
					return
				}
				refreshDocumentSearch(searchSvc, record)
				rendered := docs.Render(record, document.ViewExpanded, modules.EnabledMap())
				respondJSON(w, http.StatusCreated, sanitizeDocumentRecord(fieldSecurity, ident, p, rendered, "api"))
			default:
				http.NotFound(w, r)
			}
		})
	}
	if productionSvc != nil {
		mux.HandleFunc("POST /production/orders/", func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.HasSuffix(r.URL.Path, "/register-issue"):
				documentID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/production/orders/"), "/register-issue")
				source, err := docs.Get(documentID)
				if err != nil {
					respondError(w, err)
					return
				}
				p, ok := requireAuthorization(w, r, ident, "document.create", source.Header.LocationID, "")
				if !ok {
					return
				}
				record, err := productionSvc.CreateProductionIssueFromOrder(documentID, principalEffectiveUserID(p))
				if err != nil {
					respondError(w, err)
					return
				}
				refreshDocumentSearch(searchSvc, record)
				rendered := docs.Render(record, document.ViewExpanded, modules.EnabledMap())
				respondJSON(w, http.StatusCreated, sanitizeDocumentRecord(fieldSecurity, ident, p, rendered, "api"))
			case strings.HasSuffix(r.URL.Path, "/register-output"):
				documentID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/production/orders/"), "/register-output")
				source, err := docs.Get(documentID)
				if err != nil {
					respondError(w, err)
					return
				}
				p, ok := requireAuthorization(w, r, ident, "document.create", source.Header.LocationID, "")
				if !ok {
					return
				}
				record, err := productionSvc.CreateProductionOutputFromOrder(documentID, principalEffectiveUserID(p))
				if err != nil {
					respondError(w, err)
					return
				}
				refreshDocumentSearch(searchSvc, record)
				rendered := docs.Render(record, document.ViewExpanded, modules.EnabledMap())
				respondJSON(w, http.StatusCreated, sanitizeDocumentRecord(fieldSecurity, ident, p, rendered, "api"))
			default:
				http.NotFound(w, r)
			}
		})
	}

	mux.HandleFunc("GET /documents", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireAuthorization(w, r, ident, "document.list", effectiveLocationID(r), "")
		if !ok {
			return
		}
		locationID := effectiveLocationID(r)
		if locationID == "" && p.kind == userPrincipal {
			locationID = p.currentLocationID
		}
		items := docs.List()
		if locationID != "" {
			filtered := make([]document.Record, 0, len(items))
			for _, item := range items {
				if manualJournalReadBlocked(ident, p, item) {
					continue
				}
				if item.Header.LocationID == locationID && searchVisible(item.Header, p, policySvc) {
					rendered := docs.Render(item, document.ViewNormal, modules.EnabledMap())
					filtered = append(filtered, sanitizeDocumentRecord(fieldSecurity, ident, p, rendered, "api"))
				}
			}
			items = filtered
		} else {
			filtered := make([]document.Record, 0, len(items))
			for i := range items {
				if manualJournalReadBlocked(ident, p, items[i]) {
					continue
				}
				if !searchVisible(items[i].Header, p, policySvc) {
					continue
				}
				rendered := docs.Render(items[i], document.ViewNormal, modules.EnabledMap())
				filtered = append(filtered, sanitizeDocumentRecord(fieldSecurity, ident, p, rendered, "api"))
			}
			items = filtered
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": items})
	})

	mux.HandleFunc("POST /documents", func(w http.ResponseWriter, r *http.Request) {
		var req createDocumentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid document create payload"))
			return
		}
		if req.OrganizationID == "" {
			respondError(w, shared.Validation("organization_id is required"))
			return
		}
		if commercialSvc != nil && isCommercialManagedType(req.Type) {
			req.Payload = commercialSvc.NormalizePayload(req.Type, req.Payload)
		} else if procurementSvc != nil && isProcurementManagedType(req.Type) {
			req.Payload = procurementSvc.NormalizePayload(req.Type, req.Payload)
		} else if inventorySvc != nil && isInventoryManagedType(req.Type) {
			req.Payload = inventorySvc.NormalizePayload(req.Type, req.Payload)
		} else if fulfillmentSvc != nil && isFulfillmentManagedType(req.Type) {
			req.Payload = fulfillmentSvc.NormalizePayload(req.Type, req.Payload)
		} else if deliverySvc != nil && isDeliveryManagedType(req.Type) {
			req.Payload = deliverySvc.NormalizePayload(req.Type, req.Payload)
		} else if returnsSvc != nil && isReturnsManagedType(req.Type) {
			req.Payload = returnsSvc.NormalizePayload(req.Type, req.Payload)
		} else if supplierReturnsSvc != nil && isSupplierReturnsManagedType(req.Type) {
			req.Payload = supplierReturnsSvc.NormalizePayload(req.Type, req.Payload)
		} else if productionSvc != nil && isProductionManagedType(req.Type) {
			req.Payload = productionSvc.NormalizePayload(req.Type, req.Payload)
		} else if recallSvc != nil && isRecallManagedType(req.Type) {
			req.Payload = recallSvc.NormalizePayload(req.Type, req.Payload)
		} else if payrollSvc != nil && isPayrollManagedType(req.Type) {
			req.Payload = payrollSvc.NormalizePayload(req.Type, req.Payload)
		}
		p, ok := requireAuthorization(w, r, ident, "document.create", req.LocationID, "")
		if !ok {
			return
		}
		if isManualJournalCreate(req.Type, req.Payload) && !principalAllowsPermission(ident, p, "finance.journal.create", locationIDForDocumentCreate(req.LocationID, p)) {
			respondError(w, shared.Forbidden("manual journal creation is not allowed"))
			return
		}
		if !principalAllowsDocumentType(p, req.Type) {
			respondError(w, shared.Forbidden("delegation grant does not allow this document type"))
			return
		}
		locationID := req.LocationID
		if locationID == "" && p.kind == userPrincipal {
			locationID = p.currentLocationID
		}
		candidate := document.Record{
			Header: document.Header{
				Type:           req.Type,
				Status:         "draft",
				OrganizationID: req.OrganizationID,
				LocationID:     locationID,
			},
			Body: document.Body{Payload: req.Payload},
		}
		if err := validateDocumentWrite(fieldSecurity, ident, p, candidate, req.Payload, "", "api"); err != nil {
			respondError(w, err)
			return
		}
		if err := validateDocumentPayloadForType(modules, req.Type, req.Payload); err != nil {
			respondError(w, err)
			return
		}
		record, err := docs.Create(req.Type, req.OrganizationID, locationID, principalEffectiveUserID(p), req.Payload)
		if err != nil {
			incActionMetric(obs, "create", "error")
			respondError(w, err)
			return
		}
		refreshDocumentSearch(searchSvc, record)
		recordAudit(auditSvc, principalAuditEvent(p, audit.Event{
			ID:             "audit:document:create:" + record.Header.ID + ":" + record.Header.ETag,
			Action:         "document.create",
			TargetType:     "document",
			TargetID:       record.Header.ID,
			OccurredAt:     time.Now().UTC(),
			FromState:      "",
			ToState:        record.Header.Status,
			OrganizationID: record.Header.OrganizationID,
			LocationID:     record.Header.LocationID,
			ChangeSummary:  map[string]any{"fields": []string{"payload", "status"}},
			Metadata: map[string]any{
				"document_type":     record.Header.Type,
				"version":           record.Header.Version,
				"effective_user_id": principalEffectiveUserID(p),
			},
		}))
		incActionMetric(obs, "create", "success")
		respondJSON(w, http.StatusCreated, sanitizeDocumentRecord(fieldSecurity, ident, p, record, "api"))
	})

	mux.HandleFunc("GET /documents/", func(w http.ResponseWriter, r *http.Request) {
		if documentID, ok := documentLinkCollectionPath(r.URL.Path); ok {
			record, err := docs.Get(documentID)
			if err != nil {
				respondError(w, err)
				return
			}
			p, ok := requireAuthorization(w, r, ident, "document.read", record.Header.LocationID, "")
			if !ok {
				return
			}
			if !principalAllowsDocumentType(p, record.Header.Type) {
				respondError(w, shared.Forbidden("delegation grant does not allow this document type"))
				return
			}
			if manualJournalReadBlocked(ident, p, record) {
				respondError(w, shared.Forbidden("manual journal access is not allowed"))
				return
			}
			respondJSON(w, http.StatusOK, map[string]any{"items": sanitizeDocumentRecord(fieldSecurity, ident, p, record, "api").Links})
			return
		}
		if documentID, ok := documentAttachmentCollectionPath(r.URL.Path); ok {
			record, err := docs.Get(documentID)
			if err != nil {
				respondError(w, err)
				return
			}
			p, ok := requireAuthorization(w, r, ident, "document.read", record.Header.LocationID, "")
			if !ok {
				return
			}
			if !principalAllowsDocumentType(p, record.Header.Type) {
				respondError(w, shared.Forbidden("delegation grant does not allow this document type"))
				return
			}
			if manualJournalReadBlocked(ident, p, record) {
				respondError(w, shared.Forbidden("manual journal access is not allowed"))
				return
			}
			respondJSON(w, http.StatusOK, map[string]any{"items": sanitizeDocumentRecord(fieldSecurity, ident, p, record, "api").Attachments})
			return
		}
		documentID, ok := documentIDFromPath(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		record, err := docs.Get(documentID)
		if err != nil {
			respondError(w, err)
			return
		}
		p, ok := requireAuthorization(w, r, ident, "document.read", record.Header.LocationID, "")
		if !ok {
			return
		}
		if !principalAllowsDocumentType(p, record.Header.Type) {
			respondError(w, shared.Forbidden("delegation grant does not allow this document type"))
			return
		}
		if manualJournalReadBlocked(ident, p, record) {
			respondError(w, shared.Forbidden("manual journal access is not allowed"))
			return
		}
		viewMode := documentViewMode(r)
		if viewMode == document.ViewRaw {
			if _, ok := requireAuthorization(w, r, ident, "configuration.read", record.Header.LocationID, "configuration.read"); !ok {
				return
			}
		}
		rendered := docs.Render(record, viewMode, modules.EnabledMap())
		if viewMode == document.ViewExpanded || viewMode == document.ViewRaw {
			rendered = filterDocumentExtensionsForPrincipal(rendered, modules, ident, policySvc, p)
		}
		rendered = sanitizeDocumentRecord(fieldSecurity, ident, p, rendered, "api")
		respondJSON(w, http.StatusOK, rendered)
	})

	mux.HandleFunc("PUT /documents/", func(w http.ResponseWriter, r *http.Request) {
		if documentID, moduleKey, ok := documentExtensionPath(r.URL.Path); ok {
			current, err := docs.Get(documentID)
			if err != nil {
				respondError(w, err)
				return
			}
			p, ok := requireAuthorization(w, r, ident, "document.update_draft", current.Header.LocationID, "")
			if !ok {
				return
			}
			if !principalAllowsDocumentType(p, current.Header.Type) {
				respondError(w, shared.Forbidden("delegation grant does not allow this document type"))
				return
			}
			if isManualJournalRecord(current) && !principalAllowsPermission(ident, p, "finance.journal.create", current.Header.LocationID) {
				respondError(w, shared.Forbidden("manual journal draft updates are not allowed"))
				return
			}
			if !modules.IsEnabled(moduleKey) {
				respondError(w, shared.Conflict("module is disabled"))
				return
			}
			if !extensionWriteAllowed(current, moduleKey, modules, ident, policySvc, p) {
				respondError(w, shared.Forbidden("document extension write is not allowed"))
				return
			}
			var req updateDocumentExtensionRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				respondError(w, shared.Validation("invalid document extension payload"))
				return
			}
			if err := validateDocumentWrite(fieldSecurity, ident, p, current, req.Payload, "extensions."+moduleKey, "api"); err != nil {
				respondError(w, err)
				return
			}
			record, err := docActions.UpdateExtension(documentID, moduleKey, requestActingContext(r, p), req.Payload, req.ExpectedVersion, req.ExpectedETag)
			if err != nil {
				incActionMetric(obs, "update_extension", "error")
				respondError(w, err)
				return
			}
			incActionMetric(obs, "update_extension", "success")
			rendered := filterDocumentExtensionsForPrincipal(docs.Render(record, document.ViewExpanded, modules.EnabledMap()), modules, ident, policySvc, p)
			respondJSON(w, http.StatusOK, sanitizeDocumentRecord(fieldSecurity, ident, p, rendered, "api"))
			return
		}
		documentID, ok := documentIDFromPath(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		current, err := docs.Get(documentID)
		if err != nil {
			respondError(w, err)
			return
		}
		p, ok := requireAuthorization(w, r, ident, "document.update_draft", current.Header.LocationID, "")
		if !ok {
			return
		}
		if !principalAllowsDocumentType(p, current.Header.Type) {
			respondError(w, shared.Forbidden("delegation grant does not allow this document type"))
			return
		}
		if isManualJournalRecord(current) && !principalAllowsPermission(ident, p, "finance.journal.create", current.Header.LocationID) {
			respondError(w, shared.Forbidden("manual journal draft updates are not allowed"))
			return
		}
		if commercialDocumentUpdateLocked(current.Header.Type, current.Header.Status) || procurementDocumentUpdateLocked(current.Header.Type, current.Header.Status) || inventoryDocumentUpdateLocked(current.Header.Type, current.Header.Status) || deliveryDocumentUpdateLocked(current.Header.Type, current.Header.Status) || returnsDocumentUpdateLocked(current.Header.Type, current.Header.Status) || supplierReturnsDocumentUpdateLocked(current.Header.Type, current.Header.Status) || productionDocumentUpdateLocked(current.Header.Type, current.Header.Status) || recallDocumentUpdateLocked(current.Header.Type, current.Header.Status) || payrollDocumentUpdateLocked(current.Header.Type, current.Header.Status) {
			respondError(w, shared.Conflict("business documents can only be edited while draft or rejected"))
			return
		}
		var req updateDocumentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid document update payload"))
			return
		}
		if commercialSvc != nil && isCommercialManagedType(current.Header.Type) {
			req.Payload = commercialSvc.NormalizePayload(current.Header.Type, req.Payload)
		} else if procurementSvc != nil && isProcurementManagedType(current.Header.Type) {
			req.Payload = procurementSvc.NormalizePayload(current.Header.Type, req.Payload)
		} else if inventorySvc != nil && isInventoryManagedType(current.Header.Type) {
			req.Payload = inventorySvc.NormalizePayload(current.Header.Type, req.Payload)
		} else if fulfillmentSvc != nil && isFulfillmentManagedType(current.Header.Type) {
			req.Payload = fulfillmentSvc.NormalizePayload(current.Header.Type, req.Payload)
		} else if deliverySvc != nil && isDeliveryManagedType(current.Header.Type) {
			req.Payload = deliverySvc.NormalizePayload(current.Header.Type, req.Payload)
		} else if returnsSvc != nil && isReturnsManagedType(current.Header.Type) {
			req.Payload = returnsSvc.NormalizePayload(current.Header.Type, req.Payload)
		} else if supplierReturnsSvc != nil && isSupplierReturnsManagedType(current.Header.Type) {
			req.Payload = supplierReturnsSvc.NormalizePayload(current.Header.Type, req.Payload)
		} else if productionSvc != nil && isProductionManagedType(current.Header.Type) {
			req.Payload = productionSvc.NormalizePayload(current.Header.Type, req.Payload)
		} else if recallSvc != nil && isRecallManagedType(current.Header.Type) {
			req.Payload = recallSvc.NormalizePayload(current.Header.Type, req.Payload)
		} else if payrollSvc != nil && isPayrollManagedType(current.Header.Type) {
			req.Payload = payrollSvc.NormalizePayload(current.Header.Type, req.Payload)
		}
		if err := validateDocumentWrite(fieldSecurity, ident, p, current, req.Payload, "", "api"); err != nil {
			respondError(w, err)
			return
		}
		if err := validateDocumentPayloadForType(modules, current.Header.Type, req.Payload); err != nil {
			respondError(w, err)
			return
		}
		record, err := docActions.UpdateDraft(documentID, requestActingContext(r, p), req.Payload, req.ExpectedVersion, req.ExpectedETag)
		if err != nil {
			incActionMetric(obs, "update", "error")
			respondError(w, err)
			return
		}
		incActionMetric(obs, "update", "success")
		respondJSON(w, http.StatusOK, sanitizeDocumentRecord(fieldSecurity, ident, p, docs.Render(record, document.ViewExpanded, modules.EnabledMap()), "api"))
	})

	mux.HandleFunc("POST /documents/", func(w http.ResponseWriter, r *http.Request) {
		if documentID, ok := documentLinkCollectionPath(r.URL.Path); ok {
			record, err := docs.Get(documentID)
			if err != nil {
				respondError(w, err)
				return
			}
			if _, ok := requireAuthorization(w, r, ident, "document.update_draft", record.Header.LocationID, ""); !ok {
				return
			}
			var req createDocumentLinkRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				respondError(w, shared.Validation("invalid document link payload"))
				return
			}
			link, err := docs.AddLink(documentID, strings.TrimSpace(req.LinkedDocumentID), strings.TrimSpace(req.LinkType), req.Metadata)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusCreated, link)
			return
		}
		if documentID, ok := documentAttachmentCollectionPath(r.URL.Path); ok {
			record, err := docs.Get(documentID)
			if err != nil {
				respondError(w, err)
				return
			}
			if _, ok := requireAuthorization(w, r, ident, "document.update_draft", record.Header.LocationID, ""); !ok {
				return
			}
			var req createDocumentAttachmentRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				respondError(w, shared.Validation("invalid document attachment payload"))
				return
			}
			attachment, err := docs.AddAttachment(documentID, strings.TrimSpace(req.AttachmentType), strings.TrimSpace(req.FileName), strings.TrimSpace(req.ContentType), strings.TrimSpace(req.StorageKey), req.SizeBytes)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusCreated, attachment)
			return
		}
		documentID, ok := documentActionPath(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		current, err := docs.Get(documentID)
		if err != nil {
			respondError(w, err)
			return
		}
		var req actionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, http.ErrBodyNotAllowed) {
			respondError(w, shared.Validation("invalid action payload"))
			return
		}
		if req.Action == "" {
			respondError(w, shared.Validation("action is required"))
			return
		}

		permissionByAction := map[string]string{
			"submit":         "document.submit",
			"approve":        "document.approve",
			"mark_delivered": "document.approve",
			"reject":         "document.reject",
			"reopen":         "document.reopen",
			"cancel":         "document.cancel",
		}
		permissionKey, exists := permissionByAction[req.Action]
		if !exists {
			respondError(w, shared.Validation("unsupported document action"))
			return
		}
		authOpts := authorizationOptions{UserPermission: permissionKey, LocationID: current.Header.LocationID}
		if cfg != nil && (req.Action == "approve" || req.Action == "reject") {
			if candidatePrincipal, hasPrincipal := currentPrincipal(r); hasPrincipal && candidatePrincipal.kind == userPrincipal {
				_, _, _, actorApprovalRequired := resolveTOTPState(cfg.AuthPolicy(), ident, principalEffectiveUserID(candidatePrincipal))
				authOpts.RequireStepUp = actorApprovalRequired
			}
		}
		p, ok := requireAuthorizationWithOptions(w, r, ident, authOpts)
		if !ok {
			return
		}
		if !principalAllowsDocumentType(p, current.Header.Type) {
			respondError(w, shared.Forbidden("delegation grant does not allow this document type"))
			return
		}
		if isManualJournalRecord(current) {
			extraPermission := manualJournalPermissionForAction(req.Action)
			if extraPermission != "" && !principalAllowsPermission(ident, p, extraPermission, current.Header.LocationID) {
				respondError(w, shared.Forbidden("manual journal action is not allowed"))
				return
			}
		}
		if commercialSvc != nil && isCommercialManagedType(current.Header.Type) {
			validationErr := commercialSvc.ValidateAction(current, req.Action, principalEffectiveUserID(p))
			if validationErr != nil {
				incActionMetric(obs, req.Action, "error")
				respondError(w, validationErr)
				return
			}
		}
		if procurementSvc != nil && isProcurementManagedType(current.Header.Type) {
			var validationErr error
			switch req.Action {
			case "approve":
				validationErr = procurementSvc.ValidateApprove(current)
			case "cancel":
				validationErr = procurementSvc.ValidateCancel(current)
			}
			if validationErr != nil {
				incActionMetric(obs, req.Action, "error")
				respondError(w, validationErr)
				return
			}
		}
		if inventorySvc != nil && isInventoryManagedType(current.Header.Type) {
			var validationErr error
			switch req.Action {
			case "approve":
				validationErr = inventorySvc.ValidateApprove(current)
			case "cancel":
				validationErr = inventorySvc.ValidateCancel(current)
			}
			if validationErr != nil {
				incActionMetric(obs, req.Action, "error")
				respondError(w, validationErr)
				return
			}
		}
		if fulfillmentSvc != nil && isFulfillmentManagedType(current.Header.Type) {
			var validationErr error
			switch req.Action {
			case "approve":
				validationErr = fulfillmentSvc.ValidateApprove(current)
			case "cancel":
				validationErr = fulfillmentSvc.ValidateCancel(current)
			}
			if validationErr != nil {
				incActionMetric(obs, req.Action, "error")
				respondError(w, validationErr)
				return
			}
		}
		if deliverySvc != nil && isDeliveryManagedType(current.Header.Type) {
			var validationErr error
			switch req.Action {
			case "approve":
				validationErr = deliverySvc.ValidateApprove(current)
			case "cancel":
				validationErr = deliverySvc.ValidateCancel(current)
			case "mark_delivered":
				validationErr = deliverySvc.ValidateMarkDelivered(current)
			}
			if validationErr != nil {
				incActionMetric(obs, req.Action, "error")
				respondError(w, validationErr)
				return
			}
		}
		if returnsSvc != nil && isReturnsManagedType(current.Header.Type) {
			var validationErr error
			switch req.Action {
			case "approve":
				validationErr = returnsSvc.ValidateApprove(current)
			case "cancel":
				validationErr = returnsSvc.ValidateCancel(current)
			}
			if validationErr != nil {
				incActionMetric(obs, req.Action, "error")
				respondError(w, validationErr)
				return
			}
		}
		if supplierReturnsSvc != nil && isSupplierReturnsManagedType(current.Header.Type) {
			var validationErr error
			switch req.Action {
			case "approve":
				validationErr = supplierReturnsSvc.ValidateApprove(current)
			case "cancel":
				validationErr = supplierReturnsSvc.ValidateCancel(current)
			}
			if validationErr != nil {
				incActionMetric(obs, req.Action, "error")
				respondError(w, validationErr)
				return
			}
		}
		if productionSvc != nil && isProductionManagedType(current.Header.Type) {
			var validationErr error
			switch req.Action {
			case "approve":
				validationErr = productionSvc.ValidateApprove(current)
			case "cancel":
				validationErr = productionSvc.ValidateCancel(current)
			}
			if validationErr != nil {
				incActionMetric(obs, req.Action, "error")
				respondError(w, validationErr)
				return
			}
		}
		if recallSvc != nil && isRecallManagedType(current.Header.Type) {
			var validationErr error
			switch req.Action {
			case "approve":
				validationErr = recallSvc.ValidateApprove(current)
			case "cancel":
				validationErr = recallSvc.ValidateCancel(current)
			}
			if validationErr != nil {
				incActionMetric(obs, req.Action, "error")
				respondError(w, validationErr)
				return
			}
		}

		var record document.Record
		switch req.Action {
		case "submit":
			record, err = docActions.Submit(documentID, requestActingContext(r, p), req.ExpectedVersion, req.ExpectedETag)
		case "approve":
			record, err = docActions.Approve(documentID, requestActingContext(r, p), req.ExpectedVersion, req.ExpectedETag)
		case "reject":
			record, err = docActions.Reject(documentID, requestActingContext(r, p), req.ExpectedVersion, req.ExpectedETag)
		case "reopen":
			record, err = docActions.Reopen(documentID, requestActingContext(r, p), req.ExpectedVersion, req.ExpectedETag)
		case "cancel":
			record, err = docActions.Cancel(documentID, requestActingContext(r, p), req.ExpectedVersion, req.ExpectedETag)
		default:
			record, err = docActions.Transition(documentID, requestActingContext(r, p), req.ExpectedVersion, req.ExpectedETag, req.Action, "document."+req.Action)
		}
		if err != nil {
			incActionMetric(obs, req.Action, "error")
			respondError(w, err)
			return
		}
		if commercialSvc != nil && isCommercialManagedType(record.Header.Type) && (req.Action == "submit" || req.Action == "approve" || req.Action == "reject" || req.Action == "cancel") {
			postCommitWarning := ""
			sideEffectErr := commercialSvc.HandleAction(record, req.Action, principalEffectiveUserID(p), req.Note)
			if sideEffectErr != nil {
				postCommitWarning = "commercial post-commit synchronization failed"
				log.Printf("documents: commercial post-commit sync failed for action=%s document_id=%s: %v", req.Action, record.Header.ID, sideEffectErr)
			}
			record, err = docs.Get(record.Header.ID)
			if err != nil {
				respondError(w, err)
				return
			}
			if postCommitWarning != "" {
				w.Header().Set("X-Orbyte-Warning", postCommitWarning)
			}
		}
		if procurementSvc != nil && isProcurementManagedType(record.Header.Type) && (req.Action == "approve" || req.Action == "cancel") {
			postCommitWarning := ""
			var sideEffectErr error
			switch req.Action {
			case "approve":
				sideEffectErr = procurementSvc.HandleApprovedDocument(record, principalEffectiveUserID(p))
			case "cancel":
				sideEffectErr = procurementSvc.HandleCanceledDocument(record, principalEffectiveUserID(p))
			}
			if sideEffectErr != nil {
				postCommitWarning = "procurement post-commit synchronization failed"
				log.Printf("documents: procurement post-commit sync failed for action=%s document_id=%s: %v", req.Action, record.Header.ID, sideEffectErr)
			}
			record, err = docs.Get(record.Header.ID)
			if err != nil {
				respondError(w, err)
				return
			}
			if postCommitWarning != "" {
				w.Header().Set("X-Orbyte-Warning", postCommitWarning)
			}
		}
		if inventorySvc != nil && isInventoryManagedType(record.Header.Type) && (req.Action == "approve" || req.Action == "cancel") {
			postCommitWarning := ""
			var sideEffectErr error
			switch req.Action {
			case "approve":
				sideEffectErr = inventorySvc.HandleApprovedDocument(record, principalEffectiveUserID(p))
			case "cancel":
				sideEffectErr = inventorySvc.HandleCanceledDocument(record, principalEffectiveUserID(p))
			}
			if sideEffectErr != nil {
				postCommitWarning = "inventory post-commit synchronization failed"
				log.Printf("documents: inventory post-commit sync failed for action=%s document_id=%s: %v", req.Action, record.Header.ID, sideEffectErr)
			}
			record, err = docs.Get(record.Header.ID)
			if err != nil {
				respondError(w, err)
				return
			}
			if postCommitWarning != "" {
				w.Header().Set("X-Orbyte-Warning", postCommitWarning)
			}
		}
		if fulfillmentSvc != nil && isFulfillmentManagedType(record.Header.Type) && (req.Action == "approve" || req.Action == "cancel") {
			postCommitWarning := ""
			var sideEffectErr error
			switch req.Action {
			case "approve":
				sideEffectErr = fulfillmentSvc.HandleApprovedDocument(record, principalEffectiveUserID(p))
			case "cancel":
				sideEffectErr = fulfillmentSvc.HandleCanceledDocument(record, principalEffectiveUserID(p))
			}
			if sideEffectErr != nil {
				postCommitWarning = "fulfillment post-commit synchronization failed"
				log.Printf("documents: fulfillment post-commit sync failed for action=%s document_id=%s: %v", req.Action, record.Header.ID, sideEffectErr)
			}
			record, err = docs.Get(record.Header.ID)
			if err != nil {
				respondError(w, err)
				return
			}
			if postCommitWarning != "" {
				w.Header().Set("X-Orbyte-Warning", postCommitWarning)
			}
		}
		if deliverySvc != nil && isDeliveryManagedType(record.Header.Type) && (req.Action == "approve" || req.Action == "cancel" || req.Action == "mark_delivered") {
			postCommitWarning := ""
			var sideEffectErr error
			switch req.Action {
			case "approve":
				sideEffectErr = deliverySvc.HandleApprovedDocument(record, principalEffectiveUserID(p))
			case "cancel":
				sideEffectErr = deliverySvc.HandleCanceledDocument(record, principalEffectiveUserID(p))
			case "mark_delivered":
				sideEffectErr = deliverySvc.HandleMarkedDelivered(record, principalEffectiveUserID(p))
			}
			if sideEffectErr != nil {
				postCommitWarning = "delivery post-commit synchronization failed"
				log.Printf("documents: delivery post-commit sync failed for action=%s document_id=%s: %v", req.Action, record.Header.ID, sideEffectErr)
			}
			record, err = docs.Get(record.Header.ID)
			if err != nil {
				respondError(w, err)
				return
			}
			if postCommitWarning != "" {
				w.Header().Set("X-Orbyte-Warning", postCommitWarning)
			}
		}
		if returnsSvc != nil && isReturnsManagedType(record.Header.Type) && (req.Action == "approve" || req.Action == "cancel") {
			postCommitWarning := ""
			var sideEffectErr error
			switch req.Action {
			case "approve":
				sideEffectErr = returnsSvc.HandleApprovedDocument(record, principalEffectiveUserID(p))
			case "cancel":
				sideEffectErr = returnsSvc.HandleCanceledDocument(record, principalEffectiveUserID(p))
			}
			if sideEffectErr != nil {
				postCommitWarning = "returns post-commit synchronization failed"
				log.Printf("documents: returns post-commit sync failed for action=%s document_id=%s: %v", req.Action, record.Header.ID, sideEffectErr)
			}
			record, err = docs.Get(record.Header.ID)
			if err != nil {
				respondError(w, err)
				return
			}
			if postCommitWarning != "" {
				w.Header().Set("X-Orbyte-Warning", postCommitWarning)
			}
		}
		if supplierReturnsSvc != nil && isSupplierReturnsManagedType(record.Header.Type) && (req.Action == "approve" || req.Action == "cancel") {
			postCommitWarning := ""
			var sideEffectErr error
			switch req.Action {
			case "approve":
				sideEffectErr = supplierReturnsSvc.HandleApprovedDocument(record, principalEffectiveUserID(p))
			case "cancel":
				sideEffectErr = supplierReturnsSvc.HandleCanceledDocument(record, principalEffectiveUserID(p))
			}
			if sideEffectErr != nil {
				postCommitWarning = "supplier return post-commit synchronization failed"
				log.Printf("documents: supplier return post-commit sync failed for action=%s document_id=%s: %v", req.Action, record.Header.ID, sideEffectErr)
			}
			record, err = docs.Get(record.Header.ID)
			if err != nil {
				respondError(w, err)
				return
			}
			if postCommitWarning != "" {
				w.Header().Set("X-Orbyte-Warning", postCommitWarning)
			}
		}
		if productionSvc != nil && isProductionManagedType(record.Header.Type) && (req.Action == "approve" || req.Action == "cancel") {
			postCommitWarning := ""
			var sideEffectErr error
			switch req.Action {
			case "approve":
				sideEffectErr = productionSvc.HandleApprovedDocument(record, principalEffectiveUserID(p))
			case "cancel":
				sideEffectErr = productionSvc.HandleCanceledDocument(record, principalEffectiveUserID(p))
			}
			if sideEffectErr != nil {
				postCommitWarning = "production post-commit synchronization failed"
				log.Printf("documents: production post-commit sync failed for action=%s document_id=%s: %v", req.Action, record.Header.ID, sideEffectErr)
			}
			record, err = docs.Get(record.Header.ID)
			if err != nil {
				respondError(w, err)
				return
			}
			if postCommitWarning != "" {
				w.Header().Set("X-Orbyte-Warning", postCommitWarning)
			}
		}
		if recallSvc != nil && isRecallManagedType(record.Header.Type) && (req.Action == "approve" || req.Action == "cancel") {
			postCommitWarning := ""
			var sideEffectErr error
			switch req.Action {
			case "approve":
				sideEffectErr = recallSvc.HandleApprovedDocument(record, principalEffectiveUserID(p))
			case "cancel":
				sideEffectErr = recallSvc.HandleCanceledDocument(record, principalEffectiveUserID(p))
			}
			if sideEffectErr != nil {
				postCommitWarning = "recall post-commit synchronization failed"
				log.Printf("documents: recall post-commit sync failed for action=%s document_id=%s: %v", req.Action, record.Header.ID, sideEffectErr)
			}
			record, err = docs.Get(record.Header.ID)
			if err != nil {
				respondError(w, err)
				return
			}
			if postCommitWarning != "" {
				w.Header().Set("X-Orbyte-Warning", postCommitWarning)
			}
		}
		incActionMetric(obs, req.Action, "success")
		rendered := docs.Render(record, document.ViewExpanded, modules.EnabledMap())
		rendered = filterDocumentExtensionsForPrincipal(rendered, modules, ident, policySvc, p)
		respondJSON(w, http.StatusOK, sanitizeDocumentRecord(fieldSecurity, ident, p, rendered, "api"))
	})

	mux.HandleFunc("DELETE /documents/", func(w http.ResponseWriter, r *http.Request) {
		if documentID, linkID, ok := documentLinkItemPath(r.URL.Path); ok {
			record, err := docs.Get(documentID)
			if err != nil {
				respondError(w, err)
				return
			}
			if _, ok := requireAuthorization(w, r, ident, "document.update_draft", record.Header.LocationID, ""); !ok {
				return
			}
			if err := docs.RemoveLink(documentID, linkID); err != nil {
				respondError(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if documentID, attachmentID, ok := documentAttachmentItemPath(r.URL.Path); ok {
			record, err := docs.Get(documentID)
			if err != nil {
				respondError(w, err)
				return
			}
			if _, ok := requireAuthorization(w, r, ident, "document.update_draft", record.Header.LocationID, ""); !ok {
				return
			}
			if err := docs.RemoveAttachment(documentID, attachmentID); err != nil {
				respondError(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
	})
}

func commercialDocumentUpdateLocked(documentType string, status string) bool {
	switch strings.ToLower(strings.TrimSpace(documentType)) {
	case "sales_order", "invoice", "credit_note", "payment_receipt", "payment_refund", "ledger_posting":
		normalizedStatus := strings.ToLower(strings.TrimSpace(status))
		return normalizedStatus != "" && normalizedStatus != "draft" && normalizedStatus != "rejected"
	default:
		return false
	}
}

func procurementDocumentUpdateLocked(documentType string, status string) bool {
	switch strings.ToLower(strings.TrimSpace(documentType)) {
	case "purchase_request", "purchase_order", "goods_receipt", "vendor_bill", "payment_out", "vendor_credit_note":
		normalizedStatus := strings.ToLower(strings.TrimSpace(status))
		return normalizedStatus != "" && normalizedStatus != "draft" && normalizedStatus != "rejected"
	default:
		return false
	}
}

func isCommercialManagedType(documentType string) bool {
	switch strings.ToLower(strings.TrimSpace(documentType)) {
	case "sales_order", "invoice", "credit_note", "payment_receipt", "payment_refund", "ledger_posting":
		return true
	default:
		return false
	}
}

func isProcurementManagedType(documentType string) bool {
	switch strings.ToLower(strings.TrimSpace(documentType)) {
	case "purchase_request", "purchase_order", "goods_receipt", "vendor_bill", "payment_out", "vendor_credit_note":
		return true
	default:
		return false
	}
}

func inventoryDocumentUpdateLocked(documentType string, status string) bool {
	switch strings.ToLower(strings.TrimSpace(documentType)) {
	case "stock_receipt", "stock_issue", "stock_adjustment", "stock_transfer", "stock_movement", "sales_fulfillment":
		normalizedStatus := strings.ToLower(strings.TrimSpace(status))
		return normalizedStatus != "" && normalizedStatus != "draft" && normalizedStatus != "rejected"
	default:
		return false
	}
}

func isInventoryManagedType(documentType string) bool {
	switch strings.ToLower(strings.TrimSpace(documentType)) {
	case "stock_receipt", "stock_issue", "stock_adjustment", "stock_transfer", "goods_receipt":
		return true
	default:
		return false
	}
}

func isFulfillmentManagedType(documentType string) bool {
	switch strings.ToLower(strings.TrimSpace(documentType)) {
	case "sales_fulfillment":
		return true
	default:
		return false
	}
}

func deliveryDocumentUpdateLocked(documentType string, status string) bool {
	switch strings.ToLower(strings.TrimSpace(documentType)) {
	case "delivery_order":
		normalizedStatus := strings.ToLower(strings.TrimSpace(status))
		return normalizedStatus != "" && normalizedStatus != "draft" && normalizedStatus != "rejected"
	default:
		return false
	}
}

func isDeliveryManagedType(documentType string) bool {
	switch strings.ToLower(strings.TrimSpace(documentType)) {
	case "delivery_order":
		return true
	default:
		return false
	}
}

func returnsDocumentUpdateLocked(documentType string, status string) bool {
	switch strings.ToLower(strings.TrimSpace(documentType)) {
	case "sales_return", "return_receipt":
		normalizedStatus := strings.ToLower(strings.TrimSpace(status))
		return normalizedStatus != "" && normalizedStatus != "draft" && normalizedStatus != "rejected"
	default:
		return false
	}
}

func supplierReturnsDocumentUpdateLocked(documentType string, status string) bool {
	switch strings.ToLower(strings.TrimSpace(documentType)) {
	case "supplier_return":
		normalizedStatus := strings.ToLower(strings.TrimSpace(status))
		return normalizedStatus != "" && normalizedStatus != "draft" && normalizedStatus != "rejected"
	default:
		return false
	}
}

func productionDocumentUpdateLocked(documentType string, status string) bool {
	switch strings.ToLower(strings.TrimSpace(documentType)) {
	case "production_order", "production_issue", "production_output":
		normalizedStatus := strings.ToLower(strings.TrimSpace(status))
		return normalizedStatus != "" && normalizedStatus != "draft" && normalizedStatus != "rejected"
	default:
		return false
	}
}

func isReturnsManagedType(documentType string) bool {
	switch strings.ToLower(strings.TrimSpace(documentType)) {
	case "sales_return", "return_receipt":
		return true
	default:
		return false
	}
}

func isSupplierReturnsManagedType(documentType string) bool {
	switch strings.ToLower(strings.TrimSpace(documentType)) {
	case "supplier_return":
		return true
	default:
		return false
	}
}

func isProductionManagedType(documentType string) bool {
	switch strings.ToLower(strings.TrimSpace(documentType)) {
	case "production_order", "production_issue", "production_output":
		return true
	default:
		return false
	}
}

func isRecallManagedType(documentType string) bool {
	switch strings.ToLower(strings.TrimSpace(documentType)) {
	case "recall_case", "recall_action":
		return true
	default:
		return false
	}
}

func isPayrollManagedType(documentType string) bool {
	switch strings.ToLower(strings.TrimSpace(documentType)) {
	case "payroll_run", "payroll_adjustment", "payroll_payment_batch", "payroll_payment":
		return true
	default:
		return false
	}
}

func recallDocumentUpdateLocked(documentType, status string) bool {
	normalizedType := strings.ToLower(strings.TrimSpace(documentType))
	normalizedStatus := strings.ToLower(strings.TrimSpace(status))
	if normalizedStatus == "" {
		return false
	}
	switch normalizedType {
	case "recall_case", "recall_action":
		return normalizedStatus != "draft" && normalizedStatus != "rejected"
	default:
		return false
	}
}

func payrollDocumentUpdateLocked(documentType, status string) bool {
	switch strings.ToLower(strings.TrimSpace(documentType)) {
	case "payroll_run", "payroll_adjustment", "payroll_payment_batch", "payroll_payment":
		normalizedStatus := strings.ToLower(strings.TrimSpace(status))
		return normalizedStatus != "" && normalizedStatus != "draft" && normalizedStatus != "rejected"
	default:
		return false
	}
}

func refreshDocumentSearch(searchSvc *search.Service, record document.Record) {
	if searchSvc == nil {
		return
	}
	searchSvc.RefreshDocument(record)
}

func isManualJournalRecord(record document.Record) bool {
	return strings.TrimSpace(record.Header.Type) == "ledger_posting" && strings.TrimSpace(stringValue(record.Body.Payload["journal_source_kind"])) == "manual"
}

func isManualJournalCreate(documentType string, payload map[string]any) bool {
	if strings.TrimSpace(documentType) != "ledger_posting" {
		return false
	}
	kind := strings.TrimSpace(stringValue(payload["journal_source_kind"]))
	return kind == "" || kind == "manual"
}

func manualJournalReadBlocked(ident *identity.Service, p principal, record document.Record) bool {
	return isManualJournalRecord(record) && !principalAllowsPermission(ident, p, "finance.journal.read", record.Header.LocationID)
}

func manualJournalPermissionForAction(action string) string {
	switch strings.TrimSpace(action) {
	case "submit":
		return "finance.journal.submit"
	case "approve":
		return "finance.journal.approve"
	case "reject":
		return "finance.journal.reject"
	case "cancel":
		return "finance.journal.cancel"
	case "reopen":
		return "finance.journal.create"
	default:
		return ""
	}
}

func locationIDForDocumentCreate(locationID string, p principal) string {
	locationID = strings.TrimSpace(locationID)
	if locationID != "" {
		return locationID
	}
	return p.currentLocationID
}

func stringValue(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func documentLinkCollectionPath(path string) (string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 3 || parts[0] != "documents" || parts[2] != "links" {
		return "", false
	}
	return strings.TrimSpace(parts[1]), parts[1] != ""
}

func documentAttachmentCollectionPath(path string) (string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 3 || parts[0] != "documents" || parts[2] != "attachments" {
		return "", false
	}
	return strings.TrimSpace(parts[1]), parts[1] != ""
}

func documentLinkItemPath(path string) (string, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 4 || parts[0] != "documents" || parts[2] != "links" {
		return "", "", false
	}
	return strings.TrimSpace(parts[1]), strings.TrimSpace(parts[3]), parts[1] != "" && parts[3] != ""
}

func documentAttachmentItemPath(path string) (string, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 4 || parts[0] != "documents" || parts[2] != "attachments" {
		return "", "", false
	}
	return strings.TrimSpace(parts[1]), strings.TrimSpace(parts[3]), parts[1] != "" && parts[3] != ""
}

func filterDocumentExtensionsForPrincipal(record document.Record, modules *module.Service, ident *identity.Service, policySvc *policy.Service, p principal) document.Record {
	rawExtensions, ok := record.Body.Payload["extensions"].(map[string]any)
	if !ok {
		return record
	}
	filtered := map[string]any{}
	defs := modules.List()
	defByModule := map[string]module.DocumentExtension{}
	for _, detail := range defs {
		for _, ext := range detail.Manifest.DocumentExtensions {
			defByModule[detail.Manifest.Key+":"+ext.DocumentType] = ext
		}
	}
	for moduleKey, payload := range rawExtensions {
		if !modules.IsEnabled(moduleKey) {
			continue
		}
		extDef, ok := defByModule[moduleKey+":"+record.Header.Type]
		if !ok {
			filtered[moduleKey] = payload
			continue
		}
		if extDef.ReadPermissionKey != "" && !principalAllowsAll(ident, p, []string{extDef.ReadPermissionKey}) {
			continue
		}
		if policySvc != nil {
			decision := policySvc.Evaluate(policy.Request{
				HookKey:        "documents.extension.view",
				ActorID:        principalActorID(p),
				OrganizationID: record.Header.OrganizationID,
				LocationID:     record.Header.LocationID,
				ScopeID:        record.Header.LocationID,
				Inputs: map[string]any{
					"module_key":      moduleKey,
					"document_type":   record.Header.Type,
					"document_id":     record.Header.ID,
					"document_status": record.Header.Status,
					"organization_id": record.Header.OrganizationID,
					"location_id":     record.Header.LocationID,
				},
			})
			if !decision.Allowed {
				continue
			}
		}
		filtered[moduleKey] = payload
	}
	if len(filtered) == 0 {
		delete(record.Body.Payload, "extensions")
		return record
	}
	record.Body.Payload["extensions"] = filtered
	return record
}

func extensionWriteAllowed(record document.Record, moduleKey string, modules *module.Service, ident *identity.Service, policySvc *policy.Service, p principal) bool {
	for _, detail := range modules.List() {
		if detail.Manifest.Key != moduleKey {
			continue
		}
		for _, ext := range detail.Manifest.DocumentExtensions {
			if ext.DocumentType != record.Header.Type {
				continue
			}
			if ext.WritePermissionKey != "" && !principalAllowsAll(ident, p, []string{ext.WritePermissionKey}) {
				return false
			}
			if policySvc == nil {
				return true
			}
			decision := policySvc.Evaluate(policy.Request{
				HookKey:        "documents.extension.write",
				ActorID:        principalActorID(p),
				OrganizationID: record.Header.OrganizationID,
				LocationID:     record.Header.LocationID,
				ScopeID:        record.Header.LocationID,
				Inputs: map[string]any{
					"module_key":      moduleKey,
					"document_type":   record.Header.Type,
					"document_id":     record.Header.ID,
					"document_status": record.Header.Status,
				},
			})
			return decision.Allowed
		}
	}
	return true
}

func searchVisible(header document.Header, p principal, policySvc *policy.Service) bool {
	if policySvc == nil {
		return true
	}
	decision := policySvc.Evaluate(policy.Request{
		HookKey:        "documents.search.visibility",
		ActorID:        principalActorID(p),
		OrganizationID: header.OrganizationID,
		LocationID:     header.LocationID,
		ScopeID:        header.LocationID,
		Inputs: map[string]any{
			"document_id":   header.ID,
			"document_type": header.Type,
			"status":        header.Status,
		},
	})
	return decision.Allowed
}

func incActionMetric(obs *observability.Service, action, outcome string) {
	if obs == nil {
		return
	}
	_ = obs.RecordMetric("document.actions.total", map[string]string{"action": action, "outcome": outcome}, 1)
	obs.Inc("document.actions.total")
	obs.Inc("document.actions." + action + "." + outcome + ".total")
}

func effectiveLocationID(r *http.Request) string {
	return strings.TrimSpace(r.URL.Query().Get("location_id"))
}

func documentIDFromPath(path string) (string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 || parts[0] != "documents" {
		return "", false
	}
	return parts[1], true
}

func documentActionPath(path string) (string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 3 || parts[0] != "documents" || parts[2] != "actions" {
		return "", false
	}
	return parts[1], true
}

func documentExtensionPath(path string) (string, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 4 || parts[0] != "documents" || parts[2] != "extensions" {
		return "", "", false
	}
	return parts[1], parts[3], true
}

func documentViewMode(r *http.Request) document.ViewMode {
	switch strings.TrimSpace(r.URL.Query().Get("view")) {
	case string(document.ViewExpanded):
		return document.ViewExpanded
	case string(document.ViewRaw):
		return document.ViewRaw
	default:
		return document.ViewNormal
	}
}
