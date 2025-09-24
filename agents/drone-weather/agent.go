package droneweather

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log"
	"os"
	"time"

	"agent-stack/internal/models"
	"agent-stack/shared/config"
	"agent-stack/shared/email"
	"agent-stack/shared/scheduler"
)

// DroneMetrics represents the metrics collected during a drone weather check
type DroneMetrics struct {
	WeatherFetched bool `json:"weather_fetched"`
	TFRsChecked    bool `json:"tfrs_checked"`
	IsFlyable      bool `json:"is_flyable"`
	EmailSent      bool `json:"email_sent"`
}

// GetSummary implements the scheduler.Metrics interface
func (m DroneMetrics) GetSummary() string {
	if m.IsFlyable && m.EmailSent {
		return "good weather conditions detected, email sent with TFR info"
	} else if m.IsFlyable {
		return "good weather conditions detected, no email sent"
	} else {
		return "poor weather conditions, no email sent"
	}
}

// DroneWeatherAgent implements the scheduler.Agent interface
type DroneWeatherAgent struct {
	config        *config.Config
	weatherClient *WeatherClient
	tfrClient     *TFRClient
	emailSender   *email.Sender
}

func NewDroneWeatherAgent(cfg *config.Config) *DroneWeatherAgent {
	return &DroneWeatherAgent{
		config: cfg,
	}
}

func (d *DroneWeatherAgent) Name() string {
	return "Drone Weather Agent"
}

func (d *DroneWeatherAgent) GetSchedule() string {
	return d.config.DroneWeather.Schedule
}

func (d *DroneWeatherAgent) Initialize() error {
	log.Printf("Initializing %s...", d.Name())

	if d.weatherClient == nil {
		d.weatherClient = NewWeatherClient(&d.config.DroneWeather)
		log.Println("Weather client initialized")
	}

	if d.tfrClient == nil {
		d.tfrClient = NewTFRClient(&d.config.DroneWeather)
		log.Println("TFR client initialized")
	}

	if d.emailSender == nil {
		d.emailSender = email.NewSender(&d.config.Email)
		log.Println("Email sender initialized")
	}

	// Validate required configuration
	if d.config.DroneWeather.HomeLatitude == 0 || d.config.DroneWeather.HomeLongitude == 0 {
		return fmt.Errorf("home coordinates must be configured (home_latitude and home_longitude)")
	}

	if d.config.DroneWeather.HomeName == "" {
		return fmt.Errorf("home location name must be configured (home_name)")
	}

	log.Printf("Configured for %s (%.4f, %.4f)",
		d.config.DroneWeather.HomeName,
		d.config.DroneWeather.HomeLatitude,
		d.config.DroneWeather.HomeLongitude)

	return nil
}

func (d *DroneWeatherAgent) RunOnce(ctx context.Context, events *scheduler.AgentEvents) error {
	startTime := time.Now()
	metrics := DroneMetrics{}

	// Fetch weather data
	log.Println("Fetching weather data...")
	weatherData, err := d.weatherClient.GetCurrentWeather(ctx,
		d.config.DroneWeather.HomeLatitude,
		d.config.DroneWeather.HomeLongitude)
	if err != nil {
		if events != nil && events.OnCriticalFailure != nil {
			events.OnCriticalFailure(fmt.Errorf("failed to fetch weather data: %w", err), time.Since(startTime))
		}
		return fmt.Errorf("failed to fetch weather data: %w", err)
	}
	metrics.WeatherFetched = true

	// Analyze weather conditions
	weatherAnalysis := d.weatherClient.AnalyzeWeatherConditions(weatherData)
	log.Printf("Weather analysis: flyable=%t, temp=%.1f°C, wind=%.1f km/h, visibility=%.1f km, time=%s",
		weatherAnalysis.IsFlyable, weatherData.Temperature, weatherData.WindSpeed,
		weatherData.Visibility, weatherData.Time.Format("15:04 MST"))

	// Check TFRs
	log.Println("Checking TFRs...")
	tfrCheck, err := d.tfrClient.CheckTFRs(ctx,
		d.config.DroneWeather.HomeLatitude,
		d.config.DroneWeather.HomeLongitude)
	if err != nil {
		// TFR check failure is not critical - we can still make decisions based on weather
		if events != nil && events.OnPartialFailure != nil {
			events.OnPartialFailure(fmt.Errorf("failed to check TFRs: %w", err), time.Since(startTime))
		}
		log.Printf("Warning: Failed to check TFRs: %v", err)

		// Create a default TFR check when API fails
		tfrCheck = &models.TFRCheck{
			HasActiveTFRs: true, // Mark as having TFRs when check fails (informational warning)
			ActiveTFRs:    []*models.TFR{},
			CheckRadius:   d.config.DroneWeather.SearchRadiusMiles,
			CheckTime:     time.Now(),
			Summary:       "TFR check failed - verify airspace restrictions manually before flying",
		}
	} else {
		metrics.TFRsChecked = true
	}

	log.Printf("TFR check: %s", tfrCheck.Summary)

	// Determine if flying conditions are good based on weather only
	// TFRs are informational - pilots can still fly outside restricted areas
	isFlyable := weatherAnalysis.IsFlyable
	metrics.IsFlyable = isFlyable

	// Send email if weather conditions are good (TFRs are shown as informational)
	if isFlyable {
		log.Println("Conditions are good for flying - sending email notification...")

		report := &models.DroneFlightReport{
			Date:            time.Now(),
			LocationName:    d.config.DroneWeather.HomeName,
			WeatherAnalysis: weatherAnalysis,
			TFRCheck:        tfrCheck,
			IsFlyable:       true,
			Summary:         "Excellent conditions for drone flying!",
		}

		body, err := d.generateEmailBody(report)
		if err != nil {
			if events != nil && events.OnCriticalFailure != nil {
				events.OnCriticalFailure(fmt.Errorf("failed to generate email body: %w", err), time.Since(startTime))
			}
			return fmt.Errorf("failed to generate email body: %w", err)
		}

		subject := fmt.Sprintf("Good Day for Drone Flying in %s", report.LocationName)
		if err := d.emailSender.SendHTML(subject, body); err != nil {
			if events != nil && events.OnCriticalFailure != nil {
				events.OnCriticalFailure(fmt.Errorf("failed to send email report: %w", err), time.Since(startTime))
			}
			return fmt.Errorf("failed to send email report: %w", err)
		}
		metrics.EmailSent = true
	} else {
		log.Println("Conditions not suitable for flying - no email sent")

		// Log reasons why not flyable (weather only)
		for _, reason := range weatherAnalysis.Reasons {
			log.Printf("Weather issue: %s", reason)
		}
	}

	// Record successful completion
	duration := time.Since(startTime)
	if events != nil && events.OnSuccess != nil {
		events.OnSuccess(metrics, duration)
	}

	log.Printf("Drone weather check complete: flyable=%t, email_sent=%t", metrics.IsFlyable, metrics.EmailSent)

	return nil
}

// generateEmailBody creates HTML email content for drone weather report
func (d *DroneWeatherAgent) generateEmailBody(report *models.DroneFlightReport) (string, error) {
	// Read template from external file
	templatePath := "agents/drone-weather/email_template.html"
	tmplBytes, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to read email template: %w", err)
	}

	tmpl, err := template.New("email").Parse(string(tmplBytes))
	if err != nil {
		return "", fmt.Errorf("failed to parse email template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, report); err != nil {
		return "", fmt.Errorf("failed to execute email template: %w", err)
	}

	return buf.String(), nil
}
