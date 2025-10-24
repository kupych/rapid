package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("RAPID v0.0.3 - Rapid API Dialogue")
		fmt.Println("Usage: rapid <base-url>")
		fmt.Println()
		fmt.Println("Warning: this is a WIP. Real functionality coming soon.")
		fmt.Println("Star: https://github.com/kupych/rapid")
		return
	}

	baseURL := os.Args[1]
	fmt.Printf("RAPID connected to %s\n", baseURL)
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("> ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "exit" || input == "quit" { break }

		if strings.HasPrefix(input, "g(") {
			path := strings.TrimSuffix(strings.TrimPrefix(input, "g("), ")")
			url := buildURL(baseURL, path)
			resp, err := http.Get(url)
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
		fmt.Printf("✓ %d %s\n", resp.StatusCode, http.StatusText(resp.StatusCode))

		var data interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			fmt.Println(string(body))
		} else {
			pretty, _ := json.MarshalIndent(data, "", " ")
			fmt.Println(string(pretty))

		}     
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

