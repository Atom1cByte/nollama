# NOllama

NOllama is a highly interactive, premium Terminal User Interface (TUI) client for the [Ollama](https://ollama.com/) REST API. Built entirely with Go and the [Charmbracelet](https://charm.sh/) ecosystem, it provides a seamless, aesthetically pleasing, and highly responsive experience for interacting with local or remote Large Language Models (LLMs).

## ✨ Features

- **Interactive TUI**: Built with Bubble Tea, featuring multiple states (URL Configuration, Loading, Model Selection, and Chat).
- **Multi-URL Support**: Manage multiple Ollama endpoints simultaneously
  - Add URLs one at a time
  - Bulk add URLs (comma-separated or new-line separated)
  - Load URLs from a file
- **Aggregated Model View**: View all models from all configured Ollama instances in one place
- **API Statistics**: View statistics for all configured endpoints (model count, total size, connection status)
- **Customizable Connection**: Manually specify the Ollama API base URL directly from the interface.
- **Real-Time Data Streaming**: Seamlessly streams text generation responses chunk-by-chunk using Go channels.
- **Performance Visualization**: Integrated live streamline chart displaying generation speed in Tokens per Second (TPS), powered by `ntcharts`.
- **Fluid Animations**: 60fps physics-based spring animations for progress visualizations, powered by `harmonica`.
- **Mouse Support**: Navigate the interface (e.g., clicking "Back to List") using your mouse, thanks to `bubblezone`.
- **Premium Design System**: Tailored color palettes, distinct text styles, and rounded borders implemented via `lipgloss`.

## 📸 Overview of the Layout

1. **URL List**: Manage multiple Ollama endpoints with options to add, remove, bulk add, or load from file
2. **Model Selection**: A cleanly formatted list of all pulled models retrieved from your Ollama instance.
3. **All Models View**: Combined view of models from all configured Ollama endpoints
4. **API Statistics**: Overview of all endpoints with model counts and total sizes
5. **Chat Interface**: An immersive chat window featuring:
   - Chat history.
   - Text input box for user prompts.
   - Dynamic spring-animated progress bar when generating.
   - A real-time token/sec line chart updated as the LLM streams its response.

## ⌨️ Keyboard Controls

| Key | Action |
|-----|--------|
| `Enter` | Connect to selected URL / Select model / Send message |
| `a` | Add a new URL |
| `b` | Bulk add URLs (comma or newline separated) |
| `d` | Delete selected URL |
| `f` | Load URLs from file (prompts for file path) |
| `m` | View all models from all endpoints |
| `s` | View API statistics |
| `v` | View statistics (shortcut) |
| `Esc` | Go back / Cancel |
| `q` | Quit |

## URL Format

You can enter URLs in various formats - NOllama will automatically detect HTTP/HTTPS:

```bash
# Full URL
http://localhost:11434
https://ollama.example.com:11434

# Just host:port (auto-adds http://)
localhost:11434
alpha.nollama.net:11434

# Just host (auto-adds http:// and default port)
localhost          # becomes http://localhost:11434
ollama.example.com  # becomes http://ollama.example.com:11434
```

## 📁 URL File Format

You can create a file with URLs (one per line or comma-separated):

```text
# Example urls.txt
http://localhost:11434
http://alpha.nollama.net:11434
https://ollama.example.com:11434

# Or comma-separated:
http://localhost:11434, http://alpha.nollama.net:11434
```

Lines starting with `#` are treated as comments.

## ⚙️ Architecture & Codebase

The codebase is organized into three primary areas:

```text
├── main.go             # Entry point
├── api/
│   └── ollama.go       # Ollama API Client (Single & Multi-client support)
└── tui/
    ├── model.go        # Bubble Tea State Machine and TUI Logic
    └── styles.go       # Lip Gloss Custom Styling
```

### 1. The API Client (`api/ollama.go`)

This package abstracts the communication directly with the Ollama API.

#### Single Client
- **List Models**: A `GET` request to `/api/tags` returns a list of models currently available in the Ollama instance.
- **Chat**: A `POST` request to `/api/chat` with a payload containing the model name, message history, and a `stream: true` flag. Ollama server responds continuously with JSON chunks delimited by newlines until the generation is complete (`"done": true`).

#### Multi-Client (`api.MultiClient`)
The `MultiClient` provides aggregate functionality across multiple Ollama endpoints:
- `AddURL(url)` / `RemoveURL(url)` - Manage multiple endpoints
- `ListAllModels()` - Aggregates models from all configured endpoints
- `GetStats()` - Returns statistics (model count, total size) for each endpoint
- `GetClientForModel(modelName)` - Finds the endpoint serving a specific model

### 2. The TUI Layer (`tui/model.go` & `tui/styles.go`)

The application's interface leverages the Elm architecture (Model, View, Update) enabled by the powerful [Bubble Tea](https://github.com/charmbracelet/bubbletea) framework.

#### State Machine (`ModelsState`)
The application gracefully transitions through several states: `stateURLInput` ➔ `stateLoading` ➔ `stateModelSelection` ➔ `stateChat`. Each state defines what components are rendered in the `View()` function and what key/mouse events are handled in the `Update()` function.

#### Core Integrations Used "A LOT" 
- **[Bubble Tea](https://github.com/charmbracelet/bubbletea)** and **[Bubbles](https://github.com/charmbracelet/bubbles)**: Provides the event loop, viewport constraints, text inputs, lists, and spinners.
- **[Lip Gloss](https://github.com/charmbracelet/lipgloss)** (`tui/styles.go`): Heavily used to define adaptive colors, padding, flex-like sizing, and margins. For example `PrimaryColor`, `ContainerStyle`, and `ChatUserStyle`.
- **[Harmonica](https://github.com/charmbracelet/harmonica)**: Introduces a `harmonica.Spring` instance running at `1.0/60.0` tick rate. During a chat response generation, a progress bar interpolates its width continuously based on the mathematical spring update, rendering a buttery smooth visual effect.
- **[BubbleZone](https://github.com/lrstanley/bubblezone)** (`bz`): Overlays the TUI with physical dimensions to capture mouse events. By marking strings (like the "Back" button) with `zone.Mark()`, we check if a mouse click resides strictly within `zone.Get("back_btn").InBounds()`.
- **[Ntcharts](https://github.com/NimbleMarkets/ntcharts)**: Uses the `streamlinechart.Model` initialized in the chat state. It listens to the `tps` (tokens per second) parsed from the `ChatStream()` output and actively `Push()`es these data points to the graph during active generation.

### 3. Application Runner (`main.go`)

Initializes the `tui.NewModel` and executes the `tea.NewProgram`. It importantly configures `tea.WithAltScreen()` to prevent cluttering the user's terminal history and `tea.WithMouseCellMotion()` which pushes underlying terminal mouse events to the Bubble Tea application loop, required by BubbleZone functionality.

## 🛠️ Usage Setup

1. Make sure you have Go installed on your machine (`>=1.20`).
2. Make sure you have at least one model downloaded in your Ollama library (e.g., `ollama run llama3`).
3. Clone this repository and run:
   ```bash
   go mod tidy
   go run main.go
   ```
4. You'll start at the URL list screen where you can:
   - Press `Enter` on a URL to connect and view its models
   - Press `a` to add a new URL manually
   - Press `b` to bulk add URLs (comma or newline separated)
   - Press `f` to load URLs from a file (enter a file path)
   - Press `d` to delete the selected URL
   - Press `m` to view all models from all configured endpoints
   - Press `s` or `v` to view API statistics

5. Select a model to start chatting!
