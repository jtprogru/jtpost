package main

import (
	"github.com/jtprogru/jtpost/internal/cli"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Передаём версию в CLI
	cli.SetVersion(version, commit, date)
	cli.Execute()
}
