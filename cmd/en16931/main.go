// en16931 — EN 16931:2017 invoice validator and UBL 2.1 renderer.
//
// This tool implements the business rules defined in the European Standard
// for electronic invoicing (EN 16931-1:2017) and produces output in the
// UBL 2.1 syntax binding specified in EN 16931-3-2.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/apayne185/en16931-toolkit/internal/model"
	"github.com/apayne185/en16931-toolkit/internal/ubl"
	"github.com/apayne185/en16931-toolkit/internal/validate"
)

const usage = `en16931 — EN 16931:2017 e-invoice validator and UBL 2.1 renderer

Usage:
  en16931 validate <invoice.json>            Check business rule compliance
  en16931 render   <invoice.json> [-o file]  Render to UBL 2.1 XML (validates first)

Examples:
  en16931 validate examples/simple_invoice.json
  en16931 render   examples/simple_invoice.json
  en16931 render   examples/simple_invoice.json -o out.xml
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "validate":
		fs := flag.NewFlagSet("validate", flag.ExitOnError)
		fs.Usage = func() { fmt.Fprintln(os.Stderr, "usage: en16931 validate <invoice.json>") }
		_ = fs.Parse(os.Args[2:])
		if fs.NArg() == 0 {
			fs.Usage()
			os.Exit(1)
		}
		inv := mustLoad(fs.Arg(0))
		runValidate(inv)

	case "render":
		fs := flag.NewFlagSet("render", flag.ExitOnError)
		outFile := fs.String("o", "", "write XML to `file` instead of stdout")
		fs.Usage = func() {
			fmt.Fprintln(os.Stderr, "usage: en16931 render <invoice.json> [-o output.xml]")
		}
		_ = fs.Parse(os.Args[2:])
		if fs.NArg() == 0 {
			fs.Usage()
			os.Exit(1)
		}
		inv := mustLoad(fs.Arg(0))
		runRender(inv, *outFile)

	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(1)
	}
}

func mustLoad(path string) *model.Invoice {
	f, err := os.Open(path)
	if err != nil {
		fatalf("cannot open %s: %v", path, err)
	}
	defer f.Close()

	var inv model.Invoice
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&inv); err != nil {
		fatalf("invalid JSON in %s: %v", path, err)
	}
	return &inv
}

func runValidate(inv *model.Invoice) {
	errs := validate.Validate(inv)
	if len(errs) == 0 {
		fmt.Printf("✓  %s passes EN 16931:2017 (%d rules checked)\n",
			inv.Number, ruleCount())
		return
	}

	noun := "error"
	if len(errs) != 1 {
		noun = "errors"
	}
	fmt.Fprintf(os.Stderr, "✗  Validation failed (%d %s):\n\n", len(errs), noun)
	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "   %-14s %s\n", e.Code, e.Message)
		if e.Path != "" {
			fmt.Fprintf(os.Stderr, "   %-14s └─ at: %s\n", "", e.Path)
		}
		fmt.Fprintln(os.Stderr)
	}
	os.Exit(1)
}

func runRender(inv *model.Invoice, outFile string) {
	errs := validate.Validate(inv)
	if len(errs) > 0 {
		fmt.Fprintf(os.Stderr,
			"✗  Cannot render: %d validation error(s). Run 'validate' for details.\n", len(errs))
		os.Exit(1)
	}

	xmlBytes, err := ubl.Render(inv)
	if err != nil {
		fatalf("render error: %v", err)
	}

	if outFile == "" {
		os.Stdout.Write(xmlBytes)
		return
	}

	if err := os.WriteFile(outFile, xmlBytes, 0o644); err != nil {
		fatalf("cannot write %s: %v", outFile, err)
	}
	fmt.Printf("✓  Wrote UBL 2.1 invoice to %s (%d bytes)\n", outFile, len(xmlBytes))
}

// ruleCount returns the number of distinct BR-* codes implemented.
// Updated manually when new rules are added.
func ruleCount() int {
	rules := strings.Fields(`BR-2 BR-3 BR-4 BR-5 BR-6 BR-7 BR-8 BR-9 BR-10 BR-16 BR-18
BR-19 BR-20 BR-21 BR-25 BR-26 BR-S-1 BR-S-2 BR-S-6 BR-Z-1 BR-Z-2
BR-E-1 BR-E-2 BR-AE-1 BR-AE-3 BR-K-1 BR-G-1 BR-O-1
BR-CO-9 BR-CO-11 BR-CO-12 BR-CO-13 BR-CO-14 BR-CO-15 BR-CO-16 BR-CO-17 BR-CO-18`)
	return len(rules)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
