/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package deliver_test

import (
	"time"

	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/pkg/errors"

	"github.com/hyperledger/fabric-x-common/common/deliver"
	"github.com/hyperledger/fabric-x-common/common/deliver/mock"
	"github.com/hyperledger/fabric-x-common/protoutil"
	"github.com/hyperledger/fabric-x-common/tools/test"
)

var _ = ginkgo.Describe("SessionAccessControl", func() {
	var (
		fakeChain         *mock.Chain
		envelope          *cb.Envelope
		fakePolicyChecker *mock.PolicyChecker
		expiresAt         deliver.ExpiresAtFunc
	)

	ginkgo.BeforeEach(func() {
		envelope = &cb.Envelope{
			Payload: protoutil.MarshalOrPanic(&cb.Payload{
				Header: &cb.Header{},
			}),
		}

		fakeChain = &mock.Chain{}
		fakePolicyChecker = &mock.PolicyChecker{}
		expiresAt = func([]byte) time.Time { return time.Time{} }
	})

	ginkgo.It("evaluates the policy", func() {
		sac, err := deliver.NewSessionAC(fakeChain, envelope, fakePolicyChecker, "chain-id", expiresAt)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		err = sac.Evaluate()
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		gomega.Expect(fakePolicyChecker.CheckPolicyCallCount()).To(gomega.Equal(1))
		env, cid := fakePolicyChecker.CheckPolicyArgsForCall(0)
		gomega.Expect(env).To(test.ProtoEqual(envelope))
		gomega.Expect(cid).To(gomega.Equal("chain-id"))
	})

	ginkgo.Context("when policy evaluation returns an error", func() {
		ginkgo.BeforeEach(func() {
			fakePolicyChecker.CheckPolicyReturns(errors.New("no-access-for-you"))
		})

		ginkgo.It("returns the evaluation error", func() {
			sac, err := deliver.NewSessionAC(fakeChain, envelope, fakePolicyChecker, "chain-id", expiresAt)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = sac.Evaluate()
			gomega.Expect(err).To(gomega.MatchError("no-access-for-you"))
		})
	})

	ginkgo.It("caches positive policy evaluation", func() {
		sac, err := deliver.NewSessionAC(fakeChain, envelope, fakePolicyChecker, "chain-id", expiresAt)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		for i := 0; i < 5; i++ {
			err = sac.Evaluate()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		}
		gomega.Expect(fakePolicyChecker.CheckPolicyCallCount()).To(gomega.Equal(1))
	})

	ginkgo.Context("when the config sequence changes", func() {
		ginkgo.BeforeEach(func() {
			fakePolicyChecker.CheckPolicyReturnsOnCall(2, errors.New("access-now-denied"))
		})

		ginkgo.It("re-evaluates the policy", func() {
			sac, err := deliver.NewSessionAC(fakeChain, envelope, fakePolicyChecker, "chain-id", expiresAt)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(sac.Evaluate()).To(gomega.Succeed())
			gomega.Expect(fakePolicyChecker.CheckPolicyCallCount()).To(gomega.Equal(1))
			gomega.Expect(sac.Evaluate()).To(gomega.Succeed())
			gomega.Expect(fakePolicyChecker.CheckPolicyCallCount()).To(gomega.Equal(1))

			fakeChain.SequenceReturns(2)
			gomega.Expect(sac.Evaluate()).To(gomega.Succeed())
			gomega.Expect(fakePolicyChecker.CheckPolicyCallCount()).To(gomega.Equal(2))
			gomega.Expect(sac.Evaluate()).To(gomega.Succeed())
			gomega.Expect(fakePolicyChecker.CheckPolicyCallCount()).To(gomega.Equal(2))

			fakeChain.SequenceReturns(3)
			gomega.Expect(sac.Evaluate()).To(gomega.MatchError("access-now-denied"))
			gomega.Expect(fakePolicyChecker.CheckPolicyCallCount()).To(gomega.Equal(3))
		})
	})

	ginkgo.Context("when an identity expires", func() {
		ginkgo.BeforeEach(func() {
			expiresAt = func([]byte) time.Time {
				return time.Now().Add(250 * time.Millisecond)
			}
		})

		ginkgo.It("returns an identity expired error", func() {
			sac, err := deliver.NewSessionAC(fakeChain, envelope, fakePolicyChecker, "chain-id", expiresAt)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = sac.Evaluate()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Eventually(sac.Evaluate).Should(
				gomega.MatchError(gomega.ContainSubstring("deliver client identity expired")),
			)
		})
	})

	ginkgo.Context("when the envelope cannot be represented as signed data", func() {
		ginkgo.BeforeEach(func() {
			envelope = &cb.Envelope{}
		})

		ginkgo.It("returns an error", func() {
			_, expectedError := protoutil.EnvelopeAsSignedData(envelope)
			gomega.Expect(expectedError).To(gomega.HaveOccurred())

			_, err := deliver.NewSessionAC(fakeChain, envelope, fakePolicyChecker, "chain-id", expiresAt)
			gomega.Expect(err).To(gomega.Equal(expectedError))
		})
	})
})
