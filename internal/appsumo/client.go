package appsumo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const DefaultBaseURL = "https://appsumo.com"
const maxResponseBodyBytes = 10 << 20

type ClientOptions struct {
	BaseURL    string
	Cookie     string
	HTTPClient *http.Client
}

type Client struct {
	baseURL string
	cookie  string
	http    *http.Client
}

func NewClient(options ClientOptions) *Client {
	baseURL := strings.TrimRight(options.BaseURL, "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	httpClient := options.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		baseURL: baseURL,
		cookie:  strings.TrimSpace(options.Cookie),
		http:    httpClient,
	}
}

func (c *Client) AuthStatus(ctx context.Context) (*AuthStatus, error) {
	var raw struct {
		User struct {
			IsAuthenticated bool   `json:"is_authenticated"`
			Email           string `json:"email"`
			CustomerID      string `json:"customer_id"`
		} `json:"user"`
	}
	if err := c.getJSON(ctx, "/api/sessions/current/", nil, &raw); err != nil {
		return nil, err
	}
	return &AuthStatus{
		HasCookie:     c.cookie != "",
		Authenticated: raw.User.IsAuthenticated,
	}, nil
}

func (c *Client) FetchProductsPage(ctx context.Context, page int) (*ProductsPage, error) {
	if page < 1 {
		page = 1
	}
	var envelope ProductsEnvelope
	if err := c.getJSON(ctx, "/api/v2/account/products/", map[string]string{"page": fmt.Sprintf("%d", page)}, &envelope); err != nil {
		return nil, err
	}
	return &envelope.Products, nil
}

func (c *Client) FetchAllProducts(ctx context.Context) ([]Product, int, error) {
	var all []Product
	total := 0
	for pageNum := 1; pageNum <= 200; pageNum++ {
		page, err := c.FetchProductsPage(ctx, pageNum)
		if err != nil {
			return nil, 0, err
		}
		if pageNum == 1 {
			total = page.Count
		}
		all = append(all, page.Results...)
		if page.Next == nil || strings.TrimSpace(*page.Next) == "" {
			return all, total, nil
		}
	}
	return nil, 0, fmt.Errorf("pagination exceeded safety cap")
}

func (c *Client) FetchProductsCSV(ctx context.Context) ([]byte, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v2/account/products/download/", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, readErr := readBody(resp.Body, maxResponseBodyBytes)
	if readErr != nil {
		return nil, readErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s returned HTTP %d", req.URL.Path, resp.StatusCode)
	}
	return body, nil
}

// getHTML reads a rendered page body. Public product pages embed their data as
// a __NEXT_DATA__ island, which is the only published source for a slug's deal id.
func (c *Client) getHTML(ctx context.Context, path string) ([]byte, error) {
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, readErr := readBody(resp.Body, maxResponseBodyBytes)
	if readErr != nil {
		return nil, readErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s returned HTTP %d", req.URL.Path, resp.StatusCode)
	}
	return body, nil
}

func (c *Client) getJSON(ctx context.Context, path string, query map[string]string, target any) error {
	req, err := c.newRequest(ctx, http.MethodGet, path, query)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, readErr := readBody(resp.Body, maxResponseBodyBytes)
	if readErr != nil {
		return readErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GET %s returned HTTP %d", req.URL.Path, resp.StatusCode)
	}
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(strings.ToLower(contentType), "application/json") {
		return fmt.Errorf("GET %s returned non-json content; session may be unauthenticated", req.URL.Path)
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode %s: %w", req.URL.Path, err)
	}
	return nil
}

func (c *Client) newRequest(ctx context.Context, method, path string, query map[string]string) (*http.Request, error) {
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, err
	}
	rel, err := url.Parse(path)
	if err != nil {
		return nil, err
	}
	full := base.ResolveReference(rel)
	values := full.Query()
	for key, value := range query {
		values.Set(key, value)
	}
	full.RawQuery = values.Encode()

	req, err := http.NewRequestWithContext(ctx, method, full.String(), nil)
	if err != nil {
		return nil, err
	}
	if c.cookie != "" && shouldSendCookie(full) {
		req.Header.Set("Cookie", c.cookie)
	}
	req.Header.Set("Accept", "application/json, text/csv;q=0.9, */*;q=0.1")
	req.Header.Set("User-Agent", "appsumo-cli/0.1")
	return req, nil
}

func shouldSendCookie(target *url.URL) bool {
	host := strings.TrimSuffix(strings.ToLower(target.Hostname()), ".")
	if host == "localhost" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return true
	}
	return target.Scheme == "https" && host == "appsumo.com"
}

func readBody(body io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response body exceeded %d byte limit", limit)
	}
	return data, nil
}
