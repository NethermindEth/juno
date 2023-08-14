package utils

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"gopkg.in/cheggaaa/pb.v1"
)

var snapshots = map[string]string{
	"mainnet": "https://juno-snapshot.s3.us-east-2.amazonaws.com/mainnet/juno_mainnet_v0.5.0_136902.tar",
	"goerli":  "https://juno-snapshot.s3.us-east-2.amazonaws.com/goerli/juno_goerli_v0.5.0_839969.tar",
	"goerli2": "https://juno-snapshot.s3.us-east-2.amazonaws.com/goerli2/juno_goerli2_v0.5.0_135973.tar",
}

const (
	downloadBufferSize = 8192 // 8KB
	dirPermissionMod   = 0o770
)

type Downloader interface {
	DownloadAndWrite(ctx context.Context, reader io.Reader, writer io.Writer, bar *pb.ProgressBar) error
}

type RealDownloader struct{}

func (d *RealDownloader) DownloadAndWrite(ctx context.Context, reader io.Reader, writer io.Writer, bar *pb.ProgressBar) error {
	buffer := make([]byte, downloadBufferSize)
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("canceled download")
		default:
			n, err := reader.Read(buffer)
			if err != nil {
				if err == io.EOF {
					bar.Finish()
					return nil
				}
				return err
			}

			_, err = writer.Write(buffer[:n])
			if err != nil {
				return err
			}
		}
	}
}

func DownloadFile(ctx context.Context, network, location string, downloader Downloader) error {
	if _, ok := snapshots[network]; !ok {
		return fmt.Errorf("the %s network is not supported", network)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, snapshots[network], nil)
	if err != nil {
		return err
	}
	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return err
	}
	defer resp.Body.Close()

	outFilePath := filepath.Join(location, filepath.Base(snapshots[network]))
	out, err := create(outFilePath)
	if err != nil {
		return err
	}
	defer out.Close()

	bar := pb.New(int(resp.ContentLength)).SetUnits(pb.U_BYTES)
	bar.Start()

	reader := bar.NewProxyReader(resp.Body)

	err = downloader.DownloadAndWrite(ctx, reader, out, bar)
	if err != nil {
		return err
	}

	return nil
}

func create(p string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(p), dirPermissionMod); err != nil {
		return nil, err
	}
	return os.Create(p)
}
