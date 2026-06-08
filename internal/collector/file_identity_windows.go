//go:build windows

package collector

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func fileIdentity(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(windows.Handle(file.Fd()), &info); err != nil {
		return "", err
	}

	return fmt.Sprintf("%d-%d-%d", info.VolumeSerialNumber, info.FileIndexHigh, info.FileIndexLow), nil
}
