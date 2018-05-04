package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"pointTest/tcpProxy/tcpmvc"
	"sync"
)

//处理代理服务的具体对象，
//管理与后端服务器的链接,将HTTP请求转为TCP请求与后端应用服务器交换数据
type domainWorker struct {
	domain      string                            //对应的域名
	tcpW        *tcpWorker                        //对应的tcpWorker
	respWriters map[uint32]chan map[string][]byte //等待返回的HTTP请求
	requestId   uint32                            //请求id标识
	mu          sync.Mutex
}

//向用户返回HTTP请求结果
func (p *domainWorker) httpResponse(args map[string][]byte) {

	requestIdByte, ok := args["requestId"]
	if !ok {
		fmt.Println("domainWorker-httpResponse:args索引requestId无法找到.")
		return
	}
	requestId := binary.LittleEndian.Uint32(requestIdByte)
	ch, ok := p.respWriters[requestId]
	if !ok {
		fmt.Println("domainWorker-httpResponse:respWriters索引" + string(requestIdByte) + "无法找到.")
		return
	}
	ch <- args
	return
}

//来自用户的HTTP请求
func (p *domainWorker) httpHandleFunc(w http.ResponseWriter, r *http.Request) {
	if p.tcpW.conn == nil {
		fmt.Println("domainWorker-httpHandleFunc:p.tcpW.conn")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	reqBytes, err := httputil.DumpRequest(r, true)
	if err != nil {
		fmt.Println("domainWorker-httpHandleFunc:httputil.DumpRequest = err")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	//向应用服务器发送HTTP处理请求
	data := tcpmvc.NewData()
	data.Model = "tcpWorker"
	data.Method = "HttpRequest"
	data.Args["domain"] = []byte(r.Host)
	data.Args["request"] = reqBytes
	requestId := p.getRequestId()
	data.Args["requestId"] = make([]byte, 4)
	binary.LittleEndian.PutUint32(data.Args["requestId"], requestId)
	err = p.tcpW.tmvc.Write(data)
	if err != nil {
		fmt.Println("domainWorker-httpHandleFunc:p.tcpW.tmvc.Write = " + err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ch := make(chan map[string][]byte)
	p.respWriters[requestId] = ch
	select {
	case args := <-ch:
		status, ok := args["status"]
		if !ok {
			fmt.Println("domainWorker-httpResponse:args索引status无法找到.")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if string(status) != "200" {
			fmt.Println("status != 200")
			http.Error(w, string(args["msg"]), http.StatusNotFound)
			return
		}
		responseByte, ok := args["resp"]
		if !ok {
			fmt.Println("domainWorker-httpResponse:args索引resp无法找到.")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		byteReader := bytes.NewReader(responseByte)
		bufioReader := bufio.NewReader(byteReader)
		response, err := http.ReadResponse(bufioReader, nil)
		bodyBytes, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return
		}
		for k, v := range response.Header {
			for _, v2 := range v {
				w.Header().Add(k, v2)
			}
		}
		w.WriteHeader(response.StatusCode)
		wLength, err := w.Write(bodyBytes)
		if err != nil {
			return
		}
		if wLength != len(bodyBytes) {
			fmt.Println("写入数据不完整")
		}
	}
	delete(p.respWriters, requestId)
	close(ch)
	fmt.Printf("count domainWorker.respWriters %d\n", len(p.respWriters))
	return
}

func (p *domainWorker) getRequestId() uint32 {
	p.mu.Lock()
	p.requestId++
	p.mu.Unlock()
	return p.requestId
}
