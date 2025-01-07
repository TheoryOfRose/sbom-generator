package main

import (
	"fmt"
	"os"

	"github.com/CycloneDX/cyclonedx-go"
)

func CDXParser(filePath string) (*cyclonedx.BOM, error) {
	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		return nil, err
	}
	defer file.Close()

	// Parse the CycloneDX SBOM
	var bom cyclonedx.BOM
	decoder := cyclonedx.NewBOMDecoder(file, cyclonedx.BOMFileFormatJSON)
	if err := decoder.Decode(&bom); err != nil {
		fmt.Printf("Error decoding BOM: %v\n", err)
		return nil, err
	}

	return &bom, nil
}
