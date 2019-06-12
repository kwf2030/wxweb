package wxweb

import (
  "encoding/xml"
  "io/ioutil"
  "net/http"
  "net/url"
  "strings"

  "github.com/kwf2030/commons/pipeline"
  "github.com/kwf2030/commons/time2"
)

type redirectReq struct {
  *Bot
}

func (r *redirectReq) Handle(ctx *pipeline.HandlerContext, val interface{}) {
  redirect, e := r.do()
  if e != nil {
    r.handler.OnSignIn(e)
    return
  }
  if redirect == nil || redirect.PassTicket == "" || redirect.SKey == "" || redirect.WXSid == "" || redirect.WXUin == 0 {
    r.handler.OnSignIn(ErrResp)
    return
  }
  r.session.PassTicket = redirect.PassTicket
  r.session.Sid = redirect.WXSid
  r.session.SKey = redirect.SKey
  r.session.Uin = redirect.WXUin
  r.session.BaseReq = baseReq{
    DeviceId: deviceId(),
    Sid:      redirect.WXSid,
    SKey:     redirect.SKey,
    Uin:      redirect.WXUin,
  }
  r.selectBaseUrl()
  r.updatePaths()
  ctx.Fire(val)
}

func (r *redirectReq) do() (*redirectResp, error) {
  u, _ := url.Parse(r.session.RedirectUrl)
  // 返回的地址可能没有fun和version两个参数，而此请求必须这两个参数
  q := u.Query()
  q.Set("fun", "new")
  q.Set("version", "v2")
  u.RawQuery = q.Encode()
  req, _ := http.NewRequest("GET", u.String(), nil)
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
  return parseRedirectResp(resp)
}

func (r *redirectReq) selectBaseUrl() {
  u, _ := url.Parse(r.session.RedirectUrl)
  host := u.Hostname()
  r.session.Host = host
  switch {
  case strings.Contains(host, "wx2"):
    r.session.Host = "wx2.qq.com"
    r.session.SyncCheckHost = "webpush.wx2.qq.com"
    r.session.Referer = "https://wx2.qq.com/"
    r.session.BaseUrl = "https://wx2.qq.com/cgi-bin/mmwebwx-bin"
  }
}

func parseRedirectResp(resp *http.Response) (*redirectResp, error) {
  body, e := ioutil.ReadAll(resp.Body)
  if e != nil {
    return nil, e
  }
  dump("3_"+time2.NowStrf(time2.DateTimeMsFormat5), body)
  ret := &redirectResp{}
  e = xml.Unmarshal(body, ret)
  if e != nil {
    return nil, e
  }
  return ret, nil
}

type redirectResp struct {
  XMLName     xml.Name `xml:"error"`
  Ret         int      `xml:"ret"`
  Message     string   `xml:"message"`
  IsGrayScale int      `xml:"isgrayscale"`
  PassTicket  string   `xml:"pass_ticket"`
  SKey        string   `xml:"skey"`
  WXSid       string   `xml:"wxsid"`
  WXUin       int64    `xml:"wxuin"`
}
