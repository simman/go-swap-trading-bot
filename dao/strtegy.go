package dao

import (
	c "github.com/ostafen/clover"
	"github.com/simman/go-swap-trading-bot/config"
	"github.com/simman/go-swap-trading-bot/global"
	"log"
	"time"
)

var StrtegyDao = &Strtegy{tableName: "strtegy"}

type Strtegy struct {
	tableName string
}

func (s *Strtegy) InsertOne(doc *c.Document) (string, error) {
	return global.DB.InsertOne(s.tableName, doc)
}

func (s *Strtegy) InsertOrUpdate(strtegyConf config.StrtegyItem) {
	if _, id := s.CanTx(strtegyConf); id != "" {
		updates := make(map[string]interface{})
		updates["CreateTime"] = time.Now().Unix()
		global.DB.Query(s.tableName).Where(c.Field("_id").Eq(id)).Update(updates)
	} else {
		strtegyDoc := c.NewDocument()
		strtegyDoc.Set("Price", strtegyConf.Price)
		strtegyDoc.Set("Percent", strtegyConf.Percent)
		strtegyDoc.Set("CreateTime", time.Now().Unix())
		s.InsertOne(strtegyDoc)
	}
}

func (s *Strtegy) CanTx(strtegyConf config.StrtegyItem) (bool, string) {
	docs, err := global.DB.Query(s.tableName).Where(c.Field("Price").Eq(strtegyConf.Price).And(c.Field("Percent").Eq(strtegyConf.Percent))).FindAll()

	if err != nil {
		log.Println(err)
		return false, ""
	}

	if len(docs) > 0 {
		st := docs[0]
		if time.Now().Unix()-int64(st.Get("CreateTime").(float64)) < int64(strtegyConf.Gap) {
			return false, st.ObjectId()
		} else {
			return true, docs[0].ObjectId()
		}
	}
	return true, ""
}
