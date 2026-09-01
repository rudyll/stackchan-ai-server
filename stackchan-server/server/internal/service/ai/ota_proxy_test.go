/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

package ai

import "testing"

func TestOTAResponseUsesAdvertisedPort(t *testing.T) {
	response := otaResponse("192.168.1.100", 12801)
	websocket, ok := response["websocket"].(map[string]any)
	if !ok {
		t.Fatal("websocket response is missing")
	}
	if got, want := websocket["url"], "ws://192.168.1.100:12801/xiaozhi/ws"; got != want {
		t.Fatalf("websocket URL = %v, want %s", got, want)
	}
}
