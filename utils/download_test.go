package utils

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/cheggaaa/pb.v1"
)

type MockDownloader struct {
	DownloadAndWriteFunc func(ctx context.Context, reader io.Reader, writer io.Writer, bar *pb.ProgressBar) error
}

func (m *MockDownloader) DownloadAndWrite(ctx context.Context, reader io.Reader, writer io.Writer, bar *pb.ProgressBar) error {
	return m.DownloadAndWriteFunc(ctx, reader, writer, bar)
}

func TestDownloadSnapshot(t *testing.T) {
	t.Run("Successful download", func(t *testing.T) {
		mockDownloader := &MockDownloader{
			DownloadAndWriteFunc: func(ctx context.Context, reader io.Reader, writer io.Writer, bar *pb.ProgressBar) error {
				return nil
			},
		}

		err := DownloadFile(context.Background(), "mainnet", createTemporaryDir(t), mockDownloader)
		assert.NoError(t, err)
	})

	t.Run("Unsuccessful download because of wrong unsupported network", func(t *testing.T) {
		mockDownloader := &MockDownloader{
			DownloadAndWriteFunc: func(ctx context.Context, reader io.Reader, writer io.Writer, bar *pb.ProgressBar) error {
				return nil
			},
		}
		err := DownloadFile(context.Background(), "unsupported_network", createTemporaryDir(t), mockDownloader)
		assert.Error(t, err)
		assert.Equal(t, "the unsupported_network network is not supported", err.Error())
	})

	t.Run("Unsuccessful download because of download cancel", func(t *testing.T) {
		mockDownloader := &MockDownloader{
			DownloadAndWriteFunc: func(ctx context.Context, reader io.Reader, writer io.Writer, bar *pb.ProgressBar) error {
				return context.Canceled
			},
		}

		err := DownloadFile(context.Background(), "mainnet", createTemporaryDir(t), mockDownloader)
		assert.Error(t, err)
		assert.Equal(t, "context canceled", err.Error())
	})

	t.Run("Unsuccessful download", func(t *testing.T) {
		mockDownloader := &MockDownloader{
			DownloadAndWriteFunc: func(ctx context.Context, reader io.Reader, writer io.Writer, bar *pb.ProgressBar) error {
				return errors.New("download error")
			},
		}

		err := DownloadFile(context.Background(), "mainnet", createTemporaryDir(t), mockDownloader)
		assert.Error(t, err)
		assert.Equal(t, "download error", err.Error())
	})
}

func createTemporaryDir(t testing.TB) string {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "download_+test")
	if err != nil {
		log.Fatal(err)
	}

	// the next line should never return an error as if there is an error, it will be of type *PathError
	// which can't happen here
	defer os.RemoveAll(tempDir)

	return tempDir
}
