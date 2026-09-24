-- +goose Up
-- deve_trocar_pin: o admin criado no bootstrap precisa trocar o PIN no primeiro acesso.
-- ultima_tentativa_em: tentativas antigas (mais de 15 min) deixam de contar para o bloqueio.
-- bloqueios: bloqueios seguidos sem login correto, para o bloqueio progressivo.
ALTER TABLE usuarios
    ADD COLUMN deve_trocar_pin BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN ultima_tentativa_em TIMESTAMPTZ,
    ADD COLUMN bloqueios INT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE usuarios
    DROP COLUMN IF EXISTS bloqueios,
    DROP COLUMN IF EXISTS ultima_tentativa_em,
    DROP COLUMN IF EXISTS deve_trocar_pin;
