package embeds

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed dist/*
var DistFS embed.FS

// SPAHandler retorna um http.Handler que serve a SPA estática embutida com fallback para index.html
func SPAHandler() (http.Handler, error) {
	sub, err := fs.Sub(DistFS, "dist")
	if err != nil {
		return nil, err
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// Se o arquivo existe no FS e não é diretório, serve o arquivo estático
		f, err := sub.Open(path)
		if err == nil {
			stat, err := f.Stat()
			_ = f.Close()
			if err == nil && !stat.IsDir() {
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// Se não encontrou (rotas do frontend como /estoque, /tarefas), serve o index.html
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	}), nil
}
