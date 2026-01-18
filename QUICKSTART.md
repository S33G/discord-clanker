# Quick Start Guide

Get Discord Prompter running in 5 minutes!

## Prerequisites

- Docker & Docker Compose installed
- Discord Bot Token ([Get one here](https://discord.com/developers/applications))
- Discord Server (Guild) ID

## Steps

### 1. Get Your Discord Bot Token

1. Go to https://discord.com/developers/applications
2. Click "New Application" and give it a name
3. Go to "Bot" section → Click "Add Bot"
4. **Enable these intents:**
   - ✅ Message Content Intent
   - ✅ Server Members Intent
5. Copy the bot token (keep it secret!)

### 2. Invite Bot to Your Server

1. Go to "OAuth2" → "URL Generator"
2. Select scopes: `bot`, `applications.commands`
3. Select permissions: `274878295040` (or use the checkboxes):
   - Read Messages/View Channels
   - Send Messages
   - Send Messages in Threads
   - Create Public Threads
   - Manage Threads
   - Embed Links
   - Read Message History
   - Use Slash Commands
4. Copy the generated URL and open it in your browser
5. Select your server and authorize

### 3. Get Your Guild ID

1. In Discord, enable Developer Mode (Settings → Advanced → Developer Mode)
2. Right-click your server icon → Copy ID
3. Save this ID

### 4. Deploy the Bot

```bash
# Clone repository
git clone https://github.com/s33g/discord-prompter.git
cd discord-prompter

# Copy example config
cp config/config.example.yaml config/config.yaml

# Edit config with your details
nano config/config.yaml
```

**Update these values in config.yaml:**

```yaml
discord:
  token: "YOUR_BOT_TOKEN_HERE"  # Paste your bot token

guilds:
  - id: "YOUR_GUILD_ID_HERE"    # Paste your server ID
    name: "My Server"           # Your server name (any name)
    enabled_models:
      - "ollama/llama3.2"       # Or any model you want
```

**Start the bot:**

```bash
# If you have Ollama running locally:
docker-compose up -d

# Or if using OpenAI/other provider:
# Edit config.yaml to add your provider, then:
docker-compose up -d

# Check logs
docker-compose logs -f bot
```

### 5. Test It!

1. Go to your Discord server
2. Type `/ask` - you should see the command autocomplete
3. Type `/ask prompt:"Hello! How are you?"`
4. The bot will create a thread with the response
5. Continue chatting in the thread!

## Troubleshooting

### Bot doesn't appear online

- Check logs: `docker-compose logs bot`
- Verify token is correct in config
- Ensure bot is invited to the server

### Commands don't show up

- Wait 1-2 minutes for Discord to sync commands
- Check guild ID matches your server
- Try kicking and re-inviting the bot

### "Model not found" error

- Check that Ollama is running: `curl http://localhost:11434/api/tags`
- Verify model name matches in config
- Or configure a different provider (OpenAI, etc.)

### Rate limit errors

- Check Redis is running: `docker-compose ps`
- Increase rate limits in config for your role

## Next Steps

- Read [README.md](README.md) for full documentation
- See [DEPLOYMENT.md](DEPLOYMENT.md) for production setup
- Check [PROJECT_STATUS.md](PROJECT_STATUS.md) for features

## Quick Commands

```bash
# View logs
docker-compose logs -f bot

# Restart bot
docker-compose restart bot

# Stop everything
docker-compose down

# Rebuild after code changes
docker-compose build bot
docker-compose up -d

# Run tests
go test ./...
```

## Example Usage

Once running, you can:

- `/ask prompt:"Explain quantum computing"` - Start a conversation
- `/models` - See available models
- `/prompts` - See system prompts
- `/usage` - Check your usage stats
- Type in threads for multi-turn conversations
- Use buttons:
  - 🔄 Regenerate - Try again with same prompt
  - 📋 Copy - Copy response as text
  - 🗑️ Clear Context - Reset conversation
  - ⚙️ Settings - Change model or prompt

## Support

- Issues: https://github.com/s33g/discord-prompter/issues
- Full docs: See README.md

Enjoy your LLM-powered Discord bot! 🚀
