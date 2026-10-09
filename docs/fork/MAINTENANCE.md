# Manutenção do fork cesar-carlos

## Objetivo e decisões aprovadas

Este fork de [Evolution Go](https://github.com/evolution-foundation/evolution-go)
mantém funcionalidades utilizáveis enquanto correções aguardam integração oficial.
A distribuição é independente e não implica endosso da Evolution Foundation.
Preserve `LICENSE`, `NOTICE`, `TRADEMARKS.md`, atribuições e os componentes visuais
existentes. A política foi aprovada pelo responsável pelo fork em 2026-10-09.

- `origin`: `https://github.com/cesar-carlos/evolution-go.git`, destino das alterações.
- `upstream`: `https://github.com/evolution-foundation/evolution-go.git`, fonte oficial.
- `main`: código integrado e validado do fork; histórico publicado não é reescrito.
- Base oficial: [`.github/upstream-base.json`](../../.github/upstream-base.json).
- Diferenças e evidências: [PATCHES.md](PATCHES.md); instruções gerais: [AGENTS.md](../../AGENTS.md).
- Releases estáveis oficiais são a base normal. Não acompanhar `develop` automaticamente.
- Cada correção local só é retirada quando a implementação oficial resolver seu
  comportamento completo, comprovado pelos testes relevantes. Merge sem conflito
  e PR fechado/mesclado não comprovam equivalência funcional.
- Novas correções de listas/botões, alterações do Manager e refatorações independentes
  exigem tarefas próprias. A primeira release contém as correções já integradas.
- Publicação de imagem não atualiza servidores. Homologação e implantação são etapas distintas.

## Antes de alterar código

Leia este guia e o inventário antes de corrigir bugs, trocar dependências, sincronizar
o upstream ou publicar. Consulte as rules por escopo conforme `AGENTS.md`.
Verifique árvore de trabalho, remotes e HEAD; preserve mudanças existentes.
Não assuma que branch de PR, fork de terceiros ou tag oficial contém uma correção:
compare código e testes. Use commits específicos para dependências e registre o motivo.

Atualize o item do inventário afetado no mesmo conjunto de mudanças. Para um problema
novo, atribua o próximo ID `F-NNN`, sem reutilizar IDs. Registre problema, comportamento
esperado, commits/tag, PR/issue, testes/evidências, limitações, data e base comparada,
e condição objetiva de retirada. Estados permitidos:

| Estado | Significado |
|---|---|
| `local` | Correção necessária na base oficial adotada |
| `parcialmente coberto pelo upstream` | Parte oficial validada; complemento local identificado |
| `substituído pelo upstream` | Código local redundante retirado; versão substituta e testes registrados |
| `exclusivo do fork` | Processo/configuração particular, avaliado separadamente em cada atualização |

Mantenha entradas substituídas como histórico. Preserve os testes que protegem o
comportamento, adaptando-os à implementação oficial. Evidência inexistente é pendência,
não aprovação. A data de revisão do inventário não significa teste real com WhatsApp.

## Atualizar a base oficial

1. Confirme `git status --short`, registre HEAD, release e digest atualmente usados.
   Faça backup de bancos e sessões antes de qualquer implantação com migrações.
2. Confira os remotes. Se necessário, configure `git remote add upstream
   https://github.com/evolution-foundation/evolution-go.git`; preserve `origin`.
3. Busque a release escolhida explicitamente: `git fetch --no-tags upstream
   refs/tags/<tag>:refs/tags/upstream-<tag>`. Confira release, tag e SHA completo
   com `git rev-parse upstream-<tag>^{commit}`. Os marcadores `<...>` devem ser substituídos.
4. Crie `codex/sync-upstream-<versao>` a partir da `main` validada e consulte
   `git merge-base HEAD upstream-<tag>`. Havendo ancestral comum, execute
   `git merge --no-ff upstream-<tag>` na branch de integração.
5. Revise todos os itens ativos do inventário. Adote soluções oficiais completas;
   mantenha complementos de soluções parciais; adapte patches ainda necessários.
   Resolva conflitos por comportamento, sem preferência global `ours`/`theirs`.
6. Atualize o inventário e os três campos da base oficial, usando o SHA do commit
   apontado pela tag (não o SHA de um objeto tag anotado). Atualize `VERSION` para
   `<versao-oficial>-cesar.1` e os exemplos de imagem afetados.
7. Execute a validação de `AGENTS.md`, confira o diff contra a base oficial e
   submeta a integração a revisão. Mescle na `main` somente com os checks aprovados.

### Histórico sem ancestral comum

Na comparação de 2026-10-09, `main` e `develop` oficiais não compartilham ancestral.
Revalide isso a cada atualização. Se a release escolhida vier de histórico independente,
não use `--allow-unrelated-histories` nem force-push para fingir um merge normal.

Crie uma branch de migração a partir da nova tag. Transporte por problema apenas
as mudanças ainda necessárias, usando o inventário para adaptar patches e testes.
Compare as árvores antiga/nova, contratos e configuração, e mantenha a `main`
anterior acessível por tag/branch de arquivo. Registre o novo plano de migração e
obtenha decisão explícita sobre a transição da branch principal antes de trocar
seu histórico. Atualize a base registrada para a nova origem real.

## Publicar uma versão do fork

`VERSION` é a fonte para Makefile, Docker e versão compilada. A primeira versão é
`0.7.2-cesar.1`, tag `v0.7.2-cesar.1`. Incremente `N` em `X.Y.Z-cesar.N` para cada
publicação na mesma base. Esse sufixo é um identificador SemVer de pré-release;
não representa uma release oficial. Tags publicadas não são movidas/reutilizadas.

1. Prepare versão, inventário, base e documentação em uma branch `codex/...`.
2. Execute testes, confira a CI do commit final e integre a alteração na `main`.
3. Crie uma tag anotada no commit validado: `git tag -a v<versao> -m "Fork <versao>"`.
4. Publique somente essa tag: `git push origin v<versao>`.
5. Acompanhe o workflow **Publish fork release**. Ele verifica tag/VERSION,
   pertencimento à `main`, tag oficial/SHA/base, qualidade e identidade da imagem.
6. Confira a GitHub Release e o pacote público
   [GHCR](https://github.com/cesar-carlos/evolution-go/pkgs/container/evolution-go).
   Na primeira publicação, defina a visibilidade do pacote como pública em
   **Package settings → Change visibility → Public**. Essa configuração do GHCR
   é independente da visibilidade do repositório; pode exigir a interface GitHub.
7. Verifique pull anônimo pelo digest, manifesto e arquiteturas; registre a evidência
   na descrição da release. A primeira publicação só está concluída após essa verificação.

A CI roda em PRs e pushes na `main`; imagens são publicadas apenas por tags
`v*-cesar.*` válidas. A formatação em releases compara os arquivos alterados com a
base oficial registrada; em PRs compara com a base do PR. Base ausente/inválida falha.
Não executar formatadores sobre todo o projeto para ocultar dívida anterior.

A imagem é `ghcr.io/cesar-carlos/evolution-go:<versao>`, com alias
`sha-<SHA-completo>`, para `linux/amd64` e `linux/arm64`. Não há alias `latest`.
O workflow usa `GITHUB_TOKEN`, permissões por job e metadados de origem, revisão,
versão e base. A GitHub Release é criada após build, verificação e smoke checks.

### Falha parcial e reexecução

O candidato é publicado pelo SHA; a versão é promovida ao digest verificado.
Ao reexecutar o mesmo run/tag, o workflow verifica os aliases existentes. Só reutiliza
imagem com a mesma revisão, versão, base e arquiteturas. Aliases divergentes,
autenticação inválida e falhas de consulta interrompem o processo. Uma versão
existente de outro conteúdo jamais autoriza sobrescrita. Release existente deve
ter notas idênticas, normalizando apenas CRLF/LF e quebras de linha finais do
cliente/API; alterações editoriais posteriores exigem revisão consciente.

Se a imagem foi publicada mas a release falhou, corrija a causa e reexecute o job/run
original sem mover a tag. Se o código da própria release precisar mudar, incremente
a versão e publique uma tag nova; documente a tentativa anterior.

## Instalação, homologação e reversão

Para uma instalação reproduzível, copie da release o digest verificado:

```sh
docker pull ghcr.io/cesar-carlos/evolution-go@sha256:<digest-da-release>
```

No Compose raiz, `EVOLUTION_IMAGE` permite usar esse mesmo valor completo. O default
identifica a versão inicial; use digest em produção. As demais configurações seguem
[.env.example](../../.env.example). Prepare a configuração antes de executar o Compose;
este guia não autoriza iniciar serviços de produção durante testes da manutenção.

Os checks automatizados não usam licença, WhatsApp, banco de produção nem credenciais
reais. Smoke checks conferem os recursos da imagem e executam apenas `server -h`.
Homologação funcional deve registrar versão/digest, ambiente e resultados para sessões,
consultas, menções, eventos, configurações e isolamento. Renderização/cliques de
listas/botões e comportamento em aparelhos continuam pendências próprias.

Antes de implantar, registre o digest anterior e revise migrações de aplicação,
whatsmeow e formato de sessões. Voltar a imagem não desfaz migrações. Se o banco não
for compatível, planeje restauração consistente de banco/sessões antes da reversão.
A primeira release deste fork não possui release anterior própria: registre o digest
da instalação que ela substituir. Após implantação autorizada, confira saúde,
reconexões e entrega de eventos; registre resultados e eventuais regressões.

Referências: [Git merge](https://git-scm.com/docs/git-merge),
[sincronizar forks](https://docs.github.com/en/pull-requests/how-tos/work-with-forks/syncing-a-fork),
[GHCR](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry).
