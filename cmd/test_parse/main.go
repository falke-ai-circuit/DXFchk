package main

import (
	"fmt"
	"os"

	"github.com/falke-ai-circuit/DXFchk/internal/dxf"
	"github.com/falke-ai-circuit/DXFchk/internal/compare"
)

func main() {
	// Test with template file
	templateFile := "/tmp/AA.dxf"
	moduleFile := "/tmp/284_p1010.dxf"

	if len(os.Args) > 1 {
		templateFile = os.Args[1]
	}
	if len(os.Args) > 2 {
		moduleFile = os.Args[2]
	}

	// Test 1: Parse template DXF
	fmt.Println("=== Test 1: Parse template DXF ===")
	drawing, err := dxf.ReadFile(templateFile)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Count entity types
	typeCounts := make(map[string]int)
	for _, e := range drawing.Entities {
		typeCounts[e.Type]++
	}
	fmt.Printf("Entity types:\n")
	for t, c := range typeCounts {
		fmt.Printf("  %s: %d\n", t, c)
	}

	// Look for INSERT entities with ATTRIBs
	insertCount := 0
	templateFound := ""
	for _, e := range drawing.Entities {
		if e.Type == "INSERT" {
			insertCount++
			blockName := e.GetStringValue(2)
			if len(e.Attribs) > 0 {
				for _, att := range e.Attribs {
					tag := att.GetStringValue(2)
					text := att.GetStringValue(1)
					if tag == "$(TEMPLATE)" {
						templateFound = text
						fmt.Printf("  INSERT '%s' has $(TEMPLATE) = '%s'\n", blockName, text)
					}
				}
			}
		}
	}
	fmt.Printf("Total INSERTs: %d\n", insertCount)
	fmt.Printf("Template attribute: '%s'\n", templateFound)

	// Test 2: Extract content
	fmt.Println("\n=== Test 2: Extract content ===")
	content := drawing.ExtractContent(3)
	fmt.Printf("Blocks: %d types\n", len(content.Blocks))
	fmt.Printf("Lines: %d layers\n", len(content.Lines))
	fmt.Printf("Polylines: %d layers\n", len(content.Polylines))
	fmt.Printf("Template: '%s'\n", content.Template)

	// Show block names
	fmt.Println("Block names:")
	for name, positions := range content.Blocks {
		fmt.Printf("  %s: %d instances\n", name, len(positions))
	}

	// Test 3: Content hash
	fmt.Println("\n=== Test 3: Content hash ===")
	hash := compare.ContentHash(content)
	fmt.Printf("Hash: %s\n", hash)

	// Test 4: Get template name from module
	fmt.Println("\n=== Test 4: Parse module DXF ===")
	moduleDrawing, err := dxf.ReadFile(moduleFile)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	moduleTemplate := moduleDrawing.GetTemplateAttribute()
	fmt.Printf("Module template attribute: '%s'\n", moduleTemplate)

	moduleContent := moduleDrawing.ExtractContent(3)
	fmt.Printf("Module blocks: %d types\n", len(moduleContent.Blocks))
	fmt.Printf("Module lines: %d layers\n", len(moduleContent.Lines))
	fmt.Printf("Module polylines: %d layers\n", len(moduleContent.Polylines))

	moduleHash := compare.ContentHash(moduleContent)
	fmt.Printf("Module hash: %s\n", moduleHash)

	// Test 5: Build template map from TemplatesEclipse folder
	fmt.Println("\n=== Test 5: Build template map ===")
	templateDir := "/tmp/TemplatesEclipse"
	if _, err := os.Stat(templateDir); err == nil {
		tmap := compare.BuildTemplateMap(templateDir, false, func(done, total int) bool {
			if done%100 == 0 {
				fmt.Printf("  Scanned %d/%d templates...\n", done, total)
			}
			return true
		})
		fmt.Printf("Template map: %d entries\n", len(tmap))
		// Show first 10
		count := 0
		for name, path := range tmap {
			if count >= 10 {
				break
			}
			fmt.Printf("  %s -> %s\n", name, path)
			count++
		}
	} else {
		fmt.Println("  (template dir not available locally)")
	}

	fmt.Println("\n=== All tests passed ===")
}