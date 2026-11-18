# Formato della mappa

La mappa è l'unica fonte di verità del gateway: **ciò che non è dichiarato
non esiste**. Schema: [`schema/map/gateway-map.schema.json`](../schema/map/gateway-map.schema.json).
Esempio completo: [`schema/map/examples/portale-pratiche.yaml`](../schema/map/examples/portale-pratiche.yaml).

## Principi

- **Deny by default**: rotte, parametri, form e campi non dichiarati → 404 o
  scartati; il legacy non viene contattato.
- **Ricostruzione**: ogni richiesta al legacy è generata da `legacy_request`.
- **Tipi stretti**: ogni valore in ingresso ha tipo e vincoli. `text` e `file`
  sono ammessi solo con `rules` o `scan`.
- **Autorizzazione nel gateway** (`roles`, `authz`, `scope`).
- **Provenienza**: `origin` (agent/human), `confidence`, `risk`.
- **Versioni immutabili**, firmate alla pubblicazione.
- **Segreti fuori dalla mappa**: solo riferimenti `vault://`.

## Sezioni

| Sezione | Contenuto |
| --- | --- |
| `meta` | Schema, tenant, app, versione, stato, autore, approvatore |
| `legacy` | `base_url`, `reach` (connettore o diretto, destinazioni ammesse), login, sessione scaduta, rilevamento errori |
| `roles` | Autenticazione sul gateway e account legacy per ruolo |
| `defaults` | Rate limit, budget verso il legacy, cache, vincoli stringa, template di errore |
| `types` | Tipi riutilizzabili (`id_pratica`, `codice_fiscale`, ...) |
| `routes` | Pagine in lettura (`GET`): richiesta al legacy, `render`, link, permessi |
| `forms` | Scritture: campi tipizzati, richiesta ricostruita, esito, limiti |
| `flows` | Sequenze di form con stato lato server |
| `api` | Endpoint JSON per le parti dinamiche |
| `compat` | URL storici serviti o reindirizzati |
| `callbacks` | Chiamate in ingresso da terzi, con verifica |
| `external_links` | Domini esterni ammessi |
| `behavior` | Soglie comportamentali |

## Resa

- `render: faithful` (predefinita): HTML del legacy trasformato, aspetto
  identico.
- `render: template`: vista nuova del gateway; richiede `template` ed
  `extract`.

## Tipi di campo

| Tipo | Vincoli | Rischio predefinito |
| --- | --- | --- |
| `int` | `min`, `max` | basso |
| `enum` | `values` | basso |
| `bool`, `date` | formato fisso | basso |
| `string` | `pattern` o `charset`, `max_length` | medio |
| `email` | formato, `max_length` | medio |
| `text` | `charset`, `max_length`, `rules` (obbligatorie) | alto |
| `file` | `mime`, `max_size_kb`, `scan` (obbligatorio) | alto |

## Ciclo di vita

`draft` → `in_review` → `approved` → `published` → `retired`.

| Classe di modifica | Esempi | Approvazione |
| --- | --- | --- |
| Allarga la superficie | Nuova rotta/form, nuovo ruolo, campo meno restrittivo, nuovo `text`/`file` | Sempre, da una persona diversa dall'autore |
| Restringe la superficie | Rotta rimossa, vincolo più stretto | Automatica in modalità automatica |
| Solo resa | Selettori, testi | Automatica in modalità automatica |
| Comportamento e limiti | Soglie, rate limit, budget | Automatica se più restrittiva |

La classe è calcolata dal sistema sul diff, non dichiarata dall'agent. Una
mappa importata entra come proposta e segue le stesse regole.

## Note

- In YAML la chiave `on` viene letta come booleano: nella mappa si usa
  `target` (es. `authz.cittadino.target: params.id`).
