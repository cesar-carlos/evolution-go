# Contribuições da revisão de mensagens interativas

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

Não foram enviados novos comentários nem alteradas branches de terceiros nesta
entrega. Os links e arquivos acima tornam as propostas revisáveis; publicação de
contribuições aos autores é uma etapa própria, após autorização de envio.
