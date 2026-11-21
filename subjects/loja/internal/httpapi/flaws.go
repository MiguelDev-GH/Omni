package httpapi

import "net/http"

// Estas rotas imitam arquivos expostos por engano, para que o Nuclei do Omni
// tenha o que encontrar. Os valores são falsos e só ficam acessíveis porque
// config.Load impede a loja de ouvir fora do loopback.

const envFile = "APP_ENV=producao\n" +
	"DB_HOST=127.0.0.1\n" +
	"DB_USER=admin\n" +
	"DB_PASSWORD=FALSO-senha-da-cobaia\n" +
	"AWS_ACCESS_KEY_ID=FALSO-AKIAEXEMPLO\n" +
	"AWS_SECRET_ACCESS_KEY=FALSO-segredo-da-cobaia\n"

const gitConfig = "[core]\n" +
	"\trepositoryformatversion = 0\n" +
	"\tfilemode = true\n" +
	"\tbare = false\n" +
	"[remote \"origin\"]\n" +
	"\turl = https://github.com/exemplo/loja-cobaia.git\n" +
	"\tfetch = +refs/heads/*:refs/remotes/origin/*\n"

func registerFlaws(mux *http.ServeMux) {
	for path, body := range map[string]string{
		"GET /.env":        envFile,
		"GET /.git/config": gitConfig,
		"GET /.git/HEAD":   "ref: refs/heads/main\n",
	} {
		mux.HandleFunc(path, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte(body))
		})
	}
}
