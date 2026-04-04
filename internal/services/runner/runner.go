package runner

import "os/exec"

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
