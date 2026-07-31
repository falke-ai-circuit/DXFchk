#!/usr/bin/env python3
"""
Template Creation Stability Test v2
- Uses fs-list to get file sizes, fs-read to download and hash locally
- For each non-_modN output group:
  1. Create template from EVERY DXF in the group
  2. Compare all created templates — must be identical (same hash)
  3. If the group name matches an original template, compare created vs original — must be 100% identical
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
    return "Authorization", "Bearer " + _token[0]

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

def fs_hash(path, timeout=60):
    """Get SHA256 hash of file on remote agent using fs-hash endpoint."""
    hn, hv = hdr()
    r = subprocess.run(["curl", "-s", "-X", "POST", f"{BASE}/api/v1/agents/{AGENT_ID}/fs-hash",
         "-H", "Content-Type: application/json",
         "-H", f"{hn}: {hv}",
         "-d", json.dumps({"path": path}),
         "--max-time", str(timeout)],
         capture_output=True, text=True, timeout=timeout + 5)
    resp = json.loads(r.stdout)
    if resp.get("ok"):
        return resp["data"].get("hash", "")
    print(f"    fs-hash error: {json.dumps(resp)[:200]}")
    return None

def dxfchk_create(source_file, template_name, template_folder):
    r = subprocess.run(["curl", "-s", "-X", "POST", f"{DXFCHK_URL}/api/v1/template/create",
         "-H", "Content-Type: application/json",
         "-d", json.dumps({"source_file": source_file, "template_name": template_name, "template_folder": template_folder, "save_to_mod_folder": False}),
         "--max-time", "60"], capture_output=True, text=True, timeout=65)
    return json.loads(r.stdout)

def main():
    print("=== Template Creation Stability Test v2 ===\n")
    
    login()
    print(f"PROBE login OK")
    
    # Get DXFchk settings
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
    
    # Separate _modN and non-_modN
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
    test_dir = "C:\\Users\\Administrator\\Desktop\\template_stability_test"
    exec_cmd(f"mkdir {test_dir}", timeout=10)
    
    # START with AMR02p3 group (user specifically asked for this)
    # Then proceed to all other non-_modN groups
    
    all_results = {}
    all_anomalies = []
    
    groups_to_test = sorted(non_modn_folders)
    # Make sure AMR02p3 is first
    if "AMR02p3" in groups_to_test:
        groups_to_test.remove("AMR02p3")
        groups_to_test.insert(0, "AMR02p3")
    
    total_templates_created = 0
    
    for group in groups_to_test:
        print(f"\n{'='*60}")
        print(f"Group: {group}")
        print(f"{'='*60}")
        
        group_path = f"{output_folder}\\{group}"
        file_entries = fs_list(group_path)
        dxf_files = [e["name"] for e in file_entries if e.get("name", "").endswith(".dxf")]
        
        if not dxf_files:
            print(f"  SKIP: no DXF files")
            continue
        
        print(f"  DXF files: {len(dxf_files)}")
        
        # Check for matching original template
        original_template_name = f"{group}.dxf"
        has_original = original_template_name in original_templates
        print(f"  Original template: {original_template_name} {'EXISTS' if has_original else 'NOT FOUND'}")
        
        # Create template from each DXF
        created_hashes = {}
        for dxf_file in dxf_files:
            source_path = f"{group_path}\\{dxf_file}"
            template_name = f"{group}_test_{dxf_file.replace('.dxf', '')}"
            
            result = dxfchk_create(source_path, template_name, test_dir)
            
            if result.get("ok"):
                template_path = result.get("template_path", "")
                h = fs_hash(template_path)
                if h:
                    created_hashes[dxf_file] = h
                    total_templates_created += 1
                    if len(dxf_files) <= 5:
                        print(f"  ✅ {dxf_file} → {h[:16]}...")
                    elif len(created_hashes) == 1 or len(created_hashes) == len(dxf_files):
                        print(f"  ✅ {dxf_file} → {h[:16]}... ({len(created_hashes)}/{len(dxf_files)})")
                else:
                    print(f"  ❌ {dxf_file} → hash failed")
                    all_anomalies.append(f"{group}/{dxf_file}: hash failed")
            else:
                err = result.get("error", str(result)[:200])
                print(f"  ❌ {dxf_file} → {err}")
                all_anomalies.append(f"{group}/{dxf_file}: create error")
            
            time.sleep(0.1)  # Small delay to not overload
        
        # Compare all hashes within group
        if len(created_hashes) > 1:
            unique_hashes = set(created_hashes.values())
            if len(unique_hashes) == 1:
                print(f"  ✅ ALL {len(created_hashes)} templates IDENTICAL")
            else:
                print(f"  ❌ ANOMALY: {len(unique_hashes)} different hashes from {len(created_hashes)} files")
                # Show which differ
                by_hash = {}
                for f, h in created_hashes.items():
                    by_hash.setdefault(h, []).append(f)
                for h, files in by_hash.items():
                    if len(files) == 1:
                        print(f"    UNIQUE hash {h[:16]}... ← {files[0]}")
                    else:
                        print(f"    hash {h[:16]}... ← {len(files)} files")
                all_anomalies.append(f"{group}: {len(unique_hashes)} different hashes")
        elif len(created_hashes) == 1:
            print(f"  ✅ 1 file (trivially identical)")
        
        # Compare with original template
        if has_original and created_hashes:
            original_path = f"{template_folder}\\{original_template_name}"
            orig_hash = fs_hash(original_path)
            if orig_hash:
                print(f"  Original hash: {orig_hash[:16]}...")
                any_match = False
                for f, h in created_hashes.items():
                    if h == orig_hash:
                        print(f"  ✅ {f}: MATCHES original template!")
                        any_match = True
                    else:
                        print(f"  ❌ {f}: DIFFERS from original (created={h[:16]}... vs original={orig_hash[:16]}...)")
                        all_anomalies.append(f"{group}/{f}: created differs from original")
                if not any_match:
                    # Download both for detailed comparison
                    print(f"  ⚠️  No created template matches original — will download for diff")
        elif has_original and not created_hashes:
            print(f"  ⚠️  Original exists but no templates created to compare")
        
        all_results[group] = {
            "files": len(dxf_files),
            "created": len(created_hashes),
            "has_original": has_original,
            "all_identical": len(set(created_hashes.values())) == 1 if created_hashes else True,
        }
        
        # Clean up test files for this group to save disk space
        for dxf_file in dxf_files:
            template_name = f"{group}_test_{dxf_file.replace('.dxf', '')}"
            template_path = f"{test_dir}\\{template_name}.dxf"
            exec_cmd(f"del {template_path}", timeout=5)
        
        time.sleep(0.2)
    
    # Final report
    print(f"\n\n{'='*60}")
    print(f"STABILITY TEST RESULTS")
    print(f"{'='*60}")
    print(f"Groups tested: {len(all_results)}")
    print(f"Total templates created: {total_templates_created}")
    print(f"Anomalies: {len(all_anomalies)}")
    
    if all_anomalies:
        print(f"\n⚠️  ANOMALIES:")
        for a in all_anomalies:
            print(f"  - {a}")
    else:
        print(f"\n✅ ALL TEMPLATES STABLE — no anomalies detected")
    
    # Clean up
    exec_cmd(f"rmdir {test_dir}", timeout=10)
    print(f"\nTest folder cleaned up")

if __name__ == "__main__":
    main()