/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

package ai

import "sync"

// upstreamCreds holds the real Xiaozhi WebSocket URL and token fetched during OTA.
type upstreamCreds struct {
	URL     string
	Token   string
	Version int
}

var (
	sessionMu sync.RWMutex
	// Key: Device-Id (MAC address). Value: upstreamCreds from the real OTA response.
	sessionStore = map[string]upstreamCreds{}
)

func storeDeviceCreds(deviceID string, creds upstreamCreds) {
	sessionMu.Lock()
	sessionStore[deviceID] = creds
	sessionMu.Unlock()
}

func loadDeviceCreds(deviceID string) (upstreamCreds, bool) {
	sessionMu.RLock()
	creds, ok := sessionStore[deviceID]
	sessionMu.RUnlock()
	return creds, ok
}
