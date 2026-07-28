#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
DXF Processing Module

Contains functions for processing DXF files, extracting geometry data,
and creating content hashes.
"""

import ezdxf
import hashlib
import json

def get_blocks_lines_polylines_from_dxf(dxf_path, decimals=3, consider_polyline_reversal_as_same=True, cached_results=None):
    """
    Reads a DXF file with ezdxf, returning three dictionaries:
      - blocks_dict (INSERT blocks), ignoring COMPANY and CUSTOMER
      - lines_dict (LINE entities), keyed by layer
      - polylines_dict (LWPOLYLINE/POLYLINE), keyed by layer

    If consider_polyline_reversal_as_same=True, polylines are normalized
    so reversing the vertices does not count as different.
    
    If cached_results is provided, it will check if the file is in the cache and return
    cached results if available to avoid re-parsing the file.
    """
    # Check if we have cached results for this file
    if cached_results is not None and dxf_path in cached_results:
        return cached_results[dxf_path]
    
    doc = ezdxf.readfile(dxf_path)
    msp = doc.modelspace()

    blocks_dict = {}
    lines_dict = {}
    polylines_dict = {}

    # Process all entities at once to avoid multiple iterations
    entities = list(msp)
    
    for entity in entities:
        dxftype = entity.dxftype()

        # BLOCKS (INSERT) - ignoring "COMPANY" and "CUSTOMER"
        if dxftype == 'INSERT':
            block_name = entity.dxf.name
            if block_name in ("COMPANY", "CUSTOMER"):
                continue
            insert_pt = entity.dxf.insert
            coord = (
                round(insert_pt.x, decimals),
                round(insert_pt.y, decimals),
                round(insert_pt.z, decimals)
            )
            blocks_dict.setdefault(block_name, []).append(coord)

        # LINES (LINE)
        elif dxftype == 'LINE':
            layer_name = entity.dxf.layer
            start_pt = entity.dxf.start
            end_pt = entity.dxf.end
            start_coord = (
                round(start_pt.x, decimals),
                round(start_pt.y, decimals),
                round(start_pt.z, decimals)
            )
            end_coord = (
                round(end_pt.x, decimals),
                round(end_pt.y, decimals),
                round(end_pt.z, decimals)
            )
            # Sort endpoints so (A->B) equals (B->A)
            sorted_coords = tuple(sorted([start_coord, end_coord]))
            lines_dict.setdefault(layer_name, []).append(sorted_coords)

        # POLYLINES (LWPOLYLINE, POLYLINE)
        elif dxftype in ("LWPOLYLINE", "POLYLINE"):
            layer_name = entity.dxf.layer

            if dxftype == 'LWPOLYLINE':
                pts = []
                for px, py, *_ in entity.get_points():
                    pts.append((
                        round(px, decimals),
                        round(py, decimals),
                        0.0
                    ))
            else:  # POLYLINE
                pts = []
                for v in entity.vertices:
                    px, py, pz = v.dxf.location.x, v.dxf.location.y, v.dxf.location.z
                    pts.append((
                        round(px, decimals),
                        round(py, decimals),
                        round(pz, decimals)
                    ))

            if consider_polyline_reversal_as_same:
                rev_points = list(reversed(pts))
                tup_points = tuple(pts)
                tup_rev = tuple(rev_points)
                normalized = min(tup_points, tup_rev)
            else:
                normalized = tuple(pts)

            polylines_dict.setdefault(layer_name, []).append(normalized)

    # Cache the results if a cache is provided
    if cached_results is not None:
        cached_results[dxf_path] = (blocks_dict, lines_dict, polylines_dict)
        
    return blocks_dict, lines_dict, polylines_dict


def compare_dict_of_lists(dict1, dict2):
    """
    Compare two dictionaries of lists and return:
    - common_keys: keys present in both dictionaries
    - only_in_1: keys present only in the first dictionary
    - only_in_2: keys present only in the second dictionary
    - diff: detailed differences for each key
    """
    keys1 = set(dict1.keys())
    keys2 = set(dict2.keys())

    common_keys = keys1.intersection(keys2)
    only_in_1 = keys1 - keys2
    only_in_2 = keys2 - keys1

    # Pre-allocate diff dictionary for better performance
    diff = {}
    
    # Process all common keys in a single loop
    for k in common_keys:
        set1 = set(dict1[k])
        set2 = set(dict2[k])
        common = set1.intersection(set2)
        only_in_1_for_key = set1 - set2
        only_in_2_for_key = set2 - set1
        
        # Only store differences if they exist
        if only_in_1_for_key or only_in_2_for_key:
            diff[k] = {
                "common": common,
                "only_in_1": only_in_1_for_key,
                "only_in_2": only_in_2_for_key
            }

    return common_keys, only_in_1, only_in_2, diff


def create_content_hash(blocks_dict, lines_dict, polylines_dict):
    """
    Creates a hash from DXF content data to identify files with identical modifications.
    
    Args:
        blocks_dict (dict): Dictionary containing block data
        lines_dict (dict): Dictionary containing line data
        polylines_dict (dict): Dictionary containing polyline data
        
    Returns:
        str: A hexadecimal hash string representing the content
    """
    # Convert complex data to a string representation
    content_str = json.dumps([blocks_dict, lines_dict, polylines_dict], sort_keys=True)
    return hashlib.md5(content_str.encode('utf-8')).hexdigest() 