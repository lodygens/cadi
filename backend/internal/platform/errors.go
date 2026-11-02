package platform

import (
	"context"
	"log/slog"
	"net/http"
)

// ErrorCode est le code d'erreur renvoyé dans le corps JSON (D4).
type ErrorCode string

// Codes d'erreur des contrats public et interne.
const (
	CodeInvalidRequest   ErrorCode = "requete_invalide"
	CodeUnauthenticated  ErrorCode = "non_authentifie"
	CodeForbidden        ErrorCode = "interdit"
	CodeNotFound         ErrorCode = "introuvable"           // 404
	CodeMethodNotAllowed ErrorCode = "methode_non_autorisee" // 405
	CodeTooLarge         ErrorCode = "trop_volumineux"
	CodeInternal         ErrorCode = "erreur_interne"
)

// WriteError écrit {"code":…}, Content-Type: application/json, Cache-Control: no-store.
func WriteError(w http.ResponseWriter, status int, code ErrorCode) {}

// RequestErrorHandler : 413 trop_volumineux si err enveloppe *http.MaxBytesError, sinon
// 400 requete_invalide ; cause tracée.
func RequestErrorHandler(log *slog.Logger) func(w http.ResponseWriter, r *http.Request, err error) {
	return nil
}

// ResponseErrorHandler : 500 erreur_interne ; cause tracée avec RequestID.
func ResponseErrorHandler(log *slog.Logger) func(w http.ResponseWriter, r *http.Request, err error) {
	return nil
}

// Recover transforme une panic en 500 erreur_interne tracé.
func Recover(log *slog.Logger, h http.Handler) http.Handler { return nil }

// AccessLog écrit la trace d'accès (D4) et attribue l'identifiant de requête.
func AccessLog(log *slog.Logger, h http.Handler) http.Handler { return nil }

// RequestID renvoie l'identifiant de la requête en cours.
func RequestID(ctx context.Context) string { return "" }

// WithLogAttrs ajoute des attributs à la trace d'accès de la requête en cours (ex : "appelant").
func WithLogAttrs(ctx context.Context, attrs ...slog.Attr) {}

// SetRoute fixe le champ "route" de la trace d'accès (gabarit ou "inconnue") ; prioritaire sur r.Pattern.
func SetRoute(ctx context.Context, route string) {}
