package main

import (
  "bytes"
  "fmt"
  "html"
  "os/exec"
  "runtime"
  "time"

  "github.com/kwf2030/commons/conv"
  "github.com/kwf2030/wechatbot"
)

func main() {
  wechatbot.SetLogLevel("debug")
  bot := wechatbot.CreateBot(false)

  // 用来接收二维码
  qrChan := make(chan string)
  go func(c chan string) {
    // 读取到的是完整的二维码地址
    <-c

    // 下载二维码，
    // 参数为完整路径，如果为空就下载到系统临时目录，
    // 返回二维码图片的完整路径
    p, e := bot.DownloadQRCode("")
    if e != nil {
      panic(e)
    }

    switch runtime.GOOS {
    case "windows":
      e = exec.Command("cmd.exe", "/c", p).Start()
    case "linux":
      e = exec.Command("eog", p).Start()
    default:
      fmt.Printf("QR Code saved to [%s], please open it manually", p)
    }
  }(qrChan)

  // 启动Bot，会一直阻塞到登录成功
  ch, e := bot.Start(qrChan)
  if e != nil {
    println(e.Error())
    return
  }

  // 登录成功后即可开始接收消息，
  // 不要阻塞消息接收的channel（缓存大小为NumCPU+1）
  for op := range ch {
    go dispatch(op, bot)
  }

  var buf bytes.Buffer
  buf.WriteString("WeChatBot[%s] run stat:\n")
  buf.WriteString("  started at: %s\n")
  buf.WriteString("  stopped at: %s\n")
  buf.WriteString("  totally online for %.2f hours\n")
  fmt.Printf(buf.String(), bot.Self.Nickname, bot.StartTimeStr, bot.StopTimeStr, bot.StopTime.Sub(bot.StartTime).Hours())

  // 完全退出前最好等待1秒左右时间，以便退出请求和各种清理操作完成
  time.Sleep(time.Second)
}

func dispatch(op *wechatbot.Op, bot *wechatbot.Bot) {
  if op.What != wechatbot.MsgOp {
    return
  }
  msg := op.Msg
  fmt.Printf("[*]MsgType=%d\n%s\n", msg.Type, html.UnescapeString(msg.Content))

  var reply string
  switch msg.Type {
  case wechatbot.MsgText:
    reply = "收到文本"

  case wechatbot.MsgImage:
    reply = "收到图片"

  case wechatbot.MsgAnimEmotion:
    reply = "收到动画表情"

  case wechatbot.MsgLink:
    reply = "收到链接"

  case wechatbot.MsgCard:
    reply = "收到名片"

  case wechatbot.MsgLocation:
    reply = "收到位置"

  case wechatbot.MsgVoice:
    reply = "收到语音"

  case wechatbot.MsgVideo:
    reply = "收到视频"

  case wechatbot.MsgVerify:
    // 自动通过验证、备注并添加到联系人
    info := msg.Raw["RecommendInfo"].(map[string]interface{})
    _, e := bot.VerifyAndRemark(conv.String(info, "UserName"), conv.String(info, "Ticket"))
    if e != nil {
      println(e.Error())
      return
    }
    reply = "你好，朋友"
  }

  if reply == "" {
    return
  }

  e := msg.ReplyText(reply)
  if e != nil {
    println(e.Error())
  }
}
