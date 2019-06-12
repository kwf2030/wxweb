package wxweb

import (
  "bytes"
  "encoding/json"
  "io/ioutil"
  "net/http"
  "net/url"

  "github.com/kwf2030/commons/pipeline"
  "github.com/kwf2030/commons/time2"
)

const notifyUrlPath = "/webwxstatusnotify"

type notifyReq struct {
  *Bot
}

func (r *notifyReq) Handle(ctx *pipeline.HandlerContext, val interface{}) {
  e := r.do()
  if e != nil {
    r.handler.OnSignIn(e)
    return
  }
  ctx.Fire(val)
}

func (r *notifyReq) do() error {
  addr, _ := url.Parse(r.session.BaseUrl + notifyUrlPath)
  q := addr.Query()
  q.Set("pass_ticket", r.session.PassTicket)
  addr.RawQuery = q.Encode()
  m := make(map[string]interface{}, 5)
  m["BaseRequest"] = r.session.BaseReq
  m["ClientMsgId"] = timestampString13()
  m["Code"] = 3
  m["FromUserName"] = r.session.UserName
  m["ToUserName"] = r.session.UserName
  buf, _ := json.Marshal(m)
  req, _ := http.NewRequest("POST", addr.String(), bytes.NewReader(buf))
  req.Header.Set("Content-Type", contentType)
  req.Header.Set("Referer", r.session.Referer)
  req.Header.Set("User-Agent", userAgent)
  resp, e := r.client.Do(req)
  if e != nil {
    return e
  }
  defer resp.Body.Close()
  if resp.StatusCode != http.StatusOK {
    return ErrReq
  }
  body, e := ioutil.ReadAll(resp.Body)
  if e != nil {
    return e
  }
  dump("5_"+time2.NowStrf(time2.DateTimeMsFormat5), body)
  return nil
}
