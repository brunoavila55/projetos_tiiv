-- name: ListarUsuariosAtivos :many
SELECT u.id, u.nome, u.cor, f.atualizado_em AS foto_atualizada_em
FROM usuarios u
LEFT JOIN usuario_fotos f ON f.usuario_id = u.id
WHERE u.ativo = true
ORDER BY u.nome ASC;

-- name: ListarTodosUsuarios :many
SELECT u.id, u.nome, u.cor, u.papel, u.ativo, u.tentativas_falhas, u.bloqueado_ate, u.criado_em, u.tema,
       u.setor_id, s.nome AS setor_nome,
       f.atualizado_em AS foto_atualizada_em
FROM usuarios u
JOIN setores s ON s.id = u.setor_id
LEFT JOIN usuario_fotos f ON f.usuario_id = u.id
WHERE sqlc.narg('setor_id')::uuid IS NULL OR u.setor_id = sqlc.narg('setor_id')
ORDER BY u.nome ASC;

-- name: BuscarUsuarioPorID :one
SELECT id, nome, cor, papel, ativo, tentativas_falhas, bloqueado_ate, criado_em, tema, setor_id
FROM usuarios
WHERE id = $1;

-- name: BuscarUsuarioPorIDComPin :one
SELECT id, nome, cor, pin_hash, papel, ativo, tentativas_falhas, bloqueado_ate, criado_em, tema, setor_id
FROM usuarios
WHERE id = $1;

-- name: ContarUsuarios :one
SELECT count(*)
FROM usuarios;

-- name: ContarSuperadminsAtivos :one
SELECT count(*)
FROM usuarios
WHERE papel = 'superadmin' AND ativo = true;

-- name: CriarUsuario :one
INSERT INTO usuarios (nome, cor, pin_hash, papel, ativo, setor_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, nome, cor, papel, ativo, tentativas_falhas, bloqueado_ate, criado_em, tema, setor_id;

-- name: AtualizarUsuario :one
UPDATE usuarios
SET nome = $2, cor = $3, papel = $4, ativo = $5, setor_id = $6
WHERE id = $1
RETURNING id, nome, cor, papel, ativo, tentativas_falhas, bloqueado_ate, criado_em, tema, setor_id;

-- name: AtualizarTemaUsuario :one
UPDATE usuarios
SET tema = $2
WHERE id = $1
RETURNING id, tema;

-- name: AtualizarPin :one
UPDATE usuarios
SET pin_hash = $2
WHERE id = $1
RETURNING id;

-- name: AtualizarProprioPin :exec
-- Troca feita pelo próprio usuário: cumpre a troca obrigatória e zera as tentativas
UPDATE usuarios
SET pin_hash = $2, deve_trocar_pin = false, tentativas_falhas = 0, bloqueado_ate = NULL, bloqueios = 0
WHERE id = $1;

-- name: ReservarTentativaPin :one
-- Conta a tentativa ANTES de comparar o PIN, numa única instrução atômica:
-- requisições simultâneas não passam juntas pela checagem de bloqueio.
-- Sem linha retornada = usuário inexistente, inativo ou bloqueado.
-- A contagem recomeça após 15 min sem tentativas ou ao fim de um bloqueio
-- (o WHERE garante que bloqueado_ate, se preenchido, já expirou). A 5ª
-- tentativa seguida bloqueia por 5 min, 15 min, 45 min e depois 1 h.
UPDATE usuarios
SET tentativas_falhas = CASE
        WHEN ultima_tentativa_em IS NULL OR ultima_tentativa_em < now() - interval '15 minutes'
             OR bloqueado_ate IS NOT NULL THEN 1
        ELSE tentativas_falhas + 1
    END,
    bloqueado_ate = CASE
        WHEN ultima_tentativa_em >= now() - interval '15 minutes' AND bloqueado_ate IS NULL
             AND tentativas_falhas + 1 >= 5
            THEN now() + least(interval '5 minutes' * power(3, bloqueios), interval '1 hour')
    END,
    bloqueios = CASE
        WHEN ultima_tentativa_em >= now() - interval '15 minutes' AND bloqueado_ate IS NULL
             AND tentativas_falhas + 1 >= 5
            THEN bloqueios + 1
        ELSE bloqueios
    END,
    ultima_tentativa_em = now()
WHERE id = $1 AND ativo = true AND (bloqueado_ate IS NULL OR bloqueado_ate <= now())
RETURNING id, nome, cor, pin_hash, papel, tema, bloqueado_ate, deve_trocar_pin, setor_id;

-- name: ZerarTentativasFalhas :exec
UPDATE usuarios
SET tentativas_falhas = 0, bloqueado_ate = NULL, bloqueios = 0
WHERE id = $1;

-- name: MarcarTrocaPinObrigatoria :exec
UPDATE usuarios
SET deve_trocar_pin = true
WHERE id = $1;

-- name: DesbloquearUsuario :one
UPDATE usuarios
SET tentativas_falhas = 0, bloqueado_ate = NULL, bloqueios = 0
WHERE id = $1
RETURNING id, nome, cor, papel, ativo, tentativas_falhas, bloqueado_ate, criado_em, tema, setor_id;

-- name: ObterFotoUsuario :one
SELECT f.conteudo, f.mime, f.atualizado_em, u.ativo AS usuario_ativo
FROM usuario_fotos f
JOIN usuarios u ON u.id = f.usuario_id
WHERE f.usuario_id = $1;

-- name: ObterVersaoFotoUsuario :one
SELECT atualizado_em
FROM usuario_fotos
WHERE usuario_id = $1;

-- name: SalvarFotoUsuario :one
INSERT INTO usuario_fotos (usuario_id, conteudo, mime, atualizado_em)
VALUES ($1, $2, $3, now())
ON CONFLICT (usuario_id) DO UPDATE
SET conteudo = EXCLUDED.conteudo, mime = EXCLUDED.mime, atualizado_em = now()
RETURNING atualizado_em;

-- name: RemoverFotoUsuario :exec
DELETE FROM usuario_fotos
WHERE usuario_id = $1;
