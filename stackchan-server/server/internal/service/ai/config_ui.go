/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

package ai

import (
	"encoding/json"
	"net/http"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
)

// StartConfigUI serves only on the add-on ingress port (not a host-mapped
// port). Home Assistant authenticates access before proxying it here.
func StartConfigUI() {
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
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(configUIHTML))
	})
	go func() {
		g.Log().Infof(gctx.New(), "[CONFIG] ingress settings UI listening on :8099")
		if err := http.ListenAndServe(":8099", mux); err != nil {
			g.Log().Errorf(gctx.New(), "[CONFIG] ingress UI: %v", err)
		}
	}()
}

const configUIHTML = `<!doctype html><html lang="zh-CN"><meta name="viewport" content="width=device-width,initial-scale=1"><title>StackChan 设置</title><style>
body{margin:0;background:#10131a;color:#e8edf5;font:15px system-ui,-apple-system,sans-serif}main{max-width:850px;margin:auto;padding:24px}h1{font-size:24px}nav{display:flex;flex-wrap:wrap;gap:8px;margin:20px 0}button{border:0;border-radius:8px;padding:10px 14px;background:#273246;color:#fff;cursor:pointer}button.active,button.save{background:#3d7eff}.panel{display:none;background:#18202d;padding:20px;border-radius:12px}.panel.active{display:block}.field{margin:13px 0}.field label{display:block;font-weight:650;margin-bottom:5px}.hint{font-size:12px;color:#a7b2c6;margin:4px 0}input,select,textarea{box-sizing:border-box;width:100%;padding:10px;border:1px solid #3c4960;border-radius:7px;background:#101722;color:#fff}textarea{min-height:110px;font-family:ui-monospace,monospace}.provider{display:none}.provider.show{display:block}.notice{margin:14px 0;color:#8de0ad}.warn{color:#ffd280}</style><main>
<h1>StackChan AI 设置</h1><p class="hint">保存后，新建对话会使用新设置；让设备断开重连即可立即切换。</p><nav><button class="tab active" data-tab="basic">基础</button><button class="tab" data-tab="voice">语音管线</button><button class="tab" data-tab="background">后台任务</button><button class="tab" data-tab="devices">设备 Profile</button></nav>
<section id="basic" class="panel active"><div class="field"><label>AI Provider</label><select name="provider" id="provider"><option value="openai">OpenAI Realtime</option><option value="gemini">Gemini Live</option><option value="tokenhub">Tencent TokenHub（LLM）</option><option value="openrouter">OpenRouter（LLM）</option><option value="openai_compatible">OpenAI-compatible（LLM）</option></select></div>
<div class="provider" data-for="openai"><div class="field"><label>OpenAI API Key</label><input name="openai_api_key" type="password"></div><div class="field"><label>Realtime model</label><input name="openai_realtime_model" placeholder="gpt-realtime"></div><div class="field"><label>Voice</label><input name="openai_tts_voice" placeholder="alloy"></div></div>
<div class="provider" data-for="gemini"><div class="field"><label>Gemini API Key</label><input name="gemini_api_key" type="password"></div><div class="field"><label>Live model</label><input name="gemini_model" placeholder="gemini-2.5-flash-native-audio-latest"></div><div class="field"><label>Voice</label><input name="gemini_voice" placeholder="Aoede"></div></div>
<div class="provider" data-for="tokenhub"><div class="field"><label>TokenHub Base URL</label><input name="tokenhub_base_url"></div><div class="field"><label>TokenHub API Key</label><input name="tokenhub_api_key" type="password"></div></div><div class="provider" data-for="openrouter"><div class="hint">OpenRouter uses https://openrouter.ai/api/v1 automatically.</div><div class="field"><label>OpenRouter API Key</label><input name="openrouter_api_key" type="password"></div></div><div class="provider" data-for="openai_compatible"><div class="field"><label>LLM Base URL</label><input name="llm_base_url"></div><div class="field"><label>LLM API Key</label><input name="llm_api_key" type="password"></div></div>
<div class="field"><label>System prompt</label><textarea name="system_prompt"></textarea></div></section>
<section id="voice" class="panel"><p class="hint">仅在 TokenHub、OpenRouter 或 OpenAI-compatible 模式使用。三段可指向不同的兼容服务。</p><div class="field"><label>STT Base URL / API Key / Model</label><input name="stt_base_url" placeholder="https://.../v1"><input name="stt_api_key" type="password" placeholder="API Key"><input name="stt_model" placeholder="transcription model"></div><div class="field"><label>LLM Base URL / API Key / Model</label><input name="llm_base_url" placeholder="optional override"><input name="llm_api_key" type="password" placeholder="optional override"><input name="llm_model" placeholder="chat model"></div><div class="field"><label>TTS Base URL / API Key / Model / Voice</label><input name="tts_base_url" placeholder="https://.../v1"><input name="tts_api_key" type="password" placeholder="API Key"><input name="tts_model" placeholder="speech model"><input name="tts_voice" placeholder="voice"></div><div class="field"><label>Initial audio buffer / maximum wait (ms)</label><input name="audio_prebuffer_ms" type="number" placeholder="300"><input name="audio_prebuffer_max_wait_ms" type="number" placeholder="900"></div><p class="warn hint">声音与模型由服务商决定，页面使用自由输入而非可能过期的固定下拉列表。</p></section>
<section id="background" class="panel"><p class="hint">仅 OpenAI Realtime 前台使用。长任务进入每台设备独立的后台队列，短 HA 控制仍走实时工具。</p><div class="field"><label>启用后台任务</label><select name="background_tasks_enabled"><option value="false">关闭</option><option value="true">开启</option></select></div><div class="field"><label>Agent Base URL / API Key / Model</label><input name="background_agent_base_url" placeholder="https://api.openai.com"><input name="background_agent_api_key" type="password" placeholder="留空时复用 LLM / compatible / OpenAI Key"><input name="background_agent_model" placeholder="必须填写 Chat Completions 模型"></div><div class="field"><label>任务超时（秒）</label><input name="background_agent_timeout_seconds" type="number" min="10" max="1800" placeholder="300"></div><div class="field"><label>后台 Agent prompt</label><textarea name="background_agent_prompt" placeholder="定义后台 Agent 的执行和结果格式"></textarea></div><p class="warn hint">后台 Agent 会连接 Home Assistant 并可能执行设备操作。请使用受信任的模型和最小权限令牌。</p></section>
<section id="devices" class="panel"><p class="hint">按设备 WebSocket Device-Id 覆盖 provider、prompt、模型或声音。API Key 仍使用基础设置。</p><textarea name="device_profiles" placeholder='{"AA:BB:CC:DD:EE:FF":{"system_prompt":"...","tts_voice":"..."}}'></textarea></section><p id="status" class="notice"></p><button class="save" id="save">保存设置</button></main><script>
const q=s=>document.querySelector(s),all=s=>[...document.querySelectorAll(s)];let data={};function show(){let p=q('#provider').value;all('.provider').forEach(x=>x.classList.toggle('show',x.dataset.for===p))}all('.tab').forEach(b=>b.onclick=()=>{all('.tab,.panel').forEach(x=>x.classList.remove('active'));b.classList.add('active');q('#'+b.dataset.tab).classList.add('active')});fetch('api/settings').then(r=>r.json()).then(x=>{data=x;all('[name]').forEach(e=>e.value=x[e.name]||'');show()});q('#provider').onchange=show;q('#save').onclick=()=>{all('[name]').forEach(e=>data[e.name]=e.value);fetch('api/settings',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify(data)}).then(r=>r.ok?(q('#status').textContent='已保存。重新连接设备后生效。'):(q('#status').textContent='保存失败。'))};</script></html>`
