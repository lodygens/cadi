package platform

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// Monitor contrôle périodiquement une dépendance et tient l'état de santé (D9).
type Monitor struct {
	// Champs à poser par l'implémentation : check, interval, timeout, file, now, log,
	// et l'état protégé (dernier résultat, dernier succès).
}

// NewMonitor crée un moniteur ; il ne démarre rien.
func NewMonitor(check func(ctx context.Context) error, interval, timeout time.Duration,
	file string, now func() time.Time, log *slog.Logger) *Monitor {
	return nil
}

// Start : premier contrôle synchrone, puis boucle en goroutine jusqu'à l'annulation de ctx.
func (m *Monitor) Start(ctx context.Context) {}

// Healthy : dernier contrôle réussi et dernier succès de moins de HealthMaxAge.
func (m *Monitor) Healthy() bool { return false }

// WriteHealthFile : RFC 3339 UTC, fichier temporaire du même répertoire puis renommage.
func WriteHealthFile(path string, t time.Time) error { return ErrNotImplemented }

// CheckHealthFile : nil si la date lue est à moins de maxAge de now (dans les deux sens).
func CheckHealthFile(path string, now time.Time, maxAge time.Duration) error {
	return ErrNotImplemented
}

// CheckHTTP (healthcheck de l'API) : nil si GET url répond 200 ou 503 en moins de timeout.
func CheckHTTP(ctx context.Context, client *http.Client, url string, timeout time.Duration) error {
	return ErrNotImplemented
}

// LocalURL : ":8080" ou "0.0.0.0:8080" → "http://127.0.0.1:8080".
func LocalURL(addr string) (string, error) { return "", ErrNotImplemented }
