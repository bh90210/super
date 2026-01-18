package dupload

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/bh90210/super/server/api"
)

var _ api.DuploadServer = (*Service)(nil)

type Service struct {
	api.UnimplementedDuploadServer

	LibraryPath string
}

func NewService(downloadPath string) (*Service, error) {
	s := &Service{
		LibraryPath: downloadPath,
	}

	return s, nil
}

var ErrErrInvalidPath = errors.New("invalid path")

var ErrErrInvalidSize = errors.New("invalid size")

func (s *Service) Upload(request api.Dupload_UploadServer) error {
	fmt.Println("dupload.Upload called")

	// Get ths files path first.
	r, err := request.Recv()
	if err != nil {
		return err
	}

	if r.GetPath() == "" {
		return ErrErrInvalidPath
	}

	fmt.Println("dupload.Upload path:", r.GetPath())

	f, err := os.Create(filepath.Join(s.LibraryPath, r.GetPath()))
	if err != nil {
		return err
	}

	defer f.Close()

	fmt.Println("dupload.Upload created file:", f.Name())

	// Get the file size.
	r, err = request.Recv()
	if err != nil {
		return err
	}

	sizeSoFar := int64(0)
	var done bool
	go func() {
		for {
			if done {
				return
			}

			request.Send(&api.UploadResponse{
				Response: &api.UploadResponse_Progress{
					Progress: sizeSoFar,
				},
			})

			time.Sleep(1 * time.Second)
		}
	}()

	// Start receiving the file.
	for {
		r, err := request.Recv()
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}

		if r.GetData() == nil {
			break
		}

		if _, err := f.Write(r.GetData()); err != nil {
			return err
		}

		sizeSoFar += int64(len(r.GetData()))
	}

	done = true

	fmt.Println("dupload.Upload completed file:", f.Name(), "size:", sizeSoFar)

	return nil
}
