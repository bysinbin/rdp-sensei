package store

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

const keychainService = "rdp-sensei"

// SavePasswordToKeychain saves or updates a password in the macOS Keychain.
func SavePasswordToKeychain(account, password string) error {
	if runtime.GOOS != "darwin" || account == "" {
		return nil
	}

	// -U flag updates item if it already exists
	cmd := exec.Command("security", "add-generic-password",
		"-s", keychainService,
		"-a", account,
		"-w", password,
		"-U",
	)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("keychain save error: %w (%s)", err, strings.TrimSpace(errBuf.String()))
	}
	return nil
}

// GetPasswordFromKeychain retrieves a password for the given account from macOS Keychain.
func GetPasswordFromKeychain(account string) (string, error) {
	if runtime.GOOS != "darwin" || account == "" {
		return "", nil
	}

	cmd := exec.Command("security", "find-generic-password",
		"-s", keychainService,
		"-a", account,
		"-w",
	)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("keychain get error: %w (%s)", err, strings.TrimSpace(errBuf.String()))
	}
	return strings.TrimSpace(outBuf.String()), nil
}

// DeletePasswordFromKeychain removes a stored password for the given account.
func DeletePasswordFromKeychain(account string) error {
	if runtime.GOOS != "darwin" || account == "" {
		return nil
	}

	cmd := exec.Command("security", "delete-generic-password",
		"-s", keychainService,
		"-a", account,
	)
	_ = cmd.Run() // Ignore if doesn't exist
	return nil
}
