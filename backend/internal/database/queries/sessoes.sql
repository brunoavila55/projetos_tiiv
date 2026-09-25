-- name: CriarSessao :one
INSERT INTO sessoes (token_hash, usuario_id, expira_em, ultimo_uso_em)
VALUES ($1, $2, $3, $4)
RETURNING id, token_hash, usuario_id, expira_em, ultimo_uso_em, criado_em;

-- name: BuscarSessaoPorHash :one
SELECT 
    s.id,
    s.token_hash,
    s.usuario_id,
    s.expira_em,
    s.ultimo_uso_em,
    s.criado_em,
    u.nome AS usuario_nome,
    u.cor AS usuario_cor,
    u.papel AS usuario_papel,
    u.ativo AS usuario_ativo,
    u.tema AS usuario_tema,
    u.deve_trocar_pin AS usuario_deve_trocar_pin,
    st.id AS setor_id,
    st.nome AS setor_nome
FROM sessoes s
JOIN usuarios u ON u.id = s.usuario_id
-- Setor de trabalho: o escolhido na sessão (só superadmin) ou o do usuário
JOIN setores st ON st.id = COALESCE(CASE WHEN u.papel = 'superadmin' THEN s.setor_id END, u.setor_id)
WHERE s.token_hash = $1 AND u.ativo = true;

-- name: AtualizarUltimoUsoSessao :exec
UPDATE sessoes
SET ultimo_uso_em = $2
WHERE id = $1;

-- name: DeletarSessaoPorHash :exec
DELETE FROM sessoes
WHERE token_hash = $1;

-- name: DeletarSessoesPorUsuario :exec
DELETE FROM sessoes
WHERE usuario_id = $1;

-- name: DeletarOutrasSessoesUsuario :exec
DELETE FROM sessoes
WHERE usuario_id = $1 AND id <> $2;

-- name: DeletarSessoesExpiradas :exec
DELETE FROM sessoes
WHERE expira_em < now();
