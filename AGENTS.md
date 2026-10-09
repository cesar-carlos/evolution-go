# Evolution Go — instruções de trabalho

API WhatsApp em Go/Gin, com whatsmeow, PostgreSQL/GORM, sessões em PostgreSQL ou
SQLite e eventos por Webhook, WebSocket, RabbitMQ e NATS. Leia `go.mod`, `VERSION`,
`pkg/routes/routes.go` e `.env.example` para valores e contratos atuais.

Antes de corrigir bugs, atualizar dependências, sincronizar o upstream ou publicar,
leia [manutenção do fork](docs/fork/MAINTENANCE.md) e
[inventário de correções](docs/fork/PATCHES.md). Esses documentos registram o escopo
aprovado e as decisões; mantenha o inventário atualizado conforme a política do guia.
A base oficial adotada está em `.github/upstream-base.json`.

## Princípios e precedência

1. Respeite as instruções do usuário e o escopo da tarefa; preserve trabalho existente.
2. Correção, isolamento entre instâncias e gerenciamento seguro de recursos vêm antes
   de imitar o legado. Não reproduza races, erros ignorados ou vazamento de credenciais.
3. Preserve contratos públicos, nomenclatura e organização locais. Uma mudança
   incompatível precisa ser intencional, documentada e acompanhada de migração quando cabível.
4. Faça a menor mudança **completa**, incluindo testes e documentação necessários.
   Extrações locais necessárias cabem na tarefa; refatorações independentes ficam separadas.
5. Prefira funções e tipos coesos. Não crie camadas ou interfaces sem necessidade real.

Este arquivo centraliza precedência e validação. Leia as rules abaixo conforme o
escopo, mesmo quando a ferramenta não carregar arquivos `.mdc` automaticamente.
Não duplique estas instruções nas rules; mantenha nelas os detalhes especializados.

## Mapa das rules (`.cursor/rules/`)

| Rule | Quando consultar |
|---|---|
| `project-overview.mdc` | Estrutura e fontes de verdade |
| `fork-community-scope.mdc` | Toda alteração; limites do escopo e estilo local |
| `architecture-solid-clean.mdc` | Responsabilidades e dependências entre camadas |
| `maintainability-modern-go.mdc` | Coesão, extrações e manutenção |
| `go-standards.mdc` | Código Go, erros, contextos, concorrência e recursos |
| `testing-quality.mdc` | Lógica, correções, testes e CI |
| `gin-api-swagger.mdc` | HTTP, autenticação, isolamento e contratos |
| `whatsmeow-integration.mdc` | Sessões, envio, QR, passkey e reconexão |
| `persistence-events-storage.mdc` | Banco, transações, eventos, storage e configuração |
| `documentation.mdc` | API, env, Swagger e documentação |
| `contributing.mdc` | Commits e PRs |

## Validação

- Formate **apenas os arquivos Go alterados**: `gofmt -w caminho/arquivo.go`.
  `make fmt` executa `go fmt ./...`; não é um comando limitado ao diff.
- Durante a implementação, execute `go test ./pkg/pacote/...` para os pacotes afetados
  e consumidores relevantes. Execute `go vet` nesses pacotes quando alterar código Go.
- Para alterações concorrentes, execute também `go test -race ./pkg/pacote/...`
  em ambiente com CGO e compilador C compatíveis.
- Na CI: `go build ./...`, `go vet ./...`, `go test -timeout 5m ./...` e
  `go test -race -timeout 5m ./...`. Os testes automatizados não precisam de WhatsApp,
  licença, banco de produção ou credenciais reais.
- `golangci-lint` é complementar, quando disponível; não instalar `@latest` ou
  atualizar dependências incidentalmente. A base obrigatória da CI está acima.
- Mudou contrato HTTP: atualize anotações e execute `make swagger`; não edite os
  arquivos gerados manualmente. Mudou configuração: atualize `.env.example`.
- Revise `git diff --check` e o diff final. Relate verificações executadas, resultados
  e limitações; ferramenta ausente não significa teste aprovado.
- Documentação isolada não exige testes Go: confira links, comandos e consistência.

O Makefile pressupõe ferramentas de shell Unix. Em PowerShell, prefira os comandos
Go equivalentes; para CI e dependências nativas, use o ambiente Linux configurado no
workflow. Não execute E2E com envio real como parte de uma verificação unitária.
