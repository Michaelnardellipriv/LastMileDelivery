//go:build integration

package apitest

import (
	"net/http"
	"strings"
	"testing"
)

func TestNoteLibere(t *testing.T) {
	f := preparaFixture(t)
	op, c1, c2 := comeOperatore(f), comeCorriere(f.idCorriere1), comeCorriere(f.idCorriere2)

	id := f.nuovaSpedizione(t, "NOTE")
	f.assegna(t, id, f.idCorriere1, 201)
	f.nota(t, id, c1, map[string]any{"nota": "x"}, 409)
	f.stato(t, id, c1, map[string]any{"stato": "ritirata"}, 200)
	righeStorico := f.storico(t, id)

	t.Run("operatore_e_corriere_assegnato_201", func(t *testing.T) {
		r := f.nota(t, id, op, map[string]any{"nota": "  pacco fragile  "}, 201)
		if r.corpo["nota"] != "pacco fragile" || r.corpo["is_interna"] != true || r.corpo["motivo"] != nil {
			t.Errorf("nota interna dell'operatore: %s", r.grezzo)
		}
		if ruolo, idAutore := autoreNota(r.corpo); ruolo != "operatore" || idAutore != f.idOperatore {
			t.Errorf("autore operatore: %s", r.grezzo)
		}
		r = f.nota(t, id, c1, map[string]any{"nota": "citofono rotto"}, 201)
		if ruolo, idAutore := autoreNota(r.corpo); ruolo != "corriere" || idAutore != f.idCorriere1 || r.corpo["is_interna"] != true {
			t.Errorf("nota interna del corriere: %s", r.grezzo)
		}
		if n := f.conta(t, `SELECT count(*) FROM note WHERE id_spedizione = $1 AND is_interna`, id); n != 2 {
			t.Errorf("attese 2 note interne salvate, trovate %d", n)
		}
		if n := f.storico(t, id); n != righeStorico {
			t.Errorf("una nota non cambia la spedizione: storico da %d a %d righe", righeStorico, n)
		}
	})

	t.Run("validazione_400", func(t *testing.T) {
		for _, corpo := range []map[string]any{{}, {"nota": "   "}, {"nota": strings.Repeat("a", 2001)}} {
			if r := f.nota(t, id, op, corpo, 400); !contiene(campiInErrore(r), "nota") {
				t.Errorf("%v: errore atteso sul campo nota: %s", corpo, r.grezzo)
			}
		}
		for _, chi := range []map[string]string{op, c1} {
			f.nota(t, id, chi, map[string]any{"nota": "x", "is_interna": false}, 400)
		}
	})

	t.Run("non_autorizzati_403", func(t *testing.T) {
		for _, chi := range []map[string]string{c2, comeCliente(f.idDestinatario), comeAmministratore(f)} {
			f.nota(t, id, chi, map[string]any{"nota": "x"}, 403)
		}
	})

	t.Run("stati_ammessi_per_il_corriere", func(t *testing.T) {
		consegnata := f.nuovaSpedizione(t, "NOTE-CONS")
		f.consegnaCompleta(t, consegnata)
		f.nota(t, consegnata, c1, map[string]any{"nota": "lasciato al portiere"}, 409)
		f.nota(t, consegnata, op, map[string]any{"nota": "lasciato al portiere"}, 201)

		persa := f.nuovaSpedizione(t, "NOTE-PERSA")
		f.assegna(t, persa, f.idCorriere1, 201)
		f.stato(t, persa, c1, map[string]any{"stato": "ritirata"}, 200)
		f.stato(t, persa, op, map[string]any{"stato": "persa", "motivo": "furto", "nota": "x"}, 200)
		f.nota(t, persa, c1, map[string]any{"nota": "x"}, 409)
		f.nota(t, persa, op, map[string]any{"nota": "denuncia presentata"}, 201)

		creata := f.nuovaSpedizione(t, "NOTE-CREATA")
		f.nota(t, creata, op, map[string]any{"nota": "cliente ha chiamato"}, 201)
	})

	t.Run("note_interne_solo_da_post_note", func(t *testing.T) {
		altra := f.nuovaSpedizione(t, "NOTE-STATO")
		f.assegna(t, altra, f.idCorriere1, 201)
		r := f.stato(t, altra, c1, map[string]any{"stato": "ritirata", "nota": "ritirato"}, 400)
		if !contiene(campiInErrore(r), "nota") {
			t.Errorf("la nota con il cambio di stato è ammessa solo per annullata e persa: %s", r.grezzo)
		}
	})

	t.Run("spedizione_inesistente_e_id_non_valido", func(t *testing.T) {
		f.nota(t, 999999999, op, map[string]any{"nota": "x"}, 404)
		if r := f.chiama(t, http.MethodPost, "/api/spedizioni/abc/note", map[string]any{"nota": "x"}, op); r.stato != http.StatusBadRequest {
			t.Errorf("id non valido: atteso 400, ottenuto %d", r.stato)
		}
	})
}
