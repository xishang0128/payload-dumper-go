package file

import (
	"io"
	"os"
)

// LocalFile implements Reader for local files.
type LocalFile struct {
	file *os.File
	size int64
}

// NewLocalFile opens a local file for reading.
func NewLocalFile(path string) (*LocalFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}

	return &LocalFile{file: file, size: stat.Size()}, nil
}

func (f *LocalFile) ReadAt(p []byte, off int64) (int, error) {
	return f.file.ReadAt(p, off)
}

func (f *LocalFile) Close() error {
	return f.file.Close()
}

func (f *LocalFile) Size() int64 {
	return f.size
}

func (f *LocalFile) Read(offset int64, size int) ([]byte, error) {
	data := make([]byte, size)
	n, err := f.file.ReadAt(data, offset)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return data[:n], nil
}
