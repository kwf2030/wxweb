package wxweb

import (
  "errors"
  "fmt"
  "net/http"
  "net/http/cookiejar"
  "os"
  "path"
  "strconv"
  "strings"
  "sync"
  "time"

  "github.com/buger/jsonparser"
  "github.com/kwf2030/commons/pipeline"
  "github.com/kwf2030/commons/time2"
  "golang.org/x/net/publicsuffix"
)

const (
  stateUnknown = iota

  // 已扫码未确认
  StateScan

  // 等待确认超时
  StateScanTimeout

  // 已确认（正在登录）
  StateConfirm

  // 登录成功（此时可以正常收发消息）
  StateRunning

  // 已下线（主动、被动或异常）
  StateStop
)

const (
  // 图片消息存放目录
  attrImageDir = "wxweb.image_dir"

  // 语音消息存放目录
  attrVoiceDir = "wxweb.voice_dir"

  // 视频消息存放目录
  attrVideoDir = "wxweb.video_dir"

  // 文件消息存放目录
  attrFileDir = "wxweb.file_dir"

  // 头像存放路径
  attrAvatarPath = "wxweb.avatar_path"

  // 正在登录时用时间戳作为key，保证bots中有记录且可查询这个Bot
  attrRandUin = "wxweb.rand_uin"

  rootDir = "wxweb"
  dumpDir = rootDir + "/dump/"

  contentType = "application/json; charset=UTF-8"
  userAgent   = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/71.0.3578.98 Safari/537.36"
)

var (
  ErrInvalidArgs = errors.New("invalid args")

  ErrInvalidState = errors.New("invalid state")

  ErrReq = errors.New("request failed")

  ErrResp = errors.New("response invalid")

  ErrScanTimeout = errors.New("scan timeout")

  ErrContactNotFound = errors.New("contact not found")
)

var (
  dumpEnabled = false

  botsMutex = &sync.RWMutex{}
  bots      = make(map[int64]*Bot, 4)
)

type Handler interface {
  // 登录成功（error == nil），
  // 登录失败（error != nil）
  OnSignIn(error)

  // 退出/下线
  OnSignOut()

  // 收到二维码（需扫码登录），
  // 参数为二维码链接
  OnQRCode(string)

  // 联系人更新，如：
  // 好友资料更新、删除好友或被好友删除等，
  // 建群、加入群、被拉入群、群改名、群成员变更、退群或被群主移出群等，
  // 第二个参数暂时没用
  OnContact(*Contact, int)

  // 收到消息，
  // 第二个参数暂时没用
  OnMessage(*Message, int)
}

func init() {
  e := os.MkdirAll(rootDir, os.ModePerm)
  if e != nil {
    return
  }
  updatePaths()
}

func updatePaths() {
  time.AfterFunc(time2.UntilTomorrow(), func() {
    for _, b := range RunningBots() {
      b.updatePaths()
    }
    updatePaths()
  })
}

func EachBot(f func(*Bot) bool) {
  arr := make([]*Bot, 0, 2)
  botsMutex.RLock()
  for _, v := range bots {
    arr = append(arr, v)
  }
  botsMutex.RUnlock()
  for _, v := range arr {
    if !f(v) {
      break
    }
  }
}

func CountBots() int {
  l := 0
  botsMutex.RLock()
  l = len(bots)
  botsMutex.RUnlock()
  return l
}

func EnableDump(enabled bool) {
  if dumpEnabled = enabled; enabled {
    os.MkdirAll(dumpDir, os.ModePerm)
  }
}

type Bot struct {
  handler Handler

  client  *http.Client
  session *session
  req     *wxReq

  signInPipeline *pipeline.Pipeline

  self     *Contact
  contacts *Contacts

  attr *sync.Map

  StartTime time.Time
  StopTime  time.Time
}

func New() *Bot {
  jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
  s := &session{}
  s.init()
  bot := &Bot{
    client: &http.Client{
      Jar:     jar,
      Timeout: time.Minute * 2,
    },
    session:        s,
    signInPipeline: pipeline.New(),
    attr:           &sync.Map{},
  }
  bot.req = &wxReq{bot}
  k := time2.Timestamp()
  bot.attr.Store(attrRandUin, k)
  botsMutex.Lock()
  bots[k] = bot
  botsMutex.Unlock()
  return bot
}

func GetBotByUUID(uuid string) *Bot {
  if uuid == "" {
    return nil
  }
  var ret *Bot
  EachBot(func(b *Bot) bool {
    if b.session.UUID == uuid {
      ret = b
      return false
    }
    return true
  })
  return ret
}

func GetBotByUin(uin int64) *Bot {
  var ret *Bot
  EachBot(func(b *Bot) bool {
    if b.session.Uin == uin {
      ret = b
      return false
    }
    return true
  })
  return ret
}

func RunningBots() []*Bot {
  ret := make([]*Bot, 0, 4)
  botsMutex.RLock()
  for _, v := range bots {
    if v.session.State == StateRunning {
      ret = append(ret, v)
    }
  }
  botsMutex.RUnlock()
  if len(ret) == 0 {
    return nil
  }
  return ret
}

func (bot *Bot) Self() *Contact {
  return bot.self
}

func (bot *Bot) Contacts() *Contacts {
  return bot.contacts
}

func (bot *Bot) Start(handler Handler) {
  if handler == nil {
    return
  }
  bot.handler = handler
  bot.signInPipeline.AddLast("qr", &qrReq{bot}).
    AddLast("scan", &scanReq{bot}).
    AddLast("redirect", &redirectReq{bot}).
    AddLast("init", &initReq{bot}).
    AddLast("notify", &notifyReq{bot}).
    AddLast("contacts", &contactsReq{bot}).
    AddLast("sync", &syncReq{bot})
  bot.signInPipeline.Fire(nil)
  if k, ok := bot.attr.Load(attrRandUin); ok {
    botsMutex.Lock()
    delete(bots, k.(int64))
    botsMutex.Unlock()
  }
}

func (bot *Bot) Stop() {
  bot.StopTime = time2.Now()
  bot.session.State = StateStop
  bot.req.SignOut()
}

func (bot *Bot) Release() {
  bot.handler = nil
  bot.client = nil
  bot.session = nil
  bot.req = nil
  bot.signInPipeline = nil
  bot.self = nil
  bot.contacts = nil
  bot.attr = nil
}

func (bot *Bot) updatePaths() {
  if bot.session.Uin == 0 {
    return
  }
  uin := strconv.FormatInt(bot.session.Uin, 10)
  dir := path.Join(rootDir, uin, time2.NowStrf(time2.DateFormat))
  e := os.MkdirAll(dir, os.ModePerm)
  if e != nil {
    return
  }

  image := path.Join(dir, "image")
  e = os.MkdirAll(image, os.ModePerm)
  if e == nil {
    bot.attr.Store(attrImageDir, image)
  }

  voice := path.Join(dir, "voice")
  e = os.MkdirAll(voice, os.ModePerm)
  if e == nil {
    bot.attr.Store(attrVoiceDir, voice)
  }

  video := path.Join(dir, "video")
  e = os.MkdirAll(video, os.ModePerm)
  if e == nil {
    bot.attr.Store(attrVideoDir, video)
  }

  file := path.Join(dir, "file")
  e = os.MkdirAll(file, os.ModePerm)
  if e == nil {
    bot.attr.Store(attrFileDir, file)
  }

  bot.attr.Store(attrAvatarPath, path.Join(rootDir, uin, "avatar.jpg"))
}

type session struct {
  Host          string
  SyncCheckHost string
  Referer       string
  BaseUrl       string

  State int

  UUID      string
  QRCodeUrl string

  RedirectUrl string

  SKey       string
  Sid        string
  Uin        int64
  PassTicket string
  BaseReq    baseReq

  SyncKey   syncKey
  UserName  string
  AvatarUrl string

  WuFile int
}

func (s *session) init() {
  s.Host = "wx.qq.com"
  s.SyncCheckHost = "webpush.weixin.qq.com"
  s.Referer = "https://wx.qq.com/"
  s.BaseUrl = "https://wx.qq.com/cgi-bin/mmwebwx-bin"
}

type baseReq struct {
  DeviceId string `json:"DeviceID"`
  Sid      string `json:"Sid"`
  SKey     string `json:"Skey"`
  Uin      int64  `json:"Uin"`
}

type syncKeyItem struct {
  Key int
  Val int
}

type syncKey struct {
  Count int
  List  []syncKeyItem
}

func parseSyncKey(data []byte) syncKey {
  cnt, _ := jsonparser.GetInt(data, "Count")
  if cnt <= 0 {
    return syncKey{}
  }
  arr := make([]syncKeyItem, 0, cnt)
  jsonparser.ArrayEach(data, func(v []byte, _ jsonparser.ValueType, i int, e error) {
    if e != nil {
      return
    }
    key, _ := jsonparser.GetInt(v, "Key")
    val, _ := jsonparser.GetInt(v, "Val")
    arr = append(arr, syncKeyItem{int(key), int(val)})
  }, "List")
  if len(arr) == 0 {
    return syncKey{}
  }
  return syncKey{Count: int(cnt), List: arr}
}

func (sk *syncKey) expand() string {
  var sb strings.Builder
  n := sk.Count - 1
  for i := 0; i <= n; i++ {
    item := sk.List[i]
    fmt.Fprintf(&sb, "%d_%d", item.Key, item.Val)
    if i != n {
      sb.WriteString("|")
    }
  }
  return sb.String()
}
