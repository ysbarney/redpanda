// Copyright 2021 Vectorized, Inc.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.md
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0

package certmanager

import "fmt"

// cert-manager has limit of 64 bytes on the common name of certificate
const (
	nameLimit       = 64
	separatorLength = 1 // we use - as separator
)

// CommonName is certificate CN that is shortened to 64 chars
type CommonName string

// NewCommonName ensures the name does not exceed the limit of 64 bytes. It always
// shortens the cluster name and keeps the whole suffix.
// Suffix and name will be separated with -
func NewCommonName(clusterName, suffix string) CommonName {
	suffixLength := len(suffix)
	maxClusterNameLength := nameLimit - suffixLength - separatorLength
	if len(clusterName) > maxClusterNameLength {
		clusterName = clusterName[:maxClusterNameLength]
	}
	return CommonName(fmt.Sprintf("%s-%s", clusterName, suffix))
}
