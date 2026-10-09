# Contribuições e revisões upstream

Revisão dos heads de 2026-10-09. O código integrado e as condições de retirada
continuam no [inventário](PATCHES.md); este arquivo registra os complementos
que podem ser propostos aos PRs de origem. Uma contribuição precisa ser adaptada
à base do PR escolhido e validada nela antes de ser enviada. Commits completos
do fork incluem diferenças que não pertencem necessariamente ao PR original.

## Evolution Go

| PR / head comparado | Complemento implementado no fork | Regressões para acompanhar a contribuição |
|---|---|---|
| [#212](https://github.com/evolution-foundation/evolution-go/pull/212), `ae69dac8108f378fa269cb30e15f482e83198134` | Validar tipo SHORT e quantidade 1 na orientação EXIF; limitar pixels antes da decodificação; manter todas as orientações e byte orders. F-015. | `TestEXIFTagValidation`, `TestEveryEXIFTransform`, `TestOversizedBitmapRejectedBeforeDecode`, seeds de `FuzzEXIFParsing`. |
| [#207](https://github.com/evolution-foundation/evolution-go/pull/207), `72ee1b317048bb3566f4ee60f89604d6e20e66aa` | Resolver URLs relativas pela URL final, propagar cancelamento HTTP/upload, preservar EXIF e restringir destinos. F-016. | `TestPreviewRelativeImageAfterRedirect`, `TestPreviewCancellationAndNetworkPolicy`, `TestPreviewEXIFAndCancelledSend`, `TestPreviewCancellationDuringUpload`, `TestPreviewRejectsCredentialRedirect`. |
| [#184](https://github.com/evolution-foundation/evolution-go/pull/184), `b959d3b255a5a224ccff263f0a5d8e65041a9ef7` | Overlay com ciclo de vida, listeners removíveis, foco, teclado e restauração de scroll; ações visíveis em tablets touch. F-017. | `tests/ui/manager.test.cjs`: remontagem/resize/foco, tablet e bundle real com REST simulado. |
| [#182](https://github.com/evolution-foundation/evolution-go/pull/182), `caf4b6c7dcab6f9d24d70f25f1f66811dd53a608` | Sender com autenticação por instância, credenciais em memória, stores PostgreSQL/SQLite existentes e recursos limitados. F-018. | `pkg/sender/handler`, `pkg/sender/service`, `tests/ui/sender.test.cjs`. |

O #207 já implementa limites de bytes, pixels e timeout, respeito aos metadados do
chamador, fallback inline para newsletters e preparação do preview uma vez. Esses
itens são preservados; não devem ser apresentados como contribuições novas.

O #184 mistura a correção visual com um Dockerfile que busca outra árvore Git e
aplica patches adicionais. A proposta visual deve usar os assets legíveis e os
testes do fork, preservando o build normal da árvore revisada.

No #182, qualquer relato relacionado a credenciais deve seguir o canal privado
definido na [rule de contribuição](../../.cursor/rules/contributing.mdc).
Não incluir credenciais, logs reais, DSNs ou dados de usuários em exemplos.

Os PRs #120, #121, #122, #135, #136 e #137 já receberam a revisão anterior nas
respectivas branches. Recomparar seus heads antes de propor outro complemento.

## Whatsmeow

| PR / head comparado | Complemento implementado no fork da biblioteca |
|---|---|
| [#1235](https://github.com/tulir/whatsmeow/pull/1235), `9a4bd9bb15d5cdfe3b8529905d742aa68336f186` | Desembrulhar payloads de forma limitada e nil-safe, incluindo ViewOnceMessageV2Extension; impedir duplicação de biz/hsm e preservar nós adicionais não relacionados. |
| [#1221](https://github.com/tulir/whatsmeow/pull/1221), `ebdf9dcfe3fe60608890c0652dc68f221ee317a2` | Cobrir InteractiveResponseMessage e TemplateButtonReplyMessage, inclusive encapsulados e com nós adicionais explícitos. |
| [#1146](https://github.com/tulir/whatsmeow/pull/1146), `0d52107ba61d27f2e16e7d6d0b13a3ba1b0de812` | Preservar o tipo protobuf da lista nos atributos biz; manter alterações de mediatype para grupos e novos tipos de fluxo em propostas separadas. |

O código da biblioteca está fixado pelo `replace` de `go.mod`, com origem e
licença preservadas em `FORK-MAINTENANCE.md`. Regressões adicionais:
`TestWrappedListPreservesSelectionContract`,
`TestMalformedInteractiveAttributesAreBounded`,
`TestResponseNodesAndExplicitOverrides`.

## Evidências e limites

Na revisão, funções extraídas dos heads oficiais reproduziram: perda de nó para
ViewOnceMessageV2Extension, panic com wrapper vazio, EXIF de tipo inválido aceito e
URL relativa resolvida antes do redirect. No navegador, o overlay do #184 manteve
ações invisíveis em tablet, scroll bloqueado após resize e dois backdrops após
remontagem; o overlay do fork passou nos mesmos cenários.

A regressão da biblioteca falha no código anterior do fork porque SINGLE_SELECT
era anunciado como product_list. A correção preserva o tipo do payload. Esse
teste comprova consistência interna e preservação dos IDs/contexto; não comprova
aceitação do servidor nem seleção real. A homologação descrita em
[INTERACTIVE-COMPATIBILITY.md](INTERACTIVE-COMPATIBILITY.md) permanece necessária.

Na entrega de mensagens interativas não foram enviados novos comentários nem
alteradas branches de terceiros. Os links e arquivos acima tornam as propostas revisáveis; publicação de
contribuições aos autores é uma etapa própria, após autorização de envio.

## Armazenamento de sessões — revisão em 2026-10-09

Estado inicial: **revisados, com implementação pendente**. Referência principal:
[#117](https://github.com/evolution-foundation/evolution-go/pull/117).
A integração será seletiva, não um merge integral das alternativas abaixo.

| PR | Head comparado | Aproveitamento / defeito evitado |
|---|---|---|
| #117 | `03289559d547911d92ad58837db98faeb0c5fd8e` | Retry após falha, compartilhamento e regressão; evitar pool PostgreSQL duplicado e globais mutáveis. |
| #194 | `411a2f19308c7708a14682cc10c4a7de7ecd9361` | Compartilhamento; adicionar limites pelo authDB existente. |
| #206 | `f2a73c715a286aefa5c10fa8521dccc7efa56581` | Reutilização de authDB; rejeitar falha permanente via sync.Once. |
| #174 | `33b6d186c27b05ece57abe76d9640318f647c7f9` | Referência para reutilização do pool. |
| #178 | `ae4f3ecfa64469bfab3abb4a9b869a331574c725` | Referência para reutilização do pool; evitar Upgrade a cada início e fallback que reabre pools. |
| #102 | `a54c075c39e53b96849c041eee80e4b68b25cee2` | Origem da proposta de pool compartilhado; rejeitar cache permanente de erros. |
| #131 | `f5892fdb14cb1f631f414a708b82685da8527d6e` | Revisado como alternativa; corrida entre leitura fora do mutex e escrita reproduzida com -race. |
| #200 | `58cf23ab5e27c8f70757ab7a56f52bd9efa83e41` | Revisado como alternativa; não transportar mapa concorrente sem proteção nem fechamento antes de parar consumidores. |

Heads serão revalidados antes da contribuição ao #117. A revisão isolada reproduziu
falha permanente no #206 e corrida no #131. Os helpers de #117/#194 recuperaram após
falha e compartilharam o container em 32 chamadas. Isso não valida o ciclo completo,
WhatsApp real, nem implica aprovação do mantenedor.
### Integração no fork

F-019 foi integrado seletivamente pelo [PR #12](https://github.com/cesar-carlos/evolution-go/pull/12),
commit `0a50034`, com CI e PostgreSQL real aprovados. Foram aproveitados os comportamentos
de recuperação/compartilhamento do #117/#194 e reutilização do handle do #174/#178/#206;
nenhum desses PRs foi mesclado integralmente. #102/#131/#200 permanecem referências
revisadas, sem incorporar suas implementações. F-020 trata o ciclo de vida separadamente.

A integração de testes revelou que a dependência fixada consulta nomes de tabelas
em todos os schemas durante Upgrade. O fixture PostgreSQL passou a criar um banco
temporário próprio (role de testes com CREATEDB), evitando interferência com Sender.
Isso não altera o schema de produção nem atualiza dependências.

A adaptação ao head do #117 está no [patch 5bb65ac](https://github.com/cesar-carlos/evolution-go/commit/5bb65ac),
branch `codex/pr117-shared-auth-store`, baseada em `03289559d547911d92ad58837db98faeb0c5fd8e`.
Preserva módulo `github.com/EvolutionAPI/evolution-go`, go.mod/go.sum e submódulo
`0923702fb3fac8525241f15331b92116485d69eb`. Na própria base passaram build, vet,
testes completos e race em Docker Go 1.25, incluindo PostgreSQL 16 e SQLite reais.
F-020 foi integrado separadamente pelos PRs #13/#15 com CI aprovada; não faz parte desse
patch. A contribuição foi publicada no [comentário do #117](https://github.com/evolution-foundation/evolution-go/pull/117#issuecomment-6088147609),
com diagnóstico, créditos, comandos reproduzíveis e link para o patch. A release
[0.7.2-cesar.4](https://github.com/cesar-carlos/evolution-go/releases/tag/v0.7.2-cesar.4)
e sua evidência verificam a distribuição integrada. Não foi aberto PR upstream duplicado
nem alterada a branch do autor.
Os autores das propostas recebem crédito nas notas do patch; não houve merge
integral nem declaração de aprovação do mantenedor.
