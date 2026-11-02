package config

import "log/slog"

// Secret encapsule une valeur secrète ; toutes ses représentations valent "***" (D5, CA14).
// Les méthodes sont sur le type valeur.
type Secret struct{ v string }

// NewSecret crée un Secret.
func NewSecret(v string) Secret { return Secret{v: v} }

// Reveal renvoie la valeur, au point d'usage seulement.
func (s Secret) Reveal() string { return "" }

// String doit valoir "***".
func (Secret) String() string { return "" }

// GoString doit valoir "***".
func (Secret) GoString() string { return "" }

// LogValue doit valoir "***".
func (Secret) LogValue() slog.Value { return slog.Value{} }

// MarshalJSON doit valoir "\"***\"".
func (Secret) MarshalJSON() ([]byte, error) { return nil, errNotImplemented }

// MarshalText doit valoir "***".
func (Secret) MarshalText() ([]byte, error) { return nil, errNotImplemented }
