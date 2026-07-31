#!/usr/bin/env python3
"""
Reverse-engineer template creation — v2
Instead of downloading full files (too slow via PROBE fs-read), use PowerShell
on Vegas to extract ONLY the differing lines between template and module.
This is much faster: the comparison runs on the remote machine.
"""
import json, subprocess, base64, sys, time, os
from collections import defaultdict

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

def exec_cmd(command, timeout=60):
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

def fs_write(path, content_b64, mode="0644"):
    hn, hv = hdr()
    r = subprocess.run(["curl", "-s", "-X", "POST", f"{BASE}/api/v1/agents/{AGENT_ID}/fs-write",
         "-H", "Content-Type: application/json",
         "-H", f"{hn}: {hv}",
         "-d", json.dumps({"path": path, "data": content_b64, "mode": mode}),
         "--max-time", "15"], capture_output=True, text=True, timeout=20)
    resp = json.loads(r.stdout)
    return resp.get("ok", False)

def compare_files_remote(template_path, module_path):
    """
    Use PowerShell on Vegas to compare two files line by line.
    Returns list of diffs: {line, code, orig, mod}
    """
    # Write a PS1 script that compares two files and outputs JSON
    script = f'''
$orig = Get-Content -Path "{template_path}" -Encoding Default
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
$diffs | ConvertTo-Json -Compress
'''
    script_b64 = base64.b64encode(script.encode("utf-16-le")).decode()
    
    # Write script to Vegas
    script_path = "C:\\Users\\Administrator\\Desktop\\compare_dxf.ps1"
    # Write as UTF8 with BOM for PowerShell
    script_utf8 = "\ufeff" + script
    script_b64_utf8 = base64.b64encode(script_utf8.encode("utf-8")).decode()
    fs_write(script_path, script_b64_utf8)
    
    # Execute
    stdout, exit_code = exec_cmd(f"powershell -ExecutionPolicy Bypass -File {script_path}", timeout=120)
    
    # Clean up
    exec_cmd(f"del {script_path}", timeout=5)
    
    if not stdout:
        return []
    
    try:
        # Parse JSON output
        result = json.loads(stdout)
        if isinstance(result, list):
            return result
        elif isinstance(result, dict):
            return [result]
    except:
        # Try to parse line by line
        pass
    
    return []

def main():
    login()
    print("PROBE login OK\n")
    
    # Get settings
    r = subprocess.run(f"curl -s {DXFCHK_URL}/api/v1/settings", shell=True, capture_output=True, text=True, timeout=10)
    settings = json.loads(r.stdout)
    output_folder = settings.get("output_folder", "")
    template_folder = settings.get("template_folder", "")
    print(f"Output: {output_folder}")
    print(f"Templates: {template_folder}\n")
    
    # List output subfolders
    entries = fs_list(output_folder)
    all_folders = [e["name"] for e in entries if e.get("is_dir", False)]
    
    non_modn = []
    for f in all_folders:
        if "_mod" in f and f.split("_mod")[-1].isdigit():
            continue
        non_modn.append(f)
    
    # List original templates
    tmpl_entries = fs_list(template_folder)
    original_templates = {e["name"] for e in tmpl_entries if e.get("name", "").endswith(".dxf")}
    print(f"Non-_modN groups: {len(non_modn)}")
    print(f"Original templates: {len(original_templates)}\n")
    
    groups_with_original = sorted([g for g in non_modn if f"{g}.dxf" in original_templates])
    print(f"Groups with matching original template: {len(groups_with_original)}\n")
    
    # Analyze ALL groups with original templates
    groups_to_analyze = groups_with_original  # all 62
    
    all_diffs = []
    group_summaries = []
    
    for gi, group in enumerate(groups_to_analyze):
        group_path = f"{output_folder}\\{group}"
        file_entries = fs_list(group_path)
        dxf_files = [e["name"] for e in file_entries if e.get("name", "").endswith(".dxf")]
        
        if not dxf_files:
            continue
        
        # Use first file as representative
        dxf_file = dxf_files[0]
        mod_path = f"{group_path}\\{dxf_file}"
        orig_path = f"{template_folder}\\{group}.dxf"
        
        print(f"[{gi+1}/{len(groups_to_analyze)}] {group} ({len(dxf_files)} files, using {dxf_file})...")
        
        # Compare on remote
        diffs = compare_files_remote(orig_path, mod_path)
        
        if not diffs:
            print(f"  No diffs or comparison failed")
            continue
        
        # Categorize
        code_counts = defaultdict(int)
        code_1_diffs = []
        code_8_diffs = []
        other_diffs = []
        
        for d in diffs:
            code = str(d.get("code", "?"))
            code_counts[code] += 1
            d["group"] = group
            d["file"] = dxf_file
            d["template_name"] = group
            
            # Extract module ID from filename
            parts = dxf_file.replace(".dxf", "").split("_")
            d["module_id"] = ".".join(parts[:3]) if len(parts) >= 3 else dxf_file
            
            if code == "1":
                code_1_diffs.append(d)
            elif code == "8":
                code_8_diffs.append(d)
            else:
                other_diffs.append(d)
            
            all_diffs.append(d)
        
        # Name replacement count
        name_repl = sum(1 for d in code_1_diffs if group in d.get("orig", ""))
        val_changes = len(code_1_diffs) - name_repl
        
        summary = {
            "group": group,
            "file": dxf_file,
            "files_in_group": len(dxf_files),
            "total_diffs": len(diffs),
            "code_1": len(code_1_diffs),
            "code_8": len(code_8_diffs),
            "other": len(other_diffs),
            "name_repl": name_repl,
            "val_changes": val_changes,
            "other_codes": dict(code_counts),
        }
        group_summaries.append(summary)
        
        print(f"  diffs={len(diffs)} c1={len(code_1_diffs)} c8={len(code_8_diffs)} other={len(other_diffs)} name_repl={name_repl} val={val_changes}")
        
        if other_diffs:
            print(f"  ⚠️  Other codes: {dict(code_counts)}")
            for d in other_diffs[:3]:
                print(f"    code={d.get('code')} orig='{str(d.get('orig',''))[:60]}' mod='{str(d.get('mod',''))[:60]}'")
        
        # Show sample code-1
        for d in code_1_diffs[:3]:
            is_name = group in d.get("orig", "")
            tag = "NAME" if is_name else "VALUE"
            print(f"  [{tag}] '{str(d.get('orig',''))[:50]}' → '{str(d.get('mod',''))[:50]}'")
        
        sys.stdout.flush()
        time.sleep(0.3)
    
    # Cross-group analysis
    print(f"\n{'='*60}")
    print(f"CROSS-GROUP PATTERN ANALYSIS")
    print(f"{'='*60}")
    print(f"Groups analyzed: {len(group_summaries)}")
    print(f"Total diffs: {sum(s['total_diffs'] for s in group_summaries)}")
    
    all_code_1 = [d for d in all_diffs if str(d.get("code")) == "1"]
    all_code_8 = [d for d in all_diffs if str(d.get("code")) == "8"]
    all_other = [d for d in all_diffs if str(d.get("code")) not in ("1", "8")]
    
    print(f"\nCode 1 (attribute values): {len(all_code_1)}")
    print(f"Code 8 (layers): {len(all_code_8)}")
    print(f"Other codes: {len(all_other)}")
    
    # Code 1 breakdown
    name_repl_total = sum(1 for d in all_code_1 if d.get("template_name", "") in d.get("orig", ""))
    val_change_total = len(all_code_1) - name_repl_total
    print(f"\nCode 1 breakdown:")
    print(f"  Template name → module ID: {name_repl_total}")
    print(f"  Other value changes: {val_change_total}")
    
    # Layer patterns
    print(f"\nLayer (code 8) patterns:")
    layer_patterns = defaultdict(int)
    for d in all_code_8:
        layer_patterns[(d.get("orig", ""), d.get("mod", ""))] += 1
    for (o, m), count in sorted(layer_patterns.items(), key=lambda x: -x[1]):
        print(f"  '{o}' → '{m}': {count}")
    
    # Other codes
    if all_other:
        print(f"\nOther code patterns:")
        other_by_code = defaultdict(list)
        for d in all_other:
            other_by_code[str(d.get("code"))].append(d)
        for code, diffs in sorted(other_by_code.items()):
            print(f"\n  Code {code} ({len(diffs)} diffs):")
            seen = set()
            for d in diffs:
                key = (str(d.get("orig",""))[:40], str(d.get("mod",""))[:40])
                if key not in seen:
                    seen.add(key)
                    print(f"    '{str(d.get('orig',''))[:60]}' → '{str(d.get('mod',''))[:60]}'")
                    if len(seen) >= 10:
                        break
    
    # "Other value changes" — these are the key to understanding normalization
    other_values = [d for d in all_code_1 if d.get("template_name", "") not in d.get("orig", "")]
    print(f"\n\n'Other value' changes (not template name):")
    print(f"  Total: {len(other_values)}")
    # Group by original value
    by_orig = defaultdict(list)
    for d in other_values:
        by_orig[d.get("orig", "")].append(d)
    
    print(f"\n  Unique original values: {len(by_orig)}")
    print(f"\n  Top patterns (original → module value):")
    for orig, mods in sorted(by_orig.items(), key=lambda x: -len(x[1]))[:30]:
        mod_samples = set(d.get("mod", "")[:40] for d in mods[:5])
        print(f"    '{orig[:50]}' → {len(mods)} times")
        for m in list(mod_samples)[:2]:
            print(f"      e.g. '{m}'")
    
    # Save full results
    with open("/opt/data/dxfchk/template_analysis_full.json", "w") as f:
        json.dump({
            "group_summaries": group_summaries,
            "layer_patterns": {f"{o}|{m}": c for (o, m), c in layer_patterns.items()},
            "sample_diffs": all_diffs[:1000],
        }, f, indent=2, default=str)
    
    print(f"\nFull analysis saved to template_analysis_full.json")

if __name__ == "__main__":
    main()