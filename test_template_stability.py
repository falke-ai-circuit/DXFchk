#!/usr/bin/env python3
"""
Template Creation Stability Test
For each non-_modN output group:
  1. Create template from EVERY DXF in the group
  2. Compare all created templates — must be identical (same hash)
  3. If the group name matches an original template, compare created vs original — must be 100% identical
"""
import json
import subprocess
import base64
import sys
import os
import hashlib
import tempfile
import time

BASE = "http://187.124.31.229:80"
DXFCHK_URL = "http://187.124.31.229:18643"
AGENT_ID = "vegas-c2022"

_state = {"token": None}

def probe_login():
    r = subprocess.run(
        ["curl", "-s", "-X", "POST", f"{BASE}/api/v1/login",
         "-H", "Content-Type: application/json",
         "-d", json.dumps({"username": "admin", "password": "falke-admin-2026"}),
         "--max-time", "10"],
        capture_output=True, text=True, timeout=15
    )
    resp = json.loads(r.stdout)
    _state["token"] = resp["data"]["token"]
    return _state["token"]

def auth_header():
    return "Authorization", "Bearer " + _state["token"]

def probe_exec(command, timeout=30):
    h_name, h_val = auth_header()
    payload = json.dumps({"command": command, "timeout": timeout})
    r = subprocess.run(
        ["curl", "-s", "-X", "POST", f"{BASE}/api/v1/agents/{AGENT_ID}/exec",
         "-H", "Content-Type: application/json",
         "-H", f"{h_name}: {h_val}",
         "-d", payload,
         "--max-time", str(timeout + 10)],
        capture_output=True, text=True, timeout=timeout + 15
    )
    resp = json.loads(r.stdout)
    if resp.get("ok"):
        return resp["data"].get("stdout", ""), resp["data"].get("exit_code", 0)
    return f"ERROR: {resp.get('error', {}).get('message', 'unknown')}", -1

def probe_fs_list(path):
    h_name, h_val = auth_header()
    r = subprocess.run(
        ["curl", "-s", "-X", "POST", f"{BASE}/api/v1/agents/{AGENT_ID}/fs-list",
         "-H", "Content-Type: application/json",
         "-H", f"{h_name}: {h_val}",
         "-d", json.dumps({"path": path}),
         "--max-time", "15"],
        capture_output=True, text=True, timeout=20
    )
    resp = json.loads(r.stdout)
    return resp["data"].get("entries", []) if resp.get("ok") else []

def probe_fs_read(path, timeout=120):
    h_name, h_val = auth_header()
    r = subprocess.run(
        ["curl", "-s", "-X", "POST", f"{BASE}/api/v1/agents/{AGENT_ID}/fs-read",
         "-H", "Content-Type: application/json",
         "-H", f"{h_name}: {h_val}",
         "-d", json.dumps({"path": path}),
         "--max-time", str(timeout)],
        capture_output=True, text=True, timeout=timeout + 5
    )
    resp = json.loads(r.stdout)
    if resp.get("ok"):
        data = resp["data"].get("data", "")
        return base64.b64decode(data).decode("utf-8", errors="replace")
    return None

def probe_fs_hash(path):
    h_name, h_val = auth_header()
    r = subprocess.run(
        ["curl", "-s", "-X", "POST", f"{BASE}/api/v1/agents/{AGENT_ID}/fs-hash",
         "-H", "Content-Type: application/json",
         "-H", f"{h_name}: {h_val}",
         "-d", json.dumps({"path": path}),
         "--max-time", "30"],
        capture_output=True, text=True, timeout=35
    )
    resp = json.loads(r.stdout)
    if resp.get("ok"):
        return resp["data"].get("hash", "")
    return None

def dxfchk_api(path, method="GET", body=None):
    url = f"{DXFCHK_URL}{path}"
    data = None
    if body:
        data = json.dumps(body).encode()
    headers = {"Content-Type": "application/json"}
    req_cmd = ["curl", "-s", "-X", method, url, "-H", "Content-Type: application/json"]
    if body:
        req_cmd.extend(["-d", json.dumps(body)])
    r = subprocess.run(req_cmd, capture_output=True, text=True, timeout=60)
    try:
        return json.loads(r.stdout)
    except:
        return {"error": r.stdout[:500]}

def sha256_file(filepath):
    h = hashlib.sha256()
    with open(filepath, "rb") as f:
        for chunk in iter(lambda: f.read(8192), b""):
            h.update(chunk)
    return h.hexdigest()

def sha256_bytes(data):
    return hashlib.sha256(data).hexdigest()

def main():
    print("=== Template Creation Stability Test ===\n")
    
    # Login to PROBE
    token = probe_login()
    print(f"PROBE login: {token[:8]}...")
    
    # Get DXFchk settings
    settings = dxfchk_api("/api/v1/settings")
    output_folder = settings.get("output_folder", "")
    template_folder = settings.get("template_folder", "")
    print(f"Output folder: {output_folder}")
    print(f"Template folder: {template_folder}")
    print()
    
    # Step 1: List all output subfolders
    entries = probe_fs_list(output_folder)
    all_folders = [e["name"] for e in entries if e.get("is_dir", False)]
    
    # Separate _modN and non-_modN
    modn_folders = []
    non_modn_folders = []
    for f in all_folders:
        # Check if folder name has _modN suffix
        if "_mod" in f:
            suffix = f.split("_mod")[-1]
            if suffix.isdigit():
                modn_folders.append(f)
                continue
        non_modn_folders.append(f)
    
    print(f"Total output folders: {len(all_folders)}")
    print(f"Non-_modN groups: {len(non_modn_folders)}")
    print(f"_modN folders: {len(modn_folders)}")
    print()
    
    # Step 2: List original templates available
    template_entries = probe_fs_list(template_folder)
    original_templates = {e["name"] for e in template_entries if e.get("name", "").endswith(".dxf")}
    print(f"Original templates: {len(original_templates)}")
    print()
    
    # Step 3: Create a test folder on Vegas for template outputs
    test_dir = "C:\\Users\\Administrator\\Desktop\\template_stability_test"
    probe_exec(f'mkdir {test_dir}', timeout=10)
    
    results = {}  # group_name -> {file: hash, created: bool, matches_original: bool}
    anomalies = []
    
    for group in sorted(non_modn_folders):
        print(f"\n--- Group: {group} ---")
        group_path = f"{output_folder}\\{group}"
        
        # List DXF files in this group
        file_entries = probe_fs_list(group_path)
        dxf_files = [e["name"] for e in file_entries if e.get("name", "").endswith(".dxf")]
        print(f"  DXF files: {len(dxf_files)}")
        
        if len(dxf_files) == 0:
            print(f"  SKIP: no DXF files")
            continue
        
        # Check if this group has a matching original template
        original_template_name = f"{group}.dxf"
        has_original = original_template_name in original_templates
        print(f"  Original template {original_template_name}: {'EXISTS' if has_original else 'NOT FOUND'}")
        
        # For each DXF file, create a template via the API
        created_hashes = {}
        for dxf_file in dxf_files:
            source_path = f"{group_path}\\{dxf_file}"
            print(f"  Creating template from: {dxf_file}...")
            
            # Call DXFchk template create API
            result = dxfchk_api("/api/v1/template/create", method="POST", body={
                "source_file": source_path,
                "template_name": f"{group}_test_{dxf_file.replace('.dxf', '')}",
                "template_folder": test_dir,
                "save_to_mod_folder": False
            })
            
            if result.get("ok"):
                template_path = result.get("template_path", "")
                # Get hash of created template
                h = probe_fs_hash(template_path)
                if h:
                    created_hashes[dxf_file] = h
                    print(f"    → SHA256: {h[:16]}...")
                else:
                    print(f"    → HASH FAILED")
                    anomalies.append(f"{group}/{dxf_file}: hash failed")
            else:
                err = result.get("error", str(result)[:200])
                print(f"    → ERROR: {err}")
                anomalies.append(f"{group}/{dxf_file}: create error: {err}")
        
        # Step 4: Compare all created hashes within this group
        if len(created_hashes) > 1:
            unique_hashes = set(created_hashes.values())
            if len(unique_hashes) == 1:
                print(f"  ✅ ALL {len(created_hashes)} templates identical (same hash)")
            else:
                print(f"  ❌ ANOMALY: {len(unique_hashes)} different hashes from {len(created_hashes)} files")
                for f, h in created_hashes.items():
                    print(f"    {f}: {h[:16]}...")
                anomalies.append(f"{group}: {len(unique_hashes)} different hashes")
        elif len(created_hashes) == 1:
            print(f"  ✅ Only 1 file (trivially identical)")
        
        # Step 5: If original template exists, compare created vs original
        if has_original and created_hashes:
            original_path = f"{template_folder}\\{original_template_name}"
            original_hash = probe_fs_hash(original_path)
            if original_hash:
                print(f"  Original template hash: {original_hash[:16]}...")
                for f, h in created_hashes.items():
                    if h == original_hash:
                        print(f"  ✅ {f}: matches original template!")
                    else:
                        print(f"  ❌ {f}: DIFFERS from original template!")
                        # Download both and compare
                        print(f"     Created hash: {h}")
                        print(f"     Original hash: {original_hash}")
                        anomalies.append(f"{group}/{f}: created template differs from original")
        
        results[group] = {
            "files": dxf_files,
            "created_hashes": created_hashes,
            "has_original": has_original,
        }
        
        # Don't flood — small delay between groups
        time.sleep(0.5)
    
    # Final report
    print("\n\n=== STABILITY TEST RESULTS ===\n")
    print(f"Groups tested: {len(results)}")
    print(f"Total templates created: {sum(len(r['created_hashes']) for r in results.values())}")
    print(f"Anomalies: {len(anomalies)}")
    
    if anomalies:
        print("\n⚠️  ANOMALIES FOUND:")
        for a in anomalies:
            print(f"  - {a}")
    else:
        print("\n✅ ALL TEMPLATES STABLE — no anomalies detected")
    
    # Clean up test folder
    probe_exec(f'cmd /c rmdir /s /q "{test_dir}"', timeout=10)
    print(f"\nTest folder cleaned up: {test_dir}")

if __name__ == "__main__":
    main()