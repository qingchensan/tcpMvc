package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"pointTest/tcpProxy/tcpmvc"
)

func main() {
	coon, err := net.Dial("tcp", "localhost:7000")
	if err != nil {
		fmt.Println("连接代理服务器失败.")
		return
	} else {
		fmt.Println("连接代理服务器成功.")
	}
	defer coon.Close()
	mvc := tcpmvc.New(coon.(*net.TCPConn))
	tWorker := &tcpWorker{conn: coon.(*net.TCPConn), domain: "127.0.0.1:7100", proxyDomain: "127.0.0.1:8030"}
	tWorker.mvc = mvc
	mvc.Include(tWorker)
	go func() {
		mvc.StartHandle() //监听并分发来自server的消息
		fmt.Println("与服务器失去连接。")
	}()
	tWorker.RegisterDomain()
	//接收控制台消息
	for {
		var msg string
		fmt.Scanln(&msg)
		if msg == "quit" {
			break
		}
		b := []byte(msg)
		data := tcpmvc.NewData()
		data.Model = "tcpWorker"
		data.Method = "Message"
		data.Args["msg"] = b
		err = mvc.Write(data)
		if err != nil {
			fmt.Printf("发送错误：%s\n", err.Error())
		}
	}
}

type tcpWorker struct {
	conn        *net.TCPConn
	mvc         *tcpmvc.Mvc
	error_log   string      //错误日志文件路径
	access_log  string      //日志文件路径
	errorLog    *log.Logger //错误日志
	accessFiel  *log.Logger //日志
	domain      string      //代理的域名
	proxyDomain string      //转发本地的域名
}

//来自proxy的普通消息
func (t *tcpWorker) Message(args map[string][]byte) {
	fmt.Printf("来自代理消息：%s\n", args["msg"])
}

func (t *tcpWorker) RegisterDomain() {
	data := tcpmvc.NewData()
	data.Model = "tcpWorker"
	data.Method = "Register"
	data.Args["domain"] = []byte(t.domain)
	err := t.mvc.Write(data)
	if err != nil {
		fmt.Printf("发送消息错误：%s\n", err.Error())
		return
	}
	fmt.Printf("注册域名：%s\n", t.domain)
}

//来自proxy的http请求
func (t *tcpWorker) HttpRequest(args map[string][]byte) {
	fmt.Println("来自proxy的http请求")
	reqBytes, ok := args["request"]
	data := tcpmvc.NewData()
	data.Model = "tcpWorker"
	data.Method = "HttpResponse"
	data.Args = args
	if !ok {
		data.Args["status"] = []byte("400")
		data.Args["msg"] = []byte("未包含request参数")
		t.mvc.Write(data)
		return
	}
	req, err := http.ReadRequest(bufio.NewReader(bytes.NewBuffer(reqBytes)))
	if err != nil {
		fmt.Println("http.ReadRequest失败：" + err.Error())
		data.Args["status"] = []byte("400")
		data.Args["msg"] = []byte("解析request失败：" + err.Error())
		t.mvc.Write(data)
		return
	}
	req.Host = t.proxyDomain
	req.URL, _ = url.Parse(fmt.Sprintf("http://%s%s", req.Host, req.RequestURI))
	req.RequestURI = ""
	client := new(http.Client)
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("client.Do失败：" + err.Error())
		data.Args["status"] = []byte("500")
		data.Args["msg"] = []byte("client.Do失败：" + err.Error())
		t.mvc.Write(data)
		if err != nil {
			fmt.Println("向服务端回写HTTP结果失败0")
		}
		return
	}
	defer resp.Body.Close()
	data.Args["status"] = []byte(resp.Status)
	data.Args["resp"], err = httputil.DumpResponse(resp, true)
	if err != nil {
		fmt.Println("编码request失败：" + err.Error())
		data.Args["status"] = []byte("500")
		data.Args["msg"] = []byte("编码request失败：" + err.Error())
		err = t.mvc.Write(data)
		if err != nil {
			fmt.Println("向服务端回写HTTP结果失败1")
		}
		return
	}
	data.Args["status"] = []byte("200")
	err = t.mvc.Write(data)
	if err != nil {
		fmt.Println("向服务端回写HTTP结果失败2")
	}
	return
}
