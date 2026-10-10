//go:build !darwin

package main

import (
	"fmt"
	"github.com/egoist/mygo/ui"
)

func systemOpenWithApplications([]string) ([]openWithApplication, error) {
	return nil, fmt.Errorf("Application selection currently requires macOS")
}

func systemApplicationIcon(string) (*ui.Bitmap, error) { return nil, nil }
