/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package grpcmetrics_test

import (
	"context"
	"errors"
	"io"
	"net"

	"github.com/hyperledger/fabric-lib-go/common/metrics/metricsfakes"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/hyperledger/fabric-x-common/common/grpcmetrics"
	"github.com/hyperledger/fabric-x-common/common/grpcmetrics/fakes"
	"github.com/hyperledger/fabric-x-common/common/grpcmetrics/testpb"
	"github.com/hyperledger/fabric-x-common/tools/test"
)

var _ = ginkgo.Describe("Interceptor", func() {
	var (
		fakeEchoService   *fakes.EchoServiceServer
		echoServiceClient testpb.EchoServiceClient

		fakeRequestDuration   *metricsfakes.Histogram
		fakeRequestsReceived  *metricsfakes.Counter
		fakeRequestsCompleted *metricsfakes.Counter
		fakeMessagesSent      *metricsfakes.Counter
		fakeMessagesReceived  *metricsfakes.Counter

		unaryMetrics  *grpcmetrics.UnaryMetrics
		streamMetrics *grpcmetrics.StreamMetrics

		listener        net.Listener
		serveCompleteCh chan error
		server          *grpc.Server
	)

	ginkgo.BeforeEach(func() {
		var err error
		listener, err = net.Listen("tcp", "127.0.0.1:0")
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		fakeEchoService = &fakes.EchoServiceServer{}
		fakeEchoService.EchoStub = func(ctx context.Context, msg *testpb.Message) (*testpb.Message, error) {
			msg.Sequence++
			return msg, nil
		}
		fakeEchoService.EchoStreamStub = func(stream testpb.EchoService_EchoStreamServer) error {
			for {
				msg, err := stream.Recv()
				if err == io.EOF {
					return nil
				}
				if err != nil {
					return err
				}

				msg.Sequence++
				err = stream.Send(msg)
				if err != nil {
					return err
				}
			}
		}

		fakeRequestDuration = &metricsfakes.Histogram{}
		fakeRequestDuration.WithReturns(fakeRequestDuration)
		fakeRequestsReceived = &metricsfakes.Counter{}
		fakeRequestsReceived.WithReturns(fakeRequestsReceived)
		fakeRequestsCompleted = &metricsfakes.Counter{}
		fakeRequestsCompleted.WithReturns(fakeRequestsCompleted)
		fakeMessagesSent = &metricsfakes.Counter{}
		fakeMessagesSent.WithReturns(fakeMessagesSent)
		fakeMessagesReceived = &metricsfakes.Counter{}
		fakeMessagesReceived.WithReturns(fakeMessagesReceived)

		unaryMetrics = &grpcmetrics.UnaryMetrics{
			RequestDuration:   fakeRequestDuration,
			RequestsReceived:  fakeRequestsReceived,
			RequestsCompleted: fakeRequestsCompleted,
		}

		streamMetrics = &grpcmetrics.StreamMetrics{
			RequestDuration:   fakeRequestDuration,
			RequestsReceived:  fakeRequestsReceived,
			RequestsCompleted: fakeRequestsCompleted,
			MessagesSent:      fakeMessagesSent,
			MessagesReceived:  fakeMessagesReceived,
		}

		server = grpc.NewServer(
			grpc.StreamInterceptor(grpcmetrics.StreamServerInterceptor(streamMetrics)),
			grpc.UnaryInterceptor(grpcmetrics.UnaryServerInterceptor(unaryMetrics)),
		)

		testpb.RegisterEchoServiceServer(server, fakeEchoService)
		serveCompleteCh = make(chan error, 1)
		go func() { serveCompleteCh <- server.Serve(listener) }()

		cc, err := grpc.Dial(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		echoServiceClient = testpb.NewEchoServiceClient(cc)
	})

	ginkgo.AfterEach(func() {
		err := listener.Close()
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Eventually(serveCompleteCh).Should(gomega.Receive())
	})

	ginkgo.Describe("Unary Metrics", func() {
		ginkgo.It("records request duration", func() {
			resp, err := echoServiceClient.Echo(context.Background(), &testpb.Message{Message: "yo"})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(resp).To(test.ProtoEqual(&testpb.Message{Message: "yo", Sequence: 1}))

			gomega.Expect(fakeRequestDuration.WithCallCount()).To(gomega.Equal(1))
			labelValues := fakeRequestDuration.WithArgsForCall(0)
			gomega.Expect(labelValues).To(gomega.Equal([]string{
				"service", "testpb_EchoService",
				"method", "Echo",
				"code", "OK",
			}))
			gomega.Expect(fakeRequestDuration.ObserveCallCount()).To(gomega.Equal(1))
			gomega.Expect(fakeRequestDuration.ObserveArgsForCall(0)).NotTo(gomega.BeZero())
			gomega.Expect(fakeRequestDuration.ObserveArgsForCall(0)).To(gomega.BeNumerically("<", 1.0))
		})

		ginkgo.It("records requests received before requests completed", func() {
			fakeRequestsReceived.AddStub = func(delta float64) {
				defer ginkgo.GinkgoRecover()
				gomega.Expect(fakeRequestsCompleted.AddCallCount()).To(gomega.Equal(0))
			}

			resp, err := echoServiceClient.Echo(context.Background(), &testpb.Message{Message: "yo"})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(resp).To(test.ProtoEqual(&testpb.Message{Message: "yo", Sequence: 1}))

			gomega.Expect(fakeRequestsReceived.WithCallCount()).To(gomega.Equal(1))
			labelValues := fakeRequestsReceived.WithArgsForCall(0)
			gomega.Expect(labelValues).To(gomega.Equal([]string{
				"service", "testpb_EchoService",
				"method", "Echo",
			}))
			gomega.Expect(fakeRequestsReceived.AddCallCount()).To(gomega.Equal(1))
			gomega.Expect(fakeRequestsReceived.AddArgsForCall(0)).To(gomega.BeNumerically("~", 1.0))
		})

		ginkgo.It("records requests completed after requests received", func() {
			fakeRequestsCompleted.AddStub = func(delta float64) {
				defer ginkgo.GinkgoRecover()
				gomega.Expect(fakeRequestsReceived.AddCallCount()).To(gomega.Equal(1))
			}

			resp, err := echoServiceClient.Echo(context.Background(), &testpb.Message{Message: "yo"})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(resp).To(test.ProtoEqual(&testpb.Message{Message: "yo", Sequence: 1}))

			gomega.Expect(fakeRequestsCompleted.WithCallCount()).To(gomega.Equal(1))
			labelValues := fakeRequestsCompleted.WithArgsForCall(0)
			gomega.Expect(labelValues).To(gomega.Equal([]string{
				"service", "testpb_EchoService",
				"method", "Echo",
				"code", "OK",
			}))
			gomega.Expect(fakeRequestsCompleted.AddCallCount()).To(gomega.Equal(1))
			gomega.Expect(fakeRequestsCompleted.AddArgsForCall(0)).To(gomega.BeNumerically("~", 1.0))
		})
	})

	ginkgo.Describe("Stream Metrics", func() {
		ginkgo.It("records request duration", func() {
			streamClient, err := echoServiceClient.EchoStream(context.Background())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			streamMessages(streamClient)

			gomega.Expect(fakeRequestDuration.WithCallCount()).To(gomega.Equal(1))
			labelValues := fakeRequestDuration.WithArgsForCall(0)
			gomega.Expect(labelValues).To(gomega.Equal([]string{
				"service", "testpb_EchoService",
				"method", "EchoStream",
				"code", "OK",
			}))
			gomega.Expect(fakeRequestDuration.ObserveCallCount()).To(gomega.Equal(1))
			gomega.Expect(fakeRequestDuration.ObserveArgsForCall(0)).NotTo(gomega.BeZero())
			gomega.Expect(fakeRequestDuration.ObserveArgsForCall(0)).To(gomega.BeNumerically("<", 1.0))
		})

		//nolint:dupl // 200-219 lines are duplicate of 222-241.
		ginkgo.It("records requests received before requests completed", func() {
			fakeRequestsReceived.AddStub = func(delta float64) {
				defer ginkgo.GinkgoRecover()
				gomega.Expect(fakeRequestsCompleted.AddCallCount()).To(gomega.Equal(0))
			}

			streamClient, err := echoServiceClient.EchoStream(context.Background())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			streamMessages(streamClient)

			gomega.Expect(fakeRequestsReceived.WithCallCount()).To(gomega.Equal(1))
			labelValues := fakeRequestDuration.WithArgsForCall(0)
			gomega.Expect(labelValues).To(gomega.Equal([]string{
				"service", "testpb_EchoService",
				"method", "EchoStream",
				"code", "OK",
			}))
			gomega.Expect(fakeRequestsReceived.AddCallCount()).To(gomega.Equal(1))
			gomega.Expect(fakeRequestsReceived.AddArgsForCall(0)).To(gomega.BeNumerically("~", 1.0))
		})

		//nolint:dupl // 200-219 lines are duplicate of 222-241.
		ginkgo.It("records requests completed after requests received", func() {
			fakeRequestsCompleted.AddStub = func(delta float64) {
				defer ginkgo.GinkgoRecover()
				gomega.Expect(fakeRequestsReceived.AddCallCount()).To(gomega.Equal(1))
			}

			streamClient, err := echoServiceClient.EchoStream(context.Background())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			streamMessages(streamClient)

			gomega.Expect(fakeRequestsReceived.WithCallCount()).To(gomega.Equal(1))
			labelValues := fakeRequestDuration.WithArgsForCall(0)
			gomega.Expect(labelValues).To(gomega.Equal([]string{
				"service", "testpb_EchoService",
				"method", "EchoStream",
				"code", "OK",
			}))
			gomega.Expect(fakeRequestsReceived.AddCallCount()).To(gomega.Equal(1))
			gomega.Expect(fakeRequestsReceived.AddArgsForCall(0)).To(gomega.BeNumerically("~", 1.0))
		})

		ginkgo.It("records messages sent", func() {
			streamClient, err := echoServiceClient.EchoStream(context.Background())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			streamMessages(streamClient)

			gomega.Expect(fakeMessagesSent.WithCallCount()).To(gomega.Equal(1))
			labelValues := fakeMessagesSent.WithArgsForCall(0)
			gomega.Expect(labelValues).To(gomega.Equal([]string{
				"service", "testpb_EchoService",
				"method", "EchoStream",
			}))

			gomega.Expect(fakeMessagesSent.AddCallCount()).To(gomega.Equal(2))
			for i := 0; i < fakeMessagesSent.AddCallCount(); i++ {
				gomega.Expect(fakeMessagesSent.AddArgsForCall(0)).To(gomega.BeNumerically("~", 1.0))
			}
		})

		ginkgo.It("records messages received", func() {
			streamClient, err := echoServiceClient.EchoStream(context.Background())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			streamMessages(streamClient)

			gomega.Expect(fakeMessagesReceived.WithCallCount()).To(gomega.Equal(1))
			labelValues := fakeMessagesReceived.WithArgsForCall(0)
			gomega.Expect(labelValues).To(gomega.Equal([]string{
				"service", "testpb_EchoService",
				"method", "EchoStream",
			}))

			gomega.Expect(fakeMessagesReceived.AddCallCount()).To(gomega.Equal(2))
			for i := 0; i < fakeMessagesReceived.AddCallCount(); i++ {
				gomega.Expect(fakeMessagesReceived.AddArgsForCall(0)).To(gomega.BeNumerically("~", 1.0))
			}
		})

		ginkgo.Context("when stream recv returns an error", func() {
			var errCh chan error

			ginkgo.BeforeEach(func() {
				errCh = make(chan error)
				fakeEchoService.EchoStreamStub = func(svs testpb.EchoService_EchoStreamServer) error {
					return <-errCh
				}
			})

			ginkgo.It("does not increment the update count", func() {
				streamClient, err := echoServiceClient.EchoStream(context.Background())
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				err = streamClient.Send(&testpb.Message{Message: "hello"})
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				errCh <- errors.New("oh bother")
				_, err = streamClient.Recv()
				gomega.Expect(err).To(gomega.MatchError(status.Errorf(codes.Unknown, "oh bother")))

				err = streamClient.CloseSend()
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				_, err = streamClient.Recv()
				gomega.Expect(err).To(gomega.MatchError(status.Errorf(codes.Unknown, "oh bother")))

				gomega.Expect(fakeMessagesReceived.AddCallCount()).To(gomega.Equal(0))
			})
		})
	})
})

func streamMessages(streamClient testpb.EchoService_EchoStreamClient) {
	err := streamClient.Send(&testpb.Message{Message: "hello"})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	err = streamClient.Send(&testpb.Message{Message: "hello", Sequence: 2})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	msg, err := streamClient.Recv()
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(msg).To(test.ProtoEqual(&testpb.Message{Message: "hello", Sequence: 1}))
	msg, err = streamClient.Recv()
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(msg).To(test.ProtoEqual(&testpb.Message{Message: "hello", Sequence: 3}))

	err = streamClient.CloseSend()
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	msg, err = streamClient.Recv()
	gomega.Expect(err).To(gomega.Equal(io.EOF))
	gomega.Expect(msg).To(gomega.BeNil())
}
