package handlers

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"

	"tiiv/backend/internal/database/sqlc"
)

type resultadoPin int

const (
	pinCorreto resultadoPin = iota
	pinIncorreto
	pinBloqueou    // esta tentativa errada atingiu o limite e bloqueou a conta
	pinJaBloqueado // conta já estava bloqueada; o PIN nem foi comparado
	pinUsuarioInvalido
)

// conferirPin reserva a tentativa no banco antes de rodar o bcrypt (ver
// ReservarTentativaPin) e zera o contador em caso de acerto. Serve ao login e
// à troca do próprio PIN, que compartilham o mesmo limite de tentativas.
func conferirPin(ctx context.Context, q *sqlc.Queries, uID pgtype.UUID, pin string) (sqlc.ReservarTentativaPinRow, resultadoPin, time.Time, error) {
	user, err := q.ReservarTentativaPin(ctx, uID)
	if errors.Is(err, pgx.ErrNoRows) {
		atual, err := q.BuscarUsuarioPorID(ctx, uID)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && !atual.Ativo) {
			return user, pinUsuarioInvalido, time.Time{}, nil
		}
		if err != nil {
			return user, 0, time.Time{}, err
		}
		return user, pinJaBloqueado, atual.BloqueadoAte.Time, nil
	}
	if err != nil {
		return user, 0, time.Time{}, err
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PinHash), []byte(pin)) != nil {
		if user.BloqueadoAte.Valid {
			return user, pinBloqueou, user.BloqueadoAte.Time, nil
		}
		return user, pinIncorreto, time.Time{}, nil
	}

	if err := q.ZerarTentativasFalhas(ctx, uID); err != nil {
		return user, 0, time.Time{}, err
	}
	return user, pinCorreto, time.Time{}, nil
}

// minutosAte arredonda para cima o tempo restante de bloqueio (mínimo 1)
func minutosAte(t time.Time) string {
	m := int(math.Ceil(time.Until(t).Minutes()))
	if m < 1 {
		m = 1
	}
	return fmt.Sprintf("%d minuto(s)", m)
}
