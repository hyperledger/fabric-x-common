/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package cryptogen

import (
	"bytes"
	"text/template"

	"github.com/cockroachdb/errors"
)

type hostnameData struct {
	Prefix string
	Index  int
	Domain string
}

type specData struct {
	Hostname   string
	Domain     string
	CommonName string
}

func renderOrgSpec(orgSpec *OrgSpec, nodeType int) error {
	var prefix string
	switch nodeType {
	case NodeTypeOrderer:
		prefix = OrdererPrefix
	default: // msp.NodeTypePeer
		prefix = PeersPrefix
	}

	publicKeyAlg := getPublicKeyAlg(orgSpec.Template.PublicKeyAlgorithm)
	// First process all of our templated nodes
	for i := range orgSpec.Template.Count {
		data := hostnameData{
			Prefix: prefix,
			Index:  i + orgSpec.Template.Start,
			Domain: orgSpec.Domain,
		}

		hostname, err := parseTemplateWithDefault(orgSpec.Template.Hostname, defaultHostnameTemplate, data)
		if err != nil {
			return err
		}

		orgSpec.Specs = append(orgSpec.Specs, NodeSpec{
			Hostname:           hostname,
			SANS:               orgSpec.Template.SANS,
			PublicKeyAlgorithm: publicKeyAlg,
		})
	}

	// Touch up all general node-specs to add the domain
	for _, spec := range orgSpec.Specs {
		err := renderNodeSpec(orgSpec.Domain, &spec)
		if err != nil {
			return err
		}
	}

	// Process the CA node-spec in the same manner
	if len(orgSpec.CA.Hostname) == 0 {
		orgSpec.CA.Hostname = DefaultCaHostname
	}
	return renderNodeSpec(orgSpec.Domain, &orgSpec.CA)
}

func renderNodeSpec(domain string, spec *NodeSpec) error {
	data := specData{
		Hostname: spec.Hostname,
		Domain:   domain,
	}

	// Process our CommonName
	cn, err := parseTemplateWithDefault(spec.CommonName, defaultCNTemplate, data)
	if err != nil {
		return err
	}

	spec.CommonName = cn
	data.CommonName = cn

	if spec.PublicKeyAlgorithm == "" {
		spec.PublicKeyAlgorithm = ECDSA
	}

	// Save off our original, unprocessed SANS entries
	origSANS := spec.SANS

	// Set our implicit SANS entries for CN/Hostname
	spec.SANS = []string{cn, spec.Hostname}

	// Finally, process any remaining SANS entries
	for _, _san := range origSANS {
		san, parseErr := parseTemplate(_san, data)
		if parseErr != nil {
			return parseErr
		}
		spec.SANS = append(spec.SANS, san)
	}

	return nil
}

func parseTemplate(input string, data any) (string, error) {
	t, err := template.New("parse").Parse(input)
	if err != nil {
		return "", errors.Wrap(err, "error parsing template")
	}

	output := new(bytes.Buffer)
	err = t.Execute(output, data)
	if err != nil {
		return "", errors.Wrap(err, "error executing template")
	}

	return output.String(), nil
}

func parseTemplateWithDefault(input, defaultInput string, data any) (string, error) {
	// Use the default if the input is an empty string
	if len(input) == 0 {
		input = defaultInput
	}
	return parseTemplate(input, data)
}
