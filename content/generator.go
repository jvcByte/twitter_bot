package content

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type HuggingFaceRequest struct {
	Inputs     string                 `json:"inputs"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

type HuggingFaceResponse []struct {
	GeneratedText string `json:"generated_text"`
}

func GenerateAIPost(apiKey string) (string, error) {
	prompts := []string{
		"Write a short, engaging tweet about the latest trends in artificial intelligence:",
		"Create a witty tech observation in under 200 characters:",
		"Share an interesting fact about programming or software development:",
		"Write a motivational tweet for developers:",
	}

	prompt := prompts[len(prompts)%4]

	reqBody := HuggingFaceRequest{
		Inputs: prompt,
		Parameters: map[string]interface{}{
			"max_length":    100,
			"temperature":   0.8,
			"top_p":         0.9,
			"do_sample":     true,
			"return_full_text": false,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", 
		"https://api-inference.huggingface.co/models/gpt2", 
		bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var hfResp HuggingFaceResponse
	if err := json.Unmarshal(body, &hfResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(hfResp) == 0 {
		return "", fmt.Errorf("no response from AI")
	}

	post := hfResp[0].GeneratedText + "\n\n#AI #Tech"
	
	if len(post) > 280 {
		post = post[:270] + "...\n\n#AI"
	}

	return post, nil
}
