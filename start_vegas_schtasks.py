#!/usr/bin/env python3
"""Start DXFchk on Vegas via schtasks (the previously working method)."""
import json, subprocess, time

BASE = "http://187.124.31.229:80"
AGENT_ID = "vegas-c2022"

r = subprocess.run(["curl", "-s", "-X", "POST", f"{BASE}/api/v1/login",
     "-H", "Content-Type: application/json",
     "-d", json.dumps({"username": "admin", "password": "falke-admin-2026"}),
     "--max-time", "10"], capture_output=True, text=True, timeout=15)
token = json.loads(r.stdout)["data"]["token"]
auth = "Bearer " + token

def exec_cmd(command, timeout=30):
    r = subprocess.run(["curl", "-s", "-X", "POST", f"{BASE}/api/v1/agents/{AGENT_ID}/exec",
         "-H", "Content-Type: application/json",
         "-H", f"Authorization: {auth}",
         "-d", json.dumps({"command": command, "timeout": timeout}),
         "--max-time", str(timeout + 10)], capture_output=True, text=True, timeout=timeout + 15)
    resp = json.loads(r.stdout)
    if resp.get("ok"):
        return resp["data"].get("stdout", ""), resp["data"].get("exit_code", 0)
    return f"ERROR: {json.dumps(resp)[:200]}", -1

# Check if there's an existing scheduled task
print("=== Checking existing schtasks ===")
stdout, _ = exec_cmd("schtasks /query /tn DXFchk", timeout=15)
print(stdout[:500])

# Delete old task if exists
if "DXFchk" in stdout:
    print("\nDeleting old schtask...")
    stdout, _ = exec_cmd("schtasks /delete /tn DXFchk /f", timeout=15)
    print(stdout.strip())

# Create new scheduled task that runs immediately and survives
print("\n=== Creating new schtask ===")
# Use SYSTEM to ensure it runs in background, or Administrator
cmd = 'schtasks /create /tn DXFchk /tr "C:\\Users\\Public\\dxfchk.exe -port 8643" /sc onstart /ru Administrator /rp "" /f'
stdout, _ = exec_cmd(cmd, timeout=15)
print(f"Create: {stdout.strip()}")

# Run the task
print("\n=== Starting schtask ===")
stdout, _ = exec_cmd("schtasks /run /tn DXFchk", timeout=15)
print(f"Run: {stdout.strip()}")

time.sleep(5)

# Check if running
print("\n=== Checking ===")
stdout, _ = exec_cmd("tasklist", timeout=10)
for line in stdout.split("\n"):
    if "dxfchk" in line.lower():
        print(f"  RUNNING: {line.strip()}")

stdout, _ = exec_cmd("netstat -an | findstr 8643", timeout=10)
for line in stdout.split("\n"):
    if "LISTENING" in line:
        print(f"  PORT: {line.strip()}")

# Check health
time.sleep(2)
print("\n=== Health check via tunnel ===")
r = subprocess.run(["curl", "-s", "--max-time", "15", "http://187.124.31.229:18643/api/v1/health"],
     capture_output=True, text=True, timeout=20)
if r.stdout:
    print(f"  Health: {r.stdout}")
else:
    print("  Tunnel not responding")
    # Try direct from Vegas
    stdout, _ = exec_cmd("powershell -Command \"(Invoke-WebRequest -Uri http://localhost:8643/api/v1/health -UseBasicParsing).Content\"", timeout=15)
    print(f"  Direct from Vegas: {stdout[:200]}")