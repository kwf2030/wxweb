package wxweb

import (
  "io/ioutil"
  "net/http"
  "net/url"

  "github.com/buger/jsonparser"
  "github.com/kwf2030/commons/pipeline"
  "github.com/kwf2030/commons/time2"
)

const contactsUrlPath = "/webwxgetcontact"

type contactsReq struct {
  *Bot
}

func (r *contactsReq) Handle(ctx *pipeline.HandlerContext, val interface{}) {
  arr, e := r.do()
  if e != nil {
    r.handler.OnSignIn(e)
    return
  }
  r.contacts = initContacts(arr, r.Bot)
  r.StartTime = time2.Now()
  r.session.State = StateRunning
  botsMutex.Lock()
  bots[r.session.Uin] = r.Bot
  botsMutex.Unlock()
  r.handler.OnSignIn(nil)
  ctx.Fire(val)
}

func (r *contactsReq) do() ([]*Contact, error) {
  addr, _ := url.Parse(r.session.BaseUrl + contactsUrlPath)
  q := addr.Query()
  q.Set("pass_ticket", r.session.PassTicket)
  q.Set("r", timestampString13())
  q.Set("seq", "0")
  q.Set("skey", r.session.SKey)
  addr.RawQuery = q.Encode()
  req, _ := http.NewRequest("GET", addr.String(), nil)
  req.Header.Set("Referer", r.session.Referer)
  req.Header.Set("User-Agent", userAgent)
  resp, e := r.client.Do(req)
  if e != nil {
    return nil, e
  }
  defer resp.Body.Close()
  if resp.StatusCode != http.StatusOK {
    return nil, ErrReq
  }
  return parseContactsResp(resp)
}

func parseContactsResp(resp *http.Response) ([]*Contact, error) {
  body, e := ioutil.ReadAll(resp.Body)
  if e != nil {
    return nil, e
  }
  dump("6_"+time2.NowStrf(time2.DateTimeMsFormat5), body)
  cnt, _ := jsonparser.GetInt(body, "MemberCount")
  if cnt == 0 {
    cnt = 5000
  }
  arr := make([]*Contact, 0, cnt)
  _, e = jsonparser.ArrayEach(body, func(v []byte, _ jsonparser.ValueType, _ int, e error) {
    if e != nil {
      return
    }
    c := buildContact(v, nil)
    if c != nil && c.UserName != "" {
      arr = append(arr, c)
    }
  }, "MemberList")
  if e == nil || e == jsonparser.KeyPathNotFoundError {
    return arr, nil
  }
  return nil, e
}
