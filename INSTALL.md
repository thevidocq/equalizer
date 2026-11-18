# Installazione

> **Stato:** prototipo in costruzione. I passi marcati *(disponibile)* hanno
> già il codice e la configurazione nel repository; gli altri descrivono
> l'installazione prevista e verranno completati man mano (vedi
> `CHANGELOG.md`). Oggi il gateway serve le pagine in lettura di un legacy
> raggiungibile **direttamente** (`legacy.reach.mode: direct`), senza ancora
> riscriverne l'HTML; con `mode: connector` risponde 502 finché il connettore
> non è pronto. Form e login non sono ancora serviti.

Il prototipo gira su **una VPS** con Docker Compose. Il legacy da proteggere
può stare ovunque: lo raggiunge il **connettore**, installato accanto a lui.

## 1. Requisiti

### VPS

- 4 vCPU, 16 GB di RAM, 160 GB SSD.
- Ubuntu 24.04 LTS.
- Un nome DNS per il gateway (es. `gw.example.com`) e uno per il relay
  (es. `relay.example.com`), entrambi verso l'IP della VPS.

### Macchina del connettore

- Linux con Docker, nella stessa rete del legacy.
- Uscita verso Internet sulla porta **8443** (verso il relay).
- Nessuna porta in ingresso necessaria.

## 2. Preparare la VPS

```bash
# Docker e plugin Compose
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER   # poi esci e rientra

# Firewall: solo gateway (80/443), relay (8443), SSH e VPN di amministrazione
sudo ufw default deny incoming
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw allow 8443/tcp
sudo ufw allow 51820/udp        # WireGuard per accedere al pannello
sudo ufw enable
```

Il pannello **non** è esposto su Internet: si raggiunge tramite VPN WireGuard
o tunnel SSH (`ssh -L 8080:localhost:8080 utente@vps`).

## 3. Scaricare il progetto *(disponibile)*

```bash
git clone https://github.com/thevidocq/equalizer.git
cd equalizer
```

Verifica dei contratti *(disponibile)*:

```bash
pip install jsonschema pyyaml openapi-spec-validator
python3 scripts/validate_schemas.py
```

## 4. Configurare *(disponibile)*

```bash
cd deploy/compose
cp .env.example .env
nano .env        # dominio, email, password di Grafana, EQ_UID/EQ_GID (id -u, id -g)
```

Valori principali in `.env`:

| Variabile | Esempio | Significato |
| --- | --- | --- |
| `EQ_GATEWAY_DOMAIN` | `gw.example.com` | Dominio pubblico del gateway (certificato automatico) |
| `EQ_ACME_EMAIL` | `admin@example.com` | Email per Let's Encrypt |
| `EQ_GATEWAY_REPLICAS` | `2` | Repliche del gateway |
| `EQ_LOG_LEVEL` | `info` | Livello dei log (`debug`, `info`, `warn`, `error`) |
| `EQ_MAP_KEY_ID` | `k1` | Identificativo della chiave di firma delle mappe |
| `EQ_GRAFANA_PASSWORD` | *(scegline una)* | Password dell'utente `admin` di Grafana |
| `EQ_UID` / `EQ_GID` | `1000` | Utente della VPS che possiede i file creati da `eqtool` |

## 5. Chiave di firma e prima mappa *(disponibile)*

Finché il pannello non esiste, le mappe si firmano a mano con `eqtool`.

```bash
# dalla cartella deploy/compose
docker compose --profile tools run --rm eqtool keygen -out secrets/map-signing.key

# converte e valida una mappa YAML (dalla radice del repository)
cd ../..
pip install jsonschema pyyaml
python3 scripts/map_to_json.py schema/map/examples/portale-pratiche.yaml > deploy/compose/maps/mappa.json

# firma la mappa: il gateway legge maps/current.json
cd deploy/compose
docker compose --profile tools run --rm eqtool sign \
  -key secrets/map-signing.key -map maps/mappa.json -key-id k1 -out maps/current.json
```

Il gateway ricarica la mappa da solo quando `maps/current.json` cambia (entro 15 secondi).

## 6. Avviare *(disponibile)*

```bash
docker compose up -d --build
docker compose ps            # gateway "healthy", due repliche
```

Verifiche rapide:

```bash
curl -I https://gw.example.com/            # rotta mappata: pagina del legacy (mode: direct) o 502 (mode: connector)
curl -I https://gw.example.com/wp-admin    # non mappata: 404
```

### Vedere i log *(disponibile)*

Grafana ascolta solo su localhost della VPS. Dal tuo computer:

```bash
ssh -L 3000:localhost:3000 utente@vps
```

Apri `http://localhost:3000` (utente `admin`, password di `.env`), vai in
**Explore**, sorgente **Loki**, e usa query come:

```text
{service="gateway"}                                  tutti i log del gateway
{service="gateway", stream="security"}               solo eventi di sicurezza
{service="gateway"} |= "r-4ffd4c18cbf386ec"           una richiesta precisa
{service="gateway", level="error"}                    solo errori
```

La pagina Log del pannello arriverà con panel-ui; Grafana resta per le analisi approfondite.

## 7. Installare il connettore accanto al legacy

1. Nel pannello: **Siti protetti → Proteggi un sito**, inserisci dominio
   pubblico e indirizzo interno del legacy.
2. Nella scheda **Connettori** genera un token di registrazione (monouso,
   valido 24 ore).
3. Sulla macchina del connettore:

```bash
docker run -d --name eq-connector --restart unless-stopped \
  -e RELAY_URL=relay.example.com:8443 \
  -e ENROLL_TOKEN=<token dal pannello> \
  equalizer/connector:latest
```

4. Nel pannello il connettore risulta **online** entro pochi secondi.

## 8. Verifiche

| Controllo | Come |
| --- | --- |
| Servizi attivi | Pannello → **Stato dei servizi**, tutto "operativo" |
| Connettore | Pannello → sito → **Connettori**: online, latenza mostrata |
| Percorso completo | Controllo sintetico end-to-end verde |
| Log | Pannello → **Log**, filtro per servizio |
| Esposizione | Dopo il passaggio, il legacy non risponde da Internet |

## 9. Aggiornare

```bash
git pull
cd deploy/compose
docker compose pull
docker compose up -d --build
```

## 10. Disinstallare

```bash
cd deploy/compose
docker compose down          # aggiungi -v per cancellare anche i dati
docker rm -f eq-connector    # sulla macchina del connettore
```
