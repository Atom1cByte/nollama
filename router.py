import json
import os
import time
import threading
from typing import Dict, List, Optional, Any
from dataclasses import dataclass, field, asdict
from collections import defaultdict
from concurrent.futures import ThreadPoolExecutor, as_completed
import requests

from config import Config

SCAN_FILE = os.path.join(Config.BASE_DIR, "scrape", "myresults.json")
ROUTER_CACHE_FILE = os.path.join(Config.BASE_DIR, "router_cache.json")

@dataclass
class ServerInfo:
    url: str
    ip: str
    version: str
    country: str
    city: str
    org: str
    asn: str
    models: List[str] = field(default_factory=list)
    latency: float = 0
    status: str = "unknown"
    last_checked: float = 0
    failed_models: Dict[str, str] = field(default_factory=dict)
    successful_models: List[str] = field(default_factory=list)

@dataclass
class ModelInfo:
    name: str
    servers: List[Dict] = field(default_factory=list)
    server_count: int = 0
    
    def to_dict(self):
        return {
            "name": self.name,
            "server_count": self.server_count,
            "servers": self.servers
        }

class ModelRouter:
    _instance = None
    _lock = threading.RLock()
    
    def __new__(cls):
        if cls._instance is None:
            cls._instance = super().__new__(cls)
            cls._instance._initialized = False
        return cls._instance
    
    def __init__(self):
        if self._initialized:
            return
        
        self._servers: Dict[str, ServerInfo] = {}
        self._models: Dict[str, ModelInfo] = {}
        self._initialized = True
        self._loading = False
        self._scan_loaded = False
        
        self._load_from_cache()
    
    def mark_model_failed(self, url: str, model: str, error: str):
        with self._lock:
            if url in self._servers:
                self._servers[url].failed_models[model] = error
                self._servers[url].status = "failed"
                self._save_to_cache()
    
    def mark_model_success(self, url: str, model: str):
        with self._lock:
            if url in self._servers:
                if model not in self._servers[url].successful_models:
                    self._servers[url].successful_models.append(model)
                if model in self._servers[url].failed_models:
                    del self._servers[url].failed_models[model]
                self._servers[url].status = "online"
                self._save_to_cache()
    
    def test_model_on_server(self, url: str, model: str, timeout: float = 10.0) -> tuple[bool, str]:
        try:
            response = requests.post(
                f"{url}/api/chat",
                json={
                    "model": model,
                    "messages": [{"role": "user", "content": "hi"}],
                    "stream": False
                },
                timeout=timeout
            )
            
            if response.status_code == 200:
                self.mark_model_success(url, model)
                return True, "ok"
            elif response.status_code == 400:
                try:
                    err = response.json()
                    error_msg = err.get("error", "Unknown error")
                except:
                    error_msg = response.text[:200]
                
                self.mark_model_failed(url, model, error_msg)
                return False, error_msg
            else:
                return False, f"HTTP {response.status_code}"
                
        except requests.Timeout:
            return False, "Timeout"
        except Exception as e:
            return False, str(e)
    
    def find_working_server(self, model_name: str, preferred_country: Optional[str] = None, test: bool = True) -> Optional[tuple[str, bool]]:
        model = self._models.get(model_name)
        if not model or not model.servers:
            return None
        
        candidates = model.servers
        
        if preferred_country:
            country_candidates = [s for s in candidates if s.get("country") == preferred_country]
            if country_candidates:
                candidates = country_candidates
        
        for server in candidates:
            url = server["url"]
            
            if url not in self._servers:
                continue
            
            server_info = self._servers[url]
            
            if model_name in server_info.failed_models:
                continue
            
            if model_name in server_info.successful_models:
                return url, True
            
            if not test:
                return url, False
            
            works, _ = self.test_model_on_server(url, model_name)
            if works:
                return url, True
        
        return None
    
    def _load_from_cache(self):
        if os.path.exists(ROUTER_CACHE_FILE):
            try:
                with open(ROUTER_CACHE_FILE, "r") as f:
                    data = json.load(f)
                
                for url, s in data.get("servers", {}).items():
                    self._servers[url] = ServerInfo(**s)
                
                for name, m in data.get("models", {}).items():
                    self._models[name] = ModelInfo(**m)
                
                self._scan_loaded = True
            except Exception as e:
                print(f"Failed to load router cache: {e}")
    
    def _save_to_cache(self):
        data = {
            "servers": {url: asdict(s) for url, s in self._servers.items()},
            "models": {name: asdict(m) for name, m in self._models.items()},
            "updated": time.time()
        }
        
        try:
            with open(ROUTER_CACHE_FILE, "w") as f:
                json.dump(data, f, indent=2)
        except Exception as e:
            print(f"Failed to save router cache: {e}")
    
    def load_scan_data(self, force_refresh: bool = False):
        if self._scan_loaded and not force_refresh and self._servers:
            return
        
        with self._lock:
            if self._loading:
                return
            
            self._loading = True
            
            if not os.path.exists(SCAN_FILE):
                self._loading = False
                return
            
            try:
                servers_data = [json.loads(line) for line in open(SCAN_FILE, "r")]
                
                for srv in servers_data:
                    ip = srv.get("ip_str", "")
                    port = srv.get("port", 11434)
                    url = f"http://{ip}:{port}"
                    
                    location = srv.get("location", {})
                    country = location.get("country_name", "Unknown")
                    city = location.get("city", "")
                    
                    ollama_data = srv.get("ollama", {})
                    models = list(ollama_data.keys()) if ollama_data else []
                    
                    server_info = ServerInfo(
                        url=url,
                        ip=ip,
                        version=srv.get("version", "unknown"),
                        country=country,
                        city=city,
                        org=srv.get("org", ""),
                        asn=srv.get("asn", ""),
                        models=models,
                        status="online" if models else "no_models",
                        last_checked=time.time()
                    )
                    
                    self._servers[url] = server_info
                    
                    for model_name in models:
                        if model_name not in self._models:
                            self._models[model_name] = ModelInfo(name=model_name)
                        
                        self._models[model_name].servers.append({
                            "url": url,
                            "ip": ip,
                            "country": country,
                            "city": city,
                            "org": srv.get("org", ""),
                            "latency": 0
                        })
                        self._models[model_name].server_count = len(self._models[model_name].servers)
                
                self._scan_loaded = True
                self._save_to_cache()
                
            except Exception as e:
                print(f"Failed to load scan data: {e}")
            finally:
                self._loading = False
    
    def refresh_latencies(self, sample_size: int = 50, timeout: float = 3.0):
        servers_with_models = [s for s in self._servers.values() if s.models]
        
        if not servers_with_models:
            return
        
        sample = servers_with_models[:sample_size]
        
        def check_latency(server):
            try:
                start = time.time()
                response = requests.head(f"{server.url}/", timeout=timeout)
                latency = time.time() - start
                return server.url, latency if response.status_code < 500 else 999
            except:
                return server.url, 999
        
        with ThreadPoolExecutor(max_workers=20) as executor:
            futures = {executor.submit(check_latency, s): s for s in sample}
            for future in as_completed(futures):
                url, latency = future.result()
                if url in self._servers:
                    self._servers[url].latency = latency
    
    def get_all_models(self) -> List[ModelInfo]:
        return list(self._models.values())
    
    def get_model(self, name: str) -> Optional[ModelInfo]:
        return self._models.get(name)
    
    def find_best_server(self, model_name: str, preferred_country: Optional[str] = None, test: bool = True) -> Optional[str]:
        result = self.find_working_server(model_name, preferred_country, test)
        return result[0] if result else None
    
    def find_best_server_with_fallback(self, model_name: str, preferred_country: Optional[str] = None) -> tuple[Optional[str], bool]:
        return self.find_working_server(model_name, preferred_country, test=True) or (None, False)
    
    def get_servers_for_model(self, model_name: str) -> List[Dict]:
        model = self._models.get(model_name)
        return model.servers if model else []
    
    def get_stats(self) -> Dict[str, Any]:
        total_servers = len(self._servers)
        servers_with_models = sum(1 for s in self._servers.values() if s.models)
        total_models = len(self._models)
        
        model_server_counts = [m.server_count for m in self._models.values()]
        avg_servers_per_model = sum(model_server_counts) / len(model_server_counts) if model_server_counts else 0
        
        countries = defaultdict(int)
        for s in self._servers.values():
            if s.models:
                countries[s.country] += 1
        
        return {
            "total_servers": total_servers,
            "servers_with_models": servers_with_models,
            "total_models": total_models,
            "avg_servers_per_model": round(avg_servers_per_model, 2),
            "countries": dict(countries),
            "cache_age": time.time() - os.path.getmtime(ROUTER_CACHE_FILE) if os.path.exists(ROUTER_CACHE_FILE) else 0
        }
    
    def search_models(self, query: str, limit: int = 50) -> List[ModelInfo]:
        query = query.lower()
        results = []
        
        for model in self._models.values():
            if query in model.name.lower():
                results.append(model)
            
            if len(results) >= limit:
                break
        
        return sorted(results, key=lambda m: m.server_count, reverse=True)

router = ModelRouter()