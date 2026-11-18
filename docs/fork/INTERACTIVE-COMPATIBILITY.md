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

Complemento local F-028 (2026-11-18, ainda sem release): o envio de menus
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

Essas alterações corrigem divergências locais de transporte; não comprovam
aceitação remota nem renderização móvel. A versão publicada 0.7.3-cesar.2 ainda
apresentou lista rejeitada e carrossel ausente no celular, com cards visíveis
no Desktop. Cliente/SO móvel e comportamento de navegação não foram confirmados.
Registrar renderização, seleção e resposta após homologar a futura versão.

`ButtonClick` preserva os campos existentes e reconhece respostas encapsuladas,
incluindo `selected_row_id` no JSON de native-flow. Os nós de envio e o parser de
recebimento são testados separadamente. CTA de URL/telefone pode abrir uma ação
sem produzir uma resposta de clique: não inferir confirmação onde ela não existe.

## Homologação pendente

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

Até essa homologação, as mudanças de protocolo são compatibilidade experimental.

Os complementos preparados para os PRs de origem estão em
[UPSTREAM-CONTRIBUTIONS.md](UPSTREAM-CONTRIBUTIONS.md).
