#!/usr/bin/env python3
"""Start DXFchk on Vegas — create wrapper batch + schtask."""
import json, subprocess, base64, time

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

def fs_write(path, content_b64, mode="0644"):
    r = subprocess.run(["curl", "-s", "-X", "POST", f"{BASE}/api/v1/agents/{AGENT_ID}/fs-write",
         "-H", "Content-Type: application/json",
         "-H", f"Authorization: {auth}",
         "-d", json.dumps({"path": path, "data": content_b64, "mode": mode}),
         "--max-time", "15"], capture_output=True, text=True, timeout=20)
    return json.loads(r.stdout).get("ok", False)

# Step 1: Write a wrapper batch file
print("=== Writing wrapper batch ===")
batch_content = "@echo off\r\nC:\\Users\\Public\\dxfchk.exe -port 8643\r\n"
batch_b64 = base64.b64encode(batch_content.encode()).decode()
ok = fs_write("C:\\Users\\Public\\start_dxfchk.bat", batch_b64)
print(f"  Write batch: {ok}")

# Step 2: Create schtask using the batch file
print("\n=== Creating schtask ===")
stdout, _ = exec_cmd('schtasks /create /tn DXFchk /tr "C:\\Users\\Public\\start_dxfchk.bat" /sc onstart /ru Administrator /f', timeout=15)
print(f"  Create: {stdout.strip()}")

# Step 3: Run the task
print("\n=== Starting schtask ===")
stdout, _ = exec_cmd("schtasks /run /tn DXFchk", timeout=15)
print(f"  Run: {stdout.strip()}")

time.sleep(5)

# Step 4: Check
print("\n=== Checking ===")
stdout, _ = exec_cmd("tasklist", timeout=10)
found = False
for line in stdout.split("\n"):
    if "dxfchk" in line.lower():
        print(f"  RUNNING: {line.strip()}")
        found = True

if not found:
    print("  NOT RUNNING")
    # Try direct start
    print("  Trying direct start...")
    stdout, _ = exec_cmd("C:\\Users\\Public\\start_dxfchk.bat", timeout=5)
    time.sleep(3)
    stdout, _ = exec_cmd("tasklist", timeout=10)
    for line in stdout.split("\n"):
        if "dxfchk" in line.lower():
            print(f"  RUNNING: {line.strip()}")

stdout, _ = exec_cmd("netstat -an | findstr 8643", timeout=10)
for line in stdout.split("\n"):
    if "LISTENING" in line:
        print(f"  PORT LISTENING: {line.strip()}")

# Step 5: Health check
time.sleep(2)
print("\n=== Health check ===")
# Direct from Vegas
stdout, _ = exec_cmd('powershell -Command "try { (Invoke-WebRequest -Uri http://localhost:8643/api/v1/health -UseBasicParsing -TimeoutSec 5).Content } catch { $_.Exception.Message }"', timeout=15)
print(f"  Direct: {stdout.strip()[:200]}")

# Via tunnel
r = subprocess.run(["curl", "-s", "--max-time", "15", "http://187.124.31.229:18643/api/v1/health"],
     capture_output=True, text=True, timeout=20)
if r.stdout:
    print(f"  Tunnel: {r.stdout}")
else:
    print("  Tunnel: no response")