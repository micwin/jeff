package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"jeff/internal/config"
	"jeff/internal/sidecars"
)

const (
	vaultlineDefaultAddr = "127.0.0.1:8428"
	vaultlineStoreName   = "jeff"
)

type vaultlineStoreInfo struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Available bool   `json:"available"`
	Sealed    bool   `json:"sealed"`
}

func managedVaultlineArgs(ctx context.Context, cc *commandContext, runner *vaultlineRunner, args []string) ([]string, error) {
	cfg, err := cc.loadConfig()
	if err != nil {
		return nil, err
	}
	addr := strings.TrimSpace(cfg.Vaultline.Addr)
	baseArgs := vaultlineBaseArgs(addr)
	if err := ensureVaultlineDaemon(ctx, runner, baseArgs); err != nil {
		return nil, err
	}
	if err := ensureJeffVaultlineStore(ctx, cc, cfg, addr, runner, baseArgs); err != nil {
		return nil, err
	}
	return append(baseArgs, args...), nil
}

func ensureVaultlineDaemon(ctx context.Context, runner *vaultlineRunner, baseArgs []string) error {
	if err := runner.Run(ctx, append(baseArgs, "health"), sidecars.Stdio{
		Stdout: io.Discard,
		Stderr: io.Discard,
	}); err != nil {
		return fmt.Errorf("vaultline daemon is not running or is unreachable; start it with 'vaultline daemon' or 'jeff vl daemon': %w", err)
	}
	return nil
}

func ensureJeffVaultlineStore(ctx context.Context, cc *commandContext, cfg *config.Config, addr string, runner *vaultlineRunner, baseArgs []string) error {
	passphrase := strings.TrimSpace(cfg.Vaultline.JeffStorePassphrase)
	if passphrase == "" {
		var err error
		passphrase, err = generateVaultlinePassphrase()
		if err != nil {
			return err
		}
		cfg.Vaultline.JeffStorePassphrase = passphrase
		if err := cc.saveConfig(cfg); err != nil {
			return err
		}
	}
	storePath, err := cc.store.VaultlineStoreDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(storePath), 0o755); err != nil {
		return fmt.Errorf("create vaultline parent dir: %w", err)
	}
	info, err := showJeffVaultlineStore(ctx, runner, baseArgs)
	if err != nil {
		exists, existsErr := pathExists(storePath)
		if existsErr != nil {
			return existsErr
		}
		if exists {
			if err := addJeffVaultlineStore(ctx, runner, baseArgs, storePath); err != nil {
				return err
			}
		} else if err := createJeffVaultlineStore(ctx, addr, storePath, passphrase); err != nil {
			return err
		}
		info, err = showJeffVaultlineStore(ctx, runner, baseArgs)
		if err != nil {
			return err
		}
	}
	if err := assertJeffVaultlineStorePath(info, storePath); err != nil {
		return err
	}
	if !info.Sealed {
		return nil
	}
	return unsealJeffVaultlineStore(ctx, runner, baseArgs, passphrase)
}

func vaultlineBaseArgs(addr string) []string {
	if strings.TrimSpace(addr) == "" {
		return nil
	}
	return []string{"--addr", strings.TrimSpace(addr)}
}

func showJeffVaultlineStore(ctx context.Context, runner *vaultlineRunner, baseArgs []string) (*vaultlineStoreInfo, error) {
	out, err := runner.Output(ctx, append(baseArgs, "store", "show", vaultlineStoreName))
	if err != nil {
		return nil, err
	}
	var info vaultlineStoreInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return nil, fmt.Errorf("parse vaultline store info: %w", err)
	}
	return &info, nil
}

func addJeffVaultlineStore(ctx context.Context, runner *vaultlineRunner, baseArgs []string, storePath string) error {
	if err := runner.Run(ctx, append(baseArgs, "store", "add", vaultlineStoreName, storePath), sidecars.Stdio{
		Stdout: io.Discard,
		Stderr: io.Discard,
	}); err != nil {
		return fmt.Errorf("register Jeff Vaultline store: %w", err)
	}
	return nil
}

func createJeffVaultlineStore(ctx context.Context, addr, storePath, passphrase string) error {
	if strings.TrimSpace(addr) == "" {
		addr = vaultlineDefaultAddr
	}
	payload, err := json.Marshal(map[string]any{
		"name":                vaultlineStoreName,
		"path":                storePath,
		"initialize":          true,
		"passphrase":          passphrase,
		"remember_passphrase": false,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+addr+"/api/v1/stores", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create Jeff Vaultline store failed: %s", strings.TrimSpace(string(body)))
	}
	return nil
}

func assertJeffVaultlineStorePath(info *vaultlineStoreInfo, expected string) error {
	if info == nil {
		return errors.New("missing Jeff Vaultline store info")
	}
	actual, err := filepath.Abs(info.Path)
	if err != nil {
		return err
	}
	expected, err = filepath.Abs(expected)
	if err != nil {
		return err
	}
	if filepath.Clean(actual) != filepath.Clean(expected) {
		return fmt.Errorf("vaultline store %q points to %s, expected %s", vaultlineStoreName, actual, expected)
	}
	return nil
}

func unsealJeffVaultlineStore(ctx context.Context, runner *vaultlineRunner, baseArgs []string, passphrase string) error {
	if err := runner.Run(ctx, append(baseArgs, "store", "unseal", vaultlineStoreName, "--value", passphrase, "--transient"), sidecars.Stdio{
		Stdout: io.Discard,
		Stderr: io.Discard,
	}); err != nil {
		return fmt.Errorf("unseal Jeff Vaultline store: %w", err)
	}
	info, err := showJeffVaultlineStore(ctx, runner, baseArgs)
	if err != nil {
		return err
	}
	if info.Sealed {
		return errors.New("Jeff Vaultline store is still sealed after unseal")
	}
	return nil
}

func generateVaultlinePassphrase() (string, error) {
	var buf [48]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("stat %s: %w", path, err)
}
