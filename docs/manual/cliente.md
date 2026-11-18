# Manuale dell'amministratore del cliente

Per chi protegge i propri siti con Equalizer: aggiunta di un sito,
connettore, mappa, collaudo, passaggio, uso quotidiano.

> **Stato:** il pannello è in sviluppo. Il manuale descrive il funzionamento
> previsto; lo stato reale di ogni funzione è in `CHANGELOG.md`.

## 1. Il pannello

- **Sidebar**: Panoramica, elenco dei **siti protetti** con un pallino di
  stato (verde protetto, giallo in collaudo o con proposte, rosso allarme),
  **+ Proteggi un sito**, Impostazioni, Audit.
- **Barra in alto**: notifiche e menu utente.
- **Pagina del sito**: schede Panoramica, Proposte, Mappa, Anteprima,
  Connettori, Capacità, Eventi, Log, Puntamento.

Ruoli del cliente: **amministratore**, **editor** (modifica la mappa),
**approvatore**, **auditor** (sola lettura). Chi modifica non può approvare
la propria modifica.

## 2. Proteggere un sito

Il sito originale continua a funzionare come prima per tutta la procedura:
nessun disservizio.

### Passo 1 — Dati del sito

**+ Proteggi un sito**: dominio pubblico (es. `portale.comune.it`) e
indirizzo interno del legacy (es. `http://10.20.0.15:8080`).

### Passo 2 — Connettore

1. Il pannello genera un token (monouso, 24 ore) e mostra il comando.
2. Su una macchina con Docker nella rete del legacy:

```bash
docker run -d --name eq-connector --restart unless-stopped \
  -e RELAY_URL=<indirizzo del relay> \
  -e ENROLL_TOKEN=<token> \
  equalizer/connector:latest
```

3. Il pannello mostra **Connettore online**. Serve solo l'uscita verso
   Internet sulla porta 8443: nessuna porta in ingresso.

Per la ridondanza puoi installare un secondo connettore con un nuovo token.

### Passo 3 — Crawl

1. Inserisci gli account di test per ogni ruolo (anonimo, utente,
   operatore...). Vengono salvati nel vault.
2. Avvia il crawl. In qualche ora l'agent propone la mappa, ad esempio:
   "42 pagine, 8 form, 3 campi da controllare".

### Passo 4 — Revisione

1. **Proposte**: per ogni proposta vedi il diff, il rischio e la motivazione.
2. **Anteprima**: legacy e sito protetto affiancati, con l'esito del
   confronto visivo.
3. **Editor mappa**: correggi i campi segnalati (es. quali caratteri ammette
   il campo "note").
4. Le modifiche che aprono nuove pagine o allargano i campi richiedono
   l'approvazione di una seconda persona.

### Passo 5 — Collaudo

- **Prova**: il sito protetto risponde su un indirizzo di collaudo
  (es. `nuovo.portale.it`). Navigalo come un utente.
- **Ombra**: il gateway riceve una copia del traffico reale e misura quante
  pagine copre la mappa. Il pannello mostra la percentuale e cosa manca.

### Passo 6 — Passaggio

Scheda **Puntamento**, una checklist guidata:

1. Abbassa il TTL del record DNS almeno 24 ore prima.
2. Imposta il CNAME indicato dal pannello. Con **Cloudflare**: CNAME nella
   dashboard; se il proxy è arancione, SSL in modalità "Full (strict)".
3. Il pannello verifica la propagazione.
4. Qualche giorno in **parallelo**, poi **definitivo**.
5. **Chiudi l'accesso pubblico al legacy** (vecchio IP, porta 443). Le sonde
   lo verificano: finché il legacy risponde da Internet, la protezione non
   vale.

## 3. Uso quotidiano

### Panoramica del sito

| Gruppo | Cosa vedi |
| --- | --- |
| Traffico | Richieste, visitatori unici stimati, banda, pagine più viste |
| Protezione | Richieste bloccate per motivo, campi rifiutati, rate limit, percorsi e IP più bloccati |
| Prestazioni | Latenza (gateway e legacy), cache, errori del legacy |
| Copertura | URL non mappati, URL storici non riconosciuti, confronto visivo |
| Collegamento | Connettori online, latenza del tunnel |
| Agent | Ultimo crawl, proposte in attesa |

### Notifiche tipiche

- "Il legacy ha cambiato la pagina X: proposta pronta" → apri Proposte e
  approva.
- "Connettore offline" → controlla la macchina del connettore e la sua rete.
- "Legacy esposto" → chiudi subito l'accesso pubblico al vecchio IP.

## 4. Configurazioni

### Capacità

Scheda **Capacità**, entro i limiti del piano:

- repliche minime e massime e soglia di autoscaling;
- pool condiviso o dedicato (se il piano lo prevede);
- **cache** delle pagine pubbliche (riduce il carico sul legacy);
- **budget verso il legacy**: richieste al secondo e contemporanee massime.

### Provider LLM

**Impostazioni → LLM**: usa il provider della piattaforma oppure il tuo
(Claude, ChatGPT o modello locale con indirizzo interno). La chiave è in sola
scrittura. Con i provider cloud è attivo il mascheramento dei dati personali.

### Log

Scheda **Log** del sito: ricerca, live tail, export. Vedi solo i log dei tuoi
siti. Per un problema su una pagina attiva la **traccia richiesta** per
qualche minuto e riproduci il problema.

### Inoltro al tuo SIEM

**Impostazioni → Inoltro SIEM**: destinazione (syslog, Splunk HEC, Sentinel,
webhook), formato (JSON, CEF, LEEF), flussi (sicurezza, audit, applicativi),
facility syslog per flusso, messaggio di prova. Ricevi solo i dati dei tuoi
siti.

### Import ed export

**Impostazioni → Import ed export**: esporta il tenant o un singolo sito
(mappe, capacità, impostazioni; segreti esclusi). Un import mostra prima
l'anteprima e le mappe entrano come proposte da approvare.

## 5. Rollback

**Mappa → Versioni**: ogni pubblicazione è una versione. Se una modifica crea
problemi, ripubblica la versione precedente con un clic.
