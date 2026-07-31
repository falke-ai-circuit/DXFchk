#!/usr/bin/env python3
"""Debug fs-hash endpoint."""
import json, subprocess

BASE = "http://187.124.31.229:80"
AGENT_ID = "vegas-c2022"

# Login
r = subprocess.run(["curl", "-s", "-X", "POST", f"{BASE}/api/v1/login",
     "-H", "Content-Type: application/json",
     "-d", json.dumps({"username": "admin", "password": "falke-admin-2026"}),
     "--max-time", "10"], capture_output=True, text=True, timeout=15)
token = json.loads(r.stdout)["data"]["token"]

# Test file path (already created in previous run)
test_file = "C:\\Users\\Administrator\\Desktop\\template_stability_test\\AMR02p3_test_602_p5000_p0006.dxf"

# Try fs-hash
r2 = subprocess.run(["curl", "-s", "-X", "POST", f"{BASE}/api/v1/agents/{AGENT_ID}/fs-hash",
     "-H", "Content-Type: application/json",
     "-H", f"Authorization: Bearer {token}",
     "-d", json.dumps({"path": test_file}),
     "--max-time", "30"], capture_output=True, text=True, timeout=35)
print("fs-hash response:", r2.stdout[:500])

# Try fs-read (just first 100 bytes to test)
r3 = subprocess.run(["curl", "-s", "-X", "POST", f"{BASE}/api/v1/agents/{AGENT_ID}/fs-read",
     "-H", "Content-Type: application/json",
     "-H", f"Authorization: Bearer {token}",
     "-d", json.dumps({"path": test_file, "limit": 100}),
     "--max-time", "60"], capture_output=True, text=True, timeout=65)
resp3 = json.loads(r3.stdout)
if resp3.get("ok"):
    import base64
    data = resp3["data"].get("data", "")
    if data:
        decoded = base64.b64decode(data)
        print(f"fs-read OK, got {len(decoded)} bytes")
        print(f"First 100 chars: {decoded[:100]}")
    else:
        print(f"fs-read OK but no data: {json.dumps(resp3['data'])[:200]}")
else:
    print(f"fs-read error: {json.dumps(resp3)[:300]}")

# Try exec with powershell to get hash
r4 = subprocess.run(["curl", "-s", "-X", "POST", f"{BASE}/api/v1/agents/{AGENT_ID}/exec",
     "-H", "Content-Type: application/json",
     "-H", f"Authorization: Bearer {token}",
     "-d", json.dumps({"command": f"powershell -Command \"(Get-FileHash '{test_file}' -Algorithm SHA256).Hash\"", "timeout": 15}),
     "--max-time", "25"], capture_output=True, text=True, timeout=30)
resp4 = json.loads(r4.stdout)
if resp4.get("ok"):
    print(f"PS hash: {resp4['data'].get('stdout', '')[:100]}")
else:
    print(f"PS error: {json.dumps(resp4)[:300]}")

# Try simple exec with dir
r5 = subprocess.run(["curl", "-s", "-X", "POST", f"{BASE}/api/v1/agents/{AGENT_ID}/exec",
     "-H", "Content-Type: application/json",
     "-H", f"Authorization: Bearer {token}",
     "-d", json.dumps({"command": f"dir {test_file}", "timeout": 10}),
     "--max-time", "20"], capture_output=True, text=True, timeout=25)
resp5 = json.loads(r5.stdout)
if resp5.get("ok"):
    print(f"dir result: {resp5['data'].get('stdout', '')[:200]}")
else:
    print(f"dir error: {json.dumps(resp5)[:300]}")