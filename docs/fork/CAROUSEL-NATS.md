# Carrossel e cliques no NATS global

Correções integradas pelo [PR #19](https://github.com/cesar-carlos/evolution-go/pull/19),
incluídas na release `0.7.3-cesar.2`. A publicação não implica implantação;
consulte os registros de releases e de produção.

Investigação posterior em 2026-11-18: a validação de payload/mídia de F-026
não corrigia a ausência de metadados biz no transporte do carrossel. F-028,
incluído na release 0.7.3-cesar.3, acrescenta esses nós e ajusta o roteamento de
listas rejeitadas com 405. Veja [compatibilidade interativa](INTERACTIVE-COMPATIBILITY.md).
Em 2026-11-18, a lista com uma opção e o carrossel com dois cards foram
confirmados no celular na 0.7.3-cesar.3, com as três respostas recebidas pela API.
O [registro de homologação](INTERACTIVE-COMPATIBILITY.md#homologação-móvel-de-f-028--2026-11-18)
delimita esse cenário; NATS estava desabilitado e não foi homologado.

## POST /send/carousel

O contrato anterior continua aceitando `id` como destino de botões URL/CALL.
Também são aceitos `url` para URL e `phoneNumber` para CALL. Se o alias e `id`
forem enviados juntos, devem ser iguais. `COPY_CODE` é alias de `COPY` e usa
`copyCode`. O tipo é insensível a maiúsculas; omitido significa `REPLY`.

Exemplo de card:

```json
{
  "number": "5511999999999",
  "body": "Confira as opções",
  "cards": [{
    "header": {"imageUrl": "https://example.org/produto.jpg"},
    "body": {"text": "Produto"},
    "buttons": [{"type": "URL", "displayText": "Ver produto", "url": "https://example.org/produto"}]
  }]
}
```

Validação retorna **400 antes de conectar ou baixar mídia** para corpo nulo,
número ausente, lista de cards vazia, `body.text` vazio, imagem e vídeo simultâneos,
URL de mídia inválida, tipo de botão desconhecido/PIX, rótulo ou destino vazio,
destinos conflitantes e mistura de REPLY com CTA no mesmo card. URL deve ser
HTTP(S) absoluta, sem credenciais. Cards sem mídia continuam permitidos.

Migração: clientes que dependiam da conversão silenciosa de tipos desconhecidos
para REPLY devem corrigir o tipo. Separe REPLY e CTA em cards distintos. Corrija
campos vazios e escolha apenas uma mídia por card. Payloads válidos no contrato
anterior mantêm seus campos; não é necessário trocar `id` pelo alias.

Downloads e uploads usam o contexto da requisição e prazo conjunto de 30 segundos
por mídia. A preparação tem prazo total de 2 minutos e cada download é limitado
a 32 MiB. HTTP não-2xx, leitura incompleta, limite excedido ou falha de upload
interrompem a preparação: nenhum carrossel parcial é enviado. O handler mantém
o erro operacional em **500**. Uploads já concluídos antes da falha de outro card
não são revertidos. O envio final usa o pipeline compartilhado existente com o mesmo contexto,
incluindo cancelamento da espera e da chamada ao WhatsApp. Um timeout após
transmissão não prova que a mensagem não foi recebida; não há retry automático.

Título, subtítulo, miniatura, dimensões/EXIF, texto principal e footer são
preservados. `formatJid` e `quoted` agora são repassados ao pipeline compartilhado.

## BUTTON_CLICK no NATS global

Com `NATS_GLOBAL_ENABLED=true` e `BUTTON_CLICK` em `NATS_GLOBAL_EVENTS`, o evento
`ButtonClick` é encaminhado ao subject **`buttonclick`**, mantendo payload e
identidade da instância. Desabilitado ou sem essa assinatura, não é encaminhado.
Não há novas variáveis nem mudança de nomes de eventos. Mantém-se a entrega
best effort do NATS existente, sem garantia de persistência ou retry adicional.

## Validação

Regressões cobrem aliases/legado, escaping JSON, validação HTTP antes do serviço,
falhas HTTP/upload, tamanho, cancelamento na abertura da sessão e em
download/upload, preservação de
metadados e encaminhamento NATS com assinatura, desabilitado e falha do produtor.
Testes usam HTTP local e produtor falso; não usam WhatsApp, NATS real ou produção.
A homologação móvel de F-028 acima cobre um cenário; os demais clientes e
combinações em Android, iOS e Web continuam dependendo de homologação específica.

Referências: [issue #51](https://github.com/evolution-foundation/evolution-go/issues/51),
[issue #193](https://github.com/evolution-foundation/evolution-go/issues/193).

Validação local concluída em 2026-11-17, Docker Linux/Go 1.26.10, CGO,
dependências nativas de mídia, PostgreSQL 16 descartável e SQLite temporário:

- `go build ./...` e `go vet ./...`: aprovados.
- `go test -count=1 -timeout 5m ./...`: aprovado.
- `go test -race -count=1 -timeout 5m ./...`: aprovado.
- Testes normais e com race de `go.mau.fi/whatsmeow`: aprovados.
- `make swagger` com swag v1.16.3, links locais e contrato gerado: conferidos.
- `git diff --check`: aprovado; formatação limitada aos Go alterados.

Isso é validação local, não execução de CI nem homologação de WhatsApp real.
