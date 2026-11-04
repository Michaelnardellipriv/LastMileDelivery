# Backlog iniziale — Lastmile Delibery / Veloce24

|  |  |
| :---- | :---- |
| **Team** | *(GLS, componenti)* |
| **Cliente** | *Veloce24* |
| **Data** | 23/09/2026 |

*8-10 voci, ordinate per valore e rischio: prima le cose che fanno paura, perché sono quelle che possono far cambiare l'architettura. Ogni voce deve avere un "Fatto quando" verificabile — se non sapete come si verifica, la voce non è pronta.*

## Formato

Come \<attore\> voglio \<azione\> per \<beneficio\>

Fatto quando:

\- ...

\- ...

> **Esempio** Come tecnico voglio vedere solo gli interventi assegnati a me, per non perdere tempo tra le segnalazioni altrui. **Fatto quando:** la lista mostra solo i miei interventi · un tecnico non vede quelli di un collega nemmeno chiamando l'API a mano · la lista è ordinata per priorità.

## Voci

### 1\. Come utente voglio registrarmi e accedere per accedere alla mia area per svolgere le mie azioni **Fatto quando:** Ogni utente dell app riesce con le sue credenziali ad autenticarsi alla sua area e non a quelle degli altri 

>   
Rischio: Alto

2\. Come utente voglio fare logout dalla mia area per accedere con un altro account **Fatto quando:** Ogni utente dell’app riesce ad accedere con più di 1 account diverso

Rischio: Basso

3\. Come utente voglio poter modificare la mia password oppure recuperarla se dimenticata **Fatto quando:** Ogni utente dell’app a modificare la propria password e recuperarla  
Rischio: Medio

4\. Come operatore logistico voglio poter creare nuove spedizioni nella mia area **Fatto quando:** l’ operatore logistico riesce ad inserire i dati corretti nel form e l'operatore vede la spedizione appena creata non ancora assegnata a nessuno  
>   
Rischio: Alto 

### 5\. Come utente voglio vedere le spedizioni che devo vedere e il loro stato e poterle filtrare per data numero ecc

**Fatto quando:** l’utente vede solo le sue spedizioni e le può filtrare e ricercare  
Rischio: Alto

6\. Come operatore voglio poter eliminare/modificare una spedizione appena creata che non é stata ritirata **Fatto quando:** l’operatore la può modificare e mettere come disdetta se non già ritirata  
>   
Rischio: Basso

>   
7\. Come operatore logistico voglio poter assegnare le spedizioni ai corrieri che non sono ancora state assegnate a nessuno   
**Fatto quando:** L'operatore logistico può assegnare le spedizioni e i corrieri vedono per quel giorno le spedizioni che l' operatore gli ha assegnato  
>   
Rischio: alto 

8\. Come operatore logistico devo poter riassegnare le spedizioni ai corrieri o riprogrammarle **Fatto quando:** I corrieri vedono i cambiamenti fatti 

Rischio: medio

### 9\. Come corriere voglio poter contrassegnare le spedizioni in ritirate, in consegna, e consegnate oppure non consegnate e mettere anche il numero di tentativi di consegna

**Fatto quando:** Il corriere può modificare gli stati e l'operatore può vederli  
>   
>   
Rischio alto

10\.  Come corriere voglio poter condividere la mia posizione in tempo reale quando la spedizione é in consegna **Fatto quando:** Il cliente destinatario vede la posizione GPS non appena la spedizione diventa in consegna  
>   
Rischio medio

11\. Come operatore logistico voglio poter visualizzare il tracking spedizioni in consegna  
**Fatto quando:** Vedo la posizione delle spedizioni in consegna   
>   
Rischio medio

12\. Come operatore logistico voglio avere i suggerimenti dall’AI per classificare problematiche interne e per fare il riassunto delle note operative **Fatto quando:** l’operatore vede i suggerimenti

Rischio basso

13\. Come operatore logistico voglio poter classificare le spedizioni in perse annullate e riportarle a ritirate **Fatto quando:**  
Il corriere e il cliente destinatario vedono i nuovi stati e nello storico si registrano questi cambi di stato 

14\. Come applicazione devo archiviare lo storico in base allo stato impostato alle spedizioni e la operatore e l’ amministratore devono poterlo vedere e non eliminare **Fatto quando:** lo storico viene impostato automaticamente e la tabella nel db si riempie 

15\. Come operatore e corriere voglio inserire delle note operative interne sulla spedizione e il suo stato **Fatto quando:** anche gli altri operatori vedono le note

Rischio basso

16\. Come amministratore voglio poter disattivare creare nuovi utenti e gestire i permessi e configurazioni **Fatto quando:** Gli utenti che crea l' amministratore effettivamente poi possono essere usati e le sue modifiche hanno effetto su quello che fanno gli utenti  
>   
Rischio alto

17\. Come operatore voglio poter modificare lo stato in perso della spedizione se il corriere non la trova **Fatto quando:** L’operatore logistico vede i pacchi persi e puo fare il reso

Rischio alto

18\. Come piattaforma voglio poter archiviare in automatico le spedizioni in consegnata non consegnata e perse   
 o annullate **Fatto quando:** Nessuno vede più le spedizioni e le vede soltanto l’amministratore nello storico