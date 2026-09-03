#!/usr/bin/env python3
"""Exercise both real launchers without HA credentials, AI calls or hardware.

Requires Docker with Compose v2 and an already-built image. Only containers and
volumes created under this invocation's unique names are removed on exit.
"""

import argparse
import json
import os
from pathlib import Path
import socket
import subprocess
import tempfile
import time
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen
import uuid

ROOT = Path(__file__).resolve().parents[1]
ICON = (ROOT / "stackchan-server/server/internal/service/ai/assets/stackchan-mark.png").read_bytes()
HOST = "stackchan.example"
TOKEN = "container-test-token"


def run(*args, **kwargs):
    return subprocess.run(args, check=True, capture_output=True, text=True, **kwargs).stdout.strip()


def request(base, path, token="", values=None, expected=200):
    headers = {"Authorization": "Bearer " + token} if token else {}
    data = None
    if values is not None:
        data = json.dumps(values).encode()
        headers["Content-Type"] = "application/json"
    req = Request(base + path, headers=headers, data=data, method="PUT" if data else "GET")
    try:
        response = urlopen(req, timeout=5)
    except HTTPError as error:
        response = error
    with response:
        assert response.status == expected, (path, response.status, expected)
        return response.read(), response.headers


def wait_ready(base):
    deadline = time.monotonic() + 90
    while time.monotonic() < deadline:
        try:
            request(base, "/")
            return
        except (URLError, TimeoutError, ConnectionError):
            time.sleep(1)
    raise AssertionError("Settings listener did not become ready: " + base)


def free_ports():
    # Keep both reservations until distinct ports have been selected.
    with socket.socket() as first, socket.socket() as second:
        first.bind(("127.0.0.1", 0))
        second.bind(("127.0.0.1", 0))
        return first.getsockname()[1], second.getsockname()[1]


def check_runtime(settings_port, device_port, advertised_port, ha):
    base = f"http://127.0.0.1:{settings_port}"
    wait_ready(base)
    token = "" if ha else TOKEN
    if not ha:
        for path in ("/api/settings", "/api/device-setup", "/assets/private-file"):
            request(base, path, expected=401)
        request(base, "/api/settings", token="incorrect", expected=401)
    html, headers = request(base, "/", token)
    assert b'assets/stackchan-icon.png' in html
    assert headers["X-Frame-Options"] == ("SAMEORIGIN" if ha else "DENY")
    assert request(base, "/assets/stackchan-icon.png")[0] == ICON
    settings = json.loads(request(base, "/api/settings", token)[0])
    assert settings["ui_ha_enabled"] == str(ha).lower(), settings["ui_ha_enabled"]
    assert settings["gemini_enable_tools"] == "false", settings["gemini_enable_tools"]
    assert settings["gemini_enable_search"] == str(ha).lower()
    setup = json.loads(request(base, "/api/device-setup", token)[0])
    assert setup == {"ha_enabled": ha, "host": HOST, "port": advertised_port,
                     "ota_url": f"http://{HOST}:{advertised_port}/xiaozhi/ota/"}, setup
    ota = json.loads(request(f"http://127.0.0.1:{device_port}", "/xiaozhi/ota/")[0])
    assert ota["websocket"]["url"] == f"ws://{HOST}:{advertised_port}/xiaozhi/ws", ota
    request(base, "/api/settings", token, {"ha_enabled": "false"}, expected=400)
    request(base, "/api/settings", token, {"system_prompt": "Container persistence check"})
    print(f"PASS {'HA' if ha else 'standalone'}: settings, auth, logo, device setup, OTA, save", flush=True)
    return base, token


def check_persistence(base, token):
    wait_ready(base)
    settings = json.loads(request(base, "/api/settings", token)[0])
    assert settings["system_prompt"] == "Container persistence check"
    print("PASS settings survive restart", flush=True)


def published_port(container, port):
    return int(run("docker", "port", container, f"{port}/tcp").rsplit(":", 1)[1])


def restart_ha_and_check(container, token):
    previous_ui = published_port(container, 8099)
    run("docker", "restart", container)
    # Docker can reassign ephemeral published ports after restart.
    restarted_ui = published_port(container, 8099)
    print(f"HA settings port before/after restart: {previous_ui}/{restarted_ui}", flush=True)
    check_persistence(f"http://127.0.0.1:{restarted_ui}", token)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--image", required=True)
    parser.add_argument("--platform", required=True)
    args = parser.parse_args()
    name = "stackchan-smoke-" + uuid.uuid4().hex[:12]
    with tempfile.TemporaryDirectory(prefix=name) as directory:
        work = Path(directory)
        options = work / "options.json"
        options.write_text(json.dumps({"local_host": HOST, "ha_enabled": True,
                                       "gemini_enable_tools": False, "gemini_enable_search": True}))
        env = os.environ.copy()
        # Never inherit a developer's provider credentials or deployment paths.
        env = {k: v for k, v in env.items() if not k.startswith(("STACKCHAN_", "COMPOSE_"))}
        ws_port, ui_port = free_ports()
        env.update(STACKCHAN_LOCAL_HOST=HOST, STACKCHAN_WS_PORT=str(ws_port),
                   STACKCHAN_SETTINGS_PORT=str(ui_port), STACKCHAN_SETTINGS_TOKEN=TOKEN)
        envfile = work / "empty.env"
        envfile.write_text("")
        override = work / "compose.json"
        override.write_text(json.dumps({"services": {"stackchan": {
            "image": args.image, "platform": args.platform, "volumes": ["test-data:/data"]}},
            "volumes": {"test-data": {}}}))
        compose = ["docker", "compose", "--env-file", str(envfile), "-p", name,
                   "-f", str(ROOT / "stackchan-server/docker-compose.standalone.yml"), "-f", str(override)]
        container = ""
        compose_started = False
        try:
            container = run("docker", "create", "--name", name + "-ha", "--platform", args.platform,
                            "-v", "/data", "-p", "127.0.0.1::8099", "-p", "127.0.0.1::12800", args.image)
            run("docker", "cp", str(options), container + ":/data/options.json")
            run("docker", "start", container)
            ha_ui = published_port(container, 8099)
            ha_ws = published_port(container, 12800)
            base, token = check_runtime(ha_ui, ha_ws, 12800, ha=True)
            restart_ha_and_check(container, token)
            compose_started = True
            run(*compose, "up", "-d", "--no-build", "--pull", "never", env=env)
            base, token = check_runtime(ui_port, ws_port, ws_port, ha=False)
            run(*compose, "restart", env=env)
            check_persistence(base, token)
        except Exception:
            if container:
                logs = subprocess.run(["docker", "logs", "--tail", "100", container],
                                      capture_output=True, text=True)
                print(logs.stdout + logs.stderr)
            if compose_started:
                print(subprocess.run([*compose, "logs", "--no-color", "--tail", "100"],
                                     env=env, capture_output=True, text=True).stdout)
            raise
        finally:
            if compose_started:
                run(*compose, "down", "--volumes", env=env)
            if container:
                run("docker", "rm", "-f", "-v", container)


if __name__ == "__main__":
    main()
