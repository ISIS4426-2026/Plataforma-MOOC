package handler

import (
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// scratchPrefix is the name every media job's scratch directory starts with
// (`os.MkdirTemp(workDir, "mooc-media-*")` in processAudiovisual). The sweeper
// below matches on it so it can never remove anything else that lives in the
// same directory.
const scratchPrefix = "mooc-media-"

// SweepStaleScratch removes the scratch directories an earlier worker process
// left behind, and reports how many it removed and how many bytes that freed.
//
// # Why this exists (E3, #218)
//
// A job's scratch directory holds the original, which can be up to the
// configured maximum (2 GiB by default), plus every rendition. `defer
// os.RemoveAll` cleans it on every normal exit, including a cancelled context.
// It cannot run when the process is killed -- the drain deadline of C7 expires
// with a long video still encoding, the container is OOM-killed, the VM is
// stopped -- and the directory stays in the named volume `worker_scratch`,
// which survives a container restart. The retried job gets a NEW directory,
// so nothing ever reclaims the old one, and on a 30 GiB disk a handful of
// interrupted jobs fill it and every later job dies with "no space left".
//
// Nothing in a scratch directory is the only copy of anything: the original is
// in the object store and the derivatives are regenerated from it, so removing
// it is always safe. That is also what "rebuildable temporary" means.
//
// # The one assumption
//
// It runs ONCE, at startup, before the worker accepts a job, and it removes
// every match. That is only correct if the scratch directory belongs to a
// single worker process, which is how the deployment is built (one worker
// container per VM, with its own named volume). An empty `root` means the
// system temp directory, which other processes share, so it is never swept.
func SweepStaleScratch(root string, logger *slog.Logger) (removed int, freed int64, err error) {
	if root == "" {
		return 0, 0, nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, err
	}
	var firstErr error
	for _, e := range entries {
		// Directories only, by exact prefix, and never through a symlink: an entry that is a link to
		// somewhere else is not ours to follow.
		if !strings.HasPrefix(e.Name(), scratchPrefix) || e.Type()&fs.ModeSymlink != 0 || !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		size := dirSize(dir)
		if err := os.RemoveAll(dir); err != nil {
			logger.Warn("could not remove a stale scratch directory", slog.String("dir", dir), slog.String("error", err.Error()))
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		removed++
		freed += size
		logger.Info("removed a stale scratch directory left by an interrupted job",
			slog.String("dir", dir), slog.Int64("bytes", size))
	}
	return removed, freed, firstErr
}

func dirSize(dir string) int64 {
	var total int64
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}
