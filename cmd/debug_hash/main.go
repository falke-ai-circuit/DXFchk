package main

import (
	"fmt"
	"os"
	"strings"
	"math"
	"sort"
	"strconv"
	"path/filepath"
	
	"github.com/falke-ai-circuit/DXFchk/internal/dxf"
)

func main() {
	dxfPath := os.Args[1]
	drawing, err := dxf.ReadFile(dxfPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	
	content := drawing.ExtractContent(3)
	
	// Print the JSON string that ContentHash would produce
	jsonStr := pythonJSON(content.Blocks, content.Lines, content.Polylines)
	
	// Write to file
	outPath := filepath.Join(filepath.Dir(dxfPath), "..", "go_hash.json")
	os.WriteFile(outPath, []byte(jsonStr), 0644)
	
	fmt.Printf("Written %d chars to %s\n", len(jsonStr), outPath)
	fmt.Printf("Blocks: %d types, %d total\n", len(content.Blocks), countBlocks(content.Blocks))
	fmt.Printf("Lines: %d layers, %d total\n", len(content.Lines), countLines(content.Lines))
	fmt.Printf("Polylines: %d layers, %d total\n", len(content.Polylines), countPolylines(content.Polylines))
	
	// Print first 500 chars
	if len(jsonStr) > 500 {
		fmt.Printf("First 500: %s\n", jsonStr[:500])
	}
}

func countBlocks(m map[string][][3]float64) int {
	n := 0
	for _, v := range m { n += len(v) }
	return n
}
func countLines(m map[string][][2][3]float64) int {
	n := 0
	for _, v := range m { n += len(v) }
	return n
}
func countPolylines(m map[string][][][3]float64) int {
	n := 0
	for _, v := range m { n += len(v) }
	return n
}

func pythonFloat(f float64) string {
	if f == math.Trunc(f) && !math.IsInf(f, 0) && !math.IsNaN(f) {
		return fmt.Sprintf("%.1f", f)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

func pythonJSON(blocks map[string][][3]float64, lines map[string][][2][3]float64, polylines map[string][][][3]float64) string {
	var sb strings.Builder
	sb.WriteByte('[')
	sb.WriteString(pythonJSONBlocks(blocks))
	sb.WriteString(", ")
	sb.WriteString(pythonJSONLines(lines))
	sb.WriteString(", ")
	sb.WriteString(pythonJSONPolylines(polylines))
	sb.WriteByte(']')
	return sb.String()
}

func pythonJSONBlocks(m map[string][][3]float64) string {
	keys := make([]string, 0, len(m))
	for k := range m { keys = append(keys, k) }
	sort.Strings(keys)
	var sb strings.Builder
	sb.WriteByte('{')
	for i, k := range keys {
		if i > 0 { sb.WriteString(", ") }
		sb.WriteString(strconv.Quote(k))
		sb.WriteString(": ")
		sb.WriteByte('[')
		for j, v := range m[k] {
			if j > 0 { sb.WriteString(", ") }
			sb.WriteString("[")
			sb.WriteString(pythonFloat(v[0]))
			sb.WriteString(", ")
			sb.WriteString(pythonFloat(v[1]))
			sb.WriteString(", ")
			sb.WriteString(pythonFloat(v[2]))
			sb.WriteString("]")
		}
		sb.WriteByte(']')
	}
	sb.WriteByte('}')
	return sb.String()
}

func pythonJSONLines(m map[string][][2][3]float64) string {
	keys := make([]string, 0, len(m))
	for k := range m { keys = append(keys, k) }
	sort.Strings(keys)
	var sb strings.Builder
	sb.WriteByte('{')
	for i, k := range keys {
		if i > 0 { sb.WriteString(", ") }
		sb.WriteString(strconv.Quote(k))
		sb.WriteString(": ")
		sb.WriteByte('[')
		for j, pair := range m[k] {
			if j > 0 { sb.WriteString(", ") }
			sb.WriteString("[[")
			sb.WriteString(pythonFloat(pair[0][0]))
			sb.WriteString(", ")
			sb.WriteString(pythonFloat(pair[0][1]))
			sb.WriteString(", ")
			sb.WriteString(pythonFloat(pair[0][2]))
			sb.WriteString("], [")
			sb.WriteString(pythonFloat(pair[1][0]))
			sb.WriteString(", ")
			sb.WriteString(pythonFloat(pair[1][1]))
			sb.WriteString(", ")
			sb.WriteString(pythonFloat(pair[1][2]))
			sb.WriteString("]]")
		}
		sb.WriteByte(']')
	}
	sb.WriteByte('}')
	return sb.String()
}

func pythonJSONPolylines(m map[string][][][3]float64) string {
	keys := make([]string, 0, len(m))
	for k := range m { keys = append(keys, k) }
	sort.Strings(keys)
	var sb strings.Builder
	sb.WriteByte('{')
	for i, k := range keys {
		if i > 0 { sb.WriteString(", ") }
		sb.WriteString(strconv.Quote(k))
		sb.WriteString(": ")
		sb.WriteByte('[')
		for j, poly := range m[k] {
			if j > 0 { sb.WriteString(", ") }
			sb.WriteByte('[')
			for l, v := range poly {
				if l > 0 { sb.WriteString(", ") }
				sb.WriteString("[")
				sb.WriteString(pythonFloat(v[0]))
				sb.WriteString(", ")
				sb.WriteString(pythonFloat(v[1]))
				sb.WriteString(", ")
				sb.WriteString(pythonFloat(v[2]))
				sb.WriteString("]")
			}
			sb.WriteByte(']')
		}
		sb.WriteByte(']')
	}
	sb.WriteByte('}')
	return sb.String()
}