/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

package ai

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestFetchOpenAIModelsUsesBearerTokenAndSortsResults(t *testing.T) {
	// The response body is supplied by the transport so the test never needs
	// to bind a local listening socket in a restricted test environment.
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://provider.test/v1/models" {
			return nil, fmt.Errorf("path = %s, want https://provider.test/v1/models", request.URL)
		}
		if got, want := request.Header.Get("Authorization"), "Bearer secret"; got != want {
			return nil, fmt.Errorf("authorization = %q, want %q", got, want)
		}
		return jsonResponse(`{"data":[{"id":"z-model","owned_by":"test"},{"id":"a-model"}]}`), nil
	})}

	models, err := fetchOpenAIModelsWithClient(context.Background(), "https://provider.test/v1", "secret", client)
	if err != nil {
		t.Fatalf("fetchOpenAIModels() error = %v", err)
	}
	if len(models) != 2 || models[0].ID != "a-model" || models[1].ID != "z-model" {
		t.Fatalf("models = %#v, want sorted model list", models)
	}
}

func TestFetchGeminiModelsKeepsLiveModelsOnly(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Query().Get("key") != "secret" {
			return nil, fmt.Errorf("missing Gemini API key")
		}
		return jsonResponse(`{"models":[{"name":"models/gemini-2.0-flash","displayName":"Live","supportedGenerationMethods":["bidiGenerateContent","generateContent"]},{"name":"models/gemini-2.0-flash-lite","displayName":"Text only","supportedGenerationMethods":["generateContent"]}]}`), nil
	})}

	models, err := fetchGeminiModelsWithClient(context.Background(), "secret", client)
	if err != nil {
		t.Fatalf("fetchGeminiModels() error = %v", err)
	}
	if len(models) != 1 || models[0].ID != "gemini-2.0-flash" {
		t.Fatalf("models = %#v, want only the Live model", models)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestDiscoverProviderCatalogRequiresCredentials(t *testing.T) {
	result := discoverProviderCatalog(context.Background(), "gemini", map[string]string{})
	if result.Error == "" {
		t.Fatal("expected a missing API key error")
	}
	if result.ModelKey != "gemini_model" || result.VoiceKey != "gemini_voice" {
		t.Fatalf("field mapping = %q/%q, want gemini_model/gemini_voice", result.ModelKey, result.VoiceKey)
	}
	if len(result.Voices) == 0 {
		t.Fatal("expected Gemini voice catalog even when model discovery is unavailable")
	}
}

func TestDiscoverProviderCatalogUsesEachOpenAIStyleProviderEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		settings map[string]string
		endpoint string
		modelKey string
	}{
		{
			name:     "openai",
			provider: "openai",
			settings: map[string]string{"openai_api_key": "openai-key"},
			endpoint: "https://api.openai.com/v1/models",
			modelKey: "openai_realtime_model",
		},
		{
			name:     "openrouter",
			provider: "openrouter",
			settings: map[string]string{"openrouter_api_key": "router-key"},
			endpoint: "https://openrouter.ai/api/v1/models",
			modelKey: "llm_model",
		},
		{
			name:     "tokenhub",
			provider: "tokenhub",
			settings: map[string]string{"tokenhub_base_url": "https://tokenhub.test/v1", "tokenhub_api_key": "token-key"},
			endpoint: "https://tokenhub.test/v1/models",
			modelKey: "llm_model",
		},
		{
			name:     "openai-compatible",
			provider: "openai_compatible",
			settings: map[string]string{"llm_base_url": "https://compatible.test/v1", "llm_api_key": "compatible-key"},
			endpoint: "https://compatible.test/v1/models",
			modelKey: "llm_model",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if request.URL.String() != test.endpoint {
					return nil, fmt.Errorf("endpoint = %s, want %s", request.URL, test.endpoint)
				}
				if request.Header.Get("Authorization") == "" {
					return nil, fmt.Errorf("missing Authorization header")
				}
				return jsonResponse(`{"data":[{"id":"model-b"},{"id":"model-a"}]}`), nil
			})}

			result := discoverProviderCatalogWithClient(context.Background(), test.provider, test.settings, client)
			if result.Error != "" {
				t.Fatalf("discoverProviderCatalog() error = %q", result.Error)
			}
			if result.ModelKey != test.modelKey {
				t.Fatalf("model key = %q, want %q", result.ModelKey, test.modelKey)
			}
			if len(result.Models) != 2 || result.Models[0].ID != "model-a" || result.Models[1].ID != "model-b" {
				t.Fatalf("models = %#v, want sorted model list", result.Models)
			}
		})
	}
}

func TestModelsEndpointRejectsInvalidURL(t *testing.T) {
	if _, err := modelsEndpoint("file:///tmp/provider"); err == nil {
		t.Fatal("expected non-http model URL to be rejected")
	}
}

func TestFetchJSONDoesNotExposeCredentialsInNetworkErrors(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("dial failed for %s", request.URL)
	})}
	err := fetchJSON(context.Background(), client, "https://provider.test/models?key=secret", "", &map[string]any{})
	if err == nil {
		t.Fatal("expected network error")
	}
	if strings.Contains(err.Error(), "secret") {
		t.Fatalf("network error leaked credential: %v", err)
	}
}
