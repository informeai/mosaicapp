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
internal/chunker/         divisão de conteúdo em blocos (content-defined chunking)
internal/store/           persistência em SQLite (arquivos, blocos, referências)
internal/dedup/           serviço que liga chunker + store
app.go, main.go           aplicação Wails (bindings expostos ao frontend)
frontend/                 UI (HTML/CSS/JS puro, empacotado com Vite)
build/appicon.png         ícone-fonte do app (1024x1024); Wails gera .ico/.icns a partir dele
build/windows/installer/  fonte WiX (.wxs) do instalador .msi do Windows
.github/workflows/build.yml  pipeline de build (um job por SO: Linux, macOS, Windows)
```

## Rodando em desenvolvimento

Pré-requisitos: Go 1.23+, Node.js, e a [CLI do Wails](https://wails.io/docs/gettingstarted/installation):

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

No Linux, o Wails também exige as bibliotecas de desenvolvimento do
GTK3/WebKit2GTK (pré-requisito do próprio Wails, não deste projeto). Em
distribuições recentes (ex.: Ubuntu 24.04) o pacote disponível é o
WebKit2GTK 4.1, não o 4.0 padrão do Wails, então é preciso a tag de build
`webkit2_41`:

```bash
sudo apt install build-essential libgtk-3-dev libwebkit2gtk-4.1-dev libsoup-3.0-dev
```

Com tudo instalado:

```bash
wails dev                              # modo desenvolvimento, com hot reload do frontend
wails build -tags webkit2_41           # Linux: gera o binário em build/bin
wails build                            # macOS/Windows: idem, sem a tag
```

## Build de release (CI)

Um único workflow do GitHub Actions (`.github/workflows/build.yml`) compila
o app para as três plataformas, cada uma em seu próprio job/runner
(`build-linux` → `ubuntu-latest`, `build-macos` → `macos-latest`,
`build-windows` → `windows-latest`), rodando em sequência (via `needs`) para
não haver corrida ao criar a mesma Release. É disparado manualmente pela
aba Actions do GitHub (`workflow_dispatch`), na branch `main`, com um campo
obrigatório `version` (ex.: `1.0.0`). Cada job cria (ou atualiza, se já
existir) a Release `vX.Y.Z` e anexa o binário da sua plataforma — ao final,
a Release fica com os três artefatos.

| Plataforma | Saída |
|---|---|
| Linux   | `mosaic-<versão>-linux-amd64.tar.gz` |
| macOS   | `mosaic-<versão>-macos-universal.zip` (binário universal Intel + Apple Silicon, assinado ad-hoc) |
| Windows | `mosaicapp.exe` + `mosaic-<versão>-windows-amd64.msi` |

O instalador do Windows é gerado com o [WiX Toolset v5](https://wixtoolset.org/)
a partir de `build/windows/installer/mosaic.wxs`: instala o app em
`Program Files\Mosaic`, cria o atalho no Menu Iniciar e usa o ícone gerado a
partir de `build/appicon.png`. Para gerar o MSI localmente em uma máquina
Windows com .NET SDK instalado:

```powershell
wails build -platform windows/amd64 -clean
dotnet tool install --global wix
wix build build/windows/installer/mosaic.wxs -arch x64 `
  -d ProductVersion=1.0.0 `
  -d BuildDir=build/bin `
  -d IconPath=build/windows/icon.ico `
  -out build/bin/mosaic-1.0.0-windows-amd64.msi
```

Nenhuma das três plataformas assina/notariza os binários com certificado
pago (macOS: Apple Developer ID; Windows: certificado de assinatura de
código) — sem eles, o macOS mostra o aviso padrão do Gatekeeper e o Windows
o do SmartScreen no primeiro uso.

## Testes

A lógica de chunking, deduplicação e persistência é testada sem depender do
Wails/GUI:

```bash
go test ./...
```
