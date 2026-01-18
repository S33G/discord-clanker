# Discord Clanker

A feature-rich Discord bot for interacting with LLMs (Large Language Models) through thread-based conversations. Supports any OpenAI-compatible API including Ollama, OpenAI, OpenRouter, and more.

## Features

✨ **Thread-Based Conversations** - Each `/ask` command creates a dedicated thread with full context
🤖 **Multi-Provider Support** - Works with Ollama, OpenAI, Claude, and any OpenAI-compatible API
🎯 **Smart Context Management** - Automatic token counting and context window management
🔐 **Role-Based Access Control** - Discord role-based permissions and model access
⚡ **Rate Limiting** - Configurable request and token limits per role
💾 **Redis-Backed** - Fast, persistent storage with automatic TTL
🔄 **Hot-Reload** - Update configuration without restarting
🎨 **Interactive Buttons** - Regenerate, copy, clear context, change settings
📊 **Usage Tracking** - Monitor token usage with configurable retention

## Quick Start

### Prerequisites

- **Docker & Docker Compose** (recommended)
- **Go 1.23+** (for local development)
- **Discord Bot Token** ([Get one here](https://discord.com/developers/applications))
- **Ollama** (optional, for local LLMs)

### 1. Get Discord Bot Token

1. Go to https://discord.com/developers/applications
2. Click "New Application" and name it
3. Go to "Bot" section → "Add Bot"
4. **Enable Intents:**
   - ✅ Message Content Intent
   - ✅ Server Members Intent
5. Copy the bot token

### 2. Invite Bot to Server

1. Go to "OAuth2" → "URL Generator"
2. Select scopes: `bot`, `applications.commands`
3. Select bot permissions (or use integer `274878295040`):
   - Read Messages/View Channels
   - Send Messages
   - Send Messages in Threads
   - Create Public Threads
   - Manage Threads
   - Embed Links
   - Read Message History
   - Use Slash Commands
4. Copy the URL and open in browser to invite bot

### 3. Get Your Guild ID

1. Enable Developer Mode in Discord (Settings → Advanced → Developer Mode)
2. Right-click your server icon → Copy Server ID

### 4. Setup Configuration

```bash
# Clone or download this repository
cd discord-prompter

# Copy example config
cp config/config.example.yaml config/config.yaml

# Edit config with your guild ID
nano config/config.yaml
# Replace YOUR_GUILD_ID_HERE with your actual guild ID

# Create environment file
cp .env.example .env
nano .env
# Add your DISCORD_TOKEN
```

### 5. Run with Docker (Recommended)

```bash
# Start services
docker-compose up -d

# View logs
docker-compose logs -f discord-prompter

# Stop services
docker-compose down
```

That's it! The bot should now be online in your Discord server.

## Usage

### Basic Commands

**Start a conversation:**
```
/ask prompt:Explain Docker networking
```

**Use a specific model:**
```
/ask prompt:Write a Python function model:gpt-4o
```

**Use a custom system prompt:**
```
/ask prompt:Write a story system_prompt:creative
```

**List available models:**
```
/models
```

**List system prompts:**
```
/prompts
```

**Check your usage:**
```
/usage
```

### Thread Interactions

Once a conversation thread is created, you can:

- **Reply normally** - Just send messages in the thread
- **🔄 Regenerate** - Re-run the last prompt
- **📋 Copy** - Copy the response to clipboard
- **🗑️ Clear Context** - Reset conversation history
- **⚙️ Settings** - Change model or system prompt mid-conversation

### Admin Commands

```
/reload - Reload configuration without restart (requires reload_config permission)
```

## Configuration

### Structure

```yaml
redis:
  address: "redis:6379"
  password_env: REDIS_PASSWORD

providers:
  - name: ollama-local
    base_url: http://host.docker.internal:11434/v1
    models:
      - id: llama3.2
        display_name: "Llama 3.2"

guilds:
  - id: "YOUR_GUILD_ID"
    enabled_models:
      - ollama-local/llama3.2
    rbac:
      roles:
        - discord_role: "Member"
          permissions:
            - use_models
          allowed_models:
            - ollama-local/llama3.2
```

See `config/config.example.yaml` for full configuration options.

### Environment Variables

```bash
# Required
DISCORD_TOKEN=your_bot_token_here
REDIS_PASSWORD=your_redis_password

# Optional (for cloud LLM providers)
OPENAI_API_KEY=sk-...
OPENROUTER_API_KEY=sk-or-v1-...
ANTHROPIC_API_KEY=sk-ant-...
```

### RBAC (Role-Based Access Control)

Configure permissions per Discord role:

```yaml
rbac:
  roles:
    - discord_role: "Admin"
      permissions:
        - unlimited_tokens
        - reload_config
      allowed_models: ["*"]  # All models
    
    - discord_role: "Member"
      permissions:
        - use_models
      allowed_models:
        - ollama-local/llama3.2
```

**Available Permissions:**
- `use_models` - Can use AI models
- `manage_prompts` - Can manage system prompts
- `unlimited_rate` - Bypass rate limits
- `unlimited_tokens` - Bypass token limits
- `reload_config` - Can reload configuration
- `view_all_usage` - Can view all users' usage

### Rate Limiting

```yaml
rate_limits:
  default:
    requests_per_minute: 10
    requests_per_hour: 100
  roles:
    Admin:
      requests_per_minute: 0  # 0 = unlimited
```

### Token Limiting

```yaml
token_limits:
  default:
    tokens_per_period: 50000
    period_hours: 24  # Daily reset
  roles:
    Pro:
      tokens_per_period: 200000
      period_hours: 168  # Weekly reset
```

## Local Development

### Run Locally

```bash
# Start Redis
docker run -d -p 6379:6379 redis:7-alpine

# Build and run
make build
make run

# Or just
go run ./cmd/bot --config config/config.yaml
```

### Run Tests

```bash
# All tests
make test

# Specific package
go test -v ./internal/config/...

# With coverage
go test -cover ./...
```

### Hot Reload Configuration

```bash
# Method 1: Use /reload command in Discord (requires permission)
/reload

# Method 2: Send SIGHUP signal
docker-compose kill -s SIGHUP discord-prompter
```

## Using with Ollama

### Install Ollama

```bash
curl -fsSL https://ollama.com/install.sh | sh
```

### Pull Models

```bash
ollama pull llama3.2
ollama pull codellama
ollama pull deepseek-coder
```

### Configure

In `config/config.yaml`:

```yaml
providers:
  - name: ollama-local
    base_url: http://host.docker.internal:11434/v1  # Docker
    # base_url: http://localhost:11434/v1  # Local dev
    api_key_env: ""  # No auth needed
    models:
      - id: llama3.2
        display_name: "Llama 3.2"
        context_window: 8192
```

## Architecture

```
discord-prompter/
├── cmd/bot/              # Main application
├── internal/
│   ├── bot/              # Discord bot logic
│   ├── config/           # Configuration loading
│   ├── conversation/     # Thread management & context
│   ├── llm/              # LLM client & registry
│   ├── rbac/             # Role-based access control
│   ├── ratelimit/        # Rate & token limiting
│   └── storage/          # Redis storage layer
├── config/               # Configuration files
├── scripts/lua/          # Redis Lua scripts
└── docker-compose.yaml   # Docker setup
```

## Troubleshooting

**Bot not responding:**
- Check `DISCORD_TOKEN` is correct
- Verify bot has required intents enabled
- Check bot has permissions in channel

**Commands not appearing:**
- Wait 5-10 minutes for Discord to register commands
- Try kicking and re-inviting the bot
- Check bot has "Use Slash Commands" permission

**Rate limit errors:**
- Check your role in `config.yaml`
- Verify role name matches exactly (case-sensitive)
- Admins can bypass with `unlimited_rate` permission

**LLM connection errors:**
- **Ollama:** Verify it's running: `curl http://localhost:11434/v1/models`
- **Docker:** Use `host.docker.internal` instead of `localhost`
- **API Keys:** Check environment variables are set

**Redis connection failed:**
- Check Redis is running: `docker-compose ps`
- Verify `REDIS_PASSWORD` matches in both services
- Check Redis is healthy: `docker-compose logs redis`

## Performance Tips

- Use local Ollama for unlimited, fast responses
- Set appropriate `max_context_tokens` for your models
- Configure `conversation_ttl_hours` to clean up old threads
- Use role-based `token_limits` to manage costs

## Contributing

Contributions welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Write tests for new features
4. Ensure all tests pass: `make test`
5. Submit a pull request

## License

MIT License - see LICENSE file for details

## Support

- **Issues:** [GitHub Issues](https://github.com/s33g/discord-prompter/issues)
- **Discussions:** [GitHub Discussions](https://github.com/s33g/discord-prompter/discussions)

## Acknowledgments

- Built with [discordgo](https://github.com/bwmarrin/discordgo)
- Token counting via [tiktoken-go](https://github.com/pkoukk/tiktoken-go)
- Logging with [zerolog](https://github.com/rs/zerolog)
