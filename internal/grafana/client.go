package grafana

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func New(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   strings.TrimSpace(token),
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type DataSource struct {
	ID       int    `json:"id"`
	UID      string `json:"uid"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	URL      string `json:"url"`
	Database string `json:"database"`
	Access   string `json:"access"`
}

type SearchDashboard struct {
	UID   string `json:"uid"`
	Title string `json:"title"`
	Type  string `json:"type"`
	URI   string `json:"uri"`
}

type Health struct {
	Database string `json:"database"`
	Version  string `json:"version"`
}

type QueryRequest struct {
	From    string         `json:"from"`
	To      string         `json:"to"`
	Queries []QueryPayload `json:"queries"`
}

type QueryPayload struct {
	RefID      string         `json:"refId"`
	Datasource map[string]any `json:"datasource"`
	Expr       string         `json:"expr,omitempty"`
	QueryType  string         `json:"queryType,omitempty"`
	MaxLines   int            `json:"maxLines,omitempty"`
	Instant    bool           `json:"instant,omitempty"`
	Raw        map[string]any `json:"-"`
}

func (q QueryPayload) MarshalJSON() ([]byte, error) {
	type alias QueryPayload
	base := map[string]any{}
	b, err := json.Marshal(alias(q))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &base); err != nil {
		return nil, err
	}
	for k, v := range q.Raw {
		base[k] = v
	}
	if q.MaxLines <= 0 {
		delete(base, "maxLines")
	}
	if q.QueryType == "" {
		delete(base, "queryType")
	}
	if q.Expr == "" {
		delete(base, "expr")
	}
	if !q.Instant {
		delete(base, "instant")
	}
	return json.Marshal(base)
}

func (c *Client) GetDataSources(ctx context.Context) ([]DataSource, error) {
	var out []DataSource
	if err := c.getJSON(ctx, "/api/datasources", &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetHealth(ctx context.Context) (*Health, error) {
	out := &Health{}
	if err := c.getJSON(ctx, "/api/health", out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) SearchDashboards(ctx context.Context) ([]SearchDashboard, error) {
	var out []SearchDashboard
	if err := c.getJSON(ctx, "/api/search?type=dash-db", &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetDashboardByUID(ctx context.Context, uid string) (map[string]any, error) {
	out := map[string]any{}
	u := "/api/dashboards/uid/" + url.PathEscape(uid)
	if err := c.getJSON(ctx, u, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) Query(ctx context.Context, req QueryRequest) (map[string]any, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(ctx, http.MethodPost, "/api/ds/query", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("grafana query failed: %s - %s", resp.Status, strings.TrimSpace(string(b)))
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) getJSON(ctx context.Context, endpoint string, out any) error {
	resp, err := c.do(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("grafana api failed: %s - %s", resp.Status, strings.TrimSpace(string(b)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) do(ctx context.Context, method, endpoint string, body []byte) (*http.Response, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, err
	}
	u.Path = path.Join(u.Path, endpoint)
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return c.http.Do(req)
}

func ResolveSourceByNameOrUID(sources []DataSource, hint string) (*DataSource, error) {
	hint = strings.TrimSpace(hint)
	for i := range sources {
		s := &sources[i]
		if s.UID == hint || strings.EqualFold(s.Name, hint) {
			return s, nil
		}
	}
	return nil, fmt.Errorf("source %q not found", hint)
}

func SourceByUID(sources []DataSource, uid string) *DataSource {
	for i := range sources {
		if sources[i].UID == uid {
			return &sources[i]
		}
	}
	return nil
}
