package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/atotto/clipboard"
	"github.com/chzyer/readline"
	"github.com/tidwall/gjson"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"rapid/providers"
	"rapid/spec"
	"regexp"
	"runtime"
	"strconv"
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

const (
	CReset  = "\033[0m"
	CRed    = "\033[31m"
	CGreen  = "\033[32m"
	CYellow = "\033[33m"
	CBlue   = "\033[34m"
	CGray   = "\033[90m"
)

func main() {
	debugFlag := flag.Bool("debug", false, "Enable debug mode")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		fmt.Println("RAPID v0.2.8 - Rapid API Dialogue")
		fmt.Println("Usage: rapid [--debug] <base-url>")
		fmt.Println()
		fmt.Println("Warning: this is a WIP. More functionality coming soon.")
		fmt.Println("Star: https://github.com/kupych/rapid")
		return
	}

	debug := *debugFlag
	variables, headers := loadVariables(".rapidvars")
	requestCount := 0
	responseHistory := []string{}
	startTime := time.Now()
	baseURL := args[0]
	baseURL = detectScheme(baseURL)
	lastCommand := ""
	var currentSpec *spec.Spec

	fmt.Printf("RAPID connected to %s\n", baseURL)
	fmt.Println()

	// Try to restore session
	cwd, _ := os.Getwd()
	if session, err := spec.LoadSession(cwd); err == nil && session != nil {
		// Restore spec if session has one
		if session.SpecSource != "" {
			if loadedSpec, err := spec.LoadSpec(session.SpecSource); err == nil {
				currentSpec = loadedSpec
				fmt.Printf("%s✓ Restored session with spec: %s v%s (%d endpoints)%s\n",
					CGreen, currentSpec.Info.Title, currentSpec.Info.Version, len(currentSpec.Paths), CReset)
			}
		}
		// Restore other session data
		if session.BaseURL != "" {
			baseURL = session.BaseURL
		}
		if session.Variables != nil {
			variables = session.Variables
		}
		if session.Headers != nil {
			headers = session.Headers
		}
	}

	var rl *readline.Instance

	// Listener for command expansion
	listener := readline.FuncListener(func(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool) {
		// Trigger expansion on space, tab, or open paren
		if key == ' ' || key == '\t' || key == '(' {
			// Get current text (line already includes the new key, so we need to exclude it)
			currentText := strings.TrimSpace(string(line))

			// If the trigger is '(', the buffer already has it, so strip it off
			if key == '(' && strings.HasSuffix(currentText, "(") {
				currentText = strings.TrimSuffix(currentText, "(")
				currentText = strings.TrimSpace(currentText)
			}

			// Get the last word (after the last space)
			words := strings.Fields(currentText)
			lastWord := ""
			if len(words) > 0 {
				lastWord = words[len(words)-1]
			}

			// Check for command abbreviations
			expansions := map[string]string{
				"delete": "delete(",
				"del":    "delete(",
				"d":      "delete(",
				"patch":  "patch(",
				"pat":    "patch(",
				"pa":     "patch(",
				"put":    "put(",
				"pu":     "put(",
				"post":   "post(",
				"pos":    "post(",
				"po":     "post(",
				"p":      "post(",
				"get":    "get(",
				"ge":     "get(",
				"g":      "get(",
			}

			for abbrev, expansion := range expansions {
				if lastWord == abbrev {
					// Replace just the last word with the expansion
					prefix := ""
					if len(words) > 1 {
						prefix = strings.Join(words[:len(words)-1], " ") + " "
					}
					rl.Operation.SetBuffer(prefix + expansion)
					rl.Operation.Refresh()
					// Consume the trigger key (space/tab/paren) since expansion already has '('
					return nil, 0, false
				}
			}
		}

		return line, pos, true
	})

	// Create autocomplete adapter
	completer := NewReadlineCompleter(&variables, currentSpec)

	var err error
	rl, err = readline.NewEx(&readline.Config{
		Prompt:       "> ",
		Listener:     listener,
		AutoComplete: completer,
	})

	if err != nil {
		panic(err)
	}

	defer rl.Close()

	for {
		input, err := rl.Readline()
		if err != nil {
			break
		}

		fmt.Print("\r\003")

		input = strings.TrimSpace(input)

		if input == "!!" {
			input = lastCommand
		} else {
			lastCommand = input
		}

		fmt.Printf("Input bytes: %v\n", []byte(input))

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
		case strings.HasPrefix(input, "$") && !strings.HasPrefix(input, "$$"):
			// Parse pipe operator first
			cleanInput, pipeOp, destination := parsePipeOperator(input)
			rest := strings.TrimPrefix(cleanInput, "$")

			var historyIndex int
			var gjsonPath string

			if strings.Contains(rest, ".") {
				parts := strings.SplitN(rest, ".", 2)
				if parts[0] == "" {
					historyIndex = 0
				} else {
					var err error
					historyIndex, err = strconv.Atoi(parts[0])
					if err != nil {
						fmt.Println(CRed + "?" + CReset)
						continue
					}
				}
				gjsonPath = parts[1]
			} else if rest == "" {
				historyIndex = 0
			} else {
				var err error
				historyIndex, err = strconv.Atoi(rest)
				if err != nil {
					fmt.Println(CRed + "?" + CReset)
					continue
				}
			}

			actualIndex := len(responseHistory) - 1 - historyIndex
			if actualIndex >= 0 && actualIndex < len(responseHistory) {
				responseBody := responseHistory[actualIndex]
				var outputContent string

				if gjsonPath != "" {
					value := gjson.Get(responseBody, gjsonPath)
					if value.Exists() {
						outputContent = value.String()
						fmt.Println(outputContent)
					} else {
						fmt.Println(CRed + "?" + CReset)
						continue
					}
				} else {
					outputContent = responseBody
					fmt.Println(outputContent)
				}

				// Handle pipe if requested
				if err := handlePipe(outputContent, pipeOp, destination); err != nil {
					fmt.Printf("%sX %v%s\n", CRed, err, CReset)
				}
			} else {
				fmt.Println(CRed + "?" + CReset)
			}
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
		case input == "?s":
			fmt.Printf("\nSession Info:\n")
			fmt.Printf("  Base URL: %s\n", baseURL)
			fmt.Printf("  Headers: %d\n", len(headers))
			fmt.Printf("  Requests: %d\n", requestCount)
			fmt.Printf("  Uptime: %s\n", time.Since(startTime).Round(time.Second))
			fmt.Printf("  Variables: %d\n", len(variables))
			if currentSpec != nil {
				fmt.Printf("  Spec: %s v%s (%d endpoints)\n", currentSpec.Info.Title, currentSpec.Info.Version, len(currentSpec.Paths))
			}
		case input == "?spec":
			// Show current spec info
			if currentSpec == nil {
				fmt.Println("No OpenAPI spec loaded")
				fmt.Println("Usage: ?spec <file_or_url>")
			} else {
				fmt.Printf("\nOpenAPI Spec:\n")
				fmt.Printf("  Title: %s\n", currentSpec.Info.Title)
				fmt.Printf("  Version: %s\n", currentSpec.Info.Version)
				if currentSpec.Info.Description != "" {
					fmt.Printf("  Description: %s\n", currentSpec.Info.Description)
				}
				fmt.Printf("  Source: %s\n", currentSpec.Source)
				fmt.Printf("  Endpoints: %d\n", len(currentSpec.Paths))
				fmt.Printf("  Loaded: %s\n", currentSpec.LoadedAt.Format("2006-01-02 15:04:05"))

				// Show detected prefix
				provider := &providers.OpenAPI{Spec: currentSpec}
				detectedPrefix := provider.DetectBasePrefix()
				if detectedPrefix != "" {
					fmt.Printf("  Base Prefix: %s (stripped from suggestions)\n", detectedPrefix)
				} else {
					fmt.Printf("  Base Prefix: (none detected)\n")
				}
			}
		case strings.HasPrefix(input, "?spec "):
			// Load spec from file or URL
			source := strings.TrimSpace(strings.TrimPrefix(input, "?spec "))
			if source == "" {
				fmt.Println("Usage: ?spec <file_or_url>")
				continue
			}

			fmt.Printf("Loading spec from %s...\n", source)
			loadedSpec, err := spec.LoadSpec(source)
			if err != nil {
				fmt.Printf("%sX Failed to load spec: %v%s\n", CRed, err, CReset)
				continue
			}

			currentSpec = loadedSpec
			completer.spec = currentSpec // Update completer's spec reference

			fmt.Printf("%s✓ Loaded spec: %s v%s (%d endpoints)%s\n",
				CGreen, currentSpec.Info.Title, currentSpec.Info.Version, len(currentSpec.Paths), CReset)

			// Save session
			session := &spec.Session{
				BaseURL:    baseURL,
				SpecSource: source,
				SpecHash:   currentSpec.Hash,
				Variables:  variables,
				Headers:    headers,
			}
			if err := spec.SaveSession(session, cwd); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to save session: %v\n", err)
			}
		case strings.HasPrefix(input, "??"):
			testInput := strings.TrimPrefix(input, "??")
			if testInput == "" {
				testInput = lastCommand
			}

			engine := NewAutocompleteEngine(variables, currentSpec)
			suggestions := engine.GetSuggestions(testInput, len(testInput))

			fmt.Printf("Suggestions for '%s':\n", testInput)
			for _, sug := range suggestions {
				fmt.Printf("  %s - %s\n", sug.Display, sug.Description)
			}
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
		case input == "?sc":
			// Clear session (spec, variables, headers)
			currentSpec = nil
			completer.spec = nil
			variables = make(map[string]interface{})
			headers = make(map[string]string)

			// Delete session file
			if err := spec.DeleteSession(cwd); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to delete session: %v\n", err)
			}

			fmt.Printf("%s✓ Session cleared%s\n", CGreen, CReset)
		case strings.HasSuffix(input, "="):
			parts := strings.SplitN(input, "=", 2)
			varToClear := strings.TrimSpace(parts[0])
			delete(variables, varToClear)
			fmt.Printf("x %s\n", varToClear)
		case strings.Contains(input, " = "):
			// Parse pipe operator from full input first
			cleanInput, pipeOp, destination := parsePipeOperator(input)

			parts := strings.SplitN(cleanInput, " = ", 2)
			if len(parts) == 2 {
				varPart := strings.TrimSpace(parts[0])
				source := strings.TrimSpace(parts[1])

				if strings.HasPrefix(source, "$") && !strings.HasPrefix(source, "$$") {
					rest := strings.TrimPrefix(source, "$")

					var historyIndex int
					var gjsonPath string

					if strings.Contains(rest, ".") {
						parts := strings.SplitN(rest, ".", 2)
						if parts[0] == "" {
							historyIndex = 0
						} else {
							var err error
							historyIndex, err = strconv.Atoi(parts[0])
							if err != nil {
								fmt.Println(CRed + "?" + CReset)
								continue
							}
						}
						gjsonPath = parts[1]
					} else if rest == "" {
						historyIndex = 0
					} else {
						var err error
						historyIndex, err = strconv.Atoi(rest)
						if err != nil {
							fmt.Println(CRed + "?" + CReset)
							continue
						}
					}

					// Get response from history
					actualIndex := len(responseHistory) - 1 - historyIndex
					if actualIndex >= 0 && actualIndex < len(responseHistory) {
						responseBody := responseHistory[actualIndex]

						if gjsonPath == "" {
							// No path, extract with mapping syntax like {id, name}
							extractVariables(varPart, responseBody, variables)
						} else {
							// Extract specific path
							value := gjson.Get(responseBody, gjsonPath)
							if debug {
								fmt.Printf("DEBUG: path='%s', exists=%v, raw=%v\n", gjsonPath, value.Exists(), value.Raw)
							}
							variables[varPart] = value.Value()
							fmt.Printf("%s = %v\n", varPart, value.Value())
						}
					} else {
						fmt.Println(CRed + "No response at that index" + CReset)
					}
					continue
				} else if isRequest(source) {
					lastParen := strings.LastIndex(source, ")")
					if lastParen == -1 {
						fmt.Println("?")
						continue
					}

					requestPart := source[:lastParen+1]
					pathPart := source[lastParen+1:]

					req, err := NewRequest(requestPart, baseURL, variables, headers, debug)
					if err != nil {
						fmt.Println("X", err)
						continue
					}
					response, err := req.Execute(variables, debug)
					if err != nil {
						fmt.Println("X", err)
						continue
					}

					if pathPart != "" {
						pathPart = strings.TrimPrefix(pathPart, ".")
						value := gjson.Get(response.Body, pathPart)
						variables[varPart] = value.Value()
						if debug {
							fmt.Printf("DEBUG: Extracted %s = %v from path %s\n", varPart, value.Value(), pathPart)
						}
					} else {
						extractVariables(varPart, response.Body, variables)
					}

					fmt.Println(response.Body)
					responseHistory = append(responseHistory, response.Body)

					// Handle pipe if requested
					if err := handlePipe(response.Body, pipeOp, destination); err != nil {
						fmt.Printf("%sX %v%s\n", CRed, err, CReset)
					}
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
			// Parse pipe operator
			cleanInput, pipeOp, destination := parsePipeOperator(input)

			req, err := NewRequest(cleanInput, baseURL, variables, headers, debug)
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
			responseHistory = append(responseHistory, response.Body)

			// Handle pipe if requested
			if err := handlePipe(response.Body, pipeOp, destination); err != nil {
				fmt.Printf("%sX %v%s\n", CRed, err, CReset)
			}
		default:
			fmt.Println("?")
		}
	}
}

// parsePipeOperator extracts pipe operator (>, >>) and destination from input
// Returns: cleanInput (without pipe), pipeOp (">", ">>", or ""), destination (file path or "")
func parsePipeOperator(input string) (cleanInput string, pipeOp string, destination string) {
	// Check for append operator first (>>)
	if idx := strings.Index(input, ">>"); idx != -1 {
		cleanInput = strings.TrimSpace(input[:idx])
		destination = strings.TrimSpace(input[idx+2:])
		pipeOp = ">>"
		return
	}

	// Check for regular pipe operator (>)
	if idx := strings.Index(input, ">"); idx != -1 {
		cleanInput = strings.TrimSpace(input[:idx])
		destination = strings.TrimSpace(input[idx+1:])
		pipeOp = ">"
		return
	}

	// No pipe operator found
	cleanInput = input
	return
}

// handlePipe handles piping content to clipboard or file
// If destination is empty, pipes to clipboard
// If destination is a path, pipes to file (overwrite with >, append with >>)
func handlePipe(content string, pipeOp string, destination string) error {
	if pipeOp == "" {
		return nil // No piping requested
	}

	// Empty destination means clipboard
	if destination == "" {
		if err := clipboard.WriteAll(content); err != nil {
			return fmt.Errorf("failed to copy to clipboard: %w", err)
		}
		fmt.Printf("%s✓ Copied to clipboard%s\n", CGreen, CReset)
		return nil
	}

	// Special destination "e" means open in editor
	if destination == "e" {
		return openInEditor(content)
	}

	// Expand ~ to home directory
	if strings.HasPrefix(destination, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		destination = filepath.Join(homeDir, destination[2:])
	}

	// Write to file
	if pipeOp == ">>" {
		// Append mode
		f, err := os.OpenFile(destination, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to open file for append: %w", err)
		}
		defer f.Close()

		if _, err := f.WriteString(content + "\n"); err != nil {
			return fmt.Errorf("failed to append to file: %w", err)
		}
		fmt.Printf("%s✓ Appended to %s%s\n", CGreen, destination, CReset)
	} else {
		// Overwrite mode
		if err := os.WriteFile(destination, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write to file: %w", err)
		}
		fmt.Printf("%s✓ Saved to %s%s\n", CGreen, destination, CReset)
	}

	return nil
}

// getEditor returns the editor to use, checking $EDITOR first, then platform-specific defaults
func getEditor() string {
	// Try $EDITOR environment variable first
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}

	// Platform-specific fallbacks
	switch runtime.GOOS {
	case "windows":
		return "notepad.exe"
	default: // darwin, linux, freebsd, etc.
		// Try common editors in order of preference
		for _, editor := range []string{"vim", "nano", "vi"} {
			if _, err := exec.LookPath(editor); err == nil {
				return editor
			}
		}
		return "vi" // Last resort fallback
	}
}

// openInEditor writes content to a temporary file and opens it in the user's editor
func openInEditor(content string) error {
	// Detect if content is JSON for appropriate file extension
	fileExt := ".txt"
	if json.Valid([]byte(content)) {
		fileExt = ".json"
	}

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "rapid-*"+fileExt)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Write content to temp file
	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write to temp file: %w", err)
	}
	tmpFile.Close()

	// Get editor
	editor := getEditor()

	// Open editor (blocking - wait for user to close)
	cmd := exec.Command(editor, tmpPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to open editor: %w", err)
	}

	fmt.Printf("%s✓ Opened in %s: %s%s\n", CGreen, editor, tmpPath, CReset)
	return nil
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

Piping:
<command> > - Pipe output to clipboard
<command> >e - Pipe output to editor ($EDITOR)
<command> > file.txt - Pipe output to file (overwrite)
<command> >> file.txt - Pipe output to file (append)

Metacommands:
$ - Show last response
? - Show this help
?v - Show variables
?vc - Clear all variables
?sc - Clear session (spec, variables, headers)
?spec - Show current OpenAPI spec info
?spec <file_or_url> - Load OpenAPI specification
??<term> - Preview autocomplete (coming soon!)
{varName} = $ - Extract variable from last response
varName = value - Set variable
varName = - Clear variable

exit,quit,q,x - Exit rapid

Examples:

  g(users)
  g(users/1)
  g(users) >
  g(users) >e
  g(users) > users.json
  $ > backup.json
  $.data.users >> all_users.json
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
		// Convert to string, removing quotes if it's a string type
		var strValue string
		switch v := value.(type) {
		case string:
			strValue = v
		default:
			strValue = fmt.Sprint(v)
		}
		result = strings.ReplaceAll(result, placeholder, strValue)
	}
	return result
}

func isRequest(input string) bool {
	return strings.HasPrefix(input, "delete(") ||
		strings.HasPrefix(input, "get(") ||
		strings.HasPrefix(input, "post(") ||
		strings.HasPrefix(input, "patch(") ||
		strings.HasPrefix(input, "put(") ||
		strings.HasPrefix(input, "d(") ||
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
	case strings.HasPrefix(cleanInput, "delete("):
		path := strings.TrimSuffix(strings.TrimPrefix(cleanInput, "delete("), ")")
		path = interpolateVars(path, variables)
		return &Request{Body: "", Headers: headers, Method: "DELETE", Url: buildURL(baseURL, path)}, nil
	case strings.HasPrefix(cleanInput, "d("):
		path := strings.TrimSuffix(strings.TrimPrefix(cleanInput, "d("), ")")
		path = interpolateVars(path, variables)
		return &Request{Body: "", Headers: headers, Method: "DELETE", Url: buildURL(baseURL, path)}, nil
	case strings.HasPrefix(cleanInput, "get("):
		path := strings.TrimSuffix(strings.TrimPrefix(cleanInput, "get("), ")")
		path = interpolateVars(path, variables)
		return &Request{Body: "", Headers: headers, Method: "GET", Url: buildURL(baseURL, path)}, nil
	case strings.HasPrefix(cleanInput, "g("):
		path := strings.TrimSuffix(strings.TrimPrefix(cleanInput, "g("), ")")
		path = interpolateVars(path, variables)
		return &Request{Body: "", Headers: headers, Method: "GET", Url: buildURL(baseURL, path)}, nil
	case strings.HasPrefix(cleanInput, "post("):
		pattern := `post\(([^\s]+)(?:\s+(.+))?\)`
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(cleanInput)
		if len(matches) < 2 {
			return nil, fmt.Errorf(CRed + "? ... post(/path {key:val})" + CReset)
		}
		path := strings.TrimSpace(matches[1])
		path = interpolateVars(path, variables)
		bodyPart := matches[2]
		body, contentType := parseBody(bodyPart, variables)
		return &Request{Body: body, ContentType: contentType, Headers: headers, Method: "POST", Url: buildURL(baseURL, path)}, nil
	case strings.HasPrefix(cleanInput, "put("):
		pattern := `put\(([^\s]+)(?:\s+(.+))?\)`
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(cleanInput)
		if len(matches) < 2 {
			return nil, fmt.Errorf(CRed + "? ... put(/path {key:val})" + CReset)
		}
		path := strings.TrimSpace(matches[1])
		path = interpolateVars(path, variables)
		body, contentType := parseBody(matches[2], variables)
		return &Request{Body: body, ContentType: contentType, Headers: headers, Method: "PUT", Url: buildURL(baseURL, path)}, nil
	case strings.HasPrefix(cleanInput, "patch("):
		pattern := `patch\(([^\s]+)(?:\s+(.+))?\)`
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(cleanInput)
		if len(matches) < 2 {
			return nil, fmt.Errorf(CRed + "? ... patch(/path {key:val})" + CReset)
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
		fmt.Printf("DEBUG: Headers: %v\n", r.Headers)
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

	fmt.Printf("%s✓ %d %s (%dms)%s\n", statusColor(resp.StatusCode), resp.StatusCode, http.StatusText(resp.StatusCode), elapsed.Milliseconds(), CReset)

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

func statusColor(code int) string {
	if code >= 400 {
		return CRed
	} else if code >= 300 {
		return CYellow
	} else {
		return CGreen
	}
}
