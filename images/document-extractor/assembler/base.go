package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
)

const baseDiffID = "34467cc93912af891dfee8a3902adf9b4f355a9bca4eaa46f658d1181a21de0d"
const expandedBaseMax = 1 << 30

type chargedReader struct {
	ctx    context.Context
	reader io.Reader
	bytes  int64
}

func (r *chargedReader) Read(p []byte) (int, error) {
	if r.ctx.Err() != nil {
		return 0, errRefused
	}
	if len(p) > 64<<10 {
		p = p[:64<<10]
	}
	n, err := r.reader.Read(p)
	r.bytes += int64(n)
	if r.bytes > expandedBaseMax {
		return n, errRefused
	}
	return n, err
}
func octal(raw []byte) (int64, error) {
	if len(raw) > 0 && raw[0]&128 != 0 {
		return 0, errRefused
	}
	text := strings.Trim(string(raw), "\x00 ")
	if text == "" {
		return 0, nil
	}
	for _, c := range text {
		if c < '0' || c > '7' {
			return 0, errRefused
		}
	}
	value, err := strconv.ParseInt(text, 8, 63)
	if err != nil {
		return 0, errRefused
	}
	return value, nil
}
func tarText(raw []byte) string { return string(bytes.SplitN(raw, []byte{0}, 2)[0]) }
func unixMode(raw int64) os.FileMode {
	mode := os.FileMode(raw & 0777)
	if raw&04000 != 0 {
		mode |= os.ModeSetuid
	}
	if raw&02000 != 0 {
		mode |= os.ModeSetgid
	}
	if raw&01000 != 0 {
		mode |= os.ModeSticky
	}
	return mode
}
func zeroPadding(reader io.Reader, size int64) error {
	var buf [512]byte
	if size < 0 || size > 512 {
		return errRefused
	}
	if _, err := io.ReadFull(reader, buf[:int(size)]); err != nil {
		return errRefused
	}
	for _, b := range buf[:int(size)] {
		if b != 0 {
			return errRefused
		}
	}
	return nil
}
func parsePAX(body []byte) (map[string]string, error) {
	values := map[string]string{}
	for len(body) > 0 {
		space := bytes.IndexByte(body, ' ')
		if space < 1 || space > 10 {
			return nil, errRefused
		}
		for _, c := range body[:space] {
			if c < '0' || c > '9' {
				return nil, errRefused
			}
		}
		length, err := strconv.Atoi(string(body[:space]))
		if err != nil || length <= space+2 || length > len(body) || body[length-1] != '\n' {
			return nil, errRefused
		}
		key, value, ok := strings.Cut(string(body[space+1:length-1]), "=")
		if !ok || len(value) > 4096 {
			return nil, errRefused
		}
		if _, duplicate := values[key]; duplicate {
			return nil, errRefused
		}
		switch key {
		case "path", "linkpath", "mtime", "atime", "ctime":
		default:
			return nil, errRefused
		}
		values[key] = value
		body = body[length:]
	}
	return values, nil
}
func unpackBase(ctx context.Context, name, root string) error {
	file, err := os.Open(name)
	if err != nil {
		return errRefused
	}
	defer file.Close()
	raw := bufio.NewReaderSize(file, 64<<10)
	compressed, err := gzip.NewReader(raw)
	if err != nil {
		return errRefused
	}
	defer compressed.Close()
	compressed.Multistream(false)
	h := sha256.New()
	charged := &chargedReader{ctx: ctx, reader: io.TeeReader(compressed, h)}
	if err = unpackUSTAR(charged, root); err != nil {
		return err
	}
	if charged.bytes < 1 || hex.EncodeToString(h.Sum(nil)) != baseDiffID {
		return errRefused
	}
	if _, err = raw.Peek(1); err != io.EOF {
		return errRefused
	}
	return nil
}
func unpackUSTAR(reader io.Reader, root string) error {
	type directory struct {
		name     string
		mode     os.FileMode
		uid, gid int
	}
	dirs := []directory{}
	seen := map[string]bool{}
	var pax map[string]string
	for members := 0; members < 100000; members++ {
		var header [512]byte
		if _, err := io.ReadFull(reader, header[:]); err != nil {
			return errRefused
		}
		if header == ([512]byte{}) {
			if pax != nil || zeroPadding(reader, 512) != nil {
				return errRefused
			}
			var buffer [64 << 10]byte
			for {
				n, err := reader.Read(buffer[:])
				for _, b := range buffer[:n] {
					if b != 0 {
						return errRefused
					}
				}
				if err == io.EOF {
					break
				}
				if err != nil || n == 0 {
					return errRefused
				}
			}
			for i := len(dirs) - 1; i >= 0; i-- {
				d := dirs[i]
				if os.Chown(d.name, d.uid, d.gid) != nil || os.Chmod(d.name, d.mode) != nil {
					return errRefused
				}
			}
			return nil
		}
		sum, err := octal(header[148:156])
		if err != nil {
			return errRefused
		}
		var actual int64
		for i, b := range header {
			if i >= 148 && i < 156 {
				actual += 32
			} else {
				actual += int64(b)
			}
		}
		if sum != actual || string(header[257:265]) != "ustar\x0000" {
			return errRefused
		}
		size, e1 := octal(header[124:136])
		mode, e2 := octal(header[100:108])
		uid, e3 := octal(header[108:116])
		gid, e4 := octal(header[116:124])
		if e1 != nil || e2 != nil || e3 != nil || e4 != nil || size > 128<<20 || mode > 07777 || uid > 2147483647 || gid > 2147483647 {
			return errRefused
		}
		kind := header[156]
		name, target := tarText(header[:100]), tarText(header[157:257])
		prefix := tarText(header[345:500])
		if prefix != "" {
			name = prefix + "/" + name
		}
		if kind == 'x' {
			if pax != nil || size > 1<<20 {
				return errRefused
			}
			body := make([]byte, int(size))
			if _, err = io.ReadFull(reader, body); err != nil {
				return errRefused
			}
			pax, err = parsePAX(body)
			if err != nil || zeroPadding(reader, (-size)&511) != nil {
				return errRefused
			}
			continue
		}
		if pax != nil {
			if value, ok := pax["path"]; ok {
				name = value
			}
			if value, ok := pax["linkpath"]; ok {
				target = value
			}
			pax = nil
		}
		name = strings.TrimSuffix(strings.TrimPrefix(name, "./"), "/")
		if name == "" || name == "." {
			if kind != '5' || size != 0 {
				return errRefused
			}
			continue
		}
		if !relativeUnixName(name) || seen[name] || strings.HasPrefix(path.Base(name), ".wh.") {
			return errRefused
		}
		seen[name] = true
		if kind != 0 && kind != '0' && kind != '5' && kind != '1' && kind != '2' || kind != '0' && kind != 0 && size != 0 {
			return errRefused
		}
		full, err := rooted(root, name)
		if err != nil {
			return errRefused
		}
		if _, err = makeRootDir(root, path.Dir(name), 0700); err != nil && path.Dir(name) != "." {
			return errRefused
		}
		switch kind {
		case '5':
			if os.MkdirAll(full, 0700) != nil {
				return errRefused
			}
			dirs = append(dirs, directory{full, unixMode(mode), int(uid), int(gid)})
		case '2':
			clean := target
			if strings.HasPrefix(clean, "/") {
				clean = strings.TrimPrefix(clean, "/")
			} else {
				clean = path.Join(path.Dir(name), clean)
			}
			if !relativeUnixName(path.Clean(clean)) || os.Symlink(target, full) != nil || os.Lchown(full, int(uid), int(gid)) != nil {
				return errRefused
			}
		case '1':
			link, err := rooted(root, strings.TrimPrefix(target, "./"))
			if err != nil {
				return errRefused
			}
			info, err := os.Lstat(link)
			if err != nil || !info.Mode().IsRegular() || os.Link(link, full) != nil {
				return errRefused
			}
		default:
			file, err := os.OpenFile(full, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if err != nil {
				return errRefused
			}
			n, copyErr := io.CopyN(file, reader, size)
			closeErr := file.Close()
			if copyErr != nil || n != size || closeErr != nil || os.Chown(full, int(uid), int(gid)) != nil || os.Chmod(full, unixMode(mode)) != nil {
				return errRefused
			}
		}
		if zeroPadding(reader, (-size)&511) != nil {
			return errRefused
		}
	}
	return errRefused
}
