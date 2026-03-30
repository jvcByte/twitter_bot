package content

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type HFMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type HuggingFaceRequest struct {
	Model    string      `json:"model"`
	Messages []HFMessage `json:"messages"`
	MaxTokens int        `json:"max_tokens"`
}

type HuggingFaceResponse struct {
	Choices []struct {
		Message HFMessage `json:"message"`
	} `json:"choices"`
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
		Model: "katanemo/Arch-Router-1.5B:hf-inference",
		Messages: []HFMessage{
			{Role: "user", Content: prompt},
		},
		MaxTokens: 100,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST",
		"https://router.huggingface.co/v1/chat/completions",
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

	if len(hfResp.Choices) == 0 {
		return "", fmt.Errorf("no response from AI")
	}

	post := hfResp.Choices[0].Message.Content + "\n\n#AI #Tech"
	
	if len(post) > 280 {
		post = post[:270] + "...\n\n#AI"
	}

	return post, nil
}
