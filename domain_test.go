package main

import "testing"

func TestCalculateParams_NoAudio(t *testing.T) {
    info := &MediaInfo{Duration: 10, HasAudio: false, FilePath: "test.mp4"}
    params := calculateParams(info, 20, true)
    
    if params.AudioChannels != 0 {
        t.Errorf("Esperava 0 canais de áudio, obteve %d", params.AudioChannels)
    }
    if params.VideoBitrate <= 50000 {
        t.Errorf("Esperava bitrate > 50000, obteve %d", params.VideoBitrate)
    }
}
