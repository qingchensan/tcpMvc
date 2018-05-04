package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
)

//每次TCP传输的数据结构由：TAG+数据长度+数据本身 组成
const (
	N_TAG int    = 4      //标识长度
	TAG   string = "tag|" //标识字符串，用来判断是否是支持本方法读取解析
)

const (
	StatusTCPLose         string = "TPC连接丢失;"
	StatusReadError       string = "读取数据失败;"
	StatusReadOver        string = "无更多数据可获取;"
	StatusUnkonwTag       string = "无法解析此数据;"
	StatusDataLengthLost  string = "无法解析数据大小;"
	StatusDataLengthZero  string = "数据长度为0;"
	StatusDataLengthError string = "数据读取不完整;"
)

//从TCP中读取本次数据
func readTcpTag(c *net.TCPConn) ([]byte, error) {
	if c == nil {
		return nil, errors.New(StatusTCPLose)
	}
	tag := make([]byte, N_TAG)
	_, err := c.Read(tag)
	if err != nil {
		if err.Error() == "EOF" {
			return nil, errors.New(StatusReadOver)
		}
		return nil, errors.New(StatusReadError + err.Error())
	}
	stag := string(tag)
	switch stag {
	case TAG:
		_, err := c.Read(tag)
		if err != nil {
			return nil, errors.New(StatusDataLengthLost)
		}
		lbody := binary.LittleEndian.Uint32(tag)
		if lbody == 0 {
			return nil, errors.New(StatusDataLengthZero)
		}
		raw := make([]byte, lbody)
		lraw, err := c.Read(raw)
		if err != nil {
			return nil, errors.New(StatusReadError + err.Error())
		}
		if uint32(lraw) != lbody {
			return nil, errors.New(StatusDataLengthError)
		}
		return raw, nil
	default:
		return nil, errors.New(StatusUnkonwTag)
	}
	return nil, errors.New(StatusUnkonwTag)
}

//给数据添加标签和长度后发送
func writeTcpTag(c *net.TCPConn, data []byte) error {
	raw := bytes.NewBuffer([]byte{})
	// 写签名
	binary.Write(raw, binary.LittleEndian, []byte(TAG))
	// 因为要占一个位置，防止data为空时破坏数据结构
	binary.Write(raw, binary.LittleEndian, int32(len(data)+1))
	binary.Write(raw, binary.LittleEndian, data)
	l, err := c.Write(raw.Bytes())
	if err != nil {
		return err
	}
	if l != len(raw.Bytes()) {
		return errors.New("写出长度与字节长度不一致。")
	}
	return nil
}

//给数据添加标签和长度后发送
func tcpWrite(c *net.TCPConn, data []byte) error {
	raw := bytes.NewBuffer([]byte{})
	// 写签名
	binary.Write(raw, binary.LittleEndian, []byte(TAG))
	// 因为要占一个位置，防止data为空时破坏数据结构
	binary.Write(raw, binary.LittleEndian, int32(len(data)+1))
	binary.Write(raw, binary.LittleEndian, data)
	l, err := c.Write(raw.Bytes())
	if err != nil {
		return err
	}
	if l != len(raw.Bytes()) {
		return errors.New("写出长度与字节长度不一致。")
	}
	return nil
}
