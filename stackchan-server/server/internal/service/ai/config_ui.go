/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

package ai

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
)

const settingsSessionCookie = "stackchan_settings_session"

const settingsSessionMaxAge = 12 * 60 * 60

var readOnlySettings = map[string]struct{}{
	"ha_enabled":    {},
	"ui_ha_enabled": {},
}

// StartConfigUI serves the settings UI on the configured address. HA add-ons
// rely on Ingress authentication; standalone runtime supplies a Bearer token.
func StartConfigUI() {
	ctx := gctx.New()
	listenAddress := g.Cfg().MustGet(ctx, "ai.settings_listen_address", ":8099").String()
	authToken := g.Cfg().MustGet(ctx, "ai.settings_auth_token", "").String()
	requireAuth := !aiBool(ctx, "ha_enabled", true)
	if requireAuth && strings.TrimSpace(authToken) == "" {
		g.Log().Errorf(ctx, "[CONFIG] standalone settings UI has no auth token; all requests will be rejected")
	}
	handler := configUIAuth(configUIHandler(), authToken, requireAuth)
	go func() {
		g.Log().Infof(gctx.New(), "[CONFIG] settings UI listening on %s (auth_required=%t)", listenAddress, requireAuth)
		if err := http.ListenAndServe(listenAddress, handler); err != nil {
			g.Log().Errorf(gctx.New(), "[CONFIG] settings UI: %v", err)
		}
	}()
}

func configUIHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/settings", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(settingsForUI(gctx.New()))
		case http.MethodPut:
			values := map[string]string{}
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128*1024)).Decode(&values); err != nil {
				http.Error(w, "invalid settings", http.StatusBadRequest)
				return
			}
			if reason := settingsUpdateError(values); reason != "" {
				http.Error(w, reason, http.StatusBadRequest)
				return
			}
			if err := writeSettings(values); err != nil {
				http.Error(w, "could not save settings", http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		default:
			w.Header().Set("Allow", "GET, PUT")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/provider-catalog", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		request := providerCatalogSettings{}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128*1024)).Decode(&request); err != nil {
			http.Error(w, "invalid provider settings", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(discoverProviderCatalog(r.Context(), request.Provider, request.Settings))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(configUIHTML))
	})
	return mux
}

func settingsUpdateError(values map[string]string) string {
	for key := range values {
		if _, ok := readOnlySettings[key]; ok {
			return "runtime mode is read-only"
		}
		if !isSettingsUIKey(key) {
			return "unsupported setting"
		}
	}
	return ""
}

func isSettingsUIKey(key string) bool {
	for _, allowed := range settingsUIKeys {
		if key == allowed {
			return true
		}
	}
	return false
}

func configUIAuth(next http.Handler, token string, required bool) http.Handler {
	if !required {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			handleConfigUILogin(w, r, token)
			return
		}
		if r.URL.Path == "/logout" && r.Method == http.MethodGet {
			http.SetCookie(w, &http.Cookie{Name: settingsSessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if settingsRequestAuthenticated(r, token) {
			next.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/" && r.Method == http.MethodGet && r.Header.Get("Authorization") == "" {
			serveConfigUILogin(w, "")
			return
		}
		w.Header().Set("WWW-Authenticate", `Bearer realm="StackChan settings"`)
		http.Error(w, "authentication required", http.StatusUnauthorized)
	})
}

func settingsRequestAuthenticated(r *http.Request, token string) bool {
	const prefix = "Bearer "
	authorization := r.Header.Get("Authorization")
	if strings.HasPrefix(authorization, prefix) && matchesSettingsToken(strings.TrimSpace(strings.TrimPrefix(authorization, prefix)), token) {
		return true
	}
	cookie, err := r.Cookie(settingsSessionCookie)
	return err == nil && matchesSettingsToken(cookie.Value, token)
}

func matchesSettingsToken(provided, token string) bool {
	return token != "" && subtle.ConstantTimeCompare([]byte(provided), []byte(token)) == 1
}

func handleConfigUILogin(w http.ResponseWriter, r *http.Request, token string) {
	if r.Method == http.MethodGet {
		serveConfigUILogin(w, "")
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8*1024)
	if err := r.ParseForm(); err != nil || !matchesSettingsToken(strings.TrimSpace(r.FormValue("token")), token) {
		w.WriteHeader(http.StatusUnauthorized)
		serveConfigUILogin(w, "Token 不正确，请重试。")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: settingsSessionCookie, Value: token, Path: "/", MaxAge: settingsSessionMaxAge, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func serveConfigUILogin(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if message == "" {
		_, _ = w.Write([]byte(configUILoginHTML))
		return
	}
	page := strings.Replace(configUILoginHTML, "<p id=\"error\"></p>", "<p id=\"error\">"+message+"</p>", 1)
	_, _ = w.Write([]byte(page))
}
