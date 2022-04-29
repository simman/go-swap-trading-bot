package dao

import (
	"log"
	"time"

	c "github.com/ostafen/clover"

	"github.com/simman/go-swap-trading-bot/global"
)

var RecordDao = &Record{tableName: "record"}

type Record struct {
	tableName string
}

func (r *Record) InsertOne(doc *c.Document) (string, error) {
	return global.DB.InsertOne(r.tableName, doc)
}

func (r *Record) QueryLastDayListMap() ([]map[string]interface{}, error) {
	docs, err := global.DB.Query(r.tableName).MatchPredicate(func(doc *c.Document) bool {
		return int64(doc.Get("CreateTime").(float64)) >= time.Now().Unix()-86400
	}).FindAll()

	if err != nil {
		log.Println(err)
		return nil, err
	}

	result := []map[string]interface{}{}

	for _, v := range docs {
		recordMap := map[string]interface{}{
			"TxHash":       v.Get("TxHash"),
			"Address":      v.Get("Address"),
			"MscPrice":     v.Get("MscPrice"),
			"AmountIn":     v.Get("AmountIn"),
			"AmountOutMin": v.Get("AmountOutMin"),
			"Status":       v.Get("Status"),
			"CreateTime":   v.Get("CreateTime"),
		}
		result = append(result, recordMap)
	}

	return result, err
}
