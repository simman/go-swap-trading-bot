package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/params"
	"github.com/simman/go-swap-trading-bot/config"
)

func WeiToEther(wei *big.Int) *big.Float {
	f := new(big.Float)
	f.SetPrec(236) //  IEEE 754 octuple-precision binary floating-point format: binary256
	f.SetMode(big.ToNearestEven)
	fWei := new(big.Float)
	fWei.SetPrec(236) //  IEEE 754 octuple-precision binary floating-point format: binary256
	fWei.SetMode(big.ToNearestEven)
	return f.Quo(fWei.SetInt(wei), big.NewFloat(params.Ether))
}

func EtherToWei(eth *big.Float) *big.Int {
	truncInt, _ := eth.Int(nil)
	truncInt = new(big.Int).Mul(truncInt, big.NewInt(params.Ether))
	fracStr := strings.Split(fmt.Sprintf("%.18f", eth), ".")[1]
	fracStr += strings.Repeat("0", 18-len(fracStr))
	fracInt, _ := new(big.Int).SetString(fracStr, 10)
	wei := new(big.Int).Add(truncInt, fracInt)
	return wei
}

func GetAmountOut(amountIn *big.Int, reserveIn *big.Int, reserveOut *big.Int) *big.Int {
	amountInWithFee := amountIn.Mul(amountIn, big.NewInt(9975))
	numerator := reserveOut.Mul(amountInWithFee, reserveOut)
	denominator := reserveIn.Mul(reserveIn, big.NewInt(10000))
	denominator = denominator.Add(denominator, amountInWithFee)
	return numerator.Div(numerator, denominator)
}

func HandleStrategy(price float64) map[string]interface{} {

	for _, v := range config.StrtegyConfig.Buy {
		if v.Price >= price {
			return map[string]interface{}{
				"m": "buy",
				// "r": v.Percent,
				"s": v,
			}
		}
	}

	for _, v := range config.StrtegyConfig.Sell {
		if v.Price <= price {
			return map[string]interface{}{
				"m": "buy",
				// "r": v.Percent,
				"s": v,
			}
		}
	}
	return nil
}

type StrtegyAmout struct {
	AmountIn     *big.Float
	AmountOutMin *big.Float
}

func CalStrtegyAmout(amount *big.Int, mscPrice float64, t string, r float64, st float64) *StrtegyAmout {
	a := WeiToEther(amount)
	b := new(big.Float).Mul(a, big.NewFloat(r))

	n := new(big.Float)
	// if sell
	//  - amount = msc count
	//  - outMin： (msc count*msc price)*st
	// buy
	//  - amount = u count
	//  - outMin：(u count/msc price)*st
	if t == "sell" {
		n = new(big.Float).Mul(b, big.NewFloat(mscPrice))
	} else {
		n = new(big.Float).Quo(b, big.NewFloat(mscPrice))
	}
	return &StrtegyAmout{
		AmountIn:     b,
		AmountOutMin: n.Mul(n, big.NewFloat(1-(st/100))), // big.NewFloat(0).Quo(b, big.NewFloat(1-st))
	}
}

func JsonPrettyPrint(in string) string {
	var out bytes.Buffer
	err := json.Indent(&out, []byte(in), "", "\t")
	if err != nil {
		return in
	}
	return out.String()
}
