package main

import (
	"flag"
	"fmt"
)

func runValidateTemplates(args []string) error {
	fs := flag.NewFlagSet("validate-templates", flag.ExitOnError)
	templatesDir := fs.String("templates", "", "templates dir (directory containing *.gotmpl and includes/)")
	distDir := fs.String("dist", "", "dist directory (default ./dist or BBBENCH_DIST_DIR)")
	_ = fs.Parse(args)

	dir := resolveDistDir(*distDir)

	_, err := loadTemplatesWithSearch(*templatesDir, dir)
	if err != nil {
		return fmt.Errorf("templates invalid: %w", err)
	}
	logger.Info("templates valid")
	fmt.Println("✔ templates are valid")
	return nil
}
