package chaincode_test

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hyperledger/fabric-admin-sdk/pkg/chaincode"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Package", func() {
	It("CCaaS", func() {
		dummyConnection := chaincode.Connection{
			Address:     "127.0.0.1:8080",
			DialTimeout: "10s",
			TLSRequired: false,
		}
		dummyMeta := chaincode.Metadata{
			Type:  "ccaas",
			Label: "basic-asset",
		}
		err := chaincode.PackageCCAAS(dummyConnection, dummyMeta, tmpDir, "chaincode.tar.gz")
		Expect(err).NotTo(HaveOccurred())
		// so far no plan to verify the file
		file, err := os.Open(tmpDir + "/chaincode.tar.gz")
		Expect(err).NotTo(HaveOccurred())
		defer file.Close()
		err = Untar(file, tmpDir)
		Expect(err).NotTo(HaveOccurred())
	})
})

// Untar takes a gzip-ed tar archive, and extracts it to dst.
// It returns an error if the tar contains any files which would escape to a
// parent of dst, or if the archive contains any files whose type is not
// a regular file or directory.
//
//nolint:cyclop,gocognit
func Untar(buffer io.Reader, dst string) error {
	gzr, err := gzip.NewReader(buffer)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()

		if errors.Is(err, io.EOF) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("could not get next tar element %w", err)
		}

		target, err := SanitizeArchivePath(dst, header.Name)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o700); err != nil {
				return fmt.Errorf("could not create directory '%s' %w", header.Name, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return fmt.Errorf("could not create directory '%s' %w", filepath.Dir(header.Name), err)
			}

			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode)) //#nosec:G115
			if err != nil {
				return fmt.Errorf("could not create file '%s' %w", header.Name, err)
			}

			// copy over contents
			_, err = io.Copy(f, tr) // #nosec G110 -- Only used on test input we provide
			if err != nil {
				return err
			}

			_ = f.Close()
		default:
			return fmt.Errorf("invalid file type '%v' contained in archive for file '%s'", header.Typeflag, header.Name)
		}
	}
}

func SanitizeArchivePath(d, t string) (string, error) {
	target := filepath.Join(d, t)
	if !strings.HasPrefix(target, filepath.Clean(d)) {
		return "", fmt.Errorf("content filepath is tainted: %s", t)
	}

	return target, nil
}
