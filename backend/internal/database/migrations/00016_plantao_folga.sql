-- +goose Up
-- Folga opcional de quem fica de plantão, marcada junto com o turno (edita e
-- some com ele). Dias inclusivos, como o turno.

ALTER TABLE plantoes ADD COLUMN folga_inicio DATE;
ALTER TABLE plantoes ADD COLUMN folga_fim DATE;
ALTER TABLE plantoes ADD CONSTRAINT chk_plantao_folga CHECK (
    (folga_inicio IS NULL AND folga_fim IS NULL)
    OR (folga_inicio IS NOT NULL AND folga_fim >= folga_inicio)
);
CREATE INDEX idx_plantoes_folga ON plantoes(setor_id, folga_inicio, folga_fim) WHERE folga_inicio IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_plantoes_folga;
ALTER TABLE plantoes DROP CONSTRAINT IF EXISTS chk_plantao_folga;
ALTER TABLE plantoes DROP COLUMN IF EXISTS folga_fim;
ALTER TABLE plantoes DROP COLUMN IF EXISTS folga_inicio;
