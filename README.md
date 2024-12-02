# FabricX Legacy Config

This library encapsulates all the config handling code from Hyperledger Fabric.

### API
* Create config block from config TX (RW set)
* Validate an update is valid according to update policy
  with respect to the previous config block
    * Orderer
* Fetch required fields
    * Orderer
    * Committer
    * Endorser


### Sources

#### Locked Versions
* [Hyperledger Fabric](https://github.com/hyperledger/fabric) v3.0.0-rc1
* [Hyperledger Fabric Config](https://github.com/hyperledger/fabric-config) v0.3.0

#### Code Fetched

* **API Source**: [Hyperledger Fabric Config](https://github.com/hyperledger/fabric-config)
    - Import

* **Use Source**: [Hyperledger Fabric](https://github.com/hyperledger/fabric)
    - hyperledger/fabric/msp
    - hyperledger/fabric/common/channelconfig
        - bundle.go: (b *Bundle) ValidateNew(nb Resources)
    - hyperledger/fabric/common/capabilities
    - hyperledger/fabric/common/configtx
    - hyperledger/fabric/common/policies

* **Tool Source**: [Hyperledger Fabric](https://github.com/hyperledger/fabric)
    - hyperledger/fabric/cmd/configtxgen
    - hyperledger/fabric/cmd/configtxlator
    - hyperledger/fabric/cmd/osnadmin
