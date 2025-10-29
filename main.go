package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("RAPID v0.0.4 - Rapid API Dialogue")
		fmt.Println("Usage: rapid <base-url>")
		fmt.Println()
		fmt.Println("Warning: this is a WIP. Real functionality coming soon.")
		fmt.Println("Star: https://github.com/kupych/rapid")
		return
	}

	lastResponse := ""

	baseURL := os.Args[1]
	fmt.Printf("RAPID connected to %s\n", baseURL)
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("> ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "exit" || input == "quit" {
			break
		}

		switch {
		case input == "exit" || input == "quit":
			return
		case input == "?":
			fmt.Print(showHelp())
		case input == "$":
			fmt.Println(lastResponse)
		case strings.HasPrefix(input, "g("):
			path := strings.TrimSuffix(strings.TrimPrefix(input, "g("), ")")
			url := buildURL(baseURL, path)
			start := time.Now()
			resp, err := http.Get(url)
			elapsed := time.Since(start)
			if err != nil {
				fmt.Println("Could not complete request: ", err)
				continue
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				fmt.Println("Could not parse response body: ", err)
				continue

			}
			fmt.Printf("✓ %d %s (%dms)\n", resp.StatusCode, http.StatusText(resp.StatusCode), elapsed.Milliseconds())

			var data interface{}
			if err := json.Unmarshal(body, &data); err != nil {
				fmt.Println(string(body))
			} else {
				pretty, _ := json.MarshalIndent(data, "", " ")
				fmt.Println(string(pretty))
				lastResponse = string(pretty)
			}
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
Commands:
g(<path>) - GET request
$ - Show last response
? - Show this help
exit,quit - Exit rapid

Examples:

  g(users)
  g(users/1)
  $
  ?
`
}
