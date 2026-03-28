package api

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type Client struct {
	BaseURL string
}

func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &Client{BaseURL: baseURL}
}

type Model struct {
	Name       string    `json:"name"`
	ModifiedAt time.Time `json:"modified_at"`
	Size       int64     `json:"size"`
	Digest     string    `json:"digest"`
	URL        string    `json:"-"`
}

type TagResponse struct {
	Models []Model `json:"models"`
}

type APIStats struct {
	URL          string
	ResponseTime time.Duration
	ModelCount   int
	TotalSize    int64
	LastUpdated  time.Time
	Error        error
}

func (c *Client) ListModels() ([]Model, error) {
	resp, err := http.Get(fmt.Sprintf("%s/api/tags", c.BaseURL))
	if err != nil {
		return nil, fmt.Errorf("connection failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	var target TagResponse
	if err := json.NewDecoder(resp.Body).Decode(&target); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}
	for i := range target.Models {
		target.Models[i].URL = c.BaseURL
	}
	return target.Models, nil
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

type MultiClient struct {
	clients []*Client
	stats   map[string]*APIStats
	mu      sync.RWMutex
}

func NewMultiClient(urls []string) *MultiClient {
	clients := make([]*Client, len(urls))
	for i, url := range urls {
		clients[i] = NewClient(url)
	}
	return &MultiClient{
		clients: clients,
		stats:   make(map[string]*APIStats),
	}
}

func (m *MultiClient) AddURL(url string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.clients {
		if c.BaseURL == url {
			return
		}
	}
	m.clients = append(m.clients, NewClient(url))
}

func (m *MultiClient) RemoveURL(url string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	newClients := make([]*Client, 0)
	for _, c := range m.clients {
		if c.BaseURL != url {
			newClients = append(newClients, c)
		}
	}
	m.clients = newClients
	delete(m.stats, url)
}

func (m *MultiClient) GetURLs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	urls := make([]string, len(m.clients))
	for i, c := range m.clients {
		urls[i] = c.BaseURL
	}
	return urls
}

func (m *MultiClient) ListAllModels() []Model {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var allModels []Model
	for _, client := range m.clients {
		models, err := client.ListModels()
		if err != nil {
			m.stats[client.BaseURL] = &APIStats{
				URL:         client.BaseURL,
				Error:       err,
				LastUpdated: time.Now(),
			}
			continue
		}
		for i := range models {
			models[i].URL = client.BaseURL
		}
		allModels = append(allModels, models...)

		totalSize := int64(0)
		for _, mod := range models {
			totalSize += mod.Size
		}
		m.stats[client.BaseURL] = &APIStats{
			URL:         client.BaseURL,
			ModelCount:  len(models),
			TotalSize:   totalSize,
			LastUpdated: time.Now(),
		}
	}
	return allModels
}

func (m *MultiClient) GetStats() map[string]*APIStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	stats := make(map[string]*APIStats)
	for k, v := range m.stats {
		stats[k] = v
	}
	return stats
}

func (m *MultiClient) GetClientForModel(modelName string) *Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, client := range m.clients {
		models, err := client.ListModels()
		if err == nil {
			for _, m := range models {
				if m.Name == modelName {
					return client
				}
			}
		}
	}
	if len(m.clients) > 0 {
		return m.clients[0]
	}
	return nil
}
