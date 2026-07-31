#!/usr/bin/env python3
"""Check and fix DXFchk on Vegas."""
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

# Check if dxfchk is running
stdout, _ = exec_cmd("tasklist", timeout=10)
print("Processes with 'dxfchk':")
for line in stdout.split("\n"):
    if "dxfchk" in line.lower():
        print(f"  {line.strip()}")

# Check port
stdout, _ = exec_cmd("netstat -an | findstr 8643", timeout=10)
print(f"\nPort 8643:\n  {stdout.strip()}")

# Check if dxfchk.exe exists
stdout, _ = exec_cmd("dir C:\\Users\\Public\\dxfchk.exe", timeout=10)
print(f"\nBinary:\n  {stdout.strip()}")

# If not running, start it
stdout, _ = exec_cmd("tasklist", timeout=10)
if "dxfchk" not in stdout.lower():
    print("\nDXFchk NOT running — starting...")
    # Use schtasks approach (was working before)
    stdout, _ = exec_cmd(
        'powershell -Command "Start-Process -FilePath C:\\Users\\Public\\dxfchk.exe -ArgumentList (\'-port\', \'8643\') -WorkingDirectory C:\\Users\\Public -WindowStyle Hidden"',
        timeout=15
    )
    print(f"  Start exit code: {_}")
    time.sleep(3)
    
    # Check again
    stdout, _ = exec_cmd("tasklist", timeout=10)
    for line in stdout.split("\n"):
        if "dxfchk" in line.lower():
            print(f"  NOW RUNNING: {line.strip()}")
    
    stdout, _ = exec_cmd("netstat -an | findstr 8643", timeout=10)
    print(f"  Port 8643: {stdout.strip()}")
else:
    print("\nDXFchk IS running")

# Final: check health via tunnel
print("\nChecking tunnel...")
time.sleep(2)
r = subprocess.run(["curl", "-s", "--max-time", "15", "http://187.124.31.229:18643/api/v1/health"],
     capture_output=True, text=True, timeout=20)
if r.stdout:
    print(f"  Health: {r.stdout}")
else:
    print("  Tunnel not responding — checking tunnel config on PROBE")
    # Check tunnel status on PROBE server
    r2 = subprocess.run(["curl", "-s", "--max-time", "10", f"{BASE}/api/v1/tunnels",
         "-H", f"Authorization: {auth}"],
         capture_output=True, text=True, timeout=15)
    if r2.stdout:
        print(f"  Tunnels: {r2.stdout[:500]}")