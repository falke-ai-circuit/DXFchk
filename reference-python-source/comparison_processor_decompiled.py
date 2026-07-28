"""
Comparison Processor Module

Contains functions and classes for comparing DXF files against templates
and organizing them based on differences.
"""

import os
import re
import shutil
import threading
import queue
import time
from collections import defaultdict

from dxf_processor import get_blocks_lines_polylines_from_dxf, compare_dict_of_lists, create_content_hash
from template_manager import get_template_name_fast, load_template_map_from_cache, save_template_map_to_cache
from utils import sanitize_filename, copy_file, move_file


class ComparisonProcessor:
    """
    Class responsible for DXF file comparison processes.
    Separates the business logic from the GUI layer.
    """

    def __init__(self, template_map, output_folder, move_files=True, group_by_content=True, log_callback=None):
        """
        Initialize the comparison processor.
        
        Args:
            template_map (dict): Dictionary mapping template names to template file paths
            output_folder (str): Folder to store results
            move_files (bool): Whether to move files instead of copying
            group_by_content (bool): Whether to group files by content differences
            log_callback (callable): Optional callback for logging messages
        """
        self.template_map = template_map
        self.output_folder = output_folder
        self.move_files = move_files
        self.group_by_content = group_by_content
        self.log_callback = log_callback

        os.makedirs(os.path.join(output_folder, "notemplate"), exist_ok=True)

        self.template_file_hashes = defaultdict(lambda: defaultdict(list))
        self.mod_folders = set()

        self.template_direct_copies = defaultdict(list)

        self.template_detailed_logs = defaultdict(list)

    def _log(self, message):
        """
        Helper method to log messages if callback is provided
        """
        if self.log_callback:
            self.log_callback(message)

    def process_file(self, dxf_file, check_stop=None):
        """
        Process a single DXF file, comparing it to templates.
        
        Args:
            dxf_file (str): Path to the DXF file to process
            check_stop (callable): Optional function to check if processing should stop
        
        Returns:
            str: 'match', 'different', or 'no_template' indicating the result
        """
        try:
            if check_stop and check_stop():
                return None

            filename = os.path.basename(dxf_file)
            self._log(f"Processing: {filename}")

            template_name = get_template_name_fast(dxf_file)

            if not template_name:
                base_name = os.path.basename(dxf_file)

                best_match = None
                for name in self.template_map:
                    if not name or not name.strip():
                        continue

                    if base_name.lower().startswith(name.lower()):
                        if not best_match or len(name) > len(best_match):
                            best_match = name

                template_name = best_match

            if not template_name or template_name not in self.template_map:
                self._handle_no_template_file(dxf_file)
                return "no_template"

            template_file = self.template_map[template_name]

            if self._files_are_identical(dxf_file, template_file):
                self._handle_matching_file(dxf_file, template_name)
                return "match"

            self._handle_different_file(dxf_file, template_file, template_name)
            return "different"
        except Exception as e:
            self._log(f"Error processing file {dxf_file}: {str(e)}")
            return "error"

    def _files_are_identical(self, dxf_file, template_file):
        """
        Check if two DXF files are identical.
        
        Args:
            dxf_file (str): Path to the DXF file to check
            template_file (str): Path to the template file to compare against
        
        Returns:
            bool: True if files are identical, False otherwise
        """
        try:
            blocks_dict, lines_dict, polylines_dict = get_blocks_lines_polylines_from_dxf(dxf_file)
            template_blocks, template_lines, template_polylines = get_blocks_lines_polylines_from_dxf(template_file)

            common_blocks, only_in_file, only_in_template, diff_blocks = compare_dict_of_lists(
                blocks_dict, template_blocks
            )
            common_lines, only_in_file_lines, only_in_template_lines, diff_lines = compare_dict_of_lists(
                lines_dict, template_lines
            )
            common_poly, only_in_file_poly, only_in_template_poly, diff_poly = compare_dict_of_lists(
                polylines_dict, template_polylines
            )

            identical = (
                not only_in_file and
                not only_in_template and
                not diff_blocks and
                not only_in_file_lines and
                not only_in_template_lines and
                not diff_lines and
                not only_in_file_poly and
                not only_in_template_poly and
                not diff_poly
            )

            return identical
        except Exception as e:
            self._log(f"Error comparing files: {str(e)}")
            return False

    def _handle_no_template_file(self, dxf_file):
        """
        Handle a file with no matching template.
        
        Args:
            dxf_file (str): Path to the DXF file
        """
        filename = os.path.basename(dxf_file)
        self._log(f"Processing: {filename}")
        self._log(f"  -> No template found for {filename}")

        target_path = os.path.join(
            self.output_folder, "notemplate", filename
        )

        copy_file(dxf_file, target_path)

    def _handle_matching_file(self, dxf_file, template_name):
        """
        Handle a file that matches its template.
        
        Args:
            dxf_file (str): Path to the DXF file
            template_name (str): Name of the matching template
        """
        filename = os.path.basename(dxf_file)
        self._log(f"Processing: {filename}")
        self._log(f"  -> Using template: {template_name}")
        self._log(f"  -> MATCH: {filename} is identical to template")

        sanitized_template_name = sanitize_filename(template_name)

        template_dir = os.path.join(self.output_folder, sanitized_template_name)
        os.makedirs(template_dir, exist_ok=True)

        target_path = os.path.join(template_dir, filename)

        self.template_direct_copies[sanitized_template_name].append(filename)

        copy_file(dxf_file, target_path)

    def _handle_different_file(self, dxf_file, template_file, template_name):
        """
        Handle a file that is different from its template.
        
        Args:
            dxf_file (str): Path to the DXF file
            template_file (str): Path to the template file
            template_name (str): Name of the template
        """
        self._compare_with_template(dxf_file, template_file, template_name)

    def _compare_with_template(self, dxf_path, template_path, template_name):
        """
        Compare a DXF file with its template.
        
        Args:
            dxf_path (str): Path to the DXF file
            template_path (str): Path to the template file
            template_name (str): Name of the template
        
        Returns:
            str: Result type - 'match' or 'different'
        """
        filename = os.path.basename(dxf_path)

        detailed_log = []
        detailed_log.append("========== DETAILED COMPARISON LOG ==========")
        detailed_log.append(f"File: {filename}")
        detailed_log.append(f"Template: {template_name} ({os.path.basename(template_path)})")
        detailed_log.append(f"Comparison Time: {time.strftime('%Y-%m-%d %H:%M:%S')}")
        detailed_log.append("==========================================\n")

        try:
            blocks_dict, lines_dict, polylines_dict = get_blocks_lines_polylines_from_dxf(dxf_path)
            template_blocks, template_lines, template_polylines = get_blocks_lines_polylines_from_dxf(template_path)

            if not any([blocks_dict, lines_dict, polylines_dict]) or not any([template_blocks, template_lines, template_polylines]):
                error_msg = f"  -> Error reading DXF data from {filename} or template"
                self._log(error_msg)
                detailed_log.append(error_msg)
                detailed_log.append("Processing failed - could not read DXF data")
                self.template_detailed_logs[template_name].append((filename, "\n".join(detailed_log), None))
                return "no_template"
        except Exception as e:
            error_msg = f"  -> Error processing DXF: {str(e)}"
            self._log(error_msg)
            detailed_log.append(error_msg)
            detailed_log.append(f"Processing failed - exception occurred: {str(e)}")
            self.template_detailed_logs[template_name].append((filename, "\n".join(detailed_log), None))
            return "no_template"

        detailed_log.append("1. ENTITY COUNTS")
        detailed_log.append(f"  File Blocks: {sum(len(items) for items in blocks_dict.values())} in {len(blocks_dict)} block types")
        detailed_log.append(f"  Template Blocks: {sum(len(items) for items in template_blocks.values())} in {len(template_blocks)} block types")
        detailed_log.append(f"  File Lines: {sum(len(items) for items in lines_dict.values())} in {len(lines_dict)} layers")
        detailed_log.append(f"  Template Lines: {sum(len(items) for items in template_lines.values())} in {len(template_lines)} layers")
        detailed_log.append(f"  File Polylines: {sum(len(items) for items in polylines_dict.values())} in {len(polylines_dict)} layers")
        detailed_log.append(f"  Template Polylines: {sum(len(items) for items in template_polylines.values())} in {len(template_polylines)} layers")
        detailed_log.append("")

        comparison_results = {}

        common_blocks, only_in_file, only_in_template, diff_blocks = compare_dict_of_lists(
            blocks_dict, template_blocks
        )
        common_lines, only_in_file_lines, only_in_template_lines, diff_lines = compare_dict_of_lists(
            lines_dict, template_lines
        )
        common_poly, only_in_file_poly, only_in_template_poly, diff_poly = compare_dict_of_lists(
            polylines_dict, template_polylines
        )

        comparison_results = {
            'blocks': {
                'common': common_blocks,
                'only_in_1': only_in_file,
                'only_in_2': only_in_template,
                'diff': diff_blocks
            },
            'lines': {
                'common': common_lines,
                'only_in_1': only_in_file_lines,
                'only_in_2': only_in_template_lines,
                'diff': diff_lines
            },
            'polylines': {
                'common': common_poly,
                'only_in_1': only_in_file_poly,
                'only_in_2': only_in_template_poly,
                'diff': diff_poly
            }
        }

        detailed_log.append("2. BLOCK DIFFERENCES")
        if only_in_file:
            detailed_log.append("  Block types only in file:")
            for block_name in sorted(only_in_file):
                detailed_log.append(f"    - {block_name} ({len(blocks_dict[block_name])} instances)")
        else:
            detailed_log.append("  No block types unique to file")

        if only_in_template:
            detailed_log.append("  Block types only in template:")
            for block_name in sorted(only_in_template):
                detailed_log.append(f"    - {block_name} ({len(template_blocks[block_name])} instances)")
        else:
            detailed_log.append("  No block types unique to template")

        if diff_blocks:
            detailed_log.append("  Blocks with differences in common block types:")
            for block_name, diff_data in sorted(diff_blocks.items()):
                only_in_file_count = len(diff_data.get("only_in_1", []))
                only_in_template_count = len(diff_data.get("only_in_2", []))
                detailed_log.append(f"    - {block_name}:")
                detailed_log.append(f"      * {only_in_file_count} instances only in file")
                detailed_log.append(f"      * {only_in_template_count} instances only in template")
                detailed_log.append(f"      * {len(diff_data.get('common', []))} common instances")
        else:
            detailed_log.append("  No differences in common block types")

        detailed_log.append("")
        detailed_log.append("3. LINE DIFFERENCES")
        if only_in_file_lines:
            detailed_log.append("  Line layers only in file:")
            for layer_name in sorted(only_in_file_lines):
                detailed_log.append(f"    - {layer_name} ({len(lines_dict[layer_name])} lines)")
        else:
            detailed_log.append("  No line layers unique to file")

        if only_in_template_lines:
            detailed_log.append("  Line layers only in template:")
            for layer_name in sorted(only_in_template_lines):
                detailed_log.append(f"    - {layer_name} ({len(template_lines[layer_name])} lines)")
        else:
            detailed_log.append("  No line layers unique to template")

        if diff_lines:
            detailed_log.append("  Lines with differences in common layers:")
            for layer_name, diff_data in sorted(diff_lines.items()):
                only_in_file_count = len(diff_data.get("only_in_1", []))
                only_in_template_count = len(diff_data.get("only_in_2", []))
                detailed_log.append(f"    - {layer_name}:")
                detailed_log.append(f"      * {only_in_file_count} lines only in file")
                detailed_log.append(f"      * {only_in_template_count} lines only in template")
                detailed_log.append(f"      * {len(diff_data.get('common', []))} common lines")
        else:
            detailed_log.append("  No differences in common line layers")

        detailed_log.append("")
        detailed_log.append("4. POLYLINE DIFFERENCES")
        if only_in_file_poly:
            detailed_log.append("  Polyline layers only in file:")
            for layer_name in sorted(only_in_file_poly):
                detailed_log.append(f"    - {layer_name} ({len(polylines_dict[layer_name])} polylines)")
        else:
            detailed_log.append("  No polyline layers unique to file")

        if only_in_template_poly:
            detailed_log.append("  Polyline layers only in template:")
            for layer_name in sorted(only_in_template_poly):
                detailed_log.append(f"    - {layer_name} ({len(template_polylines[layer_name])} polylines)")
        else:
            detailed_log.append("  No polyline layers unique to template")

        if diff_poly:
            detailed_log.append("  Polylines with differences in common layers:")
            for layer_name, diff_data in sorted(diff_poly.items()):
                only_in_file_count = len(diff_data.get("only_in_1", []))
                only_in_template_count = len(diff_data.get("only_in_2", []))
                detailed_log.append(f"    - {layer_name}:")
                detailed_log.append(f"      * {only_in_file_count} polylines only in file")
                detailed_log.append(f"      * {only_in_template_count} polylines only in template")
                detailed_log.append(f"      * {len(diff_data.get('common', []))} common polylines")
        else:
            detailed_log.append("  No differences in common polyline layers")

        has_differences = False
        for key, result in comparison_results.items():
            if result["only_in_1"] or result["only_in_2"] or result["diff"]:
                has_differences = True
                diff_msg = f"  -> Found differences in {key}: {len(result['diff'])} layer(s) with differences"
                self._log(diff_msg)

        sanitized_template_name = sanitize_filename(template_name)

        detailed_log.append("")
        detailed_log.append("5. COMPARISON SUMMARY")
        if has_differences:
            detailed_log.append(f"  RESULT: DIFFERENT - '{filename}' has differences from template")
            detailed_log.append("  File will be copied to template folder and organized into mod folders")

            content_hash = None
            if self.group_by_content:
                content_hash = create_content_hash(blocks_dict, lines_dict, polylines_dict)

            self.template_detailed_logs[template_name].append((filename, "\n".join(detailed_log), content_hash))
        else:
            detailed_log.append(f"  RESULT: MATCH - '{filename}' is identical to template")
            detailed_log.append("  File will be copied to the template folder")

            self.template_detailed_logs[template_name].append((filename, "\n".join(detailed_log), "template"))

        if not has_differences:
            template_dir = os.path.join(self.output_folder, sanitized_template_name)
            os.makedirs(template_dir, exist_ok=True)

            target_path = os.path.join(template_dir, os.path.basename(dxf_path))

            self._log(f"  -> MATCH: {filename} is identical to template")

            copy_file(dxf_path, target_path)

            return "match"
        else:
            template_dir = os.path.join(self.output_folder, sanitized_template_name)
            os.makedirs(template_dir, exist_ok=True)

            temp_target_path = os.path.join(template_dir, os.path.basename(dxf_path))

            self._log(f"  -> DIFFERENT: {filename} has differences from template")

            copy_file(dxf_path, temp_target_path)

            if self.group_by_content:
                content_hash = create_content_hash(blocks_dict, lines_dict, polylines_dict)
                self.template_file_hashes[template_name][content_hash].append({
                    'file_path': temp_target_path,
                    'file_name': os.path.basename(dxf_path)
                })
                self._log(f"  -> Grouped with content hash: {content_hash[:8]}")
            else:
                self.template_file_hashes[template_name]["default"].append({
                    'file_path': temp_target_path,
                    'file_name': os.path.basename(dxf_path)
                })

            return "different"

    def _save_detailed_logs(self):
        """
        Save detailed comparison logs for each template and mod folder
        """
        file_destinations = {}

        for template_name, hash_files in self.template_file_hashes.items():
            sanitized_template_name = sanitize_filename(template_name)

            sorted_hash_items = sorted(hash_files.items(), key=lambda x: x[0])

            for mod_idx, (hash_val, files) in enumerate(sorted_hash_items, 1):
                if not files:
                    continue

                mod_folder_name = f"{sanitized_template_name}_mod{mod_idx}"

                for file_info in files:
                    file_destinations[file_info["file_name"]] = mod_folder_name

        notemplate_folder_path = os.path.join(self.output_folder, "notemplate")
        if os.path.exists(notemplate_folder_path):
            log_file_path = os.path.join(notemplate_folder_path, "notemplate_dxfanalyze.log")

            try:
                with open(log_file_path, "w", encoding="utf-8") as f:
                    f.write("DETAILED COMPARISON LOG FOR: notemplate\n")
                    f.write(f"Created: {time.strftime('%Y-%m-%d %H:%M:%S')}\n")
                    f.write("Files in this folder have no matching template.\n")
                    f.write("=============================================\n\n")

                    files = [f for f in os.listdir(notemplate_folder_path) if f.lower().endswith(".dxf")]
                    f.write(f"Number of files without templates: {len(files)}\n\n")

                    if files:
                        f.write("Files without templates:\n")
                        for file in sorted(files):
                            f.write(f"- {file}\n")

                self._log(f"Saved detailed comparison log to: {log_file_path}")
            except Exception as e:
                self._log(f"Error saving detailed log for 'notemplate': {str(e)}")

        for template_name, logs in self.template_detailed_logs.items():
            if not logs:
                continue

            sanitized_template_name = sanitize_filename(template_name)

            folder_logs = defaultdict(list)

            for filename, log_content, destination_flag in logs:
                if destination_flag == "template":
                    folder_logs[sanitized_template_name].append(log_content)
                elif destination_flag is None:
                    continue

                if filename in file_destinations:
                    folder_name = file_destinations[filename]
                    folder_logs[folder_name].append(log_content)

            template_folder_path = os.path.join(self.output_folder, sanitized_template_name)
            if os.path.exists(template_folder_path):
                log_file_path = os.path.join(template_folder_path, f"{sanitized_template_name}_dxfanalyze.log")

                try:
                    with open(log_file_path, "w", encoding="utf-8") as f:
                        f.write(f"DETAILED COMPARISON LOG FOR: {sanitized_template_name}\n")
                        f.write(f"Created: {time.strftime('%Y-%m-%d %H:%M:%S')}\n")

                        direct_copies = self.template_direct_copies.get(sanitized_template_name, [])

                        file_count = len(folder_logs.get(sanitized_template_name, [])) + len(direct_copies)
                        f.write(f"Number of files compared: {file_count}\n")
                        f.write("=============================================\n\n")

                        if sanitized_template_name in folder_logs and folder_logs[sanitized_template_name]:
                            f.write("\n\n".join(folder_logs[sanitized_template_name]))

                        if direct_copies:
                            f.write("\n\n")

                        if direct_copies:
                            detailed_log_filenames = set()
                            for filename, _, _ in logs:
                                if filename in direct_copies:
                                    detailed_log_filenames.add(filename)

                            for filename in direct_copies:
                                if filename not in detailed_log_filenames:
                                    f.write("========== BASIC MATCH INFORMATION ==========\n")
                                    f.write(f"File: {filename}\n")
                                    f.write(f"Template: {template_name}\n")
                                    f.write(f"Comparison Time: {time.strftime('%Y-%m-%d %H:%M:%S')}\n")
                                    f.write("==========================================\n\n")
                                    f.write("RESULT: MATCH - File is identical to template\n\n")

                        if file_count == 0:
                            f.write("No files matched the template exactly.")

                    self._log(f"Saved detailed comparison log to: {log_file_path}")

                    if sanitized_template_name in folder_logs:
                        del folder_logs[sanitized_template_name]
                except Exception as e:
                    self._log(f"Error saving detailed log for '{sanitized_template_name}': {str(e)}")

            for folder_name, folder_log_contents in folder_logs.items():
                if not folder_log_contents:
                    continue

                folder_path = os.path.join(self.output_folder, folder_name)
                os.makedirs(folder_path, exist_ok=True)

                log_file_path = os.path.join(folder_path, f"{folder_name}_dxfanalyze.log")

                try:
                    with open(log_file_path, "w", encoding="utf-8") as f:
                        f.write(f"DETAILED COMPARISON LOG FOR: {folder_name}\n")
                        f.write(f"Created: {time.strftime('%Y-%m-%d %H:%M:%S')}\n")
                        f.write(f"Number of files compared: {len(folder_log_contents)}\n")
                        f.write("=============================================\n\n")

                        f.write("\n\n".join(folder_log_contents))

                    self._log(f"Saved detailed comparison log to: {log_file_path}")
                except Exception as e:
                    self._log(f"Error saving detailed log for '{folder_name}': {str(e)}")

    def finalize(self):
        """
        Finalize the comparison process by organizing mod folders.
        Files will be moved from template folders to mod folders to avoid duplicates.
        Also saves detailed comparison logs for each template.
        """
        self._log("Finalizing comparison and organizing mod folders...")

        self._save_detailed_logs()

        for template_name, hash_files in self.template_file_hashes.items():
            sanitized_template_name = sanitize_filename(template_name)
            self._log(f"Processing template: {template_name} with {len(hash_files)} different content groups")

            sorted_hash_items = sorted(hash_files.items(), key=lambda x: x[0])

            for mod_idx, (hash_val, files) in enumerate(sorted_hash_items, 1):
                if not files:
                    continue

                mod_folder_name = f"{sanitized_template_name}_mod{mod_idx}"
                mod_folder_path = os.path.join(self.output_folder, mod_folder_name)

                os.makedirs(mod_folder_path, exist_ok=True)

                self._log(f"  -> Using mod folder: {mod_folder_name} with {len(files)} files")

                for file_info in files:
                    if not os.path.exists(file_info["file_path"]):
                        continue

                    target_path = os.path.join(mod_folder_path, file_info["file_name"])

                    try:
                        move_file(file_info["file_path"], target_path)
                        self._log(f"    -> Moved {file_info['file_name']} from template folder to {mod_folder_name}")
                    except Exception as e:
                        self._log(f"    -> Error moving {file_info['file_name']}: {str(e)}")

                self.mod_folders.add(mod_folder_path)

        self._ensure_all_folders_have_logs()

        self._log(f"Created {len(self.mod_folders)} mod folders in total")

        empty_hash_groups = 0
        for template_name, hash_files in self.template_file_hashes.items():
            for hash_val, files in hash_files.items():
                if not files:
                    empty_hash_groups += 1

        if empty_hash_groups > 0:
            self._log(f"Note: {empty_hash_groups} empty hash groups were skipped")

        return

    def _ensure_all_folders_have_logs(self):
        """
        Ensure all folders in the output directory have log files
        """
        for folder_name in os.listdir(self.output_folder):
            folder_path = os.path.join(self.output_folder, folder_name)

            if not os.path.isdir(folder_path) or folder_name == "notemplate":
                continue

            log_file_path = os.path.join(folder_path, f"{folder_name}_dxfanalyze.log")

            if os.path.exists(log_file_path):
                continue

            dxf_files = [f for f in os.listdir(folder_path) if f.lower().endswith(".dxf")]

            if not dxf_files:
                continue

            self._log(f"Creating missing log file for folder: {folder_name}")

            try:
                with open(log_file_path, "w", encoding="utf-8") as f:
                    f.write(f"DETAILED COMPARISON LOG FOR: {folder_name}\n")
                    f.write(f"Created: {time.strftime('%Y-%m-%d %H:%M:%S')}\n")
                    f.write(f"Number of files: {len(dxf_files)}\n")
                    f.write("=============================================\n\n")
                    f.write("DXF FILES IN THIS FOLDER:\n")
                    for file in sorted(dxf_files):
                        f.write(f"- {file}\n")
                    f.write("\n\nNote: This is an automatically generated log for a folder that was missing a detailed log file.\n")
                    f.write("For folders like BI001, BI001p1, BO001p6, etc., detailed analysis was not recorded during processing.\n")

                self._log(f"Saved basic log file to: {log_file_path}")
            except Exception as e:
                self._log(f"Error processing folder '{folder_name}': {str(e)}")

    def get_mod_folder_count(self):
        """
        Get the number of created mod folders.
        
        Returns:
            int: Number of mod folders created
        """
        return len(self.mod_folders)


def run_comparison(template_map, search_folder, output_folder, recursive=True, move_files=True, group_by_content=True, progress_callback=None, log_callback=None):
    """
    Compare DXF files in the search folder against templates and organize them.
    
    File Organization Process:
    1. All original files in the search folder are preserved (never deleted or moved)
    2. Files are copied to the output folder structure:
       - Files without a template → "notemplate" folder
       - Files that match their template → template folder (e.g., "Template1")
       - Files with differences → first copied to template folder, then moved to mod folders
    3. Files with differences are organized into mod folders (e.g., "Template1_mod1")
       to avoid having duplicate copies in both template and mod folders
    
    Args:
        template_map (dict): Dictionary mapping template names to template file paths
        search_folder (str): Folder containing DXF files to compare
        output_folder (str): Folder to store results
        recursive (bool): Whether to search subdirectories
        move_files (bool): DEPRECATED - Files in search folder are always preserved
        group_by_content (bool): Whether to group files by content differences
        progress_callback (callable): Function to call with progress updates
        log_callback (callable): Function to call with log messages
        
    Returns:
        dict: Results of the comparison process
    """
    if log_callback:
        log_callback("Note: Original files in search folder will be preserved")
        log_callback("Note: Files with differences will be moved from template folders to mod folders to avoid duplicates")
        log_callback("Note: Detailed comparison logs will be saved for each template")

    processor = ComparisonProcessor(
        template_map=template_map,
        output_folder=output_folder,
        move_files=False,
        group_by_content=group_by_content,
        log_callback=log_callback
    )

    results = {
        "total_files": 0,
        "matched_files": 0,
        "different_files": 0,
        "no_template_files": 0,
        "mod_folder_count": 0
    }

    dxf_files = []
    if recursive:
        for root, _, files in os.walk(search_folder):
            for file in files:
                if file.lower().endswith(".dxf"):
                    dxf_files.append(os.path.join(root, file))
    else:
        dxf_files = [os.path.join(search_folder, f) for f in os.listdir(search_folder) if f.lower().endswith(".dxf")]

    if log_callback:
        log_callback(f"Found {len(dxf_files)} DXF files to process")

    results["total_files"] = len(dxf_files)

    for i, dxf_file in enumerate(dxf_files):
        if progress_callback:
            progress_callback(i, len(dxf_files))

        try:
            result = processor.process_file(dxf_file)

            if result == "match":
                results["matched_files"] += 1
            elif result == "different":
                results["different_files"] += 1
            elif result == "no_template":
                results["no_template_files"] += 1
        except Exception as e:
            log_message = f"Error processing file {dxf_file}: {str(e)}"
            if log_callback:
                log_callback(log_message)

    processor.finalize()
    results["mod_folder_count"] = processor.get_mod_folder_count()

    if log_callback:
        processed_count = results["matched_files"] + results["different_files"] + results["no_template_files"]
        log_callback(f"Processing complete. Processed {processed_count} of {len(dxf_files)} files.")

        if processed_count < len(dxf_files):
            log_callback(f"Warning: {len(dxf_files) - processed_count} files may have been skipped due to errors.")

    if progress_callback:
        progress_callback(len(dxf_files), len(dxf_files))

    return results