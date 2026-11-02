// Commande cadi-notifier. Squelette (ADR 20261030-socle, T1) : aucune logique.
package main

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/lodygens/cadi/backend/internal/config"
	"github.com/lodygens/cadi/backend/internal/platform"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Getenv, os.ReadFile, os.Stdout)
	stop()
	os.Exit(code)
}

// run : args vide = fonctionnement normal ; "healthcheck" ; autre = code 2 (D4, D9).
// Squelette : renvoie l'échec, car ErrNotImplemented n'est pas un code de sortie.
func run(ctx context.Context, args []string, getenv config.Getenv, readFile config.ReadFile, stdout io.Writer) int {
	return platform.ExitFailure
}
