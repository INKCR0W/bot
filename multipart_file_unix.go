//go:build !windows

package bot

import (
	"fmt"
	"os"
)

func createMultipartFile() (*os.File, error) {
	file, err := os.CreateTemp("", "telegram-bot-upload-*")
	if err != nil {
		return nil, err
	}
	// Unlink before writing any payload so process exit cannot leave an upload on disk.
	if err := os.Remove(file.Name()); err != nil {
		if closeErr := file.Close(); closeErr != nil {
			return nil, fmt.Errorf("%w (also failed to close temporary upload file: %v)", err, closeErr)
		}
		return nil, err
	}
	return file, nil
}
