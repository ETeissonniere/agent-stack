package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

type Config struct {
	YouTube      YouTubeConfig      `yaml:"youtube"`
	AI           AIConfig           `yaml:"ai"`
	Email        EmailConfig        `yaml:"email"`
	Guidelines   GuidelinesConfig   `yaml:"guidelines"`
	Schedule     string             `yaml:"schedule"`
	Monitoring   MonitoringConfig   `yaml:"monitoring"`
	Video        VideoConfig        `yaml:"video"`
	DroneWeather DroneWeatherConfig `yaml:"drone_weather"`
}

type YouTubeConfig struct {
	ClientID            string `yaml:"client_id" env:"GOOGLE_CLIENT_ID"`
	ClientSecret        string `yaml:"client_secret" env:"GOOGLE_CLIENT_SECRET"`
	TokenFile           string `yaml:"token_file"`
	TokenRefreshMinutes int    `yaml:"token_refresh_minutes"`
}

type AIConfig struct {
	GeminiAPIKey string `yaml:"gemini_api_key" env:"GEMINI_API_KEY"`
	Model        string `yaml:"model"`
}

type EmailConfig struct {
	SMTPServer string `yaml:"smtp_server"`
	SMTPPort   int    `yaml:"smtp_port"`
	Username   string `yaml:"username" env:"EMAIL_USERNAME"`
	Password   string `yaml:"password" env:"EMAIL_PASSWORD"`
	FromEmail  string `yaml:"from_email"`
	ToEmail    string `yaml:"to_email"`
}

type GuidelinesConfig struct {
	Criteria []string `yaml:"criteria"`
}

type MonitoringConfig struct {
	HealthPort int `yaml:"health_port"`
}

type VideoConfig struct {
	ShortMinutes int `yaml:"short_minutes"`
	LongMinutes  int `yaml:"long_minutes"`
}

type DroneWeatherConfig struct {
	HomeLatitude       float64 `yaml:"home_latitude"`
	HomeLongitude      float64 `yaml:"home_longitude"`
	HomeName           string  `yaml:"home_name"`
	SearchRadiusMiles  int     `yaml:"search_radius_miles"`
	MaxWindSpeedKmh    int     `yaml:"max_wind_speed_kmh"`
	MinVisibilityKm    int     `yaml:"min_visibility_km"`
	MaxPrecipitationMm float64 `yaml:"max_precipitation_mm"`
	MinTempC           float64 `yaml:"min_temp_c"`
	MaxTempC           float64 `yaml:"max_temp_c"`
	WeatherURL         string  `yaml:"weather_url"`
	TFRURL             string  `yaml:"tfr_url"`
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	configFile := os.Getenv("CONFIG_FILE")
	if configFile == "" {
		configFile = "config.yaml"
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configFile, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", configFile, err)
	}

	if cfg.YouTube.ClientID == "" {
		cfg.YouTube.ClientID = os.Getenv("GOOGLE_CLIENT_ID")
	}
	if cfg.YouTube.ClientSecret == "" {
		cfg.YouTube.ClientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")
	}
	if cfg.YouTube.TokenFile == "" {
		cfg.YouTube.TokenFile = "data/youtube_token.json"
	}
	if cfg.YouTube.TokenRefreshMinutes == 0 {
		cfg.YouTube.TokenRefreshMinutes = 30 // Default to 30 minutes
	}
	if cfg.AI.GeminiAPIKey == "" {
		cfg.AI.GeminiAPIKey = os.Getenv("GEMINI_API_KEY")
	}
	if cfg.Email.Username == "" {
		cfg.Email.Username = os.Getenv("EMAIL_USERNAME")
	}
	if cfg.Email.Password == "" {
		cfg.Email.Password = os.Getenv("EMAIL_PASSWORD")
	}

	// No external monitoring services - self-contained only

	if cfg.AI.Model == "" {
		cfg.AI.Model = "gemini-2.5-flash"
	}
	if cfg.Video.LongMinutes == 0 {
		cfg.Video.LongMinutes = 60
	}
	if cfg.Video.ShortMinutes == 0 {
		cfg.Video.ShortMinutes = 1
	}
	if cfg.Schedule == "" {
		// 6-field cron with seconds: daily at 09:00:00
		cfg.Schedule = "0 0 9 * * *"
	}

	if cfg.Monitoring.HealthPort == 0 {
		cfg.Monitoring.HealthPort = 8080
	}

	// Optional override via environment variable to align Docker healthchecks.
	// Use a single variable name to avoid confusion.
	if v := os.Getenv("HEALTHCHECK_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			cfg.Monitoring.HealthPort = p
		}
	}

	// Set defaults for drone weather configuration
	if cfg.DroneWeather.WeatherURL == "" {
		cfg.DroneWeather.WeatherURL = "https://api.open-meteo.com/v1/forecast"
	}
	if cfg.DroneWeather.TFRURL == "" {
		cfg.DroneWeather.TFRURL = "https://tfr.faa.gov/tfr2/list.html"
	}
	if cfg.DroneWeather.MaxWindSpeedKmh == 0 {
		cfg.DroneWeather.MaxWindSpeedKmh = 25 // ~15 mph converted to km/h
	}
	if cfg.DroneWeather.MinVisibilityKm == 0 {
		cfg.DroneWeather.MinVisibilityKm = 5 // ~3 miles converted to km
	}
	if cfg.DroneWeather.MaxPrecipitationMm == 0 {
		cfg.DroneWeather.MaxPrecipitationMm = 0
	}
	if cfg.DroneWeather.MinTempC == 0 {
		cfg.DroneWeather.MinTempC = 4.4 // 40°F in Celsius
	}
	if cfg.DroneWeather.MaxTempC == 0 {
		cfg.DroneWeather.MaxTempC = 35.0 // 95°F in Celsius
	}
	if cfg.DroneWeather.SearchRadiusMiles == 0 {
		cfg.DroneWeather.SearchRadiusMiles = 25
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	if c.YouTube.ClientID == "" {
		return fmt.Errorf("YouTube client ID is required (set GOOGLE_CLIENT_ID or youtube.client_id)")
	}
	if c.AI.GeminiAPIKey == "" {
		return fmt.Errorf("Gemini API key is required (set GEMINI_API_KEY or ai.gemini_api_key)")
	}
	if c.Email.Username == "" {
		return fmt.Errorf("Email username is required (set EMAIL_USERNAME or email.username)")
	}
	if c.Email.Password == "" {
		return fmt.Errorf("Email password is required (set EMAIL_PASSWORD or email.password)")
	}
	return nil
}
