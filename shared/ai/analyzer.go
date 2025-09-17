package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"agent-stack/internal/models"
	"agent-stack/shared/config"

	"google.golang.org/genai"
)

type Analyzer struct {
	client            *genai.Client
	model             string
	guidelines        []string
	longVideoMinutes  int
	shortVideoMinutes int
}

func NewAnalyzer(cfg *config.Config) (*Analyzer, error) {
	ctx := context.Background()

	// Configure client with API key
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: cfg.AI.GeminiAPIKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	a := &Analyzer{
		client:            client,
		model:             cfg.AI.Model,
		guidelines:        cfg.Guidelines.Criteria,
		longVideoMinutes:  cfg.Video.LongMinutes,
		shortVideoMinutes: cfg.Video.ShortMinutes,
	}

	return a, nil
}

func (a *Analyzer) AnalyzeVideo(ctx context.Context, video *models.Video) (*models.Analysis, error) {
	if video == nil {
		return nil, fmt.Errorf("video cannot be nil")
	}
	if video.URL == "" {
		return nil, fmt.Errorf("video URL is required")
	}

	// Check video duration for skipping or fallback thresholds
	durationMinutes := video.DurationSeconds / 60

	// Skip short videos if configured
	if a.shortVideoMinutes > 0 && durationMinutes > 0 && durationMinutes <= a.shortVideoMinutes {
		log.Printf("Skipping short video: %s (%d minutes) - %s", video.Title, durationMinutes, video.ChannelTitle)
		return nil, ErrShortVideoSkipped
	}
	useFallback := a.longVideoMinutes > 0 && durationMinutes > a.longVideoMinutes

	if useFallback {
		log.Printf("Using metadata-only analysis for long video: %s (%d minutes) - %s", video.Title, durationMinutes, video.ChannelTitle)
		return a.analyzeMetadataOnly(ctx, video)
	}

	prompt := a.buildAnalysisPrompt(video, false)

	parts := []*genai.Part{
		genai.NewPartFromText(prompt),
		genai.NewPartFromURI(video.URL, "video/mp4"),
	}

	contents := []*genai.Content{
		genai.NewContentFromParts(parts, genai.RoleUser),
	}

	result, err := a.client.Models.GenerateContent(ctx, a.model, contents, nil)
	if err != nil {
		// If token limit error, fallback to metadata analysis
		if strings.Contains(err.Error(), "token count") || strings.Contains(err.Error(), "INVALID_ARGUMENT") {
			log.Printf("Token limit exceeded for video %s (%d minutes), falling back to metadata-only analysis", video.Title, durationMinutes)
			return a.analyzeMetadataOnly(ctx, video)
		}
		return nil, fmt.Errorf("failed to analyze video %s: %w", video.ID, err)
	}

	responseText := result.Text()
	if responseText == "" {
		log.Printf("Empty response from AI for video %s, falling back to metadata-only analysis. This could indicate content filtering, API issues, or video accessibility problems.", video.Title)
		return a.analyzeMetadataOnly(ctx, video)
	}

	analysis, err := a.parseAnalysisResponse(responseText, video)
	if err != nil {
		return nil, fmt.Errorf("failed to parse analysis response for video %s: %w", video.ID, err)
	}

	return analysis, nil
}

// ErrShortVideoSkipped signals the caller that the video was intentionally skipped due to duration
var ErrShortVideoSkipped = errors.New("short video skipped")

func (a *Analyzer) buildAnalysisPrompt(video *models.Video, metadataOnly bool) string {
	guidelines := strings.Join(a.guidelines, "\n- ")

	var analysisType, instructions, summaryDesc, reasoningDesc string
	var descriptionLength int

	if metadataOnly {
		analysisType = "analyzes YouTube video metadata"
		instructions = `INSTRUCTIONS:
1. Analyze ONLY the metadata provided (title, channel, description, etc.)
2. Evaluate the video against the criteria listed above based on metadata alone
3. Be conservative - long-form content from reputable channels/topics may be valuable
4. Consider the channel reputation, topic relevance, and description quality
5. Note that this is a metadata-only analysis without video content`
		summaryDesc = "Brief 2-3 sentence summary based on the title and description"
		reasoningDesc = "Specific explanation of why this video does/doesn't meet the criteria based on metadata"
		descriptionLength = 1000
	} else {
		analysisType = "analyzes YouTube videos"
		instructions = `INSTRUCTIONS:
1. Analyze the actual video content provided
2. Evaluate the video against the criteria listed above
3. Focus on the actual content quality, educational value, and relevance
4. Be selective - only recommend videos that provide clear educational value or professional development`
		summaryDesc = "Brief 2-3 sentence summary of the actual video content and key points"
		reasoningDesc = "Specific explanation of why this video does/doesn't meet the criteria based on the actual content"
		descriptionLength = 500
	}

	metadataNote := ""
	if metadataOnly {
		metadataNote = fmt.Sprintf(" (%d minutes)", video.DurationSeconds/60)
	}

	prompt := fmt.Sprintf(`You are an AI assistant that %s to determine if they are worth watching based on specific criteria.

EVALUATION CRITERIA:
- %s

VIDEO METADATA:
Title: %s
Channel: %s
Description: %s
Duration: %s%s
View Count: %d
Published: %s

%s

Please provide your analysis in the following JSON format:
{
  "is_relevant": boolean,
  "summary": "%s",
  "reasoning": "%s",
  "value_proposition": "What specific knowledge, skills, or insights the viewer would gain from watching this video",
  "score": number (1-10, where 10 is highest relevance to the criteria)
}`,
		analysisType,
		guidelines,
		video.Title,
		video.ChannelTitle,
		truncateString(video.Description, descriptionLength),
		video.Duration,
		metadataNote,
		video.ViewCount,
		video.PublishedAt.Format("2006-01-02 15:04"),
		instructions,
		summaryDesc,
		reasoningDesc,
	)

	if !metadataOnly {
		prompt += "\n\nBase your evaluation on the actual video content, not just the title and description."
	} else {
		prompt += "\n\nNote: This analysis is based solely on metadata as the video content could not be processed due to length."
	}

	return prompt
}

func (a *Analyzer) parseAnalysisResponse(response string, video *models.Video) (*models.Analysis, error) {
	startIdx := strings.Index(response, "{")
	endIdx := strings.LastIndex(response, "}")

	if startIdx == -1 || endIdx == -1 {
		return nil, fmt.Errorf("no JSON found in response: %s", response)
	}

	jsonStr := response[startIdx : endIdx+1]

	var result struct {
		IsRelevant bool   `json:"is_relevant"`
		Summary    string `json:"summary"`
		Reasoning  string `json:"reasoning"`
		ValueProp  string `json:"value_proposition"`
		Score      int    `json:"score"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		// Try to sanitize and parse again
		sanitizedJSON := a.sanitizeJSON(jsonStr)
		if sanitizedErr := json.Unmarshal([]byte(sanitizedJSON), &result); sanitizedErr != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON '%s': %w (sanitized version also failed: %v)", jsonStr, err, sanitizedErr)
		}
		log.Printf("Warning: Had to sanitize malformed JSON for video %s", video.Title)
	}

	if result.Summary == "" {
		return nil, fmt.Errorf("analysis summary is required but was empty")
	}

	if result.Score < 1 {
		result.Score = 1
	} else if result.Score > 10 {
		result.Score = 10
	}

	return &models.Analysis{
		Video:      video,
		IsRelevant: result.IsRelevant,
		Summary:    result.Summary,
		Reasoning:  result.Reasoning,
		ValueProp:  result.ValueProp,
		Score:      result.Score,
	}, nil
}

func (a *Analyzer) analyzeMetadataOnly(ctx context.Context, video *models.Video) (*models.Analysis, error) {
	prompt := a.buildAnalysisPrompt(video, true)

	parts := []*genai.Part{
		genai.NewPartFromText(prompt),
	}

	contents := []*genai.Content{
		genai.NewContentFromParts(parts, genai.RoleUser),
	}

	result, err := a.client.Models.GenerateContent(ctx, a.model, contents, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze video metadata %s: %w", video.ID, err)
	}

	responseText := result.Text()
	if responseText == "" {
		return nil, fmt.Errorf("no analysis response received for video %s", video.ID)
	}

	analysis, err := a.parseAnalysisResponse(responseText, video)
	if err != nil {
		return nil, fmt.Errorf("failed to parse metadata analysis response for video %s: %w", video.ID, err)
	}

	return analysis, nil
}

func (a *Analyzer) sanitizeJSON(jsonStr string) string {
	// Handle common JSON formatting issues from AI responses
	// 1. Fix unescaped quotes within string values
	// This is a simple approach - split by lines and fix quotes within string values

	lines := strings.Split(jsonStr, "\n")
	var sanitizedLines []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Look for lines that contain string values (have : followed by ")
		if strings.Contains(line, ":") && strings.Contains(line, "\"") {
			// Find the position after the colon and first quote
			colonIdx := strings.Index(line, ":")
			if colonIdx != -1 {
				beforeColon := line[:colonIdx+1]
				afterColon := strings.TrimSpace(line[colonIdx+1:])

				// If this is a string value (starts and might end with ")
				if strings.HasPrefix(afterColon, "\"") {
					// Find the last quote (should be the closing quote)
					lastQuoteIdx := strings.LastIndex(afterColon, "\"")
					if lastQuoteIdx > 0 {
						// Extract the string content (between first and last quotes)
						stringContent := afterColon[1:lastQuoteIdx]
						// Escape any unescaped quotes in the content
						stringContent = strings.ReplaceAll(stringContent, "\"", "\\\"")

						// Check if there's a comma after the closing quote
						remainder := afterColon[lastQuoteIdx+1:]

						// Reconstruct the line
						line = beforeColon + " \"" + stringContent + "\"" + remainder
					}
				}
			}
		}

		sanitizedLines = append(sanitizedLines, line)
	}

	return strings.Join(sanitizedLines, "\n")
}

func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength] + "..."
}
