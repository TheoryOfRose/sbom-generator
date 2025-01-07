package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

type PypiRepo struct {
	BaseUrl string
}

func NewPypiRepo() *PypiRepo {
	return &PypiRepo{
		BaseUrl: "https://pypi.org/pypi",
	}
}

func (p *PypiRepo) GetComponentInfo(name, version string, visited map[string]bool) (*Component, error) {
	// 순환 참조 방지
	componentKey := fmt.Sprintf("%s@%s", name, version)
	if visited[componentKey] {
		fmt.Printf("Circular dependency detected for package: %s\n", componentKey)
		return nil, fmt.Errorf("circular dependency detected for package: %s", componentKey)
	}
	visited[componentKey] = true

	// PyPI API 요청 URL 생성
	url := fmt.Sprintf("%s/%s/%s/json", p.BaseUrl, name, version)

	// PyPI API 요청
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch package info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch package info: status code %d", resp.StatusCode)
	}

	// Content-Type 확인
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		return nil, fmt.Errorf("unexpected content type: %s", contentType)
	}

	// JSON 응답 파싱
	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %w in %s", err, url)
	}

	info := data["info"].(map[string]interface{})
	urls := data["urls"].([]interface{})
	requiresDist, _ := info["requires_dist"].([]interface{})

	// 작성자 정보
	author := "Unknown"
	if a, ok := info["author"].(string); ok && a != "" {
		author = a
	}

	// SHA256 해시값 및 타임스탬프 추출
	var hash, timestamp string
	if len(urls) > 0 {
		firstURL := urls[0].(map[string]interface{})
		hash = firstURL["digests"].(map[string]interface{})["sha256"].(string)
		timestamp = firstURL["upload_time_iso_8601"].(string)
	}

	// Component 생성
	component := &Component{
		ComponentName: name,
		TimeStamp:     timestamp,
		SupplierName:  "Unknown",
		VersionString: version,
		Author:        author,
		Hash:          hash,
		UID:           fmt.Sprintf("pkg:pypi/%s@%s", name, version),
		Include:       []Component{},
	}

	// Parse dependencies
	re := regexp.MustCompile(`^([a-zA-Z0-9_-]+)([><=]+[\d.]+|==[\d.]+)?(;.+)?$`)
	for _, req := range requiresDist {
		reqStr := req.(string)

		// Extract dependency name, version, and condition
		matches := re.FindStringSubmatch(reqStr)
		if len(matches) < 2 {
			continue
		}

		depName := matches[1]
		depVersion := matches[2] // Handles `>=`, `<=`, `==`, etc.

		// 버전이 명시되지 않은 경우 최신 버전으로 설정
		if depVersion == "" {
			fmt.Printf("No version specified for dependency: %s. Fetching latest version.\n", depName)

			// 최신 버전 가져오기
			latestURL := fmt.Sprintf("%s/%s/json", p.BaseUrl, depName)
			latestResp, err := http.Get(latestURL)
			if err != nil {
				fmt.Printf("Failed to fetch latest version for dependency: %s: %v\n", depName, err)
				continue
			}
			defer latestResp.Body.Close()

			if latestResp.StatusCode != http.StatusOK {
				fmt.Printf("Failed to fetch latest version for dependency: %s: status code %d\n", depName, latestResp.StatusCode)
				continue
			}

			var latestData map[string]interface{}
			if err := json.NewDecoder(latestResp.Body).Decode(&latestData); err != nil {
				fmt.Printf("Failed to decode JSON for latest version of dependency: %s: %v\n", depName, err)
				continue
			}

			latestInfo := latestData["info"].(map[string]interface{})
			depVersion = latestInfo["version"].(string)
		}

		// Normalize version
		if strings.HasPrefix(depVersion, "==") {
			depVersion = strings.TrimPrefix(depVersion, "==") // Exact match
		} else if strings.HasPrefix(depVersion, ">=") || strings.HasPrefix(depVersion, ">") {
			// Extract minimum version
			depVersion = regexp.MustCompile(`[0-9.]+`).FindString(depVersion)
		} else if strings.Contains(depVersion, ",") {
			// Handle range like `>=2.0,<3.0`
			parts := strings.Split(depVersion, ",")
			for _, part := range parts {
				if strings.HasPrefix(part, ">=") || strings.HasPrefix(part, ">") {
					depVersion = regexp.MustCompile(`[0-9.]+`).FindString(part)
					break
				}
			}
		}

		// Recursively fetch dependency details
		depComponent, err := p.GetComponentInfo(depName, depVersion, visited)
		if err != nil {
			fmt.Printf("Failed to fetch dependency: %s@%s: %v\n", depName, depVersion, err)
			continue
		}

		component.Include = append(component.Include, *depComponent)
	}

	return component, nil
}
