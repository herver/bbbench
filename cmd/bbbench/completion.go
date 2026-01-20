package main

import (
	"fmt"
)

func runCompletion(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: bbbench completion [bash|zsh|fish]")
	}
	switch args[0] {
	case "bash":
		fmt.Print(bashCompletion)
	case "zsh":
		fmt.Print(zshCompletion)
	case "fish":
		fmt.Print(fishCompletion)
	default:
		return fmt.Errorf("unknown shell: %s", args[0])
	}
	return nil
}
