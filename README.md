# mosaicapp

Utilitário desktop (Go + [Wails](https://wails.io)) para deduplicação de arquivos,
100% offline, com armazenamento em SQLite local.

## Como funciona

O app divide cada arquivo enviado em blocos de tamanho variável usando
*content-defined chunking* (um gear hash no estilo FastCDC): os pontos de
corte dependem do conteúdo, não de offsets fixos, então trechos idênticos —
mesmo em arquivos diferentes ou em posições diferentes — geram blocos
idênticos. Cada bloco é identificado pelo seu hash SHA-256 e guardado uma
única vez na tabela `chunks`; o arquivo em si é apenas uma lista ordenada de
referências a esses blocos (tabela `file_chunks`). É esse "mosaico" de
blocos compartilhados entre arquivos que produz a deduplicação.

- **Enviar arquivo**: abre um seletor nativo, quebra o arquivo em blocos,
  grava no SQLite só os blocos ainda não existentes e cria o registro do
  arquivo.
- **Listar arquivos**: mostra os arquivos já enviados, com o tamanho, número
  de blocos e quanto foi economizado por deduplicação.
- **Reconstruir e baixar**: no ícone de cada card, remonta o arquivo a
  partir dos blocos (na ordem original) e abre o diálogo nativo de salvar.

O banco SQLite fica em `~/.config/mosaicapp/mosaic.db` (Linux),
`~/Library/Application Support/mosaicapp/mosaic.db` (macOS) ou
`%AppData%\mosaicapp\mosaic.db` (Windows) — não depende de rede.

## Estrutura

```
internal/chunker/  divisão de conteúdo em blocos (content-defined chunking)
internal/store/    persistência em SQLite (arquivos, blocos, referências)
internal/dedup/    serviço que liga chunker + store
app.go, main.go    aplicação Wails (bindings expostos ao frontend)
frontend/          UI (HTML/CSS/JS puro, empacotado com Vite)
```

## Rodando em desenvolvimento

Pré-requisitos: Go 1.23+, Node.js, e a [CLI do Wails](https://wails.io/docs/gettingstarted/installation):

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

No Linux, o Wails também exige as bibliotecas de desenvolvimento do
GTK3/WebKit2GTK (pré-requisito do próprio Wails, não deste projeto):

```bash
sudo apt install build-essential libgtk-3-dev libwebkit2gtk-4.1-dev
```

Com tudo instalado:

```bash
wails dev     # modo desenvolvimento, com hot reload do frontend
wails build   # gera o binário de produção em build/bin
```

## Testes

A lógica de chunking, deduplicação e persistência é testada sem depender do
Wails/GUI:

```bash
go test ./...
```
