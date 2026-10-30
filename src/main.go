// CADI : détecte les rappels de produits publiés sur RappelConso (API open data
// de data.economie.gouv.fr) pour une enseigne donnée (Leclerc, Auchan, Carrefour…).
//
// Exemples :
//
//	go run . -enseigne leclerc
//	go run . -enseigne auchan -days 30 -all
//	go run . -enseigne carrefour -gtins mes_produits.txt -json
//	go run . -list
//
// Par défaut, seuls les rappels jamais vus sont affichés : les identifiants déjà vus
// sont mémorisés dans ./state/<enseigne>.json. Lancé régulièrement (cron), le programme
// ne signale donc que les nouveautés.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Enseignes : termes recherchés dans le champ « distributeurs » (et la marque).
// Les termes sont comparés après normalisation (minuscules, sans accents).
// ---------------------------------------------------------------------------

type Enseigne struct {
	Nom    string
	Termes []string // sous-chaînes cherchées
	Mots   []string // mots entiers (pour les noms courts ambigus)
}

var enseignes = map[string]Enseigne{
	"leclerc":     {Nom: "E.Leclerc", Termes: []string{"leclerc", "marque repere"}},
	"auchan":      {Nom: "Auchan", Termes: []string{"auchan"}},
	"carrefour":   {Nom: "Carrefour", Termes: []string{"carrefour"}},
	"intermarche": {Nom: "Intermarché", Termes: []string{"intermarche", "les mousquetaires", "netto"}},
	"systemeu":    {Nom: "Système U", Termes: []string{"systeme u", "super u", "hyper u", "u express", "magasins u", "cooperative u"}},
	"lidl":        {Nom: "Lidl", Mots: []string{"lidl"}},
	"aldi":        {Nom: "Aldi", Mots: []string{"aldi"}},
	"casino":      {Nom: "Casino", Termes: []string{"geant casino", "groupe casino", "supermarches casino", "casino supermarches"}, Mots: []string{"casino"}},
	"monoprix":    {Nom: "Monoprix", Termes: []string{"monoprix"}},
	"franprix":    {Nom: "Franprix", Termes: []string{"franprix"}},
	"cora":        {Nom: "Cora", Mots: []string{"cora"}},
	"picard":      {Nom: "Picard", Mots: []string{"picard"}},
	"grandfrais":  {Nom: "Grand Frais", Termes: []string{"grand frais"}},
	"biocoop":     {Nom: "Biocoop", Termes: []string{"biocoop"}},
}

// ---------------------------------------------------------------------------
// Noms de champs possibles : le schéma de RappelConso a changé plusieurs fois
// (V1 → V2), on accepte donc plusieurs variantes pour chaque information.
// ---------------------------------------------------------------------------

var (
	champsID           = []string{"id", "rappel_guid", "rappelguid", "reference_fiche", "numero_fiche"}
	champsDate         = []string{"date_publication", "date_de_publication"}
	champsDistrib      = []string{"distributeurs"}
	champsMarque       = []string{"marque_produit", "nom_de_la_marque_du_produit"}
	champsModele       = []string{"modeles_ou_references", "noms_des_modeles_ou_references", "libelle"}
	champsTitre        = []string{"libelle", "titre"}
	champsCategorie    = []string{"sous_categorie_produit", "sous_categorie_de_produit", "categorie_produit", "categorie_de_produit"}
	champsMotif        = []string{"motif_rappel", "motif_du_rappel"}
	champsRisques      = []string{"risques_encourus", "risques_encourus_par_le_consommateur"}
	champsLots         = []string{"identification_produits", "identification_des_produits"}
	champsGTIN         = []string{"gtin", "code_barre", "codes_barres"}
	champsLien         = []string{"lien_vers_la_fiche_rappel", "lien_vers_la_fiche_rappelconso"}
	champsConduite     = []string{"conduites_a_tenir_par_le_consommateur"}
	champsZone         = []string{"zone_geographique_de_vente"}
	champsFinProcedure = []string{"date_de_fin_de_la_procedure_de_rappel"}
)

const (
	defaultBase    = "https://data.economie.gouv.fr/api/explore/v2.1"
	defaultDataset = "rappelconso-v2-gtin-espaces"
	pageSize       = 100
)

type Record map[string]any

// Rappel est la vue normalisée d'un enregistrement, utilisée pour l'affichage.
type Rappel struct {
	ID            string    `json:"id"`
	Date          time.Time `json:"date_publication"`
	Titre         string    `json:"titre,omitempty"`
	Marque        string    `json:"marque,omitempty"`
	Modele        string    `json:"modele,omitempty"`
	Categorie     string    `json:"categorie,omitempty"`
	GTINs         []string  `json:"gtins,omitempty"`
	Lots          string    `json:"lots,omitempty"`
	Motif         string    `json:"motif,omitempty"`
	Risques       string    `json:"risques,omitempty"`
	Conduite      string    `json:"conduite_a_tenir,omitempty"`
	Distributeurs string    `json:"distributeurs,omitempty"`
	Zone          string    `json:"zone,omitempty"`
	FinProcedure  string    `json:"fin_procedure,omitempty"`
	Lien          string    `json:"lien,omitempty"`
	TrouveDans    string    `json:"trouve_dans"`          // "distributeurs" ou "marque"
	GTINSuivi     bool      `json:"gtin_suivi,omitempty"` // true si un GTIN correspond à la liste -gtins
}

func main() {
	var (
		ensFlag  = flag.String("enseigne", "", "enseigne à surveiller (voir -list)")
		list     = flag.Bool("list", false, "affiche les enseignes connues")
		days     = flag.Int("days", 14, "fenêtre de recherche en jours")
		all      = flag.Bool("all", false, "affiche tous les rappels de la fenêtre, même déjà vus")
		stateDir = flag.String("state", "state", "dossier où mémoriser les rappels déjà vus")
		gtinFile = flag.String("gtins", "", "fichier de GTIN suivis (un par ligne) : ne garde que ces produits")
		asJSON   = flag.Bool("json", false, "sortie JSON")
		marque   = flag.Bool("marque", true, "chercher aussi l'enseigne dans le champ marque (marques de distributeur)")
		base     = flag.String("base", defaultBase, "URL de base de l'API Explore v2.1")
		dataset  = flag.String("dataset", defaultDataset, "identifiant du jeu de données")
		maxRows  = flag.Int("max", 5000, "nombre maximum d'enregistrements lus")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage : rappelconso -enseigne <nom> [options]\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *list {
		keys := make([]string, 0, len(enseignes))
		for k := range enseignes {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("%-12s %s\n", k, enseignes[k].Nom)
		}
		return
	}

	key := normalise(*ensFlag)
	key = strings.NewReplacer(" ", "", ".", "", "-", "").Replace(key)
	ens, ok := enseignes[key]
	if !ok {
		fmt.Fprintf(os.Stderr, "enseigne inconnue %q (voir -list)\n", *ensFlag)
		os.Exit(2)
	}

	var suivis map[string]bool
	if *gtinFile != "" {
		var err error
		if suivis, err = lireGTINs(*gtinFile); err != nil {
			fatal(err)
		}
	}

	depuis := time.Now().AddDate(0, 0, -*days)
	client := &http.Client{Timeout: 30 * time.Second}

	records, err := fetch(client, *base, *dataset, depuis, *maxRows)
	if err != nil {
		fatal(err)
	}

	var rappels []Rappel
	for _, r := range records {
		rp, ok := filtrer(r, ens, *marque, depuis)
		if !ok {
			continue
		}
		if suivis != nil {
			for _, g := range rp.GTINs {
				if suivis[g] {
					rp.GTINSuivi = true
				}
			}
			if !rp.GTINSuivi {
				continue
			}
		}
		rappels = append(rappels, rp)
	}
	rappels = dedoublonner(rappels)
	sort.Slice(rappels, func(i, j int) bool { return rappels[i].Date.After(rappels[j].Date) })

	// Filtrage « nouveaux uniquement » via le fichier d'état.
	statePath := filepath.Join(*stateDir, key+".json")
	vus, err := lireEtat(statePath)
	if err != nil {
		fatal(err)
	}
	var aAfficher []Rappel
	for _, rp := range rappels {
		if *all || !vus[rp.ID] {
			aAfficher = append(aAfficher, rp)
		}
		vus[rp.ID] = true
	}
	if err := ecrireEtat(statePath, vus); err != nil {
		fmt.Fprintf(os.Stderr, "attention : état non sauvegardé : %v\n", err)
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if aAfficher == nil {
			aAfficher = []Rappel{}
		}
		_ = enc.Encode(aAfficher)
		return
	}
	afficher(os.Stdout, ens, aAfficher, *days, *all)
}

// ---------------------------------------------------------------------------
// Récupération paginée
// ---------------------------------------------------------------------------

func fetch(client *http.Client, base, dataset string, depuis time.Time, maxRows int) ([]Record, error) {
	orderBy := champsDate[0] + " DESC"
	var out []Record
	for offset := 0; offset < maxRows; offset += pageSize {
		page, status, err := fetchPage(client, base, dataset, orderBy, offset)
		if err != nil && status == http.StatusBadRequest && orderBy != "" {
			// Le nom du champ de date a peut-être changé : on retente sans tri.
			fmt.Fprintln(os.Stderr, "attention : tri par date refusé par l'API, lecture sans tri")
			orderBy = ""
			offset -= pageSize
			continue
		}
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}
		out = append(out, page...)

		// Résultats triés par date décroissante : on s'arrête dès qu'on sort de la fenêtre.
		if orderBy != "" {
			if d, ok := dateDe(page[len(page)-1]); ok && d.Before(depuis) {
				break
			}
		}
		if len(page) < pageSize {
			break
		}
		// L'API Explore refuse offset+limit > 10000.
		if offset+2*pageSize > 10000 {
			break
		}
	}
	return out, nil
}

func fetchPage(client *http.Client, base, dataset, orderBy string, offset int) ([]Record, int, error) {
	q := url.Values{}
	q.Set("limit", fmt.Sprint(pageSize))
	q.Set("offset", fmt.Sprint(offset))
	if orderBy != "" {
		q.Set("order_by", orderBy)
	}
	u := fmt.Sprintf("%s/catalog/datasets/%s/records?%s", strings.TrimRight(base, "/"), url.PathEscape(dataset), q.Encode())

	var lastErr error
	for tentative := 0; tentative < 3; tentative++ {
		if tentative > 0 {
			time.Sleep(time.Duration(tentative*2) * time.Second)
		}
		resp, err := client.Get(u)
		if err != nil {
			lastErr = err
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("API : HTTP %d", resp.StatusCode)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			return nil, resp.StatusCode, fmt.Errorf("API : HTTP %d : %s", resp.StatusCode, tronquer(string(body), 300))
		}
		var payload struct {
			Results []Record `json:"results"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, resp.StatusCode, fmt.Errorf("réponse JSON invalide : %w", err)
		}
		return payload.Results, resp.StatusCode, nil
	}
	return nil, 0, lastErr
}

// ---------------------------------------------------------------------------
// Filtrage et normalisation
// ---------------------------------------------------------------------------

func filtrer(r Record, ens Enseigne, avecMarque bool, depuis time.Time) (Rappel, bool) {
	d, okDate := dateDe(r)
	if okDate && d.Before(depuis) {
		return Rappel{}, false
	}
	distrib := champ(r, champsDistrib)
	marque := champ(r, champsMarque)

	trouve := ""
	switch {
	case correspond(distrib, ens):
		trouve = "distributeurs"
	case avecMarque && correspond(marque, ens):
		trouve = "marque"
	default:
		return Rappel{}, false
	}

	id := champ(r, champsID)
	if id == "" {
		id = champ(r, champsLien)
	}
	if id == "" {
		id = champ(r, champsDate) + "|" + marque + "|" + champ(r, champsModele)
	}
	return Rappel{
		ID:            id,
		Date:          d,
		Titre:         champ(r, champsTitre),
		Marque:        marque,
		Modele:        champ(r, champsModele),
		Categorie:     champ(r, champsCategorie),
		GTINs:         extraireGTINs(champ(r, champsGTIN)),
		Lots:          champ(r, champsLots),
		Motif:         champ(r, champsMotif),
		Risques:       champ(r, champsRisques),
		Conduite:      champ(r, champsConduite),
		Distributeurs: distrib,
		Zone:          champ(r, champsZone),
		FinProcedure:  champ(r, champsFinProcedure),
		Lien:          champ(r, champsLien),
		TrouveDans:    trouve,
	}, true
}

func correspond(texte string, ens Enseigne) bool {
	if texte == "" {
		return false
	}
	n := normalise(texte)
	for _, t := range ens.Termes {
		if strings.Contains(n, t) {
			return true
		}
	}
	if len(ens.Mots) > 0 {
		for _, mot := range regexp.MustCompile(`[a-z0-9]+`).FindAllString(n, -1) {
			for _, m := range ens.Mots {
				if mot == m {
					return true
				}
			}
		}
	}
	return false
}

var accents = strings.NewReplacer(
	"à", "a", "â", "a", "ä", "a", "á", "a", "ã", "a",
	"ç", "c",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"î", "i", "ï", "i", "í", "i",
	"ô", "o", "ö", "o", "ó", "o",
	"ù", "u", "û", "u", "ü", "u", "ú", "u",
	"ÿ", "y", "œ", "oe", "æ", "ae",
	"’", "'", " ", " ",
)

func normalise(s string) string {
	s = accents.Replace(strings.ToLower(s))
	return strings.Join(strings.Fields(s), " ")
}

// champ renvoie la première valeur non vide parmi les noms candidats, convertie en texte.
func champ(r Record, noms []string) string {
	for _, n := range noms {
		v, ok := r[n]
		if !ok || v == nil {
			continue
		}
		if s := texte(v); s != "" {
			return s
		}
	}
	return ""
}

func texte(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case float64:
		// Les GTIN numériques arrivent en float64 : on évite la notation scientifique.
		return fmt.Sprintf("%.0f", x)
	case []any:
		parts := make([]string, 0, len(x))
		for _, e := range x {
			if s := texte(e); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, " | ")
	case bool:
		return fmt.Sprint(x)
	default:
		b, _ := json.Marshal(x)
		return string(b)
	}
}

func dateDe(r Record) (time.Time, bool) {
	s := champ(r, champsDate)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02", "02/01/2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

var reChiffres = regexp.MustCompile(`\d+`)

// extraireGTINs récupère les codes de 8 à 14 chiffres, quel que soit le séparateur
// (espace, virgule, point-virgule…).
func extraireGTINs(s string) []string {
	var out []string
	vus := map[string]bool{}
	for _, g := range reChiffres.FindAllString(s, -1) {
		if len(g) >= 8 && len(g) <= 14 && !vus[g] {
			vus[g] = true
			out = append(out, g)
		}
	}
	return out
}

// dedoublonner fusionne les lignes d'une même fiche (le jeu V2 a une ligne par produit/GTIN).
func dedoublonner(in []Rappel) []Rappel {
	idx := map[string]int{}
	var out []Rappel
	for _, r := range in {
		if i, ok := idx[r.ID]; ok {
			for _, g := range r.GTINs {
				if !contient(out[i].GTINs, g) {
					out[i].GTINs = append(out[i].GTINs, g)
				}
			}
			out[i].GTINSuivi = out[i].GTINSuivi || r.GTINSuivi
			continue
		}
		idx[r.ID] = len(out)
		out = append(out, r)
	}
	return out
}

func contient(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Fichiers : GTIN suivis et état
// ---------------------------------------------------------------------------

func lireGTINs(path string) (map[string]bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, l := range strings.Split(string(b), "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		for _, g := range extraireGTINs(l) {
			out[g] = true
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s : aucun GTIN valide trouvé", path)
	}
	return out, nil
}

func lireEtat(path string) (map[string]bool, error) {
	vus := map[string]bool{}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return vus, nil
	}
	if err != nil {
		return nil, err
	}
	var ids []string
	if err := json.Unmarshal(b, &ids); err != nil {
		return nil, fmt.Errorf("%s illisible : %w", path, err)
	}
	for _, id := range ids {
		vus[id] = true
	}
	return vus, nil
}

func ecrireEtat(path string, vus map[string]bool) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	ids := make([]string, 0, len(vus))
	for id := range vus {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	b, _ := json.MarshalIndent(ids, "", "  ")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ---------------------------------------------------------------------------
// Affichage
// ---------------------------------------------------------------------------

func afficher(w io.Writer, ens Enseigne, rappels []Rappel, days int, all bool) {
	quoi := "nouveau(x) rappel(s)"
	if all {
		quoi = "rappel(s)"
	}
	fmt.Fprintf(w, "%s : %d %s sur les %d derniers jours\n", ens.Nom, len(rappels), quoi, days)
	for _, r := range rappels {
		fmt.Fprintln(w, strings.Repeat("─", 72))
		titre := r.Titre
		if titre == "" {
			titre = strings.TrimSpace(r.Marque + " " + r.Modele)
		}
		date := "date inconnue"
		if !r.Date.IsZero() {
			date = r.Date.Format("02/01/2006")
		}
		fmt.Fprintf(w, "%s  %s\n", date, titre)
		ligne(w, "Marque", r.Marque)
		ligne(w, "Catégorie", r.Categorie)
		ligne(w, "GTIN", strings.Join(r.GTINs, ", "))
		ligne(w, "Lots", tronquer(r.Lots, 300))
		ligne(w, "Motif", r.Motif)
		ligne(w, "Risques", r.Risques)
		ligne(w, "Que faire", tronquer(r.Conduite, 200))
		ligne(w, "Distributeurs", tronquer(r.Distributeurs, 200))
		if r.TrouveDans == "marque" {
			ligne(w, "Remarque", "enseigne trouvée dans la marque (marque de distributeur), pas dans la liste des distributeurs")
		}
		ligne(w, "Fiche", r.Lien)
	}
}

func ligne(w io.Writer, label, v string) {
	if v != "" {
		fmt.Fprintf(w, "  %-14s %s\n", label+" :", strings.ReplaceAll(v, "\n", " "))
	}
}

func tronquer(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "erreur :", err)
	os.Exit(1)
}
