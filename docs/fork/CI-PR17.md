# Diagnóstico de Actions no PR #17

Consulta em 2026-11-17, somente API/terminal. Produção não foi acessada ou alterada.

## Evidências

- PR #17 aberto, sem conflito, head na branch `codex/sync-upstream-0.7.3`.
- `GET /repos/cesar-carlos/evolution-go/actions/permissions`: `enabled: true`,
  `allowed_actions: all`; permissões do token de workflow continuam somente leitura.
- Workflow `quality.yml`: `state: active`.
- `GET /repos/cesar-carlos/evolution-go/actions/policies`: nenhuma política.
- Nenhum run de Go Quality associado à branch, inclusive após fechar/reabrir o
  mesmo PR com autenticação Git existente. Os checks Cursor são independentes.
- `POST /repos/cesar-carlos/evolution-go/actions/workflows/quality.yml/dispatches`:
  HTTP 422, `Actions has been disabled for this repository.`
- Reaplicar `enabled: true` sem mudar a allowlist retornou 204; habilitar o
  workflow também retornou 204. Novo dispatch continuou com o mesmo erro 422.

O erro é consistente com um estado desabilitado controlado pelo GitHub, distinto
das configurações normais do repositório. A API não informa o motivo desse bloqueio.
A [documentação oficial](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/enabling-features-for-your-repository/managing-github-actions-settings-for-a-repository#managing-github-actions-permissions-for-your-repository)
orienta contatar GitHub Support para revisão quando alterar as configurações não
restaura Actions. Nenhuma proteção foi removida e nenhum check foi marcado como
aprovado artificialmente.

## Retomada após a restauração

O commit `0e9f183` adiciona disparo manual do mesmo workflow, com base obrigatória.
Não é um bypass do bloqueio: usa a mesma infraestrutura e permissões de Actions.
O workflow valida existência/ancestralidade da base e executa as mesmas suítes
Go, PostgreSQL, race, Whatsmeow, monitor e navegador.

Payload para o endpoint de dispatch acima:

```json
{"ref":"codex/sync-upstream-0.7.3","inputs":{"format_base":"98bcc1f5aa53078cfb176d437efd971b8e3b4527"}}
```

Depois de restaurar Actions, executar esse dispatch, confirmar o SHA atual do
PR no run, acompanhar todos os jobs e corrigir eventuais falhas. Até isso ocorrer,
manter a integração sem merge/release. A validação local já registrada no inventário
não substitui uma execução aprovada no GitHub.

## Texto para revisão pelo suporte

O repositório público `cesar-carlos/evolution-go` não inicia Actions no PR #17.
As APIs de permissões e workflows indicam enabled/active, sem políticas de eventos
ou atores. O dispatch manual de `quality.yml` retorna HTTP 422 com
`Actions has been disabled for this repository.` Reaplicar enable do repositório
e do workflow retorna 204, mas não restaura a execução. Solicitamos revisão do
estado de desabilitação e orientação sobre o motivo e a restauração.

Este texto foi apenas preparado; nenhum chamado foi enviado.

## Atualização após o merge — 2026-11-17

O bloqueio descrito acima deixou de impedir a execução após o merge autorizado
do PR #17. O workflow [Go Quality de 312e3db](https://github.com/cesar-carlos/evolution-go/actions/runs/44135935581)
concluiu com sucesso: build, vet, testes, race detector, regressões da biblioteca
Whatsmeow e testes de navegador. A API comprova a retomada, mas não identifica a
causa da liberação. O diagnóstico acima é histórico, não uma pendência atual.
