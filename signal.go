package pzip

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func SetupSignalContext() context.Context {
	shutdownHandler := make(chan os.Signal, 2)
	ctx, cancel := context.WithCancel(context.Background())
	signal.Notify(shutdownHandler, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
	go func() {
		s := <-shutdownHandler
		_, _ = fmt.Fprintf(os.Stderr, "\nReceived signal: %s. Stopping...\n", s.String())
		_, _ = fmt.Fprintln(os.Stderr, "Send the signal again to force a shutdown.")
		cancel()
		<-shutdownHandler
		os.Exit(1)
	}()
	return ctx
}
