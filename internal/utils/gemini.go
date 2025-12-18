package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// GeminiNameMatchResult holds the AI analysis result
type GeminiNameMatchResult struct {
	AreSimilar     bool    `json:"are_similar"`
	Confidence     float64 `json:"confidence"`
	Explanation    string  `json:"explanation"`
	SuggestedMatch string  `json:"suggested_match,omitempty"`
}

// GeminiRequest represents a request to Gemini API
type GeminiRequest struct {
	Contents []GeminiContent `json:"contents"`
}

// GeminiContent represents content in Gemini API request
type GeminiContent struct {
	Parts []GeminiPart `json:"parts"`
}

// GeminiPart represents a part in Gemini content
type GeminiPart struct {
	Text string `json:"text"`
}

// GeminiResponse represents Gemini API response
type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// CheckNamesWithGemini uses Google's Gemini AI to check if two names are likely the same person
// This is particularly useful for Persian names with various spellings
func CheckNamesWithGemini(name1, name2 string) (*GeminiNameMatchResult, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY not set")
	}

	prompt := fmt.Sprintf(`You are an expert in Persian and Arabic names. Analyze these two names and determine if they could refer to the same person.

Name 1: %s
Name 2: %s

Consider:
1. Persian spelling variations (like محمد vs محمّد)
2. Space variations (محمد علی vs محمدعلی)
3. Arabic vs Persian character differences (ي vs ی, ك vs ک)
4. Common nicknames and formal names
5. Transliteration differences

Respond ONLY with a JSON object (no markdown, no code blocks):
{"are_similar": true/false, "confidence": 0.0-1.0, "explanation": "brief explanation in English"}`, name1, name2)

	reqBody := GeminiRequest{
		Contents: []GeminiContent{
			{
				Parts: []GeminiPart{
					{Text: prompt},
				},
			},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent?key=%s", apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var geminiResp GeminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to parse Gemini response: %v", err)
	}

	if geminiResp.Error != nil {
		return nil, fmt.Errorf("Gemini API error: %s", geminiResp.Error.Message)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response from Gemini")
	}

	responseText := geminiResp.Candidates[0].Content.Parts[0].Text
	responseText = strings.TrimSpace(responseText)
	
	// Clean up potential markdown code blocks
	responseText = strings.TrimPrefix(responseText, "```json")
	responseText = strings.TrimPrefix(responseText, "```")
	responseText = strings.TrimSuffix(responseText, "```")
	responseText = strings.TrimSpace(responseText)

	var result GeminiNameMatchResult
	if err := json.Unmarshal([]byte(responseText), &result); err != nil {
		// If parsing fails, try to extract info manually
		return &GeminiNameMatchResult{
			AreSimilar:  strings.Contains(strings.ToLower(responseText), "true") || strings.Contains(responseText, "similar"),
			Confidence:  0.5,
			Explanation: responseText,
		}, nil
	}

	return &result, nil
}

// CheckNameListWithGemini checks a name against a list of existing names using AI
// Returns the most likely matches
func CheckNameListWithGemini(targetName string, existingNames map[string]string) ([]NameMatchResult, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY not set")
	}

	// Build the list of names for the prompt
	var namesList strings.Builder
	for id, name := range existingNames {
		namesList.WriteString(fmt.Sprintf("- ID: %s, Name: %s\n", id, name))
	}

	prompt := fmt.Sprintf(`You are an expert in Persian and Arabic names. A user wants to add a person named "%s" to a family tree.

Here are existing names in the tree:
%s

Check if the new name could be a duplicate of any existing name. Consider:
1. Persian spelling variations
2. Space variations (محمد علی vs محمدعلی)
3. Arabic vs Persian characters
4. Common nicknames

Respond ONLY with a JSON array (no markdown, no code blocks). If no matches found, return empty array [].
Format: [{"person_id": "id", "name": "name", "similarity": 0.0-1.0, "match_type": "ai"}]

Only include names with similarity > 0.7`, targetName, namesList.String())

	reqBody := GeminiRequest{
		Contents: []GeminiContent{
			{
				Parts: []GeminiPart{
					{Text: prompt},
				},
			},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent?key=%s", apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var geminiResp GeminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return nil, err
	}

	if geminiResp.Error != nil {
		return nil, fmt.Errorf("Gemini API error: %s", geminiResp.Error.Message)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response from Gemini")
	}

	responseText := geminiResp.Candidates[0].Content.Parts[0].Text
	responseText = strings.TrimSpace(responseText)
	responseText = strings.TrimPrefix(responseText, "```json")
	responseText = strings.TrimPrefix(responseText, "```")
	responseText = strings.TrimSuffix(responseText, "```")
	responseText = strings.TrimSpace(responseText)

	var results []NameMatchResult
	if err := json.Unmarshal([]byte(responseText), &results); err != nil {
		return nil, fmt.Errorf("failed to parse Gemini results: %v", err)
	}

	return results, nil
}
