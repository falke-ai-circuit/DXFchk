#!/usr/bin/env python3
"""Analyze what differs between original template and a module to understand what needs to be normalized."""
import json, subprocess, base64

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

def fs_read_file(filepath, timeout=120):
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
            return base64.b64decode(data).decode("utf-8", errors="replace")
    return None

def main():
    login()
    
    template_folder = "C:\\Users\\Administrator\\Desktop\\V6049_CELEBRITY_REFLECTION\\templates"
    output_folder = "C:\\Users\\Administrator\\Desktop\\V6049_CELEBRITY_REFLECTION\\output"
    
    # Read original template
    orig_path = f"{template_folder}\\AMR02p3.dxf"
    orig_text = fs_read_file(orig_path)
    
    # Read module
    mod_path = f"{output_folder}\\AMR02p3\\602_p5000_p0006.dxf"
    mod_text = fs_read_file(mod_path)
    
    if not orig_text or not mod_text:
        print("Failed to read files")
        return
    
    orig_lines = orig_text.replace("\r\n", "\n").split("\n")
    mod_lines = mod_text.replace("\r\n", "\n").split("\n")
    
    print(f"Original template: {len(orig_lines)} lines, {len(orig_text)} bytes")
    print(f"Module: {len(mod_lines)} lines, {len(mod_text)} bytes")
    print(f"Size diff: {len(mod_text) - len(orig_text)} bytes")
    print()
    
    # Line-by-line diff (since template creation only changes $(TEMPLATE), 
    # the rest should be identical EXCEPT for attribute values)
    diffs = []
    max_lines = max(len(orig_lines), len(mod_lines))
    for i in range(max_lines):
        o = orig_lines[i].strip() if i < len(orig_lines) else "<MISSING>"
        m = mod_lines[i].strip() if i < len(mod_lines) else "<MISSING>"
        if o != m:
            # Get context (previous line = DXF code)
            prev_o = orig_lines[i-1].strip() if i > 0 else ""
            diffs.append({
                "line": i+1,
                "code": prev_o,
                "orig": o,
                "mod": m,
            })
    
    print(f"Total differing lines: {len(diffs)}")
    print()
    
    # Group by DXF code
    by_code = {}
    for d in diffs:
        by_code.setdefault(d["code"], []).append(d)
    
    print("Diffs by DXF code:")
    for code in sorted(by_code.keys()):
        entries = by_code[code]
        print(f"\n  Code {code} ({len(entries)} diffs):")
        for e in entries[:10]:
            print(f"    Line {e['line']}: orig='{e['orig'][:60]}' → mod='{e['mod'][:60]}'")
        if len(entries) > 10:
            print(f"    ... and {len(entries)-10} more")
    
    # Check if all diffs are code 1 (attribute values)
    code_1_diffs = by_code.get("1", [])
    other_diffs = {k: v for k, v in by_code.items() if k != "1"}
    
    print(f"\n\nSummary:")
    print(f"  Code 1 (attribute value) diffs: {len(code_1_diffs)}")
    print(f"  Other code diffs: {sum(len(v) for v in other_diffs.values())}")
    
    if other_diffs:
        print(f"\n  ⚠️  Non-attribute-value diffs found!")
        for code, entries in other_diffs.items():
            print(f"    Code {code}: {len(entries)} diffs")
            for e in entries[:3]:
                print(f"      Line {e['line']}: orig='{e['orig'][:60]}' → mod='{e['mod'][:60]}'")
    
    # Show what the original template's placeholder values are
    print(f"\n\nOriginal template attribute values (code 1 near $(TEMPLATE) or $(...)):")
    for i in range(len(orig_lines) - 1):
        if orig_lines[i].strip() == "1" and i+1 < len(orig_lines):
            val = orig_lines[i+1].strip()
            if val.startswith("$(") or val.startswith("$"):
                # Get context
                for j in range(i, min(i+10, len(orig_lines)-1)):
                    if orig_lines[j].strip() == "2" and j+1 < len(orig_lines):
                        attr_name = orig_lines[j+1].strip()
                        print(f"  {attr_name} = '{val}'")
                        break

if __name__ == "__main__":
    main()