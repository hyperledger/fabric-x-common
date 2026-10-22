/*
Copyright IBM Corp. 2017 All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package rest

import (
	"net/http"
)

// NewRouter routes the configtxlator REST API.
func NewRouter() *http.ServeMux {
	router := http.NewServeMux()
	router.HandleFunc("POST /protolator/encode/{msgName}", Encode)
	router.HandleFunc("POST /protolator/decode/{msgName}", Decode)
	router.HandleFunc("POST /configtxlator/compute/update-from-configs", ComputeUpdateFromConfigs)
	return router
}
