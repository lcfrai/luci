//go:build windows

package management

import (
	"encoding/hex"
	"errors"
	"golang.org/x/sys/windows"
	"strings"
	"unsafe"
)

// Catalog-signed Windows driver tools require membership verification of the
// exact retained file, not merely a valid signature on a nearby .cat file.
// Microsoft primary sample: Windows-classic-samples/Samples/Security/CodeSigning.
// All trust checks are cache-only. No cert/network/package acquisition occurs.
type managedModelCatalogInfo struct {
	size uint32
	path [260]uint16
}
type managedModelTrustCatalog struct {
	size, version                  uint32
	catalog, memberTag, memberPath *uint16
	memberFile                     windows.Handle
	hash                           *byte
	hashBytes                      uint32
	catalogContext                 uintptr
	admin                          windows.Handle
}

func verifyManagedModelCatalog(path *uint16, file windows.Handle, vendor string) (result error) {
	dll := windows.NewLazySystemDLL("wintrust.dll")
	var admin windows.Handle
	algorithm, _ := windows.UTF16PtrFromString("SHA256")
	ok, _, _ := dll.NewProc("CryptCATAdminAcquireContext2").Call(uintptr(unsafe.Pointer(&admin)), 0, uintptr(unsafe.Pointer(algorithm)), 0, 0)
	if ok == 0 || admin == 0 {
		return errors.New("installed catalog authority unavailable")
	}
	defer func() {
		ok, _, _ := dll.NewProc("CryptCATAdminReleaseContext").Call(uintptr(admin), 0)
		if ok == 0 {
			result = errors.Join(result, errors.New("catalog authority cleanup unavailable"))
		}
	}()
	var size uint32
	ok, _, _ = dll.NewProc("CryptCATAdminCalcHashFromFileHandle2").Call(uintptr(admin), uintptr(file), uintptr(unsafe.Pointer(&size)), 0, 0)
	if ok == 0 || size != 32 {
		return errors.New("installed catalog member hash unavailable")
	}
	hash := make([]byte, size)
	ok, _, _ = dll.NewProc("CryptCATAdminCalcHashFromFileHandle2").Call(uintptr(admin), uintptr(file), uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&hash[0])), 0)
	if ok == 0 || size != 32 {
		return errors.New("installed catalog member hash unavailable")
	}
	catalog, _, _ := dll.NewProc("CryptCATAdminEnumCatalogFromHash").Call(uintptr(admin), uintptr(unsafe.Pointer(&hash[0])), uintptr(size), 0, 0)
	if catalog == 0 {
		return errors.New("installed catalog membership unavailable")
	}
	defer func() {
		ok, _, _ := dll.NewProc("CryptCATAdminReleaseCatalogContext").Call(uintptr(admin), catalog, 0)
		if ok == 0 {
			result = errors.Join(result, errors.New("catalog member cleanup unavailable"))
		}
	}()
	info := managedModelCatalogInfo{}
	info.size = uint32(unsafe.Sizeof(info))
	ok, _, _ = dll.NewProc("CryptCATCatalogInfoFromContext").Call(catalog, uintptr(unsafe.Pointer(&info)), 0)
	if ok == 0 {
		return errors.New("installed catalog path unavailable")
	}
	catalogPath := windows.UTF16ToString(info.path[:])
	system, e := windows.GetSystemDirectory()
	if e != nil || !strings.HasPrefix(strings.ToLower(catalogPath), strings.ToLower(system+`\CatRoot\`)) {
		return errors.New("driver catalog is outside the Windows catalog store")
	}
	if e = managedModelOrdinaryPath(catalogPath, false); e != nil {
		return e
	}
	catalogUTF, e := windows.UTF16PtrFromString(catalogPath)
	if e != nil {
		return e
	}
	lock, e := windows.CreateFile(catalogUTF, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if e != nil {
		return e
	}
	defer func() {
		if e := windows.CloseHandle(lock); e != nil {
			result = errors.Join(result, errors.New("original catalog file lock cleanup unavailable"))
		}
	}()
	tag, _ := windows.UTF16PtrFromString(strings.ToUpper(hex.EncodeToString(hash)))
	member := managedModelTrustCatalog{catalog: catalogUTF, memberTag: tag, memberPath: path, memberFile: file, hash: &hash[0], hashBytes: size, admin: admin}
	member.size = uint32(unsafe.Sizeof(member))
	d := windows.WinTrustData{UIChoice: windows.WTD_UI_NONE, RevocationChecks: windows.WTD_REVOKE_NONE, UnionChoice: windows.WTD_CHOICE_CATALOG, FileOrCatalogOrBlobOrSgnrOrCert: unsafe.Pointer(&member), StateAction: windows.WTD_STATEACTION_VERIFY, ProvFlags: windows.WTD_CACHE_ONLY_URL_RETRIEVAL}
	d.Size = uint32(unsafe.Sizeof(d))
	e = windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, &d)
	defer func() {
		d.StateAction = windows.WTD_STATEACTION_CLOSE
		if e := windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, &d); e != nil {
			result = errors.Join(result, errors.New("catalog trust cleanup unavailable"))
		}
	}()
	if e != nil {
		return errors.New("installed catalog signature or exact member cannot be verified offline")
	}
	return managedModelTrustVendor(&d, vendor)
}
