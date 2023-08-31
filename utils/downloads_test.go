package utils_test

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownload(t *testing.T) {
	tempFile := createTempFile(t)
	tarFile := tarFile(t, tempFile)
	mockServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		http.ServeFile(rw, r, tarFile)
	}))
	defer mockServer.Close()
	defer os.Remove(tempFile)
	defer os.Remove(tarFile)

	t.Run("unsupported network", func(t *testing.T) {
		_, err := utils.Download(context.Background(), "unsupported", mockServer.URL)
		assert.Equal(t, "the unsupported network is not supported", err.Error())
	})
	t.Run("normal case", func(t *testing.T) {
		_, err := utils.Download(context.Background(), "mainnet", mockServer.URL)
		require.NoError(t, err)
	})
}

func TestExtract(t *testing.T) {
	tempFile := createTempFile(t)
	tarFile := tarFile(t, tempFile)
	defer os.Remove(tempFile)
	defer os.Remove(tarFile)

	tempDir, err := os.MkdirTemp("", "mock-extracted-tar-*")
	if err != nil {
		t.Fatal("failed to create temporary directory", err)
	}
	defer os.RemoveAll(tempDir)

	err = utils.ExtractTar(tarFile, tempDir)
	require.NoError(t, err)
	extractedFiles, _ := os.ReadDir(tempDir)
	assert.Equal(t, 1, len(extractedFiles))
	extractedFilePath := filepath.Join(tempDir, extractedFiles[0].Name())
	extractedFile, err := os.Open(extractedFilePath)
	if err != nil {
		t.Fatalf("failed opening the extracted file %s", extractedFilePath)
	}
	extractedContent, err := io.ReadAll(extractedFile)
	if err != nil {
		t.Fatalf("couldn't read the file content for %s", extractedFilePath)
	}
	assert.Equal(t, []byte("random content"), extractedContent)
}

func createTempFile(t testing.TB) string {
	t.Helper()

	tempTxtFile, err := os.CreateTemp("", "mock-file-*.txt")
	defer tempTxtFile.Close() //nolint:staticcheck
	if err != nil {
		t.Fatal("failed creating a temporary text file")
	}
	_, err = tempTxtFile.WriteString("random content")
	if err != nil {
		t.Fatal("failed writing to the temporary created file")
	}

	return tempTxtFile.Name()
}

func tarFile(t testing.TB, source string) string {
	t.Helper()

	sourceFile, err := os.Open(source)
	if err != nil {
		t.Fatal("failed opening file", source)
	}
	defer sourceFile.Close()

	// Create the destination .tar file
	destinationFile, err := os.CreateTemp("", "mock-tar-*.tar") // maybe remove .tar
	if err != nil {
		t.Fatal("failed creating temporary tar file to")
	}
	defer destinationFile.Close()

	gzipWriter := gzip.NewWriter(destinationFile)
	// Create a new tar writer
	tarWriter := tar.NewWriter(gzipWriter)
	defer gzipWriter.Close()
	defer tarWriter.Close()

	// Get file information
	fileInfo, err := sourceFile.Stat()
	if err != nil {
		t.Fatal("error getting file source info for", source)
	}

	// Create a tar header
	header := &tar.Header{
		Name: fileInfo.Name(),
		Size: fileInfo.Size(),
		Mode: int64(fileInfo.Mode()),
	}

	// Write the header to the tar file
	err = tarWriter.WriteHeader(header)
	if err != nil {
		t.Fatal("error writing tar header")
	}

	// Copy the file content to the tar file
	_, err = io.Copy(tarWriter, sourceFile)
	if err != nil {
		t.Fatal("error copynig tar content")
	}

	return destinationFile.Name()
}
