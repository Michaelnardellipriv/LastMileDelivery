# Attori e casi d'uso — \[Last Mile Delivery\] / \[Veloce24\]

|  |  |
| :---- | :---- |
| **Team** | *(GLS, Dennis Pasquali, Francesco Plateroti, Michael Nardelli, Gianluca Vivaldi, Amine Reddad, Christian Pellegrini )* |
| **Cliente** | *(Last mile delivery S.r.l.)* |
| **Data** | 23/09/2026 |
| **Versione** | v1 |

## Attori

*Un attore è un ruolo, non una persona: la stessa persona può essere due attori. Prendeteli dalla sezione "utenti e ruoli" della richiesta cliente.*

| Attore | Cosa ottiene dal sistema |
| :---- | :---- |
| *Cliente mittente* | *crea la spedizione, puo aprire ticket di assistenza* |
| Operatore logistico | assegna spedizioni,risponde ai ticket e organizza spedizioni per pacchi non recapitati |
| Corriere | constrassegna lo stato dei pacchi in consegna per spedizioni dirette in ritiro per spedizioni non dirette e consegnato per le spedizioni avvenute |
| Magazziniere | contrassegna i pacchi in in spedizione |
| Cliente destinatario | monitora stato delle sue spedizioni, apre ticket di assistenza |
| Amministratore | gestisce utenti eliminandoli creandone di nuovi e i loro permessi |

## Casi d'uso

*Formato minimo: attore \+ azione \+ risultato osservabile. Un caso d'uso senza attore è una funzione che nessuno ha chiesto; un attore senza casi d'uso è un ruolo inutile. Numerateli: li richiamerete nel backlog.*

### UC1 — creazione *spedizione*

> *Esempio: il tecnico aggiorna lo stato di un intervento assegnato e registra le note di lavorazione; il segnalatore vede il nuovo stato.*  
> 

- **Attore**: Cliente mittente  
- **Precondizione**: *deve aver inserito i dati di spedizione in maniera corretta*  
- **Risultato osservabile**: *crea una nuova spedizione non assegnata ancora a nessuno*

### UC2 — *assegnazione spedizione*

- **Attore**: Operatore logistico  
- **Precondizione**: ci devono essere spedizioni non ancora assegnate a nessuno  
- **Risultato osservabile**: assegna le spedizioni ai corrieri e questi si ritrovano le spedizioni da fare

### UC3 — *ritiro spedizione* 

- **Attore**:  Corriere  
- **Precondizione**: ha dei ritiri assegnati  
- **Risultato osservabile**: contrassegna le spedizioni come ritirate 

### UC4 — *spedizione in consegna*

- **Attore**:  Magazziniere  
- **Precondizione**: se ha delle spedizioni in magazzino  
- **Risultato osservabile**: contrassegna le spedizioni come in consegna

### UC5 — *consegna spedizione diretta*

- **Attore**:  Corriere  
- **Precondizione**: se ha delle spedizioni dirette   
- **Risultato osservabile**: contrassegna le spedizioni come in consegna

### UC6 — *consegna spedizione*

- **Attore**:  Corriere  
- **Precondizione**: se ha delle spedizioni che ha appena consegnato  
- **Risultato osservabile**: contrassegna le spedizioni come consegnate

### UC7 — *destinatario monitora spedizione*

- **Attore**:  cliente destinatario  
- **Precondizione**: se ha delle spedizioni dirette verso di lui   
- **Risultato osservabile**: visualizza le spedizioni in consegna verso di lui o quelle già   
- consegnate da non più di tot giorni

### 

### UC8 — *reso spedizione non consegnata* 

- **Attore**:  Operatore logistico  
- **Precondizione**: il pacco viene contrassegnato dal corriere come non consegnato e rimane in giacenza per tot giorni  
- **Risultato osservabile**: riorganizza una nuova spedizione per restituire il pacco al mittente

*Aggiungete altri UC quanti servono. Un progetto reale ne avrà molti più di tre: oggi bastano i principali, quelli che spiegano il senso del sistema a chi non lo conosce.*
