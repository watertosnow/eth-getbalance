package main

import (
	"bufio"
	"context"
	"errors"
	"eth-getbalance/config"
	"fmt"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"log"
	"math/big"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type AddressBalance struct {
	Address string
	Balance uint64
}

var contractAddress = common.HexToAddress("0xcA11bde05977b3631167028862bE2a173976CA11")

type AggregateCall3 struct {
	Target       common.Address
	AllowFailure bool
	CallData     []byte
}

type Aggregate3Result struct {
	Success    bool
	ReturnData []byte
}

func Aggregate3Read(client *ethclient.Client, multicall3Abi abi.ABI, calls []AggregateCall3) (results []Aggregate3Result, err error) {

	calldata, err := multicall3Abi.Pack("aggregate3", calls)
	if err != nil {
		return nil, errors.New("Failed to pack data: " + err.Error())
	}

	result, err := client.CallContract(context.Background(), ethereum.CallMsg{
		To:   &contractAddress,
		Data: calldata,
	}, nil)
	if err != nil {
		return nil, errors.New("Failed to call contract: " + err.Error())
	}

	parsedResult, err := multicall3Abi.Unpack("aggregate3", result)
	if err != nil {
		return nil, errors.New("Failed to unpack data: " + err.Error())
	}

	for _, v := range parsedResult {
		// fmt.Printf("%T\n", v)
		for _, vv := range v.([]struct {
			Success    bool    "json:\"success\""
			ReturnData []uint8 "json:\"returnData\""
		}) {
			results = append(results, Aggregate3Result{
				Success:    vv.Success,
				ReturnData: vv.ReturnData,
			})
		}
	}

	return results, nil
}

type EthBlockScanner struct {
	client     *ethclient.Client
	parsedABI  abi.ABI
	hasBalance []AddressBalance
}

func NewEthBlockScanner(clientUrl string, apikey string, parsedABI abi.ABI) *EthBlockScanner {
	var client *ethclient.Client
	var err error
	if apikey != "" {
		rpcClient, err := rpc.DialOptions(context.Background(), clientUrl, rpc.WithHeader("x-api-key", apikey))
		if err != nil {
			log.Fatalf("Failed to create RPC client: %s", err)
		}
		client = ethclient.NewClient(rpcClient)
	} else {
		client, err = ethclient.Dial(clientUrl)
	}

	if err != nil {
		log.Fatalf("Failed to connect to the Ethereum client: %v", err)
	}
	return &EthBlockScanner{client, parsedABI, []AddressBalance{}}
}

func (self *EthBlockScanner) GetEthBalance(addresses []string) uint64 {
	// 构造调用参数
	calls := []AggregateCall3{}

	var addressToQuery common.Address
	// 逐行读取并处理每个地址
	for _, address := range addresses {
		// 将地址转换为 common.Address 类型
		addressToQuery = common.HexToAddress(address)
		calldata, err := self.parsedABI.Pack("getEthBalance", addressToQuery)
		if err != nil {
			log.Fatal("Failed to pack data: " + err.Error())
		}

		calls = append(calls, AggregateCall3{
			Target:       contractAddress,
			AllowFailure: false,
			CallData:     calldata,
		})
	}

	// 调用智能合约
	results, err := Aggregate3Read(self.client, self.parsedABI, calls)
	if err != nil {
		log.Fatal("Failed to call contract: " + err.Error())
	}

	// 打印结果
	for i, result := range results {
		if result.Success {
			// 转换为big integer
			balance := new(big.Int).SetBytes(result.ReturnData)
			// 如果余额大于0
			if balance.Cmp(big.NewInt(0)) > 0 {
				self.hasBalance = append(self.hasBalance, AddressBalance{addresses[i], balance.Uint64()})
			}
			fmt.Printf("%v, Balance: %v\n", addresses[i], balance)
		} else {
			// 不应该发生
			log.Fatal("Call %d failed\n %v", i, addresses[i])
		}
	}
	return 0
}

func WeiToEth(wei uint64) *big.Float {
	// 1 ETH = 10^18 Wei
	ethValue := new(big.Float).SetUint64(wei)
	weiPerEth := new(big.Float).SetInt(big.NewInt(1e18))
	ethValue.Quo(ethValue, weiPerEth)
	return ethValue
}

func main() {
	start := time.Now()

	conf := config.GetConfig("conf")
	allScanner := []*EthBlockScanner{}

	// 智能合约地址和 ABI
	contractABI := `[{"inputs":[{"components":[{"internalType":"address","name":"target","type":"address"},{"internalType":"bytes","name":"callData","type":"bytes"}],"internalType":"struct Multicall3.Call[]","name":"calls","type":"tuple[]"}],"name":"aggregate","outputs":[{"internalType":"uint256","name":"blockNumber","type":"uint256"},{"internalType":"bytes[]","name":"returnData","type":"bytes[]"}],"stateMutability":"payable","type":"function"},{"inputs":[{"components":[{"internalType":"address","name":"target","type":"address"},{"internalType":"bool","name":"allowFailure","type":"bool"},{"internalType":"bytes","name":"callData","type":"bytes"}],"internalType":"struct Multicall3.Call3[]","name":"calls","type":"tuple[]"}],"name":"aggregate3","outputs":[{"components":[{"internalType":"bool","name":"success","type":"bool"},{"internalType":"bytes","name":"returnData","type":"bytes"}],"internalType":"struct Multicall3.Result[]","name":"returnData","type":"tuple[]"}],"stateMutability":"payable","type":"function"},{"inputs":[{"components":[{"internalType":"address","name":"target","type":"address"},{"internalType":"bool","name":"allowFailure","type":"bool"},{"internalType":"uint256","name":"value","type":"uint256"},{"internalType":"bytes","name":"callData","type":"bytes"}],"internalType":"struct Multicall3.Call3Value[]","name":"calls","type":"tuple[]"}],"name":"aggregate3Value","outputs":[{"components":[{"internalType":"bool","name":"success","type":"bool"},{"internalType":"bytes","name":"returnData","type":"bytes"}],"internalType":"struct Multicall3.Result[]","name":"returnData","type":"tuple[]"}],"stateMutability":"payable","type":"function"},{"inputs":[{"components":[{"internalType":"address","name":"target","type":"address"},{"internalType":"bytes","name":"callData","type":"bytes"}],"internalType":"struct Multicall3.Call[]","name":"calls","type":"tuple[]"}],"name":"blockAndAggregate","outputs":[{"internalType":"uint256","name":"blockNumber","type":"uint256"},{"internalType":"bytes32","name":"blockHash","type":"bytes32"},{"components":[{"internalType":"bool","name":"success","type":"bool"},{"internalType":"bytes","name":"returnData","type":"bytes"}],"internalType":"struct Multicall3.Result[]","name":"returnData","type":"tuple[]"}],"stateMutability":"payable","type":"function"},{"inputs":[],"name":"getBasefee","outputs":[{"internalType":"uint256","name":"basefee","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"uint256","name":"blockNumber","type":"uint256"}],"name":"getBlockHash","outputs":[{"internalType":"bytes32","name":"blockHash","type":"bytes32"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"getBlockNumber","outputs":[{"internalType":"uint256","name":"blockNumber","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"getChainId","outputs":[{"internalType":"uint256","name":"chainid","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"getCurrentBlockCoinbase","outputs":[{"internalType":"address","name":"coinbase","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"getCurrentBlockDifficulty","outputs":[{"internalType":"uint256","name":"difficulty","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"getCurrentBlockGasLimit","outputs":[{"internalType":"uint256","name":"gaslimit","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"getCurrentBlockTimestamp","outputs":[{"internalType":"uint256","name":"timestamp","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"addr","type":"address"}],"name":"getEthBalance","outputs":[{"internalType":"uint256","name":"balance","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"getLastBlockHash","outputs":[{"internalType":"bytes32","name":"blockHash","type":"bytes32"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"bool","name":"requireSuccess","type":"bool"},{"components":[{"internalType":"address","name":"target","type":"address"},{"internalType":"bytes","name":"callData","type":"bytes"}],"internalType":"struct Multicall3.Call[]","name":"calls","type":"tuple[]"}],"name":"tryAggregate","outputs":[{"components":[{"internalType":"bool","name":"success","type":"bool"},{"internalType":"bytes","name":"returnData","type":"bytes"}],"internalType":"struct Multicall3.Result[]","name":"returnData","type":"tuple[]"}],"stateMutability":"payable","type":"function"},{"inputs":[{"internalType":"bool","name":"requireSuccess","type":"bool"},{"components":[{"internalType":"address","name":"target","type":"address"},{"internalType":"bytes","name":"callData","type":"bytes"}],"internalType":"struct Multicall3.Call[]","name":"calls","type":"tuple[]"}],"name":"tryBlockAndAggregate","outputs":[{"internalType":"uint256","name":"blockNumber","type":"uint256"},{"internalType":"bytes32","name":"blockHash","type":"bytes32"},{"components":[{"internalType":"bool","name":"success","type":"bool"},{"internalType":"bytes","name":"returnData","type":"bytes"}],"internalType":"struct Multicall3.Result[]","name":"returnData","type":"tuple[]"}],"stateMutability":"payable","type":"function"}]`
	// 解析智能合约 ABI
	parsedABI, err := abi.JSON(strings.NewReader(contractABI))
	if err != nil {
		log.Fatal(err)
	}

	for i := range conf.App.Num {
		index := int(i) % len(conf.Client.Uri)
		allScanner = append(allScanner, NewEthBlockScanner(conf.Client.Uri[index], conf.Client.Key[index], parsedABI))
	}

	var doneFlag int64
	var count int64
	// channel
	addressChan := make(chan []string)
	hasBalanceChan := make(chan []AddressBalance, len(allScanner))
	var wg sync.WaitGroup

	for _, scanner := range allScanner {
		go func() {
			wg.Add(1)
			for {
				breakFlag := false
				select {
				case addresses := <-addressChan:
					{
						scanner.GetEthBalance(addresses)
						atomic.AddInt64(&count, int64(len(addresses)))
						if len(scanner.hasBalance) > 0 || atomic.LoadInt64(&doneFlag) > 0 {
							hasBalanceChan <- scanner.hasBalance
							atomic.AddInt64(&doneFlag, 1)

							breakFlag = true
						}
					}
				case <-time.After(1 * time.Second):
					if atomic.LoadInt64(&doneFlag) > 0 {
						hasBalanceChan <- scanner.hasBalance
						atomic.AddInt64(&doneFlag, 1)
						breakFlag = true
					}
				}
				if breakFlag {
					break
				}
			}

			wg.Done()
		}()
	}

	// 读取地址
	// 打开文本文件
	file, err := os.Open(conf.App.Filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	// 创建一个 Scanner 对象来逐行读取文件内容
	scanner := bufio.NewScanner(file)

	addresses := []string{}
	i := 1
	// 逐行读取并处理每个地址
	for scanner.Scan() {
		address := scanner.Text()
		addresses = append(addresses, address)
		if i%999 == 0 {
			i = 1
			if atomic.LoadInt64(&doneFlag) > 0 {
				break
			}
			elapsed := time.Since(start)
			fmt.Printf("%v 已执行完: %d\n", elapsed, atomic.LoadInt64(&count))
			for {
				breakFlag := false
				select {
				case addressChan <- addresses:
					addresses = []string{}
					breakFlag = true
				case <-time.After(1 * time.Second):
					{
						fmt.Printf("等待中...\n")
						if atomic.LoadInt64(&doneFlag) > 0 {
							breakFlag = true
						}
					}
				}
				if breakFlag {
					break
				}
			}

			if atomic.LoadInt64(&doneFlag) > 0 {
				break
			}
		} else {
			i++
		}
	}
	if i > 0 && atomic.LoadInt64(&doneFlag) == 0 {
		select {
		case addressChan <- addresses:
		case <-time.After(1 * time.Second):

		}
	}

	atomic.AddInt64(&doneFlag, 1)
	wg.Wait()
	close(addressChan)
	fmt.Printf("====================================\n")
	fmt.Printf("====================================\n")
	fmt.Printf("已执行完: %d\n", atomic.LoadInt64(&count))
	fmt.Printf("有余额的地址: \n")

	for {
		breakFlag := false
		select {
		case hasBalanceAddresses := <-hasBalanceChan:
			for _, address := range hasBalanceAddresses {
				fmt.Printf("%s, Balance: %.18f \n", address.Address, WeiToEth(address.Balance))
			}
		default:
			breakFlag = true
		}
		if breakFlag {
			break
		}
	}

	fmt.Printf("====================================\n")
	elapsed := time.Since(start)
	fmt.Println("Time elapsed:", elapsed)
}
