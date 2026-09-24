package embeds

import (
	"crypto/sha256"
	"encoding/base64"
	"io/fs"
	"regexp"
)

// O SvelteKit inicializa a SPA com um <script> inline no index.html; o hash dele
// entra no script-src para a CSP não precisar de 'unsafe-inline'.
var scriptInlineRegex = regexp.MustCompile(`(?s)<script>(.*?)</script>`)

// HashesScriptsInline devolve as fontes CSP ('sha256-...') dos scripts inline do index.html
func HashesScriptsInline() ([]string, error) {
	index, err := fs.ReadFile(DistFS, "dist/index.html")
	if err != nil {
		return nil, err
	}

	var hashes []string
	for _, m := range scriptInlineRegex.FindAllSubmatch(index, -1) {
		soma := sha256.Sum256(m[1])
		hashes = append(hashes, "'sha256-"+base64.StdEncoding.EncodeToString(soma[:])+"'")
	}
	return hashes, nil
}
