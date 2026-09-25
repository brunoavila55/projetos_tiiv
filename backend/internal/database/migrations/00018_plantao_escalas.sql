-- +goose Up
-- A escala passa a ter três tipos e o sobreaviso sai: plantão interno (a
-- equipe do setor) e, para os técnicos externos, plantão noturno e plantão de
-- domingo, cada um por cidade. O plantão que já existia vira interno.

DELETE FROM plantoes WHERE tipo = 'sobreaviso';

ALTER TABLE plantoes DROP CONSTRAINT IF EXISTS plantoes_tipo_check;
UPDATE plantoes SET tipo = 'interno';
ALTER TABLE plantoes ALTER COLUMN tipo SET DEFAULT 'interno';
ALTER TABLE plantoes ADD CONSTRAINT plantoes_tipo_check CHECK (tipo IN ('interno', 'noturno', 'domingo'));

-- Cidade só nos plantões dos técnicos externos
ALTER TABLE plantoes ADD COLUMN cidade VARCHAR(30);
ALTER TABLE plantoes ADD CONSTRAINT chk_plantao_cidade CHECK (
    (tipo = 'interno' AND cidade IS NULL)
    OR (tipo <> 'interno' AND cidade IN ('sao_gabriel', 'bage', 'passo_fundo'))
);

-- +goose Down
-- Tudo volta a ser plantão; o sobreaviso apagado não volta
ALTER TABLE plantoes DROP CONSTRAINT IF EXISTS chk_plantao_cidade;
ALTER TABLE plantoes DROP COLUMN IF EXISTS cidade;
ALTER TABLE plantoes DROP CONSTRAINT IF EXISTS plantoes_tipo_check;
UPDATE plantoes SET tipo = 'plantao';
ALTER TABLE plantoes ALTER COLUMN tipo SET DEFAULT 'plantao';
ALTER TABLE plantoes ADD CONSTRAINT plantoes_tipo_check CHECK (tipo IN ('plantao', 'sobreaviso'));
