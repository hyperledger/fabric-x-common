# Cryptogen: Complete Guide to Crypto Material Generation

## Table of Contents

1. [Overview](#overview)
2. [Quick Start](#quick-start)
3. [How Cryptogen Works (The Big Picture)](#how-cryptogen-works-the-big-picture)
4. [config_crypto.yaml Reference](#config_cryptoyaml-reference)
5. [Organization Types](#organization-types)
6. [Node Specification: Template vs Specs](#node-specification-template-vs-specs)
7. [User Specification](#user-specification)
8. [Certificate Authority (CA) Configuration](#certificate-authority-ca-configuration)
9. [Template Variables](#template-variables)
10. [Output Directory Structure](#output-directory-structure)
11. [Certificate Chain of Trust](#certificate-chain-of-trust)
12. [NodeOUs and Role-Based Access](#nodeous-and-role-based-access)
13. [Key Algorithms](#key-algorithms)
14. [Extending an Existing Network](#extending-an-existing-network)
15. [Complete Configuration Examples](#complete-configuration-examples)
16. [Troubleshooting](#troubleshooting)

---

## Overview

`cryptogen` is a command-line utility that generates all the cryptographic key material
(private keys, public certificates, TLS certificates) needed to bootstrap a
Hyperledger Fabric network. It reads a YAML configuration file (`config_crypto.yaml`)
and produces a structured directory tree of certificates organized by organization,
node, and user.

At a high level, the tool takes a declarative description of your network topology
and outputs everything each component needs to prove its identity and communicate
securely. You describe *what* organizations, nodes, and users you want; cryptogen
figures out *how* to create all the keys and certificates they need.

```
+------------------------+       +--------------------+       +---------------------+
|  config_crypto.yaml    |       |     cryptogen      |       |   crypto-config/    |
|                        | ----> |                    | ----> |   (certificates,    |
|  - OrdererOrgs         |       |  1. Parse config   |       |    private keys,    |
|  - PeerOrgs            |       |  2. Render templates|       |    TLS material)    |
|  - GenericOrgs         |       |  3. Generate CAs   |       |                     |
|  - Users               |       |  4. Sign certs     |       |                     |
+------------------------+       +--------------------+       +---------------------+
```

### What It Generates

For each organization defined in the config, cryptogen creates a complete set of
cryptographic material. This material falls into six categories, each serving a
distinct purpose in the Fabric network:

- **Signing CA** -- A self-signed Certificate Authority used to issue enrollment
  (identity) certificates. Every node and user in the organization gets a certificate
  signed by this CA. Other organizations can verify a member's identity by checking
  that their certificate was signed by the org's CA.

- **TLS CA** -- A separate self-signed Certificate Authority dedicated to TLS
  communication. Keeping TLS certificates on a separate CA from identity certificates
  is a security best practice -- compromising one does not compromise the other.

- **Organization MSP** -- The "verifying MSP" for the organization as a whole. It
  contains only the CA certificates (no private keys), and is used by other
  organizations to verify that a certificate belongs to this org. Think of this as
  the org's public trust bundle.

- **Node MSPs** -- Each peer or orderer node gets its own "local MSP" containing a
  private key and a signed certificate. This is what the node uses to sign
  transactions and proposals. Unlike the organization MSP, a local MSP contains
  secret key material and must be kept secure.

- **User MSPs** -- Each user (including the automatically created Admin) gets a local
  MSP with their own private key and certificate. Application clients use these to
  sign transaction proposals before submitting them to the network.

- **TLS Certificates** -- Each node gets a TLS server certificate (for accepting
  incoming connections) and each user gets a TLS client certificate (for connecting
  to nodes). Both are signed by the organization's TLS CA.

---

## Quick Start

### Commands

Cryptogen supports four commands. The most common workflow is to write a config
file and run `generate`:

```bash
# Show the default config template -- useful as a starting point
cryptogen showtemplate

# Generate crypto material using the built-in default config
cryptogen generate

# Generate with a custom config file
cryptogen generate --config ./my-crypto-config.yaml

# Generate to a custom output directory (default is "crypto-config")
cryptogen generate --config ./my-crypto-config.yaml --output ./my-crypto

# Extend existing crypto material with new nodes/users
cryptogen extend --config ./extended-config.yaml --input ./crypto-config

# Show version information
cryptogen version
```

### Minimal Working Example

The simplest useful configuration defines one orderer organization and one peer
organization. This is enough to bootstrap a basic Fabric network for development:

```yaml
# minimal-crypto-config.yaml
OrdererOrgs:
  - Name: OrdererOrg
    Domain: example.com
    Template:
      Count: 1          # Creates 1 orderer node

PeerOrgs:
  - Name: Org1
    Domain: org1.example.com
    Template:
      Count: 2          # Creates 2 peer nodes
    Users:
      Count: 1          # Creates 1 user (+ Admin is always created)
```

Run it with:

```bash
cryptogen generate --config minimal-crypto-config.yaml
```

This creates a `crypto-config/` directory with all the keys and certificates needed
for one orderer, two peers, an admin, and one application user.

---

## How Cryptogen Works (The Big Picture)

Understanding the internal pipeline helps you debug configuration issues and predict
the tool's output. Cryptogen processes your config in three sequential phases, then
generates organizations in parallel for speed.

**Phase 1: Parse.** The YAML file is deserialized into a `Config` struct with three
arrays -- one for each organization type.

**Phase 2: Render Templates.** Template-based node definitions (e.g., `Count: 3`)
are expanded into explicit node specifications. The tool also forces the
Organizational Unit (OU) based on the org type: `"orderer"` for OrdererOrgs,
`"peer"` for PeerOrgs. GenericOrgs skip template expansion entirely.

**Phase 3: Generate.** Each organization is processed in its own goroutine. Within
each goroutine, the tool creates CAs, signs certificates, and builds the MSP
directory tree.

```
                         config_crypto.yaml
                                |
                                v
                  +----------------------------+
                  |      1. PARSE CONFIG       |
                  |    YAML --> Config struct   |
                  |                            |
                  |  Config {                  |
                  |    OrdererOrgs []OrgSpec   |
                  |    PeerOrgs    []OrgSpec   |
                  |    GenericOrgs []OrgSpec   |
                  |  }                         |
                  +----------------------------+
                                |
                                v
                  +----------------------------+
                  |    2. RENDER TEMPLATES      |
                  |                            |
                  |  Template.Count=3 becomes: |
                  |    Spec[0]: orderer0       |
                  |    Spec[1]: orderer1       |
                  |    Spec[2]: orderer2       |
                  |                            |
                  |  Force OU per org type:    |
                  |    OrdererOrgs -> "orderer"|
                  |    PeerOrgs    -> "peer"   |
                  +----------------------------+
                                |
                                v
                  +----------------------------+
                  |  3. GENERATE (in parallel)  |
                  |                            |
                  |    For each organization:  |
                  +----------------------------+
                      |         |         |
         +------------+    +----+----+    +------------+
         v                 v              v
  +--------------+  +--------------+  +--------------+
  | OrdererOrg1  |  | PeerOrg1     |  | PeerOrg2     |
  | (goroutine)  |  | (goroutine)  |  | (goroutine)  |
  +--------------+  +--------------+  +--------------+
         |                 |                 |
         v                 v                 v
  +------------------------------------------------+
  |         Per-Organization Generation            |
  |                                                |
  |  a) Generate Signing CA (key + self-signed)    |
  |  b) Generate TLS CA (key + self-signed)        |
  |  c) Generate Org Verifying MSP                 |
  |  d) For each node: generate Local MSP + TLS    |
  |  e) For each user: generate Local MSP + TLS    |
  |  f) Copy admin cert to nodes (if no NodeOUs)   |
  +------------------------------------------------+
                        |
                        v
               +------------------+
               |  crypto-config/  |
               |  (output tree)   |
               +------------------+
```

### Certificate Signing Flow

Every certificate in the output (except the two self-signed CA certs) is signed by
one of the organization's Certificate Authorities. The Signing CA issues identity
certificates that nodes and users use to sign transactions. The TLS CA issues
transport-layer certificates that nodes and users use for encrypted communication.

This separation means that each organization has two independent trust chains. A
compromise of the TLS CA does not affect identity verification, and vice versa.

```
                +---------------------+
                |   CA Private Key    |
                |   (generated once)  |
                +---------------------+
                          |
            +-------------+-------------+
            |                           |
            v                           v
  +------------------+       +------------------+
  |   Signing CA     |       |    TLS CA        |
  |  (self-signed)   |       |  (self-signed)   |
  +------------------+       +------------------+
       |    |    |                |    |    |
       v    v    v                v    v    v
    +-----+-----+-----+     +-----+-----+-----+
    |Node |Node |User |     |Node |Node |User |
    |Sign |Sign |Sign |     |TLS  |TLS  |TLS  |
    |Cert |Cert |Cert |     |Cert |Cert |Cert |
    +-----+-----+-----+     +-----+-----+-----+

   Signing Certificates      TLS Certificates
   (identity/enrollment)     (secure communication)
```

---

## config_crypto.yaml Reference

The configuration file is a standard YAML document with three top-level keys. Each
key holds an array of organization specifications (`OrgSpec`). You can include any
combination of the three sections -- all are optional, but at least one organization
should be defined for the tool to produce useful output.

```
+----------------------------------------------------------------+
|                    config_crypto.yaml                           |
+----------------------------------------------------------------+
|                                                                |
|  OrdererOrgs:    # Organizations that run orderer nodes        |
|    - OrgSpec                                                   |
|    - OrgSpec                                                   |
|    - ...                                                       |
|                                                                |
|  PeerOrgs:       # Organizations that run peer nodes           |
|    - OrgSpec                                                   |
|    - OrgSpec                                                   |
|    - ...                                                       |
|                                                                |
|  GenericOrgs:    # Organizations with custom node roles        |
|    - OrgSpec                                                   |
|    - ...                                                       |
|                                                                |
+----------------------------------------------------------------+
```

### OrgSpec Structure

Each organization is defined by an `OrgSpec`. The `OrgSpec` bundles together the
organization's identity (`Name`, `Domain`), its Certificate Authority configuration
(`CA`), the nodes it operates (`Template` and/or `Specs`), and the users that belong
to it (`Users`).

The `Name` field is used as the directory name in the output. The `Domain` field
appears in certificate Common Names and Subject Alternative Names, tying the
cryptographic identity to a DNS domain. `EnableNodeOUs` controls whether roles are
encoded directly into the certificates (recommended for production -- see the
[NodeOUs section](#nodeous-and-role-based-access)).

```
+----------------------------------------------------------------+
|                         OrgSpec                                |
+----------------------------------------------------------------+
|                                                                |
|  Name: string           # Organization name (e.g. "Org1")     |
|  Domain: string         # Domain (e.g. "org1.example.com")    |
|  EnableNodeOUs: bool    # Enable NodeOU-based role identity    |
|                                                                |
|  CA: NodeSpec           # Certificate Authority configuration  |
|    +----------------------------------------------------------+|
|    | Hostname, CommonName, Country, Province, Locality,       ||
|    | OrganizationalUnit, StreetAddress, PostalCode,           ||
|    | PublicKeyAlgorithm                                       ||
|    +----------------------------------------------------------+|
|                                                                |
|  Template: NodeTemplate # Generate nodes from a template       |
|    +----------------------------------------------------------+|
|    | Count, Start, Hostname, SANS, PublicKeyAlgorithm         ||
|    +----------------------------------------------------------+|
|                                                                |
|  Specs: []NodeSpec      # Explicitly defined nodes             |
|    +----------------------------------------------------------+|
|    | [ {Hostname, CommonName, SANS, PublicKeyAlgorithm, ...} ]||
|    +----------------------------------------------------------+|
|                                                                |
|  Users: UsersSpec       # User accounts to generate            |
|    +----------------------------------------------------------+|
|    | Count, PublicKeyAlgorithm, Specs: []UserSpec             ||
|    +----------------------------------------------------------+|
|                                                                |
+----------------------------------------------------------------+
```

### Field-by-Field Reference

The following table lists every configurable field with its type, default value, and
a description of its effect. Fields marked *required* must be provided; all others
are optional and fall back to sensible defaults.

| Field | Type | Default | Description |
|---|---|---|---|
| `Name` | string | *required* | Organization display name, used as directory name |
| `Domain` | string | *required* | Domain name, used in certificate CN and SANS |
| `EnableNodeOUs` | bool | `false` | Generate `config.yaml` with NodeOU identifiers |
| `CA.Hostname` | string | `"ca"` | CA hostname, used in certificate file naming |
| `CA.CommonName` | string | `"{{.Hostname}}.{{.Domain}}"` | CA certificate Common Name |
| `CA.Country` | string | `"US"` | X.509 subject Country |
| `CA.Province` | string | `"California"` | X.509 subject State/Province |
| `CA.Locality` | string | `"San Francisco"` | X.509 subject Locality |
| `CA.OrganizationalUnit` | string | `""` | X.509 subject OU (for the CA cert itself) |
| `CA.StreetAddress` | string | `""` | X.509 subject Street Address |
| `CA.PostalCode` | string | `""` | X.509 subject Postal Code |
| `CA.PublicKeyAlgorithm` | string | `"ecdsa"` | `"ecdsa"` (P-256) or `"ed25519"` |
| `Template.Count` | int | `0` | Number of nodes to generate from the template |
| `Template.Start` | int | `0` | Starting index for generated hostnames |
| `Template.Hostname` | string | `"{{.Prefix}}{{.Index}}"` | Go template for hostname generation |
| `Template.SANS` | []string | `[]` | Additional Subject Alternative Names (Go templates) |
| `Template.PublicKeyAlgorithm` | string | `"ecdsa"` | Key algorithm for all templated nodes |
| `Specs[].Hostname` | string | *required* | Node hostname |
| `Specs[].CommonName` | string | `"{{.Hostname}}.{{.Domain}}"` | Certificate CN (Go template) |
| `Specs[].SANS` | []string | `[]` | Additional SANs (CN and Hostname are added automatically) |
| `Specs[].PublicKeyAlgorithm` | string | `"ecdsa"` | Key algorithm for this specific node |
| `Specs[].OrganizationalUnit` | string | forced by org type | OU (auto-set for OrdererOrgs/PeerOrgs; required for GenericOrgs) |
| `Specs[].Party` | string | `""` | Sub-directory grouping for GenericOrgs |
| `Users.Count` | int | `0` | Number of numbered users to generate (in addition to Admin) |
| `Users.PublicKeyAlgorithm` | string | `"ecdsa"` | Key algorithm for count-based users |
| `Users.Specs[].Name` | string | *required* | User name (becomes `Name@Domain`) |
| `Users.Specs[].PublicKeyAlgorithm` | string | inherits from `Users` | Key algorithm override for this user |

---

## Organization Types

Cryptogen supports three types of organizations. The type determines how the tool
assigns Organizational Unit (OU) values to node certificates, whether templates are
available, and where in the output tree the material is placed. Choosing the right
type is important because it affects how the Fabric runtime identifies node roles.

The following comparison summarizes the key differences:

```
+------------------------+------------------------+------------------------+
|      OrdererOrgs       |       PeerOrgs         |      GenericOrgs       |
+------------------------+------------------------+------------------------+
|                        |                        |                        |
| OU forced to "orderer" | OU forced to "peer"    | OU set per-node        |
| for ALL nodes          | for ALL nodes          | (must be explicit)     |
|                        |                        |                        |
| Templates: YES         | Templates: YES         | Templates: NO          |
|                        |                        | (must use Specs)       |
|                        |                        |                        |
| Output directory:      | Output directory:      | Output directory:      |
| ordererOrganizations/  | peerOrganizations/     | organizations/         |
|                        |                        |                        |
| Node subdirectory:     | Node subdirectory:     | Node subdirectory:     |
| orderers/              | peers/                 | orderers/ or peers/    |
|                        |                        | (based on OU)          |
|                        |                        |                        |
| Admin: always created  | Admin: always created  | Admin: always created  |
| Users: via Users spec  | Users: via Users spec  | Users: via Users spec  |
+------------------------+------------------------+------------------------+
```

### When to Use Each Type

Most networks only need `OrdererOrgs` and `PeerOrgs`. Use `GenericOrgs` only when
a single organization needs to operate nodes with different roles (e.g., both
orderers and peers under the same CA), or when you need multi-party sub-groupings
within an organization.

| Use Case | Organization Type |
|---|---|
| Standard ordering service nodes | `OrdererOrgs` |
| Standard peer nodes for endorsement/commit | `PeerOrgs` |
| Mixed roles in the same org (e.g., router + endorser) | `GenericOrgs` |
| Multi-party organizations with sub-grouping | `GenericOrgs` |

### OrdererOrgs vs PeerOrgs

OrdererOrgs and PeerOrgs behave identically except for two things: the OU value
stamped into every node certificate, and the subdirectory name used in the output.
OrdererOrgs force `OU=orderer` and place nodes under `orderers/`. PeerOrgs force
`OU=peer` and place nodes under `peers/`.

```
OrdererOrgs:                           PeerOrgs:
  - Name: MyOrderer                      - Name: MyPeer
    Domain: orderer.example.com            Domain: peer.example.com
    Template:                              Template:
      Count: 3                               Count: 2

    All nodes get OU = "orderer"           All nodes get OU = "peer"
              |                                      |
              v                                      v
  ordererOrganizations/              peerOrganizations/
    MyOrderer/                         MyPeer/
      orderers/                          peers/
        orderer0.orderer.example.com       peer0.peer.example.com
        orderer1.orderer.example.com       peer1.peer.example.com
        orderer2.orderer.example.com
```

### GenericOrgs -- Mixed Roles

GenericOrgs are designed for advanced topologies where a single organization operates
nodes with different roles. Because the tool cannot infer the role from the org type,
you must set `OrganizationalUnit` explicitly on every node in the `Specs` array.
Templates are not supported for GenericOrgs -- this is intentional, since each node
may have a different OU.

The optional `Party` field creates an additional sub-directory level, which is useful
when multiple parties within a consortium share a single organizational CA but
operate their own nodes.

```yaml
GenericOrgs:
  - Name: JointOrg
    Domain: joint-org.com
    Specs:
      - Hostname: router-1.joint-org.com
        OrganizationalUnit: orderer      # <-- Must specify OU
        Party: party-1                   # <-- Optional sub-grouping
      - Hostname: endorser.joint-org.com
        OrganizationalUnit: peer         # <-- Different OU in same org
```

The `Party` field controls directory nesting. Nodes with a party are placed under
`orderers/<party>/` or `peers/<party>/`. Nodes without a party are placed directly
under `orderers/` or `peers/`:

```
organizations/
  JointOrg/
    orderers/
      party-1/                      <-- Party creates a sub-directory
        router-1.joint-org.com/
          msp/
          tls/
      party-2/
        router-2.joint-org.com/
          msp/
          tls/
    peers/
      endorser.joint-org.com/       <-- No party = directly under peers/
        msp/
        tls/
```

---

## Node Specification: Template vs Specs

You have two ways to define which nodes an organization operates: **Template**
generates multiple nodes automatically from a pattern, while **Specs** lets you
define each node individually with full control over hostnames, SANs, and key
algorithms. The two approaches are NOT mutually exclusive -- you can use both in the
same organization, and the resulting node lists are merged.

### Template: Generate Multiple Nodes Automatically

The `Template` section is the most common way to define nodes. You specify a
`Count`, and the tool generates that many nodes with sequentially numbered hostnames.
The `Start` field controls the first index number (default 0). The `Hostname` field
is a Go template string that can reference the `{{.Prefix}}` (org type),
`{{.Index}}` (current number), and `{{.Domain}}` (org domain).

```yaml
Template:
  Count: 3           # Generate 3 nodes
  Start: 0           # Start indexing from 0 (default)
  Hostname: "{{.Prefix}}-{{.Index}}.{{.Domain}}"
  SANS:
    - "{{.Hostname}}.alt.{{.Domain}}"
  PublicKeyAlgorithm: ecdsa
```

The tool expands this template into individual node specifications. The `Prefix`
value is automatically set based on the organization type -- `"orderer"` for
OrdererOrgs and `"peer"` for PeerOrgs:

```
+---------------------------------------------+
|  Template Expansion (OrdererOrg)            |
+---------------------------------------------+
|                                             |
|  Count=3, Start=0, Prefix="orderer"         |
|                                             |
|  Index 0 --> orderer-0.example.com          |
|  Index 1 --> orderer-1.example.com          |
|  Index 2 --> orderer-2.example.com          |
|                                             |
|  (PeerOrg would use Prefix="peer")          |
|                                             |
|  Index 0 --> peer-0.example.com             |
|  Index 1 --> peer-1.example.com             |
|  Index 2 --> peer-2.example.com             |
+---------------------------------------------+
```

You can use a custom `Start` index to control the numbering. This is useful when
extending a network -- you can start new nodes at a higher index to avoid collisions
with existing nodes:

```yaml
Template:
  Count: 2
  Start: 5       # Start at index 5 instead of 0
```

This produces nodes named `orderer5.example.com` and `orderer6.example.com`.

### Specs: Define Nodes Explicitly

The `Specs` section gives you full control over each individual node. Use this when
you need custom hostnames, specific SANs (like internal IP addresses), or per-node
key algorithm overrides. Every field in a `NodeSpec` can be set independently.

```yaml
Specs:
  - Hostname: orderer-west
    CommonName: orderer-west.example.com
    SANS:
      - "orderer-west.internal.example.com"
      - "10.0.0.1"
    PublicKeyAlgorithm: ecdsa

  - Hostname: orderer-east
    CommonName: orderer-east.example.com
    SANS:
      - "orderer-east.internal.example.com"
      - "10.0.0.2"
```

The `CommonName` defaults to `"{{.Hostname}}.{{.Domain}}"` if not specified, so you
only need to provide it when you want a CN that differs from that pattern.

### Combining Template + Specs

When you use both `Template` and `Specs` in the same organization, the tool first
expands the template into a list of node specs, then appends the explicitly defined
specs. The combined list determines all the nodes that get crypto material.

This is useful when most of your nodes follow a pattern but a few need special
treatment:

```yaml
OrdererOrgs:
  - Name: OrdererOrg
    Domain: example.com
    Template:
      Count: 2              # Auto-generates orderer0, orderer1
    Specs:
      - Hostname: orderer-backup   # Adds a third, explicit node
```

The result is three nodes total:

```
Template generates:     Specs defines:        Combined result:
+------------------+    +-----------------+   +-----------------------------+
| orderer0         |    | orderer-backup  |   | orderer0.example.com       |
| orderer1         | +  |                 | = | orderer1.example.com       |
+------------------+    +-----------------+   | orderer-backup.example.com |
                                              +-----------------------------+
```

Be careful with name collisions -- if a template-generated hostname matches a
spec-defined hostname, you will get a conflict. Use `Template.Start` to offset
the numbering and avoid overlaps.

### YAML Anchors for Reusing Templates

When multiple organizations share the same node topology (same count, same hostname
pattern, same key algorithm), you can avoid duplication by using YAML anchors (`&`)
and aliases (`*`). The anchor defines a reusable block, and the alias references it.
Each organization will still use its own `Domain`, so the generated hostnames will
be unique.

```yaml
OrdererOrgs:
  - Name: Org1
    Domain: org1.com
    Template: &SharedTemplate     # Define anchor
      Count: 1
      Start: 1
      Hostname: "{{.Prefix}}-{{.Index}}.{{.Domain}}"
      PublicKeyAlgorithm: ecdsa

  - Name: Org2
    Domain: org2.com
    Template: *SharedTemplate     # Reuse the same template
```

---

## User Specification

Users represent human operators or application clients that interact with the Fabric
network. Each user gets their own local MSP (with a private key for signing
transactions) and a TLS client certificate (for secure communication with nodes).

Users are defined in the `Users` section of an organization. An **Admin** user is
always created automatically for every organization -- you never need to define it
in your config. The Admin user is special: its certificate is used to establish
administrative authority over the organization's nodes (see
[NodeOUs](#nodeous-and-role-based-access) for details on how admin identity works).

### User Generation Methods

There are two ways to specify users, and they can be combined. **Count-based** users
are numbered automatically (`User1@domain`, `User2@domain`, etc.). **Spec-based**
users let you assign meaningful names (`appuser@domain`, `auditor@domain`). Both
approaches can be used together, and the Admin user is always added on top.

```
+-----------------------------------------------------------+
|                   Users Specification                     |
+-----------------------------------------------------------+
|                                                           |
|  Users:                                                   |
|    Count: 2              # Generates User1@, User2@       |
|    PublicKeyAlgorithm: ecdsa                              |
|    Specs:                                                 |
|      - Name: appuser     # Generates appuser@             |
|      - Name: auditor     # Generates auditor@             |
|                                                           |
|  + Admin (always auto-generated)                          |
|                                                           |
+-----------------------------------------------------------+
|                                                           |
|  Total users created (for domain org1.example.com):       |
|    1. Admin@org1.example.com      (automatic, ECDSA)      |
|    2. User1@org1.example.com      (from Count)            |
|    3. User2@org1.example.com      (from Count)            |
|    4. appuser@org1.example.com    (from Specs)            |
|    5. auditor@org1.example.com    (from Specs)            |
|                                                           |
+-----------------------------------------------------------+
```

### User vs Node OUs

Every user certificate is stamped with an Organizational Unit (OU) that identifies
their role. The Admin user gets a special OU to distinguish it from regular
application users. Note that when `EnableNodeOUs` is `false`, the Admin is given
`OU=client` instead of `OU=admin`, because without NodeOUs the admin identity is
established through certificate distribution rather than OU inspection.

| User | OU (EnableNodeOUs=true) | OU (EnableNodeOUs=false) |
|---|---|---|
| Admin | `admin` | `client` |
| All other users | `client` | `client` |

### Example

The following config creates two peer nodes and three users (Admin + User1 + testuser)
for Org1. Note that `testuser` uses Ed25519 while the other users use ECDSA -- you
can override the key algorithm on a per-user basis:

```yaml
PeerOrgs:
  - Name: Org1
    Domain: org1.example.com
    Template:
      Count: 2
    Users:
      Count: 1                        # Creates User1@org1.example.com
      PublicKeyAlgorithm: ecdsa
      Specs:
        - Name: testuser              # Creates testuser@org1.example.com
          PublicKeyAlgorithm: ed25519  # Override key algorithm for this user
```

---

## Certificate Authority (CA) Configuration

Each organization gets **two** Certificate Authorities, both derived from the single
`CA` section in your config. The first is the **Signing CA**, which issues identity
certificates for nodes and users. The second is the **TLS CA**, which issues
transport-layer security certificates. Both CAs are self-signed (there is no
intermediate or root CA hierarchy -- each org is its own root of trust).

The `CA` section accepts the same fields as a `NodeSpec`. The `Hostname` and
`CommonName` control naming. The `Country`, `Province`, `Locality`, and other X.509
subject fields are stamped into both the CA certificates and every certificate the
CA signs.

The TLS CA's Common Name is automatically prefixed with `"tls"`. For example, if
your CA has `CommonName: MyOrgCA`, the Signing CA will have `CN=MyOrgCA` and the
TLS CA will have `CN=tlsMyOrgCA`. This ensures the two CAs are distinguishable.

```
+-------------------------------------------------------------------+
|                      CA Configuration                             |
+-------------------------------------------------------------------+
|                                                                   |
|  CA:                                                              |
|    Hostname: ca                  # Used in cert file naming       |
|    CommonName: MyOrgCA           # CN in the certificate          |
|    Country: US                   # X.509 Subject fields           |
|    Province: California          #   (all optional, have defaults)|
|    Locality: San Francisco       #                                |
|    OrganizationalUnit: Fabric    #                                |
|    StreetAddress: 123 Main St    #                                |
|    PostalCode: 94105             #                                |
|    PublicKeyAlgorithm: ecdsa     # "ecdsa" or "ed25519"           |
|                                                                   |
+-------------------------------------------------------------------+
            |                                       |
            v                                       v
  +--------------------+                 +--------------------+
  |    Signing CA      |                 |     TLS CA         |
  +--------------------+                 +--------------------+
  | CN: MyOrgCA        |                 | CN: tlsMyOrgCA     |
  | Self-signed        |                 | Self-signed        |
  | 10-year expiry     |                 | 10-year expiry     |
  |                    |                 |                    |
  | Signs:             |                 | Signs:             |
  |  - Node identity   |                 |  - TLS server      |
  |  - User identity   |                 |  - TLS client      |
  +--------------------+                 +--------------------+
  |                    |                 |                    |
  | Saved to:          |                 | Saved to:          |
  |  ca/priv_sk        |                 |  tlsca/priv_sk     |
  |  ca/ca.MyOrgCA     |                 |  tlsca/tlsca.      |
  |    -cert.pem       |                 |    tlsMyOrgCA      |
  +--------------------+                 |    -cert.pem       |
                                         +--------------------+
```

### CA Certificate Properties

All CA certificates share the following properties. These are hardcoded in the tool
and cannot be changed via configuration:

- **Validity**: ~10 years (3650 days), backdated 5 minutes to account for clock skew
- **Self-signed**: The CA certificate is its own issuer
- **Key Usage**: Digital Signature, Key Encipherment, Cert Sign, CRL Sign
- **Extended Key Usage**: Client Auth, Server Auth
- **Subject Key Identifier**: SHA-256 hash of the public key (RFC 7093, Method 4)
- **Serial Number**: Random 128-bit integer

### Default X.509 Subject

When you omit the X.509 subject fields from your CA config, the following defaults
are used. These defaults are also inherited by every certificate the CA signs:

| Field | Default Value |
|---|---|
| Country | `US` |
| Province | `California` |
| Locality | `San Francisco` |

---

## Template Variables

Cryptogen uses Go's `text/template` syntax in several configuration fields. Templates
are evaluated at render time, and the available variables differ depending on which
field you are templating. This section documents every variable available in each
context.

### Hostname Template (Template.Hostname)

The `Template.Hostname` field controls how hostnames are generated for template-based
nodes. It has access to three variables:

- **`{{.Prefix}}`** -- The Organizational Unit for this org type. Set to `"orderer"`
  for OrdererOrgs and `"peer"` for PeerOrgs.
- **`{{.Index}}`** -- The current node index, starting at `Template.Start` (default 0).
- **`{{.Domain}}`** -- The organization's domain from `OrgSpec.Domain`.

If you omit the `Hostname` field entirely, the default template `"{{.Prefix}}{{.Index}}"`
is used, which produces hostnames like `orderer0`, `peer1`, etc.

```
+------------------------------------------------+
|  Available Variables in Hostname Template      |
+------------------------------------------------+
|                                                |
|  {{.Prefix}}  = Organization Unit              |
|                 "orderer" for OrdererOrgs      |
|                 "peer"    for PeerOrgs          |
|                                                |
|  {{.Index}}   = Current node index             |
|                 Starts at Template.Start        |
|                 (default 0)                    |
|                                                |
|  {{.Domain}}  = Organization domain            |
|                 From OrgSpec.Domain            |
|                                                |
+------------------------------------------------+
```

Here are some examples showing how different templates expand:

| Template | Org Type | Index | Domain | Result |
|---|---|---|---|---|
| *(default)* `{{.Prefix}}{{.Index}}` | Orderer | 0 | example.com | `orderer0` |
| *(default)* `{{.Prefix}}{{.Index}}` | Peer | 1 | example.com | `peer1` |
| `{{.Prefix}}-{{.Index}}.{{.Domain}}` | Orderer | 0 | example.com | `orderer-0.example.com` |
| `{{.Prefix}}-{{.Index}}.{{.Domain}}` | Peer | 2 | org1.com | `peer-2.org1.com` |
| `node{{.Index}}` | Orderer | 3 | example.com | `node3` |

### CommonName Template (Specs[].CommonName)

The `CommonName` field on a `NodeSpec` or templated node determines the certificate's
Common Name (CN). It supports a different set of variables than the hostname template,
because it is evaluated after the hostname has already been resolved:

- **`{{.Hostname}}`** -- The node's resolved hostname.
- **`{{.Domain}}`** -- The organization's domain.
- **`{{.CommonName}}`** -- Self-referential; the current value of CommonName before
  template evaluation. Use with care.

If you omit the `CommonName`, the default template `"{{.Hostname}}.{{.Domain}}"` is
used. For example, a node with `Hostname=peer0` in an org with `Domain=org1.com`
gets `CN=peer0.org1.com`.

```
+------------------------------------------------+
|  Available Variables in CommonName Template    |
+------------------------------------------------+
|                                                |
|  {{.Hostname}}   = Node hostname               |
|                    From NodeSpec.Hostname       |
|                                                |
|  {{.Domain}}     = Organization domain         |
|                    From OrgSpec.Domain          |
|                                                |
|  {{.CommonName}} = (self-referential, use      |
|                     with care)                 |
|                                                |
+------------------------------------------------+
```

### SANS Template (Specs[].SANS and Template.SANS)

Subject Alternative Names (SANs) allow a single certificate to be valid for multiple
hostnames or IP addresses. This is essential when a node is reachable via multiple
DNS names or IP addresses (e.g., an internal address and an external address).

SAN templates have access to the same variables as CommonName templates, with the
`{{.CommonName}}` now fully resolved:

- **`{{.Hostname}}`** -- The node's resolved hostname.
- **`{{.Domain}}`** -- The organization's domain.
- **`{{.CommonName}}`** -- The node's resolved Common Name.

Two implicit SAN entries are always added automatically -- you do not need to include
them in your SANS list:

1. The resolved `CommonName`
2. The `Hostname`

If a SAN entry looks like an IP address (e.g., `"172.16.0.1"`), it is automatically
placed in the certificate's IP SAN field. All other values are treated as DNS names.

```
+------------------------------------------------+
|  Available Variables in SANS Template          |
+------------------------------------------------+
|                                                |
|  {{.Hostname}}   = Node hostname               |
|  {{.Domain}}     = Organization domain         |
|  {{.CommonName}} = Resolved Common Name        |
|                                                |
+------------------------------------------------+
```

Here is an example showing how the SANS list is constructed:

```
Config:
  SANS:
    - "{{.Hostname}}.internal.{{.Domain}}"
    - "172.16.0.1"

With Hostname="peer0", Domain="org1.com", CN="peer0.org1.com":

Final SANS list (4 entries):
  - peer0.org1.com                (implicit: CN)
  - peer0                         (implicit: Hostname)
  - peer0.internal.org1.com       (from template)
  - 172.16.0.1                    (literal IP -- placed in IP SAN field)
```

---

## Output Directory Structure

Cryptogen produces a single output directory (default: `crypto-config/`) containing
all the generated material. The directory is organized in a three-level hierarchy:
organization type, organization name, then entity (node or user).

### Top-Level Layout

The top level separates organizations by type. Each type gets its own directory.
Only directories for types that appear in your config are created.

```
crypto-config/                            <-- root output directory
|
+-- ordererOrganizations/                 <-- all OrdererOrgs
|     +-- OrgName/
|           +-- ...
|
+-- peerOrganizations/                    <-- all PeerOrgs
|     +-- OrgName/
|           +-- ...
|
+-- organizations/                        <-- all GenericOrgs
      +-- OrgName/
            +-- ...
```

### Per-Organization Layout (OrdererOrg Example)

Within each organization directory, the material is split into five subdirectories.
The `ca/` and `tlsca/` directories hold the CA key material. The `msp/` directory
holds the organization's verifying MSP. The `orderers/` (or `peers/`) directory holds
per-node local MSPs. The `users/` directory holds per-user local MSPs.

Each node and user directory contains an `msp/` subdirectory (with signing keys and
certificates) and a `tls/` subdirectory (with TLS keys and certificates).

```
ordererOrganizations/
  OrdererOrg/
  |
  +-- ca/                                   Signing CA artifacts
  |     +-- priv_sk                           CA private key (PKCS8 PEM)
  |     +-- ca.OrdererOrgCA-cert.pem          CA self-signed certificate
  |
  +-- tlsca/                                TLS CA artifacts
  |     +-- priv_sk                           TLS CA private key
  |     +-- tlsca.tlsOrdererOrgCA-cert.pem    TLS CA self-signed certificate
  |
  +-- msp/                                  Organization Verifying MSP
  |     +-- cacerts/                          Signing CA certificate
  |     |     +-- ca.OrdererOrgCA-cert.pem
  |     +-- tlscacerts/                       TLS CA certificate
  |     |     +-- tlsca.tlsOrdererOrgCA-cert.pem
  |     +-- admincerts/                       Admin cert (only if no NodeOUs)
  |     +-- knowncerts/                       All org member certificates
  |     |     +-- orderer0.example.com-cert.pem
  |     |     +-- Admin@example.com-cert.pem
  |     +-- config.yaml                       NodeOU config (only if EnableNodeOUs)
  |     (no keystore/ or signcerts/ -- this is a verifying MSP)
  |
  +-- orderers/                             Per-node artifacts
  |     +-- orderer0.example.com/
  |     |     +-- msp/                        Node Local MSP
  |     |     |     +-- cacerts/
  |     |     |     |     +-- ca.OrdererOrgCA-cert.pem
  |     |     |     +-- tlscacerts/
  |     |     |     |     +-- tlsca.tlsOrdererOrgCA-cert.pem
  |     |     |     +-- keystore/
  |     |     |     |     +-- priv_sk           Node signing private key
  |     |     |     +-- signcerts/
  |     |     |     |     +-- orderer0.example.com-cert.pem
  |     |     |     +-- admincerts/             Admin cert (only if no NodeOUs)
  |     |     |     +-- config.yaml             (only if EnableNodeOUs)
  |     |     |
  |     |     +-- tls/                        Node TLS artifacts
  |     |           +-- server.crt              TLS server certificate
  |     |           +-- server.key              TLS server private key
  |     |           +-- ca.crt                  TLS CA certificate
  |     |
  |     +-- orderer1.example.com/
  |           +-- (same structure)
  |
  +-- users/                                Per-user artifacts
        +-- Admin@example.com/
        |     +-- msp/                        User Local MSP
        |     |     +-- cacerts/
        |     |     +-- tlscacerts/
        |     |     +-- keystore/
        |     |     |     +-- priv_sk           User signing private key
        |     |     +-- signcerts/
        |     |     |     +-- Admin@example.com-cert.pem
        |     |     +-- admincerts/
        |     |     +-- config.yaml             (only if EnableNodeOUs)
        |     |
        |     +-- tls/                        User TLS artifacts
        |           +-- client.crt              TLS client certificate
        |           +-- client.key              TLS client private key
        |           +-- ca.crt                  TLS CA certificate
        |
        +-- User1@example.com/
              +-- (same structure as Admin, but with OU="client")
```

### PeerOrg Layout Differences

PeerOrgs follow the exact same structure as OrdererOrgs, with one difference: the
node subdirectory is named `peers/` instead of `orderers/`:

```
peerOrganizations/
  Org1/
    +-- ca/
    +-- tlsca/
    +-- msp/
    +-- peers/                              <-- "peers" instead of "orderers"
    |     +-- peer0.org1.example.com/
    |     +-- peer1.org1.example.com/
    +-- users/
          +-- Admin@org1.example.com/
          +-- User1@org1.example.com/
```

### TLS File Naming Convention

TLS certificates and keys are named according to the entity's role. Server nodes
(orderers and peers) get `server.crt` and `server.key` because they accept incoming
TLS connections. Users (including Admin) get `client.crt` and `client.key` because
they initiate outgoing TLS connections. Both include `ca.crt`, which is the TLS CA
certificate needed to verify the other party.

| Entity Type | Certificate | Private Key | CA Certificate |
|---|---|---|---|
| Orderer node | `server.crt` | `server.key` | `ca.crt` |
| Peer node | `server.crt` | `server.key` | `ca.crt` |
| Admin user | `client.crt` | `client.key` | `ca.crt` |
| Client user | `client.crt` | `client.key` | `ca.crt` |

### Verifying MSP vs Local MSP

Cryptogen generates two kinds of MSP (Membership Service Provider) directories. It
is important to understand the difference because they serve fundamentally different
purposes and contain different material.

The **Verifying MSP** (at the organization level) contains only public certificates.
It has no private keys. Other organizations use this MSP to answer the question
"does this certificate belong to OrgX?" It includes a `knowncerts/` directory listing
all known members of the organization.

The **Local MSP** (at the node/user level) contains a private key and a signed
certificate. The node or user uses this MSP to sign transactions and prove their
identity. It does NOT include `knowncerts/` because a node only needs its own
identity, not the full membership list.

```
+-------------------------------+      +-------------------------------+
|       Verifying MSP           |      |        Local MSP              |
|     (Organization level)      |      |      (Node/User level)        |
+-------------------------------+      +-------------------------------+
|                               |      |                               |
|  cacerts/      [CA cert]      |      |  cacerts/      [CA cert]      |
|  tlscacerts/   [TLS CA cert]  |      |  tlscacerts/   [TLS CA cert]  |
|  admincerts/   [admin cert]   |      |  admincerts/   [admin cert]   |
|  knowncerts/   [all certs]    |      |  keystore/     [PRIVATE KEY]  |
|                               |      |  signcerts/    [node cert]    |
|  keystore/     (empty/none)   |      |  knowncerts/   (empty/none)   |
|  signcerts/    (empty/none)   |      |                               |
|                               |      |  + tls/                       |
|  Purpose: Verify identities   |      |    server.crt / client.crt    |
|  without signing capability   |      |    server.key / client.key    |
|                               |      |    ca.crt                     |
|                               |      |                               |
|                               |      |  Purpose: Sign transactions   |
|                               |      |  and authenticate via TLS     |
+-------------------------------+      +-------------------------------+
```

### File Permissions

All generated files use restrictive permissions to protect sensitive key material:

| Type | Permission | Meaning |
|---|---|---|
| Directories | `0750` | Owner: read/write/execute. Group: read/execute. Others: none. |
| Private keys | `0600` | Owner: read/write only. No group or other access. |
| Certificates | `0650` | Owner: read/write. Group: read/execute. Others: none. |

---

## Certificate Chain of Trust

Every certificate generated by cryptogen forms a simple two-level chain of trust.
At the top of each chain is a self-signed root CA certificate. Below it are the
leaf certificates for nodes and users. There is no intermediate CA -- the root CA
directly signs every leaf certificate.

Each organization maintains two independent trust chains: one for signing/identity
and one for TLS. This means there are four total certificate types in each org:
the Signing CA, the TLS CA, the signing leaf certificates, and the TLS leaf
certificates.

The signing chain is used for identity verification -- when a peer endorses a
transaction, the endorsement is verified by checking that the peer's signing
certificate was issued by a recognized CA. The TLS chain is used for transport
security -- when two nodes establish a gRPC connection, they verify each other's
TLS certificates against the known TLS CAs.

```
+=================================================================+
|                     SIGNING CHAIN                               |
+=================================================================+
|                                                                 |
|   Root CA (Self-Signed)                                         |
|   +-------------------------------+                             |
|   | CN: MyOrgCA                   |                             |
|   | Issuer: MyOrgCA (self)        |                             |
|   | IsCA: true                    |                             |
|   | KeyUsage: CertSign, CRLSign,  |                             |
|   |           DigitalSignature,   |                             |
|   |           KeyEncipherment     |                             |
|   +-------------------------------+                             |
|          |              |              |                        |
|          v              v              v                        |
|   +-----------+  +-----------+  +-------------+                |
|   | Node Cert |  | Node Cert |  | User Cert   |                |
|   | orderer0  |  | orderer1  |  | Admin@...   |                |
|   | OU:orderer|  | OU:orderer|  | OU: admin   |                |
|   | KeyUsage: |  | KeyUsage: |  | KeyUsage:   |                |
|   | DigitalSig|  | DigitalSig|  | DigitalSig  |                |
|   +-----------+  +-----------+  +-------------+                |
|                                                                 |
+=================================================================+
|                      TLS CHAIN                                  |
+=================================================================+
|                                                                 |
|   TLS Root CA (Self-Signed)                                     |
|   +-------------------------------+                             |
|   | CN: tlsMyOrgCA                |                             |
|   | Issuer: tlsMyOrgCA (self)     |                             |
|   | IsCA: true                    |                             |
|   +-------------------------------+                             |
|          |              |              |                        |
|          v              v              v                        |
|   +-----------+  +-----------+  +-------------+                |
|   | TLS Cert  |  | TLS Cert  |  | TLS Cert    |                |
|   | server.crt|  | server.crt|  | client.crt  |                |
|   | orderer0  |  | orderer1  |  | Admin@...   |                |
|   | ExtKey:   |  | ExtKey:   |  | ExtKey:     |                |
|   | ServerAuth|  | ServerAuth|  | ClientAuth  |                |
|   | ClientAuth|  | ClientAuth|  | ServerAuth  |                |
|   +-----------+  +-----------+  +-------------+                |
|                                                                 |
+=================================================================+
```

### Certificate Fields

The following table shows the X.509 fields that cryptogen sets on each generated
certificate. The "Value" column shows how the field is determined -- some come from
your config, some are computed, and some are hardcoded defaults.

| Field | Value |
|---|---|
| Serial Number | Random 128-bit integer |
| Not Before | Current time - 5 minutes (rounded to the minute) |
| Not After | Not Before + 10 years (3650 days) |
| Subject.Country | CA's Country (default: `US`) |
| Subject.Province | CA's Province (default: `California`) |
| Subject.Locality | CA's Locality (default: `San Francisco`) |
| Subject.Organization | `OrgSpec.Domain` |
| Subject.OrganizationalUnit | Node's OU (`orderer`, `peer`, `admin`, or `client`) |
| Subject.CommonName | Resolved CN from template or explicit value |
| DNS SANs | CN + Hostname + any custom SANS entries |
| IP SANs | Any IP addresses from SANS entries |

---

## NodeOUs and Role-Based Access

`EnableNodeOUs` is a boolean flag on each organization that controls how Fabric
determines the role of a certificate holder. This is one of the most important
configuration decisions you will make, because it affects how admin privileges
are established and how the network distinguishes between peers, orderers, admins,
and clients.

### Without NodeOUs (EnableNodeOUs: false)

When NodeOUs are disabled, Fabric uses a simple file-based approach to identify
administrators: a certificate is considered an admin if and only if it appears in
the `admincerts/` directory of the MSP. All other certificates are treated as
regular members.

Cryptogen implements this by copying the Admin user's signing certificate into the
`admincerts/` folder of the organization's verifying MSP and every node's local MSP.
This means that adding or revoking admin access requires redistributing certificate
files to every node.

```
EnableNodeOUs: false

How roles are determined:
+----------------------------------------------+
| Certificate is in admincerts/ ?  --> ADMIN    |
| Otherwise                        --> MEMBER   |
+----------------------------------------------+

Admin cert gets copied to:
  OrgName/msp/admincerts/Admin@domain-cert.pem
  OrgName/orderers/node0/msp/admincerts/Admin@domain-cert.pem
  OrgName/orderers/node1/msp/admincerts/Admin@domain-cert.pem
  ...
```

### With NodeOUs (EnableNodeOUs: true)

When NodeOUs are enabled, the role is determined by the Organizational Unit (OU)
field embedded directly in the X.509 certificate. There is no need to copy admin
certificates around -- the certificate itself declares what role its holder has.

Cryptogen generates a `config.yaml` file in every MSP directory that tells the Fabric
runtime how to map OU values to roles. This file maps four OU strings to four role
types:

```
EnableNodeOUs: true

How roles are determined:
+----------------------------------------------+
| OU in certificate = "admin"    --> ADMIN      |
| OU in certificate = "client"   --> CLIENT     |
| OU in certificate = "peer"     --> PEER       |
| OU in certificate = "orderer"  --> ORDERER    |
+----------------------------------------------+
```

The generated `config.yaml` placed in each MSP directory looks like this:

```yaml
NodeOUs:
  Enable: true
  ClientOUIdentifier:
    Certificate: cacerts/ca.cert.pem
    OrganizationalUnitIdentifier: client
  PeerOUIdentifier:
    Certificate: cacerts/ca.cert.pem
    OrganizationalUnitIdentifier: peer
  AdminOUIdentifier:
    Certificate: cacerts/ca.cert.pem
    OrganizationalUnitIdentifier: admin
  OrdererOUIdentifier:
    Certificate: cacerts/ca.cert.pem
    OrganizationalUnitIdentifier: orderer
```

### Recommendation

**`EnableNodeOUs: true` is strongly recommended for production networks.** It
provides finer-grained access control (four distinct roles instead of just
admin/member), eliminates the need to distribute admin certificates to every node,
and encodes the role directly in the certificate so that access control decisions
are self-contained.

The only reason to use `EnableNodeOUs: false` is compatibility with legacy Fabric
networks that were set up before NodeOU support was added.

---

## Key Algorithms

Cryptogen supports two public key algorithms for generating key pairs. The algorithm
can be configured independently for CAs, nodes, and users. All private keys are
stored in PKCS8 PEM format in a file named `priv_sk`.

### Algorithm Comparison

| Property | ECDSA | Ed25519 |
|---|---|---|
| Curve | P-256 (secp256r1) | Curve25519 |
| Key size | 256 bits | 256 bits |
| Signature size | ~72 bytes (variable) | 64 bytes (fixed) |
| Standard | NIST (FIPS 186-4) | IETF (RFC 8032) |
| Signature normalization | Low-S required by Fabric (BIP-0146) | Not needed |
| Status | **Default** | Alternative |

ECDSA is the default because it has the longest history in Fabric and the broadest
ecosystem support. Ed25519 is a modern alternative that offers simpler implementation
(no signature malleability concerns) and deterministic signatures.

When using ECDSA, cryptogen automatically applies "Low-S normalization" to all
signatures. This is a Fabric-specific requirement: both `(r, s)` and
`(r, -s mod n)` are valid ECDSA signatures for the same message, so Fabric
normalizes to the canonical form where `s <= n/2` to prevent signature malleability
attacks.

### Setting the Algorithm

The key algorithm can be set at multiple levels. More specific settings override
less specific ones. If no algorithm is specified at any level, the default `"ecdsa"`
is used.

```
Priority (highest to lowest):
+-------------------------------------------------------+
| 1. NodeSpec.PublicKeyAlgorithm  (per-node override)   |
| 2. Template.PublicKeyAlgorithm  (for templated nodes) |
| 3. Users.PublicKeyAlgorithm     (for users)           |
| 4. CA.PublicKeyAlgorithm        (for CA keys)         |
| 5. Default: "ecdsa"                                   |
+-------------------------------------------------------+
```

Note that the CA's algorithm and a node's algorithm are independent. You can have
an ECDSA CA that signs Ed25519 node certificates, or vice versa. The algorithm
setting controls the key pair generated for the entity, not the signature algorithm
used by the CA.

### Mixed Algorithm Example

The following config shows how different entities in the same organization can use
different key algorithms:

```yaml
OrdererOrgs:
  - Name: OrdererOrg
    Domain: example.com
    CA:
      PublicKeyAlgorithm: ecdsa      # CA uses ECDSA
    Template:
      Count: 2
      PublicKeyAlgorithm: ecdsa      # Orderer nodes use ECDSA
    Specs:
      - Hostname: orderer-special
        PublicKeyAlgorithm: ed25519  # This one node uses Ed25519
```

---

## Extending an Existing Network

As your network grows, you may need to add new orderer nodes, peer nodes, or users
to existing organizations. The `extend` command lets you do this without regenerating
or disturbing any existing crypto material.

### How Extend Works

The extend process follows a "create if missing, skip if present" strategy:

1. **Load existing CAs** -- The tool reads the CA private key and certificate from
   the existing `ca/` and `tlsca/` directories. It does NOT generate new CAs.
2. **Process each node/user** -- For every node and user defined in the config, the
   tool checks whether a directory already exists for that entity.
3. **Skip existing** -- If the directory exists, the entity is left completely
   untouched. No keys are regenerated, no certificates are re-signed.
4. **Generate new** -- If the directory does not exist, new key material is generated
   and signed by the existing CA.
5. **Handle new orgs** -- If an entire organization does not exist yet, extend
   behaves identically to `generate` for that org (creating CAs from scratch).

```
BEFORE extend:                    Config for extend:
+---------------------+          +---------------------+
| orderers/           |          | Template:           |
|   orderer0/         |          |   Count: 3          |  <-- was 2
|   orderer1/         |          | Users:              |
| users/              |          |   Count: 2          |  <-- was 1
|   Admin@.../        |          |   Specs:            |
|   User1@.../        |          |     - Name: audit   |  <-- new
+---------------------+          +---------------------+

AFTER extend:
+---------------------+
| orderers/           |
|   orderer0/         |  <-- unchanged (skipped)
|   orderer1/         |  <-- unchanged (skipped)
|   orderer2/         |  <-- NEW
| users/              |
|   Admin@.../        |  <-- unchanged (skipped)
|   User1@.../        |  <-- unchanged (skipped)
|   User2@.../        |  <-- NEW
|   audit@.../        |  <-- NEW
+---------------------+
```

### Usage

```bash
# Extend with a config that has more nodes/users than the original
cryptogen extend --config ./extended-config.yaml --input ./crypto-config
```

The `--input` flag points to the existing crypto output directory (the same directory
you originally passed to `--output` during `generate`).

### Important Notes About Extend

- **CA keys must be present.** The existing `ca/priv_sk` and `tlsca/priv_sk` files
  must be intact. If they have been deleted (e.g., for security hardening in a
  CA-managed environment), extend will fail.

- **Extend never overwrites.** If a node or user directory already exists, it is
  skipped entirely -- even if the config has changed for that entity.

- **New orgs are fully generated.** If the config references an organization whose
  directory does not exist in the input, extend creates it from scratch, including
  new CAs.

- **Admin certs are re-copied.** When `EnableNodeOUs` is `false`, the admin
  certificate is re-copied to all node `admincerts/` directories, ensuring that
  newly added nodes also recognize the admin.

---

## Complete Configuration Examples

### Example 1: Simple Two-Org Network

This is the simplest production-like configuration: one orderer organization with
a single orderer node, and one peer organization with two peers and one application
user. NodeOUs are enabled on the peer org for role-based access control.

```yaml
# Two organizations: one orderer, one peer
OrdererOrgs:
  - Name: OrdererOrg
    Domain: example.com
    Template:
      Count: 1

PeerOrgs:
  - Name: Org1
    Domain: org1.example.com
    EnableNodeOUs: true
    Template:
      Count: 2
    Users:
      Count: 1
```

This produces the following output tree:

```
crypto-config/
+-- ordererOrganizations/
|     +-- OrdererOrg/
|           +-- ca/
|           +-- tlsca/
|           +-- msp/
|           +-- orderers/
|           |     +-- orderer0.example.com/
|           +-- users/
|                 +-- Admin@example.com/
|
+-- peerOrganizations/
      +-- Org1/
            +-- ca/
            +-- tlsca/
            +-- msp/    (with config.yaml for NodeOUs)
            +-- peers/
            |     +-- peer0.org1.example.com/
            |     +-- peer1.org1.example.com/
            +-- users/
                  +-- Admin@org1.example.com/
                  +-- User1@org1.example.com/
```

### Example 2: Production-Like Multi-Org Network

A more realistic configuration with a 5-node ordering service, two peer organizations
with named users, and TLS SANs for localhost development. This is representative of a
supply chain network with a manufacturer and retailer.

```yaml
OrdererOrgs:
  - Name: OrdererOrg
    Domain: orderer.example.com
    EnableNodeOUs: true
    CA:
      Hostname: ca
      CommonName: OrdererOrgCA
      Country: US
      Province: New York
      Locality: New York
      PublicKeyAlgorithm: ecdsa
    Template:
      Count: 5
      Hostname: "{{.Prefix}}{{.Index}}.{{.Domain}}"
      SANS:
        - "{{.Hostname}}"
        - "localhost"
        - "127.0.0.1"

PeerOrgs:
  - Name: Manufacturer
    Domain: manufacturer.example.com
    EnableNodeOUs: true
    CA:
      CommonName: ManufacturerCA
      PublicKeyAlgorithm: ecdsa
    Template:
      Count: 2
      SANS:
        - "localhost"
        - "127.0.0.1"
    Users:
      Count: 2
      Specs:
        - Name: inspector
        - Name: shipper

  - Name: Retailer
    Domain: retailer.example.com
    EnableNodeOUs: true
    CA:
      CommonName: RetailerCA
      PublicKeyAlgorithm: ecdsa
    Template:
      Count: 2
      SANS:
        - "localhost"
    Users:
      Count: 1
```

### Example 3: GenericOrg with Multi-Party Nodes

This advanced configuration shows a consortium where multiple parties share a single
organizational CA but operate their own nodes. The `Party` field groups nodes by
operator, and the `OrganizationalUnit` field assigns roles. This pattern is useful
for joint ventures or industry consortiums where trust is shared but operations are
distributed.

```yaml
GenericOrgs:
  - Name: ConsortiumOrg
    Domain: consortium.example.com
    EnableNodeOUs: true
    CA:
      CommonName: ConsortiumCA
      PublicKeyAlgorithm: ecdsa

    # GenericOrgs require explicit Specs (no Template)
    Specs:
      # Party A operates orderer nodes
      - Hostname: orderer-a1
        CommonName: orderer-a1.consortium.example.com
        OrganizationalUnit: orderer
        Party: party-a

      - Hostname: orderer-a2
        CommonName: orderer-a2.consortium.example.com
        OrganizationalUnit: orderer
        Party: party-a

      # Party B also operates orderer nodes
      - Hostname: orderer-b1
        CommonName: orderer-b1.consortium.example.com
        OrganizationalUnit: orderer
        Party: party-b

      # Party A also has a peer
      - Hostname: peer-a1
        CommonName: peer-a1.consortium.example.com
        OrganizationalUnit: peer
        Party: party-a

    Users:
      Specs:
        - Name: operator-a
        - Name: operator-b
```

This produces the following tree, where nodes are grouped by party:

```
crypto-config/
+-- organizations/
      +-- ConsortiumOrg/
            +-- ca/
            +-- tlsca/
            +-- msp/
            +-- orderers/
            |     +-- party-a/
            |     |     +-- orderer-a1.consortium.example.com/
            |     |     +-- orderer-a2.consortium.example.com/
            |     +-- party-b/
            |           +-- orderer-b1.consortium.example.com/
            +-- peers/
            |     +-- party-a/
            |           +-- peer-a1.consortium.example.com/
            +-- users/
                  +-- Admin@consortium.example.com/
                  +-- operator-a@consortium.example.com/
                  +-- operator-b@consortium.example.com/
```

---

## Troubleshooting

### Common Issues

The following table lists the most common errors you may encounter when using
cryptogen, along with their causes and fixes:

| Problem | Cause | Fix |
|---|---|---|
| `"hostname is empty"` | `Template.Hostname` evaluates to an empty string | Check your hostname template variables -- make sure `{{.Prefix}}`, `{{.Index}}`, and `{{.Domain}}` are spelled correctly |
| `"unsupported key algorithm: xyz"` | `PublicKeyAlgorithm` is not `"ecdsa"` or `"ed25519"` | Use lowercase `"ecdsa"` or `"ed25519"` exactly |
| Extend fails with `"no PEM blocks found"` | CA private key or certificate is missing or corrupted | Ensure `ca/priv_sk` and `ca/*-cert.pem` exist and are valid PEM files |
| Node name collision | Template and Specs produce nodes with the same hostname | Adjust `Template.Start` or change Specs hostnames to avoid overlap |
| GenericOrg nodes in wrong directory | `OrganizationalUnit` not set on Specs entries | Set `OrganizationalUnit: "orderer"` or `"peer"` explicitly on every GenericOrg spec |
| `"error parsing template"` | Invalid Go template syntax in Hostname, CommonName, or SANS | Check for unmatched `{{` `}}` braces, typos in variable names |

### Verifying Generated Certificates

After generating crypto material, you can use `openssl` to inspect the output and
verify correctness:

```bash
# View a certificate's full details (subject, issuer, validity, SANs, key usage)
openssl x509 -in crypto-config/peerOrganizations/Org1/peers/peer0/msp/signcerts/peer0-cert.pem \
  -text -noout

# Verify a certificate was signed by the organization's CA
openssl verify \
  -CAfile crypto-config/peerOrganizations/Org1/ca/ca.Org1CA-cert.pem \
  crypto-config/peerOrganizations/Org1/peers/peer0/msp/signcerts/peer0-cert.pem

# Check a TLS certificate's SANs and extended key usage
openssl x509 -in crypto-config/peerOrganizations/Org1/peers/peer0/tls/server.crt \
  -text -noout

# Inspect a private key's algorithm and parameters
openssl pkey -in crypto-config/peerOrganizations/Org1/peers/peer0/msp/keystore/priv_sk \
  -text -noout
```

---

## Appendix: Complete Data Model

This tree shows every configurable type and field in the config file at a glance.
Use it as a quick reference when writing your configuration.

```
config_crypto.yaml
|
+-- OrdererOrgs: []OrgSpec
|
+-- PeerOrgs: []OrgSpec
|
+-- GenericOrgs: []OrgSpec
      |
      +-- OrgSpec
            |
            +-- Name: string                 # Organization name
            +-- Domain: string               # DNS domain
            +-- EnableNodeOUs: bool          # Role-based identity
            |
            +-- CA: NodeSpec                 # Certificate Authority
            |     +-- Hostname: string
            |     +-- CommonName: string
            |     +-- Country: string
            |     +-- Province: string
            |     +-- Locality: string
            |     +-- OrganizationalUnit: string
            |     +-- StreetAddress: string
            |     +-- PostalCode: string
            |     +-- SANS: []string
            |     +-- PublicKeyAlgorithm: string
            |     +-- Party: string
            |
            +-- Template: NodeTemplate       # Auto-generate nodes
            |     +-- Count: int
            |     +-- Start: int
            |     +-- Hostname: string       # Go template
            |     +-- SANS: []string         # Go templates
            |     +-- PublicKeyAlgorithm: string
            |
            +-- Specs: []NodeSpec            # Explicit nodes
            |     +-- (same fields as CA)
            |
            +-- Users: UsersSpec             # User accounts
                  +-- Count: int
                  +-- PublicKeyAlgorithm: string
                  +-- Specs: []UserSpec
                        +-- Name: string
                        +-- PublicKeyAlgorithm: string
```
