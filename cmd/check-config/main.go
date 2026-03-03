package main

import (
	"fmt"
	"log"
	"os"

	"bbbench/internal/config"
)

func main() {
	configPath := "dist/default.yml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatal("Failed to load config: ", err)
	}

	fmt.Printf("Config loaded successfully from: %s\n\n", configPath)
	fmt.Printf("WorkloadsDB entries: %d\n", len(cfg.BBBench.WorkloadsDB))
	fmt.Printf("Workload types: %d\n\n", len(cfg.BBBench.Workloads))

	for typ, workloads := range cfg.BBBench.Workloads {
		fmt.Printf("%s: %d workloads\n", typ, len(workloads))
		if len(workloads) == 0 {
			fmt.Printf("  ⚠️  WARNING: No workloads defined for %s!\n", typ)
		} else {
			for i, w := range workloads {
				fmt.Printf("  %d. %s", i+1, w)
				if wdef, ok := cfg.BBBench.WorkloadsDB[w]; ok {
					jobCount := 0
					for _, count := range wdef.Process {
						if c, ok := count.(int); ok && c > 0 {
							jobCount++
						}
					}
					fmt.Printf(" (%d jobs)\n", jobCount)
				} else {
					fmt.Printf(" ⚠️  NOT FOUND IN DATABASE\n")
				}
			}
		}
		fmt.Println()
	}
}
