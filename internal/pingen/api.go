package pingen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

const UserAgent = "pingen-cli/0.1.0"

type APIError struct {
	Message   string
	Status    int
	RequestID string
}

func (err APIError) Error() string {
	if err.RequestID != "" {
		return fmt.Sprintf("%s (HTTP %d, request_id=%s)", err.Message, err.Status, err.RequestID)
	}
	return fmt.Sprintf("%s (HTTP %d)", err.Message, err.Status)
}

type Client struct {
	APIBase      string
	IdentityBase string
	AccessToken  string
	Timeout      time.Duration
}

func (c Client) GetToken(clientID, clientSecret, scope string) (map[string]any, http.Header, error) {
	endpoint := c.IdentityBase + "/auth/access-tokens"
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	if scope != "" {
		form.Set("scope", scope)
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
		"Accept":       "application/json",
	}
	status, respHeaders, body, err := c.doRequest("POST", endpoint, headers, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return nil, respHeaders, err
	}
	if status != http.StatusOK {
		return nil, respHeaders, APIError{Message: "token request failed", Status: status, RequestID: respHeaders.Get("X-Request-Id")}
	}
	payload, err := decodeJSON(body)
	return payload, respHeaders, err
}

func (c Client) ListOrganisations(params map[string]string) (map[string]any, http.Header, error) {
	endpoint := c.APIBase + "/organisations"
	endpoint = addQuery(endpoint, params)
	status, headers, body, err := c.doJSON("GET", endpoint, nil, "application/vnd.api+json")
	if err != nil {
		return nil, headers, err
	}
	if status != http.StatusOK {
		return nil, headers, APIError{Message: "list organisations failed", Status: status, RequestID: headers.Get("X-Request-Id")}
	}
	payload, err := decodeJSON(body)
	return payload, headers, err
}

func (c Client) ListLetters(orgID string, params map[string]string) (map[string]any, http.Header, error) {
	endpoint := c.APIBase + "/organisations/" + orgID + "/letters"
	endpoint = addQuery(endpoint, params)
	status, headers, body, err := c.doJSON("GET", endpoint, nil, "application/vnd.api+json")
	if err != nil {
		return nil, headers, err
	}
	if status != http.StatusOK {
		return nil, headers, APIError{Message: "list letters failed", Status: status, RequestID: headers.Get("X-Request-Id")}
	}
	payload, err := decodeJSON(body)
	return payload, headers, err
}

func (c Client) GetLetter(orgID, letterID string) (map[string]any, http.Header, error) {
	endpoint := c.APIBase + "/organisations/" + orgID + "/letters/" + letterID
	status, headers, body, err := c.doJSON("GET", endpoint, nil, "application/vnd.api+json")
	if err != nil {
		return nil, headers, err
	}
	if status != http.StatusOK {
		return nil, headers, APIError{Message: "get letter failed", Status: status, RequestID: headers.Get("X-Request-Id")}
	}
	payload, err := decodeJSON(body)
	return payload, headers, err
}

func (c Client) GetFileUpload() (string, string, http.Header, error) {
	endpoint := c.APIBase + "/file-upload"
	status, headers, body, err := c.doJSON("GET", endpoint, nil, "application/vnd.api+json")
	if err != nil {
		return "", "", headers, err
	}
	if status != http.StatusOK {
		return "", "", headers, APIError{Message: "file upload request failed", Status: status, RequestID: headers.Get("X-Request-Id")}
	}
	payload, err := decodeJSON(body)
	if err != nil {
		return "", "", headers, err
	}
	data, ok := payload["data"].(map[string]any)
	if !ok {
		return "", "", headers, APIError{Message: "file upload response missing data", Status: status}
	}
	attrs, ok := data["attributes"].(map[string]any)
	if !ok {
		return "", "", headers, APIError{Message: "file upload response missing attributes", Status: status}
	}
	urlValue, _ := attrs["url"].(string)
	sigValue, _ := attrs["url_signature"].(string)
	if urlValue == "" || sigValue == "" {
		return "", "", headers, APIError{Message: "file upload response missing url data", Status: status}
	}
	return urlValue, sigValue, headers, nil
}

func (c Client) UploadFile(uploadURL, filePath string, timeout time.Duration) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", uploadURL, file)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.ContentLength = info.Size()
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		return APIError{Message: "file upload failed", Status: resp.StatusCode}
	}
	return nil
}

func (c Client) CreateLetter(orgID string, payload map[string]any, idempotencyKey string) (map[string]any, http.Header, error) {
	endpoint := c.APIBase + "/organisations/" + orgID + "/letters"
	status, headers, body, err := c.doJSON("POST", endpoint, payload, "application/vnd.api+json", idempotencyKey)
	if err != nil {
		return nil, headers, err
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return nil, headers, APIError{Message: "create letter failed", Status: status, RequestID: headers.Get("X-Request-Id")}
	}
	payloadMap, err := decodeJSON(body)
	return payloadMap, headers, err
}

func (c Client) SendLetter(orgID, letterID string, payload map[string]any, idempotencyKey string) (map[string]any, http.Header, error) {
	endpoint := c.APIBase + "/organisations/" + orgID + "/letters/" + letterID + "/send"
	status, headers, body, err := c.doJSON("PATCH", endpoint, payload, "application/vnd.api+json", idempotencyKey)
	if err != nil {
		return nil, headers, err
	}
	if status != http.StatusOK && status != http.StatusNoContent {
		return nil, headers, APIError{Message: "send letter failed", Status: status, RequestID: headers.Get("X-Request-Id")}
	}
	if len(body) == 0 {
		return map[string]any{}, headers, nil
	}
	payloadMap, err := decodeJSON(body)
	return payloadMap, headers, err
}

func (c Client) doJSON(method, endpoint string, payload map[string]any, contentType string, extraHeaders ...string) (int, http.Header, []byte, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return 0, nil, nil, err
		}
		body = bytes.NewBuffer(encoded)
	}

	headers := map[string]string{
		"Accept":       contentType,
		"Content-Type": contentType,
	}
	if c.AccessToken != "" {
		headers["Authorization"] = "Bearer " + c.AccessToken
	}
	if len(extraHeaders) > 0 && extraHeaders[0] != "" {
		headers["Idempotency-Key"] = extraHeaders[0]
	}

	return c.doRequest(method, endpoint, headers, body)
}

func (c Client) doRequest(method, endpoint string, headers map[string]string, body io.Reader) (int, http.Header, []byte, error) {
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return 0, nil, nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	for key, value := range headers {
		if value == "" {
			continue
		}
		req.Header.Set(key, value)
	}
	client := &http.Client{Timeout: c.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, resp.Header, nil, err
	}
	return resp.StatusCode, resp.Header, responseBody, nil
}

func decodeJSON(body []byte) (map[string]any, error) {
	if len(body) == 0 {
		return map[string]any{}, nil
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return map[string]any{}, err
	}
	return payload, nil
}

func addQuery(endpoint string, params map[string]string) string {
	if len(params) == 0 {
		return endpoint
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return endpoint
	}
	values := parsed.Query()
	for key, value := range params {
		if value == "" {
			continue
		}
		values.Set(key, value)
	}
	parsed.RawQuery = values.Encode()
	return parsed.String()
}

func DefaultFileName(path string) string {
	base := filepath.Base(path)
	if base == "." || base == string(filepath.Separator) {
		return "document.pdf"
	}
	return base
}
