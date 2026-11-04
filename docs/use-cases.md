# Attori e casi d'uso — \[Nome progetto\] / \[Cliente\]

|  |  |
| :---- | :---- |
| **Team** | *(GLS, componenti)* |
| **Cliente** | *(Veloce24 S.r.l.)* |
| **Data** | 23/09/2026 |
| **Versione** | v2 |

## Attori

*Un attore è un ruolo, non una persona: la stessa persona può essere due attori. Prendeteli dalla sezione "utenti e ruoli" della richiesta cliente.*

| Attore | Cosa ottiene dal sistema |
| :---- | :---- |
| Operatore logistico | crea e assegna/rassegna le spedizioni le modifica e annulla classifica i problemi delle spedizioni assegnandogli anche delle note e modifica lo stato delle spedizioni, visualizza lo storico e posizione del corriere quando il suo pacco è in consegna  |
| Corriere | contrassegna lo stato dei pacchi in ritirato per quelli appena ritirati, in consegna per spedizioni per spedizioni in consegna e consegnato per le spedizioni appena consegnate e non consegnato per quelle non riuscite |
| Cliente destinatario | monitora stato delle sue spedizioni |
| Amministratore | gestisce utenti disattivandoli o creandone di nuovi e i loro permessi e le configurazioni e visualizza storico |

## Casi d'uso

*Formato minimo: attore \+ azione \+ risultato osservabile. Un caso d'uso senza attore è una funzione che nessuno ha chiesto; un attore senza casi d'uso è un ruolo inutile. Numerateli: li richiamerete nel backlog.*

### UC1 — creazione *spedizione*

> *Esempio: il tecnico aggiorna lo stato di un intervento assegnato e registra le note di lavorazione; il segnalatore vede il nuovo stato.*  
> 

- **Attore**: Operatore logistico  
- **Precondizione**: *deve aver inserito i dati di spedizione in maniera corretta*  
- **Risultato osservabile**: *crea una nuova spedizione non assegnata ancora a nessuno*

### UC2 — *assegnazione spedizione*

- **Attore**: Operatore logistico  
- **Precondizione**: ci devono essere spedizioni non ancora assegnate a nessuno oppure anche spedizioni già assegnate e ritirate  
- **Risultato osservabile**: assegna/riassegna le spedizioni ai corrieri e questi si ritrovano le spedizioni da fare

### UC3 — *ritiro spedizione* 

- **Attore**:  Corriere  
- **Precondizione**: ha dei ritiri assegnati  
- **Risultato osservabile**: contrassegna le spedizioni come ritirate appena le ha ritirate

### UC4 —  *spedizione in consegna*

- **Attore**:  Corriere  
- **Precondizione**: ha delle spedizioni da consegnare contrassegnate come ritirate  
- **Risultato osservabile**: contrassegna le spedizioni come in consegna 

### 

### UC5 — *consegna spedizione*

- **Attore**:  Corriere  
- **Precondizione**: se ha delle spedizioni che ha appena consegnato  
- **Risultato osservabile**: contrassegna le spedizioni come consegnate

### UC6 — *operatore monitora spedizione*

- **Attore**:  operatore   
- **Precondizione**: se ha delle spedizioni in consegna   
- **Risultato osservabile**: visualizza la posizione del corriere nelle spedizioni in consegna 

### 

### UC7 — *reso spedizione non consegnata* 

- **Attore**:  Operatore logistico  
- **Precondizione**: il pacco viene contrassegnato dal corriere come non consegnato e rimane in giacenza per tot giorni  
- **Risultato osservabile**: riorganizza una nuova spedizione per restituire il pacco al mittente 

### UC8 — *corriere riporta indietro la spedizione perche non è arrivato a consegnarla* 

- **Attore**:  Operatore  
- **Precondizione**: il pacco viene contrassegnato dal corriere come in consegna e non riesce a consegnarlo  
- **Risultato osservabile**:   
- Rimette il pacco in ritirato e smette cosi di condividere la propria spedizione

### UC9 — *corriere perde il pacco*

- **Attore**:  Operatore  
- **Precondizione**: il pacco viene perso dal corriere  
  **Risultato osservabile**: l’operatore lo mette in perso cosi che l’operatore logistico possa fare il reso

### UC10 — *spedizione archiviata*

- **Attore**:  piattaforma  
- **Precondizione**: la spedizione si trova in non consegnata persa o annullata o consegnata   
- **Risultato osservabile**:la spedizione viene messa subito nello storico visibile soltanto all’amministratore

### UC11 — gestione utenti

- **Attore**:  Amministratore  
- **Precondizione**: deve avere i provilegi massimi  
  **Risultato osservabile**:gestisce utenti permessi configuarazioni e consulta lo storico senza poter pero modificare nulla

### UC12 — *spedizione impossibile da ritirare*

- **Attore**:  Ritiro non possibile  
- **Precondizione**: il corriere non riesce a ritirare  
- **Risultato osservabile**: la spedizione rimane in non assegnata fino ad un massimo di 3 tentativi poi verrà annullata dall operatore

*Aggiungete altri UC quanti servono. Un progetto reale ne avrà molti più di tre: oggi bastano i principali, quelli che spiegano il senso del sistema a chi non lo conosce.*