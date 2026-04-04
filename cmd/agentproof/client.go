package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

type apiClient struct {
	baseURL    string
	httpClient *http.Client
	csrfToken  string
}

type documentResponse struct {
	Header struct {
		ID      string `json:"id"`
		Number  string `json:"number"`
		Status  string `json:"status"`
		Version int    `json:"version"`
		ETag    string `json:"etag"`
	} `json:"header"`
	Body struct {
		Payload map[string]any `json:"payload"`
	} `json:"body"`
}

type uiDocumentListItem struct {
	Header struct {
		ID     string `json:"id"`
		Type   string `json:"type"`
		Status string `json:"status"`
		Number string `json:"number"`
	} `json:"header"`
	Body struct {
		Payload map[string]any `json:"payload"`
	} `json:"body"`
}

func newAPIClient(baseURL string) (*apiClient, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &apiClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Jar:     jar,
			Timeout: 60 * time.Second,
		},
	}, nil
}

func (c *apiClient) login(ctx context.Context, username, password string) error {
	body := map[string]any{"username": username, "password": password}
	var resp map[string]any
	if err := c.doJSON(ctx, http.MethodPost, "/auth/login", body, &resp); err != nil {
		return err
	}
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return err
	}
	for _, cookie := range c.httpClient.Jar.Cookies(u) {
		if strings.EqualFold(cookie.Name, "orbyte_csrf") {
			c.csrfToken = cookie.Value
			break
		}
	}
	if strings.TrimSpace(c.csrfToken) == "" {
		return fmt.Errorf("login succeeded but csrf cookie is missing")
	}
	return nil
}

func (c *apiClient) createModel(ctx context.Context, key string, values map[string]any) (map[string]any, error) {
	var resp struct {
		Record struct {
			ID     string         `json:"id"`
			Values map[string]any `json:"values"`
		} `json:"record"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/models/"+key, map[string]any{"values": values}, &resp); err != nil {
		return nil, err
	}
	return map[string]any{"id": resp.Record.ID, "values": resp.Record.Values}, nil
}

func (c *apiClient) createDocument(ctx context.Context, req map[string]any) (documentFacts, error) {
	var record documentResponse
	if err := c.doJSON(ctx, http.MethodPost, "/documents", req, &record); err != nil {
		return documentFacts{}, err
	}
	return documentFacts{ID: record.Header.ID, Number: record.Header.Number, Status: record.Header.Status, Payload: record.Body.Payload}, nil
}

func (c *apiClient) postDocument(ctx context.Context, requestPath string, body any) (documentFacts, error) {
	var record documentResponse
	if err := c.doJSON(ctx, http.MethodPost, requestPath, body, &record); err != nil {
		return documentFacts{}, err
	}
	return documentFacts{ID: record.Header.ID, Number: record.Header.Number, Status: record.Header.Status, Payload: record.Body.Payload}, nil
}

func (c *apiClient) postDocumentList(ctx context.Context, requestPath string, body any) ([]documentFacts, error) {
	var resp struct {
		Items []documentResponse `json:"items"`
	}
	if err := c.doJSON(ctx, http.MethodPost, requestPath, body, &resp); err != nil {
		return nil, err
	}
	items := make([]documentFacts, 0, len(resp.Items))
	for _, item := range resp.Items {
		items = append(items, documentFacts{ID: item.Header.ID, Number: item.Header.Number, Status: item.Header.Status, Payload: item.Body.Payload})
	}
	return items, nil
}

func (c *apiClient) actionDocument(ctx context.Context, id string, version int, etag, action string) (documentFacts, int, string, error) {
	var record documentResponse
	body := map[string]any{"action": action, "expected_version": version, "expected_etag": etag}
	if err := c.doJSON(ctx, http.MethodPost, "/documents/"+id+"/actions", body, &record); err != nil {
		return documentFacts{}, 0, "", err
	}
	return documentFacts{ID: record.Header.ID, Number: record.Header.Number, Status: record.Header.Status, Payload: record.Body.Payload}, record.Header.Version, record.Header.ETag, nil
}

func (c *apiClient) getDocument(ctx context.Context, id string) (documentFacts, int, string, error) {
	var record documentResponse
	if err := c.doJSON(ctx, http.MethodGet, "/documents/"+id, nil, &record); err != nil {
		return documentFacts{}, 0, "", err
	}
	return documentFacts{ID: record.Header.ID, Number: record.Header.Number, Status: record.Header.Status, Payload: record.Body.Payload}, record.Header.Version, record.Header.ETag, nil
}

func (c *apiClient) putConfig(ctx context.Context, key string, value map[string]any) error {
	body := map[string]any{"scope": "deployment", "value": value}
	return c.doJSON(ctx, http.MethodPut, "/admin/api/config/entries/"+key+"/value", body, nil)
}

func (c *apiClient) createServicePrincipal(ctx context.Context, id, key string, allowedOperationTypes []string) (servicePrincipalOutput, error) {
	body := map[string]any{
		"id":                      id,
		"key":                     key,
		"status":                  "active",
		"allowed_operation_types": allowedOperationTypes,
	}
	var resp struct {
		ID  string `json:"id"`
		Key string `json:"key"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/service-principals", body, &resp); err != nil {
		return servicePrincipalOutput{}, err
	}
	return servicePrincipalOutput{ID: resp.ID, Key: resp.Key}, nil
}

func (c *apiClient) issueServicePrincipalToken(ctx context.Context, id string, ttlSeconds int) (string, error) {
	var resp struct {
		Token string `json:"token"`
	}
	err := c.doJSON(ctx, http.MethodPost, "/service-principals/"+id+"/tokens", map[string]any{"ttl_seconds": ttlSeconds}, &resp)
	return resp.Token, err
}

func (c *apiClient) listSessions(ctx context.Context) ([]sessionTranscript, error) {
	var resp struct {
		Items []sessionTranscript `json:"items"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/agent/api/sessions", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

func (c *apiClient) getSession(ctx context.Context, id string) (sessionTranscript, error) {
	var resp sessionTranscript
	if err := c.doJSON(ctx, http.MethodGet, "/agent/api/sessions/"+id, nil, &resp); err != nil {
		return sessionTranscript{}, err
	}
	return resp, nil
}

func (c *apiClient) listUIDocuments(ctx context.Context, documentType string, includePayload bool) ([]uiDocumentListItem, error) {
	values := url.Values{}
	if strings.TrimSpace(documentType) != "" {
		values.Set("type", strings.TrimSpace(documentType))
	}
	values.Set("sort", "updated_at")
	if includePayload {
		values.Set("include_payload", "1")
	}
	requestPath := "/ui/data/documents"
	if encoded := values.Encode(); encoded != "" {
		requestPath += "?" + encoded
	}
	var resp struct {
		Items []uiDocumentListItem `json:"items"`
	}
	if err := c.doJSON(ctx, http.MethodGet, requestPath, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

func (c *apiClient) openPOSShift(ctx context.Context, storeCode, registerCode string, openingCash float64, notes string) (map[string]any, error) {
	var resp struct {
		Record map[string]any `json:"record"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/ui/data/pos/shifts/open", map[string]any{
		"store_code":          storeCode,
		"register_code":       registerCode,
		"opening_cash_amount": openingCash,
		"notes":               notes,
	}, &resp); err != nil {
		return nil, err
	}
	return resp.Record, nil
}

func (c *apiClient) posCheckout(ctx context.Context, req map[string]any) (map[string]any, error) {
	var resp map[string]any
	if err := c.doJSON(ctx, http.MethodPost, "/ui/data/pos/checkout", req, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) doJSON(ctx context.Context, method, requestPath string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+requestPath, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if requiresCSRF(method) && strings.TrimSpace(c.csrfToken) != "" {
		req.Header.Set("X-CSRF-Token", c.csrfToken)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%s %s failed: status=%d body=%s", method, requestPath, resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if out == nil || len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode %s %s: %w", method, requestPath, err)
	}
	return nil
}

func requiresCSRF(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func writeJSONFile(pathname string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(pathname, data, 0o644)
}

func defaultOutputPath(baseDir, prefix, runID string) string {
	return path.Join(baseDir, fmt.Sprintf("%s-%s.json", prefix, runID))
}
