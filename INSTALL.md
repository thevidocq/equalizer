# Installazione

> **Stato:** i servizi non sono ancora implementati. Questa guida descrive
> l'installazione prevista del prototipo e verrà verificata e aggiornata
> man mano che i servizi arrivano (vedi `CHANGELOG.md`). I passi marcati
> *(disponibile)* funzionano già oggi.

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

## 4. Configurare

```bash
cd deploy/compose
cp .env.example .env
```

Valori principali in `.env`:

| Variabile | Esempio | Significato |
| --- | --- | --- |
| `EQ_GATEWAY_DOMAIN` | `gw.example.com` | Dominio pubblico del gateway |
| `EQ_RELAY_DOMAIN` | `relay.example.com` | Dominio del relay per i connettori |
| `EQ_ACME_EMAIL` | `admin@example.com` | Email per i certificati Let's Encrypt |
| `EQ_ADMIN_EMAIL` | `admin@example.com` | Primo amministratore della piattaforma |
| `EQ_POSTGRES_PASSWORD` | *(generata)* | Password del database |
| `EQ_SIGNING_KEY_PATH` | `./secrets/map-signing.key` | Chiave Ed25519 per firmare le mappe |
| `ANTHROPIC_API_KEY` | *(opzionale)* | Chiave per Claude; si può inserire anche dal pannello |

Generare la chiave di firma delle mappe:

```bash
mkdir -p secrets
docker run --rm -v "$PWD/secrets:/out" equalizer/tools keygen --out /out/map-signing.key
```

## 5. Avviare

```bash
docker compose up -d                       # piattaforma
docker compose --profile lab up -d         # laboratorio locale (facoltativo)
docker compose ps                          # tutti i servizi "healthy"
```

Primo accesso al pannello (via VPN o tunnel SSH): `http://localhost:8080`.
L'amministratore indicato in `EQ_ADMIN_EMAIL` riceve il link di attivazione
e configura l'MFA.

## 6. Installare il connettore accanto al legacy

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

## 7. Verifiche

| Controllo | Come |
| --- | --- |
| Servizi attivi | Pannello → **Stato dei servizi**, tutto "operativo" |
| Connettore | Pannello → sito → **Connettori**: online, latenza mostrata |
| Percorso completo | Controllo sintetico end-to-end verde |
| Log | Pannello → **Log**, filtro per servizio |
| Esposizione | Dopo il passaggio, il legacy non risponde da Internet |

## 8. Aggiornare

```bash
git pull
cd deploy/compose
docker compose pull
docker compose up -d
```

## 9. Disinstallare

```bash
cd deploy/compose
docker compose down          # aggiungi -v per cancellare anche i dati
docker rm -f eq-connector    # sulla macchina del connettore
```
