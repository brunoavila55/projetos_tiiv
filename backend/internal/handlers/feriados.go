package handlers

import (
	"sort"
	"time"
)

// Feriados que o plantão interno cobre (junto com os domingos): os nacionais,
// a Sexta-feira Santa e o 20 de setembro do RS. Carnaval e Corpus Christi são
// ponto facultativo e ficam de fora; feriado municipal também.

type Feriado struct {
	Dia  string `json:"dia"` // AAAA-MM-DD
	Nome string `json:"nome"`
}

var feriadosFixos = []struct {
	mes  time.Month
	dia  int
	nome string
}{
	{time.January, 1, "Confraternização Universal"},
	{time.April, 21, "Tiradentes"},
	{time.May, 1, "Dia do Trabalho"},
	{time.September, 7, "Independência"},
	{time.September, 20, "Revolução Farroupilha"},
	{time.October, 12, "Nossa Senhora Aparecida"},
	{time.November, 2, "Finados"},
	{time.November, 15, "Proclamação da República"},
	{time.November, 20, "Consciência Negra"},
	{time.December, 25, "Natal"},
}

// pascoa: domingo de Páscoa pelo algoritmo de Meeus/Jones/Butcher
func pascoa(ano int) time.Time {
	a := ano % 19
	b, c := ano/100, ano%100
	d, e := b/4, b%4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	h := (19*a + b - d - g + 15) % 30
	i, k := c/4, c%4
	l := (32 + 2*e + 2*i - h - k) % 7
	m := (a + 11*h + 22*l) / 451
	mes := (h + l - 7*m + 114) / 31
	dia := (h+l-7*m+114)%31 + 1
	return time.Date(ano, time.Month(mes), dia, 0, 0, 0, 0, time.UTC)
}

// feriadosDoAno: dia (meia-noite UTC, como os dias da escala) → nome
func feriadosDoAno(ano int) map[time.Time]string {
	res := make(map[time.Time]string, len(feriadosFixos)+1)
	for _, f := range feriadosFixos {
		res[time.Date(ano, f.mes, f.dia, 0, 0, 0, 0, time.UTC)] = f.nome
	}
	res[pascoa(ano).AddDate(0, 0, -2)] = "Sexta-feira Santa"
	return res
}

func ehFeriado(dia time.Time) bool {
	_, ok := feriadosDoAno(dia.Year())[dia]
	return ok
}

// diaDePlantaoInterno: o plantão interno é aos domingos e feriados
func diaDePlantaoInterno(dia time.Time) bool {
	return dia.Weekday() == time.Sunday || ehFeriado(dia)
}

// feriadosEntre lista os feriados de de a ate (inclusivos), em ordem
func feriadosEntre(de, ate time.Time) []Feriado {
	res := []Feriado{}
	for ano := de.Year(); ano <= ate.Year(); ano++ {
		for dia, nome := range feriadosDoAno(ano) {
			if !dia.Before(de) && !dia.After(ate) {
				res = append(res, Feriado{Dia: dia.Format(formatoDia), Nome: nome})
			}
		}
	}
	sort.Slice(res, func(i, j int) bool { return res[i].Dia < res[j].Dia })
	return res
}
