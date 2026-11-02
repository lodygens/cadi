package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func fakeAPI(t *testing.T, refuseOrder bool) *httptest.Server {
	now := time.Now()
	d := func(days int) string { return now.AddDate(0, 0, -days).Format(time.RFC3339) }
	rows := []string{
		fmt.Sprintf(`{"id":1,"date_publication":"%s","marque_produit":"Bonne Maman","modeles_ou_references":"Confiture fraise","distributeurs":"E.Leclerc, Auchan, Intermarché","gtin":"3608580758686 3608580758693","identification_produits":"Lot 1234 DDM 12/2027","motif_rappel":"Présence de verre","lien_vers_la_fiche_rappel":"https://rappel.conso.gouv.fr/fiche-rappel/1"}`, d(1)),
		fmt.Sprintf(`{"id":2,"date_publication":"%s","marque_produit":"Marque Repère","modeles_ou_references":"Jambon","distributeurs":"Magasins de proximité","gtin":3564700000001}`, d(3)),
		fmt.Sprintf(`{"id":3,"date_publication":"%s","marque_produit":"X","distributeurs":"Lidl","gtin":"4056489000000"}`, d(5)),
		fmt.Sprintf(`{"id":4,"date_publication":"%s","marque_produit":"Y","distributeurs":"Carrefour, Super U","gtin":"3270190000000"}`, d(40)),
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if refuseOrder && r.URL.Query().Get("order_by") != "" {
			http.Error(w, `{"error":"unknown field"}`, 400)
			return
		}
		if r.URL.Query().Get("offset") != "0" {
			fmt.Fprint(w, `{"results":[]}`)
			return
		}
		fmt.Fprintf(w, `{"total_count":%d,"results":[%s]}`, len(rows), strings.Join(rows, ","))
	}))
}

func run(t *testing.T, srv *httptest.Server, ens string, marque bool) []Rappel {
	depuis := time.Now().AddDate(0, 0, -14)
	recs, err := fetch(srv.Client(), srv.URL, "x", depuis, 1000)
	if err != nil {
		t.Fatal(err)
	}
	var out []Rappel
	for _, r := range recs {
		if rp, ok := filtrer(r, enseignes[ens], marque, depuis); ok {
			out = append(out, rp)
		}
	}
	return dedoublonner(out)
}

func TestLeclerc(t *testing.T) {
	srv := fakeAPI(t, false)
	defer srv.Close()
	got := run(t, srv, "leclerc", true)
	if len(got) != 2 {
		t.Fatalf("attendu 2 rappels, obtenu %d: %+v", len(got), got)
	}
	if got[0].ID != "1" || len(got[0].GTINs) != 2 || got[0].TrouveDans != "distributeurs" {
		t.Errorf("rappel 1 incorrect: %+v", got[0])
	}
	if got[1].TrouveDans != "marque" || got[1].GTINs[0] != "3564700000001" {
		t.Errorf("rappel 2 incorrect: %+v", got[1])
	}
}

func TestFenetreEtMots(t *testing.T) {
	srv := fakeAPI(t, true) // tri refusé -> repli sans tri
	defer srv.Close()
	if got := run(t, srv, "carrefour", true); len(got) != 0 {
		t.Errorf("rappel hors fenêtre retenu: %+v", got)
	}
	if got := run(t, srv, "lidl", true); len(got) != 1 {
		t.Errorf("lidl: %+v", got)
	}
	if got := run(t, srv, "intermarche", true); len(got) != 1 {
		t.Errorf("intermarché (accent): %+v", got)
	}
}

func TestGTINs(t *testing.T) {
	g := extraireGTINs("3017620422003; 3017620425035 lot 1234 12345678")
	if len(g) != 3 {
		t.Errorf("%v", g)
	}
}
