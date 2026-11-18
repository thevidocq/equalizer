# Manuale dell'amministratore di piattaforma

Per chi gestisce l'installazione di Equalizer (SaaS o on-prem): tenant,
piani, capacità, salute globale, log, inoltro SIEM e configurazioni.

> **Stato:** il pannello è in sviluppo. Il manuale descrive il funzionamento
> previsto; lo stato reale di ogni funzione è in `CHANGELOG.md`.

## 1. Accesso

- Il pannello si raggiunge solo via VPN o tunnel SSH, mai da Internet.
- Primo accesso con il link di attivazione inviato a `EQ_ADMIN_EMAIL`;
  l'MFA è obbligatoria.
- Ruoli di piattaforma: **amministratore** (tutto) e **operatore** (sola
  lettura su stato, log e metriche).

## 2. Stato dei servizi

**Menu → Stato dei servizi.** È la prima pagina da guardare.

| Area | Cosa controllare |
| --- | --- |
| Servizi | Ogni servizio e replica operativo; versioni allineate |
| Percorso completo | Controllo sintetico end-to-end verde per ogni pool |
| Dati | Ritardo di replica di PostgreSQL, memoria di Redis |
| Connettori | Online sul totale, per tenant |
| Dipendenze esterne | Provider LLM, certificati in scadenza, sonde di esposizione |
| Risorse | CPU, RAM, disco per nodo; repliche reali uguali alle desiderate |

Stati: **operativo**, **degradato**, **non disponibile**, sempre con il
motivo. Gli allarmi arrivano anche se il pannello è giù (Alertmanager).

## 3. Tenant e piani

1. **Menu → Piani e limiti**: definisci i piani (repliche massime, siti
   massimi, conservazione dei log, pool dedicato sì/no).
2. **Menu → Tenant → Nuovo tenant**: nome, piano, email del primo
   amministratore del cliente.
3. Il cliente riceve l'invito e configura tutto il resto dal suo pannello.

## 4. Capacità e pool

**Menu → Capacità della piattaforma.**

- **Pool**: insiemi di gateway condivisi o dedicati a un sito.
- **Regioni** disponibili (in cloud).
- **Limiti massimi** per piano: il cliente sceglie solo dentro questi limiti.
- Su una VPS le repliche crescono fino alle risorse della macchina; per
  andare oltre si aggiunge un nodo (in cloud lo fa il cluster autoscaler).

## 5. Provider LLM predefiniti

**Menu → Impostazioni → LLM.**

- Provider predefinito della piattaforma (prototipo: Claude Haiku).
- Chiavi API in sola scrittura: dopo il salvataggio non vengono mai mostrate.
- **Prova connessione** per verificare latenza ed esito.
- I tenant possono usare il predefinito o configurare il proprio provider.

## 6. Log

**Menu → Log.**

- Ricerca per tempo, servizio, livello, tenant, sito, `request_id`, testo.
- **Live tail** per seguire i log in tempo reale.
- **Export** in JSON o CSV.
- **Livelli a caldo**: per servizio, modulo e sito; il livello `debug` scade
  da solo (predefinito 30 minuti).
- **Traccia richiesta** su un sito: mostra passo per passo cosa ha fatto il
  gateway su ogni richiesta. Da usare per il debug, per tempi brevi.

## 7. Inoltro verso SIEM

**Menu → Inoltro SIEM → Nuova destinazione** (livello piattaforma).

1. **Tipo**: syslog TCP+TLS (consigliato), TCP, UDP; Splunk HEC; Sentinel;
   webhook.
2. **Formato**: JSON, CEF o LEEF.
3. **Flussi**: sicurezza, audit, applicativi (con livello minimo).
4. **Facility** per flusso (predefinite: sicurezza `local4`, audit `local5`,
   applicativi `local6`). La severità si ricava dal livello del log.
5. **Invia messaggio di prova** e verifica che arrivi al SIEM.

I messaggi passano da una coda su disco: se il SIEM non risponde, vengono
inviati quando torna disponibile.

## 8. Import ed export

**Menu → Import ed export.**

- **Export di piattaforma**: piani, pool, regioni, politiche, provider LLM
  predefiniti, elenco dei tenant. Per backup, migrazione tra installazioni,
  staging verso produzione.
- **Import**: carica l'archivio, controlla l'anteprima (diff e conflitti),
  scegli per ogni conflitto *sovrascrivi*, *salta* o *rinomina*, poi applica.
- I segreti non sono mai esportati: all'import il pannello chiede di
  reinserirli.
- Le mappe importate entrano come proposte e richiedono approvazione.

## 9. Audit

**Menu → Audit**: chi ha fatto cosa e quando (approvazioni, pubblicazioni,
cambi di capacità, import/export, modifiche ai livelli di log).

## 10. Procedure ricorrenti

| Situazione | Cosa fare |
| --- | --- |
| Controllo end-to-end rosso | Stato dei servizi → individua il componente; Log filtrati per servizio |
| Connettore offline per un cliente | Verifica dal pannello del tenant; avvisa il cliente (rete o macchina del connettore) |
| Certificato in scadenza | Controlla il rinnovo ACME e il DNS del dominio |
| Allarme di esposizione | Il legacy di un cliente risponde da Internet: avvisa subito il cliente di chiudere il vecchio IP |
| Picco di carico | Capacità → verifica repliche e budget verso il legacy |
| Aggiornamento | `git pull`, `docker compose pull`, `docker compose up -d` (vedi `INSTALL.md`) |
