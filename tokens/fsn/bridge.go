// Package fsn implements the bridge interfaces for fsn blockchain.
package fsn

import (
	"math/big"
	"strings"
	"time"

	"github.com/acentswap/bridge/log"
	"github.com/acentswap/bridge/tokens"
	"github.com/acentswap/bridge/tokens/eth"
	"github.com/acentswap/bridge/types"
)

// Bridge fsn bridge inherit from eth bridge
type Bridge struct {
	*eth.Bridge
}

// NewCrossChainBridge new fsn bridge
func NewCrossChainBridge(isSrc bool) *Bridge {
	bridge := &Bridge{Bridge: eth.NewCrossChainBridge(isSrc)}
	bridge.Inherit = bridge
	return bridge
}

// SetChainAndGateway set token and gateway config
func (b *Bridge) SetChainAndGateway(chainCfg *tokens.ChainConfig, gatewayCfg *tokens.GatewayConfig) {
	b.CrossChainBridgeBase.SetChainAndGateway(chainCfg, gatewayCfg)
	b.VerifyChainID()
	b.Init()
}

// VerifyChainID verify chain id
func (b *Bridge) VerifyChainID() {
	networkID := strings.ToLower(b.ChainConfig.NetID)
	targetChainID := eth.GetChainIDOfNetwork(eth.FsnNetworkAndChainIDMap, networkID)
	isCustom := eth.IsCustomNetwork(networkID)
	if !isCustom && targetChainID == nil {
		log.Fatalf("unsupported fusion network: %v", b.ChainConfig.NetID)
	}

	var (
		chainID *big.Int
		err     error
	)

	for {
		chainID, err = b.GetSignerChainID()
		if err == nil {
			break
		}
		log.Errorf("can not get gateway chainID. %v", err)
		log.Println("retry query gateway", b.GatewayConfig.APIAddress)
		time.Sleep(3 * time.Second)
	}

	if !isCustom && chainID.Cmp(targetChainID) != 0 {
		log.Fatalf("gateway chainID '%v' is not '%v'", chainID, b.ChainConfig.NetID)
	}

	b.SignerChainID = chainID
	b.Signer = types.MakeSigner("EIP155", chainID)

	log.Info("VerifyChainID succeed", "networkID", networkID, "chainID", chainID)
}
