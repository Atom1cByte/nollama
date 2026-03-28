import json
import logging
import os
import time
import uuid
from concurrent.futures import ThreadPoolExecutor, as_completed
from functools import wraps
from typing import Any, Dict, Iterable, List, Optional
from urllib.parse import unquote, urlparse

import requests
from flask import Flask, Response, jsonify, render_template, request, stream_with_context

from cache import ModelCache
from config import AppSettings, Config
from models import APIError, ErrorCode
from router import router

app = Flask(__name__)
app.secret_key = os.urandom(24)

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

model_cache = ModelCache()
settings = AppSettings()
servers_cache: Dict[str, Any] = {"path": None, "mtime": None, "records": [], "map": {}}


def handle_api_errors(func):
    @wraps(func)
    def decorated(*args, **kwargs):
        try:
            return func(*args, **kwargs)
        except APIError as exc:
            return jsonify(exc.to_dict()), exc.status_code
        except Exception as exc:  # pragma: no cover - defensive fallback
            logger.exception("Unexpected error")
            return jsonify(APIError(str(exc), ErrorCode.SERVER_ERROR).to_dict()), 500

    return decorated


def normalize_url(url: str) -> Optional[str]:
    raw = (url or "").strip()
    if not raw:
        return None

    candidate = raw if "://" in raw else f"http://{raw}"
    parsed = urlparse(candidate)

    host = parsed.hostname
    if not host:
        return None

    scheme = parsed.scheme or "http"
    port = parsed.port or Config.DEFAULT_PORT
    return f"{scheme}://{host}:{port}"


def parse_urls_input(text: str) -> List[str]:
    seen = set()
    urls: List[str] = []

    for line in (text or "").replace(",", "\n").splitlines():
        stripped = line.strip()
        if not stripped or stripped.startswith("#"):
            continue
        normalized = normalize_url(stripped)
        if normalized and normalized not in seen:
            seen.add(normalized)
            urls.append(normalized)

    return urls


def _is_ip_host(host: str) -> bool:
    allowed = set("0123456789.:")
    return bool(host) and set(host) <= allowed


def server_file_path() -> str:
    if os.path.exists(Config.SERVERS_FILE):
        return Config.SERVERS_FILE
    return Config.ENDPOINTS_FILE


def server_record_to_url(record: Dict[str, Any]) -> Optional[str]:
    if record.get("url"):
        return normalize_url(record.get("url", ""))

    host = record.get("ip_str") or record.get("host") or record.get("hostname")
    if not host:
        hostnames = record.get("hostnames") or []
        host = hostnames[0] if hostnames else None
    port = record.get("port") or Config.DEFAULT_PORT
    scheme = record.get("scheme") or "http"
    return normalize_url(f"{scheme}://{host}:{port}") if host else None


def normalize_server_record(item: Any) -> Optional[Dict[str, Any]]:
    if isinstance(item, str):
        url = normalize_url(item)
        if not url:
            return None
        parsed = urlparse(url)
        host = parsed.hostname or ""
        return {
            "url": url,
            "ip_str": host,
            "port": parsed.port or Config.DEFAULT_PORT,
            "scheme": parsed.scheme or "http",
            "hostnames": [] if _is_ip_host(host) else [host],
            "location": {},
            "org": "",
            "isp": "",
            "version": None,
            "tags": ["manual"],
            "ollama": {},
            "_source": "manual",
        }

    if not isinstance(item, dict):
        return None

    record = dict(item)
    url = server_record_to_url(record)
    if not url:
        return None

    parsed = urlparse(url)
    record["url"] = url
    record["scheme"] = parsed.scheme or "http"
    record["ip_str"] = record.get("ip_str") or parsed.hostname or ""
    record["port"] = record.get("port") or parsed.port or Config.DEFAULT_PORT
    record["hostnames"] = [host for host in (record.get("hostnames") or []) if host]
    record["location"] = record.get("location") or {}
    record["ollama"] = record.get("ollama") or {}
    record["org"] = record.get("org") or ""
    record["isp"] = record.get("isp") or ""
    return record


def load_servers() -> List[Dict[str, Any]]:
    path = server_file_path()
    if not os.path.exists(path):
        return []

    mtime = os.path.getmtime(path)
    if servers_cache["path"] == path and servers_cache["mtime"] == mtime:
        return [dict(record) for record in servers_cache["records"]]

    records: List[Dict[str, Any]] = []
    try:
        with open(path, "r", encoding="utf-8") as handle:
            if path.endswith(".json") and os.path.basename(path) == "servers.json":
                for line in handle:
                    stripped = line.strip()
                    if not stripped:
                        continue
                    record = normalize_server_record(json.loads(stripped))
                    if record:
                        records.append(record)
            else:
                raw = json.load(handle)
                items: Iterable[Any] = raw if isinstance(raw, list) else raw.get("endpoints", [])
                for item in items:
                    record = normalize_server_record(item)
                    if record:
                        records.append(record)
    except Exception as exc:
        logger.warning("Failed to load server file %s: %s", path, exc)
        return []

    deduped: Dict[str, Dict[str, Any]] = {}
    for record in records:
        deduped[record["url"]] = record

    result = list(deduped.values())
    servers_cache["path"] = path
    servers_cache["mtime"] = mtime
    servers_cache["records"] = [dict(record) for record in result]
    servers_cache["map"] = {record["url"]: dict(record) for record in result}
    return [dict(record) for record in result]


def save_servers(records: List[Dict[str, Any]]) -> None:
    path = Config.SERVERS_FILE
    normalized = []
    seen = set()
    for item in records:
        record = normalize_server_record(item)
        if record and record["url"] not in seen:
            seen.add(record["url"])
            normalized.append(record)

    with open(path, "w", encoding="utf-8") as handle:
        for record in normalized:
            handle.write(json.dumps(record, ensure_ascii=True) + "\n")

    servers_cache["path"] = path
    servers_cache["mtime"] = os.path.getmtime(path)
    servers_cache["records"] = [dict(record) for record in normalized]
    servers_cache["map"] = {record["url"]: dict(record) for record in normalized}


def load_endpoints() -> List[str]:
    return [record["url"] for record in load_servers()]


def get_server_map() -> Dict[str, Dict[str, Any]]:
    load_servers()
    return {url: dict(record) for url, record in servers_cache["map"].items()}


def save_endpoints(endpoints: List[str]) -> None:
    server_map = get_server_map()
    records = []
    for url in parse_urls_input("\n".join(endpoints)):
        records.append(server_map.get(url) or normalize_server_record(url))
    save_servers([record for record in records if record])


def load_urls_from_file(filepath: str) -> Optional[List[str]]:
    try:
        with open(filepath, "r", encoding="utf-8") as handle:
            return parse_urls_input(handle.read())
    except Exception:
        return None


def format_size(size_bytes: int) -> str:
    gb = size_bytes / (1024**3)
    if gb >= 1:
        return f"{gb:.2f} GB"
    mb = size_bytes / (1024**2)
    return f"{mb:.2f} MB"


def safe_json_request() -> Dict[str, Any]:
    return request.get_json(silent=True) or {}


def endpoint_snapshot(url: str) -> Dict[str, Any]:
    server = get_server_map().get(url, {})
    cache = model_cache.get(url)
    source_models = []
    for name, details in (server.get("ollama") or {}).items():
        source_models.append(
            {
                "name": name,
                "size": details.get("size", 0) if isinstance(details, dict) else 0,
                "modified_at": details.get("modified_at", "") if isinstance(details, dict) else "",
                "digest": details.get("digest", "") if isinstance(details, dict) else "",
                "url": url,
            }
        )

    def model_value(item: Any, key: str, default: Any = None) -> Any:
        if isinstance(item, dict):
            return item.get(key, default)
        return getattr(item, key, default)

    models = []
    for cached_model in cache.models if cache else source_models:
        models.append(
            {
                "name": model_value(cached_model, "name", ""),
                "size": model_value(cached_model, "size", 0),
                "modified_at": model_value(cached_model, "modified_at", ""),
                "digest": model_value(cached_model, "digest", ""),
                "url": model_value(cached_model, "url", url),
            }
        )

    return {
        "url": url,
        "display_name": (server.get("hostnames") or [urlparse(url).hostname or url])[0],
        "status": cache.status if cache else ("online" if source_models else "unknown"),
        "models": models,
        "model_count": len(models),
        "total_size": sum(model["size"] for model in models),
        "error": cache.error if cache else None,
        "response_time": cache.response_time if cache else None,
        "last_updated": cache.last_updated if cache else None,
        "cached": bool(cache),
        "hostnames": server.get("hostnames") or [],
        "domains": server.get("domains") or [],
        "org": server.get("org") or "",
        "isp": server.get("isp") or "",
        "version": server.get("version"),
        "location": server.get("location") or {},
        "tags": server.get("tags") or [],
        "vulnerability_count": len(server.get("vulns") or {}),
        "scan_model_count": len(source_models),
    }


def get_models_from_endpoint(url: str, use_cache: bool = True) -> Dict[str, Any]:
    if use_cache and model_cache.is_cache_valid(url):
        cached = model_cache.get(url)
        if cached:
            snapshot = endpoint_snapshot(url)
            return {
                "success": True,
                "models": snapshot["models"],
                "cached": True,
                "status": snapshot["status"],
                "response_time": snapshot["response_time"],
            }

    start = time.time()
    timeout = settings.get("request_timeout", Config.REQUEST_TIMEOUT)

    try:
        response = requests.get(f"{url}/api/tags", timeout=timeout)
        response_time = time.time() - start

        if response.status_code != 200:
            error = f"HTTP {response.status_code}"
            model_cache.set(url, [], status="offline", error=error, response_time=response_time)
            return {"success": False, "error": error, "cached": False}

        payload = response.json()
        models = [
            {
                "name": model.get("name", ""),
                "size": model.get("size", 0),
                "modified_at": model.get("modified_at", ""),
                "digest": model.get("digest", ""),
                "url": url,
            }
            for model in payload.get("models", [])
        ]

        if settings.get("cache_enabled", True):
            model_cache.set(url, models, status="online", response_time=response_time)

        return {
            "success": True,
            "models": models,
            "cached": False,
            "status": "online",
            "response_time": response_time,
        }
    except requests.Timeout:
        response_time = time.time() - start
        model_cache.set(url, [], status="offline", error="Timeout", response_time=response_time)
        return {"success": False, "error": "Request timeout", "cached": False}
    except Exception as exc:
        response_time = time.time() - start
        model_cache.set(url, [], status="offline", error=str(exc), response_time=response_time)
        return {"success": False, "error": str(exc), "cached": False}


def get_endpoint_stats(url: str, force_refresh: bool = False) -> Dict[str, Any]:
    result = get_models_from_endpoint(url, use_cache=not force_refresh)
    if not result["success"]:
        snapshot = endpoint_snapshot(url)
        snapshot["error"] = result["error"]
        return snapshot

    snapshot = endpoint_snapshot(url)
    snapshot["cached"] = result.get("cached", False)
    return snapshot


def collect_endpoint_snapshots(force_refresh: bool = False) -> List[Dict[str, Any]]:
    endpoints = load_endpoints()
    if not endpoints:
        return []

    if not force_refresh:
        snapshots = [endpoint_snapshot(url) for url in endpoints]
        snapshots.sort(key=lambda item: item["url"])
        return snapshots

    snapshots: List[Dict[str, Any]] = []
    with ThreadPoolExecutor(max_workers=Config.MAX_WORKERS) as executor:
        futures = {executor.submit(get_endpoint_stats, url, force_refresh): url for url in endpoints}
        for future in as_completed(futures):
            snapshots.append(future.result())

    snapshots.sort(key=lambda item: item["url"])
    return snapshots


def build_catalog(models: List[Dict[str, Any]]) -> List[Dict[str, Any]]:
    grouped: Dict[str, Dict[str, Any]] = {}
    for model in models:
        entry = grouped.setdefault(
            model["name"],
            {"name": model["name"], "instances": [], "endpoints": []},
        )
        entry["instances"].append(model)
        entry["endpoints"].append(model["url"])

    catalog = []
    for entry in grouped.values():
        unique_endpoints = sorted(set(entry["endpoints"]))
        first = entry["instances"][0]
        catalog.append(
            {
                "name": entry["name"],
                "endpoint_count": len(unique_endpoints),
                "endpoints": unique_endpoints,
                "instance_count": len(entry["instances"]),
                "sample_size": first["size"],
                "sample_size_formatted": format_size(first["size"]),
                "modified_at": first["modified_at"],
                "instances": entry["instances"],
            }
        )

    catalog.sort(key=lambda item: (-item["endpoint_count"], item["name"].lower()))
    return catalog


def get_local_model_inventory(force_refresh: bool = False) -> Dict[str, Any]:
    snapshots = collect_endpoint_snapshots(force_refresh=force_refresh)
    all_models: List[Dict[str, Any]] = []
    online = 0
    offline = 0

    for snapshot in snapshots:
        if snapshot["status"] == "online":
            online += 1
            all_models.extend(snapshot["models"])
        else:
            offline += 1

    catalog = build_catalog(all_models)
    return {
        "endpoints": snapshots,
        "models": all_models,
        "catalog": catalog,
        "summary": {
            "endpoint_count": len(snapshots),
            "online_count": online,
            "offline_count": offline,
            "model_count": len(all_models),
            "catalog_count": len(catalog),
            "total_size": sum(model["size"] for model in all_models),
        },
    }


def get_cached_inventory() -> Dict[str, Any]:
    snapshots = [endpoint_snapshot(url) for url in load_endpoints()]
    all_models: List[Dict[str, Any]] = []

    for snapshot in snapshots:
        if snapshot["status"] == "online":
            all_models.extend(snapshot["models"])

    catalog = build_catalog(all_models)
    return {
        "endpoints": snapshots,
        "models": all_models,
        "catalog": catalog,
        "summary": {
            "endpoint_count": len(snapshots),
            "online_count": sum(1 for item in snapshots if item["status"] == "online"),
            "offline_count": sum(1 for item in snapshots if item["status"] != "online"),
            "model_count": len(all_models),
            "catalog_count": len(catalog),
            "total_size": sum(model["size"] for model in all_models),
        },
    }


def build_chat_payload(
    model: str,
    messages: Optional[List[Dict[str, Any]]] = None,
    prompt: Optional[str] = None,
    stream: bool = False,
    temperature: float = 0.7,
    max_tokens: Optional[int] = None,
    stop: Optional[Any] = None,
) -> Dict[str, Any]:
    payload: Dict[str, Any] = {
        "model": model,
        "stream": stream,
        "options": {"temperature": temperature},
    }
    if messages is not None:
        payload["messages"] = messages
    if prompt is not None:
        payload["prompt"] = prompt
    if max_tokens:
        payload["options"]["num_predict"] = max_tokens
    if stop:
        payload["options"]["stop"] = stop if isinstance(stop, list) else [stop]
    return payload


def call_ollama_json(url: str, path: str, payload: Dict[str, Any], timeout: int) -> Dict[str, Any]:
    response = requests.post(f"{url}{path}", json=payload, timeout=timeout)
    if response.status_code != 200:
        raise APIError(f"Ollama error: {response.text[:300]}", ErrorCode.SERVER_ERROR, response.status_code)
    return response.json()


def stream_ollama_chat(url: str, payload: Dict[str, Any]):
    timeout = settings.get("chat_timeout", Config.CHAT_TIMEOUT)

    try:
        response = requests.post(f"{url}/api/chat", json=payload, stream=True, timeout=timeout)
        if response.status_code != 200:
            yield f"data: {json.dumps({'error': {'message': f'HTTP {response.status_code}', 'type': 'server_error'}})}\n\n"
            return

        for line in response.iter_lines():
            if not line:
                continue
            try:
                data = json.loads(line.decode("utf-8").removeprefix("data: ").strip())
            except json.JSONDecodeError:
                continue

            if data.get("done"):
                result = {
                    "choices": [{"index": 0, "delta": {}, "finish_reason": "stop"}],
                    "usage": {
                        "prompt_tokens": data.get("prompt_eval_count", 0),
                        "completion_tokens": data.get("eval_count", 0),
                        "total_tokens": data.get("prompt_eval_count", 0) + data.get("eval_count", 0),
                    },
                }
                yield f"data: {json.dumps(result)}\n\n"
                yield "data: [DONE]\n\n"
            else:
                content = data.get("message", {}).get("content", "")
                if content:
                    yield f"data: {json.dumps({'choices': [{'index': 0, 'delta': {'content': content}}]})}\n\n"
    except Exception as exc:
        yield f"data: {json.dumps({'error': {'message': str(exc), 'type': 'server_error'}})}\n\n"


def stream_ollama_generate(url: str, payload: Dict[str, Any]):
    timeout = settings.get("chat_timeout", Config.CHAT_TIMEOUT)

    try:
        response = requests.post(f"{url}/api/generate", json=payload, stream=True, timeout=timeout)
        if response.status_code != 200:
            yield f"data: {json.dumps({'error': {'message': f'HTTP {response.status_code}', 'type': 'server_error'}})}\n\n"
            return

        for line in response.iter_lines():
            if not line:
                continue
            try:
                data = json.loads(line.decode("utf-8").removeprefix("data: ").strip())
            except json.JSONDecodeError:
                continue

            if data.get("done"):
                result = {
                    "choices": [{"text": "", "index": 0, "finish_reason": "stop"}],
                    "usage": {
                        "prompt_tokens": data.get("prompt_eval_count", 0),
                        "completion_tokens": data.get("eval_count", 0),
                        "total_tokens": data.get("prompt_eval_count", 0) + data.get("eval_count", 0),
                    },
                }
                yield f"data: {json.dumps(result)}\n\n"
                yield "data: [DONE]\n\n"
            else:
                content = data.get("response", "")
                if content:
                    yield f"data: {json.dumps({'choices': [{'text': content, 'index': 0}]})}\n\n"
    except Exception as exc:
        yield f"data: {json.dumps({'error': {'message': str(exc), 'type': 'server_error'}})}\n\n"


def find_local_model_url(model_name: str) -> Optional[str]:
    endpoints = load_endpoints()
    for url in endpoints:
        result = get_models_from_endpoint(url, use_cache=True)
        if not result["success"]:
            continue
        for model in result["models"]:
            if model["name"] == model_name:
                return url
    return None


def find_router_model_url(model_name: str, preferred_country: Optional[str] = None) -> Optional[str]:
    if not router._scan_loaded:
        router.load_scan_data()
    return router.find_best_server(model_name, preferred_country)


@app.route("/")
def index():
    inventory = get_cached_inventory()
    return render_template("index.html", inventory=inventory, settings=settings.all(), colors=Config.COLORS)


@app.route("/router")
def router_ui():
    return render_template("router.html", stats=router.get_stats())


@app.route("/api/status")
def api_status():
    inventory = get_cached_inventory()
    return jsonify(
        {
            "success": True,
            "summary": inventory["summary"],
            "settings": settings.all(),
            "cache": model_cache.get_stats(),
        }
    )


@app.route("/api/endpoints", methods=["GET"])
def api_get_endpoints():
    return jsonify({"success": True, "endpoints": load_endpoints()})


@app.route("/api/endpoints", methods=["POST"])
def api_add_endpoint():
    data = safe_json_request()
    url = normalize_url(data.get("url", ""))
    if not url:
        return jsonify({"success": False, "error": "A valid endpoint URL is required"}), 400

    endpoints = load_endpoints()
    created = url not in endpoints
    if created:
        endpoints.append(url)
        save_endpoints(endpoints)
        model_cache.invalidate(url)

    return jsonify({"success": True, "url": url, "created": created})


@app.route("/api/endpoints/bulk", methods=["POST"])
def api_bulk_add_endpoints():
    data = safe_json_request()
    urls = parse_urls_input(data.get("urls", ""))
    if not urls:
        return jsonify({"success": False, "error": "No valid endpoint URLs were found"}), 400

    endpoints = load_endpoints()
    added = []
    for url in urls:
        if url not in endpoints:
            endpoints.append(url)
            added.append(url)
            model_cache.invalidate(url)

    save_endpoints(endpoints)
    return jsonify({"success": True, "added": added, "total": len(endpoints)})


@app.route("/api/endpoints/file", methods=["POST"])
def api_load_endpoints_from_file():
    data = safe_json_request()
    filepath = data.get("filepath", "")
    urls = load_urls_from_file(filepath)
    if urls is None:
        return jsonify({"success": False, "error": "Could not read the supplied file"}), 400

    endpoints = load_endpoints()
    added = []
    for url in urls:
        if url not in endpoints:
            endpoints.append(url)
            added.append(url)
            model_cache.invalidate(url)

    save_endpoints(endpoints)
    return jsonify({"success": True, "added": added, "total": len(endpoints)})


@app.route("/api/endpoints/<path:url>", methods=["DELETE"])
def api_delete_endpoint(url: str):
    decoded = unquote(url)
    endpoints = load_endpoints()
    if decoded in endpoints:
        endpoints.remove(decoded)
        save_endpoints(endpoints)
    model_cache.invalidate(decoded)
    return jsonify({"success": True, "url": decoded})


@app.route("/api/models/<path:url>")
def api_models_for_endpoint(url: str):
    decoded = unquote(url)
    force_refresh = request.args.get("refresh", "false").lower() == "true"
    return jsonify(get_models_from_endpoint(decoded, use_cache=not force_refresh))


@app.route("/api/models")
def api_all_models():
    force_refresh = request.args.get("refresh", "false").lower() == "true"
    inventory = get_local_model_inventory(force_refresh=force_refresh)
    return jsonify({"success": True, "models": inventory["models"], "summary": inventory["summary"]})


@app.route("/api/catalog")
def api_catalog():
    force_refresh = request.args.get("refresh", "false").lower() == "true"
    inventory = get_local_model_inventory(force_refresh=force_refresh)
    return jsonify({"success": True, "catalog": inventory["catalog"], "summary": inventory["summary"]})


@app.route("/api/stats")
def api_stats():
    force_refresh = request.args.get("refresh", "false").lower() == "true"
    stats = collect_endpoint_snapshots(force_refresh=force_refresh)
    return jsonify(
        {
            "success": True,
            "stats": stats,
            "total_models": sum(item["model_count"] for item in stats if item["status"] == "online"),
            "total_size": sum(item["total_size"] for item in stats if item["status"] == "online"),
            "failed_count": sum(1 for item in stats if item["status"] != "online"),
            "total_endpoints": len(stats),
            "cache_stats": model_cache.get_stats(),
        }
    )


@app.route("/api/health")
def api_health():
    stats = collect_endpoint_snapshots(force_refresh=True)
    return jsonify(
        {
            "success": True,
            "results": stats,
            "summary": {
                "total": len(stats),
                "online": sum(1 for item in stats if item["status"] == "online"),
                "offline": sum(1 for item in stats if item["status"] != "online"),
            },
        }
    )


@app.route("/api/health/<path:url>")
def api_health_single(url: str):
    return jsonify(get_endpoint_stats(unquote(url), force_refresh=True))


@app.route("/api/cache/status")
def api_cache_status():
    return jsonify(
        {
            "success": True,
            "stats": model_cache.get_stats(),
            "endpoints": {
                url: {
                    "status": cache.status,
                    "models_count": len(cache.models),
                    "last_updated": cache.last_updated,
                    "response_time": cache.response_time,
                    "error": cache.error,
                }
                for url, cache in model_cache.get_all().items()
            },
        }
    )


@app.route("/api/cache/invalidate", methods=["POST"])
def api_invalidate_cache():
    data = safe_json_request()
    url = data.get("url")
    if url:
        model_cache.invalidate(url)
        return jsonify({"success": True, "message": f"Cache invalidated for {url}"})

    model_cache.invalidate_all()
    return jsonify({"success": True, "message": "All cache invalidated"})


@app.route("/api/settings", methods=["GET"])
def api_get_settings():
    return jsonify({"success": True, "settings": settings.all()})


@app.route("/api/settings", methods=["POST"])
def api_update_settings():
    data = safe_json_request()
    for key, value in data.items():
        settings.set(key, value)
    return jsonify({"success": True, "settings": settings.all()})


@app.route("/api/chat", methods=["POST"])
def api_chat_proxy():
    data = safe_json_request()
    url = normalize_url(data.get("url", ""))
    model = data.get("model")
    messages = data.get("messages", [])

    if not url or not model:
        return jsonify({"success": False, "error": "url and model are required"}), 400

    payload = build_chat_payload(model=model, messages=messages, stream=True)
    return Response(stream_with_context(stream_ollama_chat(url, payload)), mimetype="text/event-stream")


@app.route("/api/test/chat", methods=["POST"])
def api_test_chat():
    data = safe_json_request()
    url = normalize_url(data.get("url", ""))
    model = data.get("model")
    message = data.get("message", "hello")
    if not url or not model:
        return jsonify({"success": False, "error": "url and model are required"}), 400

    try:
        payload = build_chat_payload(
            model=model,
            messages=[{"role": "user", "content": message}],
            stream=False,
        )
        result = call_ollama_json(url, "/api/chat", payload, settings.get("chat_timeout", Config.CHAT_TIMEOUT))
        return jsonify({"success": True, "response": result})
    except APIError as exc:
        return jsonify({"success": False, "error": exc.message}), exc.status_code
    except Exception as exc:
        return jsonify({"success": False, "error": str(exc)}), 500


@app.route("/api/router/load", methods=["POST"])
def api_router_load():
    data = safe_json_request()
    router.load_scan_data(force_refresh=data.get("force", False))
    return jsonify({"success": True, "message": "Scan data loaded", "stats": router.get_stats()})


@app.route("/api/router/status")
def api_router_status():
    failed = {}
    for url, info in router._servers.items():
        if info.failed_models:
            failed[url] = {"ip": info.ip, "failed_models": info.failed_models}

    return jsonify({"success": True, "stats": router.get_stats(), "failed_servers": failed})


@app.route("/api/router/reset-failures", methods=["POST"])
def api_router_reset_failures():
    data = safe_json_request()
    target_url = data.get("url")
    with router._lock:
        if target_url and target_url in router._servers:
            router._servers[target_url].failed_models = {}
            router._servers[target_url].successful_models = []
        elif not target_url:
            for server in router._servers.values():
                server.failed_models = {}
                server.successful_models = []

    router._save_to_cache()
    return jsonify({"success": True, "message": "Failures reset"})


@app.route("/api/router/models")
def api_router_models():
    query = request.args.get("q", "")
    limit = int(request.args.get("limit", 100))
    models = router.search_models(query, limit) if query else router.get_all_models()
    data = [
        {
            "id": model.name,
            "name": model.name,
            "object": "model",
            "server_count": model.server_count,
            "servers": model.servers[:10],
        }
        for model in sorted(models, key=lambda item: item.server_count, reverse=True)[:limit]
    ]
    return jsonify({"object": "list", "data": data})


@app.route("/api/router/models/<path:model_name>")
def api_router_model(model_name: str):
    model = router.get_model(model_name)
    if not model:
        return jsonify({"success": False, "error": f"Model '{model_name}' not found"}), 404
    return jsonify({"success": True, "model": model.to_dict()})


@app.route("/api/router/servers")
def api_router_servers():
    country = request.args.get("country")
    has_models = request.args.get("has_models", "true").lower() == "true"

    servers = []
    for server in router._servers.values():
        if has_models and not server.models:
            continue
        if country and server.country != country:
            continue
        servers.append(
            {
                "url": server.url,
                "ip": server.ip,
                "version": server.version,
                "country": server.country,
                "city": server.city,
                "org": server.org,
                "model_count": len(server.models),
                "latency": server.latency,
            }
        )

    return jsonify({"success": True, "count": len(servers), "servers": servers[:100]})


@app.route("/api/router/refresh-latency", methods=["POST"])
def api_router_refresh_latency():
    data = safe_json_request()
    router.refresh_latencies(sample_size=data.get("sample_size", 50), timeout=data.get("timeout", 3.0))
    return jsonify({"success": True, "message": "Latency check completed"})


@app.route("/v1/models", methods=["GET"])
@handle_api_errors
def openai_models():
    inventory = get_local_model_inventory(force_refresh=request.args.get("refresh", "false").lower() == "true")
    data = [
        {
            "id": model["name"],
            "object": "model",
            "created": int(time.time()),
            "owned_by": "ollama",
            "root": model["name"],
            "parent": None,
            "size": model["size"],
        }
        for model in inventory["models"]
    ]
    return jsonify({"object": "list", "data": data})


@app.route("/v1/models/<model_name>", methods=["GET"])
@handle_api_errors
def openai_get_model(model_name: str):
    inventory = get_local_model_inventory(force_refresh=False)
    for model in inventory["models"]:
        if model["name"] == model_name:
            return jsonify(
                {
                    "id": model["name"],
                    "object": "model",
                    "created": int(time.time()),
                    "owned_by": "ollama",
                    "root": model["name"],
                    "parent": None,
                    "size": model["size"],
                }
            )
    raise APIError(f"Model '{model_name}' not found", ErrorCode.NOT_FOUND, 404)


@app.route("/v1/chat/completions", methods=["POST"])
@handle_api_errors
def openai_chat_completions():
    data = safe_json_request()
    model = data.get("model")
    messages = data.get("messages", [])
    stream = data.get("stream", False)
    temperature = data.get("temperature", 0.7)
    max_tokens = data.get("max_tokens")
    stop = data.get("stop")
    url = normalize_url(data.get("url", "")) if data.get("url") else None

    if not model:
        raise APIError("model is required", ErrorCode.INVALID_REQUEST, 400)

    if not url:
        url = find_local_model_url(model)
        if not url:
            raise APIError(f"Model '{model}' not found on any configured endpoint", ErrorCode.INVALID_MODEL, 404)

    payload = build_chat_payload(
        model=model,
        messages=messages,
        stream=stream,
        temperature=temperature,
        max_tokens=max_tokens,
        stop=stop,
    )

    if stream:
        return Response(stream_with_context(stream_ollama_chat(url, payload)), mimetype="text/event-stream")

    result = call_ollama_json(url, "/api/chat", payload, settings.get("chat_timeout", Config.CHAT_TIMEOUT))
    content = result.get("message", {}).get("content", "")
    return jsonify(
        {
            "id": f"chatcmpl-{uuid.uuid4().hex[:8]}",
            "object": "chat.completion",
            "created": int(time.time()),
            "model": model,
            "choices": [{"index": 0, "message": {"role": "assistant", "content": content}, "finish_reason": "stop"}],
            "usage": {
                "prompt_tokens": result.get("prompt_eval_count", 0),
                "completion_tokens": result.get("eval_count", 0),
                "total_tokens": result.get("prompt_eval_count", 0) + result.get("eval_count", 0),
            },
        }
    )


@app.route("/v1/completions", methods=["POST"])
@handle_api_errors
def openai_completions():
    data = safe_json_request()
    model = data.get("model")
    prompt = data.get("prompt", "")
    stream = data.get("stream", False)
    temperature = data.get("temperature", 0.7)
    max_tokens = data.get("max_tokens")
    stop = data.get("stop")
    url = normalize_url(data.get("url", "")) if data.get("url") else None

    if not model:
        raise APIError("model is required", ErrorCode.INVALID_REQUEST, 400)

    if not url:
        url = find_local_model_url(model)
        if not url:
            raise APIError(f"Model '{model}' not found on any configured endpoint", ErrorCode.INVALID_MODEL, 404)

    payload = build_chat_payload(
        model=model,
        prompt=prompt,
        stream=stream,
        temperature=temperature,
        max_tokens=max_tokens,
        stop=stop,
    )

    if stream:
        return Response(stream_with_context(stream_ollama_generate(url, payload)), mimetype="text/event-stream")

    result = call_ollama_json(url, "/api/generate", payload, settings.get("chat_timeout", Config.CHAT_TIMEOUT))
    return jsonify(
        {
            "id": f"cmpl-{uuid.uuid4().hex[:8]}",
            "object": "text_completion",
            "created": int(time.time()),
            "model": model,
            "choices": [{"text": result.get("response", ""), "index": 0, "finish_reason": "stop"}],
            "usage": {
                "prompt_tokens": result.get("prompt_eval_count", 0),
                "completion_tokens": result.get("eval_count", 0),
                "total_tokens": result.get("prompt_eval_count", 0) + result.get("eval_count", 0),
            },
        }
    )


@app.route("/v1/embeddings", methods=["POST"])
@handle_api_errors
def openai_embeddings():
    data = safe_json_request()
    model = data.get("model")
    prompt = data.get("prompt")
    url = normalize_url(data.get("url", "")) if data.get("url") else None

    if not model or not prompt:
        raise APIError("model and prompt are required", ErrorCode.INVALID_REQUEST, 400)

    if not url:
        url = find_local_model_url(model)
        if not url:
            raise APIError(f"Model '{model}' not found on any configured endpoint", ErrorCode.INVALID_MODEL, 404)

    result = call_ollama_json(
        url,
        "/api/embeddings",
        {"model": model, "prompt": prompt},
        settings.get("chat_timeout", Config.CHAT_TIMEOUT),
    )
    return jsonify({"object": "embedding", "embedding": result.get("embedding", []), "model": model})


@app.route("/v1/router/models", methods=["GET"])
@handle_api_errors
def openrouter_models():
    query = request.args.get("q", "")
    limit = int(request.args.get("limit", 100))
    models = router.search_models(query, limit) if query else router.get_all_models()
    data = [
        {
            "id": model.name,
            "object": "model",
            "created": int(time.time()),
            "owned_by": "nollama-router",
            "root": model.name,
            "parent": None,
            "server_count": model.server_count,
            "provider": {"servers": len(model.servers), "countries": sorted({s.get("country", "Unknown") for s in model.servers})},
        }
        for model in sorted(models, key=lambda item: item.server_count, reverse=True)[:limit]
    ]
    return jsonify({"object": "list", "data": data})


@app.route("/v1/router/models/<model_name>", methods=["GET"])
@handle_api_errors
def openrouter_get_model(model_name: str):
    model = router.get_model(model_name)
    if not model:
        raise APIError(f"Model '{model_name}' not found", ErrorCode.NOT_FOUND, 404)
    return jsonify(
        {
            "id": model.name,
            "object": "model",
            "created": int(time.time()),
            "owned_by": "nollama-router",
            "root": model.name,
            "parent": None,
            "server_count": model.server_count,
            "provider": {"servers": len(model.servers), "countries": sorted({s.get("country", "Unknown") for s in model.servers})},
        }
    )


@app.route("/v1/router/chat/completions", methods=["POST"])
@handle_api_errors
def openrouter_chat_completions():
    data = safe_json_request()
    model = data.get("model")
    messages = data.get("messages", [])
    stream = data.get("stream", False)
    temperature = data.get("temperature", 0.7)
    max_tokens = data.get("max_tokens")
    stop = data.get("stop")
    preferred_country = data.get("country")
    url = normalize_url(data.get("url", "")) if data.get("url") else None

    if not model:
        raise APIError("model is required", ErrorCode.INVALID_REQUEST, 400)

    if not url:
        url = find_router_model_url(model, preferred_country)
        if not url:
            raise APIError(f"Model '{model}' not found on any router server", ErrorCode.INVALID_MODEL, 404)

    payload = build_chat_payload(
        model=model,
        messages=messages,
        stream=stream,
        temperature=temperature,
        max_tokens=max_tokens,
        stop=stop,
    )

    if stream:
        return Response(stream_with_context(stream_ollama_chat(url, payload)), mimetype="text/event-stream")

    result = call_ollama_json(url, "/api/chat", payload, settings.get("chat_timeout", Config.CHAT_TIMEOUT))
    content = result.get("message", {}).get("content", "")
    return jsonify(
        {
            "id": f"chatcmpl-{uuid.uuid4().hex[:8]}",
            "object": "chat.completion",
            "created": int(time.time()),
            "model": model,
            "provider": {"name": "nollama-router", "url": url},
            "choices": [{"index": 0, "message": {"role": "assistant", "content": content}, "finish_reason": "stop"}],
            "usage": {
                "prompt_tokens": result.get("prompt_eval_count", 0),
                "completion_tokens": result.get("eval_count", 0),
                "total_tokens": result.get("prompt_eval_count", 0) + result.get("eval_count", 0),
            },
        }
    )


@app.route("/v1/router/completions", methods=["POST"])
@handle_api_errors
def openrouter_completions():
    data = safe_json_request()
    model = data.get("model")
    prompt = data.get("prompt", "")
    stream = data.get("stream", False)
    temperature = data.get("temperature", 0.7)
    max_tokens = data.get("max_tokens")
    stop = data.get("stop")
    preferred_country = data.get("country")
    url = normalize_url(data.get("url", "")) if data.get("url") else None

    if not model:
        raise APIError("model is required", ErrorCode.INVALID_REQUEST, 400)

    if not url:
        url = find_router_model_url(model, preferred_country)
        if not url:
            raise APIError(f"Model '{model}' not found on any router server", ErrorCode.INVALID_MODEL, 404)

    payload = build_chat_payload(
        model=model,
        prompt=prompt,
        stream=stream,
        temperature=temperature,
        max_tokens=max_tokens,
        stop=stop,
    )

    if stream:
        return Response(stream_with_context(stream_ollama_generate(url, payload)), mimetype="text/event-stream")

    result = call_ollama_json(url, "/api/generate", payload, settings.get("chat_timeout", Config.CHAT_TIMEOUT))
    return jsonify(
        {
            "id": f"cmpl-{uuid.uuid4().hex[:8]}",
            "object": "text_completion",
            "created": int(time.time()),
            "model": model,
            "provider": {"name": "nollama-router", "url": url},
            "choices": [{"text": result.get("response", ""), "index": 0, "finish_reason": "stop"}],
            "usage": {
                "prompt_tokens": result.get("prompt_eval_count", 0),
                "completion_tokens": result.get("eval_count", 0),
                "total_tokens": result.get("prompt_eval_count", 0) + result.get("eval_count", 0),
            },
        }
    )


if __name__ == "__main__":
    app.run(debug=True, port=5000)
