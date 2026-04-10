package main

import (
	"context"
	"errors"
	"io"

	"github.com/spf13/cobra"

	"jeff/internal/config"
)

type commandContext struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
	store  *config.Store
	status bool
}

func (c *commandContext) loadConfig() (*config.Config, error) {
	return c.store.Load()
}

func (c *commandContext) saveConfig(cfg *config.Config) error {
	return c.store.Save(cfg)
}

type contextKey struct{}

var storeCache struct {
	dir   string
	store *config.Store
}

func setCommandContext(cmd *cobra.Command, configDir string, status bool) error {
	store, err := cachedStore(configDir)
	if err != nil {
		return err
	}

	cc := &commandContext{
		stdin:  cmd.InOrStdin(),
		stdout: cmd.OutOrStdout(),
		stderr: cmd.ErrOrStderr(),
		store:  store,
		status: status,
	}

	ctx := context.WithValue(cmd.Context(), contextKey{}, cc)
	cmd.SetContext(ctx)
	return nil
}

func commandContextFrom(cmd *cobra.Command) (*commandContext, error) {
	val := cmd.Context().Value(contextKey{})
	if val == nil {
		return nil, errors.New("command context not initialized")
	}
	cc, ok := val.(*commandContext)
	if !ok {
		return nil, errors.New("command context invalid")
	}
	return cc, nil
}

func cachedStore(configDir string) (*config.Store, error) {
	if storeCache.store != nil && storeCache.dir == configDir {
		return storeCache.store, nil
	}
	store, err := config.NewStore(configDir)
	if err != nil {
		return nil, err
	}
	storeCache.store = store
	storeCache.dir = configDir
	return store, nil
}
