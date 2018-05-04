package main

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"pointTest/tcpProxy/tcpmvc"
	"strconv"
	"time"
)

func main() {
	pServer := &ProxyServer{tcpPort: 7000, httpPort: 7100}
	pServer.Start()
}

//代理服务器，提供代理管理、将HTTP分配到具体proxyWorker
type ProxyServer struct {
	tcpPort      int                        //tcp监听端口
	tcpListen    *net.TCPListener           //tcp监听链接
	httpPort     int                        //http监听端口
	tcpWorkers   []*tcpWorker               //连接上的TCP连接
	domainProxys map[string][]*domainWorker //代理的域名
	error_log    string                     //错误日志文件路径
	access_log   string                     //日志文件路径
	errorLog     *log.Logger                //错误日志
	accessFiel   *log.Logger                //日志
}

func (p *ProxyServer) Start() {
	//日志
	if p.error_log == "" {
		p.error_log = "error.log"
	}
	errorFile, err := os.OpenFile(p.error_log, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("打开错误日志文件(%s)失败:%s", p.error_log, err.Error())
	}
	p.errorLog = log.New(errorFile, "error:", log.LstdFlags|log.Lshortfile)

	//初始化
	p.tcpWorkers = make([]*tcpWorker, 0)
	p.domainProxys = make(map[string][]*domainWorker)
	//启动TCP监听
	fmt.Println("TCP 协议转发, 建立TCP转发服务...")

	p.tcpListen, err = net.ListenTCP("tcp", &net.TCPAddr{net.ParseIP("127.0.0.1"), p.tcpPort, ""})
	if err != nil {
		fmt.Fprintf(os.Stderr, "建立TCP：%d 监听失败：%s\n", p.tcpPort, err.Error())
		p.errorLog.Fatalf("建立TCP：%d 监听失败：%s\n", p.tcpPort, err.Error())
	}
	defer p.tcpListen.Close()
	fmt.Println("监听TCP端口" + strconv.Itoa(p.tcpPort) + "成功，等待客户端连接...")

	//启动HTTP监听
	go p.httpServer()
	go p.cmd()
	for {
		conn, err := p.tcpListen.AcceptTCP()
		if err != nil {
			fmt.Fprintf(os.Stderr, "连接错误：%s", err.Error())
			continue
		}
		fmt.Println("已连接：" + conn.RemoteAddr().String() + " " + time.Now().Format("15:04:05\n"))
		go p.handleCoon(conn)
	}
}

func (p *ProxyServer) cmd() {
	for {
		var msg string
		_, err := fmt.Scanln(&msg)
		if err != nil {
			fmt.Println("接收输入错误：" + err.Error())
			continue
		}
		switch msg {
		case "stop":
			p.stop()
		case "status":
			p.status()
		default:
			fmt.Println("错误命令:" + msg)
		}
	}
}

func (p *ProxyServer) stop() {
	fmt.Println("stop")
}

func (p *ProxyServer) status() {
	var tcpSatus string
	if p.tcpListen != nil {
		tcpSatus = "运行中"
	} else {
		tcpSatus = "未运行"
	}
	fmt.Printf("监听TCP端口:%d,%s\n", p.tcpPort, tcpSatus)
	fmt.Println("\n连接上的TCP：")
	for x, v := range p.tcpWorkers {
		fmt.Printf("%d:%s\n", x, v.conn.RemoteAddr().String())
	}
	fmt.Println("\n代理的domain：")
	for x, sliDomain := range p.domainProxys {
		fmt.Printf("%s,主机数量：%d\n", x, len(sliDomain))
		for _, d := range sliDomain {
			fmt.Printf("    %s\n", d.tcpW.conn.RemoteAddr().String())
		}
	}
}

func (p *ProxyServer) handleCoon(c *net.TCPConn) {
	tcpW := NewTcpWorker()
	tcpW.conn = c
	tcpW.server = p
	defer func() {
		pan := recover()
		if pan != nil {
			fmt.Printf("recover:%v\n", pan)
			p.errorLog.Printf("%s连接意外断开\n", c.RemoteAddr().String(), pan)
			panic(pan) //这句已没必要，已经到了goroutine的末尾
		} else {
			fmt.Printf("%s客户短失去连接\n", c.RemoteAddr().String())
		}
		sTcp := c.RemoteAddr().String()
		err := p.deTcpWorker(tcpW)
		if err != nil {
			p.errorLog.Printf("关闭tcpWorker %s 失败：%s\n", sTcp, err.Error())
		} else {
			fmt.Printf("关闭tcpWorker %s 成功\n", sTcp)
		}
	}()
	mvc := tcpmvc.New(c)
	mvc.Include(tcpW)
	tcpW.tmvc = mvc
	p.tcpWorkers = append(p.tcpWorkers, tcpW)
	tcpW.Welcome()
	//监听并分发消息
	mvc.StartHandle()
}

func (p *ProxyServer) httpServer() {
	fmt.Fprintf(os.Stderr, "启动HTTP监听，端口：%d\n", p.httpPort)
	http.HandleFunc("/", p.httpHandleFunc)
	err := http.ListenAndServe("127.0.0.1:"+strconv.Itoa(p.httpPort), nil) //设置监听的端口
	if err != nil {
		fmt.Fprintf(os.Stderr, "监听HTTP:%d端口失败%s\n", p.httpPort, err.Error())
		os.Exit(1)
	} else {
		fmt.Fprintf(os.Stderr, "监听HTTP:%d端口成功.\n", p.httpPort)
	}
}

//分发用户HTTP请求
func (p *ProxyServer) httpHandleFunc(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("HTTP请求：%s\n", r.Host+r.URL.Path)
	//将HTTP连接发送到TCP中去
	tcps, ok := p.domainProxys[r.Host]
	if !ok || len(tcps) == 0 {
		fmt.Println("没有tcp后台")
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	//随机一个代理处理
	ra := rand.New(rand.NewSource(time.Now().UnixNano()))
	count := ra.Intn(len(tcps))
	tcps[count].httpHandleFunc(w, r)
	return
}

//注册代理域名
func (p *ProxyServer) registerDomain(domain string, doWorker *domainWorker) error {
	old, ok := p.domainProxys[domain]
	if !ok {
		sliceDomainProxys := make([]*domainWorker, 0)
		sliceDomainProxys = append(sliceDomainProxys, doWorker)
		p.domainProxys[domain] = sliceDomainProxys
	} else {
		for _, v := range old {
			if v.tcpW == doWorker.tcpW {
				return errors.New("该net.TCPConn已存在相同域名的代理")
			}
		}
		p.domainProxys[domain] = append(old, doWorker)
	}
	return nil
}

//删除tcpWorker实例
func (p *ProxyServer) deTcpWorker(tcp *tcpWorker) error {
	l := len(p.tcpWorkers)
	if l == 0 {
		return errors.New("tcpWorkers已为空，不能再删除")
	}
	var isde bool = false
	for k, v := range p.tcpWorkers {
		if v == tcp {
			if k == l-1 {
				p.tcpWorkers = p.tcpWorkers[:k]
			} else {
				p.tcpWorkers = append(p.tcpWorkers[:k], p.tcpWorkers[k+1:]...)
			}
			isde = true
			break
		}
	}
	if !isde {
		return errors.New("已为空，不能再删除")
	}
	err := tcp.conn.Close()
	if err != nil {
		_, errOut := fmt.Scanf("关闭TCPConn %s 失败：%s\n", tcp.conn.RemoteAddr().String(), err.Error())
		return errOut
	}
	return nil
}
