#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Utilities Module

Contains utility functions for file operations, time formatting, and other
common tasks used throughout the application.
"""

import os
import re
import shutil
import time


def format_time(seconds):
    """Format seconds into HH:MM:SS"""
    if seconds is None or seconds < 0:
        return "--:--:--"
    hours, remainder = divmod(int(seconds), 3600)
    minutes, seconds = divmod(remainder, 60)
    return f"{hours:02d}:{minutes:02d}:{seconds:02d}"


def sanitize_filename(name):
    """
    Sanitize the filename to avoid invalid characters
    """
    return re.sub(r'[^A-Za-z0-9._-]', '_', name)


def copy_file(source, destination, log_callback=None):
    """
    Copy a file from source to destination with error handling
    
    Args:
        source (str): Source file path
        destination (str): Destination file path
        log_callback (callable): Optional callback for logging messages
    
    Returns:
        bool: True if successful, False otherwise
    """
    try:
        # Create destination directory if it doesn't exist
        os.makedirs(os.path.dirname(destination), exist_ok=True)
        shutil.copy2(source, destination)
        if log_callback:
            log_callback(f"Copied to: {destination}")
        return True
    except Exception as e:
        if log_callback:
            log_callback(f"ERROR copying file: {e}")
        return False


def move_file(source, destination, log_callback=None):
    """
    Move a file from source to destination with error handling
    
    Args:
        source (str): Source file path
        destination (str): Destination file path
        log_callback (callable): Optional callback for logging messages
    
    Returns:
        bool: True if successful, False otherwise
    """
    try:
        # Create destination directory if it doesn't exist
        os.makedirs(os.path.dirname(destination), exist_ok=True)
        if os.path.exists(source):
            shutil.move(source, destination)
            if log_callback:
                log_callback(f"Moved to: {destination}")
            return True
        else:
            if log_callback:
                log_callback(f"Warning: Original file not found: {source}")
            return False
    except Exception as e:
        if log_callback:
            log_callback(f"ERROR moving file: {e}")
        return False


def estimate_completion_time(elapsed_time, processed_items, total_items):
    """
    Estimate completion time based on progress
    
    Args:
        elapsed_time (float): Time elapsed so far in seconds
        processed_items (int): Number of items processed so far
        total_items (int): Total number of items to process
    
    Returns:
        float: Estimated remaining time in seconds, or None if not enough data
    """
    if processed_items <= 0 or total_items <= 0:
        return None
        
    time_per_item = elapsed_time / processed_items
    remaining_items = total_items - processed_items
    return time_per_item * remaining_items


def count_files_in_directory(directory, extension=None, recursive=False):
    """
    Count the number of files in a directory with an optional extension filter.
    
    Args:
        directory (str): Directory to count files in
        extension (str): Optional file extension to filter by (e.g., '.dxf')
        recursive (bool): Whether to count files in subdirectories
        
    Returns:
        int: Number of files found
    """
    count = 0
    
    if not os.path.exists(directory):
        return 0
    
    if recursive:
        for root, _, files in os.walk(directory):
            if extension:
                count += sum(1 for f in files if f.lower().endswith(extension.lower()))
            else:
                count += len(files)
    else:
        files = os.listdir(directory)
        if extension:
            count = sum(1 for f in files if os.path.isfile(os.path.join(directory, f)) 
                        and f.lower().endswith(extension.lower()))
        else:
            count = sum(1 for f in files if os.path.isfile(os.path.join(directory, f)))
    
    return count


def setup_logging():
    """
    Set up logging for the application.
    
    Returns:
        logging.Logger: Configured logger object
    """
    import logging
    
    # Create logger
    logger = logging.getLogger('dxf_comparison')
    logger.setLevel(logging.INFO)
    
    # Create console handler
    console_handler = logging.StreamHandler()
    console_handler.setLevel(logging.INFO)
    
    # Create formatter
    formatter = logging.Formatter('%(asctime)s - %(levelname)s - %(message)s')
    console_handler.setFormatter(formatter)
    
    # Add handler to logger
    logger.addHandler(console_handler)
    
    return logger 