package main

import (
	"flag"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/server"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	command := flag.String("command", "/bin/bash", "command to run in the PTY")
	args := flag.String("args", "", "comma-separated arguments")
	flag.Parse()

	var argv []string
	if *args != "" {
		argv = strings.Split(*args, ",")
	}

	srv := server.New(server.Config{Command: *command, Args: argv})
	slog.Info("sandboxd listening", "addr", *addr, "command", *command)
	if err := http.ListenAndServe(*addr, srv); err != nil {
		slog.Error("server exited", "err", err)
		os.Exit(1)
	}
}
