package media

import (
	"context"
	"mantel/feh"
	"os"
	"os/exec"
	"time"

	"github.com/fsnotify/fsnotify"
	log "go.uber.org/zap"
)

func (m *Media) ProcessFileAsMedia(ctx context.Context, path string) (*File, error) {
	m.processMu.Lock()
	defer m.processMu.Unlock()
	// Get the fileinfo
	//fileInfo, err := os.Stat(path)
	//if err != nil {
	//	return err
	//}
	// Gives the modification time
	// modificationTime := fileInfo.ModTime()
	metaData, err := getMetaData(ctx, path)
	if err == nil {
		if IsImage(metaData.DurationSeconds) {
			if err := validateImage(ctx, path); err != nil {
				return nil, err
			}
		}
		log.S().Debugf("%s, %+v", path, metaData)
		return &File{Path: path, MetaData: *metaData}, err
	}
	return nil, err
}

// validateImage confirms feh (the image viewer the player package uses for
// playback) can actually load path, catching corrupt or unsupported image
// files before they're added to rotation. It's bounded by its own timeout,
// mirroring getMetaData's, so a hung feh can't block callers indefinitely.
func validateImage(ctx context.Context, path string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "feh", "-l", path)
	cmd.Env = append(os.Environ(), "DISPLAY=:0")
	if err := cmd.Run(); err != nil {
		return feh.RunError(path, err)
	}
	return nil
}

func (m *Media) QueueFile(path string) {
	m.pendingFilePaths <- path
}

func (m *Media) QueueFileChange(event fsnotify.Event) {
	m.watchEvents <- event
}

func (m *Media) RemoveFile(path string) {
	m.Lock()
	defer m.Unlock()
	for i, v := range m.allFiles {
		if v.Path == path {
			m.allFiles = append(m.allFiles[:i], m.allFiles[i+1:]...)
			break
		}
	}
	for i, v := range m.newFiles {
		if v.Path == path {
			m.newFiles = append(m.newFiles[:i], m.newFiles[i+1:]...)
			break
		}
	}
	for i, v := range m.unseenFiles {
		if v.Path == path {
			m.unseenFiles = append(m.unseenFiles[:i], m.unseenFiles[i+1:]...)
			break
		}
	}
}

func (m *Media) Worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-m.watchEvents:
			if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
				m.RemoveFile(event.Name)
				continue
			}

			if !event.Has(fsnotify.Create) && !event.Has(fsnotify.Write) {
				continue
			}
			// TODO: Insert new files at the beginning of a queue
			f, err := m.ProcessFileAsMedia(ctx, event.Name)
			if err != nil {
				log.S().Error(err)
				continue
			}
			m.Lock()
			m.allFiles = append(m.allFiles, *f)
			m.newFiles = append(m.newFiles, *f)
			m.Unlock()
		case path := <-m.pendingFilePaths:
			f, err := m.ProcessFileAsMedia(ctx, path)
			if err != nil {
				log.S().Error(err)
				continue
			}
			m.Lock()
			m.allFiles = append(m.allFiles, *f)
			m.unseenFiles = append(m.unseenFiles, *f)
			m.Unlock()
		}
	}
}
