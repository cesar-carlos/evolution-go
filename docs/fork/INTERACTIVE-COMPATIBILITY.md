# Compatibilidade de mensagens interativas

O fork adapta as propostas whatsmeow #1235, #1221 e #1146 sobre a dependência já
usada, sem alterações incidentais de criptografia. Consulte F-013/F-014 no
[inventário](PATCHES.md) e a [política de manutenção](MAINTENANCE.md).

`POST /send/button` conserva os campos e regras de combinação existentes e envia
reply como native-flow `quick_reply`. Não há wrapper artificial de documento.
O nó de negócio é construído na biblioteca e respostas não recebem esse nó.

`POST /send/list` conserva SINGLE_SELECT. IDs explícitos são preservados e precisam
ser únicos em toda a mensagem; duplicação retorna 400. Para migrar clientes que
usavam IDs repetidos, atribua um ID por opção e ajuste o roteamento da resposta.
IDs omitidos são gerados sem colisão entre seções nem com IDs explícitos.
`buttonText` omitido/vazio usa "Ver Menu". Seções precisam conter opções.

Complemento local F-028 (2026-11-18, incluído na release 0.7.3-cesar.3): o envio de menus
SINGLE_SELECT fornece `biz/list type="product_list" v="2"` por `AdditionalNodes`.
O protobuf continua SINGLE_SELECT, com os mesmos IDs e contexto. O tipo de
roteamento do nó não é necessariamente o enum do payload. A correspondência
anterior com `single_select` foi uma hipótese de consistência interna; após
rejeição remota 405, retomamos o formato relatado como funcional na
[issue #204](https://github.com/evolution-foundation/evolution-go/issues/204).
O 405 vem do WhatsApp; a API o expõe como erro operacional HTTP 500.
Não se trata de método HTTP incorreto nem de motivo para repetir o envio.

Para carrossel, F-028 fornece `biz/interactive type="native_flow" v="1"` com
`native_flow name="mixed" v="9"` e metadados de negócio equivalentes aos botões.
A biblioteca fixada reconhecia somente native-flow direto, omitindo esse nó
para CarouselMessage. A extensão é limitada aos prompts diretos dos builders;
não muda botões, PIX, respostas ou wrappers. Metadados explícitos biz/hsm têm
precedência e os demais nós adicionais são preservados. A biblioteca continua
responsável pela composição final e evita duplicação de biz.

Na versão 0.7.3-cesar.2, o teste anterior
apresentou lista rejeitada e carrossel ausente no celular, com cards visíveis
no Desktop. A homologação abaixo registra o resultado após F-028 na 0.7.3-cesar.3.

`ButtonClick` preserva os campos existentes e reconhece respostas encapsuladas,
incluindo `selected_row_id` no JSON de native-flow. Os nós de envio e o parser de
recebimento são testados separadamente. CTA de URL/telefone pode abrir uma ação
sem produzir uma resposta de clique: não inferir confirmação onde ela não existe.

## Homologação móvel de F-028 — 2026-11-18

Teste autorizado em conversa individual com a release **0.7.3-cesar.3**, commit
`d8b33f35867a10b7041c5cd5a9ba79e071f57c72`, imagem pelo digest
`sha256:1e974977e6d690b23e386836ac7e26d41db35e6bcb0b56766cd72e685b505125`.
Ambos os envios retornaram **HTTP 200**, sem o erro 405 da lista.
O usuário confirmou no celular a lista com uma opção e o carrossel com dois cards.
A API registrou os três eventos ButtonClick, com os IDs esperados de cada seleção:

| Opção do teste | Resposta recebida pela API |
|---|---|
| Confirmar lista | `list_response` |
| Confirmar card 1 | `template_button_reply` |
| Confirmar card 2 | `template_button_reply` |

Resultado: exibição no celular e retorno das três interações homologados **nesse
cenário**. Modelo do aparelho, sistema operacional e versão do cliente não foram
informados; não extrapolar para todos os clientes Android/iOS/Web, grupos,
listas multisseção ou outros tipos de mídia/botões. A confirmação não atesta
entrega a consumidores externos: NATS estava desabilitado, e não houve validação
de entrega pelo broker nem confirmação correlacionada do consumidor de webhook.
Identificadores da instância, telefone, tokens e logs brutos não fazem parte deste registro.

A [evidência da publicação](releases/v0.7.3-cesar.3.json) é o registro anterior
à homologação e permanece inalterada; este registro acrescenta o resultado funcional.

## Homologações ainda pendentes

Previews respeitam campos explícitos e permitem texto quando a página/imagem/upload
falha. Cancelar a requisição interrompe o trabalho; falha de preview não repete o
envio. O orçamento total do link é 45 s, com até 20 s para preparar preview, limites
de 4 MiB de HTML, 8 MiB de imagem e 25 milhões de pixels antes da decodificação.
Destinos privados/locais são bloqueados inclusive após redirects e DNS; instalações
que precisam deles devem configurar `LINK_PREVIEW_ALLOW_PRIVATE=true` conscientemente.
URLs com credenciais e esquemas diferentes de HTTP(S) não são usadas para preview.
Newsletters conservam miniatura inline, sem upload criptografado de HQ.

Testes locais não comprovam comportamento dos servidores/clientes WhatsApp.
Registrar versão/digest, cliente/dispositivo, destino individual/grupo, payload
anonimizado, renderização, ID da seleção e evento para Android, iOS e Web.
Cobrir reply, CTA, listas multisseção, carrosséis, menções/citações e mídia.
Inspecionar PIX sem efetuar pagamento. Não enviar mensagens reais na CI.

A validação móvel acima cobre somente o cenário descrito. As demais combinações
de protocolo continuam experimentais até homologação específica.

Os complementos preparados para os PRs de origem estão em
[UPSTREAM-CONTRIBUTIONS.md](UPSTREAM-CONTRIBUTIONS.md).
