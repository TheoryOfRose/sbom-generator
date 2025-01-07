package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

type MavenRepo struct {
	BaseUrl string `json:"baseUrl"`
	TempDir string `json:"tempDir"`
}

func NewMavenRepo() (*MavenRepo, error) {
	// Get the system's temporary directory
	tempDir := os.TempDir()

	// Create a subdirectory for MavenRepo within the temp directory
	repoTempDir := filepath.Join(tempDir, "maven-repo")
	if err := os.MkdirAll(repoTempDir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	return &MavenRepo{
		BaseUrl: "https://repo1.maven.org/maven2",
		TempDir: repoTempDir,
	}, nil
}

func (m *MavenRepo) GetTimeStamp(group, name, version string) string {
	// Construct the main URL
	groupPath := strings.ReplaceAll(group, ".", "/")
	md5Path := filepath.Join(groupPath, name, version)
	baseURL, err := url.Parse(m.BaseUrl)
	if err != nil {
		fmt.Println(fmt.Errorf("invalid base URL: %w", err))
		return ""
	}
	baseURL.Path = path.Join(baseURL.Path, md5Path)
	fullURL := baseURL.String()

	// Fetch the URL
	resp, err := http.Get(fullURL)
	if err != nil {
		fmt.Errorf("failed to fetch URL: %w", err)
		return ""
	}
	defer resp.Body.Close()

	// Read the content of the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Errorf("failed to read response body: %w", err)
		return ""
	}

	// Capture the timestamp from the html body
	// 예: `<td align="right">2025-01-07 11:32</td>`
	re := regexp.MustCompile(`(\d{4}-\d{2}-\d{2} \d{2}:\d{2})`)
	matches := re.FindStringSubmatch(string(body))
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

func (m *MavenRepo) GetMd5Hash(group, name, version string) string {
	// Construct the MD5 file URL
	groupPath := strings.ReplaceAll(group, ".", "/")
	md5Path := filepath.Join(groupPath, name, version, fmt.Sprintf("%s-%s.jar.md5", name, version))
	baseURL, err := url.Parse(m.BaseUrl)
	if err != nil {
		fmt.Println(fmt.Errorf("invalid base URL: %w", err))
		return ""
	}
	baseURL.Path = path.Join(baseURL.Path, md5Path)
	fullURL := baseURL.String()

	// Download the MD5 file
	resp, err := http.Get(fullURL)
	if err != nil {
		fmt.Println(fmt.Errorf("failed to fetch MD5 file from URL %s: %w", fullURL, err))
		return ""
	}
	defer resp.Body.Close()

	// Check if the response status is 404
	if resp.StatusCode == http.StatusNotFound {
		fmt.Printf("MD5 file not found at URL %s, returning empty string.\n", fullURL)
		return ""
	}

	// Read the content of the MD5 file
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(fmt.Errorf("failed to read MD5 file content: %w", err))
		return ""
	}

	// Check if the body contains an HTML-like response (e.g., 404 error page)
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/html") || strings.Contains(string(body), "<html>") {
		fmt.Println("MD5 file content is HTML-like, returning empty string.")
		return ""
	}

	// Convert the content to a string and trim any whitespace
	md5Hash := strings.TrimSpace(string(body))

	// Return the MD5 hash
	return md5Hash
}

func (m *MavenRepo) GetComponentInfo(group, name, version string, visited map[string]bool) (*Component, error) {
	// Unique key for visited map
	componentKey := fmt.Sprintf("%s:%s:%s", group, name, version)
	if visited[componentKey] {
		// Circular dependency detected
		fmt.Printf("Circular dependency detected: %s\n", componentKey)
		return nil, fmt.Errorf("circular dependency detected for component: %s", componentKey)
	}

	// Mark this component as visited
	visited[componentKey] = true

	// Construct the POM file URL
	groupPath := strings.ReplaceAll(group, ".", "/")
	pomPath := filepath.Join(groupPath, name, version, fmt.Sprintf("%s-%s.pom", name, version))
	baseURL, err := url.Parse(m.BaseUrl)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	baseURL.Path = path.Join(baseURL.Path, pomPath)
	fullURL := baseURL.String()

	// Download the POM file
	resp, err := http.Get(fullURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		// If the POM file is not found, return a default Component
		fmt.Printf("Failed to fetch POM file from URL %s, creating a default component.\n", fullURL)
		return &Component{
			ComponentName: name,
			TimeStamp:     m.GetTimeStamp(group, name, version),
			SupplierName:  group,
			VersionString: version,
			Author:        "Unknown",
			Hash:          m.GetMd5Hash(group, name, version),
			UID:           fmt.Sprintf("pkg:maven/%s/%s@$%s", group, name, version),
		}, nil
	}
	defer resp.Body.Close()

	// Save the POM file locally
	pomFilePath := filepath.Join(m.TempDir, fmt.Sprintf("%s-%s.pom", name, version))
	pomFile, err := os.Create(pomFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to save POM file: %w", err)
	}
	defer pomFile.Close()

	// Copy the response body to the file
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read POM file content: %w", err)
	}
	if _, err := pomFile.Write(body); err != nil {
		return nil, fmt.Errorf("failed to write POM file: %w", err)
	}

	// Parse the POM file
	parsedXML, err := ParseXML(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse XML, returning default component: %w", err)
	}

	// Determine supplierName
	var supplierName string
	if orgName, ok := parsedXML["/project/organization/name"].(string); ok && orgName != "" {
		supplierName = orgName // Use organization/name if available
	} else if org, ok := parsedXML["/project/organization"].(string); ok && org != "" {
		supplierName = org // Use organization if organization/name is not available
	} else {
		supplierName = group // Default to group
	}

	// Get developer names
	var author string
	if devNames, ok := parsedXML["/project/developers/developer/name"].([]string); ok {
		author = strings.Join(devNames, ", ")
	} else if singleDevName, ok := parsedXML["/project/developers/developer/name"].(string); ok {
		author = singleDevName
	} else {
		author = "Unknown"
	}

	// Parse dependencies
	includes := []Component{}

	// Fetch arrays for groupId, artifactId, and version
	groupIDs, _ := parsedXML["/project/dependencies/dependency/groupId"].([]string)
	artifactIDs, _ := parsedXML["/project/dependencies/dependency/artifactId"].([]string)
	versions, _ := parsedXML["/project/dependencies/dependency/version"].([]string)

	if len(groupIDs) == len(artifactIDs) && len(artifactIDs) == len(versions) {
		for i := range groupIDs {
			groupID := groupIDs[i]
			artifactID := artifactIDs[i]
			version := versions[i]

			// Recursive call to fetch dependency details
			depComponent, err := m.GetComponentInfo(groupID, artifactID, version, visited)
			if err != nil {
				fmt.Printf("Failed to fetch dependency: (%s, %s, %s): %v\n", groupID, artifactID, version, err)
				continue
			}

			includes = append(includes, *depComponent)
		}
	} else {
		fmt.Println("Mismatched dependency arrays or no dependencies found.")
	}

	// Return the parsed component information
	return &Component{
		ComponentName: name,
		TimeStamp:     m.GetTimeStamp(group, name, version),
		SupplierName:  supplierName,
		VersionString: version,
		Author:        author,
		Hash:          m.GetMd5Hash(group, name, version),
		UID:           fmt.Sprintf("pkg:maven/%s/%s@$%s", group, name, version),
		Include:       includes,
	}, nil
}

// func (m *MavenRepo) GetRootComponentInfo(group, name, version string) (*Component, error) {
// 	// Construct the POM file URL
// 	groupPath := strings.ReplaceAll(group, ".", "/")
// 	pomPath := filepath.Join(groupPath, name, version, fmt.Sprintf("%s-%s.pom", name, version))
// 	// Resolve the full URL
// 	baseURL, err := url.Parse(m.BaseUrl)
// 	if err != nil {
// 		return nil, fmt.Errorf("invalid base URL: %w", err)
// 	}
// 	baseURL.Path = path.Join(baseURL.Path, pomPath)
// 	fullURL := baseURL.String()

// 	// Download the POM file
// 	resp, err := http.Get(fullURL)
// 	if err != nil || resp.StatusCode != http.StatusOK {
// 		// If the POM file is not found, return a default Component
// 		fmt.Printf("Failed to fetch POM file from URL %s, creating a default component.\n", fullURL)
// 		return &Component{
// 			ComponentName: name,
// 			SupplierName:  group,
// 			VersionString: version,
// 			Author:        "Unknown",
// 			Hash:          m.GetMd5Hash(group, name, version),
// 			UID:           fmt.Sprintf("pkg:maven/%s/%s@$%s", group, name, version),
// 		}, nil
// 	}
// 	defer resp.Body.Close()

// 	// Save the POM file locally
// 	pomFilePath := filepath.Join(m.TempDir, fmt.Sprintf("%s-%s.pom", name, version))
// 	pomFile, err := os.Create(pomFilePath)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to save POM file: %w", err)
// 	}
// 	defer pomFile.Close()

// 	// Copy the response body to the file
// 	body, err := io.ReadAll(resp.Body)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to read POM file content: %w", err)
// 	}
// 	if _, err := pomFile.Write(body); err != nil {
// 		return nil, fmt.Errorf("failed to write POM file: %w", err)
// 	}

// 	// Parse the POM file
// 	parsedXML, err := ParseXML(body)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to parse XML, returning default component: %w", err)
// 	}

// 	// Determine supplierName
// 	var supplierName string
// 	if orgName, ok := parsedXML["/project/organization/name"].(string); ok && orgName != "" {
// 		supplierName = orgName // Use organization/name if available
// 	} else if org, ok := parsedXML["/project/organization"].(string); ok && org != "" {
// 		supplierName = org // Use organization if organization/name is not available
// 	} else {
// 		supplierName = group // Default to group
// 	}

// 	// Get developer names
// 	var author string
// 	if devNames, ok := parsedXML["/project/developers/developer/name"].([]string); ok {
// 		author = strings.Join(devNames, ", ")
// 	} else if singleDevName, ok := parsedXML["/project/developers/developer/name"].(string); ok {
// 		author = singleDevName
// 	} else {
// 		author = "Unknown"
// 	}

// 	// Parse dependencies
// 	dependencies := []string{} // Holds (group, artifact, version) as strings
// 	if depList, ok := parsedXML["/project/dependencies/dependency"].([]map[string]interface{}); ok {
// 		for _, dep := range depList {
// 			groupID := dep["groupId"].(string)
// 			artifactID := dep["artifactId"].(string)
// 			version := dep["version"].(string)
// 			dependencies = append(dependencies, fmt.Sprintf("(%s, %s, %s)", groupID, artifactID, version))
// 		}
// 	}

// 	// Return the parsed component information
// 	return &Component{
// 		ComponentName: name,
// 		SupplierName:  supplierName,
// 		VersionString: version,
// 		Author:        author,
// 		Hash:          m.GetMd5Hash(group, name, version),
// 		UID:           fmt.Sprintf("pkg:maven/%s/%s@$%s", group, name, version),
// 	}, nil
// }
