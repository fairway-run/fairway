package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("requires keygen, fingerprint, archive-file, archive-dir, scan-retained, scan-promote, or failure-packet")
	}
	switch args[0] {
	case "keygen":
		fs := flag.NewFlagSet("keygen", flag.ContinueOnError)
		privatePath := fs.String("private", "", "private key output")
		publicPath := fs.String("public", "", "public key output")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 || *privatePath == "" || *publicPath == "" {
			return errors.New("keygen requires --private and --public")
		}
		return generateKey(*privatePath, *publicPath)
	case "fingerprint":
		fs := flag.NewFlagSet("fingerprint", flag.ContinueOnError)
		publicPath := fs.String("public", "", "base64 Ed25519 public key file")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 || *publicPath == "" {
			return errors.New("fingerprint requires --public")
		}
		fingerprint, err := publicKeyFingerprint(*publicPath)
		if err != nil {
			return err
		}
		fmt.Println(fingerprint)
		return nil
	case "archive-file":
		fs := flag.NewFlagSet("archive-file", flag.ContinueOnError)
		input := fs.String("input", "", "input file")
		name := fs.String("name", "", "archive entry name")
		out := fs.String("out", "", "new tar.gz output")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 || *input == "" || *name == "" || *out == "" {
			return errors.New("archive-file requires --input, --name, and --out")
		}
		return archiveFile(*input, *name, *out)
	case "archive-dir":
		fs := flag.NewFlagSet("archive-dir", flag.ContinueOnError)
		dir := fs.String("dir", "", "input directory")
		rootName := fs.String("root-name", "", "archive root name")
		out := fs.String("out", "", "new tar.gz output")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 || *dir == "" || *rootName == "" || *out == "" {
			return errors.New("archive-dir requires --dir, --root-name, and --out")
		}
		return archiveDirectory(*dir, *rootName, *out)
	case "scan-retained":
		fs := flag.NewFlagSet("scan-retained", flag.ContinueOnError)
		dir := fs.String("dir", "", "retained output directory")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 || *dir == "" {
			return errors.New("scan-retained requires --dir")
		}
		return scanRetainedTree(*dir)
	case "scan-promote":
		fs := flag.NewFlagSet("scan-promote", flag.ContinueOnError)
		staging := fs.String("staging", "", "quiescent private staging directory")
		output := fs.String("output", "", "new atomically promoted output directory")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 || *staging == "" || *output == "" {
			return errors.New("scan-promote requires --staging and --output")
		}
		return promoteRetainedTree(*staging, *output)
	case "failure-packet":
		fs := flag.NewFlagSet("failure-packet", flag.ContinueOnError)
		output := fs.String("output", "", "new bounded failure output directory")
		staging := fs.String("staging", "", "private staging directory to remove")
		phase := fs.String("phase", "", "bounded build phase")
		exitCode := fs.String("exit-code", "", "non-zero process exit code")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		code, err := strconv.Atoi(*exitCode)
		if err != nil || code <= 0 {
			return errors.New("failure-packet requires a positive --exit-code")
		}
		if fs.NArg() != 0 || *output == "" || *staging == "" || *phase == "" {
			return errors.New("failure-packet requires --output, --staging, --phase, and --exit-code")
		}
		return writeFailurePacket(*output, *staging, *phase, code)
	default:
		return fmt.Errorf("unknown helper subcommand %q", args[0])
	}
}

var failurePhasePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,79}$`)

func validateStagingOutput(output, staging string) error {
	if !filepath.IsAbs(output) || !filepath.IsAbs(staging) || output == string(filepath.Separator) || staging == string(filepath.Separator) {
		return errors.New("staging and output paths must be bounded absolute directories")
	}
	outputParent, stagingParent := filepath.Clean(filepath.Dir(output)), filepath.Clean(filepath.Dir(staging))
	if outputParent != stagingParent || !strings.HasPrefix(filepath.Base(staging), ".fairway-sovereign-rehearsal-staging.") {
		return errors.New("staging must be a generated sibling of output")
	}
	if _, err := os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
		return errors.New("output must not already exist")
	}
	return nil
}

func writeFailurePacket(output, staging, phase string, exitCode int) error {
	if err := validateStagingOutput(output, staging); err != nil {
		return err
	}
	if !failurePhasePattern.MatchString(phase) || exitCode <= 0 {
		return errors.New("failure packet phase or exit code is invalid")
	}
	if err := os.RemoveAll(staging); err != nil {
		return errors.New("remove failed rehearsal staging")
	}
	if err := os.MkdirAll(filepath.Join(output, "diagnostics"), 0o755); err != nil {
		return errors.New("create bounded failure output")
	}
	packet := struct {
		Schema                 string `json:"schema"`
		Phase                  string `json:"phase"`
		ExitCode               int    `json:"exit_code"`
		PrivateSigningMaterial string `json:"private_signing_material"`
		AuthorityBoundary      string `json:"authority_boundary"`
	}{
		Schema:                 "fairway.sovereign-rehearsal-build-failure.v1",
		Phase:                  phase,
		ExitCode:               exitCode,
		PrivateSigningMaterial: "not_retained",
		AuthorityBoundary:      "build diagnostic only; no release, publish, install, deploy, credential, public-exposure, or live authority",
	}
	data, err := json.Marshal(packet)
	if err != nil {
		return errors.New("encode bounded failure packet")
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(output, "diagnostics", "failure.json"), data, 0o644); err != nil {
		_ = os.RemoveAll(output)
		return errors.New("write bounded failure packet")
	}
	return nil
}

func promoteRetainedTree(staging, output string) error {
	return promoteRetainedTreeWithScan(staging, output, scanRetainedTree)
}

func promoteRetainedTreeWithScan(staging, output string, scan func(string) error) error {
	if err := validateStagingOutput(output, staging); err != nil {
		return err
	}
	before, err := retainedTreeSnapshot(staging)
	if err != nil {
		return err
	}
	if err := scan(staging); err != nil {
		return err
	}
	after, err := retainedTreeSnapshot(staging)
	if err != nil {
		return err
	}
	if before != after {
		return errors.New("retained output changed during verification")
	}
	if err := os.Rename(staging, output); err != nil {
		return errors.New("atomically promote retained output")
	}
	return nil
}

func retainedTreeSnapshot(root string) (string, error) {
	rootInfo, err := os.Lstat(root)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("retained staging must be a non-symlink directory")
	}
	h := sha256.New()
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return errors.New("snapshot retained staging")
		}
		if path == root {
			return nil
		}
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("snapshot retained staging entry")
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return errors.New("snapshot path escapes retained staging")
		}
		kind := byte('d')
		if !entry.IsDir() {
			if !info.Mode().IsRegular() {
				return errors.New("snapshot contains non-regular entry")
			}
			kind = 'f'
		}
		_, _ = fmt.Fprintf(h, "%c\x00%s\x00%04o\x00%d\x00", kind, filepath.ToSlash(rel), info.Mode().Perm(), info.Size())
		if kind == 'f' {
			file, err := os.Open(path)
			if err != nil {
				return errors.New("open retained staging snapshot file")
			}
			_, copyErr := io.Copy(h, file)
			closeErr := file.Close()
			if copyErr != nil || closeErr != nil {
				return errors.New("read retained staging snapshot file")
			}
		}
		_, _ = h.Write([]byte{0})
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

var retainedSecretPattern = regexp.MustCompile(`(?i)-----BEGIN (?:(?:OPENSSH|RSA|EC) PRIVATE KEY|PRIVATE KEY)-----\r?\n[A-Za-z0-9+/=\r\n]{32,}|authorization:\s*bearer\s+[A-Za-z0-9._~+/-]{12,}|(?:private[_ -]?key|client[_ -]?secret|api[_ -]?token)\s*[:=]\s*["']?[A-Za-z0-9+/=_-]{24,}`)

func forbiddenRetainedName(name string) bool {
	name = strings.ToLower(filepath.Base(name))
	return name == ".ds_store" || strings.HasPrefix(name, "._") ||
		strings.Contains(name, "secret") || strings.Contains(name, "token") ||
		(strings.Contains(name, "private") && strings.HasSuffix(name, ".b64"))
}

func scanRetainedTree(dir string) error {
	rootInfo, err := os.Lstat(dir)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return errors.New("retained output must be a non-symlink directory")
	}
	return filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return errors.New("scan retained output")
		}
		if path == dir {
			return nil
		}
		name := strings.ToLower(entry.Name())
		if forbiddenRetainedName(name) {
			return errors.New("retained output contains a forbidden filename")
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("retained output contains a symlink")
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return errors.New("retained output contains a non-regular file")
		}
		if strings.HasSuffix(name, ".tar.gz") {
			return scanRetainedArchive(path)
		}
		return scanRetainedFile(path)
	})
}

func scanRetainedFile(path string) error {
	const maximumRetainedFile = 128 << 20
	info, err := os.Stat(path)
	if err != nil || info.Size() > maximumRetainedFile {
		return errors.New("retained output file is unavailable or exceeds scan bound")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return errors.New("read retained output file")
	}
	if retainedSecretPattern.Match(data) {
		return errors.New("retained output contains credential material")
	}
	return nil
}

func scanRetainedArchive(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return errors.New("open retained archive")
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return errors.New("open retained gzip archive")
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return errors.New("read retained tar archive")
		}
		if header.Typeflag == tar.TypeDir {
			continue
		}
		if forbiddenRetainedName(header.Name) {
			return errors.New("retained archive contains a forbidden filename")
		}
		if header.Typeflag != tar.TypeReg || header.Size < 0 || header.Size > 128<<20 {
			return errors.New("retained archive contains unsupported or oversized entry")
		}
		data, err := io.ReadAll(io.LimitReader(tr, header.Size+1))
		if err != nil || int64(len(data)) != header.Size {
			return errors.New("read retained archive entry")
		}
		if retainedSecretPattern.Match(data) {
			return errors.New("retained archive contains credential material")
		}
	}
}

func generateKey(privatePath, publicPath string) error {
	if err := requireNewPaths(privatePath, publicPath); err != nil {
		return err
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return errors.New("generate Ed25519 key")
	}
	defer zero(privateKey)
	if err := os.WriteFile(privatePath, []byte(base64.StdEncoding.EncodeToString(privateKey)+"\n"), 0o600); err != nil {
		return errors.New("write private key")
	}
	if err := os.WriteFile(publicPath, []byte(base64.StdEncoding.EncodeToString(publicKey)+"\n"), 0o644); err != nil {
		_ = os.Remove(privatePath)
		return errors.New("write public key")
	}
	return nil
}

func publicKeyFingerprint(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > 1024 {
		return "", errors.New("public key must be a bounded regular non-symlink file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", errors.New("read public key")
	}
	publicKey, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return "", errors.New("public key must contain one base64 Ed25519 public key")
	}
	digest := sha256.Sum256(publicKey)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func archiveFile(input, name, out string) error {
	if strings.Contains(name, "/") || name == "." || name == ".." || strings.TrimSpace(name) == "" {
		return errors.New("archive file name must be one safe path component")
	}
	info, err := os.Lstat(input)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("archive input must be a regular non-symlink file")
	}
	return writeArchive(out, func(tw *tar.Writer) error {
		return addArchiveFile(tw, input, name, info)
	})
}

func archiveDirectory(dir, rootName, out string) error {
	if strings.Contains(rootName, "/") || rootName == "." || rootName == ".." || strings.TrimSpace(rootName) == "" {
		return errors.New("archive root name must be one safe path component")
	}
	rootInfo, err := os.Lstat(dir)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return errors.New("archive input must be a non-symlink directory")
	}
	var paths []string
	err = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == dir {
			return nil
		}
		if name := entry.Name(); name == ".DS_Store" || strings.HasPrefix(name, "._") {
			return errors.New("archive directory contains macOS metadata noise")
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("archive directory contains a symlink")
		}
		if !entry.IsDir() && !entry.Type().IsRegular() {
			return errors.New("archive directory contains a non-regular file")
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(paths)
	return writeArchive(out, func(tw *tar.Writer) error {
		if err := tw.WriteHeader(normalizedHeader(rootName+"/", 0, true)); err != nil {
			return err
		}
		for _, path := range paths {
			info, err := os.Lstat(path)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(dir, path)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return errors.New("archive path escapes input directory")
			}
			name := rootName + "/" + filepath.ToSlash(rel)
			if info.IsDir() {
				if err := tw.WriteHeader(normalizedHeader(name+"/", 0, true)); err != nil {
					return err
				}
				continue
			}
			if err := addArchiveFile(tw, path, name, info); err != nil {
				return err
			}
		}
		return nil
	})
}

func writeArchive(out string, write func(*tar.Writer) error) (err error) {
	if _, err := os.Lstat(out); !errors.Is(err, os.ErrNotExist) {
		return errors.New("archive output must not already exist")
	}
	file, err := os.OpenFile(out, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return errors.New("create archive output")
	}
	cleanup := true
	defer func() {
		_ = file.Close()
		if cleanup {
			_ = os.Remove(out)
		}
	}()
	gz, err := gzip.NewWriterLevel(file, gzip.BestCompression)
	if err != nil {
		return err
	}
	gz.Header.ModTime = time.Unix(0, 0).UTC()
	gz.Header.OS = 255
	tw := tar.NewWriter(gz)
	if err := write(tw); err != nil {
		_ = tw.Close()
		_ = gz.Close()
		return err
	}
	if err := tw.Close(); err != nil {
		_ = gz.Close()
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func addArchiveFile(tw *tar.Writer, input, name string, info os.FileInfo) error {
	header := normalizedHeader(name, info.Size(), false)
	if info.Mode().Perm()&0o111 != 0 {
		header.Mode = 0o755
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	file, err := os.Open(input)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(tw, file)
	return err
}

func normalizedHeader(name string, size int64, dir bool) *tar.Header {
	header := &tar.Header{Name: name, Mode: 0o644, Size: size, ModTime: time.Unix(0, 0).UTC(), AccessTime: time.Time{}, ChangeTime: time.Time{}, Uid: 0, Gid: 0, Uname: "root", Gname: "root", Format: tar.FormatPAX}
	if dir {
		header.Typeflag = tar.TypeDir
		header.Mode = 0o755
		header.Size = 0
	} else {
		header.Typeflag = tar.TypeReg
	}
	return header
}

func requireNewPaths(paths ...string) error {
	for _, path := range paths {
		if strings.TrimSpace(path) == "" || !filepath.IsAbs(path) {
			return errors.New("helper output paths must be absolute")
		}
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			return errors.New("helper output path already exists")
		}
	}
	return nil
}

func zero(value []byte) {
	for i := range value {
		value[i] = 0
	}
}
