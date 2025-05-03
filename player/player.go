package player

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/bh90210/super/api"
	"github.com/ebitengine/oto/v3"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
	"github.com/hajimehoshi/go-mp3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Player .
type Player struct {
	Oto  *oto.Player
	File *bytes.Reader

	otoCtx *oto.Context
	logger *slog.Logger

	localCache string
}

func (p *Player) Init(logger *slog.Logger) error {
	homeDir, err := os.UserHomeDir()
	p.localCache = filepath.Join(homeDir, ".super")
	err = os.Mkdir(p.localCache, 0755)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		log.Fatal("creating local cache", err)
	}

	p.logger = logger

	op := &oto.NewContextOptions{}
	op.SampleRate = 44100
	op.ChannelCount = 2
	op.Format = oto.FormatSignedInt16LE

	otoCtx, readyChan, err := oto.NewContext(op)
	if err != nil {
		p.logger.Error("oto.NewContext failed", err)
		return err
	}

	p.otoCtx = otoCtx

	<-readyChan

	p.logger.Info("oto context ready")

	return nil
}

func (p *Player) New(track string, volume float64) {
	if p.Oto != nil {
		if p.Oto.IsPlaying() {
			p.Oto.Pause()
		}

		p.Oto.Close()
	}

	h := sha256.New()
	h.Write([]byte(track))
	hashed := h.Sum(nil)
	hashedTrack := fmt.Sprintf("%x", hashed)

	raw, err := os.ReadFile(filepath.Join(p.localCache, hashedTrack))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		p.logger.Error("os.Open failed", "error", err)
		return
	}

	if errors.Is(err, fs.ErrNotExist) {
		conn, err := grpc.NewClient("super.aeroponics.club:80", grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			p.logger.Error("grpc.NewClient", "error", err)
			return
		}

		defer conn.Close()

		client := api.NewLibraryClient(conn)
		response, err := client.Download(context.Background(), &api.DownloadRequest{
			Path: track,
		}, grpc.MaxCallRecvMsgSize(2024*1024*1024), grpc.MaxCallSendMsgSize(2024*1024*1024))
		if err != nil {
			p.logger.Error("client.Download", "error", err)
			return
		}

		err = os.WriteFile(filepath.Join(p.localCache, hashedTrack), response.Data, 0644)
		if err != nil {
			p.logger.Error("os.WriteFile", "error", err)
			return
		}

		raw = response.Data
	}

	p.File = bytes.NewReader(raw)

	var newPlayer *oto.Player
	format := filepath.Ext(track)
	switch format {
	case ".mp3":
		decodedMp3, err := mp3.NewDecoder(p.File)
		if err != nil {
			p.logger.Error("mp3.NewDecoder failed", "error", err)
		}

		newPlayer = p.otoCtx.NewPlayer(decodedMp3)

	case ".wav":
		d, err := wav.DecodeWithoutResampling(p.File)
		if err != nil {
			p.logger.Error("wav.DecodeWithoutResampling failed", "error", err)
		}

		newPlayer = p.otoCtx.NewPlayer(d)

	default:
		p.logger.Error("unsupported format", "error", format)
		return
	}

	if p.Oto != nil {
		if p.Oto.IsPlaying() {
			p.Oto.Pause()
		}

		p.Oto.Close()
	}

	p.Oto = newPlayer

	p.Oto.SetVolume(volume)
	p.Oto.Play()
}
