# Prompt — App interno do setor (calendário, atendimentos, tarefas, estoque)

## Contexto

Construir uma aplicação web **interna**, acessada apenas pela rede local (não exposta à internet), para uso de uma equipe pequena. Módulos:

1. **Calendário**: marcar reuniões entre usuários do sistema.
2. **Atendimentos**: registrar atendimentos feitos (nome do cliente + o que foi feito). Clientes **não** são cadastrados no banco; o nome é texto livre.
3. **Tarefas**: lista de tarefas com status, prazo e responsável.
4. **Estoque básico**: controle de material do setor baseado em **movimentações** (entrada/saída/ajuste), não em edição direta de quantidade.

Acesso estilo **PDV**: a tela inicial mostra os usuários, a pessoa toca/clica no próprio nome e digita o PIN.

## Stack obrigatória

- **Backend:** Go 1.23+, `net/http` (roteamento nativo do Go 1.22+) ou `chi`; `pgx/v5` para acesso ao banco; `sqlc` para gerar queries tipadas; `goose` para migrations.
- **Banco:** PostgreSQL 16.
- **Frontend:** SvelteKit com `adapter-static` em modo SPA (fallback `index.html`), TypeScript, Tailwind CSS. Calendário com `@event-calendar/core` (nativo Svelte).
- **Entrega:** o build do front é embutido no binário Go via `go:embed`. Uma única imagem Docker (build multi-stage: Node → Go → imagem final mínima) + container do Postgres, orquestrados com Docker Compose.
- Não usar Supabase, Firebase ou qualquer serviço externo. Tudo roda self-hosted.

## Convenções gerais

- Todas as datas em `timestamptz`; exibição no fuso `America/Sao_Paulo`.
- API REST em JSON sob `/api/*`; todo o resto serve a SPA.
- Erros da API no formato `{ "error": "mensagem legível" }` com status HTTP adequado.
- Configuração por variáveis de ambiente (`DATABASE_URL`, `LISTEN_ADDR`, `SESSION_TTL`, `COOKIE_SECURE`, `ADMIN_NOME`, `ADMIN_PIN`).
- Interface em português do Brasil.

---

## P0 — Fundação (bloqueante)

### P0.1 Estrutura do projeto e deploy

- Monorepo com `/backend` (Go) e `/frontend` (SvelteKit).
- `Dockerfile` multi-stage gerando um único binário que serve API + SPA.
- `docker-compose.yml` com `app` e `postgres` (volume persistente, healthcheck no Postgres, `app` depende do Postgres saudável).
- Migrations rodam automaticamente na inicialização do app.
- `Makefile` com `dev`, `build`, `migrate`, `sqlc`, `test`.

**Critérios de aceite**
- [ ] `docker compose up -d` sobe o sistema do zero e a aplicação responde na porta configurada.
- [ ] `GET /api/health` retorna 200 com status do banco.
- [ ] Recarregar qualquer rota da SPA (ex.: `/estoque`) não retorna 404.
- [ ] Reiniciar os containers não perde dados.

### P0.2 Modelo de dados

Criar as tabelas (via migrations):

- `usuarios`: `id`, `nome`, `cor` (hex, para avatar e calendário), `pin_hash`, `papel` (`admin` | `usuario`), `ativo`, `tentativas_falhas`, `bloqueado_ate`, `criado_em`.
- `sessoes`: `id`, `token_hash`, `usuario_id`, `expira_em`, `ultimo_uso_em`, `criado_em`.
- `eventos`: `id`, `titulo`, `descricao`, `inicio`, `fim`, `dia_inteiro`, `criado_por`, `criado_em`, `atualizado_em`. CHECK `fim >= inicio`.
- `evento_participantes`: `evento_id`, `usuario_id` (PK composta). Participantes são **apenas usuários do sistema**.
- `atendimentos`: `id`, `cliente_nome` (texto livre, obrigatório), `descricao` (obrigatório), `data_atendimento`, `usuario_id`, `criado_em`, `atualizado_em`.
- `tarefas`: `id`, `titulo`, `descricao`, `status` (`pendente` | `em_andamento` | `concluida`), `prioridade` (`baixa` | `media` | `alta`), `prazo` (opcional), `criado_por`, `responsavel_id` (opcional), `concluida_em`, `criado_em`, `atualizado_em`.
- `itens_estoque`: `id`, `nome` (único), `unidade` (ex.: un, m, cx), `categoria`, `estoque_minimo`, `saldo`, `ativo`, `criado_em`. CHECK `saldo >= 0`.
- `movimentacoes_estoque`: `id`, `item_id`, `tipo` (`entrada` | `saida` | `ajuste`), `quantidade`, `saldo_resultante`, `motivo`, `usuario_id`, `criado_em`.

Índices nas FKs e nas colunas de filtro por data.

**Critérios de aceite**
- [ ] Migrations aplicam e revertem (`goose up` / `goose down`) sem erro.
- [ ] Constraints impedem evento com fim antes do início e saldo negativo.
- [ ] Queries geradas via `sqlc`, sem SQL montado por concatenação de strings.

### P0.3 Autenticação estilo PDV

- `GET /api/auth/usuarios`: pública, retorna apenas `id`, `nome`, `cor` dos usuários ativos.
- `POST /api/auth/login` com `{ usuario_id, pin }`.
- PIN numérico de exatamente 4 dígitos, armazenado com bcrypt. PINs **não** precisam ser únicos entre usuários.
- Após 5 tentativas erradas, o usuário fica bloqueado por 5 minutos; login bem-sucedido zera o contador.
- Sessão em cookie `HttpOnly`, `SameSite=Strict`, `Secure` configurável por env (LAN pode rodar em HTTP). No banco guarda-se apenas o hash do token.
- Expiração por inatividade (padrão 30 min) e absoluta (padrão 12 h).
- `POST /api/auth/logout` invalida a sessão.
- Primeiro admin criado na inicialização a partir de `ADMIN_NOME` e `ADMIN_PIN` caso não exista nenhum usuário.
- Middleware exige sessão válida em todas as rotas `/api/*`, exceto `health`, `auth/usuarios` e `auth/login`.

**Frontend**
- Tela inicial: grade de cartões com avatar (inicial do nome + cor) e nome.
- Ao selecionar, abre teclado numérico grande (funciona por toque e pelo teclado físico), com os dígitos mascarados e botão de apagar.
- Mensagem clara para PIN incorreto e para usuário bloqueado (com tempo restante).
- Botão "Sair" sempre visível; logout automático por inatividade leva de volta à grade de usuários.

**Critérios de aceite**
- [ ] Login correto leva ao painel; incorreto mostra erro sem revelar nada além de "PIN incorreto".
- [ ] A 6ª tentativa após 5 erros retorna bloqueio, mesmo com PIN correto, até o tempo expirar.
- [ ] Dois usuários podem ter o mesmo PIN e cada um acessa apenas a própria conta.
- [ ] Nenhum PIN ou token em texto puro no banco ou nos logs.
- [ ] Chamada à API sem sessão retorna 401 e o front redireciona para a grade.

---

## P1 — Módulos principais

### P1.1 Gestão de usuários (admin)

- CRUD de usuários: nome, cor, papel, ativo.
- Admin redefine PIN de qualquer usuário e desbloqueia usuários.
- Cada usuário pode trocar o próprio PIN informando o atual.
- Usuário desativado some da grade de login e não consegue acessar; seus registros históricos permanecem.

**Critérios de aceite**
- [ ] Usuário comum recebe 403 nas rotas de administração.
- [ ] Não é possível desativar ou rebaixar o último admin ativo.
- [ ] Desativar um usuário encerra as sessões dele.

### P1.2 Calendário

- Visões mensal, semanal e diária; navegação entre períodos; botão "Hoje".
- Criar evento clicando/arrastando no calendário ou por botão; editar arrastando/redimensionando.
- Formulário: título, descrição, início, fim, dia inteiro, participantes (seleção múltipla de usuários).
- O criador é incluído como participante automaticamente.
- Filtro "Meus eventos" (onde sou participante) e "Todos".
- Evento colorido pela cor do criador.
- Apenas o criador ou um admin pode editar/excluir.
- API: `GET /api/eventos?inicio=&fim=` retorna apenas eventos no intervalo.

**Critérios de aceite**
- [ ] Evento criado aparece para todos os participantes no filtro "Meus eventos".
- [ ] Arrastar um evento persiste o novo horário.
- [ ] Participante que não é criador vê o evento mas não consegue editá-lo (UI e API).
- [ ] Horários exibidos corretamente em `America/Sao_Paulo`.

### P1.3 Atendimentos

- Formulário: nome do cliente (texto livre), data/hora do atendimento (padrão: agora), descrição do que foi feito.
- Lista paginada, ordenada do mais recente, com busca por nome do cliente e texto da descrição, e filtros por período e por usuário.
- Visualização detalhada de um atendimento.
- Autor ou admin pode editar/excluir; `atualizado_em` é registrado.

**Critérios de aceite**
- [ ] Registrar um atendimento leva menos de 3 interações após abrir o formulário (campos obrigatórios mínimos).
- [ ] Busca por parte do nome do cliente encontra o registro, sem diferenciar maiúsculas/acentos.
- [ ] Filtro por período e usuário combinam corretamente com a busca.

### P1.4 Tarefas

- Campos: título, descrição, prioridade, prazo (opcional), responsável (opcional, qualquer usuário).
- Visões: "Minhas tarefas" (sou responsável), "Criadas por mim" e "Todas".
- Mudança de status com um clique; ao concluir, grava `concluida_em`.
- Tarefas atrasadas (prazo vencido e não concluídas) destacadas visualmente.
- Ordenação padrão: não concluídas primeiro, depois por prazo e prioridade.
- Criador, responsável ou admin podem editar; apenas criador ou admin exclui.

**Critérios de aceite**
- [ ] Tarefa atribuída a outro usuário aparece em "Minhas tarefas" dele.
- [ ] Reabrir uma tarefa concluída limpa `concluida_em`.
- [ ] Tarefas atrasadas aparecem destacadas e no topo das não concluídas.

### P1.5 Estoque

- CRUD de itens (admin): nome, unidade, categoria, estoque mínimo, ativo. Saldo **não** é editável diretamente.
- Movimentações (qualquer usuário): entrada, saída ou ajuste, com quantidade e motivo (obrigatório para saída e ajuste).
- Ajuste define o saldo final contado; o sistema calcula e registra a diferença.
- Cada movimentação atualiza `itens_estoque.saldo` e grava `saldo_resultante` **na mesma transação**, com `SELECT ... FOR UPDATE` no item.
- Saída maior que o saldo é recusada.
- Movimentações são imutáveis: não há edição nem exclusão; correções são feitas com novo ajuste.
- Lista de itens com saldo atual, busca, filtro por categoria e destaque para itens abaixo do mínimo.
- Histórico de movimentações por item e geral, filtrável por período, tipo e usuário.

**Critérios de aceite**
- [ ] Duas saídas simultâneas do mesmo item nunca deixam o saldo negativo (teste de concorrência).
- [ ] O saldo do item sempre bate com o último `saldo_resultante` do histórico.
- [ ] Não existe rota que altere ou apague uma movimentação.
- [ ] Itens abaixo do estoque mínimo aparecem destacados na lista.

### P1.6 Painel inicial

- Após o login: próximos eventos do usuário (hoje e amanhã), tarefas pendentes atribuídas a ele (atrasadas primeiro) e itens abaixo do estoque mínimo.

**Critérios de aceite**
- [ ] Painel carrega com uma única chamada `GET /api/painel`.
- [ ] Cada bloco leva à tela completa do módulo correspondente.

---

## P2 — Qualidade e operação

### P2.1 Testes

- Testes de integração do backend contra Postgres real (testcontainers ou banco de teste no compose).
- Cobrir: login/bloqueio, permissões por papel e autoria, concorrência do estoque, filtro de intervalo do calendário.

**Critérios de aceite**
- [ ] `make test` roda toda a suíte e passa.
- [ ] Teste de concorrência do estoque dispara ao menos 20 saídas em paralelo.

### P2.2 Backup

- Serviço no compose que roda `pg_dump` diário em formato custom para um volume dedicado, mantendo os últimos 7 dias.
- Documentar no README como restaurar.

**Critérios de aceite**
- [ ] Após 24 h existe um dump válido no volume de backup.
- [ ] Restaurar o dump num banco vazio recupera todos os dados (procedimento testado e descrito).

### P2.3 Logs e erros

- Logs estruturados (`log/slog`, JSON) com método, rota, status, duração e `usuario_id` quando houver.
- Recuperação de panic no middleware retornando 500 sem derrubar o processo.
- Front exibe toast de erro amigável para qualquer falha da API.

**Critérios de aceite**
- [ ] Nenhum log contém PIN, token ou cookie.
- [ ] Um panic forçado numa rota não derruba o servidor.

### P2.4 Responsividade

- Todas as telas utilizáveis em celular (a grade de login e o teclado numérico especialmente).

**Critérios de aceite**
- [ ] Fluxo completo (login, registrar atendimento, dar saída no estoque, concluir tarefa) funciona em tela de 375 px sem rolagem horizontal.

---

## P3 — Desejáveis

- Exportar atendimentos e movimentações de estoque para CSV com os filtros aplicados.
- Tema escuro.
- Instalação como PWA (ícone na tela inicial; sem suporte offline).
- Comentários em tarefas.
- Eventos recorrentes no calendário (semanal/mensal).

**Critérios de aceite**
- [ ] CSV exportado abre corretamente no Excel/LibreOffice com acentos (UTF-8 com BOM, separador `;`).
- [ ] Tema escolhido persiste por usuário.

---

## Fora de escopo

- Exposição na internet, HTTPS obrigatório, SSO ou login por senha.
- Cadastro de clientes.
- Vínculo entre atendimentos e estoque.
- Notificações por e-mail ou push.

## Entregáveis

- Código completo no monorepo, `README.md` com instruções de instalação, variáveis de ambiente, backup/restauração e criação do primeiro admin.
- Implementar na ordem P0 → P1 → P2 → P3, garantindo que todos os critérios de aceite de uma prioridade passem antes de avançar.