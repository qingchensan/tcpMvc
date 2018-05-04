package tcpmvc

import (
	"fmt"
	"net"
	"strconv"
	"testing"
)

func Test(t *testing.T) {
	tcp := &net.TCPConn{}
	mvc := New(tcp)
	a := &aa{}
	a.File1 = 89
	mvc.Include(a)
	mvc.Models["aa"]["Ff"].Call(nil)
	fmt.Println(mvc)
}

type aa struct {
	File1 int
}

func (a *aa) Ff() {
	fmt.Println("file1=" + strconv.Itoa(a.File1))
}
