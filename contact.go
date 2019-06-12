package wxweb

import (
  "sync"

  "github.com/buger/jsonparser"
  "github.com/kwf2030/commons/conv"
)

const (
  contactUnknown = iota

  // 好友
  ContactFriend

  // 群
  ContactGroup

  // 公众号
  ContactMPS

  // 系统
  ContactSystem
)

var (
  jsonPathUserName    = []string{"UserName"}
  jsonPathNickName    = []string{"NickName"}
  jsonPathRemarkName  = []string{"RemarkName"}
  jsonPathVerifyFlag  = []string{"VerifyFlag"}
  jsonPathMemberCount = []string{"MemberCount"}

  jsonKeyMemberList = "MemberList"
  jsonKeyUserName   = "UserName"
  jsonKeyNickName   = "NickName"
)

type Contact struct {
  bot  *Bot
  attr *sync.Map

  // 联系人类型，
  // 个人和群为0，
  // 订阅号为8，
  // 企业号为24（包括扩微信支付），
  // 系统号为56(微信团队官方帐号），
  // 29（未知，招行信用卡为29）
  VerifyFlag int

  // Type是VerifyFlag解析后的值
  Type int

  // UserName每次登录都不一样，
  // 群以@@开头，其他以@开头，系统帐号则直接是名字，如：
  // weixin（微信团队）/filehelper（文件传输助手）/fmessage（朋友消息推荐）
  UserName string

  // 昵称，如果是群，表示群名称
  NickName string

  // 备注（仅好友有该字段）
  RemarkName string

  // 成员列表（仅群有该字段），
  // 只在调用Update后才有值，UserName->NickName
  Members map[string]string

  // 原始数据
  raw []byte
}

func buildContact(data []byte, bot *Bot) *Contact {
  if len(data) == 0 {
    return nil
  }
  ret := &Contact{bot: bot, attr: &sync.Map{}, raw: data}
  jsonparser.EachKey(data, func(i int, v []byte, _ jsonparser.ValueType, e error) {
    if e != nil {
      return
    }
    switch i {
    case 0:
      ret.UserName, _ = jsonparser.ParseString(v)
    case 1:
      ret.NickName, _ = jsonparser.ParseString(v)
    case 2:
      ret.RemarkName, _ = jsonparser.ParseString(v)
    case 3:
      n, _ := jsonparser.ParseInt(v)
      if n != 0 {
        ret.VerifyFlag = int(n)
      }
    case 4:
      cnt, _ := jsonparser.ParseInt(v)
      if cnt > 0 {
        ret.Members = make(map[string]string, cnt)
      }
    }
  }, jsonPathUserName, jsonPathNickName, jsonPathRemarkName, jsonPathVerifyFlag, jsonPathMemberCount)
  if ret.Members != nil {
    v, _, _, _ := jsonparser.Get(data, jsonKeyMemberList)
    if len(v) > 0 {
      buildMembers(v, ret.Members)
    }
  }
  switch ret.VerifyFlag {
  case 0:
    ret.Type = contactType(ret.UserName)
  case 8, 24:
    ret.Type = ContactMPS
  case 56:
    ret.Type = ContactSystem
  default:
    ret.Type = contactUnknown
  }
  return ret
}

func buildMembers(data []byte, m map[string]string) {
  jsonparser.ArrayEach(data, func(v []byte, _ jsonparser.ValueType, _ int, e error) {
    if e != nil {
      return
    }
    userName, _ := jsonparser.GetString(v, jsonKeyUserName)
    nickName, _ := jsonparser.GetString(v, jsonKeyNickName)
    if userName != "" {
      m[userName] = nickName
    }
  })
}

func GetContact(userName string) *Contact {
  if userName == "" {
    return nil
  }
  var ret *Contact
  EachBot(func(b *Bot) bool {
    if b.contacts != nil {
      if c := b.contacts.Get(userName); c != nil {
        ret = c
        return false
      }
    }
    return true
  })
  return ret
}

func (c *Contact) Bot() *Bot {
  return c.bot
}

func (c *Contact) Raw() []byte {
  return c.raw
}

func (c *Contact) Update() *Contact {
  ret, _ := c.bot.GetContactFromServer(c.UserName)
  if ret == nil {
    return nil
  }
  ret.bot.contacts.Add(ret)
  return ret
}

func (c *Contact) SendText(text string) error {
  if text == "" {
    return ErrInvalidArgs
  }
  return c.bot.sendText(c.UserName, text)
}

func (c *Contact) SendImage(data []byte, filename string) (string, error) {
  if len(data) == 0 || filename == "" {
    return "", ErrInvalidArgs
  }
  return c.bot.sendMedia(c.UserName, data, filename, MsgImage, sendImageUrlPath)
}

func (c *Contact) SendVideo(data []byte, filename string) (string, error) {
  if len(data) == 0 || filename == "" {
    return "", ErrInvalidArgs
  }
  return c.bot.sendMedia(c.UserName, data, filename, MsgVideo, sendVideoUrlPath)
}

func (c *Contact) SetAttr(attr interface{}, value interface{}) {
  c.attr.Store(attr, value)
}

func (c *Contact) GetAttr(attr interface{}, defaultValue interface{}) interface{} {
  if v, ok := c.attr.Load(attr); ok {
    return v
  }
  return defaultValue
}

func (c *Contact) GetAttrString(attr string, defaultValue string) string {
  if v, ok := c.attr.Load(attr); ok {
    return conv.String(v, defaultValue)
  }
  return defaultValue
}

func (c *Contact) GetAttrInt(attr string, defaultValue int) int {
  if v, ok := c.attr.Load(attr); ok {
    return conv.Int(v, defaultValue)
  }
  return defaultValue
}

func (c *Contact) GetAttrInt64(attr string, defaultValue int64) int64 {
  if v, ok := c.attr.Load(attr); ok {
    return conv.Int64(v, defaultValue)
  }
  return defaultValue
}

func (c *Contact) GetAttrUint(attr string, defaultValue uint) uint {
  if v, ok := c.attr.Load(attr); ok {
    return conv.Uint(v, defaultValue)
  }
  return defaultValue
}

func (c *Contact) GetAttrUint64(attr string, defaultValue uint64) uint64 {
  if v, ok := c.attr.Load(attr); ok {
    return conv.Uint64(v, defaultValue)
  }
  return defaultValue
}

func (c *Contact) GetAttrBool(attr string, defaultValue bool) bool {
  if v, ok := c.attr.Load(attr); ok {
    return conv.Bool(v)
  }
  return defaultValue
}

func contactType(userName string) int {
  switch {
  case len(userName) < 2:
    return contactUnknown
  case userName[0:2] == "@@":
    return ContactGroup
  case userName[0:1] == "@":
    return ContactFriend
  default:
    return ContactSystem
  }
}
