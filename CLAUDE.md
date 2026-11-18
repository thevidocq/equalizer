# Istruzioni per chi sviluppa Equalizer

Questo file serve a chiunque riprenda lo sviluppo, in particolare a un'altra
sessione o a un altro account Claude. Leggilo per intero prima di toccare il
codice.

## Da dove ripartire

1. Leggi [`CHANGELOG.md`](CHANGELOG.md): l'ultima voce dice **cosa è stato
   fatto, cosa è rimasto a metà e quali sono i prossimi passi**.
2. Leggi [`docs/architecture.md`](docs/architecture.md) e
   [`docs/map-format.md`](docs/map-format.md).
3. Esegui `python3 scripts/validate_schemas.py`: deve passare prima e dopo il
   tuo lavoro.

## Requisiti non negoziabili

1. **Interfaccia identica al 100%** al sito legacy (resa fedele predefinita).
2. **Il legacy non è mai esposto**: lo raggiunge solo il connettore.
3. **Ricostruzione, mai inoltro**: nessun byte del client arriva al legacy così com'è.
4. **Deny by default**: ciò che non è nella mappa non esiste.
5. **Tutto configurabile dal pannello**, compresi LLM, capacità, log e inoltro SIEM.
6. **Modularità**: niente codice monolitico, né backend né frontend.

## Regole di sviluppo

- **Contratti prima del codice.** I confini tra servizi sono in `schema/`.
  Se un servizio ha bisogno di qualcosa di nuovo, si cambia prima il
  contratto, poi il codice. I tipi Go e Python si generano dagli schemi.
- **Un servizio per cartella**, nessun database condiviso tra servizi.
- **Go**: un pacchetto, una responsabilità; dipendenze verso interfacce
  piccole; niente pacchetti `utils`/`common`. Soglia di allarme: 400 righe
  per file.
- **Pipeline di resa a passi**: ogni passo implementa la stessa interfaccia
  `Step`; l'ordine si configura.
- **React**: organizzazione per funzionalità (`features/<nome>/`), pagine che
  compongono componenti, dati solo tramite hook e client generato da
  OpenAPI, `shared/ui` non importa mai da `features/`. Soglia: 200 righe per file.
- **Configurazione da variabili d'ambiente**, servizi senza stato locale.
- **Log JSON** conformi a `schema/events/log.schema.json`, con `request_id`
  propagato. Mai segreti o dati personali nei log.
- **Lingua**: documentazione e messaggi per le persone in italiano;
  identificatori nel codice in inglese.

## Flusso di lavoro Git

- Un branch per attività (`feat/...`, `fix/...`, `docs/...`), poi pull request
  verso `main`.
- **Ogni commit aggiunge una voce a `CHANGELOG.md`** nel formato indicato
  all'inizio di quel file, con: cosa è cambiato, perché, stato e prossimi
  passi. La CI rifiuta le PR che non toccano il changelog.
- Messaggi di commit in italiano, all'imperativo, prima riga sotto i 72
  caratteri.

## Ambiente di sviluppo nelle sessioni Claude

- Il proxy dei moduli Go (`proxy.golang.org`) può essere bloccato. I moduli
  ospitati su GitHub si scaricano con `GOPROXY=direct GOSUMDB=off`.
- I moduli `golang.org/x/...` non si scaricano direttamente: clonare il
  mirror GitHub (es. `github.com/golang/net`) e usare un `go.work` locale con
  `replace`, **senza committarlo**. `go.mod` e `go.sum` restano quelli
  standard, così la build normale (CI, VPS) funziona.
- Docker può non avere il demone: validare il Compose con
  `docker compose config --quiet`; le immagini le costruisce la CI.

## Sicurezza del laboratorio

- Le app vulnerabili del laboratorio non pubblicano porte e stanno su una
  rete Docker `internal`.
- Le campagne di attacco si eseguono **solo** contro sistemi di proprietà del
  progetto o con autorizzazione scritta del proprietario.

## Decisioni già prese

| Tema | Decisione |
| --- | --- |
| Nome | Equalizer (dominio `equalizer.ai` in valutazione) |
| Repository | GitHub `thevidocq/equalizer`, CI con GitHub Actions |
| Linguaggi | Go (gateway, relay, connettore, orchestrator, panel-api), Python (agent), TypeScript + React (pannello) |
| LLM iniziale | Claude Haiku via API; locale e ChatGPT configurabili |
| Componenti UI | Libreria propria in `panel-ui/src/shared/ui`, su Radix UI e Tailwind CSS |
| Prototipo | Una VPS con Docker Compose; driver Kubernetes dopo |
| Laboratorio e server remoti | Preparati dal proprietario del progetto |
