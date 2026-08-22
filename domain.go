package main

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
)

type MediaInfo struct {
	Duration float64
	HasAudio bool
	Width    int
	Height   int
	FPS      float64
	FilePath string
}

type CompressionParams struct {
	TargetMB      int
	TargetBytes   int64
	VideoBitrate  int
	AudioBitrate  int
	AudioChannels int
	MaxWidth      int
	MaxHeight     int
	UseAV1        bool
	OutputPath    string
}

func calculateParams(info *MediaInfo, targetMB int, useAV1 bool) *CompressionParams {
	targetBytes := int64(float64(targetMB) * 1024 * 1024 * 0.95)
	targetBits := float64(targetBytes) * 8

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
		VideoBitrate:  int(math.Max(50000, videoBitrateRaw)),
		AudioBitrate:  audioBitrate,
		AudioChannels: audioChannels,
		MaxWidth:      maxW,
		MaxHeight:     maxH,
		UseAV1:        useAV1,
		OutputPath:    outPath,
	}
}
