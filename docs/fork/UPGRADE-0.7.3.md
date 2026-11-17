# Integração da base oficial 0.7.3

Revisão em 2026-11-17. Release e imagem `0.7.3-cesar.1` publicadas e verificadas,
sem implantação em produção. O [registro da publicação](releases/v0.7.3-cesar.1.json)
contém digest, arquiteturas e evidências. A distribuição anterior é
`0.7.2-cesar.4`; seu digest permanece no registro da release anterior.

## Origem e preservação dos patches

- Oficial: tag `0.7.3`, commit `80d7c72c0727e091425d10f5654b7ccf07e01abe`.
- Ancestral comum: `9337afc47e10b86cc896a6f432240e40fee95dd1` (0.7.2).
- Whatsmeow oficial: `54038813c47f3f351eb6fcaafab0e1b99d676850`.
- Whatsmeow do fork: `3e04a7b2fcdfab3a0243983738e448bdbaccb858`, fixado por
  pseudo-version no go.mod, com F-013 e regressões de F-025 preservadas.
- Go mínimo 1.26.0; Docker e validação local usam Go 1.26.10.

O merge conserva o authStore por serviço, cancelamento, WAL e shutdown de
F-019/F-020. O teste oficial de retry/reuso é coberto pelo teste existente
`TestAuthStoreRetryReuseAndSQLiteSessions`, que também verifica persistência,
WAL, limite do pool e fechamento. Não foi introduzido o singleton global oficial.
O registro explícito do driver PostgreSQL e seu teste foram incorporados.

O módulo de licença acompanha o oficial, preservando a liberação dos assets do
Sender. Licença, marcas e atribuições permanecem. O executável legado rastreado
na raiz não é atualizado nem usado na validação: builds vêm de `cmd/evolution-go`.

## Contrato da blocklist

Esta integração adota intencionalmente o contrato oficial de 0.7.3 em
`POST /user/block`, `POST /user/unblock` e `GET /user/blocklist`. O envelope
`message`/`data` é preservado, mas `data.JIDs` deixa de existir. Clientes devem
consumir `data.Items`, cujos objetos contêm `LID`, `PN` e `Active`, além do novo
`data.AddressingMode`. `data.DHash` continua disponível. Identidades são strings
JID; uma identidade ausente pode ser vazia. Não invente PN a partir de LID.

Exemplo de resposta:

```json
{"message":"success","data":{"AddressingMode":"lid","DHash":"hash","Items":[{"LID":"123@lid","PN":"5511999999999@s.whatsapp.net","Active":true}]}}
```

A biblioteca passa a exigir uma identidade LID resolvível para bloquear um
número. Uma falha dessa resolução pode retornar erro. Anotações e Swagger foram
regenerados; uma regressão HTTP verifica os três endpoints e seus campos.

## Banco, implantação e reversão

O store Whatsmeow passa ao schema v16: migrações 15 (companion meta nonce) e 16
(WASA root secret). Não há novas variáveis de ambiente. A implantação deve fazer
backup de bancos e sessões, registrar imagem/digest anterior e verificar as
migrações em homologação. Os testes desta integração usam PostgreSQL descartável
e SQLite temporário; não acessam produção.

As notas oficiais declaram as novas colunas compatíveis com a imagem anterior.
Isso não substitui validar reversão com os dados do ambiente e não desfaz uma
migração. Siga [MAINTENANCE.md](MAINTENANCE.md) antes de implantar ou reverter.

## F-025

A produção usa o patch `0a13d3117a80f33e1325e967e0a4cae800bc09f2` sobre a
biblioteca antiga. Na nova base, o commit oficial
`a2381054887e16d22428430fdc4d2b027a7f29a7` implementa os mesmos caminhos de ACK
e newsletter. O merge mantém o código oficial e o teste `status_ack_test.go`.
A regressão verifica tentativa de ACK para media/text, ausência de evento de
conversa e marcação de newsletter. Um overlay que remove o ACK faz o teste falhar.
Isso comprova o comportamento local; não comprova aceitação remota ou recuperação
de mensagens indecifráveis. Não houve conexão real ao WhatsApp nesta validação.

## Validação

Resultados finais e revisão dos itens F-001–F-025 estão no
[inventário](PATCHES.md#revisão-da-base-oficial-073--2026-11-17).
