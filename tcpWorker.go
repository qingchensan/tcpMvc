package main

import (
	"fmt"
	"net"
	"pointTest/tcpProxy/tcpmvc"
)

type tcpWorker struct {
	conn    *net.TCPConn
	server  *ProxyServer //所属的ProxyServer
	tmvc    *tcpmvc.Mvc  //所关联的mvc对象
	domains map[string]*domainWorker
}

func NewTcpWorker() *tcpWorker {
	t := &tcpWorker{}
	t.domains = make(map[string]*domainWorker)
	return t
}

//来自后端的普通消息
func (p *tcpWorker) Message(args map[string][]byte) {
	fmt.Printf("来自后端消息：%s\n", args["msg"])
}

func (p *tcpWorker) Welcome() {
	data := tcpmvc.NewData()
	data.Model = "tcpWorker"
	data.Method = "Message"
	data.Args["msg"] = []byte("show me domain")
	err := p.tmvc.Write(data)
	if err != nil {
		fmt.Println("向后端写入数据失败" + err.Error())
	} else {
		fmt.Println("向后端写入数据成功")
	}
}

//来自后端的HTTP回复
func (p *tcpWorker) HttpResponse(args map[string][]byte) {
	domain, ok := args["domain"]
	if !ok {
		fmt.Println("tcpWorker-HttpResponse:索引domain无法找到")
		return
	}
	worker, ok := p.domains[string(domain)]
	if !ok {
		fmt.Printf("tcpWorker-HttpResponse:无该域名（%s）代理\n", domain)
		return
	}
	worker.httpResponse(args)
}

//domain注册
func (p *tcpWorker) Register(args map[string][]byte) {
	outData := tcpmvc.NewData()
	outData.Model = "tcpWorker"
	outData.Method = "Message"
	domain, ok := args["domain"]
	if !ok {
		outData.Args["msg"] = []byte("参数缺少domain")
		p.tmvc.Write(outData)
		return
	}
	//在本tcpWorker注册
	sDomain := string(domain)
	_, ok = p.domains[sDomain]
	if ok {
		outData.Args["msg"] = []byte("已注册过该域名：" + sDomain)
		p.tmvc.Write(outData)
		return
	}
	dWorker := &domainWorker{domain: sDomain, tcpW: p, respWriters: make(map[uint32]chan map[string][]byte)}
	dWorker.domain = sDomain
	dWorker.tcpW = p
	p.domains[sDomain] = dWorker
	//在tcpProxy注册
	err := p.server.registerDomain(sDomain, dWorker)
	if err != nil {
		outData.Args["msg"] = []byte("注册域名失败：" + err.Error())
		p.tmvc.Write(outData)
		return
	}
	outData.Args["msg"] = []byte("成功注册域名：" + sDomain)
	p.tmvc.Write(outData)
	return
}
