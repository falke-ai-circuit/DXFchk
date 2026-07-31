#!/usr/bin/env python3
"""
Reverse-engineer Valmet DNA Explorer template creation by analyzing multiple
template/module pairs. For each pair where we have the original template:
1. Download both files
2. Line-by-line diff
3. Categorize every change by DXF code and pattern
4. Build normalization rules
"""
import json, subprocess, base64, hashlib, sys, os, time, re
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

def fs_read_text(filepath, timeout=180):
    hn, hv = hdr()
    for attempt in range(3):
        try:
            r = subprocess.run(["curl", "-s", "-X", "POST", f"{BASE}/api/v1/agents/{AGENT_ID}/fs-read",
                 "-H", "Content-Type: application/json",
                 "-H", f"{hn}: {hv}",
                 "-d", json.dumps({"path": filepath}),
                 "--max-time", str(timeout)],
                 capture_output=True, text=True, timeout=timeout + 10)
            resp = json.loads(r.stdout)
            if resp.get("ok"):
                data = resp["data"].get("data", "")
                if data:
                    return base64.b64decode(data).decode("utf-8", errors="replace")
            return None
        except subprocess.TimeoutExpired:
            print(f"  [retry {attempt+1}/3] timeout reading {filepath[-40:]}...")
            time.sleep(2)
    return None

def main():
    login()
    print("PROBE login OK\n")
    
    # Get settings
    r = subprocess.run(["curl", "-s", f"{DXFCHK_URL}/api/v1/settings"], capture_output=True, text=True, timeout=10)
    settings = json.loads(r.stdout)
    output_folder = settings.get("output_folder", "")
    template_folder = settings.get("template_folder", "")
    print(f"Output: {output_folder}")
    print(f"Templates: {template_folder}\n")
    
    # List output subfolders
    entries = fs_list(output_folder)
    all_folders = [e["name"] for e in entries if e.get("is_dir", False)]
    
    # Filter to non-_modN groups only
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
    
    # Find groups that have matching original templates
    groups_with_original = []
    for group in non_modn:
        if f"{group}.dxf" in original_templates:
            groups_with_original.append(group)
    
    print(f"Groups with matching original template: {len(groups_with_original)}")
    print()
    
    # Analyze each pair — download both files, do detailed diff
    # To avoid flooding memory, analyze up to 20 groups
    # Pick diverse groups (different first chars)
    groups_to_analyze = sorted(groups_with_original)[:20]
    
    all_diffs = []  # list of {group, file, code, orig_val, mod_val, line_num, context}
    group_summaries = []
    
    for gi, group in enumerate(groups_to_analyze):
        print(f"[{gi+1}/{len(groups_to_analyze)}] {group}...")
        
        group_path = f"{output_folder}\\{group}"
        file_entries = fs_list(group_path)
        dxf_files = [e["name"] for e in file_entries if e.get("name", "").endswith(".dxf")]
        
        if not dxf_files:
            continue
        
        # Use first file as representative
        dxf_file = dxf_files[0]
        mod_path = f"{group_path}\\{dxf_file}"
        orig_path = f"{template_folder}\\{group}.dxf"
        
        # Download both
        orig_text = fs_read_text(orig_path)
        mod_text = fs_read_text(mod_path)
        
        if not orig_text or not mod_text:
            print(f"  Failed to download")
            continue
        
        orig_lines = orig_text.replace("\r\n", "\n").split("\n")
        mod_lines = mod_text.replace("\r\n", "\n").split("\n")
        
        # Line-by-line diff
        max_lines = max(len(orig_lines), len(mod_lines))
        group_diffs = []
        
        for i in range(max_lines):
            o = orig_lines[i].strip() if i < len(orig_lines) else "<MISSING>"
            m = mod_lines[i].strip() if i < len(mod_lines) else "<MISSING>"
            if o != m:
                # Find the DXF code (previous line)
                code = "?"
                for j in range(i-1, max(0, i-5), -1):
                    if j < len(orig_lines):
                        c = orig_lines[j].strip()
                        if c and not c.startswith("<") and len(c) < 10:
                            # Check if it's a numeric DXF code
                            try:
                                int(c)
                                code = c
                                break
                            except:
                                pass
                
                group_diffs.append({
                    "line": i + 1,
                    "code": code,
                    "orig": o[:200],
                    "mod": m[:200],
                })
        
        # Categorize diffs
        code_counts = defaultdict(int)
        code_1_patterns = []
        code_8_patterns = []
        other_patterns = []
        
        for d in group_diffs:
            code_counts[d["code"]] += 1
            if d["code"] == "1":
                code_1_patterns.append(d)
            elif d["code"] == "8":
                code_8_patterns.append(d)
            else:
                other_patterns.append(d)
        
        summary = {
            "group": group,
            "file": dxf_file,
            "total_diffs": len(group_diffs),
            "code_1": len(code_1_patterns),
            "code_8": len(code_8_patterns),
            "other": len(other_patterns),
            "other_codes": dict(code_counts),
        }
        
        # Analyze code 1 patterns
        # Pattern: template_name → module_id (e.g., AMR02p3 → 602.5000.0006)
        template_name = group
        # Extract module ID from filename (e.g., 602_p5000_p0006 → 602.5000.0006)
        parts = dxf_file.replace(".dxf", "").split("_")
        module_id = ".".join(parts[:3]) if len(parts) >= 3 else dxf_file
        
        name_replacements = 0
        value_changes = 0
        layer_changes = 0
        
        for d in code_1_patterns:
            if template_name in d["orig"]:
                name_replacements += 1
            else:
                value_changes += 1
        
        for d in code_8_patterns:
            layer_changes += 1
        
        summary["name_replacements"] = name_replacements
        summary["value_changes"] = value_changes
        summary["layer_changes"] = layer_changes
        summary["module_id"] = module_id
        
        group_summaries.append(summary)
        
        # Store detailed diffs for pattern analysis
        for d in group_diffs:
            d["group"] = group
            d["module_id"] = module_id
            d["template_name"] = template_name
            all_diffs.append(d)
        
        print(f"  {len(dxf_files)} files | diffs: {len(group_diffs)} | code1={len(code_1_patterns)} code8={len(code_8_patterns)} other={len(other_patterns)}")
        print(f"  name_repl={name_replacements} val_changes={value_changes} layer={layer_changes}")
        if other_patterns:
            other_codes = [d["code"] for d in other_patterns]
            unique_other = set(other_codes)
            print(f"  ⚠️  Other codes: {dict(code_counts)}")
        
        # Show sample code-1 diffs
        if code_1_patterns:
            print(f"  Sample code-1 diffs:")
            for d in code_1_patterns[:5]:
                is_name = template_name in d["orig"]
                tag = "NAME" if is_name else "VALUE"
                print(f"    [{tag}] '{d['orig'][:60]}' → '{d['mod'][:60]}'")
        
        # Show sample code-8 diffs
        if code_8_patterns:
            print(f"  Sample code-8 (layer) diffs:")
            for d in code_8_patterns[:3]:
                print(f"    '{d['orig']}' → '{d['mod']}'")
        
        print()
        sys.stdout.flush()
        time.sleep(0.2)
    
    # Cross-group pattern analysis
    print(f"\n{'='*60}")
    print(f"CROSS-GROUP PATTERN ANALYSIS")
    print(f"{'='*60}")
    print(f"Groups analyzed: {len(group_summaries)}")
    print(f"Total diffs: {sum(s['total_diffs'] for s in group_summaries)}")
    print()
    
    # Analyze all code-1 diffs across groups
    all_code_1 = [d for d in all_diffs if d["code"] == "1"]
    all_code_8 = [d for d in all_diffs if d["code"] == "8"]
    all_other = [d for d in all_diffs if d["code"] not in ("1", "8")]
    
    print(f"Code 1 (attribute values): {len(all_code_1)} diffs")
    print(f"Code 8 (layers): {len(all_code_8)} diffs")
    print(f"Other codes: {len(all_other)} diffs")
    
    # Categorize code-1 diffs
    name_repl = 0
    value_change = 0
    for d in all_code_1:
        if d["template_name"] in d["orig"]:
            name_repl += 1
        else:
            value_change += 1
    
    print(f"\nCode 1 breakdown:")
    print(f"  Template name → module ID: {name_repl}")
    print(f"  Other value changes: {value_change}")
    
    # What are the "other value changes"?
    other_values = [d for d in all_code_1 if d["template_name"] not in d["orig"]]
    print(f"\n  Sample 'other' value changes:")
    seen = set()
    for d in other_values:
        key = (d["orig"][:40], d["mod"][:40])
        if key not in seen:
            seen.add(key)
            print(f"    '{d['orig'][:60]}' → '{d['mod'][:60]}' [group={d['group']}]")
            if len(seen) >= 30:
                break
    
    # Analyze code-8 (layer) patterns
    print(f"\nCode 8 (layer) patterns:")
    layer_patterns = defaultdict(int)
    for d in all_code_8:
        layer_patterns[(d["orig"], d["mod"])] += 1
    for (o, m), count in sorted(layer_patterns.items(), key=lambda x: -x[1]):
        print(f"  '{o}' → '{m}': {count} times")
    
    # Analyze other codes
    if all_other:
        print(f"\nOther code patterns:")
        other_by_code = defaultdict(list)
        for d in all_other:
            other_by_code[d["code"]].append(d)
        for code, diffs in sorted(other_by_code.items()):
            print(f"\n  Code {code} ({len(diffs)} diffs):")
            seen = set()
            for d in diffs:
                key = (d["orig"][:40], d["mod"][:40])
                if key not in seen:
                    seen.add(key)
                    print(f"    '{d['orig'][:60]}' → '{d['mod'][:60]}'")
                    if len(seen) >= 10:
                        break
    
    # Summary table
    print(f"\n{'='*60}")
    print(f"NORMALIZATION RULES (reverse-engineered)")
    print(f"{'='*60}")
    
    print(f"\nRule 1: Template name → Module ID")
    print(f"  Pattern: '{template_name}' → '{module_id}' (in code 1 values)")
    print(f"  Occurrences: {name_repl} across {len(group_summaries)} groups")
    
    print(f"\nRule 2: Layer changes")
    for (o, m), count in sorted(layer_patterns.items(), key=lambda x: -x[1]):
        print(f"  '{o}' → '{m}': {count} times")
    
    print(f"\nRule 3: Other value changes (I/O descriptions, units, etc.)")
    print(f"  {value_change} changes — these are module-specific implementation values")
    print(f"  Need to determine placeholder pattern for each")
    
    # Save full analysis to file
    with open("/opt/data/dxfchk/template_analysis_results.json", "w") as f:
        json.dump({
            "group_summaries": group_summaries,
            "all_diffs": all_diffs[:500],  # cap for storage
            "layer_patterns": {f"{o}|{m}": c for (o, m), c in layer_patterns.items()},
        }, f, indent=2)
    
    print(f"\nFull analysis saved to: template_analysis_results.json")

if __name__ == "__main__":
    main()