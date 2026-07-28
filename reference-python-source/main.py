#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
DXF Comparison Tool - Main Entry Point

This is the main entry point for the DXF Comparison Tool, which compares DXF files
against templates and organizes them by similarities and differences.
"""

import tkinter as tk
from gui import DXFCompareApp

def main():
    """Initialize and run the application."""
    root = tk.Tk()
    app = DXFCompareApp(root)
    root.mainloop()

if __name__ == "__main__":
    main() 