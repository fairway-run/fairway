package knowledge

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"golang.org/x/sys/unix"
)

type boundDirectory struct {
	file *os.File
}

type boundIdentity struct {
	dev uint64
	ino uint64
}

var tempSequence atomic.Uint64

func openBoundDirectory(projectRoot, relativeDirectory string, create bool) (*boundDirectory, error) {
	clean := filepath.Clean(filepath.FromSlash(strings.TrimSpace(relativeDirectory)))
	if clean == "." {
		clean = ""
	}
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return nil, errors.New("knowledge directory path is unsafe")
	}
	fd, err := unix.Open(projectRoot, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, errors.New("open knowledge project directory")
	}
	current := os.NewFile(uintptr(fd), projectRoot)
	for _, part := range strings.Split(clean, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		nextFD, openErr := unix.Openat(int(current.Fd()), part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		if openErr != nil && create && errors.Is(openErr, unix.ENOENT) {
			if mkdirErr := unix.Mkdirat(int(current.Fd()), part, 0o755); mkdirErr != nil && !errors.Is(mkdirErr, unix.EEXIST) {
				_ = current.Close()
				return nil, errors.New("create bound knowledge directory")
			}
			nextFD, openErr = unix.Openat(int(current.Fd()), part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		}
		if openErr != nil {
			_ = current.Close()
			return nil, errors.New("open bound knowledge directory")
		}
		next := os.NewFile(uintptr(nextFD), part)
		_ = current.Close()
		current = next
	}
	return &boundDirectory{file: current}, nil
}

func (d *boundDirectory) Close() error {
	if d == nil || d.file == nil {
		return nil
	}
	return d.file.Close()
}

func splitBoundPath(relativePath string) (string, string, error) {
	clean := filepath.Clean(filepath.FromSlash(strings.TrimSpace(relativePath)))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", "", errors.New("knowledge file path is unsafe")
	}
	name := filepath.Base(clean)
	if name == "." || name == string(filepath.Separator) || strings.Contains(name, string(filepath.Separator)) {
		return "", "", errors.New("knowledge file name is unsafe")
	}
	dir := filepath.Dir(clean)
	if dir == "." {
		dir = ""
	}
	return dir, name, nil
}

func readBoundProjectFile(paths resolvedPaths, relativePath string, limit int64, stage string, hook func(string)) ([]byte, boundIdentity, error) {
	dirRel, name, err := splitBoundPath(relativePath)
	if err != nil {
		return nil, boundIdentity{}, err
	}
	dir, err := openBoundDirectory(paths.project, dirRel, false)
	if err != nil {
		return nil, boundIdentity{}, err
	}
	defer dir.Close()
	data, identity, err := readBoundFileAt(dir, name, limit, stage, hook)
	return data, identity, err
}

func readBoundFileAt(dir *boundDirectory, name string, limit int64, stage string, hook func(string)) ([]byte, boundIdentity, error) {
	fd, err := unix.Openat(int(dir.file.Fd()), name, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, boundIdentity{}, errors.New("open bound knowledge file")
	}
	file := os.NewFile(uintptr(fd), name)
	defer file.Close()
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil || stat.Mode&unix.S_IFMT != unix.S_IFREG {
		return nil, boundIdentity{}, errors.New("bound knowledge path is not a regular file")
	}
	if stat.Size > limit {
		return nil, boundIdentity{}, errors.New("bound knowledge file exceeds size limit")
	}
	if hook != nil && stage != "" {
		hook(stage)
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, boundIdentity{}, errors.New("read bound knowledge file")
	}
	return data, boundIdentity{dev: uint64(stat.Dev), ino: uint64(stat.Ino)}, nil
}

func boundFileExists(dir *boundDirectory, name string) (bool, error) {
	var stat unix.Stat_t
	err := unix.Fstatat(int(dir.file.Fd()), name, &stat, unix.AT_SYMLINK_NOFOLLOW)
	if errors.Is(err, unix.ENOENT) {
		return false, nil
	}
	if err != nil {
		return false, errors.New("inspect bound knowledge file")
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG {
		return false, errors.New("bound knowledge path is not a regular file")
	}
	return true, nil
}

func createBoundFile(dir *boundDirectory, name string, data []byte, mode uint32) error {
	fd, err := unix.Openat(int(dir.file.Fd()), name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC|unix.O_NOFOLLOW, mode)
	if err != nil {
		return errors.New("create bound knowledge file")
	}
	file := os.NewFile(uintptr(fd), name)
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = unix.Unlinkat(int(dir.file.Fd()), name, 0)
		return errors.New("write bound knowledge file")
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = unix.Unlinkat(int(dir.file.Fd()), name, 0)
		return errors.New("sync bound knowledge file")
	}
	if err := file.Close(); err != nil {
		_ = unix.Unlinkat(int(dir.file.Fd()), name, 0)
		return errors.New("close bound knowledge file")
	}
	return nil
}

func replaceBoundFile(dir *boundDirectory, name string, expected, next []byte, mode uint32, stage string, hook func(string)) error {
	current, identity, err := readBoundFileAt(dir, name, int64(max(len(expected), len(next)))+1, "", nil)
	if err != nil || !bytes.Equal(current, expected) {
		return errors.New("knowledge file changed during operation")
	}
	if hook != nil && stage != "" {
		hook(stage)
	}
	var stat unix.Stat_t
	if err := unix.Fstatat(int(dir.file.Fd()), name, &stat, unix.AT_SYMLINK_NOFOLLOW); err != nil ||
		stat.Mode&unix.S_IFMT != unix.S_IFREG || uint64(stat.Dev) != identity.dev || uint64(stat.Ino) != identity.ino {
		return errors.New("knowledge file identity changed during operation")
	}
	tempName := fmt.Sprintf(".fairway-knowledge-%d-%d", os.Getpid(), tempSequence.Add(1))
	if err := createBoundFile(dir, tempName, next, mode); err != nil {
		return err
	}
	if err := unix.Renameat(int(dir.file.Fd()), tempName, int(dir.file.Fd()), name); err != nil {
		_ = unix.Unlinkat(int(dir.file.Fd()), tempName, 0)
		return errors.New("promote bound knowledge temp file")
	}
	return nil
}

func removeBoundFile(dir *boundDirectory, name string) {
	_ = unix.Unlinkat(int(dir.file.Fd()), name, 0)
}
