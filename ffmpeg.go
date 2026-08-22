package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

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

func buildFFmpegArgs(info *MediaInfo, params *CompressionParams, pass int) []string {
	args := []string{"-hide_banner", "-y", "-i", info.FilePath}

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
		re := regexp.MustCompile(`out_time_(?:ms|us)=(\d+)`)

		go func() {
			for scanner.Scan() {
				line := scanner.Text()
				if matches := re.FindStringSubmatch(line); len(matches) > 1 {
					val, _ := strconv.ParseFloat(matches[1], 64)
					if val > 1e12 {
						val = val / 1000000.0
					} else if val > 1e9 {
						val = val / 1000.0
					}

					pct := (val / info.Duration) * 100.0
					if pct > 100 {
						pct = 100
					}
					reporter(pct)
				}
			}
		}()

		return cmd.Wait()
	}

	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}
