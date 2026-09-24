package handlers

import "strings"

// celulaCSV neutraliza fórmulas em campos de texto livre das exportações:
// Excel e LibreOffice executam células que começam com =, +, -, @, tab ou CR.
// O apóstrofo faz a planilha tratar o valor como texto literal.
func celulaCSV(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}
