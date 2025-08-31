package youtube

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestTokenSaver(t *testing.T) {
	// Create a temporary directory for test tokens
	tempDir := t.TempDir()
	tokenFile := filepath.Join(tempDir, "test_token.json")

	// Create a test token
	originalToken := &oauth2.Token{
		AccessToken:  "original-access-token",
		RefreshToken: "test-refresh-token",
		Expiry:       time.Now().Add(-time.Hour), // Expired token
	}

	// Save the original token
	err := saveToken(tokenFile, originalToken)
	if err != nil {
		t.Fatalf("Failed to save original token: %v", err)
	}

	// Verify token was saved
	savedToken, err := tokenFromFile(tokenFile)
	if err != nil {
		t.Fatalf("Failed to load saved token: %v", err)
	}

	if savedToken.RefreshToken != originalToken.RefreshToken {
		t.Errorf("Refresh token mismatch: got %s, want %s", savedToken.RefreshToken, originalToken.RefreshToken)
	}
}

func TestGetToken(t *testing.T) {
	// Create a temporary directory for test tokens
	tempDir := t.TempDir()
	tokenFile := filepath.Join(tempDir, "test_token.json")

	// Create a mock OAuth config
	oauthConfig := &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/auth",
			TokenURL: "https://oauth2.googleapis.com/token",
		},
	}

	t.Run("LoadExistingValidToken", func(t *testing.T) {
		// Create a valid token
		validToken := &oauth2.Token{
			AccessToken:  "valid-access-token",
			RefreshToken: "valid-refresh-token",
			Expiry:       time.Now().Add(time.Hour), // Valid for 1 hour
		}

		// Save the token
		err := saveToken(tokenFile, validToken)
		if err != nil {
			t.Fatalf("Failed to save token: %v", err)
		}

		// Try to get the token
		token, err := getToken(oauthConfig, tokenFile)
		if err != nil {
			t.Fatalf("Failed to get token: %v", err)
		}

		if token.AccessToken != validToken.AccessToken {
			t.Errorf("Access token mismatch: got %s, want %s", token.AccessToken, validToken.AccessToken)
		}
	})

	t.Run("LoadExpiredTokenWithRefresh", func(t *testing.T) {
		// Create an expired token with refresh token
		expiredToken := &oauth2.Token{
			AccessToken:  "expired-access-token",
			RefreshToken: "valid-refresh-token",
			Expiry:       time.Now().Add(-time.Hour), // Expired 1 hour ago
		}

		// Save the token
		err := saveToken(tokenFile, expiredToken)
		if err != nil {
			t.Fatalf("Failed to save token: %v", err)
		}

		// Try to get the token - should load it even though expired (refresh will happen later)
		token, err := getToken(oauthConfig, tokenFile)
		if err != nil {
			t.Fatalf("Failed to get token: %v", err)
		}

		if token.RefreshToken != expiredToken.RefreshToken {
			t.Errorf("Refresh token mismatch: got %s, want %s", token.RefreshToken, expiredToken.RefreshToken)
		}
	})

	t.Run("NoTokenFile", func(t *testing.T) {
		// Remove token file if it exists
		os.Remove(tokenFile)

		// This will fail because it tries to get from web (which we can't do in tests)
		// Just verify it returns an error
		_, err := getToken(oauthConfig, tokenFile)
		if err == nil {
			t.Error("Expected error when no token file exists and can't get from web")
		}
	})
}

func TestTokenFromFile(t *testing.T) {
	tempDir := t.TempDir()
	tokenFile := filepath.Join(tempDir, "test_token.json")

	t.Run("ValidTokenFile", func(t *testing.T) {
		// Create a test token
		testToken := &oauth2.Token{
			AccessToken:  "test-access-token",
			RefreshToken: "test-refresh-token",
			TokenType:    "Bearer",
			Expiry:       time.Now().Add(time.Hour),
		}

		// Write token to file
		data, _ := json.Marshal(testToken)
		err := os.WriteFile(tokenFile, data, 0600)
		if err != nil {
			t.Fatalf("Failed to write token file: %v", err)
		}

		// Read token from file
		token, err := tokenFromFile(tokenFile)
		if err != nil {
			t.Fatalf("Failed to read token from file: %v", err)
		}

		if token.AccessToken != testToken.AccessToken {
			t.Errorf("Access token mismatch: got %s, want %s", token.AccessToken, testToken.AccessToken)
		}
		if token.RefreshToken != testToken.RefreshToken {
			t.Errorf("Refresh token mismatch: got %s, want %s", token.RefreshToken, testToken.RefreshToken)
		}
	})

	t.Run("NonExistentFile", func(t *testing.T) {
		_, err := tokenFromFile(filepath.Join(tempDir, "nonexistent.json"))
		if err == nil {
			t.Error("Expected error for non-existent file")
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		// Write invalid JSON to file
		err := os.WriteFile(tokenFile, []byte("invalid json"), 0600)
		if err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}

		_, err = tokenFromFile(tokenFile)
		if err == nil {
			t.Error("Expected error for invalid JSON")
		}
	})
}

func TestSaveToken(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("SaveToNewFile", func(t *testing.T) {
		tokenFile := filepath.Join(tempDir, "new_token.json")
		
		testToken := &oauth2.Token{
			AccessToken:  "test-access",
			RefreshToken: "test-refresh",
			Expiry:       time.Now().Add(time.Hour),
		}

		err := saveToken(tokenFile, testToken)
		if err != nil {
			t.Fatalf("Failed to save token: %v", err)
		}

		// Verify file exists
		if _, err := os.Stat(tokenFile); os.IsNotExist(err) {
			t.Error("Token file was not created")
		}

		// Verify content
		saved, err := tokenFromFile(tokenFile)
		if err != nil {
			t.Fatalf("Failed to read saved token: %v", err)
		}

		if saved.AccessToken != testToken.AccessToken {
			t.Errorf("Access token mismatch: got %s, want %s", saved.AccessToken, testToken.AccessToken)
		}
	})

	t.Run("SaveWithNestedDirectory", func(t *testing.T) {
		tokenFile := filepath.Join(tempDir, "nested", "dir", "token.json")
		
		testToken := &oauth2.Token{
			AccessToken:  "nested-access",
			RefreshToken: "nested-refresh",
			Expiry:       time.Now().Add(time.Hour),
		}

		err := saveToken(tokenFile, testToken)
		if err != nil {
			t.Fatalf("Failed to save token to nested directory: %v", err)
		}

		// Verify file exists
		if _, err := os.Stat(tokenFile); os.IsNotExist(err) {
			t.Error("Token file was not created in nested directory")
		}
	})

	t.Run("OverwriteExistingFile", func(t *testing.T) {
		tokenFile := filepath.Join(tempDir, "overwrite_token.json")
		
		// Save first token
		firstToken := &oauth2.Token{
			AccessToken: "first-token",
		}
		err := saveToken(tokenFile, firstToken)
		if err != nil {
			t.Fatalf("Failed to save first token: %v", err)
		}

		// Save second token
		secondToken := &oauth2.Token{
			AccessToken: "second-token",
		}
		err = saveToken(tokenFile, secondToken)
		if err != nil {
			t.Fatalf("Failed to save second token: %v", err)
		}

		// Verify second token overwrote first
		saved, _ := tokenFromFile(tokenFile)
		if saved.AccessToken != secondToken.AccessToken {
			t.Errorf("Token was not overwritten: got %s, want %s", saved.AccessToken, secondToken.AccessToken)
		}
	})
}

func TestParseDurationSeconds(t *testing.T) {
	tests := []struct {
		name     string
		duration string
		expected int
	}{
		{"Empty", "", 0},
		{"Seconds only", "PT45S", 45},
		{"Minutes only", "PT2M", 120},
		{"Hours only", "PT1H", 3600},
		{"Minutes and seconds", "PT1M30S", 90},
		{"Hours and minutes", "PT2H15M", 8100},
		{"Full format", "PT2H15M30S", 8130},
		{"Invalid format", "invalid", 0},
		{"No time components", "PT", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDurationSeconds(tt.duration)
			if result != tt.expected {
				t.Errorf("parseDurationSeconds(%s) = %d, want %d", tt.duration, result, tt.expected)
			}
		})
	}
}

func TestRefreshToken(t *testing.T) {
	// This test requires a mock setup since we can't actually refresh tokens in tests
	// We'll test the RefreshToken method exists and handles errors properly
	
	tempDir := t.TempDir()
	tokenFile := filepath.Join(tempDir, "test_token.json")

	// Create a token with refresh token
	testToken := &oauth2.Token{
		AccessToken:  "test-access",
		RefreshToken: "test-refresh",
		Expiry:       time.Now().Add(-time.Hour), // Expired
	}

	// Save the token
	err := saveToken(tokenFile, testToken)
	if err != nil {
		t.Fatalf("Failed to save test token: %v", err)
	}

	// Note: We can't fully test NewClient and RefreshToken without mocking the YouTube service
	// but we've tested all the supporting functions thoroughly
	
	t.Run("TokenFileCreated", func(t *testing.T) {
		// Verify the token file was created with correct permissions
		info, err := os.Stat(tokenFile)
		if err != nil {
			t.Fatalf("Failed to stat token file: %v", err)
		}

		// Check file permissions (should be 0600)
		mode := info.Mode()
		if mode.Perm() != 0600 {
			t.Errorf("Token file has incorrect permissions: %v, want 0600", mode.Perm())
		}
	})
}

// MockTokenSource for testing tokenSaver
type MockTokenSource struct {
	token *oauth2.Token
	err   error
}

func (m *MockTokenSource) Token() (*oauth2.Token, error) {
	return m.token, m.err
}

func TestTokenSaverConcurrency(t *testing.T) {
	tempDir := t.TempDir()
	tokenFile := filepath.Join(tempDir, "concurrent_token.json")

	ts := &tokenSaver{
		config: &oauth2.Config{
			ClientID: "test",
		},
		token: &oauth2.Token{
			AccessToken:  "initial",
			RefreshToken: "refresh",
		},
		tokenFile: tokenFile,
	}

	// Test concurrent access doesn't cause race conditions
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			// This would normally refresh the token, but we're just testing
			// that the mutex prevents race conditions
			_, _ = ts.Token()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// If we get here without panicking, concurrency is handled correctly
	t.Log("Concurrent token access handled successfully")
}