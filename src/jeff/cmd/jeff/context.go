package main

import (
	"io"

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
