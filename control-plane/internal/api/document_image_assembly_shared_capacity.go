package api

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

var errDocumentImageAssemblySerial = errors.New("An active or unconfirmed document image assembly retains the serial execution slot; settle that exact allocation before starting other work.")

func assemblyCaptureDemand(rows []documentImageAssemblyLease, reserveMiB int) (uint64, error) {
	if len(rows) > 4096 || reserveMiB < 0 || uint64(reserveMiB) > ^uint64(0)>>20 {
		return 0, errors.New("assembly capture reservation bound")
	}
	demand := uint64(reserveMiB) << 20
	for _, row := range rows {
		if !validDocumentImageAssemblyLease(row) {
			return 0, errors.New("assembly capture ownership invalid")
		}
		if row.State == "finished" {
			continue
		}
		if row.CaptureDiskBytes > ^uint64(0)-demand {
			return 0, errors.New("assembly capture demand overflow")
		}
		demand += row.CaptureDiskBytes
		if row.Stage!=nil{if row.Stage.DirectoryBytes>^uint64(0)-demand{return 0,errors.New("assembly staging demand overflow")};demand+=row.Stage.DirectoryBytes}
	}
	return demand, nil
}

func validateAssemblyCaptureDisk(disk resourcebudget.Disk, required uint64, now time.Time) error {
	if disk.AvailableBytes == nil || disk.ObservedAt.IsZero() || disk.ObservedAt.After(now) || now.Sub(disk.ObservedAt) > 5*time.Second || *disk.AvailableBytes < required {
		return errors.New("Exact private assembly capture volume capacity is unavailable or cannot preserve all retained capture reservations and reserve.")
	}
	return nil
}

// Every shared admission calls this once while holding the existing ledger
// lock. Container demand can deduplicate, but capture bytes never do. An exact
// native disk observation also covers ExecutionStateDir on another host volume.
func (g *executionAdmission) addAssemblyAdmissionCapacity(ctx context.Context, snap resourcebudget.Snapshot, rows []documentImageAssemblyLease, skipScope, skipID string, demand *executionCapacityDemand, reserveMiB int) error {
	if len(rows) == 0 {
		return nil
	}
	if err := addDocumentImageAssemblyCapacity(snap, rows, skipScope, skipID, demand); err != nil {
		return err
	}
	required, err := assemblyCaptureDemand(rows, reserveMiB)
	if err != nil {
		return err
	}
	active := false
	for _, row := range rows {
		if row.State != "finished" {
			active = true
		}
	}
	if !active {
		return nil
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	// prepareExecutionDirectory already confined this directory before mutate;
	// sample only its capacity, never the artifact, Docker, package or model.
	probe := g.assemblyDiskProbe
	if probe == nil {
		probe = resourcebudget.ObserveDisk
	}
	disk, err := probe(resourcebudget.DiskPath{Role: "document-image-assembly-capture", Path: filepath.Dir(g.path)})
	if err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = validateAssemblyCaptureDisk(disk, required, time.Now().UTC()); err != nil {
		return err
	}
	for _, row := range rows {
		if row.State != "finished" && !(row.Scope == skipScope && row.ID == skipID) {
			return errDocumentImageAssemblySerial
		}
	}
	return nil
}
