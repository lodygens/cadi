// Package platform regroupe les briques communes : HTTP, erreurs, traces, santé, arrêt.
// Squelette (ADR 20261030-socle, T1) : aucune logique, chaque corps renvoie
// ErrNotImplemented ou une valeur zéro.
package platform

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// ErrNotImplemented est renvoyée par tout corps du squelette.
var ErrNotImplemented = errors.New("non implémenté")

// Codes de sortie des binaires (D4).
const (
	ExitOK          = 0
	ExitFailure     = 1
	ExitConfigError = 2
)

// Paramètres de la santé (D9).
const (
	HealthInterval = 60 * time.Second
	HealthMaxAge   = 150 * time.Second
)

// NewLogger crée un logger JSON écrivant sur w.
func NewLogger(w io.Writer, level slog.Level) *slog.Logger { return nil }

// NewHTTPServer : délais de D4 ; h enveloppé par Recover, AccessLog, http.MaxBytesHandler(maxBody).
func NewHTTPServer(addr string, h http.Handler, maxBody int64, log *slog.Logger) *http.Server {
	return nil
}

// Serve écoute jusqu'à l'annulation de ctx puis Shutdown borné ; nil après un arrêt propre.
func Serve(ctx context.Context, srv *http.Server, shutdownTimeout time.Duration) error {
	return ErrNotImplemented
}
