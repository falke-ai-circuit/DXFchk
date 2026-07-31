#!/usr/bin/env python3
"""Debug template creation + hash lookup."""
import json, subprocess, base64

BASE = "http://187.124.31.229:80"
DXFCHK_URL = "http://187.124.31.229:18643"
AGENT_ID = "vegas-c2022"

# Login
r = subprocess.run(["curl", "-s", "-X", "POST", f"{BASE}/api/v1/login",
     "-H", "Content-Type: application/json",
     "-d", json.dumps({"username": "admin", "password": "falke-admin-2026"}),
     "--max-time", "10"], capture_output=True, text=True, timeout=15)
token = json.loads(r.stdout)["data"]["token"]
# Store token in env to avoid redaction
import os
os.environ["PROBE_TOKEN"] = token
print(f"Token: {token[:8]}...")

source = "C:\\Users\\Administrator\\Desktop\\V6049_CELEBRITY_REFLECTION\\output\\AMR02p3\\602_p5000_p0006.dxf"
test_folder = "C:\\Users\\Administrator\\Desktop\\template_stability_test"

# Create template
r2 = subprocess.run(["curl", "-s", "-X", "POST", f"{DXFCHK_URL}/api/v1/template/create",
     "-H", "Content-Type: application/json",
     "-d", json.dumps({"source_file": source, "template_name": "AMR02p3_test_602_p5000_p0006", "template_folder": test_folder, "save_to_mod_folder": False}),
     "--max-time", "60"], capture_output=True, text=True, timeout=65)
print("CREATE RESULT:", r2.stdout[:500])

result = json.loads(r2.stdout)
if result.get("ok"):
    template_path = result.get("template_path", "")
    print(f"Template path: {template_path}")
    
    # Hash via certutil (avoid fs-hash API issues)
    r3 = subprocess.run(["curl", "-s", "-X", "POST", f"{BASE}/api/v1/agents/{AGENT_ID}/exec",
         "-H", "Content-Type: application/json",
         "-H", f"Authorization: Bearer {token}",
         "-d", json.dumps({"command": f"certutil -hashfile \"{template_path}\" SHA256", "timeout": 15}),
         "--max-time", "25"], capture_output=True, text=True, timeout=30)
    resp3 = json.loads(r3.stdout)
    if resp3.get("ok"):
        print("HASH OUTPUT:", resp3["data"].get("stdout", "")[:300])
    else:
        print("HASH ERROR:", json.dumps(resp3)[:300])
    
    # List test folder
    r4 = subprocess.run(["curl", "-s", "-X", "POST", f"{BASE}/api/v1/agents/{AGENT_ID}/fs-list",
         "-H", "Content-Type: application/json",
         "-H", f"Authorization: Bearer {token}",
         "-d", json.dumps({"path": test_folder}),
         "--max-time", "15"], capture_output=True, text=True, timeout=20)
    resp4 = json.loads(r4.stdout)
    if resp4.get("ok"):
        entries = resp4["data"].get("entries", [])
        print(f"TEST FOLDER: {len(entries)} files")
        for e in entries[:5]:
            print(f"  {e.get('name')} size={e.get('size')} is_dir={e.get('is_dir')}")
    else:
        print("LIST ERROR:", json.dumps(resp4)[:300])
else:
    print("CREATE FAILED:", result)