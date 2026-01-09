/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package configtxgen

import (
	"errors"
	"path"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	ab "github.com/hyperledger/fabric-protos-go-apiv2/orderer"
	"github.com/hyperledger/fabric-protos-go-apiv2/orderer/etcdraft"
	"github.com/hyperledger/fabric-protos-go-apiv2/orderer/smartbft"
	"google.golang.org/protobuf/proto"

	"github.com/hyperledger/fabric-x-common/api/types"
	"github.com/hyperledger/fabric-x-common/common/util"
	"github.com/hyperledger/fabric-x-common/protoutil"
	"github.com/hyperledger/fabric-x-common/tools/pkg/identity/mocks"
	"github.com/hyperledger/fabric-x-common/tools/test"
)

const (
	mspDir         = "../../sampleconfig/crypto/SampleOrg/msp"
	garbageRule    = "garbage"
	badOrdererType = "bad-type"
)

func CreateStandardPolicies() map[string]*Policy {
	return map[string]*Policy{
		"Admins": {
			Type: "ImplicitMeta",
			Rule: "ANY Admins",
		},
		"Readers": {
			Type: "ImplicitMeta",
			Rule: "ANY Readers",
		},
		"Writers": {
			Type: "ImplicitMeta",
			Rule: "ANY Writers",
		},
	}
}

func CreateStandardOrdererPolicies() map[string]*Policy {
	policies := CreateStandardPolicies()

	policies["BlockValidation"] = &Policy{
		Type: "ImplicitMeta",
		Rule: "ANY Admins",
	}

	return policies
}

var _ = ginkgo.Describe("Encoder", func() {
	ginkgo.Describe("AddOrdererPolicies", func() {
		var (
			cg       *cb.ConfigGroup
			policies map[string]*Policy
		)

		ginkgo.BeforeEach(func() {
			cg = protoutil.NewConfigGroup()
			policies = CreateStandardOrdererPolicies()
		})

		ginkgo.It("adds the block validation policy to the group", func() {
			err := addOrdererPolicies(cg, policies, "Admins")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cg.Policies).To(gomega.HaveLen(4))

			gomega.Expect(cg.Policies["BlockValidation"].Policy).To(gomega.Equal(&cb.Policy{
				Type: int32(cb.Policy_IMPLICIT_META),
				Value: protoutil.MarshalOrPanic(&cb.ImplicitMetaPolicy{
					SubPolicy: "Admins",
					Rule:      cb.ImplicitMetaPolicy_ANY,
				}),
			}))
		})

		ginkgo.Context("when the policy map is nil", func() {
			ginkgo.BeforeEach(func() {
				policies = nil
			})

			ginkgo.It("returns an error", func() {
				err := addOrdererPolicies(cg, policies, "Admins")
				gomega.Expect(err).To(gomega.MatchError("no policies defined"))
			})
		})

		ginkgo.Context("when the policy map is missing 'BlockValidation'", func() {
			ginkgo.BeforeEach(func() {
				delete(policies, "BlockValidation")
			})

			ginkgo.It("returns an error", func() {
				err := addOrdererPolicies(cg, policies, "Admins")
				gomega.Expect(err).To(gomega.MatchError("no BlockValidation policy defined"))
			})
		})
	})

	ginkgo.Describe("AddPolicies", func() {
		var (
			cg       *cb.ConfigGroup
			policies map[string]*Policy
		)

		ginkgo.BeforeEach(func() {
			cg = protoutil.NewConfigGroup()
			policies = CreateStandardPolicies()
		})

		ginkgo.It("adds the standard policies to the group", func() {
			err := addPolicies(cg, policies, "Admins")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cg.Policies).To(gomega.HaveLen(3))

			gomega.Expect(cg.Policies["Admins"].Policy).To(gomega.Equal(&cb.Policy{
				Type: int32(cb.Policy_IMPLICIT_META),
				Value: protoutil.MarshalOrPanic(&cb.ImplicitMetaPolicy{
					SubPolicy: "Admins",
					Rule:      cb.ImplicitMetaPolicy_ANY,
				}),
			}))

			gomega.Expect(cg.Policies["Readers"].Policy).To(gomega.Equal(&cb.Policy{
				Type: int32(cb.Policy_IMPLICIT_META),
				Value: protoutil.MarshalOrPanic(&cb.ImplicitMetaPolicy{
					SubPolicy: "Readers",
					Rule:      cb.ImplicitMetaPolicy_ANY,
				}),
			}))

			gomega.Expect(cg.Policies["Writers"].Policy).To(gomega.Equal(&cb.Policy{
				Type: int32(cb.Policy_IMPLICIT_META),
				Value: protoutil.MarshalOrPanic(&cb.ImplicitMetaPolicy{
					SubPolicy: "Writers",
					Rule:      cb.ImplicitMetaPolicy_ANY,
				}),
			}))
		})

		ginkgo.Context("when the policy map is nil", func() {
			ginkgo.BeforeEach(func() {
				policies = nil
			})

			ginkgo.It("returns an error", func() {
				err := addPolicies(cg, policies, "Admins")
				gomega.Expect(err).To(gomega.MatchError("no policies defined"))
			})
		})

		ginkgo.Context("when the policy map is missing 'Admins'", func() {
			ginkgo.BeforeEach(func() {
				delete(policies, "Admins")
			})

			ginkgo.It("returns an error", func() {
				err := addPolicies(cg, policies, "Admins")
				gomega.Expect(err).To(gomega.MatchError("no Admins policy defined"))
			})
		})

		ginkgo.Context("when the policy map is missing 'Readers'", func() {
			ginkgo.BeforeEach(func() {
				delete(policies, "Readers")
			})

			ginkgo.It("returns an error", func() {
				err := addPolicies(cg, policies, "Readers")
				gomega.Expect(err).To(gomega.MatchError("no Readers policy defined"))
			})
		})

		ginkgo.Context("when the policy map is missing 'Writers'", func() {
			ginkgo.BeforeEach(func() {
				delete(policies, "Writers")
			})

			ginkgo.It("returns an error", func() {
				err := addPolicies(cg, policies, "Writers")
				gomega.Expect(err).To(gomega.MatchError("no Writers policy defined"))
			})
		})

		ginkgo.Context("when the signature policy definition is bad", func() {
			ginkgo.BeforeEach(func() {
				policies["Readers"].Type = "Signature"
				policies["Readers"].Rule = garbageRule
			})

			ginkgo.It("wraps and returns the error", func() {
				err := addPolicies(cg, policies, "Readers")
				gomega.Expect(err).To(gomega.MatchError("invalid signature policy rule 'garbage': " +
					"unrecognized token 'garbage' in policy string"))
			})
		})

		ginkgo.Context("when the implicit policy definition is bad", func() {
			ginkgo.BeforeEach(func() {
				policies["Readers"].Type = "ImplicitMeta"
				policies["Readers"].Rule = garbageRule
			})

			ginkgo.It("wraps and returns the error", func() {
				err := addPolicies(cg, policies, "Readers")
				gomega.Expect(err).To(gomega.MatchError("invalid implicit meta policy rule 'garbage': " +
					"expected two space separated tokens, but got 1"))
			})
		})

		ginkgo.Context("when the policy type is unknown", func() {
			ginkgo.BeforeEach(func() {
				policies["Readers"].Type = garbageRule
			})

			ginkgo.It("returns an error", func() {
				err := addPolicies(cg, policies, "Readers")
				gomega.Expect(err).To(gomega.MatchError("unknown policy type: garbage"))
			})
		})
	})

	ginkgo.Describe("NewChannelGroup", func() {
		var conf *Profile

		ginkgo.BeforeEach(func() {
			conf = &Profile{
				Consortium: "MyConsortium",
				Policies:   CreateStandardPolicies(),
				Application: &Application{
					Policies: CreateStandardPolicies(),
				},
				Orderer: &Orderer{
					OrdererType: "solo",
					Policies:    CreateStandardOrdererPolicies(),
				},
				Consortiums: map[string]*Consortium{
					"SampleConsortium": {},
				},
				Capabilities: map[string]bool{
					"FakeCapability": true,
				},
			}
		})

		ginkgo.It("translates the config into a config group", func() {
			cg, err := NewChannelGroup(conf)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cg.Values).To(gomega.HaveLen(4))
			gomega.Expect(cg.Values["BlockDataHashingStructure"]).NotTo(gomega.BeNil())
			gomega.Expect(cg.Values["Consortium"]).NotTo(gomega.BeNil())
			gomega.Expect(cg.Values["Capabilities"]).NotTo(gomega.BeNil())
			gomega.Expect(cg.Values["HashingAlgorithm"]).NotTo(gomega.BeNil())
			gomega.Expect(cg.Values["OrdererAddresses"]).To(gomega.BeNil())
		})

		ginkgo.Context("when the policy definition is bad", func() {
			ginkgo.BeforeEach(func() {
				conf.Policies["Admins"].Rule = garbageRule
			})

			ginkgo.It("wraps and returns the error", func() {
				_, err := NewChannelGroup(conf)
				gomega.Expect(err).To(gomega.MatchError("error adding policies to channel group: " +
					"invalid implicit meta policy rule 'garbage': expected two space separated tokens, but got 1"))
			})
		})

		ginkgo.Context("when the orderer addresses are omitted", func() {
			ginkgo.BeforeEach(func() {
				conf.Orderer.Addresses = []string{}
			})

			ginkgo.It("does not create the config value", func() {
				cg, err := NewChannelGroup(conf)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(cg.Values["OrdererAddresses"]).To(gomega.BeNil())
			})
		})

		ginkgo.Context("when the orderer config is bad", func() {
			ginkgo.BeforeEach(func() {
				conf.Orderer.OrdererType = badOrdererType
			})

			ginkgo.It("wraps and returns the error", func() {
				_, err := NewChannelGroup(conf)
				gomega.Expect(err).To(gomega.MatchError("could not create orderer group: " +
					"unknown orderer type: bad-type"))
			})

			ginkgo.Context("when the orderer config is missing", func() {
				ginkgo.BeforeEach(func() {
					conf.Orderer = nil
				})

				ginkgo.It("handles it gracefully", func() {
					_, err := NewChannelGroup(conf)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
				})
			})
		})

		ginkgo.Context("when the application config is bad", func() {
			ginkgo.BeforeEach(func() {
				conf.Application.Policies["Admins"] = &Policy{
					Type: garbageRule,
				}
			})

			ginkgo.It("wraps and returns the error", func() {
				_, err := NewChannelGroup(conf)
				gomega.Expect(err).To(gomega.MatchError("could not create application group: " +
					"error adding policies to application group: unknown policy type: garbage"))
			})
		})

		ginkgo.Context("when the consortium config is bad", func() {
			ginkgo.BeforeEach(func() {
				conf.Consortiums["SampleConsortium"].Organizations = []*Organization{
					{
						Policies: map[string]*Policy{
							"garbage-policy": {
								Type: garbageRule,
							},
						},
					},
				}
			})

			ginkgo.It("wraps and returns the error", func() {
				_, err := NewChannelGroup(conf)
				gomega.Expect(err).To(gomega.MatchError("could not create consortiums group: " +
					"failed to create consortium SampleConsortium: failed to create consortium org: 1 - " +
					"Error loading MSP configuration for org: : unknown MSP type ''"))
			})
		})
	})

	ginkgo.Describe("NewOrdererGroup", func() {
		var conf *Orderer
		var channelCapabilities map[string]bool

		ginkgo.BeforeEach(func() {
			conf = &Orderer{
				OrdererType: "solo",
				Organizations: []*Organization{
					{
						MSPDir:   mspDir,
						ID:       "SampleMSP",
						MSPType:  "bccsp",
						Name:     "SampleOrg",
						Policies: CreateStandardPolicies(),
						OrdererEndpoints: []*types.OrdererEndpoint{
							{Host: "foo", Port: 7050},
							{Host: "bar", Port: 8050},
						},
					},
				},
				Policies: CreateStandardOrdererPolicies(),
				Capabilities: map[string]bool{
					"FakeCapability": true,
				},
			}
			channelCapabilities = map[string]bool{
				"V3_0": true,
			}
		})

		ginkgo.It("translates the config into a config group", func() {
			cg, err := NewOrdererGroup(conf, channelCapabilities)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cg.Policies).To(gomega.HaveLen(4)) // BlockValidation automatically added
			gomega.Expect(cg.Policies["Admins"]).NotTo(gomega.BeNil())
			gomega.Expect(cg.Policies["Readers"]).NotTo(gomega.BeNil())
			gomega.Expect(cg.Policies["Writers"]).NotTo(gomega.BeNil())
			gomega.Expect(cg.Policies["BlockValidation"]).NotTo(gomega.BeNil())
			gomega.Expect(cg.Groups).To(gomega.HaveLen(1))
			gomega.Expect(cg.Groups["SampleOrg"]).NotTo(gomega.BeNil())
			gomega.Expect(cg.Values).To(gomega.HaveLen(5))
			gomega.Expect(cg.Values["BatchSize"]).NotTo(gomega.BeNil())
			gomega.Expect(cg.Values["BatchTimeout"]).NotTo(gomega.BeNil())
			gomega.Expect(cg.Values["ChannelRestrictions"]).NotTo(gomega.BeNil())
			gomega.Expect(cg.Values["Capabilities"]).NotTo(gomega.BeNil())
		})

		ginkgo.Context("when the policy definition is bad", func() {
			ginkgo.BeforeEach(func() {
				conf.Policies["Admins"].Rule = garbageRule
			})

			ginkgo.It("wraps and returns the error", func() {
				_, err := NewOrdererGroup(conf, channelCapabilities)
				gomega.Expect(err).To(gomega.MatchError("error adding policies to orderer group: " +
					"invalid implicit meta policy rule 'garbage': expected two space separated tokens, but got 1"))
			})
		})

		ginkgo.Context("when the consensus type is etcd/raft", func() {
			ginkgo.BeforeEach(func() {
				conf.OrdererType = "etcdraft"
				conf.EtcdRaft = &etcdraft.ConfigMetadata{
					Options: &etcdraft.Options{
						TickInterval: "500ms",
					},
				}
			})

			ginkgo.It("adds the raft metadata", func() {
				cg, err := NewOrdererGroup(conf, channelCapabilities)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(cg.Values).To(gomega.HaveLen(5))
				consensusType := &ab.ConsensusType{}
				err = proto.Unmarshal(cg.Values["ConsensusType"].Value, consensusType)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(consensusType.Type).To(gomega.Equal("etcdraft"))
				metadata := &etcdraft.ConfigMetadata{}
				err = proto.Unmarshal(consensusType.Metadata, metadata)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(metadata.Options.TickInterval).To(gomega.Equal("500ms"))
			})

			ginkgo.Context("when the raft configuration is bad", func() {
				ginkgo.BeforeEach(func() {
					conf.EtcdRaft = &etcdraft.ConfigMetadata{
						Consenters: []*etcdraft.Consenter{
							{},
						},
					}
				})

				ginkgo.It("wraps and returns the error", func() {
					_, err := NewOrdererGroup(conf, channelCapabilities)
					gomega.Expect(err).To(gomega.MatchError("cannot marshal metadata for orderer type " +
						"etcdraft: cannot load client cert for consenter :0: open : no such file or directory"))
				})
			})
		})

		ginkgo.Context("when the consensus type is BFT", func() {
			ginkgo.BeforeEach(func() {
				conf.OrdererType = "BFT"
				conf.ConsenterMapping = []*Consenter{
					{
						ID:    1,
						Host:  "host1",
						Port:  1001,
						MSPID: "MSP1",
					},
					{
						ID:            2,
						Host:          "host2",
						Port:          1002,
						MSPID:         "MSP2",
						ClientTLSCert: path.Join(mspDir, "admincerts/admincert.pem"),
						ServerTLSCert: path.Join(mspDir, "admincerts/admincert.pem"),
						Identity:      path.Join(mspDir, "admincerts/admincert.pem"),
					},
				}
				conf.SmartBFT = &smartbft.Options{}
			})

			ginkgo.It("adds the Orderers key", func() {
				cg, err := NewOrdererGroup(conf, channelCapabilities)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(cg.Values).To(gomega.HaveLen(6))
				gomega.Expect(cg.Values["Orderers"]).NotTo(gomega.BeNil())
				orderersType := &cb.Orderers{}
				err = proto.Unmarshal(cg.Values["Orderers"].Value, orderersType)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(orderersType.ConsenterMapping).To(gomega.HaveLen(2))
				consenter1 := orderersType.ConsenterMapping[0]
				gomega.Expect(consenter1.Id).To(gomega.Equal(uint32(1)))
				gomega.Expect(consenter1.ClientTlsCert).To(gomega.BeNil())
				consenter2 := orderersType.ConsenterMapping[1]
				gomega.Expect(consenter2.Id).To(gomega.Equal(uint32(2)))
				gomega.Expect(consenter2.ClientTlsCert).ToNot(gomega.BeNil())
			})

			ginkgo.It("requires V3_0", func() {
				delete(channelCapabilities, "V3_0")
				channelCapabilities["V2_0"] = true
				_, err := NewOrdererGroup(conf, channelCapabilities)
				gomega.Expect(err).To(gomega.MatchError("orderer type BFT must be used with V3_0 channel " +
					"capability: map[V2_0:true]"))
			})
		})

		ginkgo.Context("when the consensus type is unknown", func() {
			ginkgo.BeforeEach(func() {
				conf.OrdererType = badOrdererType
			})

			ginkgo.It("returns an error", func() {
				_, err := NewOrdererGroup(conf, channelCapabilities)
				gomega.Expect(err).To(gomega.MatchError("unknown orderer type: bad-type"))
			})
		})

		ginkgo.Context("when the org definition is bad", func() {
			ginkgo.BeforeEach(func() {
				conf.Organizations[0].MSPType = garbageRule
			})

			ginkgo.It("wraps and returns the error", func() {
				_, err := NewOrdererGroup(conf, channelCapabilities)
				gomega.Expect(err).To(gomega.MatchError("failed to create orderer org: 1 - " +
					"Error loading MSP configuration for org: SampleOrg: unknown MSP type 'garbage'"))
			})
		})

		ginkgo.Context("when global endpoints exist", func() {
			ginkgo.BeforeEach(func() {
				conf.Addresses = []string{"addr1", "addr2"}
			})

			ginkgo.It("wraps and returns the error", func() {
				_, err := NewOrdererGroup(conf, channelCapabilities)
				gomega.Expect(err).To(gomega.MatchError("global orderer endpoints exist, " +
					"but can not be used with V3_0 capability: [addr1 addr2]"))
			})

			ginkgo.It("is permitted when V3_0 is false", func() {
				delete(channelCapabilities, "V3_0")
				channelCapabilities["V2_0"] = true
				_, err := NewOrdererGroup(conf, channelCapabilities)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
			})
		})
	})

	ginkgo.Describe("NewApplicationGroup", func() {
		var conf *Application

		ginkgo.BeforeEach(func() {
			conf = &Application{
				Organizations: []*Organization{
					{
						MSPDir:   mspDir,
						ID:       "SampleMSP",
						MSPType:  "bccsp",
						Name:     "SampleOrg",
						Policies: CreateStandardPolicies(),
					},
				},
				ACLs: map[string]string{
					"SomeACL": "SomePolicy",
				},
				Policies: CreateStandardPolicies(),
				Capabilities: map[string]bool{
					"FakeCapability": true,
				},
			}
		})

		ginkgo.It("translates the config into a config group", func() {
			cg, err := NewApplicationGroup(conf)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cg.Policies).To(gomega.HaveLen(3))
			gomega.Expect(cg.Policies["Admins"]).NotTo(gomega.BeNil())
			gomega.Expect(cg.Policies["Readers"]).NotTo(gomega.BeNil())
			gomega.Expect(cg.Policies["Writers"]).NotTo(gomega.BeNil())
			gomega.Expect(cg.Groups).To(gomega.HaveLen(1))
			gomega.Expect(cg.Groups["SampleOrg"]).NotTo(gomega.BeNil())
			gomega.Expect(cg.Values).To(gomega.HaveLen(2))
			gomega.Expect(cg.Values["ACLs"]).NotTo(gomega.BeNil())
			gomega.Expect(cg.Values["Capabilities"]).NotTo(gomega.BeNil())
		})

		ginkgo.Context("when the policy definition is bad", func() {
			ginkgo.BeforeEach(func() {
				conf.Policies["Admins"].Rule = garbageRule
			})

			ginkgo.It("wraps and returns the error", func() {
				_, err := NewApplicationGroup(conf)
				gomega.Expect(err).To(gomega.MatchError("error adding policies to application group: " +
					"invalid implicit meta policy rule 'garbage': expected two space separated tokens, but got 1"))
			})
		})

		ginkgo.Context("when the org definition is bad", func() {
			ginkgo.BeforeEach(func() {
				conf.Organizations[0].MSPType = garbageRule
			})

			ginkgo.It("wraps and returns the error", func() {
				_, err := NewApplicationGroup(conf)
				gomega.Expect(err).To(gomega.MatchError("failed to create application org: 1 - " +
					"Error loading MSP configuration for org SampleOrg: unknown MSP type 'garbage'"))
			})
		})
	})

	ginkgo.Describe("NewConsortiumOrgGroup", func() {
		var conf *Organization

		ginkgo.BeforeEach(func() {
			conf = &Organization{
				MSPDir:   mspDir,
				ID:       "SampleMSP",
				MSPType:  "bccsp",
				Name:     "SampleOrg",
				Policies: CreateStandardPolicies(),
			}
		})

		ginkgo.It("translates the config into a config group", func() {
			cg, err := NewConsortiumOrgGroup(conf)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cg.Values).To(gomega.HaveLen(1))
			gomega.Expect(cg.Values["MSP"]).NotTo(gomega.BeNil())
			gomega.Expect(cg.Policies).To(gomega.HaveLen(3))
			gomega.Expect(cg.Policies["Admins"]).NotTo(gomega.BeNil())
			gomega.Expect(cg.Policies["Readers"]).NotTo(gomega.BeNil())
			gomega.Expect(cg.Policies["Writers"]).NotTo(gomega.BeNil())
		})

		ginkgo.Context("when the org is marked to be skipped as foreign", func() {
			ginkgo.BeforeEach(func() {
				conf.SkipAsForeign = true
			})

			ginkgo.It("returns an empty org group with mod policy set", func() {
				cg, err := NewConsortiumOrgGroup(conf)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(cg.Values).To(gomega.BeEmpty())
				gomega.Expect(cg.Policies).To(gomega.BeEmpty())
			})

			ginkgo.Context("even when the MSP dir is invalid/corrupt", func() {
				ginkgo.BeforeEach(func() {
					conf.MSPDir = garbageRule
				})

				ginkgo.It("returns without error", func() {
					_, err := NewConsortiumOrgGroup(conf)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
				})
			})
		})

		ginkgo.Context("when dev mode is enabled", func() {
			ginkgo.BeforeEach(func() {
				conf.AdminPrincipal = "Member"
			})

			ginkgo.It("does not produce an error", func() {
				_, err := NewConsortiumOrgGroup(conf)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})
		})

		ginkgo.Context("when the policy definition is bad", func() {
			ginkgo.BeforeEach(func() {
				conf.Policies["Admins"].Rule = garbageRule
			})

			ginkgo.It("wraps and returns the error", func() {
				_, err := NewConsortiumOrgGroup(conf)
				gomega.Expect(err).To(gomega.MatchError("error adding policies to consortiums org group " +
					"'SampleOrg': invalid implicit meta policy rule 'garbage': " +
					"expected two space separated tokens, but got 1"))
			})
		})
	})

	ginkgo.Describe("NewOrdererOrgGroup", func() {
		var conf *Organization

		ginkgo.BeforeEach(func() {
			conf = &Organization{
				MSPDir:   mspDir,
				ID:       "SampleMSP",
				MSPType:  "bccsp",
				Name:     "SampleOrg",
				Policies: CreateStandardPolicies(),
				OrdererEndpoints: []*types.OrdererEndpoint{
					{Host: "foo", Port: 7050},
					{Host: "bar", Port: 8050},
				},
			}
		})

		ginkgo.It("translates the config into a config group", func() {
			cg, err := NewOrdererOrgGroup(conf, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cg.Values).To(gomega.HaveLen(2))
			gomega.Expect(cg.Values["MSP"]).NotTo(gomega.BeNil())
			gomega.Expect(cg.Policies).To(gomega.HaveLen(3))
			gomega.Expect(cg.Values["Endpoints"]).NotTo(gomega.BeNil())
			gomega.Expect(cg.Policies["Admins"]).NotTo(gomega.BeNil())
			gomega.Expect(cg.Policies["Readers"]).NotTo(gomega.BeNil())
			gomega.Expect(cg.Policies["Writers"]).NotTo(gomega.BeNil())
		})

		ginkgo.Context("when the org is marked to be skipped as foreign", func() {
			ginkgo.BeforeEach(func() {
				conf.SkipAsForeign = true
			})

			ginkgo.It("returns an empty org group with mod policy set", func() {
				cg, err := NewOrdererOrgGroup(conf, nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(cg.Values).To(gomega.BeEmpty())
				gomega.Expect(cg.Policies).To(gomega.BeEmpty())
			})

			ginkgo.Context("even when the MSP dir is invalid/corrupt", func() {
				ginkgo.BeforeEach(func() {
					conf.MSPDir = garbageRule
				})

				ginkgo.It("returns without error", func() {
					_, err := NewOrdererOrgGroup(conf, nil)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
				})
			})
		})

		ginkgo.Context("when there are no ordering endpoints", func() {
			ginkgo.BeforeEach(func() {
				conf.OrdererEndpoints = []*types.OrdererEndpoint{}
			})

			ginkgo.It("does not include the endpoints in the config group with v2_0", func() {
				channelCapabilities := map[string]bool{"V2_0": true}
				cg, err := NewOrdererOrgGroup(conf, channelCapabilities)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(cg.Values["Endpoints"]).To(gomega.BeNil())
			})

			ginkgo.It("emits an error with v3_0", func() {
				channelCapabilities := map[string]bool{"V3_0": true}
				cg, err := NewOrdererOrgGroup(conf, channelCapabilities)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(cg).To(gomega.BeNil())
			})
		})

		ginkgo.Context("when dev mode is enabled", func() {
			ginkgo.BeforeEach(func() {
				conf.AdminPrincipal = "Member"
			})

			ginkgo.It("does not produce an error", func() {
				_, err := NewOrdererOrgGroup(conf, nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})
		})

		ginkgo.Context("when the policy definition is bad", func() {
			ginkgo.BeforeEach(func() {
				conf.Policies["Admins"].Rule = garbageRule
			})

			ginkgo.It("wraps and returns the error", func() {
				_, err := NewOrdererOrgGroup(conf, nil)
				gomega.Expect(err).To(gomega.MatchError("error adding policies to orderer org group " +
					"'SampleOrg': invalid implicit meta policy rule 'garbage': " +
					"expected two space separated tokens, but got 1"))
			})
		})
	})

	ginkgo.Describe("NewApplicationOrgGroup", func() {
		var conf *Organization

		ginkgo.BeforeEach(func() {
			conf = &Organization{
				MSPDir:   mspDir,
				ID:       "SampleMSP",
				MSPType:  "bccsp",
				Name:     "SampleOrg",
				Policies: CreateStandardPolicies(),
				AnchorPeers: []*AnchorPeer{
					{
						Host: "hostname",
						Port: 5555,
					},
				},
			}
		})

		ginkgo.It("translates the config into a config group", func() {
			cg, err := NewApplicationOrgGroup(conf)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cg.Values).To(gomega.HaveLen(2))
			gomega.Expect(cg.Values["MSP"]).NotTo(gomega.BeNil())
			gomega.Expect(cg.Values["AnchorPeers"]).NotTo(gomega.BeNil())
			gomega.Expect(cg.Policies).To(gomega.HaveLen(3))
			gomega.Expect(cg.Policies["Admins"]).NotTo(gomega.BeNil())
			gomega.Expect(cg.Policies["Readers"]).NotTo(gomega.BeNil())
			gomega.Expect(cg.Policies["Writers"]).NotTo(gomega.BeNil())
			gomega.Expect(cg.Values).To(gomega.HaveLen(2))
			gomega.Expect(cg.Values["MSP"]).NotTo(gomega.BeNil())
			gomega.Expect(cg.Values["AnchorPeers"]).NotTo(gomega.BeNil())
		})

		ginkgo.Context("when the org is marked to be skipped as foreign", func() {
			ginkgo.BeforeEach(func() {
				conf.SkipAsForeign = true
			})

			ginkgo.It("returns an empty org group with mod policy set", func() {
				cg, err := NewApplicationOrgGroup(conf)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(cg.Values).To(gomega.BeEmpty())
				gomega.Expect(cg.Policies).To(gomega.BeEmpty())
			})

			ginkgo.Context("even when the MSP dir is invalid/corrupt", func() {
				ginkgo.BeforeEach(func() {
					conf.MSPDir = garbageRule
				})

				ginkgo.It("returns without error", func() {
					_, err := NewApplicationOrgGroup(conf)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
				})
			})
		})

		ginkgo.Context("when the policy definition is bad", func() {
			ginkgo.BeforeEach(func() {
				conf.Policies["Admins"].Type = garbageRule
			})

			ginkgo.It("wraps and returns the error", func() {
				_, err := NewApplicationOrgGroup(conf)
				gomega.Expect(err).To(gomega.MatchError("error adding policies to application org group " +
					"SampleOrg: unknown policy type: garbage"))
			})
		})

		ginkgo.Context("when the MSP definition is bad", func() {
			ginkgo.BeforeEach(func() {
				conf.MSPDir = garbageRule
			})

			ginkgo.It("wraps and returns the error", func() {
				_, err := NewApplicationOrgGroup(conf)
				gomega.Expect(err).To(gomega.MatchError("1 - Error loading MSP configuration for org " +
					"SampleOrg: could not load a valid ca certificate from directory garbage/cacerts: " +
					"stat garbage/cacerts: no such file or directory"))
			})
		})

		ginkgo.Context("when there are no anchor peers defined", func() {
			ginkgo.BeforeEach(func() {
				conf.AnchorPeers = nil
			})

			ginkgo.It("does not encode the anchor peers", func() {
				cg, err := NewApplicationOrgGroup(conf)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(cg.Values).To(gomega.HaveLen(1))
				gomega.Expect(cg.Values["AnchorPeers"]).To(gomega.BeNil())
			})
		})
	})

	ginkgo.Describe("ChannelCreationOperations", func() {
		var (
			conf     *Profile
			template *cb.ConfigGroup
		)

		ginkgo.BeforeEach(func() {
			conf = &Profile{
				Consortium: "MyConsortium",
				Policies:   CreateStandardPolicies(),
				Application: &Application{
					Organizations: []*Organization{
						{
							Name:     "SampleOrg",
							MSPDir:   mspDir,
							ID:       "SampleMSP",
							MSPType:  "bccsp",
							Policies: CreateStandardPolicies(),
							AnchorPeers: []*AnchorPeer{
								{
									Host: "hostname",
									Port: 4444,
								},
							},
						},
					},
					Policies: CreateStandardPolicies(),
				},
			}

			var err error
			template, err = DefaultConfigTemplate(conf)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.Describe("NewChannelCreateConfigUpdate", func() {
			ginkgo.It("translates the config into a config group", func() {
				cg, err := NewChannelCreateConfigUpdate("channel-id", conf, template)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				expected := &cb.ConfigUpdate{
					ChannelId: "channel-id",
					ReadSet: &cb.ConfigGroup{
						Groups: map[string]*cb.ConfigGroup{
							"Application": {
								Groups: map[string]*cb.ConfigGroup{
									"SampleOrg": {},
								},
							},
						},
						Values: map[string]*cb.ConfigValue{
							"Consortium": {},
						},
					},
					WriteSet: &cb.ConfigGroup{
						Groups: map[string]*cb.ConfigGroup{
							"Application": {
								Version:   1,
								ModPolicy: "Admins",
								Groups: map[string]*cb.ConfigGroup{
									"SampleOrg": {},
								},
								Policies: map[string]*cb.ConfigPolicy{
									"Admins": {
										Policy: &cb.Policy{
											Type: int32(cb.Policy_IMPLICIT_META),
											Value: protoutil.MarshalOrPanic(&cb.ImplicitMetaPolicy{
												SubPolicy: "Admins",
												Rule:      cb.ImplicitMetaPolicy_ANY,
											}),
										},
										ModPolicy: "Admins",
									},
									"Readers": {
										Policy: &cb.Policy{
											Type: int32(cb.Policy_IMPLICIT_META),
											Value: protoutil.MarshalOrPanic(&cb.ImplicitMetaPolicy{
												SubPolicy: "Readers",
												Rule:      cb.ImplicitMetaPolicy_ANY,
											}),
										},
										ModPolicy: "Admins",
									},
									"Writers": {
										Policy: &cb.Policy{
											Type: int32(cb.Policy_IMPLICIT_META),
											Value: protoutil.MarshalOrPanic(&cb.ImplicitMetaPolicy{
												SubPolicy: "Writers",
												Rule:      cb.ImplicitMetaPolicy_ANY,
											}),
										},
										ModPolicy: "Admins",
									},
								},
							},
						},
						Values: map[string]*cb.ConfigValue{
							"Consortium": {
								Value: protoutil.MarshalOrPanic(&cb.Consortium{
									Name: "MyConsortium",
								}),
							},
						},
					},
				}
				gomega.Expect(expected).To(test.ProtoEqual(cg))
			})

			ginkgo.Context("when the template configuration is not the default", func() {
				ginkgo.BeforeEach(func() {
					differentConf := &Profile{
						Consortium: "MyConsortium",
						Policies:   CreateStandardPolicies(),
						Application: &Application{
							Organizations: []*Organization{
								{
									MSPDir:  mspDir,
									ID:      "SampleMSP",
									MSPType: "bccsp",
									Name:    "SampleOrg",
									AnchorPeers: []*AnchorPeer{
										{
											Host: "hostname",
											Port: 5555,
										},
									},
									Policies: CreateStandardPolicies(),
								},
							},
							Policies: CreateStandardPolicies(),
						},
					}

					var err error
					template, err = DefaultConfigTemplate(differentConf)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
				})

				ginkgo.It(
					"reflects the additional modifications designated by the channel creation profile",
					func() {
						cg, err := NewChannelCreateConfigUpdate("channel-id", conf, template)
						gomega.Expect(err).NotTo(gomega.HaveOccurred())
						gomega.Expect(
							cg.WriteSet.Groups["Application"].Groups["SampleOrg"].Values["AnchorPeers"].Version,
						).To(gomega.Equal(uint64(1)))
					},
				)
			})

			ginkgo.Context("when the application config is bad", func() {
				ginkgo.BeforeEach(func() {
					conf.Application.Policies["Admins"].Type = badOrdererType
				})

				ginkgo.It("returns an error", func() {
					_, err := NewChannelCreateConfigUpdate("channel-id", conf, template)
					gomega.Expect(err).To(gomega.MatchError("could not turn parse profile into channel " +
						"group: could not create application group: error adding policies to application group: " +
						"unknown policy type: bad-type"))
				})

				ginkgo.Context("when the application config is missing", func() {
					ginkgo.BeforeEach(func() {
						conf.Application = nil
					})

					ginkgo.It("returns an error", func() {
						_, err := NewChannelCreateConfigUpdate("channel-id", conf, template)
						gomega.Expect(err).To(gomega.MatchError("cannot define a new channel with no " +
							"Application section"))
					})
				})
			})

			ginkgo.Context("when the consortium is empty", func() {
				ginkgo.BeforeEach(func() {
					conf.Consortium = ""
				})

				ginkgo.It("returns an error", func() {
					_, err := NewChannelCreateConfigUpdate("channel-id", conf, template)
					gomega.Expect(err).To(gomega.MatchError("cannot define a new channel with no " +
						"Consortium value"))
				})
			})

			ginkgo.Context("when an update cannot be computed", func() {
				ginkgo.It("returns an error", func() {
					_, err := NewChannelCreateConfigUpdate("channel-id", conf, nil)
					gomega.Expect(err).To(gomega.MatchError("could not compute update: no channel " +
						"group included for original config"))
				})
			})
		})

		ginkgo.Describe("MakeChannelCreationTransaction", func() {
			var fakeSigner *mocks.SignerSerializer

			ginkgo.BeforeEach(func() {
				fakeSigner = &mocks.SignerSerializer{}
				fakeSigner.SerializeReturns([]byte("fake-creator"), nil)
			})

			ginkgo.It("returns an encoded and signed tx", func() {
				env, err := MakeChannelCreationTransaction("channel-id", fakeSigner, conf)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				payload := &cb.Payload{}
				err = proto.Unmarshal(env.Payload, payload)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				configUpdateEnv := &cb.ConfigUpdateEnvelope{}
				err = proto.Unmarshal(payload.Data, configUpdateEnv)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(configUpdateEnv.Signatures).To(gomega.HaveLen(1))
				gomega.Expect(fakeSigner.SerializeCallCount()).To(gomega.Equal(2))
				gomega.Expect(fakeSigner.SignCallCount()).To(gomega.Equal(2))
				gomega.Expect(fakeSigner.SignArgsForCall(0)).To(gomega.Equal(
					util.ConcatenateBytes(configUpdateEnv.Signatures[0].SignatureHeader, configUpdateEnv.ConfigUpdate),
				))
			})

			ginkgo.Context("when a default config cannot be generated", func() {
				ginkgo.BeforeEach(func() {
					conf.Application = nil
				})

				ginkgo.It("wraps and returns the error", func() {
					_, err := MakeChannelCreationTransaction("channel-id", fakeSigner, conf)
					gomega.Expect(err).To(gomega.MatchError("could not generate default config template: " +
						"channel template configs must contain an application section"))
				})
			})

			ginkgo.Context("when the signer cannot create the signature header", func() {
				ginkgo.BeforeEach(func() {
					fakeSigner.SerializeReturns(nil, errors.New("serialize-error"))
				})

				ginkgo.It("wraps and returns the error", func() {
					_, err := MakeChannelCreationTransaction("channel-id", fakeSigner, conf)
					gomega.Expect(err).To(gomega.MatchError("creating signature header failed: " +
						"serialize-error"))
				})
			})

			ginkgo.Context("when the signer cannot sign", func() {
				ginkgo.BeforeEach(func() {
					fakeSigner.SignReturns(nil, errors.New("sign-error"))
				})

				ginkgo.It("wraps and returns the error", func() {
					_, err := MakeChannelCreationTransaction("channel-id", fakeSigner, conf)
					gomega.Expect(err).To(gomega.MatchError("signature failure over config update: " +
						"sign-error"))
				})
			})

			ginkgo.Context("when no signer is provided", func() {
				ginkgo.It("returns an encoded tx with no signature", func() {
					_, err := MakeChannelCreationTransaction("channel-id", nil, conf)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
				})
			})

			ginkgo.Context("when the config is bad", func() {
				ginkgo.BeforeEach(func() {
					conf.Consortium = ""
				})

				ginkgo.It("wraps and returns the error", func() {
					_, err := MakeChannelCreationTransaction("channel-id", nil, conf)
					gomega.Expect(err).To(gomega.MatchError("config update generation failure: " +
						"cannot define a new channel with no Consortium value"))
				})
			})
		})

		ginkgo.Describe("MakeChannelCreationTransactionWithSystemChannelContext", func() {
			var (
				applicationConf *Profile
				sysChannelConf  *Profile
			)

			ginkgo.BeforeEach(func() {
				applicationConf = &Profile{
					Consortium: "SampleConsortium",
					Policies:   CreateStandardPolicies(),
					Orderer: &Orderer{
						OrdererType: "solo",
						Policies:    CreateStandardOrdererPolicies(),
					},
					Application: &Application{
						Organizations: []*Organization{
							{
								MSPDir:  mspDir,
								ID:      "Org1MSP",
								MSPType: "bccsp",
								Name:    "Org1",
								AnchorPeers: []*AnchorPeer{
									{
										Host: "my-peer",
										Port: 5555,
									},
								},
								Policies: CreateStandardPolicies(),
							},
							{
								MSPDir:   mspDir,
								ID:       "Org2MSP",
								MSPType:  "bccsp",
								Name:     "Org2",
								Policies: CreateStandardPolicies(),
							},
						},
						Policies: CreateStandardPolicies(),
					},
				}

				sysChannelConf = &Profile{
					Policies: CreateStandardPolicies(),
					Orderer: &Orderer{
						OrdererType:  "solo",
						BatchTimeout: time.Hour,
						Policies:     CreateStandardOrdererPolicies(),
					},
					Consortiums: map[string]*Consortium{
						"SampleConsortium": {
							Organizations: []*Organization{
								{
									MSPDir:   mspDir,
									ID:       "Org1MSP",
									MSPType:  "bccsp",
									Name:     "Org1",
									Policies: CreateStandardPolicies(),
								},
								{
									MSPDir:   mspDir,
									ID:       "Org2MSP",
									MSPType:  "bccsp",
									Name:     "Org2",
									Policies: CreateStandardPolicies(),
								},
							},
						},
					},
				}
			})

			ginkgo.It("returns an encoded and signed tx including differences from the system channel", func() {
				env, err := MakeChannelCreationTransactionWithSystemChannelContext(
					"channel-id", nil, applicationConf, sysChannelConf,
				)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				payload := &cb.Payload{}
				err = proto.Unmarshal(env.Payload, payload)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				configUpdateEnv := &cb.ConfigUpdateEnvelope{}
				err = proto.Unmarshal(payload.Data, configUpdateEnv)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				configUpdate := &cb.ConfigUpdate{}
				err = proto.Unmarshal(configUpdateEnv.ConfigUpdate, configUpdate)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				ws := configUpdate.WriteSet
				gomega.Expect(ws.Version).To(gomega.Equal(uint64(0)))
				gomega.Expect(ws.Groups["Application"].Policies["Admins"].Version).To(gomega.Equal(uint64(1)))
				gomega.Expect(ws.Groups["Application"].Groups["Org1"].Version).To(gomega.Equal(uint64(1)))
				gomega.Expect(ws.Groups["Application"].Groups["Org1"].Values["AnchorPeers"]).NotTo(gomega.BeNil())
				gomega.Expect(ws.Groups["Application"].Groups["Org2"].Version).To(gomega.Equal(uint64(0)))
				gomega.Expect(ws.Groups["Orderer"].Values["BatchTimeout"].Version).To(gomega.Equal(uint64(1)))
			})

			ginkgo.Context("when the system channel config is bad", func() {
				ginkgo.BeforeEach(func() {
					sysChannelConf.Orderer.OrdererType = garbageRule
				})

				ginkgo.It("wraps and returns the error", func() {
					_, err := MakeChannelCreationTransactionWithSystemChannelContext(
						"channel-id", nil, applicationConf, sysChannelConf,
					)
					gomega.Expect(err).To(gomega.MatchError("could not parse system channel config: " +
						"could not create orderer group: unknown orderer type: garbage"))
				})
			})

			ginkgo.Context("when the template cannot be computed", func() {
				ginkgo.BeforeEach(func() {
					applicationConf.Application = nil
				})

				ginkgo.It("wraps and returns the error", func() {
					_, err := MakeChannelCreationTransactionWithSystemChannelContext(
						"channel-id", nil, applicationConf, sysChannelConf,
					)
					gomega.Expect(err).To(gomega.MatchError("could not create config template: " +
						"supplied channel creation profile does not contain an application section"))
				})
			})
		})

		ginkgo.Describe("DefaultConfigTemplate", func() {
			var conf *Profile

			ginkgo.BeforeEach(func() {
				conf = &Profile{
					Policies: CreateStandardPolicies(),
					Orderer: &Orderer{
						OrdererType: "solo",
						Policies:    CreateStandardOrdererPolicies(),
					},
					Application: &Application{
						Policies: CreateStandardPolicies(),
						Organizations: []*Organization{
							{
								Name:          "Org1",
								SkipAsForeign: true,
							},
							{
								Name:          "Org2",
								SkipAsForeign: true,
							},
						},
					},
				}
			})

			ginkgo.It("returns the default config template", func() {
				cg, err := DefaultConfigTemplate(conf)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(cg.Groups).To(gomega.HaveLen(2))
				gomega.Expect(cg.Groups["Orderer"]).NotTo(gomega.BeNil())
				gomega.Expect(cg.Groups["Application"]).NotTo(gomega.BeNil())
				gomega.Expect(cg.Groups["Application"].Policies).To(gomega.BeEmpty())
				gomega.Expect(cg.Groups["Application"].Values).To(gomega.BeEmpty())
				gomega.Expect(cg.Groups["Application"].Groups).To(gomega.HaveLen(2))
			})

			ginkgo.Context("when the config cannot be turned into a channel group", func() {
				ginkgo.BeforeEach(func() {
					conf.Orderer.OrdererType = garbageRule
				})

				ginkgo.It("wraps and returns the error", func() {
					_, err := DefaultConfigTemplate(conf)
					gomega.Expect(err).To(gomega.MatchError("error parsing configuration: " +
						"could not create orderer group: unknown orderer type: garbage"))
				})
			})

			ginkgo.Context("when the application config is nil", func() {
				ginkgo.BeforeEach(func() {
					conf.Application = nil
				})

				ginkgo.It("returns an error", func() {
					_, err := DefaultConfigTemplate(conf)
					gomega.Expect(err).To(gomega.MatchError("channel template configs must contain " +
						"an application section"))
				})
			})
		})

		ginkgo.Describe("ConfigTemplateFromGroup", func() {
			var (
				applicationConf *Profile
				sysChannelGroup *cb.ConfigGroup
			)

			ginkgo.BeforeEach(func() {
				applicationConf = &Profile{
					Policies:   CreateStandardPolicies(),
					Consortium: "SampleConsortium",
					Orderer: &Orderer{
						OrdererType: "solo",
						Policies:    CreateStandardOrdererPolicies(),
					},
					Application: &Application{
						Organizations: []*Organization{
							{
								Name:          "Org1",
								SkipAsForeign: true,
							},
							{
								Name:          "Org2",
								SkipAsForeign: true,
							},
						},
					},
				}

				var err error
				sysChannelGroup, err = NewChannelGroup(&Profile{
					Policies: CreateStandardPolicies(),
					Orderer: &Orderer{
						OrdererType: "solo",
						Policies:    CreateStandardOrdererPolicies(),
					},
					Consortiums: map[string]*Consortium{
						"SampleConsortium": {
							Organizations: []*Organization{
								{
									Name:          "Org1",
									SkipAsForeign: true,
								},
								{
									Name:          "Org2",
									SkipAsForeign: true,
								},
							},
						},
					},
				})
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})

			ginkgo.It("returns a config template", func() {
				cg, err := configTemplateFromGroup(applicationConf, sysChannelGroup)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(cg.Groups).To(gomega.HaveLen(2))
				gomega.Expect(cg.Groups["Orderer"]).NotTo(gomega.BeNil())
				gomega.Expect(cg.Groups["Orderer"]).To(test.ProtoEqual(sysChannelGroup.Groups["Orderer"]))
				gomega.Expect(cg.Groups["Application"]).NotTo(gomega.BeNil())
				gomega.Expect(cg.Groups["Application"].Policies).To(gomega.HaveLen(1))
				gomega.Expect(cg.Groups["Application"].Policies["Admins"]).NotTo(gomega.BeNil())
				gomega.Expect(cg.Groups["Application"].Values).To(gomega.BeEmpty())
				gomega.Expect(cg.Groups["Application"].Groups).To(gomega.HaveLen(2))
			})

			ginkgo.Context("when the orderer system channel group has no sub-groups", func() {
				ginkgo.BeforeEach(func() {
					sysChannelGroup.Groups = nil
				})

				ginkgo.It("returns an error", func() {
					_, err := configTemplateFromGroup(applicationConf, sysChannelGroup)
					gomega.Expect(err).To(gomega.MatchError("supplied system channel group has no sub-groups"))
				})
			})

			ginkgo.Context("when the orderer system channel group has no consortiums group", func() {
				ginkgo.BeforeEach(func() {
					delete(sysChannelGroup.Groups, "Consortiums")
				})

				ginkgo.It("returns an error", func() {
					_, err := configTemplateFromGroup(applicationConf, sysChannelGroup)
					gomega.Expect(err).To(gomega.MatchError("supplied system channel group does " +
						"not appear to be system channel (missing consortiums group)"))
				})
			})

			ginkgo.Context(
				"when the orderer system channel group has no consortiums in the consortiums group",
				func() {
					ginkgo.BeforeEach(func() {
						sysChannelGroup.Groups["Consortiums"].Groups = nil
					})

					ginkgo.It("returns an error", func() {
						_, err := configTemplateFromGroup(applicationConf, sysChannelGroup)
						gomega.Expect(err).To(gomega.MatchError("system channel consortiums group " +
							"appears to have no consortiums defined"))
					})
				},
			)

			ginkgo.Context("when the orderer system channel group does not have the requested consortium", func() {
				ginkgo.BeforeEach(func() {
					applicationConf.Consortium = "bad-consortium"
				})

				ginkgo.It("returns an error", func() {
					_, err := configTemplateFromGroup(applicationConf, sysChannelGroup)
					gomega.Expect(err).To(gomega.MatchError("supplied system channel group is " +
						"missing 'bad-consortium' consortium"))
				})
			})

			ginkgo.Context("when the channel creation profile has no application section", func() {
				ginkgo.BeforeEach(func() {
					applicationConf.Application = nil
				})

				ginkgo.It("returns an error", func() {
					_, err := configTemplateFromGroup(applicationConf, sysChannelGroup)
					gomega.Expect(err).To(gomega.MatchError("supplied channel creation profile does " +
						"not contain an application section"))
				})
			})

			ginkgo.Context("when the orderer system channel group does not have all the channel creation orgs", func() {
				ginkgo.BeforeEach(func() {
					delete(sysChannelGroup.Groups["Consortiums"].Groups["SampleConsortium"].Groups, "Org1")
				})

				ginkgo.It("returns an error", func() {
					_, err := configTemplateFromGroup(applicationConf, sysChannelGroup)
					gomega.Expect(err).To(gomega.MatchError("consortium SampleConsortium does " +
						"not contain member org Org1"))
				})
			})
		})

		ginkgo.Describe("HasSkippedForeignOrgs", func() {
			var conf *Profile

			ginkgo.BeforeEach(func() {
				conf = &Profile{
					Orderer: &Orderer{
						Organizations: []*Organization{
							{
								Name: "OrdererOrg1",
							},
							{
								Name: "OrdererOrg2",
							},
						},
					},
					Application: &Application{
						Organizations: []*Organization{
							{
								Name: "ApplicationOrg1",
							},
							{
								Name: "ApplicationOrg2",
							},
						},
					},
					Consortiums: map[string]*Consortium{
						"SomeConsortium": {
							Organizations: []*Organization{
								{
									Name: "ConsortiumOrg1",
								},
								{
									Name: "ConsortiumOrg2",
								},
							},
						},
					},
				}
			})

			ginkgo.It("returns no error if all orgs are not skipped as foreign", func() {
				err := HasSkippedForeignOrgs(conf)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})

			ginkgo.Context("when the orderer group has foreign orgs", func() {
				ginkgo.BeforeEach(func() {
					conf.Orderer.Organizations[1].SkipAsForeign = true
				})

				ginkgo.It("returns an error indicating the offending org", func() {
					err := HasSkippedForeignOrgs(conf)
					gomega.Expect(err).To(gomega.MatchError("organization 'OrdererOrg2' is marked " +
						"to be skipped as foreign"))
				})
			})

			ginkgo.Context("when the application group has foreign orgs", func() {
				ginkgo.BeforeEach(func() {
					conf.Application.Organizations[1].SkipAsForeign = true
				})

				ginkgo.It("returns an error indicating the offending org", func() {
					err := HasSkippedForeignOrgs(conf)
					gomega.Expect(err).To(gomega.MatchError("organization 'ApplicationOrg2' is marked " +
						"to be skipped as foreign"))
				})
			})

			ginkgo.Context("when the consortium group has foreign orgs", func() {
				ginkgo.BeforeEach(func() {
					conf.Consortiums["SomeConsortium"].Organizations[1].SkipAsForeign = true
				})

				ginkgo.It("returns an error indicating the offending org", func() {
					err := HasSkippedForeignOrgs(conf)
					gomega.Expect(err).To(gomega.MatchError("organization 'ConsortiumOrg2' is marked " +
						"to be skipped as foreign"))
				})
			})
		})
	})

	ginkgo.Describe("Bootstrapper", func() {
		ginkgo.Describe("NewBootstrapper", func() {
			var conf *Profile

			ginkgo.BeforeEach(func() {
				conf = &Profile{
					Policies: CreateStandardPolicies(),
					Orderer: &Orderer{
						OrdererType: "solo",
						Policies:    CreateStandardOrdererPolicies(),
					},
				}
			})

			ginkgo.It("creates a new bootstrapper for the given config", func() {
				bs, err := NewBootstrapper(conf)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(bs).NotTo(gomega.BeNil())
			})

			ginkgo.Context("when the channel config is bad", func() {
				ginkgo.BeforeEach(func() {
					conf.Orderer.OrdererType = badOrdererType
				})

				ginkgo.It("wraps and returns the error", func() {
					_, err := NewBootstrapper(conf)
					gomega.Expect(err).To(gomega.MatchError("could not create channel group: " +
						"could not create orderer group: unknown orderer type: bad-type"))
				})
			})

			ginkgo.Context("when the channel config contains a foreign org", func() {
				ginkgo.BeforeEach(func() {
					conf.Orderer.Organizations = []*Organization{
						{
							Name:          "MyOrg",
							SkipAsForeign: true,
						},
					}
				})

				ginkgo.It("wraps and returns the error", func() {
					_, err := NewBootstrapper(conf)
					gomega.Expect(err).To(gomega.MatchError("all org definitions must be local during " +
						"bootstrapping: organization 'MyOrg' is marked to be skipped as foreign"))
				})
			})
		})

		ginkgo.Describe("New", func() {
			var conf *Profile

			ginkgo.BeforeEach(func() {
				conf = &Profile{
					Policies: CreateStandardPolicies(),
					Orderer: &Orderer{
						OrdererType: "solo",
						Policies:    CreateStandardOrdererPolicies(),
					},
				}
			})

			ginkgo.It("creates a new bootstrapper for the given config", func() {
				bs := New(conf)
				gomega.Expect(bs).NotTo(gomega.BeNil())
			})

			ginkgo.Context("when the channel config is bad", func() {
				ginkgo.BeforeEach(func() {
					conf.Orderer.OrdererType = badOrdererType
				})

				ginkgo.It("panics", func() {
					gomega.Expect(func() { New(conf) }).To(gomega.Panic())
				})
			})
		})

		ginkgo.Describe("Functions", func() {
			var bs *Bootstrapper

			ginkgo.BeforeEach(func() {
				bs = New(&Profile{
					Policies: CreateStandardPolicies(),
					Orderer: &Orderer{
						Policies:    CreateStandardOrdererPolicies(),
						OrdererType: "solo",
					},
				})
			})

			ginkgo.Describe("GenesisBlock", func() {
				ginkgo.It("produces a new genesis block with a default channel ID", func() {
					block := bs.GenesisBlock()
					gomega.Expect(block).NotTo(gomega.BeNil())
				})
			})

			ginkgo.Describe("GenesisBlockForChannel", func() {
				ginkgo.It("produces a new genesis block with a default channel ID", func() {
					block := bs.GenesisBlockForChannel("channel-id")
					gomega.Expect(block).NotTo(gomega.BeNil())
				})
			})
		})
	})
})
