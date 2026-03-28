package tui

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"nollama/api"

	"github.com/NimbleMarkets/ntcharts/linechart/streamlinechart"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/harmonica"
	"github.com/charmbracelet/lipgloss"
	bz "github.com/lrstanley/bubblezone"
)

type ModelsState int

const (
	stateURLInput ModelsState = iota
	stateLoading
	stateModelSelection
	stateChat
	stateError
	stateURLList
	stateMultiModelView
	stateAPITags
	stateBulkURLInput
	stateFileInput
)

type Model struct {
	client       *api.Client
	multiClient  *api.MultiClient
	state        ModelsState
	pendingState ModelsState
	models       []api.Model
	list         list.Model
	input        textinput.Model
	urlInput     textinput.Model
	bulkURLInput textinput.Model
	fileInput    textinput.Model
	viewport     viewport.Model
	spinner      spinner.Model
	messages     []api.Message
	err          error
	notification string
	notifTime    time.Time
	width        int
	height       int
	streaming    bool
	currentRes   string
	selectedMd   string
	selectedURL  string
	program      *tea.Program
	zone         *bz.Manager

	tpsChart    streamlinechart.Model
	startTime   time.Time
	lastTps     float64
	totalTokens int

	spring      harmonica.Spring
	interpolVal float64
	targetVal   float64
}

type modelItem struct {
	name string
	size int64
	url  string
}

func (i modelItem) Title() string { return i.name }
func (i modelItem) Description() string {
	return fmt.Sprintf("%.2f GB | %s", float64(i.size)/(1024*1024*1024), i.url)
}
func (i modelItem) FilterValue() string { return i.name }

type urlItem struct {
	url string
}

func (i urlItem) Title() string       { return i.url }
func (i urlItem) Description() string { return "Ollama Instance" }
func (i urlItem) FilterValue() string { return i.url }

func NewModel(initialURL string) *Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(PrimaryColor)

	ui := textinput.New()
	ui.Placeholder = "localhost:11434 or http://localhost:11434"
	ui.Focus()
	if initialURL != "" {
		ui.SetValue(normalizeURL(initialURL))
	} else {
		ui.SetValue("localhost:11434")
	}

	bulkUI := textinput.New()
	bulkUI.Placeholder = "Enter URLs (one per line or comma-separated)..."
	bulkUI.Focus()

	fileUI := textinput.New()
	fileUI.Placeholder = "Enter path to file containing URLs..."
	fileUI.Focus()

	ti := textinput.New()
	ti.Placeholder = "Type a message..."
	ti.CharLimit = 1000

	vp := viewport.New(100, 20)
	vp.SetContent("Welcome to NOllama!")

	spring := harmonica.NewSpring(1.0/60.0, 5.0, 0.4)
	chart := streamlinechart.New(40, 6)
	zone := bz.New()

	m := &Model{
		state:        stateURLList,
		spinner:      s,
		input:        ti,
		urlInput:     ui,
		bulkURLInput: bulkUI,
		fileInput:    fileUI,
		viewport:     vp,
		spring:       spring,
		tpsChart:     chart,
		zone:         zone,
	}

	m.multiClient = api.NewMultiClient(nil)
	if initialURL != "" {
		m.multiClient.AddURL(normalizeURL(initialURL))
	}

	m.list = list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	m.list.Styles.Title = TitleStyle

	return m
}

var httpPrefix = regexp.MustCompile(`^https?://`)

func normalizeURL(input string) string {
	input = strings.TrimSpace(input)
	input = strings.TrimSuffix(input, "/")

	if input == "" {
		return "http://localhost:11434"
	}

	if httpPrefix.MatchString(input) {
		u := strings.TrimPrefix(input, "http://")
		u = strings.TrimPrefix(u, "https://")
		if !strings.Contains(u, ":") {
			u = u + ":11434"
		}
		return "http://" + u
	}

	if strings.Contains(input, ":") {
		return "http://" + input
	}

	return "http://" + input + ":11434"
}

func (m *Model) SetProgram(p *tea.Program) {
	m.program = p
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.tickInterpolation())
}

func (m *Model) loadModels() tea.Cmd {
	return func() tea.Msg {
		models, err := m.client.ListModels()
		if err != nil {
			return errorMsg{err, true}
		}
		return modelsMsg{models}
	}
}

func (m *Model) loadAllModels() tea.Cmd {
	return func() tea.Msg {
		models := m.multiClient.ListAllModels()
		return modelsMsg{models}
	}
}

func (m *Model) loadURLsFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			if strings.Contains(line, ",") {
				for _, url := range strings.Split(line, ",") {
					url = strings.TrimSpace(url)
					if url != "" {
						m.multiClient.AddURL(url)
					}
				}
			} else {
				m.multiClient.AddURL(line)
			}
		}
	}
	return nil
}

type modelsMsg struct{ models []api.Model }
type errorMsg struct {
	err      error
	softFail bool
}
type partialMsg struct {
	content string
	done    bool
	tps     float64
	total   int
}
type tickMsg time.Time
type statsMsg struct{ stats map[string]*api.APIStats }

func (m *Model) tickInterpolation() tea.Cmd {
	return tea.Every(time.Second/60, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.MouseMsg:
		if m.state == stateChat {
			if m.zone.Get("back_btn").InBounds(msg) && msg.Action == tea.MouseActionRelease {
				m.state = stateMultiModelView
				return m, nil
			}
		}

	case tea.KeyMsg:
		if m.notification != "" && time.Since(m.notifTime) > 3*time.Second {
			m.notification = ""
		}
		if strings.HasPrefix(msg.String(), "ctrl+shift") ||
			strings.HasPrefix(msg.String(), "alt") ||
			msg.String() == "f1" || msg.String() == "f2" || msg.String() == "f3" ||
			msg.String() == "f4" || msg.String() == "f5" || msg.String() == "f6" ||
			msg.String() == "f7" || msg.String() == "f8" || msg.String() == "f9" ||
			msg.String() == "f10" || msg.String() == "f11" || msg.String() == "f12" {
			return m, nil
		}
		switch msg.String() {
		case "ctrl+c", "q":
			if (!m.input.Focused() && !m.urlInput.Focused() && !m.bulkURLInput.Focused() && !m.fileInput.Focused()) || msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
		case "esc":
			if m.state == stateChat {
				m.state = stateMultiModelView
			} else if m.state == stateMultiModelView || m.state == stateAPITags {
				m.state = stateURLList
				m.list.Title = "Configured Ollama URLs"
			} else if m.state == stateBulkURLInput {
				m.state = stateURLList
				m.bulkURLInput.SetValue("")
			} else if m.state == stateFileInput {
				m.state = stateURLList
				m.fileInput.SetValue("")
			} else if m.state == stateURLInput {
				m.state = stateURLList
				m.urlInput.SetValue("")
			}
		case "enter":
			if m.state == stateError {
				m.state = stateURLList
				m.urlInput.Focus()
				m.err = nil
				return m, nil
			}
			if m.state == stateURLInput {
				input := strings.TrimSpace(m.urlInput.Value())
				if input != "" {
					url := normalizeURL(input)
					m.multiClient.AddURL(url)
					m.updateURLList()
				}
				m.urlInput.SetValue("")
				m.state = stateURLList
			} else if m.state == stateBulkURLInput {
				urls := m.bulkURLInput.Value()
				if strings.Contains(urls, "\n") {
					for _, url := range strings.Split(urls, "\n") {
						url = strings.TrimSpace(url)
						if url != "" && !strings.HasPrefix(url, "#") {
							m.multiClient.AddURL(normalizeURL(url))
						}
					}
				} else if strings.Contains(urls, ",") {
					for _, url := range strings.Split(urls, ",") {
						url = strings.TrimSpace(url)
						if url != "" {
							m.multiClient.AddURL(normalizeURL(url))
						}
					}
				} else {
					url := strings.TrimSpace(urls)
					if url != "" {
						m.multiClient.AddURL(normalizeURL(url))
					}
				}
				m.bulkURLInput.SetValue("")
				m.updateURLList()
				m.state = stateURLList
			} else if m.state == stateFileInput {
				path := m.fileInput.Value()
				if err := m.loadURLsFromFile(path); err != nil {
					m.notification = fmt.Sprintf("Failed to load file: %v", err)
					m.notifTime = time.Now()
				}
				m.fileInput.SetValue("")
				m.updateURLList()
				m.state = stateURLList
			} else if m.state == stateURLList {
				it, ok := m.list.SelectedItem().(urlItem)
				if ok && it.url != "" {
					m.client = api.NewClient(it.url)
					m.pendingState = stateModelSelection
					m.state = stateLoading
					return m, m.loadModels()
				}
			} else if m.state == stateMultiModelView {
				it, ok := m.list.SelectedItem().(modelItem)
				if ok && it.name != "" {
					m.selectedMd = it.name
					m.selectedURL = it.url
					m.client = api.NewClient(it.url)
					m.state = stateChat
					m.viewport.SetContent(fmt.Sprintf("\n  Connected: %s\n  Model: %s\n  Ask me anything!\n\n", it.url, it.name))
					m.messages = nil
					m.targetVal = 0
					m.input.Focus()
				}
			} else if m.state == stateChat && m.input.Value() != "" && !m.streaming {
				input := m.input.Value()
				m.messages = append(m.messages, api.Message{Role: "user", Content: input})
				m.input.SetValue("")
				m.streaming = true
				m.currentRes = ""
				m.startTime = time.Now()
				m.tpsChart = streamlinechart.New(m.width-6, 5)
				m.targetVal = 100
				cmds = append(cmds, m.sendChatCmd())
			}
		case "d":
			if m.state == stateURLList {
				it, ok := m.list.SelectedItem().(urlItem)
				if ok {
					m.multiClient.RemoveURL(it.url)
					m.updateURLList()
				}
			}
		case "a":
			if m.state == stateURLList {
				m.urlInput.SetValue("")
				m.state = stateURLInput
				m.urlInput.Focus()
			}
		case "b":
			if m.state == stateURLList {
				m.bulkURLInput.SetValue("")
				m.state = stateBulkURLInput
				m.bulkURLInput.Focus()
			}
		case "f":
			if m.state == stateURLList {
				m.fileInput.SetValue("")
				m.state = stateFileInput
				m.fileInput.Focus()
			}
		case "m":
			if m.state == stateURLList {
				m.pendingState = stateMultiModelView
				m.state = stateLoading
				return m, m.loadAllModels()
			}
		case "s":
			if m.state == stateURLList || m.state == stateMultiModelView || m.state == stateAPITags {
				m.state = stateLoading
				return m, m.loadStats()
			}
		case "v":
			if m.state == stateURLList {
				m.state = stateAPITags
			}
		}

	case tickMsg:
		nv, _ := m.spring.Update(m.interpolVal, 0.0, m.targetVal)
		m.interpolVal = nv
		cmds = append(cmds, m.tickInterpolation())

	case modelsMsg:
		m.models = msg.models
		if m.state == stateLoading {
			if m.pendingState != 0 {
				m.state = m.pendingState
				m.pendingState = 0
			} else {
				m.state = stateModelSelection
			}
		}
		items := []list.Item{}
		if len(m.models) > 0 {
			items = make([]list.Item, len(m.models))
			for i, mod := range m.models {
				items[i] = modelItem{name: mod.Name, size: mod.Size, url: mod.URL}
			}
		}
		m.list.SetItems(items)
		if m.state == stateMultiModelView {
			m.list.Title = "All Ollama Models"
		} else {
			m.list.Title = "Ollama Models"
		}
		if m.state == stateModelSelection || m.state == stateMultiModelView {
			m.updateSize()
		}

	case statsMsg:
		m.state = stateAPITags
		m.updateStatsView(msg.stats)
		m.updateSize()

	case partialMsg:
		if msg.content != "" {
			m.currentRes += msg.content
			m.lastTps = msg.tps
			m.totalTokens = msg.total
			m.tpsChart.Push(msg.tps)
		}
		if msg.done {
			m.messages = append(m.messages, api.Message{Role: "assistant", Content: m.currentRes})
			m.streaming = false
			m.targetVal = 0
		}
		m.viewport.SetContent(m.renderChat())
		m.viewport.GotoBottom()

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateSize()

	case errorMsg:
		m.err = msg.err
		if msg.softFail {
			m.state = stateURLList
			m.err = nil
			m.notification = fmt.Sprintf("Connection failed: %v", msg.err)
			m.notifTime = time.Now()
			m.updateURLList()
		} else {
			m.state = stateError
		}

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	switch m.state {
	case stateURLInput:
		m.urlInput, cmd = m.urlInput.Update(msg)
		cmds = append(cmds, cmd)
	case stateBulkURLInput:
		m.bulkURLInput, cmd = m.bulkURLInput.Update(msg)
		cmds = append(cmds, cmd)
	case stateFileInput:
		m.fileInput, cmd = m.fileInput.Update(msg)
		cmds = append(cmds, cmd)
	case stateURLList, stateModelSelection, stateMultiModelView, stateAPITags:
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
	case stateChat:
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) updateURLList() {
	urls := m.multiClient.GetURLs()
	items := make([]list.Item, len(urls))
	for i, url := range urls {
		items[i] = urlItem{url: url}
	}
	m.list.SetItems(items)
	m.list.Title = "Configured Ollama URLs"
}

func (m *Model) updateStatsView(stats map[string]*api.APIStats) {
	var items []list.Item
	for url, stat := range stats {
		if stat.Error != nil {
			items = append(items, urlItem{url: fmt.Sprintf("%s - ERROR: %v", url, stat.Error)})
		} else {
			items = append(items, urlItem{url: fmt.Sprintf("%s | Models: %d | Size: %.2f GB",
				url, stat.ModelCount, float64(stat.TotalSize)/(1024*1024*1024))})
		}
	}
	m.list.SetItems(items)
	m.list.Title = "API Statistics"
}

func (m *Model) loadStats() tea.Cmd {
	return func() tea.Msg {
		m.multiClient.ListAllModels()
		stats := m.multiClient.GetStats()
		return statsMsg{stats}
	}
}

func (m *Model) updateSize() {
	switch m.state {
	case stateURLList, stateAPITags:
		m.list.SetSize(m.width-4, m.height-10)
	case stateModelSelection, stateMultiModelView:
		m.list.SetSize(m.width/3, m.height-14)
	case stateChat:
		m.viewport.Width = m.width - 2
		m.viewport.Height = m.height - 20
		m.input.Width = m.width - 10
		m.urlInput.Width = m.width - 20
		m.tpsChart.Resize(m.width-6, 5)
	}
}

func (m *Model) sendChatCmd() tea.Cmd {
	return func() tea.Msg {
		ch, errCh, err := m.client.ChatStream(api.ChatRequest{
			Model:    m.selectedMd,
			Messages: m.messages,
		})
		if err != nil {
			return errorMsg{err, true}
		}

		go func() {
			for partial := range ch {
				duration := time.Since(m.startTime).Seconds()
				tps := 0.0
				if duration > 0 {
					tps = float64(partial.EvalCount) / duration
				}
				if m.program != nil {
					m.program.Send(partialMsg{
						content: partial.Message.Content,
						done:    partial.Done,
						tps:     tps,
						total:   partial.EvalCount,
					})
				}
			}
			for err := range errCh {
				if m.program != nil {
					m.program.Send(errorMsg{err, true})
				}
			}
		}()
		return nil
	}
}

func (m *Model) renderChat() string {
	var b strings.Builder
	for _, msg := range m.messages {
		if msg.Role == "user" {
			b.WriteString(ChatUserStyle.Render("\n  USER:\n"))
		} else {
			b.WriteString(ChatBotStyle.Render("\n  BOT:\n"))
		}
		b.WriteString(MessageContentStyle.Render("  " + msg.Content))
		b.WriteString("\n")
	}
	if m.streaming {
		b.WriteString(ChatBotStyle.Render("\n  BOT:\n"))
		b.WriteString(MessageContentStyle.Render("  " + m.currentRes))
		b.WriteString(" " + m.spinner.View())
	}
	return b.String()
}

func (m *Model) View() string {
	if m.err != nil && m.state != stateURLList && m.state != stateMultiModelView && m.state != stateModelSelection {
		errBox := ErrorStyle.Render(
			fmt.Sprintf("  ERROR  \n\n  %v\n", m.err),
		)
		help := HelpStyle.Render("\n  [Enter] Try again  |  [q] Quit")
		return lipgloss.JoinVertical(lipgloss.Center, "\n", errBox, help)
	}

	m.err = nil

	switch m.state {
	case stateURLInput:
		header := FancyHeader.Render("  ⊕  ADD OLLAMA ENDPOINT  ")
		prompt := PromptStyle.Render("\n  Enter endpoint URL or IP:port")
		input := "\n  " + m.urlInput.View()
		help := HelpStyle.Render("\n  [Enter] Add  |  [Esc] Cancel")
		return lipgloss.JoinVertical(lipgloss.Left, header, prompt, input, help)

	case stateBulkURLInput:
		header := FancyHeader.Render("  ⊕  BULK ADD ENDPOINTS  ")
		prompt := PromptStyle.Render("\n  Enter URLs (one per line or comma-separated):")
		input := "\n  " + m.bulkURLInput.View()
		help := HelpStyle.Render("\n  [Enter] Add All  |  [Esc] Cancel")
		return lipgloss.JoinVertical(lipgloss.Left, header, prompt, input, help)

	case stateFileInput:
		header := FancyHeader.Render("  ⊕  LOAD FROM FILE  ")
		prompt := PromptStyle.Render("\n  Enter path to file containing URLs:")
		input := "\n  " + m.fileInput.View()
		help := HelpStyle.Render("\n  [Enter] Load  |  [Esc] Cancel")
		return lipgloss.JoinVertical(lipgloss.Left, header, prompt, input, help)

	case stateLoading:
		loadingContent := lipgloss.JoinVertical(lipgloss.Center,
			"\n\n",
			TitleStyle.Render(" "+m.spinner.View()+" "),
			"\n",
			InfoLabelStyle.Render(" Connecting to Ollama... "),
		)
		return BaseStyle.Render(loadingContent)

	case stateURLList:
		urls := m.multiClient.GetURLs()
		var statusBar string
		if m.notification != "" {
			statusBar = WarningStyle.Render("  " + m.notification + "  ")
		} else if len(urls) == 0 {
			statusBar = WarningStyle.Render("  No endpoints - press [a] to add  ")
		} else {
			statusBar = SuccessStyle.Render(fmt.Sprintf("  ● %d endpoint(s) ready  ", len(urls)))
		}

		header := FancyHeader.Render("  ⚡ NOLLAMA  ")
		subheader := SubtleStyle.Render("  Multi-Endpoint LLM Client  ")

		help := lipgloss.JoinHorizontal(lipgloss.Center,
			AccentStyle.Render(" [Enter] Connect "),
			AccentStyle.Render(" [a] Add "),
			AccentStyle.Render(" [b] Bulk "),
			AccentStyle.Render(" [f] File "),
			AccentStyle.Render(" [d] Del "),
			AccentStyle.Render(" [m] All Models "),
			AccentStyle.Render(" [s] Stats "),
		)

		listView := m.list.View()

		footer := lipgloss.JoinVertical(lipgloss.Left,
			"\n",
			HelpStyle.Render(help),
		)

		return lipgloss.JoinVertical(lipgloss.Left,
			header,
			subheader,
			"\n"+statusBar+"\n",
			listView,
			footer,
		)

	case stateModelSelection:
		header := FancyHeader.Render(fmt.Sprintf("  ◈ MODELS @ %s  ", m.client.BaseURL))
		help := HelpStyle.Render("\n  [Enter] Select  |  [Esc] Back  |  [q] Quit")
		return lipgloss.JoinVertical(lipgloss.Left, header, "\n", m.list.View(), help)

	case stateMultiModelView:
		header := FancyHeader.Render("  ◈ ALL AVAILABLE MODELS  ")
		help := HelpStyle.Render("\n  [Enter] Select Model  |  [Esc] Back  |  [q] Quit")
		return lipgloss.JoinVertical(lipgloss.Left, header, "\n", m.list.View(), help)

	case stateAPITags:
		header := FancyHeader.Render("  ◉ API STATISTICS  ")
		stats := m.multiClient.GetStats()
		totalModels := 0
		totalSize := int64(0)
		successCount := 0
		errorCount := 0
		for _, s := range stats {
			if s.Error == nil {
				totalModels += s.ModelCount
				totalSize += s.TotalSize
				successCount++
			} else {
				errorCount++
			}
		}

		var summary string
		if errorCount > 0 {
			summary = WarningStyle.Render(fmt.Sprintf("  ⚠ %d/%d endpoints failed  ", errorCount, len(stats)))
		} else if successCount > 0 {
			summary = SuccessStyle.Render(fmt.Sprintf("  ● All %d endpoints healthy  ", successCount))
		}

		statsSummary := InfoLabelStyle.Render(fmt.Sprintf("  Total: %d models | %.2f GB  ", totalModels, float64(totalSize)/(1024*1024*1024)))

		help := HelpStyle.Render("\n  [Enter] Refresh  |  [Esc] Back  |  [q] Quit")
		return lipgloss.JoinVertical(lipgloss.Left,
			header,
			"\n"+summary,
			statsSummary+"\n",
			m.list.View(),
			help,
		)

	case stateChat:
		backBtn := m.zone.Mark("back_btn",
			lipgloss.NewStyle().
				Foreground(AccentColor).
				Bold(true).
				Render(" ◀ Models "),
		)

		connection := SubtleStyle.Render(m.selectedURL + " › " + m.selectedMd)
		header := lipgloss.JoinHorizontal(lipgloss.Center,
			backBtn,
			"  ",
			FancyHeader.Render("  ⚡ CHAT  "),
		)
		header = lipgloss.JoinVertical(lipgloss.Left, header, connection)

		vp := ContainerStyle.Render(m.viewport.View())

		progressWidth := int(m.interpolVal * float64(m.width-4) / 100)
		if progressWidth > m.width-4 {
			progressWidth = m.width - 4
		}
		if progressWidth < 0 {
			progressWidth = 0
		}
		bar := lipgloss.NewStyle().Background(PrimaryColor).Foreground(WhiteColor).Width(progressWidth).Render(" ")
		empty := lipgloss.NewStyle().Background(DeepGrayColor).Width(m.width - 4 - progressWidth).Render(" ")
		progressBar := "\n  " + bar + empty + "\n"

		input := "\n  " + m.input.View()

		stats := ""
		if m.streaming || m.lastTps > 0 {
			stats = ChartBoxStyle.Render(
				fmt.Sprintf("  ⏱ %.2f tok/s  |  %d tokens  ", m.lastTps, m.totalTokens) + "\n" +
					m.tpsChart.View(),
			)
		}

		help := HelpStyle.Render("  [Esc] Back  |  [Enter] Send  |  [Ctrl+C] Exit")

		v := lipgloss.JoinVertical(lipgloss.Left, header, vp, progressBar, input, stats, help)
		return m.zone.Scan(v)
	}
	return ""
}
