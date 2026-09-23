-- +goose Up
-- Migration para recursos P3 (Desejáveis): Tema escuro, Recorrência em Eventos e Comentários em Tarefas

ALTER TABLE usuarios 
ADD COLUMN tema VARCHAR(20) NOT NULL DEFAULT 'sistema' CHECK (tema IN ('claro', 'escuro', 'sistema'));

ALTER TABLE eventos 
ADD COLUMN recorrencia VARCHAR(20) NOT NULL DEFAULT 'nenhuma' CHECK (recorrencia IN ('nenhuma', 'semanal', 'mensal')),
ADD COLUMN recorrencia_fim TIMESTAMPTZ;

CREATE TABLE tarefa_comentarios (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tarefa_id UUID NOT NULL REFERENCES tarefas(id) ON DELETE CASCADE,
    usuario_id UUID NOT NULL REFERENCES usuarios(id) ON DELETE RESTRICT,
    conteudo TEXT NOT NULL,
    criado_em TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_tarefa_comentarios_tarefa ON tarefa_comentarios(tarefa_id);
CREATE INDEX idx_tarefa_comentarios_criado_em ON tarefa_comentarios(criado_em);

-- +goose Down
DROP TABLE IF EXISTS tarefa_comentarios;
ALTER TABLE eventos DROP COLUMN IF EXISTS recorrencia_fim;
ALTER TABLE eventos DROP COLUMN IF EXISTS recorrencia;
ALTER TABLE usuarios DROP COLUMN IF EXISTS tema;
