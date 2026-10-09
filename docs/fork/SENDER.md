# Sender do fork

Abra `/sender` no mesmo host da API e informe manualmente o **token da instância**.
A chave global não é aceita nessa interface. `/manager`, a rota raiz e as APIs
existentes mantêm seu comportamento. O servidor nunca injeta credenciais no HTML,
inclusive em loopback ou atrás de proxy local. A licença continua necessária para APIs.

Credenciais, conversas e anexos ficam em memória nesta aba. Recarregar, sair ou trocar
instância apaga os dados; não há persistência em localStorage/sessionStorage/cookies.
Use HTTPS fora do ambiente local. `/sender/ws` usa token de instância na query para
o handshake do navegador: o logger da aplicação omite essa rota; configure também
o proxy para não registrar a query. O stream exige Origin do mesmo host e nunca aceita
um ID de instância fornecido pelo cliente nem assinatura de eventos globais.

O botão **Ativar recebimento** pede confirmação antes de habilitar WebSocket e incluir
Message/SendMessage/Receipt nas assinaturas existentes. Pausar fecha a conexão desta
aba; não modifica a configuração da instância. O produtor existente mantém uma conexão
por instância: abrir outro consumidor substitui o anterior com close code 4001;
o Sender anterior pausa para evitar disputa entre abas. Reconexões usam
backoff até 30 s e param após oito falhas consecutivas; a sessão pode ser reativada.

Texto usa `/send/text`; anexos usam `/send/media` multipart. Resultado incerto não
gera repetição automática. O histórico é limitado a 50 conversas e 100 mensagens por
conversa; mídia mantida em memória tem teto de 20 MiB e anexos enviados, de 16 MiB.
Mídia recebida sem bytes pode ser carregada explicitamente pela API existente.
Conteúdo de mensagens é texto, nunca HTML; arquivos ativos são apenas downloads.

LIDs explícitos (`@lid`) são consultados em `/sender/resolve-lids`, com até 100 IDs
por requisição, usando `client.Store.LIDs` da sessão autenticada. Funciona com os
stores PostgreSQL/SQLite existentes; não cria pools nem consulta outra sessão.
Mapeamentos ausentes conservam o LID. O novo `/sender/session` retorna identidade,
estado e assinaturas da própria instância, sem tokens.

Validação: testes de autenticação, origem, isolamento e HTML em `pkg/sender/handler`;
stores reais SQLite/PostgreSQL e cancelamento em `pkg/sender/service`; navegação,
envio simulado, mídia, XSS, reconexão e limpeza em `tests/ui/sender.test.cjs`.
Para PostgreSQL local, `SENDER_TEST_POSTGRES_DSN` deve apontar para banco de teste
descartável: o teste cria o schema Whatsmeow. A CI fornece um serviço isolado.

Os testes de navegador usam REST/WebSocket simulados e não enviam WhatsApp real.
Recebimento, renderização e envio reais permanecem sujeitos à homologação descrita em
[INTERACTIVE-COMPATIBILITY.md](INTERACTIVE-COMPATIBILITY.md). A adaptação é inspirada
no PR oficial #182 (prakash-dev-code), sem importar o bootstrap de chave ou SQL do PR.
