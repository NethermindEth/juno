package utils

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"gopkg.in/cheggaaa/pb.v1"
)

var snapshots = map[string]string{
	"mainnet": "https://juno-snapshot.s3.us-east-2.amazonaws.com/mainnet/juno_mainnet_v0.6.0_166353.tar",
	"goerli":  "https://juno-snapshot.s3.us-east-2.amazonaws.com/goerli/juno_goerli_v0.6.0_850192.tar",
	"goerli2": "https://juno-snapshot.s3.us-east-2.amazonaws.com/goerli2/juno_goerli2_v0.6.0_139043.tar",
}

const (
	downloadBufferSize = 8192 // 8KB
)

func Download(ctx context.Context, network, url string) (string, error) {
	if _, ok := snapshots[network]; !ok {
		return "", fmt.Errorf("the %s network is not supported", network)
	}

	// We added the url param for mocking tests
	if url == "" {
		url = snapshots[network]
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return "", err
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	outFile, err := os.CreateTemp("", fmt.Sprintf("%s-snapshot-*.tar", network))
	defer outFile.Close() //nolint:staticcheck
	if err != nil {
		return "", err
	}
	bar := pb.New(int(resp.ContentLength)).SetUnits(pb.U_BYTES)
	bar.Start()

	reader := bar.NewProxyReader(resp.Body)

	buffer := make([]byte, downloadBufferSize)
	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("canceled download")
		default:
			n, err := reader.Read(buffer)
			if err != nil {
				if err == io.EOF {
					bar.Finish()
					return outFile.Name(), nil
				}
				return "", err
			}
			_, err = outFile.Write(buffer[:n])
			if err != nil {
				return "", err
			}
		}
	}
}

func ExtractTar(src, destPath string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}

	// making sure the whole path exists before attempting writing to it
	err = os.MkdirAll(destPath, 0o750) //nolint:gomnd
	if err != nil {
		return err
	}
	fileReader, err := gzip.NewReader(srcFile)
	if err != nil {
		return err
	}

	tarBallReader := tar.NewReader(fileReader)
	defer fileReader.Close()

	for {
		header, err := tarBallReader.Next()
		if err != nil {
			if err == io.EOF {
				os.Remove(srcFile.Name())
				return nil
			}
			return err
		}

		filename := filepath.Join(destPath, filepath.Base(header.Name))

		switch header.Typeflag {
		case tar.TypeDir:
			// we don't want to create the top level directory as the 'destPath' contains the full path to the snapshot
			continue
		case tar.TypeReg:
			writer, err := os.Create(filename)
			if err != nil {
				return err
			}
			// golangci-lint will throw an error: G110: Potential DoS vulnerability via decompression bomb (gosec)
			// but this is not possible in this situation, as the payload is pre-defined
			// and the file size is not too big
			_, err = io.Copy(writer, tarBallReader) //nolint:gosec
			if err != nil {
				writer.Close()
				return err
			}
			err = os.Chmod(filename, os.FileMode(header.Mode))
			if err != nil {
				writer.Close()
				return err
			}
			writer.Close()
		default:
			fmt.Printf("unable to untar type: %c in file %s", header.Typeflag, filename)
		}
	}
}
