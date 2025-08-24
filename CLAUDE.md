# YouTube Curator - Technical Documentation

A Go application that analyzes YouTube subscription videos using AI and sends curated email digests.

## Architecture

### Components

- **YouTube Client** (`agents/youtube-curator/youtube/`): OAuth 2.0 authentication and video fetching
- **AI Analyzer** (`shared/ai/`): Gemini 2.5 Flash video analysis with relevance scoring
- **Email Sender** (`shared/email/`): SMTP-based HTML email reports  
- **Scheduler** (`shared/scheduler/`): Cron-based daily execution with health monitoring
- **Configuration** (`shared/config/`): YAML config with environment variable overrides

### Data Models

- **Video**: YouTube video metadata
- **Analysis**: AI analysis with relevance score (1-10)
- **EmailReport**: Formatted email digest

## Configuration

Copy `config.example.yaml` to `config.yaml` and configure with your settings.

Required environment variables:
- `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET`: YouTube OAuth credentials  
- `GEMINI_API_KEY`: Google AI Studio API key
- `EMAIL_USERNAME` / `EMAIL_PASSWORD`: SMTP credentials

Configuration is managed in `config.yaml` with environment variable overrides.

## API Setup

### YouTube OAuth
1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create project → Enable YouTube Data API v3
3. Create OAuth 2.0 Desktop Application credentials
4. Set `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` environment variables

### Gemini AI
1. Go to [Google AI Studio](https://makersuite.google.com/app/apikey) 
2. Create API key → Set `GEMINI_API_KEY` environment variable

First run will prompt for OAuth authorization via browser flow.

## Development

### Local Development
```bash
go mod download
go run agents/youtube-curator/cmd/main.go --once
```

### Docker
```bash
docker-compose up -d
# Test with: docker run --env-file .env agent-stack ./youtube-curator --once
```

## Monitoring

Health check endpoint available at `:8080/health` when running. Container logs available via `docker logs youtube-curator`.