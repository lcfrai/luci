package resourcebudget

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
)

const LocalStorageBasis = "operator-confirmed-local-layout; minimum-free-space-across-all-mounted-writable-local-volumes"

// LocalStorageConfirmation is private host configuration, never a project
// grant or an automatic assertion that Docker uses a particular VHD. Only the
// management gateway's current host operator may publish a new revision.
type LocalStorageConfirmation struct {
	Version        int       `json:"version"`
	Revision       uint64    `json:"revision"`
	ConfirmedAt    time.Time `json:"confirmedAt"`
	ConfirmedBy    string    `json:"confirmedBy"`
	EngineID       string    `json:"engineId"`
	DesktopVersion string    `json:"desktopVersion"`
	WindowsSID     string    `json:"windowsSid"`
	VHDPath        string    `json:"vhdPath"`
	FileIdentity   string    `json:"fileIdentity"`
	VolumeGUID     string    `json:"volumeGuid"`
	SettingsSHA256 string    `json:"settingsSha256"`
}

// LocalStorageStatus is restricted to the host Super Admin endpoint. Paths,
// file identities, settings hashes and SID must not enter team resource DTOs.
type LocalStorageStatus struct {
	Platform            string    `json:"platform"`
	Scope               string    `json:"scope"`
	Required            bool      `json:"required"`
	State               string    `json:"state"`
	Revision            uint64    `json:"revision"`
	ReasonCode          string    `json:"reasonCode"`
	Message             string    `json:"message"`
	Basis               string    `json:"basis"`
	ObservedAt          time.Time `json:"observedAt,omitzero"`
	ConfirmedAt         time.Time `json:"confirmedAt,omitzero"`
	VHDPath             string    `json:"vhdPath,omitempty"`
	DesktopVersion      string    `json:"desktopVersion,omitempty"`
	MountedVolumeCount  int       `json:"mountedVolumeCount,omitempty"`
	LimitingVolume      string    `json:"limitingVolume,omitempty"`
	HostAvailableBytes  *uint64   `json:"hostAvailableBytes,omitempty"`
	GuestAvailableBytes *uint64   `json:"guestAvailableBytes,omitempty"`
}

// HostStorageBound contains no operator path or native identifiers. The
// measured lower bound is separate from the Linux guest's filesystem space.
type HostStorageBound struct {
	Required           bool   `json:"required"`
	State              string `json:"state"`
	ReasonCode         string `json:"reasonCode"`
	Message            string `json:"message"`
	Disk               *Disk  `json:"disk"`
	MountedVolumeCount int    `json:"mountedVolumeCount"`
}

// LocalStorageObservation is native probe output kept inside the controller.
// No VHD contents are read. Only the Desktop settings file is hashed.
type LocalStorageObservation struct {
	WindowsSID, VHDPath, FileIdentity, VolumeGUID, SettingsSHA256 string
	ObservedAt                                                    time.Time
	MinimumAvailableBytes                                         uint64
	MountedVolumeCount                                            int
	LimitingVolume                                                string
}

type LocalStorageError struct{ Code, Message string }

func (e *LocalStorageError) Error() string { return e.Message }

type localVolume struct {
	guid, signature, label string
	available              uint64
}

func localStorageVolumeEligible(mounted bool, kind, flags uint32) (bool, error) {
	if !mounted || kind == 5 {
		return false, nil
	}
	if kind != 2 && kind != 3 && kind != 6 {
		return false, errors.New("a mounted storage volume has an unsupported or unknown locality")
	}
	return flags&0x00080000 == 0, nil
}

func localStorageVolumeMinimum(before, after []localVolume, target string) (uint64, string, error) {
	if len(before) == 0 || len(before) != len(after) {
		return 0, "", errors.New("The complete mounted local-volume inventory is required")
	}
	matched := false
	minimum := ^uint64(0)
	limiting := ""
	seen := map[string]bool{}
	for i, row := range before {
		key := strings.ToLower(row.guid)
		if seen[key] || !storageVolumePattern.MatchString(row.guid) || key != strings.ToLower(after[i].guid) || row.signature != after[i].signature || row.label == "" {
			return 0, "", errors.New("The mounted local-volume inventory changed or is ambiguous; retry the observation")
		}
		seen[key] = true
		if strings.EqualFold(row.guid, target) {
			matched = true
		}
		if row.available < minimum {
			minimum, limiting = row.available, row.label
		}
	}
	if !matched {
		return 0, "", errors.New("The selected VHD does not resolve to an eligible mounted writable local volume")
	}
	return minimum, limiting, nil
}

var ErrLocalStorageConfirmation = errors.New("The private Docker storage confirmation is missing, unreadable or invalid; a host Super Admin must review local storage in Settings.")
var storageHashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
var storageSIDPattern = regexp.MustCompile(`^S-1-[0-9-]{1,180}$`)
var storageVolumePattern = regexp.MustCompile(`(?i)^\x5c\x5c\?\x5cVolume\{[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\}\x5c$`)

// ValidLocalVHDPath deliberately accepts a normal drive-rooted path only.
// Native resolution separately checks every reparse target is a mounted local
// volume. The operator may use a normal directory mount or local junction.
func ValidLocalVHDPath(path string) bool {
	if len(path) < 8 || len(path) > 32700 || !((path[0] >= 'A' && path[0] <= 'Z') || (path[0] >= 'a' && path[0] <= 'z')) || path[1:3] != ":\\" || strings.ContainsAny(path[2:], "/:\x00\r\n") {
		return false
	}
	parts := strings.Split(path[3:], "\\")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || strings.TrimRight(part, " .") != part || strings.ContainsAny(part, `<>"|?*`) {
			return false
		}
		for _, r := range part {
			if unicode.IsControl(r) {
				return false
			}
		}
		stem := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		dos := []rune(stem)
		if stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" || stem == "CLOCK$" || stem == "CONIN$" || stem == "CONOUT$" || len(dos) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && strings.ContainsRune("0123456789¹²³", dos[3]) {
			return false
		}
	}
	ext := strings.ToLower(parts[len(parts)-1])
	return strings.HasSuffix(ext, ".vhdx") || strings.HasSuffix(ext, ".vhd")
}

func (r LocalStorageConfirmation) Valid() bool {
	return r.Version == 1 && r.Revision > 0 && !r.ConfirmedAt.IsZero() && len(r.ConfirmedBy) > 0 && len(r.ConfirmedBy) <= 200 && len(r.EngineID) > 0 && len(r.EngineID) <= 128 && len(r.DesktopVersion) > 0 && len(r.DesktopVersion) <= 200 && storageSIDPattern.MatchString(r.WindowsSID) && ValidLocalVHDPath(r.VHDPath) && len(r.FileIdentity) > 0 && len(r.FileIdentity) <= 200 && storageVolumePattern.MatchString(r.VolumeGUID) && storageHashPattern.MatchString(r.SettingsSHA256)
}

func ReadLocalStorageConfirmation(path string) (LocalStorageConfirmation, error) {
	var out LocalStorageConfirmation
	if !filepath.IsAbs(path) {
		return out, ErrLocalStorageConfirmation
	}
	root, e := OpenLocalStorageDirectory(filepath.Dir(path))
	if e != nil {
		return out, e
	}
	defer root.Close()
	name := filepath.Base(path)
	before, e := root.Lstat(name)
	if e != nil {
		return out, e
	}
	if !before.Mode().IsRegular() || before.Size() > 16<<10 {
		return out, ErrLocalStorageConfirmation
	}
	f, e := root.Open(name)
	if e != nil {
		return out, ErrLocalStorageConfirmation
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil || !os.SameFile(before, info) || !localStorageRecordPrivate(f, info) {
		return out, ErrLocalStorageConfirmation
	}
	d := json.NewDecoder(io.LimitReader(f, (16<<10)+1))
	d.DisallowUnknownFields()
	if d.Decode(&out) != nil || !errors.Is(d.Decode(new(any)), io.EOF) || !out.Valid() {
		return LocalStorageConfirmation{}, ErrLocalStorageConfirmation
	}
	after, e := root.Lstat(name)
	if e != nil || !os.SameFile(info, after) || info.Size() != after.Size() || !info.ModTime().Equal(after.ModTime()) {
		return LocalStorageConfirmation{}, ErrLocalStorageConfirmation
	}
	return out, nil
}

// OpenLocalStorageDirectory pins each existing parent without following
// symlinks/junctions. The host-private policy root cannot become worker data.
func OpenLocalStorageDirectory(directory string) (*os.Root, error) {
	if !filepath.IsAbs(directory) || filepath.Clean(directory) != directory {
		return nil, ErrLocalStorageConfirmation
	}
	volume := filepath.VolumeName(directory)
	if strings.HasPrefix(volume, `\\`) {
		return nil, ErrLocalStorageConfirmation
	}
	base := volume + string(filepath.Separator)
	root, e := os.OpenRoot(base)
	if e != nil {
		return nil, e
	}
	for _, component := range strings.Split(strings.TrimPrefix(directory, base), string(filepath.Separator)) {
		if component == "" {
			continue
		}
		if component == "." || component == ".." {
			root.Close()
			return nil, ErrLocalStorageConfirmation
		}
		before, e := root.Lstat(component)
		if e != nil {
			root.Close()
			return nil, e
		}
		if !before.IsDir() || before.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			root.Close()
			return nil, ErrLocalStorageConfirmation
		}
		next, e := root.OpenRoot(component)
		if e != nil {
			root.Close()
			return nil, e
		}
		after, e := next.Stat(".")
		root.Close()
		if e != nil || !os.SameFile(before, after) {
			next.Close()
			return nil, ErrLocalStorageConfirmation
		}
		root = next
	}
	return root, nil
}

// ValidateLocalStoragePins does not assert an engine/VHD binding. It makes an
// earlier operator assertion expire when its corroborating facts change.
func ValidateLocalStoragePins(r LocalStorageConfirmation, engine, version string, o LocalStorageObservation) error {
	if !r.Valid() || engine != r.EngineID || version != r.DesktopVersion || o.WindowsSID != r.WindowsSID || !strings.EqualFold(o.VHDPath, r.VHDPath) || o.FileIdentity != r.FileIdentity || !strings.EqualFold(o.VolumeGUID, r.VolumeGUID) || o.SettingsSHA256 != r.SettingsSHA256 {
		return errors.New("Docker, Windows identity, the selected VHD or Desktop settings changed. A host Super Admin must verify the active and pending local storage locations again in Settings.")
	}
	return nil
}

// ValidateHostStorage is image-independent: callers supply a justified demand
// for either worker allocation or runtime image preparation. Nil is permitted
// only when that host/platform explicitly does not require a backing bound.
func ValidateHostStorage(bound *HostStorageBound, required bool, demand uint64, now time.Time, maxAge time.Duration) error {
	if !required && (bound == nil || !bound.Required) {
		return nil
	}
	if bound != nil && (bound.State == "confirmation-required" || bound.State == "invalidated" || bound.State == "pending-observation") {
		return &LocalStorageError{Code: "confirmation_required", Message: "A host Super Admin must confirm the current active and pending local Docker storage locations in Settings before new heavy work."}
	}
	if bound == nil || !bound.Required || bound.State != "confirmed" || bound.Disk == nil || bound.Disk.AvailableBytes == nil || bound.Disk.ObservedAt.IsZero() || bound.Disk.ObservedAt.After(now) || now.Sub(bound.Disk.ObservedAt) > maxAge || bound.MountedVolumeCount < 1 {
		return &LocalStorageError{Code: "unavailable", Message: "Fresh confirmed Windows Docker backing-storage capacity is required. Review Docker storage in host Settings before starting new heavy work."}
	}
	if *bound.Disk.AvailableBytes < demand {
		return &LocalStorageError{Code: "insufficient_capacity", Message: "The lowest free space across mounted local Windows volumes is below this operation's disk estimate and reserve. Review the limiting volume in host Settings; Docker guest free space is separate."}
	}
	return nil
}

func ProbeLocalStorage(ctx context.Context, path string) (LocalStorageObservation, error) {
	if !ValidLocalVHDPath(path) {
		return LocalStorageObservation{}, errors.New("Choose the existing VHD or VHDX using its full normal local-drive path. Network, device and alternate-stream paths are unsupported.")
	}
	return probeLocalStorage(ctx, path)
}
