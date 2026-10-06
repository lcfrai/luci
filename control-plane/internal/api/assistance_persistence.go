package api

import (
	"os"
	"path/filepath"
)

// Paid-request receipts are a dispatch boundary. Keep their persistence
// separate from credential/configuration writes: publish only after file sync,
// then flush the renamed entry and newly created directory ancestry.
func writeAssistanceReceipt(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".assist-receipt-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = codexPrivateFile(name); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return publishAssistanceReceipt(name, path)
}
