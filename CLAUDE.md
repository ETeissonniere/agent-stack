# Agent Stack - Technical Documentation

A Go application that hosts multiple intelligent automation agents for daily tasks and notifications.

## Architecture

### Shared Components

- **Scheduler** (`shared/scheduler/`): Cron-based execution with health monitoring
- **Configuration** (`shared/config/`): YAML config with environment variable overrides
- **Email Sender** (`shared/email/`): SMTP-based HTML email reports
- **Monitoring** (`shared/monitoring/`): Health check endpoints and status tracking

### YouTube Curator Agent (`agents/youtube-curator/`)

- **YouTube Client** (`youtube/`): OAuth 2.0 authentication and video fetching
- **AI Analyzer** (`shared/ai/`): Gemini 2.5 Flash video analysis with relevance scoring
- **Agent** (`agent.go`): Main agent implementation following scheduler interface

### Drone Weather Agent (`agents/drone-weather/`)

- **Weather Client** (`weather.go`): Open-Meteo API integration for weather data
- **TFR Client** (`tfr.go`): FAA Temporary Flight Restrictions monitoring
- **Agent** (`agent.go`): Main agent implementation with email notifications
- **Email Template** (`email_template.html`): HTML template for flight condition reports

### Data Models (`internal/models/`)

**YouTube Curator:**
- **Video**: YouTube video metadata
- **Analysis**: AI analysis with relevance score (1-10)
- **EmailReport**: Formatted email digest

**Drone Weather:**
- **WeatherData**: Weather conditions from Open-Meteo API
- **WeatherAnalysis**: Analyzed flying conditions with safety recommendations
- **TFR**: Temporary Flight Restriction data from FAA
- **TFRCheck**: TFR search results around home location
- **DroneFlightReport**: Complete flight conditions report for email

## Configuration

Copy `config.example.yaml` to `config.yaml` and configure with your settings.

The configuration is now organized by agent with shared components at the root level:

- **Shared Components** (used by all agents):
  - `email`: SMTP configuration for notifications
  - `monitoring`: Health check endpoints

- **YouTube Curator Agent** (`youtube_curator`):
  - `youtube`: OAuth credentials and token management
  - `ai`: Gemini API configuration
  - `video`: Duration filtering preferences
  - `guidelines`: Content analysis criteria
  - `schedule`: Agent-specific cron schedule

- **Drone Weather Agent** (`drone_weather`):
  - Location and weather threshold settings
  - TFR monitoring configuration
  - `schedule`: Agent-specific cron schedule

Required environment variables:
- `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET`: YouTube OAuth credentials (YouTube Curator only)
- `GEMINI_API_KEY`: Google AI Studio API key (YouTube Curator only)
- `EMAIL_USERNAME` / `EMAIL_PASSWORD`: SMTP credentials (required for both agents)

Optional environment variables:
- `CONFIG_FILE`: Custom config file path (default: `./config.yaml`)
- `HEALTHCHECK_PORT`: Health monitoring port for both app and Docker (default: 8080)

### Drone Weather Agent Configuration

The Drone Weather Agent requires configuration of your home location and safety thresholds:

```yaml
# Shared configuration used by all agents
email:
  smtp_server: "smtp.mail.me.com"
  smtp_port: 587
  username: "" # Set via EMAIL_USERNAME env var
  password: "" # Set via EMAIL_PASSWORD env var
  from_email: "your@email.com"
  to_email: "notifications@yourdomain.com"

monitoring:
  health_port: 8080

# YouTube Curator Agent Configuration
youtube_curator:
  youtube:
    client_id: "" # Set via GOOGLE_CLIENT_ID env var
    client_secret: "" # Set via GOOGLE_CLIENT_SECRET env var
    token_file: "data/youtube_token.json"
    token_refresh_minutes: 30

  ai:
    gemini_api_key: "" # Set via GEMINI_API_KEY env var
    model: "gemini-2.5-flash"

  video:
    short_minutes: 1
    long_minutes: 60

  guidelines:
    criteria:
      - "Educational content about programming, technology, or software development"
      - "High-quality tutorials or explanations of complex topics"
      # ... add your criteria here

  schedule: "0 0 9 * * *" # Daily at 9 AM

# Drone Weather Agent Configuration
drone_weather:
  # Your home flying location
  home_latitude: 37.7749
  home_longitude: -122.4194
  home_name: "San Francisco Bay Area"

  # TFR search radius around home location
  search_radius_miles: 25

  # Weather safety thresholds
  max_wind_speed_kmh: 25    # 25 km/h wind speed limit
  min_visibility_km: 5      # 5 km visibility requirement
  max_precipitation_mm: 0   # No precipitation allowed
  min_temp_c: 4.4          # 4.4°C minimum temperature
  max_temp_c: 35.0         # 35°C maximum temperature

  # API endpoints (defaults provided)
  weather_url: "https://api.open-meteo.com/v1/forecast"
  tfr_url: "https://tfr.faa.gov/tfr2/list.html"

  schedule: "0 0 9 * * *" # Daily at 9 AM
```

**Key Configuration Parameters:**
- **Location Settings**: Configure `drone_weather.home_latitude`, `drone_weather.home_longitude`, and `drone_weather.home_name` for your primary flying location
- **Safety Thresholds**: Adjust weather limits based on your drone capabilities and skill level
- **TFR Monitoring**: Set `drone_weather.search_radius_miles` to define how far to check for temporary flight restrictions
- **API Endpoints**: Use default endpoints or customize for different weather/TFR data sources
- **Schedules**: Each agent now has its own schedule configuration allowing independent timing

### Video Filtering Configuration

The YouTube Curator agent includes video duration filters to skip very short or very long videos:

- `youtube_curator.video.short_minutes`: Skip videos shorter than this duration (default: 1 minute)
- `youtube_curator.video.long_minutes`: Skip videos longer than this duration (default: 60 minutes)

This helps focus analysis on substantive content while avoiding shorts and overly long videos.

### YouTube Token Management

The application automatically manages YouTube OAuth tokens to prevent expiration:

- **Automatic Token Refresh**: Tokens are automatically refreshed when they expire during API calls
- **Background Refresh**: A background goroutine refreshes tokens every 30 minutes (configurable via `youtube_curator.youtube.token_refresh_minutes`)
- **Persistent Storage**: Refreshed tokens are immediately saved to disk at `youtube_curator.youtube.token_file` location
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

**Agent-specific schedules**:
- Configure `youtube_curator.schedule` for YouTube Curator agent timing
- Configure `drone_weather.schedule` for Drone Weather agent timing
- Each agent runs independently according to its own schedule

## API Setup

### YouTube Curator Agent

#### YouTube OAuth
1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create project → Enable YouTube Data API v3
3. Create OAuth 2.0 credentials of type `TVs and Limited Input devices`
4. Set `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` environment variables (some device clients do not issue a secret; leave it blank if not provided)

#### Gemini AI
1. Go to [Google AI Studio](https://makersuite.google.com/app/apikey)
2. Create API key → Set `GEMINI_API_KEY` environment variable

First run will prompt for OAuth authorization via browser flow.

### Drone Weather Agent

#### Open-Meteo Weather API
- **Free API**: No API key required for basic usage
- **Endpoint**: `https://api.open-meteo.com/v1/forecast`
- **Features**: Current weather, hourly forecasts, multiple weather variables
- **Rate Limits**: 10,000 API calls per day (free tier)

#### FAA TFR Data
- **Public Data**: No API key required
- **Endpoint**: `https://tfr.faa.gov/tfr2/list.html`
- **Data Source**: Official FAA Temporary Flight Restrictions
- **Update Frequency**: Real-time updates from FAA systems
- **Coverage**: United States airspace only

The Drone Weather Agent uses public APIs that don't require authentication, making setup simpler than the YouTube Curator.

## Development

### Local Development

#### YouTube Curator Agent
```bash
go mod download
go run agents/youtube-curator/cmd/main.go --once
```

#### Drone Weather Agent
```bash
go mod download
go run cmd/main.go --agent=drone-weather --once
```

### Docker
```bash
docker-compose up -d
# Test YouTube Curator: docker run --env-file .env agent-stack ./youtube-curator --once
# Test Drone Weather: docker run --env-file .env agent-stack ./drone-weather --once
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

## Drone Weather Agent Implementation

### Weather Analysis Process

1. **Data Collection**: Fetches current weather and 24-hour forecast from Open-Meteo API
2. **Safety Analysis**: Compares weather conditions against configured thresholds
3. **TFR Checking**: Searches for active Temporary Flight Restrictions within configured radius
4. **Decision Logic**: Determines if conditions are safe for drone flying based on weather only
5. **Notifications**: Sends email alerts with detailed reports when conditions are favorable

### Weather Monitoring Features

- **Real-time Data**: Current weather conditions with timezone-aware timestamps
- **Hourly Forecasts**: Wind speed and gust predictions for next 24 hours
- **Visual Charts**: QuickChart.io integration for wind speed visualization
- **Multi-unit Support**: Displays both metric and imperial units for temperature and wind
- **Comprehensive Checks**: Wind speed, visibility, precipitation, and temperature analysis

### TFR Integration

- **FAA Data Source**: Parses official FAA Temporary Flight Restriction data
- **Geographical Filtering**: Identifies TFRs within configurable radius of home location
- **Informational Only**: TFRs are shown as warnings, not blocking factors for good weather notifications
- **Fallback Handling**: Continues operation even if TFR data is unavailable

### Email Notifications

- **Conditional Sending**: Only sends emails when weather conditions are good for flying
- **Rich HTML Format**: Styled email template with weather details and wind charts
- **Comprehensive Reports**: Includes current conditions, forecasts, TFR status, and safety recommendations
- **SMTP Flexibility**: Supports various email providers with TLS encryption

### Safety Features

- **Conservative Defaults**: Safe thresholds for beginner/intermediate pilots
- **Configurable Limits**: Easily adjust weather thresholds based on experience and equipment
- **Multiple Factors**: Considers wind, visibility, precipitation, and temperature simultaneously
- **Timezone Handling**: Properly handles timezone conversion for accurate time displays
