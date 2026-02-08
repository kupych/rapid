package spec

import "time"

// Spec represents a parsed OpenAPI specification
type Spec struct {
	Version    string              // OpenAPI version (e.g., "3.0.0", "3.1.0")
	Info       SpecInfo            // API info (title, description, version)
	Servers    []Server            // API servers
	Paths      map[string]PathItem // API endpoints keyed by path
	Components Components          // Reusable components

	Source   string    // File path or URL where spec was loaded from
	LoadedAt time.Time // When the spec was loaded
	Hash     string    // SHA256 hash of original spec content for cache validation
}

// SpecInfo contains API metadata
type SpecInfo struct {
	Title       string // API title
	Description string // API description
	Version     string // API version
	Contact     Contact
	License     License
}

// Contact contains contact information
type Contact struct {
	Name  string
	URL   string
	Email string
}

// License contains license information
type License struct {
	Name string
	URL  string
}

// Server represents an API server
type Server struct {
	URL         string
	Description string
	Variables   map[string]ServerVariable
}

// ServerVariable represents a server URL variable
type ServerVariable struct {
	Enum        []string
	Default     string
	Description string
}

// PathItem represents an API endpoint
type PathItem struct {
	Path       string      // The path (e.g., "/users")
	Summary    string      // Summary for the path
	Parameters []Parameter // Path/query parameters
	Get        *Operation  // GET operation
	Post       *Operation  // POST operation
	Put        *Operation  // PUT operation
	Patch      *Operation  // PATCH operation
	Delete     *Operation  // DELETE operation
	Head       *Operation  // HEAD operation
	Options    *Operation  // OPTIONS operation
	Trace      *Operation  // TRACE operation
}

// Operation represents an HTTP operation
type Operation struct {
	Summary     string              // Short summary of what the operation does
	Description string              // Longer description
	OperationID string              // Unique operation identifier
	Tags        []string            // Operation tags for categorization
	Parameters  []Parameter         // Operation parameters
	RequestBody *RequestBody        // Request body schema and media types
	Responses   map[string]Response // Responses keyed by status code
	Deprecated  bool
}

// Parameter represents a request parameter
type Parameter struct {
	Name        string // Parameter name
	In          string // Where the parameter is ("query", "header", "path", "cookie")
	Description string // Parameter description
	Required    bool   // Whether parameter is required
	Deprecated  bool
	Schema      *Schema // Parameter schema
	Example     interface{}
}

// RequestBody represents request body specification
type RequestBody struct {
	Description string
	Content     map[string]MediaType // Keyed by content-type (e.g., "application/json")
	Required    bool
}

// MediaType represents a media type with schema
type MediaType struct {
	Schema   *Schema
	Example  interface{}
	Examples map[string]Example
}

// Example represents an example value
type Example struct {
	Summary       string
	Description   string
	Value         interface{}
	ExternalValue string
}

// Response represents an HTTP response
type Response struct {
	Description string
	Headers     map[string]Header
	Content     map[string]MediaType // Keyed by content-type
	Links       map[string]Link
}

// Header represents a response header
type Header struct {
	Description string
	Required    bool
	Deprecated  bool
	Schema      *Schema
}

// Link represents a link to another operation
type Link struct {
	OperationRef string
	OperationID  string
	Parameters   map[string]interface{}
	RequestBody  interface{}
	Description  string
}

// Schema represents a JSON schema
type Schema struct {
	Type        string      // "object", "array", "string", "integer", "number", "boolean", "null"
	Format      string      // e.g., "date", "date-time", "email"
	Description string      // Schema description
	Default     interface{} // Default value
	Example     interface{} // Example value

	// For objects
	Properties map[string]*Schema // Object properties
	Required   []string           // Required property names

	// For arrays
	Items *Schema // Array item schema

	// For all types
	Enum []interface{} // Allowed values

	// Constraints
	MinLength int
	MaxLength int
	Pattern   string
	Minimum   interface{}
	Maximum   interface{}

	// Composition
	AllOf []*Schema // Must validate against all schemas
	OneOf []*Schema // Must validate against exactly one schema
	AnyOf []*Schema // Must validate against any schema
	Not   *Schema   // Must not validate against schema

	// References
	Ref string // Reference to another schema

	Deprecated bool
}

// Components contains reusable components
type Components struct {
	Schemas       map[string]*Schema
	Responses     map[string]Response
	Parameters    map[string]Parameter
	Examples      map[string]Example
	RequestBodies map[string]RequestBody
	Headers       map[string]Header
	Links         map[string]Link
}

// Session represents a user session with API configuration
type Session struct {
	BaseURL    string                 // Base URL for API requests
	SpecSource string                 // File path or URL where spec was loaded from
	SpecHash   string                 // Hash of loaded spec for validation
	Variables  map[string]interface{} // User-defined variables
	Headers    map[string]string      // Default headers
	CreatedAt  time.Time              // When session was created
	UpdatedAt  time.Time              // When session was last updated
}
