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

`ButtonClick` preserva os campos existentes e reconhece respostas encapsuladas,
incluindo `selected_row_id` no JSON de native-flow. Os nós de envio e o parser de
recebimento são testados separadamente. CTA de URL/telefone pode abrir uma ação
sem produzir uma resposta de clique: não inferir confirmação onde ela não existe.

## Homologação pendente

Testes locais não comprovam comportamento dos servidores/clientes WhatsApp.
Registrar versão/digest, cliente/dispositivo, destino individual/grupo, payload
anonimizado, renderização, ID da seleção e evento para Android, iOS e Web.
Cobrir reply, CTA, listas multisseção, carrosséis, menções/citações e mídia.
Inspecionar PIX sem efetuar pagamento. Não enviar mensagens reais na CI.

Até essa homologação, as mudanças de protocolo são compatibilidade experimental.
