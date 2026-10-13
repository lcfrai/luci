//go:build linux

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
	"github.com/lcfrai/luci/control-plane/internal/microvmguest"
)

func run() error {
	encoded := flag.String("identity-base64", "", "exact previously observed kernel launch identity")
	flag.Parse()
	if flag.NArg() != 0 || os.Geteuid() != 0 || len(*encoded) > 5500 {
		return microvmguest.ErrRefused
	}
	raw, err := base64.StdEncoding.Strict().DecodeString(*encoded)
	if err != nil || len(raw) > 4096 {
		return microvmguest.ErrRefused
	}
	var identity e2bbe.CaptureIdentity
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&identity); err != nil {
		return err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return microvmguest.ErrRefused
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	receipt, err := microvmguest.CollectTask(ctx, identity)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(receipt)
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Luci guest capture refused or remains unconfirmed:", err)
		os.Exit(1)
	}
}
