# AI Capture Gateway

A single local process that proxies to multiple AI providers (OpenAI, Ollama, Docker Model Runner) while capturing request/response bodies for analysis. Features a REST Admin API and minimal web UI with pluggable storage.

## Features

- **Multi-Provider Proxy**: Routes to OpenAI, Ollama, and Docker Model Runner
- **Body-Only Capture**: Captures request/response bodies without headers for privacy
- **Streaming Support**: Handles SSE/chunked responses with chunk capture for playback
- **REST Admin API**: Query, fetch, delete, and export captured data
- **Web UI**: Browse, search, and analyze captured requests with dark mode
- **Pluggable Storage**: In-memory storage (extensible to SQLite/filesystem)
- **Privacy-Focused**: Never stores headers, local-only by default

## Quick Start

### Binary

```bash
# Build and run
go build -o capture-gateway ./cmd/gateway
./capture-gateway --config ./config.yaml

# UI available at: http://localhost:8080
# Providers available at:
#   OpenAI  → http://localhost:8080/openai
#   Ollama  → http://localhost:8080/ollama
#   DMR     → http://localhost:8080/dmr
```

### Docker

```bash
# Using Docker Compose
docker-compose up -d

# Or build and run directly
docker build -t openailogger .
docker run -p 8080:8080 -e CAPTURE_BIND=0.0.0.0 openailogger
```

## Configuration

### YAML Configuration (`config.yaml`)

```yaml
server:
  bind: "127.0.0.1"  # Bind address
  port: 8080          # Port to listen on

capture:
  max_body_mb: 20        # Maximum body size to capture (MB)
  store: "memory"        # Storage backend (memory)
  worker_pool_size: 10   # Async storage workers

routes:
  openai:
    mount: "/openai"
    upstream: "https://api.openai.com/v1"
  ollama:
    mount: "/ollama"
    upstream: "http://localhost:11434"
  dmr:
    mount: "/dmr"
    upstream: "http://localhost:3000"
```

## Client Setup

### OpenAI (Node.js)

```javascript
const openai = new OpenAI({
  baseURL: "http://localhost:8080/openai",
  apiKey: process.env.OPENAI_API_KEY
});

const response = await openai.chat.completions.create({
  model: "gpt-4o-mini",
  messages: [{ role: "user", content: "Hello!" }]
});
```

### Ollama

```bash
export OLLAMA_HOST=http://localhost:8080/ollama
ollama run llama2 "Hello world"
```

### Docker Model Runner

Set your client's base URL to `http://localhost:8080/dmr`.

## REST API

Base URL: `/api`

### Endpoints

- `GET /api/requests` - List requests with filtering
- `GET /api/requests/{id}` - Get specific request
- `GET /api/requests/{id}/chunks` - Stream playback (SSE)
- `DELETE /api/requests/{id}` - Delete request
- `GET /api/export.ndjson` - Export as NDJSON

### Query Parameters

- `provider` - Filter by provider (openai, ollama, dmr)
- `modelLike` - Filter by model name (partial match)
- `urlLike` - Filter by URL (partial match)
- `status` - Filter by HTTP status code
- `q` - Full-text search
- `from` / `to` - Time range (RFC3339 format)
- `offset` / `limit` - Pagination
- `sort` - Sort order (`ts` or `-ts`)

### Example

```bash
# Get recent OpenAI requests
curl "http://localhost:8080/api/requests?provider=openai&limit=10"

# Export all streaming requests
curl "http://localhost:8080/api/export.ndjson?stream=true" > streams.ndjson
```

## Data Model

```json
{
  "id": "uuid",
  "ts": "2024-01-01T12:00:00Z",
  "provider": "openai",
  "method": "POST",
  "url": "/chat/completions?stream=true",
  "upstream": "https://api.openai.com/v1",
  "status": 200,
  "duration_ms": 1234,
  "request_body": "{\"model\":\"gpt-4o-mini\",\"messages\":[...]}",
  "response_body": "{\"choices\":[...]}",
  "stream": true,
  "response_chunks": ["data: {...}", "data: {...}"],
  "size_req_bytes": 123,
  "size_res_bytes": 456,
  "model_hint": "gpt-4o-mini",
  "error": null
}
```
