<h1 align="center">nollama</h1>

<p align="center">
  <strong>Inventory. Selection. Dispatch.</strong>
</p>

<p align="center">
  A neoclassical front end for working with many public or private Ollama servers at once.
</p>

---

`nollama` is a lightweight Flask app for browsing large server inventories, grouping models across hosts, and sending chats through a simple web UI. It can read rich server metadata from `servers.json`, expose OpenAI-compatible local routes, and keep the operational view fast by using cached data unless you explicitly ask for a live refresh.

## At A Glance

| Layer | Purpose |
| --- | --- |
| Inventory | Read and browse large server lists from `servers.json` |
| Catalog | Group models across many hosts into one searchable view |
| Dispatch | Send chats to a specific server and model from the browser |
| Compatibility | Offer OpenAI-style local routes for tooling and clients |

## What It Does

- Reads a large `servers.json` inventory and turns it into a usable dashboard.
- Shows server metadata like hostname, location, org, Ollama version, vulnerability count, and available models.
- Groups model availability across many servers so you can find the right target quickly.
- Lets you chat directly against a chosen server and model from the browser.
- Exposes local OpenAI-style endpoints under `/v1/*`.
- Keeps router-specific routes separate under `/v1/router/*`.

## Stack

- Python
- Flask
- Requests
- Plain HTML, CSS, and JavaScript

## Run It

```bash
pip install -r requirements.txt
python app.py
```

Then open `http://localhost:5000`.

## Visual Direction

The current interface leans into a darker editorial, neoclassical poster style rather than a generic operations panel: serif typography, framed cards, bronze accents, and a more ceremonial feel for a tool that coordinates model hosts.

## Data Files

This project expects local data files that should not be committed:

- `servers.json`
- `endpoints.json`
- `model_cache.json`
- `router_cache.json`
- `scrape/*.json`

The repository `.gitignore` is set up to keep those out of Git.

## Routes

Core app routes:

- `/`
- `/api/status`
- `/api/stats`
- `/api/catalog`
- `/api/chat`

OpenAI-compatible routes:

- `/v1/models`
- `/v1/chat/completions`
- `/v1/completions`
- `/v1/embeddings`

Router-specific routes:

- `/router`
- `/api/router/*`
- `/v1/router/*`

## Note 

Yeah everything before this is true, but I made this tool to aggregate exposed ollama instances for research purposes.

Most of this project is a hacky, vibecoded mess with some of my human slop in between and you'll probably only ever use the web part or openai-compatible router. I left in old/ intentionally.

I DO NOT take responsibility for how you use this tool and DO NOT condone the usage of exposed ollama instances without consent. This project is made to raise awareness and make the internet more secure.

btw feel free to use this for it's pseudo-intended purpose of aggregating and routing your own ollama servers, too.

---

<p align="center">
  Built for people who would rather aim prompts than babysit server lists.
</p>
