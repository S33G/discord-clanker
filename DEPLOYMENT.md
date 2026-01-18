# Deployment Guide

This guide covers deploying Discord Clanker to production.

## Quick Deploy with Docker Compose (Recommended)

### 1. Prerequisites

- Docker Engine 20.10+
- Docker Compose 2.0+
- Discord Bot Token (see README.md)
- Your Discord Guild (Server) ID

### 2. Clone and Configure

```bash
# Clone the repository
git clone https://github.com/s33g/discord-prompter.git
cd discord-prompter

# Copy example config
cp config/config.example.yaml config/config.yaml

# Edit configuration
nano config/config.yaml
```

### 3. Required Configuration

Edit `config/config.yaml` and set:

```yaml
discord:
  token: "YOUR_BOT_TOKEN_HERE"  # Get from Discord Developer Portal

guilds:
  - id: "YOUR_GUILD_ID_HERE"    # Right-click server → Copy ID (enable Developer Mode)
    name: "My Server"
    enabled_models:
      - "ollama/llama3.2"       # Or any model you want to enable
```

### 4. Start Services

```bash
# Start with docker-compose
make up

# Or manually:
docker-compose up -d

# Check logs
make logs

# Or:
docker-compose logs -f bot
```

### 5. Verify Deployment

1. Check bot status: `docker-compose ps`
2. View logs: `docker-compose logs -f bot`
3. In Discord, type `/` and you should see the bot's commands
4. Test with `/ask prompt:"Hello!"`

## Environment Variables

You can override config values with environment variables:

```bash
export DISCORD_TOKEN="your-token"
export REDIS_ADDRESS="redis:6379"
export REDIS_PASSWORD="your-redis-password"

docker-compose up -d
```

## Production Deployment

### Option 1: Docker Compose (Single Server)

For most use cases, Docker Compose is sufficient:

```bash
# Production docker-compose.yml
version: '3.8'

services:
  bot:
    image: discord-prompter:latest
    restart: unless-stopped
    depends_on:
      redis:
        condition: service_healthy
    volumes:
      - ./config:/app/config:ro
      - ./data:/app/data
    environment:
      - DISCORD_TOKEN=${DISCORD_TOKEN}
      - REDIS_PASSWORD=${REDIS_PASSWORD}
    networks:
      - bot-network
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"

  redis:
    image: redis:7-alpine
    restart: unless-stopped
    command: >
      redis-server
      --requirepass ${REDIS_PASSWORD}
      --appendonly yes
      --appendfsync everysec
      --save 900 1
      --save 300 10
    volumes:
      - redis-data:/data
      - ./config/redis.conf:/usr/local/etc/redis/redis.conf:ro
    networks:
      - bot-network
    healthcheck:
      test: ["CMD", "redis-cli", "--raw", "incr", "ping"]
      interval: 10s
      timeout: 3s
      retries: 3

volumes:
  redis-data:
    driver: local

networks:
  bot-network:
    driver: bridge
```

### Option 2: Kubernetes

Example Kubernetes deployment:

```yaml
# kubernetes/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: discord-prompter
  labels:
    app: discord-prompter
spec:
  replicas: 1  # Only 1 replica for Discord bots
  selector:
    matchLabels:
      app: discord-prompter
  template:
    metadata:
      labels:
        app: discord-prompter
    spec:
      containers:
      - name: bot
        image: discord-prompter:latest
        env:
        - name: DISCORD_TOKEN
          valueFrom:
            secretKeyRef:
              name: discord-secrets
              key: token
        - name: REDIS_ADDRESS
          value: "redis-service:6379"
        - name: REDIS_PASSWORD
          valueFrom:
            secretKeyRef:
              name: redis-secrets
              key: password
        volumeMounts:
        - name: config
          mountPath: /app/config
          readOnly: true
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
      volumes:
      - name: config
        configMap:
          name: discord-prompter-config
---
apiVersion: v1
kind: Service
metadata:
  name: redis-service
spec:
  selector:
    app: redis
  ports:
  - port: 6379
    targetPort: 6379
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redis
spec:
  replicas: 1
  selector:
    matchLabels:
      app: redis
  template:
    metadata:
      labels:
        app: redis
    spec:
      containers:
      - name: redis
        image: redis:7-alpine
        args:
        - redis-server
        - --requirepass
        - $(REDIS_PASSWORD)
        - --appendonly
        - "yes"
        env:
        - name: REDIS_PASSWORD
          valueFrom:
            secretKeyRef:
              name: redis-secrets
              key: password
        volumeMounts:
        - name: redis-data
          mountPath: /data
        ports:
        - containerPort: 6379
      volumes:
      - name: redis-data
        persistentVolumeClaim:
          claimName: redis-pvc
```

## Configuration Best Practices

### Security

1. **Secrets Management**
   - Never commit tokens to Git
   - Use environment variables or secret managers
   - Rotate tokens regularly

2. **Redis Security**
   - Set a strong password
   - Use ACLs for fine-grained access
   - Enable TLS for network transit (if exposed)

3. **RBAC Configuration**
   - Start with restrictive permissions
   - Use `@everyone` role for base access
   - Create role hierarchy (e.g., admin → pro → free)

### Performance

1. **Rate Limits**
   ```yaml
   rate_limits:
     default:
       requests_per_minute: 10
       requests_per_hour: 100
   ```

2. **Token Limits**
   ```yaml
   token_limits:
     default:
       tokens_per_period: 100000
       period_hours: 24
   ```

3. **Redis Configuration**
   - Enable AOF for durability
   - Set appropriate maxmemory
   - Use eviction policy: `allkeys-lru`

### Monitoring

1. **Logs**
   ```bash
   # View bot logs
   docker-compose logs -f bot
   
   # Filter errors
   docker-compose logs bot | grep ERROR
   ```

2. **Redis Monitoring**
   ```bash
   # Connect to Redis
   docker-compose exec redis redis-cli
   
   # Check stats
   INFO stats
   INFO memory
   ```

3. **Health Checks**
   ```bash
   # Check bot is running
   docker-compose ps
   
   # Test Redis
   docker-compose exec redis redis-cli PING
   ```

## Backup and Recovery

### Backup Redis Data

```bash
# Manual backup
docker-compose exec redis redis-cli BGSAVE

# Copy RDB file
docker cp $(docker-compose ps -q redis):/data/dump.rdb ./backup/

# Or backup entire volume
docker run --rm -v discord-prompter_redis-data:/data \
  -v $(pwd)/backup:/backup alpine \
  tar czf /backup/redis-backup-$(date +%Y%m%d).tar.gz /data
```

### Restore Redis Data

```bash
# Stop services
docker-compose down

# Restore from backup
docker run --rm -v discord-prompter_redis-data:/data \
  -v $(pwd)/backup:/backup alpine \
  tar xzf /backup/redis-backup-YYYYMMDD.tar.gz -C /

# Start services
docker-compose up -d
```

## Troubleshooting

### Bot Not Responding

1. Check bot is running: `docker-compose ps`
2. Check logs: `docker-compose logs bot`
3. Verify Discord token is correct
4. Ensure bot has proper permissions in Discord server
5. Check guild ID matches in config

### Redis Connection Errors

1. Check Redis is running: `docker-compose ps redis`
2. Test connection: `docker-compose exec redis redis-cli PING`
3. Verify password matches in config
4. Check network connectivity between containers

### Rate Limit Issues

1. Check user's current usage: `/usage` command
2. Review rate limit config for user's role
3. Check Redis for usage data:
   ```bash
   docker-compose exec redis redis-cli
   KEYS rate_limit:*
   ```

### Out of Memory

1. Check Redis memory usage:
   ```bash
   docker-compose exec redis redis-cli INFO memory
   ```
2. Increase maxmemory in redis.conf
3. Reduce conversation TTL
4. Reduce message history limit

## Updating

### Update Bot Version

```bash
# Pull latest changes
git pull

# Rebuild image
docker-compose build bot

# Restart services
docker-compose up -d

# Check logs
docker-compose logs -f bot
```

### Update Configuration

```bash
# Edit config
nano config/config.yaml

# Reload without restart (if enabled)
# Use /reload command in Discord

# Or restart bot
docker-compose restart bot
```

## Scaling Considerations

⚠️ **Important**: Discord bots should NOT be scaled horizontally (multiple instances). Discord's gateway connection is stateful and each bot token can only have one active connection.

For high-load scenarios:
- Increase container resources (CPU/RAM)
- Optimize Redis performance
- Use faster LLM providers
- Implement request queuing

## Support

- **Issues**: https://github.com/s33g/discord-prompter/issues
- **Discussions**: https://github.com/s33g/discord-prompter/discussions
- **Documentation**: See README.md and config examples
