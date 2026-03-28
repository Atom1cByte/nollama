package api

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type Client struct {
	BaseURL string
}

func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &Client{BaseURL: normalizeURL(baseURL)}
}

func normalizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if u.Port() == "" {
		u.Host = u.Host + ":11434"
	}
	return u.String()
}

type Model struct {
	Name        string    `json:"name"`
	ModifiedAt  time.Time `json:"modified_at"`
	Size        int64     `json:"size"`
	Digest      string    `json:"digest"`
	Endpoint    string    `json:"endpoint"`
	ServerCount int       `json:"server_count"`
}

type TagResponse struct {
	Models []struct {
		Name       string    `json:"name"`
		ModifiedAt time.Time `json:"modified_at"`
		Size       int64     `json:"size"`
		Digest     string    `json:"digest"`
	} `json:"models"`
}

func (c *Client) ListModels() ([]Model, error) {
	cli := &http.Client{Timeout: 10 * time.Second}
	resp, err := cli.Get(fmt.Sprintf("%s/api/tags", c.BaseURL))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var target TagResponse
	if err := json.NewDecoder(resp.Body).Decode(&target); err != nil {
		return nil, err
	}

	models := make([]Model, len(target.Models))
	for i, m := range target.Models {
		models[i] = Model{
			Name:       m.Name,
			ModifiedAt: m.ModifiedAt,
			Size:       m.Size,
			Digest:     m.Digest,
			Endpoint:   c.BaseURL,
		}
	}
	return models, nil
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type ChatResponse struct {
	Model              string    `json:"model"`
	CreatedAt          time.Time `json:"created_at"`
	Message            Message   `json:"message"`
	Done               bool      `json:"done"`
	TotalDuration      int64     `json:"total_duration"`
	LoadDuration       int64     `json:"load_duration"`
	PromptEvalCount    int       `json:"prompt_eval_count"`
	PromptEvalDuration int64     `json:"prompt_eval_duration"`
	EvalCount          int       `json:"eval_count"`
	EvalDuration       int64     `json:"eval_duration"`
}

func (c *Client) ChatStream(req ChatRequest) (<-chan ChatResponse, <-chan error, error) {
	req.Stream = true
	body, err := json.Marshal(req)
	if err != nil {
		return nil, nil, err
	}

	resp, err := http.Post(fmt.Sprintf("%s/api/chat", c.BaseURL), "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, nil, err
	}

	ch := make(chan ChatResponse)
	errCh := make(chan error, 1)

	go func() {
		defer resp.Body.Close()
		defer close(ch)
		defer close(errCh)

		if resp.StatusCode != 200 {
			errCh <- fmt.Errorf("HTTP %d", resp.StatusCode)
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			var partial ChatResponse
			if err := json.Unmarshal(scanner.Bytes(), &partial); err != nil {
				continue
			}
			ch <- partial
			if partial.Done {
				break
			}
		}
		if err := scanner.Err(); err != nil {
			errCh <- err
		}
	}()

	return ch, errCh, nil
}

type EndpointStat struct {
	URL        string
	ModelCount int
	TotalSize  int64
	Error      error
}

// Router client - talks to the nollama backend
type RouterClient struct {
	BaseURL string
}

func NewRouterClient(baseURL string) *RouterClient {
	if baseURL == "" {
		baseURL = "http://localhost:5000"
	}
	return &RouterClient{BaseURL: baseURL}
}

type RouterModel struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ServerCount int    `json:"server_count"`
	Servers     []struct {
		URL     string `json:"url"`
		IP      string `json:"ip"`
		Country string `json:"country"`
		City    string `json:"city"`
	} `json:"servers"`
}

type RouterStats struct {
	TotalServers      int `json:"total_servers"`
	ServersWithModels int `json:"servers_with_models"`
	TotalModels       int `json:"total_models"`
}

type RouterStatusResponse struct {
	Success bool        `json:"success"`
	Stats   RouterStats `json:"stats"`
}

func (r *RouterClient) LoadScanData(force bool) error {
	cli := &http.Client{Timeout: 30 * time.Second}
	body, _ := json.Marshal(map[string]bool{"force": force})
	resp, err := cli.Post(fmt.Sprintf("%s/api/router/load", r.BaseURL), "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

func (r *RouterClient) GetStatus() (*RouterStats, error) {
	cli := &http.Client{Timeout: 10 * time.Second}
	resp, err := cli.Get(fmt.Sprintf("%s/api/router/status", r.BaseURL))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var status RouterStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}
	return &status.Stats, nil
}

func (r *RouterClient) ListModels(query string, limit int) ([]RouterModel, error) {
	cli := &http.Client{Timeout: 10 * time.Second}
	apiURL := fmt.Sprintf("%s/api/router/models?limit=%d", r.BaseURL, limit)
	if query != "" {
		apiURL += "&q=" + url.QueryEscape(query)
	}

	resp, err := cli.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []RouterModel `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Data, nil
}

func (r *RouterClient) ChatStream(model string, messages []Message) (<-chan ChatResponse, <-chan error, error) {
	req := map[string]interface{}{
		"model":    model,
		"messages": messages,
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, nil, err
	}

	resp, err := http.Post(fmt.Sprintf("%s/api/chat/router", r.BaseURL), "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, nil, err
	}

	ch := make(chan ChatResponse)
	errCh := make(chan error, 1)

	go func() {
		defer resp.Body.Close()
		defer close(ch)
		defer close(errCh)

		if resp.StatusCode != 200 {
			errCh <- fmt.Errorf("HTTP %d", resp.StatusCode)
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")

			// Check for error
			var errCheck struct {
				Error struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			if json.Unmarshal([]byte(data), &errCheck) == nil && errCheck.Error.Message != "" {
				errCh <- fmt.Errorf(errCheck.Error.Message)
				return
			}

			var partial ChatResponse
			if err := json.Unmarshal([]byte(data), &partial); err != nil {
				continue
			}
			ch <- partial
			if partial.Done {
				break
			}
		}
		if err := scanner.Err(); err != nil {
			errCh <- err
		}
	}()

	return ch, errCh, nil
}

func (r *RouterClient) ResetFailures(url string) error {
	cli := &http.Client{Timeout: 10 * time.Second}
	body, _ := json.Marshal(map[string]string{"url": url})
	resp, err := cli.Post(fmt.Sprintf("%s/api/router/reset-failures", r.BaseURL), "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// MultiClient - supports both direct Ollama and router modes
type MultiClient struct {
	Endpoints []string
	Clients   map[string]*Client
	Router    *RouterClient
	Mode      string // "direct" or "router"
}

func NewMultiClient() *MultiClient {
	return &MultiClient{
		Endpoints: make([]string, 0),
		Clients:   make(map[string]*Client),
		Router:    NewRouterClient("http://localhost:5000"),
		Mode:      "direct",
	}
}

func (m *MultiClient) EnableRouter() {
	m.Mode = "router"
}

func (m *MultiClient) AddURL(raw string) string {
	url := normalizeURL(raw)
	if url == "" {
		return ""
	}
	for _, e := range m.Endpoints {
		if e == url {
			return url
		}
	}
	m.Endpoints = append(m.Endpoints, url)
	m.Clients[url] = NewClient(url)
	return url
}

func (m *MultiClient) RemoveURL(url string) {
	url = normalizeURL(url)
	for i, e := range m.Endpoints {
		if e == url {
			m.Endpoints = append(m.Endpoints[:i], m.Endpoints[i+1:]...)
			delete(m.Clients, url)
			break
		}
	}
}

func (m *MultiClient) LoadFromFile(filepath string) (int, error) {
	b, err := os.ReadFile(filepath)
	if err != nil {
		return 0, err
	}
	lines := strings.Split(strings.ReplaceAll(string(b), ",", "\n"), "\n")
	added := 0
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" && !strings.HasPrefix(l, "#") {
			r := m.AddURL(l)
			if r != "" {
				added++
			}
		}
	}
	return added, nil
}

func (m *MultiClient) BulkAdd(input string) int {
	lines := strings.Split(strings.ReplaceAll(input, ",", "\n"), "\n")
	added := 0
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" && !strings.HasPrefix(l, "#") {
			r := m.AddURL(l)
			if r != "" {
				added++
			}
		}
	}
	return added
}

func (m *MultiClient) ListAllModels() []Model {
	all := make([]Model, 0)
	for _, c := range m.Clients {
		models, err := c.ListModels()
		if err == nil {
			all = append(all, models...)
		}
	}
	return all
}

func (m *MultiClient) ListRouterModels(query string) ([]RouterModel, error) {
	return m.Router.ListModels(query, 100)
}

func (m *MultiClient) LoadRouterScan(force bool) error {
	return m.Router.LoadScanData(force)
}

func (m *MultiClient) GetRouterStatus() (*RouterStats, error) {
	return m.Router.GetStatus()
}

func (m *MultiClient) GetStats() []EndpointStat {
	stats := make([]EndpointStat, 0)
	for _, url := range m.Endpoints {
		c := m.Clients[url]
		models, err := c.ListModels()
		if err != nil {
			stats = append(stats, EndpointStat{
				URL:   url,
				Error: err,
			})
			continue
		}
		var sz int64
		for _, md := range models {
			sz += md.Size
		}
		stats = append(stats, EndpointStat{
			URL:        url,
			ModelCount: len(models),
			TotalSize:  sz,
			Error:      nil,
		})
	}
	return stats
}

func (m *MultiClient) GetClientForModel(url string) *Client {
	return m.Clients[url]
}
