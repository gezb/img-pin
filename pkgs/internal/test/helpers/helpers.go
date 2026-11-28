package helpers

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/robdavid/genutil-go/errors/handler"
)

func CreateTemp(t *testing.T, ext string) string {
	defer Handle(func(e error) {
		t.Skipf("Cannot create temp file for ext %q: %s", ext, e)
	})
	name := strings.ReplaceAll(t.Name(), "/", "__")
	dst := Try(os.CreateTemp("", name+"-*.tmp"+ext))
	dst.Close()
	t.Cleanup(func() { os.Remove(dst.Name()) })
	return dst.Name()
}

// CopyTo copies filename into dstDir. If filename is a directory, all files in
// its tree are copied, preserving their relative paths. It returns the paths of
// all files copied.
func CopyTo(filename, dstDir string) []string {
	info := Try(os.Stat(filename))
	if !info.IsDir() {
		return []string{copyFile(filename, dstDir, filepath.Base(filename))}
	}
	var files []string
	var walk func(src, rel string)
	walk = func(src, rel string) {
		for _, entry := range Try(os.ReadDir(src)) {
			srcPath := filepath.Join(src, entry.Name())
			relPath := filepath.Join(rel, entry.Name())
			if entry.IsDir() {
				walk(srcPath, relPath)
			} else {
				files = append(files, copyFile(srcPath, dstDir, relPath))
			}
		}
	}
	walk(filename, "")
	return files
}

// copyFile copies src into dstDir under name and returns the destination path.
func copyFile(src, dstDir, name string) string {
	dst := filepath.Join(dstDir, name)
	Try0(os.MkdirAll(filepath.Dir(dst), 0o755))
	in := Try(os.Open(src))
	defer in.Close()
	out := Try(os.Create(dst))
	defer out.Close()
	Try(io.Copy(out, in))
	return dst
}

func CopyToTemp(t *testing.T, filename string) string {
	defer Handle(func(e error) {
		t.Skipf("Cannot create temp file for %s: %q", filename, e)
	})
	name := filepath.Base(filename)
	if Try(os.Stat(filename)).IsDir() {
		dir := Try(os.MkdirTemp("", name+"-*"))
		t.Cleanup(func() { os.RemoveAll(dir) })
		CopyTo(filename, dir)
		return dir
	}
	src := Try(os.Open(filename))
	defer src.Close()
	ext := filepath.Ext(name)
	dst := Try(os.CreateTemp("", name[:len(name)-len(ext)]+"-*.tmp"+ext))
	defer dst.Close()
	t.Cleanup(func() { os.Remove(dst.Name()) })
	Try(io.Copy(dst, src))
	return dst.Name()
}

type TempDir struct {
	Dir   string
	Files []string
}

func (td *TempDir) Delete() error { return os.RemoveAll(td.Dir) }
func (fd *TempDir) First() string { return fd.Files[0] }

// CopyToTempDir creates a temporary directory and copies the provided files to that directory
// a [TempDir] object is returned that contains details about the directory and files. It adds
// a cleanup handler to t to remove the temporary directory after the test.
func CopyToTempDir(t *testing.T, filenames ...string) (tmpDir TempDir) {
	t.Helper()
	defer Handle(func(e error) {
		t.Skipf("Cannot create temp file for %v: %s", filenames, e)
	})
	if len(filenames) == 0 {
		t.Fatal("No file names supplied to CopyToTemp")
	}
	tmpDir.Dir = Try(os.MkdirTemp("", "tmp-*"))
	t.Cleanup(func() { tmpDir.Delete() })
	for _, filename := range filenames {
		tmpDir.Files = append(tmpDir.Files, CopyTo(filename, tmpDir.Dir)...)
	}
	return
}
