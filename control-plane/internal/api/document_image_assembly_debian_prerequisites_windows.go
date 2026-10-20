//go:build windows

package api

import (
	"context"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

type debianHostVersion struct {
	size, major, minor, build, platform uint32
	csd [128]uint16
	servicePackMajor, servicePackMinor, suite uint16
	productType, reserved uint8
}

// Per-attempt fixed primitives are replaceable only by package-local fake tests.
// No function accepts an export name, DLL path, PID, command, or caller buffer.
type debianHostPlatformCalls struct {
	architecture func(*uint16, *uint16) error
	version func(*debianHostVersion) error
	i386Support func(*int32) error
	nativeDirectory func(*[debianHostDirectoryUnits]uint16) (uint32, error)
	i386Directory func(*[debianHostDirectoryUnits]uint16) (uint32, error)
}

// Construction does not Find/Load/Call any procedure. The first actual query
// can load fixed system DLLs and retain process-lifetime loader references.
// Those references are NOT positively unloaded originals or image trust proof.
func fixedDebianHostPlatformCalls() debianHostPlatformCalls {
	kernel := windows.NewLazySystemDLL("kernel32.dll")
	ntdll := windows.NewLazySystemDLL("ntdll.dll")
	version := ntdll.NewProc("RtlGetVersion")
	guest := kernel.NewProc("IsWow64GuestMachineSupported")
	nativeDir := kernel.NewProc("GetSystemDirectoryW")
	guestDir := kernel.NewProc("GetSystemWow64Directory2W")
	return debianHostPlatformCalls{
		architecture:func(p,n *uint16) error { return windows.IsWow64Process2(windows.CurrentProcess(),p,n) },
		version:func(v *debianHostVersion) error {
			if err:=version.Find();err!=nil{return errDebianHostFacts}
			status,_,_:=version.Call(uintptr(unsafe.Pointer(v)))
			if uint32(status)!=0{return errDebianHostFacts};return nil
		},
		i386Support:func(supported *int32) error {
			if err:=guest.Find();err!=nil{return errDebianHostFacts}
			status,_,_:=guest.Call(uintptr(debianHostMachineI386),uintptr(unsafe.Pointer(supported)))
			if uint32(status)!=0{return errDebianHostFacts};return nil
		},
		nativeDirectory:func(b *[debianHostDirectoryUnits]uint16)(uint32,error){
			if err:=nativeDir.Find();err!=nil{return 0,errDebianHostFacts}
			n,_,_:=nativeDir.Call(uintptr(unsafe.Pointer(&b[0])),uintptr(len(b)))
			if n==0||n>=uintptr(len(b)){return 0,errDebianHostFacts};return uint32(n),nil
		},
		i386Directory:func(b *[debianHostDirectoryUnits]uint16)(uint32,error){
			if err:=guestDir.Find();err!=nil{return 0,errDebianHostFacts}
			n,_,_:=guestDir.Call(uintptr(unsafe.Pointer(&b[0])),uintptr(len(b)),uintptr(debianHostMachineI386))
			if n==0||n>=uintptr(len(b)){return 0,errDebianHostFacts};return uint32(n),nil
		},
	}
}

// The existing native owner/census must eventually issue and retain this exact
// attempt before any query. There is deliberately NO production issuer here.
// Source authoring of an attempt/requirements is not original census admission.
// No background query goroutine or context-timeout disposal of native backing.
type debianHostObservationAttempt struct {
	mu sync.Mutex
	owner *debianProofNativeOwner
	requirements debianHostRequirements
	workDeadline, wholeDeadline time.Time
	started, pending, returned, retired bool
	processMachine, nativeMachine uint16
	version debianHostVersion
	i386Supported int32
	nativeBuffer, i386Buffer [debianHostDirectoryUnits]uint16
	facts debianHostFacts
}

// The same retained producer owns this one fixed query attempt. Register its
// backing before the first synchronous OS call; cancellation only latches it.
// These queries create no kernel HANDLEs and cannot release census slots.
func observeDebianHostPrerequisites(ctx context.Context, o *debianProofNativeOwner, r debianHostRequirements) (debianHostFacts,error) {
	a,err:=o.reserveHostObservation(ctx,r);if err!=nil{return debianHostFacts{},err}
	return a.collectFixedQueries(ctx,fixedDebianHostPlatformCalls())
}

func(o *debianProofNativeOwner)reserveHostObservation(ctx context.Context,r debianHostRequirements)(*debianHostObservationAttempt,error){
	if o==nil||ctx==nil||ctx.Err()!=nil||!r.valid()||o.census==nil{return nil,errDebianHostCensus}
	c:=o.census;c.mu.Lock();defer c.mu.Unlock()
	if c.owner!=o||c.retiring||o.nativeRetired()||!time.Now().Before(o.workLimit())||o.hostObservation!=nil{return nil,errDebianHostCensus}
	a:=&debianHostObservationAttempt{owner:o,requirements:r,workDeadline:o.workLimit(),wholeDeadline:o.wholeDeadline}
	o.hostObservation=a
	return a,nil
}

func(o *debianProofNativeOwner)retireHostObservation(){
	if o==nil||o.census==nil{return};o.census.mu.Lock();a:=o.hostObservation;o.census.mu.Unlock()
	if a!=nil{a.requestRetirement()}
}

func(o *debianProofNativeOwner)hostObservationSettled()bool{
	if o==nil||o.census==nil{return false};o.census.mu.Lock();a:=o.hostObservation;o.census.mu.Unlock()
	if a==nil{return true};a.mu.Lock();defer a.mu.Unlock();return a.returned&&!a.pending
}

func (a *debianHostObservationAttempt) current(ctx context.Context) bool {
	return a!=nil && a.owner!=nil && ctx!=nil && ctx.Err()==nil && !a.owner.stopped.Load() && !a.workDeadline.IsZero() && !a.wholeDeadline.IsZero() && a.workDeadline.Equal(a.owner.workDeadline) && a.wholeDeadline.Equal(a.owner.wholeDeadline) && a.workDeadline.Before(a.wholeDeadline) && time.Now().Before(a.workDeadline) && time.Now().Before(a.wholeDeadline)
}

func (a *debianHostObservationAttempt) requestRetirement() {
	if a==nil{return};a.mu.Lock();a.retired=true;a.mu.Unlock()
}

// Locks protect only metadata; never the synchronous native call. On blocked
// return the issuing owner must retain a and its buffers/census charge. Calling
// requestRetirement cannot interrupt that call. Later returns cannot publish.
func (a *debianHostObservationAttempt) beforeQuery(ctx context.Context) error {
	if !a.current(ctx){return errDebianHostFacts}
	a.mu.Lock();defer a.mu.Unlock()
	if a.retired||a.returned||a.pending{return errDebianHostFacts};a.pending=true;return nil
}

func (a *debianHostObservationAttempt) afterQuery(ctx context.Context, err error) error {
	a.mu.Lock();a.pending=false;retired:=a.retired;a.mu.Unlock()
	if err!=nil||retired||!a.current(ctx){return errDebianHostFacts};return nil
}

// Fixed query engine, currently reachable only from same-namespace fake tests.
// It is not an admission factory. Production entry above stays closed until the
// reviewer-owned receiver binds Product identity and retains this exact attempt.
func (a *debianHostObservationAttempt) collectFixedQueries(ctx context.Context,c debianHostPlatformCalls)(debianHostFacts,error){
	if !a.current(ctx)||!a.requirements.valid()||runtime.GOARCH!="amd64"||c.architecture==nil||c.version==nil||c.nativeDirectory==nil||a.requirements.i386Required&&(c.i386Support==nil||c.i386Directory==nil){return debianHostFacts{},errDebianHostFacts}
	a.mu.Lock();if a.started||a.retired{a.mu.Unlock();return debianHostFacts{},errDebianHostFacts};a.started=true;a.mu.Unlock()
	defer func(){a.mu.Lock();a.returned=true;a.mu.Unlock()}()
	if e:=a.beforeQuery(ctx);e!=nil{return debianHostFacts{},e}
	if e:=a.afterQuery(ctx,c.architecture(&a.processMachine,&a.nativeMachine));e!=nil{return debianHostFacts{},e}
	if a.processMachine!=debianHostMachineUnknown||a.nativeMachine!=debianHostMachineAMD64{return debianHostFacts{},errDebianHostFacts}
	a.version.size=uint32(unsafe.Sizeof(a.version))
	if e:=a.beforeQuery(ctx);e!=nil{return debianHostFacts{},e}
	if e:=a.afterQuery(ctx,c.version(&a.version));e!=nil{return debianHostFacts{},e}
	if a.version.size!=284||a.version.platform!=2||a.version.major<10||a.version.build<a.requirements.minimumBuild{return debianHostFacts{},errDebianHostFacts}
	// A zero-initialized tail cannot prove the API wrote its terminator.
	for i:=range a.nativeBuffer{a.nativeBuffer[i]=0xffff}
	if e:=a.beforeQuery(ctx);e!=nil{return debianHostFacts{},e}
	n,e:=c.nativeDirectory(&a.nativeBuffer);if e=a.afterQuery(ctx,e);e!=nil{return debianHostFacts{},e}
	native,e:=decodeDebianHostDirectory(&a.nativeBuffer,n);if e!=nil{return debianHostFacts{},e}
	f:=debianHostFacts{processMachine:a.processMachine,nativeMachine:a.nativeMachine,major:a.version.major,minor:a.version.minor,build:a.version.build,platform:a.version.platform,nativeDirectory:native,toolDomainDigest:a.requirements.toolDomainDigest}
	if a.requirements.i386Required{
		if e:=a.beforeQuery(ctx);e!=nil{return debianHostFacts{},e}
		if e:=a.afterQuery(ctx,c.i386Support(&a.i386Supported));e!=nil{return debianHostFacts{},e}
		if a.i386Supported==0{return debianHostFacts{},errDebianHostFacts}
		for i:=range a.i386Buffer{a.i386Buffer[i]=0xffff}
		if e:=a.beforeQuery(ctx);e!=nil{return debianHostFacts{},e}
		n,e=c.i386Directory(&a.i386Buffer);if e=a.afterQuery(ctx,e);e!=nil{return debianHostFacts{},e}
		f.i386Directory,e=decodeDebianHostDirectory(&a.i386Buffer,n);if e!=nil{return debianHostFacts{},e}
		f.i386Queried,f.i386Supported=true,true
	}
	if e:=validateDebianHostFacts(a.requirements,f);e!=nil{return debianHostFacts{},e}
	// Retirement/currentness is rechecked under the short publication lock; no
	// native query, output hash or other owner mutex is held here.
	a.mu.Lock();defer a.mu.Unlock()
	if a.retired||!a.current(ctx){return debianHostFacts{},errDebianHostFacts};a.facts=f;return f,nil
}
