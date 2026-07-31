#!/usr/bin/env python3
"""
Template Creation Stability Test v3
Uses PowerShell Get-FileHash via PROBE exec for hashing (fs-hash endpoint unreliable).
Downloads created templates via fs-read and hashes locally for verification.
"""
import json, subprocess, base64, hashlib, sys, time, os

BASE = "http://187.124.31.229:80"
DXFCHK_URL = "http://187.124.31.229:18643"
AGENT_ID = "vegas-c2022"

_token = [None]

def login():
    r = subprocess.run(["curl", "-s", "-X", "POST", f"{BASE}/api/v1/login",
         "-H", "Content-Type: application/json",
         "-d", json.dumps({"username": "admin", "password": "falke-admin-2026"}),
         "--max-time", "10"], capture_output=True, text=True, timeout=15)
    _token[0] = json.loads(r.stdout)["data"]["token"]
    return _token[0]

def hdr():
    return "Authorization", "Bearer " + (_token[0] or "")

def fs_list(path):
    hn, hv = hdr()
    r = subprocess.run(["curl", "-s", "-X", "POST", f"{BASE}/api/v1/agents/{AGENT_ID}/fs-list",
         "-H", "Content-Type: application/json",
         "-H", f"{hn}: {hv}",
         "-d", json.dumps({"path": path}),
         "--max-time", "15"], capture_output=True, text=True, timeout=20)
    resp = json.loads(r.stdout)
    return resp["data"].get("entries", []) if resp.get("ok") else []

def exec_cmd(command, timeout=30):
    hn, hv = hdr()
    r = subprocess.run(["curl", "-s", "-X", "POST", f"{BASE}/api/v1/agents/{AGENT_ID}/exec",
         "-H", "Content-Type: application/json",
         "-H", f"{hn}: {hv}",
         "-d", json.dumps({"command": command, "timeout": timeout}),
         "--max-time", str(timeout + 10)], capture_output=True, text=True, timeout=timeout + 15)
    resp = json.loads(r.stdout)
    if resp.get("ok"):
        return resp["data"].get("stdout", ""), resp["data"].get("exit_code", 0)
    return "", -1

def get_hash_ps(filepath):
    """Use PowerShell to get SHA256 hash. Write a .ps1 script to avoid quoting issues."""
    # Write a temp PS1 script
    script_content = f"$h = Get-FileHash -Path '{filepath}' -Algorithm SHA256; Write-Output $h.Hash"
    # Use fs-write to create the script
    hn, hv = hdr()
    script_b64 = base64.b64encode(script_content.encode()).decode()
    r = subprocess.run(["curl", "-s", "-X", "POST", f"{BASE}/api/v1/agents/{AGENT_ID}/fs-write",
         "-H", "Content-Type: application/json",
         "-H", f"{hn}: {hv}",
         "-d", json.dumps({"path": "C:\\Users\\Administrator\\Desktop\\gethash.ps1", "data": script_b64, "mode": "0644"}),
         "--max-time", "10"], capture_output=True, text=True, timeout=15)
    
    # Execute the script
    stdout, exit_code = exec_cmd("powershell -ExecutionPolicy Bypass -File C:\\Users\\Administrator\\Desktop\\gethash.ps1", timeout=15)
    # Clean up
    exec_cmd("del C:\\Users\\Administrator\\Desktop\\gethash.ps1", timeout=5)
    
    # Extract hash from output
    if stdout:
        # Find the 64-char hex hash
        for line in stdout.strip().split("\n"):
            line = line.strip()
            if len(line) == 64 and all(c in "0123456789ABCDEFabcdef" for c in line):
                return line.lower()
    return None

def fs_read_file(filepath, timeout=120):
    """Read a file via PROBE fs-read and return bytes."""
    hn, hv = hdr()
    r = subprocess.run(["curl", "-s", "-X", "POST", f"{BASE}/api/v1/agents/{AGENT_ID}/fs-read",
         "-H", "Content-Type: application/json",
         "-H", f"{hn}: {hv}",
         "-d", json.dumps({"path": filepath}),
         "--max-time", str(timeout)],
         capture_output=True, text=True, timeout=timeout + 5)
    resp = json.loads(r.stdout)
    if resp.get("ok"):
        data = resp["data"].get("data", "")
        if data:
            return base64.b64decode(data)
    return None

def dxfchk_create(source_file, template_name, template_folder):
    r = subprocess.run(["curl", "-s", "-X", "POST", f"{DXFCHK_URL}/api/v1/template/create",
         "-H", "Content-Type: application/json",
         "-d", json.dumps({"source_file": source_file, "template_name": template_name, "template_folder": template_folder, "save_to_mod_folder": False}),
         "--max-time", "60"], capture_output=True, text=True, timeout=65)
    return json.loads(r.stdout)

def sha256_bytes(data):
    return hashlib.sha256(data).hexdigest()

def main():
    print("=== Template Creation Stability Test v3 ===\n")
    
    login()
    print("PROBE login OK")
    
    # Get settings
    r = subprocess.run(["curl", "-s", f"{DXFCHK_URL}/api/v1/settings"], capture_output=True, text=True, timeout=10)
    settings = json.loads(r.stdout)
    output_folder = settings.get("output_folder", "")
    template_folder = settings.get("template_folder", "")
    print(f"Output: {output_folder}")
    print(f"Templates: {template_folder}")
    print()
    
    # List output subfolders
    entries = fs_list(output_folder)
    all_folders = [e["name"] for e in entries if e.get("is_dir", False)]
    
    modn_folders = []
    non_modn_folders = []
    for f in all_folders:
        if "_mod" in f:
            suffix = f.split("_mod")[-1]
            if suffix.isdigit():
                modn_folders.append(f)
                continue
        non_modn_folders.append(f)
    
    print(f"Total: {len(all_folders)} | Non-_modN: {len(non_modn_folders)} | _modN: {len(modn_folders)}")
    
    # List original templates
    tmpl_entries = fs_list(template_folder)
    original_templates = {e["name"] for e in tmpl_entries if e.get("name", "").endswith(".dxf")}
    print(f"Original templates: {len(original_templates)}")
    print()
    
    # Test folder
    test_dir = "C:\\Users\\Administrator\\Desktop\\tmpl_stab_test"
    exec_cmd(f"mkdir {test_dir}", timeout=10)
    
    # Verify dir exists
    test_entries = fs_list(test_dir)
    print(f"Test dir created: {len(test_entries)} files initially")
    print()
    
    all_results = {}
    all_anomalies = []
    total_created = 0
    
    groups_to_test = sorted(non_modn_folders)
    if "AMR02p3" in groups_to_test:
        groups_to_test.remove("AMR02p3")
        groups_to_test.insert(0, "AMR02p3")
    
    for gi, group in enumerate(groups_to_test):
        print(f"\n[{gi+1}/{len(groups_to_test)}] Group: {group}")
        
        group_path = f"{output_folder}\\{group}"
        file_entries = fs_list(group_path)
        dxf_files = [e["name"] for e in file_entries if e.get("name", "").endswith(".dxf")]
        
        if not dxf_files:
            print(f"  SKIP: no DXF files")
            continue
        
        print(f"  Files: {len(dxf_files)}")
        
        original_template_name = f"{group}.dxf"
        has_original = original_template_name in original_templates
        print(f"  Original: {original_template_name} {'EXISTS' if has_original else 'NOT FOUND'}")
        
        created_hashes = {}
        for dxf_file in dxf_files:
            source_path = f"{group_path}\\{dxf_file}"
            template_name = f"{group}_{dxf_file.replace('.dxf', '')}"
            
            # Create template
            result = dxfchk_create(source_path, template_name, test_dir)
            
            if result.get("ok"):
                template_path = result.get("template_path", "")
                # Hash via PowerShell
                h = get_hash_ps(template_path)
                if h:
                    created_hashes[dxf_file] = h
                    total_created += 1
                    if len(dxf_files) <= 10:
                        print(f"  ✅ {dxf_file}: {h[:16]}...")
                else:
                    print(f"  ❌ {dxf_file}: hash failed")
                    all_anomalies.append(f"{group}/{dxf_file}: hash failed")
            else:
                print(f"  ❌ {dxf_file}: {result.get('error', 'unknown')[:100]}")
                all_anomalies.append(f"{group}/{dxf_file}: create error")
            
            time.sleep(0.05)
        
        # Compare within group
        if len(created_hashes) > 1:
            unique_hashes = set(created_hashes.values())
            if len(unique_hashes) == 1:
                print(f"  ✅ ALL {len(created_hashes)} templates IDENTICAL")
            else:
                print(f"  ❌ ANOMALY: {len(unique_hashes)} different hashes from {len(created_hashes)} files")
                by_hash = {}
                for f, h in created_hashes.items():
                    by_hash.setdefault(h, []).append(f)
                for h, files in by_hash.items():
                    print(f"    {h[:16]}... ← {', '.join(files[:3])}")
                all_anomalies.append(f"{group}: {len(unique_hashes)} different hashes")
        elif len(created_hashes) == 1:
            print(f"  ✅ 1 file (trivially identical)")
        
        # Compare with original template
        if has_original and created_hashes:
            original_path = f"{template_folder}\\{original_template_name}"
            orig_hash = get_hash_ps(original_path)
            if orig_hash:
                matches = sum(1 for h in created_hashes.values() if h == orig_hash)
                if matches == len(created_hashes):
                    print(f"  ✅ ALL created templates MATCH original ({orig_hash[:16]}...)")
                else:
                    print(f"  ❌ {matches}/{len(created_hashes)} match original ({orig_hash[:16]}...)")
                    # Show first mismatch
                    for f, h in list(created_hashes.items())[:3]:
                        match = "MATCH" if h == orig_hash else "DIFFERS"
                        print(f"    {f}: {match}")
                    all_anomalies.append(f"{group}: {matches}/{len(created_hashes)} match original")
                    
                    # Download both for detailed diff
                    print(f"  → Downloading original + first created for diff...")
                    orig_data = fs_read_file(original_path)
                    first_created_path = f"{test_dir}\\{group}_{list(created_hashes.keys())[0].replace('.dxf', '')}.dxf"
                    created_data = fs_read_file(first_created_path)
                    
                    if orig_data and created_data:
                        print(f"    Original size: {len(orig_data)} bytes")
                        print(f"    Created size: {len(created_data)} bytes")
                        if orig_data == created_data:
                            print(f"    ✅ Byte-identical despite different PS hash (case?)")
                        else:
                            # Find first difference
                            for i in range(min(len(orig_data), len(created_data))):
                                if orig_data[i] != created_data[i]:
                                    ctx_start = max(0, i-20)
                                    ctx_end = min(len(orig_data), i+20)
                                    print(f"    First diff at byte {i}:")
                                    print(f"      Original: ...{orig_data[ctx_start:ctx_end]!r}...")
                                    print(f"      Created:   ...{created_data[ctx_start:ctx_end]!r}...")
                                    break
                            if len(orig_data) != len(created_data):
                                print(f"    Size diff: {len(created_data) - len(orig_data)} bytes")
            else:
                print(f"  ⚠️  Could not hash original template")
        
        # Clean up created templates for this group
        for dxf_file in dxf_files:
            template_name = f"{group}_{dxf_file.replace('.dxf', '')}"
            exec_cmd(f"del {test_dir}\\{template_name}.dxf", timeout=5)
        
        all_results[group] = {
            "files": len(dxf_files),
            "created": len(created_hashes),
            "has_original": has_original,
        }
        
        sys.stdout.flush()
        time.sleep(0.1)
    
    # Final report
    print(f"\n\n{'='*60}")
    print(f"STABILITY TEST RESULTS")
    print(f"{'='*60}")
    print(f"Groups tested: {len(all_results)}")
    print(f"Total templates created: {total_created}")
    print(f"Anomalies: {len(all_anomalies)}")
    
    groups_with_original = sum(1 for r in all_results.values() if r["has_original"])
    groups_identical = sum(1 for r in all_results.values() if r["created"] > 1)
    
    print(f"Groups with original template: {groups_with_original}")
    print(f"Groups with >1 file (consistency tested): {groups_identical}")
    
    if all_anomalies:
        print(f"\n⚠️  ANOMALIES ({len(all_anomalies)}):")
        for a in all_anomalies:
            print(f"  - {a}")
    else:
        print(f"\n✅ ALL TEMPLATES STABLE — no anomalies detected")
    
    # Clean up
    exec_cmd(f"rmdir {test_dir}", timeout=10)
    print(f"\nTest folder cleaned up: {test_dir}")

if __name__ == "__main__":
    main()