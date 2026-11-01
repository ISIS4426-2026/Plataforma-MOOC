package handler

import (
	"os"
	"path/filepath"
	"testing"
)

func escribir(t *testing.T, ruta string, bytes int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ruta, make([]byte, bytes), 0o644); err != nil {
		t.Fatal(err)
	}
}

func existe(ruta string) bool { _, err := os.Lstat(ruta); return err == nil }

// The directories an interrupted job leaves are removed, with their weight counted; nothing else in the
// same directory is touched.
func TestSweepStaleScratchRemovesOnlyTheJobDirectories(t *testing.T) {
	raiz := t.TempDir()
	escribir(t, filepath.Join(raiz, "mooc-media-1111", "original.mp4"), 1000)
	escribir(t, filepath.Join(raiz, "mooc-media-2222", "hls", "720p", "seg0.ts"), 500)
	escribir(t, filepath.Join(raiz, "otra-cosa", "dato.txt"), 10)
	escribir(t, filepath.Join(raiz, "mooc-media-es-un-archivo"), 5) // un archivo, no un directorio de trabajo

	removed, freed, err := SweepStaleScratch(raiz, nil)
	if err != nil {
		t.Fatalf("SweepStaleScratch: %v", err)
	}
	if removed != 2 || freed != 1500 {
		t.Errorf("removed=%d freed=%d, want 2 and 1500", removed, freed)
	}
	for _, borrado := range []string{"mooc-media-1111", "mooc-media-2222"} {
		if existe(filepath.Join(raiz, borrado)) {
			t.Errorf("%s should have been removed", borrado)
		}
	}
	for _, intacto := range []string{"otra-cosa", "mooc-media-es-un-archivo"} {
		if !existe(filepath.Join(raiz, intacto)) {
			t.Errorf("%s must not be touched", intacto)
		}
	}
}

// With no work directory configured the process would sweep the system temp directory, which other
// programs share. It must do nothing at all.
func TestSweepStaleScratchNeverSweepsTheSystemTempDir(t *testing.T) {
	removed, freed, err := SweepStaleScratch("", nil)
	if err != nil || removed != 0 || freed != 0 {
		t.Fatalf("removed=%d freed=%d err=%v, want all zero", removed, freed, err)
	}
}

func TestSweepStaleScratchToleratesAMissingDirectory(t *testing.T) {
	removed, _, err := SweepStaleScratch(filepath.Join(t.TempDir(), "no-existe"), nil)
	if err != nil || removed != 0 {
		t.Fatalf("removed=%d err=%v, want 0 and no error", removed, err)
	}
}

// A link named like a job directory is not followed: what it points to is not ours.
func TestSweepStaleScratchDoesNotFollowSymlinks(t *testing.T) {
	raiz, fuera := t.TempDir(), t.TempDir()
	escribir(t, filepath.Join(fuera, "importante.txt"), 7)
	if err := os.Symlink(fuera, filepath.Join(raiz, "mooc-media-enlace")); err != nil {
		t.Skipf("no se pueden crear enlaces simbolicos aqui: %v", err)
	}
	removed, _, err := SweepStaleScratch(raiz, nil)
	if err != nil || removed != 0 {
		t.Fatalf("removed=%d err=%v, want 0", removed, err)
	}
	if !existe(filepath.Join(fuera, "importante.txt")) {
		t.Fatal("the target of a symlink was removed")
	}
}
