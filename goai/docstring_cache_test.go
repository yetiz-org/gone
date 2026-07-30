package goai

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type importedPackageCacheFixture struct {
	t           *testing.T
	root        string
	packageDir  string
	payloadPath string
	callsPath   string
	importCache *sync.Map
}

func newImportedPackageCacheFixture(t *testing.T) *importedPackageCacheFixture {
	t.Helper()

	root := t.TempDir()
	packageDir := filepath.Join(root, "resolved")
	require.NoError(t, os.MkdirAll(packageDir, 0o755))
	payloadPath := filepath.Join(packageDir, "payload.go")
	payloadSource := []byte("package cachedpkg\n\ntype Payload struct {\n\tID string\n}\n")
	require.NoError(t, os.WriteFile(payloadPath, payloadSource, 0o644))

	callsPath := filepath.Join(root, "calls.log")
	fakeBin := filepath.Join(root, "bin")
	require.NoError(t, os.MkdirAll(fakeBin, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(fakeBin, "go"),
		[]byte(`#!/bin/sh
for last_arg
do
	:
done
printf '%s\n' "$last_arg" >> "$GOAI_CACHE_TEST_CALLS"
case "$last_arg" in
	*negative*)
		exit 1
		;;
esac
printf '%s\n' "$GOAI_CACHE_TEST_PACKAGE_DIR"
`),
		0o755,
	))

	t.Setenv("GOAI_CACHE_TEST_CALLS", callsPath)
	t.Setenv("GOAI_CACHE_TEST_PACKAGE_DIR", packageDir)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	return &importedPackageCacheFixture{
		t:           t,
		root:        root,
		packageDir:  packageDir,
		payloadPath: payloadPath,
		callsPath:   callsPath,
		importCache: &sync.Map{},
	}
}

func (f *importedPackageCacheFixture) builder(workDir string, buildTags ...string) *_ASTSchemaBuilder {
	return _NewASTSchemaBuilderWithPrimaryFileAndCache(
		nil,
		"example.test/root",
		workDir,
		buildTags,
		nil,
		f.importCache,
	)
}

func (f *importedPackageCacheFixture) callsFor(importPath string) int {
	data, err := os.ReadFile(f.callsPath)
	require.NoError(f.t, err)

	return strings.Count(string(data), importPath+"\n")
}

func TestImportedPackageFilesCachesPositiveAndNegativeResults(t *testing.T) {
	fixture := newImportedPackageCacheFixture(t)
	positiveImport := "example.com/positive"
	firstFiles, firstDir, firstOK := fixture.builder(fixture.root)._ImportedPackageFiles(positiveImport)
	require.True(t, firstOK)
	require.NotEmpty(t, firstFiles)
	assert.Equal(t, fixture.packageDir, firstDir)
	firstFiles[0] = nil

	secondFiles, secondDir, secondOK := fixture.builder(fixture.root)._ImportedPackageFiles(positiveImport)
	require.True(t, secondOK)
	require.NotEmpty(t, secondFiles)
	assert.NotNil(t, secondFiles[0], "cached AST slice must not alias the caller's slice")
	assert.Equal(t, fixture.packageDir, secondDir)
	assert.Equal(t, 1, fixture.callsFor(positiveImport), "same resolution context must load once")

	negativeImport := "example.com/negative"
	_, _, firstNegativeOK := fixture.builder(fixture.root)._ImportedPackageFiles(negativeImport)
	_, _, secondNegativeOK := fixture.builder(fixture.root)._ImportedPackageFiles(negativeImport)
	assert.False(t, firstNegativeOK)
	assert.False(t, secondNegativeOK)
	assert.Equal(t, 1, fixture.callsFor(negativeImport), "failed loads must be cached for the extractor lifetime")
}

func TestImportedPackageFilesSeparatesResolutionContexts(t *testing.T) {
	fixture := newImportedPackageCacheFixture(t)
	firstWorkDir := filepath.Join(fixture.root, "first-workdir")
	secondWorkDir := filepath.Join(fixture.root, "second-workdir")
	require.NoError(t, os.MkdirAll(firstWorkDir, 0o755))
	require.NoError(t, os.MkdirAll(secondWorkDir, 0o755))

	workDirImport := "example.com/workdir"
	_, _, firstWorkDirOK := fixture.builder(firstWorkDir)._ImportedPackageFiles(workDirImport)
	_, _, secondWorkDirOK := fixture.builder(secondWorkDir)._ImportedPackageFiles(workDirImport)
	require.True(t, firstWorkDirOK)
	require.True(t, secondWorkDirOK)
	assert.Equal(t, 2, fixture.callsFor(workDirImport), "different work directories must not share resolutions")

	buildTagsImport := "example.com/buildtags"
	_, _, firstBuildTagsOK := fixture.builder(fixture.root, "first")._ImportedPackageFiles(buildTagsImport)
	_, _, secondBuildTagsOK := fixture.builder(fixture.root, "second")._ImportedPackageFiles(buildTagsImport)
	require.True(t, firstBuildTagsOK)
	require.True(t, secondBuildTagsOK)
	assert.Equal(t, 2, fixture.callsFor(buildTagsImport), "different build tags must not share parsed packages")
}

func TestImportedPackageFilesReusesParsedASTByResolvedDirectory(t *testing.T) {
	fixture := newImportedPackageCacheFixture(t)
	firstWorkDir := filepath.Join(fixture.root, "first-workdir")
	secondWorkDir := filepath.Join(fixture.root, "second-workdir")
	require.NoError(t, os.MkdirAll(firstWorkDir, 0o755))
	require.NoError(t, os.MkdirAll(secondWorkDir, 0o755))

	importPath := "example.com/parse-reuse"
	_, _, firstOK := fixture.builder(firstWorkDir)._ImportedPackageFiles(importPath)
	require.True(t, firstOK)
	require.NoError(t, os.Remove(fixture.payloadPath))

	files, dir, secondOK := fixture.builder(secondWorkDir)._ImportedPackageFiles(importPath)
	require.True(t, secondOK, "same resolved directory must reuse its parsed AST")
	require.NotEmpty(t, files)
	assert.NotNil(t, files[0])
	assert.Equal(t, fixture.packageDir, dir)
	assert.Equal(t, 2, fixture.callsFor(importPath), "different work directories still resolve independently")
}

func TestImportedPackageFilesCollapsesConcurrentLoads(t *testing.T) {
	fixture := newImportedPackageCacheFixture(t)
	concurrentImport := "example.com/concurrent"
	results := make(chan bool, 8)
	var waitGroup sync.WaitGroup
	for range 8 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()

			files, dir, ok := fixture.builder(fixture.root)._ImportedPackageFiles(concurrentImport)
			results <- ok && len(files) > 0 && dir == fixture.packageDir
		}()
	}
	waitGroup.Wait()
	close(results)

	for result := range results {
		assert.True(t, result)
	}
	assert.Equal(t, 1, fixture.callsFor(concurrentImport), "concurrent same-key loads must collapse to one")
}
