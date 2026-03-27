package twitter

import (
	"fmt"
	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
)

type Client struct {
	client *twitter.Client
}

func NewClient(apiKey, apiSecret, accessToken, accessSecret string) *Client {
	config := oauth1.NewConfig(apiKey, apiSecret)
	token := oauth1.NewToken(accessToken, accessSecret)
	httpClient := config.Client(oauth1.NoContext, token)

	return &Client{
		client: twitter.NewClient(httpClient),
	}
}

func (c *Client) Tweet(message string) error {
	if len(message) > 280 {
		return fmt.Errorf("tweet exceeds 280 characters: %d", len(message))
	}

	_, _, err := c.client.Statuses.Update(message, nil)
	if err != nil {
		return fmt.Errorf("failed to post tweet: %w", err)
	}

	fmt.Printf("✅ Tweet posted successfully: %s\n", message)
	return nil
}
