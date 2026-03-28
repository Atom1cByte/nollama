import os

class Config:
    BASE_DIR = os.path.dirname(os.path.abspath(__file__))
    
    SERVERS_FILE = os.path.join(BASE_DIR, "servers.json")
    ENDPOINTS_FILE = os.path.join(BASE_DIR, "endpoints.json")
    CACHE_FILE = os.path.join(BASE_DIR, "model_cache.json")
    SETTINGS_FILE = os.path.join(BASE_DIR, "settings.json")
    
    DEFAULT_PORT = 11434
    REQUEST_TIMEOUT = 10
    CHAT_TIMEOUT = 120
    
    MAX_WORKERS = 20
    
    CACHE_TTL_SECONDS = 3600
    
    API_VERSION = "v1"
    
    COLORS = {
        "primary": "#7D56F4",
        "secondary": "#B581FD",
        "accent": "#F96987",
        "warning": "#F98E69",
        "success": "#2EDC20",
        "white": "#FFFFFF",
        "gray": "#B0B0B0",
        "deep_gray": "#3A3A3A"
    }

class AppSettings:
    _instance = None
    _settings = {}
    
    def __new__(cls):
        if cls._instance is None:
            cls._instance = super().__new__(cls)
            cls._instance._load()
        return cls._instance
    
    def _load(self):
        if os.path.exists(Config.SETTINGS_FILE):
            try:
                import json
                with open(Config.SETTINGS_FILE, "r") as f:
                    self._settings = json.load(f)
            except:
                self._settings = self._defaults()
        else:
            self._settings = self._defaults()
    
    def _defaults(self):
        return {
            "cache_enabled": True,
            "cache_ttl": 3600,
            "auto_refresh": False,
            "refresh_interval": 300,
            "max_retries": 3,
            "request_timeout": 10,
            "chat_timeout": 120,
            "theme": "dark"
        }
    
    def get(self, key, default=None):
        return self._settings.get(key, default)
    
    def set(self, key, value):
        self._settings[key] = value
        self._save()
    
    def _save(self):
        import json
        with open(Config.SETTINGS_FILE, "w") as f:
            json.dump(self._settings, f, indent=2)
    
    def all(self):
        return self._settings.copy()
