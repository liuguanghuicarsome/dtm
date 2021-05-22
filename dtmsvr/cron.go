package dtmsvr

import (
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/yedf/dtm"
	"github.com/yedf/dtm/common"
)

func CronPreparedOnce(expire time.Duration) {
	db := DbGet()
	ss := []SagaModel{}
	db.Must().Model(&SagaModel{}).Where("update_time < date_sub(now(), interval ? second)", int(expire/time.Second)).Where("status = ?", "prepared").Find(&ss)
	writeTransLog("", "saga fetch prepared", fmt.Sprint(len(ss)), -1, "")
	if len(ss) == 0 {
		return
	}
	for _, sm := range ss {
		writeTransLog(sm.Gid, "saga touch prepared", "", -1, "")
		db.Must().Model(&sm).Update("id", sm.ID)
		resp, err := dtm.RestyClient.R().SetQueryParam("gid", sm.Gid).Get(sm.TransQuery)
		common.PanicIfError(err)
		body := resp.String()
		if strings.Contains(body, "FAIL") {
			preparedExpire := time.Now().Add(time.Duration(-Config.PreparedExpire) * time.Second)
			logrus.Printf("create time: %s prepared expire: %s ", sm.CreateTime.Local(), preparedExpire.Local())
			status := common.If(sm.CreateTime.Before(preparedExpire), "canceled", "prepared").(string)
			writeTransLog(sm.Gid, "saga canceled", status, -1, "")
			db.Must().Model(&sm).Where("status = ?", "prepared").Update("status", status)
		} else if strings.Contains(body, "SUCCESS") {
			saveCommitedSagaModel(&sm)
			ProcessCommitedSaga(sm.Gid)
		}
	}
}

func CronPrepared() {
	for {
		defer handlePanic()
		CronPreparedOnce(10 * time.Second)
	}
}

func CronCommitedOnce(expire time.Duration) {
	db := DbGet()
	ss := []SagaModel{}
	db.Must().Model(&SagaModel{}).Where("update_time < date_sub(now(), interval ? second)", int(expire/time.Second)).Where("status = ?", "commited").Find(&ss)
	writeTransLog("", "saga fetch commited", fmt.Sprint(len(ss)), -1, "")
	if len(ss) == 0 {
		return
	}
	for _, sm := range ss {
		writeTransLog(sm.Gid, "saga touch commited", "", -1, "")
		db.Must().Model(&sm).Update("id", sm.ID)
		ProcessCommitedSaga(sm.Gid)
	}
}

func CronCommited() {
	for {
		defer handlePanic()
		CronCommitedOnce(10 * time.Second)
	}
}

func handlePanic() {
	if err := recover(); err != nil {
		logrus.Printf("----panic %s handlered", err.(error).Error())
	}
}
