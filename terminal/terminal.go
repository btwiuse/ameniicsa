package terminal

import (
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/btwiuse/ameniicsa/ptyx"
	"github.com/btwiuse/ameniicsa/util"
	"github.com/creack/pty"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

type Terminal interface {
	Size() (int, int, error)
	Record(string, io.Writer) error
	Write([]byte) error
}

type Pty struct {
	Stdin  *os.File
	Stdout *os.File
}

func NewTerminal() Terminal {
	return &Pty{Stdin: os.Stdin, Stdout: os.Stdout}
}

func (p *Pty) Size() (int, int, error) {
	return pty.Getsize(p.Stdout)
}

func (p *Pty) Record(command string, w io.Writer) error {
	// start command in pty
	cmd := exec.Command("sh", "-c", command)
	cmd.Env = append(os.Environ(), "ASCIINEMA_REC=1")
	master, err := pty.Start(cmd)
	if err != nil {
		return err
	}
	defer master.Close()

	// install WINCH signal handler
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGWINCH)
	defer signal.Stop(signals)
	go func() {
		for _ = range signals {
			p.resize(master)
		}
	}()
	defer close(signals)

	// put stdin in raw mode (if it's a tty)
	fd := p.Stdin.Fd()
	if terminal.IsTerminal(int(fd)) {
		oldState, err := terminal.MakeRaw(int(fd))
		if err != nil {
			return err
		}
		defer terminal.Restore(int(fd), oldState)
	}

	// do initial resize
	p.resize(master)

	// start stdin -> master copying
	stop := util.Copy(master, p.Stdin)

	// copy pty master -> p.stdout & w

	stdout := transform.NewWriter(w, unicode.UTF8.NewEncoder())
	defer stdout.Close()

	stdoutWaitChan := make(chan struct{})
	go func() {
		io.Copy(io.MultiWriter(p.Stdout, stdout), master)
		stdoutWaitChan <- struct{}{}
	}()

	// wait for the process to exit and reap it
	cmd.Wait()

	// wait for master -> stdout copying to finish
	//
	// sometimes after process exits reading from master blocks forever (race condition?)
	// we're using timeout here to overcome this problem
	select {
	case <-stdoutWaitChan:
	case <-time.After(200 * time.Millisecond):
	}

	// stop stdin -> master copying
	stop()

	return nil
}

func (p *Pty) Write(data []byte) error {
	_, err := p.Stdout.Write(data)
	if err != nil {
		return err
	}

	err = p.Stdout.Sync()
	if err != nil {
		return err
	}

	return nil
}

func (p *Pty) resize(f *os.File) {
	var rows, cols int

	if terminal.IsTerminal(int(p.Stdout.Fd())) {
		rows, cols, _ = p.Size()
	} else {
		rows = 24
		cols = 80
	}

	ptyx.Setsize(f, rows, cols)
}
