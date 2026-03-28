# NOllama - Specification Document

## Overview

NOllama is a Terminal User Interface (TUI) client for the Ollama REST API. It allows users to manage multiple Ollama endpoints, view models across all endpoints, and chat with LLMs.

---

## Core Features

### 1. Multi-Endpoint Management
- Add Ollama endpoints by URL, IP:port, or hostname
- Bulk add multiple URLs (comma-separated or newline-separated)
- Load URLs from a text file
- Delete endpoints from the list
- Auto-normalize URLs (add http:// prefix, :11434 default port)

### 2. Model Viewing
- View models from a single endpoint
- View all models aggregated from all configured endpoints
- Each model shows: name, size in GB, endpoint URL

### 3. API Statistics
- View status of all endpoints (connected/failed)
- Model count per endpoint
- Total size (GB) per endpoint
- Overall summary across all endpoints

### 4. Chat Interface
- Select a model and start chatting
- Streamed text responses
- Real-time performance metrics (tokens/sec, total tokens)
- Message history

---

## URL Handling

### Input Formats Accepted
| Input | Normalized Output |
|-------|-------------------|
| `localhost:11434` | `http://localhost:11434` |
| `alpha.nollama.net:11434` | `http://alpha.nollama.net:11434` |
| `ollama.example.com` | `http://ollama.example.com:11434` |
| `http://localhost:11434` | `http://localhost:11434` |
| `https://ollama.example.com:11434` | `https://ollama.example.com:11434` |
| `edge.nollama.net:11434/api/tags` | `http://edge.nollama.net:11434` (strips path) |

### File Format for Loading URLs
```
# Lines starting with # are comments
http://localhost:11434
http://alpha.nollama.net:11434
https://ollama.example.com:11434

# Or comma-separated on one line:
http://localhost:11434, http://alpha.nollama.net:11434
```

---

## Keyboard Controls

### URL List Screen (Main Screen)
| Key | Action |
|-----|--------|
| `Enter` | Connect to selected endpoint and view its models |
| `a` | Add new endpoint (opens input) |
| `b` | Bulk add endpoints (opens input) |
| `f` | Load endpoints from file (opens input) |
| `d` | Delete selected endpoint |
| `m` | View all models from all endpoints |
| `s` or `v` | View API statistics |
| `Esc` | Cancel / Go back |
| `q` | Quit application |

### Add URL Screen
| Key | Action |
|-----|--------|
| `Enter` | Add the URL and return to list |
| `Esc` | Cancel and return to list |

### Bulk Add Screen
| Key | Action |
|-----|--------|
| `Enter` | Parse and add all URLs |
| `Esc` | Cancel and return to list |

### File Input Screen
| Key | Action |
|-----|--------|
| `Enter` | Load URLs from file path |
| `Esc` | Cancel and return to list |

### Model Selection Screen
| Key | Action |
|-----|--------|
| `Enter` | Select model and start chat |
| `Esc` | Return to URL list |
| `q` | Quit |

### All Models Screen
| Key | Action |
|-----|--------|
| `Enter` | Select model and start chat |
| `Esc` | Return to URL list |
| `q` | Quit |

### Statistics Screen
| Key | Action |
|-----|--------|
| `Enter` | Refresh statistics |
| `Esc` | Return to URL list |
| `q` | Quit |

### Chat Screen
| Key | Action |
|-----|--------|
| `Enter` | Send message |
| `Esc` | Return to model selection |
| `Ctrl+C` | Quit |

---

## Error Handling

All errors are "soft failures" - the app never crashes or stops entirely.

### Connection Failures
- When an endpoint is unreachable or returns an error
- Shows a warning notification at top of screen
- Returns to the URL list automatically
- User can try other endpoints

### Invalid Input
- Empty URL input: ignored (nothing added)
- Invalid file path: shows notification, returns to list

### Chat Errors
- Shows notification
- Returns to model selection

---

## UI Layout

### Main Screen (URL List)
```
┌─────────────────────────────────────────────────────┐
│  ⚡ NOLLAMA                                     │
│  Multi-Endpoint LLM Client                         │
│                                                     │
│  ● 2 endpoint(s) ready                             │
│  ┌───────────────────────────────────────────────┐  │
│  │ http://localhost:11434                        │  │
│  │ http://alpha.nollama.net:11434              │  │
│  └───────────────────────────────────────────────┘  │
│                                                     │
│  [Enter] Connect  [a] Add  [b] Bulk  [f] File      │
│  [d] Del  [m] All Models  [s] Stats                │
└─────────────────────────────────────────────────────┘
```

### Model Selection Screen
```
┌─────────────────────────────────────────────────────┐
│  ◈ MODELS @ http://localhost:11434                 │
│  ┌───────────────────────────────────────────────┐  │
│  │ llama3                    4.78 GB             │  │
│  │ codellama                 3.80 GB             │  │
│  │ mistral                   4.10 GB             │  │
│  └───────────────────────────────────────────────┘  │
│                                                     │
│  [Enter] Select  │  [Esc] Back  │  [q] Quit        │
└─────────────────────────────────────────────────────┘
```

### All Models Screen
```
┌─────────────────────────────────────────────────────┐
│  ◈ ALL AVAILABLE MODELS                            │
│  ┌───────────────────────────────────────────────┐  │
│  │ llama3                    4.78 GB  localhost  │  │
│  │ codellama                 3.80 GB  localhost  │  │
│  │ mistral                   4.10 GB  192.168.1  │  │
│  └───────────────────────────────────────────────┘  │
│                                                     │
│  [Enter] Select Model  │  [Esc] Back  │  [q] Quit │
└─────────────────────────────────────────────────────┘
```

### Statistics Screen
```
┌─────────────────────────────────────────────────────┐
│  ◉ API STATISTICS                                  │
│  ⚠ 1/2 endpoints failed                            │
│  Total: 3 models | 12.68 GB                        │
│  ┌───────────────────────────────────────────────┐  │
│  │ http://localhost:11434 │ Models: 3 │ 12.68 GB │  │
│  │ http://alpha.nollama.net:11434 - ERROR      │  │
│  └───────────────────────────────────────────────┘  │
│                                                     │
│  [Enter] Refresh  │  [Esc] Back  │  [q] Quit      │
└─────────────────────────────────────────────────────┘
```

### Chat Screen
```
┌─────────────────────────────────────────────────────┐
│  ◀ Models  │  ⚡ CHAT                               │
│  localhost:11434 › llama3                           │
│  ┌───────────────────────────────────────────────┐  │
│  │                                               │  │
│  │  USER:                                        │  │
│  │  Hello                                        │  │
│  │                                               │  │
│  │  BOT:                                         │  │
│  │  Hello! How can I help you today?            │  │
│  │                                               │  │
│  └───────────────────────────────────────────────┘  │
│  ████████████████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░  │
│  > Type a message...                               │
│                                                     │
│  ⏱ 45.20 tok/s  │  128 tokens                      │
│  [██████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░] │
│                                                     │
│  [Esc] Back  │  [Enter] Send  │  [Ctrl+C] Exit    │
└─────────────────────────────────────────────────────┘
```

---

## Color Palette

| Color | Hex | Usage |
|-------|-----|-------|
| Primary | #7D56F4 | Headers, selected items, bot messages |
| Secondary | #B581FD | Accents, secondary elements |
| Accent | #F96987 | User messages, errors |
| Warning | #F98E69 | Warnings, pending states |
| Success | #2EDC20 | Success messages, connected status |
| White | #FFFFFF | Message content |
| Gray | #B0B0B0 | Subtle text, unselected items |
| Deep Gray | #3A3A3A | Borders, backgrounds |

---

## API Endpoints Used

### List Models
- **Endpoint**: `GET {baseURL}/api/tags`
- **Response**:
```json
{
  "models": [
    {
      "name": "llama3",
      "modified_at": "2024-01-15T10:30:00Z",
      "size": 3826793472,
      "digest": "sha256:..."
    }
  ]
}
```

### Chat
- **Endpoint**: `POST {baseURL}/api/chat`
- **Request**:
```json
{
  "model": "llama3",
  "messages": [
    {"role": "user", "content": "Hello"}
  ],
  "stream": true
}
```
- **Response** (streamed, newline-delimited JSON):
```json
{"model":"llama3","message":{"role":"assistant","content":"Hello"},"done":false}
{"model":"llama3","message":{"role":"assistant","content":"! How"},"done":false}
{"model":"llama3","done":true,"total_duration":5000000000,"eval_count":10}
```

---

## Data Structures

### Model
```go
type Model struct {
    Name        string    // e.g., "llama3"
    ModifiedAt  time.Time // last modified timestamp
    Size        int64     // size in bytes
    Digest      string    // sha256 digest
    URL         string    // source endpoint (added client-side)
}
```

### APIStats
```go
type APIStats struct {
    URL          string        // endpoint URL
    ModelCount   int           // number of models
    TotalSize    int64         // total size in bytes
    LastUpdated  time.Time     // when stats were fetched
    Error        error         // connection error if any
}
```

### Message
```go
type Message struct {
    Role    string // "user" or "assistant"
    Content string // message text
}
```

---

## Implementation Notes

1. **URL Normalization**: Always strip paths, add http:// prefix if missing, add :11434 port if missing
2. **Concurrent Requests**: When loading all models, query each endpoint in parallel
3. **Soft Errors**: Never crash on errors - always return to a usable state
4. **State Management**: Track current screen + pending next screen for async operations
5. **Notifications**: Show temporary warnings that auto-clear after 3+ seconds
