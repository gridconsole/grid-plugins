//go:build !linux && !darwin

package env

import (
	"errors"
	"os/exec"
	"time"
)

func supported() error {
	return errors.New("dev environments require macOS or Linux; Windows job object supervision is not implemented, use WSL2")
}
func configure(*exec.Cmd) error     { return supported() }
func stop(int, time.Duration) error { return supported() }
