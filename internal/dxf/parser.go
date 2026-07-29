package dxf

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// CodePair represents a single DXF code/value pair
type CodePair struct {
	Code  int
	Value string // raw string value, convert as needed
}

// Entity is a collection of code pairs belonging to one DXF entity
type Entity struct {
	Type string
	Pairs []CodePair
	// ATTRIB entities that follow an INSERT (when code 66 = 1)
	Attribs []Entity
}

// Drawing represents a parsed DXF file
type Drawing struct {
	Entities []Entity
}

// ReadFile reads a DXF file from disk
func ReadFile(path string) (*Drawing, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening DXF file: %w", err)
	}
	defer f.Close()
	return ReadFromReader(f)
}

// ReadFromReader reads a DXF file from any io.Reader
func ReadFromReader(r io.Reader) (*Drawing, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024) // 10MB max line

	var pairs []CodePair
	for scanner.Scan() {
		codeLine := scanner.Text()
		if !scanner.Scan() {
			break
		}
		valueLine := scanner.Text()

		code, err := strconv.Atoi(strings.TrimSpace(codeLine))
		if err != nil {
			continue // skip malformed lines
		}
		pairs = append(pairs, CodePair{Code: code, Value: strings.TrimRight(valueLine, "\r")})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading DXF: %w", err)
	}

	return parsePairs(pairs), nil
}

// parsePairs converts raw code pairs into structured entities
func parsePairs(pairs []CodePair) *Drawing {
	d := &Drawing{}
	var currentEntity *Entity
	var currentInsert *Entity // track INSERT for ATTRIB collection
	inEntities := false

	for i := 0; i < len(pairs); i++ {
		p := pairs[i]

		// Track sections
		if p.Code == 0 && p.Value == "SECTION" {
			if i+1 < len(pairs) && pairs[i+1].Code == 2 {
				if strings.TrimSpace(pairs[i+1].Value) == "ENTITIES" {
					inEntities = true
				}
			}
			continue
		}
		if p.Code == 0 && p.Value == "ENDSEC" {
			inEntities = false
			continue
		}
		if p.Code == 0 && p.Value == "EOF" {
			break
		}

		if !inEntities {
			continue
		}

		// Code 0 = new entity
		if p.Code == 0 {
			entityType := strings.TrimSpace(p.Value)

			// If we were collecting ATTRIBs for an INSERT, check if done
			if currentInsert != nil && entityType != "ATTRIB" {
				currentInsert = nil
			}

			if entityType == "ATTRIB" && currentInsert != nil {
				// This ATTRIB belongs to the current INSERT
				attrib := Entity{Type: "ATTRIB"}
				attrib.Pairs = collectEntityPairs(pairs, &i)
				currentInsert.Attribs = append(currentInsert.Attribs, attrib)
				i-- // adjust for loop increment
			} else {
				// New entity
				currentEntity = &Entity{Type: entityType}
				currentEntity.Pairs = collectEntityPairs(pairs, &i)
				i-- // adjust for loop increment
				d.Entities = append(d.Entities, *currentEntity)

				if entityType == "INSERT" {
					// Check if this INSERT has attributes (code 66 = 1)
					hasAttribs := false
					for _, cp := range currentEntity.Pairs {
						if cp.Code == 66 {
							val := strings.TrimSpace(cp.Value)
							if val == "1" {
								hasAttribs = true
							}
							break
						}
					}
					if hasAttribs {
							currentInsert = &d.Entities[len(d.Entities)-1]
						}
					}
					}
					}
	}

	return d
}

// collectEntityPairs reads code pairs until the next code=0 entity
func collectEntityPairs(pairs []CodePair, i *int) []CodePair {
	*i++ // move past the code=0 line
	var result []CodePair
	for *i < len(pairs) {
		if pairs[*i].Code == 0 {
			*i-- // back up so the main loop sees the code=0
			break
		}
		result = append(result, pairs[*i])
		*i++
	}
	return result
}

// GetStringValue gets a string code pair value, returns "" if not found
func (e *Entity) GetStringValue(code int) string {
	for _, p := range e.Pairs {
		if p.Code == code {
			return strings.TrimSpace(p.Value)
		}
	}
	return ""
}

// GetFloatValue gets a float code pair value, returns 0.0 if not found
func (e *Entity) GetFloatValue(code int) float64 {
	for _, p := range e.Pairs {
		if p.Code == code {
			v, err := strconv.ParseFloat(strings.TrimSpace(p.Value), 64)
			if err == nil {
				return v
			}
		}
	}
	return 0.0
}

// GetAttribValue returns the text value of an ATTRIB with the given tag
func (e *Entity) GetAttribValue(tag string) string {
	for _, att := range e.Attribs {
		// ATTRIB tag is in code 2
		attTag := att.GetStringValue(2)
		if strings.EqualFold(attTag, tag) {
			// ATTRIB text value is in code 1
			return att.GetStringValue(1)
		}
	}
	return ""
}