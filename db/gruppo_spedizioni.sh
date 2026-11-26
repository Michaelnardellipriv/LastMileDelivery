#!/bin/sh
# Gruppo del database delle spedizioni (mappa sede → database, vedi docs/microservizi.md).
#
# Gira una volta sola, alla prima creazione del container (docker-entrypoint-initdb.d),
# subito dopo lo schema e prima dei dati. Con GRUPPO_SPEDIZIONI vuota o 0 non fa niente:
# il database è il gruppo 0, come un'installazione con un solo database delle spedizioni.
# Con GRUPPO_SPEDIZIONI=n gli id di ogni tabella partono da n × 10^12 + 1.
#
# Un gruppo diverso da 0 si crea senza dati di prova (DATI=nessuno): i dati di prova sono
# delle sedi del gruppo 0.
#
# Niente "exit" né "set -e": se il file non fosse eseguibile l'entrypoint di postgres lo
# leggerebbe con "." e un exit fermerebbe la creazione del database (vedi db/Dockerfile).
imposta_gruppo_spedizioni() {
    gruppo_spedizioni="${GRUPPO_SPEDIZIONI:-0}"
    if [ "$gruppo_spedizioni" = "0" ]; then
        return 0
    fi
    # le variabili di psql (:'gruppo') si sostituiscono solo in quello che legge da stdin
    echo "SELECT fn_imposta_gruppo(:'gruppo'::integer);" |
        psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" -v gruppo="$gruppo_spedizioni" ||
        return 1
    echo "database delle spedizioni del gruppo $gruppo_spedizioni"
}

# l'esito della funzione è quello dello script (o fa fallire l'entrypoint, se letto con ".")
imposta_gruppo_spedizioni
