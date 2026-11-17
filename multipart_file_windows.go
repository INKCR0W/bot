package bot

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const fileFlagDeleteOnClose = 0x04000000

func createMultipartFile() (*os.File, error) {
	var suffix [16]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return nil, err
	}
	name, err := filepath.Abs(filepath.Join(os.TempDir(), "telegram-bot-upload-"+hex.EncodeToString(suffix[:])))
	if err != nil {
		return nil, err
	}
	path, err := syscall.UTF16PtrFromString(extendedUploadPath(name))
	if err != nil {
		return nil, err
	}
	// CREATE_NEW and exclusive access keep the spool private. A non-inheritable
	// delete-on-close handle lets Windows remove it even if the process exits.
	handle, err := syscall.CreateFile(path, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil,
		syscall.CREATE_NEW, syscall.FILE_ATTRIBUTE_NORMAL|fileFlagDeleteOnClose, 0)
	if err != nil {
		return nil, &os.PathError{Op: "create", Path: name, Err: err}
	}
	return os.NewFile(uintptr(handle), name), nil
}

func extendedUploadPath(name string) string {
	if strings.HasPrefix(name, `\\?\`) {
		return name
	}
	if strings.HasPrefix(name, `\\`) {
		return `\\?\UNC\` + name[2:]
	}
	return `\\?\` + name
}
