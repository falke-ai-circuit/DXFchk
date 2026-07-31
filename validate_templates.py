#!/usr/bin/env python3
"""
Validate enhanced template creation against original templates.
For each group with an original template:
1. Create a template from the first module using the NEW normalization rules
2. Compare the created template vs the original template (line-by-line diff)
3. Report: exact match / close match / mismatch
4. If mismatch, show exactly which lines differ
"""
import json, subprocess, base64, sys, time, os
from collections import defaultdict

BASE = "http://187.124.31.229:80"
DXFCHK_URL = "http://187.124.31.229:18643"
AGENT_ID = "vegas-c2022"
OUTFILE = "/opt/data/dxfchk/validation_results.json"

_token = [None]

def login():
    r = subprocess.run(["curl", "-s", "-X", "POST", f"{BASE}/api/v1/login",
         "-H", "Content-Type: application/json",
         "-d", json.dumps({"username": "admin", "password": "falke-admin-2026"}),
         "--max-time", "15"], capture_output=True, text=True, timeout=20)
    _token[0] = json.loads(r.stdout)["data"]["token"]

def hdr():
    return "Authorization", "Bearer " + (_token[0] or "")

def fs_list(path):
    hn, hv = hdr()
    for attempt in range(3):
        try:
            r = subprocess.run(["curl", "-s", "-X", "POST", f"{BASE}/api/v1/agents/{AGENT_ID}/fs-list",
                 "-H", "Content-Type: application/json", "-H", f"{hn}: {hv}",
                 "-d", json.dumps({"path": path}),
                 "--max-time", "15"], capture_output=True, text=True, timeout=20)
            resp = json.loads(r.stdout)
            if resp.get("ok"):
                return resp["data"].get("entries", [])
            return []
        except:
            time.sleep(2)
    return []

def exec_cmd(command, timeout=120):
    hn, hv = hdr()
    for attempt in range(3):
        try:
            r = subprocess.run(["curl", "-s", "-X", "POST", f"{BASE}/api/v1/agents/{AGENT_ID}/exec",
                 "-H", "Content-Type: application/json", "-H", f"{hn}: {hv}",
                 "-d", json.dumps({"command": command, "timeout": timeout}),
                 "--max-time", str(timeout + 10)], capture_output=True, text=True, timeout=timeout + 15)
            resp = json.loads(r.stdout)
            if resp.get("ok"):
                return resp["data"].get("stdout", ""), resp["data"].get("exit_code", 0)
            return "", -1
        except:
            time.sleep(2)
    return "", -1

def fs_write(path, content_b64, mode="0644"):
    hn, hv = hdr()
    r = subprocess.run(["curl", "-s", "-X", "POST", f"{BASE}/api/v1/agents/{AGENT_ID}/fs-write",
         "-H", "Content-Type: application/json", "-H", f"{hn}: {hv}",
         "-d", json.dumps({"path": path, "data": content_b64, "mode": mode}),
         "--max-time", "15"], capture_output=True, text=True, timeout=20)
    return json.loads(r.stdout).get("ok", False)

def dxfchk_create(source_file, template_name, template_folder):
    for attempt in range(3):
        try:
            r = subprocess.run(["curl", "-s", "-X", "POST", f"{DXFCHK_URL}/api/v1/template/create",
                 "-H", "Content-Type: application/json",
                 "-d", json.dumps({"source_file": source_file, "template_name": template_name,
                                   "template_folder": template_folder, "save_to_mod_folder": False}),
                 "--max-time", "120"], capture_output=True, text=True, timeout=125)
            return json.loads(r.stdout)
        except:
            print(f"  [retry {attempt+1}/3] create timeout...")
            time.sleep(3)
    return {"ok": False, "error": "timeout"}

def compare_files_remote(template_path, module_path):
    script = f'''$orig = Get-Content -Path "{template_path}" -Encoding Default
$mod = Get-Content -Path "{module_path}" -Encoding Default
$max = [Math]::Max($orig.Count, $mod.Count)
$diffs = @()
for ($i = 0; $i -lt $max; $i++) {{
    $o = if ($i -lt $orig.Count) {{ $orig[$i].Trim() }} else {{ "<MISSING>" }}
    $m = if ($i -lt $mod.Count) {{ $mod[$i].Trim() }} else {{ "<MISSING>" }}
    if ($o -ne $m) {{
        $code = "?"
        for ($j = $i - 1; $j -ge [Math]::Max(0, $i - 5); $j--) {{
            if ($j -lt $orig.Count) {{
                $c = $orig[$j].Trim()
                $n = 0
                if ([int]::TryParse($c, [ref]$n) -and $c.Length -lt 10) {{
                    $code = $c
                    break
                }}
            }}
        }}
        $diffs += [PSCustomObject]@{{line=$i+1; code=$code; orig=$o; mod=$m}}
    }}
}}
$diffs | ConvertTo-Json -Compress -Depth 3'''
    script_path = "C:\\Users\\Administrator\\Desktop\\cmp_val.ps1"
    fs_write(script_path, base64.b64encode(("\ufeff" + script).encode("utf-8")).decode())
    stdout, _ = exec_cmd(f"powershell -ExecutionPolicy Bypass -File {script_path}", timeout=90)
    exec_cmd(f"del {script_path}", timeout=5)
    if not stdout:
        return []
    try:
        result = json.loads(stdout)
        return result if isinstance(result, list) else [result]
    except:
        return []

def main():
    login()
    print("PROBE login OK\n")
    
    # Get settings
    for attempt in range(5):
        try:
            r = subprocess.run(f"curl -s --max-time 30 {DXFCHK_URL}/api/v1/settings", shell=True, capture_output=True, text=True, timeout=35)
            if r.stdout:
                settings = json.loads(r.stdout)
                break
        except:
            print(f"  [retry {attempt+1}/5] settings timeout...")
            time.sleep(3)
    else:
        print("FAILED to get DXFchk settings — is DXFchk running?")
        return
    
    output_folder = settings.get("output_folder", "")
    template_folder = settings.get("template_folder", "")
    
    # If folders are empty, load from active project
    if not output_folder or not template_folder:
        active_id = settings.get("active_project_id", "")
        if active_id:
            r2 = subprocess.run(f"curl -s --max-time 30 {DXFCHK_URL}/api/v1/projects", shell=True, capture_output=True, text=True, timeout=35)
            projects = json.loads(r2.stdout)
            for proj in projects.get("projects", []):
                if proj.get("id") == active_id:
                    if not output_folder:
                        output_folder = proj.get("output_folder", "")
                    if not template_folder:
                        template_folder = proj.get("template_folder", "")
                    break
    
    print(f"Output: {output_folder}")
    print(f"Templates: {template_folder}\n")
    
    # Create test folder
    test_dir = "C:\\Users\\Administrator\\Desktop\\tmpl_val_test"
    exec_cmd(f"mkdir {test_dir}", timeout=10)
    
    # List groups with original templates
    entries = fs_list(output_folder)
    all_folders = [e["name"] for e in entries if e.get("is_dir", False)]
    non_modn = [f for f in all_folders if not ("_mod" in f and f.split("_mod")[-1].isdigit())]
    
    tmpl_entries = fs_list(template_folder)
    original_templates = {e["name"] for e in tmpl_entries if e.get("name", "").endswith(".dxf")}
    groups_with_original = sorted([g for g in non_modn if f"{g}.dxf" in original_templates])
    print(f"Groups with original template: {len(groups_with_original)}\n")
    
    results = []
    exact_matches = 0
    close_matches = 0
    mismatches = 0
    
    for gi, group in enumerate(groups_with_original):
        group_path = f"{output_folder}\\{group}"
        file_entries = fs_list(group_path)
        dxf_files = [e["name"] for e in file_entries if e.get("name", "").endswith(".dxf")]
        
        if not dxf_files:
            continue
        
        dxf_file = dxf_files[0]
        mod_path = f"{group_path}\\{dxf_file}"
        orig_path = f"{template_folder}\\{group}.dxf"
        
        # Create template from module using NEW normalization
        template_name = group  # Use same name as original template
        result = dxfchk_create(mod_path, template_name, test_dir)
        
        if not result.get("ok"):
            print(f"[{gi+1}/{len(groups_with_original)}] {group}: CREATE FAILED — {result.get('error','?')[:100]}")
            continue
        
        created_path = result.get("template_path", "")
        
        # Compare created template vs original template
        diffs = compare_files_remote(orig_path, created_path)
        
        if len(diffs) == 0:
            print(f"[{gi+1}/{len(groups_with_original)}] {group}: ✅ EXACT MATCH (0 diffs)")
            exact_matches += 1
        elif len(diffs) <= 10:
            print(f"[{gi+1}/{len(groups_with_original)}] {group}: ⚠️  CLOSE MATCH ({len(diffs)} diffs)")
            close_matches += 1
            for d in diffs[:5]:
                print(f"    line {d.get('line')} code={d.get('code')} orig='{str(d.get('orig',''))[:50]}' mod='{str(d.get('mod',''))[:50]}'")
        else:
            print(f"[{gi+1}/{len(groups_with_original)}] {group}: ❌ MISMATCH ({len(diffs)} diffs)")
            mismatches += 1
            # Show first 5 diffs
            for d in diffs[:5]:
                print(f"    line {d.get('line')} code={d.get('code')} orig='{str(d.get('orig',''))[:50]}' mod='{str(d.get('mod',''))[:50]}'")
        
        results.append({
            "group": group,
            "file": dxf_file,
            "diffs": len(diffs),
            "status": "exact" if len(diffs) == 0 else ("close" if len(diffs) <= 10 else "mismatch"),
            "sample_diffs": diffs[:10],
        })
        
        # Clean up created file
        exec_cmd(f"del {created_path}", timeout=5)
        sys.stdout.flush()
        time.sleep(0.3)
    
    # Summary
    print(f"\n{'='*60}")
    print(f"VALIDATION RESULTS")
    print(f"{'='*60}")
    print(f"Groups tested: {len(results)}")
    print(f"  ✅ Exact match:  {exact_matches}")
    print(f"  ⚠️  Close match:   {close_matches}")
    print(f"  ❌ Mismatch:      {mismatches}")
    print()
    
    if mismatches:
        print("Mismatched groups:")
        for r in results:
            if r["status"] == "mismatch":
                print(f"  {r['group']}: {r['diffs']} diffs")
    
    if close_matches:
        print("\nClose match groups (need refinement):")
        for r in results:
            if r["status"] == "close":
                print(f"  {r['group']}: {r['diffs']} diffs")
    
    # Save results
    with open(OUTFILE, "w") as f:
        json.dump(results, f, indent=2, default=str)
    
    # Clean up
    exec_cmd(f"rmdir {test_dir}", timeout=10)
    print(f"\nResults saved to {OUTFILE}")

if __name__ == "__main__":
    main()