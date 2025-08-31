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

Optional environment variables:
- `CONFIG_FILE`: Custom config file path (default: `./config.yaml`)
- `HEALTHCHECK_PORT`: Health monitoring port for both app and Docker (default: 8080)

Configuration is managed in `config.yaml` with environment variable overrides.

### Video Filtering Configuration

The application includes video duration filters to skip very short or very long videos:

- `video.short_minutes`: Skip videos shorter than this duration (default: 1 minute)
- `video.long_minutes`: Skip videos longer than this duration (default: 60 minutes)

This helps focus analysis on substantive content while avoiding shorts and overly long videos.

### YouTube Token Management

The application automatically manages YouTube OAuth tokens to prevent expiration:

- **Automatic Token Refresh**: Tokens are automatically refreshed when they expire during API calls
- **Background Refresh**: A background goroutine refreshes tokens every 30 minutes (configurable via `youtube.token_refresh_minutes`)
- **Persistent Storage**: Refreshed tokens are immediately saved to disk at `youtube.token_file` location
- **Pre-run Refresh**: Tokens are proactively refreshed before each scheduled run as an extra safety measure
- **Graceful Shutdown**: The background refresher properly stops when the application exits

This ensures your YouTube authentication stays valid indefinitely without manual intervention. The refresh token from the initial OAuth flow is preserved and used to obtain new access tokens automatically.

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

- Endpoints: `/health` (200 OK or 503) and `/status` (text summary)
- Port: configured via `monitoring.health_port` in `config.yaml` (default 8080)
- Docker healthchecks: configurable via a single `HEALTHCHECK_PORT` variable used by both the app (override) and Docker healthchecks. Set it in `.env` to keep everything in sync.
- Logs: view with `docker logs youtube-curator`

## Agent Interface

Agents implement the scheduler contract in `shared/scheduler/scheduler.go`:

```go
// Metrics defines the common interface for agent metrics
type Metrics interface {
    GetSummary() string
}

// AgentEvents provides callbacks for monitoring agent execution
type AgentEvents struct {
    OnSuccess         func(metrics Metrics, duration time.Duration)
    OnPartialFailure  func(err error, duration time.Duration)
    OnCriticalFailure func(err error, duration time.Duration)
}

// Agent defines the interface that all agents must implement
type Agent interface {
    Name() string
    Initialize() error
    RunOnce(ctx context.Context, events *AgentEvents) error
}
```

Notes:
- Agents receive monitoring callbacks through `AgentEvents` for cleaner separation of concerns.
- `OnSuccess`: Called when agent completes successfully, receives metrics implementing the `Metrics` interface.
- `OnPartialFailure`: Called for recoverable errors (e.g., email send failures) that don't stop execution.
- `OnCriticalFailure`: Called for unrecoverable errors that require stopping execution.
- The scheduler handles all monitoring internally, agents provide domain-specific metrics via the `Metrics` interface.
- Scheduler prevents overlapping runs via `cron.SkipIfStillRunning`.
