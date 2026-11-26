# Immagine di uno dei due database: PARTE=globale (utenti, sedi, indirizzi: servizio
# utenti) oppure PARTE=spedizioni (spedizioni, storico, impostazioni: servizio spedizioni).
#   docker build --build-arg PARTE=globale    -t lastmile/db-globale    db/
#   docker build --build-arg PARTE=spedizioni -t lastmile/db-spedizioni db/
#
# DATI=prova (predefinito, DEV e TEST) carica anche i dati di prova, con utenti dalla
# password nota. Per PROD: DATI=nessuno, solo lo schema (il primo amministratore si crea
# con db/globale/primo_amministratore.sql, vedi docs/deployment.md).
#   docker build --build-arg PARTE=globale --build-arg DATI=nessuno -t lastmile/db-globale:2.1.0-prod db/
FROM postgres:16

ARG PARTE=globale
ARG DATI=prova
COPY ${PARTE}/schema.sql /docker-entrypoint-initdb.d/01-schema.sql
# subito dopo lo schema, prima dei dati: gruppo del database delle spedizioni
# (variabile GRUPPO_SPEDIZIONI, vuota o 0 = gruppo 0; nel database globale resta vuota)
COPY gruppo_spedizioni.sh /docker-entrypoint-initdb.d/01-z-gruppo-spedizioni.sh
COPY ${PARTE}/seed.sql /tmp/02-seed.sql
RUN if [ "$DATI" = "prova" ]; then mv /tmp/02-seed.sql /docker-entrypoint-initdb.d/02-seed.sql; \
    elif [ "$DATI" = "nessuno" ]; then rm /tmp/02-seed.sql; \
    else echo "DATI deve essere prova o nessuno" >&2; exit 1; fi
# dopo schema e dati: crea l'utente normale con cui si collega il backend (Row-Level Security)
COPY ruolo_applicazione.sh /docker-entrypoint-initdb.d/03-ruolo-applicazione.sh
# permessi dell'utente del backend (least privilege): li applica lo script qui sopra;
# fuori da docker-entrypoint-initdb.d, che altrimenti lo eseguirebbe da solo
COPY privilegi_applicazione.sql /usr/local/share/lastmile/privilegi_applicazione.sql
# su Windows il file può avere i fine riga CRLF, che sh non legge
# eseguibili: l'entrypoint di postgres esegue i .sh eseguibili come processi a parte, gli
# altri li legge con "." nella sua shell (un exit lì fermerebbe la creazione del database).
# Su Linux i permessi del contesto di build possono essere 644, quindi si impostano qui.
RUN sed -i 's/\r$//' /docker-entrypoint-initdb.d/03-ruolo-applicazione.sh /docker-entrypoint-initdb.d/01-z-gruppo-spedizioni.sh \
 && chmod 0755 /docker-entrypoint-initdb.d/03-ruolo-applicazione.sh /docker-entrypoint-initdb.d/01-z-gruppo-spedizioni.sh

EXPOSE 5432

HEALTHCHECK --interval=5s --timeout=3s --retries=10 \
  CMD pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB" || exit 1
