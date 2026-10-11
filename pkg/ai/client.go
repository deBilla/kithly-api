// Package ai wraps the Google Generative AI (Gemini) SDK for wish generation.
package ai

import (
	"context"
	"fmt"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

const model = "gemini-2.5-flash"

// Client wraps the Gemini generative model.
type Client struct {
	model *genai.GenerativeModel
}

// New creates an AI Client using the provided Gemini API key.
func New(ctx context.Context, apiKey string) (*Client, error) {
	gc, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("ai.New: %w", err)
	}
	m := gc.GenerativeModel(model)
	m.SetTemperature(0.9)
	return &Client{model: m}, nil
}

// WishInput holds the parameters for wish generation, matching the TypeScript AI service.
type WishInput struct {
	Tone     string // heartfelt | funny | inspirational | casual
	FromName string
	ToName   string
	Occasion string
}

// GenerateWish generates a personalised wish message. The tone controls the
// style — matching the TypeScript AIService.generateWish contract exactly.
func (c *Client) GenerateWish(ctx context.Context, input WishInput) (string, error) {
	toneDescriptions := map[string]string{
		"heartfelt":     "warm, emotional, and deeply sincere",
		"funny":         "light-hearted, humorous, and playful",
		"inspirational": "motivating, uplifting, and encouraging",
		"casual":        "relaxed, friendly, and conversational",
	}
	toneDesc := toneDescriptions[input.Tone]
	if toneDesc == "" {
		toneDesc = "warm, emotional, and deeply sincere"
	}

	prompt := fmt.Sprintf(
		"Write exactly ONE %s wish message from %s to %s for their %s.\n\n"+
			"Requirements:\n"+
			"- Provide only a single wish, not multiple options\n"+
			"- Keep it to 2-3 sentences\n"+
			"- The tone should be %s\n"+
			"- Make it suitable for a group video card\n"+
			"- Do not include any alternatives, variations, or \"you could also\" suggestions\n"+
			"- Return only the wish message itself, nothing else\n"+
			"- Start naturally, do not use generic openings like \"Dear %s\"\n\n"+
			"Wish message:",
		toneDesc, input.FromName, input.ToName, input.Occasion, toneDesc, input.ToName,
	)

	resp, err := c.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("ai.GenerateWish: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("ai.GenerateWish: empty response from model")
	}

	text, ok := resp.Candidates[0].Content.Parts[0].(genai.Text)
	if !ok {
		return "", fmt.Errorf("ai.GenerateWish: unexpected response part type")
	}

	return string(text), nil
}
