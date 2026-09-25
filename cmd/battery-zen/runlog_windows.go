//go:build windows

package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/Prajwal-Prathiksh/battery-zen/internal/config"
)

func configureRunLogging(cfg config.Config) (func(), error) {
	file, err := os.OpenFile(filepath.Join(cfg.LogDir, "background.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	log.SetOutput(file)
	return func() { _ = file.Close() }, nil
}
