# Auditoria das sete branches — 2026-11-17

Comparação após `git fetch origin --prune`, entre a `main`
`98bcc1f5aa53078cfb176d437efd971b8e3b4527` e o código do PR #17 em
`2cf0b13586e17ae441bd69076c48ae4663e810e9` (0.7.3-cesar.1).

**Resultado: nenhuma correção das sete branches ficou de fora.** Os commits
originais não são ancestrais da main, mas os comportamentos foram integrados
por outros commits/adaptações. Não é necessário mesclar novamente essas branches.
O PR #17 continua separado da main; esta auditoria não faz merge nem implantação.

## Evidência por branch

| Branch em origin / head | Inventário | Comparação com main e PR #17 |
|---|---|---|
| `fix/user-avatar-timeout` / `302ae6b` | F-001 | `avatar_query.go`, seus testes, testes HTTP, contexto do repositório e índice de clientes são idênticos nas duas bases. O arquivo compartilhado `user_queries.go` incorpora também o enriquecimento de F-002. Handler preserva prazo, erro de foto ausente e envelope. |
| `feat/user-info-picture-url` / `ad0c62e` | F-002 | Testes de queries HTTP/serviço, contexto SQL e índice de clientes são idênticos. A diferença em `user_queries.go` delega avatar a `queryAvatar` e restringe sua interface; preserva integralmente a consulta/enriquecimento de usuário, ordenação, orçamento e parada por rate limit. |
| `fix/decrypt-incoming-message-edit` / `cefe7fe` | F-003 | `message_edits.go` e os três arquivos de regressão são idênticos. Integração no handler mantém cópia do evento, preparação antes de trocar identidades, flags e fallback; o contexto passou a acompanhar o ciclo de vida da sessão. |
| `fix/websocket-concurrent-write` / `c88a00d` | F-004 | Serialização, deadline e limpeza por identidade preservados. Testes de produtor e entrega são idênticos; lifecycle ganhou regressão adicional para close code 4001. Produtor acrescenta aviso de substituição sem retirar sincronização. Logs continuam sem o token. |
| `fix/instance-settings-partial-update` / `069e60b` | F-005 | Modelo, handler de settings, helpers e testes HTTP/SQL/connect preservados. A única diferença no teste de regressão de connect adiciona `GetClient` ao stub para o lifecycle atual. Exemplo de eventos corrigido no último commit da branch também está presente. |
| `fix/mention-all-media` / `57e93d7` | F-006 | `mention.go` e todas as suas regressões são idênticos. `SendMessage` usa os helpers para menções explícitas e mentionAll; novos builders de listas/botões continuam encaminhando os campos para ele. Consulta de grupo usa cliente/contexto da sessão. |
| `codex/pr117-shared-auth-store` / `5bb65ac` | F-019/F-020 | `auth_store.go` é igual ao da contribuição mais o `Shutdown` que encerra sessões antes do store. Teste PostgreSQL é idêntico; testes unitários ampliam a garantia de que o pool emprestado não será fechado. Entrypoint preserva PingContext limitado e fechamento após falha. |

A branch de PR #117 pertence ao histórico independente de develop; não possui
merge-base com main. Foi comparada a contribuição de armazenamento e seus pontos
de integração, sem transportar o restante desse histórico nem seu módulo antigo.
O antigo `TestGetAuthContainerRetryAndReuse` é substituído por
`TestAuthStoreRetryReuseAndSQLiteSessions`: este testa falha e recuperação no mesmo
store, reuso, persistência SQLite, WAL, limite de pool e fechamento. A cobertura
não foi perdida pela ausência do arquivo antigo.

A comparação main → PR #17 mantém integralmente esses helpers e regressões.
As alterações em usuário da atualização 0.7.3 são as assinaturas da biblioteca e
o contrato da blocklist, descritos em [UPGRADE-0.7.3.md](UPGRADE-0.7.3.md).
Em `whatsmeow.go`, apenas o import do driver PostgreSQL muda de lugar para config.

## Validação

Validação desta auditoria: os quatro comandos abaixo passaram, sem falhas
nem races detectadas, com `-count=1` para reexecutar os testes. `git diff --check`
também passou. Ambiente Docker Linux com
Go 1.26.10, CGO, dependências nativas e PostgreSQL 16 descartável. Os comandos
usam `GOFLAGS=-mod=readonly`, e ambas as variáveis de integração PostgreSQL
apontam exclusivamente para o banco descartável:

```sh
go build ./...
go vet ./...
go test -count=1 -timeout 5m ./...
go test -race -count=1 -timeout 5m ./...
```

Não houve acesso a produção, envio WhatsApp real ou alteração de dependências.
Checks locais não equivalem a CI aprovada; o bloqueio de execução no GitHub está
registrado em [CI-PR17.md](CI-PR17.md).
