# Agent Stack

A collection of intelligent automation agents that automate daily tasks and notifications.

## Agents

### 🎥 YouTube Curator
Analyzes your YouTube subscriptions using AI to find the most relevant videos worth watching.

**Features:**
- 🤖 **AI-Powered Analysis**: Uses Google Gemini 2.5 to analyze video content, titles, and transcripts
- 📺 **YouTube Integration**: Automatically fetches new videos from your subscriptions
- 📧 **Smart Email Digests**: Sends beautifully formatted HTML emails with relevant videos
- ⚙️ **Customizable Criteria**: Define your own guidelines for video selection
- ⏰ **Scheduled Execution**: Runs daily via cron to keep you updated
- 🗃️ **Smart Deduplication**: Avoids re-analyzing videos from the last 7 days
- ⚡ **Content Filtering**: Automatically filters out YouTube Shorts (≤60 seconds) to focus on substantive content
- 🎬 **Long Video Handling**: Special metadata-only analysis for extra-long videos (>1 hour) to avoid token limits
- 🔄 **Automatic Token Refresh**: YouTube OAuth tokens are automatically refreshed to prevent expiration

**Example Email Output:**

![Email Example](images/email-example.png)

### 🚁 Drone Weather Agent
Monitors weather conditions and airspace restrictions to determine safe drone flying conditions.

**Features:**
- 🌤️ **Weather Monitoring**: Fetches real-time weather data from Open-Meteo API
- ✈️ **TFR Checking**: Monitors FAA Temporary Flight Restrictions in your area
- 📊 **Safety Analysis**: Analyzes wind speed, visibility, precipitation, and temperature against safe flying thresholds
- 📈 **Wind Charts**: Generates visual wind speed forecasts using QuickChart
- 📧 **Smart Notifications**: Sends email alerts only when conditions are good for flying
- ⚙️ **Configurable Thresholds**: Customize weather limits based on your drone and skill level
- 🌍 **Location-Based**: Configure for any location with latitude/longitude coordinates

**Weather Criteria Monitored:**
- Wind speed (default max: 15 mph)
- Visibility (default min: 3 miles)
- Precipitation (default max: 0 mm)
- Temperature range (default: 40-95°F / 4.4-35°C)
- Active TFRs within configurable radius (default: 25 miles)

## Features

- 🐳 **Docker Ready**: Optimized for deployment on Raspberry Pi and other platforms
- 🔒 **Secure**: API keys managed via environment variables  
- 📊 **Built-in Monitoring**: Health checks and status tracking
- ⏰ **Scheduled Execution**: Runs daily via cron scheduler

## Quick Start

### Prerequisites

1. **YouTube OAuth 2.0 Credentials**
   - Go to [Google Cloud Console](https://console.cloud.google.com/)
   - Enable YouTube Data API v3
   - Create OAuth 2.0 credentials (`TVs and Limited Input devices`)
   - Configure OAuth consent screen with YouTube readonly scope
   - Using the older Desktop credential type will cause Google to return `invalid_request` during device authorization

2. **Google AI Studio API Key**
   - Visit [Google AI Studio](https://makersuite.google.com/app/apikey)
   - Create a new API key for Gemini access

3. **Email Configuration**
   - For iCloud: Enable 2FA and create an app-specific password
   - For other providers: Ensure SMTP access is enabled

### Installation

#### Option 1: Docker (Recommended)

```bash
# Clone the repository
git clone https://github.com/ETeissonniere/agent-stack.git
cd agent-stack

# Create environment file
cp .env.example .env
# Edit .env with your OAuth credentials and email settings

# Configure your preferences, edit config.yaml with your criteria and settings

# Build the container
docker build -t agent-stack .

# First-time setup (interactive OAuth authorization)
# The data volume persists OAuth tokens and video analysis state
docker run --rm -it --env-file .env \
  -v $(pwd)/config.yaml:/app/config.yaml:ro \
  -v $(pwd)/data:/app/data \
  agent-stack ./youtube-curator --once

# After OAuth setup, run with Docker Compose
docker-compose up -d
```

> **Prebuilt image:** GitHub Actions publishes multi-architecture images (amd64/arm64/armv7) to `ghcr.io/eteissonniere/agent-stack`. Docker Compose already references this image, so `docker-compose up -d` will pull it automatically. Run `docker-compose pull` to refresh.

```bash
# Fetch the published image
docker pull ghcr.io/eteissonniere/agent-stack:latest

# First-time setup using the published image (same volume mounts as above)
docker run --rm -it --env-file .env \
  -v $(pwd)/config.yaml:/app/config.yaml:ro \
  -v $(pwd)/data:/app/data \
  ghcr.io/eteissonniere/agent-stack:latest ./youtube-curator --once
```

If you prefer to build locally (e.g., after code changes), run `docker-compose build` to rebuild the image before starting the stack.

#### Option 2: Local Build

```bash
# Clone and build
git clone https://github.com/ETeissonniere/agent-stack.git
cd agent-stack
go build -o youtube-curator ./agents/youtube-curator/cmd

# Set environment variables
export GOOGLE_CLIENT_ID="your-client-id.apps.googleusercontent.com"
export GOOGLE_CLIENT_SECRET="your-client-secret"
export GEMINI_API_KEY="your-gemini-api-key"
export EMAIL_USERNAME="your-email@icloud.com"
export EMAIL_PASSWORD="your-app-specific-password"

# Run once to test
./youtube-curator --once
```

## Configuration

### Environment Variables

Create a `.env` file:

```bash
GOOGLE_CLIENT_ID=your_client_id_here.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=your_client_secret_here
GEMINI_API_KEY=your_gemini_api_key_here
EMAIL_USERNAME=your-email@icloud.com
EMAIL_PASSWORD=your_app_specific_password
```

### Configuration File

Edit `config.yaml`:

```yaml
youtube:
  client_id: "" # Set via GOOGLE_CLIENT_ID env var
  client_secret: "" # Set via GOOGLE_CLIENT_SECRET env var
  token_file: "data/youtube_token.json"
  token_refresh_minutes: 30 # Auto-refresh tokens every 30 minutes

ai:
  gemini_api_key: "" # Set via GEMINI_API_KEY env var
  model: "gemini-2.5-flash"

email:
  smtp_server: "smtp.mail.me.com"  # iCloud SMTP
  smtp_port: 587
  username: "" # Set via EMAIL_USERNAME env var
  password: "" # Set via EMAIL_PASSWORD env var
  from_email: "your-email@icloud.com"
  to_email: "your-email@icloud.com"

guidelines:
  criteria:
    - "Educational content about programming, technology, or software development"
    - "High-quality tutorials or explanations of complex topics"
    - "Industry insights from experienced professionals"
    - "New technology announcements or reviews"
    - "Content that would help with professional development"
    - "Avoid clickbait or overly promotional content"
    - "Prefer content from established creators with good reputation"

schedule: "0 0 9 * * *" # Daily at 9 AM

monitoring:
  # Port for health endpoints `/health` and `/status`
  health_port: 8080

video:
  # Skip videos at or below this length
  short_minutes: 1
  # Fallback to metadata-only above this duration
  long_minutes: 60

drone_weather:
  # Your home flying location
  home_latitude: 37.7749
  home_longitude: -122.4194
  home_name: "San Francisco Bay Area"

  # TFR search radius around home location
  search_radius_miles: 25

  # Weather safety thresholds
  max_wind_speed_mph: 15
  min_visibility_miles: 3
  max_precipitation_mm: 0
  min_temp_c: 4.4   # 40°F
  max_temp_c: 35.0  # 95°F

  # API endpoints (defaults provided)
  weather_url: "https://api.open-meteo.com/v1/forecast"
  tfr_url: "https://tfr.faa.gov/tfr2/list.html"
```

### Video Settings

 - `short_minutes`: Minutes threshold to skip short videos (e.g., YouTube Shorts). Defaults to 1.
 - `long_minutes`: Minutes threshold to switch to metadata-only analysis for very long videos. Defaults to 60.

### Drone Weather Settings

Configure the drone weather agent for your location and safety preferences:

 - `home_latitude`/`home_longitude`: Your primary flying location coordinates
 - `home_name`: Descriptive name for your location (used in emails)
 - `search_radius_miles`: Radius to check for TFRs around your location (default: 25)
 - `max_wind_speed_mph`: Maximum safe wind speed for flying (default: 15)
 - `min_visibility_miles`: Minimum required visibility (default: 3)
 - `max_precipitation_mm`: Maximum precipitation allowed (default: 0)
 - `min_temp_c`/`max_temp_c`: Safe temperature range in Celsius

### YouTube Token Management

The application automatically manages YouTube OAuth tokens:
- **Automatic Refresh**: Tokens refresh automatically when expired during API calls
- **Background Refresh**: Runs every 30 minutes (configurable via `token_refresh_minutes`)
- **Persistent Storage**: Refreshed tokens are saved to disk immediately
- **No Manual Re-auth**: Once authenticated, tokens stay valid indefinitely

## Usage

### Commands

```bash
# Run once for testing
./youtube-curator --once

# Run with scheduler (default)
./youtube-curator
```

### Docker Commands

```bash
# View logs
docker logs youtube-curator

# Run once in container
docker-compose run youtube-curator ./youtube-curator --once

# Rebuild after changes
docker-compose up --build
```

## Email Setup

### iCloud Mail Configuration

1. Enable two-factor authentication for your Apple ID
2. Generate an app-specific password:
   - Go to appleid.apple.com
   - Sign in and go to Security section
   - Generate password for "Agent YouTube"
3. Use your full iCloud email as username
4. Use the generated password as the password

### Other Email Providers

- **Gmail**: Use app passwords with 2FA enabled
- **Outlook**: Use app passwords or OAuth2
- **Custom SMTP**: Update server and port in config

## Customization

### Video Selection Criteria

Modify the `guidelines.criteria` in `config.yaml` to match your interests:

```yaml
guidelines:
  criteria:
    - "Machine learning and AI tutorials"
    - "Software architecture discussions"
    - "DevOps and infrastructure content"
    - "Startup and business insights"
    - "Avoid content shorter than 10 minutes"
    - "Prefer channels with over 10k subscribers"
```

### Scheduling

The application uses a 6-field CRON format with seconds. Common examples:
- `"0 0 9 * * *"` - Daily at 9:00 AM
- `"0 30 8 * * 1"` - Mondays at 8:30 AM
- `"0 0 */6 * * *"` - Every 6 hours

For complete CRON format documentation, see `CLAUDE.md`.

### Monitoring

- Endpoints: `/health` (200/503) and `/status` (plain text summary)
- Port: configured via `monitoring.health_port` (default 8080)
- Docker healthchecks: configurable via a single `HEALTHCHECK_PORT` environment variable used by both the app (override) and Docker healthchecks.
  - To change the port in Docker: set `HEALTHCHECK_PORT=9090` in `.env` or your shell
  - Alternatively, change `monitoring.health_port` in `config.yaml` and set `HEALTHCHECK_PORT` to match

### AI Model Selection

You can change the Gemini model in config:
- `gemini-2.5-flash` (recommended, fastest with latest features)
- `gemini-2.5-flash-lite` (ultra-fast, lower cost option)
- `gemini-2.5-pro` (most advanced with thinking capabilities)

## Troubleshooting

### Common Issues

**No videos found:**
- Verify YouTube API key is correct
- Check if you have public subscriptions
- Ensure API quotas aren't exceeded

**Token expired:**
- The app now automatically refreshes tokens
- If issues persist, delete `data/youtube_token.json` and re-authenticate
- Check logs for token refresh errors

**Email not sending:**
- Verify SMTP credentials
- Check app-specific password for iCloud
- Test with `--once` flag for detailed logs

**AI analysis failing:**
- Verify Gemini API key
- Check API rate limits
- Ensure model name is correct

**Transcript errors:**
- Some videos don't have transcripts available
- Non-English videos may have limited transcript support
- Analysis continues without transcript if unavailable

### Logs

Check application logs for detailed error information:

```bash
# Docker logs
docker logs youtube-curator -f

# Local logs
# Application logs to stdout
```

Health check server listens on port 8080 by default. Configure via `monitoring.health_port` in `config.yaml`.

## Security Notes

- Store API keys in environment variables only
- Use app-specific passwords for email accounts
- Regularly rotate API keys
- Monitor API usage for unexpected charges

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

This project is open source. See LICENSE file for details.

## Support

For issues and questions:
1. Check the troubleshooting section
2. Review logs for error details
3. Open an issue with reproduction steps

## Project Structure

```
agent-stack/
├── agents/                    # Individual agent implementations
│   ├── youtube-curator/       # YouTube curation agent
│   │   ├── cmd/               # Agent entry point
│   │   ├── youtube/           # YouTube API client
│   │   └── agent.go           # Main agent implementation
│   └── drone-weather/         # Drone weather monitoring agent
│       ├── weather.go         # Weather API client (Open-Meteo)
│       ├── tfr.go             # TFR checking (FAA)
│       ├── agent.go           # Main agent implementation
│       └── email_template.html # Email template for flight reports
├── shared/                    # Shared libraries
│   ├── config/                # Configuration management
│   ├── monitoring/            # Health checks and monitoring
│   ├── email/                 # Email notifications
│   ├── storage/               # Persistent state management
│   └── ai/                    # AI/LLM integrations
├── internal/                  # Shared data models
│   └── models/                # Common data structures (weather, TFR, etc.)
├── data/                      # Persistent data (OAuth tokens, video state)
├── docker-compose.yml         # Container orchestration
└── config.yaml              # Application configuration
```

## Architecture

See `CLAUDE.md` for detailed technical documentation, including:
- Component architecture
- API integration details
- Development guidelines
- Security considerations
