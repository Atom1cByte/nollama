package tui

import (
	"fmt"
	"strings"
	"time"

	"cli/api"

	"github.com/NimbleMarkets/ntcharts/linechart/streamlinechart"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/harmonica"
	"github.com/charmbracelet/lipgloss"
	bz "github.com/lrstanley/bubblezone"
)

type ModelsState int

const (
	stateURLList ModelsState = iota
	stateURLInput
	stateBulkURLInput
	stateFileInput
	stateStats
	stateAllModels
	stateModelSelection
	stateLoading
	stateChat
	stateError
	stateRouterLoad
	stateRouterModels
	stateRouterSearch
)

type Model struct {
	multiClient *api.MultiClient
	state       ModelsState

	// Data
	currentErr error
	models     []api.Model
	stats      []api.EndpointStat
	selectedMd api.Model
	messages   []api.Message

	// Router data
	routerModels        []api.RouterModel
	selectedRouterModel api.RouterModel

	// UI Components
	urlList    list.Model
	modelList  list.Model
	statsTable table.Model

	singleInput textinput.Model // Used for single URL, or file path
	bulkInput   textinput.Model // (Since textinput is single line, maybe just split by comma?)

	chatInput textinput.Model
	viewport  viewport.Model
	spinner   spinner.Model

	// Chat specific visual
	width      int
	height     int
	streaming  bool
	currentRes string

	program *tea.Program
	zone    *bz.Manager

	tpsChart    streamlinechart.Model
	startTime   time.Time
	lastTps     float64
	totalTokens int

	spring      harmonica.Spring
	interpolVal float64
	targetVal   float64

	// Router mode
	routerLoaded bool
}

// Structs for list items
type urlItem struct{ url string }

func (i urlItem) Title() string       { return i.url }
func (i urlItem) Description() string { return "Ollama Endpoint" }
func (i urlItem) FilterValue() string { return i.url }

type modelItem struct{ mod api.Model }

func (i modelItem) Title() string { return i.mod.Name }
func (i modelItem) Description() string {
	return fmt.Sprintf("%s | %.2f GB", i.mod.Endpoint, float64(i.mod.Size)/(1024*1024*1024))
}
func (i modelItem) FilterValue() string { return i.mod.Name }

type routerModelItem struct{ mod api.RouterModel }

func (i routerModelItem) Title() string { return i.mod.Name }
func (i routerModelItem) Description() string {
	return fmt.Sprintf("%d servers available", i.mod.ServerCount)
}
func (i routerModelItem) FilterValue() string { return i.mod.Name }

func NewModel() *Model {
	mClient := api.NewMultiClient()

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(PrimaryColor)

	ui := textinput.New()
	ui.Placeholder = "Enter URL (e.g. localhost:11434) or path"
	ti := textinput.New()
	ti.Placeholder = "Type a message..."
	ti.CharLimit = 1000

	vp := viewport.New(100, 20)
	vp.SetContent("Welcome to NOllama CLI!")

	spring := harmonica.NewSpring(1.0/60.0, 5.0, 0.4)
	chart := streamlinechart.New(40, 6)
	zone := bz.New()

	ul := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	ul.Title = "Ollama Endpoints"
	ul.Styles.Title = TitleStyle

	ml := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	ml.Title = "Available Models"
	ml.Styles.Title = TitleStyle

	columns := []table.Column{
		{Title: "Endpoint", Width: 35},
		{Title: "Models", Width: 10},
		{Title: "Size (GB)", Width: 15},
		{Title: "Status", Width: 20},
	}
	st := table.New(table.WithColumns(columns), table.WithFocused(true), table.WithHeight(10))

	return &Model{
		multiClient: mClient,
		state:       stateURLList,
		spinner:     s,
		urlList:     ul,
		modelList:   ml,
		statsTable:  st,
		singleInput: ui,
		chatInput:   ti,
		viewport:    vp,
		spring:      spring,
		tpsChart:    chart,
		zone:        zone,
	}
}

func (m *Model) SetProgram(p *tea.Program) {
	m.program = p
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.tickInterpolation())
}

// Msgs
type modelsMsg struct{ models []api.Model }
type statsMsg struct{ stats []api.EndpointStat }
type routerModelsMsg struct{ models []api.RouterModel }
type routerStatusMsg struct{ stats *api.RouterStats }
type routerLoadMsg struct{ err error }
type errorMsg struct{ err error }
type partialMsg struct {
	content string
	done    bool
	tps     float64
	total   int
}
type tickMsg time.Time

func (m *Model) tickInterpolation() tea.Cmd {
	return tea.Every(time.Second/60, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if !m.singleInput.Focused() && !m.chatInput.Focused() {
				return m, tea.Quit
			}
		case "esc":
			switch m.state {
			case stateURLInput, stateBulkURLInput, stateFileInput:
				m.singleInput.Blur()
				m.singleInput.SetValue("")
				m.state = stateURLList
			case stateStats, stateAllModels, stateModelSelection:
				m.state = stateURLList
			case stateChat:
				m.state = stateModelSelection
			case stateError:
				m.state = stateURLList
			}

		case "a":
			if m.state == stateURLList {
				m.state = stateURLInput
				m.singleInput.Placeholder = "Enter a single URL..."
				m.singleInput.Focus()
				return m, nil
			}
		case "b":
			if m.state == stateURLList {
				m.state = stateBulkURLInput
				m.singleInput.Placeholder = "Comma-separated URLs..."
				m.singleInput.Focus()
				return m, nil
			}
		case "f":
			if m.state == stateURLList {
				m.state = stateFileInput
				m.singleInput.Placeholder = "Path to file..."
				m.singleInput.Focus()
				return m, nil
			}
		case "d":
			if m.state == stateURLList {
				it, ok := m.urlList.SelectedItem().(urlItem)
				if ok {
					m.multiClient.RemoveURL(it.url)
					m.syncURLList()
				}
				return m, nil
			}
		case "m":
			if m.state == stateURLList {
				m.state = stateLoading
				return m, func() tea.Msg {
					mods := m.multiClient.ListAllModels()
					return modelsMsg{mods}
				}
			}
		case "s", "v":
			if m.state == stateURLList && !m.singleInput.Focused() && !m.chatInput.Focused() {
				m.state = stateLoading
				return m, func() tea.Msg {
					st := m.multiClient.GetStats()
					return statsMsg{st}
				}
			}

		// Router keys
		case "r":
			if m.state == stateURLList {
				m.state = stateRouterLoad
				return m, func() tea.Msg {
					err := m.multiClient.LoadRouterScan(true)
					if err != nil {
						return routerLoadMsg{err}
					}
					stats, err := m.multiClient.GetRouterStatus()
					if err != nil {
						return routerLoadMsg{err}
					}
					return routerStatusMsg{stats}
				}
			}
		case "l":
			if m.state == stateURLList && m.routerLoaded {
				m.state = stateLoading
				return m, func() tea.Msg {
					mods, err := m.multiClient.ListRouterModels("")
					if err != nil {
						return errorMsg{err}
					}
					return routerModelsMsg{mods}
				}
			}

		case "enter":
			if m.state == stateError {
				m.state = stateURLList
				m.currentErr = nil
				return m, nil
			} else if m.state == stateURLInput {
				v := m.singleInput.Value()
				if v != "" {
					m.multiClient.AddURL(v)
				}
				m.singleInput.SetValue("")
				m.singleInput.Blur()
				m.syncURLList()
				m.state = stateURLList
			} else if m.state == stateBulkURLInput {
				m.multiClient.BulkAdd(m.singleInput.Value())
				m.singleInput.SetValue("")
				m.singleInput.Blur()
				m.syncURLList()
				m.state = stateURLList
			} else if m.state == stateFileInput {
				_, err := m.multiClient.LoadFromFile(m.singleInput.Value())
				m.singleInput.SetValue("")
				m.singleInput.Blur()
				if err != nil {
					m.currentErr = err
					m.state = stateError
					return m, nil
				}
				m.syncURLList()
				m.state = stateURLList
			} else if m.state == stateURLList {
				it, ok := m.urlList.SelectedItem().(urlItem)
				if ok {
					m.state = stateLoading
					return m, func() tea.Msg {
						c := m.multiClient.GetClientForModel(it.url)
						mods, err := c.ListModels()
						if err != nil {
							return errorMsg{err}
						}
						return modelsMsg{mods}
					}
				}
			} else if m.state == stateModelSelection || m.state == stateAllModels {
				it, ok := m.modelList.SelectedItem().(modelItem)
				if ok {
					m.selectedMd = it.mod
					m.selectedRouterModel = api.RouterModel{} // Clear router model
					m.state = stateChat
					m.viewport.SetContent(fmt.Sprintf("\n  Connected: %s\n  Model: %s\n  Ask me anything!\n\n", m.selectedMd.Endpoint, m.selectedMd.Name))
					m.messages = nil
					m.targetVal = 0
					m.chatInput.Focus()
				}
			} else if m.state == stateRouterModels {
				it, ok := m.modelList.SelectedItem().(routerModelItem)
				if ok {
					m.selectedRouterModel = it.mod
					m.selectedMd = api.Model{} // Clear direct model
					m.state = stateChat
					m.viewport.SetContent(fmt.Sprintf("\n  Router Mode\n  Model: %s\n  Servers: %d\n  Ask me anything!\n\n", it.mod.Name, it.mod.ServerCount))
					m.messages = nil
					m.targetVal = 0
					m.chatInput.Focus()
				}
			} else if m.state == stateChat && m.chatInput.Value() != "" && !m.streaming {
				input := m.chatInput.Value()
				m.messages = append(m.messages, api.Message{Role: "user", Content: input})
				m.chatInput.SetValue("")
				m.streaming = true
				m.currentRes = ""
				m.startTime = time.Now()
				m.tpsChart = streamlinechart.New(m.width-6, 5)
				m.targetVal = 100
				return m, m.sendChatCmd()
			}
		}

	case tickMsg:
		nv, _ := m.spring.Update(m.interpolVal, 0.0, m.targetVal)
		m.interpolVal = nv
		cmds = append(cmds, m.tickInterpolation())

	case modelsMsg:
		m.models = msg.models
		items := make([]list.Item, len(m.models))
		for i, mod := range m.models {
			items[i] = modelItem{mod}
		}
		m.modelList.SetItems(items)
		if m.state == stateLoading {
			m.state = stateModelSelection
		}
		m.updateSize()

	case statsMsg:
		m.stats = msg.stats
		var rows []table.Row
		for _, s := range m.stats {
			status := "OK"
			if s.Error != nil {
				status = "ERR: " + s.Error.Error()
			}
			rows = append(rows, table.Row{
				s.URL,
				fmt.Sprintf("%d", s.ModelCount),
				fmt.Sprintf("%.2f", float64(s.TotalSize)/(1024*1024*1024)),
				status,
			})
		}
		m.statsTable.SetRows(rows)
		m.state = stateStats

	case routerLoadMsg:
		m.currentErr = msg.err
		m.state = stateError

	case routerStatusMsg:
		m.routerLoaded = true
		m.state = stateURLList

	case routerModelsMsg:
		m.routerModels = msg.models
		items := make([]list.Item, len(m.routerModels))
		for i, mod := range m.routerModels {
			items[i] = routerModelItem{mod}
		}
		m.modelList.SetItems(items)
		m.state = stateRouterModels
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
		m.currentErr = msg.err
		m.state = stateError

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	switch m.state {
	case stateURLInput, stateBulkURLInput, stateFileInput:
		m.singleInput, cmd = m.singleInput.Update(msg)
		cmds = append(cmds, cmd)
	case stateURLList:
		m.urlList, cmd = m.urlList.Update(msg)
		cmds = append(cmds, cmd)
	case stateModelSelection, stateAllModels:
		m.modelList, cmd = m.modelList.Update(msg)
		cmds = append(cmds, cmd)
	case stateStats:
		m.statsTable, cmd = m.statsTable.Update(msg)
		cmds = append(cmds, cmd)
	case stateChat:
		m.chatInput, cmd = m.chatInput.Update(msg)
		cmds = append(cmds, cmd)
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) syncURLList() {
	items := make([]list.Item, len(m.multiClient.Endpoints))
	for i, u := range m.multiClient.Endpoints {
		items[i] = urlItem{u}
	}
	m.urlList.SetItems(items)
}

func (m *Model) updateSize() {
	h := m.height - 10
	if h < 5 {
		h = 5
	}
	w := m.width - 2
	if w < 10 {
		w = 10
	}
	m.urlList.SetSize(w/2, h)
	m.modelList.SetSize(w/2, h)
	m.viewport.Width = w
	m.viewport.Height = h - 10
	m.singleInput.Width = w
	m.chatInput.Width = w

	// Recreate table with new width logic if needed, simplify here
	tableWidth := w - 4
	if tableWidth > 0 {
		m.tpsChart.Resize(tableWidth, 5)
	}
}

func (m *Model) sendChatCmd() tea.Cmd {
	// Check if we're using a router model
	if m.selectedRouterModel.Name != "" {
		return m.sendRouterChatCmd()
	}

	// Direct mode
	return func() tea.Msg {
		c := m.multiClient.GetClientForModel(m.selectedMd.Endpoint)
		if c == nil {
			return errorMsg{fmt.Errorf("No client for endpoint")}
		}
		ch, errCh, err := c.ChatStream(api.ChatRequest{
			Model:    m.selectedMd.Name,
			Messages: m.messages,
		})
		if err != nil {
			return errorMsg{err}
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
					m.program.Send(errorMsg{err})
				}
			}
		}()
		return nil
	}
}

func (m *Model) sendRouterChatCmd() tea.Cmd {
	return func() tea.Msg {
		ch, errCh, err := m.multiClient.Router.ChatStream(
			m.selectedRouterModel.Name,
			m.messages,
		)
		if err != nil {
			return errorMsg{err}
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
					m.program.Send(errorMsg{err})
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
	if m.state == stateError {
		return fmt.Sprintf("\n  ERROR:\n  %v\n\n  [Enter] Try again | [q] Quit", m.currentErr)
	}

	helpKeyBindings := HelpStyle.Render("\n  [a] Add URL | [b] Bulk Add | [f] Load File | [d] Delete | [m] All Models | [s] Stats | [Esc] Back")

	if m.routerLoaded {
		helpKeyBindings = HelpStyle.Render("\n  [a] Add URL | [b] Bulk Add | [f] Load File | [d] Delete | [m] All Models | [s] Stats | [r] Refresh Router | [l] List Router Models | [Esc] Back")
	}

	switch m.state {
	case stateURLList:
		if m.routerLoaded {
			return BaseStyle.Render(m.urlList.View() + "\n" + helpKeyBindings + "\n\n  " + SuccessStatusStyle.Render("✓ Router loaded - press [l] to browse models from 1000+ servers"))
		}
		return BaseStyle.Render(m.urlList.View() + "\n" + helpKeyBindings)

	case stateURLInput, stateBulkURLInput, stateFileInput:
		header := TitleStyle.Render(" NOLLAMA | URL CONFIGURATION ")
		prompt := "\n  " + m.singleInput.Placeholder + "\n"
		input := "\n  " + m.singleInput.View()
		help := HelpStyle.Render("\n\n  [Enter] Submit | [Esc] Cancel")
		return BaseStyle.Render(lipgloss.JoinVertical(lipgloss.Left, header, prompt, input, help))

	case stateRouterLoad:
		return "\n\n  " + m.spinner.View() + " Loading router (1000+ servers)... this may take a moment..."

	case stateLoading:
		return "\n\n  " + m.spinner.View() + " Communicating with endpoints..."

	case stateModelSelection, stateAllModels:
		return BaseStyle.Render(m.modelList.View() + "\n" + HelpStyle.Render("\n  [Enter] Chat | [Esc] Back"))

	case stateRouterModels:
		header := TitleStyle.Render(" ROUTER MODELS (1000+ servers) ")
		return BaseStyle.Render(lipgloss.JoinVertical(lipgloss.Left, header, m.modelList.View(), HelpStyle.Render("\n  [Enter] Chat | [l] Refresh | [Esc] Back")))

	case stateStats:
		header := TitleStyle.Render(" API ENDPOINT STATISTICS ")
		tableRender := m.statsTable.View()
		return BaseStyle.Render(lipgloss.JoinVertical(lipgloss.Left, header, tableRender, HelpStyle.Render("\n  [Esc] Back to List")))

	case stateChat:
		backBtn := m.zone.Mark("back", lipgloss.NewStyle().Foreground(SecondaryColor).Bold(true).Render(" ← Back "))

		var header string
		if m.selectedRouterModel.Name != "" {
			header = lipgloss.JoinHorizontal(lipgloss.Center, backBtn, "  ", TitleStyle.Render(" NOLLAMA | ROUTER | "+m.selectedRouterModel.Name))
		} else {
			header = lipgloss.JoinHorizontal(lipgloss.Center, backBtn, "  ", TitleStyle.Render(" NOLLAMA | "+m.selectedMd.Name+" @ "+m.selectedMd.Endpoint))
		}

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

		input := "\n  " + m.chatInput.View()

		stats := ""
		if m.streaming || m.lastTps > 0 {
			stats = ChartBoxStyle.Render(
				fmt.Sprintf(" Performance: %.2f tokens/s | Total: %d tokens\n", m.lastTps, m.totalTokens) +
					m.tpsChart.View(),
			)
		}

		help := HelpStyle.Render("\n  [Esc] Back | [Enter] Chat | [Ctrl+C] Exit")
		v := lipgloss.JoinVertical(lipgloss.Left, header, vp, progressBar, input, stats, help)
		return m.zone.Scan(v)
	}
	return ""
}
