# Inventário de diferenças do fork

Base comparada em **2026-10-09**: oficial `0.7.2`, commit
`9337afc47e10b86cc896a6f432240e40fee95dd1`. Revisão inicial: 31 commits adicionais
até `83d8c96`; merges não são contados como problemas independentes.
Política de estados, atualização e retirada: [MAINTENANCE.md](MAINTENANCE.md).
Todos os itens abaixo têm essa data/base de comparação, salvo revisão posterior explícita.
Nenhum item foi considerado substituído pelo upstream nesta base.

Evidência geral: os seis PRs foram revisados e os checks Go completos foram executados
localmente em Docker antes de sua integração. A [CI de 83d8c96](https://github.com/cesar-carlos/evolution-go/actions/runs/37832975972)
aprovou build, vet, testes e race detector; falhou somente na publicação Docker por
ausência de credenciais. Isso não comprova integrações reais nem todos os caminhos concorrentes.

## F-001 — Avatar: JID canônico, prazos e contrato HTTP

- Estado: `local`.
- Problema/esperado: consultas de foto com JID incorreto, cancelamento incompleto e
  respostas inválidas; usar a sessão autenticada, resolver LID localmente, limitar
  o orçamento a 8 s, preservar formato JSON e mapear erros sem expor detalhes internos.
- Commits: `59248af`, `f00f930`, `346a07f`, `2905409`, `7f91622`.
- Referência: [PR #120](https://github.com/evolution-foundation/evolution-go/pull/120), issue #76.
- Evidências: `pkg/user/service/avatar_query_test.go`, testes HTTP de avatar,
  `pkg/instance/repository/query_context_test.go`; cancelamento, payload nulo,
  ausência de imagem, preview, autenticação e isolamento cobertos.
- Limitações: foto real e comportamento do servidor WhatsApp exigem homologação.
- Retirada: solução oficial equivalente para JID, prazos desde startup/DB, erros e contrato,
  com as regressões adaptadas e aprovadas.

## F-002 — Informações de usuário com PictureURL e enriquecimento limitado

- Estado: `local`.
- Problema/esperado: falta de PictureURL e consultas opcionais lentas; preservar dados
  básicos, ordenar/deduplicar resultados, aplicar orçamento total e interromper consultas
  dispensáveis após cancelamento/rate limit. Canonicalização usa apenas a sessão correta.
- Commits: `e2f1627`, `e6a204b`, `67e9625`, `2905409`, `e323425`, `97b699e`.
- Referência: [PR #121](https://github.com/evolution-foundation/evolution-go/pull/121).
- Evidências: `pkg/user/service/user_queries_test.go`, testes HTTP de user queries e
  erros WA; `pkg/whatsmeow/service/query_clients_test.go` cobre o índice concorrente de clientes.
- Limitações: o índice protegido cobre consultas; não significa que todos os mapas legados
  do projeto estejam protegidos. Latência/foto real não foram medidas nesta revisão.
- Retirada: paridade oficial do campo e contratos, limites, falhas opcionais e isolamento,
  com as regressões aprovadas.

## F-003 — Descriptografia de edições e flags de ação nos eventos

- Estado: `local`.
- Problema/esperado: edições criptografadas e flags de exclusão inconsistentes;
  descriptografar antes da troca LID/PN, copiar payloads sem mutar eventos compartilhados,
  preservar fallback e separar instâncias.
- Commits: `2885ceb`, `00e91ca`, `ffe3945`, `ed05110`, `e21f776`.
- Referência: [PR #122](https://github.com/evolution-foundation/evolution-go/pull/122).
- Evidências: `pkg/whatsmeow/service/message_edits*_test.go`, incluindo fixtures e
  handler; documentação de eventos atualizada.
- Limitações: fixtures não substituem envio/recebimento real de edição/revogação.
- Retirada: comportamento oficial completo para edições, exclusão/revogação, fallback
  e isolamento, preservando os testes de regressão.

## F-004 — WebSocket: escritas serializadas e ciclo de vida

- Estado: `local`.
- Problema/esperado: escritas simultâneas causam panic e limpeza antiga remove conexão
  substituta; serializar envio por conexão, limitar escritas e fechar/remover por identidade.
- Commits: `2e540ac`, `6ad714b`.
- Referência: [PR #135](https://github.com/evolution-foundation/evolution-go/pull/135), issue #99.
- Evidências: testes de produtor/ciclo de vida em `pkg/events/websocket` e
  `pkg/whatsmeow/service/websocket_delivery_test.go`; race detector na suíte.
- Limitações: entrega não é persistente/garantida; carga e redes reais exigem homologação.
- Retirada: serialização, limites, fechamento e preservação de substitutas cobertos oficialmente.

## F-005 — Configurações parciais e isolamento da instância

- Estado: `local`.
- Problema/esperado: conexão/settings apagam flags e eventos omitidos; distinguir
  omissão de false/vazio, persistir atualizações parciais e rejeitar acesso a outra instância.
- Commits: `1931961`, `bbf76f6`, `aa7f31b`.
- Referência: [PR #136](https://github.com/evolution-foundation/evolution-go/pull/136), issue #111.
- Evidências: testes de advanced settings, persistência e connect settings nos três
  níveis de `pkg/instance`; anotações/Swagger e guia atualizados.
- Limitações: mocks SQL não demonstram todas as particularidades de PostgreSQL real.
- Retirada: contratos de omissão, autorização e persistência equivalentes no oficial,
  com regressões de isolamento e falhas aprovadas.

## F-006 — Menções em mídia, botões, listas e mensagens encapsuladas

- Estado: `local`.
- Problema/esperado: panic em documento com legenda e menções ignoradas; localizar o
  protobuf real, preservar wrappers/contexto e priorizar PN sem inventar identidade para LID.
- Commits: `68d673a`, `f007e58`.
- Referência: [PR #137](https://github.com/evolution-foundation/evolution-go/pull/137), issue #114.
- Evidências: `pkg/sendMessage/service/send_service_mention_test.go`, payloads diretos/
  encapsulados, contextos, tipos incompletos e identidades de participantes.
- Limitações: notificação real e renderização de mensagens interativas não estão comprovadas;
  aplicar menções não corrige o protocolo de envio de listas/botões.
- Retirada: paridade oficial nos payloads e preservação de contexto, com testes aprovados.

## F-007 — Middleware de participantes de grupo aceita arrays

- Estado: `local`.
- Problema/esperado: middleware incompatível com corpo array em `/group/participant`;
  usar o middleware apropriado sem mudar o formato público da rota.
- Commit: `7013bf0`; referência: issue #97, sem PR específico identificado nesta auditoria.
- Evidências: diff de `pkg/routes/routes.go`; roteiro opcional `scripts/internal-e2e`.
- Pendência: teste automatizado HTTP específico e homologação real não demonstrados.
- Retirada: roteamento/middleware oficial equivalente, com cobertura do corpo array,
  autenticação e isolamento antes de remover o ajuste.

## F-008 — Logs: configuração e redução da exposição de credenciais

- Estado: `local`.
- Problema/esperado: nomes de env divergentes, logs ruidosos e exposição de tokens/URLs;
  respeitar WADEBUG/LOGTYPE, aceitar alias LOG_TYPE, sanitizar mensagens e omitir
  URLs autenticadas do RabbitMQ e token de query no access log WebSocket.
- Commit: `0fb2be0`; referência: manutenção local, sem PR identificado.
- Evidências: `pkg/logger/sanitize_test.go`, diff em config/env, logger, RabbitMQ e
  inicialização Gin; exemplos de ambiente atualizados.
- Limitações: sanitização não comprova ausência de segredos em todos os logs futuros;
  precedência das env e todos os destinos não têm cobertura específica nesta auditoria.
- Retirada: solução oficial com mesma proteção e compatibilidade de configuração, validada.

## F-009 — Compose local e limites operacionais

- Estado: `exclusivo do fork`.
- Problema/esperado: ambiente local com PostgreSQL, pools e limite de logs;
  Compose raiz configura timeouts/conexões e rotação json-file de 100 MB × 5.
- Commits: `16a799f`, `0fb2be0`; imagem/override passam a ser próprios na tag `v0.7.2-cesar.1`.
- Referência: configuração operacional local, sem PR upstream.
- Evidências: `docker-compose.yml`, `.env.example`; validação estrutural do Compose
  na release. Não executar esse stack como efeito colateral dos testes.
- Limitações: max_connections=500 depende de recursos disponíveis; não é recomendação
  universal de produção. Instalações existentes devem revisar configuração.
- Retirada: somente se um ambiente oficial satisfizer o uso local e a decisão for registrada.

## F-010 — Roteiros opcionais de integração do fork

- Estado: `exclusivo do fork`.
- Problema/esperado: reproduzir #97/#99/#111 com um ambiente real, fora da CI unitária.
- Commit: `e85610e`; referências: issues #97, #99, #111.
- Evidências: `scripts/internal-e2e/README.md`, `run.sh`, `lib.sh` e env de exemplo.
- Limitações: podem enviar mensagens reais; não executados na validação unitária/release.
  Não transportar incidentalmente em PRs para o upstream.
- Retirada: decisão explícita de aposentadoria ou substituição por ferramenta equivalente.

## F-011 — Instruções de IA, qualidade e geração de documentação

- Estado: `exclusivo do fork`.
- Problema/esperado: orientar manutenção coesa e validar regressões/contratos;
  centralizar precedência em AGENTS, especialização nas rules e checks reais na CI.
- Commits: `16a799f`, `83d8c96`; `7f91622` também fixa geração Swagger com dependências
  e versão documentada da ferramenta. Swagger gerado acompanha as mudanças funcionais.
- Referências: PR #120 para o contrato/geração; demais instruções são particulares do fork.
- Evidências: rules/AGENTS, template, Makefile e workflow de qualidade; CI geral mencionada acima.
- Limitações: regras não garantem implementação correta; ferramentas executadas e limitações
  devem constar no relatório. Não editar Swagger gerado manualmente.
- Retirada: reavaliar por necessidade do fork; não remover automaticamente ao sincronizar.

## F-012 — Base rastreável, versionamento e publicação independente

- Estado: `exclusivo do fork`.
- Problema/esperado: workflow aponta ao Docker Hub oficial e depende de credenciais ausentes;
  publicar versões próprias no GHCR, com origem/digest verificáveis e retomada segura.
- Introdução: tag `v0.7.2-cesar.1`; arquivos de manutenção, scripts/release e workflows.
- Referência: plano aprovado pelo responsável em 2026-10-09; sem PR upstream.
- Evidências: testes dos guards de versão/base/imagem, actionlint, CI completa e
  manifesto/smoke checks da release. Consulte o run da tag para os resultados finais.
- Retomada das notas: a leitura em JSON evita o terminador acrescentado por `gh --jq`;
  regressão cobre CRLF/LF e rejeita alterações de conteúdo/digest.
- Limitações: visibilidade pública precisa ser configurada no pacote; publicação não
  executa homologação funcional nem implantação. `VERSION` é a fonte de build.
- Retirada: decisão explícita de retornar à distribuição oficial, com paridade de correções
  e transição de instalação documentadas.

## Cobertura da auditoria dos 31 commits

| Commits | Itens |
|---|---|
| `16a799f` | F-009, F-011 |
| `59248af`, `f00f930`, `346a07f`, `7f91622` | F-001 (geração Swagger também F-011) |
| `e2f1627`, `e6a204b`, `67e9625`, `e323425`, `97b699e` | F-002 |
| `2905409` | Documentação F-001/F-002 |
| `2885ceb`, `00e91ca`, `ffe3945`, `ed05110`, `e21f776` | F-003 |
| `2e540ac`, `6ad714b` | F-004 |
| `1931961`, `bbf76f6`, `aa7f31b` | F-005 |
| `68d673a`, `f007e58` | F-006 |
| `7013bf0` | F-007 |
| `0fb2be0` | F-008/F-009 |
| `e85610e` | F-010 |
| `83d8c96` | F-011 |
| `15df3f7`, `882fd48`, `7ba8f68`, `1f46cfa` | Merges de F-003/F-001/F-002; conferir diff dos pais ao migrar |

## Pendências fora da primeira release

Investigar listas/botões a partir da [issue #204](https://github.com/evolution-foundation/evolution-go/issues/204)
e das propostas whatsmeow #1235/#1221. Não assumir que atualizar a biblioteca resolve
automaticamente o envio. Imagens com proporção/EXIF (PR #212), interface móvel (PR #184)
e outros PRs não foram incorporados nesta tarefa. Registrar novos itens quando houver implementação.
