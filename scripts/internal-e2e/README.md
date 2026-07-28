# Internal e2e (fork only)

Suite de verificação **local do fork**. Não incluir em PRs para o upstream `evolution-foundation/evolution-go`.

Cobre as correções internas:

| Fix | O que valida |
|-----|----------------|
| `#97` `/group/participant` | Middleware aceita `participants` como array; list/info do grupo; envio de texto (opcional) |
| `#99` websocket write | `go test -race` no producer |
| `#111` settings parciais | PUT advanced-settings com um campo não zera os demais |

## Uso

```bash
cp scripts/internal-e2e/.env.example scripts/internal-e2e/.env
# edite INSTANCE_NAME / GROUP_JID / etc.

./scripts/internal-e2e/run.sh
```

Ou sem gravar token em arquivo:

```bash
export INSTANCE_ID=… INSTANCE_TOKEN=…   # Manager → Instância → Token
./scripts/internal-e2e/run.sh
```

Variáveis úteis:

- `RUN_SEND_GROUP=1` — envia texto no grupo (default: 1)
- `RUN_UNIT=1` — unitários com `-race` (#99 / #111) (default: 1)
- `SKIP_LIVE=1` — só unitários

Auth da API de grupo/send/settings: header `apikey` = **token da instância** (não o `GLOBAL_API_KEY`).

**Nota:** em `SE7E SINOP`, `cesar-teste` não é admin — o e2e do `#97` valida middleware/handler (array `participants`), não add/remove real.
