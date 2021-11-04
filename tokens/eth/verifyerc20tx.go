package eth

import (
	"bytes"
	"errors"
	"math/big"
	"strings"

	"github.com/acentswap/bridge/common"
	"github.com/acentswap/bridge/log"
	"github.com/acentswap/bridge/params"
	"github.com/acentswap/bridge/tokens"
	"github.com/acentswap/bridge/tokens/tools"
	"github.com/acentswap/bridge/types"
)

// verifyErc20SwapinTx verify erc20 swapin with pairID
func (b *Bridge) verifyErc20SwapinTx(swapInfo *tokens.TxSwapInfo, allowUnstable bool, token *tokens.TokenConfig, receipt *types.RPCTxReceipt) (*tokens.TxSwapInfo, error) {
	err := b.verifyErc20SwapinTxReceipt(swapInfo, receipt, token)
	if err != nil {
		return swapInfo, err
	}

	err = b.checkSwapinInfo(swapInfo)
	if err != nil {
		return swapInfo, err
	}

	if !allowUnstable {
		log.Info("verify erc20 swapin stable pass",
			"identifier", params.GetIdentifier(), "pairID", swapInfo.PairID,
			"from", swapInfo.From, "to", swapInfo.To, "bind", swapInfo.Bind,
			"value", swapInfo.Value, "txid", swapInfo.Hash,
			"height", swapInfo.Height, "timestamp", swapInfo.Timestamp)
	}
	return swapInfo, nil
}

func (b *Bridge) verifyErc20SwapinTxReceipt(swapInfo *tokens.TxSwapInfo, receipt *types.RPCTxReceipt, token *tokens.TokenConfig) error {
	if receipt.Recipient == nil {
		return tokens.ErrTxWithWrongContract
	}

	swapInfo.TxTo = strings.ToLower(receipt.Recipient.String()) // TxTo
	swapInfo.From = strings.ToLower(receipt.From.String())      // From

	if !token.AllowSwapinFromContract &&
		!common.IsEqualIgnoreCase(swapInfo.TxTo, token.ContractAddress) &&
		!b.ChainConfig.IsInCallByContractWhitelist(swapInfo.TxTo) {
		return tokens.ErrTxWithWrongContract
	}

	from, to, value, err := ParseErc20SwapinTxLogs(receipt.Logs, token.ContractAddress, token.DepositAddress)
	if err != nil {
		if !errors.Is(err, tokens.ErrTxWithWrongReceiver) {
			log.Debug(b.ChainConfig.BlockChain+" ParseErc20SwapinTxLogs failed", "tx", swapInfo.Hash, "err", err)
		}
		return err
	}
	swapInfo.To = strings.ToLower(to)     // To
	swapInfo.Value = value                // Value
	swapInfo.Bind = strings.ToLower(from) // Bind
	return nil
}

// ParseErc20SwapinTxLogs parse erc20 swapin tx logs
func ParseErc20SwapinTxLogs(logs []*types.RPCLog, contractAddress, checkToAddress string) (from, to string, value *big.Int, err error) {
	transferLogExist := false
	for _, log := range logs {
		if log.Removed != nil && *log.Removed {
			continue
		}
		if !common.IsEqualIgnoreCase(log.Address.String(), contractAddress) {
			continue
		}
		if len(log.Topics) != 3 || log.Data == nil {
			continue
		}
		if !bytes.Equal(log.Topics[0][:], erc20CodeParts["LogTransfer"]) {
			continue
		}
		transferLogExist = true
		to = common.BytesToAddress(log.Topics[2][:]).String()
		if !common.IsEqualIgnoreCase(to, checkToAddress) {
			continue
		}
		from = common.BytesToAddress(log.Topics[1][:]).String()
		value = common.GetBigInt(*log.Data, 0, 32)
		return from, to, value, nil
	}
	if transferLogExist {
		err = tokens.ErrTxWithWrongReceiver
	} else {
		err = tokens.ErrDepositLogNotFound
	}
	return "", "", nil, err
}

func (b *Bridge) checkSwapinInfo(swapInfo *tokens.TxSwapInfo) error {
	if swapInfo.Bind == swapInfo.To {
		return tokens.ErrTxWithWrongSender
	}
	if !tokens.CheckSwapValue(swapInfo.PairID, swapInfo.Value, b.IsSrc) {
		return tokens.ErrTxWithWrongValue
	}
	token := b.GetTokenConfig(swapInfo.PairID)
	if token == nil {
		return tokens.ErrUnknownPairID
	}
	return b.checkSwapinBindAddress(swapInfo.Bind, token.AllowSwapinFromContract)
}

func (b *Bridge) checkSwapinBindAddress(bindAddr string, allowContractAddress bool) error {
	if !tokens.DstBridge.IsValidAddress(bindAddr) {
		log.Warn("wrong bind address in swapin", "bind", bindAddr)
		return tokens.ErrTxWithWrongMemo
	}
	if params.MustRegisterAccount() && !tools.IsAddressRegistered(bindAddr) {
		return tokens.ErrTxSenderNotRegistered
	}
	if params.IsSwapServer && !allowContractAddress &&
		!b.ChainConfig.IsInCallByContractWhitelist(bindAddr) {
		isContract, err := b.IsContractAddress(bindAddr)
		if err != nil {
			log.Warn("query is contract address failed", "bindAddr", bindAddr, "err", err)
			return tokens.ErrRPCQueryError
		}
		if isContract {
			return tokens.ErrBindAddrIsContract
		}
	}
	return nil
}