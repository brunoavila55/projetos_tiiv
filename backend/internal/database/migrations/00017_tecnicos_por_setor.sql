-- +goose Up
-- Técnicos passam a ser de um setor, como os demais cadastros; os registros de
-- entrada/saída seguem o setor do técnico. Os que já existem ficam no NOC.

ALTER TABLE tecnicos ADD COLUMN setor_id UUID REFERENCES setores(id) ON DELETE RESTRICT;
UPDATE tecnicos SET setor_id = (SELECT id FROM setores WHERE nome = 'NOC');
ALTER TABLE tecnicos ALTER COLUMN setor_id SET NOT NULL;

-- O mesmo nome pode existir em setores diferentes
DROP INDEX IF EXISTS idx_tecnicos_nome_unico;
CREATE UNIQUE INDEX idx_tecnicos_setor_nome_unico ON tecnicos (setor_id, lower(nome));

-- +goose Down
-- Falha se o mesmo nome existir em dois setores (renomeie um antes)
DROP INDEX IF EXISTS idx_tecnicos_setor_nome_unico;
CREATE UNIQUE INDEX idx_tecnicos_nome_unico ON tecnicos (lower(nome));
ALTER TABLE tecnicos DROP COLUMN IF EXISTS setor_id;
