package spec

import (
	"crypto/sha256"
	"fmt"
	"github.com/pb33f/libopenapi"
	"time"
)

// ParseSpec parses raw OpenAPI specification bytes (YAML or JSON) and returns a Spec struct.
func ParseSpec(rawData []byte) (*Spec, error) {
	// Create document from raw data
	doc, err := libopenapi.NewDocument(rawData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI spec: %w", err)
	}

	// Build the V3 model
	docModel, err := doc.BuildV3Model()
	if err != nil {
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
