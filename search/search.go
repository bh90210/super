package search

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"sort"
	"sync"

	"github.com/bh90210/super/api"
	"github.com/bh90210/super/super"
	"github.com/blevesearch/bleve"
	badger "github.com/dgraph-io/badger/v4"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Search struct {
	index bleve.Index
	conn  *grpc.ClientConn
	db    *badger.DB
	list  []api.File
	mu    sync.Mutex
}

func NewSearch() (s *Search, err error) {
	s = &Search{}

	// We need to create the local cache directory, if not already created.
	err = os.Mkdir(super.LocalStorage(super.SearchStore), 0755)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		slog.Error("creating local cache", err)
		return
	}

	// Then try opening the search index.
	blindex, err := bleve.Open(super.LocalStorage(super.SearchStore))
	if err != nil && !errors.Is(err, bleve.ErrorIndexMetaMissing) {
		slog.Error("bleve.Open", err)
		return
	}

	// If the index doesn't exist, create it.
	if errors.Is(err, bleve.ErrorIndexMetaMissing) {
		slog.Info("creating new search index")
		mapping := bleve.NewIndexMapping()
		blindex, err = bleve.New(super.LocalStorage(super.SearchStore), mapping)
		if err != nil {
			slog.Error("bleve.New", err)
			return
		}
	}

	s.index = blindex

	// Try to create local data storage directory, if not already created.
	err = os.Mkdir(super.LocalStorage(super.DataStore), 0755)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		slog.Error("creating local cache", err)
		return
	}

	// Then try opening badger (local data storage).
	s.db, err = badger.Open(badger.DefaultOptions(super.LocalStorage(super.DataStore)))
	if err != nil {
		slog.Error("badger.Open", err)
		return
	}

	// Load the current list from local storage.
	s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefix := []byte(super.File)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(v []byte) error {
				var file api.File
				buf := bytes.NewBuffer(v)
				g := gob.NewDecoder(buf)
				err = g.Decode(&file)
				if err != nil {
					slog.Error("gob.Decode", err)
					return err
				}

				s.list = append(s.list, file)

				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	// Create a new gRPC connection to the server.
	s.conn, err = grpc.NewClient(super.SuperServer, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	// Get the current index from local storage.
	var index uint64
	err = s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("index"))
		if err != nil && !errors.Is(err, badger.ErrKeyNotFound) {
			slog.Error("badger.Get", err)
			return err
		}

		if errors.Is(err, badger.ErrKeyNotFound) {
			index = 0
		} else {
			valCopy, err := item.ValueCopy(nil)
			if err != nil {
				slog.Error("badger.ValueCopy", err)
				return err
			}

			buf := bytes.NewBuffer(valCopy)
			g := gob.NewDecoder(buf)
			err = g.Decode(&index)
			if err != nil {
				slog.Error("gob.Decode", err)
				return err
			}
		}

		return nil
	})

	// Connect to the server and get the library.
	library := api.NewLibraryClient(s.conn)
	// Send the current index to the server.
	response, err := library.Get(context.Background(), &api.LibraryRequest{
		Index: index,
	})
	if err != nil {
		slog.Error("library.Get", err)
		return
	}

	// Server will respond with the current index and all the updates.
	go s.incoming(response, index)

	return
}

func (s *Search) incoming(response api.Library_GetClient, index uint64) {
	for {
		// Message contains the current index and all files that
		// need to be added or removed from the search index and
		// the local storage representation.
		message, err := response.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			slog.Error("library.Recv", err)
			return
		}

		// Assign the new index value.
		index = message.Index

		// Update the local storage & index.
		err = s.db.Update(func(txn *badger.Txn) error {
			// Add new files.
			for _, file := range message.AddIndex {
				buf := bytes.NewBuffer(nil)
				g := gob.NewEncoder(buf)
				err = g.Encode(file)
				if err != nil {
					slog.Error("gob.Encode", err)
					return err
				}

				err = txn.Set([]byte(super.File+file.Path), buf.Bytes())
				if err != nil {
					slog.Error("badger.Set", err)
					return err
				}
			}

			// Remove obsolete files.
			for _, file := range message.RemoveIndex {
				err = txn.Delete([]byte(super.File + file.Path))
				if err != nil {
					slog.Error("badger.Delete", err)
					return err
				}
			}

			// Update the index.
			buf := bytes.NewBuffer(nil)
			g := gob.NewEncoder(buf)
			err = g.Encode(index)
			if err != nil {
				slog.Error("gob.Encode", err)
				return err
			}

			err = txn.Set([]byte("index"), buf.Bytes())
			return err
		})
		if err != nil {
			slog.Error("badger.Set", err)
			return
		}

		// Add new files to the search index and s.list field.
		for _, file := range message.AddIndex {
			err = s.index.Index(file.Path, file)
			if err != nil {
				slog.Error("index.Index", "file", file.Path, "error", err)
				return
			}

			s.mu.Lock()
			s.list = append(s.list, *file)
			s.mu.Unlock()
		}

		// Remove obsolete files from the search index and s.list field.
		for _, file := range message.RemoveIndex {
			err = s.index.Delete(file.Path)
			if err != nil {
				slog.Error("index.Delete", err)
				return
			}

			for i, f := range s.list {
				if f.Path == file.Path {
					s.mu.Lock()
					s.list = append(s.list[:i], s.list[i+1:]...)
					s.mu.Unlock()
					break
				}
			}
		}
	}
}

func (s *Search) List() []api.File {
	mapped := make(map[string]*api.File)

	s.mu.Lock()
	for _, v := range s.list {
		mapped[v.Path] = &v
	}
	s.mu.Unlock()

	sorted := []string{}
	for k := range mapped {
		sorted = append(sorted, k)
	}

	sort.Strings(sorted)

	var list []api.File
	for _, k := range sorted {
		list = append(list, *mapped[k])
	}

	return list
}

func (s *Search) Search(query string) ([]api.File, error) {
	q := bleve.NewQueryStringQuery(query)
	searchRequest := bleve.NewSearchRequest(q)
	searchRequest.Size = 100
	searchResult, err := s.index.Search(searchRequest)
	if err != nil {
		slog.Error("index.Search", err)
		return nil, err
	}

	var files []api.File
	for _, v := range searchResult.Hits {
		for _, f := range s.list {
			if f.Path == v.ID {
				files = append(files, f)
				break
			}
		}
	}

	slog.Info("search.Search", "query", query, "result", searchResult)

	return files, nil
}
