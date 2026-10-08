//go:build windows

package resourcebudget

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func localStorageRecordPrivate(f *os.File, info os.FileInfo) bool {
	if !info.Mode().IsRegular() {
		return false
	}
	h := windows.Handle(f.Fd())
	var details windows.ByHandleFileInformation
	if windows.GetFileInformationByHandle(h, &details) != nil || details.NumberOfLinks != 1 {
		return false
	}
	sd, e := windows.GetSecurityInfo(h, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if e != nil {
		return false
	}
	control, _, e := sd.Control()
	if e != nil || control&windows.SE_DACL_PROTECTED == 0 {
		return false
	}
	acl, _, e := sd.DACL()
	if e != nil || acl == nil || acl.AceCount == 0 {
		return false
	}
	user, e := windows.GetCurrentProcessToken().GetTokenUser()
	if e != nil {
		return false
	}
	for i := uint32(0); i < uint32(acl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if windows.GetAce(acl, i, &ace) != nil || ace == nil || ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Header.AceSize < uint16(unsafe.Sizeof(windows.ACCESS_ALLOWED_ACE{})) {
			return false
		}
		if !(*windows.SID)(unsafe.Pointer(&ace.SidStart)).Equals(user.User.Sid) {
			return false
		}
	}
	return true
}

// Native handles establish file identity and its final local volume without
// reading any VHD bytes. Network/reparse targets that do not resolve to a
// mounted local volume are rejected by the caller's complete inventory.
func openLocalStorageFile(path string, read bool) (*os.File, string, string, error) {
	name, e := windows.UTF16PtrFromString(path)
	if e != nil {
		return nil, "", "", e
	}
	access := uint32(windows.FILE_READ_ATTRIBUTES)
	if read {
		access |= windows.GENERIC_READ
	}
	h, e := windows.CreateFile(name, access, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if e != nil {
		return nil, "", "", e
	}
	f := os.NewFile(uintptr(h), "local-storage-probe")
	failed := true
	defer func() {
		if failed {
			f.Close()
		}
	}()
	var info windows.ByHandleFileInformation
	if e = windows.GetFileInformationByHandle(h, &info); e != nil {
		return nil, "", "", e
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 || info.NumberOfLinks != 1 {
		return nil, "", "", errors.New("storage file must be a regular single-link file")
	}
	buf := make([]uint16, 32768)
	n, e := windows.GetFinalPathNameByHandle(h, &buf[0], uint32(len(buf)), 1) // VOLUME_NAME_GUID, normalized path
	if e != nil || n == 0 || n >= uint32(len(buf)) {
		return nil, "", "", errors.New("storage file must resolve to a normal local volume")
	}
	final := windows.UTF16ToString(buf[:n])
	end := strings.Index(final, "}\\")
	if end < 0 {
		return nil, "", "", errors.New("storage file has no local volume identity")
	}
	guid := final[:end+2]
	if !storageVolumePattern.MatchString(guid) {
		return nil, "", "", errors.New("storage file has no local volume identity")
	}
	identity := fmt.Sprintf("%08x:%08x%08x:%08x%08x", info.VolumeSerialNumber, info.FileIndexHigh, info.FileIndexLow, info.CreationTime.HighDateTime, info.CreationTime.LowDateTime)
	failed = false
	return f, guid, identity, nil
}

func nativeStorageSettingsHash() (string, error) {
	// Worker HOME/APPDATA are deliberately redirected. The OS KnownFolder is
	// authoritative for the logged-in Windows user's Docker Desktop profile.
	roaming, e := windows.KnownFolderPath(windows.FOLDERID_RoamingAppData, 0)
	if e != nil {
		return "", e
	}
	settings := filepath.Join(roaming, "Docker", "settings-store.json")
	f, _, id, e := openLocalStorageFile(settings, true)
	if e != nil {
		return "", errors.New("Docker Desktop settings-store.json could not be read from the Windows user profile")
	}
	defer f.Close()
	before, e := f.Stat()
	if e != nil || before.Size() <= 0 || before.Size() > 2<<20 {
		return "", errors.New("Docker Desktop settings are missing or exceed the bounded fingerprint limit")
	}
	h := sha256.New()
	n, e := io.Copy(h, io.LimitReader(f, (2<<20)+1))
	if e != nil || n != before.Size() {
		return "", errors.New("Docker Desktop settings changed during fingerprinting")
	}
	after, e := f.Stat()
	if e != nil || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return "", errors.New("Docker Desktop settings changed during fingerprinting")
	}
	current, _, currentID, e := openLocalStorageFile(settings, false)
	if e != nil {
		return "", errors.New("Docker Desktop settings changed during fingerprinting")
	}
	current.Close()
	if id != currentID {
		return "", errors.New("Docker Desktop settings changed during fingerprinting")
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func volumeMountPaths(guid string) ([]string, error) {
	name, _ := windows.UTF16PtrFromString(guid)
	buffer := make([]uint16, 256)
	var needed uint32
	e := windows.GetVolumePathNamesForVolumeName(name, &buffer[0], uint32(len(buffer)), &needed)
	if errors.Is(e, windows.ERROR_MORE_DATA) {
		if needed == 0 || needed > 32768 {
			return nil, errors.New("mounted volume path inventory exceeds its bound")
		}
		buffer = make([]uint16, needed)
		e = windows.GetVolumePathNamesForVolumeName(name, &buffer[0], uint32(len(buffer)), &needed)
	}
	if e != nil || needed > uint32(len(buffer)) {
		return nil, errors.New("mounted volume path inventory is incomplete")
	}
	var paths []string
	start := 0
	for i, value := range buffer {
		if value != 0 {
			continue
		}
		if i == start {
			break
		}
		paths = append(paths, windows.UTF16ToString(buffer[start:i]))
		start = i + 1
	}
	sort.Strings(paths)
	return paths, nil
}

func nativeLocalVolumes(ctx context.Context, measure bool) ([]localVolume, error) {
	buf := make([]uint16, 128)
	h, e := windows.FindFirstVolume(&buf[0], uint32(len(buf)))
	if e != nil {
		return nil, errors.New("Windows local-volume inventory is unavailable")
	}
	defer windows.FindVolumeClose(h)
	var rows []localVolume
	seen := map[string]bool{}
	for count := 0; ; count++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if count >= 128 {
			return nil, errors.New("Windows local-volume inventory exceeds its bound")
		}
		guid := windows.UTF16ToString(buf)
		if !storageVolumePattern.MatchString(guid) || seen[strings.ToLower(guid)] {
			return nil, errors.New("Windows local-volume inventory is ambiguous")
		}
		seen[strings.ToLower(guid)] = true
		paths, e := volumeMountPaths(guid)
		if e != nil {
			return nil, e
		}
		// Unmounted recovery volumes cannot hold the normal mounted path in
		// this explicitly supported layout. Do not count them as data drives.
		if len(paths) > 0 {
			name, _ := windows.UTF16PtrFromString(guid)
			kind := windows.GetDriveType(name)
			if kind != windows.DRIVE_CDROM {
				if _, e := localStorageVolumeEligible(true, kind, 0); e != nil {
					return nil, e
				}
				var flags uint32
				if windows.GetVolumeInformation(name, nil, 0, nil, nil, &flags, nil, 0) != nil {
					return nil, errors.New("a mounted local volume's capabilities are unavailable")
				}
				if eligible, _ := localStorageVolumeEligible(true, kind, flags); eligible {
					row := localVolume{guid: strings.ToLower(guid), signature: strings.Join(paths, "\x00"), label: ""}
					for _, p := range paths {
						if len(p) == 3 && p[1:] == ":\\" {
							row.label = strings.ToUpper(p)
							break
						}
					}
					if measure {
						var total, free uint64
						if windows.GetDiskFreeSpaceEx(name, &row.available, &total, &free) != nil || total == 0 || row.available > total {
							return nil, errors.New("available space could not be measured on every eligible mounted local volume")
						}
					}
					rows = append(rows, row)
				}
			}
		}
		e = windows.FindNextVolume(h, &buf[0], uint32(len(buf)))
		if errors.Is(e, windows.ERROR_NO_MORE_FILES) {
			break
		}
		if e != nil {
			return nil, errors.New("Windows local-volume inventory is incomplete")
		}
	}
	if len(rows) == 0 {
		return nil, errors.New("no mounted writable local data volume was observed")
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].guid < rows[j].guid })
	for i := range rows {
		if rows[i].label == "" {
			// Only the host Super Admin status includes this selected mount
			// path. Shared/project capacity DTOs contain no limiting label.
			rows[i].label = strings.Split(rows[i].signature, "\x00")[0]
		}
	}
	return rows, nil
}

func probeLocalStorage(ctx context.Context, path string) (LocalStorageObservation, error) {
	var out LocalStorageObservation
	if ctx.Err() != nil {
		return out, ctx.Err()
	}
	user, e := windows.GetCurrentProcessToken().GetTokenUser()
	if e != nil {
		return out, errors.New("Windows user identity is unavailable")
	}
	f, volume, identity, e := openLocalStorageFile(path, false)
	if e != nil {
		return out, errors.New("The selected existing VHD could not be identified on a local Windows volume. Check its actual Docker Desktop location and access.")
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil || info.Size() == 0 {
		return out, errors.New("The selected VHD must be an existing nonempty regular file")
	}
	hash, e := nativeStorageSettingsHash()
	if e != nil {
		return out, e
	}
	before, e := nativeLocalVolumes(ctx, true)
	if e != nil {
		return out, e
	}
	after, e := nativeLocalVolumes(ctx, false)
	if e != nil {
		return out, e
	}
	minimum, limiting, e := localStorageVolumeMinimum(before, after, volume)
	if e != nil {
		return out, e
	}
	// Reopen the selected name to reject replacement/retargeting while the
	// inventory was collected. Growing file size is not an identity change.
	current, currentVolume, currentIdentity, e := openLocalStorageFile(path, false)
	if e != nil {
		return out, errors.New("The selected VHD changed during observation")
	}
	current.Close()
	if currentIdentity != identity || !strings.EqualFold(currentVolume, volume) {
		return out, errors.New("The selected VHD changed during observation")
	}
	if ctx.Err() != nil {
		return out, ctx.Err()
	}
	out = LocalStorageObservation{WindowsSID: user.User.Sid.String(), VHDPath: path, FileIdentity: identity, VolumeGUID: volume, SettingsSHA256: hash, ObservedAt: time.Now().UTC(), MinimumAvailableBytes: minimum, MountedVolumeCount: len(before), LimitingVolume: limiting}
	return out, nil
}
