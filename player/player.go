package player

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/bh90210/super/api"
	"github.com/dhowden/tag"
	"github.com/ebitengine/oto/v3"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
	"github.com/hajimehoshi/go-mp3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Player .
type Player struct {
	Oto *oto.Player
	// File *bytes.Reader

	otoCtx *oto.Context
	logger *slog.Logger

	localCache string
	metadata   TrackMeta
	mu         sync.RWMutex
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
		fmt.Println("downloading track", track)

		raw = make([]byte, 0)

		conn, err := grpc.NewClient("localhost:80",
			// conn, err := grpc.NewClient("super.aeroponics.club:80",
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			p.logger.Error("grpc.NewClient", "error", err)
			return
		}

		defer conn.Close()

		client := api.NewLibraryClient(conn)

		response, err := client.Download(context.Background(), &api.DownloadRequest{
			Path: track,
		})
		if err != nil {
			p.logger.Error("client.Download", "error", err)
			return
		}

		for {
			data, err := response.Recv()
			if err != nil && !errors.Is(err, io.EOF) {
				p.logger.Error("response.Recv failed", "error", err)
				return
			}

			if data != nil {
				fmt.Println("downloading track", track, "size", len(data.Data))
				if len(data.Data) != 0 {
					raw = append(raw, data.Data...)
				}
			}

			if errors.Is(err, io.EOF) {
				break
			}
		}

		err = os.WriteFile(filepath.Join(p.localCache, hashedTrack), raw, 0644)
		if err != nil {
			p.logger.Error("os.WriteFile failed", "error", err)
			return
		}
	}

	format := filepath.Ext(track)

	p.mu.Lock()
	p.metadata.Format = format
	p.metadata.Path = track
	p.metadata.file = bytes.NewReader(raw)
	p.metadata.Size = len(raw)
	p.mu.Unlock()

	var newPlayer *oto.Player
	switch format {
	case ".mp3":
		decodedMp3, err := mp3.NewDecoder(p.metadata.file)
		if err != nil {
			p.logger.Error("mp3.NewDecoder failed", "error", err)
		}

		newPlayer = p.otoCtx.NewPlayer(decodedMp3)

	case ".wav":
		decodedWav, err := wav.DecodeWithoutResampling(p.metadata.file)
		if err != nil {
			p.logger.Error("wav.DecodeWithoutResampling failed", "error", err)
		}

		newPlayer = p.otoCtx.NewPlayer(decodedWav)

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

	// Remove the images from the size.
	// TODO: this needs to be replaced with a proper solution.
	f, err := os.Open(track)
	if err != nil {
		p.logger.Error("os.Open failed", "error", err)
		return
	}

	defer f.Close()

	m, err := tag.ReadFrom(f)
	if err != nil && !errors.Is(err, tag.ErrNoTagsFound) {
		p.logger.Error("tag.ReadFrom failed", "error", err)
		return
	}

	if !errors.Is(err, tag.ErrNoTagsFound) {
		if m != nil {
			pic := m.Picture()
			if pic != nil {
				if pic.Data != nil {
					p.metadata.Size -= len(pic.Data)
				}
			}
		}
	}
}

type TrackMeta struct {
	Size   int
	Length int
	Path   string
	Format string

	file *bytes.Reader
}

func (p *Player) Meta() *TrackMeta {
	// p.mu.RLock()
	// defer p.mu.RUnlock()

	return &TrackMeta{
		Size:   p.metadata.Size,
		Length: p.metadata.file.Len(),
		Format: p.metadata.Format,
		Path:   p.metadata.Path,
	}
}
