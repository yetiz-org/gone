// Command goai is the OpenAPI generation CLI for gone-framework projects.
//
// Subcommands:
//
//	goai emit --root <dir> [-o <file>] [-args ...]
//	    Delegate to a project-side entry point that imports goai. The CLI
//	    looks for, in order, ./cmd/goaispec, ./cmd/goaiprobe, then ./goaispec
//	    underneath --root, and runs `go run` against the first one found.
//	    Anything after `-args` is forwarded verbatim to that binary.
//
//	    Why the indirection: Go cannot dynamically import handler packages,
//	    so walking a route tree requires a project-side binary that imports
//	    the project's handler package at compile time. `goai emit` is a
//	    convenience wrapper that locates and executes that binary for you.
//
//	goai merge --generated <file> --existing <file> [-o <file>]
//	    Three-way merge between auto-generated and hand-tuned OpenAPI yaml
//	    documents. Existing wins for overlapping keys; new
//	    paths/schemas/components from generated are appended. Pure file
//	    operation — no route walking, no Go package import.
//
//	goai lint <file>
//	    Validate that an OpenAPI yaml file parses, declares openapi 3.x,
//	    has a non-empty info object, and at least one path. Pure file
//	    operation.
//
//	goai version
//	    Print the bundled goai package version.
//
//	goai help
//	    Print this usage message.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/yetiz-org/gone/goai"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	cmd, args := os.Args[1], os.Args[2:]

	switch cmd {
	case "emit":
		exit(runEmit(args))
	case "lint":
		exit(runLint(args))
	case "merge":
		exit(runMerge(args))
	case "version":
		fmt.Println("goai " + goai.Version)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "goai: unknown subcommand %q\n", cmd)
		printUsage()
		os.Exit(2)
	}
}

func exit(err error) {
	if err == nil {
		return
	}

	fmt.Fprintf(os.Stderr, "goai: %v\n", err)
	os.Exit(1)
}

func printUsage() {
	fmt.Println(`goai — OpenAPI generation CLI for gone-framework projects.

Usage:
  goai emit --root <dir> [-o <file>] [-args ...]
  goai merge --generated <file> --existing <file> [-o <file>]
  goai lint <file>
  goai version
  goai help

emit delegates to ./cmd/goaispec (or ./cmd/goaiprobe, or ./goaispec) under
--root. Everything after -args is forwarded to the project binary.
merge and lint are pure file operations and require no project code.`)
}

// runEmit locates and executes a project-side entry point under --root that
// imports goai and walks the project's route tree. It tries these paths in
// order and runs the first one that exists with `go run`:
//
//	<root>/cmd/goaispec
//	<root>/cmd/goaiprobe
//	<root>/goaispec
//
// Anything supplied after `-args` on the command line is forwarded to the
// project binary verbatim, so callers can pass `-o`, `-title`, etc.
func runEmit(args []string) error {
	fs := flag.NewFlagSet("goai emit", flag.ContinueOnError)
	root := fs.String("root", ".", "project root containing a goai entry point")
	output := fs.String("o", "", "output path (forwarded to the project binary as -o)")

	// The remainder after `-args` is forwarded verbatim. Pre-split so flag
	// parsing does not choke on project-specific flags.
	var passthrough []string
	cli := args
	for i, a := range args {
		if a == "-args" || a == "--args" {
			cli = args[:i]
			passthrough = args[i+1:]

			break
		}
	}

	if err := fs.Parse(cli); err != nil {
		return err
	}

	candidates := []string{
		filepath.Join(*root, "cmd", "goaispec"),
		filepath.Join(*root, "cmd", "goaiprobe"),
		filepath.Join(*root, "goaispec"),
	}

	var entry string
	for _, c := range candidates {
		if dirExists(c) {
			entry = c

			break
		}
	}

	if entry == "" {
		return fmt.Errorf("no project entry point found under %s. "+
			"Add a tiny cmd/goaispec/main.go that calls goai.RunCLI(...) — "+
			"see github.com/yetiz-org/gone/goai/examples/full for a template.", *root)
	}

	rel, err := filepath.Rel(*root, entry)
	if err != nil {
		rel = entry
	}

	runArgs := []string{"run", "./" + rel}
	if *output != "" {
		runArgs = append(runArgs, "-o", *output)
	}

	runArgs = append(runArgs, passthrough...)

	cmd := exec.Command("go", runArgs...)
	cmd.Dir = *root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("delegate %s failed: %w", entry, err)
	}

	return nil
}

// runMerge performs a pure file-to-file three-way merge using
// goai.Merge3Way. Useful in CI / Makefile when you have a generated
// OpenAPI yaml and want to layer it onto a hand-tuned base without
// spinning up Go again.
func runMerge(args []string) error {
	fs := flag.NewFlagSet("goai merge", flag.ContinueOnError)
	generated := fs.String("generated", "", "auto-generated OpenAPI yaml")
	existing := fs.String("existing", "", "hand-tuned baseline yaml (wins on overlap)")
	output := fs.String("o", "-", "output file; '-' for stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *generated == "" || *existing == "" {
		return fmt.Errorf("merge requires both --generated and --existing")
	}

	gBytes, err := os.ReadFile(*generated)
	if err != nil {
		return fmt.Errorf("read generated %s: %w", *generated, err)
	}

	eBytes, err := os.ReadFile(*existing)
	if err != nil {
		return fmt.Errorf("read existing %s: %w", *existing, err)
	}

	merged, err := goai.Merge3Way(gBytes, eBytes, nil)
	if err != nil {
		return err
	}

	if *output == "-" {
		_, err = os.Stdout.Write(merged)

		return err
	}

	if err := os.WriteFile(*output, merged, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", *output, err)
	}

	fmt.Fprintf(os.Stderr, "goai: wrote %d bytes to %s\n", len(merged), *output)

	return nil
}

// runLint performs a lightweight structural check on an OpenAPI yaml file:
// the document must parse, declare openapi 3.x, carry a non-empty info
// object, and have at least one path. This catches the most common "ship
// broken yaml" failure modes without depending on a full validator like
// Spectral or Redocly Lint.
func runLint(args []string) error {
	fs := flag.NewFlagSet("goai lint", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("lint requires a yaml file path")
	}

	path := fs.Arg(0)
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	report, err := goai.LintYAML(body)
	if err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	if len(report.Errors) > 0 {
		fmt.Fprintf(os.Stderr, "goai lint: %d error(s) in %s\n", len(report.Errors), path)
		for _, e := range report.Errors {
			fmt.Fprintln(os.Stderr, "  -", e)
		}

		return fmt.Errorf("lint failed")
	}

	fmt.Fprintf(os.Stderr, "goai lint: %s ok (%d paths, %d operations)\n", path, report.Paths, report.Operations)

	return nil
}

// dirExists reports whether the given path exists and is a directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return info.IsDir()
}
