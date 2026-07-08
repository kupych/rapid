package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

// Session holds all per-session state. Both the interactive REPL and the
// .rapidvars startup script drive it through Execute.
type Session struct {
	baseURL         string
	variables       map[string]interface{}
	headers         map[string]string
	endpoints       *EndpointStore
	responseHistory []string
	requestCount    int
	startTime       time.Time
	debug           bool
	lastCommand     string
	out             io.Writer // io.Discard while the startup script runs
	scriptFile      string    // path of the startup script, for ?r re-runs
	inScript        bool      // guards against ?r recursion inside the script
}

func NewSession(baseURL string, variables map[string]interface{}, headers map[string]string, endpoints *EndpointStore, debug bool) *Session {
	return &Session{
		baseURL:         baseURL,
		variables:       variables,
		headers:         headers,
		endpoints:       endpoints,
		responseHistory: []string{},
		startTime:       time.Now(),
		debug:           debug,
		out:             os.Stdout,
	}
}

// printf/println write informational output, suppressed in silent mode.
func (s *Session) printf(format string, a ...interface{}) {
	fmt.Fprintf(s.out, format, a...)
}

func (s *Session) println(a ...interface{}) {
	fmt.Fprintln(s.out, a...)
}

// errorf/errorln write errors, which always show even in silent mode.
func (s *Session) errorf(format string, a ...interface{}) {
	fmt.Printf(format, a...)
}

func (s *Session) errorln(a ...interface{}) {
	fmt.Println(a...)
}

// Execute runs one REPL command. It returns false when the session should
// end (exit/quit), true otherwise.
func (s *Session) Execute(input string) bool {
	switch {
	case input == "exit" || input == "quit" || input == "q" || input == "x":
		return false
	case input == "?d":
		s.debug = !s.debug
		if s.debug {
			s.println("Debug ON")
		} else {
			s.println("Debug OFF")
		}
	case input == "?":
		s.printf("%s", showHelp())
	case input == "?r":
		runStartupScript(s, s.scriptFile)
	case strings.HasPrefix(input, "$") && !strings.HasPrefix(input, "$$"):
		// Parse pipe operator first
		cleanInput, pipeOp, destination := parsePipeOperator(input)
		rest := strings.TrimPrefix(cleanInput, "$")

		historyIndex, gjsonPath, ok := parseHistoryRef(rest)
		if !ok {
			s.errorln(CRed + "?" + CReset)
			return true
		}

		actualIndex := len(s.responseHistory) - 1 - historyIndex
		if actualIndex >= 0 && actualIndex < len(s.responseHistory) {
			responseBody := s.responseHistory[actualIndex]
			var outputContent string

			if gjsonPath != "" {
				value := gjson.Get(responseBody, gjsonPath)
				if value.Exists() {
					outputContent = value.String()
					s.println(outputContent)
				} else {
					s.errorln(CRed + "?" + CReset)
					return true
				}
			} else {
				outputContent = responseBody
				s.println(outputContent)
			}

			// Handle pipe if requested
			if err := handlePipe(outputContent, pipeOp, destination); err != nil {
				s.errorf("%sX %v%s\n", CRed, err, CReset)
			}
		} else {
			s.errorln(CRed + "?" + CReset)
		}
	case input == "?v":
		if len(s.variables) == 0 {
			s.println("{ }")
			return true
		}
		names := make([]string, 0, len(s.variables))
		for name := range s.variables {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			s.printf("%s = %s\n", name, previewValue(s.variables[name]))
		}
	case input == "?h":
		if len(s.headers) == 0 {
			s.println("< >")
			return true
		}
		for name, value := range s.headers {
			s.printf("<%s: %v>\n", name, value)
		}
	case input == "?hc":
		s.headers = make(map[string]string)
		s.println("< >")
	case input == "?e":
		known := s.endpoints.ForBase(s.baseURL)
		if len(known) == 0 {
			s.println("( )")
			return true
		}
		paths := make([]string, 0, len(known))
		for path := range known {
			paths = append(paths, path)
		}
		sort.Strings(paths)
		for _, path := range paths {
			s.printf("%-40s %s%s%s\n", path, CGray, strings.Join(known[path], ", "), CReset)
		}
	case input == "?ec":
		s.endpoints.Clear(s.baseURL)
		s.endpoints.Save()
		s.println("( )")
	case input == "?s":
		s.printf("\nSession Info:\n")
		s.printf("  Base URL: %s\n", s.baseURL)
		s.printf("  Headers: %d\n", len(s.headers))
		s.printf("  Requests: %d\n", s.requestCount)
		s.printf("  Uptime: %s\n", time.Since(s.startTime).Round(time.Second))
		s.printf("  Variables: %d\n", len(s.variables))
	case strings.HasPrefix(input, "??"):
		testInput := strings.TrimPrefix(input, "??")
		if testInput == "" {
			testInput = s.lastCommand
		}

		engine := NewAutocompleteEngine(s.variables, s.headers, &s.responseHistory, s.endpoints.ForBase(s.baseURL))
		suggestions := engine.GetSuggestions(testInput, len(testInput))

		s.printf("Suggestions for '%s':\n", testInput)
		if len(suggestions) == 0 {
			s.println("  (none)")
		}
		for _, sug := range suggestions {
			s.printf("  %-20s %s%s%s\n", sug.Display, CGray, sug.Description, CReset)
		}
	case strings.HasPrefix(input, "?h "):
		parts := strings.SplitN(strings.TrimPrefix(input, "?h "), ":", 2)
		if len(parts) == 2 {
			name := strings.ToLower(strings.TrimSpace(parts[0]))
			value := interpolateVars(strings.TrimSpace(parts[1]), s.variables)
			s.headers[name] = value
			s.printf("<%s: %v>\n", name, value)
		} else {
			name := strings.TrimSpace(parts[0])
			delete(s.headers, name)
			s.printf("x <%s>\n", name)
		}
	case input == "?vc" || input == "?clear":
		s.variables = make(map[string]interface{})
		s.println("{ }")
	case strings.HasSuffix(input, "="):
		parts := strings.SplitN(input, "=", 2)
		varToClear := strings.TrimSpace(parts[0])
		delete(s.variables, varToClear)
		s.printf("x %s\n", varToClear)
	case strings.Contains(input, " = "):
		// Parse pipe operator from full input first
		cleanInput, pipeOp, destination := parsePipeOperator(input)

		parts := strings.SplitN(cleanInput, " = ", 2)
		if len(parts) == 2 {
			varPart := strings.TrimSpace(parts[0])
			source := strings.TrimSpace(parts[1])

			if strings.HasPrefix(source, "$") && !strings.HasPrefix(source, "$$") {
				rest := strings.TrimPrefix(source, "$")

				historyIndex, gjsonPath, ok := parseHistoryRef(rest)
				if !ok {
					s.errorln(CRed + "?" + CReset)
					return true
				}

				// Get response from history
				actualIndex := len(s.responseHistory) - 1 - historyIndex
				if actualIndex >= 0 && actualIndex < len(s.responseHistory) {
					responseBody := s.responseHistory[actualIndex]

					if gjsonPath == "" {
						// No path, extract with mapping syntax like {id, name}
						s.extractVariables(varPart, responseBody)
					} else {
						// Extract specific path
						value := gjson.Get(responseBody, gjsonPath)
						if s.debug {
							s.printf("DEBUG: path='%s', exists=%v, raw=%v\n", gjsonPath, value.Exists(), value.Raw)
						}
						s.variables[varPart] = value.Value()
						s.printf("%s = %s\n", varPart, previewValue(value.Value()))
					}
				} else {
					s.errorln(CRed + "No response at that index" + CReset)
				}
				return true
			} else if isRequest(source) {
				lastParen := strings.LastIndex(source, ")")
				if lastParen == -1 {
					s.errorln("?")
					return true
				}

				requestPart := source[:lastParen+1]
				pathPart := source[lastParen+1:]

				req, err := NewRequest(requestPart, s.baseURL, s.variables, s.headers, s.debug, s.out)
				if err != nil {
					s.errorln("X", err)
					return true
				}
				response, err := req.Execute(s.variables, s.debug, s.out)
				if err != nil {
					s.errorln("X", err)
					return true
				}

				if response.Status < 400 && s.endpoints.Add(s.baseURL, req.Method, extractEndpointPath(requestPart, s.variables)) {
					s.endpoints.Save()
				}

				if pathPart != "" {
					pathPart = strings.TrimPrefix(pathPart, ".")
					value := gjson.Get(response.Body, pathPart)
					s.variables[varPart] = value.Value()
					if s.debug {
						s.printf("DEBUG: Extracted %s = %v from path %s\n", varPart, value.Value(), pathPart)
					}
				} else {
					s.extractVariables(varPart, response.Body)
				}

				s.println(response.Body)
				s.responseHistory = append(s.responseHistory, response.Body)

				// Handle pipe if requested
				if err := handlePipe(response.Body, pipeOp, destination); err != nil {
					s.errorf("%sX %v%s\n", CRed, err, CReset)
				}
				return true
			} else {
				s.variables[varPart] = source
				s.printf("%s = %s\n", varPart, source)
			}
		} else {
			s.errorln("?")
			return true
		}
	case isRequest(input):
		// Parse pipe operator
		cleanInput, pipeOp, destination := parsePipeOperator(input)

		req, err := NewRequest(cleanInput, s.baseURL, s.variables, s.headers, s.debug, s.out)
		if err != nil {
			s.errorln("X", err)
			return true
		}
		response, err := req.Execute(s.variables, s.debug, s.out)
		if err != nil {
			s.errorln("X", err)
			return true
		}

		if response.Status < 400 && s.endpoints.Add(s.baseURL, req.Method, extractEndpointPath(cleanInput, s.variables)) {
			s.endpoints.Save()
		}

		s.println(response.Body)
		s.responseHistory = append(s.responseHistory, response.Body)

		// Handle pipe if requested
		if err := handlePipe(response.Body, pipeOp, destination); err != nil {
			s.errorf("%sX %v%s\n", CRed, err, CReset)
		}
	default:
		// Bare variable name, optionally with a path and pipe: `user.email >`
		cleanInput, pipeOp, destination := parsePipeOperator(input)
		output, ok := lookupVariable(cleanInput, s.variables)
		if !ok {
			s.errorln("?")
			return true
		}
		s.println(output)
		if err := handlePipe(output, pipeOp, destination); err != nil {
			s.errorf("%sX %v%s\n", CRed, err, CReset)
		}
	}
	return true
}

// parseHistoryRef splits a history reference like "1.data.id" into an index
// and a gjson path. A missing index means the latest response.
func parseHistoryRef(rest string) (historyIndex int, gjsonPath string, ok bool) {
	if strings.Contains(rest, ".") {
		parts := strings.SplitN(rest, ".", 2)
		gjsonPath = parts[1]
		if parts[0] == "" {
			return 0, gjsonPath, true
		}
		historyIndex, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, "", false
		}
		return historyIndex, gjsonPath, true
	}
	if rest == "" {
		return 0, "", true
	}
	historyIndex, err := strconv.Atoi(rest)
	if err != nil {
		return 0, "", false
	}
	return historyIndex, "", true
}

func (s *Session) extractVariables(varPart string, response string) {
	mappings := parseVarMappings(varPart)
	for responseUrl, varName := range mappings {
		value := gjson.Get(response, responseUrl)
		if value.Exists() {
			s.variables[varName] = value.Value()
			s.printf("%s = %s\n", varName, previewValue(value.Value()))
		}
	}
}
