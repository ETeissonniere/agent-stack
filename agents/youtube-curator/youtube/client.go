package youtube

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "os"
    "path/filepath"
    "regexp"
    "strconv"
    "strings"
    "time"

	"agent-stack/internal/models"
	"agent-stack/shared/config"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

type Client struct {
	service *youtube.Service
	config  *config.YouTubeConfig
}

func NewClient(cfg *config.YouTubeConfig) (*Client, error) {
	ctx := context.Background()
	
	// Create OAuth2 config with no redirect URL (OOB flow)
	oauthConfig := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  "urn:ietf:wg:oauth:2.0:oob", // Out-of-band flow
		Scopes:       []string{"https://www.googleapis.com/auth/youtube.readonly"},
		Endpoint:     google.Endpoint,
	}

	// Get OAuth2 token
	token, err := getToken(oauthConfig, cfg.TokenFile)
	if err != nil {
		return nil, fmt.Errorf("failed to get OAuth token: %w", err)
	}

	// Create authenticated HTTP client
	httpClient := oauthConfig.Client(ctx, token)

	// Create YouTube service
	service, err := youtube.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("failed to create YouTube service: %w", err)
	}

	return &Client{
		service: service,
		config:  cfg,
	}, nil
}

func getToken(config *oauth2.Config, tokenFile string) (*oauth2.Token, error) {
	// Try to load token from file
	tok, err := tokenFromFile(tokenFile)
	if err == nil {
		// Check if token is still valid or can be refreshed
		if tok.Valid() || tok.RefreshToken != "" {
			return tok, nil
		}
	}

	// If token doesn't exist or is invalid, get new one
	tok, err = getTokenFromWeb(config)
	if err != nil {
		return nil, err
	}

	// Save token to file
	if err := saveToken(tokenFile, tok); err != nil {
		log.Printf("Warning: Failed to save token: %v", err)
	}
	return tok, nil
}

func getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	// Generate auth URL for out-of-band flow
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	
	fmt.Printf("\n" + strings.Repeat("=", 80) + "\n")
	fmt.Printf("YOUTUBE OAUTH SETUP REQUIRED\n")
	fmt.Printf(strings.Repeat("=", 80) + "\n")
	fmt.Printf("1. Visit this URL in your browser:\n\n")
	fmt.Printf("   %v\n\n", authURL)
	fmt.Printf("2. Complete the authorization process\n")
	fmt.Printf("3. Copy the authorization code from the browser\n")
	fmt.Printf("4. Paste it below and press Enter\n")
	fmt.Printf(strings.Repeat("-", 80) + "\n")
	fmt.Printf("Authorization code: ")

	var authCode string
	if _, err := fmt.Scanln(&authCode); err != nil {
		return nil, fmt.Errorf("unable to read authorization code: %w", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve token from web: %w", err)
	}
	
	fmt.Printf("\n✅ Authorization successful! Token saved.\n")
	fmt.Printf(strings.Repeat("=", 80) + "\n\n")
	
	return tok, nil
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func saveToken(path string, token *oauth2.Token) error {
    // Ensure parent directory exists
    if dir := filepath.Dir(path); dir != "." && dir != "" {
        if err := os.MkdirAll(dir, 0700); err != nil {
            return fmt.Errorf("unable to create token directory: %w", err)
        }
    }

    f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
    if err != nil {
        return fmt.Errorf("unable to cache oauth token: %w", err)
    }
    defer f.Close()
	
	if err := json.NewEncoder(f).Encode(token); err != nil {
		return fmt.Errorf("failed to encode oauth token: %w", err)
	}
	fmt.Printf("Token saved to: %s\n", path)
	return nil
}

func parseDurationSeconds(duration string) int {
	if duration == "" {
		return 0
	}
	
	// Parse ISO 8601 duration format (e.g., "PT1M30S", "PT45S", "PT2H15M30S")
	re := regexp.MustCompile(`PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?`)
	matches := re.FindStringSubmatch(duration)
	
	if len(matches) == 0 {
		return 0
	}
	
	var totalSeconds int
	
	// Hours
	if matches[1] != "" {
		if hours, err := strconv.Atoi(matches[1]); err == nil {
			totalSeconds += hours * 3600
		}
	}
	
	// Minutes
	if matches[2] != "" {
		if minutes, err := strconv.Atoi(matches[2]); err == nil {
			totalSeconds += minutes * 60
		}
	}
	
	// Seconds
	if matches[3] != "" {
		if seconds, err := strconv.Atoi(matches[3]); err == nil {
			totalSeconds += seconds
		}
	}
	
	return totalSeconds
}

func (c *Client) GetSubscriptionVideos(ctx context.Context, maxResults int64) ([]*models.Video, error) {
    since := time.Now().AddDate(0, 0, -1) // Last 24 hours
	
	// Step 1: Get user's subscriptions
	subscriptionsCall := c.service.Subscriptions.List([]string{"snippet"}).
		Mine(true).
		MaxResults(50)

	subscriptionsResponse, err := subscriptionsCall.Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get subscriptions: %w", err)
	}

	if len(subscriptionsResponse.Items) == 0 {
		log.Println("No subscriptions found")
		return []*models.Video{}, nil
	}

	log.Printf("Found %d subscriptions", len(subscriptionsResponse.Items))

	// Step 2: Get channel upload playlist IDs in batches
	var channelIDs []string
	for _, sub := range subscriptionsResponse.Items {
		channelIDs = append(channelIDs, sub.Snippet.ResourceId.ChannelId)
	}

	channelUploadPlaylists := make(map[string]string) // channelID -> uploadPlaylistID
	batchSize := 50

	for i := 0; i < len(channelIDs); i += batchSize {
		end := i + batchSize
		if end > len(channelIDs) {
			end = len(channelIDs)
		}

		batchIDs := channelIDs[i:end]
		channelsCall := c.service.Channels.List([]string{"contentDetails"}).
			Id(strings.Join(batchIDs, ","))

		channelsResponse, err := channelsCall.Do()
		if err != nil {
			log.Printf("Failed to get channel details for batch: %v", err)
			continue
		}

		for _, channel := range channelsResponse.Items {
			if channel.ContentDetails != nil && channel.ContentDetails.RelatedPlaylists != nil {
				uploadPlaylistID := channel.ContentDetails.RelatedPlaylists.Uploads
				if uploadPlaylistID != "" {
					channelUploadPlaylists[channel.Id] = uploadPlaylistID
				}
			}
		}
	}

	log.Printf("Got upload playlists for %d channels", len(channelUploadPlaylists))

	// Step 3: Get recent videos from upload playlists
	var allVideoIDs []string
    if len(channelUploadPlaylists) == 0 {
        log.Println("No upload playlists resolved for subscriptions")
        return []*models.Video{}, nil
    }

    videosPerChannel := maxResults / int64(len(channelUploadPlaylists))
	if videosPerChannel < 1 {
		videosPerChannel = 1
	}
	if videosPerChannel > 5 { // Reasonable limit per channel
		videosPerChannel = 5
	}

	for channelID, playlistID := range channelUploadPlaylists {
		playlistCall := c.service.PlaylistItems.List([]string{"snippet"}).
			PlaylistId(playlistID).
			MaxResults(videosPerChannel)

		playlistResponse, err := playlistCall.Do()
		if err != nil {
			log.Printf("Failed to get playlist items for channel %s: %v", channelID, err)
			continue
		}

		// Filter videos from last 24 hours
		for _, item := range playlistResponse.Items {
			if publishedAt, err := time.Parse(time.RFC3339, item.Snippet.PublishedAt); err == nil {
				if publishedAt.After(since) {
					allVideoIDs = append(allVideoIDs, item.Snippet.ResourceId.VideoId)
				}
			}
		}

		// Stop if we have enough videos
		if int64(len(allVideoIDs)) >= maxResults {
			break
		}
	}

	if len(allVideoIDs) == 0 {
		log.Println("No recent videos found from subscriptions")
		return []*models.Video{}, nil
	}

	// Limit to maxResults
	if int64(len(allVideoIDs)) > maxResults {
		allVideoIDs = allVideoIDs[:maxResults]
	}

	log.Printf("Found %d recent videos from subscriptions", len(allVideoIDs))

	// Step 4: Get detailed video information in batches
	var allVideos []*models.Video
	var skippedShorts int
	
	for i := 0; i < len(allVideoIDs); i += batchSize {
		end := i + batchSize
		if end > len(allVideoIDs) {
			end = len(allVideoIDs)
		}

		batchIDs := allVideoIDs[i:end]
		videosCall := c.service.Videos.List([]string{"snippet", "contentDetails", "statistics"}).
			Id(strings.Join(batchIDs, ","))

		videosResponse, err := videosCall.Do()
		if err != nil {
			log.Printf("Failed to get video details for batch: %v", err)
			continue
		}

		for _, item := range videosResponse.Items {
			// Skip YouTube Shorts (videos ≤60 seconds)
			durationSeconds := parseDurationSeconds(item.ContentDetails.Duration)
			if durationSeconds <= 60 && durationSeconds > 0 {
				log.Printf("Skipping YouTube Short: %s (%ds) - %s", item.Snippet.Title, durationSeconds, item.Snippet.ChannelTitle)
				skippedShorts++
				continue
			}

			video := &models.Video{
				ID:              item.Id,
				Title:           item.Snippet.Title,
				Description:     item.Snippet.Description,
				ChannelID:       item.Snippet.ChannelId,
				ChannelTitle:    item.Snippet.ChannelTitle,
				Duration:        item.ContentDetails.Duration,
				DurationSeconds: durationSeconds,
				URL:             fmt.Sprintf("https://www.youtube.com/watch?v=%s", item.Id),
			}

			if item.Snippet.Thumbnails.High != nil {
				video.ThumbnailURL = item.Snippet.Thumbnails.High.Url
			} else if item.Snippet.Thumbnails.Medium != nil {
				video.ThumbnailURL = item.Snippet.Thumbnails.Medium.Url
			}

			if publishedAt, err := time.Parse(time.RFC3339, item.Snippet.PublishedAt); err == nil {
				video.PublishedAt = publishedAt
			}

			if item.Statistics != nil {
				video.ViewCount = int64(item.Statistics.ViewCount)
			}

			allVideos = append(allVideos, video)
		}
	}

	log.Printf("Retrieved %d videos from %d subscriptions (skipped %d shorts)", len(allVideos), len(subscriptionsResponse.Items), skippedShorts)

	return allVideos, nil
}
