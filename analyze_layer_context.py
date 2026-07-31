#!/usr/bin/env python3
"""
Position-aware layer analysis: for each layer diff between template and module,
extract the surrounding DXF context (entity type, block name, attribute name)
to find what distinguishes "0 → N_COM_HIDDEN" from "N_COM_HIDDEN stays N_COM_HIDDEN".
"""
import json, subprocess, base64, sys, time

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

def analyze_layer_context(template_path, module_path):
    """
    For each code-8 (layer) diff, extract:
    - The entity type (code 0 before the entity)
    - The block name (code 2 in INSERT entities)
    - The attribute name (code 2 in ATTRIB/ATTDEF)
    - The code-1 value (text/attribute value)
    
    This tells us WHAT entity has the layer change, helping us find the pattern.
    """
    script = f'''
$orig = Get-Content -Path "{template_path}" -Encoding Default
$mod = Get-Content -Path "{module_path}" -Encoding Default
$results = @()

for ($i = 0; $i -lt [Math]::Max($orig.Count, $mod.Count); $i++) {{
    $o = if ($i -lt $orig.Count) {{ $orig[$i].Trim() }} else {{ "<MISSING>" }}
    $m = if ($i -lt $mod.Count) {{ $mod[$i].Trim() }} else {{ "<MISSING>" }}
    
    # Only look at code-8 (layer) diffs
    if ($o -ne $m -and $i -gt 0 -and $orig[$i-1].Trim() -eq "8") {{
        # Walk backwards to find entity type (code 0), block name (code 2), code 1 value
        $entityType = "?"
        $blockName = "?"
        $code1Val = "?"
        $attrName = "?"
        
        for ($j = $i - 1; $j -ge [Math]::Max(0, $i - 30); $j--) {{
            $code = $orig[$j].Trim()
            if ($code -eq "0" -and $j+1 -lt $orig.Count) {{
                $entityType = $orig[$j+1].Trim()
                break
            }}
            if ($code -eq "2" -and $blockName -eq "?") {{
                $blockName = $orig[$j+1].Trim()
            }}
            if ($code -eq "1" -and $code1Val -eq "?") {{
                $code1Val = $orig[$j+1].Trim()
            }}
        }}
        
        # Also check what the module has at the same position
        $modEntityType = "?"
        for ($j = $i - 1; $j -ge [Math]::Max(0, $i - 30); $j--) {{
            if ($j -lt $mod.Count -and $mod[$j].Trim() -eq "0" -and $j+1 -lt $mod.Count) {{
                $modEntityType = $mod[$j+1].Trim()
                break
            }}
        }}
        
        $results += [PSCustomObject]@{{
            line = $i + 1
            tmpl_layer = $o
            mod_layer = $m
            entity = $entityType
            block = $blockName
            code1 = $code1Val
        }}
    }}
}}

$results | ConvertTo-Json -Compress -Depth 3
'''
    script_path = "C:\\Users\\Administrator\\Desktop\\layer_ctx.ps1"
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
    
    r = subprocess.run(f"curl -s --max-time 30 {DXFCHK_URL}/api/v1/settings", shell=True, capture_output=True, text=True, timeout=35)
    settings = json.loads(r.stdout)
    output_folder = settings.get("output_folder", "")
    template_folder = settings.get("template_folder", "")
    
    if not output_folder or not template_folder:
        r2 = subprocess.run(f"curl -s --max-time 30 {DXFCHK_URL}/api/v1/projects", shell=True, capture_output=True, text=True, timeout=35)
        projects = json.loads(r2.stdout)
        for proj in projects.get("projects", []):
            if proj.get("id") == settings.get("active_project_id"):
                if not output_folder:
                    output_folder = proj.get("output_folder", "")
                if not template_folder:
                    template_folder = proj.get("template_folder", "")
                break
    
    print(f"Output: {output_folder}")
    print(f"Templates: {template_folder}\n")
    
    # Analyze 5 groups with known layer diffs
    test_groups = ["AMR02p3", "BO002p1", "MF001Hp1a", "MS001Hp1", "MF001p4"]
    
    all_layer_diffs = []
    
    for group in test_groups:
        group_path = f"{output_folder}\\{group}"
        file_entries = fs_list(group_path)
        dxf_files = [e["name"] for e in file_entries if e.get("name", "").endswith(".dxf")]
        if not dxf_files:
            continue
        
        dxf_file = dxf_files[0]
        mod_path = f"{group_path}\\{dxf_file}"
        orig_path = f"{template_folder}\\{group}.dxf"
        
        print(f"=== {group} ({dxf_file}) ===")
        diffs = analyze_layer_context(orig_path, mod_path)
        
        if not diffs:
            print("  No layer diffs or failed")
            continue
        
        print(f"  Layer diffs: {len(diffs)}")
        
        # Group by (tmpl_layer, mod_layer, entity_type, block_name)
        from collections import defaultdict
        by_pattern = defaultdict(list)
        for d in diffs:
            key = (d.get("tmpl_layer",""), d.get("mod_layer",""), d.get("entity",""), d.get("block","")[:30])
            by_pattern[key].append(d)
        
        for key, entries in sorted(by_pattern.items(), key=lambda x: -len(x[1])):
            tmpl_l, mod_l, entity, block = key
            print(f"  {tmpl_l:20s} → {mod_l:20s} | entity={entity:12s} block={block:30s} | {len(entries)}x")
            # Show first entry with code1 value
            if entries:
                print(f"    code1='{entries[0].get('code1','')[:50]}'")
        
        for d in diffs:
            d["group"] = group
            all_layer_diffs.append(d)
        
        print()
        sys.stdout.flush()
        time.sleep(0.3)
    
    # Cross-group analysis: what entity types have "0 → N_COM_HIDDEN" vs "N_COM_HIDDEN stays"?
    print(f"\n{'='*60}")
    print(f"CROSS-GROUP LAYER PATTERN ANALYSIS")
    print(f"{'='*60}")
    
    from collections import defaultdict
    by_entity = defaultdict(lambda: defaultdict(int))
    for d in all_layer_diffs:
        tmpl_l = d.get("tmpl_layer", "")
        mod_l = d.get("mod_layer", "")
        entity = d.get("entity", "")
        by_entity[entity][f"{tmpl_l} → {mod_l}"] += 1
    
    for entity, patterns in sorted(by_entity.items()):
        print(f"\n  Entity: {entity}")
        for pattern, count in sorted(patterns.items(), key=lambda x: -x[1]):
            print(f"    {pattern}: {count}x")
    
    # KEY: which entity types have tmpl="0" → mod="N_COM_HIDDEN"?
    # And which have tmpl="N_COM_HIDDEN" → mod="N_COM_EVAL_FALSE"?
    print(f"\n{'='*60}")
    print(f"KEY: Entity types where template has '0' → module has COM_HIDDEN")
    print(f"{'='*60}")
    for entity, patterns in sorted(by_entity.items()):
        zero_to_hidden = sum(count for pattern, count in patterns.items() if pattern.startswith("0 →") and "COM_HIDDEN" in pattern)
        if zero_to_hidden > 0:
            print(f"  {entity}: {zero_to_hidden}x")
    
    print(f"\n{'='*60}")
    print(f"KEY: Entity types where template has COM_HIDDEN → module has EVAL_FALSE")
    print(f"{'='*60}")
    for entity, patterns in sorted(by_entity.items()):
        hidden_to_eval = sum(count for pattern, count in patterns.items() if "COM_HIDDEN →" in pattern and "EVAL_FALSE" in pattern)
        if hidden_to_eval > 0:
            print(f"  {entity}: {hidden_to_eval}x")
    
    # Save
    with open("/opt/data/dxfchk/layer_context_analysis.json", "w") as f:
        json.dump(all_layer_diffs, f, indent=2, default=str)
    print(f"\nSaved to layer_context_analysis.json")

if __name__ == "__main__":
    main()