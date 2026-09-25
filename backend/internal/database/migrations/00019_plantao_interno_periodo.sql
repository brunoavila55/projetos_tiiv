-- +goose Up
-- O plantão interno passa a ser por período: manhã e tarde, cada um com a
-- sua pessoa. O noturno e o de domingo não têm período.
ALTER TABLE plantoes ADD COLUMN periodo VARCHAR(5);

-- O interno que já existia cobria o dia todo: vira manhã e ganha uma cópia
-- à tarde com a mesma pessoa. A folga fica só no turno da manhã, para não
-- aparecer duas vezes.
UPDATE plantoes SET periodo = 'manha' WHERE tipo = 'interno';
INSERT INTO plantoes (nome, tipo, cidade, periodo, inicio, fim, observacao, criado_por, setor_id)
SELECT nome, tipo, cidade, 'tarde', inicio, fim, observacao, criado_por, setor_id
FROM plantoes
WHERE tipo = 'interno';

ALTER TABLE plantoes ADD CONSTRAINT chk_plantao_periodo CHECK (
    (tipo = 'interno' AND periodo IN ('manha', 'tarde'))
    OR (tipo <> 'interno' AND periodo IS NULL)
);

-- +goose Down
-- Volta a um interno por dia: fica o da manhã
ALTER TABLE plantoes DROP CONSTRAINT IF EXISTS chk_plantao_periodo;
DELETE FROM plantoes WHERE tipo = 'interno' AND periodo = 'tarde';
ALTER TABLE plantoes DROP COLUMN IF EXISTS periodo;
