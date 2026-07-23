package releaserehearsal

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	maxAssuranceEntryBytes = 512 << 20
	maxAssuranceTotalBytes = 1 << 30
)

// ExtractAssurance expands the candidate assurance archive through a bounded,
// symlink-free extractor. The output path must not already exist.
func ExtractAssurance(dir, version, output string) error {
	dir, err := validateDirectory(dir)
	if err != nil {
		return err
	}
	if !versionPattern.MatchString(version) {
		return errors.New("version must match vX.Y.Z")
	}
	if strings.TrimSpace(output) == "" {
		return errors.New("assurance extraction output is required")
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return errors.New("resolve assurance extraction output")
	}
	if _, err := os.Lstat(output); err == nil {
		return errors.New("assurance extraction output already exists")
	} else if !os.IsNotExist(err) {
		return errors.New("inspect assurance extraction output")
	}
	parent := filepath.Dir(output)
	parentInfo, err := os.Lstat(parent)
	if err != nil || !parentInfo.IsDir() || parentInfo.Mode()&os.ModeSymlink != 0 {
		return errors.New("assurance extraction parent must be a non-symlink directory")
	}

	archiveName := "fairway_" + version + "_release_assurance.tar.gz"
	if _, err := inspectAsset(dir, archiveName); err != nil {
		return err
	}
	archive, err := os.Open(filepath.Join(dir, archiveName))
	if err != nil {
		return errors.New("open assurance archive")
	}
	defer archive.Close()
	compressed, err := gzip.NewReader(archive)
	if err != nil {
		return errors.New("open assurance gzip archive")
	}
	defer compressed.Close()

	staging, err := os.MkdirTemp(parent, ".fairway-assurance-extract-")
	if err != nil {
		return errors.New("create assurance extraction staging directory")
	}
	if err := os.Chmod(staging, 0o700); err != nil {
		_ = os.RemoveAll(staging)
		return errors.New("secure assurance extraction staging directory")
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(staging)
		}
	}()

	root := "fairway-" + version + "-release-assurance"
	seen := map[string]bool{}
	var total int64
	reader := tar.NewReader(compressed)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.New("read assurance archive")
		}
		name := header.Name
		clean := path.Clean(name)
		if name == "" || clean != name || path.IsAbs(name) || strings.Contains(name, `\`) ||
			(clean != root && !strings.HasPrefix(clean, root+"/")) {
			return fmt.Errorf("assurance archive contains unsafe path: %q", name)
		}
		if seen[clean] {
			return fmt.Errorf("assurance archive contains duplicate path: %s", clean)
		}
		seen[clean] = true
		target := filepath.Join(staging, filepath.FromSlash(clean))
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o700); err != nil {
				return errors.New("create assurance archive directory")
			}
		case tar.TypeReg, tar.TypeRegA:
			if header.Size < 0 || header.Size > maxAssuranceEntryBytes || total > maxAssuranceTotalBytes-header.Size {
				return fmt.Errorf("assurance archive entry exceeds extraction limit: %s", clean)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return errors.New("create assurance archive parent")
			}
			file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
			if err != nil {
				return errors.New("create assurance archive file")
			}
			written, copyErr := io.CopyN(file, reader, header.Size)
			closeErr := file.Close()
			if copyErr != nil || written != header.Size {
				return errors.New("extract assurance archive file")
			}
			if closeErr != nil {
				return errors.New("close assurance archive file")
			}
			total += header.Size
		default:
			return fmt.Errorf("assurance archive contains unsupported entry type: %s", clean)
		}
	}
	manifest := filepath.Join(staging, root, "manifest.json")
	manifestInfo, err := os.Lstat(manifest)
	if err != nil || !manifestInfo.Mode().IsRegular() || manifestInfo.Mode()&os.ModeSymlink != 0 {
		return errors.New("assurance archive is missing its regular manifest")
	}
	if err := os.Rename(staging, output); err != nil {
		return errors.New("promote extracted assurance archive")
	}
	cleanup = false
	return nil
}
