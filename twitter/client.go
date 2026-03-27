package twitter

import (
	"fmt"
	"os"
	"time"
)

type Client struct {
	username string
	password string
}

func NewClient(username, password, _, _ string) *Client {
	return &Client{
		username: username,
		password: password,
	}
}

func (c *Client) Tweet(message string) error {
	if len(message) > 280 {
		return fmt.Errorf("tweet exceeds 280 characters: %d", len(message))
	}

	// Save tweet to file for manual posting
	filename := fmt.Sprintf("tweets_%s.txt", time.Now().Format("2006-01-02"))
	timestamp := time.Now().Format("15:04:05")
	
	content := fmt.Sprintf("[%s]\n%s\n\n", timestamp, message)
	
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()
	
	if _, err := f.WriteString(content); err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	fmt.Printf("\n✅ Tweet saved to %s\n", filename)
	fmt.Printf("📋 Copy and post manually:\n\n%s\n", message)
	fmt.Println("\n" + "="*60)
	
	return nil
}
