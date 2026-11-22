package orchestrator

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/MiguelDev-GH/omni/internal/arsenal"
)

const (
	defaultSocket = "/var/run/docker.sock"
	// maxStdout limita a saída padrão guardada para o parser.
	maxStdout = 32 << 20
	nanoCPUs  = 1_000_000_000 // 1 CPU
	memLimit  = 512 << 20     // 512 MiB
	pidsLimit = 512
)

// DockerRunner roda as ferramentas em containers efêmeros pela API do
// Docker Engine no socket Unix. Cada container tem rede do host, 1 CPU,
// 512 MiB (ou o limite da Spec) e é sempre removido no fim, inclusive em
// erro ou cancelamento.
type DockerRunner struct {
	c *http.Client
}

// NewDockerRunner usa o socket de DOCKER_HOST (só unix://) ou o padrão
// /var/run/docker.sock. Não conecta: Available diz se o Docker responde.
func NewDockerRunner() (*DockerRunner, error) {
	sock := defaultSocket
	if h := os.Getenv("DOCKER_HOST"); h != "" {
		p, ok := strings.CutPrefix(h, "unix://")
		if !ok {
			return nil, fmt.Errorf("DOCKER_HOST %q não suportado; use unix://", h)
		}
		sock = p
	}
	tr := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", sock)
		},
		Proxy: nil,
	}
	return &DockerRunner{c: &http.Client{Transport: tr}}, nil
}

func (d *DockerRunner) do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	if body == nil {
		return d.send(ctx, method, path, "", nil)
	}
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return d.send(ctx, method, path, "application/json", b)
}

// send faz a requisição com um corpo já pronto, do tipo contentType.
func (d *DockerRunner) send(ctx context.Context, method, path, contentType string, body []byte) (*http.Response, error) {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://docker"+path, r)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := d.c.Do(req)
	if err != nil {
		var opErr *net.OpError
		if errors.As(err, &opErr) && opErr.Op == "dial" {
			return nil, fmt.Errorf("%w: %v", ErrDockerIndisponivel, err)
		}
		return nil, err
	}
	return resp, nil
}

// apiError lê a mensagem de erro de uma resposta da API e fecha o corpo.
func apiError(resp *http.Response, what string) error {
	defer func() { _ = resp.Body.Close() }()
	var e struct {
		Message string `json:"message"`
	}
	_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&e)
	return fmt.Errorf("docker: %s: HTTP %d: %s", what, resp.StatusCode, e.Message)
}

// Available confirma que o daemon responde.
func (d *DockerRunner) Available(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	resp, err := d.do(ctx, http.MethodGet, "/_ping", nil)
	if err != nil {
		if errors.Is(err, ErrDockerIndisponivel) {
			return err
		}
		return fmt.Errorf("%w: %v", ErrDockerIndisponivel, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: _ping respondeu HTTP %d", ErrDockerIndisponivel, resp.StatusCode)
	}
	return nil
}

// Run baixa a imagem se preciso, cria e inicia o container, copia stdout e
// stderr para logs e devolve a saída padrão.
func (d *DockerRunner) Run(ctx context.Context, s Spec, logs io.Writer) ([]byte, error) {
	archive, err := tarFiles(s.Files)
	if err != nil {
		return nil, err
	}
	if err := d.ensureImage(ctx, s.Image, logs); err != nil {
		return nil, err
	}
	id, err := d.create(ctx, s)
	if err != nil {
		return nil, err
	}
	defer d.remove(id)

	if archive != nil {
		resp, err := d.send(ctx, http.MethodPut, "/containers/"+id+"/archive?path=/", "application/x-tar", archive)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			return nil, apiError(resp, "copiar arquivos para o container")
		}
		_ = resp.Body.Close()
	}

	resp, err := d.do(ctx, http.MethodPost, "/containers/"+id+"/start", nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotModified {
		return nil, apiError(resp, "iniciar container")
	}
	_ = resp.Body.Close()

	stdout, err := d.follow(ctx, id, logs)
	if err != nil {
		return nil, err
	}
	code, err := d.wait(ctx, id)
	if err != nil {
		return nil, err
	}
	if s.OutputFile != "" {
		file, found, err := d.readFile(ctx, id, s.OutputFile, maxStdout-len(stdout))
		if err != nil {
			return nil, err
		}
		if found {
			stdout = append(append(stdout, arsenal.OutputFileMarker...), file...)
		}
	}
	if code != 0 && !s.AnyExit {
		return stdout, fmt.Errorf("a ferramenta terminou com código %d", code)
	}
	return stdout, nil
}

func (d *DockerRunner) ensureImage(ctx context.Context, image string, logs io.Writer) error {
	resp, err := d.do(ctx, http.MethodGet, "/images/"+image+"/json", nil)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	_, _ = fmt.Fprintf(logs, "Baixando a imagem %s...\n", image)
	resp, err = d.do(ctx, http.MethodPost, "/images/create?fromImage="+url.QueryEscape(image), nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return apiError(resp, "baixar imagem")
	}
	defer func() { _ = resp.Body.Close() }()
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		var msg struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(sc.Bytes(), &msg) == nil && msg.Error != "" {
			return fmt.Errorf("docker: baixar imagem %s: %s", image, msg.Error)
		}
	}
	return sc.Err()
}

type mount struct {
	Type   string `json:"Type"`
	Source string `json:"Source"`
	Target string `json:"Target"`
}

func (d *DockerRunner) create(ctx context.Context, s Spec) (string, error) {
	mounts := []mount{}
	for name, path := range s.Volumes {
		mounts = append(mounts, mount{Type: "volume", Source: name, Target: path})
	}
	mem := s.Memory
	if mem <= 0 {
		mem = memLimit
	}
	body := map[string]any{
		"Image":  s.Image,
		"Cmd":    s.Args, // lista de argumentos, nunca uma linha de shell
		"Labels": map[string]string{"omni.arsenal": "1"},
		"HostConfig": map[string]any{
			"NetworkMode": "host",
			"NanoCpus":    nanoCPUs,
			"Memory":      mem,
			"MemorySwap":  mem,
			"PidsLimit":   pidsLimit,
			"SecurityOpt": []string{"no-new-privileges"},
			"Mounts":      mounts,
		},
	}
	resp, err := d.do(ctx, http.MethodPost, "/containers/create?name=omni-arsenal-"+randomID(), body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusCreated {
		return "", apiError(resp, "criar container")
	}
	defer func() { _ = resp.Body.Close() }()
	var out struct {
		ID string `json:"Id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.ID, nil
}

// follow lê o fluxo multiplexado de logs até o container terminar.
func (d *DockerRunner) follow(ctx context.Context, id string, logs io.Writer) ([]byte, error) {
	resp, err := d.do(ctx, http.MethodGet, "/containers/"+id+"/logs?follow=true&stdout=true&stderr=true", nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, apiError(resp, "ler saída")
	}
	defer func() { _ = resp.Body.Close() }()
	var stdout bytes.Buffer
	hdr := make([]byte, 8)
	for {
		if _, err := io.ReadFull(resp.Body, hdr); err != nil {
			if errors.Is(err, io.EOF) {
				return stdout.Bytes(), nil
			}
			return nil, err
		}
		n := int64(binary.BigEndian.Uint32(hdr[4:]))
		if hdr[0] == 1 {
			if int64(stdout.Len())+n > maxStdout {
				return nil, errors.New("a saída da ferramenta passou de 32 MiB")
			}
			if _, err := io.CopyN(io.MultiWriter(&stdout, logs), resp.Body, n); err != nil {
				return nil, err
			}
			continue
		}
		if _, err := io.CopyN(logs, resp.Body, n); err != nil {
			return nil, err
		}
	}
}

func (d *DockerRunner) wait(ctx context.Context, id string) (int, error) {
	resp, err := d.do(ctx, http.MethodPost, "/containers/"+id+"/wait", nil)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return 0, apiError(resp, "esperar container")
	}
	defer func() { _ = resp.Body.Close() }()
	var out struct {
		StatusCode int `json:"StatusCode"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, err
	}
	return out.StatusCode, nil
}

// readFile lê um arquivo do container parado pelo tar de GET
// /containers/{id}/archive; found é falso quando o arquivo não existe.
func (d *DockerRunner) readFile(ctx context.Context, id, name string, limit int) (b []byte, found bool, err error) {
	resp, err := d.do(ctx, http.MethodGet, "/containers/"+id+"/archive?path="+url.QueryEscape(name), nil)
	if err != nil {
		return nil, false, err
	}
	if resp.StatusCode == http.StatusNotFound {
		_ = resp.Body.Close()
		return nil, false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, apiError(resp, "ler arquivo de saída")
	}
	defer func() { _ = resp.Body.Close() }()
	tr := tar.NewReader(resp.Body)
	for {
		hdr, err := tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, false, nil
			}
			return nil, false, err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if hdr.Size > int64(limit) {
			return nil, false, errors.New("a saída da ferramenta passou de 32 MiB")
		}
		b, err := io.ReadAll(tr)
		return b, err == nil, err
	}
}

// remove apaga o container mesmo depois de ctx cancelado.
func (d *DockerRunner) remove(id string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := d.do(ctx, http.MethodDelete, "/containers/"+id+"?force=true&v=true", nil)
	if err == nil {
		_ = resp.Body.Close()
	}
}

// tarFiles monta o tar extraído na raiz do container, com os diretórios
// pais e os arquivos somente leitura; nil quando não há arquivos. Recusa
// caminho relativo ou com "..".
func tarFiles(files map[string][]byte) ([]byte, error) {
	if len(files) == 0 {
		return nil, nil
	}
	names := make([]string, 0, len(files))
	for name := range files {
		if !path.IsAbs(name) || path.Clean(name) != name || name == "/" || strings.Contains(name, "..") {
			return nil, fmt.Errorf("caminho de arquivo inválido para o container: %q", name)
		}
		names = append(names, name)
	}
	sort.Strings(names)
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	dirs := map[string]bool{}
	for _, name := range names {
		var parents []string
		for dir := path.Dir(name); dir != "/"; dir = path.Dir(dir) {
			parents = append([]string{dir}, parents...)
		}
		for _, dir := range parents {
			if dirs[dir] {
				continue
			}
			dirs[dir] = true
			hdr := &tar.Header{Typeflag: tar.TypeDir, Name: dir[1:] + "/", Mode: 0o555, ModTime: time.Unix(0, 0)}
			if err := tw.WriteHeader(hdr); err != nil {
				return nil, err
			}
		}
		b := files[name]
		hdr := &tar.Header{Typeflag: tar.TypeReg, Name: name[1:], Mode: 0o444, Size: int64(len(b)), ModTime: time.Unix(0, 0)}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := tw.Write(b); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func randomID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
