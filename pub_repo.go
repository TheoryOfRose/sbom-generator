package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type PubDevRepo struct {
	BaseUrl string
}

func NewPubDevRepo() *PubDevRepo {
	return &PubDevRepo{
		BaseUrl: "https://pub.dev/api/packages",
	}
}

func (p *PubDevRepo) GetComponentInfo(name, version string, visited map[string]bool) (*Component, error) {
	// 이미 방문한 패키지와 버전은 무시
	packageKey := fmt.Sprintf("%s@%s", name, version)
	if visited[packageKey] {
		fmt.Printf("Circular dependency detected for package: %s\n", packageKey)
		return nil, fmt.Errorf("circular dependency detected for package: %s", packageKey)
	}

	// Mark as visited
	visited[packageKey] = true

	// API URL 생성
	url := fmt.Sprintf("%s/%s", p.BaseUrl, name)

	// API 요청
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch package info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch package info: status code %d", resp.StatusCode)
	}

	// JSON 응답 파싱
	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// 특정 버전에 해당하는 데이터 가져오기
	var selectedVersion map[string]interface{}
	if version == "" {
		// 버전이 지정되지 않은 경우 최신 버전 사용
		selectedVersion = data["latest"].(map[string]interface{})
	} else {
		// 버전이 지정된 경우 versions에서 해당 버전 검색
		versions := data["versions"].([]interface{})
		for _, v := range versions {
			vData := v.(map[string]interface{})
			if vData["version"].(string) == version {
				selectedVersion = vData
				break
			}
		}
		if selectedVersion == nil {
			return nil, fmt.Errorf("version %s not found for package %s", version, name)
		}
	}

	// pubspec 데이터 추출
	pubspec := selectedVersion["pubspec"].(map[string]interface{})

	// 기본 패키지 정보 생성
	author, ok := pubspec["author"].(string)
	if !ok {
		author = "Unknown"
	}

	component := &Component{
		ComponentName: name,
		TimeStamp:     selectedVersion["published"].(string),
		SupplierName:  "Unknown",
		VersionString: version,
		Author:        author,
		Hash:          selectedVersion["archive_sha256"].(string),
		UID:           fmt.Sprintf("pkg:pub/%s@%s", name, version),
		Include:       []Component{},
	}

	// 의존성 처리
	if dependencies, ok := pubspec["dependencies"].(map[string]interface{}); ok {
		for depName, depVersion := range dependencies {
			var depVersionString string

			// 의존성 버전이 없는 경우 무시
			switch depVersion := depVersion.(type) {
			case string:
				// "^4.0.0" 형식을 "4.0.0"으로 변환
				if strings.HasPrefix(depVersion, "^") {
					depVersionString = depVersion[1:]
				} else if strings.HasPrefix(depVersion, ">=") {
					// ">=1.1.2 <3.0.0"에서 최소 버전 추출
					splitVersion := strings.Split(depVersion, " ")
					for _, part := range splitVersion {
						if strings.HasPrefix(part, ">=") {
							depVersionString = part[2:] // ">=" 제거
							break
						}
					}
				} else {
					depVersionString = depVersion
				}
			case map[string]interface{}:
				// SDK 관련 의존성 (예: "flutter": { "sdk": "flutter" }) 무시
				fmt.Printf("Skipping dependency %s: unsupported format\n", depName)
				continue
			default:
				fmt.Printf("Skipping dependency %s: no version specified\n", depName)
				continue
			}

			// 의존성 패키지 정보 가져오기 (재귀 호출)
			depComponent, err := p.GetComponentInfo(depName, depVersionString, visited)
			if err != nil {
				fmt.Printf("Failed to fetch dependency: %s@%s: %v\n", depName, depVersionString, err)
				continue
			}
			component.Include = append(component.Include, *depComponent)
		}
	}

	return component, nil
}
