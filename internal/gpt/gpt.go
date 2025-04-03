package gpt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	gogpt "github.com/sashabaranov/go-openai"
)

var ErrTooManyRequests = errors.New("too many requests")

type Config struct {
	Keys    []string
	Timeout time.Duration
}

type Client struct {
	id int
	*gogpt.Client
}

type Handler struct {
	sync.Mutex
	cfg     Config
	index   int
	clients []*Client
}

type expectedType struct {
	Translations []string `json:"translations"`
}

func New(cfg Config) *Handler {
	h := &Handler{
		cfg:     cfg,
		clients: make([]*Client, len(cfg.Keys)),
	}
	for i, key := range cfg.Keys {
		c := &Client{
			id:     i,
			Client: gogpt.NewClient(key),
		}
		h.clients[i] = c
	}
	return h
}

func (h *Handler) Translate(ctx context.Context, text string, lang string) (string, error) {
	var lastErr error

	// Try up to 3 times
	for attempt := 0; attempt < 3; attempt++ {
		// Construct system prompt for translation instructions
		systemPrompt := "You are a professional translator. Translate the text exactly as provided without adding any comments, explanations, or additional text. Maintain the original formatting including any HTML, markdown, or special characters. Do not alter placeholders, variables, or code snippets."

		// Construct clear user prompt
		userPrompt := fmt.Sprintf("Translate the following text to %s. Keep any markdown, HTML tags, and special characters (including [], {}, <>, etc.) unchanged:\n\n%s", lang, text)

		// Create chat completion request
		completionReq := gogpt.ChatCompletionRequest{
			Model: "gpt-3.5-turbo",
			Messages: []gogpt.ChatCompletionMessage{
				{
					Role:    "system",
					Content: systemPrompt,
				},
				{
					Role:    "user",
					Content: userPrompt,
				},
			},
			Temperature: 0.1,
			MaxTokens:   1024,
		}

		h.Lock()
		client := h.clients[h.index]
		h.index = (h.index + 1) % len(h.clients)
		h.Unlock()

		resp, err := client.CreateChatCompletion(ctx, completionReq)
		if err != nil {
			var apiErr *gogpt.APIError
			if errors.As(err, &apiErr) {
				switch apiErr.HTTPStatusCode {
				case 429:
					// Rate limit error
					lastErr = fmt.Errorf("API rate limit exceeded: %w", err)
					fmt.Printf("Rate limit exceeded, waiting before retry (attempt %d/3)...\n", attempt+1)
					time.Sleep(time.Duration(2+attempt) * time.Second)
					continue
				case 500, 502, 503, 504:
					// Server error
					lastErr = fmt.Errorf("OpenAI server error: %w", err)
					time.Sleep(time.Duration(1+attempt) * time.Second)
					continue
				}
			}

			// Check for context deadline exceeded or timeout
			if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "timeout") {
				lastErr = fmt.Errorf("request timed out: %w", err)
				time.Sleep(time.Duration(1+attempt) * time.Second)
				continue
			}

			lastErr = fmt.Errorf("error creating chat completion: %w", err)
			continue
		}

		if len(resp.Choices) > 0 {
			result := strings.TrimSpace(resp.Choices[0].Message.Content)

			// Check for valid translation
			if result == "" || result == " " {
				lastErr = fmt.Errorf("received empty translation")
				continue
			}

			return result, nil
		}

		lastErr = fmt.Errorf("no choices in response")
	}

	// All attempts failed
	return "", fmt.Errorf("failed to translate after 3 attempts: %w", lastErr)
}

func (h *Handler) BatchTranslate(ctx context.Context, texts []string, lang string) ([]string, error) {
	var lastErr error

	// Try up to 3 times
	for attempt := 0; attempt < 3; attempt++ {
		// Construct system prompt for batch translation instructions
		systemPrompt := "You are a professional translator. Translate the array of texts exactly as provided without adding comments or explanations. Maintain all formatting including HTML, markdown, and special characters. Return your response ONLY as a valid JSON object in this exact format: {\"translations\": [\"translated text 1\", \"translated text 2\", ...]}"

		// Create the JSON array of text to translate
		textsJSON, err := json.Marshal(texts)
		if err != nil {
			return nil, fmt.Errorf("error marshalling texts: %w", err)
		}

		// Construct clear user prompt
		userPrompt := fmt.Sprintf("Translate this array of texts to %s. Keep any markdown, HTML tags, and special characters (including [], {}, <>, etc.) unchanged. Return ONLY a JSON object with a 'translations' array.\n\n%s", lang, string(textsJSON))

		// Create chat completion request
		completionReq := gogpt.ChatCompletionRequest{
			Model: "gpt-3.5-turbo",
			Messages: []gogpt.ChatCompletionMessage{
				{
					Role:    "system",
					Content: systemPrompt,
				},
				{
					Role:    "user",
					Content: userPrompt,
				},
			},
			Temperature: 0.1,
			MaxTokens:   2048,
		}

		h.Lock()
		client := h.clients[h.index]
		h.index = (h.index + 1) % len(h.clients)
		h.Unlock()

		resp, err := client.CreateChatCompletion(ctx, completionReq)
		if err != nil {
			var apiErr *gogpt.APIError
			if errors.As(err, &apiErr) {
				switch apiErr.HTTPStatusCode {
				case 429:
					// Rate limit error
					lastErr = fmt.Errorf("API rate limit exceeded: %w", err)
					fmt.Printf("Rate limit exceeded, waiting before retry (attempt %d/3)...\n", attempt+1)
					time.Sleep(time.Duration(2+attempt) * time.Second)
					continue
				case 500, 502, 503, 504:
					// Server error
					lastErr = fmt.Errorf("OpenAI server error: %w", err)
					time.Sleep(time.Duration(1+attempt) * time.Second)
					continue
				}
			}

			// Check for context deadline exceeded or timeout
			if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "timeout") {
				lastErr = fmt.Errorf("request timed out: %w", err)
				time.Sleep(time.Duration(1+attempt) * time.Second)
				continue
			}

			lastErr = fmt.Errorf("error creating chat completion: %w", err)
			continue
		}

		if len(resp.Choices) > 0 {
			content := resp.Choices[0].Message.Content
			content = strings.TrimSpace(content)

			// Try different parsing approaches
			var translations []string

			// Try parsing as {"translations": [...]}
			var result struct {
				Translations []string `json:"translations"`
			}

			if err := json.Unmarshal([]byte(content), &result); err == nil && len(result.Translations) == len(texts) {
				translations = result.Translations
			} else {
				// Try parsing as direct array
				if strings.HasPrefix(content, "[") && strings.HasSuffix(content, "]") {
					if err := json.Unmarshal([]byte(content), &translations); err != nil || len(translations) != len(texts) {
						lastErr = fmt.Errorf("failed to parse response as JSON array: %w", err)
						continue
					}
				} else {
					// If still not working, try to extract JSON from the text
					startIdx := strings.Index(content, "{")
					endIdx := strings.LastIndex(content, "}")
					if startIdx >= 0 && endIdx > startIdx {
						jsonContent := content[startIdx : endIdx+1]
						if err := json.Unmarshal([]byte(jsonContent), &result); err == nil && len(result.Translations) == len(texts) {
							translations = result.Translations
						} else {
							lastErr = fmt.Errorf("failed to extract valid JSON response: %v", err)
							continue
						}
					} else {
						lastErr = fmt.Errorf("response did not contain valid JSON")
						continue
					}
				}
			}

			// Validate translations
			for i, translation := range translations {
				if translation == "" || translation == " " {
					lastErr = fmt.Errorf("received empty translation for text: %s", texts[i])
					continue
				}
			}

			return translations, nil
		}

		lastErr = fmt.Errorf("no choices in response")
	}

	// All attempts failed
	return nil, fmt.Errorf("failed to batch translate after 3 attempts: %w", lastErr)
}
