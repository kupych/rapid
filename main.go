package main

import (
	"encoding/json"
	"fmt"
	"github.com/chzyer/readline"
	"github.com/tidwall/gjson"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

func main() {
	variables := make(map[string]interface{})

	if len(os.Args) < 2 {
		fmt.Println("RAPID v0.0.7 - Rapid API Dialogue")
		fmt.Println("Usage: rapid <base-url>")
		fmt.Println()
		fmt.Println("Warning: this is a WIP. More functionality coming soon.")
		fmt.Println("Star: https://github.com/kupych/rapid")
		return
	}

	lastResponse := ""

	baseURL := os.Args[1]
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

		switch {
		case input == "exit" || input == "quit":
			return
		case input == "?":
			fmt.Print(showHelp())
		case input == "$":
			fmt.Println(lastResponse)
		case input == "?v":
			for name, value := range variables {
				fmt.Printf("%s = %v\n", name, value)
			}
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
				} else {
					//TODO extract vars directly from request
					fmt.Println("?")
				}
			} else {
				fmt.Println("?")
				continue
			}
		case strings.HasPrefix(input, "g("):
			path := strings.TrimSuffix(strings.TrimPrefix(input, "g("), ")")
			path = interpolateVars(path, variables)
			makeRequest("GET", buildURL(baseURL, path), "", &lastResponse)

		case strings.HasPrefix(input, "p("):
			pattern := `p\(([^{]+)\s*(\{[^}]+\})\)`
			re := regexp.MustCompile(pattern)
			matches := re.FindStringSubmatch(input)
			if len(matches) != 3 {
				fmt.Println("? ... p(/path {key:val})")
				continue
			}
			path := strings.TrimSpace(matches[1])
			path = interpolateVars(path, variables)
			requestBody := parseCJSON(matches[2])
			makeRequest("POST", buildURL(baseURL, path), requestBody, &lastResponse)

		case strings.HasPrefix(input, "pu("):
			pattern := `pu\(([^{]+)\s*(\{[^}]+\})\)`
			re := regexp.MustCompile(pattern)
			matches := re.FindStringSubmatch(input)
			if len(matches) != 3 {
				fmt.Println("? ... pu(/path {key:val})")
				continue
			}
			path := strings.TrimSpace(matches[1])
			path = interpolateVars(path, variables)
			requestBody := parseCJSON(matches[2])
			makeRequest("PUT", buildURL(baseURL, path), requestBody, &lastResponse)

		case strings.HasPrefix(input, "pa("):
			pattern := `pa\(([^{]+)\s*(\{[^}]+\})\)`
			re := regexp.MustCompile(pattern)
			matches := re.FindStringSubmatch(input)
			if len(matches) != 3 {
				fmt.Println("? ... pa(/path {key:val})")
				continue
			}
			path := strings.TrimSpace(matches[1])
			path = interpolateVars(path, variables)
			requestBody := parseCJSON(matches[2])
			makeRequest("PATCH", buildURL(baseURL, path), requestBody, &lastResponse)

		case strings.HasPrefix(input, "d("):
			path := strings.TrimSuffix(strings.TrimPrefix(input, "d("), ")")
			path = interpolateVars(path, variables)
			makeRequest("DELETE", buildURL(baseURL, path), "", &lastResponse)
		default:
			fmt.Println("?")
		}
	}
}

func buildURL(baseURL, path string) string {
	baseURL = strings.TrimSuffix(baseURL, "/")

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return baseURL + path
}

func showHelp() string {
	return `
Requests:
g(<path>) - GET request
p(<path>{key:val}) - POST request
pu(<path>{key:val}) - PUT request
pa(<path>{key:val}) - PATCH request
d(<path>) - DELETE request

Metacommands:
$ - Show last response
? - Show this help
?v - Show variables
{varName} = $ - Extract variable from last response
exit,quit,q,x - Exit rapid

Examples:

  g(users)
  g(users/1)
  $
	{id, email} = $
	g(users/${id})
  ?
	?v
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
	for responsePath, varName := range mappings {
		value := gjson.Get(response, responsePath)
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
