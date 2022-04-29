package bot

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/thoas/go-funk"
	"log"
	"math/big"
	"os"
	"runtime"
	"time"

	c "github.com/ostafen/clover"

	"github.com/simman/go-swap-trading-bot/config"
	"github.com/simman/go-swap-trading-bot/dao"
	"github.com/simman/go-swap-trading-bot/global"
	"github.com/simman/go-swap-trading-bot/utils"
	"github.com/spf13/viper"
	"github.com/umbracle/ethgo"
	"github.com/umbracle/ethgo/abi"
	"github.com/umbracle/ethgo/blocktracker"
	"github.com/umbracle/ethgo/contract"
	"github.com/umbracle/ethgo/jsonrpc"
	"github.com/umbracle/ethgo/wallet"
)

var (
	//InitCodeHash = ethgo.HexToHash("0x96e8ac4277198ff8b6f785478aa9a39f403cb768dd02cbee326c3e7da348845f")
	//FACTORY      = ethgo.HexToAddress("0xcA143Ce32Fe78f1f7019d7d551a6402fC5350c73")
	ROUTER       = ethgo.HexToAddress("0x10ed43c718714eb63d5aa57b78b54704e256024e")
	PANCAKE_PAIR = ethgo.HexToAddress("0xe7d75E07F3A00b10C390a533C19F5d2D1C9CE313")
	MSC          = ethgo.HexToAddress("0xeacAd6c99965cDE0f31513dd72DE79FA24610767")
	USDT         = ethgo.HexToAddress("0x55d398326f99059fF775485246999027B3197955")
)

type Bot struct {
	wallet        *wallet.Key
	jsonRpcClient *jsonrpc.Client
	ctx           context.Context
	contracts     *Contracts
	lastBlock     *ethgo.Block
	balance       *Balance
	lastMscPrice  float64
	running       bool
}

type Contracts struct {
	router *contract.Contract
	pair   *contract.Contract
	msc    *contract.Contract
	usdt   *contract.Contract
}

type Balance struct {
	MSC  *big.Int
	BNB  *big.Int
	USDT *big.Int
}

var trading = false

func NewBot(context context.Context) *Bot {
	return &Bot{
		ctx:       context,
		contracts: &Contracts{},
		balance:   &Balance{},
	}
}

func (b *Bot) Start() {
	// create wallet
	buf, _ := hex.DecodeString(viper.GetString("wallet.privateKey"))
	if w, err := wallet.NewWalletFromPrivKey(buf); err == nil {
		b.wallet = w
	} else {
		log.Println("create wallet err")
		log.Fatal(err)
	}

	// create rpc client
	if client, err := jsonrpc.NewClient(viper.GetString("node.rpc")); err == nil {
		b.jsonRpcClient = client
	} else {
		log.Println("create rpc client err")
		log.Fatal(err)
	}

	b.initContract()

	go b.loopBalance()

	go b.syncBlock()

	b.running = true
}

func (b *Bot) loopBalance() {
	for {
		if !b.running {
			// if running is false, exit current thread
			runtime.Goexit()
		}
		if balance, err := b.contracts.usdt.Call("balanceOf", ethgo.Latest, b.wallet.Address()); err == nil {
			b.balance.USDT = balance["0"].(*big.Int)
			// log.Printf("usdt: %f", utils.WeiToEther(u["0"].(*big.Int)))
		}

		if balance, err := b.contracts.msc.Call("balanceOf", ethgo.Latest, b.wallet.Address()); err == nil {
			b.balance.MSC = balance["0"].(*big.Int)
			// log.Printf("msc: %f", utils.WeiToEther(u["0"].(*big.Int)))
		}

		if balance, err := b.jsonRpcClient.Eth().GetBalance(b.wallet.Address(), ethgo.Latest); err == nil {
			b.balance.BNB = balance
		}

		time.Sleep(500 * time.Millisecond)
	}
}

func (b *Bot) logBalance() {
	log.Printf("Balance, BNB: %f - MSC: %f - USDT: %f", utils.WeiToEther(b.balance.BNB), utils.WeiToEther(b.balance.MSC), utils.WeiToEther(b.balance.USDT))
}

func (b *Bot) syncBlock() {
	logger := log.New(os.Stderr, "", log.LstdFlags)
	tracker := blocktracker.NewJSONBlockTracker(logger, b.jsonRpcClient.Eth())
	tracker.PollInterval = 1 * time.Second
	blocks := make(chan *ethgo.Block)
	err := tracker.Track(b.ctx, func(block *ethgo.Block) error {
		blocks <- block
		return nil
	})
	if err != nil {
		log.Println(err)
	}

	for {
		select {
		case block := <-blocks:
			if !b.running {
				// if running is false, exit current thread
				runtime.Goexit()
			}
			b.lastBlock = block
			b.calMscPrice()
		case <-time.After(5 * time.Second):
			// log.Fatal("timeout")
			// log.Println("timeout")
		}
	}
}

func (b *Bot) calMscPrice() {
	reserves, err := b.contracts.pair.Call("getReserves", ethgo.Latest)
	if reserves == nil || err != nil {
		log.Println(err)
		return
	}
	reserveOut := reserves["reserve0"].(*big.Int)
	reserveIn := reserves["reserve1"].(*big.Int)
	x := utils.GetAmountOut(big.NewInt(1000000000000000000), reserveIn, reserveOut)
	mscPrice, _ := utils.WeiToEther(x).Float64()
	b.lastMscPrice = mscPrice
	s := utils.HandleStrategy(mscPrice)
	if s == nil {
		return
	}

	strtegy := s["s"].(config.StrtegyItem)

	if b.balance.MSC == nil || b.balance.BNB == nil || b.balance.USDT == nil {
		return
	}

	b.logBalance()

	st := viper.GetFloat64("strtegy.st")

	// 检查配置项
	if strtegy.Percent <= 0 || strtegy.Percent >= 1 {
		panic(fmt.Errorf("buy to sell percent must > 0 and < 1"))
	}

	if s["m"] == "sell" {
		strtegyAmout := utils.CalStrtegyAmout(b.balance.MSC, mscPrice, "sell", strtegy.Percent, st)
		log.Printf("Current MSC price: %f, The【Sell】policy is matched. Percent: %f, 【MSC】quantity sold: %f, Minimum expected value: %f", mscPrice, strtegy.Percent, strtegyAmout.AmountIn, strtegyAmout.AmountOutMin)
		b.swap(strtegyAmout.AmountIn, strtegyAmout.AmountOutMin, []ethgo.Address{MSC, USDT}, strtegy, mscPrice)
	} else if s["m"] == "buy" {
		strtegyAmout := utils.CalStrtegyAmout(b.balance.USDT, mscPrice, "buy", strtegy.Percent, st)
		log.Printf("Current MSC price: %f, The【Buy】policy is matched. Percent: %f, Cost【USDT】quantity: %f, Minimum expected value: %f", mscPrice, strtegy.Percent, strtegyAmout.AmountIn, strtegyAmout.AmountOutMin)
		b.swap(strtegyAmout.AmountIn, strtegyAmout.AmountOutMin, []ethgo.Address{USDT, MSC}, strtegy, mscPrice)
	}
}

func (b *Bot) swap(amountIn *big.Float, amountOutMin *big.Float, address []ethgo.Address, strtegyConfig config.StrtegyItem, mscPrice float64) {
	if trading {
		return
	}
	trading = true

	flagTradingFlase := func() {
		trading = false
	}
	defer flagTradingFlase()

	// can tx
	if i, _ := dao.StrtegyDao.CanTx(strtegyConfig); i == false {
		return
	}

	txn, err := b.contracts.router.Txn("swapExactTokensForTokens", utils.EtherToWei(amountIn), utils.EtherToWei(amountOutMin), address, b.WalletAddress(), time.Now().Unix()+120)
	if err != nil {
		log.Println(err)
		if funk.Contains(err.Error(), "INSUFFICIENT_OUTPUT_AMOUNT") {
			//log.Println("...")
		}
		return
	}

	doc := c.NewDocument()
	doc.Set("TxHash", txn.Hash().String())
	coinName := func(address ethgo.Address) string {
		return funk.ShortIf(address.String() == MSC.Address().String(), "MSC", "USDT").(string)
	}
	doc.Set("Address", fmt.Sprintf("%s-%s", coinName(address[0]), coinName(address[1])))
	doc.Set("MscPrice", mscPrice)
	doc.Set("AmountIn", amountIn)
	doc.Set("AmountOutMin", amountOutMin)
	doc.Set("Status", -1)
	doc.Set("CreateTime", time.Now().Unix())
	dao.RecordDao.InsertOne(doc)

	flagTxFailed := func(txHash string, status int) {
		updates := make(map[string]interface{})
		updates["Status"] = status
		global.DB.Query("record").Where((*c.Criteria)(c.Field("TxHash").Eq(txHash))).Update(updates)
	}

	e := txn.Do()
	if e != nil {
		log.Println(e)
		flagTxFailed(txn.Hash().String(), 0)
		return
	}
	// log.Println(txn.Hash())
	receipt, err := txn.Wait()
	if err != nil {
		log.Println(err)
		flagTxFailed(txn.Hash().String(), 0)
		return
	}

	if receipt.Status == 1 {
		flagTxFailed(txn.Hash().String(), 1)
		dao.StrtegyDao.InsertOrUpdate(strtegyConfig)
	} else {
		log.Println("tx failed")
		flagTxFailed(txn.Hash().String(), 0)
	}
	trading = false
}

// Init Contract
func (b *Bot) initContract() {
	// factoryContract := contract.NewContract(FACTORY, abis.Factory, jsonRpcClient)
	// event, ok := factoryContract.Event("PairCreated")
	// if !ok {
	// 	log.Println("not ok")
	// }
	usdtAbi, _ := abi.NewABIFromList([]string{
		"function balanceOf(address account) external view returns (uint256)",
	})
	b.contracts.usdt = contract.NewContract(USDT, usdtAbi, contract.WithJsonRPC(b.jsonRpcClient.Eth()), contract.WithSender(b.wallet))

	mscAbi, _ := abi.NewABIFromList([]string{
		"function balanceOf(address account) external view returns (uint256)",
	})
	b.contracts.msc = contract.NewContract(MSC, mscAbi, contract.WithJsonRPC(b.jsonRpcClient.Eth()), contract.WithSender(b.wallet))

	pairAbi, _ := abi.NewABIFromList([]string{
		"function getReserves() external view returns (uint112 reserve0, uint112 reserve1, uint32 blockTimestampLast)",
	})
	b.contracts.pair = contract.NewContract(PANCAKE_PAIR, pairAbi, contract.WithJsonRPC(b.jsonRpcClient.Eth()), contract.WithSender(b.wallet))

	routerAbi := abi.MustNewABI(`[{"inputs":[{"internalType":"uint256","name":"amountIn","type":"uint256"},{"internalType":"uint256","name":"amountOutMin","type":"uint256"},{"internalType":"address[]","name":"path","type":"address[]"},{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"deadline","type":"uint256"}],"name":"swapExactTokensForTokens","outputs":[{"internalType":"uint256[]","name":"amounts","type":"uint256[]"}],"stateMutability":"nonpayable","type":"function"}]`)
	b.contracts.router = contract.NewContract(ROUTER, routerAbi, contract.WithJsonRPC(b.jsonRpcClient.Eth()), contract.WithSender(b.wallet))
}

func (b *Bot) Stop() {
	b.running = false

	err := b.jsonRpcClient.Close()
	if err != nil {
		return
	}
}

func (b *Bot) Running() bool {
	return b.running
}

func (b *Bot) WalletAddress() string {
	return b.wallet.Address().String()
}

func (b *Bot) Balance() *Balance {
	return b.balance
}

func (b *Bot) MscLastPrice() float64 {
	return b.lastMscPrice
}
