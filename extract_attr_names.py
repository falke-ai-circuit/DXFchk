#!/usr/bin/env python3
"""
Extract attribute names (code 2) for each differing value.
For each diff found, look at the surrounding lines to find the ATTRIB/ATTDEF name.
This tells us WHICH attribute each value belongs to, enabling precise normalization.
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

def fs_list(path):
    hn, hv = hdr()
    r = subprocess.run(["curl", "-s", "-X", "POST", f"{BASE}/api/v1/agents/{AGENT_ID}/fs-list",
         "-H", "Content-Type: application/json", "-H", f"{hn}: {hv}",
         "-d", json.dumps({"path": path}),
         "--max-time", "15"], capture_output=True, text=True, timeout=20)
    resp = json.loads(r.stdout)
    return resp["data"].get("entries", []) if resp.get("ok") else []

def extract_attr_names_remote(template_path, module_path):
    """
    Use PowerShell to extract, for each ATTRIB/ATTDEF entity, the attribute name (code 2),
    attribute value (code 1), and whether it differs between template and module.
    Returns list of {attr_name, template_value, module_value, line_num}
    """
    script = f'''
$orig = Get-Content -Path "{template_path}" -Encoding Default
$mod = Get-Content -Path "{module_path}" -Encoding Default
$results = @()

# Parse ATTRIB and ATTDEF entities from BOTH files
# In DXF, entities start with code 0, and ATTDEF/ATTRIB have:
#   code 1 = text value
#   code 2 = attribute name (for ATTDEF) or block name (for ATTRIB in some contexts)
# For ATTRIB: code 1 = value, code 2 = attribute tag (name)
# For ATTDEF: code 1 = default text, code 2 = attribute tag (name)

function Parse-Attribs($lines, $entityType) {{
    $attribs = @()
    for ($i = 0; $i -lt $lines.Count; $i++) {{
        if ($lines[$i].Trim() -eq "0" -and $i+1 -lt $lines.Count -and $lines[$i+1].Trim() -eq $entityType) {{
            $code1 = ""
            $code2 = ""
            $lineNum = $i + 1
            for ($j = $i+2; $j -lt [Math]::Min($i+30, $lines.Count); $j++) {{
                $code = $lines[$j].Trim()
                if ($code -eq "0") {{ break }}
                if ($code -eq "1" -and $j+1 -lt $lines.Count) {{
                    $code1 = $lines[$j+1].Trim()
                }}
                if ($code -eq "2" -and $j+1 -lt $lines.Count) {{
                    $code2 = $lines[$j+1].Trim()
                }}
            }}
            if ($code2 -ne "" -or $code1 -ne "") {{
                $attribs += [PSCustomObject]@{{line=$lineNum; name=$code2; value=$code1; type=$entityType}}
            }}
        }}
    }}
    return $attribs
}}

$origAttribs = Parse-Attribs $orig "ATTRIB"
$origAttdefs = Parse-Attribs $orig "ATTDEF"
$modAttribs = Parse-Attribs $mod "ATTRIB"
$modAttdefs = Parse-Attribs $mod "ATTDEF"

# Compare ATTRIBs by line number (same structure = same line positions)
$maxAttribs = [Math]::Max($origAttribs.Count, $modAttribs.Count)
for ($i = 0; $i -lt $maxAttribs; $i++) {{
    $o = if ($i -lt $origAttribs.Count) {{ $origAttribs[$i] }} else {{ $null }}
    $m = if ($i -lt $modAttribs.Count) {{ $modAttribs[$i] }} else {{ $null }}
    if ($o -and $m -and $o.value -ne $m.value) {{
        $results += [PSCustomObject]@{{line=$o.line; attr_name=$o.name; tmpl_val=$o.value; mod_val=$m.value; entity="ATTRIB"}}
    }}
}}

# Compare ATTDEFs
$maxAttdefs = [Math]::Max($origAttdefs.Count, $modAttdefs.Count)
for ($i = 0; $i -lt $maxAttdefs; $i++) {{
    $o = if ($i -lt $origAttdefs.Count) {{ $origAttdefs[$i] }} else {{ $null }}
    $m = if ($i -lt $modAttdefs.Count) {{ $modAttdefs[$i] }} else {{ $null }}
    if ($o -and $m -and $o.value -ne $m.value) {{
        $results += [PSCustomObject]@{{line=$o.line; attr_name=$o.name; tmpl_val=$o.value; mod_val=$m.value; entity="ATTDEF"}}
    }}
}}

$results | ConvertTo-Json -Compress -Depth 3
'''
    script_path = "C:\\Users\\Administrator\\Desktop\\extract_attrs.ps1"
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
    
    for attempt in range(3):
        try:
            r = subprocess.run(f"curl -s --max-time 30 {DXFCHK_URL}/api/v1/settings", shell=True, capture_output=True, text=True, timeout=35)
            if r.stdout:
                settings = json.loads(r.stdout)
                break
        except:
            print(f"  [retry {attempt+1}/3] settings timeout...")
            time.sleep(2)
    else:
        print("FAILED to get settings")
        return
    output_folder = settings["output_folder"]
    template_folder = settings["template_folder"]
    
    entries = fs_list(output_folder)
    all_folders = [e["name"] for e in entries if e.get("is_dir", False)]
    non_modn = [f for f in all_folders if not ("_mod" in f and f.split("_mod")[-1].isdigit())]
    
    tmpl_entries = fs_list(template_folder)
    original_templates = {e["name"] for e in tmpl_entries if e.get("name", "").endswith(".dxf")}
    groups_with_original = sorted([g for g in non_modn if f"{g}.dxf" in original_templates])
    print(f"Groups to analyze: {len(groups_with_original)}\n")
    
    # For each group, extract attribute names + template values + module values
    all_attr_diffs = []
    
    for gi, group in enumerate(groups_with_original):
        group_path = f"{output_folder}\\{group}"
        file_entries = fs_list(group_path)
        dxf_files = [e["name"] for e in file_entries if e.get("name", "").endswith(".dxf")]
        if not dxf_files:
            continue
        
        dxf_file = dxf_files[0]
        mod_path = f"{group_path}\\{dxf_file}"
        orig_path = f"{template_folder}\\{group}.dxf"
        
        print(f"[{gi+1}/{len(groups_with_original)}] {group}...", end="", flush=True)
        
        attrs = extract_attr_names_remote(orig_path, mod_path)
        
        if not attrs:
            print(" NO ATTRIBS/FAILED")
            continue
        
        parts = dxf_file.replace(".dxf", "").split("_")
        module_id = ".".join(parts[:3]) if len(parts) >= 3 else dxf_file
        
        for a in attrs:
            a["group"] = group
            a["module_id"] = module_id
            a["file"] = dxf_file
            all_attr_diffs.append(a)
        
        print(f" {len(attrs)} attr diffs")
        sys.stdout.flush()
        time.sleep(0.2)
    
    # Analyze
    print(f"\n{'='*60}")
    print(f"ATTRIBUTE NAME → PLACEHOLDER MAPPING")
    print(f"{'='*60}")
    print(f"Total attribute diffs: {len(all_attr_diffs)}")
    
    # Group by attribute name
    by_attr = defaultdict(list)
    for a in all_attr_diffs:
        name = str(a.get("attr_name", ""))
        by_attr[name].append(a)
    
    print(f"Unique attribute names: {len(by_attr)}")
    print()
    
    # For each attribute name, show template values vs module values
    print(f"Attribute name → Template (placeholder) values:")
    print(f"{'='*60}")
    
    for name in sorted(by_attr.keys(), key=lambda x: -len(by_attr[x])):
        diffs = by_attr[name]
        tmpl_vals = set(str(d.get("tmpl_val", ""))[:60] for d in diffs)
        mod_vals = set(str(d.get("mod_val", ""))[:60] for d in diffs)
        
        # Is the template value always the same (a fixed placeholder)?
        if len(tmpl_vals) <= 3:
            print(f"\n  {name} ({len(diffs)} diffs):")
            for tv in tmpl_vals:
                print(f"    Template: '{tv}'")
            print(f"    Module values ({len(mod_vals)} unique):")
            for mv in list(mod_vals)[:5]:
                print(f"      → '{mv}'")
            if len(mod_vals) > 5:
                print(f"      ... and {len(mod_vals)-5} more")
        else:
            print(f"\n  {name} ({len(diffs)} diffs, {len(tmpl_vals)} unique template values):")
            for tv in list(tmpl_vals)[:3]:
                print(f"    Template: '{tv}'")
            print(f"    Module values sample:")
            for mv in list(mod_vals)[:3]:
                print(f"      → '{mv}'")
    
    # Save
    with open("/opt/data/dxfchk/attr_name_mapping.json", "w") as f:
        json.dump({
            "all_attr_diffs": all_attr_diffs,
            "by_attr": {name: len(diffs) for name, diffs in by_attr.items()},
        }, f, indent=2, default=str)
    
    print(f"\nSaved to attr_name_mapping.json")

if __name__ == "__main__":
    main()