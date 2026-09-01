/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

type catalogOption struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

type providerCatalog struct {
	Provider string          `json:"provider"`
	ModelKey string          `json:"model_key"`
	VoiceKey string          `json:"voice_key"`
	Models   []catalogOption `json:"models"`
	Voices   []catalogOption `json:"voices"`
	Warnings []string        `json:"warnings,omitempty"`
	Error    string          `json:"error,omitempty"`
}

type providerCatalogSettings struct {
	Provider string            `json:"provider"`
	Settings map[string]string `json:"settings"`
}

type openAIModelList struct {
	Data []struct {
		ID      string `json:"id"`
		OwnedBy string `json:"owned_by"`
	} `json:"data"`
}

type geminiModelList struct {
	Models []struct {
		Name                   string   `json:"name"`
		BaseModelID            string   `json:"baseModelId"`
		DisplayName            string   `json:"displayName"`
		Description            string   `json:"description"`
		SupportedActionMethods []string `json:"supportedGenerationMethods"`
	} `json:"models"`
}

var openAIVoices = []catalogOption{
	{ID: "alloy"}, {ID: "ash"}, {ID: "ballad"}, {ID: "coral"}, {ID: "echo"},
	{ID: "sage"}, {ID: "shimmer"}, {ID: "verse"}, {ID: "marin"}, {ID: "cedar"},
}

var geminiVoices = []catalogOption{
	{ID: "Aoede"}, {ID: "Charon"}, {ID: "Fenrir"}, {ID: "Kore"}, {ID: "Puck"},
	{ID: "Leda"}, {ID: "Orus"}, {ID: "Zephyr"}, {ID: "Achernar"}, {ID: "Achird"},
	{ID: "Algenib"}, {ID: "Algieba"}, {ID: "Alnilam"}, {ID: "Autonoe"}, {ID: "Callirrhoe"},
	{ID: "Despina"}, {ID: "Enceladus"}, {ID: "Erinome"}, {ID: "Gacrux"}, {ID: "Iapetus"},
	{ID: "Laomedeia"}, {ID: "Pulcherrima"}, {ID: "Rasalgethi"}, {ID: "Sadachbia"},
	{ID: "Sadaltager"}, {ID: "Schedar"}, {ID: "Sulafat"}, {ID: "Umbriel"},
	{ID: "Vindemiatrix"}, {ID: "Zubenelgenubi"},
}

func discoverProviderCatalog(ctx context.Context, provider string, settings map[string]string) providerCatalog {
	return discoverProviderCatalogWithClient(ctx, provider, settings, &http.Client{Timeout: 10 * time.Second})
}

func discoverProviderCatalogWithClient(ctx context.Context, provider string, settings map[string]string, client *http.Client) providerCatalog {
	provider = strings.TrimSpace(provider)
	result := providerCatalog{Provider: provider, Models: []catalogOption{}, Voices: []catalogOption{}}
	if settings == nil {
		settings = map[string]string{}
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	var (
		baseURL  string
		apiKey   string
		modelKey string
		voiceKey string
	)
	switch provider {
	case "openai":
		baseURL, apiKey = "https://api.openai.com/v1", settings["openai_api_key"]
		modelKey, voiceKey = "openai_realtime_model", "openai_tts_voice"
		result.Voices = cloneCatalogOptions(openAIVoices)
		result.Warnings = append(result.Warnings, "OpenAI does not expose a Realtime voice-list endpoint; the voice list contains the current built-in Realtime voices.")
	case "gemini":
		apiKey = settings["gemini_api_key"]
		modelKey, voiceKey = "gemini_model", "gemini_voice"
		result.Voices = cloneCatalogOptions(geminiVoices)
		result.Warnings = append(result.Warnings, "Gemini voice choices are a built-in capability catalog; model availability is checked against the Gemini models API.")
	case "openrouter":
		baseURL, apiKey = "https://openrouter.ai/api/v1", settings["openrouter_api_key"]
		modelKey, voiceKey = "llm_model", "tts_voice"
		result.Warnings = append(result.Warnings, "OpenRouter model discovery lists chat models; choose a model that supports the configured STT/LLM/TTS pipeline.")
	case "tokenhub":
		baseURL = override(settings["tokenhub_base_url"], settings["compatible_base_url"])
		apiKey = override(settings["tokenhub_api_key"], settings["compatible_api_key"])
		modelKey, voiceKey = "llm_model", "tts_voice"
		result.Warnings = append(result.Warnings, "TokenHub voice choices depend on the TTS provider; enter a voice supported by that provider.")
	case "openai_compatible":
		baseURL = override(settings["llm_base_url"], settings["compatible_base_url"])
		apiKey = override(settings["llm_api_key"], settings["compatible_api_key"])
		modelKey, voiceKey = "llm_model", "tts_voice"
		result.Warnings = append(result.Warnings, "OpenAI-compatible endpoints do not share a standard voice catalog; model discovery checks the configured LLM endpoint only.")
	default:
		result.Error = fmt.Sprintf("unsupported provider %q", provider)
		return result
	}
	result.ModelKey, result.VoiceKey = modelKey, voiceKey
	if strings.TrimSpace(apiKey) == "" {
		result.Error = fmt.Sprintf("API key is required for provider %s", provider)
		return result
	}

	requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var err error
	if provider == "gemini" {
		result.Models, err = fetchGeminiModelsWithClient(requestCtx, apiKey, client)
	} else {
		result.Models, err = fetchOpenAIModelsWithClient(requestCtx, baseURL, apiKey, client)
	}
	if err != nil {
		result.Error = err.Error()
	}
	return result
}

func fetchOpenAIModels(ctx context.Context, baseURL, apiKey string) ([]catalogOption, error) {
	return fetchOpenAIModelsWithClient(ctx, baseURL, apiKey, &http.Client{Timeout: 10 * time.Second})
}

func fetchOpenAIModelsWithClient(ctx context.Context, baseURL, apiKey string, client *http.Client) ([]catalogOption, error) {
	endpoint, err := modelsEndpoint(baseURL)
	if err != nil {
		return nil, err
	}
	var payload openAIModelList
	if err := fetchJSON(ctx, client, endpoint, apiKey, &payload); err != nil {
		return nil, err
	}
	models := make([]catalogOption, 0, len(payload.Data))
	for _, model := range payload.Data {
		if strings.TrimSpace(model.ID) == "" {
			continue
		}
		models = append(models, catalogOption{ID: model.ID, Name: model.ID, Description: model.OwnedBy})
	}
	sortCatalogOptions(models)
	return limitCatalogOptions(models, 200), nil
}

func fetchGeminiModels(ctx context.Context, apiKey string) ([]catalogOption, error) {
	return fetchGeminiModelsWithClient(ctx, apiKey, &http.Client{Timeout: 10 * time.Second})
}

func fetchGeminiModelsWithClient(ctx context.Context, apiKey string, client *http.Client) ([]catalogOption, error) {
	endpoint, err := url.Parse("https://generativelanguage.googleapis.com/v1beta/models")
	if err != nil {
		return nil, err
	}
	query := endpoint.Query()
	query.Set("key", apiKey)
	query.Set("pageSize", "1000")
	endpoint.RawQuery = query.Encode()
	var payload geminiModelList
	if err := fetchJSON(ctx, client, endpoint.String(), "", &payload); err != nil {
		return nil, err
	}
	models := make([]catalogOption, 0, len(payload.Models))
	for _, model := range payload.Models {
		if !supportsGeminiLive(model.SupportedActionMethods) {
			continue
		}
		id := strings.TrimPrefix(strings.TrimSpace(model.Name), "models/")
		if id == "" {
			id = strings.TrimSpace(model.BaseModelID)
		}
		if id == "" {
			continue
		}
		models = append(models, catalogOption{ID: id, Name: model.DisplayName, Description: model.Description})
	}
	sortCatalogOptions(models)
	return limitCatalogOptions(models, 200), nil
}

func supportsGeminiLive(methods []string) bool {
	for _, method := range methods {
		if strings.EqualFold(strings.TrimSpace(method), "bidiGenerateContent") {
			return true
		}
	}
	return false
}

func modelsEndpoint(baseURL string) (string, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return "", fmt.Errorf("model endpoint base URL is required")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", fmt.Errorf("model endpoint base URL must be an http(s) URL")
	}
	return baseURL + "/models", nil
}

func fetchJSON(ctx context.Context, client *http.Client, endpoint, apiKey string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create model request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	if apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+apiKey)
	}
	response, err := client.Do(request)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("model request timed out; check provider availability")
		}
		if errors.Is(err, context.Canceled) {
			return fmt.Errorf("model request canceled")
		}
		// Do not return the underlying error: Gemini puts the API key in the
		// query string, and net/http may include the full URL in its error.
		return fmt.Errorf("model request failed; check provider availability")
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("model request returned HTTP %d", response.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(target); err != nil {
		return fmt.Errorf("decode model response: %w", err)
	}
	return nil
}

func cloneCatalogOptions(options []catalogOption) []catalogOption {
	return append([]catalogOption(nil), options...)
}

func sortCatalogOptions(options []catalogOption) {
	sort.Slice(options, func(i, j int) bool {
		return strings.ToLower(options[i].ID) < strings.ToLower(options[j].ID)
	})
}

func limitCatalogOptions(options []catalogOption, limit int) []catalogOption {
	if len(options) <= limit {
		return options
	}
	return options[:limit]
}
