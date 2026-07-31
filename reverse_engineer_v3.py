#!/usr/bin/env python3
"""
Reverse-engineer template creation v3 — incremental save, all 62 groups.
Uses PowerShell compare on Vegas, saves results after each group.
"""
import json, subprocess, base64, sys, time, os
from collections import defaultdict

BASE = "http://187.124.31.229:80"
DXFCHK_URL = "http://187.124.31.229:18643"
AGENT_ID = "vegas-c2022"
OUTFILE = "/opt/data/dxfchk/template_analysis_full.json"

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
         "-H", "Content-Type: application/json", "-H", f"{hn}: {hv}",
         "-d", json.dumps({"path": path}),
         "--max-time", "15"], capture_output=True, text=True, timeout=20)
    resp = json.loads(r.stdout)
    return resp["data"].get("entries", []) if resp.get("ok") else []

def exec_cmd(command, timeout=120):
    hn, hv = hdr()
    r = subprocess.run(["curl", "-s", "-X", "POST", f"{BASE}/api/v1/agents/{AGENT_ID}/exec",
         "-H", "Content-Type: application/json", "-H", f"{hn}: {hv}",
         "-d", json.dumps({"command": command, "timeout": timeout}),
         "--max-time", str(timeout + 10)], capture_output=True, text=True, timeout=timeout + 15)
    resp = json.loads(r.stdout)
    if resp.get("ok"):
        return resp["data"].get("stdout", ""), resp["data"].get("exit_code", 0)
    return "", -1

def fs_write(path, content_b64, mode="0644"):
    hn, hv = hdr()
    r = subprocess.run(["curl", "-s", "-X", "POST", f"{BASE}/api/v1/agents/{AGENT_ID}/fs-write",
         "-H", "Content-Type: application/json", "-H", f"{hn}: {hv}",
         "-d", json.dumps({"path": path, "data": content_b64, "mode": mode}),
         "--max-time", "15"], capture_output=True, text=True, timeout=20)
    return json.loads(r.stdout).get("ok", False)

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
    script_path = "C:\\Users\\Administrator\\Desktop\\cmp_dxf.ps1"
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

def save_results(data):
    with open(OUTFILE, "w") as f:
        json.dump(data, f, indent=2, default=str)

def main():
    login()
    print("PROBE login OK\n")
    
    r = subprocess.run(f"curl -s {DXFCHK_URL}/api/v1/settings", shell=True, capture_output=True, text=True, timeout=10)
    settings = json.loads(r.stdout)
    output_folder = settings["output_folder"]
    template_folder = settings["template_folder"]
    
    entries = fs_list(output_folder)
    all_folders = [e["name"] for e in entries if e.get("is_dir", False)]
    non_modn = [f for f in all_folders if not ("_mod" in f and f.split("_mod")[-1].isdigit())]
    
    tmpl_entries = fs_list(template_folder)
    original_templates = {e["name"] for e in tmpl_entries if e.get("name", "").endswith(".dxf")}
    groups_with_original = sorted([g for g in non_modn if f"{g}.dxf" in original_templates])
    print(f"Groups to analyze: {len(groups_with_original)}\n")
    
    # Load existing results if any (resume support)
    all_diffs = []
    group_summaries = []
    done_groups = set()
    if os.path.exists(OUTFILE):
        try:
            with open(OUTFILE) as f:
                old = json.load(f)
                group_summaries = old.get("group_summaries", [])
                all_diffs = old.get("all_diffs", [])
                done_groups = {s["group"] for s in group_summaries}
                print(f"Resuming: {len(done_groups)} groups already done")
        except:
            pass
    
    for gi, group in enumerate(groups_with_original):
        if group in done_groups:
            continue
        
        group_path = f"{output_folder}\\{group}"
        file_entries = fs_list(group_path)
        dxf_files = [e["name"] for e in file_entries if e.get("name", "").endswith(".dxf")]
        
        if not dxf_files:
            continue
        
        dxf_file = dxf_files[0]
        mod_path = f"{group_path}\\{dxf_file}"
        orig_path = f"{template_folder}\\{group}.dxf"
        
        print(f"[{gi+1}/{len(groups_with_original)}] {group} ({len(dxf_files)} files)...", end="", flush=True)
        
        diffs = compare_files_remote(orig_path, mod_path)
        
        if not diffs:
            print(" FAILED/IDENTICAL")
            continue
        
        parts = dxf_file.replace(".dxf", "").split("_")
        module_id = ".".join(parts[:3]) if len(parts) >= 3 else dxf_file
        
        code_counts = defaultdict(int)
        for d in diffs:
            code = str(d.get("code", "?"))
            code_counts[code] += 1
            d["group"] = group
            d["template_name"] = group
            d["module_id"] = module_id
            d["file"] = dxf_file
            all_diffs.append(d)
        
        c1 = code_counts.get("1", 0)
        c8 = code_counts.get("8", 0)
        other = sum(v for k, v in code_counts.items() if k not in ("1", "8"))
        name_repl = sum(1 for d in diffs if str(d.get("code")) == "1" and group in str(d.get("orig", "")))
        
        summary = {
            "group": group, "file": dxf_file, "files_in_group": len(dxf_files),
            "total_diffs": len(diffs), "code_1": c1, "code_8": c8, "other": other,
            "name_repl": name_repl, "val_changes": c1 - name_repl,
        }
        group_summaries.append(summary)
        done_groups.add(group)
        
        print(f" diffs={len(diffs)} c1={c1} c8={c8} other={other}")
        
        # Save after each group
        save_results({"group_summaries": group_summaries, "all_diffs": all_diffs})
        time.sleep(0.2)
    
    # Final analysis
    print(f"\n{'='*60}")
    print(f"ANALYSIS COMPLETE: {len(group_summaries)} groups")
    print(f"{'='*60}")
    
    all_code_1 = [d for d in all_diffs if str(d.get("code")) == "1"]
    all_code_8 = [d for d in all_diffs if str(d.get("code")) == "8"]
    all_other = [d for d in all_diffs if str(d.get("code")) not in ("1", "8")]
    
    print(f"Code 1: {len(all_code_1)} | Code 8: {len(all_code_8)} | Other: {len(all_other)}")
    
    # Layer patterns
    layer_patterns = defaultdict(int)
    for d in all_code_8:
        layer_patterns[(str(d.get("orig", "")), str(d.get("mod", "")))] += 1
    print(f"\nLayer patterns:")
    for (o, m), count in sorted(layer_patterns.items(), key=lambda x: -x[1]):
        print(f"  '{o}' → '{m}': {count}")
    
    # Other codes
    if all_other:
        other_by_code = defaultdict(list)
        for d in all_other:
            other_by_code[str(d.get("code"))].append(d)
        print(f"\nOther codes:")
        for code, diffs in sorted(other_by_code.items()):
            print(f"  Code {code}: {len(diffs)} diffs")
            seen = set()
            for d in diffs[:10]:
                key = (str(d.get("orig",""))[:40], str(d.get("mod",""))[:40])
                if key not in seen:
                    seen.add(key)
                    print(f"    '{str(d.get('orig',''))[:60]}' → '{str(d.get('mod',''))[:60]}'")
    
    # "Other value" changes in code 1 (not name replacements)
    other_values = [d for d in all_code_1 if d.get("template_name", "") not in str(d.get("orig", ""))]
    by_orig = defaultdict(list)
    for d in other_values:
        by_orig[str(d.get("orig", ""))].append(d)
    
    print(f"\n'Other value' changes (not name replacement): {len(other_values)}")
    print(f"Unique original (template) values: {len(by_orig)}")
    print(f"\nTop 40 patterns:")
    for orig, mods in sorted(by_orig.items(), key=lambda x: -len(x[1]))[:40]:
        mod_samples = list(set(str(d.get("mod", ""))[:40] for d in mods[:5]))
        print(f"  T='{orig[:50]}' ({len(mods)}x) → M='{mod_samples[0][:40] if mod_samples else '?'}'")
    
    save_results({"group_summaries": group_summaries, "all_diffs": all_diffs,
                   "layer_patterns": {f"{o}|{m}": c for (o, m), c in layer_patterns.items()}})
    print(f"\nSaved to {OUTFILE}")

if __name__ == "__main__":
    main()