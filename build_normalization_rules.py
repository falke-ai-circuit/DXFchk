#!/usr/bin/env python3
"""
Analyze the saved template_analysis_full.json data to build normalization rules.
Focus on finding the ATTRIBUTE NAME (code 2 near each code 1 diff) to map
each template placeholder value to its attribute.
"""
import json
from collections import defaultdict

with open("/opt/data/dxfchk/template_analysis_full.json") as f:
    data = json.load(f)

all_diffs = data["all_diffs"]
group_summaries = data["group_summaries"]

# We already know the patterns. Let me build the normalization rules.
# For each diff, we know: line, code, orig (template value), mod (module value),
# group (template name), module_id, file

# RULE 1: Template name → Module ID replacement
# Pattern: template_name appears in orig, replaced by module_id in mod
# This is a DIRECT replacement — the template value contains the template name,
# and the module replaces it with the module ID.
# To normalize a module back to a template: replace module_id with template_name

# RULE 2: Layer changes (code 8)
# Pattern: template has layer X, module has layer Y
# To normalize: replace module layer Y with template layer X
layer_patterns = data.get("layer_patterns", {})

# RULE 3: Value changes — these need attribute names to understand.
# From the analysis, we know these categories:
# 3a. Empty template value → module fills with specific value (e.g., date, reference)
# 3b. Keyword template value → module replaces with specific value (PROJECT, TEMPLATE, DISPLAY)
# 3c. AP code → module-specific area code
# 3d. DEVICETAG → module-specific device tag
# 3e. Text descriptions → module-specific I/O names
# 3f. Numeric values → module-specific parameters

# The key question: can we normalize these back?
# For 3a (empty): Set back to empty string
# For 3b (keyword): Set back to the keyword (PROJECT, TEMPLATE, etc.)
# For 3c (AP code): This is harder — we need to know the original AP code
# For 3d (DEVICETAG): Set back to DEVICETAGn
# For 3e (text): Cannot normalize without knowing the original template value
# For 3f (numeric): Cannot normalize without knowing the original template value

# BUT: if we have the original template, we can extract all these values directly.
# For _modN groups where we DON'T have the original template, we need to infer them.

print("=== NORMALIZATION RULES (from 23 groups, 25,860 diffs) ===\n")

# Rule 1: Name replacement
name_repl = [d for d in all_diffs if str(d.get("code")) == "1" and d.get("template_name", "") in str(d.get("orig", ""))]
print(f"Rule 1: Template Name → Module ID")
print(f"  Count: {len(name_repl)} diffs")
print(f"  Pattern: In code-1 values, replace template_name with module_id")
print(f"  Example: 'AMR02p3.PL' → '602.5000.0006.PL'")
print(f"  Reverse (module→template): Replace module_id with template_name in all code-1 values")
print()

# Rule 2: Layer changes
print(f"Rule 2: Layer Changes (code 8)")
total_layer = sum(int(v) for v in layer_patterns.values())
print(f"  Count: {total_layer} diffs")
for key, count in sorted(layer_patterns.items(), key=lambda x: -int(x[1])):
    orig, mod = key.split("|", 1)
    print(f"  '{orig}' → '{mod}': {count} times")
print(f"  Reverse (module→template): Replace module layers with template layers")
print()

# Rule 3: Value changes
other_vals = [d for d in all_diffs if str(d.get("code")) == "1" and d.get("template_name", "") not in str(d.get("orig", ""))]
print(f"Rule 3: Value Changes (non-name, code 1)")
print(f"  Count: {len(other_vals)} diffs")

# Group by template (original) value to find the fixed placeholder patterns
by_orig = defaultdict(list)
for d in other_vals:
    by_orig[str(d.get("orig", ""))].append(d)

# These are the values that are CONSISTENT across templates (the "placeholder" values)
# If a template value is always the same across different groups, it's a fixed placeholder
print(f"  Unique template values: {len(by_orig)}")
print()

# Categorize
categories = {
    "fixed_placeholder": [],   # Values that appear as placeholders (empty, keywords, DEVICETAG)
    "generic_reference": [],  # Values that are generic references (AP01, etc.)
    "design_specific": [],    # Values that are specific to each template's design
}

for orig, diffs in by_orig.items():
    groups = set(d.get("group", "") for d in diffs)
    if orig in ("", " ", "PROJECT", "TEMPLATE", "DISPLAY", "INSCODE", "MC", "MI"):
        categories["fixed_placeholder"].append((orig, diffs, groups))
    elif orig.startswith("DEVICETAG") or orig.startswith("AP") and len(orig) <= 6:
        categories["generic_reference"].append((orig, diffs, groups))
    else:
        categories["design_specific"].append((orig, diffs, groups))

print(f"  Fixed placeholders (can normalize): {len(categories['fixed_placeholder'])}")
for orig, diffs, groups in categories["fixed_placeholder"]:
    mod_samples = list(set(str(d.get("mod", ""))[:50] for d in diffs[:5]))
    print(f"    T='{orig}' ({len(diffs)}x in {len(groups)} groups) → M='{mod_samples[0] if mod_samples else '?'}'")

print(f"\n  Generic references (can normalize if we know the pattern): {len(categories['generic_reference'])}")
for orig, diffs, groups in categories["generic_reference"]:
    mod_samples = list(set(str(d.get("mod", ""))[:50] for d in diffs[:5]))
    print(f"    T='{orig}' ({len(diffs)}x in {len(groups)} groups)")
    for m in mod_samples[:3]:
        print(f"      → M='{m}'")

print(f"\n  Design-specific (harder to normalize): {len(categories['design_specific'])}")
for orig, diffs, groups in categories["design_specific"][:10]:
    mod_samples = list(set(str(d.get("mod", ""))[:50] for d in diffs[:5]))
    print(f"    T='{orig[:50]}' ({len(diffs)}x in {len(groups)} groups)")
    for m in mod_samples[:2]:
        print(f"      → M='{m}'")

# KEY INSIGHT: Can we create a template from a module WITHOUT the original?
print(f"\n{'='*60}")
print(f"STRATEGY FOR CREATING TEMPLATES FROM MODULES")
print(f"{'='*60}")
print(f"""
Given a module DXF, to create a template:

1. CHANGE $(TEMPLATE) attribute value → new template name (already implemented)

2. REPLACE module_id → template_name in all code-1 values
   - Module ID = derived from filename: XXX_pYYYY_pZZZZ → XXX.YYYY.ZZZZ
   - Template name = the new template name
   - Pattern: Replace all occurrences of module_id string with template_name

3. NORMALIZE LAYERS (code 8):
   - Replace 'N_COM_HIDDEN' → '0' (where N = 1,2,3,4)
   - Replace 'N_COM_EVAL_FALSE' → 'N_COM_HIDDEN' (wait, or → '0'?)
   - Actually: template has '0', module has 'N_COM_HIDDEN' or 'N_COM_EVAL_FALSE'
   - Need to reverse: module 'N_COM_HIDDEN' → template '0'
   - And: module 'N_COM_EVAL_FALSE' → template 'N_COM_HIDDEN' → '0'?
   - Looking at the data: '0' → '1_COM_HIDDEN' and '1_COM_HIDDEN' → '1_COM_EVAL_FALSE'
   - So the chain is: 0 (template) → 1_COM_HIDDEN → 1_COM_EVAL_FALSE (module)
   - Reverse: 1_COM_EVAL_FALSE → 1_COM_HIDDEN → 0? Or directly → 0?
   - From the data, the TEMPLATE has '0', so both module layers should go back to '0'
   - BUT: some modules have '1_COM_HIDDEN' and some have '1_COM_EVAL_FALSE'
   - The template has '0' for BOTH cases
   - So: normalize ALL *_COM_* layers back to '0'

4. NORMALIZE FIXED PLACEHOLDER VALUES (code 1):
   - Empty values: already empty in template → leave as-is
   - 'PROJECT' → module has project number → can't normalize without original
   - 'TEMPLATE' → module has system type → can't normalize without original
   - 'DISPLAY' → module has display number → can't normalize without original
   - DEVICETAGn → module has specific tag → normalize back to 'DEVICETAGn'
   - AP01/AP02 → module has specific AP code → normalize back to 'AP01'?

5. DESIGN-SPECIFIC VALUES: These are the hardest.
   - Text descriptions (I/O names, equipment names)
   - Numeric parameters
   - These vary per module and CANNOT be normalized without the original template
   - OPTION: Leave them as-is (they'll be module-specific in the template)
   - OPTION: Try to find patterns and replace with generic placeholders

CONCLUSION:
- Rules 1-3 are fully automatable (name, $(TEMPLATE), layers)
- Rule 4 (DEVICETAG) is partially automatable
- Rule 5 (design-specific) requires either:
  a) The original template to copy values from
  b) Cross-module analysis to find common patterns
  c) User manual intervention

FOR _modN GROUPS (no original template):
- Apply rules 1-3 automatically
- For rule 4, normalize DEVICETAG → DEVICETAGn pattern
- For rule 5, use the FIRST module in the group as reference
  (all modules in same group have same structure, so design-specific
   values from one module can serve as template defaults)
""")

# Check: within a group, are the "other value" changes consistent across modules?
# If yes, we can use any module to create the template
print(f"{'='*60}")
print(f"CONSISTENCY CHECK: Are design-specific values consistent within groups?")
print(f"{'='*60}")
# We only have first-file data per group, but let's check if the same template
# values appear across different groups
cross_group = defaultdict(set)
for orig, diffs, groups in categories["design_specific"]:
    for g in groups:
        cross_group[orig].add(g)

multi_group = {k: v for k, v in cross_group.items() if len(v) > 1}
print(f"Design-specific values appearing in multiple groups: {len(multi_group)}")
for orig, groups in sorted(multi_group.items(), key=lambda x: -len(x[1]))[:10]:
    print(f"  '{orig[:50]}' → {len(groups)} groups: {list(groups)[:3]}")