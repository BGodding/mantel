package media

import (
	"context"
	"io/fs"
	"mantel/watcher"
	"math/rand"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	log "go.uber.org/zap"
	"gopkg.in/vansante/go-ffprobe.v2"
)

type File struct {
	Path     string
	MetaData ffprobe.Format
}

// imageDurationThreshold is the ffprobe duration (in seconds) below which a
// file is treated as a still image rather than a video.
const imageDurationThreshold = .1

// IsImage reports whether d (a file's ffprobe duration in seconds) indicates
// the file is a still image rather than a video.
func IsImage(d float64) bool {
	return d < imageDurationThreshold
}

type Media struct {
	sync.RWMutex
	foldersPending []string
	foldersScanned []string
	// The pool of all possible files to show
	allFiles []File
	// New files that we want to show right away
	newFiles []File
	// Files that need to be shown in the current slideshow cycle
	unseenFiles      []File
	pendingFilePaths chan string
	watch            *watcher.Watcher
	watchEvents      chan fsnotify.Event
	playStartTime    time.Time
	// processMu serializes calls to ProcessFileAsMedia (which shells out to
	// ffprobe/feh) without holding the RWMutex above, so a slow or hung
	// subprocess doesn't block GetRandomFile/RemoveFile on the display loop.
	processMu sync.Mutex
}

func Init(ctx context.Context, paths []string) (*Media, error) {
	instance := Media{}
	instance.AddDirectories(paths)
	var err error
	instance.pendingFilePaths = make(chan string, 8192)
	instance.watchEvents = make(chan fsnotify.Event, 64)
	if instance.watch, err = watcher.Init(paths, instance.watchEvents); err != nil {
		return nil, err
	}
	if err := instance.Discover(ctx); err != nil {
		return nil, err
	}
	return &instance, nil
}

func (m *Media) Run(ctx context.Context) error {
	m.playStartTime = time.Now()
	go m.Worker(ctx)
	return m.watch.Run(ctx)
}

func (m *Media) AddDirectories(paths []string) {
	// Check to make sure the directory is new
	m.Lock()
	defer m.Unlock()
	for _, path := range paths {
		if slices.Contains(m.foldersScanned, path) {
			continue
		}
		m.foldersPending = append(m.foldersPending, path)
	}
}

func (m *Media) GetRandomFile() File {
	m.Lock()
	defer m.Unlock()
	var selectedFile File

	if len(m.newFiles) > 0 {
		selectedFile, m.newFiles = m.newFiles[0], m.newFiles[1:]
	} else if len(m.unseenFiles) > 0 {
		selectedIndex := rand.Intn(len(m.unseenFiles))
		// log.S().Debugf("Unseen selection %+v %+v %+v", len(m.unseenFiles), selectedIndex, m.unseenFiles[selectedIndex].Path)
		selectedFile = m.unseenFiles[selectedIndex]
		m.unseenFiles = append(m.unseenFiles[:selectedIndex], m.unseenFiles[selectedIndex+1:]...)
	} else if len(m.allFiles) > 0 {
		log.S().Warnf("Finished playlist, duration was %s total items re-added %d", time.Since(m.playStartTime).String(), len(m.allFiles))
		m.playStartTime = time.Now()
		m.unseenFiles = nil
		m.unseenFiles = make([]File, len(m.allFiles))
		copy(m.unseenFiles, m.allFiles)
		// Need to manually lock around the recursion...gross
		m.Unlock()
		selectedFile = m.GetRandomFile()
		m.Lock()
	}

	return selectedFile
}

// Take in a list of file paths and return with metadata on all media files
func (m *Media) Discover(ctx context.Context) error {
	m.Lock()
	defer m.Unlock()
	for _, folder := range m.foldersPending {
		filepath.WalkDir(folder, func(s string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				// Get the fileinfo
				// fileInfo, err := os.Stat(s)
				// if err != nil {
				// 	return err
				// }

				// Gives the modification time
				// modificationTime := fileInfo.ModTime()
				// log.S().Debug("Name of the file:", fileInfo.Name(),
				// 	" Last modified time of the file:",
				// 	modificationTime)

				// if metaData, err := getMetaData(ctx, s); err == nil {
				// 	m.allFiles = append(m.allFiles, File{Path: s, MetaData: *metaData})
				// 	log.S().Debugf("%d %s %+v", len(m.allFiles), s, metaData)
				// }
				// rclone uses files with the ext .partial for partial transfers, ignore these
				if !strings.Contains(filepath.Ext(s), ".partial") {
					m.QueueFile(s)
				}
			}
			return nil
		})
		m.foldersScanned = append(m.foldersScanned, folder)
	}
	m.foldersPending = nil
	return nil
}

func getMetaData(ctx context.Context, path string) (*ffprobe.Format, error) {
	ctxPlusTimeout, cancelFn := context.WithTimeout(ctx, 5*time.Second)
	defer cancelFn()

	data, err := ffprobe.ProbeURL(ctxPlusTimeout, path)
	if err != nil {
		return nil, err
	}
	return data.Format, err
}

//func readImageMetadata(imgFile *os.File) error {
//	metaData, err := exif.Decode(imgFile)
//	if err != nil {
//		return err
//	}
//	if tag, err := metaData.Get(""); err != nil {
//		tag.String()
//	}
//
//	jsonByte, err := metaData.MarshalJSON()
//	if err != nil {
//		log.S().Fatal(err.Error())
//	}
//
//	jsonString := string(jsonByte)
//	fmt.Println(jsonString)
//
//	return nil
//}
