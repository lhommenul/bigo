# Paperless SMTP Gateway

Réception d'emails push vers Paperless-ngx via serveur SMTP dédié, avec auto-configuration DNS/OVH.

## Principe

Tu fais suivre un mail à `edf@docs.ton domaine.fr` → le serveur SMTP reçoit la PJ en direct, l'envoie à Paperless et taggue avec `edf`.

## Déploiement

```yaml
# docker-compose.yml
services:
  smtp-gateway:
    image: ghcr.io/tonuser/paperless-smtp-gateway
    ports:
      - "25:25"
    env_file: .env
    networks:
      - paperless-network

networks:
  paperless-network:
    external: true
```

## Créer les clés API OVH

1. Va sur [https://eu.api.ovh.com/createToken/](https://eu.api.ovh.com/createToken/)
2. Remplis :
   - **Script name** : `paperless-smtp-gateway`
   - **Validity** : `Unlimited`
3. Ajoute ces droits exacts :

| Méthode | Path |
|---|---|
| `GET` | `/domain/zone/*` |
| `POST` | `/domain/zone/*` |
| `DELETE` | `/domain/zone/*` |

4. Clique sur **Create keys**
5. Copie les trois clés (Application Key, Application Secret, Consumer Key) dans le `.env`

> Ça permet au gateway de créer/mettre à jour les enregistrements DNS (A + MX) et de supprimer l'ancien A si ton IP change.

## Variables d'environnement

| Variable | Description |
|---|---|
| `PAPERLESS_API_TOKEN` | Token API Paperless-ngx |
| `PAPERLESS_URL` | URL Paperless (défaut: `http://paperless:8000`) |
| `DOMAIN` | Ton domaine OVH (ex: `thirdshop.fr`) |
| `SUBDOMAIN` | Sous-domaine MX (défaut: `docs`) |
| `SMTP_LISTEN_ADDR` | Adresse d'écoute (défaut: `:25`) |
| `SMTP_HOSTNAME` | Hostname du serveur SMTP |
| `ALLOWED_SENDERS` | Expéditeurs autorisés (séparés par espaces, vide = tous) |
| `OVH_APP_KEY` | Clé API OVH (optionnel : auto-config DNS) |
| `OVH_APP_SECRET` | Secret API OVH |
| `OVH_CONSUMER_KEY` | Consumer key OVH |
| `OVH_ENDPOINT` | Endpoint OVH (défaut: `ovh-eu`) |
| `DDNS_ENABLED` | Mise à jour auto de l'IP (défaut: `true`) |
| `DDNS_INTERVAL` | Intervalle de vérification IP (défaut: `5m`) |

## Exemple `.env`

```env
PAPERLESS_API_TOKEN=abc123
PAPERLESS_URL=http://192.168.1.50:8000
DOMAIN=thirdshop.fr
SUBDOMAIN=docs
ALLOWED_SENDERS=moi@gmail.com factures@edf.fr
OVH_APP_KEY=xxxx
OVH_APP_SECRET=xxxx
OVH_CONSUMER_KEY=xxxx
```

## Build

```bash
docker compose up -d --build
```

## Utilisation

Depuis ta boîte mail, transfère une facture à `edf@docs.thirdshop.fr` → le document apparaît dans Paperless taggué `edf`.
