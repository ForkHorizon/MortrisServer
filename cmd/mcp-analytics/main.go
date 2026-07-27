// Command mcp-analytics exposes the existing read-only analytics HTTP API
// (internal/httpapi/server_analytics_routes.go) as MCP tools, so an LLM can
// compose queries in plain language instead of the dashboard UI. It is a
// thin proxy: every tool just calls the corresponding endpoint and returns
// its JSON body — no query logic is duplicated here.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// apiClient logs in once with ANALYTICS_EMAIL/ANALYTICS_PASSWORD and
// attaches the resulting session cookie by hand rather than via
// http.CookieJar: the cookie is marked Secure (internal/adminauth/cookies.go),
// which a jar refuses to send back over a plain-http ANALYTICS_BASE_URL.
type apiClient struct {
	baseURL    string
	email      string
	password   string
	httpClient *http.Client

	mu      sync.Mutex
	session string
}

func newAPIClient() (*apiClient, error) {
	baseURL := os.Getenv("ANALYTICS_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	email := os.Getenv("ANALYTICS_EMAIL")
	password := os.Getenv("ANALYTICS_PASSWORD")
	if email == "" || password == "" {
		return nil, fmt.Errorf("ANALYTICS_EMAIL and ANALYTICS_PASSWORD must be set")
	}
	return &apiClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		email:      email,
		password:   password,
		httpClient: &http.Client{},
	}, nil
}

func (c *apiClient) login(ctx context.Context) error {
	body, _ := json.Marshal(map[string]string{"username": c.email, "password": c.password})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/auth/login", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("login failed (%s): %s", resp.Status, string(b))
	}
	for _, ck := range resp.Cookies() {
		if ck.Name == "session" {
			c.mu.Lock()
			c.session = ck.Value
			c.mu.Unlock()
			return nil
		}
	}
	return fmt.Errorf("login response had no session cookie")
}

// get calls a read-only analytics endpoint and returns its raw JSON body,
// re-logging in once on a 401 (the session idle/absolute timeout in
// internal/adminauth/cookies.go).
func (c *apiClient) get(ctx context.Context, path string, params url.Values) (string, error) {
	for attempt := 0; attempt < 2; attempt++ {
		c.mu.Lock()
		session := c.session
		c.mu.Unlock()
		if session == "" {
			if err := c.login(ctx); err != nil {
				return "", err
			}
			c.mu.Lock()
			session = c.session
			c.mu.Unlock()
		}

		u := c.baseURL + path
		if len(params) > 0 {
			u += "?" + params.Encode()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("Cookie", "session="+session)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return "", err
		}
		b, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return "", err
		}

		if resp.StatusCode == http.StatusUnauthorized && attempt == 0 {
			c.mu.Lock()
			c.session = ""
			c.mu.Unlock()
			continue
		}
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("%s: %s", resp.Status, string(b))
		}
		return string(b), nil
	}
	return "", fmt.Errorf("re-authentication failed")
}

func setIf(v url.Values, key, val string) {
	if val != "" {
		v.Set(key, val)
	}
}

// call runs a GET against the analytics API and wraps the result (or
// error) as a CallToolResult. Errors are returned as a tool error rather
// than a Go error so the model sees the API's message and can retry with
// corrected arguments.
func call(ctx context.Context, c *apiClient, path string, params url.Values) (*mcp.CallToolResult, any, error) {
	body, err := c.get(ctx, path, params)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
		}, nil, nil
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: body}}}, nil, nil
}

type dateRangeArgs struct {
	Project  string `json:"project" jsonschema:"project ID (required)"`
	From     string `json:"from,omitempty" jsonschema:"RFC3339 start time, defaults to 7 days ago"`
	To       string `json:"to,omitempty" jsonschema:"RFC3339 end time, defaults to now"`
	Timezone string `json:"timezone,omitempty" jsonschema:"IANA timezone, defaults to UTC"`
}

func (a dateRangeArgs) params() url.Values {
	v := url.Values{"project": {a.Project}}
	setIf(v, "from", a.From)
	setIf(v, "to", a.To)
	setIf(v, "timezone", a.Timezone)
	return v
}

type eventCountsArgs struct {
	Project       string `json:"project" jsonschema:"project ID (required)"`
	From          string `json:"from,omitempty" jsonschema:"RFC3339 start time, defaults to 7 days ago"`
	To            string `json:"to,omitempty" jsonschema:"RFC3339 end time, defaults to now"`
	Timezone      string `json:"timezone,omitempty" jsonschema:"IANA timezone, defaults to UTC"`
	Name          string `json:"name,omitempty" jsonschema:"filter to one cataloged event name"`
	AppVersion    string `json:"app_version,omitempty"`
	BuildNumber   string `json:"build_number,omitempty"`
	Platform      string `json:"platform,omitempty"`
	PropertyKey   string `json:"property_key,omitempty" jsonschema:"cataloged property key; requires name and property_value"`
	PropertyValue string `json:"property_value,omitempty" jsonschema:"requires property_key"`
}

type recentEventsArgs struct {
	Project          string `json:"project" jsonschema:"project ID (required)"`
	Name             string `json:"name,omitempty" jsonschema:"exact event name, not catalog-checked"`
	Platform         string `json:"platform,omitempty"`
	AppVersion       string `json:"app_version,omitempty"`
	InstallID        string `json:"install_id,omitempty"`
	Limit            int    `json:"limit,omitempty" jsonschema:"max 500, default 100"`
	BeforeReceivedAt string `json:"before_received_at,omitempty" jsonschema:"keyset pagination cursor, pairs with before_event_id"`
	BeforeEventID    string `json:"before_event_id,omitempty"`
}

type funnelArgs struct {
	Project       string `json:"project" jsonschema:"project ID (required)"`
	From          string `json:"from,omitempty" jsonschema:"RFC3339 start time, defaults to 7 days ago"`
	To            string `json:"to,omitempty" jsonschema:"RFC3339 end time, defaults to now"`
	Steps         string `json:"steps" jsonschema:"comma-separated cataloged product event names, 2-5 steps, in order"`
	WindowSeconds int    `json:"window_seconds,omitempty" jsonschema:"max 86400, default 24h completion window"`
}

func registerTools(server *mcp.Server, client *apiClient) {
	registerOverviewTool(server, client)
	registerEventCountsTool(server, client)
	registerRecentEventsTool(server, client)
	registerFunnelTool(server, client)
	registerRetentionTool(server, client)
}

func registerOverviewTool(server *mcp.Server, client *apiClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "overview",
		Description: "Project-wide event overview for a date range: totals, active installations, daily trend.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args dateRangeArgs) (*mcp.CallToolResult, any, error) {
		return call(ctx, client, "/api/v1/analytics/overview", args.params())
	})
}

func registerEventCountsTool(server *mcp.Server, client *apiClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "event_counts",
		Description: "Event counts and daily trend for a date range, optionally filtered by event name, app version, build, platform, or one cataloged property.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args eventCountsArgs) (*mcp.CallToolResult, any, error) {
		v := url.Values{"project": {args.Project}}
		setIf(v, "from", args.From)
		setIf(v, "to", args.To)
		setIf(v, "timezone", args.Timezone)
		setIf(v, "name", args.Name)
		setIf(v, "app_version", args.AppVersion)
		setIf(v, "build_number", args.BuildNumber)
		setIf(v, "platform", args.Platform)
		setIf(v, "property_key", args.PropertyKey)
		setIf(v, "property_value", args.PropertyValue)
		return call(ctx, client, "/api/v1/analytics/events", v)
	})
}

func registerRecentEventsTool(server *mcp.Server, client *apiClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "recent_events",
		Description: "Live tail of the most recent raw events, newest first, optionally filtered and keyset-paginated.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args recentEventsArgs) (*mcp.CallToolResult, any, error) {
		v := url.Values{"project": {args.Project}}
		setIf(v, "name", args.Name)
		setIf(v, "platform", args.Platform)
		setIf(v, "app_version", args.AppVersion)
		setIf(v, "install_id", args.InstallID)
		if args.Limit != 0 {
			v.Set("limit", strconv.Itoa(args.Limit))
		}
		setIf(v, "before_received_at", args.BeforeReceivedAt)
		setIf(v, "before_event_id", args.BeforeEventID)
		return call(ctx, client, "/api/v1/analytics/events/recent", v)
	})
}

func registerFunnelTool(server *mcp.Server, client *apiClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "funnel",
		Description: "Step-by-step conversion funnel over 2-5 cataloged product events within a completion window.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args funnelArgs) (*mcp.CallToolResult, any, error) {
		v := url.Values{"project": {args.Project}, "steps": {args.Steps}}
		setIf(v, "from", args.From)
		setIf(v, "to", args.To)
		if args.WindowSeconds != 0 {
			v.Set("window_seconds", strconv.Itoa(args.WindowSeconds))
		}
		return call(ctx, client, "/api/v1/analytics/funnel", v)
	})
}

func registerRetentionTool(server *mcp.Server, client *apiClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "retention",
		Description: "Cohort retention (day-N return rate) for installations first seen in the given date range.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args dateRangeArgs) (*mcp.CallToolResult, any, error) {
		return call(ctx, client, "/api/v1/analytics/retention", args.params())
	})
}

func main() {
	client, err := newAPIClient()
	if err != nil {
		log.Fatal(err)
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "mortris-analytics", Version: "0.1.0"}, nil)
	registerTools(server, client)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
