package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/chzyer/readline"
	"github.com/tidwall/gjson"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

type Request struct {
	Method      string
	ContentType string
	Headers     map[string]string
	Url         string
	Body        string
}

type Response struct {
	Body   string
	Status int
}

func main() {
	debugFlag := flag.Bool("debug", false, "Enable debug mode")
	flag.Parse()

	args := flag.Args()
	variables, headers := loadVariables(".rapidvars")

	fmt.Println(debugFlag)
	if len(args) < 1 {
		fmt.Println("RAPID v0.2.0 - Rapid API Dialogue")
		fmt.Println("Usage: rapid [--debug] <base-url>")
		fmt.Println()
		fmt.Println("Warning: this is a WIP. More functionality coming soon.")
		fmt.Println("Star: https://github.com/kupych/rapid")
		return
	}

	debug := *debugFlag

	lastResponse := ""

	baseURL := args[0]
	baseURL = detectScheme(baseURL)
	fmt.Printf("RAPID connected to %s\n", baseURL)
	fmt.Println()

	rl, err := readline.New("> ")

	if err != nil {
		panic(err)
	}

	defer rl.Close()

	for {
		input, err := rl.Readline()
		if err != nil {
			break
		}

		input = strings.TrimSpace(input)

		switch {
		case input == "exit" || input == "quit" || input == "q" || input == "x":
			return
		case input == "?d":
			debug = !debug
			if debug {
				fmt.Println("Debug ON")
			} else {
				fmt.Println("Debug OFF")
			}
		case input == "?":
			fmt.Print(showHelp())
		case input == "$":
			fmt.Println(lastResponse)
		case input == "?v":
			if len(variables) == 0 {
				fmt.Println("{ }")
				continue
			}
			for name, value := range variables {
				fmt.Printf("%s = %v\n", name, value)
			}
		case input == "?h":
			if len(headers) == 0 {
				fmt.Println("< >")
				continue
			}
			for name, value := range headers {
				fmt.Printf("<%s: %v>\n", name, value)
			}

		case input == "?hc":
			headers = make(map[string]string)
			fmt.Println("< >")
		case strings.HasPrefix(input, "?h "):
			parts := strings.SplitN(strings.TrimPrefix(input, "?h "), ":", 2)
			if len(parts) == 2 {
				name := strings.ToLower(strings.TrimSpace(parts[0]))
				value := interpolateVars(strings.TrimSpace(parts[1]), variables)
				headers[name] = value
				fmt.Printf("<%s: %v>\n", name, value)
			} else {
				name := strings.TrimSpace(parts[0])
				delete(headers, name)
				fmt.Printf("x <%s>\n", name)
			}
		case input == "?vc" || input == "?clear":
			variables = make(map[string]interface{})
			fmt.Println("{ }")
		case strings.HasSuffix(input, "="):
			parts := strings.SplitN(input, "=", 2)
			varToClear := strings.TrimSpace(parts[0])
			delete(variables, varToClear)
			fmt.Printf("x %s\n", varToClear)
		case strings.Contains(input, " = "):
			parts := strings.SplitN(input, " = ", 2)
			if len(parts) == 2 {
				varPart := strings.TrimSpace(parts[0])
				source := strings.TrimSpace(parts[1])

				if source == "$" {
					extractVariables(varPart, lastResponse, variables)
					continue
				} else if strings.HasPrefix(source, "$.") {
					path := strings.TrimPrefix(source, "$.")
					value := gjson.Get(lastResponse, path)
					variables[varPart] = value.Value()
					continue
				} else if isRequest(source) {
					req, err := NewRequest(source, baseURL, variables, headers, debug)
					if err != nil {
						fmt.Println("X", err)
						continue
					}
					response, err := req.Execute(variables, debug)
					if err != nil {
						fmt.Println("X", err)
						continue
					}
					fmt.Println(response.Body)
					lastResponse = response.Body
					extractVariables(varPart, response.Body, variables)
					continue
				} else {
					variables[varPart] = source
					fmt.Printf("%s = %s\n", varPart, source)
				}
			} else {
				fmt.Println("?")
				continue
			}
		case isRequest(input):
			req, err := NewRequest(input, baseURL, variables, headers, debug)
			if err != nil {
				fmt.Println("X", err)
				continue
			}
			response, err := req.Execute(variables, debug)
			if err != nil {
				fmt.Println("X", err)
				continue
			}
			fmt.Println(response.Body)
			lastResponse = response.Body
		default:
			fmt.Println("?")
		}
	}
}

func buildURL(baseURL, path string) string {
	baseURL = strings.TrimSuffix(baseURL, "/")

	if strings.HasPrefix(path, "//") {
		return "https:" + path
	}

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return baseURL + path
}

func showHelp() string {
	return `
Requests:
g(<path>) - GET request
p(<path> {key:val}) - POST request
pu(<path> {key:val}) - PUT request
pa(<path> {key:val}) - PATCH request
d(<path>) - DELETE request

Metacommands:
$ - Show last response
? - Show this help
?v - Show variables
?vc - Clear all variables
{varName} = $ - Extract variable from last response
varName = value - Set variable
varName = - Clear variable

exit,quit,q,x - Exit rapid

Examples:

  g(users)
  g(users/1)
  $
	{id, email} = $
	g(users/${id})
  ?
	?v
	name = John
	g(users/${name})
`
}

func parseCJSON(condensed string) string {
	inner := strings.Trim(condensed, "{}")

	pairs := strings.Split(inner, ",")

	body := make(map[string]string)
	for _, pair := range pairs {
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			body[key] = value
		}
	}

	jsonBytes, _ := json.Marshal(body)
	return string(jsonBytes)
}

func makeRequest(method, url, reqBody string, lastResponse *string) {
	var body io.Reader
	if reqBody != "" {
		body = strings.NewReader(reqBody)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		fmt.Println("X", err)
		return
	}

	if reqBody != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	elapsed := time.Since(start)

	if err != nil {
		fmt.Println("X", err)
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Could not parse response body: ", err)
		return

	}
	fmt.Printf("✓ %d %s (%dms)\n", resp.StatusCode, http.StatusText(resp.StatusCode), elapsed.Milliseconds())

	var data interface{}
	if err := json.Unmarshal(respBody, &data); err != nil {
		fmt.Println(string(respBody))
	} else {
		pretty, _ := json.MarshalIndent(data, "", " ")
		fmt.Println(string(pretty))
		*lastResponse = string(pretty)
	}

}

func detectScheme(url string) string {
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		return url
	}
	resp, err := http.Head("https://" + url)
	if err == nil && resp.StatusCode < 400 {
		resp.Body.Close()
		return "https://" + url
	}
	return "http://" + url
}

func parseVarNames(vars string) (varList []string) {
	vars = strings.Trim(vars, "{}")
	parts := strings.Split(vars, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func extractVariables(varPart string, response string, variables map[string]interface{}) {
	mappings := parseVarMappings(varPart)
	for responseUrl, varName := range mappings {
		value := gjson.Get(response, responseUrl)
		if value.Exists() {
			variables[varName] = value.Value()
			fmt.Printf("%s = %v\n", varName, value.Value())
		}
	}
}

func parseVarMappings(varPart string) map[string]string {
	result := make(map[string]string)
	vars := strings.Trim(varPart, "{}")
	parts := strings.Split(vars, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, ":") {
			kv := strings.SplitN(part, ":", 2)
			result[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		} else {
			result[part] = part
		}
	}
	return result
}

func interpolateVars(path string, variables map[string]interface{}) string {
	result := path
	for varName, value := range variables {
		placeholder := "${" + varName + "}"
		result = strings.ReplaceAll(result, placeholder, fmt.Sprint(value))
	}
	return result
}

func isRequest(input string) bool {
	return strings.HasPrefix(input, "d(") ||
		strings.HasPrefix(input, "g(") ||
		strings.HasPrefix(input, "p(") ||
		strings.HasPrefix(input, "pa(") ||
		strings.HasPrefix(input, "pu(")
}

func NewRequest(input string, baseURL string, variables map[string]interface{}, sessionHeaders map[string]string, debug bool) (*Request, error) {
	inlineHeaders, cleanInput := parseInlineHeaders(input)

	if debug {
		fmt.Printf("DEBUG NewRequest: original='%s'\n", input)
		fmt.Printf("DEBUG NewRequest: cleanInput='%s'\n", cleanInput)
		fmt.Printf("DEBUG NewRequest: inlineHeaders=%v\n", inlineHeaders)
	}

	headers := make(map[string]string)

	for k, v := range sessionHeaders {
		headers[strings.ToLower(k)] = v
	}

	for k, v := range inlineHeaders {
		headers[k] = interpolateVars(v, variables)
	}

	switch {
	case strings.HasPrefix(cleanInput, "d("):
		path := strings.TrimSuffix(strings.TrimPrefix(cleanInput, "d("), ")")
		path = interpolateVars(path, variables)
		return &Request{Body: "", Headers: headers, Method: "DELETE", Url: buildURL(baseURL, path)}, nil
	case strings.HasPrefix(cleanInput, "g("):
		path := strings.TrimSuffix(strings.TrimPrefix(cleanInput, "g("), ")")
		path = interpolateVars(path, variables)
		return &Request{Body: "", Headers: headers, Method: "GET", Url: buildURL(baseURL, path)}, nil
	case strings.HasPrefix(cleanInput, "p("):
		pattern := `p\(([^\s]+)(?:\s+(.+))?\)`
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(cleanInput)
		if len(matches) < 2 {
			return nil, fmt.Errorf("? ... p(/path {key:val})")
		}
		path := strings.TrimSpace(matches[1])
		path = interpolateVars(path, variables)
		bodyPart := matches[2]
		body, contentType := parseBody(bodyPart, variables)
		return &Request{Body: body, ContentType: contentType, Headers: headers, Method: "POST", Url: buildURL(baseURL, path)}, nil

	case strings.HasPrefix(cleanInput, "pu("):
		pattern := `pu\(([^\s]+)(?:\s+(.+))?\)`
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(cleanInput)
		if len(matches) < 2 {
			return nil, fmt.Errorf("? ... pu(/path {key:val})")
		}
		path := strings.TrimSpace(matches[1])
		path = interpolateVars(path, variables)
		body, contentType := parseBody(matches[2], variables)
		return &Request{Body: body, ContentType: contentType, Headers: headers, Method: "PUT", Url: buildURL(baseURL, path)}, nil

	case strings.HasPrefix(cleanInput, "pa("):
		pattern := `pa\(([^\s]+)(?:\s+(.+))?\)`
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(cleanInput)
		if len(matches) < 2 {
			return nil, fmt.Errorf("? ... pa(/path {key:val})")
		}
		path := strings.TrimSpace(matches[1])
		path = interpolateVars(path, variables)
		body, contentType := parseBody(matches[2], variables)
		return &Request{Body: body, ContentType: contentType, Headers: headers, Method: "PATCH", Url: buildURL(baseURL, path)}, nil
	default:
		return nil, fmt.Errorf("?")
	}
}

func (r *Request) Execute(variables map[string]interface{}, debug bool) (Response, error) {
	var body io.Reader
	if r.Body != "" {
		body = strings.NewReader(r.Body)
	}

	req, err := http.NewRequest(r.Method, r.Url, body)
	if err != nil {
		return Response{}, err
	}

	for k, v := range r.Headers {
		req.Header.Set(k, v)
	}

	if authToken, exists := variables["$$auth"]; exists {
		req.Header.Set("Authorization", "Bearer "+fmt.Sprint(authToken))
	}
	if r.Body != "" && r.ContentType != "" {
		req.Header.Set("Content-Type", r.ContentType)
	}

	if debug {
		fmt.Printf("DEBUG: %s %s\n", r.Method, r.Url)
		fmt.Printf("DEBUG: Headers: %v\n", r.Header)
		fmt.Printf("DEBUG: Body: %v\n", r.Body)
	}

	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	elapsed := time.Since(start)

	if err != nil {
		return Response{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, fmt.Errorf("could not read response body: %w", err)
	}

	fmt.Printf("✓ %d %s (%dms)\n", resp.StatusCode, http.StatusText(resp.StatusCode), elapsed.Milliseconds())

	var data interface{}
	if err := json.Unmarshal(respBody, &data); err != nil {
		// Non-JSON response - return as-is
		return Response{Body: string(respBody), Status: resp.StatusCode}, nil
	}

	pretty, _ := json.MarshalIndent(data, "", " ")
	return Response{Body: string(pretty), Status: resp.StatusCode}, nil
}

func parseBody(bodyPart string, variables map[string]interface{}) (body string, contentType string) {
	bodyPart = strings.TrimSpace(bodyPart)
	bodyPart = interpolateVars(bodyPart, variables)

	if bodyPart == "" {
		return "", ""
	}

	if strings.HasPrefix(bodyPart, "?") {
		formData := strings.TrimPrefix(bodyPart, "?")

		values := url.Values{}
		pairs := strings.Split(formData, "&")

		for _, pair := range pairs {
			parts := strings.SplitN(pair, "=", 2)
			if len(parts) == 2 {
				values.Add(parts[0], parts[1])
			}
		}

		return values.Encode(), "application/x-www-form-urlencoded"
	}

	if strings.HasPrefix(bodyPart, "\"") && strings.HasSuffix(bodyPart, "\"") {
		return strings.Trim(bodyPart, "\""), "text/plain"
	}

	if strings.HasPrefix(bodyPart, "{") {
		return parseCJSON(bodyPart), "application/json"
	}

	return "", ""
}

func loadVariables(filename string) (map[string]interface{}, map[string]string) {
	vars := make(map[string]interface{})
	headers := make(map[string]string)

	data, err := os.ReadFile(filename)
	if err != nil {
		return vars, headers
	}

	raw := make(map[string]interface{})
	json.Unmarshal(data, &raw)

	for k, v := range raw {
		if strings.HasPrefix(k, "$$header:") {
			headerName := strings.TrimPrefix(k, "$$header:")
			headers[strings.ToLower(headerName)] = fmt.Sprint(v)
		} else {
			vars[k] = v
		}

	}
	return vars, headers
}

func parseInlineHeaders(input string) (map[string]string, string) {
	headers := make(map[string]string)
	pattern := `\s?<([^:]+):([^>]+)>`

	re := regexp.MustCompile(pattern)

	matches := re.FindAllStringSubmatch(input, -1)

	for _, match := range matches {
		headers[strings.ToLower(strings.TrimSpace(match[1]))] = strings.TrimSpace(match[2])
	}

	return headers, strings.TrimSpace(re.ReplaceAllString(input, ""))
}
