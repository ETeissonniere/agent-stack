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

### Schedule Configuration

The application uses a 6-field CRON format (with seconds) powered by `robfig/cron/v3`:

**Format**: `second minute hour day month weekday`

**Field ranges**:
- `second`: 0-59
- `minute`: 0-59  
- `hour`: 0-23
- `day`: 1-31
- `month`: 1-12 (or Jan-Dec)
- `weekday`: 0-6 (0=Sunday, or Sun-Sat)

**Special characters**:
- `*` (any): matches any value
- `?` (any): same as `*`, used for day/weekday
- `-` (range): `1-5` means 1,2,3,4,5
- `,` (list): `1,3,5` means 1 or 3 or 5
- `/` (step): `*/15` means every 15 units
- `@yearly`, `@monthly`, `@weekly`, `@daily`, `@hourly`

**Examples**:
- `"0 0 9 * * *"` - Daily at 9:00 AM
- `"0 30 8 * * 1"` - Mondays at 8:30 AM
- `"0 0 */6 * * *"` - Every 6 hours
- `"0 15 10 * * 1-5"` - Weekdays at 10:15 AM
- `"0 0 8 1 * *"` - First day of every month at 8:00 AM
- `"30 45 23 * * 0"` - Sundays at 11:45:30 PM

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