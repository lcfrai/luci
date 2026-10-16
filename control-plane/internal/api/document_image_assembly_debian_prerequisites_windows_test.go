//go:build windows

package api

import (
	"context"
	"sync"
	"testing"
	"time"
)

func fakeDebianHostAttempt(guest bool)*debianHostObservationAttempt{
	now:=time.Now();owner:=&debianProofNativeOwner{workDeadline:now.Add(40*time.Second),wholeDeadline:now.Add(60*time.Second)}
	return &debianHostObservationAttempt{owner:owner,requirements:syntheticDebianHostRequirements(guest),workDeadline:owner.workDeadline,wholeDeadline:owner.wholeDeadline}
}

func fakeDebianHostCalls(events *[]string)debianHostPlatformCalls{
	return debianHostPlatformCalls{
		architecture:func(p,n *uint16)error{*events=append(*events,"architecture");*p=0;*n=debianHostMachineAMD64;return nil},
		version:func(v *debianHostVersion)error{*events=append(*events,"version");v.major,v.build,v.platform=10,19041,2;return nil},
		i386Support:func(s *int32)error{*events=append(*events,"i386-support");*s=1;return nil},
		nativeDirectory:func(b *[debianHostDirectoryUnits]uint16)(uint32,error){*events=append(*events,"native-directory");return syntheticDebianHostDirectory(b,`C:\Windows\System32`),nil},
		i386Directory:func(b *[debianHostDirectoryUnits]uint16)(uint32,error){*events=append(*events,"i386-directory");return syntheticDebianHostDirectory(b,`C:\Windows\SysWOW64`),nil},
	}
}

func TestDebianHostProductionEntryRefusesAbsentCensus(t *testing.T){
	a:=fakeDebianHostAttempt(false)
	if _,e:=observeDebianHostPrerequisites(context.Background(),a.owner,a.requirements);e!=errDebianHostCensus{t.Fatalf("missing census did not refuse: %v",e)}
}

func TestDebianHostFixedFakeQueryOrderAndConditionalGuest(t *testing.T){
	for _,guest:=range []bool{false,true}{
		a:=fakeDebianHostAttempt(guest);events:=[]string{};f,e:=a.collectFixedQueries(context.Background(),fakeDebianHostCalls(&events));if e!=nil{t.Fatal(e)}
		want:=[]string{"architecture","version","native-directory"};if guest{want=append(want,"i386-support","i386-directory")}
		if len(events)!=len(want){t.Fatalf("query count %v",events)};for i:=range want{if events[i]!=want[i]{t.Fatalf("query order %v",events)}}
		if f.i386Queried!=guest||f.i386Supported!=guest||a.pending||!a.returned{t.Fatal("wrong query/return state")}
		if _,e=a.collectFixedQueries(context.Background(),fakeDebianHostCalls(&events));e==nil||len(events)!=len(want){t.Fatal("attempt reused")}
	}
}

func TestDebianHostFixedFakeFailureStopsFollowingEffects(t *testing.T){
	for _,name:=range []string{"architecture-error","arm64","version-error","old-build","directory-error","guest-error","guest-false","guest-directory-error"}{
		t.Run(name,func(t *testing.T){a:=fakeDebianHostAttempt(true);events:=[]string{};c:=fakeDebianHostCalls(&events);want:=1
			switch name{
			case "architecture-error":c.architecture=func(*uint16,*uint16)error{events=append(events,"architecture");return errDebianHostFacts}
			case "arm64":c.architecture=func(p,n *uint16)error{events=append(events,"architecture");*p=0;*n=0xaa64;return nil}
			case "version-error":want=2;c.version=func(*debianHostVersion)error{events=append(events,"version");return errDebianHostFacts}
			case "old-build":want=2;c.version=func(v *debianHostVersion)error{events=append(events,"version");v.major,v.build,v.platform=10,19040,2;return nil}
			case "directory-error":want=3;c.nativeDirectory=func(*[debianHostDirectoryUnits]uint16)(uint32,error){events=append(events,"native-directory");return 0,errDebianHostFacts}
			case "guest-error":want=4;c.i386Support=func(*int32)error{events=append(events,"i386-support");return errDebianHostFacts}
			case "guest-false":want=4;c.i386Support=func(*int32)error{events=append(events,"i386-support");return nil}
			case "guest-directory-error":want=5;c.i386Directory=func(*[debianHostDirectoryUnits]uint16)(uint32,error){events=append(events,"i386-directory");return 0,errDebianHostFacts}
			}
			if _,e:=a.collectFixedQueries(context.Background(),c);e==nil||len(events)!=want||a.pending||!a.returned{t.Fatalf("failure did not stop exact sequence: %v %v",events,e)}
		})
	}
}

func TestDebianHostFakeRetirementRetainsBlockedBackingAndRejectsLateFacts(t *testing.T){
	a:=fakeDebianHostAttempt(false);events:=[]string{};c:=fakeDebianHostCalls(&events)
	entered,release,done:=make(chan struct{}),make(chan struct{}),make(chan error,1)
	var releaseOnce sync.Once;unblock:=func(){releaseOnce.Do(func(){close(release)})};defer unblock()
	c.architecture=func(p,n *uint16)error{events=append(events,"architecture");close(entered);<-release;*p=0;*n=debianHostMachineAMD64;return nil}
	go func(){_,e:=a.collectFixedQueries(context.Background(),c);done<-e}()
	select{case<-entered:case<-time.After(time.Second):t.Fatal("fake call not entered")}
	a.requestRetirement()
	a.mu.Lock();pending,returned:=a.pending,a.returned;a.mu.Unlock()
	if !pending||returned{unblock();<-done;t.Fatal("blocked backing falsely settled")}
	unblock()
	select{case e:=<-done:if e==nil{t.Fatal("late facts promoted")};case<-time.After(time.Second):t.Fatal("fake call did not return")}
	if len(events)!=1||a.pending||!a.returned||a.facts.nativeDirectory!=""{t.Fatal("retirement lost state or allowed later query")}
}

func TestDebianHostFakeLateOwnerStopAndCancellationPreventPublication(t *testing.T){
	for _,name:=range []string{"owner-stop","context-cancel","attempt-retire"}{
		t.Run(name,func(t *testing.T){a:=fakeDebianHostAttempt(false);events:=[]string{};ctx,cancel:=context.WithCancel(context.Background());defer cancel();c:=fakeDebianHostCalls(&events)
			c.nativeDirectory=func(b *[debianHostDirectoryUnits]uint16)(uint32,error){events=append(events,"native-directory");n:=syntheticDebianHostDirectory(b,`C:\Windows\System32`);switch name{case "owner-stop":a.owner.stopped.Store(true);case "context-cancel":cancel();case "attempt-retire":a.requestRetirement()};return n,nil}
			if _,e:=a.collectFixedQueries(ctx,c);e==nil||len(events)!=3||a.facts.nativeDirectory!=""{t.Fatal("late stop promoted platform facts")}
		})
	}
}

func TestDebianHostFakeMissingWrittenTerminatorRefuses(t *testing.T){
	a:=fakeDebianHostAttempt(false);events:=[]string{};c:=fakeDebianHostCalls(&events)
	c.nativeDirectory=func(b *[debianHostDirectoryUnits]uint16)(uint32,error){
		s:=`C:\Windows\System32`;for i:=range s{b[i]=uint16(s[i])};return uint32(len(s)),nil
	}
	if _,e:=a.collectFixedQueries(context.Background(),c);e==nil{t.Fatal("unwritten NUL accepted from initial zero tail")}
}

func TestDebianHostFakeOwnerClockAndCancellationRefuseBeforeQuery(t *testing.T){
	for _,name:=range []string{"nil-owner","stopped-owner","expired","foreign-clock","cancelled","unset-requirements"}{
		t.Run(name,func(t *testing.T){a:=fakeDebianHostAttempt(false);events:=[]string{};ctx,cancel:=context.WithCancel(context.Background());defer cancel()
			switch name{
			case "nil-owner":a.owner=nil
			case "stopped-owner":a.owner.stopped.Store(true)
			case "expired":a.workDeadline=time.Now().Add(-time.Second);a.owner.workDeadline=a.workDeadline
			case "foreign-clock":a.workDeadline=a.workDeadline.Add(time.Second)
			case "cancelled":cancel()
			case "unset-requirements":a.requirements.minimumBuild=0
			}
			if _,e:=a.collectFixedQueries(ctx,fakeDebianHostCalls(&events));e==nil||len(events)!=0{t.Fatal("unadmitted fake effect")}
		})
	}
}
