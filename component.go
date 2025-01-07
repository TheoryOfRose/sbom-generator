package main

type Component struct {
	ComponentName string      `json:"componentName"` // 컴포넌트 이름
	TimeStamp     string      `json:"timeStamp"`     // 타임스탬프
	SupplierName  string      `json:"supplierName"`  // 공급자 이름
	VersionString string      `json:"versionString"` // 버전 문자열
	Author        string      `json:"author"`        // 작성자
	Hash          string      `json:"hash"`          // 컴포넌트의 해시 값
	UID           string      `json:"uid"`           // 고유 ID
	Include       []Component `json:"include"`       // 포함된 하위 컴포넌트
}
