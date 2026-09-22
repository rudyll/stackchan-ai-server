/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

package ai

import "testing"

func TestNormalizeOpenAIBaseURLAcceptsBothSpellings(t *testing.T) {
	// The settings UI hints at ".../v1" while the request helpers add the full
	// "/v1/..." path, so a base URL copied from the hint used to produce
	// "/v1/v1/audio/transcriptions" and every pipeline stage answered 404.
	tests := []struct {
		baseURL string
		want    string
	}{
		{baseURL: "https://api.minimaxi.com/v1", want: "https://api.minimaxi.com"},
		{baseURL: "https://api.minimaxi.com/v1/", want: "https://api.minimaxi.com"},
		{baseURL: "https://api.minimaxi.com", want: "https://api.minimaxi.com"},
		{baseURL: " http://127.0.0.1:8801/v1 ", want: "http://127.0.0.1:8801"},
		{baseURL: "https://openrouter.ai/api/v1", want: "https://openrouter.ai/api"},
		{baseURL: "https://gateway.test/openai/v1", want: "https://gateway.test/openai"},
		{baseURL: "", want: ""},
	}
	for _, test := range tests {
		if got := normalizeOpenAIBaseURL(test.baseURL); got != test.want {
			t.Fatalf("normalizeOpenAIBaseURL(%q) = %q, want %q", test.baseURL, got, test.want)
		}
	}
}

func TestNewOpenAIClientBuildsSingleVersionedPath(t *testing.T) {
	client := newOpenAIClient("https://api.minimaxi.com/v1", "key", "MiniMax-M3", "", "", "", "")
	if got, want := client.baseURL+"/v1/chat/completions", "https://api.minimaxi.com/v1/chat/completions"; got != want {
		t.Fatalf("chat completions URL = %q, want %q", got, want)
	}
}

func TestModelsEndpointMatchesRuntimePath(t *testing.T) {
	// Discovery and the voice pipeline must agree about the base URL, otherwise
	// a configuration that passes the model check still fails every turn.
	for _, baseURL := range []string{"https://compatible.test", "https://compatible.test/v1"} {
		got, err := modelsEndpoint(baseURL)
		if err != nil {
			t.Fatalf("modelsEndpoint(%q) error = %v", baseURL, err)
		}
		if want := "https://compatible.test/v1/models"; got != want {
			t.Fatalf("modelsEndpoint(%q) = %q, want %q", baseURL, got, want)
		}
	}
}
