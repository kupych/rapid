package spec

import (
	"crypto/sha256"
	"fmt"
	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"time"
)

// ParseSpec parses raw OpenAPI specification bytes (YAML or JSON) and returns a Spec struct.
// The raw bytes can be YAML or JSON format. The function validates and converts to our simplified structure.
func ParseSpec(rawData []byte) (*Spec, error) {
	// Create document from raw data
	doc, err := libopenapi.NewDocument(rawData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI spec: %w", err)
	}

	// Build the high-level model
	docModel, errs := doc.BuildModel()
	if len(errs) > 0 {
		// Even with errors, we might have a partial model - let's continue if we have one
		if docModel == nil {
			return nil, fmt.Errorf("failed to build OpenAPI model: %v", errs[0])
		}
	}

	// Only OpenAPI 3.x is supported
	v3Model, ok := docModel.(*v3.Document)
	if !ok {
		return nil, fmt.Errorf("unsupported OpenAPI version; only 3.x is supported")
	}

	spec := &Spec{
		Version:  v3Model.Version,
		LoadedAt: time.Now(),
		Paths:    make(map[string]PathItem),
		Info: SpecInfo{
			Title:       v3Model.Info.Title,
			Description: v3Model.Info.Description,
			Version:     v3Model.Info.Version,
		},
	}

	// Parse contact info
	if v3Model.Info.Contact != nil {
		spec.Info.Contact = Contact{
			Name:  v3Model.Info.Contact.Name,
			URL:   v3Model.Info.Contact.URL,
			Email: v3Model.Info.Contact.Email,
		}
	}

	// Parse license info
	if v3Model.Info.License != nil {
		spec.Info.License = License{
			Name: v3Model.Info.License.Name,
			URL:  v3Model.Info.License.URL,
		}
	}

	// Parse servers
	if v3Model.Servers != nil {
		for _, srv := range v3Model.Servers {
			server := Server{
				URL:         srv.URL,
				Description: srv.Description,
				Variables:   make(map[string]ServerVariable),
			}
			if srv.Variables != nil {
				for name, srvVar := range srv.Variables {
					server.Variables[name] = ServerVariable{
						Default:     srvVar.Default,
						Description: srvVar.Description,
						Enum:        srvVar.Enum,
					}
				}
			}
			spec.Servers = append(spec.Servers, server)
		}
	}

	// Parse paths
	if v3Model.Paths != nil {
		for path, pathItem := range v3Model.Paths.PathItems {
			pi := PathItem{
				Path:       path,
				Summary:    pathItem.Summary,
				Parameters: parseParameters(pathItem.Parameters),
			}

			// Parse operations
			if pathItem.Get != nil {
				pi.Get = parseOperation(pathItem.Get)
			}
			if pathItem.Post != nil {
				pi.Post = parseOperation(pathItem.Post)
			}
			if pathItem.Put != nil {
				pi.Put = parseOperation(pathItem.Put)
			}
			if pathItem.Patch != nil {
				pi.Patch = parseOperation(pathItem.Patch)
			}
			if pathItem.Delete != nil {
				pi.Delete = parseOperation(pathItem.Delete)
			}
			if pathItem.Head != nil {
				pi.Head = parseOperation(pathItem.Head)
			}
			if pathItem.Options != nil {
				pi.Options = parseOperation(pathItem.Options)
			}
			if pathItem.Trace != nil {
				pi.Trace = parseOperation(pathItem.Trace)
			}

			spec.Paths[path] = pi
		}
	}

	// Parse components (schemas and other reusable components)
	if v3Model.Components != nil {
		spec.Components = Components{
			Schemas:       make(map[string]*Schema),
			Responses:     make(map[string]Response),
			Parameters:    make(map[string]Parameter),
			Examples:      make(map[string]Example),
			RequestBodies: make(map[string]RequestBody),
			Headers:       make(map[string]Header),
			Links:         make(map[string]Link),
		}

		if v3Model.Components.Schemas != nil {
			for name, schema := range v3Model.Components.Schemas {
				spec.Components.Schemas[name] = parseSchema(schema)
			}
		}

		if v3Model.Components.Responses != nil {
			for name, resp := range v3Model.Components.Responses {
				spec.Components.Responses[name] = parseResponse(resp)
			}
		}

		if v3Model.Components.Parameters != nil {
			for name, param := range v3Model.Components.Parameters {
				spec.Components.Parameters[name] = parseParameter(param)
			}
		}

		if v3Model.Components.RequestBodies != nil {
			for name, rb := range v3Model.Components.RequestBodies {
				spec.Components.RequestBodies[name] = parseRequestBody(rb)
			}
		}

		if v3Model.Components.Headers != nil {
			for name, header := range v3Model.Components.Headers {
				spec.Components.Headers[name] = parseHeader(header)
			}
		}
	}

	// Compute hash of the raw data
	hash := sha256.Sum256(rawData)
	spec.Hash = fmt.Sprintf("%x", hash)

	return spec, nil
}

// parseOperation converts a high-level Operation to our Operation struct
func parseOperation(op *v3.Operation) *Operation {
	if op == nil {
		return nil
	}

	operation := &Operation{
		Summary:     op.Summary,
		Description: op.Description,
		OperationID: op.OperationID,
		Tags:        op.Tags,
		Deprecated:  op.Deprecated,
		Parameters:  parseParameters(op.Parameters),
		Responses:   make(map[string]Response),
	}

	if op.RequestBody != nil {
		operation.RequestBody = parseRequestBody(op.RequestBody)
	}

	if op.Responses != nil {
		for code, resp := range op.Responses.Codes {
			operation.Responses[code] = parseResponse(resp)
		}
	}

	return operation
}

// parseParameters converts a list of high-level Parameters to our Parameter structs
func parseParameters(params []*v3.Parameter) []Parameter {
	var result []Parameter
	if params == nil {
		return result
	}

	for _, p := range params {
		param := Parameter{
			Name:        p.Name,
			In:          p.In,
			Description: p.Description,
			Required:    p.Required,
			Deprecated:  p.Deprecated,
		}

		if p.Schema != nil {
			param.Schema = parseSchema(p.Schema)
		}

		result = append(result, param)
	}

	return result
}

// parseRequestBody converts a high-level RequestBody to our RequestBody struct
func parseRequestBody(rb *v3.RequestBody) *RequestBody {
	if rb == nil {
		return nil
	}

	reqBody := &RequestBody{
		Description: rb.Description,
		Required:    rb.Required,
		Content:     make(map[string]MediaType),
	}

	if rb.Content != nil {
		for contentType, mediaType := range rb.Content {
			mt := MediaType{
				Examples: make(map[string]Example),
			}

			if mediaType.Schema != nil {
				mt.Schema = parseSchema(mediaType.Schema)
			}

			if mediaType.Example != nil {
				mt.Example = mediaType.Example.Value
			}

			if mediaType.Examples != nil {
				for exName, ex := range mediaType.Examples {
					mt.Examples[exName] = Example{
						Summary:       ex.Summary,
						Description:   ex.Description,
						Value:         ex.Value,
						ExternalValue: ex.ExternalValue,
					}
				}
			}

			reqBody.Content[contentType] = mt
		}
	}

	return reqBody
}

// parseResponse converts a high-level Response to our Response struct
func parseResponse(resp *v3.Response) Response {
	response := Response{
		Description: resp.Description,
		Headers:     make(map[string]Header),
		Content:     make(map[string]MediaType),
	}

	if resp.Headers != nil {
		for name, header := range resp.Headers {
			response.Headers[name] = parseHeader(header)
		}
	}

	if resp.Content != nil {
		for contentType, mediaType := range resp.Content {
			mt := MediaType{
				Examples: make(map[string]Example),
			}

			if mediaType.Schema != nil {
				mt.Schema = parseSchema(mediaType.Schema)
			}

			if mediaType.Example != nil {
				mt.Example = mediaType.Example.Value
			}

			if mediaType.Examples != nil {
				for exName, ex := range mediaType.Examples {
					mt.Examples[exName] = Example{
						Summary:       ex.Summary,
						Description:   ex.Description,
						Value:         ex.Value,
						ExternalValue: ex.ExternalValue,
					}
				}
			}

			response.Content[contentType] = mt
		}
	}

	return response
}

// parseHeader converts a high-level Header to our Header struct
func parseHeader(h *v3.Header) Header {
	header := Header{
		Description: h.Description,
		Required:    h.Required,
		Deprecated:  h.Deprecated,
	}

	if h.Schema != nil {
		header.Schema = parseSchema(h.Schema)
	}

	return header
}

// parseParameter converts a high-level Parameter to our Parameter struct
func parseParameter(p *v3.Parameter) Parameter {
	param := Parameter{
		Name:        p.Name,
		In:          p.In,
		Description: p.Description,
		Required:    p.Required,
		Deprecated:  p.Deprecated,
	}

	if p.Schema != nil {
		param.Schema = parseSchema(p.Schema)
	}

	return param
}

// parseSchema converts a high-level Schema to our Schema struct
func parseSchema(s *base.Schema) *Schema {
	if s == nil {
		return nil
	}

	schema := &Schema{
		Type:        s.Type,
		Format:      s.Format,
		Description: s.Description,
		Deprecated:  s.Deprecated,
		Properties:  make(map[string]*Schema),
		Required:    s.Required,
		MinLength:   int(s.MinLength),
		MaxLength:   int(s.MaxLength),
		Pattern:     s.Pattern,
	}

	if s.Default != nil {
		schema.Default = s.Default.Value
	}

	if s.Example != nil {
		schema.Example = s.Example.Value
	}

	if s.Enum != nil {
		for _, e := range s.Enum {
			if e != nil {
				schema.Enum = append(schema.Enum, e.Value)
			}
		}
	}

	if s.Minimum != nil {
		schema.Minimum = s.Minimum
	}

	if s.Maximum != nil {
		schema.Maximum = s.Maximum
	}

	if s.Ref != "" {
		schema.Ref = s.Ref
	}

	// Handle object properties
	if s.Properties != nil {
		for propName, prop := range s.Properties {
			schema.Properties[propName] = parseSchema(prop)
		}
	}

	// Handle array items
	if s.Items != nil {
		schema.Items = parseSchema(s.Items)
	}

	// Handle composition schemas
	if s.AllOf != nil {
		for _, sch := range s.AllOf {
			schema.AllOf = append(schema.AllOf, parseSchema(sch))
		}
	}

	if s.OneOf != nil {
		for _, sch := range s.OneOf {
			schema.OneOf = append(schema.OneOf, parseSchema(sch))
		}
	}

	if s.AnyOf != nil {
		for _, sch := range s.AnyOf {
			schema.AnyOf = append(schema.AnyOf, parseSchema(sch))
		}
	}

	if s.Not != nil {
		schema.Not = parseSchema(s.Not)
	}

	return schema
}

// ComputeHash computes the SHA256 hash of data and returns it as a hex string
func ComputeHash(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}
