# Inventário de diferenças do fork

Base atual em integração: oficial **0.7.3**, commit
`80d7c72c0727e091425d10f5654b7ccf07e01abe`, candidato `0.7.3-cesar.1`.
A [revisão de 2026-11-17](#revisão-da-base-oficial-073--2026-11-17) atualiza
os estados abaixo; as comparações anteriores são mantidas como histórico.

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
- Limitações: F-020 ampliou a sincronização para os consumidores de sessões;
  latência/foto real não foram medidas nesta revisão.
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
- Commits: `16a799f`, `0fb2be0`, `6037aad`; imagem/override próprios na tag `v0.7.2-cesar.1`.
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
- Commits: `16a799f`, `83d8c96`, `6037aad`, `d7aa824`; `7f91622` também fixa geração Swagger com dependências
  e versão documentada da ferramenta. Swagger gerado acompanha as mudanças funcionais.
- Referências: PR #120 para o contrato/geração; demais instruções são particulares do fork.
- Evidências: rules/AGENTS, template, Makefile e workflow de qualidade; CI geral mencionada acima.
- Revisão em 2026-11-17, PR #17: `quality.yml` aceita `workflow_dispatch` com
  `format_base` obrigatório, validado como ancestral antes do check de formatação.
  Actionlint 1.7.9 e `git diff --check` passaram. O dispatch autenticado retornou
  HTTP 422 `Actions has been disabled for this repository.`, mesmo com permissões
  `enabled: true`, workflow ativo e nenhuma política de eventos/atores. Reabrir
  o PR não iniciou run; reaplicar enable do repositório e workflow retornou 204,
  mas o dispatch permaneceu bloqueado. Não há CI aprovada para este PR.
  Evidências e retomada: [diagnóstico de Actions](CI-PR17.md).
- Revisão documental em 2026-10-09: rule de persistência/Docker passou a referenciar
  a política de imagem do fork, removendo a indicação conflitante da imagem upstream.
- Limitações: regras não garantem implementação correta; ferramentas executadas e limitações
  devem constar no relatório. Não editar Swagger gerado manualmente.
- Retirada: reavaliar por necessidade do fork; não remover automaticamente ao sincronizar.

## F-012 — Base rastreável, versionamento e publicação independente

- Estado: `exclusivo do fork`.
- Problema/esperado: workflow aponta ao Docker Hub oficial e depende de credenciais ausentes;
  publicar versões próprias no GHCR, com origem/digest verificáveis e retomada segura.
- Introdução: tag `v0.7.2-cesar.1`; arquivos de manutenção, scripts/release e workflows.
- Commits: `6037aad`, `d7aa824`; `141bce7` corrige a retomada das notas após a tag inicial.
- Referência: plano aprovado pelo responsável em 2026-10-09; sem PR upstream.
- Evidências: testes dos guards de versão/base/imagem, actionlint, CI completa e
  manifesto/smoke checks da release. Consulte o run da tag para os resultados finais.
- Retomada das notas: a leitura em JSON evita o terminador acrescentado por `gh --jq`;
  regressão cobre CRLF/LF e rejeita alterações de conteúdo/digest.
- Publicação e acesso anônimo: [registro verificável](releases/v0.7.2-cesar.1.json),
  com digest, arquiteturas, comandos e run da primeira release.
- Limitações: visibilidade pública precisa ser configurada no pacote; publicação não
  executa homologação funcional nem implantação. `VERSION` é a fonte de build.
- Retirada: decisão explícita de retornar à distribuição oficial, com paridade de correções
  e transição de instalação documentadas.

## F-013 — Protocolo interativo e seleção sem ambiguidade

Complemento local em 2026-11-17: a validação e mídia de carrossel foram tratadas
em F-026. Os payloads de listas/botões e os patches da biblioteca permanecem.
Complemento em 2026-11-18: F-028 substitui, no envio da aplicação, a hipótese
de equivalência entre enum SINGLE_SELECT e atributo `single_select`; mantém
o histórico abaixo e os patches de respostas/wrappers da biblioteca.

- Commits: `f5fd551`, complemento `cdc7388`; [PR do fork #3](https://github.com/cesar-carlos/evolution-go/pull/3).
- Estado: `local`; comparação em 2026-10-09, base oficial `0.7.2`.
- Problema/esperado: native-flow sem metadados, duplicação de `biz`, respostas com
  nós indevidos e IDs de lista repetidos. Usar payloads diretos, um responsável pelos
  nós, IDs únicos e validação antes de operações externas. Carrosséis escapam JSON.
- Dependência: [cesar-carlos/whatsmeow](https://github.com/cesar-carlos/whatsmeow/commit/14e0a4ffa1fc6472e97b36a344d3936d118c5177),
  baseada em `b572e5bcb92bbc285b68cb6d6540da3093330e04`, fixada por pseudo-version
  no `replace` de go.mod. Licença MPL-2.0 preservada; manutenção descrita no fork da biblioteca.
  O complemento `14e0a4f` sucede `471f98b`: os atributos da lista acompanham o tipo
  protobuf, e a consulta de atributos reutiliza o desembrulhamento limitado/nil-safe.
  A versão anterior anunciava SINGLE_SELECT como product_list; essa correspondência
  foi corrigida para single_select, sem afirmar aceitação remota comprovada.
- Referências: whatsmeow #1235 (felps-dev), #1221 (gsdev-br), #1146 (TobyG74), Evolution Go issue #204.
- Evidências: testes de nós/respostas/wrappers no módulo da biblioteca; testes de
  builders e validação HTTP em `pkg/sendMessage`. Testes reproduziram falhas anteriores.
  `TestWrappedListPreservesSelectionContract` falhou na implementação anterior;
  também são cobertos atributos em wrappers vazios/cíclicos/profundos, respostas
  template/native-flow e biz/hsm explícitos duplicados, preservando metadados.
- Mudança de validação: `rowId` explícito duplicado passa a retornar 400. Clientes devem
  fornecer IDs únicos; IDs omitidos são gerados sem colisão. `buttonText` vazio usa "Ver Menu".
- Limitações: atributos de protocolo observados em PRs não constituem especificação
  oficial. Renderização/cliques em Android/iOS/Web e PIX continuam pendentes de homologação.
- Retirada: upstream com nós/respostas equivalentes, preservação de contratos e testes
  aprovados, seguido de homologação. Remover `replace` somente após comprovar essa paridade.

## F-014 — Parsing de respostas interativas

Complemento local em 2026-11-17: F-027 corrige o encaminhamento ao NATS global;
o parser e os contratos de `ButtonClick` permanecem.

- Commits: `f5fd551`; [PR do fork #3](https://github.com/cesar-carlos/evolution-go/pull/3).
- Estado: `local`; comparação em 2026-10-09, base oficial `0.7.2`.
- Problema/esperado: reconhecer respostas encapsuladas e IDs de seleção native-flow,
  preservar o contrato `ButtonClick` e não mutar mensagens compartilhadas.
- Referências: caminhos de resposta discutidos em whatsmeow #1221/#1235 e issue #204.
- Evidências: `button_click_test.go`, formatos legado/template/lista/native-flow,
  wrappers, JSON inválido, payload vazio e preservação da mensagem original.
- Limitações: testes do parser não comprovam entrega pelos brokers nem interação real.
- Retirada: comportamento oficial equivalente com testes de parsing e isolamento preservados.

## F-015 — Dimensões, EXIF e limites de miniaturas

- Commits: `550fcda`; [PR do fork #4](https://github.com/cesar-carlos/evolution-go/pull/4).
- Estado: `local`; comparação em 2026-10-09, base oficial `0.7.2`.
- Problema/esperado: imagens sem dimensões são recortadas nos clientes; miniaturas
  ignoram rotação/espelhamento. Preencher dimensões nos caminhos de mídia/status/
  botões/carrosséis e aplicar EXIF às miniaturas, com limite antes de decodificar pixels.
- Referência: Evolution Go #212, mediam4kers; helpers adaptados com validação adicional.
- Evidências: testes de dimensões, orientações 1–8 com pixels assimétricos, EXIF big/
  little endian, tipo/count/offset inválidos, limite de bitmap e seeds de fuzzing.
- Limitações: orientação visual no WhatsApp e todos os formatos reais exigem homologação.
  Imagem inválida mantém envio sem dimensões/miniatura, conforme fallback anterior.
- Retirada: suporte oficial equivalente em todos os caminhos e limites de recursos,
  com regressões aprovadas e homologação registrada.

## F-016 — Preview de links com fallback, contexto e fetch limitado

- Commits: `7f2f906`, regressões `cdc7388`; [PR do fork #5](https://github.com/cesar-carlos/evolution-go/pull/5).
- Estado: `local`; comparação em 2026-10-09, base oficial `0.7.2`.
- Problema/esperado: preview pequeno, metadados explícitos sobrescritos, URLs relativas
  incorretas após redirects, EXIF perdido e operações sem cancelamento. Preparar uma
  vez, preservar dados, produzir miniaturas inline/HQ e transmitir a mensagem uma vez.
- Referência: Evolution Go #207 (cateim), adaptado com tratamento de EXIF e contexto.
- Evidências: testes do PR adaptados e regressões de redirects, cancelamento, limites,
  upload com falha, newsletter, EXIF e preservação de texto/metadados. Helpers de envio
  usam cliente da instância pelo índice sincronizado e contexto nas operações afetadas.
  A revisão adiciona cancelamento durante upload e rejeição de redirects com
  credenciais, sem vazar essas credenciais no erro.
- Configuração: `LINK_PREVIEW_ALLOW_PRIVATE=false`; acesso a redes locais/privadas
  exige opt-in explícito. A política vale também após redirects e resolução DNS.
  Para instalações que usavam previews internos, habilitar somente em ambiente confiável.
- Limitações: HQ em newsletters mantém fallback inline; aparência remota depende de
  homologação. O helper Go SendLink passa a receber contexto; o contrato HTTP permanece.
- Retirada: paridade oficial de metadados, I/O limitado/cancelável, fallback e proteção
  de destinos, preservando testes e compatibilidade documentada.

## F-017 — Menu acessível do Manager em dispositivos touch

- Commits: `5d492d2`; [PR do fork #6](https://github.com/cesar-carlos/evolution-go/pull/6).
- Estado: `local`; comparação em 2026-10-09, base oficial `0.7.2`.
- Problema/esperado: navegação e ações invisíveis em telas touch. Overlay legível
  em `manager-mobile.css/js`, sem modificar bundles, Dockerfile ou branding.
- Referência: Evolution Go #184 (douglasanpa), com ciclo de vida/foco reimplementados.
- Evidências: `tests/ui/manager.test.cjs` executado com Playwright/Edge: foco, Tab,
  Escape, remontagem, resize, scroll e ações em tablet touch. Os testes passaram; validação adicional usa o bundle real do Manager com API simulada.
- Limitações: seletores dependem da estrutura do bundle; revalidar ao trocar Manager.
  Testes usam fixture e bundle real com API simulada, sem sessão WhatsApp real.
- Retirada: Manager oficial com navegação touch/teclado equivalente e regressões aprovadas.

## F-018 — Sender com autenticação de instância e recursos limitados

- Commits: `60d68dd`, `6dd14bf`, `39b4001`, regressões `cdc7388`; [PR do fork #7](https://github.com/cesar-carlos/evolution-go/pull/7)
  e [complemento #8](https://github.com/cesar-carlos/evolution-go/pull/8).
- Estado: `local`; comparação em 2026-10-09, base oficial `0.7.2`.
- Problema/esperado: interface de chat ausente. Adaptação independente de Evolution
  Go #182 (prakash-dev-code), sem injeção da chave global ou pools SQL no handler.
- Comportamento: `/sender`, credenciais em memória, envio REST existente, eventos
  restritos à instância, reconexão/limpeza, histórico e mídia limitados. Manager/raiz preservados.
- Evidências: testes HTTP de auth/origem/isolamento e HTML loopback/proxy; testes
  de stores reais SQLite/PostgreSQL; cinco testes de navegador Manager/Sender
  com REST/WS simulados, inclusive XSS, troca de instância, mídia, limites, estados
  de receipt inválidos e reconexão. Metadados retidos também têm limites de tamanho.
- Documentação: [SENDER.md](SENDER.md). Swagger gerado; assets copiados na imagem.
  CI executa testes de navegador e regressões da dependência Whatsmeow fixada.
- Limitações: produtor WS permite uma conexão por instância (4001 pausa o anterior). Query token
  precisa ser omitido dos logs do proxy. Funcionamento remoto exige homologação.
- Retirada: Sender oficial com isolamento, credenciais não expostas, stores em ambos
  os dialetos e gestão de recursos equivalentes, com regressões e homologação aprovadas.

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

## Validação da segunda release e pendências de homologação

F-013–F-018 compõem `0.7.2-cesar.2`, mantendo a base oficial em `0.7.2`.
Build, vet, testes e race detector completos passaram em Docker (Go 1.25.0 Linux),
assim como testes da dependência Whatsmeow, stores SQLite/PostgreSQL reais em ambiente
isolado, actionlint 1.7.7 e 12 guards de release. Cinco testes Playwright/Edge passaram;
a CI também os executa no Chromium Linux e detectou overflow do input de arquivo,
corrigido em `6dd14bf`. Consulte os checks dos PRs e o run da tag para o resultado final.

Renderização e cliques em Android/iOS/Web permanecem pendentes. Não houve envio real,
pagamento PIX ou implantação em produção. A publicação, os smoke tests de ambas as arquiteturas, os labels e o download
anônimo por digest foram aprovados no [registro da segunda release](releases/v0.7.2-cesar.2.json).
A primeira tentativa foi cancelada após downloads lentos no mirror Ubuntu; a segunda
passou, preservando a tag e o digest da versão anterior.

## Complementos da revisão de contribuições

F-013/F-016/F-018 receberam complementos de código ou regressões após a comparação
dos heads de 2026-10-09. O mapa dos PRs, diferenças novas e verificações de origem
está em [UPSTREAM-CONTRIBUTIONS.md](UPSTREAM-CONTRIBUTIONS.md). Os demais ajustes
de EXIF, Manager e Sender já estavam integrados e foram preservados.
O fork da biblioteca passou build, vet, testes e race detector em Docker Go 1.25.0.
A integração `cdc7388`, preparada para `0.7.2-cesar.3`, também passou build, vet,
testes e race detector completos do projeto, regressões da biblioteca fixada,
12 guards de publicação e cinco testes Playwright/Edge. A CI revalida PostgreSQL
isolado e Chromium Linux. A base oficial continua em `0.7.2`.
A homologação Android/iOS/Web continua pendente; não houve envio real nesta revisão.

## Publicação dos complementos — 0.7.2-cesar.3

Os complementos foram integrados na `main` pelo [PR do fork #10](https://github.com/cesar-carlos/evolution-go/pull/10),
commit `c73f3a9cfdaa87128c537449f49812e37966ac62`, sem alterar a base oficial.
A CI da `main` e o workflow da tag passaram. A segunda tentativa do workflow
concluiu a publicação; a primeira foi cancelada durante downloads lentos no mirror
Ubuntu, antes de executar o job de publicação.

O [registro da terceira release](releases/v0.7.2-cesar.3.json) preserva o digest,
manifesto/aliases, download anônimo e smoke tests de `linux/amd64` e `linux/arm64`.
Versão compilada, commit da biblioteca, labels e recursos Manager/Sender foram
conferidos nas duas imagens. O digest de `0.7.2-cesar.2` permanece preservado.
O mesmo registro é anexado à GitHub Release, sem substituir assets anteriores.

Não houve implantação em produção nem homologação real de WhatsApp. As limitações
interativas e o protocolo experimental continuam descritos em F-013 e no guia de
compatibilidade; publicação aprovada não representa comprovação de renderização/cliques.

## F-019 — Store de autenticação compartilhado e recuperável

- Estado: `parcialmente coberto pelo upstream` desde 0.7.3; armazenamento local
  integrado pelo PR #12, commit `0a50034`. A revisão de 2026-11-17 mantém
  cancelamento por chamador, WAL correto, ownership e shutdown do fork.
- Validação: CI [37974834132](https://github.com/cesar-carlos/evolution-go/actions/runs/37974834132) aprovada (Go/race/PostgreSQL e navegador).
- Commits: `0a50034` (store/entrypoint) e `68f4725` (isolamento do fixture PostgreSQL).
- Comparação: 2026-10-09, base oficial `0.7.2`, issue #186.
- Referências: #117 (guilhermeCassettari e Ay0rus), #194, #174/#178 e #206;
  integração seletiva descrita em [UPSTREAM-CONTRIBUTIONS.md](UPSTREAM-CONTRIBUTIONS.md).
- Problema/esperado: StartClient abria um pool por chamada. Reutilizar authDB no
  PostgreSQL e um único main.db SQLite, sem cache permanente de falhas.
- Código: gerenciador privado por serviço, inicialização única em andamento,
  Upgrade antes da publicação, deadline de 30 s e espera cancelável por chamador.
  SQLite limita uma conexão, ativa WAL e é fechado pelo gerenciador. PostgreSQL
  é emprestado do entrypoint, que limita o pool e usa PingContext de 10 s.
- Evidências: auth_store_test.go cobre recuperação, persistência real de sessão,
  32 chamadas simultâneas, cancelamento independente, deadline e shutdown.
  auth_store_postgres_test.go verifica PostgreSQL real, 20 ciclos × 32 chamadas,
  preservação do handle/limite e ausência de crescimento acumulado de conexões.
- Limitações: homologação real de WhatsApp pendente. A serialização e o
  encerramento completo das sessões pertencem à entrega de ciclo de vida.
- Retirada: store oficial com reutilização, recuperação, contextos e ownership
  equivalentes, comprovados pelas regressões SQLite/PostgreSQL.
## F-020 — Ciclo de conexão por instância e encerramento dos workers

- Estado: `local`; commits `68f4725`, `0f52e07`, `1b3a032` e `6115fa2`,
  integrados pelos PRs #13/#15.
- CI: [entrega principal](https://github.com/cesar-carlos/evolution-go/actions/runs/37978099885),
  [complemento](https://github.com/cesar-carlos/evolution-go/actions/runs/37979506985) e
  [commit final](https://github.com/cesar-carlos/evolution-go/actions/runs/37979856414)
  aprovados (Go/PostgreSQL/race e navegador).
- Comparação: 2026-10-09, base oficial `0.7.2`; issue #186 e revisão do #200.
  #200/#131 são referências de defeitos evitados, não merges incorporados.
- Problema/esperado: mapas/canais compartilhados permitiam corridas, workers órfãos
  e reconexão após parada intencional. Serializar operações por instância e impedir
  callbacks/limpeza antigos de afetar uma substituta.
- Implementação: registro privado, identidade por execução, snapshots de settings,
  cancelamento difundido, conclusão explícita e reconexões coalescidas/limitadas.
  A intenção de parada bloqueia novos retries antes de aguardar o gate da instância,
  evitando a corrida entre cancelar uma reconexão e enfileirar outra.
  Transporte permanece vivo para logout depois de parar os workers. QR/passkey têm
  sinal de disponibilidade; polling não cria ciclos de QR após expiração.
  DeviceProps é clonado por cliente, evitando mutação global entre instâncias.
  Atualizações de settings e respostas/confirmações HTTP de passkey pertencem à
  execução; o token da cerimônia é revalidado após vincular a operação. Elas são canceladas e
  aguardadas na parada; a limpeza remove a cerimônia antiga antes de outra execução.
- Evidências: session_lifecycle_test.go cobre 32 inícios/paradas, isolamento,
  workers, substituição, exclusão durante startup, cinco retries, cancelamento,
  shutdown, espera de pareamento e ordem worker/logout/transporte.
  TestStopIntentRejectsRacingReconnectBeforeGate reproduz a admissão indevida de
  reconexão ao remover o guard via overlay Go; passou com a correção e -race.
  TestRuntimeOperationBelongsToExecution e TestOldCeremonyCannotAuthorizeReplacementClient
  cobrem cancelamento, tokens antigos/acesso entre instâncias e barreira de requisições
  passkey. session_qr_test.go verifica cancelamento da rotação, expiração sem reinício e
  preservação da cerimônia passkey ativa. Build/vet/testes completos/race Docker,
  PostgreSQL/SQLite reais, biblioteca, navegador e 12 guards de publicação passaram.
  Workflows conferidos com actionlint 1.7.12; CI do PR aprovada.
- Contratos: endpoints/payloads preservados. Semântica corrigida de parada descrita
  no [guia](MAINTENANCE.md#sessões-e-parada-de-instâncias); sem novas anotações Swagger.
- Limitações: pareamento, reconexão e passkey reais pendentes; nenhum envio real nos
  testes. Slots de instâncias removidas conservam identidade/token em memória até
  o shutdown para rejeitar trabalho atrasado; não conservam clientes/pools ativos.
- Retirada: ciclo oficial com isolamento, ownership e retries equivalentes,
  comprovado pelas regressões do fork, incluindo ausência de reinício intencional.

## Publicação de armazenamento e ciclo de vida — 0.7.2-cesar.4

F-019/F-020 foram integrados pelos PRs #12/#13/#15; a versão foi preparada no #14.
Commit da tag: `bf85d753c3803158f0a1ee280180cd4914ad9db9`, validado na main e pelo
workflow da publicação. Base oficial 0.7.2 e patches anteriores preservados.

O [registro da quarta release](releases/v0.7.2-cesar.4.json) contém digest, aliases,
manifesto, download anônimo e smoke checks das duas arquiteturas. Versão compilada,
labels, biblioteca e recursos Manager/Sender foram conferidos. O mesmo registro
está anexado à GitHub Release; a imagem é pública e não publica alias latest.

O digest anterior (v3) permanece disponível para reversão. A comparação v3/v4 de
go.mod/go.sum, configuração e modelos não mostrou mudanças; não há novas migrações
de aplicação. Antes de reverter produção, conferir banco/sessões e backup conforme
o [guia](MAINTENANCE.md). Não houve acesso ao banco nem implantação em produção.
Pareamento, reconexão, passkey e renderização/cliques reais continuam pendentes.

## F-021 — QR após logout remoto e favicon do Manager

- Estado: `local`; comparação em 2026-10-14, base oficial `0.7.2`.
- Problema/esperado: um 401 (`logged out from another device`) cancelava o contexto
  da conexão durante o `DELETE` da sessão. O dispositivo e o JID permaneciam, o
  polling de `/instance/qr` reconectava a sessão revogada e a API respondia
  `session ended before connecting`, sem emitir QR. O HTML do Manager também
  pedia `/vite.svg`, arquivo ausente no build.
- Comportamento: o logout remoto apaga o dispositivo com um contexto próprio,
  limpa JID e QR e impede a reconexão automática. A mesma consulta de QR inicia
  um pareamento novo quando o JID foi descartado. `/vite.svg` e `/favicon.ico`
  respondem 204; o HTML do Manager aponta para `/favicon.ico`.
- Evidências: `session_logout_test.go` cobre descarte do JID, ausência de
  reconexão e o placeholder de proxy `null`.
- Limitações: o pareamento real com o celular continua dependente do WhatsApp.
- Retirada: fluxo oficial que descarta a sessão revogada e gera QR sem reutilizar
  o dispositivo removido.

## F-022 — Versão do handshake rejeitada com HTTP 405

- Estado: `parcialmente coberto pelo upstream` desde 0.7.3: a biblioteca atualiza
  a revisão padrão. Mantemos configuração/fetch dinâmico e regressões do handshake.
  Comparação original em 2026-11-03, base oficial `0.7.2`.
- Problema/esperado: o WhatsApp fechava o websocket com `Client outdated (405)`
  porque o handshake continuava na versão embutida na biblioteca
  (`2.3000.1042386815`). A revisão obtida de `web.whatsapp.com/sw.js`, ou de
  `WHATSAPP_VERSION_*`, só preenchia `DeviceProps.Version`. O handshake deve
  anunciar essa mesma revisão em `UserAgent.AppVersion`.
- Comportamento: `store.SetWAVersion` recebe a versão configurada ou a revisão
  atual da web antes de abrir o cliente. Conexões simultâneas esperam essa
  publicação. Sem as duas fontes, permanece o padrão da biblioteca. A leitura
  da revisão aceita o JSON escapado do service worker e, se a página de erro
  não trouxer o corpo, o parâmetro `cv` do cabeçalho.
- Evidências: `web_version_test.go` cobre o service worker escapado, JSON simples,
  revisão ausente e a atualização do payload de handshake.
- Limitações: a versão é global no processo. A aceitação pelo WhatsApp depende
  da revisão publicada na web e não é coberta por teste unitário.
- Retirada: a biblioteca fixada já anuncia uma revisão aceita e o projeto deixa
  de precisar sobrepor `UserAgent.AppVersion`.

## F-023 — Evento Archive não pode entrar em panic

- Estado: `local`; comparação em 2026-11-03, base oficial `0.7.2`.
- Problema/esperado: o handler fazia type assert de `postMap["data"]` para
  `map[string]interface{}`, mas o valor ainda era `*events.Archive`. O evento
  encerrava com panic e não seguia para o webhook.
- Comportamento: o payload de Archive é montado como mapa JSON, com JID em texto.
- Evidências: `TestArchiveEventDataIsJSONSafe`.
- Limitações: o teste não publica o webhook.
- Retirada: o handler oficial trata Archive sem assert do evento bruto.

## F-024 — Alerta por e-mail quando a API ou as sessões permanecem fora

- Estado: `exclusivo do fork`; comparação em 2026-11-03, base oficial `0.7.2`.
- Problema/esperado: `/server/ok` continua saudável quando o WhatsApp recusa as
  sessões. O aviso precisa observar a API e as instâncias pareadas de fora do
  processo, porque um container parado não envia o próprio alerta.
- Comportamento: `scripts/monitor/watch.py` lê a configuração de
  `/etc/evolution-go-monitor.env`. A API sem resposta espera 2 minutos; a perda
  total ou de metade das sessões conectadas espera 10 minutos, para um restart
  reconectar. O mesmo episódio se repete depois de 6 horas. Duas leituras
  saudáveis enviam a recuperação. Sem `MONITOR_SMTP_HOST` ou
  `MONITOR_EMAIL_TO`, o aviso fica pendente e a tentativa seguinte reenvia.
- Evidências: `scripts/monitor/test_watch.py`.
- Limitações: a entrega depende de um SMTP informado pelo operador. O teste não
  abre conexão nem consulta o banco.
- Retirada: o fork deixar de operar um vigilante próprio para essas interrupções.

## F-025 — Confirmação de nós status do WhatsApp

- Estado: `substituído pelo upstream` na integração da base 0.7.3, em 2026-11-17.
- Origem: alteração em produção, ainda sem commit no Evolution Go; biblioteca
  [0a13d31](https://github.com/cesar-carlos/whatsmeow/commit/0a13d3117a80f33e1325e967e0a4cae800bc09f2).
- Problema/esperado: nós `status` de mídia/texto sem ACK eram associados a quedas
  com `stream:error` e fila offline retida. Confirmar status sem convertê-lo em
  conversa; preservar eventos de newsletter e `IsNewsletterStatus`.
- Solução oficial: [a238105](https://github.com/tulir/whatsmeow/commit/a2381054887e16d22428430fdc4d2b027a7f29a7),
  presente em `54038813c47f3f351eb6fcaafab0e1b99d676850`. Após o merge, client.go
  e message.go do fork da biblioteca são idênticos à base oficial; não há handler
  duplicado nem necessidade de transportar novamente o patch de produção.
- Dependência integrada: `3e04a7b2fcdfab3a0243983738e448bdbaccb858`, mantendo F-013.
- Evidências: `TestStatusNodeIsAcknowledgedWithoutBecomingAChat` preservado e
  reforçado, passando normal e com race. A chamada pelo handler registrado deve
  tentar ACK para media/text (erro de transporte desconectado observado), não
  publicar conversa e marcar newsletter. Remover somente o ACK via overlay faz
  o teste falhar com `did not attempt an ACK`.
- Limitações: sem WhatsApp real; o teste não demonstra aceitação remota, recuperação
  de mensagens indecifráveis ou entrega da fila offline. A implantação posterior
  está no [registro de produção](deployments/2026-11-17-0.7.3-cesar.1.md).
- Retirada: implementação local substituída pelo comportamento oficial equivalente;
  teste de regressão mantido e registro histórico preservado.

## Revisão da base oficial 0.7.3 — 2026-11-17

Comparação: `9337afc47e10b86cc896a6f432240e40fee95dd1` →
`80d7c72c0727e091425d10f5654b7ccf07e01abe`. Candidato `0.7.3-cesar.1`.
O [guia de atualização](UPGRADE-0.7.3.md) registra dependência, contrato blocklist,
schema v16 e condições de implantação/reversão. Na integração inicial, nada foi
implantado ou publicado como release; a branch da biblioteca foi publicada para
resolver seu SHA. Posteriormente, em 2026-11-17, a release `0.7.3-cesar.1` foi
publicada com [CI, imagem e downloads anônimos verificados](releases/v0.7.3-cesar.1.json).
Depois da publicação, a versão foi [implantada em produção](deployments/2026-11-17-0.7.3-cesar.1.md)
com backup restaurado em teste, configurações preservadas e recuperação das 21
sessões anteriormente ativas. O registro da release conserva o retrato da
publicação; o registro de implantação documenta esta etapa posterior.

Todos os testes existentes do fork foram preservados. A comparação dos caminhos
alterados pela release e a suíte completa sustentam a seguinte decisão por item:

| Item | Estado nesta base | Decisão/evidência |
|---|---|---|
| F-001 | `local` | Avatar, prazos, erros e testes preservados; oficial só adapta outras APIs de usuário. |
| F-002 | `local` | PictureURL e enriquecimento limitado preservados; testes de queries passam. |
| F-003 | `local` | Preparação de edições e flags preservadas; fixtures da nova biblioteca passam. |
| F-004 | `local` | Escritas WS e limpeza por identidade preservadas; suíte e race passam. |
| F-005 | `local` | Settings parciais e isolamento preservados; testes HTTP/SQL passam. |
| F-006 | `local` | Menções em wrappers/mídia preservadas; testes de envio passam. |
| F-007 | `local` | Middleware de arrays permanece; sem retirada por ausência de nova solução oficial. |
| F-008 | `local` | Sanitização e configuração de logs mantidas; regressões passam. |
| F-009 | `exclusivo do fork` | Compose preservado; referência padrão avança ao candidato, sem iniciar stack. |
| F-010 | `exclusivo do fork` | E2E opcional preservado, não executado (envia mensagens reais). |
| F-011 | `exclusivo do fork` | Rules/CI preservadas; Swagger regenerado pelo make, sem edição manual. |
| F-012 | `exclusivo do fork` | Nova base/VERSION rastreáveis; guards de publicação preservados. |
| F-013 | `local` | Nós interativos, precedência e wrappers continuam no fork Whatsmeow; testes/race passam. |
| F-014 | `local` | Parsing de respostas e preservação de eventos mantidos; regressões passam. |
| F-015 | `local` | EXIF/dimensões/limites mantidos; regressões de mídia passam. |
| F-016 | `local` | Preview limitado, cancelamento, rede e metadados preservados; testes passam. |
| F-017 | `local` | Overlay touch preservado; Chromium cobre fixture e bundle real. |
| F-018 | `local` | Sender e liberação de assets na licença preservados; HTTP, stores e navegador passam. |
| F-019 | `parcialmente coberto pelo upstream` | Pool/retry oficiais sobrepostos; mantemos store por serviço, WAL, cancelamento e fechamento. SQLite/PostgreSQL e race passam. |
| F-020 | `local` | Lifecycle e workers preservados integralmente; regressões concorrentes passam. |
| F-021 | `local` | Descarte após logout remoto e favicon preservados; testes passam. |
| F-022 | `parcialmente coberto pelo upstream` | Revisão padrão atualizada; preservamos configuração/fetch e sincronização do handshake. |
| F-023 | `local` | Serialização do evento Archive preservada; regressão passa. |
| F-024 | `exclusivo do fork` | Monitor mantido; 11 testes passam sem SMTP/API reais. |
| F-025 | `substituído pelo upstream` | ACK equivalente na nova biblioteca; regressão preservada e mutação sem ACK detectada. |

A atualização HTTP da blocklist é intencional e documentada: `Items` substitui
`JIDs`; `AddressingMode` é adicionado. O teste HTTP verifica os três endpoints,
identidades PN/LID, Active e o envelope existente. Anotações e Swagger acompanham
o contrato. Não altera os patches F-001/F-002.

Validação local em Docker Linux, Go 1.26.10, CGO habilitado e dependências nativas:
build/vet/testes/race completos da aplicação e biblioteca; `go mod verify`;
SQLite temporário e PostgreSQL 16 descartável; 12 guards de release; 11 testes
do monitor; cinco regressões Playwright 1.62.1/Chromium; `make swagger` com swag
v1.16.3; actionlint 1.7.9; formatação dos Go alterados e `git diff --check`.
Os resultados locais não substituem os checks do commit final no GitHub.
A implantação e a reconexão das sessões foram verificadas posteriormente no
registro acima. Homologação de envio interativo, novo pareamento e passkey
continua pendente; saúde e reconexão não comprovam esses fluxos.

Auditoria adicional em 2026-11-17: as sete branches de contribuição foram
comparadas com a main `98bcc1f` e o PR #17 `2cf0b13`. Nenhum complemento de
F-001–F-006 ou F-019/F-020 ficou ausente; implementações, pontos de integração
e regressões estão registrados em [BRANCH-AUDIT-0.7.3.md](BRANCH-AUDIT-0.7.3.md).
Build/vet completos e testes/race com `-count=1` passaram novamente em Linux,
incluindo PostgreSQL 16 descartável e SQLite temporário. Nenhuma mudança de
código ou merge adicional foi necessário.


## F-026 — Validação e falhas de mídia no carrossel

- Estado: `local`; revisão em 2026-11-17 sobre main `8e3bff1`, base oficial 0.7.3.
- Implementação: `8421648`, [PR #19](https://github.com/cesar-carlos/evolution-go/pull/19), integrado em `19816da`; release `0.7.3-cesar.2`.
- Problema: falhas HTTP/upload ignoradas, leitura sem limite, tipos desconhecidos
  convertidos em REPLY, campos URL/CALL ignorados e panic no corpo JSON nulo.
- Correção: preparação cancelável e limitada, aborta no erro de mídia; validação
  antes de operações externas; preserva id e aceita url/phoneNumber/COPY_CODE,
  rejeitando conflitos. Mantém metadados e repassa formatJid/quoted. A abertura
  de sessão também respeita o contexto da requisição.
- Referência: [issue #51](https://github.com/evolution-foundation/evolution-go/issues/51).
- Evidências: `carousel_test.go`, `interactive_validation_test.go`; cenários de
  erro, cancelamento, contratos antigos/aliases e preservação do payload.
- Contrato/migração/limitações: [CAROUSEL-NATS.md](CAROUSEL-NATS.md).
  Sem homologação em aparelho; publicação registrada abaixo.
- Retirada: upstream equivalente em validação, recursos e serialização, comprovado
  pelas regressões e homologação real de envio/cliques.

## F-027 — BUTTON_CLICK no NATS global

- Estado: `local`; revisão em 2026-11-17 sobre main `8e3bff1`, base oficial 0.7.3.
- Implementação: `8421648`, [PR #19](https://github.com/cesar-carlos/evolution-go/pull/19), integrado em `19816da`; release `0.7.3-cesar.2`.
- Problema: assinatura BUTTON_CLICK aceita, mas sem mapeamento no dispatch global.
- Correção: ButtonClick mapeia para BUTTON_CLICK, publica no subject buttonclick
  apenas quando habilitado e assinado; preserva payload e identificação.
- Referência: [issue #193](https://github.com/evolution-foundation/evolution-go/issues/193).
  Esta tarefa corrige BUTTON_CLICK, não os outros eventos mencionados na issue.
- Evidências: `TestGlobalNATSButtonClick` verifica o dispatch real com produtor
  falso, inclusive desabilitado, sem assinatura e falha sem duplicar tentativas.
- Limitações: entrega best effort existente; não comprova broker real nem clique
  em aparelho. [Contrato](CAROUSEL-NATS.md).
- Retirada: upstream inclui o mapeamento e passa as regressões de dispatch.

Validação conjunta de F-026/F-027: build/vet/testes/race completos da aplicação
e testes/race da biblioteca fixada aprovados em Docker Linux/Go 1.26.10, com
PostgreSQL 16 descartável e SQLite temporário. Swagger regenerado por make
(swag v1.16.3), links e diff conferidos; detalhes em [CAROUSEL-NATS.md](CAROUSEL-NATS.md).

## Release 0.7.3-cesar.2

Preserva a base oficial 0.7.3, a dependência Whatsmeow e todos os patches
anteriores; adiciona F-026/F-027. Não há novas dependências, variáveis ou migrações
de banco em relação a 0.7.3-cesar.1. A validação do carrossel é mais estrita;
consulte [contrato e migração](CAROUSEL-NATS.md). Publicação e implantação devem
ser verificadas e registradas separadamente pelo digest.

Publicação 0.7.3-cesar.2 verificada em 2026-11-17: [evidência da imagem](releases/v0.7.3-cesar.2.json). Workflow aprovado e pulls anônimos amd64/arm64 confirmados pelo digest.

## F-028 — Metadados de transporte de listas e carrosséis

- Estado: `local`; revisão em 2026-11-18 sobre main `e06d2fb`, base oficial 0.7.3.
- Implementação: `cb64934`, PR #22 integrado em `802887e`; release `0.7.3-cesar.3`. go.mod permanece
  fixado em Whatsmeow `3e04a7b2fcdf`, preservando os patches anteriores.
- Problema: lista rejeitada pelo WhatsApp com 405 (API HTTP 500); carrossel
  ausente no celular apesar de cards visíveis no Desktop. A biblioteca omite
  biz para CarouselMessage e usa `single_select` no nó de roteamento de listas.
- Correção: no pipeline de envio, AdditionalNodes fornece biz/list product_list
  v2 para SINGLE_SELECT e biz/interactive native_flow v1/mixed v9 para carrossel.
  Inclui metadados de negócio, preserva overrides biz/hsm e nós adicionais.
  Não altera protobuf, IDs/contexto, mídia, botões/PIX ou parsing de respostas.
- Referências: [Evolution Go #204](https://github.com/evolution-foundation/evolution-go/issues/204),
  [Whatsmeow #1146](https://github.com/tulir/whatsmeow/pull/1146),
  [Whatsmeow #1235](https://github.com/tulir/whatsmeow/pull/1235).
- Evidências: `interactive_nodes_test.go` cobre serialização binária dos nós,
  IDs e payload inalterados, precedência, preservação de metadados sem mutação
  do chamador e exclusão de respostas/outros tipos; suítes existentes preservadas.
- Homologação em 2026-11-18 na 0.7.3-cesar.3: lista com uma opção e carrossel
  com dois cards confirmados no celular pelo usuário; ambos os envios HTTP 200,
  sem erro 405. A API recebeu três ButtonClick com os IDs esperados:
  um `list_response` e dois `template_button_reply`.
- Limitações: homologação restrita a esse cenário; aparelho/SO/versão do cliente
  não informados. NATS desabilitado, sem validação de entrega ao broker ou de
  consumidores externos. Contrato HTTP/env inalterado. Este registro documental
  não altera produção nem executa novos envios.
- Retirada: biblioteca fixada fornece os mesmos nós para listas/carrosséis,
  preserva precedência/respostas e passa regressões e homologação em aparelhos.
- Detalhes: [compatibilidade interativa](INTERACTIVE-COMPATIBILITY.md).

Validação local de F-028 em 2026-11-18: Docker Linux/Go 1.26.10, CGO,
dependências nativas de mídia, PostgreSQL 16 descartável e SQLite temporário.
`go build ./...`, `go vet ./...`, `go test -count=1 -timeout 5m ./...` e
`go test -race -count=1 -timeout 5m ./...` aprovados. Testes normais e com race
de `go.mau.fi/whatsmeow` também aprovados. Formatação restrita aos Go alterados,
links locais e `git diff --check` conferidos. Estes resultados são locais;
os checks de CI devem ser consultados no PR da branch de implementação.
Swagger não regenerado porque o contrato HTTP permanece inalterado.

## Release 0.7.3-cesar.3

Release com F-028 e todos os patches anteriores, sobre a base oficial 0.7.3.
Sem mudanças de dependências, banco, configuração ou contrato HTTP em relação
a 0.7.3-cesar.2. Metadados de transporte corrigidos; lista com uma opção e
carrossel com dois cards homologados no celular em 2026-11-18, com as três
respostas recebidas pela API. Escopo e limitações no registro de F-028 acima.
Publicação, digest e implantação são verificados separadamente.

Publicação 0.7.3-cesar.3 verificada em 2026-11-18: [evidência da imagem](releases/v0.7.3-cesar.3.json). Workflow aprovado e pulls anônimos amd64/arm64 confirmados pelo digest.
