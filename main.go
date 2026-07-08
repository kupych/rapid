package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/atotto/clipboard"
	"github.com/chzyer/readline"
	"github.com/tidwall/gjson"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
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
		fmt.Println("RAPID v0.5.0 - Rapid API Dialogue")
		fmt.Println("Usage: rapid [--debug] <base-url>")
		fmt.Println()
		fmt.Println("Warning: this is a WIP. More functionality coming soon.")
		fmt.Println("Star: https://github.com/kupych/rapid")
		return
	}

	debug := *debugFlag
	endpoints := LoadEndpointStore()
	baseURL := args[0]
	baseURL = detectScheme(baseURL)

	session := NewSession(baseURL, make(map[string]interface{}), make(map[string]string), endpoints, debug)

	fmt.Printf("RAPID connected to %s\n", baseURL)
	runStartupScript(session, ".rapidvars")
	fmt.Println()

	var rl *readline.Instance

	// Create autocomplete adapter and inline suggestion painter
	completer := NewReadlineCompleter(&session.variables, &session.headers, &session.responseHistory, endpoints, baseURL)
	painter := NewGhostPainter(completer, "> ")

	// Listener for command expansion and ghost text acceptance
	listener := readline.FuncListener(func(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool) {
		// Right arrow / Ctrl+F at end of line accepts the inline suggestion
		if key == readline.CharForward && pos == len(line) {
			if ghost := painter.Ghost(); len(ghost) > 0 {
				accepted := append(append([]rune{}, line...), ghost...)
				return accepted, len(accepted), true
			}
		}

		// Typing ')' inside an open CJSON body closes every nested object and
		// array plus the function call in one stroke. The buffer already holds
		// the typed ')', so balance the text before it and replace.
		if key == ')' && pos == len(line) {
			before := string(line[:pos-1])
			if closers := closingSuffix(before); strings.ContainsAny(closers, "}]") {
				rl.Operation.SetBuffer(before + closers)
				rl.Operation.Refresh()
				return nil, 0, false
			}
		}

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

	var err error
	rl, err = readline.NewEx(&readline.Config{
		Prompt:       "> ",
		Listener:     listener,
		AutoComplete: completer,
		Painter:      painter,
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
			input = session.lastCommand
		} else {
			session.lastCommand = input
		}

		if !session.Execute(input) {
			return
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

// lookupVariable resolves a bare variable reference like `user` or
// `user.address.city`, pretty-printing structured values
func lookupVariable(input string, variables map[string]interface{}) (string, bool) {
	value, ok := resolveVariable(input, variables)
	if !ok {
		return "", false
	}
	return formatVarValue(value), true
}

// resolveVariable resolves a variable reference, optionally with a gjson path
// after the name (e.g. "user.address.city"), to its raw value.
func resolveVariable(input string, variables map[string]interface{}) (interface{}, bool) {
	name, path := input, ""
	if i := strings.Index(input, "."); i != -1 {
		name, path = input[:i], input[i+1:]
	}

	value, exists := variables[name]
	if !exists {
		return nil, false
	}

	if path == "" {
		return value, true
	}

	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return nil, false
	}
	result := gjson.GetBytes(jsonBytes, path)
	if !result.Exists() {
		return nil, false
	}
	return result.Value(), true
}

// formatVarValue pretty-prints structured values, scalars come out bare
func formatVarValue(value interface{}) string {
	switch value.(type) {
	case map[string]interface{}, []interface{}:
		if pretty, err := json.MarshalIndent(value, "", " "); err == nil {
			return string(pretty)
		}
	}
	return fmt.Sprint(value)
}

// previewValue renders a value as a single truncated line for ?v listings
func previewValue(value interface{}) string {
	var s string
	switch value.(type) {
	case map[string]interface{}, []interface{}:
		if compact, err := json.Marshal(value); err == nil {
			s = string(compact)
		} else {
			s = fmt.Sprint(value)
		}
	default:
		s = fmt.Sprint(value)
	}

	if r := []rune(s); len(r) > 60 {
		s = string(r[:57]) + "..."
	}
	return s
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
?e - Show learned endpoints (autocompleted in requests)
?ec - Forget learned endpoints for this base URL
?r - Re-run the .rapidvars startup script (e.g. to refresh auth)
??<term> - Preview autocomplete suggestions
Tab completes, right arrow accepts the inline suggestion
{varName} = $ - Extract variable from last response
varName = value - Set variable
varName = - Clear variable
varName - Show variable value (pipes work: varName >e)
varName.path - Drill into object variables
Vars work as paths and bodies: addUser = users/new, then post(addUser body)
A leading slash is always literal: g(/users) never resolves variables

exit,quit,q,x - Exit rapid

Startup script (.rapidvars):
Runs silently when a session starts; any REPL command works.
Lines before the first @host section run for every session,
@host sections only when the host matches the base URL.
A file starting with { is still read as legacy JSON variables.

  # .rapidvars
  env = staging

  @api.example.com
  token = post(auth/login {user:stefan, pass:secret}).access_token
  ?h authorization: Bearer ${token}

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

// parseCJSON expands condensed JSON into real JSON. Objects nest:
// {a{b{c:d}}} and {a:{b:{c:d}}} both expand to {"a":{"b":{"c":"d"}}}.
// The colon after a key is optional when its value is a nested object or
// array. Arrays use brackets: {tags[a,b]} -> {"tags":["a","b"]}. Scalar
// values are inferred as numbers, bools, or null, with a quoted form
// ("007") forcing a literal string.
func parseCJSON(condensed string) string {
	p := &cjsonParser{input: condensed}
	jsonBytes, _ := json.Marshal(p.parseValue())
	return string(jsonBytes)
}

type cjsonParser struct {
	input string
	pos   int
}

func (p *cjsonParser) parseValue() interface{} {
	p.skipSpace()
	if p.pos < len(p.input) {
		switch p.input[p.pos] {
		case '{':
			return p.parseObject()
		case '[':
			return p.parseArray()
		}
	}
	return inferScalar(p.parseScalar())
}

func (p *cjsonParser) parseObject() map[string]interface{} {
	obj := make(map[string]interface{})
	p.pos++ // consume '{'
	for p.pos < len(p.input) {
		p.skipSpace()
		if p.pos >= len(p.input) || p.input[p.pos] == '}' {
			p.pos++ // consume '}' (or stop at end for unclosed input)
			break
		}

		key := p.parseKey()
		p.skipSpace()
		if p.pos < len(p.input) && p.input[p.pos] == ':' {
			p.pos++ // optional colon
		}

		if key != "" {
			obj[key] = p.parseValue()
		}

		p.skipSpace()
		if p.pos < len(p.input) && p.input[p.pos] == ',' {
			p.pos++
		}
	}
	return obj
}

func (p *cjsonParser) parseArray() []interface{} {
	arr := []interface{}{}
	p.pos++ // consume '['
	for p.pos < len(p.input) {
		p.skipSpace()
		if p.pos >= len(p.input) || p.input[p.pos] == ']' {
			p.pos++ // consume ']' (or stop at end for unclosed input)
			break
		}

		arr = append(arr, p.parseValue())

		p.skipSpace()
		if p.pos < len(p.input) && p.input[p.pos] == ',' {
			p.pos++
		}
	}
	return arr
}

// parseKey reads up to the next structural character. A quoted key (' or ")
// is read to its matching close quote and unwrapped, so interpolated JSON
// keys like "name": work and delimiters inside quotes are kept.
func (p *cjsonParser) parseKey() string {
	if p.pos < len(p.input) && (p.input[p.pos] == '"' || p.input[p.pos] == '\'') {
		quote := p.input[p.pos]
		p.pos++
		start := p.pos
		for p.pos < len(p.input) && p.input[p.pos] != quote {
			p.pos++
		}
		key := p.input[start:p.pos]
		if p.pos < len(p.input) {
			p.pos++ // consume the closing quote
		}
		return key
	}

	start := p.pos
	for p.pos < len(p.input) && !strings.ContainsRune(":{[,}]", rune(p.input[p.pos])) {
		p.pos++
	}
	return strings.TrimSpace(p.input[start:p.pos])
}

// parseScalar reads a value up to the next structural delimiter. A value that
// begins with a quote (' or ") is read to its matching close quote, so any
// delimiters inside it are kept; a quote in the middle of a bare word (e.g.
// O'Brien) is just a character.
func (p *cjsonParser) parseScalar() string {
	start := p.pos
	if p.pos < len(p.input) && (p.input[p.pos] == '"' || p.input[p.pos] == '\'') {
		quote := p.input[p.pos]
		p.pos++
		for p.pos < len(p.input) && p.input[p.pos] != quote {
			p.pos++
		}
		if p.pos < len(p.input) {
			p.pos++ // consume the closing quote
		}
		return strings.TrimSpace(p.input[start:p.pos])
	}

	for p.pos < len(p.input) && p.input[p.pos] != ',' && p.input[p.pos] != '}' && p.input[p.pos] != ']' {
		p.pos++
	}
	return strings.TrimSpace(p.input[start:p.pos])
}

func (p *cjsonParser) skipSpace() {
	for p.pos < len(p.input) && (p.input[p.pos] == ' ' || p.input[p.pos] == '\t') {
		p.pos++
	}
}

// inferScalar maps a raw CJSON scalar to its JSON type. A quoted value (single
// or double) is taken literally (so "007" or '2' stay strings); otherwise
// true/false/null and numbers are recognised. Bare digit runs that overflow
// int64 stay strings rather than lose precision as floats (e.g. long IDs).
func inferScalar(s string) interface{} {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}

	switch s {
	case "true":
		return true
	case "false":
		return false
	case "null":
		return nil
	case "":
		return ""
	}

	if strings.ContainsAny(s, ".eE") {
		if f, err := strconv.ParseFloat(s, 64); err == nil && !math.IsInf(f, 0) && !math.IsNaN(f) {
			return f
		}
	} else if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	return s
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
		case map[string]interface{}, []interface{}:
			if jsonBytes, err := json.Marshal(v); err == nil {
				strValue = string(jsonBytes)
			} else {
				strValue = fmt.Sprint(v)
			}
		default:
			strValue = fmt.Sprint(v)
		}
		result = strings.ReplaceAll(result, placeholder, strValue)
	}
	return result
}

// resolvePath expands a request path. A path (or the part before the query
// string) that is exactly the name of a string variable resolves to its
// value, so endpoints can be aliased: addUser = users/new, then
// post(addUser {..}). A leading slash forces a literal path — g(/users)
// never consults variables. ${var} refs are interpolated afterwards, so an
// alias like users/${id}/posts binds at request time. The second return
// reports whether an alias fired.
func resolvePath(path string, variables map[string]interface{}) (string, bool) {
	base, query := path, ""
	if i := strings.Index(path, "?"); i != -1 {
		base, query = path[:i], path[i:]
	}
	aliased := false
	if !strings.HasPrefix(base, "/") {
		if v, ok := variables[base]; ok {
			if s, isStr := v.(string); isStr {
				base = s
				aliased = true
			}
		}
	}
	return interpolateVars(base+query, variables), aliased
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

func NewRequest(input string, baseURL string, variables map[string]interface{}, sessionHeaders map[string]string, debug bool, out io.Writer) (*Request, error) {
	inlineHeaders, cleanInput := parseInlineHeaders(input)

	// resolve expands the path, echoing a dim note when an alias variable
	// fired so substitution is never silent.
	resolve := func(path string) string {
		resolved, aliased := resolvePath(path, variables)
		if aliased {
			fmt.Fprintf(out, "%s→ %s%s\n", CGray, resolved, CReset)
		}
		return resolved
	}

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
		path = resolve(path)
		return &Request{Body: "", Headers: headers, Method: "DELETE", Url: buildURL(baseURL, path)}, nil
	case strings.HasPrefix(cleanInput, "d("):
		path := strings.TrimSuffix(strings.TrimPrefix(cleanInput, "d("), ")")
		path = resolve(path)
		return &Request{Body: "", Headers: headers, Method: "DELETE", Url: buildURL(baseURL, path)}, nil
	case strings.HasPrefix(cleanInput, "get("):
		path := strings.TrimSuffix(strings.TrimPrefix(cleanInput, "get("), ")")
		path = resolve(path)
		return &Request{Body: "", Headers: headers, Method: "GET", Url: buildURL(baseURL, path)}, nil
	case strings.HasPrefix(cleanInput, "g("):
		path := strings.TrimSuffix(strings.TrimPrefix(cleanInput, "g("), ")")
		path = resolve(path)
		return &Request{Body: "", Headers: headers, Method: "GET", Url: buildURL(baseURL, path)}, nil
	case strings.HasPrefix(cleanInput, "post("):
		pattern := `post\(([^\s]+)(?:\s+(.+))?\)`
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(cleanInput)
		if len(matches) < 2 {
			return nil, fmt.Errorf(CRed + "? ... post(/path {key:val})" + CReset)
		}
		path := strings.TrimSpace(matches[1])
		path = resolve(path)
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
		path = resolve(path)
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
		path = resolve(path)
		body, contentType := parseBody(matches[2], variables)
		return &Request{Body: body, ContentType: contentType, Headers: headers, Method: "PATCH", Url: buildURL(baseURL, path)}, nil
	default:
		return nil, fmt.Errorf("?")
	}
}

func (r *Request) Execute(variables map[string]interface{}, debug bool, out io.Writer) (Response, error) {
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
		token := fmt.Sprint(authToken)
		if parts := strings.SplitN(token, ":", 2); len(parts) == 2 {
			req.SetBasicAuth(parts[0], parts[1])
		} else {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}
	if r.Body != "" && r.ContentType != "" {
		req.Header.Set("Content-Type", r.ContentType)
	}

	if debug {
		fmt.Printf("DEBUG: %s %s\n", r.Method, r.Url)
		fmt.Printf("DEBUG: Headers: %v\n", req.Header)
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

	fmt.Fprintf(out, "%s✓ %d %s (%dms)%s\n", statusColor(resp.StatusCode), resp.StatusCode, http.StatusText(resp.StatusCode), elapsed.Milliseconds(), CReset)

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
		return encodeFormBody(bodyPart), "application/x-www-form-urlencoded"
	}

	if strings.HasPrefix(bodyPart, "\"") && strings.HasSuffix(bodyPart, "\"") {
		return strings.Trim(bodyPart, "\""), "text/plain"
	}

	if strings.HasPrefix(bodyPart, "{") {
		return parseCJSON(bodyPart), "application/json"
	}

	// Bare variable name, optionally with a path: post(users body). A string
	// value holding CJSON expands like an inline body; structured values
	// (extracted from responses) are sent as JSON directly.
	if value, ok := resolveVariable(bodyPart, variables); ok {
		switch v := value.(type) {
		case map[string]interface{}, []interface{}:
			if jsonBytes, err := json.Marshal(v); err == nil {
				return string(jsonBytes), "application/json"
			}
		case string:
			s := strings.TrimSpace(v)
			if strings.HasPrefix(s, "{") {
				return parseCJSON(s), "application/json"
			}
			if strings.HasPrefix(s, "?") {
				return encodeFormBody(s), "application/x-www-form-urlencoded"
			}
			return s, "text/plain"
		default:
			return fmt.Sprint(v), "text/plain"
		}
	}

	return "", ""
}

// encodeFormBody turns a ?key=val&key2=val2 body into its url-encoded form.
func encodeFormBody(formData string) string {
	values := url.Values{}
	for _, pair := range strings.Split(strings.TrimPrefix(formData, "?"), "&") {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			values.Add(parts[0], parts[1])
		}
	}
	return values.Encode()
}

// runStartupScript loads .rapidvars into the session. A file starting with
// '{' keeps its legacy meaning: a JSON object of variables plus $$header:
// entries. Anything else is a startup script of REPL commands, one per line.
// Lines before the first @host section run for every session; an @host
// section runs only when it matches the session base URL. Script commands
// run silently (errors still print); --debug shows their full output.
func runStartupScript(s *Session, filename string) {
	if s.inScript || filename == "" {
		return
	}
	s.inScript = true
	defer func() { s.inScript = false }()
	s.scriptFile = filename

	data, err := os.ReadFile(filename)
	if err != nil {
		return
	}

	content := strings.TrimSpace(string(data))
	if content == "" {
		return
	}

	if strings.HasPrefix(content, "{") {
		raw := make(map[string]interface{})
		if err := json.Unmarshal(data, &raw); err != nil {
			fmt.Printf("%sX %s: %v%s\n", CRed, filename, err, CReset)
			return
		}
		for k, v := range raw {
			if strings.HasPrefix(k, "$$header:") {
				headerName := strings.TrimPrefix(k, "$$header:")
				s.headers[strings.ToLower(headerName)] = fmt.Sprint(v)
			} else {
				s.variables[k] = v
			}
		}
		return
	}

	if !s.debug {
		prevOut := s.out
		s.out = io.Discard
		defer func() { s.out = prevOut }()
	}

	active := true // global until the first @host section
	commands := 0
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "@") {
			active = hostMatches(strings.TrimPrefix(line, "@"), s.baseURL)
			continue
		}
		if !active {
			continue
		}
		commands++
		if !s.Execute(line) {
			break
		}
	}
	if commands > 0 {
		fmt.Printf("%s%s: %d command(s)%s\n", CGray, filename, commands, CReset)
	}
}

// hostMatches compares an @section header against the session base URL by
// host[:port], ignoring scheme and path on either side.
func hostMatches(section, baseURL string) bool {
	return strings.EqualFold(hostOf(section), hostOf(baseURL))
}

func hostOf(url string) string {
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	return strings.SplitN(url, "/", 2)[0]
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
