from dataclasses import dataclass, field
from typing import List, Optional, Dict, Any
from datetime import datetime
import uuid

@dataclass
class Endpoint:
    url: str
    name: Optional[str] = None
    enabled: bool = True
    priority: int = 0
    tags: List[str] = field(default_factory=list)
    metadata: Dict[str, Any] = field(default_factory=dict)
    
    def to_dict(self) -> Dict:
        return {
            "url": self.url,
            "name": self.name or self.url,
            "enabled": self.enabled,
            "priority": self.priority,
            "tags": self.tags,
            "metadata": self.metadata
        }

@dataclass 
class Model:
    name: str
    size: int
    modified_at: str
    digest: str
    url: str
    
    @property
    def size_formatted(self) -> str:
        gb = self.size / (1024 ** 3)
        if gb >= 1:
            return f"{gb:.2f} GB"
        mb = self.size / (1024 ** 2)
        return f"{mb:.2f} MB"
    
    def to_dict(self) -> Dict:
        return {
            "name": self.name,
            "size": self.size,
            "size_formatted": self.size_formatted,
            "modified_at": self.modified_at,
            "digest": self.digest,
            "url": self.url
        }

@dataclass
class ChatMessage:
    role: str
    content: str
    
    def to_dict(self) -> Dict:
        return {"role": self.role, "content": self.content}

@dataclass
class ChatRequest:
    model: str
    messages: List[ChatMessage]
    temperature: float = 0.7
    top_p: float = 0.9
    max_tokens: Optional[int] = None
    stream: bool = True
    stop: Optional[List[str]] = None
    frequency_penalty: Optional[float] = None
    presence_penalty: Optional[float] = None
    
    @classmethod
    def from_openai(cls, data: Dict) -> "ChatRequest":
        messages = [ChatMessage(**m) for m in data.get("messages", [])]
        return cls(
            model=data.get("model", ""),
            messages=messages,
            temperature=data.get("temperature", 0.7),
            top_p=data.get("top_p", 0.9),
            max_tokens=data.get("max_tokens"),
            stream=data.get("stream", True),
            stop=data.get("stop"),
            frequency_penalty=data.get("frequency_penalty"),
            presence_penalty=data.get("presence_penalty")
        )

@dataclass
class CompletionRequest:
    prompt: str
    model: str
    temperature: float = 0.7
    max_tokens: Optional[int] = None
    stream: bool = True
    stop: Optional[List[str]] = None
    
    @classmethod
    def from_openai(cls, data: Dict) -> "CompletionRequest":
        return cls(
            prompt=data.get("prompt", ""),
            model=data.get("model", ""),
            temperature=data.get("temperature", 0.7),
            max_tokens=data.get("max_tokens"),
            stream=data.get("stream", True),
            stop=data.get("stop")
        )

@dataclass
class EmbeddingsRequest:
    model: str
    prompt: str
    
    @classmethod
    def from_openai(cls, data: Dict) -> "EmbeddingsRequest":
        return cls(
            model=data.get("model", ""),
            prompt=data.get("prompt", "")
        )

class APIError(Exception):
    def __init__(self, message: str, code: str = "internal_error", status_code: int = 500):
        self.message = message
        self.code = code
        self.status_code = status_code
        super().__init__(message)
    
    def to_dict(self) -> Dict:
        return {
            "error": {
                "message": self.message,
                "type": self.code,
                "code": self.code
            }
        }

class ErrorCode:
    INVALID_REQUEST = "invalid_request_error"
    NOT_FOUND = "not_found_error"
    INVALID_MODEL = "invalid_model_error"
    SERVER_ERROR = "server_error"
    TIMEOUT = "timeout_error"
    RATE_LIMIT = "rate_limit_error"
    PERMISSION = "permission_error"