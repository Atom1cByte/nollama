import json
import os
import time
import threading
from typing import Dict, List, Optional, Any
from dataclasses import dataclass, asdict
from config import Config

@dataclass
class CachedModel:
    name: str
    size: int
    modified_at: str
    digest: str
    url: str
    cached_at: float
    last_verified: float

@dataclass
class EndpointCache:
    url: str
    models: List[CachedModel]
    last_updated: float
    status: str
    error: Optional[str]
    response_time: float

class ModelCache:
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
        self._cache: Dict[str, EndpointCache] = {}
        self._loading = {}
        self._initialized = True
        self._load_from_disk()
    
    def _load_from_disk(self):
        if not os.path.exists(Config.CACHE_FILE):
            return
        try:
            with open(Config.CACHE_FILE, "r") as f:
                data = json.load(f)
            
            for url, cache_data in data.get("endpoints", {}).items():
                models = [CachedModel(**m) for m in cache_data.get("models", [])]
                self._cache[url] = EndpointCache(
                    url=url,
                    models=models,
                    last_updated=cache_data.get("last_updated", 0),
                    status=cache_data.get("status", "unknown"),
                    error=cache_data.get("error"),
                    response_time=cache_data.get("response_time", 0)
                )
        except Exception as e:
            print(f"Failed to load cache: {e}")
    
    def _save_to_disk(self):
        data = {
            "endpoints": {},
            "version": "1.0"
        }
        
        for url, cache in self._cache.items():
            data["endpoints"][url] = {
                "models": [asdict(m) for m in cache.models],
                "last_updated": cache.last_updated,
                "status": cache.status,
                "error": cache.error,
                "response_time": cache.response_time
            }
        
        try:
            with open(Config.CACHE_FILE, "w") as f:
                json.dump(data, f, indent=2)
        except Exception as e:
            print(f"Failed to save cache: {e}")
    
    def is_cache_valid(self, url: str) -> bool:
        if url not in self._cache:
            return False
        
        cache = self._cache[url]
        age = time.time() - cache.last_updated
        
        settings = AppSettings()
        ttl = settings.get("cache_ttl") or Config.CACHE_TTL_SECONDS
        
        return age < ttl and cache.status == "online"
    
    def get(self, url: str) -> Optional[EndpointCache]:
        with self._lock:
            return self._cache.get(url)
    
    def get_all(self) -> Dict[str, EndpointCache]:
        with self._lock:
            return self._cache.copy()
    
    def get_all_models(self) -> List[CachedModel]:
        with self._lock:
            models = []
            for cache in self._cache.values():
                if cache.status == "online":
                    models.extend(cache.models)
            return models
    
    def set(self, url: str, models: List[Dict], status: str = "online", error: Optional[str] = None, response_time: float = 0):
        with self._lock:
            cached_models = [
                CachedModel(
                    name=m.get("name", ""),
                    size=m.get("size", 0),
                    modified_at=m.get("modified_at", ""),
                    digest=m.get("digest", ""),
                    url=url,
                    cached_at=time.time(),
                    last_verified=time.time()
                )
                for m in models
            ]
            
            self._cache[url] = EndpointCache(
                url=url,
                models=cached_models,
                last_updated=time.time(),
                status=status,
                error=error,
                response_time=response_time
            )
            
            self._save_to_disk()
    
    def invalidate(self, url: str):
        with self._lock:
            if url in self._cache:
                del self._cache[url]
                self._save_to_disk()
    
    def invalidate_all(self):
        with self._lock:
            self._cache.clear()
            self._save_to_disk()
    
    def get_stats(self) -> Dict[str, Any]:
        with self._lock:
            total_models = sum(len(c.models) for c in self._cache.values() if c.status == "online")
            total_size = sum(sum(m.size for m in c.models) for c in self._cache.values() if c.status == "online")
            online = sum(1 for c in self._cache.values() if c.status == "online")
            offline = sum(1 for c in self._cache.values() if c.status == "offline")
            
            timestamps = [c.last_updated for c in self._cache.values()]
            max_timestamp = max(timestamps) if timestamps else 0
            
            return {
                "total_endpoints": len(self._cache),
                "online_endpoints": online,
                "offline_endpoints": offline,
                "total_models": total_models,
                "total_size": total_size,
                "cache_age": time.time() - max_timestamp if max_timestamp > 0 else 0
            }

from config import AppSettings