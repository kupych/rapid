package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("RAPID v0.0.1 - Rapid API Dialogue")
		fmt.Println("Usage: rapid <base-url>")
		fmt.Println()
		fmt.Println("Warning: this is a WIP. Real functionality coming soon.")
		fmt.Println("Star: https://github.com/kupych/rapid")
		return
	}

	baseURL := os.Args[1]
	fmt.Printf("RAPID connected to %s\n", baseURL)
	fmt.Println("v0.0.1: No commands yet. Stay tuned.")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("> ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "exit" || input == "quit" {
			fmt.Println("Bye")
			break
		}

		fmt.Println("Coming soon. Check Github for updates.")
	}
}

