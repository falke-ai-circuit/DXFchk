package compare

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/falke-ai-circuit/DXFchk/internal/dxf"
)

// BuildTemplateMap scans a folder for DXF files and builds a map of
// template_name → file_path by reading the $(TEMPLATE) attribute from each.
//
// Port of Python template_manager.build_template_map()
func BuildTemplateMap(templateDir string, recursive bool, progressFn func(int, int) bool) TemplateMap {
	templateMap := make(TemplateMap)

	// Find all DXF files
	var dxfFiles []string
	if recursive {
		filepath.Walk(templateDir, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".dxf") {
				dxfFiles = append(dxfFiles, path)
			}
			return nil
		})
	} else {
		entries, _ := os.ReadDir(templateDir)
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".dxf") {
				dxfFiles = append(dxfFiles, filepath.Join(templateDir, e.Name()))
			}
		}
	}

	total := len(dxfFiles)
	for i, f := range dxfFiles {
		if progressFn != nil && i%5 == 0 {
			if !progressFn(i, total) {
				return templateMap
			}
		}

		// Parse DXF and extract $(TEMPLATE) attribute
		// Recover from panics on malformed files
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Skip this file on panic
				}
			}()
			drawing, err := dxf.ReadFile(f)
			if err != nil {
				return
			}
			templateName := drawing.GetTemplateAttribute()
			if templateName != "" {
				templateMap[templateName] = f
			}
		}()
	}

	if progressFn != nil {
		progressFn(total, total)
	}

	return templateMap
}

// GetTemplateName extracts the $(TEMPLATE) attribute from a DXF file
// Port of Python template_manager.get_template_name_from_dxf()
func GetTemplateName(dxfPath string) (string, error) {
	drawing, err := dxf.ReadFile(dxfPath)
	if err != nil {
		return "", err
	}
	return drawing.GetTemplateAttribute(), nil
}