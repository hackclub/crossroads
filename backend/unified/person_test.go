package unified

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/scherepi/crossroads-backend/airtable"
)

func TestFindByPerson(t *testing.T) {
	var mu sync.Mutex
	formulas := map[string]string{} // table → formula
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		table := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		mu.Lock()
		formulas[table] = r.URL.Query().Get("filterByFormula")
		mu.Unlock()
		var records []map[string]any
		switch table {
		case TableID:
			records = []map[string]any{
				{"id": "rec1", "fields": map[string]any{"YSWS–Name": []any{"Athena"}, "Approved At": "2026-03-02", "Hours Spent": 6.5,
					"Code URL": "https://github.com/a/old", "Description": "An   old\nproject"}},
				{"id": "rec2", "fields": map[string]any{"YSWS–Name": []any{"Summer of Making"}, "Approved At": "2026-08-14", "Hours Spent": 20.0,
					"Override Hours Spent": 12.0, "Code URL": "https://github.com/a/new"}},
			}
		case AuthorsTableID:
			records = []map[string]any{{"id": "rec3", "fields": map[string]any{"Unified Trust Level": "YSWS Resident", "Weighted Grants–Total": 3.5}}}
		}
		json.NewEncoder(w).Encode(map[string]any{"records": records})
	}))
	defer srv.Close()
	a := &Airtable{client: airtable.New(airtable.Config{Token: "t", BaseID: BaseID, BaseURL: srv.URL})}

	p, err := a.FindByPerson(context.Background(), " Sana@Example.com ", `U1"x`)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Found || p.TrustLevel != "YSWS Resident" || p.WeightedGrants != 3.5 || len(p.Projects) != 2 {
		t.Fatalf("got %+v", p)
	}
	// Newest first, override hours win, descriptions on one line.
	if p.Projects[0].Program != "Summer of Making" || p.Projects[0].Hours != 12 || p.Projects[1].Description != "An old project" {
		t.Errorf("projects %+v", p.Projects)
	}
	if got := formulas[TableID]; got != `LOWER(TRIM({Email})) = "sana@example.com"` {
		t.Errorf("projects formula %s", got)
	}
	if got := formulas[AuthorsTableID]; got != `OR({Slack ID} = "U1\"x",LOWER(TRIM({Hack Club Auth Email})) = "sana@example.com")` {
		t.Errorf("authors formula %s", got)
	}

	// Nothing to search by: nothing asked.
	formulas = map[string]string{}
	if p, err := a.FindByPerson(context.Background(), "", ""); err != nil || p.Found || len(p.Projects) != 0 || len(formulas) != 0 {
		t.Errorf("empty: %+v %v %v", p, err, formulas)
	}
}
