package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultAPIURL = "http://127.0.0.1:18080"

type Client struct {
	BaseURL     string
	Token       string
	RequestID   string
	AuditActor  string
	AuditSource string
	HTTPClient  *http.Client
}

type APIError struct {
	StatusCode int
	Status     string
	Body       string
}

func (e *APIError) Error() string {
	message := strings.TrimSpace(e.Body)
	var body struct {
		Error string `json:"error"`
	}
	if json.Unmarshal([]byte(message), &body) == nil && strings.TrimSpace(body.Error) != "" {
		message = body.Error
	}
	if message == "" {
		message = e.Status
	}
	return fmt.Sprintf("mbox API request failed: %s", message)
}

func NewClient(baseURL string, token string) (*Client, error) {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultAPIURL
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid API URL %q", baseURL)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return &Client{
		BaseURL: parsed.String(),
		Token:   strings.TrimSpace(token),
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (c *Client) JSON(ctx context.Context, method string, path string, payload any, out any) error {
	response, err := c.Raw(ctx, method, path, payload)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if out == nil {
		_, _ = io.Copy(io.Discard, response.Body)
		return nil
	}
	return json.NewDecoder(response.Body).Decode(out)
}

func (c *Client) Raw(ctx context.Context, method string, path string, payload any) (*http.Response, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return nil, err
	}
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Set("Accept", "application/json")
	c.setRequestHeaders(request.Header)
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return response, nil
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(response.Body, 32*1024))
	return nil, &APIError{
		StatusCode: response.StatusCode,
		Status:     response.Status,
		Body:       string(data),
	}
}

func (c *Client) RawBody(ctx context.Context, method string, path string, body io.Reader, contentType string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(contentType) != "" {
		request.Header.Set("Content-Type", strings.TrimSpace(contentType))
	}
	request.Header.Set("Accept", "application/json")
	c.setRequestHeaders(request.Header)
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return response, nil
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(response.Body, 32*1024))
	return nil, &APIError{
		StatusCode: response.StatusCode,
		Status:     response.Status,
		Body:       string(data),
	}
}

func (c *Client) setRequestHeaders(headers http.Header) {
	if c.Token != "" {
		headers.Set("Authorization", "Bearer "+c.Token)
	}
	if strings.TrimSpace(c.RequestID) != "" {
		headers.Set("X-Mbox-Request-ID", strings.TrimSpace(c.RequestID))
	}
	if strings.TrimSpace(c.AuditActor) != "" {
		headers.Set("X-Mbox-Audit-Actor", strings.TrimSpace(c.AuditActor))
	}
	if strings.TrimSpace(c.AuditSource) != "" {
		headers.Set("X-Mbox-Audit-Source", strings.TrimSpace(c.AuditSource))
	}
}
