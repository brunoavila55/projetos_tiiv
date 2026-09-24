-- name: CriarTicket :one
INSERT INTO tickets (solicitante_nome, titulo, descricao, prioridade, origem_ip)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListarTickets :many
SELECT
    tk.id, tk.numero, tk.solicitante_nome, tk.titulo, tk.descricao,
    tk.prioridade, tk.status, tk.tarefa_id, tk.tratado_em, tk.criado_em,
    u.nome AS tratado_por_nome,
    ta.status AS tarefa_status
FROM tickets tk
LEFT JOIN usuarios u ON u.id = tk.tratado_por
LEFT JOIN tarefas ta ON ta.id = tk.tarefa_id
WHERE (sqlc.narg('status')::text IS NULL OR tk.status = sqlc.narg('status'))
ORDER BY
    CASE WHEN tk.status = 'aberto' THEN 0 ELSE 1 END,
    CASE WHEN tk.status = 'aberto' THEN
        CASE tk.prioridade WHEN 'alta' THEN 1 WHEN 'media' THEN 2 ELSE 3 END
    ELSE 0 END,
    CASE WHEN tk.status = 'aberto' THEN tk.criado_em END ASC,
    tk.tratado_em DESC NULLS LAST,
    tk.criado_em DESC
LIMIT 200;

-- name: ContarTicketsAbertos :one
SELECT count(*) FROM tickets WHERE status = 'aberto';

-- name: BloquearTicket :one
SELECT * FROM tickets WHERE id = $1 FOR UPDATE;

-- name: MarcarTicketTratado :one
UPDATE tickets
SET status = $2, tarefa_id = $3, tratado_por = $4, tratado_em = now()
WHERE id = $1
RETURNING *;
