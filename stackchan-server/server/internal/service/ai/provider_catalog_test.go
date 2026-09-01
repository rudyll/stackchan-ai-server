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

func TestModelsEndpointRejectsInvalidURL(t *testing.T) {
	if _, err := modelsEndpoint("file:///tmp/provider"); err == nil {
		t.Fatal("expected non-http model URL to be rejected")
	}
}
