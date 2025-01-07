package main

// AppendComponentToCsv 함수: Component 리스트를 CSV 데이터에 추가
func AppendComponentToCsv(component Component, csvData *[][]string, depth int) {
	// Relationship 설정
	relationship := "Primary"
	indent := ""

	if depth > 0 {
		relationship = "Included In"

		// Include 관계일 때 들여쓰기와 기호 추가
		for i := 0; i < depth-1; i++ {
			indent += "  " // 각 계층당 공백 두 칸 추가
		}
		indent += "┗"
	}

	// 현재 Component 데이터를 준비
	row := []string{
		indent + component.ComponentName,
		component.TimeStamp,
		component.SupplierName,
		component.VersionString,
		component.Author,
		component.Hash,
		component.UID,
		relationship,
	}

	// Null 또는 빈 문자열("")이 있는지 검사
	for _, field := range row {
		if field == "" {
			return // 해당 Component는 추가하지 않음
		}
	}

	// 모든 필드가 유효하면 CSV 데이터에 추가
	*csvData = append(*csvData, row)

	// Include Components 처리
	for _, includedComponent := range component.Include {
		AppendComponentToCsv(includedComponent, csvData, depth+1)
	}
}
