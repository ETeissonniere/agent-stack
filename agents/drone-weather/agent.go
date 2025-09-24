package droneweather

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log"
	"net/smtp"
	"time"

	"agent-stack/internal/models"
	"agent-stack/shared/config"
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
		return "good flying conditions detected, email sent"
	} else if m.IsFlyable {
		return "good flying conditions detected, no email sent"
	} else {
		return "poor flying conditions, no email sent"
	}
}

// DroneWeatherAgent implements the scheduler.Agent interface
type DroneWeatherAgent struct {
	config        *config.Config
	weatherClient *WeatherClient
	tfrClient     *TFRClient
}

func NewDroneWeatherAgent(cfg *config.Config) *DroneWeatherAgent {
	return &DroneWeatherAgent{
		config: cfg,
	}
}

func (d *DroneWeatherAgent) Name() string {
	return "Drone Weather Agent"
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
	log.Printf("Weather analysis: flyable=%t, temp=%.1f°F, wind=%.1f mph, visibility=%.1f mi",
		weatherAnalysis.IsFlyable, weatherAnalysis.TempF, weatherAnalysis.WindSpeedMph, weatherAnalysis.VisibilityMi)

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

		// Create a safe default TFR check that assumes TFRs might be active
		tfrCheck = &models.TFRCheck{
			HasActiveTFRs: true, // Assume TFRs present when check fails (safe default)
			ActiveTFRs:    []*models.TFR{},
			CheckRadius:   d.config.DroneWeather.SearchRadiusMiles,
			CheckTime:     time.Now(),
			Summary:       "TFR check failed - assuming restrictions present for safety",
		}
	} else {
		metrics.TFRsChecked = true
	}

	log.Printf("TFR check: %s", tfrCheck.Summary)

	// Determine if flying conditions are good
	isFlyable := weatherAnalysis.IsFlyable && !tfrCheck.HasActiveTFRs
	metrics.IsFlyable = isFlyable

	// Only send email if conditions are good for flying
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

		if err := d.sendEmailReport(report); err != nil {
			if events != nil && events.OnCriticalFailure != nil {
				events.OnCriticalFailure(fmt.Errorf("failed to send email report: %w", err), time.Since(startTime))
			}
			return fmt.Errorf("failed to send email report: %w", err)
		}
		metrics.EmailSent = true
	} else {
		log.Println("Conditions not suitable for flying - no email sent")

		// Log reasons why not flyable
		if !weatherAnalysis.IsFlyable {
			for _, reason := range weatherAnalysis.Reasons {
				log.Printf("Weather issue: %s", reason)
			}
		}
		if tfrCheck.HasActiveTFRs {
			log.Printf("TFR issue: %s", tfrCheck.Summary)
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

// sendEmailReport sends a drone weather report via email
func (d *DroneWeatherAgent) sendEmailReport(report *models.DroneFlightReport) error {
	subject := fmt.Sprintf("✈️ Good Day for Drone Flying in %s", report.LocationName)

	body, err := d.generateEmailBody(report)
	if err != nil {
		return fmt.Errorf("failed to generate email body: %w", err)
	}

	return d.sendViaSMTP(subject, body)
}

// sendViaSMTP sends email using SMTP configuration
func (d *DroneWeatherAgent) sendViaSMTP(subject, body string) error {
	auth := smtp.PlainAuth("", d.config.Email.Username, d.config.Email.Password, d.config.Email.SMTPServer)

	to := []string{d.config.Email.ToEmail}
	msg := []byte(fmt.Sprintf(`To: %s
From: %s
Subject: %s
MIME-Version: 1.0
Content-Type: text/html; charset=UTF-8

%s`, d.config.Email.ToEmail, d.config.Email.FromEmail, subject, body))

	addr := fmt.Sprintf("%s:%d", d.config.Email.SMTPServer, d.config.Email.SMTPPort)
	return smtp.SendMail(addr, auth, d.config.Email.FromEmail, to, msg)
}

// generateEmailBody creates HTML email content for drone weather report
func (d *DroneWeatherAgent) generateEmailBody(report *models.DroneFlightReport) (string, error) {
	tmplStr := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Drone Weather Report</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; max-width: 800px; margin: 0 auto; padding: 20px; }
        .header { background-color: #2196F3; color: white; padding: 20px; border-radius: 8px; margin-bottom: 20px; text-align: center; }
        .summary { background-color: #E8F5E8; padding: 15px; border-radius: 8px; margin-bottom: 20px; border-left: 4px solid #4CAF50; }
        .weather { background-color: #f8f9fa; padding: 15px; border-radius: 8px; margin-bottom: 20px; }
        .tfr { background-color: #f8f9fa; padding: 15px; border-radius: 8px; margin-bottom: 20px; }
        .good { color: #4CAF50; font-weight: bold; }
        .warning { color: #FF9800; font-weight: bold; }
        .metric { display: inline-block; margin: 10px 15px 10px 0; }
        .metric-label { font-weight: bold; color: #666; }
        .metric-value { font-size: 18px; color: #2196F3; }
        .footer { text-align: center; color: #666; font-size: 12px; margin-top: 30px; border-top: 1px solid #ddd; padding-top: 15px; }
        .wind-dir { font-size: 14px; color: #666; }
    </style>
</head>
<body>
    <div class="header">
        <h1>🚁 Drone Weather Report</h1>
        <h2>{{.LocationName}}</h2>
        <p>{{.Date.Format "Monday, January 2, 2006 at 3:04 PM"}}</p>
    </div>

    <div class="summary">
        <h2>✅ {{.Summary}}</h2>
        <p><strong>Weather:</strong> {{if .WeatherAnalysis.IsFlyable}}<span class="good">Suitable</span>{{else}}<span class="warning">Not suitable</span>{{end}} for flying</p>
        <p><strong>TFRs:</strong> {{.TFRCheck.Summary}}</p>
    </div>

    <div class="weather">
        <h3>🌤️ Weather Conditions</h3>
        <div class="metric">
            <div class="metric-label">Temperature</div>
            <div class="metric-value">{{printf "%.0f°F" .WeatherAnalysis.TempF}}</div>
        </div>
        <div class="metric">
            <div class="metric-label">Wind Speed</div>
            <div class="metric-value">{{printf "%.0f mph" .WeatherAnalysis.WindSpeedMph}}</div>
        </div>
        <div class="metric">
            <div class="metric-label">Visibility</div>
            <div class="metric-value">{{printf "%.1f mi" .WeatherAnalysis.VisibilityMi}}</div>
        </div>
        <div class="metric">
            <div class="metric-label">Precipitation</div>
            <div class="metric-value">{{printf "%.1f mm" .WeatherAnalysis.Data.Precipitation}}</div>
        </div>

        <p><strong>Wind Forecast:</strong> {{.WeatherAnalysis.WindForecast}}</p>
        <p><strong>Best Flying Window:</strong> {{.WeatherAnalysis.BestWindow}}</p>
        <p class="wind-dir"><strong>Wind Direction:</strong> {{.WeatherAnalysis.Data.WindDir}}°</p>
    </div>

    <div class="tfr">
        <h3>🚫 Airspace Status</h3>
        <p><strong>TFR Check:</strong> {{.TFRCheck.Summary}}</p>
        <p><strong>Search Radius:</strong> {{.TFRCheck.CheckRadius}} miles</p>
        {{if .TFRCheck.HasActiveTFRs}}
            <div class="warning">
                <p><strong>⚠️ Active Restrictions Found:</strong></p>
                <ul>
                {{range .TFRCheck.ActiveTFRs}}
                    <li>{{.Name}} ({{.Type}}): {{.Reason}}</li>
                {{end}}
                </ul>
            </div>
        {{else}}
            <p class="good">✅ No active flight restrictions in the area</p>
        {{end}}
    </div>

    <div class="footer">
        <p><strong>Happy flying! 🚁</strong></p>
        <p>Generated by Drone Weather Agent • Weather data from Open-Meteo</p>
        <p style="font-style: italic; color: #888; margin: 15px 0;">"Safety first - always check NOTAMs and local regulations before flying"</p>
        <hr style="border: none; border-top: 1px solid #ddd; margin: 20px 0;">
        <p>Made with ❤️ by <a href="https://eliottteissonniere.com" style="color: #2196F3; text-decoration: none;">Eliott Teissonniere</a></p>
        <p><a href="https://github.com/ETeissonniere/agent-stack" style="color: #2196F3; text-decoration: none;">⭐ Star us on GitHub</a></p>
    </div>
</body>
</html>
`

	tmpl, err := template.New("email").Parse(tmplStr)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, report); err != nil {
		return "", err
	}

	return buf.String(), nil
}