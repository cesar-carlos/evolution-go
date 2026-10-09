> **Distribuição independente mantida por cesar-carlos.** Este fork preserva correções
> locais enquanto acompanha as releases oficiais de Evolution Go. Não implica
> endosso da Evolution Foundation. As informações e atribuições do projeto original
> são preservadas abaixo.
>
> [Manutenção e decisões](docs/fork/MAINTENANCE.md) ·
> [Inventário das correções](docs/fork/PATCHES.md) ·
> [Releases do fork](https://github.com/cesar-carlos/evolution-go/releases) ·
> [Imagem GHCR](https://github.com/cesar-carlos/evolution-go/pkgs/container/evolution-go)

Versão deste código: **0.7.2-cesar.2**, com [compatibilidade interativa](docs/fork/INTERACTIVE-COMPATIBILITY.md),
previews/imagens e interfaces Manager/Sender.
[Release e digest da versão](https://github.com/cesar-carlos/evolution-go/releases/tag/v0.7.2-cesar.2).

```sh
docker pull ghcr.io/cesar-carlos/evolution-go@sha256:57f495abfbce1a2d7665e4a632ad8f26186316be6dcbf2f0b0173f991766299f
```

O digest acima foi verificado por download anônimo em ambas as arquiteturas.
[Registro da verificação](docs/fork/releases/v0.7.2-cesar.2.json). Use essa referência
completa em `EVOLUTION_IMAGE` para instalação reproduzível.

Imagem inicial (referência de reversão): `ghcr.io/cesar-carlos/evolution-go:0.7.2-cesar.1`.
Para instalação reproduzível da primeira release, use o digest público verificado:

```sh
docker pull ghcr.io/cesar-carlos/evolution-go@sha256:8f1771b2dd75e7f07b57d42550d6373d5037abcad3b0197f50154a648fa218ae
```

O Compose raiz aceita `EVOLUTION_IMAGE` com essa referência completa. Consulte
[.env.example](.env.example) para configuração e o guia de manutenção para
homologação, atualizações e reversão. Listas/botões foram adaptados nesta versão;
a homologação de exibição e cliques no WhatsApp permanece pendente.

[Release publicada](https://github.com/cesar-carlos/evolution-go/releases/tag/v0.7.2-cesar.1) ·
[Registro da verificação pública](docs/fork/releases/v0.7.2-cesar.1.json).

Interface de chat do fork: `/sender`, com token de instância informado manualmente.
Consulte [uso e limites do Sender](docs/fork/SENDER.md).

---

<p align="center">
  <a href="https://evolutionfoundation.com.br">
    <img src="./public/hover-evolution.png" alt="Evolution Foundation" />
  </a>
</p>

<h1 align="center">Evolution Go</h1>

<p align="center">
  High-performance WhatsApp API built in Go — part of the Evolution Foundation ecosystem.
</p>

<p align="center">
  <a href="https://github.com/evolution-foundation/evolution-go/releases/latest"><img src="https://img.shields.io/github/v/release/evolution-foundation/evolution-go?include_prereleases&label=version&color=00ffa7" alt="Latest version" /></a>
  <a href="https://opensource.org/licenses/Apache-2.0"><img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg" alt="License: Apache 2.0" /></a>
  <a href="https://docs.evolutionfoundation.com.br"><img src="https://img.shields.io/badge/Docs-evolutionfoundation.com.br-00ffa7" alt="Documentation" /></a>
  <a href="https://evolutionfoundation.com.br/community"><img src="https://img.shields.io/badge/Community-Join%20us-white" alt="Community" /></a>
  <a href="https://hub.docker.com/r/evoapicloud/evolution-go"><img src="https://img.shields.io/badge/Docker-evoapicloud-blue" alt="Docker image" /></a>
</p>

<p align="center">
  <a href="https://evolutionfoundation.com.br">Website</a> &middot;
  <a href="https://docs.evolutionfoundation.com.br">Documentation</a> &middot;
  <a href="https://evolutionfoundation.com.br/community">Community</a> &middot;
  <a href="mailto:suporte@evofoundation.com.br">Support</a>
</p>


---

## About

**Evolution Go** is a high-performance WhatsApp API built in Go. Part of the Evolution Foundation ecosystem, it provides a robust, lightweight solution for WhatsApp integration using the [whatsmeow](https://github.com/tulir/whatsmeow) library.

## Part of the Evolution Foundation ecosystem

Evolution Go is one of the messaging engines maintained by Evolution Foundation. It is used as a WhatsApp provider by the [Evo CRM Community](https://github.com/evolution-foundation/evo-crm-community) and other projects in the ecosystem.

---

## Features

- **High performance** — built with Go for minimal resource usage
- **RESTful API** — clean, well-documented REST endpoints with Swagger
- **Real-time events** — WebSocket, Webhook, AMQP/RabbitMQ and NATS support
- **Media support** — images, videos, audio, documents with MinIO/S3 storage
- **Message storage** — optional PostgreSQL persistence
- **QR code pairing** — built-in QR code generation for device linking
- **License management** — built-in licensing with registration, activation, and heartbeat
- **Docker ready** — production-ready Docker configuration

---

## Quick Start

### Docker (recommended)

```bash
git clone https://github.com/evolution-foundation/evolution-go.git
cd evolution-go
make docker-build
make docker-run
```

### Local development

```bash
git clone https://github.com/evolution-foundation/evolution-go.git
cd evolution-go

# Setup, configure and run
make setup
cp .env.example .env
make dev
```

> Run `make help` to see all available commands. See [COMMANDS.md](./COMMANDS.md) for detailed workflows.

---

## Configuration

Create a `.env` file:

```env
# Server
SERVER_PORT=8080
CLIENT_NAME=evolution

# Security
GLOBAL_API_KEY=your-secure-api-key-here

# Database
POSTGRES_AUTH_DB=postgresql://postgres:password@localhost:5432/evogo_auth?sslmode=disable
POSTGRES_USERS_DB=postgresql://postgres:password@localhost:5432/evogo_users?sslmode=disable
DATABASE_SAVE_MESSAGES=false

# Logging
WADEBUG=DEBUG
LOGTYPE=console

# Optional
# AMQP_URL=amqp://guest:guest@localhost:5672/
# NATS_URL=nats://localhost:4222
# WEBHOOK_URL=https://your-webhook-url.com/webhook
# MINIO_ENABLED=true
# MINIO_ENDPOINT=localhost:9000
# MINIO_ACCESS_KEY=minioadmin
# MINIO_SECRET_KEY=minioadmin
```

| Variable | Description | Default |
|---|---|---|
| `SERVER_PORT` | Server port | `8080` |
| `CLIENT_NAME` | Client identifier | `evolution` |
| `GLOBAL_API_KEY` | API authentication key | **Required** |
| `DATABASE_SAVE_MESSAGES` | Enable message storage | `false` |
| `WADEBUG` | WhatsApp debug level | `INFO` |

---

## License Activation

Evolution Go requires a license to operate. On first run:

1. Start the server — API endpoints return `503` until activated
2. Open the **Manager** at `http://localhost:8080/manager/login`
3. Enter your API URL and `GLOBAL_API_KEY`
4. Complete the license registration flow
5. Once activated, the API is fully operational

The license status persists in the database (`runtime_configs` table). Heartbeats are sent periodically to maintain activation.

---

## API Documentation

Swagger UI available at:

```
http://localhost:8080/swagger/index.html
```

### Key Endpoints

| Method | Endpoint | Description |
|---|---|---|
| `POST` | `/instance/create` | Create WhatsApp instance |
| `GET` | `/instance/{name}/qrcode` | Get QR code for pairing |
| `POST` | `/message/sendText` | Send text message |
| `POST` | `/message/sendMedia` | Send media message |
| `GET` | `/instance/{name}/status` | Get instance status |
| `DELETE` | `/instance/{name}` | Delete instance |

---

## Project Structure

```
evolution-go/
├── cmd/evolution-go/     # Application entry point
├── pkg/
│   ├── core/            # License management & middleware
│   ├── instance/        # Instance management
│   ├── message/         # Message handling
│   ├── sendMessage/     # Message sending
│   ├── routes/          # HTTP routes
│   ├── middleware/      # Auth & validation middleware
│   ├── config/          # Configuration
│   ├── events/          # Event producers (AMQP, NATS, Webhook, WS)
│   └── storage/         # Media storage (MinIO)
├── docs/                # Swagger documentation
├── Dockerfile
├── Makefile
└── VERSION
```

---

## Tech Stack

| Component | Technology |
|---|---|
| Language | Go 1.24+ |
| HTTP framework | Gin |
| WhatsApp | [whatsmeow](https://github.com/tulir/whatsmeow) |
| Database | PostgreSQL |
| ORM | GORM |
| Message queue | RabbitMQ, NATS |
| Object storage | MinIO/S3 |
| Documentation | Swagger/OpenAPI |
| Container | Docker |

---

## Documentation

| Resource | Link |
|---|---|
| Website | [evolutionfoundation.com.br](https://evolutionfoundation.com.br) |
| Documentation | [docs.evolutionfoundation.com.br](https://docs.evolutionfoundation.com.br) |
| Community | [evolutionfoundation.com.br/community](https://evolutionfoundation.com.br/community) |
| Docker Hub | [evoapicloud/evolution-go](https://hub.docker.com/r/evoapicloud/evolution-go) |
| Changelog | [CHANGELOG.md](./CHANGELOG.md) |
| Contributing | [CONTRIBUTING.md](./CONTRIBUTING.md) |
| Security | [SECURITY.md](./SECURITY.md) |

---

## Hosting

Deploy Evolution Go with optimized infrastructure through our HostGator partnership:

[**Evolution Go VPS — HostGator**](https://evolution-api.com/vps-evolution-go)

---

## Telemetry

Evolution Go collects anonymous telemetry data (routes used, API version) to help improve the service. **No sensitive or personal data is collected.**

---

## Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](./CONTRIBUTING.md) for guidelines on how to submit issues, propose features, and open pull requests.

Join our [community](https://evolutionfoundation.com.br/community) to discuss ideas and collaborate.

---

## Security

For security issues, **do not open a public issue**. Email **suporte@evofoundation.com.br** or use GitHub's private vulnerability reporting. See [SECURITY.md](./SECURITY.md) for details.

---

## Acknowledgments

- [whatsmeow](https://github.com/tulir/whatsmeow) by [Tulir Asokan](https://github.com/tulir) — WhatsApp protocol library
- [Evolution API](https://github.com/evolution-foundation/evolution-api) — Node.js sister project

---

## License

Evolution Go is licensed under the Apache License 2.0, with additional brand-protection conditions (LOGO/copyright preservation and Usage Notification requirement). See [LICENSE](./LICENSE) for full details.

For licensing inquiries, contact **suporte@evofoundation.com.br**.

## Trademarks

"Evolution Foundation", "Evolution" and "Evolution Go" are trademarks of Evolution Foundation. See [TRADEMARKS.md](./TRADEMARKS.md) for the brand assets policy.

Third-party attributions are documented in [NOTICE](./NOTICE).

---

<p align="center">
  Made by <a href="https://evolutionfoundation.com.br">Evolution Foundation</a> · © 2026
</p>
