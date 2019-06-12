package wxweb

import (
  "strings"
  "sync"
)

type Contacts struct {
  data map[string]*Contact
  mu   *sync.RWMutex
  bot  *Bot
}

func initContacts(contacts []*Contact, bot *Bot) *Contacts {
  ret := &Contacts{
    data: make(map[string]*Contact, 5000),
    mu:   &sync.RWMutex{},
    bot:  bot,
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
  cs.data[c.UserName] = c
  cs.mu.Unlock()
}

func (cs *Contacts) Remove(userName string) {
  if userName == "" {
    return
  }
  cs.mu.Lock()
  delete(cs.data, userName)
  cs.mu.Unlock()
}

func (cs *Contacts) Get(userName string) *Contact {
  if userName == "" {
    return nil
  }
  cs.mu.RLock()
  ret := cs.data[userName]
  cs.mu.RUnlock()
  return ret
}

func (cs *Contacts) Find(keyword string) *Contact {
  if keyword == "" {
    return nil
  }
  cs.mu.RLock()
  var ret *Contact
  for _, c := range cs.data {
    if (c.NickName != "" && strings.Contains(c.NickName, keyword)) ||
      (c.RemarkName != "" && strings.Contains(c.RemarkName, keyword)) {
      ret = c
      break
    }
  }
  cs.mu.RUnlock()
  return ret
}

func (cs *Contacts) Count() int {
  cs.mu.RLock()
  ret := len(cs.data)
  cs.mu.RUnlock()
  return ret
}

func (cs *Contacts) Each(f func(*Contact) bool) {
  cs.mu.RLock()
  arr := make([]*Contact, 0, len(cs.data))
  for _, c := range cs.data {
    arr = append(arr, c)
  }
  cs.mu.RUnlock()
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
