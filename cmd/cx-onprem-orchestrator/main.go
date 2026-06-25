// Command cx-onprem-orchestrator is a single CLI that orchestrates an arbitrary
// subset of Checkmarx scanners (CxSAST on-prem, SCA via SCA Resolver, KICS, 2ms
// secrets, Container Security), replicates the cx CLI threshold/filter syntax,
// passes exact native flags to each engine, and reduces results to one exit code.
package main

import (
	"os"

	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/cli"

	// Register the available engine scanners.
	_ "github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/scanner/containers"
	_ "github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/scanner/kics"
	_ "github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/scanner/sast"
	_ "github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/scanner/sca"
	_ "github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/scanner/secrets"
)

func main() {
	os.Exit(cli.Execute())
}
