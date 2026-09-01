/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

package ai

import "testing"

func TestOpenAIRealtimeSessionUpdateUsesCurrentAudioShape(t *testing.T) {
	event := openAIRealtimeSessionUpdate("instructions", "marin", nil)
	session, ok := event["session"].(map[string]any)
	if !ok {
		t.Fatal("session config missing")
	}
	if _, ok := session["input_audio_format"]; ok {
		t.Fatal("legacy input_audio_format must not be sent")
	}
	if _, ok := session["output_audio_format"]; ok {
		t.Fatal("legacy output_audio_format must not be sent")
	}
	if got := session["output_modalities"]; got == nil {
		t.Fatal("output_modalities missing")
	}

	audio, ok := session["audio"].(map[string]any)
	if !ok {
		t.Fatal("audio config missing")
	}
	input, ok := audio["input"].(map[string]any)
	if !ok {
		t.Fatal("audio.input config missing")
	}
	inputFormat, ok := input["format"].(map[string]any)
	if !ok {
		t.Fatal("audio.input.format missing")
	}
	if inputFormat["type"] != "audio/pcm" || inputFormat["rate"] != serverSampleRate {
		t.Fatalf("unexpected input format: %#v", inputFormat)
	}
	if _, ok := input["transcription"]; !ok {
		t.Fatal("audio.input.transcription missing")
	}
	if _, ok := input["turn_detection"]; !ok {
		t.Fatal("audio.input.turn_detection missing")
	}

	output, ok := audio["output"].(map[string]any)
	if !ok {
		t.Fatal("audio.output config missing")
	}
	outputFormat, ok := output["format"].(map[string]any)
	if !ok {
		t.Fatal("audio.output.format missing")
	}
	if outputFormat["type"] != "audio/pcm" || outputFormat["rate"] != serverSampleRate {
		t.Fatalf("unexpected output format: %#v", outputFormat)
	}
	if output["voice"] != "marin" {
		t.Fatalf("unexpected voice: %#v", output["voice"])
	}
}

func TestResamplePCM16ToRealtimeRate(t *testing.T) {
	input := []int16{-1000, 1000}
	output := resamplePCM16(input, deviceSampleRate, serverSampleRate)
	if len(output) != 3 {
		t.Fatalf("expected 3 samples, got %d", len(output))
	}
	if output[0] != input[0] || output[len(output)-1] != input[len(input)-1] {
		t.Fatalf("resampling changed endpoints: %#v", output)
	}
	if output[1] <= -1000 || output[1] >= 1000 {
		t.Fatalf("expected interpolated middle sample, got %d", output[1])
	}
}
