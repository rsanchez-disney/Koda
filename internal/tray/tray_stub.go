//go:build !darwin && !windows

package tray

import "fmt"

func Run(sr, version string)      { fmt.Println("Tray not supported on this platform") }
func EnableAutoStart() error      { return fmt.Errorf("not supported") }
func DisableAutoStart() error     { return fmt.Errorf("not supported") }
func AutoStartEnabled() bool      { return false }
