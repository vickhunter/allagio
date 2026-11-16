// Command outboundvet is the go vet tool for the outbound-HTTP lint.
// It is its own module so the API module does not take a tools dependency.
package main

import "golang.org/x/tools/go/analysis/singlechecker"

func main() {
	singlechecker.Main(Analyzer)
}
