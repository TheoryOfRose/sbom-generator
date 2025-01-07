package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func SaveComponentsAsJSON(components []Component, outputFile string) error {
	// JSON 변환
	jsonData, err := json.MarshalIndent(components, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to convert components to JSON: %w", err)
	}

	// JSON 파일 저장
	err = os.WriteFile(outputFile, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("failed to save JSON file: %w", err)
	}

	return nil
}
