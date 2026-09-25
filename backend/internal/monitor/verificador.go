package monitor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"tiiv/backend/internal/database"
	"tiiv/backend/internal/database/sqlc"
)

const (
	StatusPendente = "pendente"
	StatusOnline   = "online"
	StatusOffline  = "offline"

	// Falhas seguidas até considerar o alvo fora do ar (evita alarme por um
	// pacote perdido)
	FalhasParaQueda = 2

	// De quanto em quanto tempo o laço procura monitores com verificação vencida
	cicloVerificador = 10 * time.Second
	// Verificações simultâneas no máximo
	verificacoesParalelas = 16
)

var fusoSP = func() *time.Location {
	if loc, err := time.LoadLocation("America/Sao_Paulo"); err == nil {
		return loc
	}
	return time.Local
}()

// ProximoEstado calcula status e falhas seguidas depois de uma verificação
func ProximoEstado(statusAtual string, falhasSeguidas int32, sucesso bool) (string, int32) {
	if sucesso {
		return StatusOnline, 0
	}
	falhasSeguidas++
	if falhasSeguidas >= FalhasParaQueda {
		return StatusOffline, falhasSeguidas
	}
	return statusAtual, falhasSeguidas
}

type Verificador struct {
	db *database.DB
}

func NovoVerificador(db *database.DB) *Verificador {
	return &Verificador{db: db}
}

// Rodar verifica os monitores vencidos a cada ciclo até o contexto acabar
func (v *Verificador) Rodar(ctx context.Context) {
	slog.Info("monitor de disponibilidade iniciado")
	ticker := time.NewTicker(cicloVerificador)
	defer ticker.Stop()
	for {
		if err := v.VerificarPendentes(ctx); err != nil && ctx.Err() == nil {
			slog.Error("falha ao listar monitores para verificar", "erro", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// VerificarPendentes checa, em paralelo, todo monitor ativo com verificação
// vencida e espera todos terminarem (o próximo ciclo nunca repete um em curso).
func (v *Verificador) VerificarPendentes(ctx context.Context) error {
	monitores, err := v.db.Queries.ListarMonitoresParaVerificar(ctx)
	if err != nil {
		return err
	}

	vagas := make(chan struct{}, verificacoesParalelas)
	var wg sync.WaitGroup
	for _, m := range monitores {
		wg.Add(1)
		vagas <- struct{}{}
		go func() {
			defer func() { <-vagas; wg.Done() }()
			if _, err := v.VerificarEGravar(ctx, m); err != nil && ctx.Err() == nil {
				slog.Error("falha ao gravar verificação do monitor", "monitor", m.Nome, "erro", err)
			}
		}()
	}
	wg.Wait()
	return nil
}

// VerificarEGravar checa o alvo e grava o resultado. A checagem roda fora da
// transação; a gravação trava a linha e parte do estado atual, então uma
// verificação manual concorrente com o laço não abre queda em dobro.
func (v *Verificador) VerificarEGravar(ctx context.Context, m sqlc.Monitores) (sqlc.Monitores, error) {
	latencia, erroChecagem := Verificar(ctx, m.Tipo, m.Alvo)

	tx, err := v.db.Pool.Begin(ctx)
	if err != nil {
		return m, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	q := v.db.Queries.WithTx(tx)

	atual, err := q.BloquearMonitor(ctx, m.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return m, nil // excluído durante a verificação
	}
	if err != nil {
		return m, err
	}
	// Alvo trocado ou monitor desligado durante a verificação: descarta
	if !atual.Ativo || atual.Tipo != m.Tipo || atual.Alvo != m.Alvo {
		return atual, nil
	}

	status, falhas := ProximoEstado(atual.Status, atual.FalhasSeguidas, erroChecagem == nil)
	params := sqlc.RegistrarVerificacaoParams{ID: atual.ID, Status: status, FalhasSeguidas: falhas}
	if erroChecagem == nil {
		params.LatenciaMs = pgtype.Int4{Int32: int32(max(latencia.Milliseconds(), 1)), Valid: true}
	} else {
		params.UltimoErro = erroChecagem.Error()
	}
	if err := q.RegistrarVerificacao(ctx, params); err != nil {
		return m, err
	}

	switch {
	case status == StatusOffline && atual.Status != StatusOffline:
		if err := abrirQueda(ctx, q, atual, params.UltimoErro); err != nil {
			return m, err
		}
		slog.Warn("monitor fora do ar", "monitor", atual.Nome, "alvo", atual.Alvo, "erro", params.UltimoErro)
	case status == StatusOnline && atual.Status == StatusOffline:
		if err := q.FecharQuedaAberta(ctx, atual.ID); err != nil {
			return m, err
		}
		slog.Info("monitor voltou", "monitor", atual.Nome, "alvo", atual.Alvo)
	}

	if err := tx.Commit(ctx); err != nil {
		return m, err
	}
	return v.db.Queries.ObterMonitor(ctx, m.ID)
}

// abrirQueda registra o início da queda e, se o monitor pede, abre um ticket
// na fila da equipe.
func abrirQueda(ctx context.Context, q *sqlc.Queries, m sqlc.Monitores, erro string) error {
	quedaID, err := q.AbrirQueda(ctx, sqlc.AbrirQuedaParams{MonitorID: m.ID, Erro: erro})
	if err != nil || !m.AbrirTicket {
		return err
	}

	titulo := "Fora do ar: " + m.Nome
	if len([]rune(titulo)) > 200 {
		titulo = string([]rune(titulo)[:200])
	}
	ticket, err := q.CriarTicket(ctx, sqlc.CriarTicketParams{
		SolicitanteNome: "Monitor de disponibilidade",
		Titulo:          titulo,
		Descricao: fmt.Sprintf(
			"O monitor \"%s\" (%s %s) parou de responder em %s.\nErro: %s\n\nTicket aberto automaticamente.",
			m.Nome, m.Tipo, m.Alvo, time.Now().In(fusoSP).Format("02/01/2006 15:04"), erro,
		),
		Prioridade: "alta",
	})
	if err != nil {
		return err
	}
	return q.VincularTicketQueda(ctx, sqlc.VincularTicketQuedaParams{ID: quedaID, TicketID: ticket.ID})
}
