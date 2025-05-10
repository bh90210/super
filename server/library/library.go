package library

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bh90210/super/api"
	"github.com/charlievieth/fastwalk"
	"github.com/dhowden/tag"
	"github.com/hajimehoshi/go-mp3"
)

var _ api.LibraryServer = (*Service)(nil)

type Service struct {
	LibraryPath   string
	CachedLibrary *api.LibraryResponse

	api.UnimplementedLibraryServer
	mu sync.RWMutex
}

func NewService(libraryPath string) (*Service, error) {
	s := &Service{
		LibraryPath: libraryPath,
		CachedLibrary: &api.LibraryResponse{
			AddIndex:    []*api.File{},
			RemoveIndex: []*api.File{},
		},
	}

	walkFn := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			fmt.Println("walk", "path", path, "error", err)
			return err
		}

		if !d.IsDir() {
			if filepath.Ext(path) != ".mp3" {
				return nil
			}

			f, err := os.Open(path)
			if err != nil {
				fmt.Println("os.Open", "path", path, "error", err)
				return err
			}

			defer f.Close()

			decodedMp3, err := mp3.NewDecoder(f)
			if err != nil {
				fmt.Println("mp3.NewDecoder", "path", path, "error", err)
				return err
			}

			samples := decodedMp3.Length() / 4
			length := int(samples) / decodedMp3.SampleRate()

			d := time.Duration(length * int(time.Second))

			m, err := tag.ReadFrom(f)
			if err != nil && !errors.Is(err, tag.ErrNoTagsFound) {
				fmt.Println("tag.ReadFrom", "path", path, "error", err)
				return err
			}

			cleanPath := strings.Replace(path, s.LibraryPath, "", 1)

			if errors.Is(err, tag.ErrNoTagsFound) {
				s.mu.Lock()
				s.CachedLibrary.AddIndex = append(s.CachedLibrary.AddIndex, &api.File{
					Artist: filepath.Base(path),
					Path:   cleanPath,
				})
				s.mu.Unlock()

				return nil
			}

			s.mu.Lock()
			s.CachedLibrary.AddIndex = append(s.CachedLibrary.AddIndex, &api.File{
				Artist:   strings.ToValidUTF8(m.Artist(), ""),
				Album:    strings.ToValidUTF8(m.Album(), ""),
				Track:    strings.ToValidUTF8(m.Title(), ""),
				Duration: strings.ToValidUTF8(d.String(), ""),
				Path:     cleanPath,
			})
			s.mu.Unlock()
		}

		return nil
	}

	err := fastwalk.Walk(&fastwalk.DefaultConfig, s.LibraryPath, walkFn)
	if err != nil {
		fmt.Println("fastwalk.Walk", "path", s.LibraryPath, "error", err)
		return nil, err
	}

	return s, nil
}

func (s *Service) Get(request *api.LibraryRequest, response api.Library_GetServer) (err error) {
	slog.Info("Get", "request", request)

	switch request.Index {
	case 0:
		s.mu.RLock()
		list := &api.LibraryResponse{
			Index:    1,
			AddIndex: s.CachedLibrary.AddIndex,
		}
		s.mu.RUnlock()

		err := response.Send(list)
		if err != nil {
			fmt.Println("response.Send", "error", err)
			return err
		}

	case 1:
		err := response.Send(&api.LibraryResponse{
			Index: 1,
		})
		if err != nil {
			fmt.Println("response.Send", "error", err)
			return err
		}
	}

	return
}

func (s *Service) Download(request *api.DownloadRequest, response api.Library_DownloadServer) error {
	slog.Info("Download", "request", request)

	f, err := os.Open(filepath.Join(s.LibraryPath, request.Path))
	if err != nil {
		fmt.Println("os.ReadFile", "path", request.Path, "error", err)
		return err
	}

	defer f.Close()

	for {
		buf := make([]byte, 1024*1024)
		n, err := f.Read(buf)
		if err != nil && !errors.Is(err, io.EOF) {
			fmt.Println("f.Read", "path", request.Path, "error", err)
			return err
		}

		if len(buf) != 0 {
			e := response.Send(&api.DownloadResponse{
				Data: buf[:n],
			})
			if e != nil {
				fmt.Println("response.Send", "path", request.Path, "error", err)
				return e
			}
		}

		if errors.Is(err, io.EOF) {
			break
		}
	}

	return nil
}
