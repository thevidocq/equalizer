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
| `roles` | Ruolo dei visitatori non autenticati (`anonymous`), autenticazione sul gateway, account legacy per ruolo |
| `defaults` | Rate limit, budget verso il legacy, cache, vincoli stringa, template di errore |
| `types` | Tipi riutilizzabili (`id_pratica`, `codice_fiscale`, ...) |
| `routes` | Pagine in lettura (`GET`): richiesta al legacy, `render`, link, permessi |
| `forms` | Scritture: campi tipizzati, richiesta ricostruita, esito, limiti |
| `flows` | Sequenze di form con stato lato server |
| `api` | Endpoint JSON per le parti dinamiche |
| `compat` | URL storici serviti o reindirizzati |
| `callbacks` | Chiamate in ingresso da terzi, con verifica |
| `assets` | File statici serviti per prefisso (CSS, immagini, script, font) con i tipi ammessi |
| `external_links` | Domini esterni ammessi |
| `behavior` | Soglie comportamentali |

## Resa

- `render: faithful` (predefinita): HTML del legacy trasformato, aspetto
  identico.
- `render: template`: vista nuova del gateway; richiede `template` ed
  `extract`.

## Link e asset nelle pagine

La resa fedele riscrive ogni URL interno trovato in una pagina (`href`,
`src`, `action`, `srcset`, `<base>`, ...):

1. se corrisponde a un `links` di una rotta o a una voce `compat`, diventa la
   rotta pubblica;
2. altrimenti, se corrisponde al `legacy_request` di una rotta GET (stesso
   percorso, stesse chiavi di query), diventa quella rotta: spesso non serve
   dichiarare `links`;
3. se è sotto un prefisso `assets`, resta lo stesso percorso;
4. altrimenti diventa un percorso relativo, che il gateway rifiuta (404) e
   conta come URL non mappato.

L'host del legacy non compare mai nella pagina. Gli URL esterni restano
invariati. Gli asset vengono serviti solo se il `Content-Type` del legacy è
tra i `types` del prefisso; le immagini raster vengono controllate anche sul
contenuto reale. La query degli asset non viene mai inviata al legacy.

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

## Regole per i campi di testo

| Regola | Rifiuta |
| --- | --- |
| `no_html` | `<`, `>`, entità numeriche `&#` |
| `no_sql_meta` | `--`, `/*`, `*/`, `;`, `\`, `"`, `` ` ``, NUL; l'apostrofo è ammesso solo tra due lettere (`D'Angelo`) |
| `no_template_meta` | `<?`, `?>`, `<%`, `%>`, `${`, `#{`, `{{`, `}}` |

Una regola sconosciuta rende la mappa non caricabile. I parametri di query
non dichiarati vengono scartati; quelli ripetuti rifiutati. Senza
`roles.anonymous` nessuna pagina è pubblica.

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

## Firma

Il gateway legge un documento `SignedMap` (vedi `schema/api/panel-api.yaml`):
`payload` è il base64 del JSON compatto della mappa, `signature` la firma
Ed25519 di quei byte. Il gateway accetta solo mappe con firma valida,
`meta.schema` uguale a `gateway-map/1.0`, `meta.status` uguale a `published`
e `version` uguale a `meta.version`. Nel prototipo si firma con
`eqtool sign`; poi lo farà panel-api.

## Note

- In YAML la chiave `on` viene letta come booleano: nella mappa si usa
  `target` (es. `authz.cittadino.target: params.id`).
