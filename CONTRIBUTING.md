# CONTRIBUTING

| | |
|---|---|
| **Progetto** | *Last Mile Delivery* |
| **Team** | *GLS / Francesco Platenotti, Dennis Pasquali, Gianluca Vivaldi, Micheal Nandelli, Mohammad Amine Reddad, Pellegrini Christian* |
| **Data** | 30/09/2026* |
| **Versione** | *v0.1 — riscrivetela quando le regole cambiano davvero* |

Queste regole valgono per **tutti** i membri del team, comprese le persone
che le hanno scritte. Se una regola non viene rispettata da nessuno, cambiatela
qui — non ignoratela in silenzio.

## Branch strategy

*Definite i prefissi che usate e cosa contiene `main`. Esempio già visto a
lezione, adattatelo se il team ne vuole un altro — basta scriverlo qui.*

```
main                 sempre funzionante, nessun commit diretto
dev                  test delle funzionalità prima di metterle nel main
feature/<cosa>       nuova funzionalità
fix/<cosa>           correzione di un bug
docs/<cosa>          solo documentazione
```

> Esempio: `feature/filtro-interventi`, `fix/calcolo-priorita`, `docs/api-storage`.

*Un branch vive il tempo di un'attività: nasce da `dev`, si sviluppa, si
integra con una PR/MR, e si chiude. Non deve durare settimane.*

## Convenzioni di commit

*Formato del messaggio: prefisso + descrizione al presente, che dice il
"cosa", non il "come" (il come si legge nel diff).*

```
feat: <cosa aggiunge>
fix: <cosa corregge>
docs: <cosa documenta>
```

> Esempio: `feat: aggiungi filtro interventi per tecnico`

*Evitate messaggi come "wip", "fix", "cose varie": chi legge la storia in
futuro — anche voi, tra un mese — deve capire perché quel commit esiste.*

## Pull/merge request e review

*Rispondete a queste tre domande, in modo verificabile:*

- Chi può fare merge su `main`? Lo si fa insieme in team.
- Quante approvazioni servono prima del merge? Da tutto il team.
- Cosa NON è accettabile in una review? Approvare senza aver letto il
  diff, bloccare per una preferenza di stile senza motivo, approvarla senza dirlo a nessuno

## Gestione dei conflitti

*Regola minima: cosa si fa se un conflitto non si risolve in fretta.*

> se un conflitto non si risolve in 10 minuti, si chiama un altro
> membro del team prima di forzare una scelta da soli.

## Definition of Done

*Riprendete la Definition of Done concordata alla lezione 1 (se già scritta):
non è decorativa, è il criterio con cui si accetta o si rifiuta una PR.*

> Esempio: "una issue è fatta quando: il codice è mergiato su `main` · esiste
> un modo di verificarla (test o passi manuali) · la issue collegata è
> aggiornata a 'fatto' · nessun segreto o dato finto è rimasto nel codice.

-Requisiti e Funzionalità

Tutti i criteri di accettazione specifici ("Fatto quando") della voce di backlog sono soddisfatti e verificati.

-Qualità del Codice e Architettura

Il codice è stato revisionato (Code Review) da almeno un altro membro del team ed è integrato nel ramo principale del repository.

I vincoli di dominio sono rispettati: i passaggi di stato non validi nel workflow della spedizione vengono bloccati dal backend.   

-Sicurezza e Accessi (RBAC)

I ruoli e i permessi sono applicati: il cliente destinatario vede solo la propria spedizione (senza note interne), mentre il corriere agisce solo sulle spedizioni a lui assegnate.   

-Testing e Integrità

Le API sono documentate e testate (es. test d'integrazione o Postman/Automated tests) per verificare il successo e la gestione degli errori (HTTP 401, 403, ecc.).   

La persistenza dei dati è verificata: in caso di riavvio del server/container, i dati e lo storico delle spedizioni rimangono inalterati.   

-Documentazione e Rilascio

L'API o l'endpoint è documentato (es. OpenAPI / Swagger).   

Le istruzioni per il deploy e l'avvio locale del componente/servizio sono aggiornate.

## Issue e board

*Dove vivono le issue (piattaforma indicata dal docente) e gli stati che
usate — devono coincidere con quelli mostrati nella board del team.*

```
da fare → in corso → in revisione → fatto
```
