package wxweb

import (
  "strings"
  "sync"
)

type Contacts struct {
  data map[string]*Contact
  bot  *Bot
  mu   sync.RWMutex
}

func initContacts(contacts []*Contact, bot *Bot) *Contacts {
  ret := &Contacts{
    data: make(map[string]*Contact, 5000),
    bot:  bot,
    mu:   sync.RWMutex{},
  }
  for _, c := range contacts {
    c.bot = bot
    ret.data[c.UserName] = c
  }
  return ret
}

func (cs *Contacts) Add(c *Contact) {
  if c == nil || c.UserName == "" {
    return
  }
  cs.mu.Lock()
  defer cs.mu.Unlock()
  cs.data[c.UserName] = c
}

func (cs *Contacts) Remove(userName string) {
  if userName == "" {
    return
  }
  cs.mu.Lock()
  defer cs.mu.Unlock()
  delete(cs.data, userName)
}

func (cs *Contacts) Get(userName string) *Contact {
  if userName == "" {
    return nil
  }
  cs.mu.RLock()
  defer cs.mu.RUnlock()
  ret := cs.data[userName]
  return ret
}

func (cs *Contacts) Find(keyword string) *Contact {
  if keyword == "" {
    return nil
  }
  cs.mu.RLock()
  defer cs.mu.RUnlock()
  var ret *Contact
  for _, c := range cs.data {
    if (c.NickName != "" && strings.Contains(c.NickName, keyword)) ||
      (c.RemarkName != "" && strings.Contains(c.RemarkName, keyword)) {
      ret = c
      break
    }
  }
  return ret
}

func (cs *Contacts) Count() int {
  cs.mu.RLock()
  defer cs.mu.RUnlock()
  ret := len(cs.data)
  return ret
}

func (cs *Contacts) Each(f func(*Contact) bool) {
  cs.mu.RLock()
  defer cs.mu.RUnlock()
  arr := make([]*Contact, 0, len(cs.data))
  for _, c := range cs.data {
    arr = append(arr, c)
  }
  for _, c := range arr {
    if !f(c) {
      break
    }
  }
}

func (cs *Contacts) EachLocked(f func(*Contact) bool) {
  cs.mu.RLock()
  defer cs.mu.RUnlock()
  for _, c := range cs.data {
    if !f(c) {
      break
    }
  }
}
