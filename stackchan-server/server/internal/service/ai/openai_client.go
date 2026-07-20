/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

package ai

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
)

type openAIClient struct {
	baseURL   string
	apiKey    string
	model     string
	sttModel  string
	ttsModel  string
	ttsVoice  string
	sysPrompt string
	http      *http.Client
}

type chatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
}

type toolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func newOpenAIClient(baseURL, apiKey, model, sttModel, ttsModel, ttsVoice, sysPrompt string) *openAIClient {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	if sttModel == "" {
		sttModel = "whisper-1"
	}
	if ttsModel == "" {
		ttsModel = "tts-1"
	}
	return &openAIClient{
		baseURL:   baseURL,
		apiKey:    apiKey,
		model:     model,
		sttModel:  sttModel,
		ttsModel:  ttsModel,
		ttsVoice:  ttsVoice,
		sysPrompt: sysPrompt,
		http:      &http.Client{},
	}
}

func (c *openAIClient) doRequest(ctx context.Context, method, path string, body io.Reader, contentType string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("OpenAI %s %s: %s", method, path, string(data))
	}
	return data, nil
}

// Transcribe sends WAV audio to Whisper and returns the transcribed text.
func (c *openAIClient) Transcribe(ctx context.Context, wavBytes []byte) (string, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "audio.wav")
	if err != nil {
		return "", err
	}
	if _, err := fw.Write(wavBytes); err != nil {
		return "", err
	}
	_ = mw.WriteField("model", c.sttModel)
	mw.Close()

	data, err := c.doRequest(ctx, "POST", "/v1/audio/transcriptions", &buf, mw.FormDataContentType())
	if err != nil {
		return "", err
	}
	var resp struct {
		Text string `json:"text"`
	}
	return resp.Text, json.Unmarshal(data, &resp)
}

// Chat sends the conversation history to the model, handles HA tool calls in a loop,
// and returns the final text reply.
func (c *openAIClient) Chat(ctx context.Context, history []chatMessage, ha *haWSClient) (string, error) {
	logCtx := gctx.New()
	now := time.Now().Format("2006-01-02 15:04:05 MST")
	sysContent := fmt.Sprintf("Current date/time: %s\n\n%s", now, c.sysPrompt)

	msgs := make([]chatMessage, 0, len(history)+1)
	msgs = append(msgs, chatMessage{Role: "system", Content: sysContent})
	msgs = append(msgs, history...)
	tools := haOpenAITools()

	for {
		body, _ := json.Marshal(map[string]any{
			"model":    c.model,
			"messages": msgs,
			"tools":    tools,
		})
		data, err := c.doRequest(ctx, "POST", "/v1/chat/completions", bytes.NewReader(body), "application/json")
		if err != nil {
			return "", err
		}

		var resp struct {
			Choices []struct {
				Message struct {
					Content   string     `json:"content"`
					ToolCalls []toolCall `json:"tool_calls"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(data, &resp); err != nil || len(resp.Choices) == 0 {
			return "", fmt.Errorf("unexpected OpenAI response: %s", string(data))
		}

		choice := resp.Choices[0].Message
		if len(choice.ToolCalls) == 0 {
			return strings.TrimSpace(choice.Content), nil
		}

		// Execute tool calls and loop for the follow-up reply.
		msgs = append(msgs, chatMessage{Role: "assistant", ToolCalls: choice.ToolCalls})
		for _, tc := range choice.ToolCalls {
			var args map[string]any
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
			g.Log().Infof(logCtx, "[HA] tool=%s args=%v", tc.Function.Name, args)
			result, dispErr := dispatchHATool(ha, tc.Function.Name, args)
			if dispErr != nil {
				result = "error: " + dispErr.Error()
			}
			g.Log().Infof(logCtx, "[HA] tool=%s result=%s", tc.Function.Name, result)
			msgs = append(msgs, chatMessage{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}
}

// Speak sends text to OpenAI TTS and returns 24kHz mono int16 PCM.
func (c *openAIClient) Speak(ctx context.Context, text string) ([]int16, error) {
	body, _ := json.Marshal(map[string]any{
		"model":           c.ttsModel,
		"voice":           c.ttsVoice,
		"input":           text,
		"response_format": "pcm",
	})
	data, err := c.doRequest(ctx, "POST", "/v1/audio/speech", bytes.NewReader(body), "application/json")
	if err != nil {
		return nil, err
	}
	pcm := make([]int16, len(data)/2)
	for i := range pcm {
		pcm[i] = int16(binary.LittleEndian.Uint16(data[i*2:]))
	}
	return pcm, nil
}

// haOpenAITools converts the 7 HA tool defs (MCP inputSchema format) to OpenAI function calling format.
func haOpenAITools() []map[string]any {
	defs := haToolDefs()
	tools := make([]map[string]any, 0, len(defs))
	for _, t := range defs {
		tools = append(tools, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        t["name"],
				"description": t["description"],
				"parameters":  t["inputSchema"],
			},
		})
	}
	return tools
}
