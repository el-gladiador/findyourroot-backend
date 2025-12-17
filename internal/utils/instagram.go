package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// InstagramProfile holds basic Instagram profile data
type InstagramProfile struct {
	Username   string `json:"username"`
	FullName   string `json:"full_name"`
	AvatarURL  string `json:"avatar_url"`
	Bio        string `json:"bio"`
	IsVerified bool   `json:"is_verified"`
}

// FetchInstagramProfile fetches Instagram profile data for a given username
// This uses web scraping which may break if Instagram changes their page structure
func FetchInstagramProfile(username string) (*InstagramProfile, error) {
	// Clean username
	username = strings.TrimPrefix(username, "@")
	username = strings.TrimSpace(username)

	if username == "" {
		return nil, fmt.Errorf("username cannot be empty")
	}

	// Try multiple methods to get profile data
	profile, err := fetchViaWebScraping(username)
	if err != nil {
		// If web scraping fails, try the i.instagram.com endpoint
		profile, err = fetchViaIEndpoint(username)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch Instagram profile: %v", err)
		}
	}

	return profile, nil
}

// fetchViaWebScraping extracts profile data from the Instagram web page
func fetchViaWebScraping(username string) (*InstagramProfile, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	url := fmt.Sprintf("https://www.instagram.com/%s/", username)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Set headers to mimic a browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Connection", "keep-alive")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("Instagram user not found: %s", username)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Instagram returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	html := string(body)

	profile := &InstagramProfile{
		Username: username,
	}

	// Extract profile picture from og:image meta tag
	ogImageRegex := regexp.MustCompile(`<meta property="og:image" content="([^"]+)"`)
	if matches := ogImageRegex.FindStringSubmatch(html); len(matches) > 1 {
		profile.AvatarURL = matches[1]
	}

	// Extract full name from title or og:title
	titleRegex := regexp.MustCompile(`<meta property="og:title" content="([^"]+)"`)
	if matches := titleRegex.FindStringSubmatch(html); len(matches) > 1 {
		// Title is usually "Full Name (@username) • Instagram photos and videos"
		title := matches[1]
		if idx := strings.Index(title, " (@"); idx > 0 {
			profile.FullName = title[:idx]
		} else if idx := strings.Index(title, " •"); idx > 0 {
			profile.FullName = title[:idx]
		}
	}

	// Extract description/bio from og:description
	descRegex := regexp.MustCompile(`<meta property="og:description" content="([^"]+)"`)
	if matches := descRegex.FindStringSubmatch(html); len(matches) > 1 {
		profile.Bio = matches[1]
	}

	if profile.AvatarURL == "" {
		return nil, fmt.Errorf("could not extract profile picture for %s", username)
	}

	return profile, nil
}

// fetchViaIEndpoint tries to get profile data via Instagram's i.instagram.com endpoint
func fetchViaIEndpoint(username string) (*InstagramProfile, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	url := fmt.Sprintf("https://i.instagram.com/api/v1/users/web_profile_info/?username=%s", username)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Set headers
	req.Header.Set("User-Agent", "Instagram 76.0.0.15.395 Android (24/7.0; 640dpi; 1440x2560; samsung; SM-G930F; herolte; samsungexynos8890; en_US; 138226743)")
	req.Header.Set("X-IG-App-ID", "936619743392459")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Instagram API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse the JSON response
	var result struct {
		Data struct {
			User struct {
				ID              string `json:"id"`
				Username        string `json:"username"`
				FullName        string `json:"full_name"`
				ProfilePicURL   string `json:"profile_pic_url"`
				ProfilePicURLHD string `json:"profile_pic_url_hd"`
				Biography       string `json:"biography"`
				IsVerified      bool   `json:"is_verified"`
			} `json:"user"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if result.Data.User.Username == "" {
		return nil, fmt.Errorf("user not found")
	}

	avatarURL := result.Data.User.ProfilePicURLHD
	if avatarURL == "" {
		avatarURL = result.Data.User.ProfilePicURL
	}

	return &InstagramProfile{
		Username:   result.Data.User.Username,
		FullName:   result.Data.User.FullName,
		AvatarURL:  avatarURL,
		Bio:        result.Data.User.Biography,
		IsVerified: result.Data.User.IsVerified,
	}, nil
}

// ValidateInstagramUsername checks if a username format is valid
func ValidateInstagramUsername(username string) bool {
	username = strings.TrimPrefix(username, "@")
	if len(username) < 1 || len(username) > 30 {
		return false
	}
	// Instagram usernames can only contain letters, numbers, periods, and underscores
	validRegex := regexp.MustCompile(`^[a-zA-Z0-9._]+$`)
	return validRegex.MatchString(username)
}
