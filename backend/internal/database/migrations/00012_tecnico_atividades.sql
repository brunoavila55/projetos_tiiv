-- +goose Up
-- O que o técnico fez na visita: anotado durante o dia ou ao marcar a saída.
ALTER TABLE tecnico_registros ADD COLUMN atividades TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE tecnico_registros DROP COLUMN IF EXISTS atividades;
