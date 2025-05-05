package library

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/bh90210/super/api"
	"github.com/charlievieth/fastwalk"
	"github.com/dhowden/tag"
	"github.com/hajimehoshi/go-mp3"
	"google.golang.org/protobuf/types/known/emptypb"
)

var _ api.LibraryServer = (*Service)(nil)

type Service struct {
	LibraryPath   string
	CachedLibrary *api.LibraryResponse
	api.UnimplementedLibraryServer
}

func (s *Service) Get(context.Context, *emptypb.Empty) (*api.LibraryResponse, error) {
	if s.CachedLibrary != nil {
		return s.CachedLibrary, nil
	}

	list := &api.LibraryResponse{
		File: []*api.File{},
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

			if errors.Is(err, tag.ErrNoTagsFound) {
				list.File = append(list.File, &api.File{
					Artist: filepath.Base(path),
					Path:   path,
				})

				return nil
			}

			list.File = append(list.File, &api.File{
				Artist:   strings.ToValidUTF8(m.Artist(), ""),
				Album:    strings.ToValidUTF8(m.Album(), ""),
				Track:    strings.ToValidUTF8(m.Title(), ""),
				Duration: strings.ToValidUTF8(d.String(), ""),
				Path:     path,
			})
		}

		return nil
	}

	err := fastwalk.Walk(&fastwalk.DefaultConfig, s.LibraryPath, walkFn)
	if err != nil {
		fmt.Println("fastwalk.Walk", "path", s.LibraryPath, "error", err)
		return nil, err
	}

	mapped := make(map[string]*api.File)
	for _, v := range list.File {
		mapped[v.Path] = v
	}

	sorted := []string{}
	for k := range mapped {
		sorted = append(sorted, k)
	}

	sort.Strings(sorted)

	s.CachedLibrary = &api.LibraryResponse{
		File: []*api.File{},
	}

	for _, k := range sorted {
		s.CachedLibrary.File = append(s.CachedLibrary.File, mapped[k])
	}

	return s.CachedLibrary, nil
}

func (s *Service) Download(request *api.DownloadRequest, response api.Library_DownloadServer) error {
	f, err := os.Open(request.Path)
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
