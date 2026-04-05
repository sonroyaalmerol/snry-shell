package runner

import (
	"io"
	"os/exec"
)

type Runner interface {
	Output(args ...string) ([]byte, error)
	Run(args ...string) error
}

type execRunner struct{}

func (e execRunner) Output(args ...string) ([]byte, error) {
	return exec.Command(args[0], args[1:]...).Output()
}

func (e execRunner) Run(args ...string) error {
	return exec.Command(args[0], args[1:]...).Run()
}

func New() Runner { return execRunner{} }

// StreamReader starts a long-running command and returns its stdout for
// line-oriented reading. Closing the returned io.ReadCloser kills the process.
type StreamReader interface {
	Stream(args ...string) (io.ReadCloser, error)
}

type execStreamReader struct{}

func (execStreamReader) Stream(args ...string) (io.ReadCloser, error) {
	cmd := exec.Command(args[0], args[1:]...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &streamProcess{ReadCloser: stdout, cmd: cmd}, nil
}

type streamProcess struct {
	io.ReadCloser
	cmd *exec.Cmd
}

func (s *streamProcess) Close() error {
	s.ReadCloser.Close()
	return s.cmd.Process.Kill()
}

func NewStreamReader() StreamReader { return execStreamReader{} }
