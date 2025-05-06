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

		raw = make([]byte, 0)

		// conn, err := grpc.NewClient("localhost:80",
		conn, err := grpc.NewClient("super.aeroponics.club:80",
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

		p.metadata.streamer.file, err = os.Create(filepath.Join(p.localCache, hashedTrack))
		if err != nil {
			p.logger.Error("os.Create failed", "error", err)
			return
		}

		go func(conn *grpc.ClientConn, response api.Library_DownloadClient) {
			defer conn.Close()

			var mu sync.Mutex
			var writeIndex int
			var releaseCounter int
			var tagRead []byte
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

					tagRead = append(tagRead, data.Data...)

					writeIndex += n
				}

				if errors.Is(err, io.EOF) {
					m, err := tag.ReadFrom(bytes.NewReader(tagRead))
					if err != nil && !errors.Is(err, tag.ErrNoTagsFound) {
						p.logger.Error("tag.ReadFrom failed", "error", err)
						return
					}

					if !errors.Is(err, tag.ErrNoTagsFound) {
						if m != nil {
							pic := m.Picture()
							if pic != nil {
								if pic.Data != nil {
									p.metadata.size += len(pic.Data)
								}
							}
						}
					}
					p.metadata.streamer.finished = true
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

	p.Oto.SetVolume(volume)
	p.Oto.Play()

	// Remove the images from the size.
	// TODO: this needs to be replaced with a proper solution.
	if !p.metadata.streamer.download {
		f, err := os.Open(filepath.Join(p.localCache, hashedTrack))
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
						p.metadata.size += len(pic.Data)
					}
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

	size     int
	streamer *streamer
}

func (p *Player) Meta() *TrackMeta {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return &TrackMeta{
		Size:   int(p.metadata.streamer.Size()) - p.metadata.size,
		Length: p.metadata.streamer.Len(),
		Format: p.metadata.Format,
		Path:   p.metadata.Path,
	}
}

type streamer struct {
	*bytes.Reader
	download bool
	finished bool
	file     *os.File
	i        int
}

func (s *streamer) Read(p []byte) (n int, err error) {
	if !s.download {
		n, err := s.Reader.Read(p)
		return n, err
	}

	if s.finished {
		buf := bytes.NewBuffer([]byte{})
		// s.File.Seek(0, io.SeekStart)
		_, err := io.Copy(buf, s.file)
		if err != nil {
			slog.Error("io.Copy failed", "error", err)
			return 0, err
		}

		s.download = false
		s.file.Close()
		s.file = nil

		s.Reader = bytes.NewReader(buf.Bytes())
		// s.Reader.Seek(int64(s.i), io.SeekStart)

		n, err := s.Reader.Read(p)
		return n, err
	}

	n, err = s.file.Read(p)
	s.i += n
	return n, err
}

func (s *streamer) Seek(offset int64, whence int) (int64, error) {
	if s.download {
		return s.file.Seek(offset, whence)
	}

	return s.Reader.Seek(offset, whence)
}
