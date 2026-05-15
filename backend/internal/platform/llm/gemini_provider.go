package llm

import (
	"context"
	"fmt"
	"os"

	"google.golang.org/genai"
)

type geminiProvider struct {
	client *genai.Client
	model  string
}

var _ LLMProvider = (*geminiProvider)(nil)

func newGeminiProvider() (*geminiProvider, error) {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY is required when LLM_PROVIDER=gemini")
	}
	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		model = "gemini-2.5-flash"
	}
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  key,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("gemini client init: %w", err)
	}
	return &geminiProvider{client: client, model: model}, nil
}

func (g *geminiProvider) Generate(ctx context.Context, system, user string, expectJSON bool) (string, error) {
	cfg := &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(system, genai.RoleUser),
		Temperature:       genai.Ptr[float32](0.2),
	}
	if expectJSON {
		cfg.ResponseMIMEType = "application/json"
		cfg.ResponseSchema = parseResponseSchema
	}
	resp, err := g.client.Models.GenerateContent(ctx, g.model, genai.Text(user), cfg)
	if err != nil {
		return "", fmt.Errorf("gemini api: %w", err)
	}
	return resp.Text(), nil
}

// parseResponseSchema — strict JSON schema untuk ParseResult.
// Gemini ENFORCE output sesuai schema ini, sehingga validation hampir tidak pernah fail.
var parseResponseSchema = &genai.Schema{
	Type: genai.TypeObject,
	Properties: map[string]*genai.Schema{
		"intent": {
			Type: genai.TypeString,
			Enum: []string{
				"expense", "income", "balance", "report",
				"delete_last", "set_goal", "set_budget", "check_goal", "unknown",
			},
		},
		"amount":     {Type: genai.TypeInteger},
		"category":   {Type: genai.TypeString},
		"note":       {Type: genai.TypeString},
		"goal_name":  {Type: genai.TypeString},
		"confidence": {Type: genai.TypeNumber},
	},
	Required: []string{"intent", "amount", "category", "note", "goal_name", "confidence"},
}
