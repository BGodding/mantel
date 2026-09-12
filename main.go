package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"mantel/media"
	"mantel/player"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/dexterlb/mpvipc"
	log "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const AppVersion = "0.0.5"

type app struct {
	mediaFileHandler *media.Media
	mediaPlayer      *player.Player
	slideDuration    float64
	videoDuration    float64
}

func main() {
	version := flag.Bool("version", false, "prints current version and exits")
	pathString := flag.String("media-folders", ".", "comma seperated list of folders to watch")
	splashScreenPath := flag.String("splash-screen", "", "path to a media file to display when no other media is available")
	slideDuration := flag.Float64("media-duration", 12.0, "time for each media file to display in seconds")
	logLevel := flag.String("log-level", "info", "Level to log at")
	flag.Parse()

	if *version {
		buildInfo, _ := debug.ReadBuildInfo()
		fmt.Println(buildInfo.Main.Path, AppVersion, buildInfo.GoVersion)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup
	var a app
	var err error
	a.slideDuration = *slideDuration
	a.videoDuration = a.slideDuration * 1.5

	// Setup logging
	logConfig := log.NewDevelopmentConfig()
	level, err := zapcore.ParseLevel(*logLevel)
	if err != nil {
		level = zapcore.InfoLevel
	}
	logConfig.Level = log.NewAtomicLevelAt(level)
	logger := log.Must(logConfig.Build())
	defer logger.Sync()

	undo := log.ReplaceGlobals(logger)
	defer undo()

	// Uncomment the line below to enable pprof
	//go func() {
	//	log.S().Info(http.ListenAndServe("localhost:6060", nil))
	//}()

	a.mediaPlayer, err = player.Init()
	if err != nil {
		log.S().Fatal(err)
	}
	defer func(Conn *mpvipc.Connection) {
		err := Conn.Close()
		if err != nil {
			log.S().Error(err)
		}
	}(a.mediaPlayer.Conn)

	a.mediaFileHandler, err = media.Init(ctx, strings.Split(*pathString, ","))
	if err != nil {
		log.S().Fatal(err)
	}
	go func() {
		err := a.mediaFileHandler.Run(ctx)
		if err != nil {
			log.S().Error(err)
		}
	}()

	var splashFile *media.File
	if *splashScreenPath != "" {
		splashFile, err = a.mediaFileHandler.ProcessFileAsMedia(ctx, *splashScreenPath)
		if err != nil {
			log.S().Errorf("failed to load splash screen %q: %s", *splashScreenPath, err)
		}
	}

	// Load some initial content right away
	a.UpdateDisplay(splashFile)

	// Inf loop of getting and displaying new media files
	wg.Add(1)
	go func(wg *sync.WaitGroup, conn *mpvipc.Connection) {
		defer wg.Done()
		mediaTicker := time.NewTicker(time.Second * time.Duration(*slideDuration))
		mpvEvents, mpvStop := conn.NewEventListener()
		for {
			select {
			case <-ctx.Done():
				return
			case <-mediaTicker.C:
				// Use dynamic timing for shorter video files or we just sit on the last frame
				delayUntilNextUpdate, _ := a.UpdateDisplay(splashFile)
				mediaTicker.Reset(delayUntilNextUpdate)
			case event := <-mpvEvents:
				// Watch the events to find media files that are not playing nice and remove them from rotation
				if event.Reason == "error" {
					log.S().Errorf("EVENT %+v", event)
				} else {
					log.S().Debugf("EVENT %+v", event)
				}

			case <-mpvStop:
				log.S().Error("Received MPV stop")
			}
		}
	}(&wg, a.mediaPlayer.Conn)

	// This will block until all wait group processes have called done
	wg.Wait()
}

func (a *app) UpdateDisplay(splashMedia *media.File) (time.Duration, error) {
	// This should only return an empty file if we don't have any media to show
	file := a.mediaFileHandler.GetRandomFile()
	if file.Path == "" && splashMedia != nil {
		file = *splashMedia
	}

	if file.Path != "" {
		log.S().Infof("playing media path %#q duration %fs", file.Path, file.MetaData.DurationSeconds)
		if media.IsImage(file.MetaData.DurationSeconds) {
			if err := a.mediaPlayer.PlayImage(file.Path, a.slideDuration); err != nil {
				log.S().Error(err)
				return time.Millisecond * 100, err
			} else {
				return time.Second * time.Duration(a.slideDuration), nil
			}
		} else if file.MetaData.DurationSeconds > a.videoDuration && !strings.ContainsAny(file.Path, ",;=") {
			if err := a.mediaPlayer.PlayVideoClip(file.Path, file.MetaData.DurationSeconds, a.videoDuration); err != nil {
				return time.Millisecond * 100, err
			} else {
				return time.Second * time.Duration(a.videoDuration), nil
			}
		} else if file.MetaData.DurationSeconds <= a.videoDuration {
			if err := a.mediaPlayer.PlayVideo(file.Path); err != nil {
				log.S().Error(err)
				return time.Millisecond * 100, err
			}
			// Handle an awkward short video
			return time.Second * time.Duration(math.Max(file.MetaData.DurationSeconds, a.videoDuration/4)), nil
		} else {
			return time.Millisecond * 100, fmt.Errorf("unable to play media file %#q", filepath.Base(file.Path))
		}
	}
	return time.Second, errors.New("no media available")
}
