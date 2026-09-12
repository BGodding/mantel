package player

import (
	"context"
	"fmt"
	"mantel/feh"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/dexterlb/mpvipc"
	log "go.uber.org/zap"
)

type Player struct {
	Conn    *mpvipc.Connection
	RunOnce bool
}

func Init() (*Player, error) {
	p := Player{}
	// TODO: Also start MPV from here?
	// MPV must be started with the command below
	// mpv --image-display-duration=inf --idle=once --keep-open=yes --input-ipc-server=/tmp/mpv_socket
	p.Conn = mpvipc.NewConnection("/tmp/mpv_socket")
	err := p.Conn.Open()
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// Will pick and play a random subset of the video file if the media length is greater than the slide duration
func (p *Player) PlayVideoClip(path string, mediaLength float64, slideDuration float64) error {
	if p.Conn.IsClosed() {
		if err := p.Conn.Open(); err != nil {
			return err
		}
	}
	clipStart := 0.0
	if mediaLength > slideDuration+1 {
		// Generate a random start pos between 0 and end - and max length
		clipStart = float64(rand.Intn(int(mediaLength - slideDuration)))
	}
	log.S().Infof("playing video clip %#q from %fs to %fs of %fs ", path, clipStart, clipStart+slideDuration, mediaLength)
	// EDL names cannot contain  characters `,;=`
	if strings.ContainsAny(path, ",;=") {
		return fmt.Errorf("%q is an invalid path as it contains ',;='", path)

	}
	if _, err := p.Conn.Call("loadfile", fmt.Sprintf("edl://%s,start=%d,length=%d", path, int(clipStart), int(slideDuration))); err != nil {
		return err
	}
	return p.Conn.Set("pause", false)
}

func (p *Player) PlayVideo(path string) error {
	if p.Conn.IsClosed() {
		if err := p.Conn.Open(); err != nil {
			return err
		}
	}
	if _, err := p.Conn.Call("loadfile", path); err != nil {
		return err
	}
	return p.Conn.Set("pause", false)
}

func (p *Player) PlayImage(path string, slideDuration float64) error {
	go func() {
		// feh is expected to quit on its own via --on-last-slide once its own
		// -D delay elapses; this context is only a backstop in case it
		// doesn't, so it's given headroom beyond feh's own delay.
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(slideDuration)+(time.Second*5))
		defer cancel()
		cmd := exec.CommandContext(ctx, "feh", "--on-last-slide", "quit", "-D", strconv.FormatFloat(slideDuration+.5, 'f', -1, 64), "-Z", "-Y", "-F", path)
		cmd.Env = append(os.Environ(), "DISPLAY=:0")
		if err := cmd.Run(); err != nil {
			log.S().Error(feh.RunError(path, err))
		}
	}()
	if p.Conn.IsClosed() {
		if err := p.Conn.Open(); err != nil {
			return err
		}
	}
	return p.Conn.Set("pause", false)
}
