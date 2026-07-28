#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
DXF Comparison Tool - GUI Module

This module provides the graphical user interface for the DXF Comparison Tool,
which allows users to compare DXF files against templates and organize them
by similarities and differences.
"""

import os
import re
import sys
import json
import shutil
import tkinter as tk
from tkinter import ttk, filedialog, scrolledtext, messagebox
import threading
import queue
import time
import subprocess
from typing import Dict, List, Tuple, Set, Optional, Any
from datetime import datetime, timedelta

from comparison_processor import run_comparison
from template_manager import build_template_map, load_template_map_from_cache
from utils import setup_logging, count_files_in_directory, format_time

# Create a queue for thread-safe communication
gui_queue = queue.Queue()

# Path for storing settings
SETTINGS_FILE = os.path.join(os.path.expanduser("~"), ".dxfcompare_settings.json")

# Path for storing session data (for resume functionality)
SESSION_FILE = os.path.join(os.path.expanduser("~"), ".dxfcompare_session.json")

def get_git_version() -> str:
    """
    Get the current git commit hash to use as version information.
    Returns a formatted version string with 1.0 as the base version.
    """
    try:
        # Try to get git commit hash
        git_hash = subprocess.check_output(
            ["git", "rev-parse", "--short", "HEAD"], 
            stderr=subprocess.DEVNULL
        ).decode('utf-8').strip()
        
        # Always use 1.0 as the version but append git hash
        return f"v1.0-{git_hash}"
            
    except (subprocess.CalledProcessError, FileNotFoundError):
        # Git not available or not a git repository
        return "v1.0"

class DXFCompareApp:
    """Main application class for the DXF Comparison Tool."""
    
    def __init__(self, root: tk.Tk) -> None:
        """Initialize the application with the given root window."""
        self.root = root
        self.root.title("DXF Compare Tool")
        self.root.geometry("850x893")  # 15% slimmer width (from 1000 to 850) and 3% taller height (from 866 to 893)
        self.root.minsize(680, 750)  # Adjusted minimum width (15% slimmer)
        
        # Get version information
        self.version = get_git_version()
        
        # Set up variables
        self.template_folder = tk.StringVar()
        self.search_folder = tk.StringVar()
        self.output_folder = tk.StringVar()
        self.is_comparing = False
        self.comparison_thread = None
        self.template_thread = None
        self.log_queue = queue.Queue()
        self.log_messages = []  # Store log messages for saving to file
        
        # Time tracking variables
        self.start_time = None
        self.elapsed_time = tk.StringVar(value="00:00:00")
        self.estimated_time = tk.StringVar(value="--:--:--")
        self.timer_running = False
        
        # Session state variables for resume functionality
        self.session_state = {
            'active': False,
            'phase': None,  # 'template' or 'comparison'
            'template_map': None,
            'processed_files': [],
            'template_progress': 0,
            'comparison_progress': 0,
            'start_time': None,
            'elapsed_seconds': 0
        }
        self.session_save_timer = None
        
        # Flag to indicate if threads should stop processing
        self.should_stop = False
        
        # Load previous settings
        self._load_settings()
        
        # Create main frame
        self.main_frame = ttk.Frame(self.root, padding="10")
        self.main_frame.pack(fill=tk.BOTH, expand=True)
        
        # Set up the UI elements in the correct order
        self._setup_folder_selection()
        self._setup_options()
        self._setup_action_buttons()
        self._setup_progress_area()
        self._setup_log_area()
        
        # Add status bar with version at the bottom
        self._setup_status_bar()
        
        # Set up logging
        self.logger = setup_logging()
        
        # Start the periodic call to check the queues
        self._check_queues()
        
        # Bind close event to save settings
        self.root.protocol("WM_DELETE_WINDOW", self._on_close)
        
        # After loading settings, update the button states
        self._update_button_states()
    
    def _setup_status_bar(self) -> None:
        """Set up a status bar at the bottom with version information."""
        # Create a frame at the bottom of the main window for status information
        status_frame = ttk.Frame(self.main_frame)
        status_frame.pack(side=tk.BOTTOM, fill=tk.X, pady=(10, 5))
        
        # Separator removed
        
        # Create the version label with a lighter color and smaller font
        version_label = ttk.Label(
            status_frame, 
            text=f"Version: {self.version}", 
            font=("TkDefaultFont", 8),
            foreground="#888888"  # Lighter gray color
        )
        # Remove padding
        version_label.pack(side=tk.RIGHT, padx=2, pady=1)
    
    def _load_settings(self) -> None:
        """Load settings from settings file."""
        try:
            if os.path.exists(SETTINGS_FILE):
                with open(SETTINGS_FILE, 'r') as f:
                    settings = json.load(f)
                    
                    if 'template_folder' in settings and os.path.exists(settings['template_folder']):
                        self.template_folder.set(settings['template_folder'])
                    
                    if 'search_folder' in settings and os.path.exists(settings['search_folder']):
                        self.search_folder.set(settings['search_folder'])
                    
                    if 'output_folder' in settings and os.path.exists(settings['output_folder']):
                        self.output_folder.set(settings['output_folder'])
        except Exception as e:
            print(f"Error loading settings: {str(e)}")
    
    def _save_settings(self) -> None:
        """Save settings to settings file."""
        try:
            settings = {
                'template_folder': self.template_folder.get(),
                'search_folder': self.search_folder.get(),
                'output_folder': self.output_folder.get()
            }
            
            with open(SETTINGS_FILE, 'w') as f:
                json.dump(settings, f)
        except Exception as e:
            print(f"Error saving settings: {str(e)}")
    
    def _save_session_state(self) -> None:
        """Save the current session state for resume functionality."""
        if not self.is_comparing:
            # Don't save session state if not in a comparison operation
            return
            
        try:
            # Update elapsed time
            if self.start_time:
                self.session_state['elapsed_seconds'] = time.time() - self.start_time
            
            # Create a serializable copy of the session state
            # We can't directly serialize template_map, so store template folder instead
            serializable_state = {
                'active': True,
                'phase': self.session_state['phase'],
                'template_folder': self.template_folder.get(),
                'search_folder': self.search_folder.get(),
                'output_folder': self.output_folder.get(),
                'recursive_search': self.recursive_search.get(),
                'move_files': self.move_files.get(),
                'group_by_content': self.group_by_content.get(),
                'processed_files': self.session_state['processed_files'],
                'template_progress': self.template_progress['value'],
                'comparison_progress': self.comparison_progress['value'],
                'elapsed_seconds': self.session_state['elapsed_seconds'],
                'timestamp': time.time()
            }
            
            with open(SESSION_FILE, 'w') as f:
                json.dump(serializable_state, f)
                
            self._log_message("Session state saved", show_in_ui=False)
            
            # Schedule the next save in 30 seconds if still comparing
            if self.is_comparing:
                self.session_save_timer = self.root.after(30000, self._save_session_state)
                
        except Exception as e:
            self._log_message(f"Error saving session state: {str(e)}", error=True, show_in_ui=False)
    
    def _load_session_state(self) -> bool:
        """
        Load the session state for resume functionality.
        Returns True if a valid session was loaded, False otherwise.
        """
        try:
            if os.path.exists(SESSION_FILE):
                with open(SESSION_FILE, 'r') as f:
                    state = json.load(f)
                    
                    # Check if the session is active and not too old (24 hours max)
                    if state.get('active', False) and state.get('timestamp', 0) > time.time() - 86400:
                        self.session_state = state
                        
                        # Restore folder paths
                        if 'template_folder' in state:
                            self.template_folder.set(state['template_folder'])
                        if 'search_folder' in state:
                            self.search_folder.set(state['search_folder'])
                        if 'output_folder' in state:
                            self.output_folder.set(state['output_folder'])
                            
                        # Set recursive search option
                        if 'recursive_search' in state:
                            self.recursive_search.set(state['recursive_search'])
                            
                        # Set move files option
                        if 'move_files' in state:
                            self.move_files.set(state['move_files'])
                            
                        # Set group by content option
                        if 'group_by_content' in state:
                            self.group_by_content.set(state['group_by_content'])
                            
                        return True
            
            return False
                    
        except Exception as e:
            self._log_message(f"Error loading session state: {str(e)}", error=True)
            return False
            
    def _clear_session_state(self) -> None:
        """Clear the saved session state."""
        try:
            if os.path.exists(SESSION_FILE):
                os.remove(SESSION_FILE)
                
            # Reset session state variables
            self.session_state = {
                'active': False,
                'phase': None,
                'template_map': None,
                'processed_files': [],
                'template_progress': 0,
                'comparison_progress': 0,
                'start_time': None,
                'elapsed_seconds': 0
            }
            
        except Exception as e:
            self._log_message(f"Error clearing session state: {str(e)}", error=True)
    
    def _on_close(self) -> None:
        """Handle window close event."""
        self._save_settings()
        
        # If a comparison is running, save the session state
        if self.is_comparing:
            self._save_session_state()
            messagebox.showinfo(
                "Session Saved", 
                "The current comparison session has been saved and can be resumed later."
            )
            
        self.root.destroy()
    
    def _setup_folder_selection(self) -> None:
        """Set up the folder selection section of the UI."""
        folder_frame = ttk.LabelFrame(self.main_frame, text="Folder Selection", padding="10")
        folder_frame.pack(fill=tk.X, pady=5)
        
        # Template folder
        ttk.Label(folder_frame, text="Template Folder:").grid(row=0, column=0, sticky=tk.W, pady=5)
        ttk.Entry(folder_frame, textvariable=self.template_folder, width=80).grid(row=0, column=1, sticky=tk.W+tk.E, padx=5, pady=5)
        ttk.Button(folder_frame, text="Browse...", command=self._select_template_folder).grid(row=0, column=2, pady=5)
        
        # Search folder
        ttk.Label(folder_frame, text="Search Folder:").grid(row=1, column=0, sticky=tk.W, pady=5)
        ttk.Entry(folder_frame, textvariable=self.search_folder, width=80).grid(row=1, column=1, sticky=tk.W+tk.E, padx=5, pady=5)
        ttk.Button(folder_frame, text="Browse...", command=self._select_search_folder).grid(row=1, column=2, pady=5)
        
        # Output folder
        ttk.Label(folder_frame, text="Output Folder:").grid(row=2, column=0, sticky=tk.W, pady=5)
        ttk.Entry(folder_frame, textvariable=self.output_folder, width=80).grid(row=2, column=1, sticky=tk.W+tk.E, padx=5, pady=5)
        ttk.Button(folder_frame, text="Browse...", command=self._select_output_folder).grid(row=2, column=2, pady=5)
        
        # Add folder explanation to the right side in blue text (without the guide title)
        ttk.Label(folder_frame, text="• Template Folder: Contains original DXF template files", 
                 foreground="blue").grid(row=0, column=3, sticky=tk.W, padx=(15, 5), pady=2)
                 
        ttk.Label(folder_frame, text="• Search Folder: Contains DXF files to compare", 
                 foreground="blue").grid(row=1, column=3, sticky=tk.W, padx=(15, 5), pady=2)
                 
        ttk.Label(folder_frame, text="• Output Folder: Where results are saved:", 
                 foreground="blue").grid(row=2, column=3, sticky=tk.W, padx=(15, 5), pady=2)
                 
        ttk.Label(folder_frame, text="  - Identical files → template folders", 
                 foreground="blue").grid(row=3, column=3, sticky=tk.W, padx=(15, 5), pady=1)
                 
        ttk.Label(folder_frame, text="  - Different files → _mod folders", 
                 foreground="blue").grid(row=4, column=3, sticky=tk.W, padx=(15, 5), pady=1)
                 
        ttk.Label(folder_frame, text="  - No template → 'notemplate' folder", 
                 foreground="blue").grid(row=5, column=3, sticky=tk.W, padx=(15, 5), pady=1)
        
        # Configure the column to expand properly - give more weight to the entry column
        folder_frame.columnconfigure(1, weight=10)  # Increase weight for entry fields column
        folder_frame.columnconfigure(0, weight=1)   # Give some weight to label column
        folder_frame.columnconfigure(2, weight=1)   # Give some weight to button column
        folder_frame.columnconfigure(3, weight=2)   # Give weight to explanation column
    
    def _setup_options(self) -> None:
        """Set up the options section of the UI."""
        options_frame = ttk.LabelFrame(self.main_frame, text="Options", padding="10")
        options_frame.pack(fill=tk.X, pady=5)
        
        # Checkboxes for various options
        self.recursive_search = tk.BooleanVar(value=True)
        ttk.Checkbutton(options_frame, text="Search Subdirectories", variable=self.recursive_search).grid(row=0, column=0, sticky=tk.W, padx=5, pady=5)
        
        # Note: Files are always preserved now, but we keep the option for backward compatibility
        self.move_files = tk.BooleanVar(value=False)
        
        # Two-line note about file organization
        preserve_files_label = ttk.Label(options_frame, text="• Original files in search folder are preserved", foreground="blue")
        preserve_files_label.grid(row=0, column=1, sticky=tk.W, padx=5, pady=(5,0))
        
        organize_files_label = ttk.Label(options_frame, text="• Files with differences are moved to mod folders (no duplicates)", foreground="blue")
        organize_files_label.grid(row=1, column=1, sticky=tk.W, padx=5, pady=(0,5))
        
        self.group_by_content = tk.BooleanVar(value=True)
        ttk.Checkbutton(options_frame, text="Group by Content Differences", variable=self.group_by_content).grid(row=2, column=0, sticky=tk.W, padx=5, pady=5)
    
    def _setup_action_buttons(self) -> None:
        """Set up the action buttons section of the UI."""
        button_frame = ttk.Frame(self.main_frame)
        button_frame.pack(fill=tk.X, pady=10)
        
        # Primary action buttons on the left
        self.start_button = ttk.Button(button_frame, text="Start", command=self._start_comparison)
        self.start_button.pack(side=tk.LEFT, padx=5)
        
        self.resume_button = ttk.Button(button_frame, text="Resume", command=self._resume_comparison, state=tk.DISABLED)
        self.resume_button.pack(side=tk.LEFT, padx=5)
        
        self.stop_button = ttk.Button(button_frame, text="Stop", command=self._stop_comparison, state=tk.DISABLED)
        self.stop_button.pack(side=tk.LEFT, padx=5)
        
        # Utility buttons on the right
        self.view_structure_button = ttk.Button(button_frame, text="View Folder Structure", 
                                                command=self._show_folder_structure)
        self.view_structure_button.pack(side=tk.RIGHT, padx=5)
        
        self.clear_button = ttk.Button(button_frame, text="Clear Output Folder", command=self._clear_output_folder)
        self.clear_button.pack(side=tk.RIGHT, padx=5)
        
        # Update button states based on current folder selections
        self._update_button_states()
        
        # Check if there's a session to resume
        self._check_for_resumable_session()
    
    def _clear_output_folder(self) -> None:
        """Clear the output folder of all analysis files."""
        output_folder = self.output_folder.get()
        if not output_folder or not os.path.exists(output_folder):
            self._log_message("Output folder does not exist", error=True)
            return
        
        # Ask for confirmation
        confirm = messagebox.askyesno(
            "Confirm Clear",
            f"This will delete all files and folders in:\n{output_folder}\n\nAre you sure you want to continue?",
            icon="warning"
        )
        
        if not confirm:
            return
        
        try:
            # Delete all files and subdirectories
            for item in os.listdir(output_folder):
                item_path = os.path.join(output_folder, item)
                
                # Don't delete the log file
                if item == "dxfanalyze.log" and os.path.isfile(item_path):
                    continue
                    
                if os.path.isdir(item_path):
                    shutil.rmtree(item_path)
                else:
                    os.remove(item_path)
            
            self._log_message(f"Output folder cleared: {output_folder}")
        except Exception as e:
            self._log_message(f"Error clearing output folder: {str(e)}", error=True)
    
    def _setup_progress_area(self) -> None:
        """Set up the progress area of the UI including time tracking."""
        progress_frame = ttk.LabelFrame(self.main_frame, text="Progress", padding="10")
        progress_frame.pack(fill=tk.X, pady=5)
        
        # Status message
        ttk.Label(progress_frame, text="Status:").grid(row=0, column=0, sticky=tk.W, pady=5)
        self.status_var = tk.StringVar(value="Ready")
        ttk.Label(progress_frame, textvariable=self.status_var).grid(row=0, column=1, sticky=tk.W, pady=5)
        
        # Time displays on the right side of the status bar
        ttk.Label(progress_frame, text="Running Time:").grid(row=0, column=2, sticky=tk.E, padx=(20,5), pady=5)
        ttk.Label(progress_frame, textvariable=self.elapsed_time).grid(row=0, column=3, sticky=tk.W, padx=5, pady=5)
        
        ttk.Label(progress_frame, text="Est. Remaining:").grid(row=0, column=4, sticky=tk.E, padx=(20,5), pady=5)
        ttk.Label(progress_frame, textvariable=self.estimated_time).grid(row=0, column=5, sticky=tk.W, padx=5, pady=5)
        
        # Template loading progress - use full width
        ttk.Label(progress_frame, text="Templates:").grid(row=1, column=0, sticky=tk.W, pady=5)
        self.template_progress = ttk.Progressbar(progress_frame, orient=tk.HORIZONTAL, length=300, mode='determinate')
        self.template_progress.grid(row=1, column=1, sticky=tk.W+tk.E, pady=5, padx=5, columnspan=5)
        self.template_status = tk.StringVar(value="")
        ttk.Label(progress_frame, textvariable=self.template_status).grid(row=1, column=6, sticky=tk.W, pady=5)
        
        # Comparison progress - use full width
        ttk.Label(progress_frame, text="Comparison:").grid(row=2, column=0, sticky=tk.W, pady=5)
        self.comparison_progress = ttk.Progressbar(progress_frame, orient=tk.HORIZONTAL, length=300, mode='determinate')
        self.comparison_progress.grid(row=2, column=1, sticky=tk.W+tk.E, pady=5, padx=5, columnspan=5)
        self.comparison_status = tk.StringVar(value="")
        ttk.Label(progress_frame, textvariable=self.comparison_status).grid(row=2, column=6, sticky=tk.W, pady=5)
        
        # Configure the grid to expand properly
        progress_frame.columnconfigure(1, weight=5)  # Give more weight to the progress bar column
        for i in range(2, 6):  # Configure other columns
            progress_frame.columnconfigure(i, weight=1)
    
    def _setup_log_area(self) -> None:
        """Set up the log area of the UI."""
        log_frame = ttk.LabelFrame(self.main_frame, text="Log", padding="10")
        log_frame.pack(fill=tk.BOTH, expand=True, pady=5)
        
        self.log_text = scrolledtext.ScrolledText(log_frame, wrap=tk.WORD, height=15)
        self.log_text.pack(fill=tk.BOTH, expand=True)
        self.log_text.config(state=tk.DISABLED)
    
    def _select_template_folder(self) -> None:
        """Open dialog to select template folder."""
        folder = filedialog.askdirectory(title="Select Template Folder")
        if folder:
            self.template_folder.set(folder)
            self._log_message(f"Template folder set to: {folder}")
    
    def _select_search_folder(self) -> None:
        """Open dialog to select search folder."""
        folder = filedialog.askdirectory(title="Select Search Folder")
        if folder:
            self.search_folder.set(folder)
            self._log_message(f"Search folder set to: {folder}")
    
    def _select_output_folder(self) -> None:
        """Open dialog to select output folder."""
        folder = filedialog.askdirectory(title="Select Output Folder")
        if folder:
            self.output_folder.set(folder)
            self._log_message(f"Output folder set to: {folder}")
            self._update_button_states()
    
    def _update_button_states(self) -> None:
        """Update the states of buttons based on current conditions."""
        # Enable/disable the View Folder Structure button based on output folder existence
        output_folder = self.output_folder.get()
        if output_folder and os.path.exists(output_folder):
            self.view_structure_button.config(state=tk.NORMAL)
        else:
            self.view_structure_button.config(state=tk.DISABLED)
        
        # Similar logic can be applied for other buttons if needed
        if output_folder and os.path.exists(output_folder):
            self.clear_button.config(state=tk.NORMAL)
        else:
            self.clear_button.config(state=tk.DISABLED)
            
        # Check if there's a session to resume
        self._check_for_resumable_session()
    
    def _start_comparison(self) -> None:
        """Start the comparison process in a separate thread."""
        # Validate input
        if not self._validate_input():
            return
        
        # Save settings
        self._save_settings()
        
        # Clear any existing session state
        self._clear_session_state()
        
        # Reset the stop flag
        self.should_stop = False
        
        # Update UI state
        self.is_comparing = True
        self.start_button.config(state=tk.DISABLED)
        self.resume_button.config(state=tk.DISABLED)
        self.stop_button.config(state=tk.NORMAL)
        self.clear_button.config(state=tk.DISABLED)
        self.view_structure_button.config(state=tk.DISABLED)  # Disable during comparison
        self.status_var.set("Processing...")
        
        # Reset progress bars
        self.template_progress['value'] = 0
        self.comparison_progress['value'] = 0
        
        # Clear log
        self.log_text.config(state=tk.NORMAL)
        self.log_text.delete(1.0, tk.END)
        self.log_text.config(state=tk.DISABLED)
        self.log_messages = []  # Clear stored log messages
        
        # Start time tracking
        self.start_time = time.time()
        self.timer_running = True
        self._update_time_display()
        
        # Log start message
        self._log_message("Starting comparison process...")
        
        # Count files for progress tracking
        template_count = count_files_in_directory(self.template_folder.get(), ".dxf", self.recursive_search.get())
        search_count = count_files_in_directory(self.search_folder.get(), ".dxf", self.recursive_search.get())
        
        self._log_message(f"Found {template_count} template files and {search_count} files to check")
        
        # Start template loading thread
        self.template_thread = threading.Thread(
            target=self._build_template_map_thread,
            args=(
                self.template_folder.get(),
                self.recursive_search.get(),
                template_count
            )
        )
        self.template_thread.daemon = True
        self.template_thread.start()
    
    def _update_time_display(self) -> None:
        """Update the time display."""
        if not self.timer_running:
            return
            
        # Calculate elapsed time
        elapsed_seconds = time.time() - self.start_time
        elapsed_str = format_time(elapsed_seconds)
        self.elapsed_time.set(elapsed_str)
        
        # Log the time update occasionally to ensure it's working
        if int(elapsed_seconds) % 30 == 0:  # Log every 30 seconds
            self._log_message(f"Processing time: {elapsed_str}")
        
        # Schedule next update (every second)
        self.root.after(1000, self._update_time_display)
    
    def _build_template_map_thread(self, template_folder: str, recursive: bool, total_files: int) -> None:
        """Thread function for building template map."""
        try:
            # Update session state
            self.session_state['phase'] = 'template'
            self._save_session_state()
            
            # Pass a callback to update progress
            def progress_callback(current, total):
                # Check if we should stop
                if self.should_stop:
                    return False  # Return False to signal processing should stop
                
                gui_queue.put(("template_progress", current, total))
                
                # Update session state with progress
                self.session_state['template_progress'] = (current / total) * 100 if total > 0 else 0
                
                # Update estimated time
                if current > 0:
                    elapsed_seconds = time.time() - self.start_time
                    estimated_total = (elapsed_seconds / current) * total
                    remaining = estimated_total - elapsed_seconds
                    if remaining > 0:
                        remaining_str = format_time(remaining)
                        # Send just the time string, not the prefix
                        gui_queue.put(("estimated_time", remaining_str))
                
                return True  # Continue processing
            
            # Build template map
            template_map = build_template_map(
                template_folder, 
                recursive=recursive,
                progress_callback=progress_callback
            )
            
            # Check if we were stopped during template building
            if self.should_stop:
                gui_queue.put(("log", "Template mapping stopped by user."))
                gui_queue.put(("process_complete", None))
                return
            
            # Save template map in session state
            self.session_state['template_map'] = template_map
            
            # Log completion
            gui_queue.put(("log", f"Template mapping complete. Found {len(template_map)} unique templates."))
            gui_queue.put(("template_complete", None))
            
            # Update session state for comparison phase
            self.session_state['phase'] = 'comparison'
            self.session_state['processed_files'] = []
            self._save_session_state()
            
            # Start comparison thread
            comparison_thread = threading.Thread(
                target=self._run_comparison_thread,
                args=(
                    template_map,
                    self.search_folder.get(),
                    self.output_folder.get(),
                    self.recursive_search.get(),
                    self.move_files.get(),
                    self.group_by_content.get(),
                    []  # No processed files when starting fresh
                )
            )
            comparison_thread.daemon = True
            comparison_thread.start()
            
        except Exception as e:
            gui_queue.put(("error", f"Error in template mapping: {str(e)}"))
            gui_queue.put(("process_complete", None))
    
    def _run_comparison_thread(
        self, 
        template_map: Dict, 
        search_folder: str, 
        output_folder: str,
        recursive: bool,
        move_files: bool,
        group_by_content: bool,
        processed_files: List[str] = []  # List of already processed files
    ) -> None:
        """Thread function for running comparison."""
        try:
            # Get all DXF files to process
            all_dxf_files = []
            if recursive:
                for root, _, files in os.walk(search_folder):
                    for file in files:
                        if file.lower().endswith('.dxf'):
                            all_dxf_files.append(os.path.join(root, file))
            else:
                all_dxf_files = [
                    os.path.join(search_folder, f) 
                    for f in os.listdir(search_folder) 
                    if f.lower().endswith('.dxf')
                ]
                
            # Filter out already processed files
            to_process = [f for f in all_dxf_files if f not in processed_files]
            
            if processed_files:
                self._log_message(f"Resuming from previous session. {len(processed_files)} files already processed, {len(to_process)} files remaining.")
            
            # Initialize the comparison processor
            from comparison_processor import ComparisonProcessor
            processor = ComparisonProcessor(
                template_map, 
                output_folder, 
                move_files=move_files, 
                group_by_content=group_by_content,
                log_callback=lambda msg: gui_queue.put(("log", msg))
            )
            
            # Pass a callback to update progress
            def progress_callback(current, total):
                # Check if we should stop
                if self.should_stop:
                    return False  # Signal to stop processing
                
                # Adjust for already processed files
                adjusted_current = len(processed_files) + current
                adjusted_total = len(all_dxf_files)
                
                gui_queue.put(("comparison_progress", adjusted_current, adjusted_total))
                
                # Update session state
                self.session_state['comparison_progress'] = (adjusted_current / adjusted_total) * 100 if adjusted_total > 0 else 0
                
                # Periodically update the list of processed files in the session state
                if current % 5 == 0 or current == total:  # Every 5 files or at the end
                    current_processed = processed_files + to_process[:current]
                    self.session_state['processed_files'] = current_processed
                    self._save_session_state()
                
                # Update estimated time
                if current > 0:
                    elapsed_seconds = time.time() - self.start_time
                    estimated_total = (elapsed_seconds / current) * total
                    remaining = estimated_total - elapsed_seconds
                    if remaining > 0:
                        remaining_str = format_time(remaining)
                        # Send just the time string, not the prefix
                        gui_queue.put(("estimated_time", remaining_str))
                
                return True  # Continue processing
            
            # Pass a callback to log messages
            def log_callback(message):
                gui_queue.put(("log", message))
            
            # Process each remaining file
            results = {
                'total_files': len(all_dxf_files),
                'matched_files': 0,
                'different_files': 0,
                'no_template_files': 0,
                'mod_folder_count': 0
            }
            
            for i, dxf_file in enumerate(to_process):
                # Check if we should stop before processing each file
                if self.should_stop:
                    self._log_message("Comparison stopped by user.")
                    break
                
                # Update progress
                progress_callback(i, len(to_process))
                
                # Process the file and update results
                result = processor.process_file(
                    dxf_file,
                    check_stop=lambda: self.should_stop  # Pass a function to check if we should stop
                )
                
                # If result is None, it means processing was stopped
                if result is None:
                    self._log_message("Comparison stopped by user during file processing.")
                    break
                    
                if result == 'match':
                    results['matched_files'] += 1
                elif result == 'different':
                    results['different_files'] += 1
                elif result == 'no_template':
                    results['no_template_files'] += 1
            
            # Finalize and get mod folder count - only if not stopped
            if not self.should_stop:
                processor.finalize()
                results['mod_folder_count'] = processor.get_mod_folder_count()
                
                # Log results for completed comparison
                gui_queue.put(("log", f"Comparison complete. Results:"))
                gui_queue.put(("log", f"- Total files processed: {results['total_files']}"))
                gui_queue.put(("log", f"- Files with matches: {results['matched_files']}"))
                gui_queue.put(("log", f"- Files with differences: {results['different_files']}"))
                gui_queue.put(("log", f"- Files with no template: {results['no_template_files']}"))
                gui_queue.put(("log", f"- Number of mod folders: {results['mod_folder_count']}"))
                
                # Clear session state since we're finished
                self._clear_session_state()
            
            # Process complete
            gui_queue.put(("process_complete", results if not self.should_stop else None))
            
        except Exception as e:
            gui_queue.put(("error", f"Error in comparison process: {str(e)}"))
            gui_queue.put(("process_complete", None))
    
    def _stop_comparison(self) -> None:
        """Stop the comparison process."""
        if self.is_comparing:
            self.is_comparing = False
            self.timer_running = False
            self._log_message("Stopping comparison process...")
            
            # Set the stop flag to signal threads to exit
            self.should_stop = True
            
            # Save the session state for resuming later
            self._save_session_state()
            
            # Cancel the timer
            if self.session_save_timer:
                self.root.after_cancel(self.session_save_timer)
                self.session_save_timer = None
                
            # Notify user that session is saved
            self._log_message("Session state saved. You can resume later using the 'Resume' button.")
            messagebox.showinfo(
                "Session Saved", 
                "The current comparison session has been saved and can be resumed later using the 'Resume' button."
            )
                
            # Can't directly stop threads in Python, but we can set a flag
            # and check it in the thread functions
            gui_queue.put(("process_complete", None))
    
    def _validate_input(self) -> bool:
        """Validate user input before starting comparison."""
        # Check if folders are selected
        if not self.template_folder.get():
            self._log_message("Error: Template folder not selected", error=True)
            return False
        
        if not self.search_folder.get():
            self._log_message("Error: Search folder not selected", error=True)
            return False
        
        if not self.output_folder.get():
            self._log_message("Error: Output folder not selected", error=True)
            return False
        
        # Check if folders exist
        if not os.path.isdir(self.template_folder.get()):
            self._log_message(f"Error: Template folder '{self.template_folder.get()}' does not exist", error=True)
            return False
        
        if not os.path.isdir(self.search_folder.get()):
            self._log_message(f"Error: Search folder '{self.search_folder.get()}' does not exist", error=True)
            return False
        
        # Create output folder if it doesn't exist
        if not os.path.exists(self.output_folder.get()):
            try:
                os.makedirs(self.output_folder.get())
                self._log_message(f"Created output folder: {self.output_folder.get()}")
                self._update_button_states()  # Update button states after creating folder
            except Exception as e:
                self._log_message(f"Error creating output folder: {str(e)}", error=True)
                return False
        
        return True
    
    def _log_message(self, message: str, error: bool = False, show_in_ui: bool = True) -> None:
        """Add a message to the log area."""
        # Store message for log file
        timestamp = time.strftime("%Y-%m-%d %H:%M:%S")
        log_entry = f"[{timestamp}] {'ERROR: ' if error else ''}{message}"
        self.log_messages.append(log_entry)
        
        # Only show in UI if requested
        if show_in_ui:
            self.log_text.config(state=tk.NORMAL)
            
            if error:
                self.log_text.insert(tk.END, f"ERROR: {message}\n", "error")
                self.log_text.tag_configure("error", foreground="red")
            else:
                self.log_text.insert(tk.END, f"{message}\n")
            self.log_text.see(tk.END)
            self.log_text.config(state=tk.DISABLED)
    
    def _check_queues(self) -> None:
        """Check queues for messages from worker threads."""
        # Process all available messages
        try:
            while True:
                message = gui_queue.get_nowait()
                self._process_message(message)
                gui_queue.task_done()
        except queue.Empty:
            pass
        
        # Schedule next check
        self.root.after(100, self._check_queues)
    
    def _process_message(self, message: Tuple[str, Any]) -> None:
        """Process a message from the queue."""
        message_type = message[0]
        
        if message_type == "log":
            self._log_message(message[1])
        
        elif message_type == "error":
            self._log_message(message[1], error=True)
        
        elif message_type == "template_progress":
            current, total = message[1], message[2]
            if total > 0:
                self.template_progress['value'] = (current / total) * 100
                self.template_status.set(f"{current}/{total}")
                
                # Update estimated remaining time for template processing
                if current > 0 and self.start_time:
                    elapsed_seconds = time.time() - self.start_time
                    time_per_item = elapsed_seconds / current
                    remaining_items = total - current
                    estimated_seconds = time_per_item * remaining_items
                    # Make sure estimated time is positive
                    if estimated_seconds > 0:
                        remaining_str = format_time(estimated_seconds)
                        self.estimated_time.set(remaining_str)
                    
                    # Also update elapsed time here to ensure it's visible
                    self.elapsed_time.set(format_time(elapsed_seconds))
        
        elif message_type == "comparison_progress":
            current, total = message[1], message[2]
            if total > 0:
                self.comparison_progress['value'] = (current / total) * 100
                self.comparison_status.set(f"{current}/{total}")
                
                # Update estimated remaining time for comparison processing
                if current > 0 and self.start_time:
                    elapsed_seconds = time.time() - self.start_time
                    time_per_item = elapsed_seconds / current
                    remaining_items = total - current
                    estimated_seconds = time_per_item * remaining_items
                    # Make sure estimated time is positive
                    if estimated_seconds > 0:
                        remaining_str = format_time(estimated_seconds)
                        self.estimated_time.set(remaining_str)
                    
                    # Also update elapsed time here to ensure it's visible
                    self.elapsed_time.set(format_time(elapsed_seconds))
        
        elif message_type == "estimated_time":
            # If message has a prefix like "Est. remaining:", strip it
            time_value = message[1]
            if isinstance(time_value, str) and ":" in time_value:
                # If the time string has a prefix, extract just the time part
                if "Est. remaining: " in time_value:
                    time_value = time_value.replace("Est. remaining: ", "")
                self.estimated_time.set(time_value)
            
            # Ensure elapsed time is also updated
            if self.start_time:
                elapsed_seconds = time.time() - self.start_time
                self.elapsed_time.set(format_time(elapsed_seconds))
        
        elif message_type == "template_complete":
            self.template_progress['value'] = 100
            self.template_status.set("Complete")
            self._log_message("Template mapping complete")
        
        elif message_type == "process_complete":
            results = message[1]
            self.is_comparing = False
            self.timer_running = False
            
            # Reset the stop flag
            self.should_stop = False
            
            # Cancel session save timer if it exists
            if self.session_save_timer:
                self.root.after_cancel(self.session_save_timer)
                self.session_save_timer = None
            
            self.start_button.config(state=tk.NORMAL)
            self.stop_button.config(state=tk.DISABLED)
            # Update button states instead of directly setting them
            self._update_button_states()
            self.status_var.set("Ready")
            self._log_message("Comparison process completed")
            
            # Calculate total time
            if self.start_time:
                total_seconds = time.time() - self.start_time
                total_time = format_time(total_seconds)
                self._log_message(f"Total processing time: {total_time}")
                self.elapsed_time.set(total_time)
                self.estimated_time.set("00:00:00")
            
            # Save log to file
            self._save_log_to_file()
            
            # Show folder structure if we have results (optional now)
            if results:
                self._show_folder_structure()
    
    def _save_log_to_file(self) -> None:
        """Save the log to a file in the output folder."""
        try:
            output_folder = self.output_folder.get()
            if output_folder and os.path.exists(output_folder):
                log_file_path = os.path.join(output_folder, "dxfanalyze.log")
                with open(log_file_path, 'w', encoding='utf-8') as f:
                    f.write("\n".join(self.log_messages))
                self._log_message(f"Log saved to: {log_file_path}")
        except Exception as e:
            self._log_message(f"Error saving log file: {str(e)}", error=True)
    
    def _show_folder_structure(self) -> None:
        """Show a treeview with the folder structure of the output folder."""
        output_folder = self.output_folder.get()
        if not output_folder or not os.path.exists(output_folder):
            self._log_message("Output folder does not exist", error=True)
            return
            
        # Check if folder is empty
        if not os.listdir(output_folder):
            message = "Output folder is empty. There is no folder structure to display."
            self._log_message(message)
            messagebox.showinfo("Empty Folder", message)
            return
        
        # Create a new window for the treeview
        tree_window = tk.Toplevel(self.root)
        tree_window.title("Output Folder Structure")
        tree_window.geometry("800x600")
        
        # Add explanation text
        explanation_frame = ttk.Frame(tree_window)
        explanation_frame.pack(fill=tk.X, padx=10, pady=5)
        
        ttk.Label(explanation_frame, text="Folder and file structure in the output folder:", 
                 padding=(0, 5)).pack(side=tk.LEFT)
        
        # Add instruction about double-clicking and checkboxes
        instructions_frame = ttk.Frame(explanation_frame)
        instructions_frame.pack(side=tk.RIGHT)
        
        ttk.Label(instructions_frame, text="• Double-click on a file to open it", 
                 padding=(0, 2), foreground="blue").pack(anchor=tk.E)
        ttk.Label(instructions_frame, text="• Double-click on folder to expand/collapse", 
                 padding=(0, 2), foreground="blue").pack(anchor=tk.E)
        ttk.Label(instructions_frame, text="• Template folders are marked with [ ] or [✓]", 
                 padding=(0, 2), foreground="blue").pack(anchor=tk.E)
        ttk.Label(instructions_frame, text="• Right-click on template folder to check/uncheck", 
                 padding=(0, 2), foreground="blue").pack(anchor=tk.E)
        ttk.Label(instructions_frame, text="• Right-click for additional options", 
                 padding=(0, 2), foreground="blue").pack(anchor=tk.E)
        
        # Create frame for treeview and scrollbars
        tree_frame = ttk.Frame(tree_window)
        tree_frame.pack(fill=tk.BOTH, expand=True, padx=10, pady=10)
        
        # Create treeview
        folder_tree = ttk.Treeview(tree_frame)
        
        # Add vertical scrollbar
        vsb = ttk.Scrollbar(tree_frame, orient="vertical", command=folder_tree.yview)
        folder_tree.configure(yscrollcommand=vsb.set)
        vsb.pack(side=tk.RIGHT, fill=tk.Y)
        
        # Add horizontal scrollbar
        hsb = ttk.Scrollbar(tree_frame, orient="horizontal", command=folder_tree.xview)
        folder_tree.configure(xscrollcommand=hsb.set)
        hsb.pack(side=tk.BOTTOM, fill=tk.X)
        
        # Pack the treeview
        folder_tree.pack(side=tk.LEFT, fill=tk.BOTH, expand=True)
        
        # Set up columns
        folder_tree["columns"] = ("size", "modified", "path", "is_template", "checked")
        folder_tree.column("#0", width=300, minwidth=200)
        folder_tree.column("size", width=100, minwidth=80, anchor=tk.E)
        folder_tree.column("modified", width=150, minwidth=150)
        folder_tree.column("path", width=0, stretch=False)  # Hidden column to store full path
        folder_tree.column("is_template", width=0, stretch=False)  # Hidden column to store if folder is a template
        folder_tree.column("checked", width=0, stretch=False)  # Hidden column to store checkbox state
        
        # Set up headings
        folder_tree.heading("#0", text="Name", anchor=tk.W)
        folder_tree.heading("size", text="Size", anchor=tk.E)
        folder_tree.heading("modified", text="Modified", anchor=tk.W)
        
        # Dictionary to track checked state of template folders
        checked_templates = {}
        
        # Create a mapping of directory paths to their tree IDs
        path_to_id = {}
        
        # Add the root node (output folder)
        root_name = os.path.basename(output_folder)
        root_id = folder_tree.insert("", "end", text=root_name, values=("", "", output_folder, False, False))
        path_to_id[output_folder] = root_id
        
        # Function to determine if a folder is a template folder
        def is_template_folder(folder_name, folder_path):
            # Skip special folders that should never have checkboxes
            if folder_name.lower() == "notemplate":
                return False
                
            # Check if it's the root folder - no checkbox for root
            if folder_path == output_folder:
                return False
                
            # All other folders should have checkboxes for consistency
            # This ensures every folder has a checkbox after analysis
            return True
        
        # Function to get the item count for a folder (number of files)
        def get_item_count(folder_path):
            count = 0
            for _, _, files in os.walk(folder_path):
                count += len(files)
            return count
            
        # Function to format file size
        def format_size(size_bytes):
            if size_bytes < 1024:
                return f"{size_bytes} B"
            elif size_bytes < 1024 * 1024:
                return f"{size_bytes / 1024:.1f} KB"
            else:
                return f"{size_bytes / (1024 * 1024):.1f} MB"
        
        # Function to toggle checkbox state - kept for display purposes but not used for toggling
        def toggle_checkbox(item_id, new_state=None):
            # Get current state
            values = folder_tree.item(item_id, 'values')
            is_template = values[3] == "True"
            is_checked = values[4] == "True"
            
            if is_template:
                # If new_state is not specified, toggle the current state
                if new_state is None:
                    new_state = not is_checked
                
                # Only update if state is different
                if new_state != is_checked:
                    new_values = values[:4] + (str(new_state),)
                    folder_tree.item(item_id, values=new_values)
                    
                    # Update the display text with checkbox
                    folder_name = folder_tree.item(item_id, 'text').split(" [", 1)[0]  # Remove old checkbox
                    if new_state:
                        folder_tree.item(item_id, text=f"{folder_name} [✓]")
                    else:
                        folder_tree.item(item_id, text=f"{folder_name} [ ]")
                    
                    # Store the checked state
                    folder_path = values[2]
                    checked_templates[folder_path] = new_state
        
        # Function to populate the treeview
        def populate_tree(parent_path, parent_id):
            try:
                # Add all immediate subdirectories first
                for item in sorted(os.listdir(parent_path)):
                    item_path = os.path.join(parent_path, item)
                    mod_time = time.strftime("%Y-%m-%d %H:%M:%S", 
                                            time.localtime(os.path.getmtime(item_path)))
                    
                    if os.path.isdir(item_path):
                        # Get item count for the folder
                        item_count = get_item_count(item_path)
                        size_str = f"{item_count} items"
                        
                        # Check if this is a template folder
                        template_folder = is_template_folder(item, item_path)
                        
                        # Add checkbox indicator to template folders
                        display_name = item
                        if template_folder:
                            checked = checked_templates.get(item_path, False)
                            display_name = f"{item} [{'✓' if checked else ' '}]"
                        
                        # Insert directory
                        dir_id = folder_tree.insert(
                            parent_id, "end", text=display_name, 
                            values=(size_str, mod_time, item_path, template_folder, checked_templates.get(item_path, False))
                        )
                        path_to_id[item_path] = dir_id
                        
                        # Recursively populate subdirectories
                        populate_tree(item_path, dir_id)
                    else:
                        # Insert file
                        size = os.path.getsize(item_path)
                        size_str = format_size(size)
                        
                        # Add an icon hint for DXF and log files
                        display_name = item
                        if item.lower().endswith('.dxf'):
                            display_name = f"{item} 📐"  # Add drafting symbol for DXF files
                        elif item.lower().endswith('.log'):
                            display_name = f"{item} 📝"  # Add note symbol for log files
                            
                        folder_tree.insert(parent_id, "end", text=display_name, 
                                          values=(size_str, mod_time, item_path, False, False))
            except Exception as e:
                self._log_message(f"Error populating tree: {str(e)}", error=True)
        
        # Function to handle double-click event
        def on_double_click(event):
            # Get the selected item
            item_id = folder_tree.identify('item', event.x, event.y)
            if not item_id:
                return
                
            # Get the full path of the item
            item_path = folder_tree.item(item_id, 'values')[2]  # Path is in the third column
            
            # If it's a file, open it with the default application
            if os.path.isfile(item_path):
                try:
                    self._log_message(f"Opening file: {item_path}")
                    os.startfile(item_path)
                except Exception as e:
                    self._log_message(f"Error opening file: {str(e)}", error=True)
                    messagebox.showerror("Error Opening File", 
                                        f"Could not open the file with the default application.\n\nError: {str(e)}")
            elif os.path.isdir(item_path):
                # For directories, expand or collapse on double-click
                if folder_tree.item(item_id, 'open'):
                    folder_tree.item(item_id, open=False)
                else:
                    folder_tree.item(item_id, open=True)
        
        # Bind click events - remove single-click binding for checkboxes
        # folder_tree.bind("<Button-1>", on_tree_click) - removed
        folder_tree.bind("<Double-1>", on_double_click)
        
        # Add right-click context menu
        context_menu = tk.Menu(tree_window, tearoff=0)
        
        def on_right_click(event):
            # Get the selected item
            item_id = folder_tree.identify('item', event.x, event.y)
            if not item_id:
                return
                
            # Select the item that was right-clicked
            folder_tree.selection_set(item_id)
                
            # Get the full path of the item
            item_path = folder_tree.item(item_id, 'values')[2]  # Path is in the third column
            
            # Clear the menu
            context_menu.delete(0, tk.END)
            
            # Check if this is a template folder
            is_template = folder_tree.item(item_id, 'values')[3] == "True"
            
            # Add menu items based on file type
            if os.path.isfile(item_path):
                context_menu.add_command(label="Open File", 
                                        command=lambda: open_file(item_path))
                context_menu.add_command(label="Open Containing Folder", 
                                        command=lambda: open_containing_folder(item_path))
            elif os.path.isdir(item_path):
                context_menu.add_command(label="Open Folder", 
                                        command=lambda: open_folder(item_path))
                
                # Restore checkbox toggle options for template folders in right-click menu
                if is_template:
                    is_checked = folder_tree.item(item_id, 'values')[4] == "True"
                    context_menu.add_separator()
                    if is_checked:
                        context_menu.add_command(label="Uncheck", 
                                              command=lambda: toggle_checkbox(item_id, False))
                    else:
                        context_menu.add_command(label="Check", 
                                              command=lambda: toggle_checkbox(item_id, True))
            
            # Display the menu
            context_menu.tk_popup(event.x_root, event.y_root)
        
        def open_file(file_path):
            """Open a file with the default application"""
            try:
                self._log_message(f"Opening file: {file_path}")
                os.startfile(file_path)
            except Exception as e:
                self._log_message(f"Error opening file: {str(e)}", error=True)
                messagebox.showerror("Error Opening File", 
                                    f"Could not open the file with the default application.\n\nError: {str(e)}")
        
        def open_folder(folder_path):
            """Open a folder in Windows Explorer"""
            try:
                self._log_message(f"Opening folder: {folder_path}")
                os.startfile(folder_path)
            except Exception as e:
                self._log_message(f"Error opening folder: {str(e)}", error=True)
                messagebox.showerror("Error Opening Folder", 
                                    f"Could not open the folder in Explorer.\n\nError: {str(e)}")
        
        def open_containing_folder(file_path):
            """Open the folder containing the file and select the file"""
            try:
                folder_path = os.path.dirname(file_path)
                self._log_message(f"Opening containing folder: {folder_path}")
                
                # On Windows, this opens Explorer and selects the file
                import subprocess
                subprocess.Popen(f'explorer /select,"{file_path}"')
            except Exception as e:
                self._log_message(f"Error opening containing folder: {str(e)}", error=True)
                messagebox.showerror("Error Opening Folder", 
                                    f"Could not open the folder in Explorer.\n\nError: {str(e)}")
        
        # Bind right-click event
        folder_tree.bind("<Button-3>", on_right_click)
        
        # Populate the tree
        populate_tree(output_folder, root_id)
        
        # Expand the root node
        folder_tree.item(root_id, open=True)
        
        # Create a button frame at the bottom
        button_frame = ttk.Frame(tree_window)
        button_frame.pack(pady=10)
        
        # Add a refresh button
        def refresh_tree():
            # Save checked state before clearing
            current_checked = checked_templates.copy()
            
            # Clear the treeview
            folder_tree.delete(*folder_tree.get_children())
            
            # Reset path mapping but keep checked state
            path_to_id.clear()
            
            # Add the root node again
            root_id = folder_tree.insert("", "end", text=root_name, values=("", "", output_folder, False, False))
            path_to_id[output_folder] = root_id
            
            # Restore checked templates dictionary
            checked_templates.clear()
            checked_templates.update(current_checked)
            
            # Repopulate the tree
            populate_tree(output_folder, root_id)
            
            # Expand the root node
            folder_tree.item(root_id, open=True)
            
            self._log_message("Folder structure view refreshed")
        
        # Add buttons
        ttk.Button(button_frame, text="Refresh", command=refresh_tree).pack(side=tk.LEFT, padx=5)
        ttk.Button(button_frame, text="Close", command=tree_window.destroy).pack(side=tk.LEFT, padx=5)

    def _check_for_resumable_session(self) -> None:
        """Check if there's a session to resume and update button states accordingly."""
        if self._load_session_state():
            self.resume_button.config(state=tk.NORMAL)
        else:
            self.resume_button.config(state=tk.DISABLED)

    def _resume_comparison(self) -> None:
        """Resume the comparison process from a saved session."""
        if not self._load_session_state():
            self._log_message("No valid session to resume", error=True)
            messagebox.showerror("Error Resuming Session", "No valid session to resume.")
            return
            
        # Validate that the folders still exist
        if not os.path.isdir(self.template_folder.get()):
            self._log_message(f"Error: Template folder '{self.template_folder.get()}' does not exist", error=True)
            messagebox.showerror("Error Resuming Session", f"Template folder '{self.template_folder.get()}' does not exist.")
            return
            
        if not os.path.isdir(self.search_folder.get()):
            self._log_message(f"Error: Search folder '{self.search_folder.get()}' does not exist", error=True)
            messagebox.showerror("Error Resuming Session", f"Search folder '{self.search_folder.get()}' does not exist.")
            return
            
        if not os.path.isdir(self.output_folder.get()):
            self._log_message(f"Error: Output folder '{self.output_folder.get()}' does not exist", error=True)
            messagebox.showerror("Error Resuming Session", f"Output folder '{self.output_folder.get()}' does not exist.")
            return
        
        # Reset the stop flag
        self.should_stop = False
        
        # Update UI state
        self.is_comparing = True
        self.start_button.config(state=tk.DISABLED)
        self.resume_button.config(state=tk.DISABLED)
        self.stop_button.config(state=tk.NORMAL)
        self.clear_button.config(state=tk.DISABLED)
        self.view_structure_button.config(state=tk.DISABLED)
        self.status_var.set("Resuming...")
        
        # Set progress bars to saved values
        if 'template_progress' in self.session_state:
            self.template_progress['value'] = self.session_state['template_progress']
            
        if 'comparison_progress' in self.session_state:
            self.comparison_progress['value'] = self.session_state['comparison_progress']
        
        # Log resume message
        self._log_message("Resuming comparison process...")
        
        # Start time tracking - adjust for previous elapsed time
        self.start_time = time.time() - self.session_state.get('elapsed_seconds', 0)
        self.timer_running = True
        self._update_time_display()
        
        # Initialize session save timer
        self.session_save_timer = self.root.after(30000, self._save_session_state)
        
        # Check which phase to resume
        phase = self.session_state.get('phase')
        
        if phase == 'template':
            # Resume template mapping
            template_count = count_files_in_directory(self.template_folder.get(), ".dxf", self.recursive_search.get())
            self._log_message(f"Resuming template mapping. Found {template_count} template files.")
            
            self.template_thread = threading.Thread(
                target=self._build_template_map_thread,
                args=(
                    self.template_folder.get(),
                    self.recursive_search.get(),
                    template_count
                )
            )
            self.template_thread.daemon = True
            self.template_thread.start()
            
        elif phase == 'comparison':
            # Need to rebuild template map first
            self._log_message("Rebuilding template map from previous session...")
            
            def load_template_map_and_resume():
                # Try to load template map from cache
                template_dir = self.template_folder.get()
                template_map, cache_valid, _ = load_template_map_from_cache(template_dir)
                
                if not cache_valid:
                    # Rebuild the template map
                    template_count = count_files_in_directory(template_dir, ".dxf", self.recursive_search.get())
                    
                    # Pass a callback to update progress
                    def progress_callback(current, total):
                        gui_queue.put(("template_progress", current, total))
                    
                    template_map = build_template_map(
                        template_dir, 
                        recursive=self.recursive_search.get(),
                        progress_callback=progress_callback
                    )
                
                # Log completion
                gui_queue.put(("log", f"Template mapping complete. Found {len(template_map)} unique templates."))
                gui_queue.put(("template_complete", None))
                
                # Resume comparison thread
                search_folder = self.search_folder.get()
                output_folder = self.output_folder.get()
                
                search_count = count_files_in_directory(search_folder, ".dxf", self.recursive_search.get())
                self._log_message(f"Resuming comparison. Found {search_count} files to check.")
                
                # Get list of already processed files
                processed_files = self.session_state.get('processed_files', [])
                
                # Start comparison thread with resume info
                comparison_thread = threading.Thread(
                    target=self._run_comparison_thread,
                    args=(
                        template_map,
                        search_folder,
                        output_folder,
                        self.recursive_search.get(),
                        self.move_files.get(),
                        self.group_by_content.get(),
                        processed_files  # Pass the list of already processed files
                    )
                )
                comparison_thread.daemon = True
                comparison_thread.start()
            
            # Start a thread to load the template map and resume comparison
            template_load_thread = threading.Thread(target=load_template_map_and_resume)
            template_load_thread.daemon = True
            template_load_thread.start()
            
        else:
            # Unknown phase, start from scratch
            self._log_message("Could not determine resume point, starting from beginning...")
            self._start_comparison() 