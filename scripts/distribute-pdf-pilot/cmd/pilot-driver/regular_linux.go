//go:build linux

package main

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/sys/unix"
)

const directoryFlags = unix.O_RDONLY | unix.O_DIRECTORY | unix.O_NOFOLLOW | unix.O_NONBLOCK | unix.O_CLOEXEC
const regularFlags = unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_NONBLOCK | unix.O_CLOEXEC

func oneComponent(value string) bool {
	return value != "" && value != "." && value != ".." && !strings.ContainsAny(value, "/\\\x00")
}

func openDirectory(path string) (int, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return -1, errRefused
	}
	fd, err := unix.Open("/", directoryFlags, 0)
	if err != nil {
		return -1, errRefused
	}
	for _, part := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		if part == "" {
			continue
		}
		if !oneComponent(part) {
			unix.Close(fd)
			return -1, errRefused
		}
		child, e := unix.Openat(fd, part, directoryFlags, 0)
		unix.Close(fd)
		if e != nil {
			return -1, errRefused
		}
		fd = child
	}
	return fd, nil
}

func sameRegular(a, b unix.Stat_t) bool {
	return a.Dev == b.Dev && a.Ino == b.Ino && a.Mode == b.Mode && a.Nlink == b.Nlink && a.Size == b.Size &&
		a.Mtim == b.Mtim && a.Ctim == b.Ctim
}

func readRegularAt(directory int, name string, maximum int) ([]byte, error) {
	if !oneComponent(name) || maximum < 1 {
		return nil, errRefused
	}
	var named unix.Stat_t
	if unix.Fstatat(directory, name, &named, unix.AT_SYMLINK_NOFOLLOW) != nil || named.Mode&unix.S_IFMT != unix.S_IFREG || named.Nlink != 1 || named.Size < 1 || named.Size > int64(maximum) {
		return nil, errRefused
	}
	fd, err := unix.Openat(directory, name, regularFlags, 0)
	if err != nil {
		return nil, errRefused
	}
	file := os.NewFile(uintptr(fd), name)
	defer file.Close()
	var opened unix.Stat_t
	if unix.Fstat(fd, &opened) != nil || !sameRegular(named, opened) {
		return nil, errRefused
	}
	data, err := io.ReadAll(io.LimitReader(file, int64(maximum)+1))
	if err != nil || len(data) != int(named.Size) || len(data) > maximum {
		return nil, errRefused
	}
	var after, current unix.Stat_t
	if unix.Fstat(fd, &after) != nil || unix.Fstatat(directory, name, &current, unix.AT_SYMLINK_NOFOLLOW) != nil ||
		!sameRegular(named, after) || !sameRegular(named, current) {
		return nil, errRefused
	}
	return data, nil
}

func readBoundedRegular(path string, maximum int) ([]byte, error) {
	if path != fontPath && path != pdfPath {
		return nil, errRefused
	}
	directory, name := filepath.Split(path)
	fd, err := openDirectory(filepath.Clean(directory))
	if err != nil {
		return nil, errRefused
	}
	defer unix.Close(fd)
	return readRegularAt(fd, name, maximum)
}

type linuxPilotStage struct {
	parent *os.File
	dir *os.File
	name string
	identity unix.Stat_t
}

func sameDirectory(a, b unix.Stat_t) bool {
	return a.Dev == b.Dev && a.Ino == b.Ino && a.Mode&unix.S_IFMT == unix.S_IFDIR && b.Mode&unix.S_IFMT == unix.S_IFDIR &&
		a.Uid == b.Uid && a.Gid == b.Gid
}

func newPilotStage(attempt string) (pilotStage, error) {
	if !validHex(attempt, 16) {
		return nil, errRefused
	}
	parentFD, err := openDirectory("/out")
	if err != nil {
		return nil, errRefused
	}
	parent := os.NewFile(uintptr(parentFD), "/out")
	valid := false
	defer func() { if !valid { parent.Close() } }()
	var parentStat unix.Stat_t
	if unix.Fstat(parentFD, &parentStat) != nil || parentStat.Mode&unix.S_IFMT != unix.S_IFDIR ||
		parentStat.Mode&0777 != 0700 || parentStat.Uid != uint32(os.Getuid()) || parentStat.Gid != uint32(os.Getgid()) {
		return nil, errRefused
	}
	name := "pilot-" + attempt
	if unix.Mkdirat(parentFD, name, 0700) != nil {
		return nil, errRefused // never adopt an existing or partially used stage
	}
	dirFD, err := unix.Openat(parentFD, name, directoryFlags, 0)
	if err != nil {
		return nil, errRefused
	}
	dir := os.NewFile(uintptr(dirFD), name)
	defer func() { if !valid { dir.Close() } }()
	var identity unix.Stat_t
	if unix.Fstat(dirFD, &identity) != nil || identity.Mode&unix.S_IFMT != unix.S_IFDIR ||
		identity.Mode&0777 != 0700 || identity.Uid != uint32(os.Getuid()) || identity.Gid != uint32(os.Getgid()) {
		return nil, errRefused
	}
	valid = true
	return &linuxPilotStage{parent: parent, dir: dir, name: name, identity: identity}, nil
}

func (s *linuxPilotStage) current() error {
	if s == nil || s.parent == nil || s.dir == nil {
		return errRefused
	}
	var named, opened unix.Stat_t
	if unix.Fstatat(int(s.parent.Fd()), s.name, &named, unix.AT_SYMLINK_NOFOLLOW) != nil ||
		unix.Fstat(int(s.dir.Fd()), &opened) != nil || !sameDirectory(s.identity, named) || !sameDirectory(s.identity, opened) {
		return errRefused
	}
	return nil
}

func (s *linuxPilotStage) Read(name string, maximum int) ([]byte, error) {
	if s.current() != nil {
		return nil, errRefused
	}
	data, err := readRegularAt(int(s.dir.Fd()), name, maximum)
	if err != nil || s.current() != nil {
		return nil, errRefused
	}
	return data, nil
}

func (s *linuxPilotStage) Only(expected ...string) error {
	if s.current() != nil || len(expected) == 0 || len(expected) > 2 {
		return errRefused
	}
	if _, err := s.dir.Seek(0, io.SeekStart); err != nil {
		return errRefused
	}
	names, err := s.dir.Readdirnames(3)
	if err != nil && err != io.EOF {
		return errRefused
	}
	if len(names) != len(expected) {
		return errRefused
	}
	sort.Strings(names)
	want := append([]string(nil), expected...)
	sort.Strings(want)
	for i := range names {
		if names[i] != want[i] || !oneComponent(names[i]) {
			return errRefused
		}
	}
	return s.current()
}

func (s *linuxPilotStage) Close() error {
	if s == nil {
		return nil
	}
	var err error
	if s.dir != nil {
		err = s.dir.Close()
		s.dir = nil
	}
	if s.parent != nil {
		if e := s.parent.Close(); err == nil {
			err = e
		}
		s.parent = nil
	}
	return err
}
