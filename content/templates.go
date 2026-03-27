package content

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"time"
)

type Template struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

func GetTemplatePost() (string, error) {
	data, err := os.ReadFile("data/templates.json")
	if err != nil {
		return "", fmt.Errorf("failed to read templates: %w", err)
	}

	var templates []Template
	if err := json.Unmarshal(data, &templates); err != nil {
		return "", fmt.Errorf("failed to parse templates: %w", err)
	}

	rand.Seed(time.Now().UnixNano())
	template := templates[rand.Intn(len(templates))]

	return template.Content, nil
}
