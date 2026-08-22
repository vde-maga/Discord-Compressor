# 🎬 Discord Video Compressor

Uma ferramenta de linha de comandos (CLI) leve, rápida e universal para comprimir vídeos para os limites de tamanho do Discord (WebM com AV1/VP9 + Opus).

## ✨ Funcionalidades

- **Cálculo Dinâmico de Bitrate:** Ajusta automaticamente a resolução e o bitrate para garantir que o vídeo final respeita o limite de tamanho (ex: 20MB, 50MB).
- **Codecs Modernos:** Utiliza \`libsvtav1\` (AV1) por padrão para máxima eficiência, com fallback para \`libvpx-vp9\` se o AV1 não estiver disponível ou se for solicitado (\`--fast\`).
- **2-Pass Encoding:** Garante a melhor qualidade possível dentro do limite de tamanho estrito.
- **Zero Bloat:** Escrito em Go puro (apenas biblioteca padrão). O binário tem ~1.5MB e não requer runtime.
- **Reprodutibilidade:** Ambiente de desenvolvimento e build totalmente declarativos via Nix Flakes.

## 📋 Pré-requisitos

A ferramenta é um wrapper inteligente, pelo que depende do \`ffmpeg\` (que inclui o \`ffprobe\`) estar instalado no teu sistema com suporte aos codecs \`libsvtav1\` e \`libvpx-vp9\`.

## 🚀 Instalação

### Opção 1: Binários Pré-compilados (Recomendado)
Faz download da versão mais recente na página de [Releases](https://github.com/vde-maga/Discord-Compressor/releases).

### Opção 2: Nix
\`\`\`bash
nix run github:vde-maga/Discord-Compressor -- <video.mp4>
\`\`\`

### Opção 3: Compilar a partir do código fonte
\`\`\`bash
git clone https://github.com/vde-maga/Discord-Compressor.git
cd Discord-Compressor
go build -ldflags="-s -w" -o discord-compress .
\`\`\`

## 💻 Utilização

\`\`\`bash
# Compressão padrão (AV1, limite de 20MB)
./discord-compress video.mp4

# Compressão rápida (VP9)
./discord-compress video.mp4 --fast

# Definir um limite de tamanho personalizado (ex: 50MB)
./discord-compress video.mp4 --target-mb 50
\`\`\`

## 📄 Licença
Distribuído sob a licença MIT. Ver \`LICENSE\` para mais informações.
