package wxweb

import (
  "fmt"
  "io/ioutil"
  "net/http"
  "net/url"
  "regexp"

  "github.com/kwf2030/commons/pipeline"
  "github.com/kwf2030/commons/time2"
)

const (
  uuidUrl = "https://login.weixin.qq.com/jslogin"
  qrUrl   = "https://login.weixin.qq.com/qrcode"
)

var uuidRegex = regexp.MustCompile(`uuid\s*=\s*"(.*)"`)

type qrReq struct {
  *Bot
}

func (r *qrReq) Handle(ctx *pipeline.HandlerContext, val interface{}) {
  uuid, e := r.do()
  if e != nil {
    r.handler.OnSignIn(e)
    return
  }
  if uuid == "" {
    r.handler.OnSignIn(ErrResp)
    return
  }
  r.session.UUID = uuid
  r.session.QRCodeUrl = fmt.Sprintf("%s/%s", qrUrl, uuid)
  r.handler.OnQRCode(r.session.QRCodeUrl)
  ctx.Fire(val)
}

func (r *qrReq) do() (string, error) {
  addr, _ := url.Parse(uuidUrl)
  q := addr.Query()
  q.Set("appid", "wx782c26e4c19acffb")
  q.Set("fun", "new")
  q.Set("lang", "zh_CN")
  q.Set("_", timestampString13())
  q.Set("redirect_uri", "https://wx.qq.com/cgi-bin/mmwebwx-bin/webwxnewloginpage")
  addr.RawQuery = q.Encode()
  req, _ := http.NewRequest("GET", addr.String(), nil)
  req.Header.Set("Referer", r.session.Referer)
  req.Header.Set("User-Agent", userAgent)
  resp, e := r.client.Do(req)
  if e != nil {
    return "", e
  }
  defer resp.Body.Close()
  if resp.StatusCode != http.StatusOK {
    return "", ErrReq
  }
  return parseQRResp(resp)
}

func parseQRResp(resp *http.Response) (string, error) {
  // window.QRLogin.code = 200; window.QRLogin.uuid = "wbVC3cUBrQ==";
  body, e := ioutil.ReadAll(resp.Body)
  if e != nil {
    return "", e
  }
  dump("1_"+time2.NowStrf(time2.DateTimeMsFormat5), body)
  data := string(body)
  match := uuidRegex.FindStringSubmatch(data)
  if len(match) != 2 {
    return "", ErrResp
  }
  return match[1], nil
}
