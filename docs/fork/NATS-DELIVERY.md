# Entrega e recuperação do producer NATS

F-029, incluída na versão 0.7.3-cesar.4 em 2026-11-18.
Preserva F-027 (BUTTON_CLICK global), F-028 e os contratos de payload,
subject e isolamento entre instâncias. Não habilita NATS automaticamente.

## Configuração

- `NATS_URL`: endereço alcançável pelo container. Vazio não tenta conectar,
  inclusive a localhost. A conexão é criada na inicialização da API.
- Por instância: `natsEnable: "enabled"` ou `"true"` em `POST /instance/connect`.
  O clique usa `<instanceId>.buttonclick`. As assinaturas `MESSAGE` ou
  `BUTTON_CLICK` permitem esse evento. Omitir `subscribe` preserva as assinaturas
  existentes; enviar uma lista substitui o conjunto, afetando também outros canais.
- Global: `NATS_GLOBAL_ENABLED=true` e `NATS_GLOBAL_EVENTS=BUTTON_CLICK`, sem
  espaços. O subject é `buttonclick` e reúne cliques das instâncias. Os modos
  global e por instância podem publicar o mesmo evento em subjects distintos.
- Outros modos por instância, incluindo vazio, `false` e `disabled`, não publicam.
  `NATS_ENABLED` e `natsEnabled` são exemplos antigos incorretos.

O broker não é provisionado pela aplicação. Configure autenticação, TLS conforme
a rede e permissões de publicação/assinatura. O payload preservado contém
`instanceToken`; restrinja os consumidores e não registre payloads completos.

## Falhas e recuperação

Com URL configurada, nats.go tenta recuperar falhas de conexão inicial e quedas
posteriores: até 60 tentativas por servidor, intervalo base de 2 segundos mais
jitter da biblioteca, timeout de conexão de 2 segundos. Configuração inválida
pode impedir a criação da conexão; falhas de autenticação podem encerrar as
tentativas antes do limite. Após esgotamento, é necessário corrigir a causa e
reiniciar a API. Não há retry ilimitado ou garantia de recuperação de toda falha.
O encerramento da API fecha a conexão e interrompe reconexões.

Um envio habilitado sem conexão ativa retorna erro, não sucesso. `true` e
`enabled` são equivalentes. Não se armazenam novos envios em buffer durante
reconexão; eventos recusados não são reenviados após recuperar a conexão.

Depois de Publish, FlushTimeout aguarda até 2 segundos uma troca PING/PONG com
o servidor. Falha de Publish/flush é propagada. Um timeout depois de Publish
tem resultado incerto e não provoca reenvio automático, evitando duplicação.
Uma troca PING/PONG não é ACK de aceitação individual, persistência ou
processamento pelo consumidor; rejeições assíncronas do servidor e a recepção
efetiva também precisam ser observadas. Core NATS continua best effort,
sem JetStream/outbox. Uma falha NATS não impede os destinos independentes
WebSocket e webhook de serem tentados.

## Validação

As regressões usam o cliente nats.go real contra um peer TCP local de protocolo
controlado: preservação de subject/payload, modos, ausência de URL/conexão,
recuperação da falha inicial sem replay, limite de tentativas, erro de Publish,
timeout de flush e publicação concorrente com Close. A regressão de dispatch
cobre ButtonClick com NATS indisponível sem suprimir WebSocket/webhook.
Esses testes não substituem uma homologação com broker e consumidor reais.

Na homologação autorizada, conectar o consumidor antes dos cliques, verificar
permissões e correlacionar `instanceId`, `data.buttonId` e `data.messageId`.
Confirmar lista e dois cards no consumidor, não apenas no log da API. Testar
indisponibilidade e recuperação separadamente, sem reenviar resultados incertos.
NATS não foi habilitado nem produção alterada durante esta correção.

Referência: [Core NATS](https://docs.nats.io/learn/core-nats/).
