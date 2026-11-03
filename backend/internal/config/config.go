// Package config charge la configuration et les secrets (D5).
// Squelette (ADR 20261030-socle, T1) : aucune logique.
package config

import (
	"errors"
	"log/slog"
	"time"
)

// ErrInvalid est enveloppée avec le nom de la variable, jamais sa valeur.
var ErrInvalid = errors.New("configuration invalide")

// errNotImplemented remplace platform.ErrNotImplemented : D3 interdit à config d'importer
// platform. À supprimer à l'implémentation (T7) ; voir le rapport de T1.
var errNotImplemented = errors.New("non implémenté")

// Getenv lit une variable d'environnement.
type Getenv func(key string) string

// ReadFile lit un fichier (secrets).
type ReadFile func(path string) ([]byte, error)

// Database : paramètres communs à init-schema et au store, sans mot de passe.
type Database struct {
	Host, Name, SSLMode string
	Port                int
	StartupTimeout      time.Duration
}

// StoreTokens : CADI_STORE_TOKEN_*_FILE ; distincts.
type StoreTokens struct{ API, Collector, Notifier Secret }

// StoreClient : configuration du client du store.
type StoreClient struct {
	URL     string
	Timeout time.Duration
	Token   Secret
}

// StoreInit : sous-commande init-schema ; ne lit ni CADI_DB_APP_PASSWORD_FILE ni les jetons.
type StoreInit struct {
	DB            Database
	OwnerPassword Secret // CADI_DB_OWNER_PASSWORD_FILE
	LogLevel      slog.Level
}

// Store : ne lit jamais CADI_DB_OWNER_PASSWORD_FILE.
type Store struct {
	ListenAddr      string
	DB              Database
	AppPassword     Secret // CADI_DB_APP_PASSWORD_FILE
	MaxConns        int32
	Tokens          StoreTokens
	HealthFile      string
	LogLevel        slog.Level
	ShutdownTimeout time.Duration
}

// API : configuration de cadi-api.
type API struct {
	HTTPAddr        string
	Store           StoreClient
	LogLevel        slog.Level
	ShutdownTimeout time.Duration
}

// Worker : configuration de cadi-collector et cadi-notifier.
type Worker struct {
	HealthFile      string
	Store           StoreClient
	LogLevel        slog.Level
	ShutdownTimeout time.Duration
}

// LoadStoreInit charge la configuration de la sous-commande init-schema.
func LoadStoreInit(getenv Getenv, readFile ReadFile) (StoreInit, error) {
	return StoreInit{}, errNotImplemented
}

// LoadStore charge la configuration de cadi-store.
func LoadStore(getenv Getenv, readFile ReadFile) (Store, error) {
	return Store{}, errNotImplemented
}

// LoadAPI charge la configuration de cadi-api.
func LoadAPI(getenv Getenv, readFile ReadFile) (API, error) {
	return API{}, errNotImplemented
}

// LoadWorker charge la configuration de cadi-collector et cadi-notifier.
func LoadWorker(getenv Getenv, readFile ReadFile) (Worker, error) {
	return Worker{}, errNotImplemented
}

// LoadListenAddr pour la sous-commande healthcheck : aucune lecture de fichier de secret.
func LoadListenAddr(getenv Getenv, key, def string) (string, error) {
	return "", errNotImplemented
}

// LoadHealthFile pour la sous-commande healthcheck : aucune lecture de fichier de secret.
func LoadHealthFile(getenv Getenv) (string, error) { return "", errNotImplemented }
