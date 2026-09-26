-- name: CriarTelaTV :one
INSERT INTO telas_tv (nome, chave_hash, criado_por, setor_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListarTelasTV :many
SELECT t.id, t.nome, t.criado_em, t.ultimo_acesso_em, t.ultimo_ip, u.nome AS criador_nome
FROM telas_tv t
JOIN usuarios u ON u.id = t.criado_por
WHERE t.setor_id = $1
ORDER BY lower(t.nome);

-- name: BuscarTelaTVPorHash :one
SELECT t.*, s.nome AS setor_nome, s.modulos_desativados AS setor_modulos_desativados
FROM telas_tv t
JOIN setores s ON s.id = t.setor_id
WHERE t.chave_hash = $1;

-- name: RegistrarAcessoTelaTV :exec
-- Grava no máximo uma vez por minuto (a TV consulta a cada poucos segundos)
UPDATE telas_tv
SET ultimo_acesso_em = now(), ultimo_ip = $2
WHERE id = $1
  AND (ultimo_acesso_em IS NULL OR ultimo_acesso_em < now() - interval '1 minute' OR ultimo_ip <> $2);

-- name: DeletarTelaTV :execrows
DELETE FROM telas_tv WHERE id = $1 AND setor_id = $2;
