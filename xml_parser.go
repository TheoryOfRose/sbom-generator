package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"golang.org/x/net/html/charset"
)

func ParseXML(data []byte) (map[string]interface{}, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))

	// Set a CharsetReader to handle non-UTF-8 encodings
	decoder.CharsetReader = charset.NewReaderLabel

	result := make(map[string]interface{})
	stack := []string{}
	nestedData := make(map[string][]string)

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error decoding XML: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			// Push the current element to the stack
			stack = append(stack, t.Name.Local)

		case xml.EndElement:
			// Pop the last element from the stack
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}

		case xml.CharData:
			// Get the current path
			content := string(bytes.TrimSpace(t))
			if content != "" {
				path := "/" + strings.Join(stack, "/")

				// If the path already exists, append to the slice
				if existing, ok := nestedData[path]; ok {
					nestedData[path] = append(existing, content)
				} else {
					// Otherwise, create a new slice
					nestedData[path] = []string{content}
				}
			}
		}
	}

	// Convert nestedData to result for compatibility
	for key, values := range nestedData {
		if len(values) == 1 {
			result[key] = values[0]
		} else {
			result[key] = values
		}
	}

	return result, nil
}

func ParseXML2(data []byte) (map[string]interface{}, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	result := make(map[string]interface{})
	stack := []string{}
	currentList := make(map[string][]map[string]string) // To store repeated elements as slices

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error decoding XML: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			stack = append(stack, t.Name.Local)

			// Initialize a slice for repeated elements
			key := "/" + strings.Join(stack, "/")
			if _, exists := currentList[key]; !exists {
				currentList[key] = []map[string]string{}
			}

		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}

		case xml.CharData:
			content := string(bytes.TrimSpace(t))
			if len(stack) > 0 && content != "" {
				parentKey := "/" + strings.Join(stack[:len(stack)-1], "/")
				currentKey := "/" + strings.Join(stack, "/")

				// Handle repeated elements
				if _, exists := currentList[parentKey]; exists {
					item := make(map[string]string)
					item[stack[len(stack)-1]] = content
					currentList[parentKey] = append(currentList[parentKey], item)
				}

				result[currentKey] = content
			}
		}
	}

	// Merge collected slices into the result map
	for key, list := range currentList {
		result[key] = list
	}

	return result, nil
}

// Helper function to join element names as a path
func joinPath(stack []string) string {
	return strings.Join(stack, "/")
}
