-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "unaccent";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE usuarios (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    nome VARCHAR(255) NOT NULL,
    cor VARCHAR(7) NOT NULL,
    pin_hash TEXT NOT NULL,
    papel VARCHAR(20) NOT NULL CHECK (papel IN ('admin', 'usuario')),
    ativo BOOLEAN NOT NULL DEFAULT true,
    tentativas_falhas INT NOT NULL DEFAULT 0,
    bloqueado_ate TIMESTAMPTZ,
    criado_em TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE sessoes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash TEXT NOT NULL UNIQUE,
    usuario_id UUID NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
    expira_em TIMESTAMPTZ NOT NULL,
    ultimo_uso_em TIMESTAMPTZ NOT NULL DEFAULT now(),
    criado_em TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_sessoes_usuario_id ON sessoes(usuario_id);
CREATE INDEX idx_sessoes_token_hash ON sessoes(token_hash);
CREATE INDEX idx_sessoes_expira_em ON sessoes(expira_em);

CREATE TABLE eventos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    titulo VARCHAR(255) NOT NULL,
    descricao TEXT NOT NULL DEFAULT '',
    inicio TIMESTAMPTZ NOT NULL,
    fim TIMESTAMPTZ NOT NULL,
    dia_inteiro BOOLEAN NOT NULL DEFAULT false,
    criado_por UUID NOT NULL REFERENCES usuarios(id) ON DELETE RESTRICT,
    criado_em TIMESTAMPTZ NOT NULL DEFAULT now(),
    atualizado_em TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_evento_fim_maior_igual_inicio CHECK (fim >= inicio)
);

CREATE INDEX idx_eventos_criado_por ON eventos(criado_por);
CREATE INDEX idx_eventos_inicio_fim ON eventos(inicio, fim);

CREATE TABLE evento_participantes (
    evento_id UUID NOT NULL REFERENCES eventos(id) ON DELETE CASCADE,
    usuario_id UUID NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
    PRIMARY KEY (evento_id, usuario_id)
);

CREATE INDEX idx_evento_participantes_usuario ON evento_participantes(usuario_id);

CREATE TABLE atendimentos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cliente_nome VARCHAR(255) NOT NULL,
    descricao TEXT NOT NULL,
    data_atendimento TIMESTAMPTZ NOT NULL DEFAULT now(),
    usuario_id UUID NOT NULL REFERENCES usuarios(id) ON DELETE RESTRICT,
    criado_em TIMESTAMPTZ NOT NULL DEFAULT now(),
    atualizado_em TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_atendimentos_usuario_id ON atendimentos(usuario_id);
CREATE INDEX idx_atendimentos_data ON atendimentos(data_atendimento DESC);

CREATE TABLE tarefas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    titulo VARCHAR(255) NOT NULL,
    descricao TEXT NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'pendente' CHECK (status IN ('pendente', 'em_andamento', 'concluida')),
    prioridade VARCHAR(20) NOT NULL DEFAULT 'media' CHECK (prioridade IN ('baixa', 'media', 'alta')),
    prazo TIMESTAMPTZ,
    criado_por UUID NOT NULL REFERENCES usuarios(id) ON DELETE RESTRICT,
    responsavel_id UUID REFERENCES usuarios(id) ON DELETE SET NULL,
    concluida_em TIMESTAMPTZ,
    criado_em TIMESTAMPTZ NOT NULL DEFAULT now(),
    atualizado_em TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_tarefas_criado_por ON tarefas(criado_por);
CREATE INDEX idx_tarefas_responsavel_id ON tarefas(responsavel_id);
CREATE INDEX idx_tarefas_status_prazo ON tarefas(status, prazo);

CREATE TABLE itens_estoque (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    nome VARCHAR(255) NOT NULL UNIQUE,
    unidade VARCHAR(20) NOT NULL,
    categoria VARCHAR(100) NOT NULL DEFAULT 'Geral',
    estoque_minimo INT NOT NULL DEFAULT 0,
    saldo INT NOT NULL DEFAULT 0,
    ativo BOOLEAN NOT NULL DEFAULT true,
    criado_em TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_saldo_nao_negativo CHECK (saldo >= 0)
);

CREATE INDEX idx_itens_estoque_categoria ON itens_estoque(categoria);
CREATE INDEX idx_itens_estoque_ativo ON itens_estoque(ativo);

CREATE TABLE movimentacoes_estoque (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id UUID NOT NULL REFERENCES itens_estoque(id) ON DELETE RESTRICT,
    tipo VARCHAR(20) NOT NULL CHECK (tipo IN ('entrada', 'saida', 'ajuste')),
    quantidade INT NOT NULL,
    saldo_resultante INT NOT NULL,
    motivo TEXT NOT NULL DEFAULT '',
    usuario_id UUID NOT NULL REFERENCES usuarios(id) ON DELETE RESTRICT,
    criado_em TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_saldo_resultante_nao_negativo CHECK (saldo_resultante >= 0)
);

CREATE INDEX idx_movimentacoes_item_id ON movimentacoes_estoque(item_id);
CREATE INDEX idx_movimentacoes_usuario_id ON movimentacoes_estoque(usuario_id);
CREATE INDEX idx_movimentacoes_criado_em ON movimentacoes_estoque(criado_em DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS movimentacoes_estoque;
DROP TABLE IF EXISTS itens_estoque;
DROP TABLE IF EXISTS tarefas;
DROP TABLE IF EXISTS atendimentos;
DROP TABLE IF EXISTS evento_participantes;
DROP TABLE IF EXISTS eventos;
DROP TABLE IF EXISTS sessoes;
DROP TABLE IF EXISTS usuarios;
-- +goose StatementEnd
