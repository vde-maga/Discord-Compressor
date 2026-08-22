package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"io"
	"strings"
)

const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorCyan   = "\033[36m"
	ColorDim    = "\033[2m"
	ColorBold   = "\033[1m"
)

func printHeader(info *MediaInfo, params *CompressionParams) {
	durMin := int(info.Duration) / 60
	durSec := int(info.Duration) % 60

	fmt.Printf("%s🎬 Discord Video Compressor%s\n", ColorCyan, ColorReset)
	fmt.Printf("%s─────────────────────────────────────────%s\n", ColorDim, ColorReset)
	fmt.Printf("%s📁 Ficheiro:%s %s\n", ColorBold, ColorReset, info.FilePath)
	fmt.Printf("%s⏱  Duração:%s  %dm %02ds\n", ColorBold, ColorReset, durMin, durSec)

	audioStr := "Sem áudio"
	if params.AudioChannels == 1 {
		audioStr = fmt.Sprintf("%d kbps (mono)", params.AudioBitrate/1000)
	} else if params.AudioChannels == 2 {
		audioStr = fmt.Sprintf("%d kbps (estéreo)", params.AudioBitrate/1000)
	}
	fmt.Printf("%s🎵 Áudio:%s   %s\n", ColorBold, ColorReset, audioStr)

	codec := "VP9"
	if params.UseAV1 {
		codec = "SVT-AV1"
	}
	fmt.Printf("%s🎞  Vídeo:%s   ~%d kbps (%s / %dp)\n", ColorBold, ColorReset, params.VideoBitrate/1000, codec, params.MaxHeight)
	fmt.Printf("%s─────────────────────────────────────────%s\n", ColorDim, ColorReset)
}

func printProgress(pct float64) {
	barLen := 40
	filled := int(math.Round(float64(barLen) * pct / 100.0))
	empty := barLen - filled

	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	fmt.Printf("\r  %s%s%s  %3.0f%%  ", ColorGreen, bar, ColorReset, pct)
}

func main() {
	targetMB := flag.Int("target-mb", 20, "Limite de tamanho em MB")
	useVP9 := flag.Bool("vp9", false, "Forçar o uso do codec VP9")
	isFast := flag.Bool("fast", false, "Modo rápido (prioriza velocidade)")

	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		fmt.Printf("Uso: %sdiscord-compress%s <ficheiro> [opções]\n", ColorBold, ColorReset)
		fmt.Printf("Exemplo: discord-compress video.mp4 --target-mb 20 --fast\n\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	inputFile := args[0]

	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		fmt.Printf("%s❌ Erro: O ficheiro '%s' não existe.%s\n", ColorRed, inputFile, ColorReset)
		os.Exit(1)
	}

	fmt.Printf("%s⏳ A analisar o ficheiro...%s\n", ColorDim, ColorReset)
	info, err := probeMedia(inputFile)
	if err != nil {
		fmt.Printf("%s❌ Erro ao ler o ficheiro: %v%s\n", ColorRed, err, ColorReset)
		os.Exit(1)
	}

	useAV1 := !*useVP9 && !*isFast
	if useAV1 && !checkAV1Support() {
		fmt.Printf("%s⚠️  libsvtav1 não encontrado. A usar VP9.%s\n", ColorYellow, ColorReset)
		useAV1 = false
	}

	params := calculateParams(info, *targetMB, useAV1)
	if params.VideoBitrate <= 50000 {
		fmt.Printf("%s❌ IMPOSSÍVEL: O vídeo é muito longo para o limite de %d MB.%s\n", ColorRed, *targetMB, ColorReset)
		os.Exit(1)
	}

	// ==============================================================================
	// FAST PATH: Otimização para ficheiros já otimizados (Smart Passthrough)
	// ==============================================================================
	fileInfo, _ := os.Stat(inputFile)
	if fileInfo.Size() <= params.TargetBytes && strings.HasSuffix(strings.ToLower(inputFile), ".webm") {
		fmt.Printf("%s✅ Ficheiro já otimizado e dentro do limite (%.2f MB).%s\n", ColorGreen, float64(fileInfo.Size())/(1024*1024), ColorReset)
		fmt.Printf("%s💡 A copiar diretamente sem recomprimir (perda zero).%s\n", ColorCyan, ColorReset)

		src, err := os.Open(inputFile)
		if err != nil {
			fmt.Printf("%s❌ Erro ao abrir o ficheiro: %v%s\n", ColorRed, err, ColorReset)
			os.Exit(1)
		}
		defer src.Close()

		dst, err := os.Create(params.OutputPath)
		if err != nil {
			fmt.Printf("%s❌ Erro ao criar o ficheiro de destino: %v%s\n", ColorRed, err, ColorReset)
			os.Exit(1)
		}
		defer dst.Close()

		if _, err := io.Copy(dst, src); err != nil {
			fmt.Printf("%s❌ Erro ao copiar o ficheiro: %v%s\n", ColorRed, err, ColorReset)
			os.Exit(1)
		}

		fmt.Printf("%s✅ Sucesso!%s Ficheiro guardado como: %s\n", ColorGreen, ColorReset, params.OutputPath)
		return // Sai imediatamente com sucesso
	}
	// ==============================================================================

	printHeader(info, params)

	maxAttempts := 2
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			fmt.Printf("\n%s⚠️  Excedeu o limite. A tentar novamente com compressão agressiva...%s\n", ColorYellow, ColorReset)
			params.VideoBitrate = int(float64(params.VideoBitrate) * 0.7)
		}

		fmt.Printf("%s⏳ A compactar (Tentativa %d/%d)...%s\n", ColorCyan, attempt, maxAttempts, ColorReset)

		fmt.Printf("%s  > Passo 1/2: A analisar...%s", ColorDim, ColorReset)
		if err := runFFmpegPass(info, params, 1, nil); err != nil {
			fmt.Printf("\n%s❌ Erro no Passo 1: %v%s\n", ColorRed, err, ColorReset)
			os.Exit(1)
		}
		fmt.Printf(" %sConcluído%s\n", ColorGreen, ColorReset)

		fmt.Printf("%s  > Passo 2/2: A codificar:%s\n", ColorDim, ColorReset)
		if err := runFFmpegPass(info, params, 2, printProgress); err != nil {
			fmt.Printf("\n%s❌ Erro no Passo 2: %v%s\n", ColorRed, err, ColorReset)
			os.Exit(1)
		}
		fmt.Println()

		fileInfo, _ := os.Stat(params.OutputPath)
		if fileInfo != nil && fileInfo.Size() <= params.TargetBytes {
			finalMB := float64(fileInfo.Size()) / (1024 * 1024)
			fmt.Printf("%s─────────────────────────────────────────%s\n", ColorDim, ColorReset)
			fmt.Printf("%s✅ Sucesso!%s Ficheiro final: %s%.2f MB%s\n", ColorGreen, ColorReset, ColorBold, finalMB, ColorReset)
			fmt.Printf("%s💾 Guardado como:%s %s\n", ColorBold, ColorReset, params.OutputPath)

			os.Remove("ffmpeg2pass-0.log")
			os.Remove("ffmpeg2pass-0.log.mbtree")
			return
		}

		os.Remove("ffmpeg2pass-0.log")
		os.Remove("ffmpeg2pass-0.log.mbtree")
	}

	fmt.Printf("%s❌ Falha: Não foi possível comprimir para o tamanho alvo após %d tentativas.%s\n", ColorRed, maxAttempts, ColorReset)
	os.Exit(1)
}