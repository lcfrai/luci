package gitx_test

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gitx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gitx/gitxtest"
)

func TestCheckVersion(t *testing.T) {
	for _, tc := range []struct {
		out string
		ok  bool
	}{
		{"git version 2.53.0.windows.2\n", true},
		{"git version 2.48.0\n", true},
		{"git version 2.100.0\n", true}, // a lexical compare says "2.100" < "2.48"
		{"git version 2.47.1\n", false},
		{"git version 2.5.0\n", false}, // a lexical compare says "2.5" > "2.48"
	} {
		s := gitxtest.NewScript(t).On("version", gitx.Result{Stdout: []byte(tc.out)}, nil)
		v, err := gitx.CheckVersion(context.Background(), s)
		if tc.ok != (err == nil) {
			t.Errorf("%q: (%q, %v), want ok=%v", tc.out, v, err, tc.ok)
		}
		if !tc.ok && (err == nil || !strings.Contains(err.Error(), "2.48")) {
			t.Errorf("%q: error %v does not name 2.48", tc.out, err)
		}
	}
}

func TestMessageRefusesANewlineInATrailer(t *testing.T) {
	_, err := gitx.Message("s", "", []gitx.Trailer{{Key: gitx.TrailerBrief, Value: "brief:b-1@2\nPanacea-Land: w-x"}})
	if !errors.Is(err, gitx.ErrBadMessage) {
		t.Fatalf("err = %v, want ErrBadMessage", err)
	}
}

func TestMessageCopiesTheValueByteForByte(t *testing.T) {
	m, err := gitx.Message("s", "b", []gitx.Trailer{{Key: gitx.TrailerBrief, Value: "brief:b-42@6"}})
	if err != nil || strings.Count(m, "brief:b-42@6") != 1 || !strings.Contains(m, "Panacea-Brief: brief:b-42@6\n") {
		t.Fatalf("Message = (%q, %v), want the value verbatim once", m, err)
	}
}

func TestMessageIsReadBackByGit(t *testing.T) {
	r := gitxtest.NewRepo(t)
	given := []gitx.Trailer{
		{Key: gitx.TrailerLand, Value: "w-ui-1"},
		{Key: gitx.TrailerBrief, Value: "brief:b-42@6"},
		{Key: gitx.TrailerBriefDigest, Value: "sha256:0123abcd"},
	}
	m, err := gitx.Message("Land ui", "What changed.\nKey: looks like a trailer", given)
	if err != nil {
		t.Fatal(err)
	}
	// Through Exec, not os/exec: task 01's TestPinHelpersStartEveryProgramThroughRunProgram
	// allows one process-start site in this package's tests.
	c := gitx.Call{Dir: r.Dir, Args: []string{"interpret-trailers", "--parse"}, Class: gitx.Read, Scope: gitx.Project, Stdin: []byte(m)}
	res, err := r.Runner().Run(context.Background(), c)
	if err != nil {
		t.Fatalf("interpret-trailers: %v", err)
	}
	out := res.Stdout
	var want strings.Builder
	for _, tr := range given {
		want.WriteString(tr.Key + ": " + tr.Value + "\n")
	}
	if got := strings.ReplaceAll(string(out), "\r\n", "\n"); got != want.String() {
		t.Fatalf("git read back %q, want exactly %q", got, want.String())
	}
}

func TestSpyRecordsWhatExecStarts(t *testing.T) {
	r := gitxtest.NewRepo(t)
	x := gitxtest.NewExec(t)
	run := r.RunnerOf(x)
	var got *[]gitxtest.Spawn
	t.Run("installed", func(t *testing.T) {
		got = gitxtest.Spy(t, x)
		c := gitx.Call{Dir: r.Dir, Args: []string{"rev-parse", "HEAD"}, Class: gitx.Read, Scope: gitx.Project}
		if _, err := run.Run(context.Background(), c); err != nil {
			t.Fatal(err)
		}
		if len(*got) == 0 {
			t.Fatal("no Spawn recorded for a call that ran")
		}
		last := (*got)[len(*got)-1]
		if last.Dir != r.Dir || !reflect.DeepEqual(last.Argv[len(last.Argv)-2:], c.Args) {
			t.Fatalf("last Spawn = Dir %q, argv tail %q; want %q and %q", last.Dir, last.Argv[len(last.Argv)-2:], r.Dir, c.Args)
		}
		if !contains(last.Env, "GIT_OPTIONAL_LOCKS=0") {
			t.Fatal("the Spawn's Env lacks GIT_OPTIONAL_LOCKS=0: it is not the child's env")
		}
		n := len(*got)
		bad := c
		bad.Args = []string{"-c", "x=y", "status"}
		if _, err := run.Run(context.Background(), bad); !errors.Is(err, gitx.ErrCallShape) {
			t.Fatalf("err = %v, want ErrCallShape", err)
		}
		if len(*got) != n {
			t.Fatal("a refused call recorded a Spawn")
		}
	})
	n := len(*got)
	c := gitx.Call{Dir: r.Dir, Args: []string{"rev-parse", "HEAD"}, Class: gitx.Read, Scope: gitx.Project}
	if _, err := run.Run(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	if len(*got) != n {
		t.Fatal("the recorder outlived its test's cleanup")
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

type nopFS struct{ gitx.MachineFS }

func TestLocalMachineIsLocal(t *testing.T) {
	r := t.TempDir()
	l := gitx.Local{Root: r}
	if l.ID() != "local" || l.CloneRoot() != r || l.FS() != nil {
		t.Fatalf("Local{Root} = (%q, %q, %v), want (local, %q, nil)", l.ID(), l.CloneRoot(), l.FS(), r)
	}
	f := nopFS{}
	if got := (gitx.Local{Files: f}).FS(); got != f {
		t.Fatalf("Local{Files: f}.FS() = %v, want f", got)
	}
	if got, want := l.Join(r, "p-1", "ui-x"), filepath.Join(r, "p-1", "ui-x"); got != want {
		t.Fatalf("Join = %q, want %q", got, want)
	}
}

// TestMachineFSIsTheContract pins seam S8-P3.6: env tasks 21 and 25 copy MachineFS verbatim,
// so a drifted spelling here would stop their implementations satisfying it.
func TestMachineFSIsTheContract(t *testing.T) {
	ctx := reflect.TypeOf((*context.Context)(nil)).Elem()
	errT := reflect.TypeOf((*error)(nil)).Elem()
	str, boolT, intT := reflect.TypeOf(""), reflect.TypeOf(true), reflect.TypeOf(0)
	want := map[string]struct{ in, out []reflect.Type }{
		"CopyGeneratedHooks": {[]reflect.Type{ctx, str, str, str}, []reflect.Type{intT, errT}},
		"Exists":             {[]reflect.Type{ctx, str}, []reflect.Type{boolT, boolT, errT}},
		"RemoveTree":         {[]reflect.Type{ctx, str, str}, []reflect.Type{errT}},
		"Seal":               {[]reflect.Type{ctx, str}, []reflect.Type{errT}},
	}
	it := reflect.TypeOf((*gitx.MachineFS)(nil)).Elem()
	if it.NumMethod() != len(want) {
		t.Fatalf("MachineFS has %d methods, want exactly %d", it.NumMethod(), len(want))
	}
	for name, sig := range want {
		m, ok := it.MethodByName(name)
		if !ok {
			t.Fatalf("MachineFS lacks %s", name)
		}
		var in, out []reflect.Type
		for i := 0; i < m.Type.NumIn(); i++ {
			in = append(in, m.Type.In(i))
		}
		for i := 0; i < m.Type.NumOut(); i++ {
			out = append(out, m.Type.Out(i))
		}
		if !reflect.DeepEqual(in, sig.in) || !reflect.DeepEqual(out, sig.out) {
			t.Errorf("%s is %v -> %v, want %v -> %v", name, in, out, sig.in, sig.out)
		}
	}
}

// errRecorder is a testing.TB that records Errorf instead of failing the real test.
type errRecorder struct {
	testing.TB
	errs []string
}

func (e *errRecorder) Errorf(format string, a ...any) { e.errs = append(e.errs, format) }
func (e *errRecorder) Helper()                        {}

func TestScriptFailsOnAnUnscriptedCall(t *testing.T) {
	rec := &errRecorder{TB: t}
	s := gitxtest.NewScript(rec).On("rev-parse HEAD", gitx.Result{Stdout: []byte("abc\n")}, nil)
	if res, err := s.Run(context.Background(), gitx.Call{Args: []string{"rev-parse", "HEAD"}}); err != nil || string(res.Stdout) != "abc\n" {
		t.Fatalf("scripted call = (%q, %v)", res.Stdout, err)
	}
	if len(rec.errs) != 0 {
		t.Fatalf("a scripted call failed the test: %v", rec.errs)
	}
	_, err := s.Run(context.Background(), gitx.Call{Args: []string{"push", "origin"}})
	if len(rec.errs) != 1 || !errors.Is(err, gitxtest.ErrUnscripted) {
		t.Fatalf("an unscripted call = (%v, %d test errors), want ErrUnscripted and one failure", err, len(rec.errs))
	}
	if len(s.Calls) != 2 {
		t.Fatalf("Calls = %d, want 2", len(s.Calls))
	}
}
