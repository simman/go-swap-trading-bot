package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"github.com/thoas/go-funk"
)

type StrtegyConf struct {
	Buy  []StrtegyItem
	Sell []StrtegyItem
	ST   float64
}

type StrtegyItem struct {
	Price   float64
	Percent float64
	Gap     int
}

var (
	StrtegyConfig *StrtegyConf
)

func InitConfig() {

	viper.AddConfigPath(getCurrentDirectory())
	viper.SetConfigName("env")
	viper.SetConfigType("yaml")
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("parse env.yaml err: %s", err))
	}
	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		log.Println("env.yaml is changed, reload application...")
		// log.Println(viper.AllSettings())
		initStrtegy()
	})

	checkConfigValid()

	initStrtegy()
}

func getCurrentDirectory() string {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}
	return strings.Replace(dir, "\\", "/", -1)
}

func checkConfigValid() {
	if funk.IsEmpty(viper.GetInt("server.port")) {
		log.Fatal("please check server.port")
	}
	if funk.IsEmpty(viper.GetString("node.rpc")) {
		log.Fatal("please check node.rpc")
	}
	if funk.IsEmpty(viper.GetString("wallet.privateKey")) {
		log.Fatal("please check wallet.privateKey")
	}
}

func initStrtegy() {
	strtegyConf := viper.GetStringMap("strtegy")

	numberToFloat64 := func(num interface{}) float64 {
		if reflect.TypeOf(num).Name() == "int" {
			return float64(num.(int))
		}
		return num.(float64)
	}

	parseStrtegyItems := func(items []interface{}, order string) []StrtegyItem {
		strtegyItems := []StrtegyItem{}
		for _, v := range items {
			m := v.(map[interface{}]interface{})
			//log.Println(reflect.TypeOf(m["price"]))
			//log.Println(reflect.TypeOf(m["percent"]))
			//log.Println(reflect.TypeOf(m["gap"]))
			i := StrtegyItem{
				Price:   numberToFloat64(m["price"]),
				Percent: numberToFloat64(m["percent"]),
				Gap:     m["gap"].(int),
			}
			strtegyItems = append(strtegyItems, i)
		}
		sort.SliceStable(strtegyItems, func(i, j int) bool {
			if order == "asc" {
				return strtegyItems[i].Price < strtegyItems[j].Price
			}
			return strtegyItems[i].Price > strtegyItems[j].Price
		})
		return strtegyItems
	}

	buyStrtegyItems := parseStrtegyItems(strtegyConf["buy"].([]interface{}), "asc")
	sellStrtegyItems := parseStrtegyItems(strtegyConf["sell"].([]interface{}), "desc")

	strtegy := &StrtegyConf{
		Buy:  buyStrtegyItems,
		Sell: sellStrtegyItems,
		ST:   strtegyConf["st"].(float64),
	}

	StrtegyConfig = strtegy
}
