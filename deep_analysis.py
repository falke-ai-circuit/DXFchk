#!/usr/bin/env python3
"""
Deep analysis of template/module diffs to find normalization rules.
The goal: identify the EXACT set of changes DNA Explorer makes when creating
a module from a template, so we can reverse them to create templates from modules.
"""
import json
from collections import defaultdict

with open("/opt/data/dxfchk/template_analysis_full.json") as f:
    data = json.load(f)

group_summaries = data["group_summaries"]
all_diffs = data["all_diffs"]

print(f"Groups analyzed: {len(group_summaries)}")
print(f"Total diffs: {len(all_diffs)}")
print()

# Groups that were IDENTICAL (no diffs)
identical_groups = [s for s in group_summaries if s["total_diffs"] == 0]
diff_groups = [s for s in group_summaries if s["total_diffs"] > 0]
print(f"Identical (0 diffs): {len(identical_groups)}")
for s in identical_groups:
    print(f"  {s['group']} ({s['files_in_group']} files)")
print()
print(f"With diffs: {len(diff_groups)}")

# Categorize all code-1 diffs
all_code_1 = [d for d in all_diffs if str(d.get("code")) == "1"]
all_code_8 = [d for d in all_diffs if str(d.get("code")) == "8"]

# NAME replacements: template_name appears in orig value
name_repl = [d for d in all_code_1 if d.get("template_name", "") in str(d.get("orig", ""))]
other_vals = [d for d in all_code_1 if d.get("template_name", "") not in str(d.get("orig", ""))]

print(f"\nCode 1 total: {len(all_code_1)}")
print(f"  Name replacements: {len(name_repl)}")
print(f"  Other value changes: {len(other_vals)}")

# Analyze name replacement patterns
print(f"\n{'='*60}")
print(f"NAME REPLACEMENT PATTERNS")
print(f"{'='*60}")

name_patterns = defaultdict(int)
for d in name_repl:
    orig = str(d.get("orig", ""))
    mod = str(d.get("mod", ""))
    template_name = d.get("template_name", "")
    module_id = d.get("module_id", "")
    
    # What pattern does the replacement follow?
    # Replace template_name with module_id
    if orig.replace(template_name, module_id) == mod:
        name_patterns["direct_replace"] += 1
    elif orig.replace(template_name, module_id, 1) == mod:
        name_patterns["first_replace"] += 1
    else:
        # Check if it's a prefix/suffix pattern
        if orig.startswith(template_name):
            name_patterns["prefix"] += 1
        elif f":{template_name}" in orig:
            name_patterns["colon_prefix"] += 1
        elif f".{template_name}" in orig:
            name_patterns["dot_prefix"] += 1
        else:
            name_patterns["other"] += 1

for pattern, count in sorted(name_patterns.items(), key=lambda x: -x[1]):
    print(f"  {pattern}: {count}")

# Show sample name replacements
print(f"\nSample name replacements:")
seen = set()
for d in name_repl[:50]:
    key = (str(d.get("orig",""))[:50], str(d.get("mod",""))[:50])
    if key not in seen:
        seen.add(key)
        print(f"  '{d.get('orig','')[:60]}' → '{d.get('mod','')[:60]}' [group={d['group']}]")
        if len(seen) >= 20:
            break

# Analyze OTHER value changes (non-name replacements)
print(f"\n{'='*60}")
print(f"OTHER VALUE CHANGES (non-name replacements)")
print(f"{'='*60}")
print(f"Total: {len(other_vals)}")

# Group by template (original) value
by_orig = defaultdict(list)
for d in other_vals:
    by_orig[str(d.get("orig", ""))].append(d)

print(f"Unique original (template) values: {len(by_orig)}")

# Categorize these values
categories = defaultdict(list)
for orig, diffs in by_orig.items():
    # What kind of value is this?
    if orig.startswith("$(") or orig.startswith("$"):
        categories["placeholder"].append((orig, diffs))
    elif orig == "" or orig == " ":
        categories["empty"].append((orig, diffs))
    elif orig in ("TEMPLATE", "PROJECT", "DISPLAY", "INSCODE", "MC", "MI"):
        categories["keyword"].append((orig, diffs))
    elif orig.startswith("AP") and len(orig) <= 6:
        categories["ap_code"].append((orig, diffs))
    elif orig.startswith("pr:"):
        categories["pr_prefix"].append((orig, diffs))
    elif "." in orig and all(c.isdigit() or c == "." for c in orig):
        categories["numeric"].append((orig, diffs))
    elif orig.startswith("DEVICETAG"):
        categories["devicetag"].append((orig, diffs))
    else:
        categories["text"].append((orig, diffs))

print(f"\nCategories:")
for cat, entries in sorted(categories.items(), key=lambda x: -len(x[1])):
    print(f"  {cat}: {len(entries)} unique values, {sum(len(d) for _, d in entries)} total diffs")

# Show each category
for cat in ["placeholder", "empty", "keyword", "ap_code", "pr_prefix", "devicetag", "numeric", "text"]:
    entries = categories.get(cat, [])
    if not entries:
        continue
    print(f"\n--- {cat} ({len(entries)} unique) ---")
    for orig, diffs in entries[:15]:
        mod_samples = list(set(str(d.get("mod", ""))[:50] for d in diffs[:5]))
        print(f"  T='{orig[:50]}' ({len(diffs)}x)")
        for m in mod_samples[:3]:
            print(f"    → M='{m}'")

# KEY QUESTION: which template values are PLACEHOLDERS (start with $)?
print(f"\n{'='*60}")
print(f"PLACEHOLDER ATTRIBUTES ($(...)")
print(f"{'='*60}")

# Search all original template values for $ patterns
dollar_values = set()
for d in all_diffs:
    orig = str(d.get("orig", ""))
    if "$(" in orig or orig.startswith("$"):
        dollar_values.add(orig)
    mod = str(d.get("mod", ""))
    if "$(" in mod or mod.startswith("$"):
        dollar_values.add(mod)

print(f"Values containing $: {len(dollar_values)}")
for v in sorted(dollar_values):
    print(f"  '{v}'")

# KEY QUESTION: what are the attribute NAMES (code 2) for each changed value?
# We need to look at the context around each code-1 diff
print(f"\n{'='*60}")
print(f"ATTRIBUTE NAME → VALUE MAPPING")
print(f"{'='*60}")

# For each diff, the attribute name is the code-2 value that follows
# We need to parse the raw DXF context. But we stored line numbers.
# The attribute name (code 2) comes AFTER code 1 in ATTDEF/ATTRIB entities.
# Pattern: code 1 = value, code 2 = attribute name

# Let's group diffs by their position and find nearby code-2 values
# For now, just show which groups had identical modules
print(f"\n{'='*60}")
print(f"IDENTICAL GROUPS (module = template, 0 diffs)")
print(f"{'='*60}")
for s in identical_groups:
    print(f"  {s['group']} ({s['files_in_group']} files)")

# SUMMARY: normalization rules
print(f"\n{'='*60}")
print(f"REVERSE-ENGINEERED NORMALIZATION RULES")
print(f"{'='*60}")
print(f"""
Rule 1: TEMPLATE NAME → MODULE ID
  When DNA Explorer creates a module from a template, it replaces the template
  name with the module ID everywhere it appears in attribute values (code 1).
  Pattern: '{name_repl[0].get("template_name","")}' → '{name_repl[0].get("module_id","")}'
  Examples:""")
for d in name_repl[:8]:
    print(f"    '{d.get('orig','')[:60]}' → '{d.get('mod','')[:60]}'")

print(f"""
Rule 2: LAYER CHANGES (code 8)
  Template layers are modified when module is created.
  Top patterns:""")
layer_patterns = data.get("layer_patterns", {})
for key, count in sorted(layer_patterns.items(), key=lambda x: -x[1])[:10]:
    orig, mod = key.split("|", 1)
    print(f"    '{orig}' → '{mod}': {count} times")

print(f"""
Rule 3: VALUE CHANGES (non-name)
  {len(other_vals)} changes across {len(by_orig)} unique template values.
  These are module-specific implementation values that vary per module.
  
  Key categories:
  - Empty → filled: template has empty values, module fills them
  - Keywords (TEMPLATE, PROJECT, DISPLAY): template has generic keywords, module has specific values  
  - AP codes (AP01, AP02...): template has generic area code, module has specific area
  - Text descriptions: template has generic descriptions, module has specific I/O names
  
  These are NOT simple replacements — they are design-specific values that
  each module fills with its own implementation data.
""")

# Count of groups where diffs are ONLY name + layer (no other values)
only_name_layer = []
for s in diff_groups:
    group_diffs = [d for d in all_diffs if d.get("group") == s["group"]]
    has_other = any(str(d.get("code")) == "1" and d.get("template_name","") not in str(d.get("orig","")) for d in group_diffs)
    if not has_other:
        only_name_layer.append(s["group"])

print(f"Groups with ONLY name+layer changes (no value changes): {len(only_name_layer)}")
for g in only_name_layer:
    print(f"  {g}")