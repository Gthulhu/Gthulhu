// SPDX-FileCopyrightText: 2025 Gthulhu Team
//
// SPDX-License-Identifier: Apache-2.0
// Author: Ian Chen <ychen.desl@gmail.com>

package main

import (
	"log/slog"
	"os"
	"runtime"

	"github.com/Gthulhu/Gthulhu/internal/app"
)

func main() {
	runtime.GOMAXPROCS(1)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	if err := app.Run(os.Args[1:]); err != nil {
		os.Exit(app.ExitCode(err))
	}
}
