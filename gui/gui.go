package gui

import (
	"context"
	"io"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/bh90210/super/api"
	"github.com/bh90210/super/player"
	"github.com/wailsapp/wails/v3/pkg/application"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

type PlayerState int

const (
	STOPPED PlayerState = iota
	PLAYING
	PAUSED
)

type State struct {
	Menu
	Controls
	// Search
	// Dupload
	Active

	App    *application.App
	Player *player.Player

	// Stop the ticker.
	tickerStop chan struct{}
	// Reset the ticker.
	tickerReset chan struct{}
	// Kill the ticker.
	tickerKill chan struct{}
	reposition chan float64
	playing    PlayerState
	mu         sync.Mutex
}

type Menu struct {
	Active          string
	ActiveDownloads int
	Lists           []string
	FavoriteLists   []string
}

type Controls struct {
	PlayPause   Button
	Previous    Button
	Next        Button
	ProgressBar ProgressBar
	Time        string
	Volume      Slider
	StatusBar   StatusBar
	Deactivated bool
}

// type Search struct {
// 	Query        TextInput
// 	SearchQuery  Button
// 	FavoriteList Button
// 	NewList      Button
// 	AddToList    Button
// 	SelectedList Dropdown
// 	ImportList   Button
// }

type Dupload struct{}

type Active struct {
	Name  string
	List  map[int]*api.File
	Index int
}

type Button struct {
	Label       string
	Deactivated bool
	Visible     bool
}

type Dropdown struct {
	Options     []string
	Selected    string
	Deactivated bool
}

type Slider struct {
	Value       float64
	Deactivated bool
}

type TextInput struct {
	Text        string
	Deactivated bool
}

type StatusBar struct {
	Position1 string
	Position2 string
	Position3 string
}

type Checkbox struct {
	Checked     bool
	Deactivated bool
	Visible     bool
}

type ProgressBar struct {
	Percent     float64
	Time        string
	Deactivated bool
}

func (s *State) Init(app *application.App) {
	s.Active.List = make(map[int]*api.File)
	s.tickerReset = make(chan struct{})
	s.tickerStop = make(chan struct{})
	s.tickerKill = make(chan struct{})
	s.reposition = make(chan float64)

	s.App = app
	s.playing = STOPPED

	s.Player = &player.Player{}
	if err := s.Player.Init(s.App.Logger); err != nil {
		log.Fatal(err)
	}
}

func (s *State) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	go func() {
		// Init.
		s.App.OnEvent("ready", func(event *application.CustomEvent) {
			s.App.EmitEvent("previous", true)
			s.App.EmitEvent("next", true)
			s.App.EmitEvent("progress.bar", 0.)
			s.App.EmitEvent("time", "--:--")
			s.App.EmitEvent("play.pause", "Play")
			s.App.EmitEvent("play.pause.deactivate", true)
			s.App.EmitEvent("volume.set", "70")
			s.Controls.Volume.Value = .7
			s.App.EmitEvent("status.left", "--")
			s.App.EmitEvent("status.center", "--")
			s.App.EmitEvent("status.right", "--")
			s.List()
			s.App.OffEvent("ready")
		})
	}()

	// Listeners.
	s.App.OnEvent("front.volume.mute", func(event *application.CustomEvent) {
		s.App.EmitEvent("volume.set", "0")
		s.Controls.Volume.Value = 0.
		if s.Player.Oto != nil {
			s.Player.Oto.SetVolume(0.)
		}
	})

	s.App.OnEvent("front.volume.max", func(event *application.CustomEvent) {
		s.App.EmitEvent("volume.set", "100")
		s.Controls.Volume.Value = 1.
		if s.Player.Oto != nil {
			s.Player.Oto.SetVolume(1.)
		}
	})

	s.App.OnEvent("front.volume.set", func(event *application.CustomEvent) {
		i, err := strconv.Atoi(event.Data.(string))
		if err != nil {
			s.App.Logger.Error("strconv.Atoi", "error", err)
			return
		}

		vol := scale(float64(i), 0., 1., 0, 100)
		s.Controls.Volume.Value = vol
		if s.Player.Oto != nil {
			s.Player.Oto.SetVolume(vol)
		}
	})

	s.App.OnEvent("front.list.play", func(event *application.CustomEvent) {
		s.App.Logger.Debug("front.list.play", "event", event.Data)
		s.play(int(event.Data.(float64)))
	})

	s.App.OnEvent("front.play.pause", func(event *application.CustomEvent) {
		if s.Player.Oto != nil {
			if s.Player.Oto.IsPlaying() {
				s.Player.Oto.Pause()
				s.App.EmitEvent("play.pause", "Play")
				s.tickerStop <- struct{}{}
				s.playing = PAUSED
			} else {
				s.Player.Oto.Play()
				s.App.EmitEvent("play.pause", "Pause")
				s.tickerReset <- struct{}{}
				s.playing = PLAYING
			}
		}
	})

	s.App.OnEvent("front.next", func(event *application.CustomEvent) {
		s.play(s.Active.Index + 2)
	})

	s.App.OnEvent("front.previous", func(event *application.CustomEvent) {
		s.play(s.Active.Index)
	})

	s.App.OnEvent("front.progress", func(event *application.CustomEvent) {
		s.reposition <- event.Data.(float64)
	})

	return nil
}

func scale(unscaledNum, minAllowed, maxAllowed, min, max float64) float64 {
	return (maxAllowed-minAllowed)*(unscaledNum-min)/(max-min) + minAllowed
}

func (s *State) List() []api.File {
	// conn, err := grpc.NewClient("localhost:80", grpc.WithTransportCredentials(insecure.NewCredentials()))
	conn, err := grpc.NewClient("super.aeroponics.club:80", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		s.App.Logger.Error("grpc.NewClient", "error", err)
		return nil
	}

	defer conn.Close()

	client := api.NewLibraryClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	response, err := client.Get(ctx, &emptypb.Empty{})
	if err != nil {
		s.App.Logger.Error("client.Get", "error", err)
		return nil
	}

	var list []api.File
	for k, v := range response.File {
		s.Active.List[k] = v
		list = append(list, *v)
	}

	s.Active.Name = "--"

	s.App.EmitEvent("list", list)

	return list
}

func (s *State) play(index int) {
	s.mu.Lock()
	if s.playing == PLAYING || s.playing == PAUSED {
		s.tickerKill <- struct{}{}
	}

	s.App.EmitEvent("status.left", s.Active.Name)

	track, ok := s.Active.List[index-1]
	if ok {
		go func() {
			s.App.EmitEvent("progress.bar", 100)
			s.App.EmitEvent("segmented", nil)
			s.App.EmitEvent("time", "loading...")
		}()

		s.App.EmitEvent("play.pause", "Pause")
		s.App.EmitEvent("play.pause.deactivate", false)
		s.App.EmitEvent("status.center", track.Artist+" - "+track.Track)
		s.Active.Index = index - 1

		s.App.Logger.Debug("play", "track", track.Track, "artist", track.Artist, "index", index-1)
		s.Player.New(track.Path, s.Controls.Volume.Value, 0)
	}

	nextTrack, nextOk := s.Active.List[index]
	if nextOk {
		s.App.EmitEvent("status.right", nextTrack.Artist+" - "+nextTrack.Track)
		s.App.EmitEvent("next", false)
	} else {
		s.App.EmitEvent("status.right", "--")
		s.App.EmitEvent("next", true)
	}

	_, prevOk := s.Active.List[index-2]
	if prevOk {
		s.App.EmitEvent("previous", false)
	} else {
		s.App.EmitEvent("previous", true)
	}

	s.playing = PLAYING
	s.mu.Unlock()

	// s.App.EmitEvent("time", "loading...")
	// s.App.EmitEvent("progress.bar", 0.)

	for {
		select {
		case <-s.tickerKill:
			return

		default:
			if !s.Player.Oto.IsPlaying() {
				time.Sleep(10 * time.Millisecond)
				continue
			}
		}

		break
	}

	const tik = time.Duration(200 * time.Millisecond)
	go func() {
		ticker := time.NewTicker(tik)
		var loading bool
		for {
			select {
			case <-ticker.C:
				if !s.Player.Oto.IsPlaying() {
					ticker.Stop()
					s.playing = STOPPED

					if nextOk {
						s.play(index + 1)
						return
					}

					s.App.EmitEvent("play.pause", "Play")
					s.App.EmitEvent("play.pause.deactivate", true)
					s.App.EmitEvent("status.center", "--")
					s.App.EmitEvent("time", "--:--")
					s.App.EmitEvent("progress.bar", 0.)

					return
				}

				meta := s.Player.Meta()

				// s.App.Logger.Debug("ticker", "meta", meta)

				if meta.Size <= 0 || meta.Length <= 0 {
					if loading {
						loading = false
					} else {
						loading = true
					}
					s.App.EmitEvent("progress.bar", 100)
					s.App.EmitEvent("segmented", nil)
					s.App.EmitEvent("time", "loading...")
					continue
				}

				s.App.EmitEvent("segmented.off", nil)

				s.App.EmitEvent("time", time.Duration(
					int(float64(meta.Size-meta.Length)/44100)*int(time.Second),
				).String())
				s.App.EmitEvent("progress.bar", scale(float64(meta.Size-meta.Length), 0, 100, 0, float64(meta.Size)))

			case <-s.tickerStop:
				ticker.Stop()

			case <-s.tickerReset:
				ticker.Reset(tik)

			case <-s.tickerKill:
				return

			case newPosition := <-s.reposition:
				meta := s.Player.Meta()
				if meta.Path != "" {
					if newPosition < 0 {
						newPosition = 0
					}

					if newPosition > 100 {
						newPosition = 100
					}

					offset := scale(
						newPosition,
						0,
						float64(meta.Size),
						0,
						100,
					)

					s.App.Logger.Debug("offset", "offset", int64(offset), "newpos", newPosition, "size", meta.Size)

					if s.Player.Oto != nil {
						if meta.Download {
							if s.Player.Oto.IsPlaying() {
								s.Player.New(track.Path, s.Controls.Volume.Value, int64(offset)*4)
							} else {
								s.Player.New(track.Path, s.Controls.Volume.Value, int64(offset)*4)
								s.Player.Oto.Pause()
							}
						} else {
							_, err := s.Player.Oto.Seek(int64(offset)*4, io.SeekStart)
							if err != nil {
								s.App.Logger.Error("s.Player.Oto.Seek", "error", err)
								return
							}
						}
					}

					meta := s.Player.Meta()

					s.App.EmitEvent("time", time.Duration(
						int(float64(meta.Size-meta.Length)/44100)*int(time.Second),
					).String())
					s.App.EmitEvent("progress.bar", scale(float64(meta.Size-meta.Length), 0, 100, 0, float64(meta.Size)))
				}
			}
		}
	}()
}
