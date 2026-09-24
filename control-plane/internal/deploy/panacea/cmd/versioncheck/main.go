// Command versioncheck prints Panacea's version for an origin, through the same TLS setup the
// control plane uses (panacea.BuildHTTPClient). It is the owner's check for 7P-10, run once at
// LIVE-CHECK.md step 1 (task 20): an x509 error means PANACEA_APP_PANACEA_CA_FILE is needed,
// never a skip flag. It is not run by any test or agent.
//
//	go run ./control-plane/internal/deploy/panacea/cmd/versioncheck https://dev.panacea.kpmg
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/panacea"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: versioncheck https://<panacea origin>")
		os.Exit(2)
	}
	hc, err := panacea.BuildHTTPClient(os.Getenv("PANACEA_APP_PANACEA_CA_FILE"), 15*time.Second)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	v, err := panacea.Version(ctx, hc, os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		var e *deploy.Error
		if errors.As(err, &e) && e.Cause != nil {
			fmt.Fprintln(os.Stderr, "cause:", e.Cause) // the x509 or proxy detail 7P-10 needs
		}
		os.Exit(1)
	}
	fmt.Println(v)
}
