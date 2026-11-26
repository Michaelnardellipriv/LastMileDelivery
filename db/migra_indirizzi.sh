#!/bin/sh
# Copia gli indirizzi di ritiro e consegna dal database globale al database delle
# spedizioni di un gruppo di sedi (versione 2.4.0 → 2.5.0, vedi docs/deploy-kubernetes.md).
#
#   sh db/migra_indirizzi.sh <gruppo> [deployment del database del gruppo]
#   sh db/migra_indirizzi.sh 0                      # deploy/db-spedizioni
#   sh db/migra_indirizzi.sh 1 db-spedizioni-1
#   NAMESPACE=lastmile-prod sh db/migra_indirizzi.sh 0
#
# Copia gli indirizzi delle città delle sedi del gruppo, con lo stesso id e la sede della
# loro città, e non tocca quelli già copiati: si può rieseguire. Va eseguito due volte:
# prima di aggiornare i servizi (dopo db/spedizioni/migrazione_indirizzi.sql) e dopo,
# per prendere gli indirizzi creati dai Pod vecchi durante l'aggiornamento; poi
# db/spedizioni/migrazione_indirizzi_vincoli.sql.
set -eu

gruppo="${1:?indicare il gruppo (0, 1, ...)}"
case "$gruppo" in
    ''|*[!0-9]*) echo "il gruppo è un numero" >&2; exit 2 ;;
esac
if [ "$gruppo" = "0" ]; then predefinito=db-spedizioni; else predefinito="db-spedizioni-$gruppo"; fi
destinazione="${2:-$predefinito}"
ns="${NAMESPACE:+-n $NAMESPACE}"

# shellcheck disable=SC2086
kubectl $ns exec deploy/db-globale -- sh -c "psql -v ON_ERROR_STOP=1 -U \"\$POSTGRES_USER\" -d \"\$POSTGRES_DB\" -c \"\copy (
    SELECT i.id, se.id, i.id_citta, i.via, i.civico, i.cod_postale
    FROM indirizzi i JOIN sedi se ON se.id_citta = i.id_citta
    WHERE se.gruppo_spedizioni = $gruppo) TO STDOUT WITH (FORMAT csv)\"" |
kubectl $ns exec -i "deploy/$destinazione" -- sh -c "psql -v ON_ERROR_STOP=1 -U \"\$POSTGRES_USER\" -d \"\$POSTGRES_DB\" \
    -c \"\copy indirizzi_da_importare (id, id_sede, id_citta, via, civico, cod_postale) FROM STDIN WITH (FORMAT csv)\" \
    -c \"SELECT fn_importa_indirizzi() AS indirizzi_copiati\""
