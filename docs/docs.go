// Package docs embeds the OpenAPI 3.1 specification for the EN 16931 Toolkit API.
package docs

import _ "embed"

//go:embed openapi.yaml
var OpenAPISpec []byte
