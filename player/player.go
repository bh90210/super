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
	"github.com/bh90210/super/super"
	"github.com/ebitengine/oto/v3"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
	"github.com/hajimehoshi/go-mp3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Player .
type Player struct {
	Oto *oto.Player

	otoCtx *oto.Context
	logger *slog.Logger

	metadata Meta
	mu       sync.RWMutex
}

func (p *Player) Init(logger *slog.Logger) error {
	err := os.Mkdir(super.LocalStorage(super.MusicStore), 0755)
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

func (p *Player) New(track string, volume float64, offset int64) {
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

	raw, err := os.ReadFile(super.LocalStorage(super.MusicStore, super.Storage(hashedTrack)))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		p.logger.Error("os.Open failed", "error", err)
		return
	}

	p.mu.Lock()
	p.metadata.Format = filepath.Ext(track)
	p.metadata.Path = track
	p.metadata.streamer = &streamer{Reader: bytes.NewReader(raw)}
	p.mu.Unlock()

	var wg sync.WaitGroup
	wg.Add(1)
	if errors.Is(err, fs.ErrNotExist) {
		fmt.Println("downloading track", track)

		p.metadata.streamer.download = true
		p.metadata.Download = true

		raw = make([]byte, 0)

		conn, err := grpc.NewClient(super.SuperServer,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			p.logger.Error("grpc.NewClient", "error", err)
			return
		}

		client := api.NewLibraryClient(conn)

		response, err := client.Download(context.Background(), &api.DownloadRequest{
			Path: track,
		})
		if err != nil {
			p.logger.Error("client.Download", "error", err)
			return
		}

		go func(conn *grpc.ClientConn, response api.Library_DownloadClient) {
			defer conn.Close()

			p.metadata.streamer.file, err = os.CreateTemp(os.TempDir(), "super-")
			if err != nil {
				p.logger.Error("os.stream failed", "error", err)
				return
			}

			storedFile, err := os.Create(super.LocalStorage(super.MusicStore, super.Storage(hashedTrack)))
			if err != nil {
				p.logger.Error("os.Create failed", "error", err)
				return
			}

			var mu sync.Mutex
			var writeIndex int
			var releaseCounter int
			for {
				releaseCounter++
				if releaseCounter == 3 {
					wg.Done()
				}

				data, err := response.Recv()
				if err != nil && !errors.Is(err, io.EOF) {
					p.logger.Error("response.Recv failed", "error", err)
					return
				}

				if data != nil {
					mu.Lock()
					n, err := p.metadata.streamer.file.WriteAt(data.Data, int64(writeIndex))
					if err != nil {
						p.logger.Error("file.WriteAt failed", "error", err)
						return
					}

					err = p.metadata.streamer.file.Sync()
					if err != nil {
						p.logger.Error("file.Sync failed", "error", err)
						return
					}
					mu.Unlock()

					writeIndex += n

					_, err = storedFile.Write(data.Data)
					if err != nil {
						p.logger.Error("file.WriteAt failed", "error", err)
						return
					}
				}

				if errors.Is(err, io.EOF) {
					err = storedFile.Sync()
					if err != nil {
						p.logger.Error("file.Sync failed", "error", err)
						return
					}

					storedFile.Close()

					p.mu.Lock()
					p.metadata.streamer.finished = true
					p.mu.Unlock()
					break
				}
			}
		}(conn, response)
	} else {
		wg.Done()
	}

	wg.Wait()

	var newPlayer *oto.Player
	switch p.metadata.Format {
	case ".mp3":
		decodedMp3, err := mp3.NewDecoder(p.metadata.streamer)
		if err != nil {
			p.logger.Error("mp3.NewDecoder failed", "error", err)
		}

		newPlayer = p.otoCtx.NewPlayer(decodedMp3)

	case ".wav":
		decodedWav, err := wav.DecodeWithoutResampling(p.metadata.streamer)
		if err != nil {
			p.logger.Error("wav.DecodeWithoutResampling failed", "error", err)
		}

		newPlayer = p.otoCtx.NewPlayer(decodedWav)

	default:
		p.logger.Error("unsupported format", "error", p.metadata.Format)
		return
	}

	if p.Oto != nil {
		if p.Oto.IsPlaying() {
			p.Oto.Pause()
		}

		p.Oto.Close()
	}

	p.Oto = newPlayer
	if offset > 0 {
		_, err := p.Oto.Seek(offset, io.SeekStart)
		if err != nil {
			p.logger.Error("p.Oto.Seek failed", "error", err)
			return
		}
	}

	p.Oto.SetVolume(volume)
	p.Oto.Play()
}

type Meta struct {
	Size     int
	Length   int
	Path     string
	Format   string
	Download bool

	streamer *streamer
}

func (p *Player) Meta() *Meta {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return &Meta{
		Size:     int(p.metadata.streamer.Size()),
		Length:   p.metadata.streamer.Len(),
		Format:   p.metadata.Format,
		Path:     p.metadata.Path,
		Download: p.metadata.Download,
	}
}

type streamer struct {
	*bytes.Reader
	download bool
	finished bool
	file     *os.File
	size     int64
	i        int
	seek     int
	meta     int
}

func (s *streamer) Read(p []byte) (n int, err error) {
	if !s.download {
		n, err := s.Reader.Read(p)
		return n, err
	}

	n, err = s.file.Read(p)
	s.i += n
	return n, err
}

func (s *streamer) Seek(offset int64, whence int) (int64, error) {
	s.seek++

	if s.seek == 3 {
		slog.Info("seeking to", "offset", offset)
		s.meta = int(offset)
	}

	if s.download {
		n, err := s.file.Seek(offset, whence)

		if whence == io.SeekStart {
			s.i = int(n)
		}

		return n, err
	}

	return s.Reader.Seek(offset, whence)
}

func (s *streamer) Size() int64 {
	if s.download && s.finished {
		if s.size == 0 {
			i, err := s.file.Stat()
			if err != nil {
				slog.Error("file.Stat failed", "error", err)
				return 0
			}

			s.size = i.Size()
		}

		return s.size - int64(s.meta)
	}

	return s.Reader.Size() - int64(s.meta)
}

func (s *streamer) Len() int {
	if s.download {
		return int(s.size) - s.i
	}

	return s.Reader.Len()
}
