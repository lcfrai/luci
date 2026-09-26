package migrate

import (
	"embed"
	"io/fs"
)

//go:embed sql/*.sql
var embedded embed.FS

// Embedded returns this build's migrations in version order. A bad file name is
// a build-time bug, so it panics rather than returning an error.
func Embedded() []Migration {
	sub, err := fs.Sub(embedded, "sql")
	if err != nil {
		panic("migrate: the embedded sql directory: " + err.Error())
	}
	ms, err := Load(sub)
	if err != nil {
		panic(err.Error())
	}
	return ms
}

// Latest is the highest embedded version: the schema version `hub serve` is
// built for (contract §10.1). 1 after this task.
func Latest() int {
	ms := Embedded()
	if len(ms) == 0 {
		return 0
	}
	return ms[len(ms)-1].Version
}
