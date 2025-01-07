package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/CycloneDX/cyclonedx-go"
)

func main() {
	filePath := "./ivs.json"
	repo := "pypi"

	// Parse the CycloneDX SBOM
	bom, err := CDXParser(filePath)
	if err != nil {
		fmt.Printf("Error parsing CycloneDX SBOM: %v\n", err)
		return
	}

	// Create a new MavenRepo
	mavenRepo, err := NewMavenRepo()
	if err != nil {
		fmt.Printf("Error creating MavenRepo: %v\n", err)
		return
	}

	// Create a new PubDevRepo
	pubDevRepo := NewPubDevRepo()

	// Create a new PypiRepo
	pypiRepo := NewPypiRepo()

	// Use WaitGroup and Mutex to manage goroutines and results
	var wg sync.WaitGroup
	var mu sync.Mutex
	var components []Component
	var errors []error

	for _, component := range *bom.Components {
		wg.Add(1)

		go func(comp cyclonedx.Component) {
			defer wg.Done()
			visited := make(map[string]bool)

			// Fetch component info
			if repo == "maven" {
				fetchedComponent, err := mavenRepo.GetComponentInfo(comp.Group, comp.Name, comp.Version, visited)
				if err != nil {
					// Safely append to the errors slice
					mu.Lock()
					errors = append(errors, fmt.Errorf("error for component '%s': %w", comp.Name, err))
					mu.Unlock()
					return
				}

				// Safely append to the result slice
				mu.Lock()
				components = append(components, *fetchedComponent)
				mu.Unlock()

			} else if repo == "pubdev" {
				// Fetch pubdev component info
				fetchedComponent, err := pubDevRepo.GetComponentInfo(comp.Name, comp.Version, visited)
				if err != nil {
					// Safely append to the errors slice
					mu.Lock()
					errors = append(errors, fmt.Errorf("error for component '%s': %w", comp.Name, err))
					mu.Unlock()
					return
				}

				// Safely append to the result slice
				mu.Lock()
				components = append(components, *fetchedComponent)
				mu.Unlock()
			} else if repo == "pypi" {
				// Fetch pypi component info
				fetchedComponent, err := pypiRepo.GetComponentInfo(comp.Name, comp.Version, visited)
				if err != nil {
					// Safely append to the errors slice
					mu.Lock()
					errors = append(errors, fmt.Errorf("error for component '%s': %w", comp.Name, err))
					mu.Unlock()
					return
				}

				// Safely append to the result slice
				mu.Lock()
				components = append(components, *fetchedComponent)
				mu.Unlock()
			} else {
				fmt.Printf("Unsupported repository: %s\n", repo)
			}
		}(component)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Log all fetched components
	log.Println("Fetched Components:")
	for _, comp := range components {
		log.Printf("Component: %+v\n", comp)
	}

	// Log all errors
	if len(errors) > 0 {
		log.Println("Errors encountered:")
		for _, err := range errors {
			log.Printf("Error: %v\n", err)
		}
	} else {
		log.Println("No errors encountered.")
	}

	// Generate CSV file
	outputFile := "output.csv"
	csvFile, err := os.Create(outputFile)
	if err != nil {
		fmt.Printf("Failed to create CSV file: %v\n", err)
		return
	}
	defer csvFile.Close()

	// Create a new CSV writer
	csvFile.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(csvFile)
	defer writer.Flush()

	// Write CSV header
	csvHeader := []string{}
	if repo == "maven" {
		csvHeader = []string{"Component Name", "Timestamp", "Supplier Name", "Version String", "Author", "Hash (MD5)", "UID", "Relationship"}
	} else if repo == "pubdev" {
		csvHeader = []string{"Component Name", "Timestamp", "Supplier Name", "Version String", "Author", "Hash (SHA-256)", "UID", "Relationship"}
	} else if repo == "pypi" {
		csvHeader = []string{"Component Name", "Timestamp", "Supplier Name", "Version String", "Author", "Hash (SHA-256)", "UID", "Relationship"}
	}
	writer.Write(csvHeader)

	// Append components to CSV data
	csvData := [][]string{}
	for _, rootComponent := range components {
		AppendComponentToCsv(rootComponent, &csvData, 0)
	}

	// Write CSV data
	for _, row := range csvData {
		writer.Write(row)
	}

	fmt.Printf("CSV Output File: %s\n", outputFile)

	// JSON 파일 생성
	jsonOutputFile := "output.json"
	err = SaveComponentsAsJSON(components, jsonOutputFile)
	if err != nil {
		fmt.Printf("Failed to save JSON file: %v\n", err)
		return
	}

	fmt.Printf("JSON Output File: %s\n", jsonOutputFile)
}
