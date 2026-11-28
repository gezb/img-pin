package helpers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopyToTempDirFile(t *testing.T) {
	src := filepath.Join(t.TempDir(), "test.txt")
	require.NoError(t, os.WriteFile(src, []byte("hello"), 0o644))

	td := CopyToTempDir(t, src)
	require.Len(t, td.Files, 1)
	assert.Equal(t, "test.txt", filepath.Base(td.First()))
	content, err := os.ReadFile(td.First())
	require.NoError(t, err)
	assert.Equal(t, "hello", string(content))
}

func TestCopyToTempDirDirectory(t *testing.T) {
	src := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(src, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(src, "a.txt"), []byte("a"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(src, "sub", "b.txt"), []byte("b"), 0o644))

	td := CopyToTempDir(t, src)
	require.Len(t, td.Files, 2)
	assert.Equal(t, filepath.Join(td.Dir, "a.txt"), td.Files[0])
	assert.Equal(t, filepath.Join(td.Dir, "sub", "b.txt"), td.Files[1])
	content, err := os.ReadFile(td.Files[1])
	require.NoError(t, err)
	assert.Equal(t, "b", string(content))
}

func TestCopyToTempDirectory(t *testing.T) {
	src := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(src, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(src, "a.txt"), []byte("a"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(src, "sub", "b.txt"), []byte("b"), 0o644))

	dir := CopyToTemp(t, src)
	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
	content, err := os.ReadFile(filepath.Join(dir, "sub", "b.txt"))
	require.NoError(t, err)
	assert.Equal(t, "b", string(content))
}
