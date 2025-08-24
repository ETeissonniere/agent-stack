package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

type Config struct {
	YouTube    YouTubeConfig    `yaml:"youtube"`
	AI         AIConfig         `yaml:"ai"`
	Email      EmailConfig      `yaml:"email"`
	Guidelines GuidelinesConfig `yaml:"guidelines"`
	Schedule   string           `yaml:"schedule"`
}

type YouTubeConfig struct {
	ClientID     string `yaml:"client_id" env:"GOOGLE_CLIENT_ID"`
	ClientSecret string `yaml:"client_secret" env:"GOOGLE_CLIENT_SECRET"`
	TokenFile    string `yaml:"token_file"`
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
		cfg.YouTube.TokenFile = "youtube_token.json"
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
	if cfg.Schedule == "" {
		cfg.Schedule = "0 9 * * *" // Daily at 9 AM
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
	if c.YouTube.ClientSecret == "" {
		return fmt.Errorf("YouTube client secret is required (set GOOGLE_CLIENT_SECRET or youtube.client_secret)")
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
