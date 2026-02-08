package spec

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	"strings"
	"time"
)

// ParseSpec parses raw OpenAPI specification bytes (YAML or JSON) and returns a Spec struct.
func ParseSpec(rawData []byte) (*Spec, error) {
	// Create document from raw data
	doc, err := libopenapi.NewDocument(rawData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI spec: %w", err)
	}

	// Configure to allow circular references and skip validation
	config := &datamodel.DocumentConfiguration{
		AllowFileReferences:   false,
		AllowRemoteReferences: false,
		BypassDocumentCheck:   true,
	}
	doc.SetConfiguration(config)

	// Try to build the V3 model
	docModel, err := doc.BuildV3Model()
	if err != nil {
		// If building fails (e.g., circular references), fall back to raw parsing
		if strings.Contains(err.Error(), "circular reference") {
			return parseRawSpec(rawData, doc.GetVersion())
		}
		return nil, fmt.Errorf("failed to build OpenAPI model: %v", err)
	}

	v3Model := &docModel.Model

	spec := &Spec{
		Version:  doc.GetVersion(),
		LoadedAt: time.Now(),
		Paths:    make(map[string]PathItem),
		Info: SpecInfo{
			Title:       v3Model.Info.Title,
			Description: v3Model.Info.Description,
			Version:     v3Model.Info.Version,
		},
	}

	// Parse paths
	if v3Model.Paths != nil && v3Model.Paths.PathItems != nil {
		for pair := v3Model.Paths.PathItems.First(); pair != nil; pair = pair.Next() {
			path := pair.Key()
			pathItem := pair.Value()

			pi := PathItem{
				Path:    path,
				Summary: pathItem.Summary,
			}

			// Parse operations (simplified)
			if pathItem.Get != nil {
				pi.Get = &Operation{
					Summary:     pathItem.Get.Summary,
					Description: pathItem.Get.Description,
					OperationID: pathItem.Get.OperationId,
				}
			}
			if pathItem.Post != nil {
				pi.Post = &Operation{
					Summary:     pathItem.Post.Summary,
					Description: pathItem.Post.Description,
					OperationID: pathItem.Post.OperationId,
				}
			}
			if pathItem.Put != nil {
				pi.Put = &Operation{
					Summary:     pathItem.Put.Summary,
					Description: pathItem.Put.Description,
					OperationID: pathItem.Put.OperationId,
				}
			}
			if pathItem.Patch != nil {
				pi.Patch = &Operation{
					Summary:     pathItem.Patch.Summary,
					Description: pathItem.Patch.Description,
					OperationID: pathItem.Patch.OperationId,
				}
			}
			if pathItem.Delete != nil {
				pi.Delete = &Operation{
					Summary:     pathItem.Delete.Summary,
					Description: pathItem.Delete.Description,
					OperationID: pathItem.Delete.OperationId,
				}
			}

			spec.Paths[path] = pi
		}
	}

	// Compute hash of the raw data
	hash := sha256.Sum256(rawData)
	spec.Hash = fmt.Sprintf("%x", hash)

	return spec, nil
}

// ComputeHash computes the SHA256 hash of data and returns it as a hex string
func ComputeHash(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}

// parseRawSpec is a fallback parser for specs with circular references
// It parses the raw JSON/YAML directly without building the full model
func parseRawSpec(rawData []byte, version string) (*Spec, error) {
	var rawSpec map[string]interface{}
	if err := json.Unmarshal(rawData, &rawSpec); err != nil {
		return nil, fmt.Errorf("failed to parse spec as JSON: %w", err)
	}

	spec := &Spec{
		Version:  version,
		LoadedAt: time.Now(),
		Paths:    make(map[string]PathItem),
	}

	// Extract info
	if info, ok := rawSpec["info"].(map[string]interface{}); ok {
		if title, ok := info["title"].(string); ok {
			spec.Info.Title = title
		}
		if description, ok := info["description"].(string); ok {
			spec.Info.Description = description
		}
		if ver, ok := info["version"].(string); ok {
			spec.Info.Version = ver
		}
	}

	// Extract paths
	if paths, ok := rawSpec["paths"].(map[string]interface{}); ok {
		for path, pathData := range paths {
			pathItem, ok := pathData.(map[string]interface{})
			if !ok {
				continue
			}

			pi := PathItem{
				Path: path,
			}

			// Parse each HTTP method
			if get, ok := pathItem["get"].(map[string]interface{}); ok {
				pi.Get = &Operation{
					Summary:     getStringField(get, "summary"),
					Description: getStringField(get, "description"),
					OperationID: getStringField(get, "operationId"),
				}
			}
			if post, ok := pathItem["post"].(map[string]interface{}); ok {
				pi.Post = &Operation{
					Summary:     getStringField(post, "summary"),
					Description: getStringField(post, "description"),
					OperationID: getStringField(post, "operationId"),
				}
			}
			if put, ok := pathItem["put"].(map[string]interface{}); ok {
				pi.Put = &Operation{
					Summary:     getStringField(put, "summary"),
					Description: getStringField(put, "description"),
					OperationID: getStringField(put, "operationId"),
				}
			}
			if patch, ok := pathItem["patch"].(map[string]interface{}); ok {
				pi.Patch = &Operation{
					Summary:     getStringField(patch, "summary"),
					Description: getStringField(patch, "description"),
					OperationID: getStringField(patch, "operationId"),
				}
			}
			if del, ok := pathItem["delete"].(map[string]interface{}); ok {
				pi.Delete = &Operation{
					Summary:     getStringField(del, "summary"),
					Description: getStringField(del, "description"),
					OperationID: getStringField(del, "operationId"),
				}
			}

			spec.Paths[path] = pi
		}
	}

	// Compute hash
	hash := sha256.Sum256(rawData)
	spec.Hash = fmt.Sprintf("%x", hash)

	return spec, nil
}

// getStringField safely extracts a string field from a map
func getStringField(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}
