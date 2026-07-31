#!/usr/bin/env python3
"""Check PROBE agent status."""
import json, subprocess

BASE = "http://187.124.31.229:80"

r = subprocess.run(["curl", "-s", "-X", "POST", f"{BASE}/api/v1/login",
     "-H", "Content-Type: application/json",
     "-d", json.dumps({"username": "admin", "password": "falke-admin-2026"}),
     "--max-time", "10"], capture_output=True, text=True, timeout=15)
token = json.loads(r.stdout)["data"]["token"]

r2 = subprocess.run(["curl", "-s", f"{BASE}/api/v1/agents",
     "-H", f"Authorization: Bearer {token}",
     "--max-time", "10"], capture_output=True, text=True, timeout=15)
resp = json.loads(r2.stdout)
if isinstance(resp, list):
    agents = resp
elif isinstance(resp, dict):
    d = resp.get("data", resp)
    if isinstance(d, list):
        agents = d
    elif isinstance(d, dict):
        agents = d.get("agents", [])
    else:
        agents = []
else:
    agents = []
print(f"Connected agents: {len(agents)}")
for a in agents:
    print(f"  {a.get('agent_id', '?')}: status={a.get('status', '?')} name={a.get('name', '?')}")