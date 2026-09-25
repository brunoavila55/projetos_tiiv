-- +goose Up
-- Plantão vira módulo próprio e quem fica escalado não precisa ser usuário do
-- sistema: a escala guarda só o nome, em texto livre.

ALTER TABLE plantoes ADD COLUMN nome VARCHAR(80);
UPDATE plantoes p SET nome = u.nome FROM usuarios u WHERE u.id = p.usuario_id;
ALTER TABLE plantoes ALTER COLUMN nome SET NOT NULL;

DROP INDEX IF EXISTS idx_plantoes_usuario;
ALTER TABLE plantoes DROP COLUMN usuario_id;
-- Conflito e sugestões comparam o nome sem diferenciar maiúsculas
CREATE INDEX idx_plantoes_nome ON plantoes(setor_id, lower(nome), inicio);

-- +goose Down
-- Só volta quem tem o mesmo nome de um usuário do setor; o resto sai da escala
ALTER TABLE plantoes ADD COLUMN usuario_id UUID REFERENCES usuarios(id) ON DELETE RESTRICT;
UPDATE plantoes p SET usuario_id = (
    SELECT u.id FROM usuarios u
    WHERE lower(u.nome) = lower(p.nome) AND u.setor_id = p.setor_id
    ORDER BY u.ativo DESC, u.criado_em
    LIMIT 1
);
DELETE FROM plantoes WHERE usuario_id IS NULL;
ALTER TABLE plantoes ALTER COLUMN usuario_id SET NOT NULL;
DROP INDEX IF EXISTS idx_plantoes_nome;
ALTER TABLE plantoes DROP COLUMN nome;
CREATE INDEX idx_plantoes_usuario ON plantoes(usuario_id, inicio);
