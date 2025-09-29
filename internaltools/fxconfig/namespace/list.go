/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package namespace

import (
	"context"
	"fmt"
	"time"

	"github.com/hyperledger/fabric-x-committer/api/protoqueryservice"

	"github.com/hyperledger/fabric-x-common/cmd/common/comm"
)

// List calls the committer query service and shows all installed namespace policies.
func List(endpoint string) error {
	cl, err := comm.NewClient(comm.Config{})
	if err != nil {
		return fmt.Errorf("cannot get grpc client: %w", err)
	}

	conn, err := cl.NewDialer(endpoint)()
	if err != nil {
		return fmt.Errorf("dialing grpc client error: %w", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	client := protoqueryservice.NewQueryServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	res, err := client.GetNamespacePolicies(ctx, &protoqueryservice.Empty{})
	if err != nil {
		return fmt.Errorf("cannot query existing namespaces: %w", err)
	}

	fmt.Printf("Installed namespaces (%d total):\n", len(res.GetPolicies()))
	for i, p := range res.GetPolicies() {
		fmt.Printf("%d) %v: version %d policy: %x \n", i, p.GetNamespace(), p.GetVersion(), p.GetPolicy())
	}
	fmt.Printf("\n")

	return nil
}
