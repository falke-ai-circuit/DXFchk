#!/usr/bin/env python3
"""Deploy dxfchk.exe v0.8.1 to Vegas via bitsadmin + restart."""
import json, subprocess, time

BASE = "http://187.124.31.229:80"
AGENT_ID = "vegas-c2022"

r = subprocess.run(["curl", "-s", "-X", "POST", f"{BASE}/api/v1/login",
     "-H", "Content-Type: application/json",
     "-d", json.dumps({"username": "admin", "password": "falke-admin-2026"}),
     "--max-time", "10"], capture_output=True, text=True, timeout=15)
token = json.loads(r.stdout)["data"]["token"]

def exec_cmd(command, timeout=60):
    r = subprocess.run(["curl", "-s", "-X", "POST", f"{BASE}/api/v1/agents/{AGENT_ID}/exec",
         "-H", "Content-Type: application/json",
         "-H", f"Authorization: Bearer {token}",
         "-d", json.dumps({"command": command, "timeout": timeout}),
         "--max-time", str(timeout + 10)], capture_output=True, text=True, timeout=timeout + 15)
    resp = json.loads(r.stdout)
    if resp.get("ok"):
        return resp["data"].get("stdout", ""), resp["data"].get("exit_code", 0)
    return f"ERROR: {json.dumps(resp)[:200]}", -1

# Step 1: Stop old DXFchk
print("Stopping old DXFchk...")
stdout, _ = exec_cmd("taskkill /f /im dxfchk.exe", timeout=15)
print(f"  {stdout.strip()}")

time.sleep(2)

# Step 2: Download new binary via bitsadmin
print("Downloading dxfchk.exe v0.8.1...")
stdout, _ = exec_cmd(
    "bitsadmin /transfer dxfchk_dl http://187.124.31.229:80/download/dxfchk.exe?token=probe-secure-token-2026 C:\\Users\\Public\\dxfchk.exe",
    timeout=60
)
print(f"  {stdout.strip()}")

# Step 3: Start new DXFchk
print("Starting DXFchk v0.8.1...")
stdout, _ = exec_cmd(
    'powershell -Command "Start-Process -FilePath C:\\Users\\Public\\dxfchk.exe -ArgumentList \'-port 8643\' -WindowStyle Hidden"',
    timeout=15
)
print(f"  exit={_}")

time.sleep(3)

# Step 4: Verify
print("Verifying...")
r = subprocess.run(["curl", "-s", "--max-time", "15", "http://187.124.31.229:18643/api/v1/health"],
     capture_output=True, text=True, timeout=20)
if r.stdout:
    health = json.loads(r.stdout)
    print(f"  Health: {health}")
else:
    print("  Health check failed — tunnel may need time")
    time.sleep(5)
    r = subprocess.run(["curl", "-s", "--max-time", "15", "http://187.124.31.229:18643/api/v1/health"],
         capture_output=True, text=True, timeout=20)
    if r.stdout:
        health = json.loads(r.stdout)
        print(f"  Health (retry): {health}")
    else:
        print("  Still not responding — checking process")
        stdout, _ = exec_cmd("tasklist /fi \"imagename eq dxfchk.exe\"", timeout=10)
        print(f"  Process: {stdout.strip()}")