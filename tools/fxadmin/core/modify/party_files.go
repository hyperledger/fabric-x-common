/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package modify

import (
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cockroachdb/errors"
)

// certificateFields are the fields of a party definition that hold a
// certificate or a CRL: a single one, or a list of them.
var certificateFields = map[string]bool{
	// Party, and the node_config of each of its nodes.
	"CACerts": true, "TLSCACerts": true,
	"client_tls_cert": true, "server_tls_cert": true, "sign_cert": true,
	// Org's MSP config, and its OU identifiers.
	"root_certs": true, "intermediate_certs": true, "admins": true, "revocation_list": true,
	"tls_root_certs": true, "tls_intermediate_certs": true, "known_certs": true,
	"certificate": true,
}

// readCertificateFiles replaces, in a parsed party definition section, the
// path of each certificate and CRL file with the file's content, base64-encoded
// as the block stores it. For example:
//
//	RouterConfig: {node_config: {server_tls_cert: router-tls.pem}}
//
// becomes
//
//	RouterConfig: {node_config: {server_tls_cert: LS0tLS1CRUdJTi...}}
//
// A relative path is relative to dir, the directory of the party definition.
func readCertificateFiles(section any, dir string) error {
	switch section := section.(type) {
	case map[string]any:
		return readMapCertificateFiles(section, dir)
	case []any:
		return readListCertificateFiles(section, dir)
	default:
		// A plain value, such as a host or a port, holds no certificate.
		return nil
	}
}

// readMapCertificateFiles reads the certificate files of a map: the paths of
// its certificate fields, and those of the maps and lists it holds.
func readMapCertificateFiles(section map[string]any, dir string) error {
	for key, value := range section {
		var err error
		if certificateFields[key] {
			section[key], err = readPaths(value, dir)
		} else {
			err = readCertificateFiles(value, dir)
		}
		if err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
	}
	return nil
}

// readListCertificateFiles reads the certificate files of each item of a list.
func readListCertificateFiles(section []any, dir string) error {
	for i, item := range section {
		if err := readCertificateFiles(item, dir); err != nil {
			return fmt.Errorf("[%d]: %w", i, err)
		}
	}
	return nil
}

// readPaths reads the file of a certificate field: a path, or a list of paths.
func readPaths(value any, dir string) (any, error) {
	switch value := value.(type) {
	case nil:
		return nil, nil
	case string:
		return readPEMFile(value, dir)
	case []any:
		contents := make([]any, len(value))
		for i, path := range value {
			path, ok := path.(string)
			if !ok {
				return nil, errors.Newf("[%d]: want a file path, got %T", i, value[i])
			}
			content, err := readPEMFile(path, dir)
			if err != nil {
				return nil, fmt.Errorf("[%d]: %w", i, err)
			}
			contents[i] = content
		}
		return contents, nil
	default:
		return nil, errors.Newf("want a file path or a list of file paths, got %T", value)
	}
}

// readPEMFile reads the PEM file at path and returns its first PEM block,
// base64-encoded. Only that block is kept, so a private key in the same file
// is not copied into the block.
func readPEMFile(path, dir string) (string, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", errors.Wrapf(err, "failed to read %q", path)
	}
	block, _ := pem.Decode(content)
	if block == nil {
		return "", errors.Newf("%q holds no PEM block", path)
	}
	return base64.StdEncoding.EncodeToString(pem.EncodeToMemory(block)), nil
}
