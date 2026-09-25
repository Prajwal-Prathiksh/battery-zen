//go:build linux

package main

import "github.com/Prajwal-Prathiksh/battery-zen/internal/config"

func configureRunLogging(_ config.Config) (func(), error) {
	return func() {}, nil
}
