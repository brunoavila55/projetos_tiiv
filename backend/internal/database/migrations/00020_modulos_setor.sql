-- +goose Up
-- Módulos por setor: o superadmin desliga as funcionalidades que um setor não
-- usa (ex.: estoque no Suporte). Guarda-se só o que está desligado, então
-- todo setor começa com tudo ligado.

ALTER TABLE setores ADD COLUMN modulos_desativados TEXT[] NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE setores DROP COLUMN IF EXISTS modulos_desativados;
