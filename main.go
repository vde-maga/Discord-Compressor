package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// ==============================================================================
// 1. DOMAIN MODELS (Lógica Pura e Estruturas de Dados)
// ==============================================================================

type MediaInfo struct {
	Duration   float64
	HasAudio   bool
	Width      int
	Height     int
	FPS        float64
	FilePath   string
}

type CompressionParams struct {
	TargetMB       int
	TargetBytes    int64
	VideoBitrate   int
	AudioBitrate   int
	AudioChannels  int
	MaxWidth       int
	MaxHeight      int
	UseAV1         bool
	OutputPath     string
}

// ==============================================================================
// 2. INFRASTRUCTURE: PROBER (Interação com ffprobe - SRP)
// ==============================================================================

type ffprobeFormat struct {
	Duration string `json:"duration"`
}

type ffprobeStream struct {
	CodecType  string `json:"codec_type"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	RFrameRate string `json:"r_frame_rate"`
}

type ffprobeOutput struct {
	Format  ffprobeFormat   `json:"format"`
	Streams []ffprobeStream `json:"streams"`
}

func probeMedia(filePath string) (*MediaInfo, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", 
		"format=duration:stream=codec_type,width,height,r_frame_rate", 
		"-of", "json", filePath)
	
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe falhou: %w", err)
	}

	var probe ffprobeOutput
	if err := json.Unmarshal(out, &probe); err != nil {
		return nil, fmt.Errorf("erro ao parsear JSON do ffprobe: %w", err)
	}

	duration, _ := strconv.ParseFloat(probe.Format.Duration, 64)
	if duration <= 0 {
		return nil, fmt.Errorf("duração inválida ou zero")
	}

	info := &MediaInfo{
		Duration: duration,
		FilePath: filePath,
	}

	for _, s := range probe.Streams {
		if s.CodecType == "audio" {
			info.HasAudio = true
		}
		if s.CodecType == "video" {
			info.Width = s.Width
			info.Height = s.Height
			// Parse FPS (ex: "30/1" ou "24000/1001")
			if parts := strings.Split(s.RFrameRate, "/"); len(parts) == 2 {
				num, _ := strconv.ParseFloat(parts[0], 64)
				den, _ := strconv.ParseFloat(parts[1], 64)
				if den > 0 {
					info.FPS = num / den
				}
			}
		}
	}

	return info, nil
}

func checkAV1Support() bool {
	cmd := exec.Command("ffmpeg", "-hide_banner", "-encoders")
	out, _ := cmd.Output()
	return strings.Contains(string(out), "libsvtav1")
}

// ==============================================================================
// 3. DOMAIN: CALCULATOR (Regras de Negócio - SRP & DRY)
// ==============================================================================

func calculateParams(info *MediaInfo, targetMB int, useAV1 bool) *CompressionParams {
	targetBytes := int64(float64(targetMB) * 1024 * 1024 * 0.95) // 5% margem
	targetBits := float64(targetBytes) * 8

	// Lógica de Áudio (DRY)
	audioBitrate := 96000
	audioChannels := 2
	if info.HasAudio {
		if info.Duration > 1800 {
			audioBitrate, audioChannels = 32000, 1
		} else if info.Duration > 1200 {
			audioBitrate, audioChannels = 48000, 2
		} else if info.Duration > 600 {
			audioBitrate, audioChannels = 64000, 2
		}
	} else {
		audioBitrate, audioChannels = 0, 0
	}

	audioBitsTotal := float64(audioBitrate) * info.Duration
	videoBitrateRaw := (targetBits - audioBitsTotal) / info.Duration

	// Lógica de Resolução Adaptativa
	maxW, maxH := 1280, 720
	if videoBitrateRaw < 800000 {
		maxW, maxH = 854, 480
	}
	if videoBitrateRaw < 300000 {
		maxW, maxH = 640, 360
	}

	baseName := strings.TrimSuffix(filepath.Base(info.FilePath), filepath.Ext(info.FilePath))
	outPath := fmt.Sprintf("%s.discord.webm", baseName)

	return &CompressionParams{
		TargetMB:      targetMB,
		TargetBytes:   targetBytes,
		VideoBitrate:  int(math.Max(50000, videoBitrateRaw)), // Mínimo 50kbps
		AudioBitrate:  audioBitrate,
		AudioChannels: audioChannels,
		MaxWidth:      maxW,
		MaxHeight:     maxH,
		UseAV1:        useAV1,
		OutputPath:    outPath,
	}
}

// ==============================================================================
// 4. INFRASTRUCTURE: ENCODER (Interação com ffmpeg)
// ==============================================================================

func buildFFmpegArgs(info *MediaInfo, params *CompressionParams, pass int) []string {
	args := []string{"-hide_banner", "-y", "-i", info.FilePath}
	
	// Filtros de Vídeo
	vf := fmt.Sprintf("scale=w='min(%d,iw)':h='min(%d,ih)':force_original_aspect_ratio=decrease:flags=lanczos:force_divisible_by=2,setsar=1", params.MaxWidth, params.MaxHeight)
	if info.FPS > 30 {
		vf += ",fps=30"
	}

	if params.UseAV1 {
		args = append(args, "-c:v", "libsvtav1", "-preset", "8", "-svtav1-params", "tune=0:film-grain=0")
	} else {
		args = append(args, "-c:v", "libvpx-vp9", "-cpu-used", "3", "-row-mt", "1")
	}
	
	args = append(args, "-b:v", strconv.Itoa(params.VideoBitrate), "-vf", vf)

	if pass == 1 {
		args = append(args, "-an", "-pass", "1", "-f", "null", os.DevNull)
	} else {
		if params.AudioChannels > 0 {
			args = append(args, "-c:a", "libopus", "-b:a", strconv.Itoa(params.AudioBitrate), "-ac", strconv.Itoa(params.AudioChannels))
		} else {
			args = append(args, "-an")
		}
		args = append(args, "-pass", "2", "-progress", "pipe:1", params.OutputPath)
	}

	return args
}

func runFFmpegPass(info *MediaInfo, params *CompressionParams, pass int, reporter func(float64)) error {
	args := buildFFmpegArgs(info, params, pass)
	cmd := exec.Command("ffmpeg", args...)

	if pass == 2 {
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return err
		}
		
		if err := cmd.Start(); err != nil {
			return err
		}

		scanner := bufio.NewScanner(stdout)
		re := regexp.MustCompile(`out_time_ms=(\d+)`) // ffmpeg progress usa ms ou us dependendo da versão, vamos capturar ambos
		
		// Nota: O ffmpeg moderno usa out_time_ms. O script original usava out_time_us.
		// Vamos fazer um regex mais abrangente.
		re = regexp.MustCompile(`out_time_(?:ms|us)=(\d+)`)

		go func() {
			for scanner.Scan() {
				line := scanner.Text()
				if matches := re.FindStringSubmatch(line); len(matches) > 1 {
					val, _ := strconv.ParseFloat(matches[1], 64)
					// Heurística: se for muito grande, é microsegundos (us), senão milissegundos (ms)
					if val > 1e12 { 
						val = val / 1000000.0 // us -> seconds
					} else if val > 1e9 {
						val = val / 1000.0    // ms -> seconds
					}
					
					pct := (val / info.Duration) * 100.0
					if pct > 100 { pct = 100 }
					reporter(pct)
				}
			}
		}()
		
		err = cmd.Wait()
		return err
	}

	// Pass 1 (sem progress bar)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

// ==============================================================================
// 5. PRESENTATION: UI (Terminal e Cores - SOC)
// ==============================================================================

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
	if params.UseAV1 { codec = "SVT-AV1" }
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

// ==============================================================================
// 6. APPLICATION: ORCHESTRATOR (Main e Loop de Retry)
// ==============================================================================

func main() {
	// Parsing de argumentos simples (YAGNI: sem libs externas)
	if len(os.Args) < 2 {
		fmt.Printf("Uso: %sdiscord-compress%s <ficheiro> [--fast] [--vp9] [--target-mb 20]\n", ColorBold, ColorReset)
		os.Exit(1)
	}

	inputFile := os.Args[1]
	targetMB := 20
	useAV1 := true
	isFast := false

	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--fast":
			isFast = true
			useAV1 = false
		case "--vp9":
			useAV1 = false
		case "--target-mb":
			if i+1 < len(os.Args) {
				targetMB, _ = strconv.Atoi(os.Args[i+1])
				i++
			}
		}
	}

	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		fmt.Printf("%s❌ Erro: O ficheiro '%s' não existe.%s\n", ColorRed, inputFile, ColorReset)
		os.Exit(1)
	}

	// 1. Probing
	fmt.Printf("%s⏳ A analisar o ficheiro...%s\n", ColorDim, ColorReset)
	info, err := probeMedia(inputFile)
	if err != nil {
		fmt.Printf("%s❌ Erro ao ler o ficheiro: %v%s\n", ColorRed, err, ColorReset)
		os.Exit(1)
	}

	// Verificar suporte a AV1
	if useAV1 && !checkAV1Support() {
		fmt.Printf("%s⚠️  libsvtav1 não encontrado. A usar VP9.%s\n", ColorYellow, ColorReset)
		useAV1 = false
	}
	if isFast {
		useAV1 = false
	}

	// 2. Cálculo
	params := calculateParams(info, targetMB, useAV1)
	if params.VideoBitrate <= 50000 {
		fmt.Printf("%s❌ IMPOSSÍVEL: O vídeo é muito longo para o limite de %d MB.%s\n", ColorRed, targetMB, ColorReset)
		os.Exit(1)
	}

	printHeader(info, params)

	// 3. Encoding Loop (Retry)
	maxAttempts := 2
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			fmt.Printf("\n%s⚠️  Excedeu o limite. A tentar novamente com compressão agressiva...%s\n", ColorYellow, ColorReset)
			params.VideoBitrate = int(float64(params.VideoBitrate) * 0.7)
		}

		fmt.Printf("%s⏳ A compactar (Tentativa %d/%d)...%s\n", ColorCyan, attempt, maxAttempts, ColorReset)
		
		// Pass 1
		fmt.Printf("%s  > Passo 1/2: A analisar...%s", ColorDim, ColorReset)
		if err := runFFmpegPass(info, params, 1, nil); err != nil {
			fmt.Printf("\n%s❌ Erro no Passo 1: %v%s\n", ColorRed, err, ColorReset)
			os.Exit(1)
		}
		fmt.Printf(" %sConcluído%s\n", ColorGreen, ColorReset)

		// Pass 2
		fmt.Printf("%s  > Passo 2/2: A codificar:%s\n", ColorDim, ColorReset)
		if err := runFFmpegPass(info, params, 2, printProgress); err != nil {
			fmt.Printf("\n%s❌ Erro no Passo 2: %v%s\n", ColorRed, err, ColorReset)
			os.Exit(1)
		}
		fmt.Println() // Newline após a progress bar

		// Validação
		fileInfo, _ := os.Stat(params.OutputPath)
		if fileInfo != nil && fileInfo.Size() <= params.TargetBytes {
			finalMB := float64(fileInfo.Size()) / (1024 * 1024)
			fmt.Printf("%s─────────────────────────────────────────%s\n", ColorDim, ColorReset)
			fmt.Printf("%s✅ Sucesso!%s Ficheiro final: %s%.2f MB%s\n", ColorGreen, ColorReset, ColorBold, finalMB, ColorReset)
			fmt.Printf("%s💾 Guardado como:%s %s\n", ColorBold, ColorReset, params.OutputPath)
			
			// Limpar logs de 2-pass
			os.Remove("ffmpeg2pass-0.log")
			os.Remove("ffmpeg2pass-0.log.mbtree")
			return
		}
		
		// Limpar logs de 2-pass para a próxima tentativa
		os.Remove("ffmpeg2pass-0.log")
		os.Remove("ffmpeg2pass-0.log.mbtree")
	}

	fmt.Printf("%s❌ Falha: Não foi possível comprimir para o tamanho alvo após %d tentativas.%s\n", ColorRed, maxAttempts, ColorReset)
	os.Exit(1)
}
