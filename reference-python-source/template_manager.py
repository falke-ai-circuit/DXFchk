#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Template Manager Module

Contains functions for handling DXF templates, extracting template attributes,
and managing template caching for faster loading.
"""

import os
import ezdxf
import pickle
import datetime

def get_template_name_from_dxf(dxf_path):
    """
    Reads the attribute $(TEMPLATE) from an INSERT block's ATTRIB in the DXF.
    Returns the text value if found, otherwise None.
    """
    try:
        doc = ezdxf.readfile(dxf_path)
    except Exception:
        return None

    msp = doc.modelspace()

    for insert_entity in msp.query("INSERT"):
        for attrib in insert_entity.attribs:
            if attrib.dxf.tag.upper() == "$(TEMPLATE)":
                return attrib.dxf.text

    return None

def get_template_name_fast(dxf_path):
    """
    A faster way to extract the $(TEMPLATE) attribute without loading the entire DXF.
    Uses the ezdxf.recover module to quickly scan the file.
    """
    try:
        # Use a more efficient reader just to find the ATTRIB
        doc = ezdxf.recover.readfile(dxf_path)
        for entity in doc.modelspace():
            if entity.dxftype() == 'INSERT':
                for attrib in entity.attribs:
                    if attrib.dxf.tag.upper() == "$(TEMPLATE)":
                        return attrib.dxf.text
    except Exception:
        # Fall back to the regular method if the fast method fails
        return get_template_name_from_dxf(dxf_path)
    
    return None

def build_template_map(template_dir, recursive=False, progress_callback=None):
    """
    Scans all .dxf files in template_dir, reading the $(TEMPLATE) attribute from each.
    
    Args:
        template_dir (str): Directory containing DXF files
        recursive (bool): Whether to search subdirectories
        progress_callback (callable): Function to call with progress updates
                                     Should return True to continue, False to stop
        
    Returns:
        dict: A dictionary with template names as keys and paths to DXF files as values
    """
    template_map = {}
    
    # Try to load from cache first
    template_map_cached, cache_valid, _ = load_template_map_from_cache(template_dir)
    if cache_valid:
        if progress_callback:
            # Signal completion
            if progress_callback(1, 1) is False:  # Check if we should stop
                return {}  # Return empty map if requested to stop
        return template_map_cached
    
    # Get all DXF files (including in subdirectories if recursive=True)
    dxf_files = []
    if recursive:
        for root, _, files in os.walk(template_dir):
            for f in files:
                if f.lower().endswith(".dxf"):
                    dxf_files.append(os.path.join(root, f))
    else:
        dxf_files = [os.path.join(template_dir, f) 
                     for f in os.listdir(template_dir) 
                     if f.lower().endswith(".dxf")]
    
    # Update total for progress tracking
    total_files = len(dxf_files)
    
    # Process each file
    for i, full_path in enumerate(dxf_files):
        # Update progress if callback provided
        if progress_callback and i % 5 == 0:  # Update every 5 files
            # Check if we should stop
            if progress_callback(i, total_files) is False:
                return template_map  # Return what we've got so far
            
        # Use the faster method to get template name
        val = get_template_name_fast(full_path)
        if val:
            template_map[val] = full_path
    
    # Save to cache for future use
    save_template_map_to_cache(template_dir, template_map)
    
    # Signal completion if callback provided
    if progress_callback:
        progress_callback(total_files, total_files)
        
    return template_map

def load_template_map_from_cache(template_dir):
    """
    Attempts to load the template map from a cache file.
    
    Args:
        template_dir (str): Directory containing DXF files
        
    Returns:
        tuple: A tuple containing:
            - template_map (dict): The loaded template map or an empty dict if unavailable
            - cache_valid (bool): Boolean indicating if the cache was valid and current
            - cache_date (datetime.datetime or None): Creation date of the cache or None if cache not used
    """
    template_map = {}
    cache_valid = False
    cache_date = None
    
    cache_file = os.path.join(template_dir, ".template_map_cache.pkl")
    
    # Try to load from cache first
    if os.path.exists(cache_file):
        try:
            # Get modification time of cache file
            cache_mtime = os.path.getmtime(cache_file)
            cache_date = datetime.datetime.fromtimestamp(cache_mtime)
            
            # Check if any DXF files are newer than the cache
            cache_is_current = True
            for f in os.listdir(template_dir):
                if f.lower().endswith(".dxf"):
                    file_mtime = os.path.getmtime(os.path.join(template_dir, f))
                    if file_mtime > cache_mtime:
                        cache_is_current = False
                        break
            
            if cache_is_current:
                with open(cache_file, 'rb') as f:
                    template_map = pickle.load(f)
                    cache_valid = True
        except Exception:
            # If any error occurs, just rebuild the cache
            pass
    
    return template_map, cache_valid, cache_date

def save_template_map_to_cache(template_dir, template_map):
    """
    Saves the template map to a cache file for faster loading next time.
    
    Args:
        template_dir (str): Directory containing DXF files
        template_map (dict): The template map to save
        
    Returns:
        bool: True if successful, False otherwise
    """
    cache_file = os.path.join(template_dir, ".template_map_cache.pkl")
    
    try:
        with open(cache_file, 'wb') as f:
            pickle.dump(template_map, f)
        return True
    except Exception:
        return False 