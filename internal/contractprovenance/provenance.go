package contractprovenance

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

type OpenAPI struct {
	SHA256        string `json:"sha256"`
	Normalization string `json:"normalization"`
}

type Info struct {
	SchemaVersion       int     `json:"schemaVersion"`
	Repository          string  `json:"repository"`
	UpstreamCommit      string  `json:"upstreamCommit"`
	PublicThreadOpenAPI OpenAPI `json:"publicThreadOpenAPI"`
}

//go:embed provenance.json
var embedded []byte

func Current() Info {
	var info Info
	if err := json.Unmarshal(embedded, &info); err != nil {
		panic(fmt.Sprintf("decode embedded contract provenance: %v", err))
	}
	return info
}
