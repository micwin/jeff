package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"jeff/internal/sidecars"
)

const (
	vaultlineIdleTimeout = 5 * time.Minute
	vaultlineStoreName   = "jeff"
)

type vaultlineRuntime struct {
	Addr       string `json:"addr"`
	PID        int    `json:"pid"`
	VaultDir   string `json:"vault_dir"`
	LastUsedAt int64  `json:"last_used_at"`
}

func managedVaultlineArgs(ctx context.Context, cc *commandContext, args []string) ([]string, error) {
	rt, err := ensureVaultlineDaemon(ctx, cc)
	if err != nil {
		return nil, err
	}
	if err := ensureJeffVaultlineStore(ctx, cc, rt); err != nil {
		return nil, err
	}
	if err := touchVaultlineRuntime(cc, rt); err != nil {
		return nil, err
	}
	if err := startVaultlineWatchdog(cc, rt); err != nil {
		return nil, err
	}
	return append([]string{"--addr", rt.Addr}, args...), nil
}

func stopManagedVaultline(ctx context.Context, cc *commandContext) error {
	rt, err := loadVaultlineRuntime(cc)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	_ = sidecars.Run(ctx, "vaultline", []string{"--addr", rt.Addr, "daemon-stop"}, sidecars.Stdio{
		Stdout: io.Discard,
		Stderr: io.Discard,
	})
	_ = removeVaultlineRuntime(cc)
	return nil
}

func ensureVaultlineDaemon(ctx context.Context, cc *commandContext) (*vaultlineRuntime, error) {
	if rt, err := loadVaultlineRuntime(cc); err == nil {
		if processAlive(rt.PID) && vaultlineHealth(ctx, rt.Addr) == nil {
			return rt, nil
		}
		_ = removeVaultlineRuntime(cc)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	paths, err := vaultlinePaths(cc)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(paths.vaultDir, "stores"), 0o700); err != nil {
		return nil, fmt.Errorf("create vaultline dir: %w", err)
	}
	addr, err := freeLoopbackAddr()
	if err != nil {
		return nil, err
	}
	null, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	defer null.Close()
	process, err := sidecars.Start(ctx, "vaultline", []string{
		"daemon",
		"--addr", addr,
		"--store-dir", filepath.Join(paths.vaultDir, "stores", "default"),
		"--config-file", paths.storeConfig,
		"--daemon-config-file", paths.daemonConfig,
	}, sidecars.Stdio{
		Stdin:  null,
		Stdout: null,
		Stderr: null,
	}, nil)
	if err != nil {
		return nil, err
	}
	rt := &vaultlineRuntime{
		Addr:       addr,
		PID:        process.Pid,
		VaultDir:   paths.vaultDir,
		LastUsedAt: time.Now().Unix(),
	}
	if err := waitForVaultline(ctx, addr); err != nil {
		_ = syscall.Kill(process.Pid, syscall.SIGTERM)
		return nil, err
	}
	if err := saveVaultlineRuntime(cc, rt); err != nil {
		return nil, err
	}
	return rt, nil
}

func ensureJeffVaultlineStore(ctx context.Context, cc *commandContext, rt *vaultlineRuntime) error {
	cfg, err := cc.loadConfig()
	if err != nil {
		return err
	}
	passphrase := strings.TrimSpace(cfg.Vaultline.JeffStorePassphrase)
	if passphrase == "" {
		passphrase, err = generateVaultlinePassphrase()
		if err != nil {
			return err
		}
		cfg.Vaultline.JeffStorePassphrase = passphrase
		if err := cc.saveConfig(cfg); err != nil {
			return err
		}
	}
	out, err := sidecars.Output(ctx, "vaultline", []string{"--addr", rt.Addr, "store", "list", "--raw"})
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.TrimSpace(line) == vaultlineStoreName {
				return unsealJeffVaultlineStore(ctx, rt, passphrase)
			}
		}
	}
	storePath := filepath.Join(rt.VaultDir, "stores", vaultlineStoreName)
	if err := createJeffVaultlineStore(ctx, rt.Addr, storePath, passphrase); err != nil {
		return err
	}
	return unsealJeffVaultlineStore(ctx, rt, passphrase)
}

func createJeffVaultlineStore(ctx context.Context, addr, storePath, passphrase string) error {
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

func unsealJeffVaultlineStore(ctx context.Context, rt *vaultlineRuntime, passphrase string) error {
	_ = sidecars.Run(ctx, "vaultline", []string{"--addr", rt.Addr, "store", "unseal", vaultlineStoreName, "--value", passphrase, "--transient"}, sidecars.Stdio{
		Stdout: io.Discard,
		Stderr: io.Discard,
	})
	return nil
}

func generateVaultlinePassphrase() (string, error) {
	var buf [48]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

func waitForVaultline(ctx context.Context, addr string) error {
	deadline := time.Now().Add(5 * time.Second)
	for {
		if err := vaultlineHealth(ctx, addr); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("vaultline daemon did not become ready at %s", addr)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func vaultlineHealth(ctx context.Context, addr string) error {
	return sidecars.Run(ctx, "vaultline", []string{"--addr", addr, "health"}, sidecars.Stdio{
		Stdout: io.Discard,
		Stderr: io.Discard,
	})
}

type vaultlinePathSet struct {
	vaultDir     string
	storeConfig  string
	daemonConfig string
	runtimeDir   string
	runtimeFile  string
}

func vaultlinePaths(cc *commandContext) (*vaultlinePathSet, error) {
	vaultDir, err := cc.store.VaultlineDir()
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(vaultDir))
	id := hex.EncodeToString(sum[:])[:16]
	runtimeDir := filepath.Join(os.TempDir(), "jeff-vaultline", strconv.Itoa(os.Getuid()), id)
	return &vaultlinePathSet{
		vaultDir:     vaultDir,
		storeConfig:  filepath.Join(vaultDir, "stores.json"),
		daemonConfig: filepath.Join(vaultDir, "daemon.json"),
		runtimeDir:   runtimeDir,
		runtimeFile:  filepath.Join(runtimeDir, "runtime.json"),
	}, nil
}

func loadVaultlineRuntime(cc *commandContext) (*vaultlineRuntime, error) {
	paths, err := vaultlinePaths(cc)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(paths.runtimeFile)
	if err != nil {
		return nil, err
	}
	var rt vaultlineRuntime
	if err := json.Unmarshal(data, &rt); err != nil {
		return nil, err
	}
	return &rt, nil
}

func saveVaultlineRuntime(cc *commandContext, rt *vaultlineRuntime) error {
	paths, err := vaultlinePaths(cc)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(paths.runtimeDir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(rt, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(paths.runtimeFile, data, 0o600)
}

func touchVaultlineRuntime(cc *commandContext, rt *vaultlineRuntime) error {
	rt.LastUsedAt = time.Now().Unix()
	return saveVaultlineRuntime(cc, rt)
}

func removeVaultlineRuntime(cc *commandContext) error {
	paths, err := vaultlinePaths(cc)
	if err != nil {
		return err
	}
	return os.Remove(paths.runtimeFile)
}

func freeLoopbackAddr() (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer listener.Close()
	return listener.Addr().String(), nil
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func startVaultlineWatchdog(cc *commandContext, rt *vaultlineRuntime) error {
	paths, err := vaultlinePaths(cc)
	if err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, "--config", cc.store.Dir(), "__vl-watchdog", paths.runtimeFile)
	cmd.Stdin = nil
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func runVaultlineWatchdog(runtimeFile string) error {
	for {
		data, err := os.ReadFile(runtimeFile)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		var rt vaultlineRuntime
		if err := json.Unmarshal(data, &rt); err != nil {
			return err
		}
		if time.Since(time.Unix(rt.LastUsedAt, 0)) >= vaultlineIdleTimeout {
			_ = sidecars.Run(context.Background(), "vaultline", []string{"--addr", rt.Addr, "daemon-stop"}, sidecars.Stdio{
				Stdout: io.Discard,
				Stderr: io.Discard,
			})
			_ = os.Remove(runtimeFile)
			return nil
		}
		time.Sleep(15 * time.Second)
	}
}
