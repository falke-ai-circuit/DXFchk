package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/falke-ai-circuit/DXFchk/internal/api"
	"github.com/falke-ai-circuit/DXFchk/internal/compare"
)

var cliVersion = "v0.2.0"

func main() {
	// Subcommand: "cli" for headless comparison, default (no subcommand) starts web server
	if len(os.Args) > 1 && os.Args[1] == "cli" {
		os.Args = append(os.Args[:1], os.Args[2:]...) // remove "cli" from args
		runCLI()
		return
	}
	runServer()
}

func runServer() {
	port := flag.Int("port", 8643, "HTTP server port")
	flag.Parse()

	server := api.NewServer()

	addr := fmt.Sprintf(":%d", *port)
	fmt.Printf("DXFchk %s starting on http://localhost%s\n", cliVersion, addr)
	fmt.Printf("API: http://localhost:%d/api/v1/health\n", *port)
	fmt.Printf("CLI mode: dxfchk cli -templates <dir> -search <dir>\n")

	if err := http.ListenAndServe(addr, server); err != nil {
		fmt.Fprintf(os.Stderr, "Server failed: %v\n", err)
		os.Exit(1)
	}
}

func runCLI() {
	var (
		templateDir = flag.String("templates", "", "Template folder containing DXF templates")
		searchDir    = flag.String("search", "", "Search folder containing DXF modules to compare")
		outputDir    = flag.String("output", "", "Output folder for results (default: <search>/DXFchk_output)")
		recursive    = flag.Bool("recursive", true, "Search subdirectories")
		groupContent = flag.Bool("group", true, "Group different files by content hash into _modN folders")
		verbose      = flag.Bool("verbose", false, "Print detailed logs")
		jsonOutput   = flag.Bool("json", false, "Output results as JSON")
	)
	flag.Parse()

	if *templateDir == "" || *searchDir == "" {
		fmt.Fprintln(os.Stderr, "DXFchk CLI — headless DXF comparison")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Usage: dxfchk cli -templates <dir> -search <dir> [-output <dir>] [options]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Options:")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Validate folders
	if _, err := os.Stat(*templateDir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: template folder does not exist: %s\n", *templateDir)
		os.Exit(1)
	}
	if _, err := os.Stat(*searchDir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: search folder does not exist: %s\n", *searchDir)
		os.Exit(1)
	}

	outDir := *outputDir
	if outDir == "" {
		outDir = filepath.Join(*searchDir, "DXFchk_output")
	}
	os.MkdirAll(outDir, 0755)

	// Log function
	logFn := func(msg string) {
		if *verbose {
			fmt.Fprintln(os.Stderr, msg)
		}
	}

	startTime := time.Now()

	// Step 1: Build template map
	fmt.Fprintf(os.Stderr, "Scanning templates in: %s\n", *templateDir)
	templateMap := compare.BuildTemplateMap(*templateDir, *recursive, nil)
	fmt.Fprintf(os.Stderr, "Found %d templates\n", len(templateMap))
	if *verbose {
		for name, path := range templateMap {
			fmt.Fprintf(os.Stderr, "  %s -> %s\n", name, filepath.Base(path))
		}
	}

	if len(templateMap) == 0 {
		fmt.Fprintln(os.Stderr, "Error: no templates found")
		os.Exit(1)
	}

	// Step 2: Run comparison
	fmt.Fprintf(os.Stderr, "\nComparing DXF files in: %s\n", *searchDir)
	fmt.Fprintf(os.Stderr, "Output folder: %s\n", outDir)
	fmt.Fprintf(os.Stderr, "Options: recursive=%v, group_by_content=%v\n\n", *recursive, *groupContent)

	results := compare.RunComparison(
		templateMap,
		*searchDir,
		outDir,
		*recursive,
		false, // move_files=false (preserve originals)
		*groupContent,
		nil, // no progress callback
		logFn,
	)

	elapsed := time.Since(startTime)

	// Summary
	matched := 0
	different := 0
	noTemplate := 0
	for _, r := range results {
		switch r.Status {
		case "match":
			matched++
		case "different":
			different++
		case "no_template":
			noTemplate++
		}
	}

	if *jsonOutput {
		output := map[string]any{
			"version":       cliVersion,
			"template_dir":  *templateDir,
			"search_dir":    *searchDir,
			"output_dir":    outDir,
			"recursive":     *recursive,
			"group_content": *groupContent,
			"total_files":   len(results),
			"matched":       matched,
			"different":     different,
			"no_template":   noTemplate,
			"elapsed_ms":    elapsed.Milliseconds(),
			"results":       results,
		}
		json.NewEncoder(os.Stdout).Encode(output)
	} else {
		fmt.Fprintf(os.Stderr, "\n========================================\n")
		fmt.Fprintf(os.Stderr, "DXFchk %s — Comparison Complete\n", cliVersion)
		fmt.Fprintf(os.Stderr, "========================================\n")
		fmt.Fprintf(os.Stderr, "Total files processed: %d\n", len(results))
		fmt.Fprintf(os.Stderr, "  Matched:     %d\n", matched)
		fmt.Fprintf(os.Stderr, "  Different:   %d\n", different)
		fmt.Fprintf(os.Stderr, "  No template: %d\n", noTemplate)
		fmt.Fprintf(os.Stderr, "Elapsed time: %s\n", elapsed.Round(time.Millisecond))
		fmt.Fprintf(os.Stderr, "Output folder: %s\n", outDir)

		// Print output folder structure
		fmt.Fprintf(os.Stderr, "\nOutput structure:\n")
		printFolderTree(outDir, 0)
	}
}

func printFolderTree(dir string, level int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	// Count files and dirs
	var dirs, files []os.DirEntry
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e)
		} else {
			files = append(files, e)
		}
	}

	indent := strings.Repeat("  ", level)

	// Print dirs first (sorted)
	for _, d := range dirs {
		name := d.Name()
		// Count files in this subfolder
		subDir := filepath.Join(dir, name)
		subEntries, _ := os.ReadDir(subDir)
		fileCount := 0
		for _, se := range subEntries {
			if !se.IsDir() && strings.HasSuffix(strings.ToLower(se.Name()), ".dxf") {
				fileCount++
			}
		}
		if fileCount > 0 {
			fmt.Fprintf(os.Stderr, "%s%s/ (%d DXF files)\n", indent, name, fileCount)
		} else {
			// Check for nested dirs
			hasNestedDirs := false
			for _, se := range subEntries {
				if se.IsDir() {
					hasNestedDirs = true
					break
				}
			}
			if hasNestedDirs {
				fmt.Fprintf(os.Stderr, "%s%s/\n", indent, name)
			} else {
				fmt.Fprintf(os.Stderr, "%s%s/ (empty)\n", indent, name)
			}
		}
		printFolderTree(subDir, level+1)
	}
}