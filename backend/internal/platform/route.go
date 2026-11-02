package platform

import "net/http"

// Route : Pattern non vide si le routeur a un gabarit pour la requête ; sinon Status vaut
// http.StatusNotFound ou http.StatusMethodNotAllowed et Allow recopie l'en-tête de net/http.
type Route struct {
	Pattern string
	Status  int
	Allow   string
}

// MatchRoute : mux.Handler(r) ; si le gabarit est vide, exécute le gestionnaire de repli renvoyé
// sur un enregistreur interne (jamais sur w) pour connaître 404 ou 405. Ne lit pas le corps,
// n'appelle aucun gestionnaire de l'application.
func MatchRoute(mux *http.ServeMux, r *http.Request) Route { return Route{} }

// WriteRouteError : 404 introuvable, ou 405 methode_non_autorisee avec Allow ; par WriteError.
func WriteRouteError(w http.ResponseWriter, rt Route) {}
