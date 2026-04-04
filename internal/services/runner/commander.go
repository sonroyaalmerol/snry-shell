package runner

import "os/exec"

type Commander interface {
	Run(args ...string) ([]byte, error)
}

type execCommander struct{}

func (e execCommander) Run(args ...string) ([]byte, error) {
	return exec.Command(args[0], args[1:]...).Output()
}

func NewCommander() Commander { return execCommander{} }
