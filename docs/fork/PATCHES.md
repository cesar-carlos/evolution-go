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

- Estado: `local`; armazenamento integrado pelo PR #12, commit `0a50034`.
- Validação: CI [37974834132](https://github.com/cesar-carlos/evolution-go/actions/runs/37974834132) aprovada (Go/race/PostgreSQL e navegador).
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

- Estado: `local`; commit `68f4725`, integrado pelo PR #13.
- CI: [37978099885](https://github.com/cesar-carlos/evolution-go/actions/runs/37978099885) aprovada (Go/PostgreSQL/race e navegador).
- Comparação: 2026-10-09, base oficial `0.7.2`; issue #186 e revisão do #200.
  #200/#131 são referências de defeitos evitados, não merges incorporados.
- Problema/esperado: mapas/canais compartilhados permitiam corridas, workers órfãos
  e reconexão após parada intencional. Serializar operações por instância e impedir
  callbacks/limpeza antigos de afetar uma substituta.
- Implementação: registro privado, identidade por execução, snapshots de settings,
  cancelamento difundido, conclusão explícita e reconexões coalescidas/limitadas.
  Transporte permanece vivo para logout depois de parar os workers. QR/passkey têm
  sinal de disponibilidade; polling não cria ciclos de QR após expiração.
  DeviceProps é clonado por cliente, evitando mutação global entre instâncias.
  Atualizações de settings e respostas/confirmações HTTP de passkey pertencem à
  execução; o token da cerimônia é revalidado após vincular a operação. Elas são canceladas e
  aguardadas na parada; a limpeza remove a cerimônia antiga antes de outra execução.
- Evidências: session_lifecycle_test.go cobre 32 inícios/paradas, isolamento,
  workers, substituição, exclusão durante startup, cinco retries, cancelamento,
  shutdown, espera de pareamento e ordem worker/logout/transporte.
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

### Candidata 0.7.2-cesar.4

Inclui F-019 (storage) e F-020 (ciclo de vida), mantendo todos os patches anteriores
e a base oficial 0.7.2. Candidata preparada em branch própria, sem movimentar tags
publicadas. Publicação, digest e download público só serão registrados após os
checks e o workflow da tag concluírem. Nenhuma implantação em produção autorizada
por esta publicação. Reversão prevista para o digest de 0.7.2-cesar.3, após conferir
compatibilidade dos bancos/sessões; não foram adicionadas migrações de aplicação
nem atualizadas dependências nesta entrega.
