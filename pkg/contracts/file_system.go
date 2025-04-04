package contracts

import "os"

type FileSystem interface {
	MkDirAll(path string, perm os.FileMode) error
	OpenFile(name string, flag int, perm os.FileMode) (*os.File, error)
	Executable() (string, error)
}
